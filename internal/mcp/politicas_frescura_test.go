package mcp

// A131 · T2 — LA FRESCURA QUE FRENA A UNA POLÍTICA ES LA DE SU CLASE, MEDIDA CON EL UMBRAL QUE LE
// TOCA Y CON EL RELOJ DEL CEREBRO.
//
// Una política de HOST decide sobre la muestra, y la muestra sólo vale si la máquina LATE y el dato
// es RECIENTE, las dos cosas contra el mismo umbral que decide «en línea» para su tier (I2 · I13).
// Una política de SERVICIO decide sobre el inventario, con el umbral del inventario, y la muestra
// del host no le cambia nada (A44). Las guardas de antes decían exactamente eso y lo medían en un
// solo punto de cada eje; las de este archivo recorren los ejes enteros —los tiers leídos del
// código, las tres horas que guarda el registro de una máquina (su último latido, la llegada de su
// muestra y la fecha que le puso el agente), un estado de la muestra por cada condición de la
// guarda de host— y entran por el barrido que corre en producción, aplicarPoliticas.
//
// «Reciente» lo decide el reloj del CEREBRO (decisión del usuario en el pulido de T2): una muestra
// fechada después de llegar se guarda con la hora en que llegó (fleet.Muestra.RecortadaALaLlegada,
// aplicada en memory.latirDeviceCon). Un reloj de agente adelantado no apaga las políticas, y
// tampoco rejuvenece una muestra vieja.
//
// Lo que NO miran, a propósito, es `puede_actuar`: la muestra fresca queda FUERA de ese indicador
// por decisión de A131·T1 (ver «LO QUE NO CONTESTA» en el doc de porQueNoActuaria). El indicador
// contesta «si la condición se cumpliera», y la frescura es parte de la condición.

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// tiersDeclarados devuelve el valor de TODA constante de tipo fleet.Tier declarada en los .go del
// paquete internal/fleet que no son de prueba (de cualquier plataforma), en el orden en que están
// escritas: el conjunto CERRADO de tiers que recorren las tablas de este archivo. Un tier nuevo
// entra solo a las dos tablas, y si el alta no lo sabe dar, la tabla se pone roja en vez de
// saltearlo.
//
// LO DECIDE go/types, NO LA FORMA EN QUE ESTÁ ESCRITA. Hasta el pulido de T2 leía sólo device.go y
// sólo `X Tier = "…"`: un `TierD = Tier("D")`, o un tier declarado en otro archivo, se salteaba y
// las tablas quedaban en verde sobre A, B y C. Preguntarle al verificador de tipos «¿qué
// constantes tienen tipo Tier?» contesta por todas las formas a la vez —conversión, literal,
// referencia a otra constante— en vez de enumerarlas. Los paquetes importados se reemplazan por
// paquetes vacíos: el tipo Tier y sus constantes no dependen de ningún import, y los errores que eso
// produce en el resto del paquete no cambian nada acá (el PISO de abajo lo confirma).
func tiersDeclarados(t *testing.T) []fleet.Tier {
	t.Helper()
	const dir = "../fleet"
	paquete, err := build.ImportDir(dir, 0)
	if err != nil {
		t.Fatalf("no se pudo leer el paquete %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var archivos []*ast.File
	for _, nombre := range append(append([]string{}, paquete.GoFiles...), paquete.IgnoredGoFiles...) {
		if strings.HasSuffix(nombre, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, nombre), nil, 0)
		if err != nil {
			t.Fatalf("no se pudo parsear %s: %v", nombre, err)
		}
		if f.Name.Name == "fleet" {
			archivos = append(archivos, f)
		}
	}
	info := &types.Info{Defs: map[*ast.Ident]types.Object{}}
	conf := types.Config{Importer: importadorVacio{}, Error: func(error) {}}
	pkg, _ := conf.Check("musubi/internal/fleet", fset, archivos, info)
	tipo, _ := pkg.Scope().Lookup("Tier").(*types.TypeName)
	if tipo == nil {
		t.Fatalf("el paquete %s ya no declara el tipo Tier: las tablas de frescura no tienen qué tiers recorrer", dir)
	}
	vistos := map[fleet.Tier]bool{}
	var out []fleet.Tier
	for _, f := range archivos {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				for _, nombre := range spec.(*ast.ValueSpec).Names {
					c, ok := info.Defs[nombre].(*types.Const)
					if !ok || !types.Identical(c.Type(), tipo.Type()) {
						continue
					}
					if c.Val().Kind() != constant.String {
						t.Fatalf("%s: %s es una constante de tipo Tier y su valor no se puede leer (%v): esta guarda no sabe qué tier es, y saltearla es justo lo que vino a evitar",
							fset.Position(nombre.Pos()), nombre.Name, c.Val())
					}
					if tier := fleet.Tier(constant.StringVal(c.Val())); !vistos[tier] {
						vistos[tier] = true
						out = append(out, tier)
					}
				}
			}
		}
	}
	// PISO: lo que el COMPILADOR conoce tiene que estar en lo que se leyó. Si el verificador de tipos
	// se degradara (un import que ahora sí importa, un archivo que no se parsea), la lista saldría
	// corta y las tablas mirarían menos tiers sin decirlo: el eje que dejaba pasar a P1-m8.
	for _, conocido := range []fleet.Tier{fleet.TierAgente, fleet.TierProtocolo, fleet.TierMovil} {
		if !vistos[conocido] {
			t.Fatalf("PISO: el tier %q existe para el compilador y la lectura de %s no lo encontró (leyó %v): las tablas de frescura quedaron mirando menos tiers de los que hay", conocido, dir, out)
		}
	}
	return out
}

// importadorVacio contesta cada import con un paquete vacío y completo. Ver tiersDeclarados.
type importadorVacio struct{}

func (importadorVacio) Import(ruta string) (*types.Package, error) {
	p := types.NewPackage(ruta, ruta[strings.LastIndex(ruta, "/")+1:])
	p.MarkComplete()
	return p, nil
}

// sondeosDeLasTablasDeFrescura son los `probe_minutes` con los que corren las tablas de este
// archivo: 0 es el default (5 min) y 10 es otro. Con un solo intervalo, un umbral calculado con la
// fórmula de OTRO tier puede coincidir por casualidad con el bueno; con dos, el de un tier sin agente
// se mueve y el de un Tier A no, que es lo que dice I2.
var sondeosDeLasTablasDeFrescura = []float64{0, 10}

// maquinaDeTier da de alta en la casa una máquina del tier dado, SIN dirección y con las
// capacidades `metrics` y `exec` que ese tier sepa honrar: un Tier C queda sin `exec`, porque el
// alta se lo negaría. Sin dirección, un Tier B que llegue a actuar no sale a la red: EjecutarPorSSH
// corta con «no tiene dirección» antes de lanzar ningún proceso.
func maquinaDeTier(t *testing.T, s *McpServer, tier fleet.Tier, nombre string) fleet.Device {
	t.Helper()
	var caps []string
	for _, c := range []fleet.Cap{fleet.CapMetrics, fleet.CapExec} {
		if fleet.TierAdmite(tier, c) {
			caps = append(caps, string(c))
		}
	}
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": nombre, "tier": string(tier), "caps": caps, "project": "casa", "os": "linux",
	}); e != nil {
		t.Fatalf("alta de %q (tier %s): %+v — un tier declarado que el alta no sabe dar es un tier que ninguna tabla puede medir", nombre, tier, e)
	}
	d, hay, err := s.engine.DevicePorNombre("casa", nombre)
	if err != nil || !hay {
		t.Fatalf("la máquina %q no quedó en el registro: hay=%v err=%v", nombre, hay, err)
	}
	if d.Tier != tier {
		t.Fatalf("%q quedó con tier %q y se pidió %q", nombre, d.Tier, tier)
	}
	if d.Address != "" {
		t.Fatalf("%q tiene dirección %q: estas tablas no pueden tocar la red", nombre, d.Address)
	}
	return d
}

// umbralesPublicados lee de musubi_fleet_list el `umbral_segundos` de cada máquina de la casa: el
// umbral con el que el inventario decide su `online`, publicado al lado para que un operador lo vea.
// Las tablas de este archivo NO escriben el umbral: lo leen de acá, porque lo que custodian es que
// la política use ESE y no uno propio.
func umbralesPublicados(t *testing.T, s *McpServer) map[string]time.Duration {
	t.Helper()
	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatalf("fleet_list: %+v", e)
	}
	filas, _ := jsonOf(t, res)["devices"].([]any)
	out := map[string]time.Duration{}
	for _, x := range filas {
		fila, _ := x.(map[string]any)
		nombre, _ := fila["name"].(string)
		seg, ok := fila["umbral_segundos"].(float64)
		if !ok || seg <= 0 {
			t.Fatalf("la fila de %q no publica un `umbral_segundos` legible (%v): sin él la tabla no tiene contra qué umbral medir", nombre, fila["umbral_segundos"])
		}
		out[nombre] = time.Duration(seg) * time.Second
	}
	return out
}

// umbralDelTier da de alta una máquina TESTIGO del tier, sólo para leer el umbral publicado que
// comparten todas las filas de ese tier: de él se derivan los estados de la muestra y las edades
// del inventario, así que tiene que existir antes que ellos.
func umbralDelTier(t *testing.T, s *McpServer, tier fleet.Tier) time.Duration {
	t.Helper()
	testigo := maquinaDeTier(t, s, tier, "testigo-del-umbral")
	u := umbralesPublicados(t, s)[testigo.Name]
	if u <= 2*time.Second {
		t.Fatalf("el tier %s publica un umbral de %s: con eso u−1s y u+1s no separan nada", tier, u)
	}
	return u
}

// accionesDePolitica cuenta los comandos que una política dejó en la bitácora de UNA máquina: los
// de origen `politica` con ese argv entero. El aviso al usuario (`musubi:avisar`) queda afuera por
// el argv. La fila se escribe cuando la política DECIDE actuar, antes de saber si el canal llegó:
// es la decisión, no el éxito del transporte.
func accionesDePolitica(t *testing.T, s *McpServer, d fleet.Device, argv []string) int {
	t.Helper()
	filas, err := s.engine.BitacoraDeComandos(d.ProjectID, d.ID, 50)
	if err != nil {
		t.Fatalf("bitácora de %q: %v", d.Name, err)
	}
	n := 0
	for _, c := range filas {
		if c.Origen == fleet.OrigenPolitica && reflect.DeepEqual(c.Argv, argv) {
			n++
		}
	}
	return n
}

// intentosDePolitica cuenta las veces que el barrido DECIDIÓ actuar con una política: los
// resultados `ok` y `error` que cuenta actuarSiCorresponde después de pasar todas las compuertas.
//
// NO ES EL VALOR QUE DEVUELVE aplicarPoliticas, y la diferencia es el punto (observación de la
// revisión de T2). aplicarPoliticas cuenta las acciones cuyo CANAL respondió bien: en un Tier B sin
// dirección —el que estas tablas usan para no tocar la red— eso depende de que correrPorSSH trague el
// «no tiene dirección» y devuelva nil. El día que lo devuelva como error, que es lo correcto, las
// tablas de frescura caerían con «0 acciones» y se leerían como un fallo de frescura. La frescura la
// mide la DECISIÓN; el transporte es de otra guarda.
func intentosDePolitica(s *McpServer, politica string) int64 {
	return contarPoliticaOK(s, politica, "ok") + contarPoliticaOK(s, politica, "error")
}

// estadoDeLaMuestra es lo que el REGISTRO sabe de la muestra de una máquina en el momento del
// barrido. Es LA ÚNICA FUENTE DE ESTADOS DE LA MUESTRA de este archivo: la recorren la tabla de host,
// que decide con ella, y la de servicio, que no tiene que mirarla. Hasta el pulido de T2 la de
// servicio tenía su propia lista escrita a mano, sin la muestra del futuro que la de host sí
// recorría, y un freno por esa propiedad copiado al camino de servicio dejaba las dos en verde.
//
// Son tres horas, y el registro guarda las tres por separado:
//   - `latido`: la edad del último `last_seen` guardado.
//   - `llegada`: la edad de la llegada de la muestra, con el reloj del cerebro. La muestra se escribe
//     con su latido y DESPUÉS se estampa `latido`: si la llegada es más vieja, los latidos siguientes
//     vinieron sin muestra (el colector murió); si es más nueva, el registro retrocedió
//     (correrPorSSH estampa la hora en que EMPEZÓ el comando, y la sonda pudo guardar una muestra
//     mientras tanto).
//   - `adelanto`: cuánto después de su llegada dice haber sido tomada, según el reloj del AGENTE.
//     Positivo es un reloj adelantado; negativo, una muestra que tardó en llegar o un reloj atrasado.
//
// Todas las edades se miden desde el `ahora` del barrido, y una negativa es del futuro.
type estadoDeLaMuestra struct {
	nombre                    string
	nuncaLatio, sinMuestra    bool
	latido, llegada, adelanto time.Duration
}

// edadSegunElAgente es `ahora − tomada` con la fecha que puso el agente.
func (e estadoDeLaMuestra) edadSegunElAgente() time.Duration { return e.llegada - e.adelanto }

// edadSegunElCerebro es el HECHO ESCRITO de la decisión del usuario, sin mirar el código: una
// muestra fechada después de su llegada vale como tomada cuando llegó; una fechada antes, con su
// fecha (más vieja es el lado prudente).
func (e estadoDeLaMuestra) edadSegunElCerebro() time.Duration {
	if e.adelanto > 0 {
		return e.llegada
	}
	return e.edadSegunElAgente()
}

// caida, rancia y fresca son las tres condiciones de la guarda de host, dichas con los hechos de
// la fila: I2 («en línea» por tier) e I13 (no se actúa sobre una muestra rancia).
func (e estadoDeLaMuestra) caida(u time.Duration) bool { return e.nuncaLatio || e.latido > u }
func (e estadoDeLaMuestra) rancia(u time.Duration) bool {
	return !e.sinMuestra && e.edadSegunElCerebro() > u
}
func (e estadoDeLaMuestra) fresca(u time.Duration) bool {
	return !e.sinMuestra && !e.caida(u) && !e.rancia(u)
}

// describir cuenta la fila con sus tres horas, para los mensajes: cada uno dice los hechos de la
// fila, no adivina el código.
func (e estadoDeLaMuestra) describir() string {
	switch {
	case e.nuncaLatio:
		return "nunca latió"
	case e.sinMuestra:
		return fmt.Sprintf("latido %s, sin muestra", cuandoFue(e.latido))
	}
	return fmt.Sprintf("latido %s; la muestra llegó %s y el agente la fechó %s: según el reloj del cerebro fue tomada %s",
		cuandoFue(e.latido), cuandoFue(e.llegada), cuandoFue(e.edadSegunElAgente()), cuandoFue(e.edadSegunElCerebro()))
}

// preparar deja en el registro el estado de la fila y lo RELEE: si lo guardado no representara el
// caso —un latido corrido por el truncado, una muestra perdida—, la fila mediría otra cosa y su
// veredicto no diría nada. Devuelve la máquina releída.
//
// La `tomada` guardada se acepta con la fecha del agente O recortada a su llegada, y a propósito:
// cuál de las dos guarda el registro es lo que la tabla MIDE por la decisión de la política, no algo
// que la fila tenga que dar por hecho. Si esta relectura exigiera la recortada, volver a creerle a
// la fecha del agente caería acá, con un motivo de «la fila no representa su caso», y no en la
// decisión que es lo que importa.
func (e estadoDeLaMuestra) preparar(t *testing.T, s *McpServer, d fleet.Device, ahora time.Time) fleet.Device {
	t.Helper()
	switch {
	case e.nuncaLatio:
	case e.sinMuestra:
		if _, err := s.engine.LatirDevice(d.ID, ahora.Add(-e.latido), ""); err != nil {
			t.Fatalf("latir sin muestra: %v", err)
		}
	default:
		latir(t, s, d.ID, muestraSana(95, ahora.Add(-e.edadSegunElAgente())), ahora.Add(-e.llegada))
		if e.latido != e.llegada {
			if _, err := s.engine.LatirDevice(d.ID, ahora.Add(-e.latido), ""); err != nil {
				t.Fatalf("latir después de la muestra: %v", err)
			}
		}
	}
	leida, _, err := s.engine.DevicePorNombre("casa", d.Name)
	if err != nil {
		t.Fatal(err)
	}
	if e.nuncaLatio != leida.LastSeen.IsZero() {
		t.Fatalf("%s: la fila dice nunca-latió=%v y el registro tiene last_seen=%v", d.Name, e.nuncaLatio, leida.LastSeen)
	}
	if got := ahora.Sub(leida.LastSeen); !e.nuncaLatio && got != e.latido {
		t.Fatalf("%s: el registro guardó el latido de hace %s y la fila lo puso de hace %s: no representa su caso", d.Name, got, e.latido)
	}
	switch {
	case e.sinMuestra && leida.UltimaMuestra != nil:
		t.Fatalf("%s: la fila no mandó muestra y el registro tiene una: no representa su caso", d.Name)
	case !e.sinMuestra && leida.UltimaMuestra == nil:
		t.Fatalf("%s: la fila mandó una muestra (%s) y el registro no la guardó: no representa su caso", d.Name, e.describir())
	}
	if !e.sinMuestra {
		if got := ahora.Sub(leida.UltimaMuestra.Tomada); got != e.edadSegunElAgente() && got != e.edadSegunElCerebro() {
			t.Fatalf("%s: la muestra quedó de hace %s, y la fila la mandó fechada de hace %s para que llegara hace %s: no representa su caso",
				d.Name, got, e.edadSegunElAgente(), e.llegada)
		}
		if v, dispara := politicaDeMemoria2().Dispara(leida.UltimaMuestra); !dispara {
			t.Fatalf("%s: la condición no se cumple (mem=%v): la fila mediría la condición y no la frescura", d.Name, v)
		}
	}
	return leida
}

// estadosDeLaMuestra es el generador: TODOS los estados de la muestra que recorren las dos tablas,
// derivados del umbral `u` de la máquina, más un PISO que exige que la lista siga teniendo un estado
// por cada condición de la guarda de host, SOLA, y los que decide el recorte al reloj del cerebro.
//
// EL ORDEN ES DE LA MÁS VIEJA A LA MÁS NUEVA, Y NO ES ESTÉTICO: la primera fila que cae es el MOTIVO
// con el que el arnés compara los sabotajes entre sí, y en este orden cada mutación cae primero en
// una fila distinta (P1-m7 y P1-m8 en latido y muestra pasados, con tiers distintos; P1-m9 en
// latido casi con la muestra pasada; creerle al agente en la muestra que llegó pasada y fechada
// adelante; P1-m6 en el registro que retrocedió).
func estadosDeLaMuestra(t *testing.T, u time.Duration) []estadoDeLaMuestra {
	t.Helper()
	type edad struct {
		nombre string
		d      time.Duration
	}
	pasada, casi, cero := edad{"u+1s", u + time.Second}, edad{"u-1s", u - time.Second}, edad{"0", 0}
	// Un reloj de agente adelantado un segundo, un umbral entero y más allá.
	adelantos := []edad{{"1s", time.Second}, {"u", u}, {"2u", 2 * u}}

	estados := []estadoDeLaMuestra{
		{nombre: "nunca-latio", nuncaLatio: true, sinMuestra: true},
		{nombre: "late-sin-muestra", sinMuestra: true},
	}
	for _, l := range []edad{pasada, casi, cero} {
		// La muestra llegó CON este latido, y el agente la fechó hace u+1s, u−1s o ahora. Con un
		// latido pasado y una fecha más nueva, el agente la fechó después de llegar.
		for _, m := range []edad{pasada, casi, cero} {
			estados = append(estados, estadoDeLaMuestra{nombre: "latido-" + l.nombre + "_muestra-" + m.nombre,
				latido: l.d, llegada: l.d, adelanto: l.d - m.d})
		}
		// Llegó con este latido fechada ADELANTE: el reloj adelantado sigue funcionando si la
		// muestra acaba de llegar, y no la salva si llegó con un latido pasado.
		for _, f := range adelantos {
			estados = append(estados, estadoDeLaMuestra{nombre: "latido-" + l.nombre + "_adelantada-" + f.nombre,
				latido: l.d, llegada: l.d, adelanto: f.d})
		}
		// El colector murió CON EL RELOJ ADELANTADO: la muestra llegó hace u+1s fechada adelante y
		// los latidos siguientes vinieron sin muestra. Es el caso que decide el recorte: según el
		// agente es reciente, según el cerebro llegó hace más que el umbral.
		if l.d < pasada.d {
			for _, f := range adelantos {
				estados = append(estados, estadoDeLaMuestra{nombre: "latido-" + l.nombre + "_llego-u+1s_adelantada-" + f.nombre,
					latido: l.d, llegada: pasada.d, adelanto: f.d})
			}
		}
	}
	// EL REGISTRO RETROCEDIÓ: una muestra reciente, y DESPUÉS un latido estampado con una hora
	// vieja. Con la muestra recortada a su llegada es lo único que separa la mitad EnLinea de la
	// guarda de la de la edad (P1-m6).
	for _, a := range []edad{casi, cero} {
		estados = append(estados, estadoDeLaMuestra{nombre: "latido-u+1s_llego-despues-" + a.nombre,
			latido: pasada.d, llegada: a.d})
	}
	// EL RELOJ DEL CEREBRO RETROCEDIÓ después de guardarla (un paso de NTP): latido y llegada quedaron
	// un umbral por delante del `ahora` del barrido. Es la única forma de que una muestra ya recortada
	// quede guardada con una fecha posterior a `ahora`, y la que un freno por «la muestra del futuro»
	// mira.
	estados = append(estados, estadoDeLaMuestra{nombre: "reloj-del-cerebro-atras-u", latido: -u, llegada: -u})

	// PISO: cada condición de la guarda de host rota SOLA, la sana, y los estados del recorte.
	requisitos := []struct {
		que string
		es  func(e estadoDeLaMuestra) bool
	}{
		{"late sin muestra", func(e estadoDeLaMuestra) bool { return e.sinMuestra && !e.caida(u) }},
		{"figura caída con una muestra reciente (el registro retrocedió)",
			func(e estadoDeLaMuestra) bool { return !e.sinMuestra && e.caida(u) && !e.rancia(u) }},
		{"late con el colector muerto y la fecha honesta",
			func(e estadoDeLaMuestra) bool { return !e.caida(u) && e.rancia(u) && e.adelanto <= 0 }},
		{"late con una muestra vieja fechada adelante hasta parecer reciente (la decide el recorte)",
			func(e estadoDeLaMuestra) bool { return !e.caida(u) && e.rancia(u) && e.edadSegunElAgente() <= u }},
		{"late con una muestra recién llegada fechada más de un umbral adelante (el reloj adelantado sigue funcionando)",
			func(e estadoDeLaMuestra) bool { return e.fresca(u) && e.edadSegunElAgente() < -u }},
		{"sana, sin adelanto", func(e estadoDeLaMuestra) bool { return e.fresca(u) && e.adelanto == 0 }},
		{"guardada con una fecha posterior al `ahora` del barrido",
			func(e estadoDeLaMuestra) bool { return !e.sinMuestra && e.edadSegunElCerebro() < 0 }},
	}
	for _, r := range requisitos {
		hay := false
		for _, e := range estados {
			hay = hay || r.es(e)
		}
		if !hay {
			t.Fatalf("PISO: ningún estado de la muestra cubre «%s» (u=%s): las tablas dejaron de recorrerlo", r.que, u)
		}
	}
	return estados
}

// A131 · T2 — UNA POLÍTICA DE HOST ACTÚA SÓLO SI LA MÁQUINA LATE Y SU MUESTRA ES RECIENTE, LAS DOS
// COSAS CONTRA EL UMBRAL QUE EL INVENTARIO PUBLICA PARA ESA MÁQUINA Y CON EL RELOJ DEL CEREBRO.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ EJES CLAVABAN LAS GUARDAS DE ANTES
//
// TestUnaPoliticaNoActuaSobreUnaMuestraRancia y TestUnaMaquinaQueLateSinMedirNoDisparaPoliticas
// dicen lo correcto —«el umbral es el MISMO que decide "en línea", y por tier»— y lo miden en UN
// punto de cada eje, así que cuatro mutaciones de A131 dejaban el paquete entero en verde:
//
//   - el TIER: las dos enrolan un Tier A, el único cuyo umbral no sale del sondeo, y ninguna
//     prueba de políticas enrolaba otro. Que los tiers sin agente subieran al tope de una hora
//     (P1-m8) no se veía.
//   - la MAGNITUD: una muestra de 30 min, veinte veces los 90 s de un Tier A, satisface a cualquier
//     umbral menor. El de sondeo (15 min) para todos los tiers (P1-m7), o sólo en el chequeo de la
//     edad (P1-m9), pasaban.
//   - las HORAS, iguales: `latir(t, s, id, muestraSana(95, hace30), hace30)` pone el latido, la
//     llegada y la `tomada` en el mismo instante, así que cualquiera de las dos mitades de la guarda
//     la sostiene sola, y sacar la mitad `EnLinea` (P1-m6) pasaba.
//
// LA MITAD EnLinea DESPUÉS DEL RECORTE. Con la muestra recortada a su llegada, una muestra reciente
// implica un latido reciente —llegan en la misma sentencia—, salvo que el registro RETROCEDA:
// correrPorSSH estampa `last_seen` con la hora en que EMPEZÓ el comando (hasta 10 min antes), y la
// sonda pudo guardar una muestra más nueva mientras tanto. Ahí la máquina figura caída con una
// muestra de hace un rato, y es la fila que pone roja a P1-m6.
//
// Exposición medida (auditoría A131): 0 pares expuestos. La única política de host es
// `vaciar-journal` (disco libre < 10 %) sobre musubi-server, un Tier A, y su disco libre no bajó del
// 23,55 % en 30 días. Los estados sí existen: en 30 días `tomada` llegó hasta 5,8 s después del
// `last_seen` guardado (altura-db) y hasta 2 s adelante del reloj del cerebro (gio, davantis-1); la
// muestra de musubi-server pasó los 90 s sin llegar a 15 min en 11 de 39.278 pasos de un minuto
// (ninguno mientras latía: 88,8 s de máximo en línea), la de davantis-1 pasó los 90 s estando en
// línea 26 min en 90 días, y la de altura-db (Tier B) pasó sus 15 min sin llegar a la hora en 5 de
// 36.036. La del recorte es DERIVADA de esos números, no medida aparte: con un adelanto de hasta
// 2 s, creerle al agente estiraba la frescura hasta 2 s más allá del umbral.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE
//
// Una subprueba por cada Tier declarado en internal/fleet (go/types) y por cada intervalo de
// sondeosDeLasTablasDeFrescura. El umbral `u` NO se escribe acá: se lee de musubi_fleet_list, el
// `umbral_segundos` que el inventario publica al lado de `online`, y cada fila tiene que publicar
// el mismo. Una máquina por cada estado de estadosDeLaMuestra; todas con la RAM al 95 % y todas
// juzgadas en UN barrido de aplicarPoliticas. El hecho contra el que se compara está escrito y no
// copiado del código: actúa ⇔ el tier admite `exec` ∧ hay muestra ∧ latido ≤ u ∧ la muestra tiene
// ≤ u según el reloj del CEREBRO (su fecha, recortada a su llegada).
//
// Un tier que no admite `exec` (hoy el C) no actúa nunca, y sus filas lo exigen igual: el día que
// lo admita, pasan a medir la frescura sin tocar esta prueba. PISO: cada fila se relee del registro
// antes del barrido para confirmar que representa su caso; en cada tier que admite `exec` hay
// máquinas que actúan y máquinas que no; y los tiers no publican todos el mismo umbral, porque sin
// eso la tabla no distingue la fórmula de un tier de la de otro. El recuento compara las DECISIONES
// (intentosDePolitica), no el éxito del canal.
//
// Cada sabotaje de abajo declara también un ARREGLO: un cambio correcto y equivalente que la tabla
// no puede castigar. Lo que mide es el umbral y las horas, no la forma en que están escritos.
//
// Sabotaje: sacar la mitad `EnLinea` de la guarda de host (P1-m6): una máquina que figura caída
// porque el registro retrocedió, con una muestra de hace un rato, dispara. El arreglo pregunta lo
// mismo sin EnLinea.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="if d.UltimaMuestra == nil || !d.EnLinea(ahora, umbral) {"
// arnes: a="if d.UltimaMuestra == nil {"
// arnes: arreglo_de="if d.UltimaMuestra == nil || !d.EnLinea(ahora, umbral) {"
// arnes: arreglo_a="if d.UltimaMuestra == nil || d.Revoked || d.LastSeen.IsZero() || ahora.Sub(d.LastSeen) > umbral {"
//
// Sabotaje: el umbral de la política sale de la fórmula de sondeo para todos los tiers (P1-m7): un
// Tier A callado hace 91 s dispara. El arreglo lo deriva por el otro camino correcto.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tumbral := s.umbralEnLinea(d)\n"
// arnes: a="\tumbral := s.umbralEnLinea(d)\n\tumbral = umbralEnLineaPara(fleet.Device{}, s.sondaIntervalo)\n"
// arnes: arreglo_de="\tumbral := s.umbralEnLinea(d)\n"
// arnes: arreglo_a="\tumbral := umbralEnLineaPara(d, s.sondaIntervalo)\n"
//
// Sabotaje: los tiers sin agente suben al tope de una hora (P1-m8): un Tier B con la muestra
// pasada de sus tres sondeos dispara. El arreglo le pasa a la fórmula sólo el tier, que es lo único
// que lee.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tumbral := s.umbralEnLinea(d)\n"
// arnes: a="\tumbral := s.umbralEnLinea(d)\n\tif d.Tier != fleet.TierAgente {\n\t\tumbral = umbralEnLineaTope\n\t}\n"
// arnes: arreglo_de="\tumbral := s.umbralEnLinea(d)\n"
// arnes: arreglo_a="\tumbral := umbralEnLineaPara(fleet.Device{Tier: d.Tier}, s.sondaIntervalo)\n"
//
// Sabotaje: sólo el chequeo de la EDAD usa la fórmula de sondeo (P1-m9): un Tier A que late con el
// colector muerto hace 91 s dispara. El arreglo escribe la misma comparación al revés.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\t\treturn false\n\t}\n\tif ahora.Sub(d.UltimaMuestra.Tomada) > umbral {\n"
// arnes: a="\t\treturn false\n\t}\n\tumbral = umbralEnLineaPara(fleet.Device{}, s.sondaIntervalo)\n\tif ahora.Sub(d.UltimaMuestra.Tomada) > umbral {\n"
// arnes: arreglo_de="\tif ahora.Sub(d.UltimaMuestra.Tomada) > umbral {\n"
// arnes: arreglo_a="\tif d.UltimaMuestra.Tomada.Before(ahora.Add(-umbral)) {\n"
//
// Sabotaje: volver a creerle a la `tomada` del agente (el recorte al reloj del cerebro, decisión del
// usuario en el pulido de T2): la muestra que llegó hace u+1s fechada adelante —un colector muerto
// con el reloj corrido— dispara. El arreglo escribe el mismo recorte sin la función.
// arnes: archivo="internal/memory/devices.go"
// arnes: de="\tmuestra = muestraAlRelojDelCerebro(muestra, ahora)\n"
// arnes: a=""
// arnes: arreglo_de="\tmuestra = muestraAlRelojDelCerebro(muestra, ahora)\n"
// arnes: arreglo_a="\tif m, err := fleet.MuestraDesdeTexto(muestra); err == nil && m != nil && m.Tomada.After(ahora) {\n\t\tm.Tomada = ahora.UTC()\n\t\tif texto, err := m.Serializar(); err == nil {\n\t\t\tmuestra = texto\n\t\t}\n\t}\n"
//
// Sabotaje: un tier declarado con la forma de CONVERSIÓN, `TierD = Tier("D")`: la tabla tiene que
// recorrerlo y caer, porque el alta no lo sabe dar. La lectura por la forma `X Tier = "…"` lo
// salteaba. El arreglo reescribe un tier existente con esa misma forma, y la tabla tiene que seguir
// recorriéndolo.
// arnes: archivo="internal/fleet/device.go"
// arnes: de="\tTierMovil Tier = \"C\"\n"
// arnes: a="\tTierMovil Tier = \"C\"\n\tTierD = Tier(\"D\")\n"
// arnes: arreglo_de="\tTierMovil Tier = \"C\"\n"
// arnes: arreglo_a="\tTierMovil = Tier(\"C\")\n"
func TestUnaPoliticaDeHostActuaSoloConLatidoYMuestraDentroDelUmbralDeSuTier(t *testing.T) {
	pol := politicaDeMemoria() // mem_pct > 90, sobre todas, journalctl
	umbralPorSondeo := map[float64]map[fleet.Tier]time.Duration{}

	for _, tier := range tiersDeclarados(t) {
		for _, pm := range sondeosDeLasTablasDeFrescura {
			t.Run(fmt.Sprintf("tier=%s/probe_minutes=%v", tier, pm), func(t *testing.T) {
				type fila struct {
					d fleet.Device
					e estadoDeLaMuestra
				}
				s := newTestServer(t, embedding.NoopProvider{})
				if err := s.ConfigurarFlota(config.FleetConfig{ProbeMinutes: pm, Policies: []config.PolicyConfig{pol}}); err != nil {
					t.Fatalf("ConfigurarFlota: %v", err)
				}
				s.buscarPrincipal = registroDePrueba(autoHeal())
				u := umbralDelTier(t, s, tier)
				if umbralPorSondeo[pm] == nil {
					umbralPorSondeo[pm] = map[fleet.Tier]time.Duration{}
				}
				umbralPorSondeo[pm][tier] = u
				var filas []fila
				for _, e := range estadosDeLaMuestra(t, u) {
					filas = append(filas, fila{d: maquinaDeTier(t, s, tier, e.nombre), e: e})
				}
				publicados := umbralesPublicados(t, s)
				for _, f := range filas {
					if publicados[f.d.Name] != u {
						t.Fatalf("%s publica un umbral de %s y el testigo de su tier %s: la fila no se mediría contra el suyo", f.d.Name, publicados[f.d.Name], u)
					}
				}
				puedeActuar := fleet.TierAdmite(tier, fleet.CapExec)

				// Al segundo: `last_seen` se guarda en RFC3339, y un `ahora` con fracción correría
				// cada latido hasta un segundo hacia atrás — justo el margen de u−1s.
				ahora := time.Now().UTC().Truncate(time.Second)
				esperadas := 0
				for i, f := range filas {
					filas[i].d = f.e.preparar(t, s, f.d, ahora)
					if puedeActuar && f.e.fresca(u) {
						esperadas++
					}
				}
				if puedeActuar && (esperadas == 0 || esperadas == len(filas)) {
					t.Fatalf("PISO: el tier %s admite `exec` y la tabla espera %d acción(es) de %d: sin filas de los dos lados no distingue nada", tier, esperadas, len(filas))
				}

				s.aplicarPoliticas("casa", ahora)

				for _, f := range filas {
					debe := puedeActuar && f.e.fresca(u)
					actuo := accionesDePolitica(t, s, f.d, pol.Run)
					switch {
					case actuo > 1:
						t.Errorf("%s: la política encoló %d comandos en UN barrido", f.d.Name, actuo)
					case debe && actuo == 0:
						t.Errorf("%s (tier %s, umbral publicado %s): %s; las dos cosas dentro del umbral, y la política NO actuó. "+
							"La guarda de frescura es más estricta que el umbral que decide «en línea», o le cree la fecha al "+
							"AGENTE para rechazar la muestra en vez de recortarla a su llegada: la política se apaga sola sobre "+
							"una máquina sana mientras el inventario la muestra `online`",
							f.d.Name, tier, u, f.e.describir())
					case !debe && actuo > 0:
						t.Errorf("%s (tier %s, umbral publicado %s): %s", f.d.Name, tier, u, porQueNoDebiaActuar(puedeActuar, f.e, u))
					}
				}
				if got := intentosDePolitica(s, pol.Name); got != int64(esperadas) {
					t.Errorf("el barrido decidió actuar %d vez/veces (resultados ok+error) y la tabla espera %d", got, esperadas)
				}
			})
		}
	}

	// PISO: los tiers no publican todos el mismo umbral, y alguno se mueve con el sondeo. Sin las dos
	// cosas, un umbral calculado con la fórmula de otro tier coincidiría con el bueno en todas las
	// filas, y P1-m7 y P1-m8 volverían a pasar.
	distintos, seMueve := false, false
	for _, porTier := range umbralPorSondeo {
		var primero time.Duration
		for _, u := range porTier {
			switch {
			case primero == 0:
				primero = u
			case u != primero:
				distintos = true
			}
		}
	}
	unos, otros := umbralPorSondeo[sondeosDeLasTablasDeFrescura[0]], umbralPorSondeo[sondeosDeLasTablasDeFrescura[1]]
	for tier, u := range unos {
		if v, hay := otros[tier]; hay && v != u {
			seMueve = true
		}
	}
	if !distintos || !seMueve {
		t.Errorf("PISO: los umbrales publicados no separan los tiers (distintos=%v) o ninguno se mueve con el sondeo (se mueve=%v): %v. "+
			"Con eso esta tabla no distingue el umbral de un tier del de otro", distintos, seMueve, umbralPorSondeo)
	}
}

// porQueNoDebiaActuar dice, para una fila donde la política de host actuó y no debía, cuál de las
// frases de I13 rompió. Lo decide con los hechos de la FILA, no adivinando el código.
func porQueNoDebiaActuar(puedeActuar bool, e estadoDeLaMuestra, u time.Duration) string {
	switch {
	case !puedeActuar:
		return "el tier no admite `exec` y la política actuó igual: la compuerta del aparato no frenó"
	case e.sinMuestra:
		return "late sin muestra y la política actuó: no hay dato sobre el cual decidir, y decidir sin dato es inventarlo"
	case e.caida(u) && e.llegada < e.latido:
		return fmt.Sprintf("la máquina figura CAÍDA —su último latido guardado fue %s— y la política actuó sobre una muestra "+
			"que llegó DESPUÉS, %s. El registro retrocedió (correrPorSSH estampa la hora en que EMPEZÓ el comando): "+
			"la muestra de una máquina que figura caída no decide nada, por reciente que sea", cuandoFue(e.latido), cuandoFue(e.llegada))
	case e.caida(u):
		return fmt.Sprintf("la máquina figura CAÍDA —%s— y la política actuó: la muestra de una máquina caída es rancia "+
			"por definición, diga lo que diga su fecha", e.describir())
	case e.adelanto > 0 && e.edadSegunElAgente() <= u:
		return fmt.Sprintf("%s, y la política actuó: con la fecha del AGENTE la muestra estaría dentro del umbral, con el reloj "+
			"del CEREBRO no. La frescura la mide el cerebro —una muestra fechada después de llegar vale como tomada al "+
			"llegar—, y un reloj adelantado no puede rejuvenecer la última muestra de un colector muerto", e.describir())
	default:
		return fmt.Sprintf("la máquina late pero su muestra es vieja —%s—: el colector murió y la política actúa sobre el último "+
			"dato bueno, que siendo el último siempre cruza la condición", e.describir())
	}
}

// cuandoFue escribe una edad para un mensaje: «hace 1m31s», o «1m30s en el futuro del reloj del
// cerebro». Un «hace -1m30s» se lee como un error de la prueba.
func cuandoFue(d time.Duration) string {
	if d < 0 {
		return fmt.Sprintf("%s en el futuro del reloj del cerebro", -d)
	}
	return fmt.Sprintf("hace %s", d)
}

// A131 · T2 — UNA POLÍTICA DE SERVICIO DECIDE POR SU INVENTARIO, Y LA MUESTRA DEL HOST NO LE CAMBIA
// NADA.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ EJES CLAVABA LA GUARDA DE ANTES
//
// TestUnaPoliticaDeServicioActuaSinTelemetriaDelHost dice tres cosas y mide cada una en un punto,
// así que tres mutaciones de A131 dejaban el paquete entero en verde:
//
//   - «una máquina que nunca reportó telemetría, O CUYO COLECTOR MURIÓ», y arma sólo la primera. Una
//     guarda de la muestra copiada en el camino de servicio que frenara únicamente cuando la muestra
//     EXISTE y está rancia (P4-m4) pasaba.
//   - llama a evaluarPolitica A MANO, por debajo del bucle del barrido. Un filtro agregado en el
//     bucle de aplicarPoliticas, antes de la bifurcación por clase (P4-m5: saltear las máquinas sin
//     muestra), no le llegaba.
//   - «el inventario viejo la frena con el umbral del INVENTARIO, no se hereda del host», y prueba
//     0 s y 3×UmbralInventario: dos edades que el umbral del host y el del inventario clasifican
//     igual. Medir la frescura del inventario con el umbral de «en línea» del host (P4-m3, 90 s en un
//     Tier A) pasaba, y dejaba las políticas de servicio inertes salvo en los 90 s que siguen a cada
//     reenvío de 5 min.
//
// Y la primera versión de ESTA tabla tenía su propia lista de cinco estados de la muestra, escrita
// a mano: sin la muestra del futuro que la tabla de host sí recorría, un freno por esa propiedad
// copiado al camino de servicio dejaba las dos en verde (revisión de T2). Ahora el eje es
// estadosDeLaMuestra, el mismo generador de la tabla de host: una propiedad nueva de la muestra
// entra a las dos tablas a la vez.
//
// Exposición medida (auditoría A131): 0. No hay ninguna política de servicio configurada —la única
// es `vaciar-journal`, de host— y las 4 máquinas tienen muestra. Los estados que la volverían viva
// existen: 125 servicios inventariados en 3 máquinas, y el inventario de davantis-1 estuvo más viejo
// que 90 s el 88,1 % del último día y más viejo que 10 min el 69,0 %: un 19 % del día en el que una
// política de servicio actuaría con el umbral bueno y no con el del host. P4-m4 no se midió (el SSH
// de lectura al servidor fue denegado).
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE
//
// Una subprueba por cada Tier (go/types) y por cada intervalo de sondeosDeLasTablasDeFrescura. Una
// máquina por cada combinación de
//
//   - un estado de estadosDeLaMuestra —el generador de la tabla de host, con un estado por cada
//     condición de su guarda rota sola, los del recorte al reloj del cerebro y la muestra guardada
//     con fecha posterior al barrido—, releído del registro antes del barrido.
//   - una edad del INVENTARIO derivada de los dos umbrales: 0; UmbralInventario ± 30 s; y, cuando
//     el umbral de «en línea» del host es menor que el del inventario (un Tier A), el punto medio
//     entre los dos: fresco para el inventario y viejo para el host. Donde el del host es mayor (un
//     Tier B con sondeo de 5 min: 15 min), UmbralInventario + 30 s es el punto al revés.
//
// nginx caído en todas, y todo entra por aplicarPoliticas. El hecho: actúa ⇔ el tier admite `exec`
// ∧ el inventario tiene a lo sumo UmbralInventario. Y el relacional, sobre el mismo servicio: actúa
// ⇔ el tier admite `exec` ∧ musubi_fleet_services dice `fresco: true`. La columna que mira el
// operador y la decisión del barrido no pueden hablar de dos frescuras; los ±30 s son porque la tool
// lee su propio reloj. PISO: la tool devuelve la fila de cada máquina, y en cada tier que admite
// `exec` hay máquinas que actúan, máquinas que no, y al menos una edad donde el umbral del host y el
// del inventario dicen cosas distintas — sin ella la tabla no distingue cuál de los dos se usa. El
// recuento compara las DECISIONES (intentosDePolitica), no el éxito del canal.
//
// Cada sabotaje de abajo declara también un ARREGLO: un cambio correcto y equivalente que la tabla
// no puede castigar. Lo que mide es qué frena a la política, no la forma en que está escrito.
//
// Sabotaje: una guarda de la muestra copiada en el camino de servicio, que frena cuando la muestra
// existe y está rancia (P4-m4). El arreglo frena por la máquina revocada, que sí le corresponde (y
// que evaluarPolitica ya frenaba antes de bifurcar).
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="func (s *McpServer) evaluarPoliticaDeServicio(pol fleet.Politica, d fleet.Device, ahora time.Time) bool {\n"
// arnes: a="func (s *McpServer) evaluarPoliticaDeServicio(pol fleet.Politica, d fleet.Device, ahora time.Time) bool {\n\tif d.UltimaMuestra != nil && ahora.Sub(d.UltimaMuestra.Tomada) > umbralEnLineaPara(d, s.sondaIntervalo) {\n\t\treturn false\n\t}\n"
// arnes: arreglo_de="func (s *McpServer) evaluarPoliticaDeServicio(pol fleet.Politica, d fleet.Device, ahora time.Time) bool {\n"
// arnes: arreglo_a="func (s *McpServer) evaluarPoliticaDeServicio(pol fleet.Politica, d fleet.Device, ahora time.Time) bool {\n\tif d.Revoked {\n\t\treturn false\n\t}\n"
//
// Sabotaje: un freno por la muestra DEL FUTURO copiado al camino de servicio (el que midió la
// revisión de T2 con la lista escrita a mano: exit 0). Con el generador único lo pone rojo la fila
// cuya muestra quedó guardada después del `ahora` del barrido. El arreglo frena por la máquina
// revocada.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="func (s *McpServer) evaluarPoliticaDeServicio(pol fleet.Politica, d fleet.Device, ahora time.Time) bool {\n"
// arnes: a="func (s *McpServer) evaluarPoliticaDeServicio(pol fleet.Politica, d fleet.Device, ahora time.Time) bool {\n\tif d.UltimaMuestra != nil && d.UltimaMuestra.Tomada.After(ahora.Add(5*time.Second)) {\n\t\treturn false\n\t}\n"
// arnes: arreglo_de="func (s *McpServer) evaluarPoliticaDeServicio(pol fleet.Politica, d fleet.Device, ahora time.Time) bool {\n"
// arnes: arreglo_a="func (s *McpServer) evaluarPoliticaDeServicio(pol fleet.Politica, d fleet.Device, ahora time.Time) bool {\n\tif d.Revoked {\n\t\treturn false\n\t}\n"
//
// Sabotaje: el barrido saltea las máquinas sin muestra ANTES de bifurcar por clase (P4-m5). El
// ancla lleva el bucle de las políticas para no confundirse con el bucle que agrega el sabotaje de
// TestConLasVentanasIlegiblesElInventarioDiceLoQueHaceElBarrido. El arreglo saltea en el mismo
// lugar a las revocadas, que no son de la muestra y no actúan por ningún camino.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tfor _, pol := range s.politicas {\n\t\tfor _, d := range devices {\n"
// arnes: a="\tfor _, pol := range s.politicas {\n\t\tfor _, d := range devices {\n\t\t\tif d.UltimaMuestra == nil {\n\t\t\t\tcontinue\n\t\t\t}\n"
// arnes: arreglo_de="\tfor _, pol := range s.politicas {\n\t\tfor _, d := range devices {\n"
// arnes: arreglo_a="\tfor _, pol := range s.politicas {\n\t\tfor _, d := range devices {\n\t\t\tif d.Revoked {\n\t\t\t\tcontinue\n\t\t\t}\n"
//
// Sabotaje: la frescura del inventario sale del umbral de «en línea» del HOST (P4-m3, portada a la
// función única). El arreglo escribe la misma frescura sin la función: lo que se mide es el umbral,
// no el nombre.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="fresco := servicioFresco(sv, ahora)"
// arnes: a="fresco := sv.Fresco(ahora, umbralEnLineaPara(d, s.sondaIntervalo))"
// arnes: arreglo_de="fresco := servicioFresco(sv, ahora)"
// arnes: arreglo_a="fresco := sv.Fresco(ahora, fleet.UmbralInventario)"
//
// Sabotaje: la función única mide con el latido de un Tier A. La columna y la política se mueven
// juntas —el relacional queda verde— y la pone roja el hecho escrito. El arreglo escribe el mismo
// umbral por su definición.
// arnes: archivo="internal/mcp/methods_servicios.go"
// arnes: de="return sv.Fresco(ahora, fleet.UmbralInventario)"
// arnes: a="return sv.Fresco(ahora, umbralEnLineaDefault)"
// arnes: arreglo_de="return sv.Fresco(ahora, fleet.UmbralInventario)"
// arnes: arreglo_a="return sv.Fresco(ahora, 2*fleet.InventarioCada)"
//
// Sabotaje: un tier declarado en OTRO archivo del paquete fleet, no en device.go: la tabla tiene que
// recorrerlo y caer, porque el alta no lo sabe dar. La lectura de device.go sola lo salteaba. El
// arreglo declara en ese archivo otro nombre para un tier que ya existe, que no es un tier nuevo:
// la tabla no se duplica ni cae.
// arnes: archivo="internal/fleet/credencial_fuente.go"
// arnes: de="\tCredencialDeVariable = \"variable\"\n)\n"
// arnes: a="\tCredencialDeVariable = \"variable\"\n)\n\n// TierD es un tier declarado fuera de device.go.\nconst TierD Tier = \"D\"\n"
// arnes: arreglo_de="\tCredencialDeVariable = \"variable\"\n)\n"
// arnes: arreglo_a="\tCredencialDeVariable = \"variable\"\n)\n\n// tierMovilAlias nombra otra vez a un tier que ya existe.\nconst tierMovilAlias = TierMovil\n"
func TestUnaPoliticaDeServicioDecidePorSuInventarioYNoPorLaMuestraDelHost(t *testing.T) {
	const margen = 30 * time.Second
	edadesDelInventario := func(u time.Duration) []time.Duration {
		edades := []time.Duration{0, fleet.UmbralInventario - margen, fleet.UmbralInventario + margen}
		if medio := (u + fleet.UmbralInventario) / 2; u+margen < medio && medio+margen < fleet.UmbralInventario {
			edades = append(edades, medio)
		}
		return edades
	}
	pol := config.PolicyConfig{
		Name: "revivir-nginx", Principal: "curador", When: string(fleet.CondServicioCaido),
		Devices: []string{"*"}, Service: "nginx", Run: []string{"systemctl", "restart", "nginx"},
		CooldownMinutes: 10,
	}

	for _, tier := range tiersDeclarados(t) {
		for _, pm := range sondeosDeLasTablasDeFrescura {
			t.Run(fmt.Sprintf("tier=%s/probe_minutes=%v", tier, pm), func(t *testing.T) {
				type fila struct {
					d    fleet.Device
					e    estadoDeLaMuestra
					edad time.Duration
				}
				s := newTestServer(t, embedding.NoopProvider{})
				if err := s.ConfigurarFlota(config.FleetConfig{ProbeMinutes: pm, Policies: []config.PolicyConfig{pol}}); err != nil {
					t.Fatalf("ConfigurarFlota: %v", err)
				}
				s.buscarPrincipal = registroQuePermiteSystemctl()
				// El umbral del host se lee de una máquina del mismo tier, dada de alta sólo para eso:
				// todas las filas lo comparten, y de él se derivan los estados de la muestra y las
				// edades del inventario.
				u := umbralDelTier(t, s, tier)
				var filas []fila
				for _, e := range estadosDeLaMuestra(t, u) {
					for _, edad := range edadesDelInventario(u) {
						d := maquinaDeTier(t, s, tier, fmt.Sprintf("%s_inventario-%ds", e.nombre, int(edad.Seconds())))
						filas = append(filas, fila{d: d, e: e, edad: edad})
					}
				}
				puedeActuar := fleet.TierAdmite(tier, fleet.CapExec)

				ahora := time.Now().UTC().Truncate(time.Second)
				esperadas, discrepan := 0, 0
				for i, f := range filas {
					filas[i].d = f.e.preparar(t, s, f.d, ahora)
					reportado := ahora.Add(-f.edad)
					if _, _, err := s.engine.ReportarServicios(f.d.ID, reportado, []fleet.ReporteServicio{
						{Nombre: "nginx", Clase: "systemd", Salud: fleet.SaludServicio{Tomada: reportado, Estado: fleet.EstadoFallado}}}); err != nil {
						t.Fatalf("ReportarServicios: %v", err)
					}
					if puedeActuar && f.edad <= fleet.UmbralInventario {
						esperadas++
					}
					if (f.edad <= u) != (f.edad <= fleet.UmbralInventario) {
						discrepan++
					}
				}
				if puedeActuar && (esperadas == 0 || esperadas == len(filas)) {
					t.Fatalf("PISO: el tier %s admite `exec` y la tabla espera %d acción(es) de %d: sin filas de los dos lados no distingue nada", tier, esperadas, len(filas))
				}
				if discrepan == 0 {
					t.Fatalf("PISO: ninguna edad del inventario separa el umbral del host (%s) del del inventario (%s): la tabla no distingue cuál de los dos se usa", u, fleet.UmbralInventario)
				}

				s.aplicarPoliticas("casa", ahora)

				// La columna que mira el operador, leída por la misma tool que usa una persona.
				res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_services", map[string]any{})
				if e != nil {
					t.Fatalf("fleet_services: %+v", e)
				}
				columna := map[string]any{}
				crudas, _ := jsonOf(t, res)["services"].([]any)
				for _, x := range crudas {
					sv, _ := x.(map[string]any)
					if sv["nombre"] == "nginx" {
						nombre, _ := sv["device"].(string)
						columna[nombre] = sv["fresco"]
					}
				}

				for _, f := range filas {
					debe := puedeActuar && f.edad <= fleet.UmbralInventario
					actuo := accionesDePolitica(t, s, f.d, pol.Run)
					// El RELACIONAL primero: la columna que mira el operador y la decisión del barrido,
					// sobre el mismo servicio caído, dicen la misma frescura. Va antes que el hecho
					// porque la primera línea que cae es el MOTIVO con el que el arnés compara los
					// sabotajes: uno que aparta a la política de la columna cae acá, y uno que mueve
					// las dos puntas juntas pasa esto y cae en el hecho. Así no se leen como uno solo.
					if fresco, hay := columna[f.d.Name].(bool); !hay {
						t.Errorf("%s: musubi_fleet_services no trae la fila de nginx (%v): no hay columna contra la cual comparar", f.d.Name, columna[f.d.Name])
					} else if (actuo > 0) != (puedeActuar && fresco) {
						t.Errorf("%s (tier %s, muestra del host: %s): la política actuó=%v y la columna `fresco` que ve el operador "+
							"dice %v, sobre el mismo servicio caído. Quien mira el inventario ve un servicio caído con noticias "+
							"frescas y ningún auto-heal, o un auto-heal sobre noticias viejas: la decisión del barrido y la columna "+
							"tienen que decir lo mismo", f.d.Name, tier, f.e.nombre, actuo > 0, fresco)
					}
					// Y el HECHO. Cada mensaje dice los hechos de la fila, no adivina el código.
					switch {
					case actuo > 1:
						t.Errorf("%s: la política encoló %d comandos en UN barrido", f.d.Name, actuo)
					case debe && actuo == 0:
						t.Errorf("%s (tier %s, muestra del host: %s; umbral de «en línea» %s): nginx caído con el inventario de hace %s, "+
							"fresco para el umbral del INVENTARIO (%s), y la política NO actuó: el auto-heal de servicios queda inerte "+
							"justo donde sirve. Si a esta edad no actúa en NINGÚN estado de la muestra, el inventario se mide con otro "+
							"umbral; si sólo falla en algunos, la frena una guarda de la muestra del host o el barrido la saltea antes "+
							"de bifurcar por clase", f.d.Name, tier, f.e.describir(), u, f.edad, fleet.UmbralInventario)
					case !debe && actuo > 0 && !puedeActuar:
						t.Errorf("%s: el tier %s no admite `exec` y la política actuó igual", f.d.Name, tier)
					case !debe && actuo > 0:
						t.Errorf("%s (tier %s, umbral de «en línea» %s): el inventario es de hace %s, pasado el umbral del INVENTARIO (%s), "+
							"y la política actuó: su frescura no se mira, o se mide con un umbral más largo que el suyo, y una máquina "+
							"que dejó de enumerar sus servicios tiene políticas actuando sobre el último estado conocido",
							f.d.Name, tier, u, f.edad, fleet.UmbralInventario)
					}
				}
				if got := intentosDePolitica(s, pol.Name); got != int64(esperadas) {
					t.Errorf("el barrido decidió actuar %d vez/veces (resultados ok+error) y la tabla espera %d", got, esperadas)
				}
			})
		}
	}
}
