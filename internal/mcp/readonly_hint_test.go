package mcp

// Verifica el invariante de la anotación MCP readOnlyHint que emite tools/list.
//
// R5 (invariante central): para CADA tool del catálogo —dormidas incluidas con
// MUSUBI_TOOLS_ALL— el hint emitido es exactamente `readOnly || ro==roLeeConEscrituraIncidental`.
// El barrido ata TODO el catálogo a la fórmula, no sólo las tools elegidas: una tool nueva mal
// clasificada cae acá sin que haya que acordarse de agregarla.
//
// 🔴 CÓMO SABOTEAR (los dos sabotajes dirigidos de la spec; cada uno ataca el invariante que
// este test declara, no una validación lateral):
//   A) En registry.go, quitar `|| s.tools[i].ro == roLeeConEscrituraIncidental` de handleToolsList.
//      TestReadOnlyHintInvariante y ...Anclas DEBEN ponerse ROJO nombrando musubi_recall (y las
//      otras incidentales): pierden el hint aunque son lecturas. Revertir.
//   B) Forzar la condición a `true` (hint en toda tool). DEBEN ponerse ROJO nombrando
//      musubi_save_observation: un mutador puro no debe traer la anotación. Revertir.

import "testing"

// hintEsperado es la fórmula declarada por la spec (R5). readOnlyHint responde "¿muta estado
// observable?", que el registro YA contesta: readOnly (no muta nada) o ro igual a
// roLeeConEscrituraIncidental (su única escritura es incidental, p.ej. musubi_recall reforzando
// el acceso de lo que devuelve).
func hintEsperado(i int, s *McpServer) bool {
	return s.tools[i].readOnly || s.tools[i].ro == roLeeConEscrituraIncidental
}

// anotacionesPorNombre arma el mapa nombre->*ToolAnnotations tal como sale por el cable de
// tools/list, para poder distinguir "hint en true" de "clave annotations ausente" (R3/R7).
func anotacionesPorNombre(t *testing.T, s *McpServer) map[string]*ToolAnnotations {
	t.Helper()
	salida, ok := s.handleToolsList().(map[string]interface{})
	if !ok {
		t.Fatalf("handleToolsList no devolvió un map[string]interface{}")
	}
	tools, ok := salida["tools"].([]Tool)
	if !ok {
		t.Fatalf("tools/list no trae []Tool")
	}
	m := make(map[string]*ToolAnnotations, len(tools))
	for _, tl := range tools {
		m[tl.Name] = tl.Annotations
	}
	return m
}

// TestReadOnlyHintInvariante — R5 + R3 + R7 sobre el catálogo COMPLETO: cada tool emite el hint
// si y sólo si cumple la fórmula, y una tool que no la cumple NO trae la clave annotations.
func TestReadOnlyHintInvariante(t *testing.T) {
	t.Setenv("MUSUBI_TOOLS_ALL", "1")
	s := NewMcpServer(nil, "", nil)
	if len(s.tools) == 0 {
		t.Fatal("registro de tools vacío")
	}
	ann := anotacionesPorNombre(t, s)

	barridas := 0
	for i := range s.tools {
		name := s.tools[i].Tool.Name
		a, presente := ann[name]
		if !presente {
			t.Errorf("%s: está en el registro pero no en la salida de tools/list", name)
			continue
		}
		got := a != nil && a.ReadOnlyHint
		if want := hintEsperado(i, s); got != want {
			t.Errorf("%s: readOnlyHint emitido=%v, esperaba %v (readOnly=%v, ro==incidental=%v)",
				name, got, want, s.tools[i].readOnly, s.tools[i].ro == roLeeConEscrituraIncidental)
		}
		// R3/R7: si no corresponde hint, la clave annotations debe estar AUSENTE (puntero nil),
		// no presente con readOnlyHint:false.
		if !hintEsperado(i, s) && a != nil {
			t.Errorf("%s: no corresponde hint pero trae annotations=%+v (debía ser nil ⇒ clave ausente)", name, *a)
		}
		// R6/D2: la anotación vive SÓLO en la copia local de handleToolsList. El spec GUARDADO en el
		// registro —el que marshalea huellaDelCatalogo— nunca la lleva, así catalog_sha256 no se mueve.
		// Si un refactor la setea sobre el spec, la huella se movería y este invariante lo caza.
		if s.tools[i].Tool.Annotations != nil {
			t.Errorf("%s: el spec guardado trae Annotations=%+v; debe vivir sólo en la copia de tools/list, o la huella se mueve",
				name, *s.tools[i].Tool.Annotations)
		}
		barridas++
	}
	if barridas < 80 {
		t.Fatalf("sólo se barrieron %d tools; MUSUBI_TOOLS_ALL no tomó o el registro se achicó", barridas)
	}
}

// TestReadOnlyHintAnclas — anclas nombradas de la spec para que el sabotaje sea legible.
func TestReadOnlyHintAnclas(t *testing.T) {
	t.Setenv("MUSUBI_TOOLS_ALL", "1")
	s := NewMcpServer(nil, "", nil)
	ann := anotacionesPorNombre(t, s)

	// R1 lectura pura + R2 lectura con escritura incidental (musubi_recall, el que motiva el cambio).
	conHint := []string{
		"musubi_recall_facts", "musubi_spec_grep", // readOnly puro
		"musubi_recall", "musubi_memory_expand", "musubi_recall_code", // roLeeConEscrituraIncidental
	}
	for _, n := range conHint {
		a, ok := ann[n]
		if !ok {
			t.Errorf("%s: ausente del catálogo", n)
			continue
		}
		if a == nil || !a.ReadOnlyHint {
			t.Errorf("%s: esperaba annotations.readOnlyHint=true (es lectura), salió %+v", n, a)
		}
	}

	// R3 mutador puro: sin la clave annotations.
	sinHint := []string{"musubi_save_observation", "musubi_save_code", "musubi_judge", "musubi_work"}
	for _, n := range sinHint {
		a, ok := ann[n]
		if !ok {
			t.Errorf("%s: ausente del catálogo", n)
			continue
		}
		if a != nil {
			t.Errorf("%s: NO debía traer annotations (muta estado), salió %+v", n, *a)
		}
	}
}
