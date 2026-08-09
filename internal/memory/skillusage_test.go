package memory

import (
	"context"
	"strings"
	"testing"
)

// Invariantes del spec «El arsenal se mide» (specs/skills-que-se-miden/) del lado de la base.

func contarSkill(t *testing.T, e *DbEngine, ctx context.Context, evs ...SkillEvent) {
	t.Helper()
	if err := e.RecordSkillEvents(ctx, evs); err != nil {
		t.Fatalf("RecordSkillEvents: %v", err)
	}
}

func filaDe(rows []SkillUsageRow, skill string) (SkillUsageRow, bool) {
	for _, r := range rows {
		if r.Skill == skill {
			return r, true
		}
	}
	return SkillUsageRow{}, false
}

// Los contadores SUMAN en vez de acumular filas: es la decisión que hace que la tabla quede acotada
// al tamaño del arsenal y no crezca con el uso.
func TestLosContadoresSumanNoAcumulanFilas(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		contarSkill(t, e, ctx, SkillEvent{Skill: "plan-ahead", Evidence: EvidenciaComodin, Kind: UsoResuelta})
	}

	var filas int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM skill_usage`).Scan(&filas); err != nil {
		t.Fatalf("contar filas: %v", err)
	}
	if filas != 1 {
		t.Errorf("5 conteos de lo mismo tienen que vivir en UNA fila; hay %d", filas)
	}

	rows, err := e.SkillUsage(ctx)
	if err != nil {
		t.Fatalf("SkillUsage: %v", err)
	}
	r, ok := filaDe(rows, "plan-ahead")
	if !ok || r.Resolved != 5 || r.PorComodin != 5 {
		t.Errorf("esperaba resolved=5 porComodin=5, obtuve %+v", r)
	}
}

// ★ A2 — «MATCHEÓ» Y «VIAJÓ SU CUERPO» SON COSAS DISTINTAS.
//
// Si se contaran juntos, la lectura «ocupa contexto en cada resolución y nadie la abrió» sería
// imposible de escribir: es exactamente la diferencia entre las dos.
func TestA2MatcheoYCuerpoSonContadoresDistintos(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	// Una skill de comodín: matcheó tres veces y su cuerpo nunca viajó.
	for i := 0; i < 3; i++ {
		contarSkill(t, e, ctx, SkillEvent{Skill: "solo-comodin", Evidence: EvidenciaComodin, Kind: UsoResuelta})
	}
	// Una de glob: matcheó dos veces y las dos se llevó el cuerpo.
	for i := 0; i < 2; i++ {
		contarSkill(t, e, ctx,
			SkillEvent{Skill: "go-hygiene", Evidence: EvidenciaGlob, Kind: UsoResuelta},
			SkillEvent{Skill: "go-hygiene", Kind: UsoCuerpoEnviado})
	}

	rows, _ := e.SkillUsage(ctx)
	comodin, _ := filaDe(rows, "solo-comodin")
	glob, _ := filaDe(rows, "go-hygiene")

	if comodin.Resolved != 3 || comodin.BodySent != 0 {
		t.Errorf("solo-comodin: esperaba resolved=3 body_sent=0, obtuve %+v", comodin)
	}
	if glob.Resolved != 2 || glob.BodySent != 2 {
		t.Errorf("go-hygiene: esperaba resolved=2 body_sent=2, obtuve %+v", glob)
	}
}

// ★ A7 — LOS CONTADORES ESTÁN ACOTADOS POR PROYECTO.
//
// Sin el scope, un miembro del cerebro central vería qué skills usa otro equipo y con qué
// frecuencia.
func TestA7LosContadoresNoCruzanProyectos(t *testing.T) {
	e := newTestEngine(t)
	crm := WithProjectScope(context.Background(), ProjectScope{ProjectID: "crm"})
	web := WithProjectScope(context.Background(), ProjectScope{ProjectID: "web"})

	contarSkill(t, e, crm, SkillEvent{Skill: "secreta-del-crm", ProjectID: "crm", Evidence: EvidenciaGlob, Kind: UsoResuelta})
	contarSkill(t, e, web, SkillEvent{Skill: "propia-de-web", ProjectID: "web", Evidence: EvidenciaGlob, Kind: UsoResuelta})

	desdeWeb, err := e.SkillUsage(web)
	if err != nil {
		t.Fatalf("SkillUsage: %v", err)
	}
	if _, hay := filaDe(desdeWeb, "secreta-del-crm"); hay {
		t.Error("FUGA: desde el proyecto web se ven los contadores del crm")
	}
	if _, hay := filaDe(desdeWeb, "propia-de-web"); !hay {
		t.Error("el proyecto tiene que ver LOS SUYOS")
	}
}

// La taxonomía es CERRADA: un kind o una evidencia inventados se saltean en vez de escribirse, y
// sin romper el lote — los conteos buenos que venían al lado tienen que llegar igual.
func TestLaTaxonomiaEsCerradaYNoRompeElLote(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	contarSkill(t, e, ctx,
		SkillEvent{Skill: "buena", Evidence: EvidenciaGlob, Kind: UsoResuelta},
		SkillEvent{Skill: "mala", Evidence: EvidenciaGlob, Kind: "se_uso_y_sirvio_mucho"},
		SkillEvent{Skill: "mala2", Evidence: "porque si", Kind: UsoResuelta},
		SkillEvent{Skill: "", Evidence: EvidenciaGlob, Kind: UsoResuelta},
		SkillEvent{Skill: "otra-buena", Evidence: EvidenciaAlcance, Kind: UsoResuelta},
	)

	rows, _ := e.SkillUsage(ctx)
	if len(rows) != 2 {
		t.Fatalf("esperaba sólo las dos válidas, obtuve %d: %+v", len(rows), rows)
	}
	if _, hay := filaDe(rows, "buena"); !hay {
		t.Error("un evento inválido no puede costarle el conteo al que venía al lado")
	}
	if _, hay := filaDe(rows, "otra-buena"); !hay {
		t.Error("un evento inválido no puede costarle el conteo al que venía después")
	}
}

// A10 — LAS CANDIDATAS SE MARCAN, y cada patrón es una lectura distinta de los contadores.
func TestA10LosTresPatrones(t *testing.T) {
	casos := []struct {
		nombre string
		fila   SkillUsageRow
		quiero string
	}{
		{"nunca pasó nada", SkillUsageRow{Skill: "a"}, "muerta"},
		{"matcheó y nadie la abrió",
			SkillUsageRow{Skill: "b", Resolved: 40, PorComodin: 40}, "retiro"},
		{"siempre por comodín pero le piden el cuerpo",
			SkillUsageRow{Skill: "c", Resolved: 12, PorComodin: 12, BodyRequested: 9}, "alcance"},
		{"matcheó por glob y su cuerpo viajó: nada que decir",
			SkillUsageRow{Skill: "d", Resolved: 30, PorGlob: 30, BodySent: 30}, ""},
	}
	for _, c := range casos {
		f := c.fila
		MarcarCandidata(&f)
		if f.Candidata != c.quiero {
			t.Errorf("%s: candidata = %q, esperaba %q", c.nombre, f.Candidata, c.quiero)
		}
		if c.quiero != "" && f.Porque == "" {
			t.Errorf("%s: una candidata sin el porqué no se puede accionar", c.nombre)
		}
	}
}

// ★ A9 — LA SALIDA NO LLAMA «UTILIDAD» A LO QUE MIDE, y dice explícitamente qué no midió.
func TestA9NoSeLlamaUtilidadALoQueNoLoEs(t *testing.T) {
	txt := FormatSkillUsage([]SkillUsageRow{
		{Skill: "plan-ahead", Resolved: 40, PorComodin: 40},
	}, nil)

	bajo := strings.ToLower(txt)
	for _, prohibida := range []string{"utilidad", "score", "puntaje", "ranking", "sirvió la skill"} {
		if strings.Contains(bajo, prohibida) {
			t.Errorf("la salida usa %q: es opinión con un número al lado", prohibida)
		}
	}
	if !strings.Contains(bajo, "no se puede medir sin juicio") {
		t.Error("la salida tiene que decir qué NO midió; si no, estos números se leen como utilidad")
	}
}
