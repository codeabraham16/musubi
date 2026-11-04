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

// UNA CARGA SE CUENTA, Y ES UN EJE DISTINTO DE «MATCHEÓ».
//
// `loaded` lo escribe el hook PostToolUse(Skill): el agente abrió la skill por la tool, el camino
// que el resolver no ve (spec skills-medir-cargas). Tiene que sumar en Loaded SIN tocar Resolved, y
// entrar con evidence vacío (como body_requested).
//
// Sabotaje que la hace fallar: sacar `case UsoCargada: r.Loaded += n` del switch de SkillUsage
// (Loaded queda en 0), o sacar `|| k == UsoCargada` de kindValido (la fila ni se escribe).
// arnes: archivo="internal/memory/skillusage.go" de="r.Loaded += n" a="r.Loaded += 0"
func TestUnaCargaSeCuentaEnSuPropioEje(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	if !kindValido(UsoCargada) {
		t.Fatal("UsoCargada tiene que ser un kind válido de la taxonomía cerrada")
	}
	for i := 0; i < 3; i++ {
		contarSkill(t, e, ctx, SkillEvent{Skill: "sdd-flow", Evidence: "", Kind: UsoCargada})
	}

	rows, _ := e.SkillUsage(ctx)
	r, ok := filaDe(rows, "sdd-flow")
	if !ok {
		t.Fatal("una carga tiene que dejar su fila")
	}
	if r.Loaded != 3 {
		t.Errorf("esperaba loaded=3, obtuve %d", r.Loaded)
	}
	if r.Resolved != 0 || r.BodySent != 0 || r.BodyRequested != 0 {
		t.Errorf("una carga NO es una resolución: esperaba los otros ejes en 0, obtuve %+v", r)
	}
}

// UN `loaded` SOBREVIVE AUNQUE VENGA UN KIND BASURA EN EL MISMO LOTE —y un kind que se PARECE a
// loaded pero no lo es se saltea igual (la taxonomía es exacta, no por prefijo).
func TestUnaCargaSobreviveAlLadoDeUnKindInvalido(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	contarSkill(t, e, ctx,
		SkillEvent{Skill: "cargada", Evidence: "", Kind: UsoCargada},
		SkillEvent{Skill: "basura", Evidence: "", Kind: "loaded_y_sirvio"},
	)

	rows, _ := e.SkillUsage(ctx)
	if r, ok := filaDe(rows, "cargada"); !ok || r.Loaded != 1 {
		t.Errorf("la carga buena tiene que llegar: ok=%v fila=%+v", ok, r)
	}
	if _, ok := filaDe(rows, "basura"); ok {
		t.Error("un kind inválido no puede escribirse aunque se parezca a loaded")
	}
}

// UNA SKILL QUE EL AGENTE CARGÓ NO ESTÁ «MUERTA», aunque el resolver nunca la haya matcheado.
//
// loaded y resolved son ejes disjuntos: una skill de `.claude/skills/` vive en loaded. Llamar
// «muerta» a algo que se cargó sería mentir, y la mentira saldría el instante en que el hook corre.
//
// Sabotaje que la hace fallar: sacar `&& r.Loaded == 0` del caso «muerta» de MarcarCandidata.
// arnes: archivo="internal/memory/skillusage.go" de="r.BodyRequested == 0 && r.Loaded == 0" a="r.BodyRequested == 0"
func TestUnaSkillCargadaNoEstaMuerta(t *testing.T) {
	cargada := SkillUsageRow{Skill: "x", Loaded: 5}
	MarcarCandidata(&cargada)
	if cargada.Candidata == "muerta" {
		t.Errorf("una skill cargada %d veces no está muerta; quedó %q (%s)",
			cargada.Loaded, cargada.Candidata, cargada.Porque)
	}

	// La «muerta» de verdad sigue siéndolo: sin ninguna señal, de ningún eje. (El guard no rompió
	// el caso real.)
	sinNada := SkillUsageRow{Skill: "y"}
	MarcarCandidata(&sinNada)
	if sinNada.Candidata != "muerta" {
		t.Errorf("sin ninguna señal tiene que seguir siendo muerta, quedó %q", sinNada.Candidata)
	}
}

// UNA SKILL QUE EL AGENTE CARGA NO ES CANDIDATA A «RETIRO», aunque resuelva sin que su cuerpo viaje.
//
// «Retiro» (matcheó y nadie abrió su cuerpo) sugiere SACAR la skill. Pero una que el agente carga con
// la tool Skill está en uso por el eje loaded, que es independiente del cuerpo del resolver: proponer
// retirarla sería un consejo falso. Es la misma incoherencia que R12 cerró para «muerta», aplicada al
// otro patrón que acciona sobre la skill (R13).
//
// Sabotaje que la hace fallar: sacar `&& r.Loaded == 0` del caso «retiro» de MarcarCandidata; la skill
// cargada volvería a salir rotulada «retiro».
// arnes: archivo="internal/memory/skillusage.go" de="r.Resolved > 0 && r.BodySent == 0 && r.BodyRequested == 0 && r.Loaded == 0" a="r.Resolved > 0 && r.BodySent == 0 && r.BodyRequested == 0"
func TestUnaSkillCargadaNoEsCandidataARetiro(t *testing.T) {
	// Resuelve y su cuerpo nunca viajó (el disparador de «retiro»), pero el agente la carga a mano.
	cargada := SkillUsageRow{Skill: "x", Resolved: 5, Loaded: 10}
	MarcarCandidata(&cargada)
	if cargada.Candidata == "retiro" {
		t.Errorf("una skill cargada %d veces está en uso: no es candidata a retiro; quedó %q (%s)",
			cargada.Loaded, cargada.Candidata, cargada.Porque)
	}

	// El “retiro” de verdad sigue siéndolo: MISMA fila (mismos contadores), pero con Loaded=0. (El
	// guard no rompió el caso real —el mismo que cubre TestA10LosTresPatrones, re-afirmado acá al lado
	// de su negación—.)
	sinCarga := SkillUsageRow{Skill: "x", Resolved: 5}
	MarcarCandidata(&sinCarga)
	if sinCarga.Candidata != "retiro" {
		t.Errorf("resuelve, su cuerpo no viajó y no se cargó: tiene que ser retiro, quedó %q", sinCarga.Candidata)
	}
}

// EL REPORTE MUESTRA «CARGÓ» por skill, sin llamarlo utilidad (R9, R11).
func TestElReporteMuestraLasCargas(t *testing.T) {
	txt := FormatSkillUsage([]SkillUsageRow{
		{Skill: "sdd-flow", Loaded: 7},
	}, nil)
	if !strings.Contains(txt, "CARGÓ") {
		t.Error("el encabezado tiene que nombrar la columna de cargas")
	}
	if !strings.Contains(txt, "7") {
		t.Error("el conteo de cargas tiene que aparecer en la fila")
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
