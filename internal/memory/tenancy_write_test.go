package memory

import (
	"errors"
	"testing"
)

// El aislamiento multi-tenant de Track 17 cerró la FALSIFICACIÓN de atribución (un writer no puede
// declarar que su memoria es de otro proyecto). Pero dejó abierta la CORRUPCIÓN de memoria ajena:
// el UPSERT por id no pisa project_id, así que escribir sobre una id de otro proyecto no la
// reasignaba — le pisaba el contenido dejándola atribuida a su dueño. Estos tests fijan el borde.
//
// Encontrado en producción: una observación del proyecto A quedó (por un token mal configurado) en
// el tenant de B; reenviarla con el token CORRECTO de A no la recuperó — actualizó la fila DENTRO
// del tenant de B, con el project_id viejo intacto.

// projectOfObs devuelve el project_id almacenado de una observación.
func projectOfObs(t *testing.T, e *DbEngine, id string) string {
	t.Helper()
	var pid string
	if err := e.db.QueryRow(`SELECT COALESCE(project_id,'') FROM observations WHERE id=?`, id).Scan(&pid); err != nil {
		t.Fatalf("no se pudo leer project_id de %s: %v", id, err)
	}
	return pid
}

// contentOfObs devuelve el contenido almacenado de una observación.
func contentOfObs(t *testing.T, e *DbEngine, id string) string {
	t.Helper()
	var c string
	if err := e.db.QueryRow(`SELECT content FROM observations WHERE id=?`, id).Scan(&c); err != nil {
		t.Fatalf("no se pudo leer content de %s: %v", id, err)
	}
	return c
}

// TestSaveObservationRechazaEscrituraCrossTenant: un writer del proyecto B NO puede pisarle el
// contenido a una observación del proyecto A conociendo su id. El error es ErrCrossTenant y la
// fila de A queda INTACTA (contenido y atribución).
func TestSaveObservationRechazaEscrituraCrossTenant(t *testing.T) {
	e := newTestEngine(t)

	if err := e.SaveObservationTypedFrom("alfa", "davantis-alfa", "obs-1", "t/a", "memoria de alfa", 0.5, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}

	err := e.SaveObservationTypedFrom("beta", "davantis-beta", "obs-1", "t/b", "contenido inyectado por beta", 0.9, "semantic", "shared", nil)
	if !errors.Is(err, ErrCrossTenant) {
		t.Fatalf("escritura cross-tenant: err = %v, esperaba ErrCrossTenant", err)
	}
	if got := contentOfObs(t, e, "obs-1"); got != "memoria de alfa" {
		t.Errorf("beta le corrompió el contenido a alfa: %q", got)
	}
	if got := projectOfObs(t, e, "obs-1"); got != "alfa" {
		t.Errorf("project_id = %q, esperaba que siguiera siendo 'alfa'", got)
	}
}

// TestSaveObservationPermiteReSaveDelMismoTenant: la guardia NO puede romper el caso normal —
// un writer re-guardando su PROPIA observación por id (el UPSERT idempotente del sync) funciona.
func TestSaveObservationPermiteReSaveDelMismoTenant(t *testing.T) {
	e := newTestEngine(t)

	if err := e.SaveObservationTypedFrom("alfa", "davantis-alfa", "obs-1", "t/a", "v1", 0.5, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTypedFrom("alfa", "davantis-alfa", "obs-1", "t/a", "v2", 0.5, "semantic", "shared", nil); err != nil {
		t.Fatalf("re-save del MISMO tenant rechazado: %v", err)
	}
	if got := contentOfObs(t, e, "obs-1"); got != "v2" {
		t.Errorf("content = %q, esperaba 'v2' (el re-save propio debe actualizar)", got)
	}
}

// TestSaveObservationAdminEscribeCualquierTenant: el caller SIN tenant efectivo (admin/federado en
// el central, o stdio local sin project_id) conserva el acceso pleno — es dueño de todo y es la
// única vía para reparar una fila mal atribuida. Backward-compat del ingest legacy.
func TestSaveObservationAdminEscribeCualquierTenant(t *testing.T) {
	e := newTestEngine(t)
	e.SetProjectID("") // sin tenant: admin/federado

	if err := e.SaveObservationTypedFrom("alfa", "davantis-alfa", "obs-1", "t/a", "memoria de alfa", 0.5, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTypedFrom("", "", "obs-1", "t/a", "reparada por admin", 0.5, "semantic", "shared", nil); err != nil {
		t.Fatalf("admin (sin tenant) rechazado: %v", err)
	}
	if got := contentOfObs(t, e, "obs-1"); got != "reparada por admin" {
		t.Errorf("content = %q, esperaba que el admin pudiera escribir", got)
	}
}

// TestSaveObservationEscribeSobreFilaSinAtribuir: una fila legacy (project_id vacío, anterior a
// Track 16) sigue siendo escribible por cualquier tenant — no hay dueño que proteger.
func TestSaveObservationEscribeSobreFilaSinAtribuir(t *testing.T) {
	e := newTestEngine(t)
	e.SetProjectID("")

	if err := e.SaveObservationTyped("obs-legacy", "t/a", "fila vieja", 0.5, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	if got := projectOfObs(t, e, "obs-legacy"); got != "" {
		t.Fatalf("preparación: project_id = %q, esperaba vacío", got)
	}
	if err := e.SaveObservationTypedFrom("alfa", "davantis-alfa", "obs-legacy", "t/a", "actualizada", 0.5, "semantic", "local", nil); err != nil {
		t.Fatalf("escritura sobre fila sin atribuir rechazada: %v", err)
	}
}

// TestDedupNoCruzaTenants: el dedup por content_hash se acota al tenant que escribe. Sin esto, un
// writer cuyo contenido coincide con el de OTRO proyecto recibía el id ajeno con deduped=true y su
// observación NO se guardaba — pérdida silenciosa de memoria (y fuga de un id ajeno).
func TestDedupNoCruzaTenants(t *testing.T) {
	e := newTestEngine(t)

	idAlfa, deduped, err := e.SaveObservationDedupedTypedFrom("alfa", "davantis-alfa", "t/a", "el mismo hecho", 0.5, "semantic", "shared", nil)
	if err != nil {
		t.Fatal(err)
	}
	if deduped {
		t.Fatal("preparación: la primera no puede venir deduplicada")
	}

	idBeta, deduped, err := e.SaveObservationDedupedTypedFrom("beta", "davantis-beta", "t/b", "el mismo hecho", 0.5, "semantic", "shared", nil)
	if err != nil {
		t.Fatal(err)
	}
	if deduped {
		t.Error("beta fue deduplicada contra una fila de alfa: su memoria se perdería en silencio")
	}
	if idBeta == idAlfa {
		t.Fatalf("beta recibió el id de alfa (%s): fuga de id ajeno + pérdida de la observación", idAlfa)
	}
	if got := projectOfObs(t, e, idBeta); got != "beta" {
		t.Errorf("project_id de la de beta = %q, esperaba 'beta'", got)
	}
}

// TestDedupSigueFuncionandoDentroDelTenant: la guardia no puede romper el dedup normal — el mismo
// contenido guardado dos veces por el MISMO tenant sigue deduplicándose.
func TestDedupSigueFuncionandoDentroDelTenant(t *testing.T) {
	e := newTestEngine(t)

	id1, _, err := e.SaveObservationDedupedTypedFrom("alfa", "davantis-alfa", "t/a", "hecho repetido", 0.5, "semantic", "shared", nil)
	if err != nil {
		t.Fatal(err)
	}
	id2, deduped, err := e.SaveObservationDedupedTypedFrom("alfa", "davantis-alfa", "t/a", "hecho repetido", 0.5, "semantic", "shared", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !deduped || id1 != id2 {
		t.Errorf("dedup dentro del tenant roto: id1=%s id2=%s deduped=%v", id1, id2, deduped)
	}
}
