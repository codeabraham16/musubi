package main

import (
	"strings"
	"testing"

	"musubi/internal/memory"
)

func TestParseRepairMode(t *testing.T) {
	cases := map[string]string{
		"--apply":   "apply",
		"--dry-run": "dry-run",
		"--plan":    "plan",
		"":          "dry-run", // default seguro
	}
	for flag, want := range cases {
		var args []string
		if flag != "" {
			args = []string{flag}
		}
		if got := parseRepairMode(args); got != want {
			t.Errorf("parseRepairMode(%q) = %q, quiero %q", flag, got, want)
		}
	}
}

func TestParseFlagValue(t *testing.T) {
	args := []string{"repair", "--check", "fts_consistency", "--apply"}
	if got := parseFlagValue(args, "--check"); got != "fts_consistency" {
		t.Errorf("parseFlagValue --check = %q", got)
	}
	if got := parseFlagValue(args, "--missing"); got != "" {
		t.Errorf("parseFlagValue de flag ausente debe ser vacío, obtuve %q", got)
	}
}

func TestFormatCheckLine(t *testing.T) {
	cases := []struct {
		name        string
		in          memory.CheckResult
		wantMark    string
		wantRepair  bool
		wantMessage string
	}{
		{"ok sin repair", memory.CheckResult{Code: "fts", Status: "ok", Message: "todo bien", Repairable: true}, "✓", false, "todo bien"},
		{"warning reparable", memory.CheckResult{Code: "orphans", Status: "warning", Message: "hay huérfanos", Repairable: true}, "!", true, "hay huérfanos"},
		{"error reparable", memory.CheckResult{Code: "fts_consistency", Status: "error", Message: "desync", Repairable: true}, "✗", true, "desync"},
		{"error no reparable", memory.CheckResult{Code: "x", Status: "error", Message: "roto", Repairable: false}, "✗", false, "roto"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			line := formatCheckLine(tc.in)
			if !strings.Contains(line, tc.wantMark) {
				t.Errorf("esperaba la marca %q en %q", tc.wantMark, line)
			}
			if !strings.Contains(line, tc.in.Code) || !strings.Contains(line, tc.wantMessage) {
				t.Errorf("la línea debe incluir code y message: %q", line)
			}
			hasRepair := strings.Contains(line, "reparable: musubi doctor repair")
			if hasRepair != tc.wantRepair {
				t.Errorf("repair hint: esperaba %v, obtuve %v en %q", tc.wantRepair, hasRepair, line)
			}
		})
	}
}
