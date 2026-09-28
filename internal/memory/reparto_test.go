package memory

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/logx"
)

// LO PROPIO PRIMERO, LO AJENO CON TOPE (decisión 8 del dueño: «aparte»).
//
// Una base local que sincroniza con alcance federado —la sala de mando— tiene notas de varios
// repos, y el hook del turno las traía mezcladas: en davantis-1, 178 notas ajenas de 1222
// inyectadas, el 74 % commits o artefactos SDD de otro repo. En «aparte», el recall deja afuera
// esos registros históricos ajenos, trae a lo sumo TopeOtrosProyectos notas ajenas cuando algo
// propio viene al caso, pone lo propio primero, y si nada propio viene al caso deja que lo ajeno
// llene el bloque, como antes. Estas pruebas fijan cada mitad por separado.

// notaDeReparto es una nota del corpus: de qué proyecto, con qué topic y cuánto pesa.
type notaDeReparto struct {
	proyecto, id, topic, texto string
	importancia                float64
}

// sembrarReparto guarda las notas con su proyecto de origen y una fecha fija, para que el orden del
// ranking no dependa del segundo en que se sembró cada una.
func sembrarReparto(t *testing.T, notas []notaDeReparto) *DbEngine {
	t.Helper()
	e := newTestEngine(t)
	for _, n := range notas {
		imp := n.importancia
		if imp == 0 {
			imp = 1
		}
		if err := e.SaveObservationTypedFrom(n.proyecto, "", n.id, n.topic, n.texto, imp, "", "", nil); err != nil {
			t.Fatalf("sembrar %s: %v", n.id, err)
		}
		if err := e.SetObservationCreatedAt(n.id, "2026-09-01 12:00:00"); err != nil {
			t.Fatal(err)
		}
	}
	return e
}

// corpusDelKiosko es la misma charla —el fichaje del kiosko— en el proyecto propio (musubi), en uno
// ajeno (altura) y sin atribuir, con registros históricos de los dos lados. Las ajenas que no son
// registro son cuatro, el doble del tope: si el tope no corta, se nota.
func corpusDelKiosko() []notaDeReparto {
	return []notaDeReparto{
		{"musubi", "p-hook", "hooks/turno", "El fichaje del kiosko aparece en el hook del turno de Musubi.", 0},
		{"musubi", "p-commit", CommitTopicKey, "fix(turno): el fichaje del kiosko ya no se repite en el hook.", 0},
		{"", "u-suelta", "notas/sueltas", "Una nota vieja sobre el fichaje del kiosko, sin proyecto.", 0},
		{"altura", "a-wifi", "feature/fichaje-f18", "El fichaje del kiosko F18 se cae cuando la Pi pierde el WiFi.", 0},
		{"altura", "a-reintento", "feature/fichaje-reintento", "El kiosko reintenta el fichaje cada 30 segundos.", 0},
		{"altura", "a-lector", "hardware/lector", "El lector de huellas del kiosko falla con el fichaje en frío.", 0},
		{"altura", "a-turnos", "rrhh/turnos", "El fichaje del kiosko cierra los turnos a medianoche.", 0},
		{"altura", "a-commit", CommitTopicKey, "fix(kiosko): el fichaje reintenta cuando vuelve el WiFi de la Pi.", 0},
		{"altura", "a-sdd", "sdd/fichaje-offline/design", "Diseño del fichaje offline del kiosko: cola local y reintento por WiFi.", 0},
	}
}

// aparte es el recall del modo «aparte» para el proyecto musubi, con un presupuesto que no corta.
func aparte(t *testing.T, e *DbEngine, consulta string) RecallResult {
	t.Helper()
	res, err := e.Recall(context.Background(), consulta, RecallOptions{
		TokenBudget: 5000, NoBump: true, ProjectScope: "musubi", TopeOtrosProyectos: 2,
	})
	if err != nil {
		t.Fatalf("Recall(%q): %v", consulta, err)
	}
	return res
}

// idsDe devuelve los ids de un resultado, en orden.
func idsDelReparto(r RecallResult) []string {
	out := make([]string, 0, len(r.Items))
	for _, it := range r.Items {
		out = append(out, it.ID)
	}
	return out
}

// ajenasDe cuenta las notas de otro proyecto que trae un resultado, con el criterio de la muralla.
func ajenasDe(r RecallResult, propio string) []string {
	var out []string
	for _, it := range r.Items {
		if !MismoProyecto(propio, it.ProjectID) {
			out = append(out, it.ID)
		}
	}
	return out
}

func trae(r RecallResult, id string) bool {
	for _, it := range r.Items {
		if it.ID == id {
			return true
		}
	}
	return false
}

// TestTopeDeAjenos: cuando algo propio viene al caso, entran EXACTAMENTE dos notas ajenas de las
// cuatro que caben —ni más, que es el tope, ni menos, que sería otro modo—, y lo propio entra entero.
//
// Sabotaje: el cupo de lo ajeno no corta.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\t\tif ajena && ajenas >= rep.tope {\n"
// arnes: a="\t\tif false && ajena && ajenas >= rep.tope {\n"
func TestTopeDeAjenos(t *testing.T) {
	e := sembrarReparto(t, corpusDelKiosko())
	res := aparte(t, e, "fichaje kiosko")

	if got := ajenasDe(res, "musubi"); len(got) != 2 {
		t.Errorf("con lo propio al caso tenían que entrar 2 notas ajenas (el tope) y entraron %d: %v\n  ids: %v", len(got), got, idsDelReparto(res))
	}
	for _, id := range []string{"p-hook", "p-commit", "u-suelta"} {
		if !trae(res, id) {
			t.Errorf("lo propio tenía que entrar entero y falta %s: %v", id, idsDelReparto(res))
		}
	}
}

// TestSinPropiosLoAjenoLlena: si nada propio ni sin atribuir viene al caso —preguntar desde Musubi
// por el WiFi de la Pi, que es de Altura—, el tope se levanta y entran las cuatro notas ajenas que
// no son registro, como antes. Sin esto, «aparte» dejaba en dos notas la consulta que ES de otro
// proyecto.
//
// Sabotaje: el tope no se levanta nunca.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\tif !entroAlgoPropio(result, rep.propio) {\n"
// arnes: a="\tif false && !entroAlgoPropio(result, rep.propio) {\n"
func TestSinPropiosLoAjenoLlena(t *testing.T) {
	e := sembrarReparto(t, corpusDelKiosko())
	res := aparte(t, e, "WiFi lector medianoche reintenta")

	if got := ajenasDe(res, "musubi"); len(got) != len(res.Items) || len(got) != 4 {
		t.Errorf("sin nada propio al caso tenían que entrar las 4 ajenas que no son registro, y entró %v (ajenas %v)", idsDelReparto(res), got)
	}
}

// TestHistoricoAjenoNoEntra: un commit o un artefacto SDD de OTRO proyecto no entra en «aparte», ni
// con lo propio al caso ni cuando el tope se levanta porque nada propio viene al caso. Y la otra
// mitad: los registros PROPIOS sí entran, porque describen este repo.
//
// Sabotaje: el filtro deja pasar los registros ajenos.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\t\tif !historicalRecord(c.topicKey) {\n"
// arnes: a="\t\tif true || !historicalRecord(c.topicKey) {\n"
//
// Sabotaje: el filtro saca también los registros propios.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\t\t\tout = append(out, c) // lo propio y lo sin atribuir pasan siempre, registros incluidos\n"
// arnes: a="\t\t\tif c.topicKey != CommitTopicKey && !isSDD(c.topicKey) {\n\t\t\t\tout = append(out, c)\n\t\t\t}\n"
func TestHistoricoAjenoNoEntra(t *testing.T) {
	e := sembrarReparto(t, corpusDelKiosko())
	for _, consulta := range []string{"fichaje kiosko", "WiFi reintenta Pi"} {
		res := aparte(t, e, consulta)
		if len(res.Items) == 0 {
			t.Fatalf("%q no trajo nada: sin resultado no se mide nada", consulta)
		}
		for _, id := range []string{"a-commit", "a-sdd"} {
			if trae(res, id) {
				t.Errorf("%q trajo el registro ajeno %s: %v", consulta, id, idsDelReparto(res))
			}
		}
	}
	if res := aparte(t, e, "fichaje kiosko"); !trae(res, "p-commit") {
		t.Errorf("el commit PROPIO tenía que entrar: %v", idsDelReparto(res))
	}
}

// TestSinAtribuirNoEsAjeno: una nota sin proyecto cuenta como propia en las tres decisiones de
// «aparte», con el criterio de la muralla (MismoProyecto): pasa el filtro aunque sea un registro,
// no gasta el cupo de lo ajeno, y cuenta como «algo propio vino al caso».
//
// Sabotaje: el filtro trata lo sin atribuir como ajeno.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\t\tif MismoProyecto(propio, c.projectID) {\n"
// arnes: a="\t\tif c.projectID == propio {\n"
//
// Sabotaje: lo sin atribuir gasta el cupo de lo ajeno.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\t\tajena := rep.tope > 0 && !MismoProyecto(rep.propio, c.projectID)\n"
// arnes: a="\t\tajena := rep.tope > 0 && c.projectID != rep.propio\n"
//
// Sabotaje: lo sin atribuir no cuenta como algo propio que vino al caso.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\t\tif MismoProyecto(propio, it.ProjectID) {\n\t\t\treturn true\n"
// arnes: a="\t\tif it.ProjectID == propio {\n\t\t\treturn true\n"
func TestSinAtribuirNoEsAjeno(t *testing.T) {
	e := sembrarReparto(t, []notaDeReparto{
		{"", "u-commit", CommitTopicKey, "fix(galpón): la balanza del galpón pesa en kilos.", 0},
		{"", "u-uno", "notas/galpon", "La balanza del galpón se calibra los lunes.", 0},
		{"", "u-dos", "notas/galpon-2", "La balanza del galpón tiene un cable flojo.", 0},
		{"", "u-tres", "notas/galpon-3", "La balanza del galpón marca de más en verano.", 0},
		{"altura", "a-uno", "galpon/balanza", "La balanza del galpón de Altura manda el peso al ERP.", 0},
		{"altura", "a-dos", "galpon/balanza-2", "La balanza del galpón de Altura se cuelga con el frío.", 0},
		{"altura", "a-tres", "galpon/balanza-3", "La balanza del galpón de Altura usa un puerto serie.", 0},
	})
	res := aparte(t, e, "balanza galpón")

	for _, id := range []string{"u-commit", "u-uno", "u-dos", "u-tres"} {
		if !trae(res, id) {
			t.Errorf("la nota sin atribuir %s tenía que entrar como propia: %v", id, idsDelReparto(res))
		}
	}
	if got := ajenasDe(res, "musubi"); len(got) != 2 {
		t.Errorf("lo sin atribuir vino al caso, así que lo ajeno tenía que quedar en el tope (2), y entró %v", got)
	}
}

// TestLoPropioVaPrimero: con el tope puesto, lo propio y lo sin atribuir van antes que lo ajeno,
// aunque lo ajeno tenga más score. Es el orden en que lo lee el agente.
//
// Sabotaje: el resultado sale en el orden del score, con lo ajeno adelante.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\treturn loPropioPrimero(empaquetar(ranked, budget, gistMax, rep), rep.propio)\n"
// arnes: a="\treturn empaquetar(ranked, budget, gistMax, rep)\n"
func TestLoPropioVaPrimero(t *testing.T) {
	notas := corpusDelKiosko()
	for i := range notas {
		if notas[i].proyecto == "altura" {
			notas[i].importancia = 5 // lo ajeno pesa más: sin la partición, va primero
		}
	}
	e := sembrarReparto(t, notas)

	federado, err := e.Recall(context.Background(), "fichaje kiosko", RecallOptions{TokenBudget: 5000, NoBump: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(federado.Items) == 0 || MismoProyecto("musubi", federado.Items[0].ProjectID) {
		t.Fatalf("el corpus tenía que poner una nota ajena primero por score, y el federado trajo %v", idsDelReparto(federado))
	}

	res := aparte(t, e, "fichaje kiosko")
	vioAjena := false
	for _, it := range res.Items {
		ajena := !MismoProyecto("musubi", it.ProjectID)
		if !ajena && vioAjena {
			t.Fatalf("lo propio tenía que ir antes que lo ajeno, y %s vino después de una ajena: %v", it.ID, idsDelReparto(res))
		}
		vioAjena = vioAjena || ajena
	}
	if !vioAjena {
		t.Fatalf("sin ninguna ajena el orden no se mide: %v", idsDelReparto(res))
	}
}

// TestMezcladoEsElDeHoy: «mezclado» es el rollback, así que tiene que ser EXACTAMENTE el hook de
// antes: los mismos ids en el mismo orden que el recall federado, con registros ajenos y más ajenas
// que el tope, y el mismo priming que el de todo el acervo.
//
// Sabotaje: «mezclado» cae en «aparte».
// arnes: archivo="internal/memory/opciones_turno.go"
// arnes: de="\tcase config.OtrosProyectosMezclado:\n\t\treturn AlcanceDelTurno{}\n"
// arnes: a="\tcase config.OtrosProyectosMezclado + \"-\":\n\t\treturn AlcanceDelTurno{}\n"
func TestMezcladoEsElDeHoy(t *testing.T) {
	e := sembrarReparto(t, corpusDelKiosko())
	ctx := context.Background()
	memCfg := config.Default().Memory

	hoy := OpcionesDeRecallDelTurno(memCfg, AlcanceDelTurno{})
	hoy.TokenBudget = 5000
	federado, err := e.Recall(ctx, "fichaje kiosko", hoy)
	if err != nil {
		t.Fatal(err)
	}
	if got := ajenasDe(federado, "musubi"); len(got) <= 2 || !trae(federado, "a-commit") {
		t.Fatalf("el corpus tenía que traer más ajenas que el tope y un registro ajeno, y el federado trajo %v", idsDelReparto(federado))
	}

	alcance := AlcanceDelTurnoSegun(config.OtrosProyectosMezclado, 2, "musubi")
	mezclado := OpcionesDeRecallDelTurno(memCfg, alcance)
	mezclado.TokenBudget = 5000
	res, err := e.Recall(ctx, "fichaje kiosko", mezclado)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(idsDelReparto(res), idsDelReparto(federado)) {
		t.Errorf("«mezclado» tenía que ser el recall de hoy:\n  mezclado: %v\n  hoy:      %v", idsDelReparto(res), idsDelReparto(federado))
	}

	todo, err := e.PrimeContext(5000)
	if err != nil {
		t.Fatal(err)
	}
	prime, err := e.PrimeContextCtx(WithProjectScope(ctx, alcance.ScopeDelPriming()), 5000)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(idsDelReparto(prime), idsDelReparto(todo)) {
		t.Errorf("en «mezclado» el priming tenía que ser el de todo el acervo:\n  mezclado: %v\n  hoy:      %v", idsDelReparto(prime), idsDelReparto(todo))
	}
}

// TestPrimingEnSuProyecto: el priming acotado trae lo propio y lo sin atribuir, y nada de otro
// proyecto. Sin scope en el ctx es el de siempre, con las notas de altura: si no, la prueba no mide.
//
// Sabotaje: el priming no aplica el alcance del ctx.
// arnes: archivo="internal/memory/prime.go"
// arnes: de="\tclause, args := projectScopeFrom(ctx).scopeClause(\"o\")\n"
// arnes: a="\tclause, args := ProjectScope{}.scopeClause(\"o\")\n"
func TestPrimingEnSuProyecto(t *testing.T) {
	e := sembrarReparto(t, corpusDelKiosko())
	todo, err := e.PrimeContext(5000)
	if err != nil {
		t.Fatal(err)
	}
	if len(ajenasDe(todo, "musubi")) == 0 {
		t.Fatalf("sin scope el priming tenía que traer lo de altura: %v", idsDelReparto(todo))
	}

	alcance := AlcanceDelTurnoSegun(config.OtrosProyectosAparte, 2, "musubi")
	res, err := e.PrimeContextCtx(WithProjectScope(context.Background(), alcance.ScopeDelPriming()), 5000)
	if err != nil {
		t.Fatal(err)
	}
	if got := ajenasDe(res, "musubi"); len(got) != 0 {
		t.Errorf("el priming en «aparte» trajo notas de otro proyecto: %v", got)
	}
	for _, id := range []string{"p-hook", "p-commit", "u-suelta"} {
		if !trae(res, id) {
			t.Errorf("el priming acotado tenía que traer %s: %v", id, idsDelReparto(res))
		}
	}
}

// TestAlcanceDelTurnoSegun: la traducción del yaml al alcance, caso por caso. Un modo desconocido cae
// al default CON un aviso, un tope negativo es «aislado», y sin proyecto propio no se acota nada.
//
// Sabotaje: el tope negativo deja de ser «aislado».
// arnes: archivo="internal/memory/opciones_turno.go"
// arnes: de="\tif tope < 0 {\n"
// arnes: a="\tif tope < -1 {\n"
//
// Sabotaje: el modo desconocido cae en silencio.
// arnes: archivo="internal/memory/opciones_turno.go"
// arnes: de="\t\tlogx.Warn(\"loop.recall_otros_proyectos no es un modo conocido: uso el default\",\n"
// arnes: a="\t\tlogx.Info(\"loop.recall_otros_proyectos no es un modo conocido: uso el default\",\n"
//
// Sabotaje: el modo desconocido cae en «mezclado» en vez del default.
// arnes: archivo="internal/memory/opciones_turno.go"
// arnes: de=", \"modos\", \"aparte | aislado | mezclado\")\n\t\tm = d.RecallOtrosProyectos\n"
// arnes: a=", \"modos\", \"aparte | aislado | mezclado\")\n\t\tm = config.OtrosProyectosMezclado\n"
//
// Sabotaje: sin proyecto propio se acota a "".
// arnes: archivo="internal/memory/opciones_turno.go"
// arnes: de="\tif propio == \"\" {\n"
// arnes: a="\tif propio == \"-\" {\n"
func TestAlcanceDelTurnoSegun(t *testing.T) {
	aparte2 := AlcanceDelTurno{ProjectScope: "musubi", TopeOtrosProyectos: 2}
	aislado := AlcanceDelTurno{ProjectScope: "musubi"}
	for _, c := range []struct {
		modo   string
		tope   int
		propio string
		quiere AlcanceDelTurno
		avisa  bool
	}{
		{"aparte", 2, "musubi", aparte2, false},
		{"", 0, "musubi", aparte2, false}, // el vacío y el cero valen el default
		{"aparte", 5, "musubi", AlcanceDelTurno{ProjectScope: "musubi", TopeOtrosProyectos: 5}, false},
		{"aparte", -1, "musubi", aislado, false}, // negativo ⇒ sin ajenas
		{" AISLADO ", 3, "musubi", aislado, false},
		{"mezclado", 2, "musubi", AlcanceDelTurno{}, false},
		{"aparte", 2, "", AlcanceDelTurno{}, false},
		{"aislado", 2, "", AlcanceDelTurno{}, false},
		{"apartado", 2, "musubi", aparte2, true}, // desconocido ⇒ default, con aviso
	} {
		var log bytes.Buffer
		restaurar := logx.Capturar(&log)
		got := AlcanceDelTurnoSegun(c.modo, c.tope, c.propio)
		restaurar()
		if got != c.quiere {
			t.Errorf("AlcanceDelTurnoSegun(%q, %d, %q) = %+v, quiere %+v", c.modo, c.tope, c.propio, got, c.quiere)
		}
		avisó := strings.Contains(log.String(), "level=WARN") && strings.Contains(log.String(), "loop.recall_otros_proyectos")
		if avisó != c.avisa {
			t.Errorf("AlcanceDelTurnoSegun(%q, …): avisó=%v, quiere %v. Log: %q", c.modo, avisó, c.avisa, log.String())
		}
	}
}
