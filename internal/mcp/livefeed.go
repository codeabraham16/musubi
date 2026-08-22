package mcp

// livefeed.go es el FEED EN VIVO: cada invocación de tool sale por un canal en el mismo instante
// en que termina, para que un panel pueda mostrar lo que está pasando MIENTRAS pasa.
//
// POR QUÉ NO SALE DE LA BASE. El ledger de uso (usageledger.go) ya guarda todas las invocaciones,
// así que la tentación es que el panel lea `tool_invocations` con un cursor. Medido sobre el
// ledger real, eso no puede ser "en vivo" por tres razones estructurales:
//
//   1. El buffer baja a disco cada 10 s por default. La demora no es de red ni de render: está
//      en el diseño del ledger, y bajarla a 1 s cambiaría el compromiso de fsync que ese diseño
//      eligió a propósito.
//   2. `created_at` se estampa con DEFAULT CURRENT_TIMESTAMP, o sea en el INSERT. Es la hora del
//      flush, no la de la llamada: medido en la base local, hasta 23 invocaciones comparten
//      timestamp. Un feed ordenado por esa columna dibuja escalones, no un río.
//   3. La resolución de esa columna es el segundo. Dos llamadas separadas por 40 ms son
//      simultáneas para la base.
//
// Publicar en proceso resuelve las tres de una: el evento sale con la hora real, con precisión de
// milisegundos y sin esperar al disco. El ledger sigue siendo la HISTORIA (sobrevive al reinicio,
// se consulta con SQL); esto es el PRESENTE. Son dos cosas distintas y conviene que no se mezclen.
//
// LO QUE NO VIAJA, y es el mismo invariante L1 del ledger: ni argumentos, ni resultados, ni
// mensajes de error. Un feed en vivo es la superficie MÁS fácil de dejar abierta por accidente, y
// un panel que muestre el contenido de `save_observation` sería una fuga con forma de gráfico. El
// struct no tiene dónde ponerlo.

import (
	"context"
	"sync"
	"time"
)

// liveRing es cuántos eventos recientes se guardan para que un panel que ABRE no arranque en
// blanco. 200 alcanza para ver el minuto anterior con tráfico normal, y son ~30 KB.
const liveRing = 200

// liveSubBuf es el buffer por suscriptor. Un navegador lento (o una pestaña en segundo plano, que
// el browser congela) no puede frenar el dispatch: si se le llena, se le DESCARTAN eventos y se le
// avisa. Preferir descartar antes que bloquear no es una optimización, es la única opción — el
// publisher corre en el camino caliente de toda tool.
const liveSubBuf = 256

// LiveEvent es una invocación tal como sale al vivo. Campos JSON cortos: van uno por línea por
// SSE y a 20 eventos/s la diferencia entre `duration_ms` y `ms` es real.
type LiveEvent struct {
	Seq int64 `json:"seq"`
	// At es cuándo TERMINÓ la llamada, con milisegundos. Se estampa en proceso, no en el flush.
	At         string  `json:"at"`
	Tool       string  `json:"tool"`
	Outcome    string  `json:"outcome"`
	DurationMs float64 `json:"ms"`
	// Principal y Project vienen de la credencial, nunca del cliente. En stdio local quedan
	// vacíos porque no hay token: ahí no hay a quién distinguir.
	Principal string `json:"principal,omitempty"`
	Project   string `json:"project,omitempty"`
	// Kind separa TRABAJO de SONDEO. Ver clasificarTool.
	Kind string `json:"kind"`
	// Perdidos, si es > 0, dice cuántos eventos se le descartaron a ESTE suscriptor justo antes
	// de éste. Va en el evento y no en un log del servidor porque el que necesita saber que la
	// vista tiene un hueco es el que la está mirando.
	Perdidos int64 `json:"perdidos,omitempty"`
}

// Los dos géneros de evento. Es una taxonomía cerrada de dos valores a propósito: apenas admita
// un tercero, el panel necesita decidir qué hacer con él y la clasificación deja de ser útil.
const (
	KindTrabajo = "trabajo"
	KindSondeo  = "sondeo"
)

// toolsDeSondeo son las tools que un poller llama solo para saber cómo está el sistema. No son
// trabajo de nadie: son el latido de los paneles, los agentes y el sync.
//
// EXISTE PORQUE EL SONDEO ES CASI TODO. Medido sobre 24 h reales: en la base local, 97.815 de
// 97.889 invocaciones (99,92%) fueron tres de estas tools; en el cerebro central, 13.919 de 18.363
// fueron `musubi_sync_pull` sola. Un feed que las muestre crudas es una pared de ruido donde el
// `save_observation` que de verdad importa pasa sin que nadie lo vea.
//
// LA LISTA ES DE SONDEO, NO DE TRABAJO, y el default es TRABAJO. Al revés —lista de trabajo,
// default sondeo— una tool nueva nacería INVISIBLE, que es la peor falla posible para un panel
// cuyo trabajo es mostrar lo que pasa. Así, lo peor que hace una tool nueva es aparecer de más.
var toolsDeSondeo = map[string]bool{
	"musubi_sync_pull":       true,
	"musubi_sync_status":     true,
	"musubi_work":            true,
	"musubi_phase":           true,
	"musubi_list_skills":     true,
	"musubi_search_skills":   true,
	"musubi_doctor":          true,
	"musubi_readiness":       true,
	"musubi_conflicts":       true,
	"musubi_insights":        true,
	"musubi_whoami":          true,
	"musubi_tokens":          true,
	"musubi_token_list":      true,
	"musubi_cognition_stats": true,
	"musubi_skill_usage":     true,
	"musubi_tool_usage":      true,
}

// clasificarTool decide si una invocación es trabajo o latido.
func clasificarTool(tool string) string {
	if toolsDeSondeo[tool] {
		return KindSondeo
	}
	return KindTrabajo
}

// liveSub es un suscriptor: su canal y cuántos eventos se le tuvieron que tirar.
type liveSub struct {
	ch        chan LiveEvent
	perdidos  int64
	proyecto  string // "" ⇒ ve todo (admin federado o sin auth)
	filtrando bool
}

// liveFeed reparte eventos a los paneles conectados y guarda los últimos para el que recién llega.
//
// Un solo mutex protege todo. Es lo correcto acá: la sección crítica de publish es un for sobre un
// puñado de suscriptores con envíos NO BLOQUEANTES, así que no puede quedarse tomada esperando a
// nadie. Un RWMutex no compraría nada porque publish —el camino caliente— igual escribe el ring.
type liveFeed struct {
	mu    sync.Mutex
	subs  map[int64]*liveSub
	next  int64 // id del próximo suscriptor
	seq   int64 // número de secuencia global de eventos
	ring  []LiveEvent
	desde int // índice del más viejo en el ring circular
	llenó bool
}

func newLiveFeed() *liveFeed {
	return &liveFeed{
		subs: make(map[int64]*liveSub),
		ring: make([]LiveEvent, liveRing),
	}
}

// publish emite un evento. Corre en el camino caliente de toda tool, así que NUNCA bloquea: los
// envíos son no bloqueantes y lo que no entra se descarta contándolo.
func (f *liveFeed) publish(ev LiveEvent) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	f.seq++
	ev.Seq = f.seq
	f.ring[f.desde] = ev
	f.desde = (f.desde + 1) % len(f.ring)
	if f.desde == 0 {
		f.llenó = true
	}

	for _, s := range f.subs {
		if s.filtrando && s.proyecto != ev.Project {
			continue
		}
		e := ev
		e.Perdidos = s.perdidos
		select {
		case s.ch <- e:
			s.perdidos = 0
		default:
			// Buffer lleno: este suscriptor no lee al ritmo al que se publica. Se cuenta y el
			// próximo evento que SÍ entre lleva el aviso, para que el panel pueda decir que
			// tiene un hueco en vez de mostrar una historia incompleta como si fuera completa.
			s.perdidos++
		}
	}
}

// subscribe registra un panel y le devuelve el backlog reciente más su canal.
//
// El backlog se arma DENTRO del mismo lock que registra el suscriptor. Si se hicieran en dos
// pasos, un evento publicado entre medio se perdería (registro después del backlog) o llegaría
// dos veces (registro antes). Los dos pasos juntos hacen que la costura no exista.
//
// `proyecto` acota lo que este suscriptor puede ver; "" con filtrar=false significa acceso
// federado. El filtro se aplica del lado del feed y no del handler: si estuviera en el handler,
// cada endpoint nuevo tendría que acordarse de aplicarlo, y ese es exactamente el olvido que
// convierte un panel en una fuga entre proyectos.
func (f *liveFeed) subscribe(proyecto string, filtrar bool) (int64, <-chan LiveEvent, []LiveEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()

	id := f.next
	f.next++
	s := &liveSub{ch: make(chan LiveEvent, liveSubBuf), proyecto: proyecto, filtrando: filtrar}
	f.subs[id] = s

	var back []LiveEvent
	n := len(f.ring)
	inicio, cant := 0, f.desde
	if f.llenó {
		inicio, cant = f.desde, n
	}
	for i := 0; i < cant; i++ {
		ev := f.ring[(inicio+i)%n]
		if filtrar && ev.Project != proyecto {
			continue
		}
		back = append(back, ev)
	}
	return id, s.ch, back
}

// unsubscribe saca un panel. Cierra su canal para que un lector bloqueado salga.
func (f *liveFeed) unsubscribe(id int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if s, ok := f.subs[id]; ok {
		delete(f.subs, id)
		close(s.ch)
	}
}

// suscriptores dice cuántos paneles hay mirando. Existe para poder probar que unsubscribe DE
// VERDAD saca al suscriptor: si no lo sacara, cada pestaña cerrada dejaría un canal muerto en el
// mapa y publish —que corre en la salida de toda tool— iteraría para siempre sobre una lista que
// sólo crece. Es una degradación lenta, del tipo que no se nota hasta que ya duele.
func (f *liveFeed) suscriptores() int {
	if f == nil {
		return 0
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.subs)
}

// publicarUso arma el LiveEvent de una invocación y lo emite. Lo llama registrarUso, que es el
// punto único por el que pasan los tres caminos de salida del dispatch.
func (s *McpServer) publicarUso(ctx context.Context, tool, outcome string, d time.Duration, fin time.Time) {
	if s.live == nil {
		return
	}
	ev := LiveEvent{
		At:         fin.Format("2006-01-02T15:04:05.000Z07:00"),
		Tool:       tool,
		Outcome:    outcome,
		DurationMs: float64(d.Microseconds()) / 1000,
		Kind:       clasificarTool(tool),
	}
	if p := principalFrom(ctx); p != nil {
		ev.Principal = p.Name
		ev.Project = p.ProjectID
	}
	s.live.publish(ev)
}
