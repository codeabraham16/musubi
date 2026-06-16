package main

import "testing"

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
