package mcp

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// centralStub imita el /mcp del cerebro central para los tests del cliente de sync. Registra
// lo que recibe (auth, method, name, id, scope) y responde según status/errores configurados.
type centralStub struct {
	mu         sync.Mutex
	status     int  // status HTTP a devolver (0 => 200)
	jsonError  bool // si true, responde 200 con un error JSON-RPC (params inválidos, permanente)
	quotaError bool // si true, responde 200 con error JSON-RPC de cuota (-32002, transitorio)
	delay      time.Duration
	// capturas del último request
	gotAuth   string
	gotMethod string
	gotName   string
	gotID     interface{}
	gotScope  string
	countByID map[string]int
}

func newCentralStub() *centralStub {
	return &centralStub{countByID: map[string]int{}}
}

func (c *centralStub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	delay := c.delay
	c.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}

	body, _ := io.ReadAll(r.Body)
	var req struct {
		ID     interface{} `json:"id"`
		Method string      `json:"method"`
		Params struct {
			Name      string `json:"name"`
			Arguments struct {
				ID    string `json:"id"`
				Scope string `json:"scope"`
			} `json:"arguments"`
		} `json:"params"`
	}
	_ = json.Unmarshal(body, &req)

	c.mu.Lock()
	c.gotAuth = r.Header.Get("Authorization")
	c.gotMethod = req.Method
	c.gotName = req.Params.Name
	c.gotID = req.ID
	c.gotScope = req.Params.Arguments.Scope
	c.countByID[req.Params.Arguments.ID]++
	status := c.status
	jsonError := c.jsonError
	quotaError := c.quotaError
	c.mu.Unlock()

	if status != 0 && status != http.StatusOK {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":null,"error":{"code":-32000,"message":"stub"}}`))
		return
	}
	if quotaError {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"x","error":{"code":-32002,"message":"cuota excedida"}}`))
		return
	}
	if jsonError {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"x","error":{"code":-32602,"message":"params inválidos"}}`))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"x","result":{"content":[{"type":"text","text":"ok"}]}}`))
}

func testItem() memory.OutboxItem {
	return memory.OutboxItem{
		ObsID:      "obs-123",
		TopicKey:   "roadmap/f2",
		Content:    "una observación compartida",
		Importance: 2.0,
		MemType:    "semantic",
		ProjectID:  "musubi",
	}
}

func newTestSyncClient(t *testing.T, url string) *SyncClient {
	t.Helper()
	t.Setenv("MUSUBI_TEST_TOKEN", "secreto-abc")
	client, err := NewSyncClient(config.SyncConfig{
		CentralURL:            url, // httptest da http://; allow_insecure permite el envío
		AuthTokenEnv:          "MUSUBI_TEST_TOKEN",
		AllowInsecureToken:    true,
		RequestTimeoutSeconds: 2,
	})
	if err != nil {
		t.Fatalf("NewSyncClient: %v", err)
	}
	return client
}

func TestSyncClientPushSuccess(t *testing.T) {
	stub := newCentralStub()
	ts := httptest.NewServer(stub)
	defer ts.Close()

	client := newTestSyncClient(t, ts.URL)
	if err := client.Push(testItem()); err != nil {
		t.Fatalf("Push debía tener éxito, error: %v", err)
	}

	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.gotAuth != "Bearer secreto-abc" {
		t.Errorf("Authorization esperado 'Bearer secreto-abc', obtuve %q", stub.gotAuth)
	}
	if stub.gotMethod != "tools/call" {
		t.Errorf("method esperado tools/call, obtuve %q", stub.gotMethod)
	}
	if stub.gotName != "musubi_save_observation" {
		t.Errorf("name esperado musubi_save_observation, obtuve %q", stub.gotName)
	}
	if stub.gotID != "obs-123" {
		t.Errorf("id del request esperado obs-123, obtuve %v", stub.gotID)
	}
	if stub.gotScope != "shared" {
		t.Errorf("scope esperado shared, obtuve %q", stub.gotScope)
	}
}

func TestSyncClientTransientErrors(t *testing.T) {
	for _, status := range []int{http.StatusInternalServerError, http.StatusBadGateway, http.StatusTooManyRequests} {
		stub := newCentralStub()
		stub.status = status
		ts := httptest.NewServer(stub)
		client := newTestSyncClient(t, ts.URL)
		err := client.Push(testItem())
		if err == nil || !errors.Is(err, errTransient) {
			t.Errorf("status %d debía ser transitorio, obtuve %v", status, err)
		}
		ts.Close()
	}
}

func TestSyncClientTimeoutIsTransient(t *testing.T) {
	stub := newCentralStub()
	stub.delay = 3 * time.Second // supera el timeout de 2s del cliente
	ts := httptest.NewServer(stub)
	defer ts.Close()

	client := newTestSyncClient(t, ts.URL)
	err := client.Push(testItem())
	if err == nil || !errors.Is(err, errTransient) {
		t.Errorf("un timeout debía ser transitorio, obtuve %v", err)
	}
}

func TestSyncClientPermanentErrors(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound} {
		stub := newCentralStub()
		stub.status = status
		ts := httptest.NewServer(stub)
		client := newTestSyncClient(t, ts.URL)
		err := client.Push(testItem())
		if err == nil || !errors.Is(err, errPermanent) {
			t.Errorf("status %d debía ser permanente, obtuve %v", status, err)
		}
		ts.Close()
	}
}

func TestSyncClientJSONRPCErrorIsPermanent(t *testing.T) {
	stub := newCentralStub()
	stub.jsonError = true // 200 con error JSON-RPC
	ts := httptest.NewServer(stub)
	defer ts.Close()

	client := newTestSyncClient(t, ts.URL)
	err := client.Push(testItem())
	if err == nil || !errors.Is(err, errPermanent) {
		t.Errorf("un error JSON-RPC debía ser permanente, obtuve %v", err)
	}
}

// TestSyncClientQuotaIsTransient valida el fix de la regresión de cuota (Track 19): un rechazo por
// cuota (-32002) del central es TRANSITORIO (se libera al pasar la ventana), NO permanente — así el
// drain reintenta con backoff en vez de DEAD-LETTEREAR memoria shared por un límite temporal.
func TestSyncClientQuotaIsTransient(t *testing.T) {
	stub := newCentralStub()
	stub.quotaError = true // 200 con error JSON-RPC de cuota (-32002)
	ts := httptest.NewServer(stub)
	defer ts.Close()

	client := newTestSyncClient(t, ts.URL)
	err := client.Push(testItem())
	if err == nil || !errors.Is(err, errTransient) {
		t.Errorf("un rechazo por cuota debía ser transitorio (reintentable), obtuve %v", err)
	}
	if errors.Is(err, errPermanent) {
		t.Errorf("un rechazo por cuota NO debía ser permanente (dead-letter): %v", err)
	}
}

// TestSyncClientIdempotencyByID: dos Push del mismo id llegan con el MISMO id; el receptor
// (con UPSERT por id) no duplicaría. Acá comprobamos que el id se preserva estable.
func TestSyncClientIdempotencyByID(t *testing.T) {
	stub := newCentralStub()
	ts := httptest.NewServer(stub)
	defer ts.Close()

	client := newTestSyncClient(t, ts.URL)
	item := testItem()
	if err := client.Push(item); err != nil {
		t.Fatal(err)
	}
	if err := client.Push(item); err != nil {
		t.Fatal(err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.countByID["obs-123"] != 2 {
		t.Errorf("el server recibió %d requests para obs-123, esperaba 2 (mismo id, dedupe lo hace el UPSERT)", stub.countByID["obs-123"])
	}
}

func TestNewSyncClientRejectsInsecureHTTP(t *testing.T) {
	// http:// sin allow_insecure_token => error permanente (no filtrar token en texto plano).
	_, err := NewSyncClient(config.SyncConfig{
		CentralURL:   "http://brain.local:7717",
		AuthTokenEnv: "X",
	})
	if err == nil || !errors.Is(err, errPermanent) {
		t.Errorf("http sin allow_insecure debía rechazarse (permanente), obtuve %v", err)
	}
	// https:// siempre OK.
	if _, err := NewSyncClient(config.SyncConfig{CentralURL: "https://brain.local:7717"}); err != nil {
		t.Errorf("https debía aceptarse, obtuve %v", err)
	}
	// Empty central_url => error.
	if _, err := NewSyncClient(config.SyncConfig{CentralURL: "  "}); err == nil {
		t.Error("central_url vacío debía fallar")
	}
}

func TestBackoffSecondsRangeAndCap(t *testing.T) {
	base, max := 5, 300
	// n=1 => [5, 6]; n=2 => [10,12]; n=3 => [20,24]; ... hasta el tope.
	for _, tc := range []struct{ n, lo, hi int }{
		{1, 5, 6},
		{2, 10, 12},
		{3, 20, 24},
		{4, 40, 48},
	} {
		// Muestrear varias veces por el jitter aleatorio.
		for i := 0; i < 50; i++ {
			got := backoffSeconds(tc.n, base, max)
			if got < tc.lo || got > tc.hi {
				t.Fatalf("backoff(n=%d) = %d, esperaba [%d,%d]", tc.n, got, tc.lo, tc.hi)
			}
		}
	}
	// Un n grande satura en max.
	for i := 0; i < 50; i++ {
		got := backoffSeconds(20, base, max)
		if got != max {
			t.Fatalf("backoff(n=20) = %d, esperaba el tope %d", got, max)
		}
	}
}
