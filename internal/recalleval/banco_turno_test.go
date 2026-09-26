package recalleval

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// Pruebas del banco que corre el ranker del HOOK por turno: las opciones de la fuente única
// (memory.OpcionesDeRecallDelTurno) y el pool del turno en vez del corpus entero. La perturbación
// por clase de tipeo, con la que se va a medir el corrector, vive en perturbar_test.go.

// TestConfigDelBancoEsLaDelHook: ConfigTurno corre, campo a campo, las opciones que arma la fuente
// única para la config por defecto, y con el pool del turno. La contracara del lado del hook es
// TestBuildTurnRecallUsaLasOpcionesDelTurno (cmd/musubi): las dos juntas dicen que el banco y el
// hook corren el mismo ranker, porque los dos son iguales a la misma función.
//
// Sabotaje: el brazo del turno se arma con las opciones de la tool musubi_recall.
// arnes: archivo="internal/recalleval/configs.go"
// arnes: de="\t\tOpts:         memory.OpcionesDeRecallDelTurno(config.Default().Memory, memory.AlcanceDelTurno{}),"
// arnes: a="\t\tOpts:         OptsDeProduccion(),"
//
// Sabotaje: el brazo del turno vuelve a correr con el pool = corpus.
// arnes: archivo="internal/recalleval/configs.go"
// arnes: de="\t\tPoolDelTurno: true,"
// arnes: a="\t\tPoolDelTurno: false,"
func TestConfigDelBancoEsLaDelHook(t *testing.T) {
	c := ConfigTurno()
	quiero := memory.OpcionesDeRecallDelTurno(config.Default().Memory, memory.AlcanceDelTurno{})
	if !reflect.DeepEqual(c.Opts, quiero) {
		t.Errorf("ConfigTurno no mide las opciones del hook:\n  banco %+v\n  hook  %+v", c.Opts, quiero)
	}
	if !c.PoolDelTurno {
		t.Error("ConfigTurno corre con pool = corpus: mide un ranker de miles de candidatos, y el hook rankea 50")
	}
	// El hook de hoy corre sin vector (la guarda de latencia no le deja construir el embebedor).
	if c.UseVector {
		t.Error("ConfigTurno enciende el vector: el hook de hoy es sólo léxico, el híbrido es otro brazo")
	}
}

// recuperadorQueAnota registra las opciones de cada Recall y no devuelve nada.
type recuperadorQueAnota struct{ opts []memory.RecallOptions }

func (r *recuperadorQueAnota) Recall(_ context.Context, _ string, o memory.RecallOptions) (memory.RecallResult, error) {
	r.opts = append(r.opts, o)
	return memory.RecallResult{}, nil
}

// TestElBancoCorreElPoolDelTurno: en el modo PoolDelTurno, Evaluate le pide a Recall el pool de las
// opciones (50, el del hook) aunque el corpus tenga miles de docs. El defecto que cierra: rankedIDs
// subía SIEMPRE el pool al corpus entero, así que un brazo «del turno» medía un pool vectorial de
// hasta 3.155 candidatos cuando el hook trae 50.
//
// Sabotaje: reponer el override del pool en rankedIDs.
// arnes: archivo="internal/recalleval/harness.go"
// arnes: de="if !cfg.PoolDelTurno && opts.CandidatePool < pool {"
// arnes: a="if opts.CandidatePool < pool {"
func TestElBancoCorreElPoolDelTurno(t *testing.T) {
	fx := &Fixture{Queries: []Query{{ID: "q", Text: "el fichaje del kiosko", Relevant: []string{"d0"}}}}
	for i := 0; i < 3155; i++ {
		fx.Docs = append(fx.Docs, Doc{ID: fmt.Sprintf("d%d", i)})
	}
	ks := []int{10}
	ctx := context.Background()
	hook := memory.OpcionesDeRecallDelTurno(config.Default().Memory, memory.AlcanceDelTurno{}).CandidatePool

	hibrido := ConfigTurno()
	hibrido.Name, hibrido.UseVector = "turno-hibrido", true
	for _, cfg := range []Config{ConfigTurno(), hibrido} {
		r := &recuperadorQueAnota{}
		if _, err := Evaluate(ctx, r, fx, cfg, hashEmbed, ks); err != nil {
			t.Fatalf("%s: %v", cfg.Name, err)
		}
		if len(r.opts) != 1 {
			t.Fatalf("%s: esperaba 1 recall, hubo %d", cfg.Name, len(r.opts))
		}
		if got := r.opts[0].CandidatePool; got != hook || hook != 50 {
			t.Errorf("%s: el banco le pidió un pool de %d y el hook rankea %d (esperaba 50)", cfg.Name, got, hook)
		}
	}

	// CONTROL: fuera del modo del turno el banco sigue subiendo el pool al corpus (el histórico). Si
	// esto no se viera, la prueba de arriba pasaría aunque el fake no estuviera mirando nada.
	r := &recuperadorQueAnota{}
	if _, err := Evaluate(ctx, r, fx, ConfigLexica(), nil, ks); err != nil {
		t.Fatal(err)
	}
	if got := r.opts[0].CandidatePool; got != len(fx.Docs) {
		t.Errorf("control: fuera del modo del turno el pool tenía que ser el corpus (%d), fue %d", len(fx.Docs), got)
	}
}
