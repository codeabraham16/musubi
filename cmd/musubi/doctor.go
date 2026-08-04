package main

import (
	"encoding/json"
	"fmt"
	"os"

	"musubi/internal/cognition"
	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// Códigos de los checks que diagnostican la CONFIG en vez de la base de datos.
//
// Viven acá y no en el registry de internal/memory a propósito: los de allá diagnostican la BASE DE
// DATOS. Meter estos ahí obligaría a que la memoria conozca la cognición y los embeddings, que es
// exactamente el acoplamiento que los pilares evitan.
const (
	cognitionGatewayCheckCode = "cognition_gateway"
	embeddingGatewayCheckCode = "embedding_gateway"
)

// configChecks diagnostica los dos porteros de privacidad: el que se para frente al motor de
// cognición y el que se para frente al proveedor de embeddings.
//
// Sin esto, apagar una guarda de privacidad sólo se veía en el log de arranque de un daemon — es
// decir, no se veía. Un riesgo que nadie mira no está mitigado.
func configChecks(root string) []memory.CheckResult {
	cfg, err := config.Load(root)
	if err != nil {
		msg := fmt.Sprintf("no se pudo leer la config para diagnosticar el portero: %v", err)
		return []memory.CheckResult{
			{Code: cognitionGatewayCheckCode, Status: "error", Message: msg},
			{Code: embeddingGatewayCheckCode, Status: "error", Message: msg},
		}
	}
	cg := cognition.InspectGateway(cfg.Cognition)
	eg := embedding.InspectGateway(cfg.Embedding)
	return []memory.CheckResult{
		{Code: cognitionGatewayCheckCode, Status: cg.Status, Message: cg.Message},
		{Code: embeddingGatewayCheckCode, Status: eg.Status, Message: eg.Message},
	}
}

// configCheck devuelve el check de config con ese código, o ok=false si el código no es de los
// que miran la config (y entonces le toca al engine).
func configCheck(root, code string) (memory.CheckResult, bool) {
	for _, c := range configChecks(root) {
		if c.Code == code {
			return c, true
		}
	}
	return memory.CheckResult{}, false
}

// runDoctor implementa el comando 'musubi doctor':
//
//	musubi doctor [--json] [--check <code>]
//	musubi doctor repair --check <code> [--plan|--dry-run|--apply]
func runDoctor(args []string) {
	root := workspaceDir()
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al abrir la base de datos: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	if len(args) > 0 && args[0] == "repair" {
		runDoctorRepair(engine, args[1:])
		return
	}

	check := parseFlagValue(args, "--check")
	asJSON := hasFlag(args, "--json")

	// --check presente pero sin valor: error explícito en vez de caer en silencio
	// al diagnóstico completo.
	if check == "" && hasFlag(args, "--check") {
		fmt.Fprintln(os.Stderr, "doctor: --check requiere un código (ej. --check fts_consistency)")
		os.Exit(1)
	}

	if check != "" {
		var res memory.CheckResult
		if c, esDeConfig := configCheck(root, check); esDeConfig {
			res = c
		} else {
			r, err := engine.RunCheck(check)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
			res = r
		}
		if asJSON {
			printJSON(res)
		} else {
			fmt.Printf("[%s] %s — %s\n", res.Status, res.Code, res.Message)
		}
		if res.Status == "error" {
			os.Exit(1)
		}
		return
	}

	rep, err := engine.Diagnose()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al diagnosticar: %v\n", err)
		os.Exit(1)
	}
	// Los porteros se suman al reporte y pueden ensuciar el estado global: una guarda de privacidad
	// apagada ES un problema del proyecto, aunque la base esté impecable.
	for _, c := range configChecks(root) {
		rep.Checks = append(rep.Checks, c)
		if c.Status != "ok" {
			rep.Status = "issues"
		}
	}
	if asJSON {
		printJSON(rep)
	} else {
		fmt.Printf("Diagnóstico: %s\n", rep.Status)
		for _, c := range rep.Checks {
			fmt.Println(formatCheckLine(c))
		}
	}
}

// formatCheckLine arma la línea legible de un check del diagnóstico: marca según
// estado (✓/!/✗) y, si es reparable y no está OK, la pista del comando de repair.
func formatCheckLine(c memory.CheckResult) string {
	mark := "✓"
	if c.Status == "warning" {
		mark = "!"
	} else if c.Status == "error" {
		mark = "✗"
	}
	repair := ""
	if c.Repairable && c.Status != "ok" {
		repair = "  (reparable: musubi doctor repair --check " + c.Code + " --apply)"
	}
	return fmt.Sprintf("  %s %s — %s%s", mark, c.Code, c.Message, repair)
}

func runDoctorRepair(engine *memory.DbEngine, args []string) {
	check := parseFlagValue(args, "--check")
	if check == "" {
		fmt.Fprintln(os.Stderr, "doctor repair requiere --check <code>")
		os.Exit(1)
	}
	mode := parseRepairMode(args)
	res, err := engine.Repair(check, mode)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al reparar: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(res.Message)
}

// parseRepairMode devuelve el modo según el flag presente; default seguro dry-run.
func parseRepairMode(args []string) string {
	switch {
	case hasFlag(args, "--apply"):
		return "apply"
	case hasFlag(args, "--plan"):
		return "plan"
	default:
		return "dry-run"
	}
}

// parseFlagValue devuelve el valor que sigue a flag (ej. "--check X" -> "X"), o "".
func parseFlagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// hasFlag indica si flag está presente en args.
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func printJSON(v interface{}) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error al serializar: %v\n", err)
		return
	}
	fmt.Println(string(data))
}
