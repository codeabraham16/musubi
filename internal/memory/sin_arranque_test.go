package memory

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// Tests de NewDbEngineSinArranque: el engine del hook PreToolUse. Lo que protegen es que abrir
// para dos consultas NO haga el trabajo de arranque de un proceso que se queda, y que aun así el
// ledger —la única escritura del hook— siga sumando.

// sembrarTrabajoPendiente deja en la base una observación 'shared' SIN gist y SIN fila de outbox:
// exactamente lo que backfillDigests y BackfillOutbox vienen a arreglar al abrir.
func sembrarTrabajoPendiente(t *testing.T, dir string) {
	t.Helper()
	if err := SembrarPlantillaDePruebas(dir); err != nil {
		t.Fatalf("sembrar plantilla: %v", err)
	}
	eng, err := NewDbEngineSinArranque(dir)
	if err != nil {
		t.Fatalf("NewDbEngineSinArranque: %v", err)
	}
	defer eng.Close()
	if _, err := eng.db.Exec(`INSERT INTO observations (id, topic_key, content, scope) VALUES ('pend-1', 't/pendiente', 'una observación compartida sin digest ni outbox', 'shared')`); err != nil {
		t.Fatalf("insertar observación pendiente: %v", err)
	}
}

func gistYOutbox(t *testing.T, e *DbEngine) (gist string, outbox int) {
	t.Helper()
	var g *string
	if err := e.db.QueryRow(`SELECT gist FROM observations WHERE id='pend-1'`).Scan(&g); err != nil {
		t.Fatalf("leer gist: %v", err)
	}
	if g != nil {
		gist = *g
	}
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM outbox WHERE obs_id='pend-1'`).Scan(&outbox); err != nil {
		t.Fatalf("contar outbox: %v", err)
	}
	return gist, outbox
}

// SA1 — sin base no crea nada: ni el archivo ni la carpeta.
func TestSinArranqueNoCreaLaBase(t *testing.T) {
	dir := t.TempDir()
	eng, err := NewDbEngineSinArranque(dir)
	if err == nil {
		eng.Close()
		t.Fatal("sin .musubi/memory.db tenía que devolver error, abrió un engine")
	}
	if !errors.Is(err, ErrBaseAusente) {
		t.Errorf("el error tiene que ser ErrBaseAusente para que el hook calle sin avisar, obtuve %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, config.DirName)); !os.IsNotExist(statErr) {
		t.Errorf("abrir sin base NO puede crear %s (stat: %v)", config.DirName, statErr)
	}
}

// SA2 — con trabajo de arranque pendiente, no lo hace: el gist sigue vacío, el outbox sin sembrar
// y el índice vectorial sin construir. El CONTROL abre la misma base con NewDbEngine y verifica
// que ese trabajo sí ocurre, porque un «sigue vacío» sobre datos que nadie iba a tocar no prueba
// nada.
func TestSinArranqueNoHaceElTrabajoDeArranque(t *testing.T) {
	dir := t.TempDir()
	sembrarTrabajoPendiente(t, dir)

	eng, err := NewDbEngineSinArranque(dir)
	if err != nil {
		t.Fatalf("NewDbEngineSinArranque: %v", err)
	}
	gist, outbox := gistYOutbox(t, eng)
	if gist != "" {
		t.Errorf("no tenía que backfillear digests, y el gist quedó %q", gist)
	}
	if outbox != 0 {
		t.Errorf("no tenía que sembrar el outbox, y hay %d fila(s)", outbox)
	}
	if eng.index != nil {
		t.Error("no tenía que armar el índice vectorial")
	}
	eng.Close()

	t.Run("control: NewDbEngine sí lo hace sobre la misma base", func(t *testing.T) {
		normal, err := NewDbEngine(dir)
		if err != nil {
			t.Fatalf("NewDbEngine: %v", err)
		}
		defer normal.Close()
		gist, outbox := gistYOutbox(t, normal)
		if gist == "" || outbox != 1 {
			t.Fatalf("el control no reprodujo el trabajo de arranque (gist=%q outbox=%d): la prueba de arriba no estaría midiendo nada", gist, outbox)
		}
	})
}

// SA3 — el ledger escribe y persiste. Es la razón para no usar el engine de sólo lectura.
func TestSinArranqueElLedgerSuma(t *testing.T) {
	dir := t.TempDir()
	if err := SembrarPlantillaDePruebas(dir); err != nil {
		t.Fatalf("sembrar plantilla: %v", err)
	}
	eng, err := NewDbEngineSinArranque(dir)
	if err != nil {
		t.Fatalf("NewDbEngineSinArranque: %v", err)
	}
	if _, err := eng.LedgerAdd("sesion-sa3", "precheck_code", 40); err != nil {
		t.Fatalf("LedgerAdd: %v", err)
	}
	eng.Close()

	otra, err := NewDbEngineSinArranque(dir)
	if err != nil {
		t.Fatalf("reabrir: %v", err)
	}
	defer otra.Close()
	l, err := otra.LedgerStatus()
	if err != nil {
		t.Fatalf("LedgerStatus: %v", err)
	}
	if l.SessionID != "sesion-sa3" || l.Surfaces["precheck_code"] != 40 {
		t.Errorf("el ledger no persistió lo sumado: sesión %q, superficies %v", l.SessionID, l.Surfaces)
	}
}

// SA4 — la guarda hacia adelante se conserva: una base más nueva que el binario no se abre.
func TestSinArranqueRechazaUnEsquemaMasNuevo(t *testing.T) {
	dir := t.TempDir()
	if err := SembrarPlantillaDePruebas(dir); err != nil {
		t.Fatalf("sembrar plantilla: %v", err)
	}
	eng, err := NewDbEngineSinArranque(dir)
	if err != nil {
		t.Fatalf("NewDbEngineSinArranque: %v", err)
	}
	if _, err := eng.db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, latestSchemaVersion()+1)); err != nil {
		t.Fatalf("subir user_version: %v", err)
	}
	eng.Close()

	if e2, err := NewDbEngineSinArranque(dir); err == nil {
		e2.Close()
		t.Fatal("abrió una base con esquema más nuevo que el binario")
	} else if !errors.Is(err, ErrSchemaTooNew) {
		t.Errorf("esperaba ErrSchemaTooNew, obtuve %v", err)
	}
}
