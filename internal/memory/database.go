// Package memory es el núcleo de persistencia de Musubi: el motor SQLite local-first
// (observaciones, embeddings, grafo, workflows), el recall por presupuesto de tokens,
// el índice vectorial IVF y el mantenimiento (consolidación, olvido, retención). Todo
// model-free.
package memory

import (
	"database/sql"
	"fmt"
	"musubi/internal/config"
	"musubi/internal/logx"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

type DbEngine struct {
	db   *sql.DB
	path string // ruta del archivo SQLite (para backups del doctor)
	// index es el índice vectorial IVF para búsqueda semántica a escala (nil si
	// está desactivado por config). Es un caché reconstruible desde SQLite.
	index *ivfIndex
	// vindexCfg es la config del índice vectorial. Se fija UNA vez en NewDbEngine;
	// las goroutines de fondo reciben una COPIA (snapshot) y nunca leen este campo,
	// para no acceder a él de forma concurrente.
	vindexCfg config.VectorIndexConfig
	// rebuilding es el guard que evita rebuilds del índice solapados.
	rebuilding atomic.Bool
	// lifecycleMu + closed + bgWG coordinan el cierre con las goroutines de fondo:
	// una vez `closed`, spawnBackground no lanza nada y Close espera (bgWG) a las
	// goroutines en vuelo antes de cerrar la base. Evita use-after-close del *sql.DB.
	lifecycleMu sync.Mutex
	closed      bool
	bgWG        sync.WaitGroup
	// projectID es el proyecto de origen que se estampa en cada observación guardada
	// (columna project_id) para la memoria híbrida local+central. Lo inyecta el
	// entrypoint tras cargar la config (ver SetProjectID). "" = sin atribución (NULL-like).
	projectID string
}

// SetProjectID fija el proyecto de origen que saveObservation estampa en cada
// observación. Lo llama el entrypoint (serve/daemon) tras resolver el project_id de la
// config o del directorio del workspace. Idempotente y barato; no toca la base.
func (e *DbEngine) SetProjectID(id string) { e.projectID = id }

// spawnBackground lanza f como goroutine RASTREADA por bgWG, salvo que el engine ya
// esté cerrado. Devuelve true si la lanzó. El registro en bgWG ocurre bajo
// lifecycleMu (igual que Close marca `closed`), así que es imposible que una
// goroutine arranque después de que Close empezó a esperar (no hay Add-after-Wait).
func (e *DbEngine) spawnBackground(f func()) bool {
	e.lifecycleMu.Lock()
	defer e.lifecycleMu.Unlock()
	if e.closed {
		return false
	}
	e.bgWG.Add(1)
	go func() {
		defer e.bgWG.Done()
		f()
	}()
	return true
}

func NewDbEngine(projectPath string) (*DbEngine, error) {
	dbPath := filepath.Join(projectPath, config.DirName, config.DBFile)

	// Asegurar que el directorio .musubi existe
	err := os.MkdirAll(filepath.Dir(dbPath), 0755)
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear la carpeta .musubi: %w", err)
	}

	// WAL + busy_timeout vía DSN para que apliquen a TODA conexión del pool:
	// lectores concurrentes y un escritor a la vez, con espera en vez de
	// "database is locked". Necesario para que varios sub-agentes coordinen sobre
	// la misma pizarra (work_units) sin colisionar.
	dsn := dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("error al abrir la base de datos: %w", err)
	}

	// Tuning explícito del pool: WAL admite muchos lectores + un escritor. Acotar las
	// conexiones abiertas evita abrir demasiadas bajo concurrencia alta (cada una es un
	// fd + memoria); el escritor se serializa de todos modos. Idle bajo libera recursos
	// en reposo. Sin esto regían los defaults de database/sql (sin tope efectivo).
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)
	db.SetConnMaxIdleTime(5 * time.Minute)

	// Esquema versionado (PRAGMA user_version): runMigrations aplica las migraciones
	// pendientes, cada una en su propia transacción. La migración baseline crea el
	// esquema y agrega las columnas de eficiencia de memoria; es idempotente sobre
	// bases preexistentes (todo CREATE ... IF NOT EXISTS / ADD COLUMN guardado), así
	// que una base ya migrada solo avanza su user_version sin reescribir nada.
	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, err
	}

	engine := &DbEngine{db: db, path: dbPath}
	// Aplicar los divisores de tokens calibrados de esta DB (o defaults) ANTES de
	// backfillear/recomputar, para que los tokens se calculen con el divisor activo.
	if err := engine.applyCalibratedDivisors(); err != nil {
		db.Close()
		return nil, err
	}
	if err := engine.backfillDigests(); err != nil {
		db.Close()
		return nil, err
	}
	// Si el estimador de tokens cambió de versión, recomputar la columna `tokens`
	// de las filas existentes (el presupuesto se mide con el estimador actual).
	if err := engine.recomputeTokensIfEstimatorChanged(); err != nil {
		db.Close()
		return nil, err
	}

	// Cerebro híbrido F2: sembrar el OUTBOX para las observaciones 'shared' que aún no tienen
	// fila (las creadas en F1 antes de que existiera el outbox, o promovidas con el sync
	// apagado). Best-effort: un fallo acá NO es fatal (igual que el backfill de digests): la
	// DB sigue usable y el próximo arranque reintenta; a lo sumo se demora una sincronización.
	if seeded, bErr := engine.BackfillOutbox(); bErr != nil {
		logx.Warn("no se pudo sembrar el outbox al abrir la base", "error", bErr)
	} else if seeded > 0 {
		logx.Info("outbox sembrado con observaciones compartidas preexistentes", "sembradas", seeded)
	}

	// Índice vectorial IVF para búsqueda semántica a escala (T1.2). Se configura
	// desde .musubi/config.yaml (defaults si está ausente). El índice es un caché
	// reconstruible desde SQLite; si ya hay suficientes embeddings se entrena en
	// segundo plano para no demorar el arranque (las búsquedas caen al full-scan
	// exacto hasta que esté listo).
	cfg, _ := config.Load(projectPath)
	engine.vindexCfg = cfg.VectorIndex
	if engine.vindexCfg.Enabled {
		engine.index = newIVFIndex()
		// Snapshot de la config para la goroutine de fondo: no debe leer engine.vindexCfg
		// (los tests pueden ajustarlo después de construir, lo que sería una carrera).
		vcfg := engine.vindexCfg
		engine.spawnBackground(func() { engine.autoBuildVectorIndex(vcfg) })
	}

	return engine, nil
}

// recomputeTokensIfEstimatorChanged recomputa gist/tokens de TODAS las filas
// cuando la versión del estimador difiere de la guardada en meta, y actualiza la
// marca. Es idempotente: si la versión coincide, no hace nada.
func (e *DbEngine) recomputeTokensIfEstimatorChanged() error {
	v, _, err := e.GetMeta(metaTokenEstimatorVersion)
	if err != nil {
		return err
	}
	if v == tokenEstimatorVersion {
		return nil
	}
	if err := e.RecomputeTokens(); err != nil {
		return err
	}
	return e.SetMeta(metaTokenEstimatorVersion, tokenEstimatorVersion)
}

// observationColumns devuelve el conjunto de columnas presentes hoy en la tabla
// observations (vía PRAGMA table_info), para hacer la migración idempotente.
func (e *DbEngine) observationColumns() (map[string]bool, error) {
	return observationColumnsOn(e.db)
}

// observationColumnsOn lee las columnas de observations sobre cualquier ejecutor
// (conexión o transacción), para que la migración baseline funcione dentro de tx.
func observationColumnsOn(x execQuerier) (map[string]bool, error) {
	rows, err := x.Query(`PRAGMA table_info(observations)`)
	if err != nil {
		return nil, fmt.Errorf("error al leer columnas de observations: %w", err)
	}
	defer rows.Close()

	cols := map[string]bool{}
	for rows.Next() {
		var (
			cid         int
			name, ctype string
			notnull, pk int
			dflt        interface{}
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("error al escanear PRAGMA table_info: %w", err)
		}
		cols[name] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar columnas del esquema: %w", err)
	}
	return cols, nil
}

// migrateObservations agrega de forma idempotente las columnas de eficiencia de
// memoria y el índice por content_hash. Se conserva como método para el auto-repair
// del doctor (doctor.go); la migración baseline usa addObservationColumns directamente.
func (e *DbEngine) migrateObservations() error {
	return addObservationColumns(e.db)
}

// addObservationColumns agrega de forma idempotente las columnas de eficiencia de
// memoria (gist, content_hash, tokens, last_accessed, access_count, importance,
// archived, superseded_by) y el índice por content_hash, sobre cualquier ejecutor.
// SQLite ADD COLUMN no reescribe la tabla.
func addObservationColumns(x execQuerier) error {
	wanted := []struct{ name, ddl string }{
		{"gist", "gist TEXT"},
		{"content_hash", "content_hash TEXT"},
		{"tokens", "tokens INTEGER NOT NULL DEFAULT 0"},
		{"last_accessed", "last_accessed DATETIME"},
		{"access_count", "access_count INTEGER NOT NULL DEFAULT 0"},
		{"importance", "importance REAL NOT NULL DEFAULT 1.0"},
		{"archived", "archived INTEGER NOT NULL DEFAULT 0"},
		{"superseded_by", "superseded_by TEXT"},
	}
	existing, err := observationColumnsOn(x)
	if err != nil {
		return err
	}
	for _, c := range wanted {
		if existing[c.name] {
			continue
		}
		if _, err := x.Exec("ALTER TABLE observations ADD COLUMN " + c.ddl); err != nil {
			return fmt.Errorf("error al migrar columna %s: %w", c.name, err)
		}
	}
	if _, err := x.Exec(`CREATE INDEX IF NOT EXISTS idx_obs_hash ON observations(content_hash)`); err != nil {
		return fmt.Errorf("error al crear índice content_hash: %w", err)
	}
	return nil
}

// backfillDigests calcula gist/content_hash/tokens (model-free) para las filas
// que aún no los tienen, en una sola pasada. Idempotente.
func (e *DbEngine) backfillDigests() error {
	rows, err := e.db.Query(`SELECT id, content FROM observations WHERE gist IS NULL OR gist = ''`)
	if err != nil {
		return fmt.Errorf("error al consultar filas para backfill: %w", err)
	}
	type pendiente struct{ id, content string }
	var pendientes []pendiente
	for rows.Next() {
		var p pendiente
		if err := rows.Scan(&p.id, &p.content); err != nil {
			rows.Close()
			return fmt.Errorf("error al escanear backfill: %w", err)
		}
		pendientes = append(pendientes, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("error al iterar filas para backfill: %w", err)
	}
	rows.Close()

	for _, p := range pendientes {
		if _, err := e.db.Exec(
			`UPDATE observations SET gist=?, content_hash=?, tokens=? WHERE id=?`,
			Gist(p.content, defaultGistMaxTokens), ContentHash(p.content), EstimateTokens(p.content), p.id,
		); err != nil {
			return fmt.Errorf("error al backfillear %s: %w", p.id, err)
		}
	}
	return nil
}

// Close marca el engine como cerrado (para que no se lancen nuevas goroutines de
// fondo), espera a que terminen las que estén en vuelo (rebuilds del índice) y recién
// entonces cierra la base. Así ninguna goroutine consulta un *sql.DB ya cerrado.
func (e *DbEngine) Close() error {
	e.lifecycleMu.Lock()
	e.closed = true
	e.lifecycleMu.Unlock()
	e.bgWG.Wait()
	return e.db.Close()
}

// initSchema crea el esquema base sobre la conexión del engine. Se conserva como
// método para los tests de idempotencia; la migración baseline usa initSchemaOn.
func (e *DbEngine) initSchema() error {
	return initSchemaOn(e.db)
}

// initSchemaOn crea todas las tablas/índices/triggers base sobre cualquier ejecutor
// (conexión o transacción). Todo es CREATE ... IF NOT EXISTS, así que es idempotente.
func initSchemaOn(x execQuerier) error {
	queries := []string{
		// Tabla de observaciones de engram
		`CREATE TABLE IF NOT EXISTS observations (
			id TEXT PRIMARY KEY,
			topic_key TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// Tabla de embeddings vectoriales
		`CREATE TABLE IF NOT EXISTS embeddings (
			observation_id TEXT PRIMARY KEY,
			vector BLOB NOT NULL,
			FOREIGN KEY(observation_id) REFERENCES observations(id) ON DELETE CASCADE
		);`,

		// Tabla de logs de compilación/telemetría para feedback
		`CREATE TABLE IF NOT EXISTS telemetry_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			file_path TEXT NOT NULL,
			error_message TEXT NOT NULL,
			suggested_patch TEXT,
			resolved INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// Tabla virtual FTS5 para búsqueda rápida sin embeddings
		`CREATE VIRTUAL TABLE IF NOT EXISTS observations_fts USING fts5(
			id UNINDEXED,
			topic_key UNINDEXED,
			content
		);`,

		// Triggers para mantener FTS5 sincronizado automáticamente
		`CREATE TRIGGER IF NOT EXISTS observations_ai AFTER INSERT ON observations BEGIN
			INSERT OR REPLACE INTO observations_fts(id, topic_key, content) VALUES (new.id, new.topic_key, new.content);
		END;`,

		`CREATE TRIGGER IF NOT EXISTS observations_ad AFTER DELETE ON observations BEGIN
			DELETE FROM observations_fts WHERE id = old.id;
		END;`,

		// El UPSERT de saveObservation (y cualquier UPDATE, ej. bumpAccess) actualiza
		// filas existentes; este trigger mantiene el índice FTS sincronizado.
		// DELETE+INSERT (no INSERT OR REPLACE): observations_fts no tiene clave única
		// en id, así que un OR REPLACE duplicaría la fila en cada UPDATE.
		// Se recrea para reemplazar la versión previa (con bug) en bases existentes.
		`DROP TRIGGER IF EXISTS observations_au;`,
		`CREATE TRIGGER observations_au AFTER UPDATE ON observations BEGIN
			DELETE FROM observations_fts WHERE id = old.id;
			INSERT INTO observations_fts(id, topic_key, content) VALUES (new.id, new.topic_key, new.content);
		END;`,

		// Memoria de código: gist + símbolos de un archivo ya leído, con un
		// fingerprint del contenido para saber si sigue fresco. Permite recordar la
		// estructura de un archivo sin re-leerlo entero (ahorro de tokens).
		`CREATE TABLE IF NOT EXISTS code_memory (
			path TEXT PRIMARY KEY,
			gist TEXT NOT NULL,
			symbols TEXT,
			fingerprint TEXT,
			tokens INTEGER NOT NULL DEFAULT 0,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// Grafo de conocimiento: entidades (nodos) deduplicadas por nombre normalizado.
		`CREATE TABLE IF NOT EXISTS entities (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			norm TEXT NOT NULL UNIQUE,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// Grafo de conocimiento: relaciones (aristas) sujeto-predicado-objeto, únicas.
		`CREATE TABLE IF NOT EXISTS relations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			from_id INTEGER NOT NULL,
			predicate TEXT NOT NULL,
			to_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(from_id, predicate, to_id),
			FOREIGN KEY(from_id) REFERENCES entities(id) ON DELETE CASCADE,
			FOREIGN KEY(to_id) REFERENCES entities(id) ON DELETE CASCADE
		);`,

		// Metadatos clave/valor (ej. timestamp del último auto-mantenimiento).
		`CREATE TABLE IF NOT EXISTS meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// Relaciones semánticas entre observaciones (resolución de conflictos
		// model-free): detecta cuándo una memoria reemplaza/contradice/se relaciona
		// con otra. Único por par (source, target).
		`CREATE TABLE IF NOT EXISTS observation_relations (
			id TEXT PRIMARY KEY,
			source_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			relation TEXT NOT NULL,
			confidence REAL NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			resolved_by TEXT,
			reason TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(source_id, target_id)
		);`,

		// Pizarra compartida del multi-agente: unidades de trabajo que el agente
		// principal postea y los sub-agentes reclaman (claim atómico), ejecutan y
		// completan. Es el ÚNICO canal de coordinación entre agentes (no comparten
		// conversación). Coordinación determinista, model-free.
		`CREATE TABLE IF NOT EXISTS work_units (
			id TEXT PRIMARY KEY,
			batch_id TEXT NOT NULL,
			seq INTEGER NOT NULL DEFAULT 0,
			title TEXT,
			spec TEXT,
			status TEXT NOT NULL DEFAULT 'open',
			claimed_by TEXT,
			result TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_work_batch_status ON work_units(batch_id, status);`,

		// Tabla de decisiones de skills (log append-only).
		`CREATE TABLE IF NOT EXISTS skill_decisions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			skill_id TEXT NOT NULL,
			name TEXT,
			decision TEXT NOT NULL,
			reason TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,

		// Motor de orquestación DAG (model-free): estado persistido de cada run de
		// workflow. Musubi NO ejecuta los steps; guarda la definición + el estado por
		// step para decir qué está listo y ser resumible entre sesiones.
		`CREATE TABLE IF NOT EXISTS workflow_runs (
			run_id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			definition TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'running',
			step_status TEXT NOT NULL DEFAULT '{}',
			step_results TEXT NOT NULL DEFAULT '{}',
			step_iters TEXT NOT NULL DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
	}

	for _, query := range queries {
		if _, err := x.Exec(query); err != nil {
			return fmt.Errorf("error ejecutando migración de esquema: %w", err)
		}
	}
	return nil
}
