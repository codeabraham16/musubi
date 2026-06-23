package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// export.go implementa 'musubi export': vuelca un snapshot JSON del estado de la
// memoria —salud, insights, ledger de tokens y mapa de conocimiento por dominio— en
// stdout (o a un archivo con --out). Es la fuente de datos para dashboards y
// observabilidad externa: model-free, read-only, una sola pasada. Reúne las mismas
// vistas que exponen las tools musubi_doctor/insights/tokens, con la forma estable
// que consumen las UIs.

// exportSnapshot es el documento JSON que produce 'musubi export'.
type exportSnapshot struct {
	GeneratedAt string                `json:"generated_at"`
	Version     string                `json:"version"`
	Health      memory.DiagnoseReport `json:"health"`
	Insights    memory.InsightsReport `json:"insights"`
	Tokens      memory.BudgetStatus   `json:"tokens"`
	Graph       exportGraph           `json:"graph"`
	Recent      []memory.ObsCard      `json:"recent"`
}

// exportGraph es el mapa de conocimiento: total de observaciones activas y su
// reparto por dominio de topic.
type exportGraph struct {
	TotalObservations int                  `json:"total_observations"`
	Domains           []memory.DomainCount `json:"domains"`
}

// buildExportSnapshot compone el snapshot read-only a partir del motor. budget es el
// presupuesto blando de sesión (memory.session_token_budget), usado para el estado del
// ledger; now permite un timestamp determinista en tests.
func buildExportSnapshot(engine *memory.DbEngine, version string, budget int, now time.Time) (exportSnapshot, error) {
	snap := exportSnapshot{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		Version:     version,
	}

	health, err := engine.Diagnose()
	if err != nil {
		return snap, fmt.Errorf("export: diagnóstico: %w", err)
	}
	snap.Health = health

	ins, err := engine.Insights()
	if err != nil {
		return snap, fmt.Errorf("export: insights: %w", err)
	}
	snap.Insights = ins

	ledger, err := engine.LedgerStatus()
	if err != nil {
		return snap, fmt.Errorf("export: ledger: %w", err)
	}
	snap.Tokens = ledger.Budget(budget)

	domains, err := engine.TopicDomainCounts()
	if err != nil {
		return snap, fmt.Errorf("export: dominios: %w", err)
	}
	snap.Graph = exportGraph{TotalObservations: ins.Observations.Active, Domains: domains}

	recent, err := engine.RecentObservations(12)
	if err != nil {
		return snap, fmt.Errorf("export: memorias recientes: %w", err)
	}
	snap.Recent = recent

	return snap, nil
}

// runExport implementa 'musubi export [--out <ruta>]'. Imprime el snapshot JSON en
// stdout, o lo escribe en --out si se pasa (con un aviso por stderr).
func runExport(args []string) {
	root := workspaceDir()
	if err := ensureWorkspace(root); err != nil {
		fmt.Fprintf(os.Stderr, "Error al preparar workspace: %v\n", err)
		os.Exit(1)
	}
	cfg, err := config.Load(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al cargar configuración: %v\n", err)
		os.Exit(1)
	}
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al abrir la base de datos: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	snap, err := buildExportSnapshot(engine, version, cfg.Memory.SessionTokenBudget, time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al serializar: %v\n", err)
		os.Exit(1)
	}

	if out := parseFlagValue(args, "--out"); out != "" {
		if err := os.WriteFile(out, append(data, '\n'), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error al escribir %s: %v\n", out, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Snapshot escrito en %s\n", out)
		return
	}
	fmt.Println(string(data))
}
