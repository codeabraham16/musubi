package mcp

// Pruebas de A131 sobre el auto-heal: guardas que clavaban UN caso de un eje y dejaban pasar el
// resto. Cada una de las de abajo recorre el eje entero —o lo deriva de su fuente— en vez de
// sumar un caso más a la lista.

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// maquinaConPolitica da de alta pc-gio con las capacidades que se le pidan y le pone encima la
// política de referencia. Existe aparte de prepararPolitica porque ésa enrola SIEMPRE con exec,
// y una tabla sobre la compuerta tiene que poder recorrer también el lado del aparato.
func maquinaConPolitica(t *testing.T, caps []string) (*McpServer, fleet.Device) {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "pc-gio", "tier": "A", "caps": caps, "project": "casa", "os": "linux",
	}); e != nil {
		t.Fatalf("enroll: %+v", e)
	}
	if err := s.ConfigurarFlota(config.FleetConfig{Policies: []config.PolicyConfig{politicaDeMemoria()}}); err != nil {
		t.Fatalf("ConfigurarFlota: %v", err)
	}
	d, existe, err := s.engine.DevicePorNombre("casa", "pc-gio")
	if err != nil || !existe {
		t.Fatalf("pc-gio no quedó dada de alta (existe=%v, err=%v)", existe, err)
	}
	return s, d
}

// politicaEnElInventario lee la política tal como la ve una PERSONA con exec sobre el proyecto:
// por musubi_fleet_list, que es lo que dibuja el panel. `visible` es false cuando el detalle no
// viajó, que pasa sólo si la máquina no admite exec (ver
// TestElDetalleDeUnaPoliticaLoVeSoloQuienPuedeEjecutarEnEsaMaquina).
func politicaEnElInventario(t *testing.T, s *McpServer) (puede bool, inertePor string, visible bool) {
	t.Helper()
	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatalf("fleet_list: %+v", e)
	}
	devs, _ := jsonOf(t, res)["devices"].([]any)
	for _, x := range devs {
		fila, _ := x.(map[string]any)
		if fila["name"] != "pc-gio" {
			continue
		}
		// PISO: si el conteo no está, la máquina no tiene la política encima y todo lo de abajo
		// compararía un indicador ausente contra una acción que no podía pasar.
		if fila["politicas_activas"] != float64(1) {
			t.Fatalf("politicas_activas = %v en pc-gio: la política no está sobre la máquina y la fila no mide nada", fila["politicas_activas"])
		}
		det, _ := fila["politicas"].([]any)
		if len(det) == 0 {
			return false, "", false
		}
		p, _ := det[0].(map[string]any)
		puede, ok := p["puede_actuar"].(bool)
		if !ok {
			t.Fatalf("puede_actuar no viajó como booleano (%v): el panel no puede distinguir una política inerte", p["puede_actuar"])
		}
		inertePor, _ = p["inerte_por"].(string)
		return puede, inertePor, true
	}
	t.Fatalf("pc-gio no figura en el inventario de una credencial con exec sobre casa")
	return false, "", false
}

// frenosDeclarados devuelve los valores de las constantes de tipo frenoDePolitica, leídos del AST
// de politicas.go. Es el conjunto CERRADO que la tabla de abajo tiene que recorrer entero.
func frenosDeclarados(t *testing.T) map[frenoDePolitica]string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "politicas.go", nil, 0)
	if err != nil {
		t.Fatalf("no se pudo parsear politicas.go: %v", err)
	}
	out := map[frenoDePolitica]string{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			tipo, ok := vs.Type.(*ast.Ident)
			if !ok || tipo.Name != "frenoDePolitica" {
				continue
			}
			for i, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("%s: una constante de frenoDePolitica no es un literal de cadena y esta guarda no la puede leer", fset.Position(v.Pos()))
				}
				valor, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("%s: %v", fset.Position(v.Pos()), err)
				}
				out[frenoDePolitica(valor)] = vs.Names[i].Name
			}
		}
	}
	// PISO: cero constantes no es «no hay frenos», es que el tipo se renombró y esta guarda quedó
	// mirando al aire. Hoy son nueve, más sinFreno.
	if len(out) < 5 {
		t.Fatalf("encontré %d constantes de frenoDePolitica en politicas.go y son al menos cinco: el tipo cambió de nombre o de forma y la cobertura de la tabla dejó de medirse", len(out))
	}
	return out
}

// frenosDelBarridoGlobal devuelve los frenos que decide el barrido GLOBAL (A132): los que devuelve
// (barridoDePoliticas).frenoSobre, leídos de su AST. No es una lista escrita acá: un freno global
// nuevo entra por nacer en esa función, y sale si deja de devolverlo.
func frenosDelBarridoGlobal(t *testing.T) map[frenoDePolitica]bool {
	t.Helper()
	porNombre := map[string]frenoDePolitica{}
	for valor, nombre := range frenosDeclarados(t) {
		porNombre[nombre] = valor
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "politicas.go", nil, 0)
	if err != nil {
		t.Fatalf("no se pudo parsear politicas.go: %v", err)
	}
	out := map[frenoDePolitica]bool{}
	for _, decl := range f.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "frenoSobre" || fd.Recv == nil || len(fd.Recv.List) != 1 {
			continue
		}
		if recv, ok := fd.Recv.List[0].Type.(*ast.Ident); !ok || recv.Name != "barridoDePoliticas" {
			continue
		}
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			ret, ok := n.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			for _, r := range ret.Results {
				if id, ok := r.(*ast.Ident); ok {
					if v, es := porNombre[id.Name]; es && v != sinFreno {
						out[v] = true
					}
				}
			}
			return true
		})
	}
	// PISO: cero no es «no hay frenos globales», es que frenoSobre se mudó o cambió de forma y la
	// excepción dejó de derivarse de su fuente.
	if len(out) == 0 {
		t.Fatal("no encontré ningún freno devuelto por (barridoDePoliticas).frenoSobre en politicas.go: la " +
			"excepción de los frenos globales dejó de derivarse de su fuente")
	}
	return out
}

// A131 · LA POLÍTICA ACTÚA EXACTAMENTE DONDE SU PRINCIPAL PODRÍA, Y EL INVENTARIO DICE LO MISMO.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ EJES CLAVABAN LAS GUARDAS DE ANTES
//
// TestUnaPoliticaNoPuedeMasQueSuPrincipal saca la concesión (exec sobre otra máquina) y deja al
// principal en el MISMO proyecto que la máquina, que además siempre se enrola con exec. Así, una
// compuerta reescrita sin la tenencia (P1-m1) o sin el lado del aparato (P1-m2) la dejaba verde.
// TestUnaPoliticaRespetaLaAllowlistDeSuPrincipal usa una allowlist con sólo el comodín: consultar
// sólo el comodín (P1-m3) o dejar pasar la lista vacía (P1-m4) también la dejaba verde.
// TestUnaPoliticaInerteSeDistingueDeUnaQueFunciona comparaba indicador contra realidad para UNA
// causa, la allowlist: un indicador sin la compuerta de exec (P3-m8) pasaba. Y ninguna comparaba
// indicador contra realidad en el eje de consentimiento, que era un defecto VIVO (LD1 / P3-L1): el
// indicador era una copia de dos compuertas y la acción ya atravesaba tres, así que una máquina en
// `pide` o `prohibido` figuraba `puede_actuar: true` y la política no actuaba nunca.
//
// Exposición medida: 0 de 1 pares (política × máquina). La única política en producción es
// `vaciar-journal` sobre `musubi-server` (tier A, caps metrics/exec/shell, sin grado declarado ⇒
// `avisa`), con el principal `auto-heal` en `exec: ["*"]` y allowlist `{"*": ["journalctl"]}`.
// Todos los defectos eran latentes: se volvían vivos con el primer `pide`, una máquina sin exec en
// `devices`, un segundo proyecto, o una entrada de la allowlist por máquina.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE ESTA TABLA
//
// Una fila por cada forma de pasar o no pasar la cadena: los tres lados de PuedeSobreDevice
// (tenencia, concesión, aparato), los cuatro pasos de la precedencia de argvPermitido, los cuatro
// grados de consentimiento más la degradación de `pide`, la resolución del principal, y la
// ventana de mantenimiento. Ésa no es autoridad —la mira aplicarPoliticas antes de llegar a
// autoridadDePolitica— y era el hermano que había quedado afuera del indicador: medido en la
// revisión de A131, con una ventana abierta el inventario decía `puede_actuar: true` sin
// `inerte_por` y el barrido contaba `mantenimiento` sin actuar. Y el estado GLOBAL del barrido
// (A132): apagado y encendido, y el tenant de la máquina fuera del tope, justo adentro, o adentro
// con el tope recortando a otro. Esas cinco filas no pasan por aplicarPoliticas —que por diseño
// corre DESPUÉS de esas compuertas— sino por el scheduler y el barrido REALES: RunFlotaScheduler con
// el intervalo que dejó ConfigurarFlota, y barrerFlotaUnaVez sobre tenants de verdad alrededor de
// casa. Las que actúan son los espejos: sin ellas, un scheduler o un barrido que decidieran con su
// propia regla quedarían en verde. Y a las dos del scheduler se les mide además el CICLO DE VIDA:
// apagado tiene que volver solo, y encendido tiene que seguir barriendo después de actuar (la
// revisión de A132 midió los dos huecos, cada uno en verde con la suite entera). En cada fila se
// afirman TRES cosas, y ninguna alcanza sola:
//
//  1. la ACCIÓN contra un HECHO escrito en la fila (¿encoló o no?). Con la decisión en una sola
//     función, un defecto adentro de ella engaña al indicador y a la acción por igual; sólo un
//     hecho independiente lo ve.
//  2. el INDICADOR contra la acción: `puede_actuar == actuó`, e `inerte_por` igual a la compuerta
//     que la fila dice que frena. Es lo que pone en rojo una compuerta agregada de un solo lado,
//     SI los fixtures de la tabla la atraviesan: una que dependa de algo que ninguna fila arma no
//     se ve. La revisión de A132 midió una así —«no aplicar políticas en un tenant cuya sonda
//     falló»— en verde con la suite entera: todas las máquinas de la tabla son Tier A, y ahí la
//     sonda no corre ni falla. Se mira dos veces, y en este orden: porQueNoActuaria directo, y
//     después lo que publica musubi_fleet_list. Así un indicador mal calculado y un inventario
//     que no le pasa la ventana caen en líneas distintas, y el arnés no los cuenta como uno.
//  3. la MÉTRICA: el resultado contado es el de la fila y ningún otro, recorriendo
//     `resultadosDePolitica` entero.
//
// Y dos controles sobre la tabla misma: cada fila se contrasta con lo que su principal podría
// como PERSONA (PuedeSobreDevice + argvPermitido, las mismas dos funciones que usa
// musubi_fleet_exec) —una política no tiene autoridad propia—, y la tabla tiene que recorrer
// TODAS las constantes de frenoDePolitica, leídas del AST: un freno nuevo sin su fila la pone roja.
//
// Sabotaje: armar la compuerta del auto-heal con el aparato y la concesión pero SIN la tenencia.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="if !PuedeSobreDevice(pr, d, fleet.CapExec) {"
// arnes: a="if !(d.Permite(fleet.CapExec) && tieneGrant(pr, fleet.CapExec, d.Name)) {"
// arnes: colision_ok="TestUnaPoliticaNoPuedeMasQueSuPrincipal"
//
// Sabotaje: que el indicador vuelva a tener su propia cadena, sin la compuerta de `exec`.
//
// Es la cadena de autoridadDePolitica ENTERA menos `exec` —registro, principal, allowlist y
// consentimiento, en orden—, para que el rojo mida SÓLO la compuerta que falta. La versión anterior
// de este sabotaje además convertía `sin_principal` en `allowlist` y borraba el consentimiento, y
// su primer rojo era `inerte_por "allowlist"` en la fila del principal, que no tiene nada que ver
// con exec: la guarda cubría el caso, pero la directiva no lo demostraba. Medido con ésta: rojo
// exactamente en las cuatro filas de exec —el indicador directo en las cuatro, y el inventario en
// las tres donde el detalle viaja (en la del aparato no viaja)—. Las compuertas copiadas van escritas con
// otras palabras (`permitido :=`, `grado :=`) a propósito: con el texto literal de las originales,
// el sabotaje duplicaba el `de` de TestUnaPoliticaRespetaLaAllowlistDeSuPrincipal y el de la
// guarda del eje de consentimiento, y el censo lo denunciaba como un pisotón.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\t_, freno, _ := s.autoridadDePolitica(pol, d)\n\treturn freno\n"
// arnes: a="\tif s.buscarPrincipal == nil {\n\t\treturn frenoSinRegistro\n\t}\n\tpr, ok := s.buscarPrincipal.porNombre(pol.Principal)\n\tif !ok {\n\t\treturn frenoSinPrincipal\n\t}\n\tif permitido := argvPermitido(pr, d, pol.Hacer); !permitido {\n\t\treturn frenoAllowlist\n\t}\n\tswitch grado := d.ConsentimientoEfectivo(); {\n\tcase grado.Bloquea():\n\t\treturn frenoConsentimientoProhibido\n\tcase grado == fleet.ConsentimientoPide:\n\t\treturn frenoConsentimientoPide\n\t}\n\treturn sinFreno\n"
//
// Sabotaje: que el indicador deje de anteponer la ventana de mantenimiento.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tif enMantenimiento {\n\t\treturn frenoMantenimiento\n\t}\n"
// arnes: a=""
//
// Sabotaje: que el inventario busque la ventana por el NOMBRE de la máquina y no por su id (el
// conjunto viene por id): el indicador la sabría anteponer, y nunca le llegaría. Un `false` fijo
// sería el sabotaje obvio y no compila —deja la variable sin usar—, así que su rojo no probaría nada.
// arnes: archivo="internal/mcp/methods_fleet.go"
// arnes: de="s.politicasSobre(p, d, barrido, enMantenimiento[d.ID])"
// arnes: a="s.politicasSobre(p, d, barrido, enMantenimiento[d.Name])"
//
// Sabotaje: que el inventario deje de anteponer el barrido APAGADO (A132): barridoVigente arma la
// lista de tenants aunque el scheduler no vaya a arrancar. Cae la fila del barrido apagado: el
// inventario dice `puede_actuar: true` y el scheduler real vuelve sin barrer.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tif s.barridoApagado() {\n\t\treturn barridoDePoliticas{apagado: true}, nil\n\t}\n"
// arnes: a=""
//
// Sabotaje: el espejo, del lado que actúa: que el scheduler decida con su PROPIA regla —un umbral
// escrito a mano— en vez de barridoApagado. Cae la fila del barrido encendido: con un sondeo de
// 100 ms el inventario dice que actuaría y el scheduler vuelve sin arrancar.
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="\tif s.barridoApagado() {\n\t\treturn\n\t}\n"
// arnes: a="\tif s.sondaIntervalo < time.Second {\n\t\treturn\n\t}\n"
// arnes: colision_ok="TestLaPoliticaActuaDondeSuPrincipalPodriaYElInventarioLoDice"
//
// Sabotaje: con el barrido apagado, que el scheduler NO VUELVA —que se quede vivo hasta que lo
// cancelen—. Es la forma mínima de la mutación que midió la revisión de A132: el «arreglo» que
// invita el pánico de un ticker en 0 es seguir corriendo con el intervalo por default, que
// actuaría a los 5 min, y ninguna espera de la prueba llega a verlo actuar. Cae SÓLO porque la
// fila exige que el scheduler VUELVA SOLO. La primera versión del sabotaje escribía
// `s.sondaIntervalo`, que es el campo que lee el inventario, así que además rompía el indicador,
// y sin la aserción del ciclo de vida seguía en rojo por esa otra línea: no probaba que la aserción
// sirviera (segunda vuelta de la revisión). Pisa el mismo `if` que el sabotaje de arriba, y los dos
// caen en filas distintas: éste en la del apagado, aquél en la del encendido.
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="\tif s.barridoApagado() {\n\t\treturn\n\t}\n"
// arnes: a="\tif s.barridoApagado() {\n\t\t<-ctx.Done()\n\t\treturn\n\t}\n"
// arnes: colision_ok="TestLaPoliticaActuaDondeSuPrincipalPodriaYElInventarioLoDice"
//
// Sabotaje: un Timer donde va el Ticker. El scheduler barre UNA vez, actúa, y no vuelve a barrer
// nunca, con el inventario diciendo que actuaría: cae porque la fila del encendido exige que siga
// barriendo después del primer comando (revisión de A132).
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="\tt := time.NewTicker(s.sondaIntervalo)\n"
// arnes: a="\tt := time.NewTimer(s.sondaIntervalo)\n"
//
// Sabotaje: que el indicador deje de mirar el TOPE del barrido: frenoSobre sin la rama de
// `fuera_del_barrido`. Cae la fila del tenant afuera: casa es el tenant 65, figura
// `puede_actuar: true`, y el barrido real no la visita.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tif !b.proyectos[proyecto] {\n\t\treturn frenoFueraDelBarrido\n\t}\n"
// arnes: a=""
//
// Sabotaje: el espejo del tope, del lado que actúa: que el barrido recorte con su propia cuenta, uno
// menos que la lista que lee el inventario. Cae la fila del tenant justo adentro: casa es el 64, el
// inventario dice que actuaría y el barrido real la deja afuera.
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="\tproyectos, recorto, err := s.proyectosDelBarrido()\n"
// arnes: a="\tproyectos, recorto, err := s.proyectosDelBarrido()\n\tproyectos = proyectos[:min(len(proyectos), proyectosParaVigilar-1)]\n"
//
// Sabotaje: que el barrido trate «hubo recorte» como «el último se va», y recorra otro en su lugar.
// Sólo lo ve la celda en que el tope recorta Y casa queda adentro, como el tenant 64 de 65: casa
// figura `puede_actuar: true` y el barrido no la visita (revisión de A132; hasta ahí la tabla
// armaba el borde sin recorte, y ésta quedaba en verde).
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="\treturn proyectos, nil\n}\n\n// barrerFlotaUnaVez hace UN barrido completo"
// arnes: a="\tif recorto {\n\t\tproyectos[proyectosParaVigilar-1] = proyectos[0]\n\t}\n\treturn proyectos, nil\n}\n\n// barrerFlotaUnaVez hace UN barrido completo"
func TestLaPoliticaActuaDondeSuPrincipalPodriaYElInventarioLoDice(t *testing.T) {
	type fila struct {
		caso          string
		retoque       func(*Principal)     // sobre autoHeal(); nil = tal cual
		sinRegistro   bool                 // s.buscarPrincipal = nil
		caps          []string             // de la máquina; nil = metrics + exec
		grado         fleet.Consentimiento // "" = no se declara
		preguntar     bool
		mantenimiento bool // una ventana abierta sobre la máquina mientras se mide
		// barrido es el estado GLOBAL que se arma, y con él el lado de acción que lo mide (A132):
		// "" = aplicarPoliticas sobre casa; "apagado" / "encendido" = el scheduler REAL con ese
		// sondeo; "fuera" / "dentro" = barrerFlotaUnaVez con casa como tenant 65 / 64.
		barrido   string
		actua     bool
		freno     frenoDePolitica
		resultado string // de resultadosDePolitica; "" = no se cuenta nada
		porque    string
	}
	execSobre := func(sel ...string) func(*Principal) {
		return func(p *Principal) { p.Fleet = map[fleet.Cap][]string{fleet.CapExec: sel, fleet.CapMetrics: {"*"}} }
	}
	allow := func(a map[string][]string) func(*Principal) {
		return func(p *Principal) { p.ExecAllow = a }
	}
	filas := []fila{
		// ── El control positivo y la resolución del principal ──
		{caso: "todo concedido, sin grado declarado", actua: true, freno: sinFreno, resultado: "ok",
			porque: "es el control positivo: sin él, cada fila de «no actúa» pasaría contra un motor de políticas que no hace nada"},
		{caso: "sin registro de principals", sinRegistro: true, actua: false, freno: frenoSinRegistro, resultado: "",
			porque: "sin registro no hay a quién nombrar; no es un fallo de la política y no se cuenta"},
		{caso: "el principal no está en el registro", retoque: func(p *Principal) { p.Name = "otro-nombre" },
			actua: false, freno: frenoSinPrincipal, resultado: "sin_principal",
			porque: "una política no tiene autoridad propia: sin su principal no queda nadie en nombre de quien actuar"},

		// ── PuedeSobreDevice: tenencia, concesión, aparato ──
		{caso: "exec por nombre sobre esta máquina", retoque: execSobre("pc-gio"), actua: true, freno: sinFreno, resultado: "ok",
			porque: "la concesión por nombre vale lo mismo que el comodín sobre la máquina que nombra"},
		{caso: "exec sólo sobre otra máquina", retoque: execSobre("otra-maquina"), actua: false, freno: frenoSinExec, resultado: "rechazada",
			porque: "CONCESIÓN: el principal no tiene exec sobre ésta"},
		{caso: "sin ninguna concesión de exec", retoque: func(p *Principal) { p.Fleet = map[fleet.Cap][]string{fleet.CapMetrics: {"*"}} },
			actua: false, freno: frenoSinExec, resultado: "rechazada",
			porque: "CONCESIÓN: sin la sección no hay capacidad, nunca «todas»"},
		{caso: "principal de otro proyecto (read=own) con exec:*", retoque: func(p *Principal) { p.ProjectID = "otro" },
			actua: false, freno: frenoSinExec, resultado: "rechazada",
			porque: "TENENCIA: la concesión no es una puerta lateral al aislamiento por proyecto, y el barrido aplica todas las políticas a todos los proyectos"},
		{caso: "principal read=all de otro proyecto con exec:*", retoque: func(p *Principal) { p.ProjectID, p.Read = "otro", ReadAll },
			actua: true, freno: sinFreno, resultado: "ok",
			porque: "read=all alcanza todos los proyectos: es lo que PuedeSobreDevice dice, y una política no puede MENOS que su principal"},
		{caso: "máquina dada de alta sin exec", caps: []string{"metrics"}, actua: false, freno: frenoSinExec, resultado: "rechazada",
			porque: "APARATO: ninguna credencial suple una capacidad que la máquina no concedió"},

		// ── argvPermitido: los cuatro pasos de la precedencia ──
		{caso: "allowlist sin sección", retoque: allow(nil), actua: true, freno: sinFreno, resultado: "ok",
			porque: "paso 1: sin sección, exec significa lo de siempre"},
		{caso: "entrada propia de la máquina, sin comodín", retoque: allow(map[string][]string{"pc-gio": {"journalctl"}}),
			actua: true, freno: sinFreno, resultado: "ok",
			porque: "paso 2: la entrada de la máquina manda"},
		{caso: "entrada propia que permite aunque el comodín no", retoque: allow(map[string][]string{"*": {"uptime"}, "pc-gio": {"journalctl"}}),
			actua: true, freno: sinFreno, resultado: "ok",
			porque: "paso 2 en el sentido permisivo: consultar sólo el comodín la frenaría de más"},
		{caso: "entrada propia VACÍA con el comodín que permite", retoque: allow(map[string][]string{"*": {"journalctl"}, "pc-gio": {}}),
			actua: false, freno: frenoAllowlist, resultado: "rechazada",
			porque: "paso 2: la entrada de la máquina manda AUNQUE ESTÉ VACÍA; vacía = cero comandos"},
		{caso: "sólo el comodín, sin el comando", retoque: allow(map[string][]string{"*": {"uptime"}}),
			actua: false, freno: frenoAllowlist, resultado: "rechazada",
			porque: "paso 3: el comodín es el techo general"},
		{caso: "sección que no nombra la máquina ni el comodín", retoque: allow(map[string][]string{"otra-maquina": {"journalctl"}}),
			actua: false, freno: frenoAllowlist, resultado: "rechazada",
			porque: "paso 4: una allowlist a medio escribir falla del lado de «no puede», no del de «puede todo»"},

		// ── El eje de consentimiento (A91) ──
		{caso: "grado libre", grado: fleet.ConsentimientoLibre, preguntar: true, actua: true, freno: sinFreno, resultado: "ok",
			porque: "`libre` es «ni se entera», elegido"},
		{caso: "grado avisa", grado: fleet.ConsentimientoAvisa, preguntar: true, actua: true, freno: sinFreno, resultado: "ok",
			porque: "`avisa` notifica sin veto"},
		{caso: "grado pide, la máquina sabe preguntar", grado: fleet.ConsentimientoPide, preguntar: true,
			actua: false, freno: frenoConsentimientoPide, resultado: "consentimiento_pide",
			porque: "`pide` promete que su usuario ACEPTE, y un barrido no tiene dónde esperar la respuesta (A86)"},
		{caso: "grado prohibido", grado: fleet.ConsentimientoProhibido, preguntar: true,
			actua: false, freno: frenoConsentimientoProhibido, resultado: "consentimiento_prohibido",
			porque: "el candado del dueño de la máquina no lo abre un temporizador"},
		{caso: "grado pide en una máquina que no sabe preguntar", grado: fleet.ConsentimientoPide, preguntar: false,
			actua: false, freno: frenoConsentimientoProhibido, resultado: "consentimiento_prohibido",
			porque: "`pide` sin forma de preguntar se endurece a `prohibido` (ConsentimientoEfectivo)"},

		// ── La pausa que no es autoridad: la ventana de mantenimiento ──
		{caso: "ventana de mantenimiento abierta, todo lo demás concedido", mantenimiento: true,
			actua: false, freno: frenoMantenimiento, resultado: "mantenimiento",
			porque: "la ventana frena el auto-heal (Ola 1), y un inventario que no lo dice convierte una ventana olvidada en una alarma apagada con el panel en verde"},

		// ── El barrido global: si corre, y sobre qué tenants (A132) ──
		{caso: "barrido apagado (probe_minutes negativo), todo lo demás concedido", barrido: "apagado",
			actua: false, freno: frenoBarridoApagado, resultado: "",
			porque: "con el barrido apagado RunFlotaScheduler tiene que volver solo, sin evaluar ninguna política: no se cuenta nada"},
		{caso: "barrido encendido: el scheduler real actúa", barrido: "encendido",
			actua: true, freno: sinFreno, resultado: "ok",
			porque: "es el espejo del apagado con el MISMO scheduler, que además tiene que seguir barriendo: sin él, uno que decidiera con su propia regla quedaría en verde"},
		{caso: "el tenant de la máquina queda fuera del tope del barrido", barrido: "fuera",
			actua: false, freno: frenoFueraDelBarrido, resultado: "",
			porque: "un tenant que no entra en proyectosParaVigilar no se barre, así que sus políticas no llegan a evaluarse"},
		{caso: "el tenant de la máquina entra justo en el tope", barrido: "dentro",
			actua: true, freno: sinFreno, resultado: "ok",
			porque: "es el espejo en el borde: casa es el tenant 64 y el barrido real la visita; sin él, un barrido que recortara uno menos quedaría en verde"},
		{caso: "el tope recorta a otro y la máquina queda adentro", barrido: "dentro-recortando",
			actua: true, freno: sinFreno, resultado: "ok",
			porque: "casa es el tenant 64 de 65: el tope recorta al de atrás, no a casa; un lado que leyera «hubo recorte» como «casa quedó afuera» sólo cae acá"},
	}

	// LA TABLA RECORRE EL CONJUNTO ENTERO, derivado de su fuente.
	cubiertos := map[frenoDePolitica]bool{}
	for _, f := range filas {
		cubiertos[f.freno] = true
	}
	var sinFila []string
	for freno, nombre := range frenosDeclarados(t) {
		if !cubiertos[freno] {
			sinFila = append(sinFila, nombre+" ("+strconv.Quote(string(freno))+")")
		}
	}
	sort.Strings(sinFila)
	if len(sinFila) > 0 {
		t.Errorf("la tabla no tiene ninguna fila para %s: una compuerta sin fila es exactamente la que "+
			"puede agregarse de un solo lado —a la acción o al inventario— sin que nada se ponga rojo", strings.Join(sinFila, ", "))
	}

	for _, f := range filas {
		t.Run(f.caso, func(t *testing.T) {
			caps := f.caps
			if caps == nil {
				caps = []string{"metrics", "exec"}
			}
			s, d := maquinaConPolitica(t, caps)
			pr := autoHeal()
			if f.retoque != nil {
				f.retoque(&pr)
			}
			s.buscarPrincipal = registroDePrueba(pr)
			if f.sinRegistro {
				s.buscarPrincipal = nil
			}
			if f.grado != "" {
				if err := s.engine.FijarCapacidadDePreguntar(d.ID, f.preguntar); err != nil {
					t.Fatalf("FijarCapacidadDePreguntar: %v", err)
				}
				if _, err := s.engine.FijarConsentimiento(d.ID, f.grado); err != nil {
					t.Fatalf("FijarConsentimiento: %v", err)
				}
			}
			ahora := time.Now()
			if f.mantenimiento {
				if _, err := s.engine.AbrirMantenimiento(fleet.Mantenimiento{
					DeviceID: d.ID, ProjectID: d.ProjectID, Principal: "gio",
					Desde: ahora.Add(-time.Minute), Hasta: ahora.Add(time.Hour), Motivo: "migración de postgres",
				}); err != nil {
					t.Fatalf("AbrirMantenimiento: %v", err)
				}
			}
			// EL ESTADO GLOBAL DEL BARRIDO (A132), armado con lo mismo que lo decide en producción: el
			// intervalo, por ConfigurarFlota; el tope, con tenants reales que ordenan antes que casa.
			switch f.barrido {
			case "apagado", "encendido":
				pm := -1.0 // negativo: el apagado explícito de EffectiveProbeInterval
				if f.barrido == "encendido" {
					pm = sondeoCortoDePrueba.Minutes()
				}
				if err := s.ConfigurarFlota(config.FleetConfig{ProbeMinutes: pm, Policies: []config.PolicyConfig{politicaDeMemoria()}}); err != nil {
					t.Fatalf("ConfigurarFlota(probe_minutes=%v): %v", pm, err)
				}
			case "fuera":
				tenantsAlrededorDeCasa(t, s, proyectosParaVigilar, 0, ahora)
			case "dentro":
				tenantsAlrededorDeCasa(t, s, proyectosParaVigilar-1, 0, ahora)
			case "dentro-recortando":
				tenantsAlrededorDeCasa(t, s, proyectosParaVigilar-1, 1, ahora)
			}
			latir(t, s, d.ID, muestraSana(95, ahora), ahora) // 95 % de RAM: la condición se cumple
			d, _, _ = s.engine.DevicePorNombre("casa", "pc-gio")

			// 1. LO QUE VE UNA PERSONA, antes de que nada dispare.
			puede, inertePor, visible := politicaEnElInventario(t, s)
			// 2. LO QUE PASA: la cola de la máquina, no el contador de acciones. Las filas del barrido
			// global actúan por el camino ENTERO que las decide; las demás, por aplicarPoliticas.
			antes := comandosDePolitica(t, s)
			var como comoCorrioElScheduler
			switch f.barrido {
			case "apagado", "encendido":
				como = correrElSchedulerReal(t, s, antes)
			case "fuera", "dentro", "dentro-recortando":
				s.barrerFlotaUnaVez(context.Background())
			default:
				s.aplicarPoliticas("casa", ahora)
			}
			actuo := comandosDePolitica(t, s) > antes

			if actuo != f.actua {
				t.Errorf("la política actuó=%v y la fila dice actuó=%v: %s", actuo, f.actua, f.porque)
			}
			// EL CICLO DE VIDA DEL SCHEDULER, no sólo la cola (revisión de A132). «No encoló nada en
			// diez segundos» no es «no actuaría»: uno que siguiera corriendo con otro intervalo actuaría
			// a los cinco minutos. Y «actuó» no es «está barriendo»: uno que barre una vez y se queda
			// quieto también actuó.
			switch f.barrido {
			case "apagado":
				if !como.volvioSolo {
					t.Errorf("con el barrido apagado el scheduler real NO volvió solo: sigue corriendo, así que actuaría " +
						"en su próximo tick con el inventario diciendo `barrido_apagado`")
				}
			case "encendido":
				if como.volvioSolo {
					t.Errorf("con el barrido encendido el scheduler real volvió solo: nadie barre, y el inventario dice que actuaría")
				}
				if !como.siguioBarriendo {
					t.Errorf("con el barrido encendido el scheduler real no volvió a barrer después del primer comando: " +
						"actuó una vez y se quedó quieto, y el inventario sigue diciendo que actuaría")
				}
			}
			// EL INDICADOR, PRIMERO DIRECTO Y DESPUÉS POR EL INVENTARIO. Son dos fallas distintas y el
			// rojo tiene que decir cuál es: si miente porQueNoActuaria, cae acá; si acierta y el
			// inventario igual dice otra cosa, la falla es de quien se lo pasa (musubi_fleet_list, que
			// le da la ventana), y cae más abajo. Las ventanas y el barrido se leen como los lee
			// musubi_fleet_list: no se le pasa la fila.
			enVentana := s.ventanasParaPoliticas(time.Now())[d.ID]
			if got := s.porQueNoActuaria(s.politicas[0], d, barridoDelInventario(t, s), enVentana); (got == sinFreno) != actuo || got != f.freno {
				t.Errorf("el indicador dice freno=%q y la política actuó=%v; la fila dice %q: el `puede_actuar` "+
					"se calcula distinto de lo que decide la acción (%s)", got, actuo, f.freno, f.porque)
			}
			if visible {
				if puede != actuo {
					t.Errorf("el inventario dice `puede_actuar: %v` y la política actuó=%v: un indicador que "+
						"contradice a la acción enseña a confiar en una alarma apagada (%s)", puede, actuo, f.porque)
				}
				if inertePor != string(f.freno) {
					t.Errorf("el inventario dice `inerte_por: %q` y la fila dice %q (vacío = ninguna compuerta la "+
						"frena): el panel manda a arreglarla al lugar equivocado, o marca inerte a una que actúa", inertePor, f.freno)
				}
			} else if d.Permite(fleet.CapExec) {
				// El detalle no viaja a NADIE cuando la máquina no admite exec: el mirón de
				// politicaEnElInventario tiene exec:* sobre casa, así que es la única razón posible.
				t.Errorf("el detalle de la política no viajó a una credencial con exec:* sobre una máquina que admite exec")
			}

			// 3. LA MÉTRICA: exactamente el resultado de la fila, recorriendo el conjunto sembrado.
			for _, r := range resultadosDePolitica {
				var quiero int64
				if r == f.resultado {
					quiero = 1
				}
				if got := contarPoliticaOK(s, "vaciar-journal", r); got != quiero {
					t.Errorf("resultado %q contó %d y se esperaba %d: las alertas PoliticaSinPermiso y "+
						"PoliticaFrenadaPorConsentimiento viven de este contador", r, got, quiero)
				}
			}

			// 4. LA FILA CONTRA LA FUENTE: lo que el principal podría como PERSONA, con las mismas dos
			// funciones que usa musubi_fleet_exec. Una fila que dijera «actúa» donde la persona no
			// podría estaría escribiendo como correcta la autoridad propia que I11 prohíbe.
			var persona bool
			if !f.sinRegistro {
				if p, ok := s.buscarPrincipal.porNombre(politicaDeMemoria().Principal); ok {
					persona = PuedeSobreDevice(p, d, fleet.CapExec) && argvPermitido(p, d, politicaDeMemoria().Run)
				}
			}
			if f.actua && !persona {
				t.Errorf("LA FILA ESTÁ MAL ESCRITA: dice que la política actúa donde su principal, como persona, no podría")
			}
			if (f.freno == frenoSinExec || f.freno == frenoAllowlist || f.freno == frenoSinPrincipal) && persona {
				t.Errorf("LA FILA ESTÁ MAL ESCRITA: dice %q, pero su principal como persona SÍ podría", f.freno)
			}
			if (f.freno == frenoConsentimientoPide || f.freno == frenoConsentimientoProhibido || f.freno == frenoMantenimiento ||
				f.freno == frenoBarridoApagado || f.freno == frenoFueraDelBarrido) && !persona {
				t.Errorf("LA FILA NO MIDE EL EJE: dice %q, pero ya la frenaría la compuerta de la persona; el grado, "+
					"la ventana o el barrido tienen que ser lo ÚNICO que la frena", f.freno)
			}
		})
	}
}

// A131 · CADA FORMA DE QUITARLE AUTORIDAD AL PRINCIPAL APAGA LA POLÍTICA, SOBRE EL MISMO SERVIDOR.
//
// TestRevocarAlPrincipalApagaLaPolitica modela «revocar» de UNA forma: borrar al principal del
// registro. Eso mide que la EXISTENCIA se re-resuelve en cada tick, y nada más: una versión que
// re-chequea la existencia pero actúa con una copia guardada del principal (P1-m5) la dejaba en
// verde, y quitarle a auto-heal el exec sobre una máquina en caliente no apagaba la política.
//
// Acá se recorren las ediciones que la recarga de principals.yaml (cada 10 s) puede producir
// —quitar exec sobre la máquina, achicar la allowlist, mudarlo de proyecto, vencerlo, borrarlo— y
// después de CADA una se restituye y se comprueba que vuelve a actuar. Esa alternancia es el
// punto: cada «no actúa» llega inmediatamente después de un disparo, que es cuando cualquier copia
// del principal está recién guardada. Y en cada paso el inventario tiene que decir lo mismo que
// la acción.
//
// Exposición medida: 0. `vaciar-journal` no disparó en la ventana medida (27 días de TSDB), así
// que ninguna copia se habría llenado; hacían falta un disparo, una edición que acote sin borrar y
// otro disparo pasado el cooldown, en el mismo proceso.
//
// Sabotaje: volver a comprobar que el principal EXISTE en cada tick pero usar una copia guardada.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tif !existe {\n\t\treturn nil, frenoSinPrincipal, \"\"\n\t}\n"
// arnes: a="\tif !existe {\n\t\treturn nil, frenoSinPrincipal, \"\"\n\t}\n\tif v, hay := s.avisosDados.Load(\"pr_cache:\" + pol.Nombre); hay {\n\t\tpr = v.(*Principal)\n\t} else {\n\t\ts.avisosDados.Store(\"pr_cache:\"+pol.Nombre, pr)\n\t}\n"
func TestCadaFormaDeQuitarleAutoridadAlPrincipalApagaLaPolitica(t *testing.T) {
	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
	vigente := func() *PrincipalRegistry { return registroDePrueba(autoHeal()) }
	con := func(retoque func(*Principal)) *PrincipalRegistry {
		p := autoHeal()
		retoque(&p)
		return registroDePrueba(p)
	}
	pasos := []struct {
		caso     string
		registro *PrincipalRegistry
		actua    bool
		freno    frenoDePolitica
	}{
		{"vigente: dispara y deja al principal recién resuelto", vigente(), true, sinFreno},
		{"se le quita exec sobre esta máquina", con(func(p *Principal) {
			p.Fleet = map[fleet.Cap][]string{fleet.CapExec: {"otra-maquina"}, fleet.CapMetrics: {"*"}}
		}), false, frenoSinExec},
		{"se le restituye", vigente(), true, sinFreno},
		{"se le achica la allowlist", con(func(p *Principal) { p.ExecAllow = map[string][]string{"*": {"uptime"}} }), false, frenoAllowlist},
		{"se le restituye", vigente(), true, sinFreno},
		{"se lo muda a otro proyecto", con(func(p *Principal) { p.ProjectID = "otro" }), false, frenoSinExec},
		{"se le restituye", vigente(), true, sinFreno},
		{"se le vence la credencial", con(func(p *Principal) { p.Expires = time.Now().Add(-time.Hour) }), false, frenoSinPrincipal},
		{"se le restituye", vigente(), true, sinFreno},
		{"se lo borra del registro", registroDePrueba(), false, frenoSinPrincipal},
		{"se le restituye", vigente(), true, sinFreno},
	}

	ahora := time.Now()
	vigentesQueActuaron := 0
	for i, p := range pasos {
		// 70 min entre pasos: más que el cooldown de 60, así que el cooldown no frena ninguno, y se
		// vuelve a latir con la RAM alta para que tampoco lo frene la muestra rancia (I13). Lo único
		// que cambia entre un paso y el siguiente es el registro.
		cuando := ahora.Add(time.Duration(i) * 70 * time.Minute)
		s.buscarPrincipal = p.registro
		latir(t, s, d.ID, muestraSana(95, cuando), cuando)

		puede, inertePor, visible := politicaEnElInventario(t, s)
		antes := comandosDePolitica(t, s)
		s.aplicarPoliticas("casa", cuando)
		actuo := comandosDePolitica(t, s) > antes
		if actuo && p.actua {
			vigentesQueActuaron++
		}

		if actuo != p.actua {
			t.Errorf("paso %d (%s): la política actuó=%v y tenía que actuar=%v. Actuar sin la autoridad que le da "+
				"el registro VIGENTE es saltarse una compuerta o usar una copia vieja de su principal: la "+
				"revocación en caliente deja de apagar la política", i, p.caso, actuo, p.actua)
		}
		if !visible {
			t.Fatalf("paso %d (%s): el detalle de la política no viajó a una credencial con exec:* sobre la máquina", i, p.caso)
		}
		if puede != actuo {
			t.Errorf("paso %d (%s): el inventario dice `puede_actuar: %v` y la política actuó=%v", i, p.caso, puede, actuo)
		}
		if inertePor != string(p.freno) {
			t.Errorf("paso %d (%s): el inventario dice `inerte_por: %q` y la compuerta que frena es %q", i, p.caso, inertePor, p.freno)
		}
	}
	// CONTROL POSITIVO: los seis pasos vigentes tienen que haber actuado. Si el motor no actuara
	// nunca, cada «no actúa» de arriba pasaría por el motivo equivocado, y sin un disparo justo antes
	// de cada edición no habría ninguna copia del principal que pudiera quedar vieja.
	if vigentesQueActuaron != 6 {
		t.Errorf("de los 6 pasos con el principal vigente actuaron %d: la alternancia disparo → edición no se ejercitó", vigentesQueActuaron)
	}
}

// A131 · LA POLÍTICA ENCOLA EL ARGV ENTERO QUE AUTORIZÓ SU ALLOWLIST, Y ÉSE ES EL QUE SE AUDITA.
//
// TestLaAccionDeUnaPoliticaQuedaEnLaMismaBitacoraQueLasPersonas busca `journalctl` en la bitácora,
// o sea argv[0]. Encolar `journalctl` pelado en vez de `journalctl --vacuum-size=200M` (P2-m2) la
// dejaba en verde, y son comandos distintos: el primero vuelca el journal entero por la salida, el
// segundo lo recorta. Lo que la allowlist validó y lo que el agente corre tienen que ser el mismo
// argv, y lo que una persona lee en la bitácora también.
//
// Exposición medida: el estado existe en producción. `vaciar-journal` tiene un argv de dos
// elementos, y las 7 acciones de política de la historia (de `prueba-de-fuego`, 2026-08-31) eran
// todas de dos elementos: las 7 habrían corrido truncadas.
//
// Sabotaje: encolar sólo el primer elemento del argv de la política.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="Argv:   pol.Hacer, Timeout: fleet.ComandoTimeoutDefault,"
// arnes: a="Argv:   pol.Hacer[:1], Timeout: fleet.ComandoTimeoutDefault,"
// arnes: colision_ok="TestLaAccionDeUnaPoliticaQuedaEnLaMismaBitacoraQueLasPersonas"
func TestLaPoliticaEncolaElArgvEnteroQueAutorizoSuAllowlist(t *testing.T) {
	cfg := politicaDeMemoria()
	// PISO: con un argv de un solo elemento, «entero» y «el primero» son lo mismo y esto no mide nada.
	if len(cfg.Run) < 2 {
		t.Fatalf("la política de referencia tiene %d elemento(s) en `run`: con menos de dos esta prueba no distingue un argv recortado", len(cfg.Run))
	}
	s, d := prepararPolitica(t, cfg, registroDePrueba(autoHeal()))
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(95, ahora), ahora)
	if n := s.aplicarPoliticas("casa", ahora); n != 1 {
		t.Fatalf("control: la política tendría que actuar una vez y actuó %d", n)
	}

	// LA COLA: lo que el agente va a correr. Un disparo deja también el `musubi:avisar` al dueño de
	// la máquina, y ése no es el comando de la política; desde A131 (T5) el aviso también sale con
	// Origen=politica —lo escribe el mismo barrido—, así que se separa por el argv y no por el origen.
	var dePolitica []fleet.Comando
	for _, c := range comandosEncolados(t, s) {
		if c.Origen == fleet.OrigenPolitica && (len(c.Argv) == 0 || c.Argv[0] != comandoAviso) {
			dePolitica = append(dePolitica, c)
		}
	}
	if len(dePolitica) != 1 {
		t.Fatalf("se encolaron %d comando(s) de política y tenía que ser 1", len(dePolitica))
	}
	if !reflect.DeepEqual(dePolitica[0].Argv, cfg.Run) {
		t.Errorf("la política encoló %q y su configuración dice %q: el agente va a correr un comando distinto "+
			"del que la allowlist de su principal autorizó y del que está escrito en la política", dePolitica[0].Argv, cfg.Run)
	}

	// LA BITÁCORA: lo que una persona audita, con la misma tool y una credencial de persona.
	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_log", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	comandos, _ := jsonOf(t, res)["comandos"].([]any)
	vistos := 0
	for _, x := range comandos {
		fila, _ := x.(map[string]any)
		crudo, _ := fila["argv"].([]any)
		var argv []string
		for _, a := range crudo {
			v, _ := a.(string)
			argv = append(argv, v)
		}
		// El aviso al dueño también es `politica` (A131 · T5): se lo saltea por el argv, como en la cola.
		if fila["origen"] != "politica" || (len(argv) > 0 && argv[0] == comandoAviso) {
			continue
		}
		vistos++
		if !reflect.DeepEqual(argv, cfg.Run) {
			t.Errorf("la bitácora muestra %q para la acción automática y la política está escrita como %q: quien "+
				"audita lee un comando que no es el que se configuró", argv, cfg.Run)
		}
	}
	if vistos != 1 {
		t.Errorf("la bitácora de las personas muestra %d acción(es) de política y tenía que mostrar 1", vistos)
	}
}

// A131 · EL DETALLE DE UNA POLÍTICA LO VE SÓLO QUIEN PUEDE EJECUTAR EN ESA MÁQUINA.
//
// TestSinExecSeVeQueHayAlgoAutomaticoPeroNoQueHace usa un mirón sin `exec` en NINGÚN lado. Así
// no distingue «exec sobre ESTA máquina» —la regla, la misma que la bitácora— de «exec en alguna
// máquina»: un gate escrito con la segunda (P3-m10) la dejaba en verde y le mostraba qué comando
// corre en pc-gio a una credencial que sólo puede ejecutar en otra.
//
// La tabla recorre cada forma en que PuedeSobreDevice dice que no —concesión sobre otra máquina,
// otro proyecto, sólo metrics, sin sección, máquina que no admite exec, máquina revocada— y las
// que dice que sí. El conjunto lo cierran los tres pasos de fleet_authz.go (aparato, tenencia,
// concesión), y cada fila se contrasta con PuedeSobreDevice mismo: la tabla no puede decir una
// regla distinta de la que la compuerta aplica.
//
// Se llama a politicasSobre y no a fleet_list porque fleet_list ya filtra las máquinas por
// proyecto, y la fila de «otro proyecto» no llegaría nunca hasta el gate que se quiere medir.
//
// Exposición medida: 0. Los 4 principals con exec lo tienen como `["*"]` y todos son de musubi, así
// que hoy «exec en alguna» y «exec sobre ésta» coinciden para cada credencial.
//
// Sabotaje: gatear el detalle con «tiene exec en alguna máquina» en vez de «sobre ésta».
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="verDetalle := PuedeSobreDevice(p, d, fleet.CapExec)"
// arnes: a="verDetalle := p == nil || len(p.Fleet[fleet.CapExec]) > 0"
// arnes: colision_ok="TestSinExecSeVeQueHayAlgoAutomaticoPeroNoQueHace"
func TestElDetalleDeUnaPoliticaLoVeSoloQuienPuedeEjecutarEnEsaMaquina(t *testing.T) {
	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
	miron := func(proyecto, read string, concesion map[fleet.Cap][]string) *Principal {
		return &Principal{Name: "mirón", Role: RoleReader, Read: read, Write: WriteNone, ProjectID: proyecto, Fleet: concesion}
	}
	sinExec := d
	sinExec.Caps = []fleet.Cap{fleet.CapMetrics}
	revocada := d
	revocada.Revoked = true

	casos := []struct {
		caso    string
		p       *Principal
		maquina fleet.Device
		ve      bool
	}{
		{"stdio local, sin principal", nil, d, true},
		{"exec:* en su proyecto", conExec("casa"), d, true},
		{"exec por nombre sobre esta máquina", miron("casa", ReadOwn, map[fleet.Cap][]string{fleet.CapExec: {"pc-gio"}}), d, true},
		{"read=all de otro proyecto con exec:*", miron("otro", ReadAll, map[fleet.Cap][]string{fleet.CapExec: {"*"}}), d, true},
		{"exec sólo sobre otra máquina", miron("casa", ReadOwn, map[fleet.Cap][]string{fleet.CapExec: {"otra-pc"}, fleet.CapMetrics: {"*"}}), d, false},
		{"exec:* pero de otro proyecto (read=own)", miron("otro", ReadOwn, map[fleet.Cap][]string{fleet.CapExec: {"*"}}), d, false},
		{"sólo metrics", miron("casa", ReadOwn, map[fleet.Cap][]string{fleet.CapMetrics: {"*"}}), d, false},
		{"sin sección fleet", miron("casa", ReadOwn, nil), d, false},
		{"exec:* sobre una máquina que no admite exec", conExec("casa"), sinExec, false},
		{"exec:* sobre una máquina revocada", conExec("casa"), revocada, false},
	}
	for _, c := range casos {
		t.Run(c.caso, func(t *testing.T) {
			if fuente := PuedeSobreDevice(c.p, c.maquina, fleet.CapExec); fuente != c.ve {
				t.Fatalf("LA FILA ESTÁ MAL ESCRITA: dice ve=%v y PuedeSobreDevice dice %v; la regla del detalle es la compuerta", c.ve, fuente)
			}
			detalle, total := s.politicasSobre(c.p, c.maquina, barridoDelInventario(t, s), false)
			if total != 1 {
				t.Errorf("politicas_activas = %d: el CONTEO se muestra a cualquiera que vea la máquina, y sin él "+
					"alguien la ve cambiar sin ninguna pista de por qué", total)
			}
			if hay := len(detalle) > 0; hay != c.ve {
				if hay {
					t.Errorf("FUGA: el detalle de la política (qué comando corre y con la autoridad de quién) "+
						"viajó a una credencial que no puede ejecutar en %s", c.maquina.Name)
				} else {
					t.Errorf("el detalle no viajó a una credencial que SÍ puede ejecutar en %s: no puede ver "+
						"qué le va a correr la automatización", c.maquina.Name)
				}
			}
		})
	}
}

// A131 · LAS PIEZAS DE LA COMPUERTA SÓLO SE USAN ADENTRO DE LA COMPUERTA.
//
// P1-m1 y P1-m2 reescribían la compuerta del auto-heal con dos de sus tres lados —`d.Permite` +
// `tieneGrant` sin la tenencia, o `alcanzaElProyecto` + `tieneGrant` sin el aparato— y la guarda
// de comportamiento no lo veía porque clavaba el otro eje. La tabla de
// TestLaPoliticaActuaDondeSuPrincipalPodriaYElInventarioLoDice lo cubre para las políticas; esto
// lo vuelve imposible de escribir en CUALQUIER camino del paquete: exec, pantalla, shell, la
// bitácora, el inventario.
//
// El conjunto de consumidores de `alcanzaElProyecto` y `tieneGrant` es uno solo, fleet_authz.go
// —medido hoy, y es el diseño: «la respuesta es una conjunción de tres lados, y ninguno puede
// suplir a otro»—. Quien necesite una decisión de flota llama a PuedeSobreDevice entera. Si algún
// día hace falta de verdad una pieza suelta, este es el lugar donde se discute, no un descuido.
//
// Sabotaje: armar la compuerta de la bitácora a mano con el aparato y la concesión, sin la tenencia.
// arnes: archivo="internal/mcp/methods_exec.go"
// arnes: de="\t\tif PuedeSobreDevice(p, d, fleet.CapExec) {\n\t\t\tnombrePorID[d.ID] = d.Name"
// arnes: a="\t\tif d.Permite(fleet.CapExec) && tieneGrant(p, fleet.CapExec, d.Name) {\n\t\t\tnombrePorID[d.ID] = d.Name"
func TestLasPiezasDeLaCompuertaDeFlotaSoloSeUsanAdentroDeLaCompuerta(t *testing.T) {
	const casa = "fleet_authz.go"
	piezas := map[string]bool{"alcanzaElProyecto": true, "tieneGrant": true}

	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("no se pudo leer el paquete: %v", err)
	}
	adentro := map[string]int{}
	var afuera []string
	fset := token.NewFileSet()
	for _, e := range entradas {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, 0)
		if err != nil {
			t.Fatalf("no se pudo parsear %s: %v", n, err)
		}
		// Se cuenta TODA referencia y no sólo las llamadas: pasar `tieneGrant` como valor a otra
		// función es la misma decisión armada afuera. Las declaraciones se saltean.
		declaradas := map[*ast.Ident]bool{}
		for _, decl := range f.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok {
				declaradas[fd.Name] = true
			}
		}
		ast.Inspect(f, func(nodo ast.Node) bool {
			id, ok := nodo.(*ast.Ident)
			if !ok || !piezas[id.Name] || declaradas[id] {
				return true
			}
			if n == casa {
				adentro[id.Name]++
			} else {
				afuera = append(afuera, fset.Position(id.Pos()).String()+" usa "+id.Name)
			}
			return true
		})
	}

	// PISO: si las piezas cambiaron de nombre, «nadie las usa afuera» sería un cero que significa
	// «no las encontré», no «está bien».
	for p := range piezas {
		if adentro[p] == 0 {
			t.Fatalf("%s no aparece usada en %s: cambió de nombre o de lugar, y esta guarda quedó mirando al aire", p, casa)
		}
	}
	sort.Strings(afuera)
	for _, x := range afuera {
		t.Errorf("%s fuera de %s. Una decisión de flota armada con piezas sueltas deja afuera algún lado de "+
			"la compuerta —tenencia, concesión o aparato— y el camino que la use puede más que la credencial. "+
			"Llamá a PuedeSobreDevice entera.", x, casa)
	}
}
