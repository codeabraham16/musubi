package mcp

import (
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"musubi/internal/memory"
)

// EL RITMO DE LA BAJADA: una máquina quieta deja de preguntarle al central cada 30 s, y una en la
// que alguien trabaja vuelve a preguntar cada 30 s.
//
// Hasta acá el dueño del candado pedía una página en cada tick aunque nadie trabajara y el central
// no tuviera nada: medido en el ledger del central el 2026-09-26, 6.207 musubi_sync_pull en un día
// —el 92,8 % de todas sus invocaciones—, casi todos páginas vacías (~135 B cada una, ver
// SyncClient.Pull). Ahora, después de k bajadas VACÍAS seguidas, el dueño saltea 2^k − 1 ticks, con
// un tope (sync.inbound_idle_max_seconds, 300 s por defecto): 60 s, 120 s, 240 s y 300 s de ahí en
// más. Una bajada con filas, o una marca de actividad nueva (memory.MetaDespertarBajada: un turno en
// cualquier terminal de esta base, o un daemon que arranca), lo devuelve al tick base.
//
// TODO VA EN TICKS, NO EN INSTANTES. El scheduler ya tiene su reloj —el Ticker—. El primer diseño
// comparaba `time.Now()` contra una próxima calculada con jitter, y la revisión del plan encontró que
// esa próxima caía siempre un pelo después del tick base, que entonces se salteaba: con actividad
// quedaba en 60 s y no en 30. Contando ticks, el ritmo base es exactamente el tick, y las pruebas lo
// recorren llamando al drain N veces, sin dormir.
//
// Vive en la memoria del proceso a propósito: sólo lo usa el dueño del candado, que es el único que
// baja. Lo que un agente ve desde OTRO daemon es la próxima que el dueño anota en la meta
// (anotarUltimaBajada), y no este estado.
type ritmoBajada struct {
	mu sync.Mutex
	// tope es el espaciado máximo entre dos bajadas vacías; negativo, o menor que un tick, deja el
	// ritmo fijo: un pedido por tick, la conducta de antes.
	tope time.Duration
	// vacias son las bajadas vacías seguidas; saltear, los ticks que faltan saltear antes de la
	// próxima.
	vacias, saltear int
	// visto es el último valor de memory.MetaDespertarBajada que leyó este proceso.
	visto string
	// azar suma, desde la segunda vacía, un tick al azar (0 o 1): la fase del Ticker ya difiere entre
	// máquinas porque arrancan en momentos distintos, y esto termina de desparejarlas. Las pruebas lo
	// fijan.
	azar func(n int) int
}

// nuevoRitmoBajada arma el ritmo con el tope de la config (ver config.SyncConfig.InboundIdleMaxSeconds).
func nuevoRitmoBajada(tope time.Duration) *ritmoBajada {
	return &ritmoBajada{tope: tope, azar: rand.Intn}
}

// tocaIr dice si ESTE tick sale a la red. Con `despierto` —hubo actividad desde la última vez que
// se miró— vuelve al ritmo base y sale ya: reinicia las vacías también, porque si sólo cortara la
// espera del momento, la primera bajada vacía después del turno lo devolvería directo al tope, y
// «vuelve a los 30 s en cuanto alguien trabaja» sería un solo pedido. Un ritmo nil —un servidor sin
// SetSyncClient— sale siempre, como antes.
func (r *ritmoBajada) tocaIr(despierto bool) bool {
	if r == nil {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if despierto {
		r.vacias, r.saltear = 0, 0 // el despertar reinicia el espaciado, no sólo la espera
		return true
	}
	if r.saltear > 0 {
		r.saltear--
		return false
	}
	return true
}

// anotar registra cómo volvió una bajada que SALIÓ A LA RED y devuelve en cuántos ticks sale la
// próxima (1 = el tick siguiente). La llama sólo anotarUltimaBajada, desde el defer de
// drainInboundOnce, así que un Pull con error no anota y un tick salteado tampoco.
//
// VACÍA ES conFilas EN CERO —las páginas del viaje menos las vacías—, NO «cero ingeridas». Una página
// que trae filas y no puede ingerir ninguna (una fila que esta base rechaza, un SQLITE_BUSY) deja el
// cursor quieto y vuelve en el tick siguiente: con «ingeridas» se espaciaría justo la bajada atascada.
func (r *ritmoBajada) anotar(conFilas int64, tick time.Duration) int {
	if r == nil {
		return 1
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if conFilas > 0 {
		r.vacias, r.saltear = 0, 0
		return 1
	}
	r.vacias++
	n := 1<<min(r.vacias, 16) - 1
	if r.vacias >= 2 {
		n += r.azar(2) // sin azar en el ritmo base ni en el primer escalón
	}
	// El tope en ticks. Un tick en cero no llega nunca (tickBajada cae a 30 s), pero dividir por él
	// tumbaría el daemon: se trata como ritmo fijo.
	topeTicks := 0
	if tick > 0 {
		topeTicks = int(r.tope / tick)
	}
	saltear := min(n, topeTicks-1) // nunca más de topeTicks entre dos pedidos
	r.saltear = max(0, saltear)    // un tope negativo o menor que un tick deja -1: ritmo fijo
	return 1 + r.saltear
}

// cedido anota que el candado es de otro proceso, y deja el ritmo en la base: cuando este proceso lo
// vuelva a tomar —el dueño cerró, o se cayó y el lease venció—, sale a la red en ese mismo tick en
// vez de seguir salteando los ticks que le quedaban de cuando era dueño, congelados mientras no lo
// fue. Sin esto, una máquina quieta podía pasar hasta el tope ENTERO sin bajar después de un cambio
// de dueño, además de lo que ya había esperado con el anterior: más de 5 min entre dos pedidos.
func (r *ritmoBajada) cedido() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.vacias, r.saltear = 0, 0
}

// marcaNueva dice si la marca de actividad cambió desde la última vez que este proceso la leyó, y la
// da por vista. La primera lectura de un proceso cuenta como nueva si hay marca: un dueño recién
// llegado baja una vez, que es lo que tiene que hacer.
func (r *ritmoBajada) marcaNueva(v string) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if v == r.visto {
		return false
	}
	r.visto = v
	return true
}

// hayActividadLocal dice si alguien trabajó sobre esta base desde la última vez que ESTE proceso miró:
// un turno en cualquier terminal (el hook del turno escribe memory.MetaDespertarBajada con todo prompt
// no vacío, «sigue» y los avisos del sistema incluidos) o un daemon que arrancó. Cuesta una lectura
// de la meta por tick, y la hace sólo el dueño del candado.
//
// Un error de lectura cuenta como SIN actividad: el peor caso es esperar hasta el tope, que es el
// mismo que sin marca, y despertar ante cada error haría que una base con problemas bajara al ritmo
// base para siempre.
//
// El ledger local de invocaciones (tool_invocations) NO se mira, a propósito: los subagentes que
// terminan ya llegan al hook como <task-notification>, la señal es escasa y llega con buffer, y una
// tool desconocida cuenta como trabajo (toolsDeSondeo), así que un sondeo nuevo mantendría la bajada
// despierta en silencio. El tope acota lo que queda afuera: un turno largo sin prompts.
func (s *McpServer) hayActividadLocal() bool {
	raw, ok, err := s.engine.GetMeta(memory.MetaDespertarBajada)
	if err != nil || !ok {
		return false
	}
	return s.ritmoBajada.marcaNueva(strings.TrimSpace(raw))
}

// despertarAnotado es el instante (unix) de la última marca de actividad de esta base, o 0 si no hay,
// no se puede leer o no es un número. Lo usa la línea «bajada» de musubi_sync_status para no
// prometer una próxima que un turno ya adelantó (ver describirUltimaBajada). Es de lectura: no la da
// por vista, eso lo hace sólo el dueño del candado al decidir si baja.
func (s *McpServer) despertarAnotado() int64 {
	raw, ok, err := s.engine.GetMeta(memory.MetaDespertarBajada)
	if err != nil || !ok {
		return 0
	}
	n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
