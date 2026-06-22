package mcp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestSourcingCache verifica hit/miss, limpieza perezosa de entradas vencidas y el modo
// inerte (TTL ≤ 0).
func TestSourcingCache(t *testing.T) {
	c := newSourcingCache(3600)
	if _, ok := c.get("k"); ok {
		t.Fatal("esperaba miss en caché vacío")
	}
	c.set("k", 42)
	if v, ok := c.get("k"); !ok || v.(int) != 42 {
		t.Fatalf("esperaba hit 42, obtuve %v (ok=%v)", v, ok)
	}

	// Una entrada vencida no debe devolverse y debe limpiarse al leerla.
	c.m["viejo"] = cacheEntry{value: 1, expires: time.Now().Add(-time.Second)}
	if _, ok := c.get("viejo"); ok {
		t.Fatal("una entrada vencida no debe devolverse")
	}
	if _, existe := c.m["viejo"]; existe {
		t.Fatal("la entrada vencida debe limpiarse al leerla")
	}

	// Caché inerte (TTL ≤ 0): set es no-op, get siempre falla.
	inert := newSourcingCache(0)
	inert.set("x", 1)
	if _, ok := inert.get("x"); ok {
		t.Fatal("un caché inerte no debe cachear")
	}
}

// TestDiscoverSkillsUsaCache verifica que dos llamadas con la misma query pegan al
// marketplace UNA sola vez (la segunda sale del caché), preservando el rate limit.
func TestDiscoverSkillsUsaCache(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		fmt.Fprint(w, respMarketplace(skillDescubierta))
	}))
	defer srv.Close()

	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n\ngo 1.26\n"), 0644); err != nil {
		t.Fatalf("no se pudo crear go.mod: %v", err)
	}

	cfg := config.SourcingConfig{
		Enabled:            true,
		MarketplaceEnabled: true,
		MarketplaceURL:     srv.URL,
		CacheSeconds:       3600, // caché activo
	}
	s := NewMcpServer(engine, root, embedding.NoopProvider{}, WithSourcing(cfg))

	for i := 0; i < 2; i++ {
		if _, rpcErr := call(t, s, "musubi_discover_skills", map[string]interface{}{"query": "go"}); rpcErr != nil {
			t.Fatalf("llamada %d falló: %+v", i, rpcErr)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("esperaba 1 hit al marketplace (2ª desde caché), obtuve %d", got)
	}
}
