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
// arnes: de="\t\tOpts:         memory.OpcionesDeRecallDelTurno(m, memory.AlcanceDelTurno{}),"
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
	// El hook de hoy corre sin embebedor (la guarda de latencia no le deja construirlo): ni vector de
	// consulta ni procedencia de vectores en el motor.
	if c.UseVector || !c.SinEmbebedor {
		t.Errorf("ConfigTurno: UseVector=%v SinEmbebedor=%v; el hook de hoy no tiene embebedor, el híbrido es otro brazo",
			c.UseVector, c.SinEmbebedor)
	}
	// Y el brazo del vector en el turno es el mismo hook con el embebedor construido.
	h := ConfigTurnoHibrido()
	if !reflect.DeepEqual(h.Opts, quiero) || !h.PoolDelTurno || !h.UseVector || h.SinEmbebedor {
		t.Errorf("ConfigTurnoHibrido no es el hook con embebedor: %+v", h)
	}
}

// TestElBrazoDelTurnoTraduceElYaml fija con valores LITERALES cómo el brazo del turno traduce un
// yaml que no es el de fábrica. Las otras pruebas comparan el banco y el hook contra la MISMA
// función (memory.OpcionesDeRecallDelTurno), así que un defecto adentro de esa función —un campo
// que deja de seguir al yaml— los mueve a los dos juntos y las deja verdes. Ésta no.
//
// Sabotaje: la fuente única ignora recall_stemming.
// arnes: archivo="internal/memory/opciones_turno.go"
// arnes: de="\t\tStemming:        memCfg.RecallStemming,"
// arnes: a="\t\tStemming:        true,"
//
// Sabotaje: la fuente única fija mmr_lambda en el valor de fábrica.
// arnes: archivo="internal/memory/opciones_turno.go"
// arnes: de="\t\tMMRLambda:   memCfg.MMRLambda,"
// arnes: a="\t\tMMRLambda:   0.75,"
func TestElBrazoDelTurnoTraduceElYaml(t *testing.T) {
	m := config.Default().Memory
	m.RecallStemming = false
	m.RecallCooccurrence = false
	m.RecallGraphCentrality = false
	m.VectorFloor = 0.42
	m.MMRLambda = 0.6
	m.CandidatePool = 77 // gobierna la tool musubi_recall, nunca el hook
	m.GistMaxTokens = 9  // idem
	quiero := memory.RecallOptions{
		NoBump:        true,
		RankedFTS:     true,
		CandidatePool: 50,
		GistMaxTokens: 24,
		VectorFloor:   0.42,
		MMRLambda:     0.6,
	}
	if got := ConfigTurnoCon(m).Opts; !reflect.DeepEqual(got, quiero) {
		t.Errorf("el brazo del turno no traduce el yaml como el hook:\n  banco  %+v\n  quiero %+v", got, quiero)
	}
}

// TestElBancoCorreElMotorDelHook: con vectores sembrados, el brazo del turno (sin embebedor, como
// el hook de hoy) da EXACTAMENTE el orden que da sin MMR, porque su motor no ve vectores con qué
// medir redundancia. El defecto que cierra: el banco sembraba los vectores con model_id ” y los
// leía con ”, así que ConfigTurno diversificaba y el hook no. El hook no construye embebedor con
// la tabla estática presente, nunca llama SetVectorModelID, y en la base real no hay ni un vector
// con model_id ”. Medido sobre la base de davantis-1: léxico del banco 0,357 / 0,211 (MRR / R@10)
// contra el hook 0,395 / 0,355.
//
// El control es el mismo brazo CON embebedor: tiene que dar otro orden en alguna consulta, o el
// corpus no distingue MMR y la igualdad de arriba no probaría nada. Por eso el corpus trae notas
// casi repetidas: con golden.json y hashEmbed, MMR 0,75 no cambiaba el orden de ninguna consulta.
//
// Sabotaje: rankedIDs no pone al motor sin procedencia en un brazo SinEmbebedor.
// arnes: archivo="internal/recalleval/harness.go"
// arnes: de="\t\tm.SetVectorModelID(\"\")\n"
// arnes: a="\t\tm.SetVectorModelID(antes)\n"
//
// Sabotaje: SeedEngine vuelve a estampar los vectores sin nombre.
// arnes: archivo="internal/recalleval/harness.go"
// arnes: de="\t\teng.SetVectorModelID(ModeloDelBanco)"
// arnes: a="\t\teng.SetVectorModelID(\"\")"
//
// Sabotaje: el brazo del turno deja de modelar el motor sin embebedor.
// arnes: archivo="internal/recalleval/configs.go"
// arnes: de="\t\tSinEmbebedor: true,"
// arnes: a="\t\tSinEmbebedor: false,"
//
// Sabotaje: el brazo sin embebedor no le devuelve al motor su procedencia.
// arnes: archivo="internal/recalleval/harness.go"
// arnes: de="\t\tdefer m.SetVectorModelID(antes)"
// arnes: a="\t\tdefer m.SetVectorModelID(\"\")"
func TestElBancoCorreElMotorDelHook(t *testing.T) {
	fx := corpusParaMMR()
	ctx := context.Background()
	eng, err := SeedEngine(t.TempDir(), fx, hashEmbed)
	if err != nil {
		t.Fatalf("SeedEngine: %v", err)
	}
	defer eng.Close()

	hook := ConfigTurno()
	sinMMR := ConfigTurno()
	sinMMR.Opts.MMRLambda = 1
	conEmbebedor := ConfigTurno()
	conEmbebedor.SinEmbebedor = false

	distintasConEmbebedor := 0
	for _, q := range fx.Queries {
		orden := func(c Config) []string {
			ids, err := rankedIDs(ctx, eng, q.Text, c, hashEmbed, len(fx.Docs))
			if err != nil {
				t.Fatalf("%s/%s: %v", c.Name, q.ID, err)
			}
			return ids
		}
		ref := orden(sinMMR)
		if got := orden(hook); !reflect.DeepEqual(got, ref) {
			t.Errorf("%s: el brazo del hook diversificó, y el hook sin embebedor no puede:\n  brazo   %v\n  sin MMR %v", q.ID, got, ref)
		}
		if !reflect.DeepEqual(orden(conEmbebedor), ref) {
			distintasConEmbebedor++
		}
	}
	if distintasConEmbebedor == 0 {
		t.Fatal("control: con embebedor (MMR 0,75) el orden no cambió en ninguna consulta; el corpus no distingue MMR y la prueba no mide nada")
	}
	if got := eng.VectorModelID(); got != ModeloDelBanco {
		t.Errorf("después de correr el brazo sin embebedor el motor quedó leyendo con %q, no con %q", got, ModeloDelBanco)
	}
	t.Logf("con embebedor, %d de %d consultas cambian de orden por MMR", distintasConEmbebedor, len(fx.Queries))
}

// corpusParaMMR es un corpus donde la diversidad SE NOTA: cuatro notas casi iguales, todas
// pertinentes, y otras pertinentes distintas. Con MMR, después de la primera casi repetida sube una
// distinta; sin MMR, las casi repetidas van juntas.
func corpusParaMMR() *Fixture {
	fx := &Fixture{}
	for i, extra := range []string{"uno", "dos", "tres", "cuatro"} {
		fx.Docs = append(fx.Docs, Doc{
			ID:      fmt.Sprintf("casi-%d", i),
			Topic:   "altura/fichaje",
			Content: "el fichaje de la raspberry del kiosko quedó con la pantalla en blanco " + extra,
		})
	}
	for i, c := range []string{
		"el fichaje de la raspberry perdió el wifi y dejó de subir marcas",
		"el kiosko carga el bundle viejo del navegador y queda en blanco",
		"la raspberry del fichaje se alcanza por ssh con el usuario altura20",
		"el deploy del cerebro central pide sudo y un reinicio del servicio",
		"el panel del cerebro dibuja los hilos con demasiada tinta",
	} {
		fx.Docs = append(fx.Docs, Doc{ID: fmt.Sprintf("otra-%d", i), Topic: "varios", Content: c})
	}
	fx.Queries = []Query{
		{ID: "q1", Text: "fichaje de la raspberry en el kiosko", Relevant: []string{"casi-0", "otra-0"}},
		{ID: "q2", Text: "kiosko en blanco", Relevant: []string{"casi-0", "otra-1"}},
		{ID: "q3", Text: "raspberry fichaje pantalla", Relevant: []string{"casi-0", "otra-2"}},
	}
	return fx
}

// recuperadorQueAnota registra las opciones de cada Recall, y con qué procedencia de vectores
// estaba el motor en ese momento, y no devuelve nada.
type recuperadorQueAnota struct {
	opts    []memory.RecallOptions
	modelo  string
	modelos []string
}

func (r *recuperadorQueAnota) Recall(_ context.Context, _ string, o memory.RecallOptions) (memory.RecallResult, error) {
	r.opts = append(r.opts, o)
	r.modelos = append(r.modelos, r.modelo)
	return memory.RecallResult{}, nil
}

func (r *recuperadorQueAnota) VectorModelID() string     { return r.modelo }
func (r *recuperadorQueAnota) SetVectorModelID(m string) { r.modelo = m }

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

	for _, c := range []struct {
		cfg    Config
		modelo string // la procedencia con la que tiene que estar el motor durante el recall
	}{
		{ConfigTurno(), ""},                // el hook de hoy: sin embebedor
		{ConfigTurnoHibrido(), "sembrado"}, // con embebedor: la del motor
	} {
		r := &recuperadorQueAnota{modelo: "sembrado"}
		if _, err := Evaluate(ctx, r, fx, c.cfg, hashEmbed, ks); err != nil {
			t.Fatalf("%s: %v", c.cfg.Name, err)
		}
		if len(r.opts) != 1 {
			t.Fatalf("%s: esperaba 1 recall, hubo %d", c.cfg.Name, len(r.opts))
		}
		if got := r.opts[0].CandidatePool; got != hook || hook != 50 {
			t.Errorf("%s: el banco le pidió un pool de %d y el hook rankea %d (esperaba 50)", c.cfg.Name, got, hook)
		}
		if r.modelos[0] != c.modelo || r.modelo != "sembrado" {
			t.Errorf("%s: el motor leyó con procedencia %q (quería %q) y quedó en %q", c.cfg.Name, r.modelos[0], c.modelo, r.modelo)
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
