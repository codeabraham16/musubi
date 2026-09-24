package memory

import (
	"encoding/json"
	"fmt"
	"testing"
)

// ledger_revision_test.go custodia lo que encontró la revisión adversarial del formato por sesión,
// antes del merge. Cada prueba nombra el hallazgo; varias son las del revisor, adaptadas.

// ledgerAddDelBinarioViejo es una copia LITERAL del LedgerAdd anterior al formato por sesión: el que
// sigue corriendo en un servidor MCP largo después de instalar el binario nuevo, hasta que se
// reinicia su sesión. Escribe su casilla de siempre, `token_ledger`.
func ledgerAddDelBinarioViejo(e *DbEngine, sessionID, surface string, tokens int) {
	l := TokenLedger{Surfaces: map[string]int{}}
	if v, ok, _ := e.GetMeta(metaTokenLedger); ok && v != "" {
		if err := json.Unmarshal([]byte(v), &l); err != nil {
			l = TokenLedger{Surfaces: map[string]int{}}
		}
	}
	if l.Surfaces == nil {
		l.Surfaces = map[string]int{}
	}
	if sessionID != "" && sessionID != l.SessionID {
		l = TokenLedger{SessionID: sessionID, Surfaces: map[string]int{}}
	}
	if tokens > 0 {
		l.Total += tokens
		if surface != "" {
			l.Surfaces[surface] += tokens
		}
	}
	data, _ := json.Marshal(l)
	_ = e.SetMeta(metaTokenLedger, string(data))
}

// HALLAZGO: con las dos versiones escribiendo la MISMA clave, un MCP viejo todavía vivo leía el
// formato nuevo como un ledger vacío, le sumaba lo suyo y lo pisaba: todas las sesiones en cero, en
// cada hidratación. Con la clave nueva, el viejo sólo pisa su propia casilla.
func TestLedgerUnBinarioViejoVivoNoBorraLasSesiones(t *testing.T) {
	e := newTestEngine(t)
	e.LedgerAdd("A", "turn_recall", 5000)
	e.LedgerAdd("B", "precheck_code", 3000)
	ledgerAddDelBinarioViejo(e, "", "hydration", 40) // musubi_memory_expand de un MCP arrancado antes
	ledgerAddDelBinarioViejo(e, "C", "turn_recall", 7)
	if a, _ := e.LedgerStatusDe("A"); a.Total != 5000 {
		t.Errorf("el binario viejo borró la sesión A: %d", a.Total)
	}
	if b, _ := e.LedgerStatusDe("B"); b.Total != 3000 {
		t.Errorf("el binario viejo borró la sesión B: %d", b.Total)
	}
}

// Y el nuevo NUNCA escribe la casilla vieja: si la escribiera, el binario viejo leería otra cosa que
// la suya y la clave nueva no aislaría nada.
func TestLedgerElFormatoNuevoNoTocaLaCasillaVieja(t *testing.T) {
	e := newTestEngine(t)
	vieja := `{"session_id":"vieja","total":262,"surfaces":{"turn_recall":262}}`
	if err := e.SetMeta(metaTokenLedger, vieja); err != nil {
		t.Fatal(err)
	}
	e.LedgerAdd("nueva", "turn_recall", 10)
	if got, _, _ := e.GetMeta(metaTokenLedger); got != vieja {
		t.Fatalf("el formato nuevo escribió la casilla vieja:\n%s", got)
	}
	// La migración igual llevó la sesión vieja al formato nuevo.
	if l, _ := e.LedgerStatusDe("vieja"); l.Total != 262 {
		t.Fatalf("la sesión de la casilla vieja tenía que migrarse: %+v", l)
	}
}

// HALLAZGO: con desalojo por antigüedad, la terminal principal que lanza un workflow de sub-agentes
// queda quieta mientras ellos escriben con sus ids, y es la más vieja justo cuando más gastó. Se
// desaloja por MENOR total. Son 70 sub-agentes para pasar el tope y ejercitar el desalojo de verdad:
// con 16, como en la prueba del revisor, pasaría sólo por el tope más alto.
func TestLedgerDesalojoProtegeLaSesionGrande(t *testing.T) {
	e := newTestEngine(t)
	e.LedgerAdd("principal", "turn_recall", 5000)
	for i := 0; i < maxSesionesEnLedger+6; i++ {
		e.LedgerAdd(fmt.Sprintf("sub-%02d", i), "precheck_code", 50)
	}
	if a, _ := e.LedgerStatusDe("principal"); a.Total != 5000 {
		t.Fatalf("la sesión principal fue desalojada mientras esperaba a sus sub-agentes: %d", a.Total)
	}
	st, _ := e.loadLedgerStore()
	if len(st.Sesiones) != maxSesionesEnLedger {
		t.Fatalf("tope de %d; quedaron %d", maxSesionesEnLedger, len(st.Sesiones))
	}
}

// La recién escrita nunca se desaloja, aunque sea la más chica: si no, la escritura que crea una
// sesión nueva con el tope lleno la borraría en el mismo acto.
func TestLedgerLaSesionRecienEscritaNoSeDesaloja(t *testing.T) {
	e := newTestEngine(t)
	for i := 0; i < maxSesionesEnLedger; i++ {
		e.LedgerAdd(fmt.Sprintf("grande-%02d", i), "turn_recall", 1000+i)
	}
	e.LedgerAdd("recien-nacida", "turn_recall", 1)
	if l, _ := e.LedgerStatusDe("recien-nacida"); l.Total != 1 {
		t.Fatalf("la sesión recién escrita fue desalojada por su propia escritura: %+v", l)
	}
	if l, _ := e.LedgerStatusDe("grande-00"); l.Total != 0 {
		t.Fatalf("la desalojada tenía que ser la más chica de las otras (grande-00), sigue con %d", l.Total)
	}
}

// HALLAZGO: preguntar escribía. precheck lee su marca una-vez-por-sesión con LedgerAdd(…, 0), y en
// la primera versión esa pregunta creaba la sesión y la subía en el orden: una sesión que sólo leía
// ocupaba un lugar y empujaba a otras al desalojo.
func TestLedgerLeerConCeroTokensNoEscribe(t *testing.T) {
	e := newTestEngine(t)
	e.LedgerAdd("A", "turn_recall", 10)
	antes, _, _ := e.GetMeta(metaTokenLedgerV2)
	if _, err := e.LedgerAdd("solo-pregunta", "precheck_sin_grafo", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := e.LedgerAdd("A", "precheck_sin_grafo", 0); err != nil {
		t.Fatal(err)
	}
	if despues, _, _ := e.GetMeta(metaTokenLedgerV2); despues != antes {
		t.Fatalf("una lectura con 0 tokens escribió el ledger:\nantes   %s\ndespués %s", antes, despues)
	}
}

// HALLAZGO: el reset vaciaba el valor entero, o sea que una terminal borraba la cuenta de todas —el
// mismo defecto que el formato por sesión vino a cerrar—. Ahora pone en cero una sola: la indicada, o
// sin id la escrita más recientemente.
func TestLedgerResetPoneEnCeroUnaSola(t *testing.T) {
	e := newTestEngine(t)
	e.LedgerAdd("A", "turn_recall", 100)
	e.LedgerAdd("B", "precheck_code", 900)
	if err := e.LedgerReset("A"); err != nil {
		t.Fatal(err)
	}
	if a, _ := e.LedgerStatusDe("A"); a.Total != 0 {
		t.Errorf("reset de A tenía que dejarla en cero: %d", a.Total)
	}
	if b, _ := e.LedgerStatusDe("B"); b.Total != 900 {
		t.Errorf("reset de A borró también a B: %d", b.Total)
	}
	if err := e.LedgerReset(""); err != nil { // sin id: la última que escribió, que es B
		t.Fatal(err)
	}
	if b, _ := e.LedgerStatusDe("B"); b.Total != 0 {
		t.Errorf("reset sin id tenía que poner en cero la última escrita (B): %d", b.Total)
	}
}

// HALLAZGO: en sólo lectura un llamado sin id devolvía la clave literal "" (ceros) y en el camino
// escribible la última sesión. La guarda existe para que el caller no tenga que distinguir el modo.
func TestLedgerSoloLecturaResuelveIgualQueEscribible(t *testing.T) {
	e := newTestEngine(t)
	e.LedgerAdd("A", "precheck_sin_grafo", 9)
	escribible, _ := e.LedgerAdd("", "precheck_sin_grafo", 0)
	e.soloLectura = true
	t.Cleanup(func() { e.soloLectura = false })
	lectura, _ := e.LedgerAdd("", "precheck_sin_grafo", 0)
	if escribible.SessionID != "A" || lectura.SessionID != escribible.SessionID || lectura.Total != escribible.Total {
		t.Fatalf("los dos modos tienen que resolver igual: escribible %+v, sólo lectura %+v", escribible, lectura)
	}
}

// LedgerSesiones es lo que deja a musubi_tokens decir de quién es cada número.
func TestLedgerSesionesListaTodasDeLaMasRecienteALaMasVieja(t *testing.T) {
	e := newTestEngine(t)
	e.LedgerAdd("A", "turn_recall", 100)
	e.LedgerAdd("B", "precheck_code", 9000)
	e.LedgerAdd("A", "turn_recall", 1)
	ss, err := e.LedgerSesiones()
	if err != nil {
		t.Fatal(err)
	}
	if len(ss) != 2 || ss[0].SessionID != "A" || ss[0].Total != 101 || ss[1].SessionID != "B" || ss[1].Total != 9000 {
		t.Fatalf("quería [A=101, B=9000] de la más reciente a la más vieja; obtuve %+v", ss)
	}
}
