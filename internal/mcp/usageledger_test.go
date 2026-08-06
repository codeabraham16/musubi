package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// Invariantes del LEDGER DE USO (specs/ledger-de-uso). Lo que se prueba es que TODA invocación
// quede registrada, que NUNCA se guarden argumentos, y que un ledger roto no pueda tumbar una tool.

// sinkEspia captura los lotes en memoria en vez de escribir a la base.
type sinkEspia struct {
	mu     sync.Mutex
	vistas []memory.ToolInvocation
	falla  error
}

func (s *sinkEspia) RecordToolInvocations(_ context.Context, batch []memory.ToolInvocation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.falla != nil {
		return s.falla
	}
	s.vistas = append(s.vistas, batch...)
	return nil
}

func (s *sinkEspia) todas() []memory.ToolInvocation {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]memory.ToolInvocation, len(s.vistas))
	copy(out, s.vistas)
	return out
}

// serverConLedger arma un server con el ledger encendido contra un sink espía.
func serverConLedger(t *testing.T, sink ledgerSink) *McpServer {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	s.ledger = newUsageLedger(sink, time.Hour) // intervalo largo: los tests hacen flush a mano
	s.ledgerRetentionDays = 90
	t.Cleanup(func() { s.ledger.close() })
	return s
}

func llamar(t *testing.T, s *McpServer, tool, args string) *RpcError {
	t.Helper()
	raw, err := json.Marshal(map[string]interface{}{
		"name":      tool,
		"arguments": json.RawMessage(args),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, rerr := s.handleToolsCall(context.Background(), raw)
	return rerr
}

// L0 — ninguna llamada escapa al ledger: ni la que sale bien ni la que falla.
func TestL0TodaLlamadaQuedaRegistrada(t *testing.T) {
	sink := &sinkEspia{}
	s := serverConLedger(t, sink)

	if rerr := llamar(t, s, "musubi_save_observation",
		`{"topic_key":"t/k","content":"el ledger registra toda invocacion de tool"}`); rerr != nil {
		t.Fatalf("save: %v", rerr)
	}
	// Una que FALLA: recall sin query obligatoria.
	if rerr := llamar(t, s, "musubi_recall", `{}`); rerr == nil {
		t.Fatal("el control no sirve: esperaba que recall sin query fallara")
	}
	s.ledger.flush()

	vistas := sink.todas()
	if len(vistas) != 2 {
		t.Fatalf("FUGA L0: esperaba 2 invocaciones registradas, obtuve %d: %+v", len(vistas), vistas)
	}
	porTool := map[string]string{}
	for _, v := range vistas {
		porTool[v.Tool] = v.Outcome
	}
	if porTool["musubi_save_observation"] != memory.OutcomeOK {
		t.Errorf("la llamada exitosa debía registrarse como ok, obtuve %q", porTool["musubi_save_observation"])
	}
	if porTool["musubi_recall"] != memory.OutcomeError {
		t.Errorf("FUGA L0: la llamada FALLIDA debía registrarse como error, obtuve %q", porTool["musubi_recall"])
	}
}

// L1 — el ledger nunca guarda argumentos ni contenido. Es el invariante de privacidad.
func TestL1ElLedgerNoGuardaArgumentosNiContenido(t *testing.T) {
	sink := &sinkEspia{}
	s := serverConLedger(t, sink)

	const secreto = "SECRETO-QUE-NO-DEBE-APARECER-EN-EL-LEDGER"
	if rerr := llamar(t, s, "musubi_save_observation",
		`{"topic_key":"t/k","content":"`+secreto+` mas texto de relleno para la observacion"}`); rerr != nil {
		t.Fatalf("save: %v", rerr)
	}
	s.ledger.flush()

	vistas := sink.todas()
	if len(vistas) == 0 {
		t.Fatal("el control no sirve: no se registró nada, así que no se está probando la privacidad")
	}
	// Serializar la invocación entera y buscar el secreto en CUALQUIER campo: si mañana alguien
	// agrega una columna con los argumentos, este test lo caza sin que haya que actualizarlo.
	crudo, err := json.Marshal(vistas)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(crudo), secreto) {
		t.Errorf("FUGA L1: el contenido de la observación llegó al ledger.\nregistrado=%s", crudo)
	}
	if strings.Contains(string(crudo), "topic_key") {
		t.Errorf("FUGA L1: los argumentos crudos llegaron al ledger.\nregistrado=%s", crudo)
	}
}

// L2 — un ledger roto NO puede hacer fallar una llamada.
func TestL2UnLedgerRotoNoTumbaLaTool(t *testing.T) {
	sink := &sinkEspia{falla: errors.New("base caída simulada")}
	s := serverConLedger(t, sink)

	if rerr := llamar(t, s, "musubi_save_observation",
		`{"topic_key":"t/k","content":"guardar debe funcionar aunque el ledger este roto"}`); rerr != nil {
		t.Fatalf("FUGA L2: un fallo del ledger tumbó la tool: %v", rerr)
	}
	s.ledger.flush() // no debe entrar en pánico ni bloquear

	// Y la tool efectivamente hizo su trabajo.
	if rerr := llamar(t, s, "musubi_recall", `{"query":"guardar debe funcionar"}`); rerr != nil {
		t.Fatalf("recall tras el fallo del ledger: %v", rerr)
	}
}

// L4/L5 — el camino caliente no espera al disco: tras la llamada la invocación está en el buffer,
// y sólo baja al sink cuando se hace flush.
func TestL5ElCaminoCalienteNoEsperaAlDisco(t *testing.T) {
	sink := &sinkEspia{}
	s := serverConLedger(t, sink)

	if rerr := llamar(t, s, "musubi_recall", `{"query":"lo que sea"}`); rerr != nil {
		t.Fatalf("recall: %v", rerr)
	}
	if n := s.ledger.pendientes(); n != 1 {
		t.Fatalf("esperaba 1 invocación esperando flush, obtuve %d", n)
	}
	if len(sink.todas()) != 0 {
		t.Error("FUGA L5: la invocación llegó al sink SIN flush; el handler esperó al disco")
	}
	s.ledger.flush()
	if len(sink.todas()) != 1 {
		t.Error("tras el flush la invocación debía estar en el sink")
	}
	if n := s.ledger.pendientes(); n != 0 {
		t.Errorf("el buffer debía quedar vacío tras el flush, quedan %d", n)
	}
}

// L6 — el buffer tiene techo: pasado el tope se descartan las nuevas en vez de crecer sin límite.
func TestL6ElBufferTieneTecho(t *testing.T) {
	l := newUsageLedger(&sinkEspia{}, time.Hour)
	for i := 0; i < ledgerBufferCap+500; i++ {
		l.record(memory.ToolInvocation{Tool: "musubi_recall", Outcome: memory.OutcomeOK})
	}
	if n := l.pendientes(); n != ledgerBufferCap {
		t.Errorf("FUGA L6: el buffer creció hasta %d, esperaba el techo de %d", n, ledgerBufferCap)
	}
}

// Sin ledger configurado el servidor se comporta exactamente como antes de la fase.
func TestSinLedgerElServidorNoRegistraNada(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if s.ledger != nil {
		t.Fatal("el server de test no debería traer ledger por default")
	}
	if rerr := llamar(t, s, "musubi_recall", `{"query":"sin ledger"}`); rerr != nil {
		t.Fatalf("recall sin ledger: %v", rerr)
	}
}

// El default del config es ENCENDIDO: omitir el bloque no puede apagar el medidor.
func TestElLedgerNaceEncendido(t *testing.T) {
	var vacio config.UsageLedgerConfig
	if !vacio.EnabledOn() {
		t.Error("un bloque usage_ledger ausente debe dejar el ledger ENCENDIDO")
	}
	no := false
	if (config.UsageLedgerConfig{Enabled: &no}).EnabledOn() {
		t.Error("enabled:false explícito debe apagarlo")
	}
	if got := vacio.EffectiveFlushSeconds(); got != 10 {
		t.Errorf("flush default esperado 10s, obtuve %d", got)
	}
	if got := vacio.EffectiveRetentionDays(); got != 90 {
		t.Errorf("retención default esperada 90d, obtuve %d", got)
	}
	if got := (config.UsageLedgerConfig{RetentionDays: -1}).EffectiveRetentionDays(); got != -1 {
		t.Errorf("una retención negativa significa 'no purgar' y debe respetarse, obtuve %d", got)
	}
}
