package memory

import (
	"errors"
	"testing"
)

// Tests del DEDUP SEMÁNTICO del acervo (pilar 'Musubi Renaissance', el afilador). Cubren: la detección
// de pares gemelos por coseno con el orden canónico correcto, el aislamiento por tenant, la exclusión de
// pares ya juzgados, el chequeo anti-gemelo, y el archivado que hereda accesos e importancia.

func saveDesignCard(t *testing.T, e *DbEngine, project, id, topic, content string, vec []float32, access int, importance float64) {
	t.Helper()
	if err := e.SaveObservationTypedFrom(project, "seed", id, topic, content, importance, "semantic", "shared", vec); err != nil {
		t.Fatalf("guardar tarjeta %s: %v", id, err)
	}
	if access > 0 {
		if _, err := e.db.Exec(`UPDATE observations SET access_count=? WHERE id=?`, access, id); err != nil {
			t.Fatalf("fijar access de %s: %v", id, err)
		}
	}
}

func TestSemanticDuplicateCandidates(t *testing.T) {
	e := newTestEngine(t)
	const proj = "musubi-design"
	// A y B son casi paralelos (coseno ~0.98); C es ortogonal a ambos. A es MÁS FUERTE (más accesos).
	saveDesignCard(t, e, proj, "a", "design-corpus/contraste-a", "contraste 4.5:1", []float32{1, 0, 0, 0}, 5, 1.0)
	saveDesignCard(t, e, proj, "b", "design-corpus/contraste-b", "el texto necesita 4.5:1", []float32{0.98, 0.2, 0, 0}, 0, 1.0)
	saveDesignCard(t, e, proj, "c", "design-corpus/espaciado", "todo múltiplo de 4", []float32{0, 1, 0, 0}, 0, 1.0)
	// De otro tenant, aunque su vector calce con A: jamás debe emparejarse (aislamiento).
	saveDesignCard(t, e, "otro", "z", "design-corpus/ajeno", "de otro proyecto", []float32{1, 0, 0, 0}, 0, 1.0)

	cands, err := e.SemanticDuplicateCandidates(proj, "design-corpus/", 0.9, 10)
	if err != nil {
		t.Fatalf("SemanticDuplicateCandidates: %v", err)
	}
	if len(cands) != 1 {
		t.Fatalf("esperaba 1 par (a,b) sobre el piso; obtuve %d: %+v", len(cands), cands)
	}
	if cands[0].A != "a" || cands[0].B != "b" {
		t.Errorf("el canónico (más fuerte) debe ser 'a' y el candidato 'b'; obtuve A=%s B=%s", cands[0].A, cands[0].B)
	}
	if cands[0].Cosine < 0.95 {
		t.Errorf("coseno esperado ~0.98, obtuve %.3f", cands[0].Cosine)
	}

	// Un par YA juzgado (not_duplicate) se excluye: no se re-propone.
	if _, err := e.UpsertObsRelation(ObsRelation{SourceID: "a", TargetID: "b", Relation: RelNotDuplicate, Status: RelStatusResolved}); err != nil {
		t.Fatalf("marcar not_duplicate: %v", err)
	}
	cands2, err := e.SemanticDuplicateCandidates(proj, "design-corpus/", 0.9, 10)
	if err != nil {
		t.Fatalf("SemanticDuplicateCandidates #2: %v", err)
	}
	if len(cands2) != 0 {
		t.Errorf("un par marcado not_duplicate no debe volver a proponerse; obtuve %+v", cands2)
	}
}

func TestArchiveAsDuplicate(t *testing.T) {
	e := newTestEngine(t)
	const proj = "musubi-design"
	saveDesignCard(t, e, proj, "canon", "design-corpus/canon", "la buena", []float32{1, 0, 0, 0}, 5, 1.0)
	saveDesignCard(t, e, proj, "dup", "design-corpus/dup", "la gemela", []float32{0.98, 0.2, 0, 0}, 3, 2.0)

	archived, err := e.ArchiveAsDuplicate(proj, "dup", "canon")
	if err != nil || !archived {
		t.Fatalf("archivar dup: archived=%v err=%v", archived, err)
	}
	// El canónico heredó accesos (5+3) e importancia máxima (max(1,2)=2).
	var access int
	var importance float64
	if err := e.db.QueryRow(`SELECT access_count, importance FROM observations WHERE id='canon'`).Scan(&access, &importance); err != nil {
		t.Fatalf("leer canon: %v", err)
	}
	if access != 8 {
		t.Errorf("el canónico debe heredar accesos: esperaba 8, obtuve %d", access)
	}
	if importance != 2.0 {
		t.Errorf("el canónico debe quedar con la importancia máxima: esperaba 2.0, obtuve %.1f", importance)
	}
	// El perdedor quedó archivado y apuntando al canónico.
	var arch int
	var sup string
	if err := e.db.QueryRow(`SELECT COALESCE(archived,0), COALESCE(superseded_by,'') FROM observations WHERE id='dup'`).Scan(&arch, &sup); err != nil {
		t.Fatalf("leer dup: %v", err)
	}
	if arch != 1 || sup != "canon" {
		t.Errorf("el perdedor debe quedar archivado apuntando al canónico; archived=%d superseded_by=%q", arch, sup)
	}

	// Idempotente: re-archivar el mismo perdedor es un no-op (archived=false, sin error).
	again, err := e.ArchiveAsDuplicate(proj, "dup", "canon")
	if err != nil || again {
		t.Errorf("re-archivar un perdedor ya archivado debe ser no-op; archived=%v err=%v", again, err)
	}

	// Guarda de tenant: fusionar con un projectID que no es el de las tarjetas debe fallar.
	saveDesignCard(t, e, proj, "otra", "design-corpus/otra", "x", []float32{1, 0, 0, 0}, 0, 1.0)
	if _, err := e.ArchiveAsDuplicate("tenant-ajeno", "otra", "canon"); err == nil {
		t.Error("archivar con un projectID ajeno a las tarjetas debe fallar (aislamiento)")
	}
}

func TestNearestVisibleByVector(t *testing.T) {
	e := newTestEngine(t)
	const proj = "musubi-design"
	saveDesignCard(t, e, proj, "a", "design-corpus/a", "uno", []float32{1, 0, 0, 0}, 0, 1.0)
	saveDesignCard(t, e, proj, "c", "design-corpus/c", "dos", []float32{0, 1, 0, 0}, 0, 1.0)

	id, _, cos, err := e.NearestVisibleByVector(proj, "design-corpus/", []float32{0.99, 0.1, 0, 0}, "")
	if err != nil {
		t.Fatalf("NearestVisibleByVector: %v", err)
	}
	if id != "a" || cos < 0.95 {
		t.Errorf("el más cercano a ~[1,0..] debe ser 'a' con coseno alto; obtuve id=%s cos=%.3f", id, cos)
	}
	// Excluyendo 'a' queda 'c' (ortogonal, coseno bajo pero es el único).
	id2, _, _, err := e.NearestVisibleByVector(proj, "design-corpus/", []float32{0.99, 0.1, 0, 0}, "a")
	if err != nil {
		t.Fatalf("NearestVisibleByVector excluyendo a: %v", err)
	}
	if id2 != "c" {
		t.Errorf("excluyendo 'a' el más cercano debe ser 'c'; obtuve %s", id2)
	}
	// Sin vector (nil): vacío, sin error (degrada blando cuando no hay embedder).
	if id3, _, _, err := e.NearestVisibleByVector(proj, "design-corpus/", nil, ""); err != nil || id3 != "" {
		t.Errorf("con vec nil debe devolver vacío sin error; id=%q err=%v", id3, err)
	}
}

// TestDeshacerUnaFusionDevuelveLaTarjetaYMarcaElPar — el «reversible» de ArchiveAsDuplicate, con reversa.
//
// Hasta el 2026-09-24 no había ningún camino para deshacer una fusión, y el etiquetado a ciegas de ese
// día encontró 8 de 28 que habían perdido algo. Esta prueba archiva y deshace, y mira CADA cosa que la
// reversa tiene que dejar como estaba, porque cada una falla distinto si falta:
//   - visible: sin limpiar superseded_by la tarjeta sigue fuera del recall aunque diga archived=0.
//   - el par marcado: sin la marca, el afilado de fondo la vuelve a proponer y la fusión se rehace sola.
//   - los accesos devueltos: si no, el canónico queda inflado con accesos que no son suyos.
//   - sync_seq avanzado: si no, un espejo que ya pasó ese número nunca se entera de que volvió.
//
// Sabotaje que la hace fallar: no marcar el par not_duplicate al deshacer.
// arnes: archivo="internal/memory/semdedup.go"
// arnes: de="if _, err := upsertObsRelationCon(tx, ObsRelation{"
// arnes: a="if _, err := (func(execQuerier, ObsRelation) (ObsRelation, error) { return ObsRelation{}, nil })(tx, ObsRelation{"
//
// Sabotaje que la hace fallar: no limpiar superseded_by al devolver la tarjeta.
// arnes: archivo="internal/memory/semdedup.go"
// arnes: de="superseded_by = NULL,\n"
// arnes: a=""
//
// Sabotaje que la hace fallar: no devolverle al canónico los accesos que heredó.
// arnes: archivo="internal/memory/semdedup.go"
// arnes: de="access_count = MAX(0, access_count - ?)"
// arnes: a="access_count = access_count + 0 * ?"
//
// Sabotaje que la hace fallar: no avanzar sync_seq al devolver la tarjeta.
// arnes: archivo="internal/memory/semdedup.go"
// arnes: de="sync_seq = (SELECT IFNULL(MAX(sync_seq),0) FROM observations) + 1"
// arnes: a="sync_seq = sync_seq"
func TestDeshacerUnaFusionDevuelveLaTarjetaYMarcaElPar(t *testing.T) {
	e := newTestEngine(t)
	const proj = "musubi-design"
	vecDup := []float32{0.98, 0.2, 0, 0}
	saveDesignCard(t, e, proj, "canon", "design-corpus/canon", "la que quedó", []float32{1, 0, 0, 0}, 5, 1.0)
	saveDesignCard(t, e, proj, "dup", "design-corpus/dup", "la que se archivó y decía algo propio", vecDup, 3, 2.0)

	if ok, err := e.ArchiveAsDuplicate(proj, "dup", "canon"); err != nil || !ok {
		t.Fatalf("precondición: archivar dup: ok=%v err=%v", ok, err)
	}
	var seqArchivada int64
	if err := e.db.QueryRow(`SELECT COALESCE(sync_seq,0) FROM observations WHERE id='dup'`).Scan(&seqArchivada); err != nil {
		t.Fatal(err)
	}

	restored, canon, err := e.RestoreDuplicate(proj, "dup", "afilador")
	if err != nil || !restored || canon != "canon" {
		t.Fatalf("deshacer: restored=%v canon=%q err=%v; esperaba true, \"canon\", nil", restored, canon, err)
	}

	if id, _, _, err := e.NearestVisibleByVector(proj, "design-corpus/", vecDup, ""); err != nil || id != "dup" {
		t.Errorf("la tarjeta devuelta no es visible: la más cercana a su propio vector es %q (err %v)", id, err)
	}
	var access int
	if err := e.db.QueryRow(`SELECT access_count FROM observations WHERE id='canon'`).Scan(&access); err != nil {
		t.Fatal(err)
	}
	if access != 5 {
		t.Errorf("el canónico debía volver a sus 5 accesos propios; tiene %d", access)
	}
	var seq int64
	if err := e.db.QueryRow(`SELECT COALESCE(sync_seq,0) FROM observations WHERE id='dup'`).Scan(&seq); err != nil {
		t.Fatal(err)
	}
	if seq <= seqArchivada {
		t.Errorf("sync_seq no avanzó al devolverla (%d → %d): un espejo que ya pasó ese número no la vuelve a bajar", seqArchivada, seq)
	}
	cands, err := e.SemanticDuplicateCandidates(proj, "design-corpus/", 0.5, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(cands) != 0 {
		t.Errorf("el par deshecho volvió a proponerse (%d candidatos): el afilado de fondo lo fusionaría de nuevo", len(cands))
	}

	// Idempotente: una tarjeta ya visible no tiene fusión que deshacer.
	if again, _, err := e.RestoreDuplicate(proj, "dup", "afilador"); err != nil || again {
		t.Errorf("deshacer dos veces debe ser no-op; restored=%v err=%v", again, err)
	}
}

// TestDeshacerRechazaLoQueNoFueUnaFusion — la reversa no es una puerta trasera para revivir cualquier
// archivada, ni cruza de tenant.
//
// Una tarjeta archivada por el olvido o por la cuota NO tiene superseded_by. Revivirla con esto
// saltearía el flujo que la archivó (y su razón); por eso se rechaza y queda como estaba.
//
// Sabotaje que la hace fallar: aceptar una archivada que no apunta a ningún canónico.
// arnes: archivo="internal/memory/semdedup.go"
// arnes: de="\tif lArch == 0 || sup == \"\" {"
// arnes: a="\tif false {"
//
// EL TENANT SE MIRA EN LAS DOS PUNTAS, y cada chequeo cubre un caso que el otro no ve. El de la
// tarjeta es el único que queda cuando el canónico ya no existe (purgado o borrado): sin él, la tarjeta
// de otro tenant se devolvería. El del canónico impide restarle accesos a una tarjeta ajena cuando un
// superseded_by apunta a través de tenants.
//
// Sabotaje que la hace fallar: no mirar el tenant de la tarjeta.
// arnes: archivo="internal/memory/semdedup.go"
// arnes: de="\tif lProj != projectID {"
// arnes: a="\tif false {"
//
// Sabotaje que la hace fallar: no mirar el tenant del canónico.
// arnes: archivo="internal/memory/semdedup.go"
// arnes: de="\tif canonicoVivo && cProj != projectID {"
// arnes: a="\tif false {"
func TestDeshacerRechazaLoQueNoFueUnaFusion(t *testing.T) {
	e := newTestEngine(t)
	const proj = "musubi-design"
	saveDesignCard(t, e, proj, "olvidada", "design-corpus/olvidada", "archivada por el olvido", []float32{1, 0, 0, 0}, 0, 1.0)
	if _, err := e.db.Exec(`UPDATE observations SET archived=1, archived_at=CURRENT_TIMESTAMP WHERE id='olvidada'`); err != nil {
		t.Fatal(err)
	}
	if ok, _, err := e.RestoreDuplicate(proj, "olvidada", "afilador"); err == nil || ok {
		t.Errorf("una archivada sin canónico no la archivó una fusión y no se deshace; restored=%v err=%v", ok, err)
	}
	var arch int
	if err := e.db.QueryRow(`SELECT archived FROM observations WHERE id='olvidada'`).Scan(&arch); err != nil || arch != 1 {
		t.Errorf("la olvidada tenía que seguir archivada; archived=%d err=%v", arch, err)
	}

	saveDesignCard(t, e, proj, "canon", "design-corpus/canon", "la buena", []float32{1, 0, 0, 0}, 0, 1.0)
	saveDesignCard(t, e, proj, "dup", "design-corpus/dup", "la gemela", []float32{0.98, 0.2, 0, 0}, 0, 1.0)
	if ok, err := e.ArchiveAsDuplicate(proj, "dup", "canon"); err != nil || !ok {
		t.Fatalf("precondición: archivar dup: ok=%v err=%v", ok, err)
	}
	if ok, _, err := e.RestoreDuplicate("tenant-ajeno", "dup", "afilador"); !errors.Is(err, ErrCrossTenant) || ok {
		t.Errorf("deshacer desde otro tenant debe fallar con ErrCrossTenant; restored=%v err=%v", ok, err)
	}

	// El canónico ya no existe: el único chequeo que queda es el de la tarjeta.
	if _, err := e.db.Exec(`DELETE FROM observations WHERE id='canon'`); err != nil {
		t.Fatalf("precondición: borrar el canónico: %v", err)
	}
	if ok, _, err := e.RestoreDuplicate("tenant-ajeno", "dup", "afilador"); !errors.Is(err, ErrCrossTenant) || ok {
		t.Errorf("con el canónico borrado, deshacer desde otro tenant debe seguir fallando con ErrCrossTenant; restored=%v err=%v", ok, err)
	}

	// Un superseded_by que cruza de tenant: la tarjeta es de este proyecto y el canónico de otro.
	saveDesignCard(t, e, "otro-proyecto", "ajena", "design-corpus/ajena", "de otro tenant", []float32{0, 1, 0, 0}, 7, 1.0)
	saveDesignCard(t, e, proj, "cruzada", "design-corpus/cruzada", "apunta afuera", []float32{0, 0.98, 0.2, 0}, 2, 1.0)
	if _, err := e.db.Exec(`UPDATE observations SET archived=1, archived_at=CURRENT_TIMESTAMP, superseded_by='ajena' WHERE id='cruzada'`); err != nil {
		t.Fatal(err)
	}
	if ok, _, err := e.RestoreDuplicate(proj, "cruzada", "afilador"); !errors.Is(err, ErrCrossTenant) || ok {
		t.Errorf("un canónico de otro tenant debe frenar la reversa con ErrCrossTenant; restored=%v err=%v", ok, err)
	}
	var accesoAjena int
	if err := e.db.QueryRow(`SELECT access_count FROM observations WHERE id='ajena'`).Scan(&accesoAjena); err != nil || accesoAjena != 7 {
		t.Errorf("a la tarjeta de otro tenant no se le toca nada; access_count=%d err=%v", accesoAjena, err)
	}
}

// TestDeshacerUnaFusionLaVuelveAlIndiceEntrenado — el espejo del RemoveBatch de ArchiveAsDuplicate.
//
// Con el índice IVF entrenado, SearchObservations rankea SÓLO lo que el índice devuelve. Archivar
// saca la tarjeta del índice; si deshacer no la vuelve a meter, queda visible para el recall léxico
// e invisible para el semántico hasta el próximo rebuild —horas—. El umbral se baja a 1 para
// entrenar con pocas filas y se sondea una sola celda, la de la propia tarjeta.
//
// Sabotaje que la hace fallar: no volver a agregar la tarjeta al índice.
// arnes: archivo="internal/memory/semdedup.go"
// arnes: de="\t\t\te.index.Add(loserID, v)\n"
// arnes: a=""
func TestDeshacerUnaFusionLaVuelveAlIndiceEntrenado(t *testing.T) {
	e := newTestEngine(t)
	e.SetVectorModelID("static:tabla@aaaa")
	const dim = 16
	data := clusteredDataset(33, 120, dim, 6)
	for _, d := range data {
		if err := e.SaveObservation(d.id, "t", "c "+d.id, d.vec); err != nil {
			t.Fatal(err)
		}
	}
	e.vindexCfg.Enabled = true
	e.vindexCfg.ExactThreshold = 1
	e.vindexCfg.NProbe = 1
	if err := e.rebuildVectorIndex(); err != nil {
		t.Fatal(err)
	}
	if !e.index.Trained() {
		t.Fatal("precondición: el índice debería estar entrenado")
	}
	perdedora, canonica := data[0].id, data[1].id
	enIndice := func() bool {
		ids, _ := e.index.Search(data[0].vec, e.vindexCfg.NProbe)
		for _, id := range ids {
			if id == perdedora {
				return true
			}
		}
		return false
	}
	if !enIndice() {
		t.Fatal("precondición: antes de archivarla, la tarjeta tiene que estar en su celda")
	}
	if ok, err := e.ArchiveAsDuplicate(e.projectID, perdedora, canonica); err != nil || !ok {
		t.Fatalf("precondición: archivar: ok=%v err=%v", ok, err)
	}
	if enIndice() {
		t.Fatal("precondición: archivar tenía que sacarla del índice")
	}

	if ok, _, err := e.RestoreDuplicate(e.projectID, perdedora, "afilador"); err != nil || !ok {
		t.Fatalf("deshacer: ok=%v err=%v", ok, err)
	}
	if !enIndice() {
		t.Error("la tarjeta devuelta no volvió al índice entrenado: el recall semántico no la ve hasta el próximo rebuild")
	}
}
