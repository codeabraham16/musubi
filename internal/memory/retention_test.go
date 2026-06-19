package memory

import (
	"sort"
	"testing"
	"time"
)

// sqliteTime formatea un instante como lo guarda SQLite (para sembrar created_at).
func sqliteTime(t time.Time) string { return t.UTC().Format(sqliteTimeLayout) }

// TestPurgeArchivedDeletesOldKeepsRest verifica la retención dura: solo borra
// archivadas vencidas, conserva archivadas recientes y las activas, y limpia
// embeddings (FK CASCADE) y relaciones semánticas (sin FK).
func TestPurgeArchivedDeletesOldKeepsRest(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	now := time.Now().UTC()
	emb := []float32{1, 0, 0, 0}

	// activa
	if err := e.SaveObservation("active", "t", "vivo", emb); err != nil {
		t.Fatal(err)
	}
	// archivada vieja (100 días) -> debe purgarse
	if err := e.SaveObservation("old", "t", "viejo archivado", emb); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET archived=1, archived_at=? WHERE id=?`,
		sqliteTime(now.AddDate(0, 0, -100)), "old"); err != nil {
		t.Fatal(err)
	}
	// archivada reciente (10 días) -> debe conservarse (dentro de la ventana de gracia)
	if err := e.SaveObservation("recent", "t", "reciente archivado", emb); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET archived=1, archived_at=? WHERE id=?`,
		sqliteTime(now.AddDate(0, 0, -10)), "recent"); err != nil {
		t.Fatal(err)
	}
	// relación semántica que referencia a "old" -> debe limpiarse
	if _, err := e.db.Exec(`INSERT INTO observation_relations (id, source_id, target_id, relation) VALUES (?,?,?,?)`,
		"rel1", "old", "active", "related"); err != nil {
		t.Fatal(err)
	}

	n, err := e.PurgeArchived(90)
	if err != nil {
		t.Fatalf("PurgeArchived: %v", err)
	}
	if n != 1 {
		t.Errorf("purgadas=%d, esperaba 1 (solo la vieja)", n)
	}

	assertExists := func(id string, want bool) {
		t.Helper()
		var c int
		if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE id=?`, id).Scan(&c); err != nil {
			t.Fatal(err)
		}
		if (c > 0) != want {
			t.Errorf("observación %q existe=%v, esperaba %v", id, c > 0, want)
		}
	}
	assertExists("active", true)
	assertExists("recent", true)
	assertExists("old", false)

	// embedding de "old" borrado por CASCADE
	var embCount int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM embeddings WHERE observation_id=?`, "old").Scan(&embCount); err != nil {
		t.Fatal(err)
	}
	if embCount != 0 {
		t.Errorf("el embedding de la observación purgada debió borrarse por CASCADE")
	}
	// relación de "old" limpiada
	var relCount int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observation_relations WHERE id=?`, "rel1").Scan(&relCount); err != nil {
		t.Fatal(err)
	}
	if relCount != 0 {
		t.Errorf("la relación que referenciaba a la observación purgada debió limpiarse")
	}
}

// TestPurgeArchivedDisabled verifica que olderThanDays <= 0 desactiva la purga.
func TestPurgeArchivedDisabled(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err := e.SaveObservation("a", "t", "x", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET archived=1, created_at=? WHERE id=?`,
		sqliteTime(time.Now().UTC().AddDate(-5, 0, 0)), "a"); err != nil {
		t.Fatal(err)
	}
	n, err := e.PurgeArchived(0)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("purga desactivada (0) no debería borrar nada, borró %d", n)
	}
}

// TestCompactRuns verifica que Compact (checkpoint + optimize + VACUUM) corre sin error.
func TestCompactRuns(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	if err := e.SaveObservation("a", "t", "x", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.Compact(true); err != nil {
		t.Fatalf("Compact(vacuum): %v", err)
	}
	// La base sigue usable tras VACUUM.
	if n, err := e.CountObservations(); err != nil || n != 1 {
		t.Errorf("tras Compact: count=%d err=%v, esperaba 1", n, err)
	}
}

// TestMaintainPipeline verifica el ciclo completo end-to-end sobre datos sembrados.
func TestMaintainPipeline(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	old := time.Now().UTC().AddDate(0, 0, -200)
	// dos casi-duplicados (se consolidan)
	_ = e.SaveObservation("d1", "t", "el patron observer en go sirve para eventos", nil)
	_ = e.SaveObservation("d2", "t", "el patron observer en go sirve para eventos.", nil)
	// ya archivada hace tiempo (archived_at viejo) -> se purga
	_ = e.SaveObservation("old", "t", "memoria fria ya archivada", nil)
	if _, err := e.db.Exec(`UPDATE observations SET archived=1, archived_at=? WHERE id=?`,
		sqliteTime(old), "old"); err != nil {
		t.Fatal(err)
	}
	// fría sin acceso: Decay la archiva EN ESTE ciclo (archived_at = ahora) -> NO debe
	// purgarse en la misma corrida (período de gracia). Valida el fix del archived_at.
	_ = e.SaveObservation("cold", "t", "memoria fria que se archiva ahora", nil)
	if _, err := e.db.Exec(`UPDATE observations SET created_at=?, last_accessed=? WHERE id=?`,
		sqliteTime(old), sqliteTime(old), "cold"); err != nil {
		t.Fatal(err)
	}

	rep, err := e.Maintain(MaintenanceOptions{
		DedupThreshold:         0.85,
		DecayHalfLifeDays:      30,
		DecayMinSalience:       0.2,
		DecayMinAgeDays:        14,
		PurgeArchivedAfterDays: 90,
		Vacuum:                 true,
	})
	if err != nil {
		t.Fatalf("Maintain: %v", err)
	}
	if rep.Consolidate.Merged != 1 {
		t.Errorf("esperaba 1 fusión, fue %d", rep.Consolidate.Merged)
	}
	if rep.Decay.Archived != 1 {
		t.Errorf("esperaba 1 archivada (cold), fue %d", rep.Decay.Archived)
	}
	if rep.Purged != 1 {
		t.Errorf("esperaba 1 purga (old), fue %d", rep.Purged)
	}
	if !rep.Compacted {
		t.Error("esperaba Compacted=true")
	}
	// 'cold' fue archivada en este ciclo: sigue existiendo (no purgada por gracia).
	var coldArchived int
	if err := e.db.QueryRow(`SELECT archived FROM observations WHERE id='cold'`).Scan(&coldArchived); err != nil {
		t.Fatalf("'cold' no debería haberse purgado en el mismo ciclo en que se archivó: %v", err)
	}
	if coldArchived != 1 {
		t.Errorf("'cold' debería estar archivada (=1), fue %d", coldArchived)
	}
}

// TestConsolidateInvertedIndexMatchesBruteForce es el GUARDARRAÍL del optimizado
// O(n²)->~O(n): el bloqueo por trigramas debe producir EXACTAMENTE los mismos
// sobrevivientes que la referencia brute-force (comparar contra todos los canónicos)
// sobre el mismo dataset y orden. Saltear candidatos sin trigramas compartidos no
// cambia el resultado (su Jaccard es 0).
func TestConsolidateInvertedIndexMatchesBruteForce(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	// Mezcla determinista: grupos de casi-duplicados + únicos.
	contents := []string{}
	bases := []string{
		"el motor de orquestacion dag de musubi es model free",
		"la busqueda semantica usa un indice ivf por centroides",
		"las migraciones versionadas usan pragma user version",
		"el recall por presupuesto de tokens empaqueta gists",
		"la pizarra compartida coordina sub agentes model free",
	}
	for _, b := range bases {
		contents = append(contents, b)       // original
		contents = append(contents, b+".")   // casi-dup (puntuación)
		contents = append(contents, b+"  ")  // casi-dup (espacios)
	}
	// algunos únicos sin parentesco
	contents = append(contents,
		"go pure sin cgo sqlite local first",
		"zephyr quetzal xilofono jacaranda",
		"un texto completamente distinto sobre redes neuronales")

	for i, c := range contents {
		if err := e.SaveObservation(idForIndex(i), "t", c, nil); err != nil {
			t.Fatal(err)
		}
	}

	const threshold = 0.85

	// Capturar la entrada EXACTAMENTE como la lee Consolidate (misma query + mismo orden).
	type ci struct {
		id, content, createdAt string
		access                 int
		importance             float64
	}
	rows, err := e.db.Query(`SELECT id, content, access_count, importance, COALESCE(created_at,'') FROM observations WHERE archived = 0 AND superseded_by IS NULL`)
	if err != nil {
		t.Fatal(err)
	}
	var all []ci
	for rows.Next() {
		var o ci
		if err := rows.Scan(&o.id, &o.content, &o.access, &o.importance, &o.createdAt); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		all = append(all, o)
	}
	rows.Close()
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].access != all[j].access {
			return all[i].access > all[j].access
		}
		if all[i].importance != all[j].importance {
			return all[i].importance > all[j].importance
		}
		return all[i].createdAt > all[j].createdAt
	})
	// Referencia brute-force: sobrevivientes esperados.
	var keptIDs []string
	var keptContent []string
	for _, o := range all {
		idx := -1
		for ki := range keptContent {
			if Similarity(o.content, keptContent[ki]) >= threshold {
				idx = ki
				break
			}
		}
		if idx == -1 {
			keptIDs = append(keptIDs, o.id)
			keptContent = append(keptContent, o.content)
		}
	}

	res, err := e.Consolidate(threshold)
	if err != nil {
		t.Fatalf("Consolidate: %v", err)
	}
	wantMerged := len(all) - len(keptIDs)
	if res.Merged != wantMerged {
		t.Fatalf("merged=%d, brute-force esperaba %d", res.Merged, wantMerged)
	}

	// Los sobrevivientes en la DB deben ser EXACTAMENTE el set predicho.
	survivors := map[string]bool{}
	srows, err := e.db.Query(`SELECT id FROM observations`)
	if err != nil {
		t.Fatal(err)
	}
	for srows.Next() {
		var id string
		if err := srows.Scan(&id); err != nil {
			srows.Close()
			t.Fatal(err)
		}
		survivors[id] = true
	}
	srows.Close()
	if len(survivors) != len(keptIDs) {
		t.Fatalf("sobrevivientes=%d, esperaba %d", len(survivors), len(keptIDs))
	}
	for _, id := range keptIDs {
		if !survivors[id] {
			t.Errorf("el canónico %q debería sobrevivir y no está", id)
		}
	}
}

// TestMigrationV2ArchivedIndex verifica que la migración v2 agrega idx_obs_archived
// y que el esquema queda en la última versión.
func TestMigrationV2ArchivedIndex(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	if v, _ := e.schemaVersion(); v != latestSchemaVersion() || v < 2 {
		t.Errorf("user_version=%d, esperaba >= 2 (última %d)", v, latestSchemaVersion())
	}
	var name string
	err = e.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='idx_obs_archived'`).Scan(&name)
	if err != nil {
		t.Fatalf("no se encontró el índice idx_obs_archived: %v", err)
	}
	// Migración v3: columna archived_at presente.
	cols, err := e.observationColumns()
	if err != nil {
		t.Fatal(err)
	}
	if !cols["archived_at"] {
		t.Error("la migración v3 debió agregar la columna archived_at")
	}
}

// TestConsolidateRepointsSupersededToCanonical valida el fix LOW: al consolidar una
// observación que era FUENTE de un supersede, los punteros superseded_by se re-apuntan
// al canónico (la oculta sigue oculta), en vez de quedar en NULL (que la resucitaría).
func TestConsolidateRepointsSupersededToCanonical(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	// Z y X son casi-duplicados; Z más fuerte (más accesos) -> canónico. X supersede a Y.
	_ = e.SaveObservation("Z", "t", "usamos postgresql para la base de datos del sistema", nil)
	_ = e.SaveObservation("X", "t", "usamos postgresql para la base de datos del sistema.", nil)
	_ = e.SaveObservation("Y", "t", "una observacion vieja distinta sobre la base", nil)
	if _, err := e.db.Exec(`UPDATE observations SET access_count=10 WHERE id='Z'`); err != nil {
		t.Fatal(err)
	}
	// X oculta a Y (Y.superseded_by = X). X queda viva (candidata a consolidar).
	if err := e.markSuperseded("Y", "X"); err != nil {
		t.Fatal(err)
	}

	if _, err := e.Consolidate(0.85); err != nil {
		t.Fatalf("Consolidate: %v", err)
	}

	// X se fusionó en Z (borrada). Y debe seguir oculta, ahora apuntando a Z (no NULL, no X borrado).
	var sup string
	if err := e.db.QueryRow(`SELECT COALESCE(superseded_by,'') FROM observations WHERE id='Y'`).Scan(&sup); err != nil {
		t.Fatalf("'Y' no debería haberse borrado: %v", err)
	}
	if sup != "Z" {
		t.Errorf("Y.superseded_by debería re-apuntar al canónico 'Z', fue %q (NULL la resucitaría)", sup)
	}
}
