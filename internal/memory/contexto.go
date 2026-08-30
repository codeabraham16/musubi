package memory

// contexto.go son las dos lecturas que la flota le hace a la MEMORIA: qué se escribió y qué
// código se tocó dentro de una ventana. Fase 5 · S14.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LAS FECHAS DE LA MEMORIA NO TIENEN EL MISMO FORMATO QUE LAS DE LA FLOTA
//
// En la MISMA base conviven dos formatos, y no es un descuido de nadie: las tablas de flota
// escriben desde Go con `time.RFC3339` (`2026-08-29T19:06:17Z`) y las de memoria dejan que SQLite
// ponga `CURRENT_TIMESTAMP`, que produce `2026-08-29 18:56:39` — sin la `T` y sin la `Z`.
//
// Medido en producción el 2026-08-30, antes de escribir una sola línea de esto:
//
//	observations.created_at  ->  2026-08-30 13:50:03
//	code_memory.updated_at   ->  2026-08-29 18:56:39
//	device_commands.creado   ->  2026-08-29T19:06:17Z
//
// Comparar una ventana en RFC3339 contra estas columnas **no da error: da vacío**. Y un vacío acá
// se lee como «no había nada escrito ese día», que es exactamente la conclusión falsa que este
// slice existe para no producir. Por eso el formato vive en una constante con nombre y no en un
// literal repetido en dos consultas.
//
// Unificar los dos formatos sería mejor y NO se hace acá: tocar cómo se escribe `created_at`
// afecta a nueve consultas de recall que hoy andan. Es un cambio de su propio tamaño.

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	"musubi/internal/fleet"
)

// formatoDeMemoria es el layout de `CURRENT_TIMESTAMP` de SQLite: UTC, sin `T` y sin zona. Es el
// formato de los BYTES GUARDADOS, y por lo tanto el que hay que usar para comparar en el `WHERE`.
const formatoDeMemoria = "2006-01-02 15:04:05"

// ────────────────────────────────────────────────────────────────────────────────────────────
// EL DRIVER CONVIERTE AL LEER Y NO AL COMPARAR, Y ESA ASIMETRÍA ES UNA TRAMPA
//
// `modernc.org/sqlite` mira el tipo DECLARADO de la columna. Sobre un `DATETIME` devuelve la
// fecha ya normalizada a RFC3339 aunque los bytes guardados no lo estén. Medido:
//
//	bytes en la tabla   ->  2026-05-05 12:00:00     (lo que compara el WHERE)
//	leído desde Go      ->  2026-05-05T12:00:00Z    (lo que recibe time.Parse)
//
// O sea que mirar lo que vuelve en Go lleva a la conclusión EQUIVOCADA sobre cómo comparar. La
// primera versión de este archivo parseaba con `formatoDeMemoria` y fallaba; y el error del otro
// lado —«corrijo el WHERE a RFC3339 para que coincida»— no da error: da vacío.
//
// Por eso se aceptan LOS DOS al leer: el driver convierte, pero una fila escrita por otra vía
// —un `UPDATE` a mano, una restauración, otra herramienta— puede volver cruda.
func parseFechaDeMemoria(s string) time.Time {
	for _, layout := range []string{time.RFC3339, formatoDeMemoria} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

// ArchivoTocado es un archivo cuyo gist se actualizó dentro de la ventana. Es lo más cerca que
// Musubi está de «qué cambió en el código» sin salir a hablar con git: no es el commit, es
// «alguien re-leyó y re-resumió este archivo», que en la práctica pasa cuando el archivo cambió.
//
// LA DIFERENCIA SE DECLARA en los huecos de la respuesta. Un `updated_at` de code_memory NO es
// una fecha de commit, y presentarlo como tal sería inventar precisión.
type ArchivoTocado struct {
	Path   string
	Cuando time.Time
}

// buildFTSFrase arma una consulta de FRASE: los tokens del término, adyacentes, entre comillas.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// NO SE REUSA `buildFTSQuery`, Y LA DIFERENCIA LA ENCONTRÉ USANDO LA TOOL CONTRA LA FLOTA REAL
//
// `buildFTSQuery` parte el texto en tokens y los une con **OR**, que es lo correcto para el
// RECALL: ahí la consulta es lenguaje natural y cada palabra suma señal al ranking.
//
// Para un ENLACE POR TÉRMINO es exactamente lo contrario. El término es el nombre de un servicio
// —`avahi-daemon`, `cognicion-db`, `NetworkManager-wait-online`— y con OR se convierte en
// `"avahi" OR "daemon"`: cualquier nota que diga «db» queda enlazada a un servicio que no
// menciona. Medido en producción: la primera corrida devolvió una nota sobre decisiones de
// roadmap enlazada a `avahi-daemon`.
//
// Eso no es un enlace flojo, es un enlace FALSO — la respuesta afirma que el texto NOMBRA algo de
// esa máquina y no lo nombra. Un `ventana` mal puesto sólo agrega ruido; un `termino` mal puesto
// inventa evidencia, que es el único error que esta tool no se puede permitir.
//
// Como FRASE, los tokens tienen que aparecer ADYACENTES, así que `cognicion-db` sólo enlaza con
// texto que diga «cognicion db» (o «cognicion-db», que tokeniza igual). NO hay respaldo a OR si
// la frase no encuentra nada: ese respaldo devolvería justo los enlaces falsos que esto elimina.
func buildFTSFrase(q string) string {
	tokens := strings.FieldsFunc(q, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	if len(tokens) == 0 {
		return ""
	}
	return `"` + strings.Join(tokens, " ") + `"`
}

// ObservacionesQueNombran devuelve las observaciones cuyo texto NOMBRA el término, como frase.
//
// Es la búsqueda del enlace `termino`. Mismo aislamiento por proyecto y mismo predicado canónico
// de visibilidad que el resto del recall: esta puerta nueva no puede ver más que las otras.
func (e *DbEngine) ObservacionesQueNombran(ctx context.Context, termino string, tope int) ([]Observation, error) {
	if tope <= 0 {
		return nil, nil
	}
	frase := buildFTSFrase(termino)
	if frase == "" {
		return nil, nil
	}
	scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("o")
	args := append([]interface{}{frase}, scopeArgs...)
	args = append(args, tope)
	rows, err := e.db.QueryContext(ctx, `
		SELECT o.id, o.topic_key, o.content, o.created_at
		FROM observations_fts f
		JOIN observations o ON o.rowid = f.rowid
		WHERE observations_fts MATCH ? AND `+visibleObsPredicate+scopeSQL+`
		ORDER BY rank LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("error al buscar el término %q: %w", termino, err)
	}
	defer rows.Close()
	var out []Observation
	for rows.Next() {
		var o Observation
		if err := rows.Scan(&o.ID, &o.TopicKey, &o.Content, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("error al escanear un acierto de término: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// ObservacionesEnVentana devuelve lo que se escribió en la memoria dentro de la ventana.
//
// El aislamiento por proyecto sale del ctx (`WithProjectScope`), igual que en todas las lecturas
// de memoria, y el predicado de visibilidad es el CANÓNICO (`visibleObsPredicate`): una
// observación archivada, superseded o en cuarentena no puede aparecer por esta puerta nueva. Es
// la Muralla 2 del recall — la única forma de saltearla es no usar la constante, y eso se ve en
// el diff.
func (e *DbEngine) ObservacionesEnVentana(ctx context.Context, v fleet.Ventana, tope int) ([]Observation, error) {
	if tope <= 0 {
		return nil, nil
	}
	if err := v.Valida(); err != nil {
		return nil, err
	}
	v = v.Normalizada()
	scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("")
	args := []interface{}{v.Desde.UTC().Format(formatoDeMemoria), v.Hasta.UTC().Format(formatoDeMemoria)}
	args = append(args, scopeArgs...)
	args = append(args, tope)

	rows, err := e.db.QueryContext(ctx, `
		SELECT id, topic_key, content, created_at FROM observations
		WHERE created_at >= ? AND created_at < ? AND `+visibleObsPredicate+scopeSQL+`
		ORDER BY created_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("error al leer las observaciones de la ventana: %w", err)
	}
	defer rows.Close()
	var out []Observation
	for rows.Next() {
		var o Observation
		if err := rows.Scan(&o.ID, &o.TopicKey, &o.Content, &o.CreatedAt); err != nil {
			return nil, fmt.Errorf("error al escanear una observación de la ventana: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// CodigoTocadoEnVentana devuelve los archivos cuyo gist se actualizó dentro de la ventana.
//
// Mismo aislamiento por proyecto y mismo formato de fecha. `code_memory` no tiene predicado de
// visibilidad —no hay archivado ni cuarentena de gists—, así que acá no hay ninguno que aplicar y
// eso se dice en vez de dejar la duda.
func (e *DbEngine) CodigoTocadoEnVentana(ctx context.Context, v fleet.Ventana, tope int) ([]ArchivoTocado, error) {
	if tope <= 0 {
		return nil, nil
	}
	if err := v.Valida(); err != nil {
		return nil, err
	}
	v = v.Normalizada()
	// El scope de code_memory se arma a mano porque `scopeClause` está escrita para la tabla de
	// observaciones. MISMO criterio, y el mismo que usa el recall: se conservan las filas sin
	// atribuir (project_id ''), que son las legacy del espacio federado.
	sc := projectScopeFrom(ctx)
	scopeSQL := ""
	args := []interface{}{v.Desde.UTC().Format(formatoDeMemoria), v.Hasta.UTC().Format(formatoDeMemoria)}
	if !sc.Federate && sc.ProjectID != "" {
		scopeSQL = ` AND (project_id = ? OR project_id = '')`
		args = append(args, sc.ProjectID)
	}
	args = append(args, tope)

	rows, err := e.db.QueryContext(ctx, `
		SELECT path, updated_at FROM code_memory
		WHERE updated_at >= ? AND updated_at < ?`+scopeSQL+`
		ORDER BY updated_at DESC LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("error al leer el código tocado en la ventana: %w", err)
	}
	defer rows.Close()
	var out []ArchivoTocado
	for rows.Next() {
		var a ArchivoTocado
		var cuando string
		if err := rows.Scan(&a.Path, &cuando); err != nil {
			return nil, fmt.Errorf("error al escanear un archivo tocado: %w", err)
		}
		// Una fecha ilegible NO se lleva puesta la fila: el path sigue siendo el dato, y perder
		// el archivo entero por no poder parsear su hora sería tirar la información útil para
		// conservar la decorativa. Queda en cero, que la superficie muestra como null.
		a.Cuando = parseFechaDeMemoria(cuando)
		out = append(out, a)
	}
	return out, rows.Err()
}
