package memory

import (
	"context"
	"testing"
)

// Invariantes de «dominios ajenos no se juzgan» (specs/dominios-ajenos-no-se-juzgan).
// Lo que se prueba es que el detector DEJE DE PROPONER relaciones entre temas incomparables,
// sin perder las que sí exigen decisión.

func TestDominioDe(t *testing.T) {
	casos := []struct{ topic, quiero string }{
		{"gio/auditoria-terminales", "gio"},
		{"roadmap/track-potencia-medida", "roadmap"},
		{"git-commit", "git-commit"},
		{"bugs", "bugs"},
		{"a/b/c", "a"},
		{"", ""},
		// Barra INICIAL: el primer segmento es vacío, así que el topic entero es el dominio.
		// Sin la guarda `i > 0`, "/x" daría dominio "" y CUALQUIER topic mal formado sería del
		// mismo dominio que otro igual de mal formado — un agujero silencioso.
		{"/sin-dominio", "/sin-dominio"},
	}
	for _, c := range casos {
		if got := dominioDe(c.topic); got != c.quiero {
			t.Errorf("dominioDe(%q) = %q, esperaba %q", c.topic, got, c.quiero)
		}
	}
}

// D1 — dominios distintos no proponen relación. D3 — el mismo dominio no cambia.
func TestD1DominiosDistintosNoSeJuzgan(t *testing.T) {
	casos := []struct {
		nombre string
		a, b   string
		ajenos bool
	}{
		{"dos auditorías de temas distintos", "gio/auditoria-terminales", "last-chaos-nostalgia/investigacion-fallas", true},
		{"roadmap contra cognición", "roadmap/track-potencia", "cognicion/prueba-motor", true},
		{"mismo dominio, notas distintas", "gio/auditoria-terminales", "gio/donde-trabaja", false},
		{"mismo dominio exacto", "bugs", "bugs", false},
		{"jerarquía profunda del mismo dominio", "sdd/cambio-a/spec", "sdd/cambio-b/design", false},
	}
	for _, c := range casos {
		got := dominiosAjenos(obsRow{topicKey: c.a}, obsRow{topicKey: c.b})
		if got != c.ajenos {
			t.Errorf("%s: dominiosAjenos(%q,%q) = %v, esperaba %v", c.nombre, c.a, c.b, got, c.ajenos)
		}
	}
}

// D2 — la excepción son los REGISTROS HISTÓRICOS (commit o contrato SDD), no sólo git-commit.
//
// La primera versión de este test afirmaba que la excepción era exclusiva de git-commit, y estaba
// MAL: rompía TestSoloLasCreenciasSeReemplazan/contrato -> nota, que sella que una creencia sí se
// puede reemplazar. Un registro histórico no es un dominio temático —es lo que pasó o lo que se
// acordó— así que puede volver obsoleta una nota de cualquier tema.
func TestD2LosRegistrosHistoricosSonLaExcepcion(t *testing.T) {
	// Un commit contra una nota de otro dominio: pasa (es la única señal cross-dominio medida).
	if dominiosAjenos(obsRow{topicKey: CommitTopicKey}, obsRow{topicKey: "bugs"}) {
		t.Error("FUGA D2: un commit debe poder relacionarse con una nota de cualquier dominio")
	}
	if dominiosAjenos(obsRow{topicKey: "arquitectura/x"}, obsRow{topicKey: CommitTopicKey}) {
		t.Error("FUGA D2: la excepción debe valer también con el commit del otro lado")
	}
	// Un contrato SDD contra una nota de otro dominio: también pasa la guarda de dominios.
	// (Si además correspondiera filtrarlo, lo hace complementaryPair, que es la otra capa.)
	if dominiosAjenos(obsRow{topicKey: "sdd/cambio/spec"}, obsRow{topicKey: "gio/nota"}) {
		t.Error("FUGA D2: un contrato SDD es registro histórico y debe pasar la guarda de dominios")
	}
	// LO QUE SÍ SE FILTRA, y es el control que le da sentido a todo lo de arriba: dos notas
	// temáticas de dominios distintos. Sin esta aserción, "la excepción funciona" podría cumplirse
	// con una guarda que no filtra absolutamente nada.
	if !dominiosAjenos(obsRow{topicKey: "gio/nota"}, obsRow{topicKey: "minecraft/otra"}) {
		t.Error("FUGA D1: dos notas temáticas de dominios distintos SÍ deben filtrarse")
	}
}

// D5 — la guarda es simétrica: cuál se guardó última es un accidente del orden de escritura.
func TestD5LaGuardaEsSimetrica(t *testing.T) {
	pares := [][2]string{
		{"gio/x", "minecraft/y"},
		{CommitTopicKey, "bugs"},
		{"roadmap/a", "roadmap/b"},
		{"sdd/c/spec", "gio/n"},
	}
	for _, p := range pares {
		ab := dominiosAjenos(obsRow{topicKey: p[0]}, obsRow{topicKey: p[1]})
		ba := dominiosAjenos(obsRow{topicKey: p[1]}, obsRow{topicKey: p[0]})
		if ab != ba {
			t.Errorf("FUGA D5: no es simétrica para (%q,%q): %v vs %v", p[0], p[1], ab, ba)
		}
	}
}

// D1 END-TO-END — el detector real deja de proponer la relación, con CONTROL: una nota del mismo
// dominio y con el mismo texto SÍ la propone, así que el test no puede pasar por "no detectó nada".
func TestD1EndToEndElDetectorNoProponeEntreDominios(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()
	_ = ctx

	const texto = "auditoria multi-agente con verificacion adversarial: refutadores independientes " +
		"tumbaron siete de doce hallazgos y el informe quedo con los cinco que sobrevivieron"

	if err := e.SaveObservationTyped("BASE", "gio/auditoria-terminales", texto, 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}

	// CONTROL: misma familia ⇒ la relación DEBE nacer. Si esto no pasa, el test de abajo no
	// prueba nada (podría estar verde porque el detector no detecta nada de nada).
	if err := e.SaveObservationTyped("MISMO", "gio/auditoria-uso-musubi", texto+" y con la misma forma", 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}
	rels, err := e.DetectRelations("MISMO", ConflictOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) == 0 {
		t.Fatal("el control no sirve: dentro del MISMO dominio el detector no propuso nada, " +
			"así que no se está probando que la guarda filtre")
	}

	// LO QUE SE PRUEBA: mismo texto, otro dominio ⇒ ninguna relación.
	if err := e.SaveObservationTyped("AJENA", "last-chaos-nostalgia/investigacion-fallas", texto+" y con la misma forma", 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}
	relsAjena, err := e.DetectRelations("AJENA", ConflictOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range relsAjena {
		if r.TargetID == "BASE" || r.TargetID == "MISMO" {
			t.Errorf("FUGA D1: se propuso una relación entre dominios distintos (%s -> %s)", r.SourceID, r.TargetID)
		}
	}
}

// D4 — la guarda NO oculta memoria: la observación filtrada sigue viva y sigue apareciendo en el
// recall. Evitar una RELACIÓN nunca puede costar una OBSERVACIÓN.
func TestD4LaGuardaNoOcultaMemoria(t *testing.T) {
	e := newTestEngine(t)
	const texto = "el portero de privacidad tapa los secretos antes de que crucen al motor externo"

	if err := e.SaveObservationTyped("A", "cognicion/portero", texto, 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTyped("B", "minecraft/servidor", texto+" segun la nota", 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.DetectRelations("B", ConflictOptions{}); err != nil {
		t.Fatal(err)
	}

	res, err := e.Recall(context.Background(), "portero privacidad secretos motor", RecallOptions{TokenBudget: 4000})
	if err != nil {
		t.Fatal(err)
	}
	vistas := map[string]bool{}
	for _, it := range res.Items {
		vistas[it.ID] = true
	}
	if !vistas["A"] || !vistas["B"] {
		t.Errorf("FUGA D4: la guarda ocultó memoria del recall (A=%v B=%v)", vistas["A"], vistas["B"])
	}
	var superseded int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE superseded_by IS NOT NULL`).Scan(&superseded); err != nil {
		t.Fatal(err)
	}
	if superseded != 0 {
		t.Errorf("FUGA D4: la guarda marcó %d observaciones como reemplazadas; nunca debe tocar superseded_by", superseded)
	}
}

// D6 — convive con la guarda del par histórico: un commit contra un contrato SDD sigue filtrado
// por complementaryPair aunque la excepción de git-commit lo deje pasar por dominios.
func TestD6ConviveConLaGuardaDelParHistorico(t *testing.T) {
	if !complementaryPair(obsRow{topicKey: CommitTopicKey}, obsRow{topicKey: "sdd/cambio/spec"}) {
		t.Error("la guarda del par histórico debe seguir filtrando commit -> contrato SDD")
	}
	// La de dominios lo dejaría pasar (git-commit es excepción); la histórica lo ataja. Las dos
	// capas son necesarias y ninguna vuelve inalcanzable a la otra.
	if dominiosAjenos(obsRow{topicKey: CommitTopicKey}, obsRow{topicKey: "sdd/cambio/spec"}) {
		t.Error("la excepción de git-commit debe dejar pasar este par a la siguiente guarda")
	}
}
