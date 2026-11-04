package memory

// Federación de specs (spec-graph F4) — los invariantes del MOTOR, en su sitio de declaración.
// ProjectSpecShared / RetractSpecProjection / ReconcileSpecProjections reusan el carril de
// observaciones (outbox, dead-letter, backfill) sin un segundo mecanismo de sync (R5). Acá se fijan el
// contrato de la fila proyectada (R4), la idempotencia por id (R6), el retiro local durable (R9), la
// reconciliación que respeta el espejo de otra máquina (R10/E11) y el nodo terminal + backfill (R11/E5).
// Cada ancla «Sabotaje» lleva su directiva `// arnes:` mecanizada; los tests de comportamiento (eco) no.

import "testing"

// contarObsProj cuenta las filas de observations con un id dado (0 o 1 para un id determinista de
// proyección). Es el detector de «se borró» / «se duplicó» / «se sembró» en los tests de abajo.
func contarObsProj(t *testing.T, e *DbEngine, id string) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE id = ?`, id).Scan(&n); err != nil {
		t.Fatalf("contar observación %q: %v", id, err)
	}
	return n
}

// TestProjectSpecSharedContrato fija R4/D4: la proyección es una obs con topic_key `spec/<id>`, scope
// `shared`, mem_type `semantic`, procedencia `human` y confidence 1.0, y en un cliente ENCOLA. El sello
// `human` (no `llm:*`) es deliberado (D4): una proyección marcada `llm:*` iría a cuarentena y no se
// promovería nunca a shared, así que no federaría jamás.
//
// Sabotaje que lo pone en ROJO: cambiar el topic_key de la proyección (`spec/` → `spn/`); el assert del
// topic_key lo detecta. Es el único sitio donde se decide el topic_key (R4).
// arnes: archivo="internal/memory/spec_projection.go"
// arnes: de="return \"spec/\" + specID"
// arnes: a="return \"spn/\" + specID"
func TestProjectSpecSharedContrato(t *testing.T) {
	e := newTestEngine(t)
	if err := e.ProjectSpecShared("s-fed", "", "contenido federable de la spec", nil); err != nil {
		t.Fatal(err)
	}
	id := specProjectionID(e.effectiveProjectID(""), "s-fed")

	var topicKey, scope, memType, provenance string
	var confidence float64
	if err := e.db.QueryRow(
		`SELECT topic_key, scope, mem_type, provenance, confidence FROM observations WHERE id = ?`, id,
	).Scan(&topicKey, &scope, &memType, &provenance, &confidence); err != nil {
		t.Fatalf("la proyección no se guardó: %v", err)
	}
	if topicKey != "spec/s-fed" {
		t.Errorf("topic_key = %q, esperaba 'spec/s-fed' (R4)", topicKey)
	}
	if scope != ScopeShared {
		t.Errorf("scope = %q, esperaba 'shared'", scope)
	}
	if memType != "semantic" {
		t.Errorf("mem_type = %q, esperaba 'semantic'", memType)
	}
	if provenance != "human" {
		t.Errorf("provenance = %q, esperaba 'human' (D4: un sello llm iría a cuarentena y no federaría)", provenance)
	}
	if confidence != 1.0 {
		t.Errorf("confidence = %v, esperaba 1.0", confidence)
	}
	if n := pendientesOutbox(t, e); n != 1 {
		t.Errorf("un cliente debe encolar la proyección: esperaba 1 fila, hay %d", n)
	}
}

// TestProjectSpecSharedUpsertReemplazaNoDuplica fija R6: un update de la misma spec REEMPLAZA la
// proyección por su id determinista en vez de acumular duplicados. La idempotencia es lo que hace
// seguro re-proyectar en cada create/update/reconcile sin inflar el central.
//
// Sabotaje que lo pone en ROJO: que el DO UPDATE SET conserve el content viejo (`excluded.content` →
// `content`); el conteo sigue en 1 (upsert por id), pero el content queda en el viejo y el assert falla.
// arnes: archivo="internal/memory/spec_projection.go"
// arnes: de="content=excluded.content,"
// arnes: a="content=content,"
func TestProjectSpecSharedUpsertReemplazaNoDuplica(t *testing.T) {
	e := newTestEngine(t)
	if err := e.ProjectSpecShared("s-upd", "", "contenido viejo", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.ProjectSpecShared("s-upd", "", "contenido nuevo", nil); err != nil {
		t.Fatal(err)
	}
	id := specProjectionID(e.effectiveProjectID(""), "s-upd")
	if n := contarObsProj(t, e, id); n != 1 {
		t.Errorf("un update de la misma spec debía REEMPLAZAR por id, no duplicar: hay %d filas", n)
	}
	var content string
	if err := e.db.QueryRow(`SELECT content FROM observations WHERE id = ?`, id).Scan(&content); err != nil {
		t.Fatal(err)
	}
	if content != "contenido nuevo" {
		t.Errorf("tras el update el content debía ser 'contenido nuevo', es %q", content)
	}
}

// TestRetractSpecProjectionBorraTodo fija R9: retirar una proyección hard-deletea la obs Y su fila de
// outbox (y el embedding), para que no quede un vector colgado ni una fila de outbox que el loop de
// sync intente empujar para siempre. Lo llama el hook en unshare/delete/cambio-de-id.
//
// Sabotaje que lo pone en ROJO: que el DELETE de la obs no borre nada (`WHERE id = ?` → `... AND 0=1`);
// la obs sobrevive al retiro y el assert del conteo==0 falla.
// arnes: archivo="internal/memory/spec_projection.go"
// arnes: de="DELETE FROM observations WHERE id = ?"
// arnes: a="DELETE FROM observations WHERE id = ? AND 0=1"
func TestRetractSpecProjectionBorraTodo(t *testing.T) {
	e := newTestEngine(t)
	if err := e.ProjectSpecShared("s-ret", "", "contenido a retirar", nil); err != nil {
		t.Fatal(err)
	}
	id := specProjectionID(e.effectiveProjectID(""), "s-ret")
	if n := contarObsProj(t, e, id); n != 1 {
		t.Fatalf("precondición: la proyección debía existir, hay %d filas", n)
	}
	if n := pendientesOutbox(t, e); n != 1 {
		t.Fatalf("precondición: la proyección debía encolar, hay %d filas", n)
	}

	if err := e.RetractSpecProjection("s-ret", ""); err != nil {
		t.Fatal(err)
	}
	if n := contarObsProj(t, e, id); n != 0 {
		t.Errorf("retirar debía hard-deletear la obs, hay %d filas (R9)", n)
	}
	if n := pendientesOutbox(t, e); n != 0 {
		t.Errorf("retirar debía borrar la fila de outbox, hay %d filas (R9)", n)
	}
}

// TestReconcileSpecProjectionsEspejoSobrevive prueba DEFENSA 3 (el sello) aislada: el reconciliador hace
// de las proyecciones de ORIGEN LOCAL una función pura de las specs `shared` en disco —siembra las que
// faltan, hard-deletea las huérfanas locales— pero DEJA INTACTA una proyección `espejo` del MISMO tenant
// (un re-onboarding desde cursor 0 re-ingiere mis propias proyecciones viejas como espejo, E4). Ésa cae
// DENTRO del namespace propio, así que DEFENSA 1 no la cubre: lo único que la salva es el SELLO. Borrarla
// sería un retiro mesh-wide (R10b), fuera de F4.
//
// Sabotaje que lo pone en ROJO: reconciliar SIN filtrar por `espejo` (`status != outboxEspejo` →
// `... || true`); entonces la espejo entra a la poda y se borra, y el assert de que sobrevive falla.
// Ataca exactamente el filtro que protege el mirror del mismo tenant (DEFENSA 3/E11).
// arnes: archivo="internal/memory/spec_projection.go"
// arnes: de="if status != outboxEspejo {"
// arnes: a="if status != outboxEspejo || true {"
func TestReconcileSpecProjectionsEspejoSobrevive(t *testing.T) {
	e := newTestEngine(t)

	// Proyección de ORIGEN LOCAL cuya spec fuente ya NO está en el FS declarado (huérfana): debe borrarse.
	if err := e.ProjectSpecShared("s-huerfana", "", "contenido huerfana", nil); err != nil {
		t.Fatal(err)
	}
	// Proyección de ORIGEN LOCAL justificada por el FS (sigue shared): se conserva.
	if err := e.ProjectSpecShared("s-viva", "", "contenido viva", nil); err != nil {
		t.Fatal(err)
	}
	// Proyección ESPEJO del MISMO tenant: un re-onboarding desde cursor 0 re-ingiere mis propias
	// proyecciones viejas como espejo (E4), con MI project_id, así que CAE DENTRO del namespace propio y
	// DEFENSA 1 no la excluye. Su spec fuente ya NO está en mi FS (no entra en expected), de modo que lo
	// ÚNICO que la salva del barrido es el SELLO (DEFENSA 3). IngestShared la marca 'espejo' (anti-loop).
	espejoID := specProjectionID(e.effectiveProjectID(""), "s-espejo")
	if _, err := e.IngestShared(SharedObs{
		ID:        espejoID,
		TopicKey:  specProjectionTopicKey("s-espejo"),
		Content:   "proyeccion propia re-ingerida como espejo",
		MemType:   "semantic",
		ProjectID: e.effectiveProjectID(""),
	}); err != nil {
		t.Fatal(err)
	}
	var status string
	if err := e.db.QueryRow(`SELECT status FROM outbox WHERE obs_id = ?`, espejoID).Scan(&status); err != nil {
		t.Fatalf("la espejo no tiene fila de outbox: %v", err)
	}
	if status != outboxEspejo {
		t.Fatalf("precondición: la espejo quedó status %q, esperaba %q", status, outboxEspejo)
	}

	// El FS declara s-viva (ya proyectada) y s-nueva (falta sembrar). NO declara s-huerfana ni s-espejo.
	if err := e.ReconcileSpecProjections([]SpecProjectionInput{
		{SpecID: "s-viva", Content: "contenido viva"},
		{SpecID: "s-nueva", Content: "contenido nueva"},
	}); err != nil {
		t.Fatal(err)
	}

	if n := contarObsProj(t, e, specProjectionID(e.effectiveProjectID(""), "s-huerfana")); n != 0 {
		t.Errorf("la huérfana de origen local debía borrarse al reconciliar, hay %d filas", n)
	}
	if n := contarObsProj(t, e, specProjectionID(e.effectiveProjectID(""), "s-viva")); n != 1 {
		t.Errorf("la viva (justificada por el FS) no debía borrarse, hay %d filas", n)
	}
	if n := contarObsProj(t, e, specProjectionID(e.effectiveProjectID(""), "s-nueva")); n != 1 {
		t.Errorf("la nueva debía sembrarse al reconciliar, hay %d filas", n)
	}
	// EL INVARIANTE (E11/R10b): la espejo del mismo tenant (re-ingerida) SOBREVIVE — la protege el sello.
	if n := contarObsProj(t, e, espejoID); n != 1 {
		t.Errorf("la espejo re-ingerida NO debía borrarse (E11/R10b, rompe la federación), hay %d filas", n)
	}
	// Y sigue siendo 'espejo' (el reconciliador no la tocó).
	if err := e.db.QueryRow(`SELECT status FROM outbox WHERE obs_id = ?`, espejoID).Scan(&status); err != nil {
		t.Fatalf("la espejo perdió su fila de outbox: %v", err)
	}
	if status != outboxEspejo {
		t.Errorf("la espejo cambió a status %q, debía seguir 'espejo'", status)
	}
}

// TestReconcileSpecProjectionsPushAjenoSobreviveEnCentralEncadenado prueba DEFENSA 1 (el namespace del
// tenant) aislada: un central ENCADENADO (con destino de sync, e.outboxEnabled=true) recibe por push una
// proyección de OTRO tenant. Como el nodo tiene destino, el push la encola 'pending' (no 'espejo'), así que
// el filtro del sello NO la protegería; la salva que su id lleva el tenant AJENO y cae FUERA del barrido del
// tenant propio. Sin DEFENSA 1 el central encadenado borra lo que le empujan en cada arranque (medido por
// integración, caso 2). El FS propio no tiene esa spec: expected vacío, como en un central.
//
// Sabotaje que lo pone en ROJO: enumerar global en vez del tenant propio (quitarle el tenant a `ownPrefix`);
// la fila ajena entra al barrido, no está en expected, su sello 'pending' != espejo ⇒ se borra.
// arnes: archivo="internal/memory/spec_projection.go"
// arnes: de="ownPrefix := specProjectionIDPrefix + e.effectiveProjectID(\"\") + \":\""
// arnes: a="ownPrefix := specProjectionIDPrefix"
func TestReconcileSpecProjectionsPushAjenoSobreviveEnCentralEncadenado(t *testing.T) {
	e := newTestEngine(t) // con destino: outboxEnabled=true por default

	// Simular el push de otra máquina: saveObservation con el id determinista de proyección y un tenant
	// AJENO. Con outboxEnabled=true el camino del push encola 'pending', igual que un central encadenado.
	ajenoID := specProjectionID("proyecto-ajeno", "s-empujada")
	if err := e.saveObservation(ajenoID, specProjectionTopicKey("s-empujada"), "proyeccion empujada por otra maquina",
		1.0, true, "semantic", ScopeShared, "proyecto-ajeno", "nodo-ajeno", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if n := contarObsProj(t, e, ajenoID); n != 1 {
		t.Fatalf("precondición: la proyección empujada debía existir, hay %d filas", n)
	}

	// El FS propio NO declara esa spec (ni ninguna): expected vacío, como en un central.
	if err := e.ReconcileSpecProjections(nil); err != nil {
		t.Fatal(err)
	}

	// EL INVARIANTE (DEFENSA 1): una proyección de OTRO tenant no es del barrido propio y SOBREVIVE.
	if n := contarObsProj(t, e, ajenoID); n != 1 {
		t.Errorf("el reconcile borró una proyección de OTRO tenant empujada al central (hay %d filas); "+
			"DEFENSA 1 (namespace del tenant) debía dejarla intacta", n)
	}
}

// TestReconcileSpecProjectionsPushPropioSobreviveEnReceptorTerminal prueba DEFENSA 2 (el gate del receptor)
// aislada, bajo el PEOR caso: colisión de tenant. Un central TERMINAL (sin destino, e.outboxEnabled=false)
// recibe por push una proyección cuyo tenant COINCIDE con el propio (dos repos con el mismo project_id). La
// fila entra sin outbox y sin sello espejo, y su id cae DENTRO del namespace propio, así que ni DEFENSA 1 ni
// DEFENSA 3 la distinguen de una huérfana local. Lo único que la salva es que en un nodo sin destino el
// barrido NO corre. Sin el gate, el central borra en silencio lo empujado en CADA arranque de `serve`
// (medido: model-free + correctitud + integración, filas 1→0).
//
// Sabotaje que lo pone en ROJO: quitar el gate del receptor (`if e.outboxEnabled && len(orphansLocal) > 0`
// → `if len(orphansLocal) > 0`); el terminal barre la fila empujada del tenant propio y la borra.
// arnes: archivo="internal/memory/spec_projection.go"
// arnes: de="if e.outboxEnabled && len(orphansLocal) > 0 {"
// arnes: a="if len(orphansLocal) > 0 {"
func TestReconcileSpecProjectionsPushPropioSobreviveEnReceptorTerminal(t *testing.T) {
	e := newTestEngine(t)
	e.SetOutboxEnabled(false) // receptor terminal: el cerebro central, sin upstream

	// Simular el push con COLISIÓN de tenant: originProjectID "" ⇒ effectiveProjectID ⇒ el tenant PROPIO, el
	// mismo id que tendría una proyección local. Con outboxEnabled=false no se encola: la fila queda sin
	// outbox y sin sello espejo, idéntica a lo que recibe un central terminal.
	propioID := specProjectionID(e.effectiveProjectID(""), "s-empujada-colision")
	if err := e.saveObservation(propioID, specProjectionTopicKey("s-empujada-colision"), "proyeccion empujada, tenant colisiona",
		1.0, true, "semantic", ScopeShared, "", "nodo-ajeno", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if n := pendientesOutbox(t, e); n != 0 {
		t.Fatalf("precondición: el receptor terminal no debía encolar, hay %d filas", n)
	}

	// El FS propio NO declara esa spec: expected vacío, como en un central.
	if err := e.ReconcileSpecProjections(nil); err != nil {
		t.Fatal(err)
	}

	// EL INVARIANTE (DEFENSA 2): en un nodo sin destino el barrido no corre, y la fila empujada SOBREVIVE
	// aunque su tenant colisione con el propio.
	if n := contarObsProj(t, e, propioID); n != 1 {
		t.Errorf("el reconcile de un receptor terminal borró una proyección empujada (hay %d filas); "+
			"DEFENSA 2 (gate del receptor) debía saltear el barrido", n)
	}
}

// TestProjectSpecSharedTerminalMarcaYBackfillSiembra fija R11/E5: en un nodo TERMINAL (sin destino) la
// proyección se ESCRIBE (marca) pero NO se encola —gatear sólo el enqueue, no la escritura, es lo que
// le da algo que sembrar al backfill—; al ganar destino y reabrir, BackfillOutbox siembra la fila
// `pending`. Sin esto, o el terminal deja una fila inmortal, o la intención de federar se pierde.
//
// Sabotaje que lo pone en ROJO: encolar aunque el outbox esté apagado (`if e.outboxEnabled` →
// `... || true`); el terminal encola y el assert de 0 filas tras la marca falla.
// arnes: archivo="internal/memory/spec_projection.go"
// arnes: de="if e.outboxEnabled {"
// arnes: a="if e.outboxEnabled || true {"
func TestProjectSpecSharedTerminalMarcaYBackfillSiembra(t *testing.T) {
	e := newTestEngine(t)
	e.SetOutboxEnabled(false) // nodo terminal: sin upstream a dónde empujar

	if err := e.ProjectSpecShared("s-term", "", "contenido terminal", nil); err != nil {
		t.Fatal(err)
	}
	id := specProjectionID(e.effectiveProjectID(""), "s-term")
	if n := contarObsProj(t, e, id); n != 1 {
		t.Errorf("el nodo terminal debía ESCRIBIR la obs (marca), hay %d filas", n)
	}
	if n := pendientesOutbox(t, e); n != 0 {
		t.Errorf("el nodo terminal NO debía encolar, hay %d filas (R11)", n)
	}

	// Gana destino y reabre: el backfill siembra la proyección acumulada.
	e.SetOutboxEnabled(true)
	if _, err := e.BackfillOutbox(); err != nil {
		t.Fatal(err)
	}
	if n := pendientesOutbox(t, e); n != 1 {
		t.Errorf("tras ganar destino, el backfill debía sembrar la proyección: esperaba 1 fila, hay %d (E5)", n)
	}
}

// TestProyeccionBajadaNoReboteAlCentral es cobertura de comportamiento (R12/E6): una proyección que
// BAJÓ de otra máquina (sello 'espejo') no vuelve a subir por el carril de federación. No lleva ancla
// mecanizada porque el sabotaje del anti-eco vive en el carril compartido (inboundsync_eco_test.go); acá
// se fija que una proyección espejo específica no reaparezca en un claim ni siquiera tras un backfill.
func TestProyeccionBajadaNoReboteAlCentral(t *testing.T) {
	e := newTestEngine(t)
	espejoID := specProjectionID("otra-maquina", "s-bajada")
	if _, err := e.IngestShared(SharedObs{
		ID:        espejoID,
		TopicKey:  specProjectionTopicKey("s-bajada"),
		Content:   "proyeccion bajada de otra maquina",
		MemType:   "semantic",
		ProjectID: "otra-maquina",
	}); err != nil {
		t.Fatal(err)
	}
	// El backfill no reencola una fila que ya tiene outbox (la espejo lo tiene, en 'espejo').
	if _, err := e.BackfillOutbox(); err != nil {
		t.Fatal(err)
	}
	items, err := e.ClaimOutboxBatch(50, 60)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range items {
		if it.ObsID == espejoID {
			t.Errorf("ECO: la proyección espejo %q volvió a subir al central (R12/E6)", espejoID)
		}
	}
}
