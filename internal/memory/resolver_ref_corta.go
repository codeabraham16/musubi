package memory

import (
	"context"
	"errors"
	"fmt"
)

// ErrRefCortaAmbigua lo devuelve ResolverRefCortaCtx cuando una ref corta de 8 hex matchea MÁS de una
// observación dentro del scope: el prefijo colisiona (≈1/2^32 por par, raro pero posible) y no hay
// forma de saber a cuál se refería el agente. A diferencia de la hidratación —que ante la ambigüedad
// trae el SUPERCONJUNTO y deja que el agente elija (hydrateByIDs)—, los consumidores por-PK del id
// (promote, discard) ACTÚAN sobre la fila: actuar sobre la equivocada, o sobre varias, es el daño. Por
// eso acá la ambigüedad es un error duro, no una resolución.
var ErrRefCortaAmbigua = errors.New("ref corta ambigua: el prefijo de 8 hex matchea más de una observación")

// ResolverRefCortaCtx traduce una REF CORTA de 8 hex —la que emite el envoltorio del hook (ver IDCorto)
// y que el agente devuelve del recall— al id COMPLETO de la observación, ESTRICTO y acotado al scope de
// la credencial. Es la mitad que la fase 1 (#766) dejó SÓLO en la hidratación (hydrateByIDs): los
// consumidores por-PK del id —PromoteObservationCtx, DescartarPropuestaCtx— buscaban por igualdad
// literal, así que una ref corta nunca matcheaba y, desde que el hook emite 8 hex, el id que el agente
// devuelve no servía para esas operaciones (la propuesta rancia quedaba viva: el modo «la memoria
// empieza a fallar»).
//
// CONTRATO (y en qué difiere de hydrateByIDs):
//   - Un id que NO es ref corta (UUID completo, «commit-…», cualquier no-8-hex) viaja VERBATIM, sin
//     tocar la base. Así un caller que ya pasa el id entero —corroborate con el id del board, el
//     sync— no cambia en nada: es un no-op. El "" también cae acá (esRefCorta("")=false) y pasa igual.
//   - Una ref corta se resuelve con substr(id,1,8)=? acotado por scopeClause —el MISMO aislamiento
//     Track 17 que hereda la hidratación— y FILTRANDO a filas con id canónico (ver IDENTIDAD): el
//     dominio del match se iguala al rango del emisor IDCorto, que sólo acorta UUID canónicos.
//   - 0 matches canónicos ⇒ ErrObservationNotFound (la ref no resuelve en este scope). 1 ⇒ su id
//     completo. ≥2 ⇒ ErrRefCortaAmbigua. NUNCA el superconjunto: acá no se adivina.
//
// IDENTIDAD. Como la ref corta es full[:8] de un UUID canónico (IDCorto sólo acorta ésos), un único
// match de prefijo ENTRE FILAS CANÓNICAS ES la fila full —su id acortado vuelve a dar la ref—, así que
// resolver a ese único match no puede devolver «otra» observación. Las filas NO canónicas que compartan
// los primeros 8 hex (p.ej. un id de 9+ hex que save_observation acepta porque EsRefCorta mira
// exactamente-8) se descartan del match: contarlas inventaría una ambigüedad que el emisor no puede
// producir. Si hay colisión entre canónicas se corta con error, en vez de actuar sobre la equivocada.
func (e *DbEngine) ResolverRefCortaCtx(ctx context.Context, id string) (string, error) {
	if !esRefCorta(id) {
		return id, nil // id completo (o forma no-corta): el camino exacto por PK de siempre
	}
	scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("")
	args := append([]interface{}{id}, scopeArgs...)
	// Sin LIMIT a propósito: el filtro de canonicidad vive en Go (esUUIDCanonico, la MISMA función que
	// usa el emisor IDCorto), no en el SQL, para que emisor y resolvedor no puedan divergir en qué es
	// «canónico». Un LIMIT en el SQL podría cortar la fila canónica detrás de filas no-canónicas que
	// comparten el prefijo y devolver un falso «no encontrada». Las filas con un prefijo de 8 dado son
	// ~1 (colisión ≈1/2^32 por par y acotado al scope), así que traerlas todas es barato.
	rows, err := e.db.QueryContext(ctx,
		`SELECT id FROM observations WHERE substr(id,1,8)=?`+scopeSQL, args...)
	if err != nil {
		return "", fmt.Errorf("error al resolver la ref corta %s: %w", id, err)
	}
	defer rows.Close()
	var encontrados []string
	for rows.Next() {
		var full string
		if err := rows.Scan(&full); err != nil {
			return "", fmt.Errorf("error al escanear la ref corta %s: %w", id, err)
		}
		// DOMINIO = RANGO DEL EMISOR. IDCorto SÓLO acorta UUID canónicos (idcorto.go), así que una ref
		// corta nunca nombra una fila no-canónica. Un id no-canónico que comparta los primeros 8 hex
		// —p.ej. uno de 9+ hex que save_observation acepta porque EsRefCorta mira exactamente-8— NO es
		// un match de esta ref: incluirlo inventaría una ambigüedad (una denegación) que el emisor no
		// puede producir. Filtrarlo acá hace que «un único match ES la fila full» valga de verdad.
		if !esUUIDCanonico(full) {
			continue
		}
		encontrados = append(encontrados, full)
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("error al iterar la ref corta %s: %w", id, err)
	}
	switch len(encontrados) {
	case 0:
		return "", fmt.Errorf("%w: %s", ErrObservationNotFound, id)
	case 1:
		return encontrados[0], nil
	default:
		return "", fmt.Errorf("%w: %s", ErrRefCortaAmbigua, id)
	}
}
