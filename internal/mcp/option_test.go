package mcp

import (
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestNewMcpServerExistingCallerSyntaxUnchanged verifica que la llamada de 3 argumentos
// sigue compilando sin modificación (compatibilidad retroactiva del variadic opts).
func TestNewMcpServerExistingCallerSyntaxUnchanged(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	// Llamada con 3 argumentos — debe compilar y no producir panic.
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	if s == nil {
		t.Fatal("NewMcpServer devolvió nil")
	}
}

// TestWithSourcingSetsCampo verifica que WithSourcing aplica la configuración al
// campo sourcing del servidor.
func TestWithSourcingSetsCampo(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	cfg := config.SourcingConfig{
		Enabled:       false,
		CatalogURL:    "https://example.com/catalog.json",
		MaxCandidates: 5,
		CacheSeconds:  60,
	}

	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithSourcing(cfg))
	if s == nil {
		t.Fatal("NewMcpServer devolvió nil")
	}

	// El campo sourcing debe reflejar el valor pasado.
	if s.sourcing.Enabled != false {
		t.Errorf("Enabled: esperaba false, obtuve %v", s.sourcing.Enabled)
	}
	if s.sourcing.CatalogURL != "https://example.com/catalog.json" {
		t.Errorf("CatalogURL incorrecto: %q", s.sourcing.CatalogURL)
	}
	if s.sourcing.MaxCandidates != 5 {
		t.Errorf("MaxCandidates: esperaba 5, obtuve %d", s.sourcing.MaxCandidates)
	}
}
