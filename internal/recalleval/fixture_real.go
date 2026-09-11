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

// EtiquetadoPorExpansion es la OTRA fuente de etiquetas, y existe porque la de arriba no puede
// cerrar la discusión sobre MMR.
//
// La etiqueta sale de `expand_count`: de los documentos de un tópico, son relevantes los que ALGÚN
// agente pidió ENTEROS después de ver el titular. Eso no lo deriva nadie de la similitud — es una
// elección tomada con el gist a la vista, y el ranker no la puede fabricar.
//
// POR QUÉ IMPORTA JUSTO ACÁ. Con `topic_key`, los relevantes de una consulta son por construcción
// los que se parecen entre sí, y MMR separa lo que se parece: el costo de relevancia que el banco
// le cobra a la diversificación está inflado de fábrica, y por eso el barrido de mmr_real_test.go
// se lee como COTA SUPERIOR y no como veredicto. Esta etiqueta no tiene ese sesgo.
//
// ★ PERO TIENE EL SUYO, Y ES DE SUPERVIVENCIA: sólo se puede expandir lo que el ranker SIRVIÓ
// primero. Un documento que el ranking actual nunca muestra no puede juntar expansiones, así que
// esta etiqueta le da la derecha al ranker que la produjo. No es el mismo sesgo que el del tópico
// —no premia el parecido— y ése es el punto; pero leerla como «la verdad» sería cambiar un sesgo
// conocido por otro sin decirlo.
//
// Las dos juntas acotan: si un cambio mejora con AMBAS etiquetas, no lo está haciendo por el sesgo
// de ninguna.
//
// ★★ ARRANCA VACÍA. La columna se empezó a escribir en la v57 (2026-09-11) y no se pudo rellenar:
// las expansiones anteriores están sumadas adentro de `access_count` sin forma de restarlas, y el
// ledger de uso no guarda argumentos a propósito. Hasta que se acumule uso real, esta fuente no
// tiene con qué etiquetar — y lo dice fallando, no devolviendo un fixture flaco.
const EtiquetadoPorExpansion = "expand_count"

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
	// Etiquetado elige de dónde salen las etiquetas de relevancia: EtiquetadoPorTopico (default,
	// el histórico) o EtiquetadoPorExpansion. Ver las dos constantes: no son intercambiables, son
	// dos sesgos distintos, y lo útil es correr las dos.
	Etiquetado string
	// MinExpandidosPorTopico es cuántos documentos EXPANDIDOS necesita un tópico para volverse
	// consulta bajo EtiquetadoPorExpansion. 0 ⇒ 2: con un solo relevante, Recall@10 sólo puede dar
	// 0 o 1 y nDCG deja de informar sobre el orden. No se mezcla con MinPorTopico, que cuenta el
	// tamaño del tópico y no el de la etiqueta.
	MinExpandidosPorTopico int
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
	if o.Etiquetado == "" {
		o.Etiquetado = EtiquetadoPorTopico
	}
	if o.MinExpandidosPorTopico <= 0 {
		o.MinExpandidosPorTopico = 2
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
	// Es la TERCERA vez en esta rama que aparece el mismo defecto: el predicado no se reescribe,
	// se interpola. Las otras tres: SampleContents (arreglado), buildObsGraph (arreglado con
	// visibleObsPredicateDe, que hubo que crear porque necesitaba la forma con alias) y
	// TopicExists (arreglado). Cuatro reimplementaciones del mismo filtro, encontradas de a una.
	// `expand_count` SE CONSULTA SÓLO SI SE LA PIDIÓ, y su ausencia es un ERROR, no un silencio.
	//
	// El fixture abre la base en mode=ro: no puede migrarla. Contra una base anterior a la v57 la
	// columna no existe, y las dos salidas cómodas son trampas: caerse al etiquetado por tópico
	// daría un informe de aspecto impecable midiendo OTRA COSA que la pedida, y un COALESCE sobre
	// una columna inexistente ni siquiera compila en SQLite. Se falla y se dice por qué.
	colExpand := "0"
	if opts.Etiquetado == EtiquetadoPorExpansion {
		tiene, err := tieneColumna(db, "observations", "expand_count")
		if err != nil {
			return nil, err
		}
		if !tiene {
			return nil, fmt.Errorf("%s no tiene la columna expand_count (esquema anterior a la v57): "+
				"esta base no puede etiquetar por expansión", rutaDB)
		}
		colExpand = "COALESCE(expand_count,0)"
	}

	filas, err := db.Query(`
		SELECT id, COALESCE(topic_key,''), ` + colTexto + `, ` + colExpand + `
		FROM observations
		WHERE COALESCE(archived,0) = 0 AND superseded_by IS NULL AND COALESCE(quarantined,0) = 0
		ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("leer observaciones: %w", err)
	}
	defer filas.Close()

	fx := &Fixture{}
	porTopico := map[string][]string{}
	expandido := map[string]bool{}
	for filas.Next() {
		var id, topic, texto string
		var expandCount int
		if err := filas.Scan(&id, &topic, &texto, &expandCount); err != nil {
			return nil, err
		}
		if strings.TrimSpace(texto) == "" {
			continue
		}
		fx.Docs = append(fx.Docs, Doc{ID: id, Topic: topic, Content: texto})
		if topic != "" {
			porTopico[topic] = append(porTopico[topic], id)
		}
		// UNA VEZ ALCANZA, y el contador no se usa como peso. Que a un documento lo hayan expandido
		// nueve veces no lo vuelve nueve veces más relevante: lo que la señal dice es que alguien lo
		// eligió teniendo el titular delante, y eso es un sí o un no. Pesar por el contador
		// reintroduciría por la ventana el rich-get-richer que N4 saca por la puerta.
		if expandCount > 0 {
			expandido[id] = true
		}
	}
	if err := filas.Err(); err != nil {
		return nil, err
	}
	if len(fx.Docs) == 0 {
		return nil, fmt.Errorf("%s no tiene observaciones vivas", rutaDB)
	}

	// LAS ARISTAS DEL GRAFO, que son lo que hace que la quinta señal RRF exista en el banco.
	// Sin esto graphRank sale vacío en toda medición y una config con GraphCentrality:true queda
	// bit-idéntica a una con false — o sea que el gate defendía una señal que nunca ejercitaba.
	//
	// Se leen SÓLO las aristas cuyas dos puntas estén en el corpus, con el mismo criterio de
	// visibilidad de arriba: una arista hacia una observación que el fixture no incluye es una
	// relación huérfana, que es exactamente lo que el check orphan_relations del doctor busca.
	vivos := make(map[string]bool, len(fx.Docs))
	for _, d := range fx.Docs {
		vivos[d.ID] = true
	}
	aristas, err := db.Query(`SELECT source_id, target_id FROM observation_relations ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("leer relaciones: %w", err)
	}
	defer aristas.Close()
	for aristas.Next() {
		var src, tgt string
		if err := aristas.Scan(&src, &tgt); err != nil {
			return nil, err
		}
		if src == tgt || !vivos[src] || !vivos[tgt] {
			continue
		}
		fx.Relaciones = append(fx.Relaciones, Relacion{Source: src, Target: tgt})
	}
	if err := aristas.Err(); err != nil {
		return nil, err
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
		// El TÓPICO sigue decidiendo cuáles son las CONSULTAS en los dos etiquetados; lo que cambia
		// es cuáles de sus documentos cuentan como relevantes. Así las dos corridas preguntan lo
		// mismo y sólo discrepan en la etiqueta, que es la única forma de atribuirle un delta a la
		// etiqueta y no a otra cosa.
		q := Query{
			ID:       "topic:" + t,
			Text:     ConsultaDesdeTopico(t),
			Relevant: ids,
			Note:     "etiquetas derivadas del topic_key (asignado por el autor, independiente del ranker)",
		}
		if opts.Etiquetado == EtiquetadoPorExpansion {
			elegidos := make([]string, 0, len(ids))
			for _, id := range ids {
				if expandido[id] {
					elegidos = append(elegidos, id)
				}
			}
			if len(elegidos) < opts.MinExpandidosPorTopico {
				continue
			}
			q.ID = "expand:" + t
			q.Relevant = elegidos
			q.Note = "etiquetas derivadas de expand_count (elegidas por un agente con el gist a la vista; " +
				"sesgo de supervivencia: sólo se expande lo que el ranker sirvió)"
		}
		fx.Queries = append(fx.Queries, q)
	}
	if len(fx.Queries) == 0 {
		// EL MENSAJE NOMBRA EL FILTRO QUE DE VERDAD MORDIÓ. Con etiquetado por expansión, decir
		// «min=3, max=50» manda a tocar los umbrales del tópico cuando lo que falta es uso: la
		// columna arrancó vacía en la v57 y se llena sola a medida que alguien expande.
		if opts.Etiquetado == EtiquetadoPorExpansion {
			return nil, fmt.Errorf("ningún topic de %s tiene al menos %d documentos expandidos: "+
				"la señal de expansión se empezó a registrar en la v57 y no se pudo rellenar hacia atrás, "+
				"así que todavía no hay con qué etiquetar", rutaDB, opts.MinExpandidosPorTopico)
		}
		return nil, fmt.Errorf("ningún topic de %s pasó los filtros (min=%d, max=%d)", rutaDB, opts.MinPorTopico, opts.MaxPorTopico)
	}
	return fx, nil
}

// tieneColumna responde si una tabla tiene una columna, sobre una base abierta en sólo lectura.
//
// Es el equivalente de lectura de agregarColumnaSiFalta (internal/memory/migrations.go), y existe
// separado a propósito: acá no se puede arreglar la falta —la base se abre en mode=ro—, sólo
// detectarla para poder fallar con un motivo.
func tieneColumna(db *sql.DB, tabla, columna string) (bool, error) {
	filas, err := db.Query(`PRAGMA table_info(` + tabla + `)`)
	if err != nil {
		return false, fmt.Errorf("leer columnas de %s: %w", tabla, err)
	}
	defer filas.Close()
	for filas.Next() {
		var (
			cid         int
			name, ctype string
			notnull, pk int
			dflt        interface{}
		)
		if err := filas.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return false, fmt.Errorf("escanear PRAGMA table_info(%s): %w", tabla, err)
		}
		if name == columna {
			return true, nil
		}
	}
	return false, filas.Err()
}

func tieneAlgunPrefijo(s string, prefijos []string) bool {
	for _, p := range prefijos {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
