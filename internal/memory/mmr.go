package memory

import (
	"fmt"
	"strings"
)

// mmr.go implementa la DIVERSIDAD del recall (MMR — Maximal Marginal Relevance).
//
// EL PROBLEMA QUE RESUELVE (medido, no supuesto)
//
// El ranker fusiona SIETE señales y hace bien su trabajo... pero NINGUNA mira lo que YA se eligió.
// Optimiza RELEVANCIA POR ITEM; nadie optimiza la utilidad del CONJUNTO — y el presupuesto de
// tokens es del CONJUNTO.
//
// Medido sobre la memoria real, consulta "banda ciega y detección de contradicciones" (60 items):
// aparecían LAS SIETE fases SDD de un cambio, LAS SIETE de otro y 5 de un tercero ⇒ 19 de 60 items
// (casi un tercio del presupuesto) eran 3 cambios contados siete veces cada uno. Varios sin aportar
// nada: el gist de `tasks` es literalmente "17 tareas.". Y la nota del principio destilado —el item
// más útil— quedaba 6ª, DEBAJO de 5 contratos del mismo cambio.
//
// Es un fallo de REDUNDANCIA, no de relevancia. Los 7 contratos SÍ son relevantes; lo que sobra es
// que estén los siete.
//
// LA TRAMPA DE ESCALAS (y por qué el coseno CRUDO no sirve como penalización)
//
// Dos razones, las dos fatales:
//
//  1. ESCALAS INCOMPATIBLES. El score RRF vive en ~0.05-0.11; el coseno, en 0.60-0.99. En
//     `λ·rel − (1−λ)·cos`, la penalización es DIEZ VECES más grande que la señal a la que se resta:
//     MMR dejaría de ser un ajuste y pasaría a SER el ranking.
//
//  2. TODO SE PARECE A TODO. Medido sobre 94.830 pares reales: dos memorias CUALESQUIERA del corpus
//     tienen coseno mediano 0.60. Penalizar sobre esa base es castigar a los items POR ESTAR
//     ESCRITOS EN EL MISMO IDIOMA y hablar del mismo dominio.
//
// Por eso la penalización mide REDUNDANCIA, no similitud: 0 en la línea de base medida, 1 en el
// duplicado exacto. Muerde donde hay redundancia REAL y calla donde no.
//
// MMR REORDENA, NO DESCARTA. Un item redundante BAJA de posición; si el presupuesto alcanza, sigue
// estando. Lo único que cambia es el ORDEN EN QUE SE GASTA EL PRESUPUESTO.

// redundancyBase es el coseno MEDIANO entre dos memorias CUALESQUIERA del corpus (medido: p50=0.60
// sobre 94.830 pares reales). Es el "piso del idioma": lo que se parecen dos textos por el solo
// hecho de estar en español y hablar de software. Por debajo de esto NO hay redundancia que
// penalizar — hay coincidencia de vocabulario.
const redundancyBase = 0.60

// redundancy reescala el coseno a [0,1]: 0 en la línea de base, 1 en el duplicado exacto.
// Un coseno de 0.98 (dos fases del mismo cambio) ⇒ 0.95. Uno de 0.62 (memorias ajenas) ⇒ 0.05.
func redundancy(cos float64) float64 {
	if cos <= redundancyBase {
		return 0
	}
	return (cos - redundancyBase) / (1 - redundancyBase)
}

// normalizeScores lleva los scores a (0,1] dividiendo por el MÁXIMO, para que la relevancia y la
// redundancia sean COMPARABLES. Sin normalizar, λ no sería un dial: sería un número mágico distinto
// para cada consulta, porque el rango del RRF depende de cuántas candidatas haya.
//
// POR QUÉ POR EL MÁXIMO Y NO MIN-MAX (lo aprendí rompiendo un test):
//
//	Min-max DEFORMA. Manda a CERO al candidato menos relevante — que entonces no puede subir por
//	mucha información nueva que aporte —, e INFLA las diferencias chicas: un RRF de 0.100 vs 0.080
//	(un 20% de distancia real) se convertiría en 1.0 vs 0.0, un abismo del 100%.
//
//	Los scores RRF son POSITIVOS y sus RATIOS son significativos. Dividir por el máximo preserva las
//	distancias reales; min-max las inventa. Y si la escala está mal, λ deja de significar algo.
//
// Todos iguales (o máximo <= 0) ⇒ todos 1.0: ninguna se privilegia y decide sólo la diversidad.
func normalizeScores(scored []scoredCandidate) []float64 {
	out := make([]float64, len(scored))
	if len(scored) == 0 {
		return out
	}
	hi := scored[0].score
	for _, s := range scored {
		hi = max(hi, s.score)
	}
	if hi <= 0 {
		for i := range out {
			out[i] = 1
		}
		return out
	}
	for i, s := range scored {
		out[i] = s.score / hi
	}
	return out
}

// vectorsFor trae los vectores de ids con la procedencia ACTUAL, en una sola consulta. Un id sin
// vector simplemente no está en el mapa (y NO se penaliza: ver diversify).
//
// OJO — acá NO va el atajo `if e.vectorModelID == "" { return }` que sí tiene observationVector.
// El filtro por model_id ya garantiza la procedencia: si el model_id es "", TODOS los embeddings de
// la base lo tienen, así que compararlos entre sí es legítimo (es un solo modelo, el anónimo). Con
// el atajo, MMR quedaba INERTE EN SILENCIO en cualquier base sin model_id — y así fue como el
// primer barrido de λ dio el MISMO R@10 para todos los valores: no estaba midiendo nada.
func (e *DbEngine) vectorsFor(ids []string) (map[string][]float32, error) {
	out := make(map[string][]float32, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	const chunk = 500
	for start := 0; start < len(ids); start += chunk {
		end := min(start+chunk, len(ids))
		batch := ids[start:end]

		ph := strings.TrimSuffix(strings.Repeat("?,", len(batch)), ",")
		args := make([]any, 0, len(batch)+1)
		for _, id := range batch {
			args = append(args, id)
		}
		args = append(args, e.vectorModelID)

		rows, err := e.db.Query(
			`SELECT observation_id, vector FROM embeddings WHERE observation_id IN (`+ph+`) AND model_id = ?`, args...)
		if err != nil {
			return nil, fmt.Errorf("error al leer los vectores para diversificar: %w", err)
		}
		for rows.Next() {
			var id string
			var b []byte
			if err := rows.Scan(&id, &b); err != nil {
				rows.Close()
				return nil, err
			}
			v, err := BytesToFloat32(b)
			if err != nil {
				continue // vector ilegible: sin vector ⇒ sin penalización (degradación segura)
			}
			out[id] = v
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// diversify reordena las candidatas por MMR: en cada paso elige la que maximiza
//
//	λ·relevancia_normalizada − (1−λ)·máxima_redundancia_contra_las_YA_elegidas
//
// λ >= 1 lo APAGA: devuelve la entrada SIN TOCARLA y sin siquiera consultar la base. El
// comportamiento anterior no es "aproximadamente" idéntico: es INALCANZABLEMENTE idéntico.
//
// λ <= 0 TAMBIÉN lo apaga, y eso NO es un detalle: el cero es el valor por defecto de Go, así que
// un caller que arme RecallOptions{} sin pensar en la diversidad recibiría "diversidad pura,
// relevancia cero" — el ranking destruido en silencio. El valor no seteado tiene que ser INOFENSIVO.
func (e *DbEngine) diversify(scored []scoredCandidate, lambda float64) []scoredCandidate {
	if lambda >= 1 || lambda <= 0 || len(scored) < 2 {
		return scored
	}

	ids := make([]string, len(scored))
	for i, s := range scored {
		ids[i] = s.id
	}
	vecs, err := e.vectorsFor(ids)
	if err != nil || len(vecs) == 0 {
		return scored // sin vectores no hay noción de redundancia: se respeta el orden del RRF
	}

	rel := normalizeScores(scored)

	out := make([]scoredCandidate, 0, len(scored))
	usada := make([]bool, len(scored))
	// maxRed[i] = redundancia de i contra la MÁS parecida de las ya elegidas. Se actualiza de a una
	// (en vez de recorrer todas las elegidas en cada paso): el máximo es incremental.
	maxRed := make([]float64, len(scored))

	for range scored {
		mejor, mejorMMR := -1, 0.0
		for i := range scored {
			if usada[i] {
				continue
			}
			mmr := lambda*rel[i] - (1-lambda)*maxRed[i]
			// El primero elegido es siempre el de mayor relevancia: en la 1ª vuelta maxRed es 0 para
			// todos, así que el máximo de λ·rel es el más relevante.
			if mejor == -1 || mmr > mejorMMR {
				mejor, mejorMMR = i, mmr
			}
		}
		usada[mejor] = true
		out = append(out, scored[mejor])

		// Propagar la redundancia de la recién elegida al resto. Sin vector ⇒ no se toca su maxRed:
		// NUNCA se castiga a una memoria por no tener embedding (eso la enterraría por una razón
		// que no tiene nada que ver con su contenido).
		vm, ok := vecs[scored[mejor].id]
		if !ok {
			continue
		}
		for i := range scored {
			if usada[i] {
				continue
			}
			vi, ok := vecs[scored[i].id]
			if !ok {
				continue
			}
			cos, err := CosineSimilarity(vm, vi)
			if err != nil {
				continue
			}
			maxRed[i] = max(maxRed[i], redundancy(float64(cos)))
		}
	}
	return out
}
