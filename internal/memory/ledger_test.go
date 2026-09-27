package memory

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
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
	if err := e.LedgerReset(""); err != nil {
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
// haría crecer sin freno. A IGUAL total se desaloja la menos recientemente escrita, no una cualquiera
// (la política completa —menor total primero— la custodia TestLedgerDesalojoProtegeLaSesionGrande).
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

// DOS TERMINALES QUE ESCRIBEN A LA VEZ NO SE PISAN. La suma corre dentro de una transacción que toma
// el lock de escritura al nacer (`_txlock=immediate`, database.go), así que la segunda terminal no
// puede leer el ledger hasta que la primera escribió el suyo. Sin eso, las dos leen el mismo valor,
// suman cada una lo suyo y la segunda escritura pisa a la primera; y como el valor lleva a TODAS las
// sesiones, pisar cuesta la cuenta entera de otra terminal.
//
// ES DETERMINÍSTICA A PROPÓSITO (A134). La versión anterior eran cuatro goroutines sumando cuarenta
// veces cada una sobre el MISMO motor, y dependía de que ninguna se quedara sin turno más que el
// `busy_timeout` (5 s): en un runner de Windows cargado una se quedó (SQLITE_BUSY en el CI de #676),
// y la prueba contaba esa falta de disponibilidad como «se comió sumas». Tampoco tenía sabotaje:
// nadie la había visto roja sin la transacción. Ahora hay dos MOTORES sobre la misma base —dos
// terminales de verdad, cada una con su pool— y el orden se fuerza: A lee y queda frenada con la
// transacción abierta; B intenta sumar; mientras A está frenada, B no puede terminar; se suelta A, y
// B lee lo que A escribió. Nada de esto depende de la velocidad de la máquina.
//
// Sabotaje: leer el ledger ANTES de abrir la transacción → B lee el valor viejo mientras A está
// frenada y, al escribir, pisa el incremento de A.
// arnes: archivo="internal/memory/ledger.go"
// arnes: de="\ttx, err := e.db.Begin()\n\tif err != nil {\n\t\treturn TokenLedger{}, fmt.Errorf(\"error al abrir la transacción del ledger: %w\", err)\n\t}\n\tdefer func() { _ = tx.Rollback() }()\n\n\tst, err := leerLedgerStoreDe(tx)\n\tif err != nil {\n\t\treturn TokenLedger{}, err\n\t}\n"
// arnes: a="\tst, err := leerLedgerStoreDe(e.db)\n\tif err != nil {\n\t\treturn TokenLedger{}, err\n\t}\n\ttx, err := e.db.Begin()\n\tif err != nil {\n\t\treturn TokenLedger{}, fmt.Errorf(\"error al abrir la transacción del ledger: %w\", err)\n\t}\n\tdefer func() { _ = tx.Rollback() }()\n"
func TestLedgerDosTerminalesALaVezNoSePisan(t *testing.T) {
	dir := dirSembrado(t)
	a, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("motor A: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	b, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("motor B: %v", err)
	}
	t.Cleanup(func() { b.Close() })

	// A queda frenada DESPUÉS de leer, con la transacción —y el lock de escritura— tomada.
	frenada := make(chan struct{})
	soltar := make(chan struct{})
	soltarUnaVez := sync.OnceFunc(func() { close(soltar) })
	var sóloLaPrimera sync.Once
	anterior := ledgerEntreLeerYEscribir
	ledgerEntreLeerYEscribir = func(sessionID string) {
		if sessionID != "terminal-a" {
			return
		}
		sóloLaPrimera.Do(func() {
			close(frenada)
			<-soltar
		})
	}
	t.Cleanup(func() { ledgerEntreLeerYEscribir = anterior })
	t.Cleanup(soltarUnaVez) // si la prueba se corta antes, A no se queda colgada para siempre

	terminoA := make(chan error, 1)
	go func() {
		_, err := a.LedgerAdd("terminal-a", "turn_recall", 7)
		terminoA <- err
	}()
	select {
	case <-frenada:
	case err := <-terminoA:
		t.Fatalf("la terminal A terminó sin pasar por el punto entre leer y escribir (err=%v): la prueba no está midiendo nada", err)
	case <-time.After(10 * time.Second):
		t.Fatal("la terminal A nunca llegó a leer el ledger")
	}

	terminoB := make(chan error, 1)
	go func() {
		_, err := b.LedgerAdd("terminal-b", "turn_recall", 5)
		terminoB <- err
	}()
	// Mientras A está frenada con el lock tomado, B NO puede haber terminado de sumar.
	select {
	case err := <-terminoB:
		soltarUnaVez()
		t.Fatalf("la terminal B terminó de sumar (err=%v) mientras A estaba frenada entre leer y escribir: "+
			"la suma no toma el lock de escritura al nacer, y las dos pueden leer el mismo valor", err)
	case <-time.After(300 * time.Millisecond):
	}

	soltarUnaVez()
	if err := <-terminoA; err != nil {
		t.Fatalf("terminal A: %v", err)
	}
	if err := <-terminoB; err != nil {
		t.Fatalf("terminal B: %v (esperó más que el busy_timeout: la transacción de A no se soltó a tiempo)", err)
	}

	// Las dos sumas están, y se leen igual desde los dos motores.
	for nombre, m := range map[string]*DbEngine{"A": a, "B": b} {
		la, _ := m.LedgerStatusDe("terminal-a")
		lb, _ := m.LedgerStatusDe("terminal-b")
		if la.Total != 7 || lb.Total != 5 {
			t.Errorf("leído desde el motor %s: terminal-a=%d (quería 7) y terminal-b=%d (quería 5): una "+
				"terminal pisó la cuenta de la otra", nombre, la.Total, lb.Total)
		}
	}
}
