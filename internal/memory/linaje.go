package memory

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// linaje.go es la VUELTA del acervo: de una ficha destilada a los artículos de los que salió, y de
// un artículo a las fichas que salieron de él. Las aristas ya existían —el destilador escribe
// `derived_from` ficha→blob desde el 2026-08-20, 1.372 en el central— pero nadie las leía para
// navegar: fuera del destilador, que las usa como marcador de «ya destilado», eran tinta muerta.
//
// SE LLAMA «LINAJE» Y NO «PROCEDENCIA» porque procedencia ya nombra otra cosa en esta casa: la
// columna observations.provenance, los cuatro sellos de una nota. Dos cosas con el mismo nombre en
// la misma tabla es cómo alguien lee una creyendo que lee la otra.
//
// DOS PIEZAS, Y LAS DOS HACEN FALTA:
//
//   - la LECTURA (LinajeCtx) sigue superseded_by en las dos puntas. El afilador funde fichas
//     gemelas y archiva la perdedora con superseded_by → canónica; el 2026-09-24 había 38 aristas
//     colgando de fichas así y 20 apuntando a blobs fundidos. Una vuelta ingenua devolvería fichas
//     muertas desde el blob, y la sobreviviente perdería las fuentes de la que absorbió.
//   - la ESCRITURA (heredarLinaje) copia las aristas derived_from del perdedor al canónico EN LA MISMA
//     TRANSACCIÓN de la fusión. Sin ella, lo que la lectura reconstruye por superseded_by dura lo
//     que tarda la purga: PurgeArchived borra la fila archivada, sus aristas en los dos extremos y
//     los superseded_by que apuntan a ella, a los 90 días. Después no queda nada que seguir, y el
//     linaje se pierde sin un solo error. La lectura queda como red para lo que se fundió antes de
//     este cambio y para los `supersedes` de la cola de conflictos, que no archivan.

// linajeSaltos acota cuántos superseded_by se siguen desde cada punta. ArchiveAsDuplicate y
// Consolidate aplanan las cadenas (re-apuntan al canónico vivo), así que en la práctica es 0 o 1:
// medido el 2026-09-24, 0 cadenas en musubi-design y 13 en el proyecto musubi. Ocho es holgura, y
// el tope existe para que un ciclo de punteros —que nada impide escribir a mano— no haga girar la
// consulta para siempre.
const linajeSaltos = 8

// linajeTope es cuántas referencias se devuelven por dirección y por id. El máximo medido son 6
// fichas por blob; el tope protege el presupuesto de quien expande, porque estas referencias no
// pasan por el max_tokens de la hidratación.
const linajeTope = 12

// LinajeRef es una punta del linaje: sólo id y topic_key, nunca el contenido. Para leer la fuente
// entera se la expande por su id, que es el viaje que esto habilita.
type LinajeRef struct {
	ID       string `json:"id"`
	TopicKey string `json:"topic_key"`
}

// Linaje es la vuelta de una observación del acervo. Los dos campos van con omitempty a propósito:
// una observación sin aristas se serializa exactamente igual que antes de que el linaje existiera.
type Linaje struct {
	// SalioDe son las fuentes crudas de una ficha destilada (los blobs `ingested/*` de los que salió).
	SalioDe []LinajeRef `json:"salio_de,omitempty"`
	// DestiladoEn son las fichas que salieron de un blob crudo.
	DestiladoEn []LinajeRef `json:"destilado_en,omitempty"`
}

// Vacio dice si no hay nada que mostrar en ninguna de las dos direcciones.
func (l Linaje) Vacio() bool { return len(l.SalioDe) == 0 && len(l.DestiladoEn) == 0 }

// LinajeCtx devuelve el linaje de cada id que tenga alguno; un id sin aristas no figura en el mapa.
//
// Cada punta se resuelve a su versión VIVA y VISIBLE: si el vecino se fundió en otro, se devuelve el
// canónico; si quedó archivado sin reemplazo, en cuarentena o del otro lado del alcance de la
// credencial, no se devuelve. El alcance sale del ctx (ProjectScope), el mismo que acotó la
// hidratación: el linaje no puede enseñar un id que la expansión no dejaría leer.
//
// Del lado de la raíz se juntan también las observaciones que se fundieron EN ella: la canónica
// hereda las fuentes de la ficha que absorbió, y un blob canónico las fichas del blob que absorbió.
func (e *DbEngine) LinajeCtx(ctx context.Context, ids []string) (map[string]Linaje, error) {
	vistos := map[string]bool{}
	unicos := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && !vistos[id] {
			vistos[id] = true
			unicos = append(unicos, id)
		}
	}
	out := map[string]Linaje{}
	if len(unicos) == 0 {
		return out, nil
	}
	// UNA sola cláusula de alcance, reusada en las dos direcciones: se calcula una vez para que la
	// frontera del tenant no pueda quedar distinta en la ida y en la vuelta.
	sc, scArgs := projectScopeFrom(ctx).scopeClause("t")
	for _, tanda := range chunkStrings(unicos, 400) {
		if err := e.linajeDeTanda(ctx, tanda, sc, scArgs, out); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// linajeDeTanda resuelve el linaje de hasta 400 raíces en una sola consulta.
//
// EL COSTO, Y POR QUÉ LA CONSULTA TIENE ESTA FORMA. superseded_by no tiene índice, así que «quién se
// fundió en X» es un barrido de observations. `fundida` lo hace UNA vez por consulta en vez de una
// por fila de la recursión. Y los CROSS JOIN no son decorativos: en SQLite fijan el orden, y sin
// ellos el planificador elegía recorrer TODAS las observaciones visibles por el índice de archived y
// buscar cada una en `vecino`, y todas las aristas para buscarlas en `absorbida`. Medido con 30.000
// observaciones y 8 raíces: 30-40 ms sin los CROSS JOIN, 8-10 ms con ellos aun con 20.000 aristas, y
// de eso ~5 ms son la pasada de `fundida`. Agregar el índice sería una migración, y este PR no la paga.
//
// `absorbida` junta cada raíz con lo que se fundió en ella (superseded_by hacia atrás). `vecino`
// arranca de las aristas derived_from de ese conjunto —la ida con alias `a`, la vuelta con alias
// `b`, para que cada dirección tenga un texto propio— y sigue superseded_by hacia adelante hasta la
// versión viva. El filtro de visibilidad y de alcance va al final, sobre la punta ya resuelta: los
// pasos intermedios son justamente las versiones muertas que hay que atravesar.
func (e *DbEngine) linajeDeTanda(ctx context.Context, tanda []string, sc string, scArgs []interface{}, out map[string]Linaje) error {
	ph := strings.TrimSuffix(strings.Repeat("?,", len(tanda)), ",")
	q := `WITH RECURSIVE
	fundida(id, en) AS MATERIALIZED (
		SELECT id, superseded_by FROM observations WHERE superseded_by IS NOT NULL
	),
	absorbida(raiz, id, salto) AS (
		SELECT id, id, 0 FROM observations WHERE id IN (` + ph + `)
		UNION
		SELECT x.raiz, f.id, x.salto + 1 FROM absorbida x
		CROSS JOIN fundida f ON f.en = x.id
		WHERE x.salto < ?
	),
	vecino(dir, raiz, id, salto) AS (
		SELECT 'ida', a.raiz, r.target_id, 0 FROM absorbida a
		CROSS JOIN observation_relations r ON r.source_id = a.id WHERE r.relation = ?
		UNION
		SELECT 'vuelta', b.raiz, r.source_id, 0 FROM absorbida b
		CROSS JOIN observation_relations r ON r.target_id = b.id WHERE r.relation = ?
		UNION
		SELECT v.dir, v.raiz, o.superseded_by, v.salto + 1 FROM vecino v
		CROSS JOIN observations o ON o.id = v.id
		WHERE o.superseded_by IS NOT NULL AND v.salto < ?
	)
	SELECT DISTINCT v.dir, v.raiz, t.id, t.topic_key, COALESCE(t.created_at, '') AS creado
	FROM vecino v CROSS JOIN observations t ON t.id = v.id
	WHERE ` + visibleObsPredicateDe("t") + sc + ` AND t.id <> v.raiz
	ORDER BY v.raiz, v.dir, creado, t.id`

	args := make([]interface{}, 0, len(tanda)+4+len(scArgs))
	for _, id := range tanda {
		args = append(args, id)
	}
	args = append(args, linajeSaltos, RelDerivedFrom, RelDerivedFrom, linajeSaltos)
	args = append(args, scArgs...)

	rows, err := e.db.QueryContext(ctx, q, args...)
	if err != nil {
		return fmt.Errorf("leer el linaje del acervo: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var dir, raiz, id, topic, creado string
		if err := rows.Scan(&dir, &raiz, &id, &topic, &creado); err != nil {
			return fmt.Errorf("escanear una punta del linaje: %w", err)
		}
		l := out[raiz]
		ref := LinajeRef{ID: id, TopicKey: topic}
		switch dir {
		case "ida":
			if len(l.SalioDe) < linajeTope {
				l.SalioDe = append(l.SalioDe, ref)
			}
		case "vuelta":
			if len(l.DestiladoEn) < linajeTope {
				l.DestiladoEn = append(l.DestiladoEn, ref)
			}
		}
		out[raiz] = l
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterar el linaje del acervo: %w", err)
	}
	return nil
}

// heredarSinPisar es el ON CONFLICT de las dos copias de heredarLinaje, escrito una sola vez para que
// las dos direcciones no puedan divergir.
//
// VA CON DO NOTHING Y NO CON UpsertObsRelation, y la diferencia importa: el upsert pisa la `relation`
// de un par que ya existe. Si el canónico ya tenía con esa punta un `not_duplicate` o un `related`,
// heredar el linaje lo borraría. Acá el par existente gana siempre. El costo, dicho: en ese caso raro
// el canónico no hereda esa punta, y a esa punta la sostiene sólo superseded_by hasta la purga.
const heredarSinPisar = ` ON CONFLICT(source_id, target_id) DO NOTHING`

// heredarSoloDerivedFrom es el filtro de relación de las dos copias, también escrito una sola vez.
// Se hereda el LINAJE y nada más: un `related`, un `contradicts` o un veredicto pendiente del
// perdedor hablan de él, no del canónico, y copiarlos le inventaría al canónico relaciones que nadie
// juzgó. Va al final del WHERE, como fragmento propio, para que se pueda sabotear sin tocar el resto.
const heredarSoloDerivedFrom = ` AND r.relation = ?`

// heredarLinaje copia al canónico las aristas derived_from del perdedor, adentro de la transacción
// de la fusión. Es lo que hace durable el linaje: ver el encabezado de este archivo.
//
// Copia las dos direcciones sin preguntar qué es el perdedor. Si es una ficha, sus salientes (de
// dónde salió) pasan a salir del canónico; si es un blob, las fichas que apuntaban a él pasan a
// apuntar al canónico. Una observación que no está en ningún linaje no tiene aristas derived_from y
// las dos sentencias no escriben nada.
//
// Las puntas iguales al canónico se saltean para no escribir una arista de una observación hacia sí
// misma. No es teórico: Consolidate funde por trigramas, y una ficha cuyo texto quedó casi igual al de
// su blob se funde con él, en cualquiera de los dos sentidos.
//
// EFECTO SOBRE EL DESTILADOR, Y ES A PROPÓSITO. El destilador da por destilado a todo blob al que le
// entra una arista derived_from (ObservationsMissingRelation, en distill.go). Cuando un blob que nunca
// se destiló absorbe a uno que sí, la vuelta le re-apunta las fichas del perdedor y el canónico sale
// de la cola: su propio texto no se destila. Es lo que se quiere. Los blobs los funde sólo
// Consolidate —el afilador filtra por prefijo de tarjeta— y los funde por trigramas, con textos casi
// iguales, así que destilar el canónico daría fichas gemelas de las que ya salieron del perdedor, y el
// afilador tendría que volver a fundirlas. Lo custodia TestElBlobQueAbsorbeAUnoDestiladoSaleDeLaCola.
func heredarLinaje(tx *sql.Tx, perdedor, canonico string) error {
	if perdedor == canonico {
		return nil
	}
	if _, err := tx.Exec(`INSERT INTO observation_relations (id, source_id, target_id, relation, confidence, status, resolved_by, reason)
		SELECT lower(hex(randomblob(16))), ?, r.target_id, r.relation, r.confidence, r.status, r.resolved_by,
		       'heredada de ' || ? || ' al fundirla'
		FROM observation_relations r
		WHERE r.source_id = ? AND r.target_id <> ?`+heredarSoloDerivedFrom+heredarSinPisar,
		canonico, perdedor, perdedor, canonico, RelDerivedFrom); err != nil {
		return fmt.Errorf("heredar las fuentes de %q en %q: %w", perdedor, canonico, err)
	}
	if _, err := tx.Exec(`INSERT INTO observation_relations (id, source_id, target_id, relation, confidence, status, resolved_by, reason)
		SELECT lower(hex(randomblob(16))), r.source_id, ?, r.relation, r.confidence, r.status, r.resolved_by,
		       'heredada de ' || ? || ' al fundirla'
		FROM observation_relations r
		WHERE r.target_id = ? AND r.source_id <> ?`+heredarSoloDerivedFrom+heredarSinPisar,
		canonico, perdedor, perdedor, canonico, RelDerivedFrom); err != nil {
		return fmt.Errorf("heredar las fichas de %q en %q: %w", perdedor, canonico, err)
	}
	return nil
}
