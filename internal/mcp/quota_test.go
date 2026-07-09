package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// El limiter cuenta por ventana deslizante: permite hasta max, rechaza el excedente, y libera
// al salir de la ventana.
func TestQuotaLimiterWindow(t *testing.T) {
	q := newQuotaLimiter(2, time.Minute)
	base := time.Unix(1_000, 0)
	// Dos llamadas separadas a propósito (allow muta la ventana; no es una expresión repetida).
	if !q.allow("p", base) {
		t.Fatal("la 1ra llamada debería entrar en la cuota")
	}
	if !q.allow("p", base) {
		t.Fatal("la 2da llamada debería entrar en la cuota")
	}
	if q.allow("p", base) {
		t.Fatal("la 3ra llamada en la ventana debería rechazarse (cuota llena)")
	}
	// Al pasar la ventana, los timestamps viejos se podan y vuelve a permitir.
	if !q.allow("p", base.Add(61*time.Second)) {
		t.Fatal("tras la ventana la cuota debería liberarse")
	}
}

// Cuota desactivada (max<=0, receptor nil o key vacía) siempre permite; distintas keys tienen
// cuotas independientes.
func TestQuotaLimiterDisabledAndPerKey(t *testing.T) {
	now := time.Unix(2_000, 0)

	var nilQ *quotaLimiter
	if !nilQ.allow("p", now) {
		t.Error("un limiter nil debería permitir siempre")
	}
	off := newQuotaLimiter(0, time.Minute)
	for i := 0; i < 5; i++ {
		if !off.allow("p", now) {
			t.Error("max<=0 debería permitir siempre")
		}
	}
	q := newQuotaLimiter(1, time.Minute)
	// Key vacía (sin principal identificable): dos llamadas, ninguna debería contar.
	if !q.allow("", now) {
		t.Error("key vacía no debería tener cuota (1)")
	}
	if !q.allow("", now) {
		t.Error("key vacía no debería tener cuota (2)")
	}
	// Keys distintas no comparten cuota.
	if !q.allow("a", now) {
		t.Error("principal 'a' debería entrar")
	}
	if !q.allow("b", now) {
		t.Error("principal 'b' debería entrar (cuota independiente de 'a')")
	}
	if q.allow("a", now) {
		t.Error("la 2da llamada de 'a' debería rechazarse (max=1)")
	}
}

// La cuota se aplica POR PRINCIPAL en el dispatch: superarla devuelve codeQuotaExceeded, tras
// autorizar (la credencial es válida) y ANTES de ejecutar el handler.
func TestQuotaEnforcedPerPrincipal(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithQuota(2))

	ctx := withPrincipal(context.Background(), &Principal{Name: "p1", Role: RoleWriter})
	params := json.RawMessage(`{"name":"musubi_save_observation","arguments":{"topic_key":"q/t","content":"x"}}`)

	for i := 0; i < 2; i++ {
		if _, rpcErr := s.handleToolsCall(ctx, params); rpcErr != nil {
			t.Fatalf("llamada %d dentro de la cuota falló: %v", i+1, rpcErr)
		}
	}
	_, rpcErr := s.handleToolsCall(ctx, params)
	if rpcErr == nil || rpcErr.Code != codeQuotaExceeded {
		t.Fatalf("la 3ra llamada debería exceder la cuota (code %d), got %+v", codeQuotaExceeded, rpcErr)
	}

	// Otro principal tiene su propia cuota: no lo afecta el gasto de p1.
	ctx2 := withPrincipal(context.Background(), &Principal{Name: "p2", Role: RoleWriter})
	if _, rpcErr := s.handleToolsCall(ctx2, params); rpcErr != nil {
		t.Fatalf("otro principal no debería estar limitado por p1: %v", rpcErr)
	}
}

// Sin principal (stdio local confiable) NO hay cuota, aunque esté configurada.
func TestQuotaSkippedWithoutPrincipal(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithQuota(1))

	params := json.RawMessage(`{"name":"musubi_save_observation","arguments":{"topic_key":"q/t","content":"x"}}`)
	for i := 0; i < 3; i++ {
		if _, rpcErr := s.handleToolsCall(context.Background(), params); rpcErr != nil {
			t.Fatalf("stdio sin principal no debería tener cuota (llamada %d): %v", i+1, rpcErr)
		}
	}
}
