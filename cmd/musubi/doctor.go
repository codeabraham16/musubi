package main

import (
	"encoding/json"
	"fmt"
	"os"

	"musubi/internal/memory"
)

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

	if check != "" {
		res, err := engine.RunCheck(check)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(1)
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
	if asJSON {
		printJSON(rep)
	} else {
		fmt.Printf("Diagnóstico: %s\n", rep.Status)
		for _, c := range rep.Checks {
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
			fmt.Printf("  %s %s — %s%s\n", mark, c.Code, c.Message, repair)
		}
	}
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
