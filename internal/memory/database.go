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

	return engine, nil
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
	}

	for _, query := range queries {
		if _, err := e.db.Exec(query); err != nil {
			return fmt.Errorf("error ejecutando migración de esquema: %w", err)
		}
	}
	return nil
}
