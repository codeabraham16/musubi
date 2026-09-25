package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
	// Una transacción para un solo INSERT, por la generación del grafo: los gists viajan en la foto
	// del push, así que un gist nuevo es un cambio que el central tiene que recibir aunque el grafo
	// del paquete no se haya re-derivado (un archivo que el grafo no indexa, o un refresh que falló).
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al iniciar el guardado de memoria de código: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(
		`INSERT INTO code_memory (path, gist, symbols, fingerprint, tokens, project_id, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(path, project_id) DO UPDATE SET
		   gist=excluded.gist, symbols=excluded.symbols,
		   fingerprint=excluded.fingerprint, tokens=excluded.tokens,
		   updated_at=CURRENT_TIMESTAMP`,
		cm.Path, cm.Gist, cm.Symbols, cm.Fingerprint, cm.Tokens, projectID,
	); err != nil {
		return fmt.Errorf("error al guardar memoria de código: %w", err)
	}
	if err := avanzarGeneracionDelGrafo(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al commitear la memoria de código: %w", err)
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
	return e.allCodeMemory(ctx, e.db)
}

// allCodeMemory es el cuerpo de AllCodeMemoryCtx contra cualquier consultor: la base o una
// transacción de lectura (FotoDelGrafoCtx necesita los gists en la MISMA foto que el grafo).
func (e *DbEngine) allCodeMemory(ctx context.Context, db consultor) ([]CodeMemory, error) {
	sc := projectScopeFrom(ctx)
	var rows *sql.Rows
	var err error
	if sc.Federate || sc.ProjectID == "" {
		// Sin scope no hay proyecto que preferir: se desempata por el más recientemente tocado.
		rows, err = db.QueryContext(ctx,
			`SELECT path, gist, symbols, fingerprint, tokens FROM (
			   SELECT path, gist, COALESCE(symbols,'') AS symbols,
			          COALESCE(fingerprint,'') AS fingerprint, tokens,
			          ROW_NUMBER() OVER (PARTITION BY path ORDER BY updated_at DESC) AS rn
			   FROM code_memory
			 ) WHERE rn = 1 ORDER BY path`)
	} else {
		rows, err = db.QueryContext(ctx,
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
	if err := avanzarGeneracionDelGrafo(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al commitear el reemplazo de gists: %w", err)
	}
	return nil
}

// PrefijoGistAutomatico marca un gist que NO escribió un agente: lo sacó el índice del grafo del
// comentario de cabecera del archivo (codeintel.ResumenDeCabecera). Va en el texto y no en una
// columna aparte para no pedir una migración de esquema, y porque así viaja solo al central en la
// foto del push, donde también hace falta distinguirlo.
//
// La marca es lo que deja convivir a los dos escritores: el índice sólo pisa o borra filas CON la
// marca, y nunca una sin ella. Por eso musubi_save_code se la saca a lo que le manda un agente —ver
// SinPrefijoAutomatico—: un gist de agente que la llevara quedaría expuesto a que el índice lo pise.
const PrefijoGistAutomatico = "[auto · cabecera] "

// EsGistAutomatico dice si un gist lo escribió el índice (lleva PrefijoGistAutomatico).
func EsGistAutomatico(gist string) bool { return strings.HasPrefix(gist, PrefijoGistAutomatico) }

// SinPrefijoAutomatico le saca la marca de automático a un texto, las veces que la tenga.
func SinPrefijoAutomatico(gist string) string {
	for EsGistAutomatico(gist) {
		gist = strings.TrimPrefix(gist, PrefijoGistAutomatico)
	}
	return strings.TrimSpace(gist)
}

// GuardarGistsAutomaticosFrom escribe los gists AUTOMÁTICOS de un proyecto y retira los que dejaron
// de tener de dónde salir, todo en UNA transacción. Devuelve cuántas filas escribió o borró.
// origin == "" ⇒ project_id del engine. Cada gist tiene que traer ya PrefijoGistAutomatico.
//
// LAS TRES REGLAS, en orden, por cada gist:
//
//  1. SI HAY UN GIST DE AGENTE DEL MISMO PATH, EN CUALQUIER project_id, NO SE ESCRIBE NADA. El de
//     agente manda aunque esté rancio: es texto que un agente escribió leyendo el archivo, y el
//     índice nunca lo pisa ni le refresca la huella —refrescarla haría pasar por fresco un texto
//     viejo—. Se mira en cualquier proyecto porque en una base real el mismo archivo tiene filas
//     con project_id vacío y 'Musubi' además de 'musubi' (medido el 2026-09-24: 57, 8 y 55), y un
//     automático al lado de un gist de agente sería una segunda versión del mismo archivo que el
//     LIMIT 1 de GetCodeMemoryCtx elegiría al azar.
//  2. SI LA FILA AUTOMÁTICA YA TIENE LA MISMA HUELLA, EL MISMO TEXTO Y LOS MISMOS SÍMBOLOS, NO SE
//     REESCRIBE. Es lo que hace idempotente al índice: sin esto cada tick avanzaría la generación
//     del grafo y saldría un push del grafo entero al central cada hora. La comparación NO es sólo
//     por huella: si sube codeintel.VersionResumenDeCabecera, el archivo no cambió pero el texto
//     sí, y la fila se tiene que reescribir.
//  3. Si no, UPSERT por (path, project_id).
//
// `retirar` son paths cuyo gist automático hay que BORRAR: el archivo perdió su cabecera o dejó de
// existir. Sólo se borran filas CON la marca y del proyecto de origen; un gist de agente del mismo
// path queda intacto. Sin esto, un automático sin fuente quedaría rancio para siempre y viajaría
// al central en cada push.
//
// La generación del grafo avanza UNA vez y sólo si algo cambió.
func (e *DbEngine) GuardarGistsAutomaticosFrom(originProjectID string, gists []CodeMemory, retirar []string) (int, error) {
	if len(gists) == 0 && len(retirar) == 0 {
		return 0, nil
	}
	projectID := originProjectID
	if projectID == "" {
		projectID = e.projectID
	}
	tx, err := e.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("error al iniciar el guardado de gists automáticos: %w", err)
	}
	defer tx.Rollback()

	cambios := 0
	for _, cm := range gists {
		if cm.Path == "" || !EsGistAutomatico(cm.Gist) {
			continue
		}
		var deAgente int
		if err := tx.QueryRow(
			`SELECT COUNT(*) FROM code_memory WHERE path = ? AND substr(gist, 1, length(?)) != ?`,
			cm.Path, PrefijoGistAutomatico, PrefijoGistAutomatico,
		).Scan(&deAgente); err != nil {
			return 0, fmt.Errorf("error al buscar el gist de agente de %s: %w", cm.Path, err)
		}
		if deAgente > 0 {
			continue
		}
		var gist, symbols, fp string
		err := tx.QueryRow(
			`SELECT gist, COALESCE(symbols,''), COALESCE(fingerprint,'') FROM code_memory
			 WHERE path = ? AND project_id = ?`, cm.Path, projectID,
		).Scan(&gist, &symbols, &fp)
		if err != nil && err != sql.ErrNoRows {
			return 0, fmt.Errorf("error al leer el gist automático de %s: %w", cm.Path, err)
		}
		if err == nil && fp == cm.Fingerprint && gist == cm.Gist && symbols == cm.Symbols {
			continue
		}
		if _, err := tx.Exec(
			`INSERT INTO code_memory (path, gist, symbols, fingerprint, tokens, project_id, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
			 ON CONFLICT(path, project_id) DO UPDATE SET
			   gist=excluded.gist, symbols=excluded.symbols,
			   fingerprint=excluded.fingerprint, tokens=excluded.tokens,
			   updated_at=CURRENT_TIMESTAMP`,
			cm.Path, cm.Gist, cm.Symbols, cm.Fingerprint, cm.Tokens, projectID,
		); err != nil {
			return 0, fmt.Errorf("error al guardar el gist automático de %s: %w", cm.Path, err)
		}
		cambios++
	}
	for _, path := range retirar {
		res, err := tx.Exec(
			`DELETE FROM code_memory WHERE path = ? AND project_id = ? AND substr(gist, 1, length(?)) = ?`,
			path, projectID, PrefijoGistAutomatico, PrefijoGistAutomatico,
		)
		if err != nil {
			return 0, fmt.Errorf("error al retirar el gist automático de %s: %w", path, err)
		}
		if n, _ := res.RowsAffected(); n > 0 {
			cambios += int(n)
		}
	}
	if cambios == 0 {
		return 0, nil
	}
	if err := avanzarGeneracionDelGrafo(tx); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("error al commitear los gists automáticos: %w", err)
	}
	return cambios, nil
}
