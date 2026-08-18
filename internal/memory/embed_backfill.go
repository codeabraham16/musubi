package memory

import (
	"fmt"

	"musubi/internal/logx"
)

// embed_backfill.go implementa el RE-EMBEDDING del histórico (T17.3): cuando se enciende la
// memoria semántica sobre una base con observaciones previas, o cuando se cambia de embedder, esas
// observaciones quedan sin vector de la procedencia actual y son INVISIBLES para el recall
// semántico (la regla de homogeneidad sólo compara vectores del mismo model_id). WarnOnEmbedModelSwitch
// avisaba de ese hueco pero no ofrecía remedio; EmbedBackfill es el remedio. Model-free: el server
// no embebe, recibe el callback de vectorización del caller (el CLI, con el provider resuelto).

// stalePredicate es el WHERE que define "observación PENDIENTE de (re)embedding": activa y sin
// vector de la procedencia ACTUAL — sin fila en embeddings (LEFT JOIN nulo) o con model_id distinto
// (vector de otro modelo, ya excluido del recall por la regla de homogeneidad). Una sola fuente de
// verdad, compartida por el COUNT (AutoEmbedBackfill) y el SELECT (EmbedBackfill): si divergieran,
// el auto-backfill podría creer que no hay nada que hacer mientras el backfill sí tiene trabajo.
// Espera un solo parámetro: el model_id actual.
func stalePredicate() string {
	return visibleObsPredicate + ` AND (em.observation_id IS NULL OR em.model_id != ?)`
}

// embedBatchSize es cuántas observaciones se le pasan al embebedor de una vez.
//
// EL NÚMERO SALE DE MEDIR CONTRA EL EMBEBEDOR REAL (bge-m3 en el server, textos de ~1.100
// caracteres, 2026-08-17), no de elegir uno redondo:
//
//	lote    ms/texto   acel.
//	   1       917,5    1,00x
//	   4       758,9    1,21x
//	   8       686,5    1,34x
//	  16       670,1    1,37x   <- el codo
//	  32       669,9    1,37x
//	  64       654,7    1,40x
//
// ⚠️ Y LA MEDICIÓN CORRIGE LA CREENCIA QUE MOTIVÓ ESTE CAMBIO. Estaba anotado que el lote daba
// 4,58×; da 1,37×. El tiempo TOTAL crece casi lineal con el tamaño del lote (0,92 s → 41,90 s de
// 1 a 64), o sea que el modelo en CPU NO paraleliza el cómputo: lo único que el lote ahorra es la
// ida y vuelta HTTP y el arranque por pedido. Es una mejora real y modesta, no un salto.
//
// 16 y no 64 porque ahí la curva ya está plana, y el resto del tamaño sólo agrega riesgo: un fallo
// a mitad tira el lote entero (que hoy se reintenta de a uno, pero se paga re-embebiendo los 15
// inocentes) y el pedido HTTP crece con textos que pueden ser dossiers. Ganar 0,03× no paga nada.
const embedBatchSize = 16

// EmbedBackfillResult resume una corrida de re-embedding del histórico.
type EmbedBackfillResult struct {
	ModelID  string `json:"model_id"` // procedencia con la que se re-embebió
	Scanned  int    `json:"scanned"`  // observaciones activas que necesitaban (re)embedding
	Embedded int    `json:"embedded"` // vectores generados y persistidos
	Skipped  int    `json:"skipped"`  // el embedder devolvió vector vacío (no se persiste)
	Failed   int    `json:"failed"`   // el embedder RECHAZÓ ese texto; queda pendiente para la próxima corrida
}

// obsPendiente es una observación a la espera de vector. Vive a nivel de paquete (y no dentro de
// EmbedBackfill) porque el reintento uno-por-uno la necesita.
type obsPendiente struct{ id, content string }

// embedUnoAUno reintenta, texto por texto, un lote que falló entero.
//
// ⚠️ ESTE ES EL REMEDIO A UNA FALLA MEDIDA, NO UNA DEFENSA HIPOTÉTICA. En el cerebro central una
// sola observación de 11.700 caracteres que este ollama rechaza con 400 (y que `truncate` no
// salva) mantuvo al backfill parado TRES DÍAS: abortaba en la primera, y como la corrida
// resumible vuelve a empezar por la misma, las otras 32 pendientes no se embebían nunca.
// "Resumible" no alcanza cuando el primer ítem siempre falla.
//
// De paso arregla el mismo bloqueo por el lado del portero de privacidad: en modo `refuse` un solo
// texto con secreto tumba el lote entero, y acá pasa a costar sólo su propio lugar.
//
// LA REGLA DE CORTE, Y POR QUÉ SE MIDE EN VEZ DE DEDUCIRSE: que falle TODO el lote admite dos
// lecturas opuestas —"esos textos son imposibles" y "el embebedor está caído"— y confundirlas
// cuesta caro en las dos direcciones. Si se elige mal hacia el verde, una corrida que termina bien
// con 33 fallidas y 0 embebidas se lee como éxito y no la mira nadie. Si se elige mal hacia el
// rojo, el estado estacionario del cerebro central (UNA observación imposible, sola en la cola)
// grita "el embebedor está caído, corré el backfill a mano" en cada arranque — un diagnóstico
// falso con una instrucción que no arregla nada.
//
// Deducirlo del progreso de la corrida no alcanza: con un lote de una sola observación no hay
// evidencia con qué. Así que en vez de adivinar se le PREGUNTA al embebedor, con un texto trivial
// que cualquiera sano acepta. Si contesta, está vivo y la culpa es de los textos: se saltean, sin
// persistir nada, y siguen pendientes. Si tampoco puede con eso, está caído y se aborta.
// La sonda cuesta un pedido, y sólo en el camino en que ya falló todo.
func embedUnoAUno(embed func([]string) ([][]float32, error), lote []obsPendiente, causa error) ([][]float32, []error, error) {
	vecs := make([][]float32, len(lote))
	errs := make([]error, len(lote))
	fallaron := 0
	var ultimo error
	for i, p := range lote {
		v, err := embed([]string{p.content})
		if err != nil {
			errs[i], ultimo, fallaron = err, err, fallaron+1
			continue
		}
		// La misma garantía de cuenta que arriba, en su versión chica: uno adentro, uno afuera.
		if len(v) != 1 {
			errs[i] = fmt.Errorf("el embebedor devolvió %d vectores para 1 texto", len(v))
			ultimo, fallaron = errs[i], fallaron+1
			continue
		}
		vecs[i] = v[0]
	}
	// Si alguna entró, el embebedor ya demostró estar vivo y no hace falta preguntárselo.
	if fallaron == len(lote) {
		if _, err := embed([]string{textoSondaEmbed}); err != nil {
			return nil, nil, fmt.Errorf("el embebedor no pudo con ninguna de las %d observación(es) del lote (empieza en %s) NI con un texto trivial: está caído o mal configurado. En lote: %v. Individual: %v. Sonda: %w",
				len(lote), lote[0].id, causa, ultimo, err)
		}
	}
	return vecs, errs, nil
}

// textoSondaEmbed es la pregunta "¿estás vivo?" hecha texto. Corto y sin nada que pueda disparar
// al portero de privacidad: la sonda tiene que medir el transporte, no chocar contra una guarda.
const textoSondaEmbed = "musubi"

// EmbedBackfill (re)genera los embeddings de las observaciones ACTIVAS que no tienen un vector con
// la procedencia (model_id) del embedder ACTUAL — las guardadas antes de encender la semántica, o
// las de otro modelo tras un cambio de embedder. embed es el callback de vectorización (el CLI le
// pasa el provider resuelto). Estampa e.vectorModelID como procedencia (igual que un save normal),
// reconstruye el índice IVF UNA sola vez al final (más barato que Add por vector) y actualiza la
// marca MetaEmbedModel para que el aviso de cambio de modelo no vuelva a dispararse. Es idempotente
// y resumible: una fila ya re-embebida cambia su model_id al actual, así que una corrida posterior
// no la vuelve a listar. Requiere un embedder nombrado (e.vectorModelID != ""): sin él no hay
// semántica que backfillear.
func (e *DbEngine) EmbedBackfill(embed func([]string) ([][]float32, error)) (EmbedBackfillResult, error) {
	return e.embedBackfill(embed, false)
}

// EmbedBackfillAll re-embebe TODAS las observaciones activas, incluidas las que ya tienen vector de
// la procedencia actual.
//
// Existe porque «mismo model_id» no siempre significa «mismo vector»: si cambia CÓMO se embebe —y
// no con qué— la procedencia no alcanza para detectarlo. El caso que lo motivó: los textos largos
// se mandaban enteros y el embebedor devolvía un vector calculado sólo sobre el primer pedazo, en
// silencio; con el troceo ese mismo modelo ahora devuelve un vector del documento COMPLETO. Las
// filas viejas no están rancias por procedencia, pero están mal, y nada las volvería a listar.
//
// Es caro (re-embebe la base entera) y por eso es explícito, no automático.
func (e *DbEngine) EmbedBackfillAll(embed func([]string) ([][]float32, error)) (EmbedBackfillResult, error) {
	return e.embedBackfill(embed, true)
}

func (e *DbEngine) embedBackfill(embed func([]string) ([][]float32, error), todas bool) (EmbedBackfillResult, error) {
	res := EmbedBackfillResult{ModelID: e.vectorModelID}
	if e.vectorModelID == "" {
		return res, fmt.Errorf("no hay un embedder nombrado configurado; encendé la memoria semántica antes de backfillear")
	}
	if embed == nil {
		return res, fmt.Errorf("embed callback nil")
	}

	// Observaciones ACTIVAS sin vector de la procedencia actual: sin fila en embeddings (LEFT JOIN
	// nulo) o con model_id distinto (vector de otro modelo, excluido del recall por homogeneidad).
	consulta := `
		SELECT o.id, o.content
		FROM observations o
		LEFT JOIN embeddings em ON o.id = em.observation_id
		WHERE ` + stalePredicate()
	args := []any{e.vectorModelID}
	if todas {
		// Sin la condición de procedencia: entran también las que YA tienen vector del modelo
		// actual. El LEFT JOIN se conserva para no cambiar la forma de la consulta.
		consulta = `
		SELECT o.id, o.content
		FROM observations o
		WHERE ` + visibleObsPredicate
		args = nil
	}
	rows, err := e.db.Query(consulta, args...)
	if err != nil {
		return res, fmt.Errorf("error al listar observaciones a re-embeber: %w", err)
	}
	var todo []obsPendiente
	for rows.Next() {
		var p obsPendiente
		if err := rows.Scan(&p.id, &p.content); err != nil {
			rows.Close()
			return res, fmt.Errorf("error al escanear observación pendiente: %w", err)
		}
		todo = append(todo, p)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return res, fmt.Errorf("error al iterar observaciones pendientes: %w", err)
	}
	rows.Close()
	res.Scanned = len(todo)

	for inicio := 0; inicio < len(todo); inicio += embedBatchSize {
		fin := min(inicio+embedBatchSize, len(todo))
		lote := todo[inicio:fin]

		textos := make([]string, len(lote))
		for i, p := range lote {
			textos[i] = p.content
		}

		var fallos []error
		vecs, err := embed(textos)
		if err != nil {
			// UNA observación imposible NO puede seguir bloqueando a las otras 15. Se reintenta
			// texto por texto para que el costo del rechazo sea su propio lugar y nada más.
			// embedUnoAUno decide si esto es "un texto malo" o "el embebedor caído" (ver allá).
			logx.Warn("el lote falló entero; se reintenta texto por texto para aislar al culpable",
				"desde", lote[0].id, "textos", len(lote), "error", err)
			vecs, fallos, err = embedUnoAUno(embed, lote, err)
			if err != nil {
				// Acá sí se aborta, con el progreso ya persistido: la corrida es resumible.
				return res, fmt.Errorf("error al embeber el lote que empieza en %s: %w", lote[0].id, err)
			}
		}

		// ⚠️ LA GUARDA QUE NO SE PUEDE SALTEAR, Y POR QUÉ SE REPITE ACÁ.
		//
		// `embedding.EmbedBatch` ya garantiza la cuenta, pero a este paquete no le llega un
		// Provider: le llega una FUNCIÓN OPACA que arma el caller. Confiar en la garantía de un
		// paquete que no se puede ver desde acá es exactamente cómo un invariante se vuelve
		// folklore.
		//
		// Y el modo de falla es el peor posible: los vectores se aparean POR ÍNDICE con las
		// observaciones. Un lote que devuelve de menos no rompe nada visible — CORRE los vectores
		// una posición y le escribe a cada observación el embedding de OTRA. La memoria queda
		// semánticamente barajada, el recall empieza a traer cosas ajenas, y no hay ningún error
		// en ningún log que lo explique. Se aborta antes de escribir una sola fila.
		if len(vecs) != len(lote) {
			return res, fmt.Errorf("el embebedor devolvió %d vectores para un lote de %d (desde %s): se aborta antes de aparear vectores con observaciones equivocadas",
				len(vecs), len(lote), lote[0].id)
		}

		for i, p := range lote {
			// Fallida ≠ salteada: al vacío el embebedor le dijo "no tengo vector", a ésta le dijo
			// que NO. Se cuentan aparte y se nombra a la culpable, que es el dato que hace falta
			// para arreglarla; no se persiste nada, así que sigue pendiente para la próxima corrida.
			if fallos != nil && fallos[i] != nil {
				res.Failed++
				logx.Warn("el embebedor rechaza esta observación; se saltea y queda pendiente",
					"id", p.id, "caracteres", len(p.content), "error", fallos[i])
				continue
			}
			if len(vecs[i]) == 0 {
				res.Skipped++
				continue
			}
			vectorBytes, err := Float32ToBytes(vecs[i])
			if err != nil {
				return res, fmt.Errorf("error al serializar el vector de %s: %w", p.id, err)
			}
			if _, err := e.db.Exec(
				`INSERT OR REPLACE INTO embeddings (observation_id, vector, model_id) VALUES (?, ?, ?)`,
				p.id, vectorBytes, e.vectorModelID,
			); err != nil {
				return res, fmt.Errorf("error al guardar el embedding de %s: %w", p.id, err)
			}
			res.Embedded++
		}
	}

	// Reconstruir el índice IVF una sola vez (si hay índice) para que los vectores nuevos entren al
	// candidateo, y persistir la marca de modelo para que WarnOnEmbedModelSwitch no vuelva a avisar.
	if res.Embedded > 0 {
		if err := e.rebuildVectorIndex(); err != nil {
			return res, fmt.Errorf("re-embedding OK pero falló el rebuild del índice: %w", err)
		}
	}
	if err := e.SetMeta(MetaEmbedModel, e.vectorModelID); err != nil {
		return res, fmt.Errorf("re-embedding OK pero falló al persistir la marca de modelo: %w", err)
	}
	return res, nil
}

// countStaleEmbeddings cuenta las observaciones PENDIENTES de (re)embedding (ver stalePredicate).
func (e *DbEngine) countStaleEmbeddings() (int, error) {
	var n int
	err := e.db.QueryRow(`
		SELECT COUNT(*)
		FROM observations o
		LEFT JOIN embeddings em ON o.id = em.observation_id
		WHERE `+stalePredicate(), e.vectorModelID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("error al contar observaciones pendientes de embedding: %w", err)
	}
	return n, nil
}

// AutoEmbedBackfill (M3) cierra SOLO el hueco de procedencia, sin intervención manual: si hay
// observaciones activas sin vector del model_id ACTUAL —memoria previa a encender la semántica, o
// vectores de otra tabla tras un cambio de modelo/checksum (N1)— lanza EmbedBackfill EN BACKGROUND.
//
// Sin esto, cambiar de modelo APAGA el recall semántico (el contrato de procedencia excluye los
// vectores viejos) hasta que alguien corra `musubi embed backfill` a mano: el server avisaba del
// hueco pero no lo remediaba.
//
// Va en background y no síncrono a propósito: un daemon bajo systemd tiene timeout de arranque, y
// re-embeber una base grande tardaría minutos y haría FALLAR el arranque de la unit. spawnBackground
// ya resuelve el cierre limpio (no lanza si el engine está cerrado; Close espera a que termine).
//
// El engine sigue siendo model-free: recibe el callback de vectorización del caller, no embebe.
func (e *DbEngine) AutoEmbedBackfill(embed func([]string) ([][]float32, error)) {
	if e.vectorModelID == "" || embed == nil {
		return // sin semántica activa no hay nada que backfillear
	}
	n, err := e.countStaleEmbeddings()
	if err != nil {
		logx.Warn("no se pudo verificar si hay memoria sin vector del modelo actual", "error", err)
		return
	}
	if n == 0 {
		return // el caso común en cada arranque: nada pendiente, ni goroutine ni ruido en el log
	}
	// Visible, no silencioso: durante la ventana del backfill esas observaciones siguen excluidas
	// del recall semántico (degradación TEMPORAL, no corrupción).
	logx.Info("re-embebiendo memoria histórica en background",
		"pendientes", n, "modelo", e.vectorModelID,
		"nota", "hasta que termine, esas observaciones no aparecen en la búsqueda semántica")
	e.spawnBackground(func() {
		res, err := e.EmbedBackfill(embed)
		if err != nil {
			logx.Warn("el re-embedding automático falló; corré `musubi embed backfill` a mano",
				"error", err, "embebidas", res.Embedded)
			return
		}
		if res.Failed > 0 {
			// Que no se lea como un verde limpio: quedaron observaciones fuera del recall semántico
			// y el próximo arranque las va a volver a intentar. El id de cada una ya se logueó.
			logx.Warn("re-embedding automático completo, pero el embebedor rechazó algunas observaciones",
				"embebidas", res.Embedded, "fallidas", res.Failed, "omitidas", res.Skipped, "modelo", res.ModelID,
				"nota", "las fallidas siguen pendientes y fuera de la búsqueda semántica")
			return
		}
		logx.Info("re-embedding automático completo",
			"embebidas", res.Embedded, "omitidas", res.Skipped, "modelo", res.ModelID)
	})
}
