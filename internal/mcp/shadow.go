package mcp

import (
	"context"
	"strings"
	"sync"
	"time"

	"musubi/internal/cognition"
	"musubi/internal/logx"
	"musubi/internal/memory"
)

// shadow.go implementa el MODO SOMBRA del detector de conflictos: por cada veredicto model-free,
// preguntarle también al motor de cognición y guardar las dos lecturas lado a lado.
//
// LA LECTURA DEL MOTOR SE DESCARTA. No modifica la relación, no reencola nada, no aparece en la
// respuesta del save. Sólo se escribe en shadow_verdicts, que ninguna consulta del camino de
// decisión lee. Es la aplicación más literal del contrato del 3er pilar —«el motor PROPONE, nunca
// escribe al libro mayor»— llevada un paso más allá: acá ni siquiera propone.
//
// POR QUÉ ES ASÍNCRONO Y CON COLA ACOTADA. El detector corre dentro del guardado, que es camino
// caliente; una llamada al motor son segundos. Encolar y contestar al toque mantiene el save con
// la latencia de siempre. La cola tiene techo y lo que no entra se TIRA y se cuenta: perder
// evidencia es aceptable, hacer esperar un guardado por una medición no lo es.
//
// DE DÓNDE SALIÓ LA IDEA. Del bot de Altura, que corre un A/B en producción (`bot_sombra`) entre
// su diccionario determinista y el modelo, descartando la lectura del modelo. Lo que NO se copió
// es el criterio de éxito: allá alcanza el ACUERDO (si el modelo coincide el 97 %, el modelo no
// agrega y la fase se apaga). Acá el acuerdo no basta, porque «relacionadas» puede ser correcto
// para los dos por razones distintas. Lo que se busca es más angosto: en qué RANGO LÉXICO se
// concentra el desacuerdo sobre los `supersedes`, que es lo que dice si el umbral está mal puesto.

// vocabularioSombra es el mismo vocabulario del detector. El juez tiene que contestar dentro de él
// o su respuesta no es comparable con la model-free, que es todo el propósito de la tabla.
var vocabularioSombra = []string{
	memory.RelSupersedes,
	memory.RelConflictsWith,
	memory.RelRelated,
	memory.RelCompatible,
	memory.RelScoped,
	memory.RelNotConflict,
}

const promptSistemaSombra = `Sos un juez de relaciones entre dos notas de una memoria técnica.
Leé las dos notas y respondé con UNA SOLA PALABRA, exactamente una de estas:

supersedes      — la nota A reemplaza a la B: dicen lo mismo y la A está más al día
conflicts_with  — se contradicen: no pueden ser ambas verdad
related          — hablan del mismo tema sin contradecirse ni reemplazarse
compatible      — conviven sin problema, aportan cosas distintas
scoped           — una es el caso general y la otra un caso particular
not_conflict     — no tienen relación relevante

No expliques. No agregues puntuación. Una palabra.`

// trabajoSombra es un par a medir. Lleva la relación ENTERA —con sus señales ya calculadas— y no
// sólo un id, porque un re-juicio posterior puede cambiarlas y la evidencia tiene que describir el
// momento de la decisión. Los CONTENIDOS, en cambio, los lee el worker: son dos consultas más y no
// tienen por qué pagarlas el guardado.
type trabajoSombra struct {
	rel memory.ObsRelation
}

// shadowWorker consume la cola y escribe evidencia. Un solo consumidor a propósito: la sombra no
// tiene que competir por el motor con las tools que el agente sí está esperando.
type shadowWorker struct {
	cola     chan trabajoSombra
	engine   memory.StorageBackend
	motor    cognition.Provider
	timeout  time.Duration
	mu       sync.Mutex
	tirados  int
	medidos  int
	fallidos int
}

// timeoutSombra acota cada consulta al motor. Generoso porque nadie espera el resultado, pero
// finito porque una consulta colgada bloquearía al único consumidor y la cola se llenaría de
// trabajo que después se tira.
const timeoutSombra = 60 * time.Second

func newShadowWorker(engine memory.StorageBackend, motor cognition.Provider, tope int) *shadowWorker {
	if tope <= 0 {
		tope = 64
	}
	return &shadowWorker{
		cola:    make(chan trabajoSombra, tope),
		engine:  engine,
		motor:   motor,
		timeout: timeoutSombra,
	}
}

// encolar deja el trabajo si hay lugar y devuelve si entró. NUNCA bloquea: el default del select
// es lo que garantiza que el guardado no espere por una medición.
func (w *shadowWorker) encolar(t trabajoSombra) bool {
	select {
	case w.cola <- t:
		return true
	default:
		w.mu.Lock()
		w.tirados++
		n := w.tirados
		w.mu.Unlock()
		// Se avisa cada 10 para no convertir un pico en una tormenta de logs, pero se avisa: una
		// sombra que descarta en silencio produce una muestra sesgada hacia los momentos de calma
		// y nadie lo notaría al leer la tabla.
		if n%10 == 1 {
			logx.Warn("cola del modo sombra llena: veredicto descartado", "descartados_totales", n)
		}
		return false
	}
}

// encolarSombra manda a medir las relaciones que el detector acaba de emitir. Es un no-op cuando
// la sombra está apagada (s.shadow == nil), así el camino de guardado se escribe una sola vez y no
// tiene una rama «si está midiendo».
//
// Se encolan TODAS las relaciones, no sólo las de señal. Filtrar acá parece un ahorro y es un
// sesgo: sin los `related` no hay con qué comparar la tasa de acuerdo de los `supersedes`, y la
// pregunta que motiva todo esto —«¿el umbral está bien puesto?»— se responde justamente mirando
// qué pasa a los dos lados de la raya.
func (s *McpServer) encolarSombra(rels []memory.ObsRelation) {
	if s.shadow == nil {
		return
	}
	for _, r := range rels {
		s.shadow.encolar(trabajoSombra{rel: r})
	}
}

// RunShadowWorker corre el bucle del modo sombra hasta que se cancela el contexto. Es un no-op si
// la sombra está apagada, para que el arranque del daemon no tenga que preguntarlo.
func (s *McpServer) RunShadowWorker(ctx context.Context) {
	if s.shadow == nil {
		return
	}
	logx.Info("modo sombra encendido: cada veredicto del detector se compara contra el motor y la lectura del motor se descarta",
		"motor", s.cognition.Name(), "cola", cap(s.shadow.cola))
	s.shadow.Run(ctx)
}

// Run procesa la cola hasta que se cancela el contexto.
func (w *shadowWorker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			w.mu.Lock()
			logx.Info("modo sombra detenido", "medidos", w.medidos, "fallidos", w.fallidos, "descartados", w.tirados)
			w.mu.Unlock()
			return
		case t := <-w.cola:
			w.procesar(ctx, t)
		}
	}
}

func (w *shadowWorker) procesar(ctx context.Context, t trabajoSombra) {
	c, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	textoSrc, textoTgt, err := w.engine.ShadowPairTexts(t.rel.SourceID, t.rel.TargetID)
	if err != nil {
		w.mu.Lock()
		w.fallidos++
		w.mu.Unlock()
		logx.Warn("no se pudieron leer los textos de un par en modo sombra", "relacion", t.rel.ID, "error", err)
		return
	}

	user := "NOTA A:\n" + textoSrc + "\n\nNOTA B:\n" + textoTgt
	crudo, err := w.motor.Ask(c, promptSistemaSombra, user)
	if err != nil {
		w.mu.Lock()
		w.fallidos++
		w.mu.Unlock()
		// Un fallo del motor NO se guarda como veredicto: una fila con judge_relation vacío
		// contaría como desacuerdo y ensuciaría justo la tasa que la tabla existe para medir.
		logx.Warn("el motor no pudo juzgar un par en modo sombra", "relacion", t.rel.ID, "error", err)
		return
	}

	veredicto := normalizarVeredicto(crudo)
	if veredicto == "" {
		w.mu.Lock()
		w.fallidos++
		w.mu.Unlock()
		logx.Warn("respuesta del motor fuera del vocabulario en modo sombra", "relacion", t.rel.ID, "respuesta", recorte(crudo, 80))
		return
	}

	if err := w.engine.SaveShadowVerdict(memory.ShadowVerdict{
		RelationID:    t.rel.ID,
		SourceID:      t.rel.SourceID,
		TargetID:      t.rel.TargetID,
		HeurRelation:  t.rel.Relation,
		HeurStatus:    t.rel.Status,
		Lex:           t.rel.Lex,
		Cosine:        t.rel.Cosine,
		JudgeRelation: veredicto,
		JudgeRaw:      crudo,
		JudgeModel:    w.motor.Name(),
	}); err != nil {
		logx.Warn("no se pudo registrar un veredicto sombra", "relacion", t.rel.ID, "error", err)
		return
	}
	w.mu.Lock()
	w.medidos++
	w.mu.Unlock()
}

// normalizarVeredicto mapea la respuesta del motor al vocabulario del detector, o "" si no cae en
// ninguno. Se compara por CONTENCIÓN y no por igualdad porque los motores agregan puntuación o una
// coletilla pese al prompt; pero el orden importa: `conflicts_with` se prueba antes que `related`
// para que una respuesta larga no se resuelva por la primera coincidencia casual.
func normalizarVeredicto(s string) string {
	limpio := strings.ToLower(strings.TrimSpace(s))
	if limpio == "" {
		return ""
	}
	for _, v := range vocabularioSombra {
		if limpio == v {
			return v
		}
	}
	// Segunda pasada, tolerante: sólo si UNA de las palabras aparece. Si aparecen dos, el motor no
	// eligió y tratarlo como elección sería inventar la etiqueta.
	var hallado string
	for _, v := range vocabularioSombra {
		if strings.Contains(limpio, v) {
			if hallado != "" {
				return ""
			}
			hallado = v
		}
	}
	return hallado
}

func recorte(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
