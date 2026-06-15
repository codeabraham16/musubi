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
	db *sql.DB
}

func NewDbEngine(projectPath string) (*DbEngine, error) {
	dbPath := filepath.Join(projectPath, config.DirName, config.DBFile)

	// Asegurar que el directorio .musubi existe
	err := os.MkdirAll(filepath.Dir(dbPath), 0755)
	if err != nil {
		return nil, fmt.Errorf("no se pudo crear la carpeta .musubi: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error al abrir la base de datos: %w", err)
	}

	engine := &DbEngine{db: db}
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
	if err := engine.backfillDigests(); err != nil {
		db.Close()
		return nil, err
	}

	return engine, nil
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
