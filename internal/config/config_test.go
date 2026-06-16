package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfig(t *testing.T, content string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, DirName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(content), 0644); err != nil {
		t.Fatalf("write error: %v", err)
	}
	return root
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("error inesperado: %v", err)
	}
	if cfg.Embedding.Provider != "none" {
		t.Errorf("esperaba provider 'none' por defecto, obtuve %q", cfg.Embedding.Provider)
	}
	if cfg.Embedding.Dimensions != 768 {
		t.Errorf("esperaba dimensions 768 por defecto, obtuve %d", cfg.Embedding.Dimensions)
	}
}

func TestLoadStartupDefaults(t *testing.T) {
	// Config legacy sin bloque startup: deben aplicarse los defaults.
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !cfg.Startup.PrimeMemory {
		t.Error("esperaba prime_memory true por defecto")
	}
	if !cfg.Startup.AutoRegen {
		t.Error("esperaba auto_regen true por defecto")
	}
	if cfg.Startup.RecallBudget != 300 {
		t.Errorf("esperaba recall_budget 300 por defecto, obtuve %v", cfg.Startup.RecallBudget)
	}
}

func TestLoadStartupCognitiveDefault(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !cfg.Startup.CognitiveBootstrap {
		t.Error("esperaba cognitive_bootstrap true por defecto")
	}
}

func TestLoadStartupCognitiveDisableRespected(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nstartup:\n  cognitive_bootstrap: false\n  recall_budget: 300\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Startup.CognitiveBootstrap {
		t.Error("cognitive_bootstrap: false explícito debería respetarse")
	}
}

func TestLoadStartupPrimeDisableRespected(t *testing.T) {
	// Bloque presente (recall_budget seteado por init) con prime_memory: false explícito.
	root := writeConfig(t, "version: \"1.0\"\nstartup:\n  prime_memory: false\n  recall_budget: 300\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Startup.PrimeMemory {
		t.Error("prime_memory: false explícito debería respetarse")
	}
}

func TestLoadParsesStartupBlock(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nstartup:\n  prime_memory: true\n  recall_budget: 150\n  auto_regen: false\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Startup.RecallBudget != 150 || cfg.Startup.AutoRegen {
		t.Errorf("bloque startup no parseado: %+v", cfg.Startup)
	}
}

func TestLoadConflictsDefaults(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !cfg.Conflicts.Enabled {
		t.Error("esperaba conflicts.enabled true por defecto")
	}
	if cfg.Conflicts.SimilarityFloor != 0.3 || cfg.Conflicts.AutoResolveThreshold != 0.7 || cfg.Conflicts.CandidatePool != 10 {
		t.Errorf("defaults de conflicts no aplicados: %+v", cfg.Conflicts)
	}
}

func TestLoadConflictsDisableRespected(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nconflicts:\n  enabled: false\n  similarity_floor: 0.3\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Conflicts.Enabled {
		t.Error("conflicts.enabled: false explícito debería respetarse")
	}
}

func TestLoadLoopDefaults(t *testing.T) {
	// Config legacy sin bloque loop: deben aplicarse los defaults.
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !cfg.Loop.PerTurnRecall {
		t.Error("esperaba per_turn_recall true por defecto")
	}
	if cfg.Loop.RecallBudget != 250 {
		t.Errorf("esperaba recall_budget 250 por defecto, obtuve %d", cfg.Loop.RecallBudget)
	}
	if !cfg.Loop.SurfaceConflicts {
		t.Error("esperaba surface_conflicts true por defecto")
	}
}

func TestLoadLoopDisableRespected(t *testing.T) {
	// Bloque presente (recall_budget seteado) con per_turn_recall: false explícito.
	root := writeConfig(t, "version: \"1.0\"\nloop:\n  per_turn_recall: false\n  recall_budget: 250\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Loop.PerTurnRecall {
		t.Error("per_turn_recall: false explícito debería respetarse")
	}
}

func TestLoadParsesLoopBlock(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nloop:\n  per_turn_recall: true\n  recall_budget: 120\n  surface_conflicts: false\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Loop.RecallBudget != 120 || cfg.Loop.SurfaceConflicts {
		t.Errorf("bloque loop no parseado: %+v", cfg.Loop)
	}
}

func TestLoadLoopCaptureDefaults(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !cfg.Loop.CaptureReminder {
		t.Error("esperaba capture_reminder true por defecto")
	}
	if cfg.Loop.ReminderAfterTurns != 5 {
		t.Errorf("esperaba reminder_after_turns 5 por defecto, obtuve %d", cfg.Loop.ReminderAfterTurns)
	}
}

func TestLoadPipelineDefaults(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !cfg.Pipeline.Enabled {
		t.Error("esperaba pipeline.enabled true por defecto")
	}
	want := []string{"explore", "plan", "code", "verify"}
	if len(cfg.Pipeline.Phases) != len(want) {
		t.Fatalf("esperaba %d fases por defecto, obtuve %v", len(want), cfg.Pipeline.Phases)
	}
	for i, p := range want {
		if cfg.Pipeline.Phases[i] != p {
			t.Errorf("fase %d: esperaba %q, obtuve %q", i, p, cfg.Pipeline.Phases[i])
		}
	}
}

func TestLoadPipelineDisableRespected(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\npipeline:\n  enabled: false\n  phases: [diseno, build]\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Pipeline.Enabled {
		t.Error("pipeline.enabled: false explícito debería respetarse")
	}
	if len(cfg.Pipeline.Phases) != 2 || cfg.Pipeline.Phases[0] != "diseno" {
		t.Errorf("phases personalizadas no parseadas: %v", cfg.Pipeline.Phases)
	}
}

func TestLoadMultiAgentDefaults(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if !cfg.MultiAgent.Enabled {
		t.Error("esperaba multiagent.enabled true por defecto")
	}
	if cfg.MultiAgent.MaxBatchUnits != 50 {
		t.Errorf("esperaba max_batch_units 50 por defecto, obtuve %d", cfg.MultiAgent.MaxBatchUnits)
	}
}

func TestLoadMultiAgentDisableRespected(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmultiagent:\n  enabled: false\n  max_batch_units: 10\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.MultiAgent.Enabled {
		t.Error("multiagent.enabled: false explícito debería respetarse")
	}
	if cfg.MultiAgent.MaxBatchUnits != 10 {
		t.Errorf("max_batch_units personalizado no parseado: %d", cfg.MultiAgent.MaxBatchUnits)
	}
}

func TestLoadConflictsEnabledOnlyFalse(t *testing.T) {
	// Bloque con SOLO enabled:false (sin campos numéricos): debe respetarse y NO
	// re-habilitarse; los numéricos toman default.
	root := writeConfig(t, "version: \"1.0\"\nconflicts:\n  enabled: false\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Conflicts.Enabled {
		t.Error("conflicts.enabled:false (solo) NO debe re-habilitarse")
	}
	if cfg.Conflicts.SimilarityFloor != 0.3 || cfg.Conflicts.CandidatePool != 10 {
		t.Errorf("los numéricos deben tomar default: %+v", cfg.Conflicts)
	}
}

func TestLoadPipelineEnabledOnlyFalse(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\npipeline:\n  enabled: false\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Pipeline.Enabled {
		t.Error("pipeline.enabled:false (solo) NO debe re-habilitarse")
	}
	if len(cfg.Pipeline.Phases) == 0 {
		t.Error("las phases deben tomar default")
	}
}

func TestLoadMultiAgentEnabledOnlyFalse(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmultiagent:\n  enabled: false\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.MultiAgent.Enabled {
		t.Error("multiagent.enabled:false (solo) NO debe re-habilitarse")
	}
	if cfg.MultiAgent.MaxBatchUnits != 50 {
		t.Errorf("max_batch_units debe tomar default, obtuve %d", cfg.MultiAgent.MaxBatchUnits)
	}
}

func TestLoadParsesEmbeddingBlock(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\nskills_auto_resolve: true\nembedding:\n  provider: ollama\n  model: nomic-embed-text\n  base_url: http://localhost:11434\n  dimensions: 768\n")

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Embedding.Provider != "ollama" {
		t.Errorf("esperaba ollama, obtuve %q", cfg.Embedding.Provider)
	}
	if cfg.Embedding.Model != "nomic-embed-text" {
		t.Errorf("modelo inesperado: %q", cfg.Embedding.Model)
	}
}

func TestLoadAppliesDefaultsForAbsentKeys(t *testing.T) {
	// Config legacy sin bloque embedding: deben aplicarse defaults.
	root := writeConfig(t, "version: \"1.0\"\nmode: local\nskills_auto_resolve: true\n")

	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Embedding.Provider != "none" {
		t.Errorf("esperaba provider 'none', obtuve %q", cfg.Embedding.Provider)
	}
	if cfg.Embedding.BaseURL == "" || cfg.Embedding.Dimensions == 0 {
		t.Errorf("defaults no aplicados: %+v", cfg.Embedding)
	}
}

func TestLoadMemoryDefaults(t *testing.T) {
	// Config legacy sin bloque memory: deben aplicarse los defaults.
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Memory.RecallTokenBudget != 400 {
		t.Errorf("esperaba recall_token_budget 400 por defecto, obtuve %d", cfg.Memory.RecallTokenBudget)
	}
	if cfg.Memory.GistMaxTokens != 24 {
		t.Errorf("esperaba gist_max_tokens 24 por defecto, obtuve %d", cfg.Memory.GistMaxTokens)
	}
	if cfg.Memory.CandidatePool != 50 {
		t.Errorf("esperaba candidate_pool 50 por defecto, obtuve %d", cfg.Memory.CandidatePool)
	}
}

func TestLoadParsesMemoryBlock(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmemory:\n  recall_token_budget: 800\n  gist_max_tokens: 32\n  candidate_pool: 100\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Memory.RecallTokenBudget != 800 || cfg.Memory.GistMaxTokens != 32 || cfg.Memory.CandidatePool != 100 {
		t.Errorf("bloque memory no parseado: %+v", cfg.Memory)
	}
}

func TestLoadMaintenanceDefaults(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Maintenance.DedupThreshold != 0.85 {
		t.Errorf("esperaba dedup_threshold 0.85 por defecto, obtuve %v", cfg.Maintenance.DedupThreshold)
	}
	if cfg.Maintenance.DecayHalfLifeDays != 30 || cfg.Maintenance.DecayMinSalience != 0.2 || cfg.Maintenance.DecayMinAgeDays != 14 {
		t.Errorf("defaults de maintenance no aplicados: %+v", cfg.Maintenance)
	}
	if cfg.Maintenance.AutoIntervalHours != 24 {
		t.Errorf("esperaba auto_interval_hours 24 por defecto, obtuve %v", cfg.Maintenance.AutoIntervalHours)
	}
}

func TestLoadMaintenanceAutoDisableRespected(t *testing.T) {
	// Bloque presente con auto_interval_hours: 0 -> se respeta (desactivado).
	root := writeConfig(t, "version: \"1.0\"\nmaintenance:\n  dedup_threshold: 0.85\n  auto_interval_hours: 0\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Maintenance.AutoIntervalHours != 0 {
		t.Errorf("auto_interval_hours: 0 explícito debería respetarse, obtuve %v", cfg.Maintenance.AutoIntervalHours)
	}
}

func TestLoadGraphDefaults(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Graph.MaxHops != 2 || cfg.Graph.MaxFacts != 50 || cfg.Graph.MaxObservations != 5 {
		t.Errorf("defaults de graph no aplicados: %+v", cfg.Graph)
	}
}

func TestLoadUpdateDefaults(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmode: local\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Update.CheckIntervalHours != 24 {
		t.Errorf("esperaba check_interval_hours 24 por defecto, obtuve %v", cfg.Update.CheckIntervalHours)
	}
}

func TestLoadUpdateDisableRespected(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nupdate:\n  check_interval_hours: -1\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Update.CheckIntervalHours != -1 {
		t.Errorf("valor negativo (desactivado) debería respetarse, obtuve %v", cfg.Update.CheckIntervalHours)
	}
}

func TestLoadParsesGraphBlock(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\ngraph:\n  max_hops: 3\n  max_facts: 100\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Graph.MaxHops != 3 || cfg.Graph.MaxFacts != 100 {
		t.Errorf("bloque graph no parseado: %+v", cfg.Graph)
	}
}

func TestLoadParsesMaintenanceBlock(t *testing.T) {
	root := writeConfig(t, "version: \"1.0\"\nmaintenance:\n  dedup_threshold: 0.9\n  decay_half_life_days: 60\n  decay_min_salience: 0.1\n  decay_min_age_days: 7\n")
	cfg, err := Load(root)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if cfg.Maintenance.DedupThreshold != 0.9 || cfg.Maintenance.DecayHalfLifeDays != 60 ||
		cfg.Maintenance.DecayMinSalience != 0.1 || cfg.Maintenance.DecayMinAgeDays != 7 {
		t.Errorf("bloque maintenance no parseado: %+v", cfg.Maintenance)
	}
}
