package memory

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestLedgerAddAndStatus(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.LedgerAdd("s1", "turn_recall", 10); err != nil {
		t.Fatalf("LedgerAdd error: %v", err)
	}
	if _, err := e.LedgerAdd("s1", "hydration", 5); err != nil {
		t.Fatalf("LedgerAdd error: %v", err)
	}
	l, err := e.LedgerStatus()
	if err != nil {
		t.Fatalf("LedgerStatus error: %v", err)
	}
	if l.SessionID != "s1" || l.Total != 15 {
		t.Errorf("esperaba session s1 total 15, obtuve %+v", l)
	}
	if l.Surfaces["turn_recall"] != 10 || l.Surfaces["hydration"] != 5 {
		t.Errorf("conteos por superficie incorrectos: %+v", l.Surfaces)
	}
}

// Una sesión nueva arranca SU cuenta en cero. Antes esta prueba se llamaba «ResetsOnNewSession»
// y describía el defecto como si fuera el contrato: la sesión nueva no reiniciaba sólo su cuenta,
// borraba la de todas. Lo que custodia que la vieja SOBREVIVA es TestLedgerSesionNuevaNoBorraLasDemas.
func TestLedgerSesionNuevaArrancaDeCero(t *testing.T) {
	e := newTestEngine(t)
	e.LedgerAdd("s1", "turn_recall", 10)
	l, err := e.LedgerAdd("s2", "turn_recall", 7)
	if err != nil {
		t.Fatalf("LedgerAdd error: %v", err)
	}
	if l.SessionID != "s2" || l.Total != 7 {
		t.Errorf("una sesión nueva arranca su propia cuenta en cero; obtuve %+v", l)
	}
}

func TestLedgerEmptySessionKeepsCurrent(t *testing.T) {
	e := newTestEngine(t)
	e.LedgerAdd("s1", "turn_recall", 10)
	// sessionID vacío (sin id de hook): acumula bajo la sesión activa, no reinicia.
	l, _ := e.LedgerAdd("", "hydration", 5)
	if l.SessionID != "s1" || l.Total != 15 {
		t.Errorf("sessionID vacío debe acumular en la sesión activa; obtuve %+v", l)
	}
}

func TestBudgetReport(t *testing.T) {
	l := TokenLedger{
		SessionID: "s1",
		Total:     8000,
		Surfaces:  map[string]int{"startup_cognitive": 5000, "turn_recall": 2000, "startup_priming": 1000},
	}

	// Con presupuesto excedido: estado "over", restante negativo, % usado 100.
	b := l.Budget(8000)
	if b.Status != "over" {
		t.Errorf("8000/8000 debe ser 'over', obtuve %q", b.Status)
	}
	if b.Budget != 8000 || b.Remaining != 0 || b.PctUsed != 100 {
		t.Errorf("budget/remaining/pct incorrectos: %+v", b)
	}
	// Desglose ordenado por gasto desc, con % del total.
	if len(b.Surfaces) != 3 || b.Surfaces[0].Surface != "startup_cognitive" {
		t.Fatalf("esperaba 3 superficies con la mayor primero, obtuve %+v", b.Surfaces)
	}
	if b.Surfaces[0].Pct != 63 { // 5000/8000 = 62.5 -> 63 (redondeo)
		t.Errorf("pct de la superficie top: esperaba 63, obtuve %d", b.Surfaces[0].Pct)
	}

	// Estado "watch" a partir del 75%.
	if got := l.Budget(10000).Status; got != "watch" { // 8000/10000 = 80%
		t.Errorf("80%% debe ser 'watch', obtuve %q", got)
	}
	// Estado "ok" por debajo del 75%.
	if got := l.Budget(16000).Status; got != "ok" { // 8000/16000 = 50%
		t.Errorf("50%% debe ser 'ok', obtuve %q", got)
	}
	// Sin presupuesto (0): "unbudgeted", sin techo ni % pero con desglose.
	un := l.Budget(0)
	if un.Status != "unbudgeted" || un.Budget != 0 || len(un.Surfaces) != 3 {
		t.Errorf("budget 0 debe dar 'unbudgeted' con desglose, obtuve %+v", un)
	}
}

// EL CONTRATO DEL QUE DEPENDE EL DASHBOARD para distinguir un acumulado de una sesión.
//
// En un proceso always-on sin hooks —el cerebro central bajo `musubi serve`— nadie pasa un
// sessionID, así que el ledger no rota y el total es un acumulado de por vida. El único dato que
// permite darse cuenta es que SessionID viene VACÍO, y para eso tiene que sobrevivir el viaje a
// JSON: si alguien le pone `omitempty` al campo o lo saca del reporte, el dashboard vuelve a
// mostrar "Tokens de sesión 2.153.453 / 8000" con la barra clavada, y nadie se entera.
func TestBudgetExponeSessionIDVacioParaDistinguirAcumulado(t *testing.T) {
	sinSesion := TokenLedger{Total: 2153453, Surfaces: map[string]int{"hydration": 2153453}}
	b := sinSesion.Budget(8000)

	if b.SessionID != "" {
		t.Errorf("un ledger sin sesión debe reportar SessionID vacío, obtuve %q", b.SessionID)
	}
	// El JSON es lo que ve el dashboard: la clave tiene que estar presente aunque valga "".
	data, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"session_id"`) {
		t.Errorf("el reporte no expone session_id: el dashboard no puede distinguir acumulado de sesión\n%s", data)
	}

	// Y con sesión, el id viaja igual: es el otro lado del mismo contrato.
	conSesion := TokenLedger{SessionID: "s1", Total: 100, Surfaces: map[string]int{"turn_recall": 100}}
	if got := conSesion.Budget(8000).SessionID; got != "s1" {
		t.Errorf("con sesión el id debe viajar al reporte, obtuve %q", got)
	}
}

func TestLedgerReset(t *testing.T) {
	e := newTestEngine(t)
	e.LedgerAdd("s1", "turn_recall", 10)
	if err := e.LedgerReset(); err != nil {
		t.Fatalf("LedgerReset error: %v", err)
	}
	l, _ := e.LedgerStatus()
	if l.Total != 0 || len(l.Surfaces) != 0 {
		t.Errorf("tras reset el ledger debe quedar vacío; obtuve %+v", l)
	}
}

// EL INVARIANTE DEL ARREGLO. La casilla era única: `if sessionID != l.SessionID` la reiniciaba
// entera, así que cada terminal que escribía BORRABA la cuenta de todas las demás. Medido el
// 2026-09-23: `musubi_tokens` decía 262 tokens de una sola superficie para una sesión que llevaba
// el día entero trabajando, con 10 procesos `musubi` sobre el mismo cuaderno.
func TestLedgerSesionNuevaNoBorraLasDemas(t *testing.T) {
	e := newTestEngine(t)
	e.LedgerAdd("s1", "turn_recall", 10)
	e.LedgerAdd("s1", "startup_priming", 30)
	e.LedgerAdd("s2", "turn_recall", 7) // otra terminal escribe
	s1, err := e.LedgerStatusDe("s1")
	if err != nil {
		t.Fatal(err)
	}
	if s1.Total != 40 || s1.Surfaces["startup_priming"] != 30 {
		t.Fatalf("la sesión s1 tenía que sobrevivir entera a que escribiera s2; obtuve %+v", s1)
	}
	if s2, _ := e.LedgerStatusDe("s2"); s2.Total != 7 {
		t.Fatalf("s2 tiene su propia cuenta: quería 7, obtuve %+v", s2)
	}
}

// Instalar el binario nuevo NO puede poner en cero el contador de la sesión en curso: el valor de
// una sola casilla se migra como una sesión más. Sin esto, la actualización repetiría en silencio
// la misma clase de pérdida que este arreglo viene a quitar.
func TestLedgerMigraElFormatoViejo(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SetMeta(metaTokenLedger, `{"session_id":"vieja","total":262,"surfaces":{"turn_recall":262}}`); err != nil {
		t.Fatal(err)
	}
	l, err := e.LedgerStatus()
	if err != nil {
		t.Fatal(err)
	}
	if l.SessionID != "vieja" || l.Total != 262 || l.Surfaces["turn_recall"] != 262 {
		t.Fatalf("el formato viejo tenía que leerse, no descartarse; obtuve %+v", l)
	}
	// Y seguir sumando sobre la migrada, no arrancar de cero.
	l2, _ := e.LedgerAdd("vieja", "turn_recall", 8)
	if l2.Total != 270 {
		t.Fatalf("la sesión migrada tenía que seguir sumando: quería 270, obtuve %+v", l2)
	}
}

// El valor vive en una sola fila de `meta`: sin tope, una máquina que abre terminales todo el día lo
// haría crecer sin freno. Se desaloja la MENOS recientemente escrita, no una cualquiera.
func TestLedgerDesalojaLaSesionMasVieja(t *testing.T) {
	e := newTestEngine(t)
	for i := 0; i < maxSesionesEnLedger+4; i++ {
		if _, err := e.LedgerAdd(fmt.Sprintf("s%02d", i), "turn_recall", 1); err != nil {
			t.Fatal(err)
		}
	}
	st, err := e.loadLedgerStore()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Sesiones) != maxSesionesEnLedger {
		t.Fatalf("tope de %d sesiones; quedaron %d", maxSesionesEnLedger, len(st.Sesiones))
	}
	for i := 0; i < 4; i++ {
		if _, sigue := st.Sesiones[fmt.Sprintf("s%02d", i)]; sigue {
			t.Errorf("s%02d era de las más viejas y tenía que salir", i)
		}
	}
	if _, sigue := st.Sesiones[fmt.Sprintf("s%02d", maxSesionesEnLedger+3)]; !sigue {
		t.Error("la sesión más nueva nunca puede ser la desalojada")
	}
}

// La suma corre dentro de una transacción. Sin ella, dos terminales que escriben a la vez leen el
// mismo valor, suman cada una lo suyo y la segunda escritura pisa a la primera. Con la casilla única
// eso perdía un incremento; ahora el valor lleva a todas las sesiones, así que pisar cuesta la cuenta
// entera de otra terminal.
func TestLedgerDosTerminalesALaVezNoSePisan(t *testing.T) {
	e := newTestEngine(t)
	const porSesion = 40
	sesiones := []string{"a", "b", "c", "d"}
	var wg sync.WaitGroup
	for _, s := range sesiones {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			for i := 0; i < porSesion; i++ {
				if _, err := e.LedgerAdd(s, "turn_recall", 1); err != nil {
					t.Errorf("LedgerAdd(%s): %v", s, err)
					return
				}
			}
		}(s)
	}
	wg.Wait()
	for _, s := range sesiones {
		if l, _ := e.LedgerStatusDe(s); l.Total != porSesion {
			t.Errorf("sesión %s: quería %d, obtuve %d — una escritura concurrente se comió sumas", s, porSesion, l.Total)
		}
	}
}
