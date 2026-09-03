package main

// Tests del RADIO DE IMPACTO: el hook PreToolUse sobre las tools que ESCRIBEN.
//
// La superficie existe porque leer y editar disparan preguntas distintas, y hasta ahora el grafo
// sólo contestaba la de leer. Lo que estos tests protegen es que la de editar conteste de verdad:
// que use el cierre transitivo y no sólo los callers directos (I2), que un archivo aislado lo DIGA
// en vez de callar (I3), y que sin grafo siga siendo inerte (I4).

import (
	"strings"
	"testing"

	"musubi/internal/memory"
)

// cadenaStore siembra un grafo con una CADENA de tres eslabones en archivos distintos:
//
//	raiz ──llama a──> medio ──llama a──> hoja
//
// Editar el archivo de hoja tiene 1 caller directo (medio) pero 2 en el cierre (medio y raiz). Es
// la diferencia que separa "miré las aristas entrantes" de "recorrí el grafo", y por eso la cadena
// tiene tres eslabones y no dos.
func cadenaStore() *fakeCodeStore {
	return &fakeCodeStore{
		mem: map[string]memory.CodeMemory{},
		graphNodes: map[string][]memory.GraphNode{
			"hoja.go": {{Key: "hoja.go#func:hoja", Kind: "func", Name: "hoja", Path: "hoja.go"}},
			"solo.go": {{Key: "solo.go#func:nadieMeLlama", Kind: "func", Name: "nadieMeLlama", Path: "solo.go"}},
		},
		inEdges: map[string][]memory.GraphEdge{
			"hoja.go#func:hoja":   {{FromKey: "medio.go#func:medio", ToKey: "hoja.go#func:hoja", Kind: "CALLS"}},
			"medio.go#func:medio": {{FromKey: "raiz.go#func:raiz", ToKey: "medio.go#func:medio", Kind: "CALLS"}},
		},
	}
}

// I1 — editar un archivo indexado devuelve el radio de impacto, no el mensaje de lectura.
func TestEditarDevuelveElRadioDeImpacto(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "hoja.go", "package p\nfunc hoja(){}\n")
	in := `{"tool_name":"Edit","tool_input":{"file_path":"hoja.go"},"session_id":"s"}`
	_, ctx := hookAdditionalContext(t, precheckOutput(cadenaStore(), root, strings.NewReader(in)))

	if !strings.Contains(ctx, "radio de impacto") {
		t.Fatalf("esperaba el radio de impacto, obtuve %q", ctx)
	}
	if strings.Contains(ctx, "grafo de código") {
		t.Errorf("editar no debe traer el mensaje de LECTURA (estructura del archivo): %q", ctx)
	}
	if !strings.Contains(ctx, "musubi_impact") {
		t.Errorf("el mensaje debe decir con qué pedir el cierre completo: %q", ctx)
	}
	// El símbolo va con su node_key entero, que es lo que musubi_impact exige como argumento:
	// nombrar la tool sin dar el argumento deja al agente adivinando el formato 'path#kind:name'.
	if !strings.Contains(ctx, "hoja.go#func:hoja") {
		t.Errorf("el mensaje debe traer el node_key listo para copiar: %q", ctx)
	}
}

// I2 — EL TEST QUE IMPORTA: el total sale del CIERRE TRANSITIVO, no de las aristas directas.
// Con la cadena raiz→medio→hoja, editar hoja.go da 1 directo y 2 en total.
func TestElTotalUsaElCierreTransitivoYNoLosDirectos(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "hoja.go", "package p\nfunc hoja(){}\n")
	in := `{"tool_name":"Edit","tool_input":{"file_path":"hoja.go"},"session_id":"s"}`
	_, ctx := hookAdditionalContext(t, precheckOutput(cadenaStore(), root, strings.NewReader(in)))

	if !strings.Contains(ctx, "1 directo(s), 1 fuera de tests · 2 en total") {
		t.Errorf("esperaba '1 directo(s), 1 fuera de tests · 2 en total' (raiz llega por medio); obtuve %q", ctx)
	}
}

// I8 — EL RANKING VA POR CALLERS DE PRODUCCIÓN, no por callers a secas.
//
// Medido en este repo: los tests dominan las listas (scoreCandidates tiene 9 callers y 8 son
// Test*). Rankear por el total pone arriba lo más TESTEADO en vez de lo más USADO, que es lo
// contrario de lo que hace falta antes de cambiar una firma: un test roto lo canta el compilador,
// un caller de producción roto no. Acá `pocoUsado` tiene 1 caller de producción contra los 4 de
// test de `muyTesteado`, y tiene que salir primero.
func TestElRankingPrefiereCallersDeProduccion(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "api.go", "package p\nfunc muyTesteado(){}\nfunc pocoUsado(){}\n")

	entrantes := func(destino string, origenes ...string) []memory.GraphEdge {
		var out []memory.GraphEdge
		for _, o := range origenes {
			out = append(out, memory.GraphEdge{FromKey: o, ToKey: destino, Kind: "CALLS"})
		}
		return out
	}
	store := &fakeCodeStore{
		mem: map[string]memory.CodeMemory{},
		graphNodes: map[string][]memory.GraphNode{
			"api.go": {
				{Key: "api.go#func:muyTesteado", Kind: "func", Name: "muyTesteado", Path: "api.go"},
				{Key: "api.go#func:pocoUsado", Kind: "func", Name: "pocoUsado", Path: "api.go"},
			},
		},
		inEdges: map[string][]memory.GraphEdge{
			"api.go#func:muyTesteado": entrantes("api.go#func:muyTesteado",
				"api_test.go#func:TestUno", "api_test.go#func:TestDos",
				"api_test.go#func:TestTres", "api_test.go#func:TestCuatro"),
			"api.go#func:pocoUsado": entrantes("api.go#func:pocoUsado", "server.go#func:Handler"),
		},
	}

	in := `{"tool_name":"Edit","tool_input":{"file_path":"api.go"},"session_id":"s"}`
	_, ctx := hookAdditionalContext(t, precheckOutput(store, root, strings.NewReader(in)))

	iProd := strings.Index(ctx, "pocoUsado ←")
	iTest := strings.Index(ctx, "muyTesteado ←")
	if iProd < 0 || iTest < 0 {
		t.Fatalf("esperaba los dos símbolos en el mensaje, obtuve %q", ctx)
	}
	if iProd > iTest {
		t.Errorf("el símbolo con caller de PRODUCCIÓN debe ir primero, no el más testeado: %q", ctx)
	}
	if !strings.Contains(ctx, "4 directo(s), 0 fuera de tests") {
		t.Errorf("los 4 callers de muyTesteado son todos de test y el mensaje debe decirlo: %q", ctx)
	}
}

// I3 — un archivo indexado SIN callers no calla: lo dice. "No hay riesgo" es información, y es
// distinta de "no sé", que es lo que significaría el silencio.
func TestArchivoAisladoLoDiceEnVezDeCallar(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "solo.go", "package p\nfunc nadieMeLlama(){}\n")
	in := `{"tool_name":"Write","tool_input":{"file_path":"solo.go"},"session_id":"s"}`
	out := precheckOutput(cadenaStore(), root, strings.NewReader(in))
	if out == "" {
		t.Fatal("un archivo indexado y aislado debe decir que no arrastra a nadie, no quedarse mudo")
	}
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "no arrastra a nadie") {
		t.Errorf("esperaba el aviso de archivo aislado, obtuve %q", ctx)
	}
}

// I4 — sin grafo, inerte. El hook corre en todos los repos, y la mayoría no indexó nada.
func TestSinGrafoElRadioDeImpactoNoDiceNada(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "otro.go", "package p\nfunc x(){}\n")
	in := `{"tool_name":"Edit","tool_input":{"file_path":"otro.go"},"session_id":"s"}`
	if out := precheckOutput(cadenaStore(), root, strings.NewReader(in)); out != "" {
		t.Errorf("un archivo fuera del grafo no debe producir salida, obtuve %q", out)
	}
}

// I5 — el radio de impacto NO está detrás de MUSUBI_CODEGRAPH_HOOK, y el de lectura sí.
//
// Son dos superficies con costo distinto: la de lectura vuelca la estructura entera del archivo en
// CADA Read (medido: 1.745 caracteres en un archivo real de este repo) y por eso nació opt-in; la
// de edición son tres líneas y dispara mucho menos seguido. Dejarla detrás del mismo flag la
// habría condenado a lo mismo que condenó a musubi_impact: existir apagada.
// (Actualizado el 2026-09-03: el flag de lectura pasó a venir ENCENDIDO, así que apagarlo ahora es
// explícito — "0". Lo que el test ata no cambió: son dos superficies con gobierno SEPARADO, y el
// radio de impacto no depende del flag de lectura en ninguna de sus dos posiciones.)
func TestElRadioDeImpactoNoDependeDelOptInDeLectura(t *testing.T) {
	t.Setenv("MUSUBI_CODEGRAPH_HOOK", "0")
	root := t.TempDir()
	writeFile(t, root, "hoja.go", "package p\nfunc hoja(){}\n")

	in := `{"tool_name":"Edit","tool_input":{"file_path":"hoja.go"},"session_id":"s"}`
	_, ctx := hookAdditionalContext(t, precheckOutput(cadenaStore(), root, strings.NewReader(in)))
	if !strings.Contains(ctx, "radio de impacto") {
		t.Errorf("el radio de impacto debe salir con el flag de LECTURA apagado, obtuve %q", ctx)
	}

	// Y el contraste: la misma config, leyendo, no inyecta la estructura.
	inRead := `{"tool_name":"Read","tool_input":{"file_path":"hoja.go"},"session_id":"s"}`
	if out := precheckOutput(cadenaStore(), root, strings.NewReader(inRead)); strings.Contains(out, "grafo de código") {
		t.Errorf("el flag de lectura gobierna la LECTURA y está apagado, obtuve %q", out)
	}
}

// I6 — lo que inyecta se contabiliza. Toda superficie de Musubi que gasta contexto se mide; una
// que no se mide es la que después nadie puede defender ni recortar.
func TestElRadioDeImpactoSeContabilizaEnElLedger(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "hoja.go", "package p\nfunc hoja(){}\n")
	store := cadenaStore()
	in := `{"tool_name":"Edit","tool_input":{"file_path":"hoja.go"},"session_id":"sesion-42"}`
	precheckOutput(store, root, strings.NewReader(in))

	if store.ledger["precheck_impacto"] <= 0 {
		t.Errorf("esperaba tokens registrados en 'precheck_impacto', obtuve %v", store.ledger)
	}
	if store.ledgerSession != "sesion-42" {
		t.Errorf("el ledger debe atribuirse a la sesión que lo gastó, obtuve %q", store.ledgerSession)
	}
}

// I7 — una tool que ni lee ni escribe sigue sin producir nada.
func TestUnaToolQueNiLeeNiEscribeSigueMuda(t *testing.T) {
	root := t.TempDir()
	in := `{"tool_name":"Bash","tool_input":{"command":"ls"},"session_id":"s"}`
	if out := precheckOutput(cadenaStore(), root, strings.NewReader(in)); out != "" {
		t.Errorf("Bash no debe producir salida, obtuve %q", out)
	}
}
