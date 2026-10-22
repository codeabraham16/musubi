package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"musubi/internal/embedding"
)

// contrato_tool_test.go SELLA el CONTRATO DE SEGURIDAD por-tool {readOnly, actuaAfuera} contra un
// golden committeado, para que no pueda cambiar en silencio (E2 del plan engram→Musubi, mapeo del
// agent_contract.go de engram). Dos piezas con trabajos distintos:
//
//  1. TestContratoPorToolFijado — el TRIPWIRE. Cualquier tool cuyo readOnly o actuaAfuera difiera
//     del golden (incluida un alta o una baja) pone ROJO el CI NOMBRANDO la tool. Atrapa en
//     particular la dirección peligrosa readOnly:true→false —una tool que antes LEÍA y ahora
//     ESCRIBE—, que hoy no cubre ninguna otra guarda: TestEveryReadOnlyToolClassified sólo recorre
//     las readOnly, así que una que deja de serlo se le escapa por abajo.
//  2. TestLaHuellaCubreElEfecto — prueba que huellaDelCatalogo PLIEGA el efecto: invertir readOnly
//     de una tool mueve la huella aunque el cable (Tool: nombre+descripción+schema) no se toque.
//     Es el hermano de TestI4, que prueba lo mismo para la descripción.
//
// El golden pinea el catálogo COMPLETO (dormidas incluidas): una dormida que cambia de efecto es tan
// silenciosa como una viva, y por eso toolslist también tiene un golden completo aparte del visible.

// contratoGolden es una fila del golden: el contrato de seguridad OBSERVABLE de una tool.
type contratoGolden struct {
	Name        string `json:"name"`
	ReadOnly    bool   `json:"read_only"`
	ActuaAfuera bool   `json:"actua_afuera"`
}

// contratosDelCatalogo arma, ordenada por nombre, la lista de contratos del catálogo COMPLETO. Usa
// buildRegistry sobre un McpServer sin inicializar —el mismo camino puro que CatalogFingerprint—, así
// que no lee estado del servidor y no puede divergir del registro que de verdad se sirve.
func contratosDelCatalogo() []contratoGolden {
	s := &McpServer{}
	entries := s.buildRegistry()
	out := make([]contratoGolden, 0, len(entries))
	for i := range entries {
		out = append(out, contratoGolden{
			Name:        entries[i].Name,
			ReadOnly:    entries[i].readOnly,
			ActuaAfuera: entries[i].actuaAfuera(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// diffContratos describe, por nombre, cada divergencia entre el golden (want) y el registro vivo
// (got): alta, baja o cambio de efecto/alcance. Es lo que hace que el fallo diga QUÉ cambió.
func diffContratos(want, got []contratoGolden) []string {
	index := func(xs []contratoGolden) map[string]contratoGolden {
		m := make(map[string]contratoGolden, len(xs))
		for _, x := range xs {
			m[x.Name] = x
		}
		return m
	}
	w, g := index(want), index(got)
	var out []string
	for name, gv := range g {
		wv, ok := w[name]
		if !ok {
			out = append(out, fmt.Sprintf("%s: tool NUEVA, falta en el golden (readOnly=%t actuaAfuera=%t)", name, gv.ReadOnly, gv.ActuaAfuera))
			continue
		}
		if wv.ReadOnly != gv.ReadOnly {
			out = append(out, fmt.Sprintf("%s: readOnly %t→%t", name, wv.ReadOnly, gv.ReadOnly))
		}
		if wv.ActuaAfuera != gv.ActuaAfuera {
			out = append(out, fmt.Sprintf("%s: actuaAfuera %t→%t", name, wv.ActuaAfuera, gv.ActuaAfuera))
		}
	}
	for name, wv := range w {
		if _, ok := g[name]; !ok {
			out = append(out, fmt.Sprintf("%s: tool BORRADA, sigue en el golden (readOnly=%t actuaAfuera=%t)", name, wv.ReadOnly, wv.ActuaAfuera))
		}
	}
	sort.Strings(out)
	return out
}

func TestContratoPorToolFijado(t *testing.T) {
	filas := contratosDelCatalogo()
	got, err := json.MarshalIndent(filas, "", "  ")
	if err != nil {
		t.Fatalf("marshal contratos: %v", err)
	}

	golden := filepath.Join("testdata", "contrato-tools.golden.json")
	if *updateGolden {
		if mkErr := os.MkdirAll("testdata", 0o755); mkErr != nil {
			t.Fatalf("mkdir testdata: %v", mkErr)
		}
		if wErr := os.WriteFile(golden, got, 0o644); wErr != nil {
			t.Fatalf("escribir golden: %v", wErr)
		}
		t.Logf("golden regenerado: %s", golden)
		return
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("leer golden (%s): %v — corré con -update para generarlo", golden, err)
	}
	if bytes.Equal(normalizeEOL(got), normalizeEOL(want)) {
		return
	}

	// Diverge: NOMBRAR la(s) tool(s) para que el fallo diga QUÉ cambió, no sólo que cambió.
	var wantFilas []contratoGolden
	if uErr := json.Unmarshal(normalizeEOL(want), &wantFilas); uErr != nil {
		t.Fatalf("parsear el golden (%s): %v — si cambió el formato, regenerá con -update", golden, uErr)
	}
	for _, d := range diffContratos(wantFilas, filas) {
		t.Errorf("contrato por-tool cambió — %s", d)
	}
	t.Errorf("el contrato de seguridad por-tool cambió respecto del golden.\n" +
		"Si el cambio es INTENCIONAL (y entendés por qué una tool cambia de efecto o alcance), regenerá con:\n" +
		"  go test ./internal/mcp -run TestContratoPorToolFijado -update")
}

func TestLaHuellaCubreElEfecto(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	_, original := huellaDelCatalogo(s.tools)

	copia := make([]toolEntry, len(s.tools))
	copy(copia, s.tools)
	tocada := false
	for i := range copia {
		if copia[i].dormant && !toolsAllEnabled() {
			continue // no entra a la huella, tocarla no probaría nada
		}
		// Invierte el EFECTO sin tocar el cable (Tool): queda idéntico nombre, descripción y schema,
		// así que lo único que puede mover la huella es el contrato plegado.
		copia[i].readOnly = !copia[i].readOnly
		tocada = true
		break
	}
	if !tocada {
		t.Fatal("no hay ninguna tool visible: el catálogo está vacío y el test no probaría nada")
	}

	if _, movida := huellaDelCatalogo(copia); movida == original {
		t.Errorf("invertir readOnly no movió la huella (%s): la huella no pliega el efecto, sólo el cable", original)
	}
}
