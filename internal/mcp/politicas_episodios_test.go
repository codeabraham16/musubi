package mcp

// A131 · T5 — CADA COMPUERTA QUE FRENA A UNA POLÍTICA SE CUENTA EN CADA TICK Y SE AVISA UNA VEZ
// POR EPISODIO.

import (
	"bytes"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// logCompartido es un destino para logx.Capturar que aguanta escrituras concurrentes: el logger es
// GLOBAL, y una goroutine que otra prueba dejó viva puede escribir mientras ésta lee.
type logCompartido struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (w *logCompartido) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.Write(p)
}

func (w *logCompartido) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.b.String()
}

// avisosEnElLog cuenta las líneas WARN o ERROR que nombran a la política: lo que un operador ve en
// el journal. Se cuentan LÍNEAS y no entradas de `avisosDados`, porque ese mapa lo comparten otros
// avisos —el embudo del aviso de acceso anota ahí sus propias claves— y un +1 ajeno se lee igual
// que «avisó» (así dio verde la primera sonda de P2-m15 sobre el árbol roto).
func avisosEnElLog(log *logCompartido, politica string) int {
	marca := "politica=" + politica
	n := 0
	for _, l := range strings.Split(log.String(), "\n") {
		if strings.Contains(l, marca) && (strings.Contains(l, "level=WARN") || strings.Contains(l, "level=ERROR")) {
			n++
		}
	}
	return n
}

// A131 · T5 — CADA FRENO SE CUENTA EN CADA TICK Y SE AVISA UNA VEZ POR EPISODIO.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ EJE CLAVABA LA GUARDA DE ANTES
//
// TestUnFalloDeConfiguracionDeUnaPoliticaSeAvisaUnaVezYSeCuentaSiempre mide la regla —el aviso es
// un ESTADO (una vez mientras dure) y la métrica una SEÑAL (cada evaluación)— para UNA sola causa,
// el principal ausente, en UN solo episodio, y cuenta los avisos por el tamaño de `avisosDados`.
// Tres mutaciones pasaban en verde por esos tres ejes clavados:
//
//   - P2-m14: el `rechazada` de la COMPUERTA metido adentro del cierre que se emite una vez. El
//     rechazo contaba 1 de cada 10, y PoliticaSinPermiso (`increase(...[1h]) > 0`) se habría
//     resuelto sola a la hora con la política todavía inerte.
//   - P2-m15: el rearme de `sin_principal` borrado. El primer episodio avisaba y la SEGUNDA
//     revocación —la del mes que viene, en el mismo proceso— pasaba muda.
//   - P2-m17: el WARN afuera del cierre, con la clave anotándose igual una vez. El mapa decía «un
//     aviso» y el journal tenía una línea por tick: 288 por día, el ruido que la regla evita.
//
// Exposición medida (auditoría A131): 0. La única política en producción, `vaciar-journal`, tiene
// sus series `rechazada` y `sin_principal` en 0 como máximo en toda la retención (~24 días), y
// PoliticaSinPermiso no aparece en ALERTS en 30 días. Los tres defectos eran latentes: se volvían
// vivos la primera vez que alguien editara principals.yaml y le sacara a la política su `exec`.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE
//
// Una fila por CADA freno de frenoDePolitica (leídos del AST de politicas.go: un freno nuevo sin
// fila pone esto rojo), y cada clave de aviso de politicas.go tiene que tener al menos una fila que
// la lleve a su condición (leídas del mismo AST, con clavesDeAvisoDelArchivo). Por fila, dos
// episodios separados por un arreglo en el que la política vuelve a actuar:
//
//  1. diez ticks con la compuerta cerrada: el resultado de la fila cuenta 10 —y ningún otro
//     resultado cuenta nada— y el log tiene UNA línea sobre la política (cero si el freno no avisa);
//  2. el arreglo: la política actúa (el control positivo: sin él, el segundo episodio no empezaría
//     desde un estado sano) y el log no suma nada;
//  3. pasado el cooldown, tres ticks con la compuerta cerrada otra vez: el conteo suma 3 y el log
//     tiene la SEGUNDA línea. «Una vez» es una vez por episodio, no una vez en la vida del proceso.
//
// El instrumento es la LÍNEA DE LOG y no el mapa (ver avisosEnElLog), y la política tiene un nombre
// propio para que ninguna otra prueba escriba líneas que se le parezcan.
//
// Sabotaje: el rechazo se cuenta una sola vez en vez de en cada tick (la forma de P2-m14).
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tcase frenoSinExec, frenoAllowlist:\n\t\ts.metrics.contarPolitica(pol.Nombre, \"rechazada\")\n"
// arnes: a="\tcase frenoSinExec, frenoAllowlist:\n\t\tif _, ya := s.avisosDados.LoadOrStore(\"rechazo_contado:\"+par, true); !ya {\n\t\t\ts.metrics.contarPolitica(pol.Nombre, \"rechazada\")\n\t\t}\n"
//
// Sabotaje: el aviso de la allowlist sin rearme — `avisarUnaVez` bajo un `if`, sin Delete en ningún lado (la forma de P2-m15).
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\ts.avisoMientras(\"allowlist:\"+par, freno == frenoAllowlist, func() {\n\t\tpermitidos, _ := comandosPermitidos(pr, d)\n\t\tlogx.Warn(\"política rechazada por la allowlist del principal\",\n\t\t\t\"politica\", pol.Nombre, \"principal\", pol.Principal, \"device\", d.Name,\n\t\t\t\"pidio\", pol.Hacer[0], \"permitidos\", permitidos)\n\t})\n"
// arnes: a="\tif freno == frenoAllowlist {\n\t\ts.avisarUnaVez(\"allowlist:\"+par, func() {\n\t\t\tpermitidos, _ := comandosPermitidos(pr, d)\n\t\t\tlogx.Warn(\"política rechazada por la allowlist del principal\",\n\t\t\t\t\"politica\", pol.Nombre, \"principal\", pol.Principal, \"device\", d.Name,\n\t\t\t\t\"pidio\", pol.Hacer[0], \"permitidos\", permitidos)\n\t\t})\n\t}\n"
//
// Sabotaje: el WARN de la compuerta afuera de su cierre, que queda vacío (la forma de P2-m17).
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\ts.avisoMientras(\"compuerta:\"+par, freno == frenoSinExec, func() {\n\t\tporque := \"el principal no tiene `exec` sobre esa máquina (su proyecto o su concesión no la alcanzan)\"\n\t\tif !d.Permite(fleet.CapExec) {\n\t\t\tporque = \"la máquina no admite `exec` (su tier o sus caps no lo incluyen, o está revocada)\"\n\t\t}\n\t\tlogx.Warn(\"política rechazada por la compuerta: \"+porque,\n\t\t\t\"politica\", pol.Nombre, \"principal\", pol.Principal, \"device\", d.Name)\n\t})\n"
// arnes: a="\ts.avisoMientras(\"compuerta:\"+par, freno == frenoSinExec, func() {})\n\tif freno == frenoSinExec {\n\t\tporque := \"el principal no tiene `exec` sobre esa máquina (su proyecto o su concesión no la alcanzan)\"\n\t\tif !d.Permite(fleet.CapExec) {\n\t\t\tporque = \"la máquina no admite `exec` (su tier o sus caps no lo incluyen, o está revocada)\"\n\t\t}\n\t\tlogx.Warn(\"política rechazada por la compuerta: \"+porque,\n\t\t\t\"politica\", pol.Nombre, \"principal\", pol.Principal, \"device\", d.Name)\n\t}\n"
func TestCadaFrenoSeCuentaEnCadaTickYSeAvisaUnaVezPorEpisodio(t *testing.T) {
	const nombre = "episodios-a131"
	type fila struct {
		freno     frenoDePolitica
		clave     string // prefijo de la clave de avisoMientras en politicas.go; "" = este freno no avisa
		resultado string // de resultadosDePolitica; "" = este freno no se cuenta
		// romper cierra la compuerta a partir de `desde`; arreglar la vuelve a abrir.
		romper   func(t *testing.T, s *McpServer, d fleet.Device, desde time.Time)
		arreglar func(t *testing.T, s *McpServer, d fleet.Device)
	}
	vigente := func(_ *testing.T, s *McpServer, _ fleet.Device) { s.buscarPrincipal = registroDePrueba(autoHeal()) }
	conRegistro := func(reg *PrincipalRegistry) func(*testing.T, *McpServer, fleet.Device, time.Time) {
		return func(_ *testing.T, s *McpServer, _ fleet.Device, _ time.Time) { s.buscarPrincipal = reg }
	}
	retocado := func(retoque func(*Principal)) *PrincipalRegistry {
		p := autoHeal()
		retoque(&p)
		return registroDePrueba(p)
	}
	grado := func(g fleet.Consentimiento) func(*testing.T, *McpServer, fleet.Device) {
		return func(t *testing.T, s *McpServer, d fleet.Device) {
			if _, err := s.engine.FijarConsentimiento(d.ID, g); err != nil {
				t.Fatalf("FijarConsentimiento(%s): %v", g, err)
			}
		}
	}
	conGrado := func(g fleet.Consentimiento) func(*testing.T, *McpServer, fleet.Device, time.Time) {
		return func(t *testing.T, s *McpServer, d fleet.Device, _ time.Time) { grado(g)(t, s, d) }
	}
	var ventana string // la ventana de mantenimiento abierta por la fila que la usa
	filas := []fila{
		{freno: frenoSinRegistro, clave: "", resultado: "",
			romper:   func(_ *testing.T, s *McpServer, _ fleet.Device, _ time.Time) { s.buscarPrincipal = nil },
			arreglar: vigente},
		{freno: frenoSinPrincipal, clave: "sin_principal:", resultado: "sin_principal",
			romper: conRegistro(registroDePrueba()), arreglar: vigente},
		{freno: frenoSinExec, clave: "compuerta:", resultado: "rechazada",
			romper: conRegistro(retocado(func(p *Principal) {
				p.Fleet = map[fleet.Cap][]string{fleet.CapExec: {"otra-maquina"}, fleet.CapMetrics: {"*"}}
			})),
			arreglar: vigente},
		{freno: frenoAllowlist, clave: "allowlist:", resultado: "rechazada",
			romper:   conRegistro(retocado(func(p *Principal) { p.ExecAllow = map[string][]string{"*": {"uptime"}} })),
			arreglar: vigente},
		{freno: frenoConsentimientoProhibido, clave: "consentimiento:", resultado: "consentimiento_prohibido",
			romper: conGrado(fleet.ConsentimientoProhibido), arreglar: grado(fleet.ConsentimientoAvisa)},
		{freno: frenoConsentimientoPide, clave: "consentimiento:", resultado: "consentimiento_pide",
			romper: conGrado(fleet.ConsentimientoPide), arreglar: grado(fleet.ConsentimientoAvisa)},
		{freno: frenoMantenimiento, clave: "", resultado: "mantenimiento",
			romper: func(t *testing.T, s *McpServer, d fleet.Device, desde time.Time) {
				m, err := s.engine.AbrirMantenimiento(fleet.Mantenimiento{
					DeviceID: d.ID, ProjectID: d.ProjectID, Principal: "gio",
					Desde: desde.Add(-time.Minute), Hasta: desde.Add(15 * time.Minute), Motivo: "episodio de prueba",
				})
				if err != nil {
					t.Fatalf("AbrirMantenimiento: %v", err)
				}
				ventana = m.ID
			},
			arreglar: func(t *testing.T, s *McpServer, d fleet.Device) {
				if ok, err := s.engine.CancelarMantenimiento(d.ID, d.ProjectID, ventana); err != nil || !ok {
					t.Fatalf("CancelarMantenimiento = %v, %v", ok, err)
				}
			}},
	}

	// ── LA TABLA RECORRE LOS DOS CONJUNTOS, leídos de politicas.go ──
	declarados := frenosDeclarados(t)
	cubiertos := map[frenoDePolitica]bool{}
	for _, f := range filas {
		cubiertos[f.freno] = true
	}
	var sinFila []string
	for freno, nombreConst := range declarados {
		// sinFreno es el estado SANO, no una compuerta: lo ejercita el arreglo de cada fila.
		if freno != sinFreno && !cubiertos[freno] {
			sinFila = append(sinFila, nombreConst+" ("+strconv.Quote(string(freno))+")")
		}
	}
	sort.Strings(sinFila)
	if len(sinFila) > 0 {
		t.Errorf("la tabla no tiene fila para %s: una compuerta sin fila es justo la que puede contar una vez "+
			"o avisar en cada tick sin que nada se ponga rojo", strings.Join(sinFila, ", "))
	}
	claves := clavesDeAvisoDelArchivo(t, "politicas.go")
	if len(claves) == 0 {
		t.Fatal("no encontré NI UN aviso en politicas.go: o se mudaron de archivo y esta guarda dejó de mirar " +
			"donde se decide, o el parser dejó de reconocer las llamadas. Ninguna de las dos es un verde")
	}
	conFila := map[string]bool{}
	for _, f := range filas {
		if f.clave != "" {
			conFila[f.clave] = true
		}
	}
	for _, k := range claves {
		if !conFila[k.clave] {
			t.Errorf("politicas.go emite el aviso %q (%s) y ninguna fila lo lleva a su condición: nadie mide que se "+
				"dé una vez por episodio y se rearme al resolverse", k.clave, k.pos)
		}
		delete(conFila, k.clave)
	}
	for k := range conFila {
		t.Errorf("hay una fila con la clave %q y politicas.go no emite ningún aviso con esa clave: la fila mide un aviso que no existe", k)
	}

	for _, f := range filas {
		t.Run(declarados[f.freno], func(t *testing.T) {
			pol := politicaDeMemoria()
			pol.Name = nombre
			s, d := prepararPolitica(t, pol, registroDePrueba(autoHeal()))
			// La máquina SABE preguntar: sin eso, `pide` se endurece a `prohibido` y su fila mediría
			// otra cosa. Y el disparo del arreglo encola su aviso en vez de dejar una línea de log.
			if err := s.engine.FijarCapacidadDePreguntar(d.ID, true); err != nil {
				t.Fatalf("FijarCapacidadDePreguntar: %v", err)
			}
			log := &logCompartido{}
			restaurar := logx.Capturar(log)
			defer restaurar()

			tick := func(cuando time.Time) int {
				latir(t, s, d.ID, muestraSana(95, cuando), cuando) // la condición se cumple en todos
				return s.aplicarPoliticas("casa", cuando)
			}
			avisos := 0
			if f.clave != "" {
				avisos = 1
			}
			exigirConteos := func(episodio string, veces, oks int64) {
				t.Helper()
				for _, r := range resultadosDePolitica {
					var quiero int64
					switch r {
					case f.resultado:
						quiero = veces
					case "ok":
						quiero = oks
					}
					if got := contarPoliticaOK(s, nombre, r); got != quiero {
						t.Errorf("%s: el resultado %q lleva contado %d en total y tenía que llevar %d. De esta serie "+
							"viven PoliticaSinPermiso y PoliticaFrenadaPorConsentimiento: contarla una vez por "+
							"episodio hace que la alerta se resuelva sola con la política todavía inerte", episodio, r, got, quiero)
					}
				}
			}

			// 1. PRIMER EPISODIO: diez ticks de un minuto con la compuerta cerrada.
			t0 := time.Now()
			f.romper(t, s, d, t0)
			for i := 0; i < 10; i++ {
				if n := tick(t0.Add(time.Duration(i) * time.Minute)); n != 0 {
					t.Fatalf("la política actuó con %s cerrado: la fila no cierra lo que dice cerrar", f.freno)
				}
			}
			exigirConteos("primer episodio", 10, 0)
			lineas := avisosEnElLog(log, nombre)
			if lineas != avisos {
				t.Errorf("primer episodio: %d línea(s) de log sobre la política en 10 ticks, y tenía que ser %d. "+
					"Un aviso que sale en cada tick son 288 líneas idénticas por día: el ruido que entierra la línea "+
					"que sí importa", lineas, avisos)
			}

			// 2. EL ARREGLO: la política vuelve a actuar, y eso no deja ninguna línea más.
			f.arreglar(t, s, d)
			if n := tick(t0.Add(20 * time.Minute)); n != 1 {
				t.Fatalf("control: arreglado %s, la política actuó %d vez/veces y tenía que actuar 1: sin esto el "+
					"segundo episodio no empieza desde un estado sano y no mide nada", f.freno, n)
			}
			if despues := avisosEnElLog(log, nombre); despues != lineas {
				t.Errorf("el tick sano dejó %d línea(s) de log sobre la política: con la compuerta abierta no hay "+
					"nada que avisar", despues-lineas)
			}
			lineas = avisosEnElLog(log, nombre)

			// 3. SEGUNDO EPISODIO, pasado el cooldown del disparo (60 min): tres ticks más.
			t1 := t0.Add(90 * time.Minute)
			f.romper(t, s, d, t1)
			for i := 0; i < 3; i++ {
				if n := tick(t1.Add(time.Duration(i) * time.Minute)); n != 0 {
					t.Fatalf("segundo episodio: la política actuó con %s cerrado", f.freno)
				}
			}
			exigirConteos("segundo episodio", 13, 1)
			if nuevas := avisosEnElLog(log, nombre) - lineas; nuevas != avisos {
				t.Errorf("segundo episodio: %d línea(s) de log sobre la política en 3 ticks, y tenía que ser %d. "+
					"«Una vez» es una vez por EPISODIO: sin el rearme, la segunda vez que se cierra la misma "+
					"compuerta —la revocación del mes que viene— pasa en silencio", nuevas, avisos)
			}
		})
	}
}
