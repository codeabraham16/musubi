package memory

import (
	"database/sql"
	"fmt"
)

// codemem.go implementa la MEMORIA DE CÓDIGO: un gist (titular) + símbolos de un
// archivo ya leído, indexados por path, con un fingerprint del contenido. Permite
// recordar la estructura de un archivo sin re-leerlo entero — el mayor costo en
// tokens de una sesión de agente no es la memoria de Musubi sino re-leer archivos.
// Es model-free: el agente provee el gist; Musubi lo guarda y rastrea su frescura.
// El fingerprint lo computa la capa MCP (que tiene acceso al filesystem del
// proyecto); el motor solo persiste y compara.

// CodeMemory es el gist persistido de un archivo de código.
type CodeMemory struct {
	Path        string `json:"path"`
	Gist        string `json:"gist"`
	Symbols     string `json:"symbols"`
	Fingerprint string `json:"fingerprint"`
	Tokens      int    `json:"tokens"`
}

// SaveCodeMemory inserta o actualiza (UPSERT por path) el gist de un archivo.
func (e *DbEngine) SaveCodeMemory(cm CodeMemory) error {
	if cm.Path == "" || cm.Gist == "" {
		return fmt.Errorf("path y gist son obligatorios")
	}
	_, err := e.db.Exec(
		`INSERT INTO code_memory (path, gist, symbols, fingerprint, tokens, updated_at)
		 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(path) DO UPDATE SET
		   gist=excluded.gist, symbols=excluded.symbols,
		   fingerprint=excluded.fingerprint, tokens=excluded.tokens,
		   updated_at=CURRENT_TIMESTAMP`,
		cm.Path, cm.Gist, cm.Symbols, cm.Fingerprint, cm.Tokens,
	)
	if err != nil {
		return fmt.Errorf("error al guardar memoria de código: %w", err)
	}
	return nil
}

// GetCodeMemory devuelve el gist guardado de un archivo (ok=false si no existe).
func (e *DbEngine) GetCodeMemory(path string) (CodeMemory, bool, error) {
	var cm CodeMemory
	err := e.db.QueryRow(
		`SELECT path, gist, COALESCE(symbols,''), COALESCE(fingerprint,''), tokens
		 FROM code_memory WHERE path = ?`, path,
	).Scan(&cm.Path, &cm.Gist, &cm.Symbols, &cm.Fingerprint, &cm.Tokens)
	if err == sql.ErrNoRows {
		return CodeMemory{}, false, nil
	}
	if err != nil {
		return CodeMemory{}, false, fmt.Errorf("error al leer memoria de código: %w", err)
	}
	return cm, true, nil
}
