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
)

// ledgerSink es lo que el ledger necesita del motor. Interfaz y no *DbEngine para que los tests
// puedan inyectar un sink que falla y verificar que un fallo del ledger NO tumba una tool (L2).
type ledgerSink interface {
	RecordToolInvocations(ctx context.Context, batch []memory.ToolInvocation) error
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
		sink:     sink,
		interval: interval,
		buf:      make([]memory.ToolInvocation, 0, 64),
		stop:     make(chan struct{}),
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
	if len(l.buf) == 0 {
		desc := l.descartes
		l.descartes = 0
		l.mu.Unlock()
		if desc > 0 {
			logx.Warn("ledger de uso: invocaciones descartadas por buffer lleno", "descartadas", desc)
		}
		return
	}
	lote := l.buf
	l.buf = make([]memory.ToolInvocation, 0, 64)
	desc := l.descartes
	l.descartes = 0
	l.mu.Unlock()

	if desc > 0 {
		logx.Warn("ledger de uso: invocaciones descartadas por buffer lleno", "descartadas", desc)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := l.sink.RecordToolInvocations(ctx, lote); err != nil {
		logx.Warn("ledger de uso: no se pudo persistir el lote; se descarta", "invocaciones", len(lote), "error", err)
	}
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
