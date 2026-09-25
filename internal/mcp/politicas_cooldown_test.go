package mcp

// A131 · T4 — EL COOLDOWN SE CUENTA DESDE EL DISPARO, SALGA COMO SALGA LA ACCIÓN, Y LO LEEN IGUAL LA
// DECISIÓN, LA BASE, EL REINICIO Y EL INVENTARIO. LA PODA SÓLO SE LLEVA LO QUE YA NO ESTÁ CONFIGURADO.
//
// Las guardas de antes decían bien cada invariante y lo medían sobre UN desenlace: la acción siempre
// salía bien (Tier A, cola vacía), la política era de host, la tabla de estado arrancaba limpia, el
// inventario se miraba sólo ANTES de que la política actuara, la lista de vivas traía una política o
// venía nil, y las homónimas diferían sólo en `run`. Ocho mutaciones de A131 dejaban los paquetes en
// verde (medido sobre la base de T4, 5b19840: dos caían sólo en el censo, por el ancla movida, y eso
// no es una guarda). Las pruebas de este archivo recorren esos ejes enteros, leídos del código donde
// el conjunto es cerrado —las condiciones y los tiers (AST), los desenlaces que actuarSiCorresponde
// cuenta después de marcar el cooldown (AST), los campos de una política (reflect)— y comparan contra
// hechos escritos en cada fila, no contra lo que calcula el código que miden.
//
// La poda vista desde el almacén —cada forma de la lista vacía, cada cantidad de vivas— la recorre
// internal/memory/politicas_poda_test.go.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// resultadosTrasElDisparo devuelve los resultados que actuarSiCorresponde cuenta DESPUÉS de marcar el
// cooldown en memoria —los literales de `contarPolitica` que siguen a `s.ultimoDisparo.Store`—, leídos
// del AST de politicas.go, sin repetir y en orden. Es el conjunto CERRADO de desenlaces de una
// decisión de actuar: hoy `error` y `ok`. Un desenlace nuevo después de la marca entra solo a la tabla
// del cooldown, y si la tabla no sabe provocarlo se pone roja en vez de saltearlo.
func resultadosTrasElDisparo(t *testing.T) []string {
	t.Helper()
	const fuente = "politicas.go"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, fuente, nil, 0)
	if err != nil {
		t.Fatalf("no se pudo parsear %s: %v", fuente, err)
	}
	var cuerpo *ast.BlockStmt
	for _, decl := range f.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name.Name == "actuarSiCorresponde" && fd.Body != nil {
			cuerpo = fd.Body
		}
	}
	if cuerpo == nil {
		t.Fatalf("no encontré actuarSiCorresponde en %s: la tabla del cooldown no sabe qué desenlaces recorrer", fuente)
	}
	marca := token.NoPos
	ast.Inspect(cuerpo, func(n ast.Node) bool {
		llamada, ok := n.(*ast.CallExpr)
		if !ok || marca != token.NoPos {
			return marca == token.NoPos
		}
		sel, ok := llamada.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Store" {
			return true
		}
		if receptor, ok := sel.X.(*ast.SelectorExpr); ok && receptor.Sel.Name == "ultimoDisparo" {
			marca = llamada.Pos()
		}
		return true
	})
	if marca == token.NoPos {
		t.Fatalf("no encontré `s.ultimoDisparo.Store` en actuarSiCorresponde: sin la marca no sé qué se cuenta después de decidir")
	}
	vistos := map[string]bool{}
	var out []string
	ast.Inspect(cuerpo, func(n ast.Node) bool {
		llamada, ok := n.(*ast.CallExpr)
		if !ok || llamada.Pos() < marca || len(llamada.Args) < 2 {
			return true
		}
		sel, ok := llamada.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "contarPolitica" {
			return true
		}
		lit, ok := llamada.Args[1].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			t.Errorf("%s: `contarPolitica` recibe un resultado que no es un literal de cadena: esta tabla deriva los "+
				"desenlaces del código, y con una variable ese camino queda sin medir sin que nada lo diga", fset.Position(llamada.Pos()))
			return true
		}
		v, err := strconv.Unquote(lit.Value)
		if err != nil {
			t.Fatalf("%s: %v", fset.Position(lit.Pos()), err)
		}
		if !vistos[v] {
			vistos[v] = true
			out = append(out, v)
		}
		return true
	})
	// PISO: hoy son dos. Menos es que la cuenta se mudó de función o cambió de nombre, y la tabla
	// volvería a medir un solo desenlace, que es el eje que clavaba la guarda vieja.
	if len(out) < 2 {
		t.Fatalf("después de la marca actuarSiCorresponde cuenta %v, y son al menos dos (`ok` y `error`): la lectura "+
			"quedó mirando al aire y la tabla del cooldown mediría un solo desenlace", out)
	}
	sort.Strings(out)
	return out
}

// tiersQueActuan son los tiers declarados (tiersDeclarados, AST de internal/fleet/device.go) que
// admiten `exec`. Sobre los demás una política no llega nunca a la marca —la compuerta la frena antes
// (frenoSinExec)— y no hay cooldown que contar.
func tiersQueActuan(t *testing.T) []fleet.Tier {
	t.Helper()
	var out []fleet.Tier
	for _, tier := range tiersDeclarados(t) {
		if fleet.TierAdmite(tier, fleet.CapExec) {
			out = append(out, tier)
		}
	}
	// PISO: hoy son dos, y son los dos caminos de la acción: encolar para el agente (A) y correr por ssh
	// dentro del barrido (B). Con uno solo, el cooldown quedaría medido sobre un transporte.
	if len(out) < 2 {
		t.Fatalf("sólo %v admiten `exec`, y la tabla del cooldown necesita los dos transportes de la acción: el tier "+
			"cambió de nombre o de capacidades y el eje quedó a medias", out)
	}
	return out
}

// transporteDeTier es por dónde le llega la acción de una política a una máquina de ese tier. Es un
// HECHO escrito (correrAccionDePolitica): la tabla mide que el cooldown no dependa del camino, así que
// el camino no puede salir del código que lo recorre.
type transporteDeTier struct {
	// direccion es adonde saldría el ssh. Vacía en un Tier A: su acción se encola y la levanta el agente
	// en su próximo latido.
	direccion string
	// sincrono dice si la acción corre DENTRO del barrido (correrPorSSH). Es el único camino donde «a
	// mitad de la acción» es un momento que existe y se puede mirar: el del comentario de
	// actuarSiCorresponde, «un cerebro que se cae a mitad».
	sincrono bool
}

// transportesPorTier dice, por tier, por dónde sale la acción.
func transportesPorTier() map[fleet.Tier]transporteDeTier {
	return map[fleet.Tier]transporteDeTier{
		fleet.TierAgente: {},
		// `.invalid` es un dominio reservado que no resuelve nunca, y además el ssh va doblado
		// (SSHFalsoParaTest): ninguna fila sale a la red.
		fleet.TierProtocolo: {direccion: "curador@maquina.invalid", sincrono: true},
	}
}

// desenlaceDeLaAccion es cómo se provoca un resultado que actuarSiCorresponde cuenta después de la
// marca, y si con él la acción llega a salir por su transporte. Las dos cosas son hechos de la fila.
type desenlaceDeLaAccion struct {
	// provocar deja el mundo listo para ese desenlace; nil si no hace falta nada.
	provocar func(t *testing.T, s *McpServer, d fleet.Device)
	// llegaAlTransporte dice si la acción sale —se encola, o corre por ssh— o se cae antes.
	llegaAlTransporte bool
}

// desenlacesPorResultado dice cómo se provoca cada desenlace de la acción.
func desenlacesPorResultado() map[string]desenlaceDeLaAccion {
	return map[string]desenlaceDeLaAccion{
		"ok": {llegaAlTransporte: true},
		// La cola llena es el fallo que se puede provocar sin romper la base: EncolarComando devuelve
		// ErrColaLlena ANTES de escribir nada, en los dos tiers. Es el caso del manifiesto (P1-m10): un
		// agente que no levanta su cola, que es justo cuando algo ya va mal.
		"error": {provocar: llenarLaColaDe, llegaAlTransporte: false},
	}
}

// llenarLaColaDe le deja a la máquina fleet.ColaMaxPorDevice comandos pendientes de una persona: lo
// que ve el techo de la cola cuando el agente no los levanta.
func llenarLaColaDe(t *testing.T, s *McpServer, d fleet.Device) {
	t.Helper()
	for i := 0; i < fleet.ColaMaxPorDevice; i++ {
		if _, err := s.engine.EncolarComando(fleet.Comando{
			DeviceID: d.ID, ProjectID: d.ProjectID, Principal: "op", Origen: fleet.OrigenPersona,
			Argv: []string{"uptime"}, Timeout: fleet.ComandoTimeoutDefault,
		}); err != nil {
			t.Fatalf("llenando la cola de %s (%d de %d): %v", d.Name, i+1, fleet.ColaMaxPorDevice, err)
		}
	}
}

// curadorDeLaTablaDeCooldown puede correr los comandos de las dos clases de receta (disparosPorCondicion)
// en toda la casa: lo que se mide es el cooldown, y ninguna fila puede quedarse sin llegar a la marca
// por la allowlist o por la compuerta.
func curadorDeLaTablaDeCooldown() Principal {
	return Principal{
		Name: "curador", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet:     map[fleet.Cap][]string{fleet.CapExec: {"*"}, fleet.CapMetrics: {"*"}},
		ExecAllow: map[string][]string{"*": {"journalctl", "systemctl"}},
	}
}

// maquinaQueActuaPor da de alta en la casa una máquina del tier dado, con metrics y exec, y con la
// dirección de su transporte si la tiene.
func maquinaQueActuaPor(t *testing.T, s *McpServer, tier fleet.Tier, tr transporteDeTier) fleet.Device {
	t.Helper()
	nombre := "maquina-" + strings.ToLower(string(tier))
	args := map[string]any{
		"name": nombre, "tier": string(tier), "project": "casa", "os": "linux",
		"caps": []string{string(fleet.CapMetrics), string(fleet.CapExec)},
	}
	if tr.direccion != "" {
		args["address"] = tr.direccion
	}
	if _, e := call(t, s, "musubi_fleet_enroll", args); e != nil {
		t.Fatalf("alta de %q (tier %s): %+v", nombre, tier, e)
	}
	d, hay, err := s.engine.DevicePorNombre("casa", nombre)
	if err != nil || !hay || d.Tier != tier || d.Address != tr.direccion {
		t.Fatalf("%q no quedó como se pidió (tier %s, dirección %q): %+v hay=%v err=%v", nombre, tier, tr.direccion, d, hay, err)
	}
	return d
}

// desenlacesContados lee lo que la métrica de la política contó: cuántas veces pasó la marca, por
// resultado (sólo los de `desenlaces`), y cuántas la frenó otra cosa (el resto de lo sembrado).
func desenlacesContados(s *McpServer, politica string, desenlaces []string) (porResultado map[string]int64, decisiones, frenadas int64) {
	porResultado = map[string]int64{}
	for _, r := range resultadosDePolitica {
		n := contarPoliticaOK(s, politica, r)
		if slices.Contains(desenlaces, r) {
			porResultado[r] = n
			decisiones += n
		} else {
			frenadas += n
		}
	}
	return porResultado, decisiones, frenadas
}

// ultimoDisparoPublicado lee el `ultimo_disparo` que musubi_fleet_list publica para UNA política sobre
// UNA máquina, como lo lee una persona con exec sobre toda la casa.
func ultimoDisparoPublicado(t *testing.T, s *McpServer, maquina, politica string) any {
	t.Helper()
	fila, hay := filasDelInventario(t, s)[maquina]
	if !hay {
		t.Fatalf("la máquina %q no figura en el inventario", maquina)
	}
	detalle, _ := fila["politicas"].([]any)
	for _, x := range detalle {
		if p, _ := x.(map[string]any); p["nombre"] == politica {
			return p["ultimo_disparo"]
		}
	}
	t.Fatalf("la política %q no figura sobre %q en el inventario: %v", politica, maquina, fila["politicas"])
	return nil
}

// disparoGuardado busca en fleet_policy_state las filas de esta política sobre esta máquina, con el
// alcance que tengan, y devuelve cuántas hay y el alcance y la hora de la última leída.
func disparoGuardado(t *testing.T, s *McpServer, politica, deviceID string) (filas int, alcance string, cuando time.Time) {
	t.Helper()
	cds, err := s.engine.CooldownsDePoliticas()
	if err != nil {
		t.Fatalf("CooldownsDePoliticas: %v", err)
	}
	for compuesta, hora := range cds[politica] {
		if dev, alc, _ := strings.Cut(compuesta, "\x00"); dev == deviceID {
			filas, alcance, cuando = filas+1, alc, hora
		}
	}
	return filas, alcance, cuando
}

// cooldownEnMemoria dice qué tiene el mapa caliente bajo esta clave: la hora, y si hay una.
func cooldownEnMemoria(s *McpServer, clave string) (time.Time, bool) {
	v, hay := s.ultimoDisparo.Load(clave)
	if !hay {
		return time.Time{}, false
	}
	cuando, ok := v.(time.Time)
	return cuando, ok
}

// ejecucionesDePolitica cuenta lo que el motor de políticas dejó para EJECUTAR en una máquina: con el
// criterio de comandosDePolitica (el aviso no cuenta), pero por máquina y sin el tope de 50 de
// comandosEncolados, que la cola llena de la tabla del cooldown ocupa entera.
func ejecucionesDePolitica(t *testing.T, s *McpServer, deviceID string) int {
	t.Helper()
	filas, err := s.engine.BitacoraDeComandos("casa", deviceID, 5*fleet.ColaMaxPorDevice)
	if err != nil {
		t.Fatalf("BitacoraDeComandos: %v", err)
	}
	n := 0
	for _, c := range filas {
		if c.Origen == fleet.OrigenPolitica && (len(c.Argv) == 0 || c.Argv[0] != comandoAviso) {
			n++
		}
	}
	return n
}

// soltarElSSH deja la marca que el ssh doblado (sshQueCuelga) espera para volver.
func soltarElSSH(soltar string) error {
	return os.WriteFile(soltar, nil, 0o644)
}

// A131 · T4 — EL COOLDOWN SE CUENTA DESDE EL DISPARO, SALGA COMO SALGA LA ACCIÓN, Y LO LEEN IGUAL
// TODOS SUS LECTORES.
//
// I14 dice que el cooldown se cuenta «desde el DISPARO y no desde el resultado», y A24 que sobrevive a
// un reinicio porque se escribe en la base ANTES de actuar. Las dos guardas que lo custodiaban
// —TestElCooldownEvitaLaTormentaDeComandosIdenticos y TestElCooldownSobreviveUnReinicioDelCerebro—
// clavaban los mismos ejes: la acción SIEMPRE salía bien (Tier A, cola vacía), la política era de
// HOST, y nadie miraba el inventario después de que actuara. Cuatro mutaciones de A131 dejaban el
// paquete entero en verde:
//
//   - P1-m10: si la acción falla, soltar el cooldown en memoria («reintentá el próximo tick»). Con la
//     cola llena la política DECIDE doce veces por hora en vez de una.
//   - P3-m3: persistir el disparo recién cuando la acción salió bien. Un disparo que falla, o un
//     cerebro que cae a mitad de un Tier B síncrono, pierde el cooldown al reiniciar.
//   - P3-m1: persistir con el alcance vacío. cargarCooldowns descarta la fila de una política de
//     SERVICIO y le rearma el cooldown en cada reinicio.
//   - P3-m7: que el inventario busque el disparo con el nombre de la máquina: `ultimo_disparo` sale
//     null siempre, también un segundo después de actuar, y la guarda sólo miraba el null de «nunca
//     actuó», que un null fijo también cumple.
//
// Acá no se suma un caso por mutación: se recorren los ejes enteros y cada lector del disparo. Una fila
// por cada condición declarada (AST: seis de host y dos de servicio), por cada tier que admite `exec`
// (AST: el que encola para su agente y el que corre por ssh dentro del barrido) y por cada resultado que
// actuarSiCorresponde cuenta después de marcar (AST: `ok` y `error`). En cada una la condición se
// cumple doce ticks seguidos y el cerebro se reinicia a los treinta segundos, y se exige:
//
//   - UNA decisión en la hora, del resultado de la fila, y ninguna frenada por otra compuerta;
//   - el disparo en memoria, en la base —con el alcance que la política mira: su servicio, o vacío— y
//     en el inventario, los tres con la hora del disparo desde el primer tick;
//   - donde la acción es síncrona, que la memoria y la base ya lo tengan A MITAD de la acción: el ssh
//     doblado no vuelve hasta que la prueba miró, porque «un cerebro que se cae a mitad» es justo el
//     caso por el que la marca va antes;
//   - tras el reinicio, el inventario con la misma hora, cero decisiones a los treinta segundos y una
//     pasado el cooldown, que vuelve a marcar memoria y base (un cooldown que no se suelta es un
//     apagado).
//
// Exposición medida (auditoría A131): 0. La única política en producción, `vaciar-journal`, es de
// host, sus siete series valen 0 (`error` nunca en 60 días) y fleet_policy_state tiene 0 filas. La
// cola llena existió una sola vez —afeb70f0, 2026-08-28/29—, antes del techo de A62 y antes de que
// hubiera políticas; hay un Tier B en la flota. Latente: se vuelve vivo con el primer disparo que
// falle, la primera política de servicio, o —el inventario— con el primer disparo a secas.
//
// Las colisiones que se declaran abajo son con esta misma prueba: cuatro sabotajes sobre la misma
// cadena —marcar, persistir, actuar, contar— que caen en aserciones distintas (la memoria tras
// decidir, la fila en la base, su alcance, la base a mitad de la acción).
//
// Sabotaje: si la acción falla, soltar el cooldown en memoria para «reintentar el próximo tick»
// (P1-m10), que es contarlo desde el éxito.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\t\ts.metrics.contarPolitica(pol.Nombre, \"error\")\n\t\treturn false\n"
// arnes: a="\t\ts.metrics.contarPolitica(pol.Nombre, \"error\")\n\t\ts.ultimoDisparo.Delete(clave)\n\t\treturn false\n"
// arnes: colision_ok="TestElCooldownSeCuentaDesdeElDisparoYLoLeenTodosSusLectores"
//
// Sabotaje: persistir el disparo recién cuando la acción salió bien (P3-m3).
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tif err := s.engine.MarcarDisparoDePolitica(pol.Nombre, d.ID, pol.Servicio, ahora); err != nil {\n\t\tlogx.Error(\"política: no se pudo persistir el cooldown (sobrevive en memoria, no a un reinicio)\",\n\t\t\t\"politica\", pol.Nombre, \"device\", d.Name, \"error\", err)\n\t}\n\n\tlogx.Info(\"política dispara\",\n\t\t\"politica\", pol.Nombre, \"device\", d.Name, \"principal\", pol.Principal,\n\t\t\"condicion\", pol.Umbral(), \"medido\", medidoParaLog(valor), \"hacer\", pol.Hacer)\n\n\tif err := s.correrAccionDePolitica(pol, pr, d, ahora); err != nil {\n\t\tlogx.Error(\"política: la acción falló\", \"politica\", pol.Nombre, \"device\", d.Name, \"error\", err)\n\t\ts.metrics.contarPolitica(pol.Nombre, \"error\")\n\t\treturn false\n\t}\n"
// arnes: a="\tlogx.Info(\"política dispara\",\n\t\t\"politica\", pol.Nombre, \"device\", d.Name, \"principal\", pol.Principal,\n\t\t\"condicion\", pol.Umbral(), \"medido\", medidoParaLog(valor), \"hacer\", pol.Hacer)\n\n\tif err := s.correrAccionDePolitica(pol, pr, d, ahora); err != nil {\n\t\tlogx.Error(\"política: la acción falló\", \"politica\", pol.Nombre, \"device\", d.Name, \"error\", err)\n\t\ts.metrics.contarPolitica(pol.Nombre, \"error\")\n\t\treturn false\n\t}\n\tif err := s.engine.MarcarDisparoDePolitica(pol.Nombre, d.ID, pol.Servicio, ahora); err != nil {\n\t\tlogx.Error(\"política: no se pudo persistir el cooldown (sobrevive en memoria, no a un reinicio)\",\n\t\t\t\"politica\", pol.Nombre, \"device\", d.Name, \"error\", err)\n\t}\n"
//
// Sabotaje: persistir el disparo con el alcance vacío, el de una política de host (P3-m1). El arreglo
// es el mismo alcance recortado a mano: la guarda mide el valor guardado, no cómo se escribe.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="s.engine.MarcarDisparoDePolitica(pol.Nombre, d.ID, pol.Servicio, ahora)"
// arnes: a="s.engine.MarcarDisparoDePolitica(pol.Nombre, d.ID, \"\", ahora)"
// arnes: arreglo_de="s.engine.MarcarDisparoDePolitica(pol.Nombre, d.ID, pol.Servicio, ahora)"
// arnes: arreglo_a="s.engine.MarcarDisparoDePolitica(pol.Nombre, d.ID, strings.TrimSpace(pol.Servicio), ahora)"
// arnes: colision_ok="TestElCooldownSeCuentaDesdeElDisparoYLoLeenTodosSusLectores"
//
// Sabotaje: persistir el disparo DESPUÉS de la acción, salga como salga (un `defer`). En un Tier A no
// se distingue; en un Tier B deja la base vacía mientras el ssh corre.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tif err := s.engine.MarcarDisparoDePolitica(pol.Nombre, d.ID, pol.Servicio, ahora); err != nil {\n"
// arnes: a="\tdefer s.engine.MarcarDisparoDePolitica(pol.Nombre, d.ID, pol.Servicio, ahora)\n\tif err := error(nil); err != nil {\n"
// arnes: colision_ok="TestElCooldownSeCuentaDesdeElDisparoYLoLeenTodosSusLectores"
//
// Sabotaje: que el inventario busque el disparo con el NOMBRE de la máquina y no con su ID (P3-m7).
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="s.ultimoDisparo.Load(pol.ClaveDeCooldown(d.ID)); hay {"
// arnes: a="s.ultimoDisparo.Load(pol.ClaveDeCooldown(d.Name)); hay {"
func TestElCooldownSeCuentaDesdeElDisparoYLoLeenTodosSusLectores(t *testing.T) {
	resultados := resultadosTrasElDisparo(t)
	desenlaces := desenlacesPorResultado()
	for _, r := range resultados {
		if _, ok := desenlaces[r]; !ok {
			t.Errorf("actuarSiCorresponde cuenta %q después de marcar el cooldown y la tabla no sabe provocarlo: ese "+
				"desenlace queda sin medir, y es justo por donde se colaban P1-m10 y P3-m3", r)
		}
	}
	for r := range desenlaces {
		if !slices.Contains(resultados, r) {
			t.Errorf("la tabla provoca %q y actuarSiCorresponde ya no lo cuenta DESPUÉS de marcar el cooldown: o el "+
				"resultado desapareció (sacalo de la tabla), o la marca se mudó después de esa rama, y entonces ese "+
				"desenlace ya no espacia la decisión —el defecto que esta tabla vino a cerrar—", r)
		}
	}
	tiers := tiersQueActuan(t)
	transportes := transportesPorTier()
	for _, tier := range tiers {
		if _, ok := transportes[tier]; !ok {
			t.Errorf("el tier %q admite `exec` y la tabla no sabe por dónde le llega la acción: su cooldown queda sin medir", tier)
		}
	}
	for tier := range transportes {
		if !slices.Contains(tiers, tier) {
			t.Errorf("la tabla tiene transporte para el tier %q, que no está declarado o no admite `exec`", tier)
		}
	}
	condiciones, disparos := condicionesConDisparo(t, "una condición sin receta es un camino hasta la marca que la "+
		"tabla del cooldown no recorre")
	if t.Failed() {
		t.FailNow()
	}

	curador := curadorDeLaTablaDeCooldown()
	for _, cond := range condiciones {
		for _, tier := range tiers {
			for _, resultado := range resultados {
				f := filaDeCooldown{
					cond: cond, disparo: disparos[cond], tier: tier, transporte: transportes[tier],
					resultado: resultado, desenlace: desenlaces[resultado], desenlaces: resultados, curador: curador,
				}
				t.Run(fmt.Sprintf("%s/tier_%s/%s", cond, tier, resultado), f.recorrer)
			}
		}
	}
}

// filaDeCooldown es una fila de la tabla del cooldown: una condición, un tier y un desenlace.
type filaDeCooldown struct {
	cond       fleet.Condicion
	disparo    disparoDeCondicion
	tier       fleet.Tier
	transporte transporteDeTier
	resultado  string
	desenlace  desenlaceDeLaAccion
	desenlaces []string // todos los resultados que pasan la marca: su suma son las decisiones
	curador    Principal
}

// recorrer corre UNA fila: el disparo, sus cuatro lectores, la hora de cooldown, el reinicio y la
// vuelta a actuar.
func (f filaDeCooldown) recorrer(t *testing.T) {
	dir, marcas := t.TempDir(), t.TempDir()
	reg := registroDePrueba(f.curador)
	pc := config.PolicyConfig{
		Name: "cooldown-" + string(f.cond), Principal: f.curador.Name, When: string(f.cond),
		Threshold: f.disparo.supera, Devices: []string{"*"}, Service: f.disparo.servicio,
		Run: f.disparo.hacer, CooldownMinutes: 60,
	}
	// HECHO: el alcance del cooldown es el servicio que la política mira; vacío en una de host.
	alcance := f.disparo.servicio
	hora := func(x time.Time) string { return x.UTC().Format(time.RFC3339) }

	s1 := servidorSobre(t, dir, pc, reg)
	d := maquinaQueActuaPor(t, s1, f.tier, f.transporte)
	if len(s1.politicas) != 1 {
		t.Fatalf("la fila configura una política y el servidor tiene %d", len(s1.politicas))
	}
	clave := s1.politicas[0].ClaveDeCooldown(d.ID)

	entro, soltar := filepath.Join(marcas, "entro"), filepath.Join(marcas, "soltar")
	if f.transporte.sincrono {
		t.Cleanup(fleet.SSHFalsoParaTest(t, sshQueCuelga(entro, soltar)))
		if !f.desenlace.llegaAlTransporte {
			// Si la acción llegara igual, que el ssh no se quede esperando: lo denuncia la aserción del
			// transporte, más abajo.
			if err := soltarElSSH(soltar); err != nil {
				t.Fatal(err)
			}
		}
	}
	if f.desenlace.provocar != nil {
		f.desenlace.provocar(t, s1, d)
	}

	// Al segundo: la base guarda el disparo en RFC3339 y el inventario lo publica igual.
	ahora := time.Now().UTC().Truncate(time.Second)
	if v := ultimoDisparoPublicado(t, s1, d.Name, pc.Name); v != nil {
		t.Fatalf("antes de actuar el inventario ya trae `ultimo_disparo: %v`: la fila no arranca de «nunca actuó»", v)
	}

	// ── 1 · EL DISPARO, Y DONDE LA ACCIÓN ES SÍNCRONA, LO QUE HAY A MITAD ─────────────────────────────
	f.disparo.preparar(t, s1, d, ahora)
	llego := false
	if !f.transporte.sincrono {
		s1.aplicarPoliticas("casa", ahora)
	} else {
		termino := make(chan struct{})
		go func() {
			defer close(termino)
			s1.aplicarPoliticas("casa", ahora)
		}()
		t.Cleanup(func() {
			_ = soltarElSSH(soltar)
			select {
			case <-termino:
			case <-time.After(time.Minute):
			}
		})
		limite := time.Now().Add(20 * time.Second)
	esperar:
		for time.Now().Before(limite) {
			if _, err := os.Stat(entro); err == nil {
				llego = true
				break
			}
			select {
			case <-termino:
				break esperar
			case <-time.After(10 * time.Millisecond):
			}
		}
		var enMemoria bool
		var filasAMitad int
		miradoAMitad := llego
		if miradoAMitad {
			// El ssh doblado sigue esperando: esto es lo que encontraría un cerebro que se cae AHORA.
			_, enMemoria = cooldownEnMemoria(s1, clave)
			filasAMitad, _, _ = disparoGuardado(t, s1, pc.Name, d.ID)
		}
		if err := soltarElSSH(soltar); err != nil {
			t.Fatal(err)
		}
		select {
		case <-termino:
		case <-time.After(45 * time.Second):
			t.Fatal("el barrido no volvió después de soltar el ssh doblado")
		}
		if _, err := os.Stat(entro); err == nil {
			llego = true
		}
		if llego != f.desenlace.llegaAlTransporte {
			t.Fatalf("la acción llegó al ssh = %v y la fila dice %v: no ejercita el desenlace %q que nombra",
				llego, f.desenlace.llegaAlTransporte, f.resultado)
		}
		// Un ssh que llega DESPUÉS de la espera no dice nada del cooldown: la fila no pudo mirar a mitad
		// de la acción, y culpar a la marca sería mandar a buscar donde no está el problema.
		if llego && !miradoAMitad {
			t.Fatalf("la acción llegó al ssh recién después de los 20 s de espera: la fila no pudo mirar memoria " +
				"y base a mitad de la acción. Es la máquina, no el cooldown: volvé a correrla con menos carga")
		}
		if llego && !enMemoria {
			t.Fatalf("A MITAD de la acción —el ssh todavía no volvió— el cooldown no está en memoria: el barrido " +
				"marca después de actuar, y una acción lenta deja la puerta abierta al próximo tick")
		}
		if llego && filasAMitad == 0 {
			t.Fatalf("A MITAD de la acción —el ssh todavía no volvió— fleet_policy_state no tiene el disparo: si el " +
				"cerebro cae ahora, el caso que nombra actuarSiCorresponde («un cerebro que se cae a mitad»), el " +
				"reinicio no lo encuentra y la política vuelve a actuar dentro del cooldown")
		}
	}

	// ── 2 · LO QUE DEJÓ LA DECISIÓN, EN CADA LECTOR ─────────────────────────────────────────────────
	if n, want := ejecucionesDePolitica(t, s1, d.ID), map[bool]int{true: 1, false: 0}[f.desenlace.llegaAlTransporte]; n != want {
		t.Fatalf("la política dejó %d comando/s en la bitácora y con el desenlace %q tenían que ser %d: la fila no "+
			"ejercita lo que nombra", n, f.resultado, want)
	}
	por, decisiones, frenadas := desenlacesContados(s1, pc.Name, f.desenlaces)
	if frenadas != 0 {
		t.Fatalf("la política no llegó a la marca: otra compuerta la frenó %d vez/veces. La fila mide el cooldown y "+
			"necesita que la decisión pase", frenadas)
	}
	if decisiones != 1 || por[f.resultado] != 1 {
		t.Fatalf("en el primer tick la política tenía que decidir UNA vez, con resultado %q, y contó %v", f.resultado, por)
	}
	if cuando, hay := cooldownEnMemoria(s1, clave); !hay || !cuando.Equal(ahora) {
		t.Fatalf("la política DECIDIÓ actuar (resultado %q) y el cooldown en memoria quedó en %v (hay=%v): el "+
			"próximo tick vuelve a decidir. Con la cola llena son doce decisiones por hora en vez de una —la "+
			"tormenta que el cooldown existe para evitar, justo cuando algo ya va mal—. I14 lo cuenta desde el "+
			"DISPARO, no desde el resultado", f.resultado, cuando, hay)
	}
	filas, alcanceGuardado, cuandoGuardado := disparoGuardado(t, s1, pc.Name, d.ID)
	if filas == 0 || !cuandoGuardado.Equal(ahora) {
		t.Fatalf("la política decidió (resultado %q) y fleet_policy_state no tiene el disparo de las %s (filas=%d, "+
			"hora=%v): el cooldown dura lo que dure el proceso, y el reinicio —lo primero que alguien hace cuando "+
			"algo va mal— lo rearma", f.resultado, hora(ahora), filas, cuandoGuardado)
	}
	if filas != 1 || alcanceGuardado != alcance {
		t.Fatalf("el disparo quedó guardado con alcance %q (%d fila/s) y la política mira %q: al arrancar, "+
			"cargarCooldowns busca el alcance de HOY y descarta la fila, así que cada reinicio le rearma el "+
			"cooldown", alcanceGuardado, filas, alcance)
	}
	if v := ultimoDisparoPublicado(t, s1, d.Name, pc.Name); v != hora(ahora) {
		t.Fatalf("la política acaba de actuar y el inventario publica `ultimo_disparo: %v`, no %s: el panel dice "+
			"«nunca actuó» (o una hora que no es) mientras el cooldown corre", v, hora(ahora))
	}

	// ── 3 · LA HORA DEL COOLDOWN ───────────────────────────────────────────────────────────────────
	for i := 1; i < 12; i++ {
		tick := ahora.Add(time.Duration(i) * 5 * time.Minute)
		f.disparo.preparar(t, s1, d, tick)
		s1.aplicarPoliticas("casa", tick)
		if por, decisiones, _ := desenlacesContados(s1, pc.Name, f.desenlaces); decisiones != 1 {
			t.Fatalf("a los %s del disparo la política llevaba %d decisiones (%v) con un cooldown de 60 min: el "+
				"cooldown no espacia la DECISIÓN cuando el resultado es %q", tick.Sub(ahora), decisiones, por, f.resultado)
		}
	}

	// ── 4 · EL REINICIO ──────────────────────────────────────────────────────────────────────────
	s1.engine.Close()
	s2 := servidorSobre(t, dir, pc, reg)
	if v := ultimoDisparoPublicado(t, s2, d.Name, pc.Name); v != hora(ahora) {
		t.Fatalf("tras el reinicio el inventario publica `ultimo_disparo: %v` y la política disparó a las %s: lo que "+
			"el cerebro recuperó de la base y lo que muestra no coinciden", v, hora(ahora))
	}
	despues := ahora.Add(30 * time.Second)
	f.disparo.preparar(t, s2, d, despues)
	s2.aplicarPoliticas("casa", despues)
	if por, decisiones, _ := desenlacesContados(s2, pc.Name, f.desenlaces); decisiones != 0 {
		t.Fatalf("a los treinta segundos del reinicio la política volvió a decidir (%v): el disparo de la primera "+
			"vida (resultado %q) no sobrevivió al reinicio", por, f.resultado)
	}
	pasada := ahora.Add(70 * time.Minute)
	f.disparo.preparar(t, s2, d, pasada)
	s2.aplicarPoliticas("casa", pasada)
	if por, decisiones, frenadas := desenlacesContados(s2, pc.Name, f.desenlaces); decisiones != 1 || por[f.resultado] != 1 || frenadas != 0 {
		t.Fatalf("pasado el cooldown el cerebro reiniciado contó %v (y %d frenadas): tenía que volver a decidir UNA "+
			"vez, con resultado %q. Un cooldown que no se suelta es un apagado", por, frenadas, f.resultado)
	}
	if cuando, hay := cooldownEnMemoria(s2, clave); !hay || !cuando.Equal(pasada) {
		t.Fatalf("la segunda decisión no volvió a marcar la memoria: quedó en %v (hay=%v), esperaba %s", cuando, hay, hora(pasada))
	}
	if _, _, cuando := disparoGuardado(t, s2, pc.Name, d.ID); !cuando.Equal(pasada) {
		t.Fatalf("la segunda decisión no volvió a marcar la base: quedó en %v, esperaba %s", cuando, hora(pasada))
	}
}

// servidorConPoliticas arma un servidor sobre un directorio DADO con VARIAS políticas: servidorSobre
// con una lista, para simular el reinicio de un cerebro que tiene más de una.
func servidorConPoliticas(t *testing.T, dir string, pols []config.PolicyConfig, reg *PrincipalRegistry) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	s := NewMcpServer(engine, dir, embedding.NoopProvider{})
	if err := s.ConfigurarFlota(config.FleetConfig{Policies: pols}); err != nil {
		t.Fatalf("ConfigurarFlota: %v", err)
	}
	s.buscarPrincipal = reg
	return s
}

// politicasDeLaTablaDeCarga son las políticas configuradas «hoy» en la tabla de la carga, en este
// orden: una de servicio, una de host y una de servicio con el nombre y el servicio escritos con
// espacios en los bordes. El ORDEN importa: la primera tiene filas que ya no le corresponden, y
// detrás vienen otras con filas buenas.
func politicasDeLaTablaDeCarga() []config.PolicyConfig {
	return []config.PolicyConfig{
		{Name: "revivir-web", Principal: "curador", When: string(fleet.CondServicioCaido), Devices: []string{"*"},
			Service: "nginx", Run: []string{"systemctl", "restart", "nginx"}, CooldownMinutes: 60},
		{Name: "vaciar-journal", Principal: "curador", When: string(fleet.CondMemPct), Threshold: 90,
			Devices: []string{"*"}, Run: []string{"journalctl", "--vacuum-size=200M"}, CooldownMinutes: 60},
		{Name: " revivir-db ", Principal: "curador", When: string(fleet.CondServicioReinicios), Threshold: 3,
			Devices: []string{"*"}, Service: " postgres ", Run: []string{"systemctl", "restart", "postgres"}, CooldownMinutes: 60},
	}
}

// A131 · T4 — AL ARRANCAR SE SIEMBRAN EXACTAMENTE LOS COOLDOWNS DE LAS POLÍTICAS COMO ESTÁN HOY.
//
// cargarCooldowns documenta que una fila cuyo alcance ya no coincide —alguien le cambió el `service` a
// la política— se IGNORA, y sigue con las demás. TestElCooldownSobreviveUnReinicioDelCerebro lo medía
// con UNA política y la tabla LIMPIA, así que cambiar ese `continue` por un `return` (P3-m2) la dejaba
// en verde: una fila vieja de una política anterior en la lista cortaba la carga de todas las
// siguientes, y la política de la guarda volvía a actuar a los treinta segundos del reinicio.
//
// Acá la tabla de estado es MIXTA y los hechos están escritos: por cada fila, si la política
// configurada hoy la hereda o no, y por qué. Hay filas que no corresponden en la PRIMERA política de la
// lista, con otras buenas detrás; y dentro de una misma política, buenas y viejas mezcladas. Se exige
// que lo sembrado sea EXACTAMENTE lo que la tabla dice —ni una fila de menos, ni una de más, cada una
// con su hora—, y se siembra veinte veces: el orden en que se recorre un mapa cambia en cada vuelta,
// así que un corte que dependa de por dónde empieza (un `break` en vez del `continue`) no pasa por
// suerte. El `return` cae siempre en la primera.
//
// La tercera política escribe el `name` y el `service` con espacios en los bordes, y el almacén guarda
// los dos recortados. Hasta A131·T4 la carga buscaba por el nombre crudo y no encontraba sus filas:
// esa política perdía el cooldown en cada reinicio (DEFECTO VIVO, latente: el único nombre en
// producción, `vaciar-journal`, no tiene bordes). Lo cierra ConfigurarFlota, que recorta el nombre en
// la entrada.
//
// Exposición medida (auditoría A131): 0. fleet_policy_state tiene 0 filas (esquema 57) y la única
// política configurada nunca actuó, así que no hay fila vieja que pueda cortar nada. Hacen falta tres
// cosas que hoy no existen: una política de servicio que ya disparó, que le cambien el `service`, y un
// reinicio.
//
// Sabotaje: al encontrar una fila cuyo alcance ya no coincide, cortar la carga entera (P3-m2). El
// arreglo es la misma regla con la condición dada vuelta: sembrar sólo lo que coincide, sin `continue`.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\t\t\tif alcance != esperada {\n\t\t\t\tcontinue\n\t\t\t}\n"
// arnes: a="\t\t\tif alcance != esperada {\n\t\t\t\treturn\n\t\t\t}\n"
// arnes: arreglo_de="\t\t\tif alcance != esperada {\n\t\t\t\tcontinue\n\t\t\t}\n\t\t\ts.ultimoDisparo.Store(pol.ClaveDeCooldown(deviceID), cuando)\n\t\t\tn++\n"
// arnes: arreglo_a="\t\t\tif alcance == esperada {\n\t\t\t\ts.ultimoDisparo.Store(pol.ClaveDeCooldown(deviceID), cuando)\n\t\t\t\tn++\n\t\t\t}\n"
//
// Sabotaje: construir la política con el nombre crudo, sin recortar, como antes de A131·T4. La poda
// horaria cae por el mismo motivo en TestLaPodaHorariaConservaElCooldownDeCadaPoliticaConfigurada.
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="Nombre:    strings.TrimSpace(pc.Name),"
// arnes: a="Nombre:    pc.Name,"
// arnes: colision_ok="TestLaPodaHorariaConservaElCooldownDeCadaPoliticaConfigurada"
func TestAlArrancarSeSiembranLosCooldownsDeLasPoliticasComoEstanHoy(t *testing.T) {
	pols := politicasDeLaTablaDeCarga()
	filas := []struct {
		politica, device, alcance string // tal como las guarda MarcarDisparoDePolitica, que recorta
		carga                     bool   // HECHO: si la política configurada hoy la hereda
		porque                    string
	}{
		{"revivir-web", "dev-a", "postgres", false, "el servicio que miraba antes de que le cambiaran el `service`"},
		{"revivir-web", "dev-a", "nginx", true, "el servicio que mira hoy"},
		{"revivir-web", "dev-b", "", false, "de cuando era una política de host"},
		{"revivir-web", "dev-b", "nginx", true, "otra máquina, el mismo servicio"},
		{"vaciar-journal", "dev-a", "nginx", false, "de cuando miraba un servicio"},
		{"vaciar-journal", "dev-a", "", true, "una política de host"},
		{"revivir-db", "dev-a", "postgres", true, "el `name` y el `service` se escribieron con bordes y el almacén guarda los dos recortados"},
		{"revivir-db", "dev-c", "mysql", false, "otro servicio, en una máquina donde no tiene ninguna fila buena"},
		{"borrada-del-archivo", "dev-a", "", false, "una política que ya no está configurada"},
	}

	// PISO: una fila que NO carga en la primera política de la lista, y filas que SÍ cargan en las que
	// vienen detrás (sin eso, cortar la carga en la primera fila vieja no se distingue de seguir); y
	// una política con filas de los dos lados (sin eso, cortar sólo esa política tampoco).
	primera := strings.TrimSpace(pols[0].Name)
	viejaEnLaPrimera, buenaDetras := false, false
	porPolitica := map[string]map[bool]bool{}
	for _, f := range filas {
		if porPolitica[f.politica] == nil {
			porPolitica[f.politica] = map[bool]bool{}
		}
		porPolitica[f.politica][f.carga] = true
		viejaEnLaPrimera = viejaEnLaPrimera || (f.politica == primera && !f.carga)
		buenaDetras = buenaDetras || (f.politica != primera && f.carga)
	}
	mezcladas := 0
	for _, lados := range porPolitica {
		if lados[true] && lados[false] {
			mezcladas++
		}
	}
	if !viejaEnLaPrimera || !buenaDetras || mezcladas == 0 {
		t.Fatalf("PISO: fila vieja en la primera política = %v, fila buena en otra detrás = %v, políticas con "+
			"filas de los dos lados = %d: sin las tres, un corte de la carga pasa por esta tabla sin verse",
			viejaEnLaPrimera, buenaDetras, mezcladas)
	}

	dir := t.TempDir()
	semilla, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	base := time.Now().UTC().Truncate(time.Second)
	esperado := map[string]time.Time{}
	porque := map[string]string{}
	for i, f := range filas {
		cuando := base.Add(-time.Duration(i+1) * time.Minute) // una hora distinta por fila
		if err := semilla.MarcarDisparoDePolitica(f.politica, f.device, f.alcance, cuando); err != nil {
			t.Fatalf("sembrar %+v: %v", f, err)
		}
		// La clave con la que la DECISIÓN buscaría ese disparo: la del dominio, para la política con ese
		// nombre y ese servicio.
		clave := fleet.Politica{Nombre: f.politica, Servicio: f.alcance}.ClaveDeCooldown(f.device)
		porque[clave] = f.politica + " · " + f.device + " · alcance " + strconv.Quote(f.alcance) + ": " + f.porque
		if f.carga {
			esperado[clave] = cuando
		}
	}
	semilla.Close()

	s := servidorConPoliticas(t, dir, pols, registroDePrueba(curadorDeLaTablaDeCooldown()))
	for vuelta := 0; vuelta < 20; vuelta++ {
		if vuelta > 0 {
			// Otra carga, como la de otro arranque: el mismo cargarCooldowns que llama ConfigurarFlota.
			s.ultimoDisparo.Clear()
			s.cargarCooldowns()
		}
		sembrado := map[string]time.Time{}
		s.ultimoDisparo.Range(func(k, v any) bool {
			clave, _ := k.(string)
			cuando, _ := v.(time.Time)
			sembrado[clave] = cuando
			return true
		})
		var faltan, sobran, malHora []string
		for clave, cuando := range esperado {
			got, hay := sembrado[clave]
			switch {
			case !hay:
				faltan = append(faltan, porque[clave])
			case !got.Equal(cuando):
				malHora = append(malHora, fmt.Sprintf("%s (sembrada %s, guardada %s)", porque[clave], got, cuando))
			}
		}
		for clave := range sembrado {
			if _, ok := esperado[clave]; !ok {
				explicacion, conocida := porque[clave]
				if !conocida {
					explicacion = strconv.Quote(clave) + ": una clave que ninguna fila guardada explica"
				}
				sobran = append(sobran, explicacion)
			}
		}
		sort.Strings(faltan)
		sort.Strings(sobran)
		sort.Strings(malHora)
		if len(faltan)+len(sobran)+len(malHora) > 0 {
			// Las cuentas van en la PRIMERA línea, que es la que el arnés compara entre sabotajes: cortar la
			// carga pierde varias filas y el nombre crudo una sola, y con el mismo encabezado los dos
			// rojos se leían como un sabotaje contado dos veces.
			t.Fatalf("carga %d de 20: lo sembrado no es lo que corresponde a las políticas de hoy (faltan %d, sobran %d, "+
				"con otra hora %d).\n  faltan (la política vuelve a actuar a los treinta segundos del reinicio): %q\n  "+
				"sobran (se hereda el enfriamiento de algo que ya no se vigila): %q\n  con otra hora: %q",
				vuelta+1, len(faltan), len(sobran), len(malHora), faltan, sobran, malHora)
		}
	}
}

// A131 · T4 — LA PODA HORARIA CONSERVA EL COOLDOWN DE CADA POLÍTICA CONFIGURADA, Y SÓLO ESE.
//
// podarEstadoDePoliticasSiToca arma la lista de vivas con los nombres de s.politicas y el almacén borra
// las filas de las demás. Las guardas de la poda la medían con UNA política viva
// (TestLaPodaDeCooldownsNoDependeDeLaRetencionDeSalidas) o llamando al almacén directo con una
// (TestPodarElEstadoDePoliticasConListaVaciaNoBorraNada), y con una sola, ligar sólo la PRIMERA viva
// (P3-m5, en el almacén; o armar la lista con la primera política, acá) no se distingue de ligarlas a
// todas: cada poda horaria se habría llevado el cooldown persistido de todas menos la primera.
//
// Acá se configuran tres —de host y de servicio— y se exige que el conjunto de políticas con estado
// después de la poda sea EXACTAMENTE el de s.politicas: se deriva de lo configurado, no de una lista
// escrita en la prueba. La tercera tiene el nombre escrito con espacios en los bordes: el almacén
// guarda el disparo recortado, y hasta A131·T4 la lista de vivas llevaba el nombre crudo, así que la
// poda borraba su cooldown cada hora por huérfano (DEFECTO VIVO, latente; lo cierra ConfigurarFlota).
//
// Exposición medida (auditoría A131): 0. Una sola política configurada (`vaciar-journal`, sin bordes)
// y fleet_policy_state con 0 filas; en 30 días nunca hubo dos políticas vivas a la vez.
//
// Sabotaje: armar la lista de vivas sólo con la primera política configurada.
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="\tfor _, p := range s.politicas {\n\t\tvivas = append(vivas, p.Nombre)\n\t}\n"
// arnes: a="\tfor _, p := range s.politicas[:1] {\n\t\tvivas = append(vivas, p.Nombre)\n\t}\n"
//
// Sabotaje: construir la política con el nombre crudo, sin recortar, como antes de A131·T4. El
// reinicio cae por el mismo motivo en TestAlArrancarSeSiembranLosCooldownsDeLasPoliticasComoEstanHoy.
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="Nombre:    strings.TrimSpace(pc.Name),"
// arnes: a="Nombre:    pc.Name,"
// arnes: colision_ok="TestAlArrancarSeSiembranLosCooldownsDeLasPoliticasComoEstanHoy"
func TestLaPodaHorariaConservaElCooldownDeCadaPoliticaConfigurada(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if err := s.ConfigurarFlota(config.FleetConfig{Policies: politicasDeLaTablaDeCarga()}); err != nil {
		t.Fatalf("ConfigurarFlota: %v", err)
	}
	// PISO: con una sola política configurada, ligar sólo la primera no se distingue de ligarlas todas.
	if len(s.politicas) < 2 {
		t.Fatalf("PISO: la prueba configura %d política/s y necesita al menos dos", len(s.politicas))
	}
	ahora := time.Now().UTC()
	for _, p := range s.politicas {
		if err := s.engine.MarcarDisparoDePolitica(p.Nombre, "dev-1", p.Servicio, ahora); err != nil {
			t.Fatalf("marcar %q: %v", p.Nombre, err)
		}
	}
	huerfanas := []string{"borrada-del-archivo", "renombrada-hace-un-mes"}
	for _, h := range huerfanas {
		if err := s.engine.MarcarDisparoDePolitica(h, "dev-1", "", ahora); err != nil {
			t.Fatalf("marcar %q: %v", h, err)
		}
	}

	s.podarEstadoDePoliticasSiToca(ahora)

	quedan := politicasConEstado(t, s)
	var perdidas []string
	for _, p := range s.politicas {
		if !quedan[p.Nombre] {
			perdidas = append(perdidas, strconv.Quote(p.Nombre))
		}
		delete(quedan, p.Nombre)
	}
	if len(perdidas) > 0 {
		t.Errorf("la poda horaria se llevó el cooldown persistido de %s, que SÍ está/n configurada/s: el próximo "+
			"reinicio la/s deja actuar antes de tiempo", strings.Join(perdidas, ", "))
	}
	for _, h := range huerfanas {
		if quedan[h] {
			t.Errorf("la poda no se llevó el estado de %q, que ya no está configurada: la tabla crece sin techo", h)
		}
		delete(quedan, h)
	}
	if len(quedan) > 0 {
		t.Errorf("después de la poda hay estado de políticas que la prueba no sembró: %v", quedan)
	}
}

// A131 · T4 — DOS POLÍTICAS HOMÓNIMAS NO ARRANCAN, DIFIERAN EN LO QUE DIFIERAN.
//
// El cooldown persistido, la métrica, los avisos y la poda se llevan POR NOMBRE, así que dos políticas
// que lo comparten se pisan: una tapa a la otra sin que nada falle. TestDosPoliticasConElMismoNombreNoArrancan
// lo medía con dos de host que diferían sólo en `run`, y la tabla de T5
// (TestDosPoliticasHomonimasNoArrancanEnNingunaPosicion) recorre la POSICIÓN con la misma forma. Así,
// deduplicar por (nombre, servicio) —la forma de ClaveDeCooldown— dejaba el paquete en verde (P2-m10):
// dos políticas de servicio homónimas sobre nginx y postgres arrancaban y compartían politicaStats.
//
// Acá el eje es el CAMPO: se recorren por reflexión los de config.PolicyConfig salvo `Name`, y por cada
// uno se arma una segunda política VÁLIDA que tiene el mismo nombre y difiere sólo en ese campo, sobre
// una base de host y otra de servicio. Un campo nuevo sin variante pone esto rojo. Los controles están
// adentro del recorrido: la variante difiere exactamente en su campo, cada una arranca sola, y con
// otro nombre arrancan juntas.
//
// Exposición medida (auditoría A131): 0. Una sola política configurada, de host; ningún nombre
// repetido.
//
// Sabotaje: deduplicar por (nombre, servicio), la forma de la clave del cooldown (P2-m10). El arreglo
// es el mismo que declara la tabla de T5: preguntar por la presencia de la clave es la misma regla.
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="\t\tif vistos[pol.Nombre] {\n\t\t\treturn fmt.Errorf(\"hay dos políticas llamadas %q: el cooldown y las métricas se llevan por nombre, así que se pisarían entre sí\", pol.Nombre)\n\t\t}\n\t\tvistos[pol.Nombre] = true\n"
// arnes: a="\t\tif vistos[pol.Nombre+\"\\x00\"+pol.Servicio] {\n\t\t\treturn fmt.Errorf(\"hay dos políticas llamadas %q: el cooldown y las métricas se llevan por nombre, así que se pisarían entre sí\", pol.Nombre)\n\t\t}\n\t\tvistos[pol.Nombre+\"\\x00\"+pol.Servicio] = true\n"
// arnes: arreglo_de="\t\tif vistos[pol.Nombre] {\n"
// arnes: arreglo_a="\t\tif _, repetida := vistos[pol.Nombre]; repetida {\n"
// arnes: colision_ok="TestDosPoliticasHomonimasNoArrancanEnNingunaPosicion"
func TestDosPoliticasHomonimasNoArrancanDifieranEnLoQueDifieran(t *testing.T) {
	deServicio := config.PolicyConfig{
		Name: "revivir", Principal: "curador", When: string(fleet.CondServicioCaido), Devices: []string{"*"},
		Service: "nginx", Run: []string{"systemctl", "restart", "nginx"}, CooldownMinutes: 60,
	}
	bases := []struct {
		clase string
		pc    config.PolicyConfig
	}{
		{"de host", politicaDeMemoria()},
		{"de servicio", deServicio},
	}
	// Cómo cambiar CADA campo sin dejar de ser una política válida de su clase. Devuelve false cuando el
	// campo no admite otro valor en esa clase: una de host no puede nombrar un servicio (Validar).
	variantes := map[string]func(p *config.PolicyConfig, deServicio bool) bool{
		"Principal": func(p *config.PolicyConfig, _ bool) bool { p.Principal += "-suplente"; return true },
		"When": func(p *config.PolicyConfig, deServicio bool) bool {
			p.When = string(fleet.CondCPUPct)
			if deServicio {
				p.When = string(fleet.CondServicioReinicios)
			}
			return true
		},
		"Threshold": func(p *config.PolicyConfig, _ bool) bool { p.Threshold += 5; return true },
		"Devices":   func(p *config.PolicyConfig, _ bool) bool { p.Devices = []string{"nas"}; return true },
		"Run": func(p *config.PolicyConfig, _ bool) bool {
			p.Run = append(slices.Clone(p.Run), "--no-pager")
			return true
		},
		"Service": func(p *config.PolicyConfig, deServicio bool) bool {
			p.Service = "postgres"
			return deServicio
		},
		"CooldownMinutes": func(p *config.PolicyConfig, _ bool) bool { p.CooldownMinutes += 15; return true },
	}

	tipo := reflect.TypeOf(config.PolicyConfig{})
	if _, ok := tipo.FieldByName("Name"); !ok {
		t.Fatal("config.PolicyConfig ya no tiene `Name`: la identidad de una política cambió de campo y esta prueba no sabe cuál comparar")
	}
	var campos []string
	for i := 0; i < tipo.NumField(); i++ {
		if n := tipo.Field(i).Name; n != "Name" {
			campos = append(campos, n)
		}
	}
	for _, c := range campos {
		if _, ok := variantes[c]; !ok {
			t.Errorf("config.PolicyConfig tiene el campo %s y la prueba no sabe variarlo: dos homónimas que difieran "+
				"sólo en él quedan sin medir. Agregale una variante válida", c)
		}
	}
	for c := range variantes {
		if !slices.Contains(campos, c) {
			t.Errorf("la prueba varía %s y config.PolicyConfig ya no lo tiene", c)
		}
	}
	if t.Failed() {
		t.FailNow()
	}

	cubiertos := map[string]bool{}
	for _, b := range bases {
		for _, campo := range campos {
			otra := b.pc
			if !variantes[campo](&otra, b.pc.Service != "") {
				continue
			}
			cubiertos[campo] = true
			t.Run(b.clase+"/"+campo, func(t *testing.T) {
				vb, vo := reflect.ValueOf(b.pc), reflect.ValueOf(otra)
				for i := 0; i < tipo.NumField(); i++ {
					igual := reflect.DeepEqual(vb.Field(i).Interface(), vo.Field(i).Interface())
					if nombre := tipo.Field(i).Name; (nombre == campo) == igual {
						t.Fatalf("la variante no difiere SÓLO en %s (el campo %s quedó igual = %v): el control no mide el eje", campo, nombre, igual)
					}
				}
				s := newTestServer(t, embedding.NoopProvider{})
				for _, sola := range []config.PolicyConfig{b.pc, otra} {
					if err := s.ConfigurarFlota(config.FleetConfig{Policies: []config.PolicyConfig{sola}}); err != nil {
						t.Fatalf("la política sola no arranca (%v): la variante no es válida y el rechazo de abajo no mediría nada", err)
					}
				}
				renombrada := otra
				renombrada.Name += "-bis"
				if err := s.ConfigurarFlota(config.FleetConfig{Policies: []config.PolicyConfig{b.pc, renombrada}}); err != nil {
					t.Fatalf("con nombres distintos no arrancan juntas (%v): se rechaza por otra cosa que el nombre", err)
				}
				err := s.ConfigurarFlota(config.FleetConfig{Policies: []config.PolicyConfig{b.pc, otra}})
				if err == nil {
					t.Fatalf("dos políticas %s llamadas %q que difieren sólo en `%s` arrancaron. El cooldown persistido, "+
						"la métrica, los avisos y la poda se llevan POR NOMBRE: una tapa a la otra sin que nada falle",
						b.clase, b.pc.Name, campo)
				}
				if !strings.Contains(err.Error(), strconv.Quote(b.pc.Name)) {
					t.Errorf("el arranque se negó, pero el error no nombra a la repetida %q: %v", b.pc.Name, err)
				}
			})
		}
	}
	for _, c := range campos {
		if !cubiertos[c] {
			t.Errorf("PISO: ninguna base admite una variante de %s: ese eje quedó sin recorrer", c)
		}
	}
}
