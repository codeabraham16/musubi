package memory

import (
	"strings"
	"testing"
)

// Tests del check/repair `llm_corroboradas_locales`: el backfill que promueve en lote las
// observaciones escritas por un LLM que YA fueron corroboradas (salieron de cuarentena) pero
// quedaron scope='local' sin viajar — el backlog de antes de que corroborar promoviera solo.

// llmCorroboradaLocal deja una observación en el estado EXACTO del backlog: provenance 'llm:*',
// quarantined=0 (corroborada) y scope='local'. Usa el método base CorroborateObservation, que sólo
// saca de cuarentena (la promoción H1 vive en una capa superior), así que reproduce el estado legacy
// sin un INSERT a mano.
func llmCorroboradaLocal(t *testing.T, e *DbEngine, topic, content string) string {
	t.Helper()
	id := proponerObs(t, e, topic, content)
	if err := e.CorroborateObservation(id); err != nil {
		t.Fatalf("CorroborateObservation(%s): %v", id, err)
	}
	return id
}

func scopeDe(t *testing.T, e *DbEngine, id string) string {
	t.Helper()
	var s string
	if err := e.db.QueryRow(`SELECT scope FROM observations WHERE id=?`, id).Scan(&s); err != nil {
		t.Fatalf("leer scope de %s: %v", id, err)
	}
	return s
}

func outboxDe(t *testing.T, e *DbEngine, id string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE obs_id=?`, id).Scan(&n); err != nil {
		t.Fatalf("contar outbox de %s: %v", id, err)
	}
	return n
}

func contentDe(t *testing.T, e *DbEngine, id string) string {
	t.Helper()
	var c string
	if err := e.db.QueryRow(`SELECT content FROM observations WHERE id=?`, id).Scan(&c); err != nil {
		t.Fatalf("leer content de %s: %v", id, err)
	}
	return c
}

// El conteo mide EXACTAMENTE el conjunto (llm + visible + local), y el apply promueve + encola; una
// 2ª corrida es no-op. El control humano y el de cuarentena quedan FUERA: si el predicado se
// relajara, el conteo dejaría de ser 2 y este test caería.
func TestLlmCorroboradasLocalesCuentaPromueveYEsIdempotente(t *testing.T) {
	e := newTestEngine(t)
	e.SetOutboxEnabled(true) // simula un binario con destino de sync configurado

	a := llmCorroboradaLocal(t, e, "t/a", "nota llm corroborada A")
	b := llmCorroboradaLocal(t, e, "t/b", "nota llm corroborada B")
	// Controles que NO deben contarse ni promoverse:
	if err := e.SaveObservation("humana", "t/h", "nota humana local", nil); err != nil {
		t.Fatalf("SaveObservation humana: %v", err)
	}
	enCuarentena := proponerObs(t, e, "t/q", "propuesta llm sin corroborar")

	// Estado inicial de las candidatas = el backlog real.
	for _, id := range []string{a, b} {
		if sc := scopeDe(t, e, id); sc != ScopeLocal {
			t.Fatalf("precondición: %s debía nacer local, está %q (¿corroborar promueve en el método base?)", id, sc)
		}
		prov, _, quar, err := e.ObservationStamp(id)
		if err != nil {
			t.Fatalf("ObservationStamp(%s): %v", id, err)
		}
		if quar || !strings.HasPrefix(prov, provenanceLLMPrefix) {
			t.Fatalf("precondición: %s debía ser llm+visible, prov=%q quar=%v", id, prov, quar)
		}
	}

	if n, err := countLlmCorroboradasLocales(e); err != nil || n != 2 {
		t.Fatalf("count = %d, err = %v; esperaba 2 (sólo a y b)", n, err)
	}

	// dry-run reporta 2 y NO muta.
	dry, err := e.Repair("llm_corroboradas_locales", "dry-run")
	if err != nil {
		t.Fatalf("Repair dry-run: %v", err)
	}
	if dry.Applied || dry.Affected != 2 {
		t.Fatalf("dry-run: Applied=%v Affected=%d; esperaba no-aplicado y 2", dry.Applied, dry.Affected)
	}
	if scopeDe(t, e, a) != ScopeLocal {
		t.Fatalf("dry-run no debió promover nada")
	}

	// apply promueve las 2, con backup.
	res, err := e.Repair("llm_corroboradas_locales", "apply")
	if err != nil {
		t.Fatalf("Repair apply: %v", err)
	}
	if res.Affected != 2 || !res.Applied {
		t.Fatalf("apply: Affected=%d Applied=%v; esperaba 2 y true", res.Affected, res.Applied)
	}
	if res.BackupPath == "" {
		t.Fatalf("apply debe respaldar la base y reportar la ruta")
	}

	for _, id := range []string{a, b} {
		if sc := scopeDe(t, e, id); sc != ScopeShared {
			t.Errorf("%s debía quedar shared, está %q", id, sc)
		}
		if outboxDe(t, e, id) != 1 {
			t.Errorf("%s debía tener 1 fila de outbox, tiene %d", id, outboxDe(t, e, id))
		}
		// El sello se conserva (T9): promover NO lava la procedencia.
		if prov, _, _, _ := e.ObservationStamp(id); !strings.HasPrefix(prov, provenanceLLMPrefix) {
			t.Errorf("%s perdió el sello llm al promover: prov=%q", id, prov)
		}
	}

	// Los controles quedaron intactos.
	if scopeDe(t, e, "humana") != ScopeLocal || outboxDe(t, e, "humana") != 0 {
		t.Errorf("la humana no debía promoverse/encolarse")
	}
	if _, _, quar, _ := e.ObservationStamp(enCuarentena); !quar {
		t.Errorf("la propuesta en cuarentena no debía salir de cuarentena")
	}
	if scopeDe(t, e, enCuarentena) != ScopeLocal || outboxDe(t, e, enCuarentena) != 0 {
		t.Errorf("la propuesta en cuarentena no debía promoverse/encolarse")
	}

	// Idempotente: 2ª corrida no toma nada ni crea outbox nuevo. Se llama la función directo (no vía
	// Repair) a propósito: Repair hace un backup con nombre a resolución de SEGUNDOS, y dos apply en
	// el mismo segundo chocarían el backup (gotcha del mecanismo de backup, ajeno a la idempotencia
	// que este bloque prueba, que es propiedad de applyPromoteLlmCorroboradasLocales).
	var outboxAntes int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM outbox`).Scan(&outboxAntes); err != nil {
		t.Fatal(err)
	}
	n2, err := applyPromoteLlmCorroboradasLocales(e)
	if err != nil {
		t.Fatalf("apply (2ª): %v", err)
	}
	if n2 != 0 {
		t.Errorf("2ª corrida: promovió %d; esperaba 0 (idempotente)", n2)
	}
	var outboxDespues int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM outbox`).Scan(&outboxDespues); err != nil {
		t.Fatal(err)
	}
	if outboxDespues != outboxAntes {
		t.Errorf("la 2ª corrida creó filas de outbox nuevas: %d -> %d", outboxAntes, outboxDespues)
	}
}

// El predicado exige visibilidad: una llm corroborada-local pero descartada (archived=1) o
// reemplazada (superseded_by) NO se cuenta ni se promueve.
func TestLlmCorroboradasLocalesNoTocaInvisibles(t *testing.T) {
	e := newTestEngine(t)
	e.SetOutboxEnabled(true)

	viva := llmCorroboradaLocal(t, e, "t/viva", "llm viva")
	descartada := llmCorroboradaLocal(t, e, "t/desc", "llm descartada")
	reemplazada := llmCorroboradaLocal(t, e, "t/reemp", "llm reemplazada")
	if _, err := e.db.Exec(`UPDATE observations SET archived=1 WHERE id=?`, descartada); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET superseded_by=? WHERE id=?`, viva, reemplazada); err != nil {
		t.Fatal(err)
	}

	if n, err := countLlmCorroboradasLocales(e); err != nil || n != 1 {
		t.Fatalf("count = %d, err = %v; esperaba 1 (sólo la viva)", n, err)
	}
	if _, err := e.Repair("llm_corroboradas_locales", "apply"); err != nil {
		t.Fatalf("Repair apply: %v", err)
	}
	if scopeDe(t, e, viva) != ScopeShared {
		t.Errorf("la viva debía promoverse")
	}
	if scopeDe(t, e, descartada) != ScopeLocal || outboxDe(t, e, descartada) != 0 {
		t.Errorf("una descartada (archived=1) no debía promoverse/encolarse")
	}
	if scopeDe(t, e, reemplazada) != ScopeLocal || outboxDe(t, e, reemplazada) != 0 {
		t.Errorf("una reemplazada (superseded_by) no debía promoverse/encolarse")
	}
}

// Al cruzar a 'shared' el contenido se redacta (C2): el secreto no viaja. Lo hace PromoteObservation,
// que el apply reusa — este test sella que el backfill NO abre un bypass de la redacción.
func TestLlmCorroboradasLocalesRedactaAlCruzar(t *testing.T) {
	e := newTestEngine(t)
	e.SetOutboxEnabled(true)

	const secreto = "AKIA1234567890ABCDEF" // fixture falso; redact lo reconoce como aws-access-key
	id := llmCorroboradaLocal(t, e, "t/sec", "credenciales de prueba: "+secreto+" fin")

	if _, err := e.Repair("llm_corroboradas_locales", "apply"); err != nil {
		t.Fatalf("Repair apply: %v", err)
	}
	got := contentDe(t, e, id)
	if strings.Contains(got, secreto) {
		t.Errorf("el secreto crudo cruzó a shared sin redactar: %q", got)
	}
	if !strings.Contains(got, "[REDACTED:") {
		t.Errorf("el contenido promovido debía quedar redactado, obtuve %q", got)
	}
}

// El `run` del check: ok cuando no hay backlog, warning+Repairable con el remedio cuando lo hay.
func TestCheckLlmCorroboradasLocalesReportaWarningYOk(t *testing.T) {
	e := newTestEngine(t)

	// Sin backlog: ok, no reparable-como-pendiente.
	ok := checkLlmCorroboradasLocales(e)
	if ok.Status != "ok" {
		t.Fatalf("base vacía: Status=%q; esperaba ok", ok.Status)
	}

	llmCorroboradaLocal(t, e, "t/a", "una llm corroborada local")
	warn := checkLlmCorroboradasLocales(e)
	if warn.Status != "warning" || !warn.Repairable {
		t.Fatalf("con backlog: Status=%q Repairable=%v; esperaba warning+reparable", warn.Status, warn.Repairable)
	}
	if !strings.Contains(warn.Message, "--apply") {
		t.Errorf("el Message debe nombrar el remedio (--apply), obtuve %q", warn.Message)
	}
	if warn.Detail != "" {
		t.Errorf("el check no debe emitir Detail (nombres de filas); obtuve %q", warn.Detail)
	}
}

// D7: sin destino de sync (outbox deshabilitado) la promoción igual pasa a 'shared' SIN encolar; el
// outbox lo siembra BackfillOutbox cuando la máquina gane destino. Y sigue siendo idempotente.
func TestLlmCorroboradasLocalesSinDestinoPromueveSinEncolar(t *testing.T) {
	e := newTestEngine(t)
	e.SetOutboxEnabled(false) // sin destino configurado

	id := llmCorroboradaLocal(t, e, "t/a", "llm sin destino")
	n, err := applyPromoteLlmCorroboradasLocales(e)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if n != 1 {
		t.Fatalf("promovió %d; esperaba 1", n)
	}
	if scopeDe(t, e, id) != ScopeShared {
		t.Errorf("sin destino igual debe promover a shared, está %q", scopeDe(t, e, id))
	}
	if outboxDe(t, e, id) != 0 {
		t.Errorf("sin destino NO debe encolar; outbox=%d", outboxDe(t, e, id))
	}
	// Idempotente: ya es shared, no vuelve a tomarse.
	if n2, _ := applyPromoteLlmCorroboradasLocales(e); n2 != 0 {
		t.Errorf("2ª corrida promovió %d; esperaba 0", n2)
	}
}

// Guarda estructural: el check NUNCA debe auto-curarse. Promover empuja al cerebro central, y eso va
// con --apply explícito (regla de oro), nunca en el AutoHeal sin supervisión.
func TestLlmCorroboradasLocalesFueraDelAutoHeal(t *testing.T) {
	if autoHealCodes["llm_corroboradas_locales"] {
		t.Fatal("llm_corroboradas_locales NO debe estar en autoHealCodes: el empuje al central exige --apply explícito")
	}
}
