package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

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
	// Este «sigue en nil» sólo mide algo si con esta misma base y esta misma config el engine normal
	// SÍ lo arma: el cero de un puntero es nil, y con el índice apagado por config (o sin armar en
	// NewDbEngine) este assert sería verde sin haber probado nada. Por eso el control lo exige abajo.
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
		// Sabotaje: saltear `engine.index = newIVFIndex()` en NewDbEngine → rojo acá.
		if normal.index == nil {
			t.Fatal("el control no armó el índice vectorial: el «no tenía que armar el índice» de arriba no estaría midiendo nada")
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

// SA5 — EL LEDGER ESPERA AL OTRO ESCRITOR Y PERSISTE.
//
// El hook escribe el ledger mientras el daemon escribe, y precheck.go descarta el error de
// LedgerAdd: una escritura perdida por SQLITE_BUSY no se ve en ningún lado. El DSN del engine
// liviano era una COPIA del de NewDbEngine, y sacarle `busy_timeout` dejaba todo verde (X1–X3 miden
// NewDbEngine, no este engine). Sabotaje: sacar `busy_timeout(5000)` de dsnEscribible → rojo acá, con
// «database is locked» al instante.
//
// NO PASA POR TIMING, y por eso tiene tres piezas y no una:
//   - El otro escritor abre SU PROPIA conexión con un DSN a mano, sin dsnEscribible: si usara el
//     compartido, el sabotaje también le cambiaría el lock a él y el caso mediría otra cosa.
//   - Antes de llamar a LedgerAdd se COMPRUEBA que el lock está tomado: un tercero sin espera tiene
//     que chocar. Sin esto, un LedgerAdd «que esperó» a un lock que nunca se tomó pasaría verde.
//   - El piso de espera se calibra contra una escritura libre en esta máquina, como en X3.
func TestSinArranqueElLedgerEsperaAlOtroEscritor(t *testing.T) {
	dir := dirSembrado(t)
	dbPath := filepath.Join(dir, config.DirName, config.DBFile)
	eng, err := NewDbEngineSinArranque(dir)
	if err != nil {
		t.Fatalf("NewDbEngineSinArranque: %v", err)
	}
	defer eng.Close()

	// Calentamiento y calibración: cuánto tarda LedgerAdd sin nadie enfrente.
	if _, err := eng.LedgerAdd("sesion-sa5", "precheck_code", 1); err != nil {
		t.Fatalf("calentamiento: %v", err)
	}
	inicioLibre := time.Now()
	if _, err := eng.LedgerAdd("sesion-sa5", "precheck_code", 1); err != nil {
		t.Fatalf("calibración: %v", err)
	}
	libre := time.Since(inicioLibre)

	ctx := context.Background()
	otro, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("abrir el otro escritor: %v", err)
	}
	defer otro.Close()
	conn, err := otro.Conn(ctx) // una conexión fija: BEGIN y COMMIT tienen que ir por la misma
	if err != nil {
		t.Fatalf("conexión del otro escritor: %v", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		t.Fatalf("el otro escritor no tomó el lock: %v", err)
	}

	tercero, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatalf("abrir la sonda: %v", err)
	}
	defer tercero.Close()
	sonda, err := tercero.Conn(ctx)
	if err != nil {
		t.Fatalf("conexión de la sonda: %v", err)
	}
	defer sonda.Close()
	if _, err := sonda.ExecContext(ctx, `BEGIN IMMEDIATE`); !esBaseBloqueada(err) {
		if err == nil {
			_, _ = sonda.ExecContext(ctx, `ROLLBACK`)
		}
		t.Fatalf("la sonda sin espera tenía que chocar contra el lock del otro escritor y obtuvo %v: el lock no está tomado y el caso no probaría ninguna espera", err)
	}

	const retencion = 700 * time.Millisecond
	soltado := make(chan error, 1)
	go func() {
		time.Sleep(retencion)
		_, err := conn.ExecContext(ctx, `COMMIT`)
		soltado <- err
	}()

	inicio := time.Now()
	_, errLedger := eng.LedgerAdd("sesion-sa5", "precheck_impacto", 25)
	esperado := time.Since(inicio)
	if err := <-soltado; err != nil {
		t.Fatalf("el otro escritor no pudo soltar el lock: %v", err)
	}
	if errLedger != nil {
		t.Fatalf("LedgerAdd no esperó al otro escritor (tardó %v): al DSN de dsnEscribible le falta `busy_timeout` y el hook perdería esta escritura callado — %v", esperado, errLedger)
	}
	if piso := libre + retencion/2; esperado < piso {
		t.Errorf("LedgerAdd tardó %v y el piso calibrado era %v (libre: %v, retención: %v): no esperó al otro escritor", esperado, piso, libre, retencion)
	}

	otra, err := NewDbEngineSinArranque(dir)
	if err != nil {
		t.Fatalf("reabrir: %v", err)
	}
	defer otra.Close()
	l, err := otra.LedgerStatus()
	if err != nil {
		t.Fatalf("LedgerStatus: %v", err)
	}
	if l.Surfaces["precheck_impacto"] != 25 || l.Surfaces["precheck_code"] != 2 {
		t.Errorf("lo sumado mientras había otro escritor no persistió: %v", l.Surfaces)
	}
}

// SA6 — UNA BASE ATRASADA SE ABRE Y SIGUE ATRASADA: el hook no migra.
//
// Migrar desde el hook le cambia el esquema por debajo a un daemon viejo que ya está corriendo (ver
// el encabezado de sin_arranque.go). Ningún caso lo miraba, y no por descuido visible:
// SembrarPlantillaDePruebas deja la base en la ÚLTIMA versión, donde migrar es un no-op, así que un
// runMigrations metido en NewDbEngineSinArranque pasaba verde. Por eso acá la base se baja a
// latest-1 antes de abrirla. Sabotaje: llamar runMigrations(db) después de SetMaxIdleConns → rojo.
//
// El CONTROL abre la misma base con NewDbEngine y verifica que ésa sí sube a latest: sin él,
// «quedó en latest-1» también sería verde si la migración no pudiera correr sobre esta base.
func TestSinArranqueNoMigraUnaBaseAtrasada(t *testing.T) {
	dir := dirSembrado(t)
	anterior := latestSchemaVersion() - 1
	eng, err := NewDbEngineSinArranque(dir)
	if err != nil {
		t.Fatalf("NewDbEngineSinArranque: %v", err)
	}
	if _, err := eng.db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, anterior)); err != nil {
		t.Fatalf("bajar user_version: %v", err)
	}
	eng.Close()

	atrasada, err := NewDbEngineSinArranque(dir)
	if err != nil {
		t.Fatalf("una base atrasada tiene que abrirse igual (la consulta que pida una columna nueva falla sola): %v", err)
	}
	v := versionDeEsquema(t, atrasada.db)
	atrasada.Close()
	if v != anterior {
		t.Fatalf("abrir con el engine del hook llevó la base de v%d a v%d: migró, y el hook no migra", anterior, v)
	}

	t.Run("control: NewDbEngine sí la migra", func(t *testing.T) {
		normal, err := NewDbEngine(dir)
		if err != nil {
			t.Fatalf("NewDbEngine sobre la base atrasada: %v", err)
		}
		defer normal.Close()
		if v := versionDeEsquema(t, normal.db); v != latestSchemaVersion() {
			t.Fatalf("el control dejó la base en v%d y no en v%d: la prueba de arriba no estaría midiendo nada", v, latestSchemaVersion())
		}
	})
}

func versionDeEsquema(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		t.Fatalf("leer user_version: %v", err)
	}
	return v
}
