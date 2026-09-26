package main

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// Pruebas de la fuente única de las opciones del turno (memory.OpcionesDeRecallDelTurno) desde el
// lado del hook. La contracara del banco vive en internal/recalleval (TestConfigDelBancoEsLaDelHook):
// las dos juntas fijan que el hook y el banco corren EL MISMO ranker, porque los dos tienen que ser
// iguales a la misma función.

// TestBuildTurnRecallUsaLasOpcionesDelTurno: lo que el hook le pasa a Recall es, campo a campo,
// memory.OpcionesDeRecallDelTurno con el presupuesto del turno encima. Nada más y nada menos.
//
// La memCfg lleva valores que NO son los default a propósito: si el hook armara sus opciones con
// config.Default() en vez de la config que recibe, o con memCfg.CandidatePool (que gobierna la tool
// musubi_recall y nunca gobernó el hook), la igualdad se rompe.
//
// Sabotaje: el hook apaga el filtro de stopwords después de pedir las opciones a la fuente única.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\topts.TokenBudget = p.presupuesto\n"
// arnes: a="\topts.TokenBudget = p.presupuesto\n\topts.RankedFTS = false\n"
func TestBuildTurnRecallUsaLasOpcionesDelTurno(t *testing.T) {
	memCfg := config.Default().Memory
	memCfg.RecallStemming = false
	memCfg.RecallGraphCentrality = false
	memCfg.VectorFloor = 0.42
	memCfg.MMRLambda = 0.6
	memCfg.CandidatePool = 77
	memCfg.GistMaxTokens = 9
	store := &fakeTurnStore{recall: memory.RecallResult{
		Count: 1,
		Items: []memory.RecallItem{{ID: "a", Gist: "algo", ContentHash: "h"}},
	}}

	buildTurnRecall(store, parametrosDelTurno{sesion: "s1", prompt: "el prompt del turno", presupuesto: 250, memCfg: memCfg})

	quiero := memory.OpcionesDeRecallDelTurno(memCfg, memory.AlcanceDelTurno{})
	quiero.TokenBudget = 250
	if !reflect.DeepEqual(store.lastOpts, quiero) {
		t.Fatalf("el hook no corre las opciones de la fuente única:\n  pasó  %+v\n  quiero %+v", store.lastOpts, quiero)
	}
	// Y lo que la igualdad fija, dicho en claro: el hook filtra stopwords y rankea 50 candidatos.
	if !store.lastOpts.RankedFTS || store.lastOpts.CandidatePool != 50 {
		t.Errorf("RankedFTS=%v CandidatePool=%d: el hook corre con RankedFTS y pool 50",
			store.lastOpts.RankedFTS, store.lastOpts.CandidatePool)
	}
}

// opcionesDelTurnoDeAntes es el literal que buildTurnRecall armaba A MANO hasta que las opciones
// pasaron a memory.OpcionesDeRecallDelTurno, copiado tal cual del código de antes (main 3fd81c33).
// Está congelado a propósito: es la referencia contra la que se prueba que el refactor no cambió
// nada. No se actualiza cuando cambie la fuente única; si un cambio del ranking del turno la deja
// atrás, lo que corresponde es borrar esta prueba, no editar el literal.
func opcionesDelTurnoDeAntes(budget int, memCfg config.MemoryConfig) memory.RecallOptions {
	return memory.RecallOptions{
		TokenBudget:     budget,
		NoBump:          true,
		RankedFTS:       true,
		Stemming:        memCfg.RecallStemming,
		Cooccurrence:    memCfg.RecallCooccurrence,
		GraphCentrality: memCfg.RecallGraphCentrality,
		VectorFloor:     memCfg.VectorFloor,
		MMRLambda:       memCfg.MMRLambda,
	}
}

// storeConLasOpcionesDeAntes es el motor real, pero cada Recall corre con las opciones de antes del
// refactor en lugar de las que le pasa el hook (conserva sólo el vector de la consulta, que no es
// parte de las opciones sino del prompt). Es el hook de antes armado sobre el hook de hoy.
type storeConLasOpcionesDeAntes struct {
	turnStore
	antes memory.RecallOptions
}

func (s storeConLasOpcionesDeAntes) Recall(ctx context.Context, q string, opts memory.RecallOptions) (memory.RecallResult, error) {
	o := s.antes
	o.QueryVector = opts.QueryVector
	return s.turnStore.Recall(ctx, q, o)
}

// TestLaSalidaDelHookEsLaDeAntesDelRefactor: con el motor REAL, el hook entero (turnOutputWith, el
// envelope JSON incluido) produce exactamente los mismos bytes con las opciones de la fuente única
// que con el literal que armaba antes a mano. Es la prueba de que sacar las opciones a
// memory.OpcionesDeRecallDelTurno fue un refactor y no un cambio de ranking.
//
// Por qué con motor real y no con el fake: el fake devuelve lo que le digan, así que dos juegos de
// opciones distintos dan la misma salida y la prueba no vería nada. El corpus está armado para que
// las opciones SE NOTEN: hay notas de relleno hechas de palabras vacías («de», «la», «que»…), que
// entran al ranking si el hook deja de filtrar stopwords, y más notas pertinentes que las que entran
// en el presupuesto, así que el orden también se ve.
//
// Sabotaje: la fuente única deja de filtrar stopwords.
// arnes: archivo="internal/memory/opciones_turno.go"
// arnes: de="\t\tRankedFTS:     true, // filtrar stopwords: es la superficie más caliente, evita ruido"
// arnes: a="\t\tRankedFTS:     false, // filtrar stopwords: es la superficie más caliente, evita ruido"
//
// Sabotaje: la fuente única achica el pool de candidatos.
// arnes: archivo="internal/memory/opciones_turno.go"
// arnes: de="\t\tCandidatePool: defaultCandidatePool,"
// arnes: a="\t\tCandidatePool: 2,"
func TestLaSalidaDelHookEsLaDeAntesDelRefactor(t *testing.T) {
	eng, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	pertinentes := []string{
		"el fichaje F18 de Altura corre en la raspberry del kiosko y sube las marcas al ERP",
		"el kiosko del fichaje quedó en blanco porque la raspberry perdió el WiFi",
		"la raspberry del fichaje se reinicia sola cuando el kiosko pierde la red",
		"para revisar el fichaje hay que entrar por ssh a la raspberry como altura20",
		"el kiosko del fichaje muestra la hora del servidor, no la de la raspberry",
		"las marcas del fichaje que no suben se reintentan desde la raspberry cada minuto",
		"el fichaje del kiosko valida la huella antes de mandar la marca",
		"la raspberry guarda un respaldo local del fichaje por si se cae la red",
		"el kiosko carga el bundle del fichaje desde la raspberry al arrancar",
		"si el fichaje no anda, lo primero es mirar si la raspberry tiene tailnet",
		"la pantalla del kiosko se apaga de noche y el fichaje sigue andando",
		"el fichaje manda la marca con la hora de la raspberry y el id del kiosko",
	}
	for i, c := range pertinentes {
		if err := eng.SaveObservation(fmt.Sprintf("fichaje-%02d", i), "altura/fichaje", c, nil); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 70; i++ {
		c := fmt.Sprintf("nota %d: que de la el en los por con para una que de la el en lo que se dijo", i)
		if err := eng.SaveObservation(fmt.Sprintf("relleno-%02d", i), "relleno", c, nil); err != nil {
			t.Fatal(err)
		}
	}

	memCfg := config.Default().Memory
	loopCfg := config.LoopConfig{PerTurnRecall: true, RecallBudget: 250, DeltaInjection: true}
	turno := func(store turnStore, sesion string) string {
		in := strings.NewReader(fmt.Sprintf(`{"session_id":%q,"prompt":"¿qué pasó con el fichaje de la raspberry en el kiosko?"}`, sesion))
		return turnOutputWith(store, loopCfg, pipeOff(), maOff(), memCfg, nil, in, nil)
	}

	// Cada lado en su propia sesión: el delta es por sesión, así que ninguno le esconde al otro lo
	// que ya inyectó.
	antes := turno(storeConLasOpcionesDeAntes{turnStore: eng, antes: opcionesDelTurnoDeAntes(loopCfg.RecallBudget, memCfg)}, "antes")
	ahora := turno(eng, "ahora")

	// CONTROL: una salida vacía en los dos lados también sería «igual», y no probaría nada.
	if !strings.Contains(ahora, "fichaje-") {
		t.Fatalf("el hook no trajo ninguna nota pertinente, así que la comparación no mide nada:\n%s", ahora)
	}
	if ahora != antes {
		t.Errorf("la salida del hook cambió con el refactor de las opciones:\n--- antes ---\n%s\n--- ahora ---\n%s", antes, ahora)
	}
}
