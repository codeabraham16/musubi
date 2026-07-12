package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Fase 0 (P0) — el doctor no podía reparar justo cuando más se lo necesitaba. Tres fallas que se
// componían: la detección era CIEGA, el backup previo LEÍA lo corrupto, y la reconstrucción también.
// El principio: nada del camino de reparación puede depender de LEER lo que está roto.

// F.b / R4-R5 — la reconstrucción deja el índice FUNCIONAL: tras el rebuild, el FTS encuentra las
// observaciones. (Antes se hacía con DELETE, que recorre el b-tree y falla sobre páginas corruptas;
// ahora con DROP+recreate, que libera páginas sin leerlas.)
func TestRebuildFTSLeavesIndexUsable(t *testing.T) {
	e := newTestEngine(t)
	for _, id := range []string{"a", "b", "c"} {
		if err := e.SaveObservation(id, "t", "kubernetes en produccion "+id, nil); err != nil {
			t.Fatal(err)
		}
	}

	n, err := applyRebuildFTS(e)
	if err != nil {
		t.Fatalf("applyRebuildFTS: %v", err)
	}
	if n != 3 {
		t.Errorf("esperaba re-poblar 3 observaciones, obtuve %d", n)
	}

	res, err := e.SearchObservationsFTS(context.Background(), "kubernetes", 10)
	if err != nil {
		t.Fatalf("búsqueda FTS tras el rebuild: %v", err)
	}
	if len(res) != 3 {
		t.Errorf("tras el rebuild el índice debe encontrar las 3 observaciones, encontró %d", len(res))
	}
}

// F.c / R6 — LOS TRIGGERS SOBREVIVEN AL DROP. Es la asunción riesgosa del cambio: los triggers están
// sobre `observations` (no sobre la tabla FTS) y la referencian por NOMBRE, así que al recrearla
// deberían seguir funcionando. Se VERIFICA, no se asume: si el DROP los matara, la memoria nueva
// dejaría de indexarse en silencio — un desastre mucho peor que la corrupción que fuimos a curar.
func TestRebuildFTSKeepsTriggersAlive(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("vieja", "t", "postgres en el servidor", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := applyRebuildFTS(e); err != nil {
		t.Fatalf("applyRebuildFTS: %v", err)
	}

	// Observación NUEVA, guardada DESPUÉS del DROP+recreate: sólo la indexa el trigger.
	if err := e.SaveObservation("nueva", "t", "redis como cache", nil); err != nil {
		t.Fatal(err)
	}

	res, err := e.SearchObservationsFTS(context.Background(), "redis", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].ID != "nueva" {
		t.Fatal("el trigger de sincronización NO sobrevivió al DROP: la memoria nueva dejó de indexarse (falla silenciosa)")
	}

	// Y un UPDATE también debe seguir propagándose al índice.
	if _, err := e.db.Exec(`UPDATE observations SET content='memcached como cache' WHERE id='nueva'`); err != nil {
		t.Fatal(err)
	}
	res, err = e.SearchObservationsFTS(context.Background(), "memcached", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 {
		t.Error("el trigger de UPDATE tampoco debe morir con el DROP")
	}
}

// F.a / R1-R3 — la detección corre el integrity-check nativo de FTS5 (además del drift de COUNT).
// Con el índice sano, reporta ok; el drift por COUNT se sigue detectando (es OTRO modo de falla).
func TestCheckFTSRunsIntegrityCheckAndDrift(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("a", "t", "contenido", nil); err != nil {
		t.Fatal(err)
	}

	// Índice sano: el integrity-check nativo pasa.
	if err := ftsIntegrityErr(e); err != nil {
		t.Fatalf("un índice sano debe pasar el integrity-check: %v", err)
	}
	if r := checkFTS(e); r.Status != "ok" {
		t.Errorf("índice sano ⇒ ok, obtuve %s (%s)", r.Status, r.Message)
	}

	// Desincronización (sin corrupción): una fila de más en el FTS ⇒ drift ⇒ reparable.
	if _, err := e.db.Exec(`INSERT INTO observations_fts(id, topic_key, content) VALUES ('fantasma','t','huerfano')`); err != nil {
		t.Fatal(err)
	}
	r := checkFTS(e)
	if r.Status == "ok" || !r.Repairable {
		t.Errorf("un drift de COUNT debe detectarse y ser reparable, obtuve %+v", r)
	}

	// Y la reparación lo cura.
	if _, err := applyRebuildFTS(e); err != nil {
		t.Fatal(err)
	}
	if r := checkFTS(e); r.Status != "ok" {
		t.Errorf("tras reparar, el check debe volver a ok, obtuve %s (%s)", r.Status, r.Message)
	}
}

// F.e / R9 — el camino feliz no cambia: con una base sana, el backup usa VACUUM INTO y produce un
// archivo válido.
func TestBackupUsesVacuumIntoOnHealthyDB(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("a", "t", "contenido", nil); err != nil {
		t.Fatal(err)
	}
	dest := t.TempDir()
	path, err := e.BackupTo(dest)
	if err != nil {
		t.Fatalf("BackupTo sobre una base sana: %v", err)
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("el backup debe existir y no estar vacío: %v", err)
	}
	if !strings.HasPrefix(filepath.Base(path), "memory.db.") {
		t.Errorf("nombre de backup inesperado: %s", path)
	}
}

// F.d / R7-R8 — el backup DE RESCATE: copiar los bytes crudos NO parsea páginas, así que funciona
// donde VACUUM INTO no puede. Se ejercita rawCopyDB directo (forzar una corrupción real de páginas
// de forma portable no es viable; el fix se ataca POR CONSTRUCCIÓN — no leer lo roto — y acá se
// verifica que el mecanismo de rescate produce el archivo).
func TestRawCopyBackupProducesFileWithoutParsingPages(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("a", "t", "contenido", nil); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "rescate.db")

	if err := rawCopyDB(e.path, dest); err != nil {
		t.Fatalf("la copia cruda de rescate debe funcionar: %v", err)
	}
	fi, err := os.Stat(dest)
	if err != nil || fi.Size() == 0 {
		t.Fatalf("la copia de rescate debe existir y no estar vacía: %v", err)
	}

	// El -wal se copia si existe: sin él, la copia del .db podría quedar sin los commits recientes.
	if _, err := os.Stat(e.path + "-wal"); err == nil {
		if _, err := os.Stat(dest + "-wal"); err != nil {
			t.Error("si hay -wal, la copia de rescate debe incluirlo (si no, faltarían los commits recientes)")
		}
	}
}

// R10 — si el origen no existe, la copia cruda falla con error (no un backup silenciosamente ausente).
func TestRawCopyFailsLoudlyOnMissingSource(t *testing.T) {
	if err := rawCopyDB(filepath.Join(t.TempDir(), "no-existe.db"), filepath.Join(t.TempDir(), "x.db")); err == nil {
		t.Error("un origen inexistente debe fallar con error, no producir un backup vacío en silencio")
	}
}
