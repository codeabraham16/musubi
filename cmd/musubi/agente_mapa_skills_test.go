package main

import (
	"context"
	"strings"
	"testing"
	"unicode/utf16"

	"musubi/internal/mcp"
)

// CON LAS SKILLS QUE MUSUBI INSTALA, EL MAPA ENTRA ENTERO Y DA UNA SKILL A CADA ALCANCE.
//
// Es el caso real y el más grande: las diez skills cognitivas que `musubi setup` escribe y que el
// arranque exporta a .claude/skills. Las pruebas de internal/mcp fijan las reglas del mapa con
// skills inventadas; ésta fija que, con las de verdad, el texto quepa en los 2048 del cliente sin
// que el recorte se coma un renglón, que cada alcance tenga su skill, y que spec-graph —que empata
// a sdd-flow en implement, porque ambas aplican a los tres alcances— comparta ese renglón sin
// desalojar a nadie.
//
// Sabotaje que la hace fallar: que orchestrate-multiagent deje de declarar su alcance.
// arnes: archivo="cmd/musubi/cognitive.go"
// arnes: de="\t\t\tAppliesTo:     []string{skills.TareaOrquestar},"
// arnes: a="\t\t\tAppliesTo:     []string{},"
//
// Sabotaje que la hace fallar: que spec-graph cambie de nombre y deje de cerrar el renglón de implement.
// arnes: archivo="cmd/musubi/cognitive.go"
// arnes: de="\t\t\tName:          \"spec-graph\","
// arnes: a="\t\t\tName:          \"spec-graphx\","
func TestElMapaDeLasSkillsCognitivasEntraEntero(t *testing.T) {
	root := t.TempDir()
	if _, err := writeCognitiveSkills(root); err != nil {
		t.Fatalf("escribir las skills cognitivas: %v", err)
	}
	if _, err := exportarSkillsAlAgente(root); err != nil {
		t.Fatalf("exportarlas al formato del agente: %v", err)
	}

	s := mcp.NewMcpServer(nil, root, nil, mcp.WithInstruccionesParaElAgente())
	resp, ok := s.Dispatch(context.Background(), mcp.JsonRpcRequest{JsonRpc: "2.0", ID: 1, Method: "initialize"})
	if !ok || resp.Error != nil {
		t.Fatalf("initialize falló: %+v", resp.Error)
	}
	res, _ := resp.Result.(map[string]interface{})
	texto, _ := res["instructions"].(string)

	// 2048 es el tope del cliente, un hecho del binario de Claude Code: se clava acá.
	if n := len(utf16.Encode([]rune(texto))); n > 2048 {
		t.Errorf("las instrucciones miden %d unidades UTF-16 y Claude Code corta en 2048", n)
	}
	for _, skill := range []string{"adversarial-review", "audit-structure-flow", "orchestrate-multiagent", "plan-ahead", "sdd-flow"} {
		if !strings.Contains(texto, "→ "+skill) {
			t.Errorf("el mapa no nombra %s: con las skills que Musubi instala, cada alcance tiene la suya", skill)
		}
	}

	// spec-graph no gana un renglón propio: empata a sdd-flow en implement (ambas aplican a los tres
	// alcances), así que viaja DETRÁS de la flecha, tras ", ", no después de "→ ". Por eso no entra
	// en el bucle de arriba y se chequea el renglón de implement entero: ordenado, cierra con las dos.
	// Si spec-graph cae del mapa —perdió su alcance o cambió de nombre—, el renglón vuelve a sdd-flow solo.
	var renglonImplement string
	for _, l := range strings.Split(texto, "\n") {
		if strings.Contains(l, "implementando") {
			renglonImplement = l
			break
		}
	}
	if !strings.HasSuffix(renglonImplement, "→ sdd-flow, spec-graph") {
		t.Errorf("el renglón de implement debe cerrar con «→ sdd-flow, spec-graph» (empate), y da: %q", renglonImplement)
	}

	t.Logf("instrucciones con el mapa: %d unidades UTF-16", len(utf16.Encode([]rune(texto))))
}
