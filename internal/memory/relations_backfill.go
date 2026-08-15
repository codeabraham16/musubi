package memory

import "fmt"

// relations_backfill.go reconstruye el DESGLOSE (lex/coseno) de las relaciones que se guardaron
// sin él.
//
// POR QUÉ EXISTE. El comentario de ObsRelation.Lex dice que un nil «no se puede reconstruir sin
// volver a scorear el par». Esto es volver a scorear el par. La consecuencia práctica de no
// tenerlo: el 2026-08-15, en el cerebro central, sólo 8 relaciones de SEÑAL (supersedes /
// conflicts_with) tenían las dos columnas cargadas, contra 965 resueltas que no. Con n=8 no se
// puede recalibrar ningún umbral, y el ritmo de acumulación era de ~1 por semana porque el 90 %
// de los veredictos nuevos son ruido. Los datos para calibrar ya existían: les faltaba el sello.
//
// LO QUE NO HACE, Y ES LO IMPORTANTE. Sólo RELLENA NULLs; nunca pisa un número que ya está. Un
// score estampado por el detector describe el contenido tal como era EN EL MOMENTO de detectarlo;
// el que se recomputa acá describe el de HOY. Convivir está bien —para un umbral que mira hacia
// adelante, el contenido actual es la base correcta— pero reemplazar un dato real por uno
// reconstruido sería perder la única medición fiel que hay. De ahí el COALESCE del UPDATE.
//
// EL COSENO SE FILTRA SOLO. Se reusan observationVector y candidateCosines tal cual, y ésas ya
// exigen `model_id = e.vectorModelID`. Un par cuyos vectores vienen de embedders distintos
// simplemente NO recibe coseno, en vez de recibir un número sin sentido: comparar por coseno
// vectores de espacios distintos no es una aproximación mala, es una cifra inventada. En el
// central eso alcanzaba a ~19 de los 73 pares de señal (mezcla de bge-m3 y potion).
//
// TAMPOCO TOCA updated_at. Rellenar el desglose no es actividad sobre la relación: si se
// refrescara la marca, 965 filas viejas pasarían a parecer recién resueltas y arruinarían
// cualquier lectura de antigüedad —incluida la del check `abandoned_runs`— por un cambio que no
// dijo nada nuevo sobre el par.
//
// Y NO EXIGE VISIBILIDAD, que es lo contrario de lo que pide casi todo el resto del paquete. El
// target de un `supersedes` está supersedido POR ESTA MISMA RELACIÓN: filtrar por
// visibleObsPredicate excluiría TODA auto-resolución exitosa, que es exactamente la población
// cuyo umbral se quiere calibrar. Medido en el central: de 73 pares de señal recuperables, exigir
// visibilidad dejaba 21 — se perdían los 50 `supersedes` completos. La condición correcta es que
// las dos filas EXISTAN y tengan contenido; para eso alcanza el JOIN.

// BackfillScoresOptions parametriza el re-scoreo. El cero corre completo y escribe.
type BackfillScoresOptions struct {
	// DryRun cuenta exactamente lo mismo que escribiría, sin escribir nada.
	DryRun bool
	// Limit acota los pares a procesar (0 = todos). Ordena por updated_at DESC, así un límite
	// chico devuelve lo más reciente, que es lo más representativo del detector actual. El cupo es
	// exacto: el JOIN ya descarta las relaciones huérfanas, así que no se gasta en filas que
	// después se tiran.
	Limit int
}

// BackfillScoresResult es lo que el backfill hizo (o haría, con DryRun).
type BackfillScoresResult struct {
	// Scanned son los pares candidatos: les falta al menos una de las dos señales y las dos
	// observaciones todavía existen. Visibles o no — ver el encabezado.
	Scanned int `json:"scanned"`
	// Signal son, de los escaneados, los que sirven para calibrar: supersedes y conflicts_with.
	// El resto (related, compatible, scoped, not_conflict) es volumen, no evidencia.
	Signal       int `json:"signal"`
	LexFilled    int `json:"lex_filled"`
	CosineFilled int `json:"cosine_filled"`
	// NoVector son los pares que quedaron sin coseno: sin embedder, sin vector, o con vectores de
	// otra procedencia. No es un error; es el caso que el filtro por model_id evita.
	NoVector int    `json:"no_vector"`
	ModelID  string `json:"model_id"`
}

// BackfillRelationScores recalcula lex (y coseno cuando se puede) para las relaciones a las que
// les falta el desglose, y lo escribe sin pisar lo ya medido.
//
// Es idempotente en EFECTO, no en recorrido: la segunda corrida no escribe nada, pero vuelve a
// mirar los pares cuyo coseno quedó en NULL. Eso no es un descuido —es que un NULL no distingue
// «todavía no se midió» de «no se puede medir» (sin embedder, o vectores de otra procedencia), y
// separarlos pediría una columna de «se intentó y no se pudo» que es más maquinaria de la que el
// problema justifica. El recorrido es trigramas sobre pares ya acotados: barato y sin efectos.
func (e *DbEngine) BackfillRelationScores(opts BackfillScoresOptions) (BackfillScoresResult, error) {
	res := BackfillScoresResult{ModelID: e.vectorModelID}

	// El JOIN es la única condición sobre las observaciones, y es a propósito: exige que existan
	// (una relación huérfana no se puede scorear) y nada más. El contenido viene en la misma
	// consulta en vez de con un loadObsRow por punta, porque loadObsRow filtra por visibilidad y
	// acá eso sería el error.
	q := `
		SELECT r.id, r.source_id, r.target_id, r.relation,
		       r.lex_score IS NULL, r.cosine_score IS NULL,
		       s.content, t.content
		FROM observation_relations r
		JOIN observations s ON s.id = r.source_id
		JOIN observations t ON t.id = r.target_id
		WHERE r.lex_score IS NULL OR r.cosine_score IS NULL
		ORDER BY r.updated_at DESC`
	if opts.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}

	// Se materializa la lista ANTES de escribir: el UPDATE de cada par modifica justo las columnas
	// por las que filtra el SELECT, y escribir con el cursor abierto sobre la misma tabla es cómo
	// se pierden filas a mitad del recorrido.
	type pendiente struct {
		id, src, tgt, rel  string
		lexNulo, cosNulo   bool
		srcTexto, tgtTexto string
	}
	var pares []pendiente
	rows, err := e.db.Query(q)
	if err != nil {
		return res, fmt.Errorf("error al listar relaciones sin desglose: %w", err)
	}
	for rows.Next() {
		var p pendiente
		if err := rows.Scan(&p.id, &p.src, &p.tgt, &p.rel, &p.lexNulo, &p.cosNulo, &p.srcTexto, &p.tgtTexto); err != nil {
			rows.Close()
			return res, fmt.Errorf("error al escanear una relación sin desglose: %w", err)
		}
		pares = append(pares, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return res, fmt.Errorf("error al recorrer las relaciones sin desglose: %w", err)
	}
	rows.Close()

	for _, p := range pares {
		res.Scanned++
		if p.rel == RelSupersedes || p.rel == RelConflictsWith {
			res.Signal++
		}

		// La MISMA función que corre en producción. Reimplementarla —aunque fuera trigrama por
		// trigrama— convertiría la calibración en una medición de otra cosa.
		lex := Similarity(p.srcTexto, p.tgtTexto)

		var cos *float64
		if v, err := e.observationVector(p.src); err != nil {
			return res, err
		} else if v != nil {
			m, err := e.candidateCosines(v, []string{p.tgt})
			if err != nil {
				return res, err
			}
			if c, hay := m[p.tgt]; hay {
				cos = &c
			}
		}
		if cos == nil {
			res.NoVector++
		}

		if p.lexNulo {
			res.LexFilled++
		}
		if p.cosNulo && cos != nil {
			res.CosineFilled++
		}
		if opts.DryRun {
			continue
		}

		// COALESCE en las dos columnas: rellena el hueco y respeta lo medido. Y sin updated_at.
		if _, err := e.db.Exec(
			`UPDATE observation_relations
			 SET lex_score = COALESCE(lex_score, ?), cosine_score = COALESCE(cosine_score, ?)
			 WHERE id = ?`,
			lex, floatOrNil(cos), p.id,
		); err != nil {
			return res, fmt.Errorf("error al escribir el desglose de la relación %q: %w", p.id, err)
		}
	}
	return res, nil
}
