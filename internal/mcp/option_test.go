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

// TestRemainingOptionsSetCampos verifica que el resto de los Option aplican su
// configuración al campo correspondiente del servidor.
func TestRemainingOptionsSetCampos(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{},
		WithMemory(config.MemoryConfig{RecallTokenBudget: 1234}),
		WithMaintenance(config.MaintenanceConfig{DedupThreshold: 0.91}),
		WithGraph(config.GraphConfig{MaxHops: 7}),
		WithConflicts(config.ConflictConfig{SimilarityFloor: 0.42}),
		WithPipeline(config.PipelineConfig{Enabled: true, Phases: []string{"design", "build"}}),
		WithMultiAgent(config.MultiAgentConfig{MaxBatchUnits: 33}),
	)
	if s == nil {
		t.Fatal("NewMcpServer devolvió nil")
	}

	if s.memory.RecallTokenBudget != 1234 {
		t.Errorf("memory.RecallTokenBudget: esperaba 1234, obtuve %d", s.memory.RecallTokenBudget)
	}
	if s.maintenance.DedupThreshold != 0.91 {
		t.Errorf("maintenance.DedupThreshold: esperaba 0.91, obtuve %v", s.maintenance.DedupThreshold)
	}
	if s.graph.MaxHops != 7 {
		t.Errorf("graph.MaxHops: esperaba 7, obtuve %d", s.graph.MaxHops)
	}
	if s.conflicts.SimilarityFloor != 0.42 {
		t.Errorf("conflicts.SimilarityFloor: esperaba 0.42, obtuve %v", s.conflicts.SimilarityFloor)
	}
	if !s.pipeline.Enabled || len(s.pipeline.Phases) != 2 {
		t.Errorf("pipeline no aplicó: %+v", s.pipeline)
	}
	if s.multiagent.MaxBatchUnits != 33 {
		t.Errorf("multiagent.MaxBatchUnits: esperaba 33, obtuve %d", s.multiagent.MaxBatchUnits)
	}
}
