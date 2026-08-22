package mcp

// usageledger.go es el lado MCP del LEDGER DE USO (F0 · track «Potencia medida»): amortigua las
// invocaciones en memoria y las baja a la base desde una goroutine aparte.
//
// POR QUÉ UN BUFFER Y NO UN INSERT DIRECTO. El handler corre CON dispatchMu tomado (write-lock
// para las tools que mutan, read-lock para las de lectura). Escribir a disco ahí adentro alargaría
// el lock en el camino caliente de toda tool. Y la goroutine de flush NO puede tomar dispatchMu:
// es la misma trampa que documenta maybeTriggerMaintenance en scheduler.go — el handler todavía lo
// tiene y re-entrarlo es deadlock. Por eso el flush va DIRECTO contra la base, y la concurrencia
// con las escrituras de las tools la resuelve SQLite con busy_timeout(5000) + WAL del DSN.

import (
	"context"
	"sync"
	"time"

	"musubi/internal/logx"
	"musubi/internal/memory"
	"musubi/internal/skills"
)

// ledgerSink es lo que el ledger necesita del motor. Interfaz y no *DbEngine para que los tests
// puedan inyectar un sink que falla y verificar que un fallo del ledger NO tumba una tool (L2).
type ledgerSink interface {
	RecordToolInvocations(ctx context.Context, batch []memory.ToolInvocation) error
	// RecordSkillEvents baja los contadores del arsenal. Va en la MISMA interfaz y con el mismo
	// buffer, no en una cañería aparte: comparte el ticker, el techo y la regla de no reencolar,
	// y sobre todo comparte la razón de fondo —no escribir a disco con dispatchMu tomado—. Dos
	// implementaciones con las mismas propiedades se desincronizan el día que se toque una.
	RecordSkillEvents(ctx context.Context, batch []memory.SkillEvent) error
}

// ledgerBufferCap es el techo del buffer en memoria. Si el flush no da abasto (base bloqueada,
// disco lento), se DESCARTAN las invocaciones nuevas en vez de crecer sin límite: la telemetría
// jamás puede ser el motivo por el que el daemon se quede sin memoria. Lo descartado se cuenta y
// se logea, porque un ledger que pierde datos en silencio es peor que no tenerlo.
const ledgerBufferCap = 4096

// usageLedger acumula invocaciones y las baja por lote.
type usageLedger struct {
	mu        sync.Mutex
	buf       []memory.ToolInvocation
	bufSkills []memory.SkillEvent
	descartes int64

	sink     ledgerSink
	interval time.Duration
	stop     chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

func newUsageLedger(sink ledgerSink, interval time.Duration) *usageLedger {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	return &usageLedger{
		sink:      sink,
		interval:  interval,
		buf:       make([]memory.ToolInvocation, 0, 64),
		bufSkills: make([]memory.SkillEvent, 0, 64),
		stop:      make(chan struct{}),
	}
}

// record encola una invocación. Es lo ÚNICO que corre en el camino caliente: un append bajo mutex,
// sin disco y sin red. Nunca devuelve error — el ledger no puede hacer fallar una llamada (L2).
func (l *usageLedger) record(inv memory.ToolInvocation) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.buf) >= ledgerBufferCap {
		l.descartes++
		return
	}
	l.buf = append(l.buf, inv)
}

// recordSkills encola conteos del arsenal. Mismo contrato que record: un append bajo mutex, sin
// disco y sin red, y nunca falla. Recibe el lote entero porque una resolución produce un conteo por
// skill activa y tomar el mutex diez veces seguidas no aporta nada.
func (l *usageLedger) recordSkills(evs []memory.SkillEvent) {
	if l == nil || len(evs) == 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, ev := range evs {
		if len(l.bufSkills) >= ledgerBufferCap {
			l.descartes++
			return
		}
		l.bufSkills = append(l.bufSkills, ev)
	}
}

// start arranca la goroutine de flush periódico.
func (l *usageLedger) start() {
	if l == nil || l.sink == nil {
		return
	}
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		t := time.NewTicker(l.interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				l.flush()
			case <-l.stop:
				l.flush() // último intento antes de morir; lo que no entre se pierde (L7)
				return
			}
		}
	}()
}

// close detiene el flush y baja lo que quede pendiente. Idempotente.
func (l *usageLedger) close() {
	if l == nil {
		return
	}
	l.stopOnce.Do(func() { close(l.stop) })
	l.wg.Wait()
}

// flush baja el buffer a la base. Ante error NO reencola: reintentar indefinidamente convertiría
// una base momentáneamente trabada en un buffer que crece hasta el techo y después descarta igual,
// pero habiendo gastado memoria y reintentos. Se logea y se sigue.
func (l *usageLedger) flush() {
	if l == nil || l.sink == nil {
		return
	}
	l.mu.Lock()
	lote := l.buf
	loteSkills := l.bufSkills
	desc := l.descartes
	l.descartes = 0
	if len(lote) > 0 {
		l.buf = make([]memory.ToolInvocation, 0, 64)
	}
	if len(loteSkills) > 0 {
		l.bufSkills = make([]memory.SkillEvent, 0, 64)
	}
	l.mu.Unlock()

	if desc > 0 {
		logx.Warn("ledger de uso: invocaciones descartadas por buffer lleno", "descartadas", desc)
	}
	if len(lote) == 0 && len(loteSkills) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if len(lote) > 0 {
		if err := l.sink.RecordToolInvocations(ctx, lote); err != nil {
			logx.Warn("ledger de uso: no se pudo persistir el lote; se descarta", "invocaciones", len(lote), "error", err)
		}
	}
	// Los dos lotes se bajan por separado a propósito: que fallen los contadores del arsenal no
	// puede costar las invocaciones de tools que ya estaban listas al lado, ni al revés.
	if len(loteSkills) > 0 {
		if err := l.sink.RecordSkillEvents(ctx, loteSkills); err != nil {
			logx.Warn("contadores de skills: no se pudo persistir el lote; se descarta", "conteos", len(loteSkills), "error", err)
		}
	}
}

// pendientesSkills es para los tests: cuántos conteos del arsenal esperan flush. Es lo que prueba
// que el camino caliente NO tocó el disco — si el conteo estuviera en la base, acá habría cero.
func (l *usageLedger) pendientesSkills() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.bufSkills)
}

// pendientes es para los tests: cuántas invocaciones esperan flush.
func (l *usageLedger) pendientes() int {
	if l == nil {
		return 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.buf)
}

// registrarUso es el punto único que llama el dispatch. Vive acá y no inline en handleToolsCall
// para que los tres caminos de salida —ok/error, rechazo por rol y rechazo por cuota— usen
// exactamente la misma forma de registrar y no se pueda olvidar un campo en uno de ellos.
func (s *McpServer) registrarUso(ctx context.Context, tool, outcome string, d time.Duration) {
	// El FEED EN VIVO va PRIMERO y fuera del guard del ledger: son dos consumidores distintos de
	// la misma señal —el presente y la historia— y apagar la historia no puede apagar el presente.
	// Con el orden al revés, `usage_ledger.enabled: false` dejaba el panel en vivo mudo sin que
	// nada lo dijera.
	s.publicarUso(ctx, tool, outcome, d, time.Now())

	if s.ledger == nil {
		return
	}
	p := principalFrom(ctx)
	inv := memory.ToolInvocation{
		Tool:     tool,
		Outcome:  outcome,
		Duration: d,
	}
	if p != nil {
		inv.Principal = p.Name
		inv.ProjectID = p.ProjectID
	}
	s.ledger.record(inv)
}

// proyectoDe devuelve el project_id de la credencial, o "" si no hay principal.
func proyectoDe(ctx context.Context) string {
	if p := principalFrom(ctx); p != nil {
		return p.ProjectID
	}
	return ""
}

// registrarActivaciones cuenta qué pasó en una resolución del arsenal.
//
// Cuenta DOS COSAS DISTINTAS y por eso son dos conteos: que la skill matcheó (con la evidencia que
// la dejó entrar) y que su cuerpo viajó. Si se contaran juntos no habría forma de escribir la
// lectura que más importa —«ocupa contexto en cada resolución y nadie la abrió»—, que es
// exactamente la diferencia entre las dos.
func (s *McpServer) registrarActivaciones(ctx context.Context, res []skills.SkillResuelta) {
	if s.ledger == nil || len(res) == 0 {
		return
	}
	proyecto := proyectoDe(ctx)
	evs := make([]memory.SkillEvent, 0, len(res)+2)
	for _, r := range res {
		evs = append(evs, memory.SkillEvent{
			Skill: r.Name, ProjectID: proyecto, Evidence: string(r.Matcheo), Kind: memory.UsoResuelta,
		})
		if r.ConCuerpo {
			evs = append(evs, memory.SkillEvent{
				Skill: r.Name, ProjectID: proyecto, Kind: memory.UsoCuerpoEnviado,
			})
		}
	}
	s.ledger.recordSkills(evs)
}

// registrarPedidosDeCuerpo cuenta que alguien pidió estas skills POR NOMBRE.
//
// Es la señal que los niveles hicieron observable: mientras la resolución entregaba todos los
// cuerpos no había ninguna decisión que mirar. Ahora el llamador ve el nivel 1 y elige gastar los
// tokens. Es lo más cerca de «sirvió» que se llega sin juicio — y por eso se guarda con el nombre
// de lo que es, un pedido, y no con el de lo que se le parece.
func (s *McpServer) registrarPedidosDeCuerpo(ctx context.Context, nombres []string) {
	if s.ledger == nil || len(nombres) == 0 {
		return
	}
	proyecto := proyectoDe(ctx)
	evs := make([]memory.SkillEvent, 0, len(nombres))
	for _, n := range nombres {
		evs = append(evs, memory.SkillEvent{Skill: n, ProjectID: proyecto, Kind: memory.UsoCuerpoPedido})
	}
	s.ledger.recordSkills(evs)
}
