package memory

import "testing"

// conflicts_spec_f5_test.go — invariantes de F5 (arbitraje spec↔observación). Cada test ataca UN
// invariante con un sabotaje AISLADO (adversarial-review §2): la defensa en profundidad (D4 sobre D2)
// taparía el sabotaje del otro guard, así que T7 (complementaryPair) NO toca markSuperseded y T8
// (markSuperseded) BYPASSEA complementaryPair llamando directo / insertando la relación a mano.
//
// Vive en su PROPIO archivo a propósito: conflicts_test.go (la suite obs↔obs preexistente) queda
// literalmente sin tocar — la forma más fuerte de R7/T11.

// supersededBy devuelve el superseded_by de una observación ("" si está visible). Para afirmar que una
// proyección NUNCA se oculta (R3).
func supersededBy(t *testing.T, e *DbEngine, id string) string {
	t.Helper()
	var sb string
	if err := e.db.QueryRow(`SELECT COALESCE(superseded_by,'') FROM observations WHERE id=?`, id).Scan(&sb); err != nil {
		t.Fatalf("leer superseded_by de %q: %v", id, err)
	}
	return sb
}

// proyectarSpec materializa la proyección de una spec `shared` (como F4a) y le fija un created_at
// EXPLÍCITO para que el orden "más nuevo" no dependa del reloj del test. Devuelve el id determinista.
func proyectarSpec(t *testing.T, e *DbEngine, specID, projectID, content, createdAt string) string {
	t.Helper()
	if err := e.ProjectSpecShared(specID, projectID, content, nil); err != nil {
		t.Fatalf("ProjectSpecShared(%q): %v", specID, err)
	}
	id := e.SpecProjectionObsID(specID, projectID)
	if _, err := e.db.Exec(`UPDATE observations SET created_at=? WHERE id=?`, createdAt, id); err != nil {
		t.Fatalf("fijar created_at de la proyección %q: %v", id, err)
	}
	return id
}

// T6 — R1/R2: al proyectar una spec, una creencia humana vieja que la contradice queda como relación
// `pending` con la proyección como SOURCE, y la creencia NO se oculta.
//
// Sabotaje que lo pone en rojo: quitar el bloque `if opts.DetectOnly { rel = pendingRel(...) }` de
// DetectRelations. El par está armado para que SIN DetectOnly auto-supersederia la creencia (lex≥auto ∧
// mismo topic ∧ source estrictamente más nuevo); el DetectOnly es lo único que lo frena a `pending`.
func TestF5ProyeccionDetectaCreenciaViejaYQuedaPending(t *testing.T) {
	e := newTestEngine(t)
	// MISMO topic que la proyección (mismo dominio): así el verde NO depende de la exención de
	// dominiosAjenos —eso lo aísla T9— sino sólo del DetectOnly que fuerza pending (R2).
	saveAt(t, e, "cre", "spec/s1", "Usamos PostgreSQL como base de datos principal del sistema.", "2026-01-01 10:00:00")
	proj := proyectarSpec(t, e, "s1", "", "Usamos PostgreSQL como base de datos principal del sistema productivo.", "2026-06-01 10:00:00")

	// DetectOnly=true: EXACTAMENTE como el seam real (conflictOpts(true) en detectSpecContradictions).
	rels, err := e.DetectRelations(proj, ConflictOptions{DetectOnly: true})
	if err != nil {
		t.Fatalf("DetectRelations: %v", err)
	}
	var r *ObsRelation
	for i := range rels {
		if rels[i].TargetID == "cre" {
			r = &rels[i]
		}
	}
	if r == nil {
		t.Fatalf("R1: debía nacer una relación proyección→creencia, obtuve %+v", rels)
	}
	if r.SourceID != proj {
		t.Errorf("R1: la proyección debe ser el SOURCE, obtuve source=%q", r.SourceID)
	}
	if r.Status != RelStatusPending {
		t.Errorf("R2: la relación debe nacer pending, obtuve status=%q (%+v)", r.Status, *r)
	}
	if sb := supersededBy(t, e, "cre"); sb != "" {
		t.Errorf("R2: la creencia NO debe quedar oculta, superseded_by=%q", sb)
	}
}

// T7 — R3 (nacimiento): una creencia NUNCA puede nacer como superseaderora de la proyección.
// complementaryPair protege a la proyección como TARGET, así que el par ni siquiera nace.
//
// Sabotaje que lo pone en rojo: quitar `|| specProjection(b)` de complementaryPair → el par procede y
// nace una relación con la proyección como target. Con DetectOnly=true queda en `pending` (no toca
// markSuperseded): el rojo cae limpio sobre la aserción de NACIMIENTO, aislando este guard de T4.
func TestF5CreenciaNoNaceComoSupersederaDeLaProyeccion(t *testing.T) {
	e := newTestEngine(t)
	// La creencia es MÁS NUEVA y casi idéntica, mismo topic: sin la protección, un save real la haría
	// auto-superseder la proyección (el bug latente que F5 cierra). Acá verificamos que no NACE siquiera.
	proj := proyectarSpec(t, e, "s1", "", "Usamos PostgreSQL como base de datos principal del sistema.", "2026-01-01 10:00:00")
	saveAt(t, e, "cre", "spec/s1", "Usamos PostgreSQL como base de datos principal del sistema productivo.", "2026-06-01 10:00:00")

	// La creencia como SOURCE (como en el save que dispara la detección). DetectOnly=true para que el
	// sabotaje caiga sobre el nacimiento y no sobre el error del guard de markSuperseded (aísla de T4).
	rels, err := e.DetectRelations("cre", ConflictOptions{DetectOnly: true})
	if err != nil {
		t.Fatalf("DetectRelations: %v", err)
	}
	for _, r := range rels {
		if r.TargetID == proj {
			t.Fatalf("R3: no debe nacer NINGUNA relación con la proyección como target, obtuve %+v", r)
		}
	}
	if sb := supersededBy(t, e, proj); sb != "" {
		t.Errorf("R3: la proyección NO debe quedar oculta, superseded_by=%q", sb)
	}
}

// T8a — R3 (efecto): markSuperseded RECHAZA ocultar una proyección, venga por donde venga. Se llama
// DIRECTO (bypassa complementaryPair) para aislar este guard de T2.
//
// Sabotaje que lo pone en rojo: quitar el guard de prefijo al entrar a markSuperseded → no devuelve
// error y la proyección queda oculta (superseded_by != NULL).
func TestF5MarkSupersededDirectoRechazaLaProyeccion(t *testing.T) {
	e := newTestEngine(t)
	proj := proyectarSpec(t, e, "s1", "", "Usamos PostgreSQL para persistencia.", "2026-01-01 10:00:00")
	saveAt(t, e, "cre", "spec/s1", "Usamos SQLite para persistencia.", "2026-06-01 10:00:00")

	if err := e.markSuperseded(proj, "cre"); err == nil {
		t.Errorf("R3: markSuperseded debe RECHAZAR con error ocultar una proyección, devolvió nil")
	}
	if sb := supersededBy(t, e, proj); sb != "" {
		t.Errorf("R3: la proyección NO debe quedar oculta, superseded_by=%q", sb)
	}
}

// T8b — R3 (efecto) + ORDEN en ResolveObsRelation: si markSuperseded rechaza, la relación debe quedar
// en `pending`, NO resuelta-sin-efecto. Se inserta la relación a mano (target=proyección) para
// bypassar complementaryPair y aislar este camino de T2.
//
// Dos sabotajes lo ponen en rojo:
//   - quitar el guard de markSuperseded → la proyección se oculta y la relación se resuelve.
//   - revertir el orden en ResolveObsRelation (UPDATE antes del efecto) → la relación queda `resolved`
//     aunque el efecto se rechace: resuelta-sin-efecto, justo el estado que el fix evita.
func TestF5ResolveObsRelationRechazaYDejaPending(t *testing.T) {
	e := newTestEngine(t)
	proj := proyectarSpec(t, e, "s1", "", "Usamos PostgreSQL para persistencia.", "2026-01-01 10:00:00")
	saveAt(t, e, "cre", "spec/s1", "Usamos SQLite para persistencia.", "2026-06-01 10:00:00")

	// Relación pendiente con la proyección como TARGET, insertada directamente (sin pasar por la
	// detección, que la bloquearía): modela una relación nacida por otra vía (datos legados F4a).
	rel, err := upsertObsRelationCon(e.db, ObsRelation{
		SourceID:   "cre",
		TargetID:   proj,
		Relation:   RelPending,
		Status:     RelStatusPending,
		Confidence: 0.5,
	})
	if err != nil {
		t.Fatalf("insertar relación pendiente: %v", err)
	}

	if err := e.ResolveObsRelation(rel.ID, RelSupersedes, "test", "arbitrado a supersede"); err == nil {
		t.Errorf("R3: ResolveObsRelation debe propagar el rechazo de markSuperseded, devolvió nil")
	}
	if sb := supersededBy(t, e, proj); sb != "" {
		t.Errorf("R3: la proyección NO debe quedar oculta, superseded_by=%q", sb)
	}
	// El corazón del fix de orden: la relación quedó SIN efecto, así que debe seguir `pending` —no
	// `resolved`—, para que el agente la vuelva a ver y actualice el SPEC.md en vez de la creencia.
	var status string
	if err := e.db.QueryRow(`SELECT status FROM observation_relations WHERE id=?`, rel.ID).Scan(&status); err != nil {
		t.Fatalf("leer status de la relación: %v", err)
	}
	if status != RelStatusPending {
		t.Errorf("orden: un supersede rechazado debe dejar la relación en pending, obtuve status=%q", status)
	}
}

// T9 — R4: la spec cruza dominios. Una proyección (dominio `spec`) contra una creencia de OTRO dominio
// no se descarta por dominiosAjenos. Es el caso REAL: la spec de `auth` contradice una nota de `deploy`.
//
// Sabotaje que lo pone en rojo: quitar `specProjection(r)` de exentoDeDominios → el par cross-domain se
// descarta (ninguno de los dos exento) → no nace relación.
func TestF5ProyeccionCruzaDominios(t *testing.T) {
	e := newTestEngine(t)
	// Creencia de dominio `deploy`, contradictoria (PostgreSQL vs MySQL) y con ALTO solape léxico: así
	// el único freno posible al par es dominiosAjenos, que es lo que este test ejercita.
	saveAt(t, e, "cre", "deploy/rollout", "Usamos PostgreSQL como base de datos principal del sistema.", "2026-01-01 10:00:00")
	proj := proyectarSpec(t, e, "s1", "", "Usamos MySQL como base de datos principal del sistema.", "2026-06-01 10:00:00")

	rels, err := e.DetectRelations(proj, ConflictOptions{DetectOnly: true})
	if err != nil {
		t.Fatalf("DetectRelations: %v", err)
	}
	encontrada := false
	for _, r := range rels {
		if r.SourceID == proj && r.TargetID == "cre" {
			encontrada = true
		}
	}
	if !encontrada {
		t.Fatalf("R4: el par cross-domain NO debe descartarse por dominiosAjenos, obtuve %+v", rels)
	}
}

// T10 — R6: aislamiento por tenant. Una proyección en el tenant A no crea relaciones contra una
// creencia del tenant B. No-regresión del guard `projectID` que ya vive en DetectRelations (corre ANTES
// que complementaryPair/dominiosAjenos, así que F5 no lo afecta). El mismo montaje en un solo tenant SÍ
// relaciona — lo prueba T6—; acá lo único que cambia es el tenant.
func TestF5AislamientoPorTenant(t *testing.T) {
	e := newTestEngine(t)
	proj := proyectarSpec(t, e, "s1", "proyA", "Usamos PostgreSQL como base de datos principal del sistema.", "2026-06-01 10:00:00")
	saveAt(t, e, "cre", "spec/s1", "Usamos PostgreSQL como base de datos principal del sistema productivo.", "2026-01-01 10:00:00")
	if _, err := e.db.Exec(`UPDATE observations SET project_id='proyB' WHERE id='cre'`); err != nil {
		t.Fatalf("fijar tenant de la creencia: %v", err)
	}

	rels, err := e.DetectRelations(proj, ConflictOptions{DetectOnly: true})
	if err != nil {
		t.Fatalf("DetectRelations: %v", err)
	}
	if len(rels) != 0 {
		t.Errorf("R6: no debe nacer relación entre tenants distintos, obtuve %+v", rels)
	}
}
