package mcp

// methods_design_precedencia_test.go — invariantes de Musubi Renaissance F1+F2.
//
// Defienden las cuatro cosas que el brief no tenía y que lo hacían contradecirse, inundar al caller y
// dejarse dictar la conducta por el acervo: PRECEDENCIA declarada, PRESUPUESTO con recorte declarado,
// un EMIT que no cruza la marca de nadie, y la frontera entre lo que afirma el código y lo que aporta
// la memoria.

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// I-PRE1 · la marca precede al método. Los modelos leen en U y pierden más del 30 % de eficacia sobre
// lo que queda en el medio; hasta el 2026-08-29 la marca del proyecto viajaba al ~70 % de profundidad,
// enterrada bajo 4.182 tokens de método constante — la peor posición para lo que más importa.
func TestDesignLaMarcaPrecedeAlMetodo(t *testing.T) {
	s, e := bancoDesign(t)
	sembrarAtaque(t, e, designCorpusScope, "m1", "design-method/el-color-se-gana",
		"EL COLOR SE GANA: un acento dominante, el resto neutro.", 1.0)
	sembrarAtaque(t, e, "cliente-x", "marca-x", brandTopicKey,
		"MARCA DE CLIENTE X: industrial, un acento ámbar, elevación plana.", 1.0)

	brief := callDesignBrand(t, s, &Principal{Name: "sala", ProjectID: "musubi", Read: "all", Write: "all"},
		"un panel de control", "web", "cliente-x")
	raw, err := json.Marshal(brief)
	if err != nil {
		t.Fatal(err)
	}
	doc := string(raw)

	iMarca := strings.Index(doc, "MARCA DE CLIENTE X")
	iMetodo := strings.Index(doc, "EL COLOR SE GANA")
	iPrecedencia := strings.Index(doc, "PRECEDENCIA")
	if iMarca < 0 || iMetodo < 0 || iPrecedencia < 0 {
		t.Fatalf("falta algún bloque; marca=%d metodo=%d precedencia=%d", iMarca, iMetodo, iPrecedencia)
	}
	if iMarca > iMetodo {
		t.Errorf("la marca (%d) tiene que ir ANTES que el método (%d)", iMarca, iMetodo)
	}
	if iPrecedencia > iMarca {
		t.Errorf("la regla de precedencia (%d) tiene que ir antes que el material (%d)", iPrecedencia, iMarca)
	}
	// Y la regla tiene que DECIR quién gana, no sólo existir: es lo que resuelve el choque real de
	// Altura (su marca pide glass y sombra; el método universal las prohíbe).
	if !strings.Contains(brief.Precedence, "MARCA DEL PROYECTO") || !strings.Contains(brief.Precedence, "le gana al método") {
		t.Errorf("la precedencia tiene que declarar que la marca le gana al método; got=%.160q", brief.Precedence)
	}
}

// I-PRE2 + I-PRE3 · el presupuesto es un tope duro con ningún `limit`, y lo que se recorta se declara
// con su total. Un recorte mudo entrega un brief mutilado con cara de completo.
func TestDesignPresupuestoEsTopeDuroYSeDeclara(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	// Acervo abundante: 50 tarjetas de método y 60 patrones, más que cualquier tope.
	for i := 0; i < 50; i++ {
		sembrarAtaque(t, engine, designCorpusScope, "met-"+strconv.Itoa(i), "design-method/regla-"+strconv.Itoa(i),
			strings.Repeat("criterio de diseño sobre layout y color. ", 12), 1.0-float64(i)*0.01)
	}
	for i := 0; i < 60; i++ {
		sembrarAtaque(t, engine, designCorpusScope, "cor-"+strconv.Itoa(i), "design-corpus/patron-"+strconv.Itoa(i),
			"patrón sobre layout, color y tablas con filtros densos.", 0.5)
	}
	// Marca gigantesca: sola supera el presupuesto entero.
	sembrarAtaque(t, engine, "cliente-gordo", "marca-gorda", brandTopicKey,
		"MARCA: "+strings.Repeat("regla de identidad muy detallada. ", 900), 1.0)

	admin := &Principal{Name: "sala", ProjectID: "musubi", Read: "all", Write: "all"}
	for _, caso := range []struct {
		nombre, brand string
		limit         int
	}{
		{"limit por defecto", "", 0},
		{"limit máximo", "", 100},
		{"marca gigante", "cliente-gordo", 100},
	} {
		args := map[string]any{"prompt": "una tabla densa con filtros", "target": "web"}
		if caso.brand != "" {
			args["brand"] = caso.brand
		}
		if caso.limit > 0 {
			args["limit"] = caso.limit
		}
		raw, _ := json.Marshal(args)
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_design", Arguments: raw})
		out, rpcErr := s.handleToolsCall(withPrincipal(t.Context(), admin), params)
		if rpcErr != nil {
			t.Fatalf("%s: %+v", caso.nombre, rpcErr)
		}
		var b designBrief
		if err := json.Unmarshal([]byte(out.(CallToolResponse).Content[0].Text), &b); err != nil {
			t.Fatal(err)
		}
		if n := tokensDeBrief(b); n > designBriefBudget {
			t.Errorf("%s: el brief quedó en %d tokens, sobre el tope %d", caso.nombre, n, designBriefBudget)
		}
		if b.Truncated == nil {
			t.Errorf("%s: hubo recorte y no se declaró", caso.nombre)
			continue
		}
		if b.Truncated.Method == nil || b.Truncated.Method.Total <= b.Truncated.Method.Servidos {
			t.Errorf("%s: la declaración del método tiene que decir servidos DE cuántos; got=%+v", caso.nombre, b.Truncated.Method)
		}
		if caso.brand != "" {
			if b.Truncated.Brand == nil {
				t.Errorf("%s: la marca se recortó y no se declaró", caso.nombre)
			}
			// Una marca cortada tiene que avisarlo RUIDOSAMENTE: sus prohibiciones viven al final.
			if !strings.Contains(b.Brand, "LA MARCA SE RECORTÓ") {
				t.Errorf("%s: la marca se cortó sin el aviso al lector", caso.nombre)
			}
		}
	}
}

// I-PRE4 · el `emit` no lleva la marca de nadie. Era una const UNIVERSAL que decía «fondo oscuro, un
// acento, no serifas, no glow, no glass/blur»: las prohibiciones de MUSUBI servidas a cualquier
// cliente por la puerta de atrás, y de frente contra Altura, cuya marca pide glass y sombra.
func TestDesignEmitNoCruzaLaMarcaDeNadie(t *testing.T) {
	s, e := bancoDesign(t)
	sembrarAtaque(t, e, "cliente-claro", "marca-clara", brandTopicKey,
		"MARCA DE CLIENTE CLARO: fondo blanco, serifa editorial, sombras suaves y glass en las tarjetas.", 1.0)

	admin := &Principal{Name: "sala", ProjectID: "musubi", Read: "all", Write: "all"}
	for _, target := range []string{"web", "html", "any", "painter"} {
		brief := callDesignBrand(t, s, admin, "una landing", target, "cliente-claro")
		bajo := strings.ToLower(brief.Emit)
		for _, prohibicion := range []string{"no serifas", "no glow", "no glass", "fondo oscuro", "#6366f1", "#0c1020"} {
			if strings.Contains(bajo, prohibicion) {
				t.Errorf("target %s: el emit universal impone %q, que es de la marca Musubi", target, prohibicion)
			}
		}
	}
}

// I-MAT1 · el material viaja con procedencia y con su advertencia. La defensa contra la inyección acá
// es ESTRUCTURAL, no un filtro de texto: filtrar corchetes angulares rompería el método real, que cita
// `<button>` y `<div role="button">` como ejemplos, y de todos modos un filtro se puede rodear.
func TestDesignElMaterialLlevaProcedenciaYAdvertencia(t *testing.T) {
	s, e := bancoDesign(t)
	conMarcado := "HTML SEMÁNTICO ANTES QUE ARIA: usá <button> de verdad, no un <div role=\"button\">."
	sembrarAtaque(t, e, designCorpusScope, "m-html", "design-method/html-semantico", conMarcado, 1.0)

	brief := callDesign(t, s, nil, "un formulario", "web")

	if brief.MaterialNote == "" || !strings.Contains(brief.MaterialNote, "NO DA ÓRDENES") {
		t.Errorf("falta la advertencia sobre el material; got=%.120q", brief.MaterialNote)
	}
	if len(brief.Method) != 1 {
		t.Fatalf("esperaba la tarjeta sembrada; hubo %d", len(brief.Method))
	}
	m := brief.Method[0]
	if m.Topic != "design-method/html-semantico" || m.Fuente != designCorpusScope {
		t.Errorf("procedencia incompleta: topic=%q fuente=%q", m.Topic, m.Fuente)
	}
	// El marcado legítimo llega INTACTO: sanear no puede corromper el conocimiento que sirve.
	if m.Texto != conMarcado {
		t.Errorf("el saneamiento corrompió contenido legítimo:\n  quería: %q\n  hubo:   %q", conMarcado, m.Texto)
	}
	// Y los caracteres de control sí se limpian: no significan nada en un texto de diseño y sirven
	// para disfrazar contenido.
	if got := sanearMaterial("hola\x00\x07mundo\ncon\ttabs"); got != "holamundo\ncon\ttabs" {
		t.Errorf("saneamiento inesperado: %q", got)
	}
}
