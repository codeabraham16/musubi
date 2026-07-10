package memory

import (
	"context"
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

// SaveCodeMemory inserta o actualiza el gist de un archivo, atribuido al project_id del engine
// (backward-compat / federado si ''). Ver SaveCodeMemoryFrom.
func (e *DbEngine) SaveCodeMemory(cm CodeMemory) error {
	return e.SaveCodeMemoryFrom("", cm)
}

// SaveCodeMemoryFrom guarda con el project_id de ORIGEN explícito (atribución multi-tenant,
// Track 17). origin == "" ⇒ project_id del engine. El UPSERT es por (path, project_id): dos
// proyectos con el mismo path YA NO se pisan el gist (antes PRIMARY KEY(path) colisionaba).
func (e *DbEngine) SaveCodeMemoryFrom(originProjectID string, cm CodeMemory) error {
	if cm.Path == "" || cm.Gist == "" {
		return fmt.Errorf("path y gist son obligatorios")
	}
	projectID := originProjectID
	if projectID == "" {
		projectID = e.projectID
	}
	_, err := e.db.Exec(
		`INSERT INTO code_memory (path, gist, symbols, fingerprint, tokens, project_id, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(path, project_id) DO UPDATE SET
		   gist=excluded.gist, symbols=excluded.symbols,
		   fingerprint=excluded.fingerprint, tokens=excluded.tokens,
		   updated_at=CURRENT_TIMESTAMP`,
		cm.Path, cm.Gist, cm.Symbols, cm.Fingerprint, cm.Tokens, projectID,
	)
	if err != nil {
		return fmt.Errorf("error al guardar memoria de código: %w", err)
	}
	return nil
}

// GetCodeMemory devuelve el gist guardado de un archivo (federado; ok=false si no existe).
func (e *DbEngine) GetCodeMemory(path string) (CodeMemory, bool, error) {
	return e.GetCodeMemoryCtx(context.Background(), path)
}

// GetCodeMemoryCtx acota la lectura al proyecto de la credencial (ctx, Track 17): con scope,
// solo el gist del proyecto pedido o el sin atribuir (project_id=''), PREFIRIENDO el del proyecto
// sobre el sin atribuir. Ausencia de scope ⇒ federado (la primera fila del path).
func (e *DbEngine) GetCodeMemoryCtx(ctx context.Context, path string) (CodeMemory, bool, error) {
	sc := projectScopeFrom(ctx)
	var cm CodeMemory
	var row *sql.Row
	if sc.Federate || sc.ProjectID == "" {
		row = e.db.QueryRowContext(ctx,
			`SELECT path, gist, COALESCE(symbols,''), COALESCE(fingerprint,''), tokens
			 FROM code_memory WHERE path = ? LIMIT 1`, path)
	} else {
		row = e.db.QueryRowContext(ctx,
			`SELECT path, gist, COALESCE(symbols,''), COALESCE(fingerprint,''), tokens
			 FROM code_memory WHERE path = ? AND (project_id = ? OR project_id = '')
			 ORDER BY (project_id = ?) DESC LIMIT 1`, path, sc.ProjectID, sc.ProjectID)
	}
	err := row.Scan(&cm.Path, &cm.Gist, &cm.Symbols, &cm.Fingerprint, &cm.Tokens)
	if err == sql.ErrNoRows {
		return CodeMemory{}, false, nil
	}
	if err != nil {
		return CodeMemory{}, false, fmt.Errorf("error al leer memoria de código: %w", err)
	}
	return cm, true, nil
}
