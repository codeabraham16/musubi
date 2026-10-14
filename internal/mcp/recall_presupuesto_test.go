package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// TestRecallElPresupuestoEsTechoDeLaRespuesta: lo que recibe el agente por musubi_recall entra en
// token_budget —el gist Y el sobre de cada item—, y bajar el presupuesto sólo corta la cola.
//
// La vara es independiente del estimador: a lo sumo 4 caracteres por token del presupuesto, que es
// el divisor más generoso que usa. Si el presupuesto volviera a contar sólo el gist, la respuesta
// pesaría ~13 caracteres por token y esto se pone en rojo.
func TestRecallElPresupuestoEsTechoDeLaRespuesta(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	for i := 0; i < 40; i++ {
		if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{
			"topic_key": fmt.Sprintf("presupuesto/nota-%02d", i),
			"content":   fmt.Sprintf("Nota %d sobre el presupuesto del recall: el sobre de cada item también pesa (variante %d).", i, i*7),
		}); e != nil {
			t.Fatalf("save %d: %+v", i, e)
		}
	}
	recall := func(budget int) (string, memory.RecallResult) {
		res, e := call(t, s, "musubi_recall", map[string]interface{}{"query": "presupuesto del recall", "token_budget": budget})
		if e != nil {
			t.Fatalf("recall @%d: %+v", budget, e)
		}
		txt := textOf(t, res)
		var r memory.RecallResult
		if err := json.Unmarshal([]byte(txt), &r); err != nil {
			t.Fatalf("respuesta ilegible @%d: %v\n%s", budget, err, txt)
		}
		return txt, r
	}

	// La referencia sale del motor con NoBump y ANTES de la tool: cada recall por la tool cuenta un
	// acceso a lo que devolvió, y eso mueve el ranking por frecuencia de la llamada siguiente. Medido:
	// con dos llamadas a la tool, una de cada tres corridas comparaba el bump y no el corte.
	const budget = 300
	ctx := context.Background()
	opts := s.opcionesDeRecall(ctx, 8000)
	opts.NoBump = true
	consulta, _ := s.corregirConsulta(ctx, "presupuesto del recall", opts)
	grande, err := s.engine.Recall(ctx, consulta, opts)
	if err != nil {
		t.Fatalf("recall de referencia: %v", err)
	}
	txt, chico := recall(budget)

	if n := utf8.RuneCountInString(txt); n > 4*budget {
		t.Errorf("con token_budget=%d la respuesta pesa %d caracteres (más de %d): el sobre de los items viaja gratis", budget, n, 4*budget)
	}
	if chico.UsedTokens > budget {
		t.Errorf("used_tokens=%d pasa token_budget=%d", chico.UsedTokens, budget)
	}
	if len(chico.Items) < 2 || len(chico.Items) >= len(grande.Items) {
		t.Fatalf("el corte no se ve: %d items con %d tokens, %d con 8000", len(chico.Items), budget, len(grande.Items))
	}
	for i, it := range chico.Items {
		if it.ID != grande.Items[i].ID {
			t.Fatalf("posición %d: %s con %d tokens y %s con 8000 — bajar el presupuesto sólo puede cortar la cola", i, it.ID, budget, grande.Items[i].ID)
		}
	}
}

// TestConJuezElCorteVaDespues: con rerank, el juez ve la cabeza del empaquetado de siempre —hasta
// sus 12 candidatos— aunque el token_budget sea chico, y el corte al techo de la respuesta viene
// después. Si el corte fuera antes, el juez vería sólo lo que entra en el presupuesto y podría
// subir al primer lugar un item distinto del que subía antes.
func TestConJuezElCorteVaDespues(t *testing.T) {
	motor := &motorEspia{respuesta: `[]`}
	s := servidorConDial(t, motor, false) // dial apagado: el juez corre sólo porque la llamada lo pide
	for i := 0; i < 30; i++ {
		if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{
			"topic_key": fmt.Sprintf("juez/nota-%02d", i),
			"content":   fmt.Sprintf("Nota %d sobre el juez del recall y el corte de la cola (variante %d).", i, i*7),
		}); e != nil {
			t.Fatalf("save %d: %+v", i, e)
		}
	}
	const budget = 200
	ctx := context.Background()
	opts := s.opcionesDeRecall(ctx, budget)
	opts.PresupuestoDeRespuesta = false
	opts.NoBump = true
	consulta, _ := s.corregirConsulta(ctx, "juez del recall", opts)
	deSiempre, err := s.engine.Recall(ctx, consulta, opts)
	if err != nil {
		t.Fatalf("recall de referencia: %v", err)
	}
	cabeza := defaultRerankTopK
	if len(deSiempre.Items) < cabeza {
		cabeza = len(deSiempre.Items)
	}

	res, e := call(t, s, "musubi_recall", map[string]interface{}{"query": "juez del recall", "token_budget": budget, "rerank": true})
	if e != nil {
		t.Fatalf("recall: %+v", e)
	}
	var r memory.RecallResult
	if err := json.Unmarshal([]byte(textOf(t, res)), &r); err != nil {
		t.Fatalf("respuesta ilegible: %v", err)
	}
	u := motor.user.Load()
	if u == nil {
		t.Fatal("el juez no corrió: la prueba no mide nada")
	}
	vistos := 0
	for _, it := range deSiempre.Items[:cabeza] {
		if strings.Contains(*u, "["+it.ID+"]") {
			vistos++
		}
	}
	if vistos != cabeza {
		t.Errorf("el juez vio %d de los %d candidatos de la cabeza de siempre: el corte fue antes del juez", vistos, cabeza)
	}
	if len(r.Items) >= cabeza {
		t.Fatalf("con token_budget=%d viajaron %d items de una cabeza de %d: el corte no se ve", budget, len(r.Items), cabeza)
	}
	if len(r.Items) > 1 && r.UsedTokens > budget {
		t.Errorf("used_tokens=%d pasa token_budget=%d", r.UsedTokens, budget)
	}
}
