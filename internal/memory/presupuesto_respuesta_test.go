package memory

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

// rankingDePrueba arma n candidatos ya rankeados, con gists de largo variable y el sobre completo
// (fecha, proyecto, score con la precisión entera de un RRF).
func rankingDePrueba(n int) []scoredCandidate {
	out := make([]scoredCandidate, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, scoredCandidate{
			candidate: candidate{
				id:         fmt.Sprintf("3f2a9c1e-0000-4000-8000-%012d", i),
				topicKey:   fmt.Sprintf("gotcha/presupuesto-%02d", i),
				gist:       "Nota de prueba sobre el presupuesto del recall " + strings.Repeat("y su sobre ", 1+i%5),
				fullTokens: 120 + i,
				createdAt:  "2026-10-14 12:34:56",
				projectID:  "musubi",
			},
			score: 1.0 / float64(61+i) * 1.7,
		})
	}
	return out
}

// TestAlTechoDeLaRespuestaCortaSoloLaCola fija las promesas del techo de la respuesta: la
// respuesta serializada entra en holguraDeLaRespuesta veces el presupuesto (a lo sumo 4 caracteres
// por token, el divisor más generoso del estimador), y con material de sobra pasa de 1 vez, así
// que la holgura medida está; lo que queda es un PREFIJO del empaquetado de siempre, y el score
// viaja con 3 cifras significativas.
func TestAlTechoDeLaRespuestaCortaSoloLaCola(t *testing.T) {
	const budget = 1500
	deSiempre := packByBudget(rankingDePrueba(80), budget, defaultGistMaxTokens, reparto{})
	r := AlTechoDeLaRespuesta(deSiempre, budget)

	if len(r.Items) < 2 || len(r.Items) >= len(deSiempre.Items) {
		t.Fatalf("el corte no se ve: %d items contra %d del empaquetado de siempre", len(r.Items), len(deSiempre.Items))
	}
	for i, it := range r.Items {
		if it.ID != deSiempre.Items[i].ID {
			t.Fatalf("posición %d: %s, y el empaquetado de siempre traía %s — tiene que ser un prefijo", i, it.ID, deSiempre.Items[i].ID)
		}
	}
	if r.Count != len(r.Items) {
		t.Errorf("count=%d con %d items", r.Count, len(r.Items))
	}
	techo := holguraDeLaRespuesta * budget
	if r.UsedTokens > techo {
		t.Errorf("used_tokens=%d pasa el techo %d", r.UsedTokens, techo)
	}
	if r.UsedTokens <= budget {
		t.Errorf("used_tokens=%d no pasa el presupuesto %d con material de sobra: la holgura medida no está", r.UsedTokens, budget)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if n := utf8.RuneCount(b); n > 4*techo {
		t.Errorf("la respuesta serializada pesa %d caracteres: más de 4 por token del techo (%d); el sobre de los items viaja gratis", n, 4*techo)
	}
	for _, it := range r.Items {
		digitos := strings.TrimLeft(strings.Replace(strconv.FormatFloat(it.Score, 'g', -1, 64), "0.", "", 1), "0")
		if len(digitos) > 3 {
			t.Errorf("score %v viaja con más de 3 cifras significativas", it.Score)
		}
	}
}

// TestAlTechoDeLaRespuestaGarantizaElPrimero: un techo menor que un item devuelve igual el mejor,
// como el empaquetado de siempre, y used_tokens dice lo que pesó de verdad.
func TestAlTechoDeLaRespuestaGarantizaElPrimero(t *testing.T) {
	const budget = 10
	deSiempre := packByBudget(rankingDePrueba(5), budget, defaultGistMaxTokens, reparto{})
	r := AlTechoDeLaRespuesta(deSiempre, budget)
	if len(r.Items) != 1 || r.Items[0].ID != deSiempre.Items[0].ID {
		t.Fatalf("esperaba sólo el primero (%s), obtuve %+v", deSiempre.Items[0].ID, r.Items)
	}
	if techo := holguraDeLaRespuesta * budget; r.UsedTokens <= techo {
		t.Errorf("used_tokens=%d: un item entero no entra en %d, tiene que decir cuánto pesó", r.UsedTokens, techo)
	}
}
