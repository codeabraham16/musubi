package memory

import "sort"

// band.go implementa la BANDA CIEGA: el rango de coseno [BandFloor, CosineFloor) donde viven las
// CONTRADICCIONES, y donde hasta ahora Musubi no miraba.
//
// POR QUÉ EXISTE UNA BANDA APARTE (y no alcanza con bajar el piso del dedup)
//
// El piso del dedup (0.85) está calibrado sobre DUPLICADOS: los casi-idénticos dan coseno ~0.99.
// Pero una CONTRADICCIÓN no es un duplicado — decir LO CONTRARIO usa OTRAS palabras — así que vive
// estructuralmente MÁS ABAJO en la escala. El detector está afinado para encontrar REDUNDANCIA, y
// la contradicción es su opuesto. Pedirle a UN umbral que haga los DOS trabajos es lo que produjo
// el falso negativo real que originó este archivo:
//
//	"NordVPN y Tailscale NO pueden coexistir"  vs  "SÍ coexisten, acá está la receta"
//	  coseno 0.806 (piso 0.85 ✗)   jaccard 0.213 (piso 0.30 ✗)   => NUNCA se relacionaron
//
// ...y sin embargo ese 0.806 es MÁS SIMILAR QUE EL 99% de los 94.830 pares reales medidos. No es
// una señal débil perdida en el ruido: es de las más fuertes que hay.
//
// Bajar el piso a 0.80 lo habría atrapado, pero TRIPLICA la cola de pendientes (medido: x2.9), o
// sea ~3 veredictos extra por CADA memoria nueva. Es volver a llenar de ruido la cola que las
// guardas estructurales acaban de limpiar.
//
// MOSTRAR NO ES ENCOLAR — la distinción que resuelve el trade-off
//
// La falla real no fue que el detector no DECIDIERA: fue que nunca le MOSTRÓ el par al agente.
//
//   - ENCOLAR una relación cuesta caro: exige un veredicto y VIVE en la cola hasta que alguien lo dé.
//   - MOSTRARLE los vecinos al agente al guardar cuesta ~cero: ya está ahí, ya tiene el contexto, y
//     puede actuar o ignorar en el acto.
//
// Por eso este archivo es de SOLO LECTURA. No importa UpsertObsRelation, no lo conoce, no puede
// crear una relación AUNQUE QUISIERA. El invariante no depende de que alguien se acuerde de no
// persistir: es imposible llegar ahí.
//
// LÍMITE DECLARADO: esto NO detecta toda contradicción. Una con coseno < BandFloor sigue invisible.
// Y NO decide si hay contradicción — evaluar el predicado ("¿esto niega aquello?") es el techo
// semántico de los embeddings estáticos, y ese juicio es del agente.

// maxBandNeighbors es el techo de vecinos que se le muestran al agente. Un aviso largo se ignora
// igual que una cola larga: la erosión no se cura mudándola de lugar.
const maxBandNeighbors = 3

// BandNeighbor es una memoria que habla del mismo tema SIN ser un duplicado. No es una relación:
// es un aviso.
type BandNeighbor struct {
	ID       string
	TopicKey string
	Gist     string
	Cosine   float64
}

// bandEnabled: la banda necesita un piso propio POR DEBAJO del piso del dedup. Si BandFloor <= 0 o
// alcanza a CosineFloor, no hay banda que mirar (y es el switch de rollback por config).
func (o ConflictOptions) bandEnabled() bool {
	return o.BandFloor > 0 && o.BandFloor < o.CosineFloor
}

// BandNeighbors devuelve las memorias que caen en la banda ciega de obsID, ordenadas por coseno
// descendente y recortadas a maxBandNeighbors, más CUÁNTAS quedaron afuera del techo.
//
// SOLO LECTURA: no escribe nada, no persiste nada, no oculta nada.
func (e *DbEngine) BandNeighbors(obsID string, opts ConflictOptions) ([]BandNeighbor, int, error) {
	opts = opts.withDefaults()
	if !opts.bandEnabled() || !opts.cosineEnabled() {
		return nil, 0, nil
	}

	src, ok, err := e.loadObsRow(obsID)
	if err != nil || !ok {
		return nil, 0, err
	}

	// La banda es una noción PURAMENTE vectorial: sin vector de la procedencia actual no hay banda
	// (no se degrada a léxico — el léxico es justamente la señal que no ve la contradicción).
	srcVec, err := e.observationVector(obsID)
	if err != nil {
		return nil, 0, err
	}
	if srcVec == nil {
		return nil, 0, nil
	}

	// Mismo pool y mismos cosenos que el detector: lo que se le muestra al agente tiene que ser LO
	// MISMO que vio el detector, medido igual.
	cands, err := e.conflictCandidates(src, srcVec, opts.CandidatePool)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]string, 0, len(cands))
	for _, c := range cands {
		ids = append(ids, c.id)
	}
	cosines, err := e.candidateCosines(srcVec, ids)
	if err != nil {
		return nil, 0, err
	}

	var out []BandNeighbor
	for _, c := range cands {
		// Las guardas estructurales valen acá también: sería absurdo sacar el ruido de la cola por
		// una puerta y metérselo al agente por la otra.
		if complementaryPair(src, c) {
			continue
		}
		cos, ok := cosines[c.id]
		if !ok {
			continue
		}
		// Semiabierta a propósito: si alcanza CosineFloor, el par YA es una relación `pending` y el
		// agente lo ve por el camino de siempre. Avisar dos veces por lo mismo entrena a ignorar.
		if cos < opts.BandFloor || cos >= opts.CosineFloor {
			continue
		}
		out = append(out, BandNeighbor{ID: c.id, TopicKey: c.topicKey, Gist: e.gistOf(c), Cosine: cos})
	}

	sort.SliceStable(out, func(i, j int) bool { return out[i].Cosine > out[j].Cosine })

	// El recorte se INFORMA, no se esconde: un truncado silencioso le dice al agente "esto es todo"
	// cuando no lo es, y esa falsa cobertura es peor que no avisar.
	omitted := 0
	if len(out) > maxBandNeighbors {
		omitted = len(out) - maxBandNeighbors
		out = out[:maxBandNeighbors]
	}
	return out, omitted, nil
}

// gistOf devuelve el resumen guardado de la observación (lo que el recall ya usa para no quemar
// tokens); si no lo tiene, cae al contenido.
func (e *DbEngine) gistOf(c obsRow) string {
	var gist string
	err := e.db.QueryRow(`SELECT COALESCE(gist,'') FROM observations WHERE id=?`, c.id).Scan(&gist)
	if err != nil || gist == "" {
		return c.content
	}
	return gist
}
