package memory

import (
	"fmt"
	"testing"
)

// seedQuotaObs guarda una observación y le fija tenant, edad (días desde created_at, con
// last_accessed NULL → la edad sale de created_at), importancia y access_count, para controlar
// exactamente su saliencia en los tests de la cuota.
func seedQuotaObs(t *testing.T, e *DbEngine, id, project string, ageDays int, importance float64, access int) {
	t.Helper()
	if err := e.SaveObservation(id, "t", "contenido de "+id, nil); err != nil {
		t.Fatalf("SaveObservation %s: %v", id, err)
	}
	if _, err := e.db.Exec(
		`UPDATE observations SET project_id=?, created_at=datetime('now',?), importance=?, access_count=? WHERE id=?`,
		project, fmt.Sprintf("-%d days", ageDays), importance, access, id,
	); err != nil {
		t.Fatalf("seed update %s: %v", id, err)
	}
}

func isArchived(t *testing.T, e *DbEngine, id string) bool {
	t.Helper()
	var a int
	if err := e.db.QueryRow(`SELECT archived FROM observations WHERE id=?`, id).Scan(&a); err != nil {
		t.Fatalf("leer archived de %s: %v", id, err)
	}
	return a == 1
}

// La cuota archiva las MÁS FRÍAS (mayor edad = menor saliencia) hasta volver bajo el techo, y
// deja intactas las más calientes. Con importancia y access iguales, la saliencia es monótona
// decreciente en la edad, así que "más frío" = "más viejo" sin ambigüedad.
func TestEnforceQuotaArchivesColdestToCeiling(t *testing.T) {
	e := newTestEngine(t)
	// 5 activas en 'p', edades 100..500 días (todas > gracia). Techo 3 → evicta las 2 más
	// frías (400 y 500 días).
	ages := map[string]int{"a": 100, "b": 200, "c": 300, "d": 400, "e": 500}
	for id, age := range ages {
		seedQuotaObs(t, e, id, "p", age, 1.0, 0)
	}

	n, err := e.EnforceQuota(QuotaOptions{MaxActivePerProject: 3, HalfLifeDays: 30, MinAgeDays: 14})
	if err != nil {
		t.Fatalf("EnforceQuota: %v", err)
	}
	if n != 2 {
		t.Fatalf("esperaba evictar 2, evicté %d", n)
	}
	// Las 2 más viejas archivadas; las 3 más nuevas activas.
	for _, id := range []string{"d", "e"} {
		if !isArchived(t, e, id) {
			t.Errorf("%s (fría) debería estar archivada", id)
		}
	}
	for _, id := range []string{"a", "b", "c"} {
		if isArchived(t, e, id) {
			t.Errorf("%s (caliente) NO debería archivarse", id)
		}
	}
}

// La cuota es POR TENANT: un proyecto que se pasa del techo no toca la memoria de otro que
// está dentro. Es la garantía que evita el daño cross-tenant en el cerebro central.
func TestEnforceQuotaPerTenantIsolation(t *testing.T) {
	e := newTestEngine(t)
	// A: 4 activas (se pasa del techo 2 → evicta 2). B: 1 activa (dentro del techo → intacta).
	for id, age := range map[string]int{"a1": 100, "a2": 200, "a3": 300, "a4": 400} {
		seedQuotaObs(t, e, id, "A", age, 1.0, 0)
	}
	seedQuotaObs(t, e, "b1", "B", 999, 1.0, 0) // vieja y fría, pero B está dentro del techo

	n, err := e.EnforceQuota(QuotaOptions{MaxActivePerProject: 2, HalfLifeDays: 30, MinAgeDays: 14})
	if err != nil {
		t.Fatalf("EnforceQuota: %v", err)
	}
	if n != 2 {
		t.Fatalf("esperaba evictar 2 (sólo de A), evicté %d", n)
	}
	if isArchived(t, e, "b1") {
		t.Error("b1 pertenece a un tenant DENTRO del techo: no debe archivarse aunque sea la más fría global")
	}
	// De A se archivan las 2 más frías (a3, a4).
	if !isArchived(t, e, "a4") || !isArchived(t, e, "a3") {
		t.Error("las 2 más frías de A deberían archivarse")
	}
}

// La importancia deliberada NO se evicta: cuenta para el techo pero la cuota la respeta. Si el
// excedente son todas protegidas, la cuota archiva sólo lo evictable (best-effort), no fuerza.
func TestEnforceQuotaProtectsImportance(t *testing.T) {
	e := newTestEngine(t)
	// Techo 1, 3 activas: 2 con importancia alta (protegidas) + 1 baja. Excedente 2, pero sólo
	// 1 es evictable → archiva 1, las 2 protegidas sobreviven aunque el proyecto quede sobre el
	// techo.
	seedQuotaObs(t, e, "imp1", "p", 300, 5.0, 0)
	seedQuotaObs(t, e, "imp2", "p", 400, 5.0, 0)
	seedQuotaObs(t, e, "low", "p", 500, 1.0, 0) // la más vieja, pero la única evictable

	n, err := e.EnforceQuota(QuotaOptions{MaxActivePerProject: 1, HalfLifeDays: 30, MinAgeDays: 14, ProtectImportance: 2.0})
	if err != nil {
		t.Fatalf("EnforceQuota: %v", err)
	}
	if n != 1 {
		t.Fatalf("esperaba evictar 1 (sólo la no protegida), evicté %d", n)
	}
	if !isArchived(t, e, "low") {
		t.Error("la observación de baja importancia debería archivarse")
	}
	if isArchived(t, e, "imp1") || isArchived(t, e, "imp2") {
		t.Error("el conocimiento deliberado (importancia alta) no debe evictarse")
	}
}

// Una observación con outbox PENDIENTE (no sincronizada) no se evicta aunque sea la más fría:
// archivarla podría dejarla varada sin llegar nunca al central. La cuota salta a la siguiente.
func TestEnforceQuotaExcludesUnsyncedOutbox(t *testing.T) {
	e := newTestEngine(t)
	// Techo 1, 2 activas frías. La MÁS fría (old) tiene outbox pendiente → protegida; se evicta
	// la otra (mid) en su lugar.
	seedQuotaObs(t, e, "old", "p", 500, 1.0, 0) // la más fría, pero no sincronizada
	seedQuotaObs(t, e, "mid", "p", 200, 1.0, 0)
	if _, err := e.db.Exec(
		`INSERT INTO outbox (obs_id, enqueued_hash, status, attempts, next_attempt_at, created_at, updated_at)
		 VALUES ('old', 'h', 'pending', 0, datetime('now'), datetime('now'), datetime('now'))`,
	); err != nil {
		t.Fatalf("encolar outbox: %v", err)
	}

	n, err := e.EnforceQuota(QuotaOptions{MaxActivePerProject: 1, HalfLifeDays: 30, MinAgeDays: 14})
	if err != nil {
		t.Fatalf("EnforceQuota: %v", err)
	}
	if n != 1 {
		t.Fatalf("esperaba evictar 1, evicté %d", n)
	}
	if isArchived(t, e, "old") {
		t.Error("la observación con outbox pendiente NO debe evictarse (se perdería sin sincronizar)")
	}
	if !isArchived(t, e, "mid") {
		t.Error("al saltar la no sincronizada, debería evictarse la siguiente más fría (mid)")
	}
}

// Las observaciones dentro del período de gracia (más nuevas que MinAgeDays) no se evictan: una
// ráfaga de ingest reciente tiene chance de ser útil antes de que la cuota la considere.
func TestEnforceQuotaRespectsMinAge(t *testing.T) {
	e := newTestEngine(t)
	// Techo 1, 2 activas: una vieja fría, una reciente (2 días < gracia 14). Se evicta la vieja;
	// la reciente queda aunque el proyecto siga sobre el techo.
	seedQuotaObs(t, e, "vieja", "p", 300, 1.0, 0)
	seedQuotaObs(t, e, "reciente", "p", 2, 1.0, 0)

	n, err := e.EnforceQuota(QuotaOptions{MaxActivePerProject: 1, HalfLifeDays: 30, MinAgeDays: 14})
	if err != nil {
		t.Fatalf("EnforceQuota: %v", err)
	}
	if n != 1 {
		t.Fatalf("esperaba evictar 1, evicté %d", n)
	}
	if !isArchived(t, e, "vieja") {
		t.Error("la vieja debería archivarse")
	}
	if isArchived(t, e, "reciente") {
		t.Error("la reciente está en período de gracia: no debe evictarse")
	}
}

// Techo <= 0 desactiva la cuota: no archiva nada, sin importar cuántas activas haya.
func TestEnforceQuotaDisabled(t *testing.T) {
	e := newTestEngine(t)
	for i := 0; i < 5; i++ {
		seedQuotaObs(t, e, fmt.Sprintf("o%d", i), "p", 100+i*100, 1.0, 0)
	}
	n, err := e.EnforceQuota(QuotaOptions{MaxActivePerProject: 0, HalfLifeDays: 30, MinAgeDays: 14})
	if err != nil {
		t.Fatalf("EnforceQuota: %v", err)
	}
	if n != 0 {
		t.Fatalf("con la cuota desactivada no debe evictar nada, evictó %d", n)
	}
}

// La cuota está cableada al ciclo Maintain y su conteo aparece en el reporte.
func TestMaintainAppliesQuota(t *testing.T) {
	e := newTestEngine(t)
	for i := 0; i < 5; i++ {
		// Edad 100..140 días: sobre la gracia pero salientes lo bastante para no caer por el
		// olvido con MinSalience bajo, así lo que archiva es la CUOTA, no el decay.
		seedQuotaObs(t, e, fmt.Sprintf("o%d", i), "p", 100+i, 1.0, 5)
	}
	rep, err := e.Maintain(MaintenanceOptions{
		DedupThreshold:         0.99, // evitar que la consolidación toque el set
		DecayHalfLifeDays:      30,
		DecayMinSalience:       0.0001, // umbral ínfimo: el decay no archiva casi nada
		DecayMinAgeDays:        14,
		PurgeArchivedAfterDays: 0, // no purgar en este test
		MaxActivePerProject:    3,
	})
	if err != nil {
		t.Fatalf("Maintain: %v", err)
	}
	if rep.Evicted != 2 {
		t.Fatalf("esperaba que la cuota evictara 2 vía Maintain, evictó %d (decay archivó %d)", rep.Evicted, rep.Decay.Archived)
	}
}
