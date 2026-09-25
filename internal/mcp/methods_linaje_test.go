package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// servidorConLinaje arma un servidor sobre una base con una ficha que salió de un blob y una nota
// suelta. Devuelve el engine para que cada prueba pueda envolverlo.
func servidorConLinaje(t *testing.T) *memory.DbEngine {
	t.Helper()
	engine := memtest.NuevoEngine(t, t.TempDir())
	for _, o := range [][2]string{
		{"ficha-lin", "design-corpus/contraste"},
		{"blob-lin", "ingested/web/articulo"},
		{"nota-lin", "notas/suelta"},
	} {
		if err := engine.SaveObservationTypedFrom(designCorpusScope, "", o[0], o[1], "contenido de la prueba de linaje", 1.0, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := engine.UpsertObsRelation(memory.ObsRelation{SourceID: "ficha-lin", TargetID: "blob-lin",
		Relation: memory.RelDerivedFrom, Status: memory.RelStatusResolved}); err != nil {
		t.Fatal(err)
	}
	return engine
}

func textoDeExpand(t *testing.T, s *McpServer, ids ...string) string {
	t.Helper()
	res, e := call(t, s, "musubi_memory_expand", map[string]interface{}{"ids": ids})
	if e != nil {
		t.Fatalf("expandir %v: %+v", ids, e)
	}
	return res.(CallToolResponse).Content[0].Text
}

// TestMemoryExpandTraeElLinaje: expandir una ficha trae de dónde salió, expandir el blob trae qué
// salió de él, y una nota sin aristas sale con los mismos cuatro campos de siempre AUNQUE viaje en la
// misma respuesta que una ficha con linaje. Es la compatibilidad con los que ya decodifican la
// respuesta: el cuerpo, el gateway y los agentes.
//
// Sabotaje que la hace fallar: la tool devuelve la hidratación sin el linaje.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="jsonResult(s.conLinaje(sctx, res))"
// arnes: a="jsonResult(res)"
//
// Sabotaje que la hace fallar: salio_de sin omitempty, que le agrega un campo nulo a toda nota.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="`json:\"salio_de,omitempty\"`"
// arnes: a="`json:\"salio_de\"`"
//
// Sabotaje que la hace fallar: destilado_en sin omitempty.
// arnes: archivo="internal/memory/linaje.go"
// arnes: de="`json:\"destilado_en,omitempty\"`"
// arnes: a="`json:\"destilado_en\"`"
func TestMemoryExpandTraeElLinaje(t *testing.T) {
	s := NewMcpServer(servidorConLinaje(t), t.TempDir(), embedding.NoopProvider{})

	ida := textoDeExpand(t, s, "ficha-lin")
	if !strings.Contains(ida, `"salio_de"`) || !strings.Contains(ida, "blob-lin") || !strings.Contains(ida, "ingested/web/articulo") {
		t.Errorf("la ficha tiene que traer salio_de con el id y el topic del blob: %s", ida)
	}
	vuelta := textoDeExpand(t, s, "blob-lin")
	if !strings.Contains(vuelta, `"destilado_en"`) || !strings.Contains(vuelta, "ficha-lin") {
		t.Errorf("el blob tiene que traer destilado_en con el id de la ficha: %s", vuelta)
	}

	// La nota viaja junto a la ficha a propósito: sola, la respuesta ni siquiera pasaría por el tipo
	// con linaje, y un campo nulo de más no se vería.
	var items []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(textoDeExpand(t, s, "ficha-lin", "nota-lin")), &items); err != nil {
		t.Fatalf("la respuesta tiene que seguir siendo un array: %v", err)
	}
	var nota map[string]json.RawMessage
	for _, it := range items {
		if string(it["id"]) == `"nota-lin"` {
			nota = it
		}
	}
	if nota == nil {
		t.Fatalf("la nota no volvió en la expansión: %v", items)
	}
	var claves []string
	for k := range nota {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	if got := strings.Join(claves, ","); got != "content,created_at,id,topic_key" {
		t.Errorf("una nota sin linaje tiene que salir con los cuatro campos de siempre; salió con %s", got)
	}
}

// linajeQueFalla es un backend real al que sólo se le rompe el linaje.
type linajeQueFalla struct{ memory.StorageBackend }

func (linajeQueFalla) LinajeCtx(context.Context, []string) (map[string]memory.Linaje, error) {
	return nil, errors.New("linaje roto a propósito")
}

// TestLaExpansionNoCaePorElLinaje: si el linaje falla, la expansión entrega el contenido igual. El
// linaje es un dato accesorio y el contenido ya está calculado: perderlo por el accesorio sería el
// peor intercambio posible.
//
// Sabotaje que la hace fallar: ante el error, conLinaje no devuelve lo hidratado.
// arnes: archivo="internal/mcp/methods_linaje.go"
// arnes: de="\t\treturn obs\n\t}\n\tif len(lin) == 0 {"
// arnes: a="\t\treturn nil\n\t}\n\tif len(lin) == 0 {"
func TestLaExpansionNoCaePorElLinaje(t *testing.T) {
	s := NewMcpServer(linajeQueFalla{servidorConLinaje(t)}, t.TempDir(), embedding.NoopProvider{})
	got := textoDeExpand(t, s, "ficha-lin")
	if !strings.Contains(got, "contenido de la prueba de linaje") {
		t.Errorf("con el linaje roto, la expansión tiene que traer el contenido igual: %s", got)
	}
}

// TestDesignTraeLasFuentesDeCadaFicha: el corpus de musubi_design dice de qué artículo salió cada
// ficha. Es donde el agente ve la ficha, y por eso es lo que puede encender el linaje: medido el
// 2026-09-24, nadie expande ids del acervo, así que colgarlo sólo de expand lo dejaba latente.
//
// Sabotaje que la hace fallar: el brief no pide las fuentes.
// arnes: archivo="internal/mcp/methods_design.go"
// arnes: de="s.adjuntarFuentes(corpusCtx, rec.Patrones)"
// arnes: a="s.adjuntarFuentes(corpusCtx, nil)"
func TestDesignTraeLasFuentesDeCadaFicha(t *testing.T) {
	s := acervoDePatrones(t, map[string]string{
		"design-corpus/tabla-densa": "Para una tabla densa, la primera columna ancla la lectura y las numéricas van a la derecha.",
	})
	// acervoDePatrones numera los ids desde «pata»; con una sola entrada, la ficha es «pata».
	if err := s.engine.SaveObservationTypedFrom(designCorpusScope, "", "fuente-de-pata", "ingested/web/manual",
		"Un manual de otra cosa: kerning y ligaduras.", 1.0, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.engine.UpsertObsRelation(memory.ObsRelation{SourceID: "pata", TargetID: "fuente-de-pata",
		Relation: memory.RelDerivedFrom, Status: memory.RelStatusResolved}); err != nil {
		t.Fatal(err)
	}

	b := callDesign(t, s, nil, "tabla densa", "web")
	var ficha *patronItem
	for i := range b.Corpus {
		if b.Corpus[i].ID == "pata" {
			ficha = &b.Corpus[i]
		}
	}
	if ficha == nil {
		t.Fatalf("control: la ficha sembrada no salió en el corpus, así que no hay dónde mirar las fuentes: %+v", b.Corpus)
	}
	if strings.Join(ficha.Fuentes, ",") != "fuente-de-pata" {
		t.Errorf("la ficha tiene que decir de qué artículo salió; fuentes=%v", ficha.Fuentes)
	}
}
