package mcp

// A131 · T2 — LA FRESCURA QUE FRENA A UNA POLÍTICA ES LA DE SU CLASE, MEDIDA CON EL UMBRAL QUE LE
// TOCA.
//
// Una política de HOST decide sobre la muestra, y la muestra sólo vale si la máquina LATE y el dato
// es RECIENTE, las dos cosas contra el mismo umbral que decide «en línea» para su tier (I2 · I13).
// Una política de SERVICIO decide sobre el inventario, con el umbral del inventario, y la muestra
// del host no le cambia nada (A44). Las guardas de antes decían exactamente eso y lo medían en un
// solo punto de cada eje; las de este archivo recorren los ejes enteros —los tiers leídos del
// código, las dos horas que mira la guarda de host, un estado de la muestra por cada condición de
// esa guarda— y entran por el barrido que corre en producción, aplicarPoliticas.
//
// Lo que NO miran, a propósito, es `puede_actuar`: la muestra fresca queda FUERA de ese indicador
// por decisión de A131·T1 (ver «LO QUE NO CONTESTA» en el doc de porQueNoActuaria). El indicador
// contesta «si la condición se cumpliera», y la frescura es parte de la condición.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strconv"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// tiersDeclarados devuelve las constantes de tipo Tier, leídas del AST de internal/fleet/device.go
// en el orden en que están escritas: el conjunto CERRADO de tiers que recorren las tablas de este
// archivo. Un tier nuevo entra solo a las dos tablas, y si el alta no lo sabe dar, la tabla se pone
// roja en vez de saltearlo.
func tiersDeclarados(t *testing.T) []fleet.Tier {
	t.Helper()
	const fuente = "../fleet/device.go"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, fuente, nil, 0)
	if err != nil {
		t.Fatalf("no se pudo parsear %s: %v", fuente, err)
	}
	var out []fleet.Tier
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
			if tipo, ok := vs.Type.(*ast.Ident); !ok || tipo.Name != "Tier" {
				continue
			}
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("%s: una constante de Tier no es un literal de cadena y esta guarda no la puede leer", fset.Position(v.Pos()))
				}
				valor, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("%s: %v", fset.Position(v.Pos()), err)
				}
				out = append(out, fleet.Tier(valor))
			}
		}
	}
	// PISO: hoy son tres (A, B, C). Menos de dos es que el tipo se renombró o se mudó de archivo, y
	// el eje del tier dejó de recorrerse: justo el que dejaba pasar a P1-m8.
	if len(out) < 2 {
		t.Fatalf("encontré %d constantes de Tier en %s y son al menos dos: el tipo cambió de nombre o de lugar y las tablas de frescura quedaron mirando un solo tier", len(out), fuente)
	}
	return out
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

// accionesDePolitica cuenta los comandos que una política dejó en la bitácora de UNA máquina: los
// de origen `politica` con ese argv entero. El aviso al usuario (`musubi:avisar`) queda afuera por
// el argv.
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

// A131 · T2 — UNA POLÍTICA DE HOST ACTÚA SÓLO SI LA MÁQUINA LATE Y SU MUESTRA ES RECIENTE, LAS DOS
// COSAS CONTRA EL UMBRAL QUE EL INVENTARIO PUBLICA PARA ESA MÁQUINA.
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
//   - las DOS HORAS, iguales: `latir(t, s, id, muestraSana(95, hace30), hace30)` pone el latido y
//     la `tomada` de la muestra en el mismo instante, así que cualquiera de las dos mitades de la
//     guarda la sostiene sola, y sacar la mitad `EnLinea` (P1-m6) pasaba. Esa mitad no sobra:
//     `tomada` la pone el RELOJ DEL AGENTE y Muestra.Valida no rechaza fechas futuras, así que una
//     máquina callada cuyo reloj iba adelantado deja una muestra que dice «ahora», y sin `EnLinea`
//     la política actúa sobre una máquina CAÍDA.
//
// Exposición medida (auditoría A131): 0 pares expuestos. La única política de host es
// `vaciar-journal` (disco libre < 10 %) sobre musubi-server, un Tier A, y su disco libre no bajó del
// 23,55 % en 30 días. Los estados sí existen: en 30 días `tomada` llegó hasta 5,8 s después del
// `last_seen` guardado (altura-db) y hasta 2 s adelante del reloj del cerebro (gio, davantis-1); la
// muestra de musubi-server pasó los 90 s sin llegar a 15 min en 11 de 39.278 pasos de un minuto
// (ninguno mientras latía: 88,8 s de máximo en línea), la de davantis-1 pasó los 90 s estando en
// línea 26 min en 90 días, y la de altura-db (Tier B) pasó sus 15 min sin llegar a la hora en 5 de
// 36.036.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE
//
// Una subprueba por cada Tier declarado en internal/fleet/device.go (AST) y por cada intervalo de
// sondeosDeLasTablasDeFrescura. El umbral `u` de cada máquina NO se escribe acá: se lee de
// musubi_fleet_list, el `umbral_segundos` que el inventario publica al lado de `online`. Una
// máquina por cada combinación de la edad del LATIDO (0, u−1s, u+1s) y la de la MUESTRA (u en el
// futuro, 0, u−1s, u+1s), más una que late sin muestra; todas con la RAM al 95 % y todas juzgadas
// en UN barrido de aplicarPoliticas. El hecho contra el que se compara está escrito y no copiado
// del código: actúa ⇔ el tier admite `exec` ∧ hay muestra ∧ latido ≤ u ∧ muestra ≤ u.
//
// Un tier que no admite `exec` (hoy el C) no actúa nunca, y sus filas lo exigen igual: el día que
// lo admita, pasan a medir la frescura sin tocar esta prueba. PISO: cada fila se relee del registro
// antes del barrido para confirmar que representa su caso; en cada tier que admite `exec` hay
// máquinas que actúan y máquinas que no; y los tiers no publican todos el mismo umbral, porque sin
// eso la tabla no distingue la fórmula de un tier de la de otro.
//
// Cada sabotaje de abajo declara también un ARREGLO: un cambio correcto y equivalente que la tabla
// no puede castigar. Lo que mide es el umbral y las dos horas, no la forma en que están escritos.
//
// Sabotaje: sacar la mitad `EnLinea` de la guarda de host (P1-m6): una máquina caída con una
// muestra que dice «ahora» dispara. El arreglo pregunta lo mismo sin EnLinea.
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
func TestUnaPoliticaDeHostActuaSoloConLatidoYMuestraDentroDelUmbralDeSuTier(t *testing.T) {
	type edad struct {
		nombre string
		de     func(u time.Duration) time.Duration // la edad, en función del umbral de la máquina
	}
	cero := edad{"0", func(time.Duration) time.Duration { return 0 }}
	casi := edad{"u-1s", func(u time.Duration) time.Duration { return u - time.Second }}
	pasada := edad{"u+1s", func(u time.Duration) time.Duration { return u + time.Second }}
	// Un reloj de agente adelantado UN umbral entero: la muestra dice venir del futuro.
	futura := edad{"futura", func(u time.Duration) time.Duration { return -u }}
	// EL ORDEN ES DE LA MÁS VIEJA A LA MÁS NUEVA, Y NO ES ESTÉTICO: la primera fila que cae es el
	// MOTIVO con el que el arnés compara los sabotajes entre sí, y en este orden cada mutación de
	// A131 cae primero en una fila distinta (P1-m7 en latido y muestra pasados; P1-m6 en latido
	// pasado con muestra casi; P1-m9 en latido casi con muestra pasada). En el orden inverso, P1-m7
	// y P1-m9 caían en la misma fila y el arnés las leía como un solo sabotaje contado dos veces.
	latidos := []edad{pasada, casi, cero}
	muestras := []edad{pasada, casi, cero, futura}
	pol := politicaDeMemoria() // mem_pct > 90, sobre todas, journalctl
	umbralPorSondeo := map[float64]map[fleet.Tier]time.Duration{}

	for _, tier := range tiersDeclarados(t) {
		for _, pm := range sondeosDeLasTablasDeFrescura {
			t.Run(fmt.Sprintf("tier=%s/probe_minutes=%v", tier, pm), func(t *testing.T) {
				type fila struct {
					d               fleet.Device
					latido, muestra edad
					sinMuestra      bool
				}
				s := newTestServer(t, embedding.NoopProvider{})
				var filas []fila
				for _, l := range latidos {
					for _, m := range muestras {
						d := maquinaDeTier(t, s, tier, "latido-"+l.nombre+"_muestra-"+m.nombre)
						filas = append(filas, fila{d: d, latido: l, muestra: m})
					}
				}
				// La que late sin muestra lleva una edad de muestra de cero sólo para que la fila tenga
				// las dos; no se usa, porque `sinMuestra` decide antes.
				filas = append(filas, fila{d: maquinaDeTier(t, s, tier, "latido-0_sin-muestra"), latido: cero, muestra: cero, sinMuestra: true})
				if err := s.ConfigurarFlota(config.FleetConfig{ProbeMinutes: pm, Policies: []config.PolicyConfig{pol}}); err != nil {
					t.Fatalf("ConfigurarFlota: %v", err)
				}
				s.buscarPrincipal = registroDePrueba(autoHeal())
				umbrales := umbralesPublicados(t, s)
				puedeActuar := fleet.TierAdmite(tier, fleet.CapExec)
				if umbralPorSondeo[pm] == nil {
					umbralPorSondeo[pm] = map[fleet.Tier]time.Duration{}
				}

				// Al segundo: `last_seen` se guarda en RFC3339, y un `ahora` con fracción correría
				// cada latido hasta un segundo hacia atrás — justo el margen de u−1s.
				ahora := time.Now().UTC().Truncate(time.Second)
				esperadas := 0
				for i, f := range filas {
					u := umbrales[f.d.Name]
					if u <= 2*time.Second {
						t.Fatalf("%s publica un umbral de %s: con eso u−1s y u+1s no separan nada", f.d.Name, u)
					}
					umbralPorSondeo[pm][tier] = u
					cuando := ahora.Add(-f.latido.de(u))
					if f.sinMuestra {
						if _, err := s.engine.LatirDevice(f.d.ID, cuando, ""); err != nil {
							t.Fatalf("latir sin muestra: %v", err)
						}
					} else {
						latir(t, s, f.d.ID, muestraSana(95, ahora.Add(-f.muestra.de(u))), cuando)
					}
					// LA FILA REPRESENTA SU CASO, leída del registro: si el latido guardado se corriera
					// (truncado, zona horaria), la fila mediría otra cosa y su veredicto no diría nada.
					leida, _, err := s.engine.DevicePorNombre("casa", f.d.Name)
					if err != nil {
						t.Fatal(err)
					}
					if got := ahora.Sub(leida.LastSeen); got != f.latido.de(u) {
						t.Fatalf("%s: el registro guardó el latido de hace %s y la fila lo puso hace %s: no representa su caso", f.d.Name, got, f.latido.de(u))
					}
					if f.sinMuestra != (leida.UltimaMuestra == nil) {
						t.Fatalf("%s: la fila dice sin-muestra=%v y el registro tiene muestra=%v", f.d.Name, f.sinMuestra, leida.UltimaMuestra != nil)
					}
					if !f.sinMuestra {
						if got := ahora.Sub(leida.UltimaMuestra.Tomada); got != f.muestra.de(u) {
							t.Fatalf("%s: la muestra quedó de hace %s y la fila la puso de hace %s", f.d.Name, got, f.muestra.de(u))
						}
						if v, dispara := politicaDeMemoria2().Dispara(leida.UltimaMuestra); !dispara {
							t.Fatalf("%s: la condición no se cumple (mem=%v): la fila mediría la condición y no la frescura", f.d.Name, v)
						}
					}
					filas[i].d = leida
					if puedeActuar && !f.sinMuestra && f.latido.de(u) <= u && f.muestra.de(u) <= u {
						esperadas++
					}
				}
				if puedeActuar && (esperadas == 0 || esperadas == len(filas)) {
					t.Fatalf("PISO: el tier %s admite `exec` y la tabla espera %d acción(es) de %d: sin filas de los dos lados no distingue nada", tier, esperadas, len(filas))
				}

				n := s.aplicarPoliticas("casa", ahora)

				for _, f := range filas {
					u := umbrales[f.d.Name]
					edadLatido, edadMuestra := f.latido.de(u), f.muestra.de(u)
					debe := puedeActuar && !f.sinMuestra && edadLatido <= u && edadMuestra <= u
					actuo := accionesDePolitica(t, s, f.d, pol.Run)
					switch {
					case actuo > 1:
						t.Errorf("%s: la política encoló %d comandos en UN barrido", f.d.Name, actuo)
					case debe && actuo == 0:
						t.Errorf("%s (tier %s, umbral publicado %s): latido %s y muestra %s, las dos dentro del umbral, y la "+
							"política NO actuó. La guarda de frescura es más estricta que el umbral que decide «en línea»: la "+
							"política se apaga sola sobre una máquina sana mientras el inventario la muestra `online`",
							f.d.Name, tier, u, edadLegible(edadLatido), edadLegible(edadMuestra))
					case !debe && actuo > 0:
						t.Errorf("%s (tier %s, umbral publicado %s): %s", f.d.Name, tier, u,
							porQueNoDebiaActuar(puedeActuar, f.sinMuestra, edadLatido, edadMuestra, u))
					}
				}
				if n != esperadas {
					t.Errorf("aplicarPoliticas dice %d acción(es) y la tabla espera %d", n, esperadas)
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
func porQueNoDebiaActuar(puedeActuar, sinMuestra bool, edadLatido, edadMuestra, u time.Duration) string {
	switch {
	case !puedeActuar:
		return "el tier no admite `exec` y la política actuó igual: la compuerta del aparato no frenó"
	case sinMuestra:
		return "late sin muestra y la política actuó: no hay dato sobre el cual decidir, y decidir sin dato es inventarlo"
	case edadLatido > u:
		return fmt.Sprintf("la máquina figura CAÍDA —latido %s— y la política actuó sobre una muestra %s. `tomada` la pone "+
			"el reloj del AGENTE: la muestra de una máquina caída es rancia por definición, diga lo que diga su fecha",
			edadLegible(edadLatido), edadLegible(edadMuestra))
	default:
		return fmt.Sprintf("la máquina late (latido %s) pero su muestra es %s: el colector murió y la política actúa "+
			"sobre el último dato bueno, que siendo el último siempre cruza la condición",
			edadLegible(edadLatido), edadLegible(edadMuestra))
	}
}

// edadLegible escribe una edad para un mensaje de error: «de hace 1m31s», o «del futuro» cuando el
// reloj del agente iba adelantado. Un «de hace -1m30s» se lee como un error de la prueba.
func edadLegible(d time.Duration) string {
	if d < 0 {
		return fmt.Sprintf("del futuro (%s adelante del reloj del cerebro)", -d)
	}
	return fmt.Sprintf("de hace %s", d)
}

// estadoDeLaMuestraDelHost es un estado de la muestra del host para la tabla de las políticas de
// servicio, con la condición de la guarda de host de evaluarPolitica que rompe —leída después del
// registro, no supuesta—.
type estadoDeLaMuestraDelHost struct {
	nombre string
	// sinMuestra es `UltimaMuestra == nil`; caida, `!EnLinea`; rancia, la edad de `tomada` pasada
	// del umbral. Son las tres condiciones de la guarda de host.
	sinMuestra, caida, rancia bool
	preparar                  func(t *testing.T, s *McpServer, d fleet.Device, ahora time.Time, u time.Duration)
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
// Una subprueba por cada Tier (AST) y por cada intervalo de sondeosDeLasTablasDeFrescura. Una
// máquina por cada combinación de
//
//   - un estado de la MUESTRA del host por cada condición de la guarda de host de evaluarPolitica,
//     y cada una SOLA: late sin muestra (`UltimaMuestra == nil`); caída con una muestra que dice ser
//     de ahora (`!EnLinea`); late con la muestra rancia, el colector muerto (la edad de `tomada`);
//     más nunca latió, y el control, muestra fresca. Cada estado se relee del registro antes del
//     barrido y tiene que romper exactamente las condiciones que dice.
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
// del inventario dicen cosas distintas — sin ella la tabla no distingue cuál de los dos se usa.
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
func TestUnaPoliticaDeServicioDecidePorSuInventarioYNoPorLaMuestraDelHost(t *testing.T) {
	estados := []estadoDeLaMuestraDelHost{
		{"nunca-latio", true, true, false, func(*testing.T, *McpServer, fleet.Device, time.Time, time.Duration) {}},
		{"late-sin-muestra", true, false, false, func(t *testing.T, s *McpServer, d fleet.Device, ahora time.Time, _ time.Duration) {
			if _, err := s.engine.LatirDevice(d.ID, ahora, ""); err != nil {
				t.Fatal(err)
			}
		}},
		{"caida-con-muestra-de-ahora", false, true, false, func(t *testing.T, s *McpServer, d fleet.Device, ahora time.Time, u time.Duration) {
			latir(t, s, d.ID, muestraSana(40, ahora), ahora.Add(-u-time.Second))
		}},
		{"colector-muerto", false, false, true, func(t *testing.T, s *McpServer, d fleet.Device, ahora time.Time, u time.Duration) {
			vieja := ahora.Add(-u - time.Second)
			latir(t, s, d.ID, muestraSana(40, vieja), vieja)
			if _, err := s.engine.LatirDevice(d.ID, ahora, ""); err != nil {
				t.Fatal(err)
			}
		}},
		{"muestra-fresca", false, false, false, func(t *testing.T, s *McpServer, d fleet.Device, ahora time.Time, _ time.Duration) {
			latir(t, s, d.ID, muestraSana(40, ahora), ahora)
		}},
	}
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
					d      fleet.Device
					estado estadoDeLaMuestraDelHost
					edad   time.Duration
				}
				s := newTestServer(t, embedding.NoopProvider{})
				if err := s.ConfigurarFlota(config.FleetConfig{ProbeMinutes: pm, Policies: []config.PolicyConfig{pol}}); err != nil {
					t.Fatalf("ConfigurarFlota: %v", err)
				}
				s.buscarPrincipal = registroQuePermiteSystemctl()
				// El umbral del host se lee de una máquina del mismo tier, dada de alta sólo para eso:
				// todas las filas lo comparten, y de él se derivan las edades del inventario.
				testigo := maquinaDeTier(t, s, tier, "testigo-del-umbral")
				u := umbralesPublicados(t, s)[testigo.Name]
				if u <= time.Second {
					t.Fatalf("el tier %s publica un umbral de %s", tier, u)
				}
				var filas []fila
				for _, e := range estados {
					for _, edad := range edadesDelInventario(u) {
						d := maquinaDeTier(t, s, tier, fmt.Sprintf("%s_inventario-%ds", e.nombre, int(edad.Seconds())))
						filas = append(filas, fila{d: d, estado: e, edad: edad})
					}
				}
				puedeActuar := fleet.TierAdmite(tier, fleet.CapExec)

				ahora := time.Now().UTC().Truncate(time.Second)
				esperadas, discrepan := 0, 0
				for i, f := range filas {
					f.estado.preparar(t, s, f.d, ahora, u)
					reportado := ahora.Add(-f.edad)
					if _, _, err := s.engine.ReportarServicios(f.d.ID, reportado, []fleet.ReporteServicio{
						{Nombre: "nginx", Clase: "systemd", Salud: fleet.SaludServicio{Tomada: reportado, Estado: fleet.EstadoFallado}}}); err != nil {
						t.Fatalf("ReportarServicios: %v", err)
					}
					// EL ESTADO SE RELEE DEL REGISTRO: cada uno tiene que romper EXACTAMENTE las
					// condiciones de la guarda de host que dice romper, o la fila mide otra cosa.
					leida, _, err := s.engine.DevicePorNombre("casa", f.d.Name)
					if err != nil {
						t.Fatal(err)
					}
					sinMuestra := leida.UltimaMuestra == nil
					caida := !leida.EnLinea(ahora, u)
					rancia := !sinMuestra && ahora.Sub(leida.UltimaMuestra.Tomada) > u
					if sinMuestra != f.estado.sinMuestra || caida != f.estado.caida || rancia != f.estado.rancia {
						t.Fatalf("%s: en el registro quedó sin-muestra=%v caída=%v rancia=%v y el estado dice %v/%v/%v: la fila no representa su caso",
							f.d.Name, sinMuestra, caida, rancia, f.estado.sinMuestra, f.estado.caida, f.estado.rancia)
					}
					filas[i].d = leida
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

				n := s.aplicarPoliticas("casa", ahora)

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
							"tienen que decir lo mismo", f.d.Name, tier, f.estado.nombre, actuo > 0, fresco)
					}
					// Y el HECHO. Cada mensaje dice los hechos de la fila, no adivina el código.
					switch {
					case actuo > 1:
						t.Errorf("%s: la política encoló %d comandos en UN barrido", f.d.Name, actuo)
					case debe && actuo == 0:
						t.Errorf("%s (tier %s, muestra del host: %s; umbral de «en línea» %s): nginx caído con el inventario de hace %s, "+
							"fresco para el umbral del INVENTARIO (%s), y la política NO actuó: el auto-heal de servicios queda inerte "+
							"justo donde sirve. Si a esta edad no actúa en NINGÚN estado de la muestra, el inventario se mide con otro "+
							"umbral; si sólo falla sin muestra fresca, la frena una guarda del host o el barrido la saltea antes de "+
							"bifurcar por clase", f.d.Name, tier, f.estado.nombre, u, f.edad, fleet.UmbralInventario)
					case !debe && actuo > 0 && !puedeActuar:
						t.Errorf("%s: el tier %s no admite `exec` y la política actuó igual", f.d.Name, tier)
					case !debe && actuo > 0:
						t.Errorf("%s (tier %s, umbral de «en línea» %s): el inventario es de hace %s, pasado el umbral del INVENTARIO (%s), "+
							"y la política actuó: su frescura no se mira, o se mide con un umbral más largo que el suyo, y una máquina "+
							"que dejó de enumerar sus servicios tiene políticas actuando sobre el último estado conocido",
							f.d.Name, tier, u, f.edad, fleet.UmbralInventario)
					}
				}
				if n != esperadas {
					t.Errorf("aplicarPoliticas dice %d acción(es) y la tabla espera %d", n, esperadas)
				}
			})
		}
	}
}
