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
// (backward-compat / federado si ”). Ver SaveCodeMemoryFrom.
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
// solo el gist del proyecto pedido o el sin atribuir (project_id=”), PREFIRIENDO el del proyecto
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

// AllCodeMemoryCtx devuelve TODOS los gists del proyecto de la credencial, para el push-on-index
// de la federación (Track 20 · F6). Es la contraparte de AllGraphNodesCtx/AllGraphEdgesCtx: hasta
// que existió, el push llevaba nodos y aristas pero NO los gists, y el central quedaba con
// code_memory en CERO — medido el 2026-08-12: 4.862 nodos federados contra 0 gists. Con el central
// vacío, `musubi_recall_code` contra el cerebro compartido no tenía nada que devolver, que es
// justamente la única vía al gist donde el proyecto no tiene hooks.
//
// DEVUELVE UN SOLO GIST POR PATH, prefiriendo el del proyecto sobre el sin atribuir — la misma
// regla de desempate que ya usa GetCodeMemoryCtx. Hace falta porque la tabla admite las dos filas
// (la PK es (path, project_id)) y en una base real conviven: los gists anteriores a la atribución
// multi-tenant quedaron con project_id=” y el mismo archivo volvió a gistearse después con el
// suyo. Medido en altura-erp el 2026-08-12: 25 filas locales, 23 paths distintos, 2 duplicados
// con el viejo de junio y el nuevo de julio.
//
// Sin el desempate explícito los dos se mandaban y ganaba el ÚLTIMO que insertara el receptor
// (ON CONFLICT DO UPDATE), con el orden entre filas de igual path sin definir. En la prueba real
// ganó el correcto, pero por casualidad: bastaba un VACUUM o un plan de consulta distinto para
// federar el gist rancio. Un empate que se resuelve solo hoy es un bug que aparece mañana.
func (e *DbEngine) AllCodeMemoryCtx(ctx context.Context) ([]CodeMemory, error) {
	sc := projectScopeFrom(ctx)
	var rows *sql.Rows
	var err error
	if sc.Federate || sc.ProjectID == "" {
		// Sin scope no hay proyecto que preferir: se desempata por el más recientemente tocado.
		rows, err = e.db.QueryContext(ctx,
			`SELECT path, gist, symbols, fingerprint, tokens FROM (
			   SELECT path, gist, COALESCE(symbols,'') AS symbols,
			          COALESCE(fingerprint,'') AS fingerprint, tokens,
			          ROW_NUMBER() OVER (PARTITION BY path ORDER BY updated_at DESC) AS rn
			   FROM code_memory
			 ) WHERE rn = 1 ORDER BY path`)
	} else {
		rows, err = e.db.QueryContext(ctx,
			`SELECT path, gist, symbols, fingerprint, tokens FROM (
			   SELECT path, gist, COALESCE(symbols,'') AS symbols,
			          COALESCE(fingerprint,'') AS fingerprint, tokens,
			          ROW_NUMBER() OVER (
			            PARTITION BY path
			            ORDER BY (project_id = ?) DESC, updated_at DESC
			          ) AS rn
			   FROM code_memory WHERE project_id = ? OR project_id = ''
			 ) WHERE rn = 1 ORDER BY path`, sc.ProjectID, sc.ProjectID)
	}
	if err != nil {
		return nil, fmt.Errorf("error al listar memoria de código: %w", err)
	}
	defer rows.Close()
	out := []CodeMemory{}
	for rows.Next() {
		var cm CodeMemory
		if err := rows.Scan(&cm.Path, &cm.Gist, &cm.Symbols, &cm.Fingerprint, &cm.Tokens); err != nil {
			return nil, fmt.Errorf("error al leer fila de memoria de código: %w", err)
		}
		out = append(out, cm)
	}
	return out, rows.Err()
}

// ReplaceProjectCodeMemoryFrom REEMPLAZA los gists de un proyecto, igual que ReplaceProjectGraphFrom
// hace con nodos y aristas: en UNA transacción borra los del origin_project_id y reinserta el set
// empujado. Así el push es idempotente y aislado por tenant — el DELETE nunca toca otro project_id.
// origin == "" ⇒ project_id del engine. Un gist sin path o sin contenido se saltea en silencio: no
// vale abortar la federación entera por una fila mal formada del emisor.
func (e *DbEngine) ReplaceProjectCodeMemoryFrom(originProjectID string, gists []CodeMemory) error {
	projectID := originProjectID
	if projectID == "" {
		projectID = e.projectID
	}
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al iniciar transacción de reemplazo de gists: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM code_memory WHERE project_id=?`, projectID); err != nil {
		return fmt.Errorf("error al limpiar gists del proyecto %q: %w", projectID, err)
	}
	for _, cm := range gists {
		if cm.Path == "" || cm.Gist == "" {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO code_memory (path, gist, symbols, fingerprint, tokens, project_id, updated_at)
			 VALUES (?,?,?,?,?,?,CURRENT_TIMESTAMP)
			 ON CONFLICT(path, project_id) DO UPDATE SET
			   gist=excluded.gist, symbols=excluded.symbols,
			   fingerprint=excluded.fingerprint, tokens=excluded.tokens,
			   updated_at=CURRENT_TIMESTAMP`,
			cm.Path, cm.Gist, cm.Symbols, cm.Fingerprint, cm.Tokens, projectID,
		); err != nil {
			return fmt.Errorf("error al guardar gist de %s: %w", cm.Path, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al commitear el reemplazo de gists: %w", err)
	}
	return nil
}
