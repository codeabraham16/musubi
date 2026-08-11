package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// relations_obs.go implementa el grafo de RELACIONES SEMÁNTICAS entre
// observaciones (resolución de conflictos model-free). A diferencia del grafo de
// hechos (entities/relations), acá las aristas vinculan dos observaciones y
// expresan cómo se relacionan sus contenidos: una reemplaza a otra, la contradice,
// es compatible, etc. El vocabulario es espejo del de Engram (probado).

// Tipos de relación entre dos observaciones.
const (
	RelPending       = "pending"        // candidato detectado, sin veredicto
	RelRelated       = "related"        // conectadas temáticamente
	RelCompatible    = "compatible"     // no se contradicen
	RelScoped        = "scoped"         // una es más general, otra más específica
	RelConflictsWith = "conflicts_with" // se contradicen directamente
	RelSupersedes    = "supersedes"     // source reemplaza/obsoleta a target
	RelNotConflict   = "not_conflict"   // explícitamente NO en conflicto
)

// Estados de una relación.
const (
	RelStatusPending  = "pending"
	RelStatusResolved = "resolved"
)

// ObsRelation es una arista entre dos observaciones (source -> target).
type ObsRelation struct {
	ID       string `json:"id"`
	SourceID string `json:"source_id"`
	TargetID string `json:"target_id"`
	Relation string `json:"relation"`
	// Confidence es la señal histórica, y NO significa lo mismo en toda la tabla: en una relación
	// pendiente es max(léxico, coseno) y en una auto-resuelta es el léxico solo. Se conserva tal cual
	// por compatibilidad; para saber de DÓNDE salió el número, mirar Lex y Cosine.
	Confidence float64 `json:"confidence"`
	Status     string  `json:"status"`
	ResolvedBy string  `json:"resolved_by,omitempty"`
	Reason     string  `json:"reason,omitempty"`
	// Lex y Cosine son las dos señales POR SEPARADO, y son punteros a propósito.
	//
	// nil significa «no se sabe»: la fila es anterior a la migración v27 y su desglose no se puede
	// reconstruir sin volver a scorear el par. Un 0 sería otra cosa —un coseno 0 quiere decir
	// ortogonales, que es información— y confundir «no medido» con «midió cero» es exactamente cómo
	// una señal ausente se cuela como si fuera un dato.
	Lex    *float64 `json:"lex,omitempty"`
	Cosine *float64 `json:"cosine,omitempty"`
}

// UpsertObsRelation inserta o actualiza la relación del par (source, target) y
// devuelve el id CANÓNICO persistido (el existente si el par ya estaba, o el
// nuevo). Es idempotente por par: el mismo par actualiza el veredicto en vez de
// duplicar.
func (e *DbEngine) UpsertObsRelation(r ObsRelation) (string, error) {
	if r.Relation == "" {
		r.Relation = RelPending
	}
	if r.Status == "" {
		r.Status = RelStatusPending
	}
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	// lex/cosine viajan como NULL cuando el caller no los conoce (un upsert a mano, un test, un
	// veredicto que no re-scorea). El COALESCE del UPDATE conserva lo que ya había en vez de pisarlo
	// con NULL: juzgar una relación no debería borrar cómo se la detectó.
	_, err := e.db.Exec(`
		INSERT INTO observation_relations (id, source_id, target_id, relation, confidence, status, resolved_by, reason, lex_score, cosine_score, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(source_id, target_id) DO UPDATE SET
			relation=excluded.relation,
			confidence=excluded.confidence,
			status=excluded.status,
			resolved_by=excluded.resolved_by,
			reason=excluded.reason,
			lex_score=COALESCE(excluded.lex_score, lex_score),
			cosine_score=COALESCE(excluded.cosine_score, cosine_score),
			updated_at=CURRENT_TIMESTAMP
	`, r.ID, r.SourceID, r.TargetID, r.Relation, r.Confidence, r.Status, nullable(r.ResolvedBy), nullable(r.Reason),
		floatOrNil(r.Lex), floatOrNil(r.Cosine))
	if err != nil {
		return "", fmt.Errorf("error al upsertar relación de observaciones: %w", err)
	}
	// El par puede haber existido con otro id (ON CONFLICT no cambia el id): devolver
	// el id realmente persistido.
	var id string
	if err := e.db.QueryRow(
		`SELECT id FROM observation_relations WHERE source_id=? AND target_id=?`,
		r.SourceID, r.TargetID,
	).Scan(&id); err != nil {
		return r.ID, nil // best-effort: si falla la relectura, devolver el tentativo
	}
	return id, nil
}

// PendingObsRelations devuelve las relaciones que aún esperan veredicto.
func (e *DbEngine) PendingObsRelations() ([]ObsRelation, error) {
	return e.queryObsRelations(`WHERE status = ?`, RelStatusPending)
}

// AllObsRelations devuelve todas las relaciones (para inspección/tests).
func (e *DbEngine) AllObsRelations() ([]ObsRelation, error) {
	return e.queryObsRelations(``)
}

// ResolveObsRelation fija el veredicto de una relación por id y la marca resuelta.
// Si el veredicto es supersedes, además oculta la observación target del recall
// (markSuperseded), reflejando que quedó obsoleta.
func (e *DbEngine) ResolveObsRelation(id, relation, resolvedBy, reason string) error {
	var sourceID, targetID string
	row := e.db.QueryRow(`SELECT source_id, target_id FROM observation_relations WHERE id=?`, id)
	if err := row.Scan(&sourceID, &targetID); err != nil {
		return fmt.Errorf("no existe la relación %q: %w", id, err)
	}

	if _, err := e.db.Exec(`
		UPDATE observation_relations
		SET relation=?, status=?, resolved_by=?, reason=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, relation, RelStatusResolved, nullable(resolvedBy), nullable(reason), id); err != nil {
		return fmt.Errorf("error al resolver relación %q: %w", id, err)
	}

	if relation == RelSupersedes {
		if err := e.markSuperseded(targetID, sourceID); err != nil {
			return err
		}
	}
	return nil
}

// ResolveObsRelationCtx es ResolveObsRelation acotada al proyecto de la credencial: sin esto, un
// principal acotado que conociera un relation_id ajeno podía juzgarlo y —con supersedes— OCULTAR la
// observación de OTRO tenant. Se valida contra el proyecto de la observación de ORIGEN de la relación
// (tras el fix #6, origen y target son del mismo tenant). Auditoría 2026-07-26 #11.
func (e *DbEngine) ResolveObsRelationCtx(ctx context.Context, id, relation, resolvedBy, reason string) error {
	sc := projectScopeFrom(ctx)
	if !sc.Federate && sc.ProjectID != "" {
		var sourceID string
		err := e.db.QueryRowContext(ctx, `SELECT source_id FROM observation_relations WHERE id=?`, id).Scan(&sourceID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// No existe: dejá que ResolveObsRelation devuelva "no existe la relación" (sin oráculo de tenant).
		case err != nil:
			return fmt.Errorf("error al verificar el scope de la relación %q: %w", id, err)
		default:
			ok, serr := e.obsInScope(ctx, sourceID)
			if serr != nil {
				return serr
			}
			if !ok {
				return fmt.Errorf("%w: no se puede juzgar la relación %s de otro proyecto", ErrCrossTenant, id)
			}
		}
	}
	return e.ResolveObsRelation(id, relation, resolvedBy, reason)
}

// PendingObsRelationsCtx es como PendingObsRelations pero ACOTA al proyecto de la credencial
// (Track 17 — aislamiento multi-tenant): solo devuelve las relaciones cuya observación de ORIGEN
// (source_id) pertenece al proyecto del caller. Ausencia de scope ⇒ federado (histórico). Se
// filtra por el source (el lado que dispara la contradicción); ambos endpoints suelen ser del
// mismo proyecto. El JOIN a observations es interno (una relación cuyo source ya no existe se
// omite, caso degenerado).
func (e *DbEngine) PendingObsRelationsCtx(ctx context.Context) ([]ObsRelation, error) {
	scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("o")
	q := `SELECT ` + colsObsRel("r.") + `
	      FROM observation_relations r
	      JOIN observations o ON o.id = r.source_id
	      WHERE r.status = ?` + scopeSQL + ` ORDER BY r.updated_at DESC`
	args := append([]interface{}{RelStatusPending}, scopeArgs...)
	rows, err := e.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("error al listar relaciones pendientes acotadas: %w", err)
	}
	return scanObsRelations(rows)
}

// PendingQuery acota qué pendientes se piden y cuántas se traen de vuelta.
//
// Existe porque la cola creció hasta las centenas y la tool no aceptaba NI UN parámetro: devolvía la
// lista entera siempre. Medido en el central el 2026-08-11: 77 KB por respuesta, y el consumidor
// principal —el panel del cuerpo— la pedía cada 4 segundos para leer UN ENTERO. Bajar 77 KB para
// mirar un número es el desperdicio; y sin `limit` ni orden, la cola tampoco se podía triar, que es
// parte de por qué terminaba barrida a plantilla en vez de arbitrada.
type PendingQuery struct {
	// MinConfidence descarta las de confianza menor. 0 = sin filtro.
	//
	// OJO CON ESTE NÚMERO: en una relación PENDIENTE `confidence` es max(léxico, coseno), y la línea
	// de base del coseno documento-contra-documento para pares SIN relación llega a 0,884 medida en
	// este mismo repo. Un umbral alto acá NO selecciona «las contradicciones más seguras»: selecciona
	// las más parecidas, que es otra cosa. Se expone porque acota el payload, no como ranking de
	// gravedad — y por eso la descripción de la tool lo dice con todas las letras.
	MinConfidence float64
	// MinLex filtra por la señal LÉXICA sola, que es la que significa algo estable: comparten
	// trigramas. Es el filtro que había que tener desde el principio y no existía — el que triaba
	// por MinConfidence creía estar ordenando por gravedad y ordenaba por parecido.
	//
	// Descarta también las filas SIN desglose (anteriores a la v27): no se puede afirmar que una
	// fila cuyo léxico no se conoce supere un umbral. Excluir por no saber es la respuesta correcta;
	// incluirla «por las dudas» metería en el resultado justo lo que el filtro quería sacar.
	MinLex float64
	// Limit trunca la LISTA devuelta. No afecta al conteo (ver PendingPage.Count).
	Limit int
	// PorConfianza ordena de mayor a menor confianza en vez de por recencia.
	PorConfianza bool
	// CountOnly evita traer las filas: sólo cuenta. Es el caso del panel que muestra un número.
	CountOnly bool
}

// PendingPage es el resultado de PendingObsRelationsQueryCtx.
type PendingPage struct {
	// Count es el TOTAL que matchea los filtros, INDEPENDIENTE de Limit.
	//
	// Que no sea len(Relations) es deliberado, y es la parte que se puede romper sin que nadie lo
	// note: un consumidor que pinta «N conflictos» y además pagina mostraría «10» para siempre si el
	// conteo viniera truncado. Un indicador que se achica solo al paginar es peor que no tenerlo.
	Count     int
	Relations []ObsRelation
	// Truncated avisa que la lista se cortó por Limit. Sin esta bandera, un cliente no puede
	// distinguir «hay 10» de «te mando 10 de 358».
	Truncated bool
}

// PendingObsRelationsQueryCtx es PendingObsRelationsCtx con filtros, orden y tope. Mantiene el mismo
// aislamiento por proyecto: sólo las relaciones cuya observación de ORIGEN es del proyecto de la
// credencial (ausencia de scope ⇒ federado).
func (e *DbEngine) PendingObsRelationsQueryCtx(ctx context.Context, q PendingQuery) (PendingPage, error) {
	scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("o")
	where := ` WHERE r.status = ?` + scopeSQL
	args := append([]interface{}{RelStatusPending}, scopeArgs...)
	if q.MinConfidence > 0 {
		where += ` AND r.confidence >= ?`
		args = append(args, q.MinConfidence)
	}
	if q.MinLex > 0 {
		// `lex_score >= ?` ya excluye los NULL por semántica de SQL, y eso es lo buscado: una fila
		// sin desglose no puede afirmarse por encima de un umbral.
		where += ` AND r.lex_score >= ?`
		args = append(args, q.MinLex)
	}
	from := ` FROM observation_relations r JOIN observations o ON o.id = r.source_id`

	page := PendingPage{Relations: []ObsRelation{}}
	// El conteo se hace SIEMPRE y con los MISMOS filtros, aunque después se trunque la lista.
	if err := e.db.QueryRowContext(ctx, `SELECT COUNT(*)`+from+where, args...).Scan(&page.Count); err != nil {
		return page, fmt.Errorf("error al contar relaciones pendientes: %w", err)
	}
	if q.CountOnly || page.Count == 0 {
		return page, nil
	}

	orden := ` ORDER BY r.updated_at DESC`
	if q.PorConfianza {
		orden = ` ORDER BY r.confidence DESC, r.updated_at DESC`
	}
	sel := `SELECT ` + colsObsRel("r.") + from + where + orden
	if q.Limit > 0 {
		sel += ` LIMIT ?`
		args = append(args, q.Limit)
	}
	rows, err := e.db.QueryContext(ctx, sel, args...)
	if err != nil {
		return page, fmt.Errorf("error al listar relaciones pendientes acotadas: %w", err)
	}
	rels, err := scanObsRelations(rows)
	if err != nil {
		return page, err
	}
	if rels != nil {
		page.Relations = rels
	}
	page.Truncated = q.Limit > 0 && page.Count > len(page.Relations)
	return page, nil
}

// queryObsRelations centraliza el SELECT con un filtro WHERE opcional.
func (e *DbEngine) queryObsRelations(where string, args ...interface{}) ([]ObsRelation, error) {
	q := `SELECT ` + colsObsRel("") + `
	      FROM observation_relations ` + where + ` ORDER BY updated_at DESC`
	rows, err := e.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("error al listar relaciones de observaciones: %w", err)
	}
	return scanObsRelations(rows)
}

// scanObsRelations escanea filas de observation_relations en []ObsRelation (columnas en el
// orden id, source_id, target_id, relation, confidence, status, resolved_by, reason).
func scanObsRelations(rows *sql.Rows) ([]ObsRelation, error) {
	defer rows.Close()
	var out []ObsRelation
	for rows.Next() {
		var r ObsRelation
		// lex/cosine se escanean como sql.NullFloat64 y NO con COALESCE a 0: el NULL de una fila
		// vieja significa «no se sabe», y aplanarlo a 0 lo volvería indistinguible de un coseno
		// realmente nulo. Es la misma distinción que defiende el tipo puntero del struct.
		var lex, cos sql.NullFloat64
		if err := rows.Scan(&r.ID, &r.SourceID, &r.TargetID, &r.Relation, &r.Confidence,
			&r.Status, &r.ResolvedBy, &r.Reason, &lex, &cos); err != nil {
			return nil, fmt.Errorf("error al escanear relación: %w", err)
		}
		if lex.Valid {
			v := lex.Float64
			r.Lex = &v
		}
		if cos.Valid {
			v := cos.Float64
			r.Cosine = &v
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar relaciones de observaciones: %w", err)
	}
	return out, nil
}

// colsObsRel es la lista de columnas que espera scanObsRelations, con el prefijo de tabla que
// corresponda ("r." cuando hay JOIN, "" cuando no).
//
// Está centralizada porque hay TRES consultas que leen relaciones y el scanner es uno solo: agregar
// una columna en dos de las tres deja la tercera devolviendo un error de scan en tiempo de
// ejecución, no de compilación. Una función que las arma es la diferencia entre olvidarse y no
// poder olvidarse.
func colsObsRel(p string) string {
	return p + `id, ` + p + `source_id, ` + p + `target_id, ` + p + `relation, ` + p + `confidence, ` +
		p + `status, COALESCE(` + p + `resolved_by,''), COALESCE(` + p + `reason,''), ` +
		p + `lex_score, ` + p + `cosine_score`
}

// floatOrNil pasa un *float64 a la capa SQL conservando la diferencia entre «no medido» (NULL) y
// «midió cero», que para el coseno son cosas distintas.
func floatOrNil(f *float64) interface{} {
	if f == nil {
		return nil
	}
	return *f
}

// nullable convierte "" en NULL para columnas opcionales.
func nullable(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ObsRelationByID devuelve una relación por su id. Existe porque quien juzga necesita saber a QUÉ
// observación apunta antes de decidir —sobre todo con `supersedes`, que la oculta— y hasta ahora el
// target sólo se resolvía adentro de ResolveObsRelation, donde nadie podía verlo.
func (e *DbEngine) ObsRelationByID(id string) (ObsRelation, error) {
	rels, err := e.queryObsRelations(`WHERE id = ?`, id)
	if err != nil {
		return ObsRelation{}, err
	}
	if len(rels) == 0 {
		return ObsRelation{}, fmt.Errorf("no existe la relación %q", id)
	}
	return rels[0], nil
}

// ReferenciasA devuelve las OTRAS relaciones que tocan a obsID, en cualquiera de las dos puntas,
// excluyendo la relación `excepto` (típicamente la que se está juzgando).
//
// POR QUÉ EXISTE. `supersedes` es el único veredicto que HACE algo —oculta la observación target del
// recall— y no dice qué apunta a ella. Se puede huerfanizar una conversación en silencio: alguien
// construye respuestas encima de una nota, otro la consolida y la reemplaza, y las respuestas quedan
// citando algo que ya nadie ve. Pasó de verdad entre dos equipos el 2026-08-10.
//
// Mira las DOS puntas a propósito: una nota puede ser citada (target) o puede citar (source), y las
// dos formas se rompen igual cuando desaparece del recall.
func (e *DbEngine) ReferenciasA(obsID, excepto string) ([]ObsRelation, error) {
	return e.queryObsRelations(
		`WHERE (source_id = ? OR target_id = ?) AND id <> ?`, obsID, obsID, excepto)
}
