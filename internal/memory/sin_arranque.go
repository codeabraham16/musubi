package memory

// El engine SIN TRABAJO DE ARRANQUE: para un proceso que vive milisegundos y hace dos consultas.
//
// PARA QUÉ. `musubi precheck --hook-mode` corre antes de CADA Read y de CADA edición, y abría la
// base con NewDbEngine, que es la apertura de un proceso que se queda: migra, backfillea digests,
// recomputa tokens, siembra el outbox y lanza el entrenamiento del índice vectorial en segundo
// plano — y `Close` ESPERA a esa goroutine. Medido en 15 días de transcripts: p50 280 ms, p99
// 5,5 s y 20 `hook_cancelled` de entre 12,9 y 20,9 s, algunos con «outbox sembrado» en stderr.
// Ninguno de esos pasos le sirve a un hook que lee un gist y suma una línea al ledger.
//
// POR QUÉ NO NewDbEngineSoloLectura. Porque el hook ESCRIBE una cosa: el ledger de tokens
// (`precheck_code`, `precheck_telemetry`, `precheck_codegraph`, `precheck_impacto` y la marca de
// una-vez-por-sesión de `precheck_sin_grafo`). En sólo lectura `LedgerAdd` no escribe
// (ledger.go) y esas superficies desaparecerían del gasto medido sin un solo error. Dos engines —uno
// de lectura y otro para el ledger— son dos pools y dos aperturas para ahorrar nada: el costo no
// estaba en poder escribir, estaba en lo que la apertura normal hace ANTES de devolver.
//
// LO QUE NO HACE, Y POR QUÉ CADA UNO.
//   - No crea `.musubi` ni la base. Un hook que materializa una base vacía en cualquier directorio
//     donde alguien lee un archivo deja un engine sano con cero memoria, que es la falla que se lee
//     como un dato. Sin base, ErrBaseAusente, y el llamador calla.
//   - No migra. Migrar es trabajo del daemon, que abre una vez por sesión; hacerlo desde el hook le
//     cambia el esquema POR DEBAJO a un daemon viejo que ya está corriendo (un binario nuevo
//     instalado a mitad de sesión), y el próximo arranque de ese daemon choca con ErrSchemaTooNew.
//     Una base atrasada se abre igual: la consulta que pida una columna nueva falla sola y esa
//     superficie calla, que es lo mismo que hacía el hook cuando no podía abrir.
//   - No backfillea digests ni recomputa tokens, no siembra el outbox y no entrena el índice
//     vectorial: son mantenimiento, y el próximo arranque del daemon los hace igual.
//
// LO QUE SÍ CONSERVA. El DSN del engine normal (WAL, `_txlock=immediate`, busy_timeout: el ledger
// escribe mientras el daemon escribe), la guarda de esquema MÁS NUEVO (fail-closed, como siempre)
// y los divisores calibrados, para que los tokens del ledger se estimen igual que antes.

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"musubi/internal/config"
)

// ErrBaseAusente dice que el proyecto no tiene `.musubi/memory.db`. Es un error distinto de «no
// pude abrirla» porque se contesta distinto: acá no hay nada que avisar, el proyecto no usa Musubi.
var ErrBaseAusente = errors.New("el proyecto no tiene base de Musubi")

// NewDbEngineSinArranque abre una base EXISTENTE sin crearla, sin migrarla y sin ninguno de los
// pasos de mantenimiento de NewDbEngine. Ver el encabezado del archivo.
func NewDbEngineSinArranque(projectPath string) (*DbEngine, error) {
	dbPath := filepath.Join(projectPath, config.DirName, config.DBFile)
	// Stat ANTES de sql.Open: el driver crea el archivo al primer uso, y con eso la guarda de
	// «no crear nada» sería decorativa.
	if fi, err := os.Stat(dbPath); err != nil || fi.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrBaseAusente, dbPath)
	}

	// El mismo DSN que NewDbEngine, a propósito: ver el comentario largo de database.go sobre
	// `_txlock=immediate`. El ledger es un leer-y-después-escribir, justo el patrón que lo necesita.
	dsn := dbPath + "?_txlock=immediate&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("error al abrir la base sin arranque: %w", err)
	}
	// Un proceso de una consulta por vez: dos conexiones alcanzan y no dejan fds colgados.
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)

	var actual int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&actual); err != nil {
		db.Close()
		return nil, fmt.Errorf("error al leer el esquema de %s: %w", dbPath, err)
	}
	// La guarda hacia adelante se conserva igual que en applyMigrations: un binario más viejo que la
	// base no opera sobre columnas que no entiende. Hacia atrás no se migra (ver el encabezado).
	if ultima := latestSchemaVersion(); actual > ultima {
		db.Close()
		return nil, fmt.Errorf("%w: la base está en el esquema v%d pero este binario solo llega a v%d", ErrSchemaTooNew, actual, ultima)
	}

	// outboxEnabled en false explícito: este engine no guarda observaciones, y si alguien lo usara
	// para eso no debe encolar desde un proceso que no decidió la política de sync.
	engine := &DbEngine{db: db, path: dbPath, outboxEnabled: false}
	if err := engine.applyCalibratedDivisors(); err != nil {
		db.Close()
		return nil, fmt.Errorf("error al aplicar los divisores calibrados de %s: %w", dbPath, err)
	}
	return engine, nil
}
