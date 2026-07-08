package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// TestVersioninfoMatchesVERSION garantiza que la versión embebida en el recurso de
// Windows (cmd/musubi/versioninfo.json, de donde salen los .syso) no diverja de la
// fuente ÚNICA de verdad: el archivo VERSION en la raíz del repo.
//
// La auditoría de producibilidad (audit/2026-07-08) encontró versioninfo.json
// congelado en 0.57.0.0 mientras el proyecto iba por 0.78 — el paso manual de
// regenerarlo se había saltado ~20 releases, así que el .exe de Windows reportaba
// una versión vieja en sus Propiedades. Este test hace que esa divergencia sea
// IMPOSIBLE de mergear: si bumpeás VERSION y te olvidás de versioninfo.json (o al
// revés), el CI falla. Combinado con el guard de release.yml (el tag debe igualar a
// VERSION), la versión tiene una sola fuente de verdad de punta a punta.
func TestVersioninfoMatchesVERSION(t *testing.T) {
	raw, err := os.ReadFile("../../VERSION")
	if err != nil {
		t.Fatalf("no se pudo leer VERSION: %v", err)
	}
	want := strings.TrimSpace(string(raw)) // ej. "0.78.0"
	if strings.Count(want, ".") != 2 {
		t.Fatalf("VERSION debe tener el formato X.Y.Z, es %q", want)
	}

	viRaw, err := os.ReadFile("versioninfo.json")
	if err != nil {
		t.Fatalf("no se pudo leer versioninfo.json: %v", err)
	}
	var vi struct {
		FixedFileInfo struct {
			FileVersion    struct{ Major, Minor, Patch, Build int }
			ProductVersion struct{ Major, Minor, Patch, Build int }
		}
		StringFileInfo struct {
			FileVersion    string
			ProductVersion string
		}
	}
	if err := json.Unmarshal(viRaw, &vi); err != nil {
		t.Fatalf("versioninfo.json inválido: %v", err)
	}

	// FixedFileInfo numérico (Major.Minor.Patch) debe igualar a VERSION; Build es 0.
	fixedFile := fmt.Sprintf("%d.%d.%d", vi.FixedFileInfo.FileVersion.Major, vi.FixedFileInfo.FileVersion.Minor, vi.FixedFileInfo.FileVersion.Patch)
	fixedProd := fmt.Sprintf("%d.%d.%d", vi.FixedFileInfo.ProductVersion.Major, vi.FixedFileInfo.ProductVersion.Minor, vi.FixedFileInfo.ProductVersion.Patch)
	// StringFileInfo lleva el cuarto componente Build: "X.Y.Z.0".
	wantStr := want + ".0"

	checks := []struct {
		name, got, want string
	}{
		{"FixedFileInfo.FileVersion", fixedFile, want},
		{"FixedFileInfo.ProductVersion", fixedProd, want},
		{"StringFileInfo.FileVersion", vi.StringFileInfo.FileVersion, wantStr},
		{"StringFileInfo.ProductVersion", vi.StringFileInfo.ProductVersion, wantStr},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, esperaba %q (según VERSION); regenerá versioninfo.json y los .syso", c.name, c.got, c.want)
		}
	}
}
