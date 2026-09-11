package recalleval

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"

	_ "modernc.org/sqlite" // driver puro-Go, el mismo que usa internal/memory
)

// FIXTURE DESDE MEMORIA REAL.
//
// POR QUÉ ESTO NO ESCRIBE NADA AL REPO. El repo de Musubi es PÚBLICO (verificado). La memoria real
// tiene IPs del tailnet, nombres de servicios y decisiones internas; volcarla a `testdata/` la
// publicaría. Por eso el fixture generado vive SÓLO EN MEMORIA, mientras dura la corrida: no hay
// archivo que alguien pueda commitear por descuido.
//
// LA DIVISIÓN DE TRABAJO CON EL FIXTURE DORADO:
//   - `testdata/golden.json` (sintético, versionado) es la RED DE REGRESIÓN: chico, determinista,
//     corre en CI y grita si el ranking model-free se degrada.
//   - éste es la MEDICIÓN: 1.210 docs contra 26 consultas —46× el corpus del dorado— que es la
//     escala donde una diferencia entre dos arms significa algo.
//
// Ninguno reemplaza al otro: uno protege, el otro mide.

// EtiquetadoPorTopico es de dónde salen las etiquetas de relevancia, y merece explicación porque es
// lo único que hace creíble a un fixture automático.
//
// La relevancia se deriva del `topic_key`: las observaciones que comparten topic son las relevantes
// para una consulta sobre ese topic. Lo que lo vuelve defendible es que **el topic lo asignó el
// autor al escribir la nota**, mucho antes y con total independencia de cómo se recupera. Derivar
// las etiquetas del propio ranking —o de un LLM— haría un banco circular: mediría si el ranker
// coincide consigo mismo.
//
// ★ LA LIMITACIÓN, DICHA DE FRENTE. Este etiquetado asume dos cosas que son falsas en general:
// que todo lo del topic es igual de relevante, y que NADA fuera del topic lo es. Una nota en
// `roadmap/track-potencia-medida` y otra en `cognicion/donde-esta-encendido-el-motor` pueden ser las
// dos relevantes para «el motor de cognición», y acá la segunda cuenta como fallo.
//
// Consecuencia: los valores ABSOLUTOS de Recall@k y nDCG salen SUBESTIMADOS y no hay que leerlos
// como «el recall es malo». Lo que sí es válido es el DELTA entre dos arms sobre el mismo fixture,
// porque el sesgo es idéntico para los dos — y el delta es justamente lo que F2 vino a medir.
const EtiquetadoPorTopico = "topic_key"

// OpcionesFixtureReal acota qué entra al fixture generado.
type OpcionesFixtureReal struct {
	// MinPorTopico es cuántas observaciones necesita un topic para volverse consulta. Con menos de
	// 2 relevantes las métricas de orden casi no informan. 0 ⇒ 3.
	MinPorTopico int
	// MaxPorTopico descarta los topics DEMASIADO grandes. Existe por un caso medido: `git-commit`
	// tiene 247 observaciones en la memoria de este proyecto — es un cajón de sastre, no un tema, y
	// una consulta con 247 relevantes de 1.210 docs no mide nada. 0 ⇒ 50.
	MaxPorTopico int
	// PrefijosExcluidos saca familias enteras de topics. 0 elementos ⇒ el default de abajo.
	PrefijosExcluidos []string
	// TextoDelDoc decide qué texto representa a cada documento: "content" (default) o "gist".
	//
	// EL DEFAULT CAMBIÓ, Y EL MOTIVO ES UNA MEDICIÓN. Antes era el gist, y eso hacía que el banco
	// midiera un brazo vectorial QUE EN PRODUCCIÓN NO EXISTE: el backfill embebe `p.content`
	// completo (embed_backfill.go), mientras que SeedEngine embebía lo que este campo devuelve.
	// Medido sobre la base real: el gist promedia 82 caracteres y el content 2.790 — el gist es el
	// 2,9% del texto. O sea que el banco vectorizaba el 3% de lo que vectoriza producción, y
	// cualquier conclusión sobre "la señal vectorial" salía de ahí.
	//
	// Se conserva "gist" como opción, y no por nostalgia: es la única forma de comparar las dos
	// representaciones en la misma corrida y ver cuánto de un delta viene de la representación y
	// cuánto del ranker.
	TextoDelDoc string
}

// Representaciones posibles de un documento en el fixture. Ver OpcionesFixtureReal.TextoDelDoc.
const (
	DocDesdeContent = "content"
	DocDesdeGist    = "gist"
)

// prefijosExcluidosPorDefecto son familias que no son TEMAS sino registros mecánicos: no describen
// un asunto sobre el que alguien preguntaría, así que como consulta no significan nada.
var prefijosExcluidosPorDefecto = []string{"git-commit", "sdd/", "project/profile"}

func (o OpcionesFixtureReal) conDefaults() OpcionesFixtureReal {
	if o.MinPorTopico <= 0 {
		o.MinPorTopico = 3
	}
	if o.MaxPorTopico <= 0 {
		o.MaxPorTopico = 50
	}
	if len(o.PrefijosExcluidos) == 0 {
		o.PrefijosExcluidos = prefijosExcluidosPorDefecto
	}
	if o.TextoDelDoc == "" {
		o.TextoDelDoc = DocDesdeContent
	}
	return o
}

// ConsultaDesdeTopico convierte un topic_key en el texto de la consulta: `roadmap/track-potencia-medida`
// ⇒ `roadmap track potencia medida`.
//
// Es a propósito una transformación TONTA y sin criterio propio. Cualquier cosa más inteligente
// —elegir palabras del contenido, pedirle una consulta a un LLM— metería en la consulta información
// derivada de lo que se está midiendo, y ahí el banco empieza a evaluarse a sí mismo.
func ConsultaDesdeTopico(topic string) string {
	r := strings.NewReplacer("/", " ", "-", " ", "_", " ")
	return strings.Join(strings.Fields(r.Replace(topic)), " ")
}

// FixtureDesdeDB arma un fixture desde una base de memoria real de Musubi, en SÓLO LECTURA.
//
// Los docs son TODAS las observaciones vivas (el corpus completo, que es lo que le da escala a la
// medición) y las consultas salen de los topics que pasan los filtros.
func FixtureDesdeDB(rutaDB string, opts OpcionesFixtureReal) (*Fixture, error) {
	opts = opts.conDefaults()

	// mode=ro: esto lee la memoria de trabajo de alguien. No la toca ni por accidente.
	db, err := sql.Open("sqlite", "file:"+rutaDB+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("abrir %s: %w", rutaDB, err)
	}
	defer db.Close()

	// El texto del doc sale de la representación pedida. Con "gist" se cae al content cuando el
	// gist está vacío, que es el comportamiento histórico.
	colTexto := `content`
	if opts.TextoDelDoc == DocDesdeGist {
		colTexto = `COALESCE(NULLIF(gist,''), content)`
	}
	// EL PREDICADO DE VISIBILIDAD, COMPLETO. Acá decía `COALESCE(archived,0) = 0 AND superseded_by
	// IS NULL` escrito a mano, y le faltaba `quarantined = 0`: el fixture metía al corpus
	// observaciones marcadas como NO CONFIABLES, que el recall real nunca devuelve. O sea que el
	// banco medía contra un corpus que producción no tiene.
	//
	// Hoy la base real tiene 0 cuarentenadas, así que el arreglo no mueve ningún número medido —
	// y se hace igual, porque el día que haya una el banco habría empezado a mentir en silencio.
	// Es la TERCERA vez en esta rama que aparece el mismo defecto (SampleContents, buildObsGraph,
	// éste): el predicado no se reescribe, se interpola desde memory.
	filas, err := db.Query(`
		SELECT id, COALESCE(topic_key,''), ` + colTexto + `
		FROM observations
		WHERE COALESCE(archived,0) = 0 AND superseded_by IS NULL AND COALESCE(quarantined,0) = 0
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("leer observaciones: %w", err)
	}
	defer filas.Close()

	fx := &Fixture{}
	porTopico := map[string][]string{}
	for filas.Next() {
		var id, topic, texto string
		if err := filas.Scan(&id, &topic, &texto); err != nil {
			return nil, err
		}
		if strings.TrimSpace(texto) == "" {
			continue
		}
		fx.Docs = append(fx.Docs, Doc{ID: id, Topic: topic, Content: texto})
		if topic != "" {
			porTopico[topic] = append(porTopico[topic], id)
		}
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	if len(fx.Docs) == 0 {
		return nil, fmt.Errorf("%s no tiene observaciones vivas", rutaDB)
	}

	topics := make([]string, 0, len(porTopico))
	for t := range porTopico {
		topics = append(topics, t)
	}
	sort.Strings(topics) // orden determinista: el fixture no puede cambiar entre corridas

	for _, t := range topics {
		ids := porTopico[t]
		if len(ids) < opts.MinPorTopico || len(ids) > opts.MaxPorTopico || tieneAlgunPrefijo(t, opts.PrefijosExcluidos) {
			continue
		}
		fx.Queries = append(fx.Queries, Query{
			ID:       "topic:" + t,
			Text:     ConsultaDesdeTopico(t),
			Relevant: ids,
			Note:     "etiquetas derivadas del topic_key (asignado por el autor, independiente del ranker)",
		})
	}
	if len(fx.Queries) == 0 {
		return nil, fmt.Errorf("ningún topic de %s pasó los filtros (min=%d, max=%d)", rutaDB, opts.MinPorTopico, opts.MaxPorTopico)
	}
	return fx, nil
}

func tieneAlgunPrefijo(s string, prefijos []string) bool {
	for _, p := range prefijos {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
