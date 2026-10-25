package memory

import (
	"context"
	"fmt"
	"strings"
)

// GetObservations devuelve el contenido completo de las observaciones indicadas
// (hidratación perezosa tras un Recall). Preserva el orden de ids, omite los que
// no existan y actualiza las estadísticas de acceso de las encontradas.
func (e *DbEngine) GetObservations(ids []string) ([]Observation, error) {
	return e.GetObservationsCtx(context.Background(), ids)
}

// GetObservationsCtx es como GetObservations pero respeta el contexto del caller
// (timeout/cancelación) tanto en la query como en el bump de accesos.
func (e *DbEngine) GetObservationsCtx(ctx context.Context, ids []string) ([]Observation, error) {
	out, _, err := e.GetObservationsBudgetCtx(ctx, ids, 0)
	return out, err
}

// GetObservationsBudget hidrata observaciones por id respetando un techo de
// tokens (budget). Empaqueta contenidos completos en orden de id hasta que el
// siguiente no entra; garantiza al menos el primero (truncado si excede el
// budget). budget <= 0 significa sin límite. Devuelve también los tokens usados,
// para contabilizarlos en el ledger. Actualiza stats de acceso de lo devuelto.
func (e *DbEngine) GetObservationsBudget(ids []string, budget int) ([]Observation, int, error) {
	return e.GetObservationsBudgetCtx(context.Background(), ids, budget)
}

// GetObservationsBudgetCtx es como GetObservationsBudget pero respeta el contexto del
// caller (timeout/cancelación) en la query y en el bump de accesos, en vez de usar un
// context.Background() interno que ignoraba el deadline del llamador.
//
// CUENTA EL ACCESO de lo que devuelve. Si el caller ya lo contó —el grounding de musubi_ask, donde
// Recall ya bumpeó esos mismos ids— usá HydrateForGroundingCtx, que no lo cuenta.
func (e *DbEngine) GetObservationsBudgetCtx(ctx context.Context, ids []string, budget int) ([]Observation, int, error) {
	out, used, found, err := e.hydrateByIDs(ctx, ids, budget)
	if err != nil {
		return nil, 0, err
	}
	if err := e.bumpAccess(ctx, found); err != nil {
		return out, used, err
	}
	// Y ADEMÁS cuenta la EXPANSIÓN, en su propia columna. Ver bumpExpand: es la misma fila y el
	// mismo momento, pero no es el mismo hecho.
	if err := e.bumpExpand(ctx, found); err != nil {
		return out, used, err
	}
	return out, used, nil
}

// bumpExpand cuenta la EXPANSIÓN: el agente leyó el gist y pidió el contenido entero.
//
// POR QUÉ ES UNA COLUMNA APARTE Y NO UN INCREMENTO MÁS EN access_count. `bumpAccess` lo llaman dos
// caminos, y lo que significan es opuesto:
//
//   - recall.go bumpea lo que ACABA DE SERVIR. Eso lo escribe el ranker sobre su propia salida: es
//     el lazo endógeno que la invariante N4 documenta y que accessRate amortigua a propósito.
//   - esto bumpea lo que el agente ELIGIÓ de entre lo servido. Es información que el ranker no
//     puede fabricar, porque llega después de que alguien leyó los titulares y decidió.
//
// Sumadas en la misma columna, la segunda es irrecuperable: no hay forma de restarle a un
// access_count la parte que puso el propio recall. Y es la única etiqueta de relevancia del sistema
// que no deriva de la similitud — la que le falta al banco de recall para dejar de medirse contra
// sí mismo (internal/recalleval/fixture_real.go, EtiquetadoPorExpansion).
//
// CORRE ADEMÁS DE bumpAccess, NO EN SU LUGAR: `access_count` queda bit-idéntico a como venía. Esta
// función sólo captura; quién usa la señal para rankear es otra decisión, y necesita la medición
// que ésta habilita.
//
// Mismo trato que bumpAccess en los dos bordes: en sólo-lectura no hay refuerzo que escribir, y el
// error nunca es motivo para que el agente se quede sin el contenido que ya está calculado.
func (e *DbEngine) bumpExpand(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	if e.soloLectura {
		return nil
	}
	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	q := `UPDATE observations
	      SET last_expanded = CURRENT_TIMESTAMP, expand_count = expand_count + 1
	      WHERE id IN (` + strings.Join(placeholders, ",") + `)`
	if _, err := e.db.ExecContext(ctx, q, args...); err != nil {
		return fmt.Errorf("error al actualizar el contador de expansiones: %w", err)
	}
	return nil
}

// HydrateForGroundingCtx hidrata por id con presupuesto igual que GetObservationsBudgetCtx pero SIN
// contabilizar el acceso.
//
// Existe para el grounding de musubi_ask: Recall ya bumpeó esos mismos ids al devolverlos, y
// fundamentar UNA pregunta es UN uso de la memoria, no dos. Contarlo de nuevo inflaría access_count
// justo sobre las memorias más consultadas y realimentaría el ranker con su propia salida — que es
// lo que la invariante N4 del recall (ver recall.go) prohíbe explícitamente.
//
// OJO: como toda hidratación por id, NO aplica el predicado canónico de visibilidad (Q0b). Es
// responsabilidad del caller que los ids vengan de un camino que sí lo aplique — en ask vienen de
// Recall.
func (e *DbEngine) HydrateForGroundingCtx(ctx context.Context, ids []string, budget int) ([]Observation, int, error) {
	out, used, _, err := e.hydrateByIDs(ctx, ids, budget)
	return out, used, err
}

// hydrateByIDs es el empaquetado PURO, sin efectos de escritura: consulta, respeta el orden de la
// lista de ids y mete contenidos completos hasta agotar el presupuesto. Devuelve también los ids
// efectivamente incluidos, para que el caller decida si contabiliza el acceso o no.
func (e *DbEngine) hydrateByIDs(ctx context.Context, ids []string, budget int) ([]Observation, int, []string, error) {
	if len(ids) == 0 {
		return []Observation{}, 0, nil, nil
	}

	// Un token pedido es o bien un id COMPLETO (camino exacto por PK, el de siempre) o bien una REF
	// CORTA de 8 hex (ver IDCorto). Se parten en dos grupos y se consultan juntos. MIENTRAS el emisor
	// del hook no esté encendido (fase 2), ningún caller manda refs cortas: todo cae en `exactos` y el
	// empaquetado toma la rama temprana BIT-IDÉNTICA a como venía. El camino de prefijos (con dedup y
	// grupo ambiguo indivisible) sólo se activa cuando aparece al menos una ref corta.
	var exactos, prefijos []string
	for _, id := range ids {
		if esRefCorta(id) {
			prefijos = append(prefijos, id)
		} else {
			exactos = append(exactos, id)
		}
	}

	var cond []string
	var args []interface{}
	if len(exactos) > 0 {
		ph := make([]string, len(exactos))
		for i, id := range exactos {
			ph[i] = "?"
			args = append(args, id)
		}
		cond = append(cond, "id IN ("+strings.Join(ph, ",")+")")
	}
	if len(prefijos) > 0 {
		// substr(id,1,8) NO usa el índice de la PK: escanea la tabla. Es tolerable porque este camino
		// lo dispara sólo un memory_expand del agente con una ref corta —bajo volumen, a demanda—; los
		// callers internos (recall, grounding) pasan ids completos y se quedan en el camino exacto.
		ph := make([]string, len(prefijos))
		for i, p := range prefijos {
			ph[i] = "?"
			args = append(args, p)
		}
		cond = append(cond, "substr(id,1,8) IN ("+strings.Join(ph, ",")+")")
	}
	// cond nunca queda vacío: len(ids) > 0 ⇒ cada id cae en exactos o en prefijos.

	// Aislamiento por proyecto (Track 17): la hidratación por id arbitrario era una fuga total
	// (memory_expand traía contenido de cualquier proyecto). Acota a la credencial del caller
	// (ausente ⇒ federado). Conserva las filas sin atribuir (project_id NULL o ''). El prefijo matchea
	// sólo dentro de este alcance, así que no reabre la fuga.
	scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("")
	args = append(args, scopeArgs...)

	rows, err := e.db.QueryContext(ctx,
		`SELECT id, topic_key, content, COALESCE(created_at,''), COALESCE(project_id,'')
		 FROM observations WHERE (`+strings.Join(cond, " OR ")+`)`+scopeSQL,
		args...,
	)
	if err != nil {
		return nil, 0, nil, fmt.Errorf("error al obtener observaciones: %w", err)
	}
	defer rows.Close()

	byID := make(map[string]Observation, len(ids))
	porPrefijo := map[string][]Observation{} // sólo se llena si hay refs cortas que resolver
	for rows.Next() {
		var o Observation
		if err := rows.Scan(&o.ID, &o.TopicKey, &o.Content, &o.CreatedAt, &o.ProjectID); err != nil {
			return nil, 0, nil, fmt.Errorf("error al escanear observación: %w", err)
		}
		byID[o.ID] = o
		if len(prefijos) > 0 && len(o.ID) >= 8 {
			p := o.ID[:8]
			porPrefijo[p] = append(porPrefijo[p], o)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, 0, nil, fmt.Errorf("error al iterar observaciones: %w", err)
	}

	out := make([]Observation, 0, len(ids))
	found := make([]string, 0, len(ids))
	used := 0

	// `agregar` empaqueta UNA observación respetando el presupuesto; devuelve false cuando el budget
	// ya se agotó y el bucle debe cortar. Dos flags:
	//   dedup  — no emitir dos veces la misma obs (una ref corta y el id completo de la misma fila se
	//            pisan; una ref corta ambigua trae varias filas). El camino exacto puro NO deduplica,
	//            para quedar bit-idéntico a como venía (ver rama temprana abajo).
	//   forzar — meter la obs aunque exceda el presupuesto, sin truncarla. Lo usa SÓLO el grupo de una
	//            ref corta ambigua: no se puede adivinar cuál de las colisiones vio el agente, así que
	//            entran todas antes que perder en silencio la que pidió (principio: cero pérdida).
	emitido := make(map[string]bool, len(ids))
	agregar := func(o Observation, dedup, forzar bool) bool {
		if dedup && emitido[o.ID] {
			return true // ya estaba; seguir sin gastar presupuesto
		}
		cost := EstimateTokens(o.Content)
		if budget > 0 && !forzar {
			if len(out) == 0 && cost > budget {
				// Garantizar el primero, truncado al presupuesto.
				o.Content = truncateToTokens(o.Content, budget)
				cost = EstimateTokens(o.Content)
			} else if used+cost > budget {
				return true // no entra; probamos el siguiente (puede ser más chico)
			}
		}
		if dedup {
			emitido[o.ID] = true
		}
		out = append(out, o)
		found = append(found, o.ID)
		used += cost
		return !(budget > 0 && used >= budget)
	}

	// CAMINO EXACTO PURO: ninguna ref corta. Es el 100% de los callers hasta que el emisor del hook se
	// encienda (fase 2). Se empaqueta sin dedup, para que desplegar sólo el resolvedor sea un no-op
	// demostrable: mismo orden, mismos duplicados si se pidiera un id dos veces, mismo corte de budget.
	if len(prefijos) == 0 {
		for _, id := range ids {
			if o, ok := byID[id]; ok {
				if !agregar(o, false, false) {
					break
				}
			}
		}
		return out, used, found, nil
	}

	// CAMINO CON REFS CORTAS: resolvemos prefijos y deduplicamos. Preserva el ORDEN de los ids pedidos.
loop:
	for _, id := range ids {
		if esRefCorta(id) {
			candidatos := porPrefijo[id]
			if len(candidatos) >= 2 {
				// Ref corta AMBIGUA: el prefijo de 8 hex matchea más de una observación. Entran TODAS
				// (superconjunto), y el presupuesto NO parte el grupo —aunque esta colisión rara
				// (~1/2^32 por expansión) lo exceda—: perder la que el agente pidió es peor que un
				// payload algo más grande. `used` sube con el costo real, así que used_tokens sigue
				// siendo honesto.
				for _, o := range candidatos {
					agregar(o, true, true)
				}
				if budget > 0 && used >= budget {
					break loop
				}
				continue
			}
			for _, o := range candidatos { // 0 ó 1 candidato
				if !agregar(o, true, false) {
					break loop
				}
			}
			continue
		}
		if o, ok := byID[id]; ok {
			if !agregar(o, true, false) {
				break loop
			}
		}
	}

	return out, used, found, nil
}
