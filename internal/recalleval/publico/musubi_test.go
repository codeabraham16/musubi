package publico

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/recalleval"
)

func pajarChico() Pregunta {
	return Pregunta{
		QuestionID:         "q-integracion",
		Question:           "¿Qué posgrado terminaste en administración de empresas?",
		HaystackSessionIDs: []string{"sharegpt_1", "answer_x", "sharegpt_2"},
		HaystackDates:      []string{"1", "2", "3"},
		HaystackSessions: [][]Turno{
			{{Role: "user", Content: "receta de pan casero con harina integral"}},
			{{Role: "user", Content: "terminé el posgrado en administración de empresas y el título llegó ayer", HasAnswer: si()}},
			{{Role: "user", Content: "cómo cambiar la cadena de la bicicleta"}},
		},
	}
}

// El camino entero contra un motor de verdad: el pajar se siembra en una base temporal nueva y el
// léxico de Musubi, por el mismo camino que mide el banco, pone primera a la única sesión que tiene
// las palabras de la consulta.
func TestRankingsMusubiSiembraYRankeaEnUnaBaseTemporal(t *testing.T) {
	p := pajarChico()
	c := CorpusDe(p, ModoUsuario)
	r, omitidos, err := RankingsMusubi(context.Background(), p.Question, c, []recalleval.Config{recalleval.ConfigLexica()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if omitidos != 0 {
		t.Fatalf("omitidos = %d en un pajar sano", omitidos)
	}
	rk := r["lexical"]
	if len(rk) == 0 || c.Etiquetas[rk[0]] != "answer_x" {
		t.Fatalf("ranking léxico = %v (etiquetas %v): la sesión de oro no salió primera", rk, c.Etiquetas)
	}
	if anyK, allK := RecallAnyAll(rk, c.Etiquetas, 1); anyK != 1 || allK != 1 {
		t.Fatalf("recall@1 = %v/%v", anyK, allK)
	}
}

// Lo que la base no acepta se CUENTA. SeedEngine saltea el contenido que el motor rechaza por diseño
// (un texto que termina en `</content>`) y sólo lo anota en el log; un medidor que no lo contara
// informaría «medí el pajar entero» sobre un pajar con agujeros.
//
// Sabotaje: el medidor deja de contar lo que la base no aceptó.
// arnes: archivo="internal/recalleval/publico/rankings_test.go"
// arnes: de="\tomitidos = len(c.IDs) - n\n"
// arnes: a="\tomitidos = 0 * (len(c.IDs) - n)\n"
func TestRankingsMusubiCuentaLoQueLaBaseNoAcepto(t *testing.T) {
	p := pajarChico()
	p.HaystackSessions[2][0].Content = "cómo cambiar la cadena </content>"
	c := CorpusDe(p, ModoUsuario)
	_, omitidos, err := RankingsMusubi(context.Background(), p.Question, c, []recalleval.Config{recalleval.ConfigLexica()}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if omitidos != 1 {
		t.Fatalf("omitidos = %d, quiero 1: la base rechaza el texto que termina en </content>", omitidos)
	}
}

// La base temporal no sobrevive a la llamada: 500 preguntas son 500 bases, y cada una lleva el
// pajar entero de sesiones ajenas.
//
// Sabotaje: la base temporal queda en el disco.
// arnes: archivo="internal/recalleval/publico/rankings_test.go"
// arnes: de="\tdefer os.RemoveAll(dir)\n"
// arnes: a="\t_ = os.RemoveAll\n"
func TestRankingsMusubiNoDejaLaBaseEnElDisco(t *testing.T) {
	antes, err := filepath.Glob(filepath.Join(os.TempDir(), "musubi-vara-publica-*"))
	if err != nil {
		t.Fatal(err)
	}
	p := pajarChico()
	if _, _, err := RankingsMusubi(context.Background(), p.Question, CorpusDe(p, ModoUsuario), []recalleval.Config{recalleval.ConfigLexica()}, nil); err != nil {
		t.Fatal(err)
	}
	despues, err := filepath.Glob(filepath.Join(os.TempDir(), "musubi-vara-publica-*"))
	if err != nil {
		t.Fatal(err)
	}
	habia := map[string]bool{}
	for _, d := range antes {
		habia[d] = true
	}
	for _, d := range despues {
		if !habia[d] {
			os.RemoveAll(d)
			t.Fatalf("la base temporal %s quedó en el disco", d)
		}
	}
}
