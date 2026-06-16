package memory

import (
	"database/sql"
	"fmt"
	"musubi/internal/config"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type DbEngine struct {
	db   *sql.DB
	path string // ruta del archivo SQLite (para backups del doctor)
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

	engine := &DbEngine{db: db, path: dbPath}
	if err := engine.initSchema(); err != nil {
		db.Close()
		return nil, err
	}
	// Migración aditiva: agrega las columnas de eficiencia de memoria a bases
	// preexistentes y backfillea gist/tokens/content_hash de filas viejas.
	if err := engine.migrateObservations(); err != nil {
		db.Close()
		return nil, err
	}
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
	rows, err := e.db.Query(`PRAGMA table_info(observations)`)
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
	return cols, nil
}

// migrateObservations agrega de forma idempotente las columnas de eficiencia de
// memoria (gist, content_hash, tokens, last_accessed, access_count, importance)
// y el índice por content_hash. SQLite ADD COLUMN no reescribe la tabla.
func (e *DbEngine) migrateObservations() error {
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
	existing, err := e.observationColumns()
	if err != nil {
		return err
	}
	for _, c := range wanted {
		if existing[c.name] {
			continue
		}
		if _, err := e.db.Exec("ALTER TABLE observations ADD COLUMN " + c.ddl); err != nil {
			return fmt.Errorf("error al migrar columna %s: %w", c.name, err)
		}
	}
	if _, err := e.db.Exec(`CREATE INDEX IF NOT EXISTS idx_obs_hash ON observations(content_hash)`); err != nil {
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

func (e *DbEngine) Close() error {
	return e.db.Close()
}

func (e *DbEngine) initSchema() error {
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
	}

	for _, query := range queries {
		if _, err := e.db.Exec(query); err != nil {
			return fmt.Errorf("error ejecutando migración de esquema: %w", err)
		}
	}
	return nil
}
