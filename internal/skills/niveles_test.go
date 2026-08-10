package skills

import (
	"strings"
	"testing"
)

// Invariantes A1–A6 del spec «Niveles en musubi_resolve_skills» (specs/skills-por-niveles/).

// arsenalConCuerpos reproduce las cuatro formas de matchear que importan acá. Los `rules` son
// distinguibles por largo para poder razonar sobre el presupuesto.
var arsenalConCuerpos = map[string]string{
	"por-archivo": "name: por-archivo\ndescription: usá cuando toques go\ntriggers: ['*.go']\nrules: " +
		strings.Repeat("a", 100) + "\n",
	"por-fase": "name: por-fase\ndescription: planifica\ntriggers: []\napplies_to: ['phase:planning']\nrules: " +
		strings.Repeat("b", 100) + "\n",
	"solo-comodin": "name: solo-comodin\ndescription: orquesta\ntriggers: ['*']\n" +
		"always_because: 'se activa por la FORMA de la tarea, no por el archivo'\nrules: " +
		strings.Repeat("c", 100) + "\n",
	"comodin-y-glob": "name: comodin-y-glob\ndescription: revisa go\ntriggers: ['*', '*.go']\nrules: " +
		strings.Repeat("d", 100) + "\n",
}

// resolverAuto resuelve y aplica la selección por defecto, que es lo que hace la tool.
func resolverAuto(t *testing.T, r *Resolver, req ResolveRequest) map[string]SkillResuelta {
	t.Helper()
	res, err := r.ResolveConDetalle(req)
	if err != nil {
		t.Fatalf("ResolveConDetalle: %v", err)
	}
	out := map[string]SkillResuelta{}
	for _, s := range SeleccionarCuerpos(res, PresupuestoDeCuerpos) {
		out[s.Name] = s
	}
	return out
}

// A1 — MATCH POR GLOB REAL ⇒ CUERPO INCLUIDO.
//
// Un glob que nombra la extensión es evidencia de que la skill aplica: recortarle el cuerpo sería
// ahorrar justo donde no hay duda.
func TestA1GlobRealTraeCuerpo(t *testing.T) {
	r := arsenalDePrueba(t, arsenalConCuerpos)

	got := resolverAuto(t, r, ResolveRequest{ModifiedFiles: []string{"main.go"}})
	sk, ok := got["por-archivo"]
	if !ok {
		t.Fatalf("por-archivo tiene que matchear main.go; got: %v", clavesDe(got))
	}
	if sk.Matcheo != PorGlob {
		t.Errorf("matcheo = %q, se esperaba %q", sk.Matcheo, PorGlob)
	}
	if !sk.ConCuerpo {
		t.Error("una skill que matcheó por un glob real tiene que llevarse su cuerpo")
	}
}

// A2 — MATCH POR ALCANCE DECLARADO ⇒ CUERPO INCLUIDO.
//
// Es la evidencia más fuerte de las tres: el llamador dijo con todas las letras qué está haciendo.
func TestA2AlcanceDeclaradoTraeCuerpo(t *testing.T) {
	r := arsenalDePrueba(t, arsenalConCuerpos)

	got := resolverAuto(t, r, ResolveRequest{Phase: FasePlanificar})
	sk, ok := got["por-fase"]
	if !ok {
		t.Fatalf("declarar %s tiene que activar por-fase; got: %v", FasePlanificar, clavesDe(got))
	}
	if sk.Matcheo != PorAlcance {
		t.Errorf("matcheo = %q, se esperaba %q", sk.Matcheo, PorAlcance)
	}
	if !sk.ConCuerpo {
		t.Error("una skill que matcheó por el alcance declarado tiene que llevarse su cuerpo")
	}
}

// ★ A3 — MATCH SÓLO POR '*' ⇒ NIVEL 1, SIN CUERPO, PERO CON EL «CUÁNDO».
//
// Es el corazón del spec: medido sobre el arsenal real son ~1.750 tokens por resolución. La skill
// SIGUE apareciendo —no matchear y no traer cuerpo son cosas distintas— y trae la cláusula que
// permite pedirlo.
func TestA3ComodinNoTraeCuerpoPeroSiElCuando(t *testing.T) {
	r := arsenalDePrueba(t, arsenalConCuerpos)

	got := resolverAuto(t, r, ResolveRequest{ModifiedFiles: []string{"main.go"}})
	sk, ok := got["solo-comodin"]
	if !ok {
		t.Fatalf("solo-comodin tiene que seguir apareciendo: matchea cualquier archivo; got: %v", clavesDe(got))
	}
	if sk.Matcheo != PorComodin {
		t.Errorf("matcheo = %q, se esperaba %q", sk.Matcheo, PorComodin)
	}
	if sk.ConCuerpo {
		t.Error("una skill que entró sólo por su '*' no tiene evidencia: su cuerpo no viaja")
	}
	if CuandoUsarla(sk.Skill) == "" {
		t.Error("sin el «cuándo», el nivel 1 queda mudo justo donde se decide si se carga la skill")
	}
}

// A4 — LA EVIDENCIA GANA AL COMODÍN.
//
// Se clasifica por la MEJOR razón que dejó entrar a la skill, no por la primera que se evalúa:
// clasificar `['*', '*.go']` como comodín le sacaría el cuerpo sin motivo.
func TestA4PrecedenciaEvidenciaSobreComodin(t *testing.T) {
	r := arsenalDePrueba(t, arsenalConCuerpos)

	got := resolverAuto(t, r, ResolveRequest{ModifiedFiles: []string{"main.go"}})
	sk := got["comodin-y-glob"]
	if sk.Matcheo != PorGlob {
		t.Errorf("matcheo = %q, se esperaba %q: el glob real es evidencia aunque también tenga '*'", sk.Matcheo, PorGlob)
	}
	if !sk.ConCuerpo {
		t.Error("tener un '*' además de un glob real no puede costarle el cuerpo")
	}

	// Y el alcance le gana al glob: la misma skill, si además declarara alcance y el llamador lo
	// declarara, se clasifica por lo más específico.
	conAlcance := Skill{Name: "x", Triggers: []string{"*", "*.go"}, AppliesTo: []string{FaseRevisar}}
	if como, _ := clasificarMatcheo(conAlcance, ResolveRequest{
		ModifiedFiles: []string{"main.go"}, Phase: FaseRevisar,
	}); como != PorAlcance {
		t.Errorf("matcheo = %q, se esperaba %q: el alcance declarado es la evidencia más fuerte", como, PorAlcance)
	}
}

// A5 — PASADO EL PRESUPUESTO, LOS CUERPOS QUE NO ENTRAN SE OMITEN.
//
// Sin techo, la regla de evidencia sola no acota nada: tocar tres .go en un arsenal de 100 skills
// traería todos los cuerpos de '*.go'.
func TestA5ElTechoCortaAunqueHayaEvidencia(t *testing.T) {
	res := []SkillResuelta{
		{Skill: Skill{Name: "a", Rules: strings.Repeat("x", 60)}, Matcheo: PorGlob},
		{Skill: Skill{Name: "b", Rules: strings.Repeat("x", 60)}, Matcheo: PorGlob},
		{Skill: Skill{Name: "c", Rules: strings.Repeat("x", 60)}, Matcheo: PorGlob},
	}

	sel := SeleccionarCuerpos(res, 100) // entran dos de 60 sólo si el techo no se respeta
	var conCuerpo, bytes int
	for _, s := range sel {
		if s.ConCuerpo {
			conCuerpo++
			bytes += len(s.Rules)
		}
	}
	if bytes > 100 {
		t.Errorf("se devolvieron %d bytes de cuerpo con un techo de 100", bytes)
	}
	if conCuerpo != 1 {
		t.Errorf("con techo 100 y cuerpos de 60 entra exactamente uno; entraron %d", conCuerpo)
	}
	if len(sel) != 3 {
		t.Errorf("el techo recorta CUERPOS, no skills: len = %d, se esperaba 3", len(sel))
	}
}

// A6 — LA SELECCIÓN ES DETERMINISTA Y NO DEPENDE DEL ORDEN DEL DISCO.
//
// LoadSkills hereda el orden de os.ReadDir. Si la selección dependiera de él, renombrar una skill le
// sacaría el cuerpo a otra.
func TestA6SeleccionDeterministaYSinDependerDelOrden(t *testing.T) {
	base := []SkillResuelta{
		{Skill: Skill{Name: "zeta", Rules: strings.Repeat("x", 60)}, Matcheo: PorGlob},
		{Skill: Skill{Name: "alfa", Rules: strings.Repeat("x", 60)}, Matcheo: PorGlob},
	}
	invertido := []SkillResuelta{base[1], base[0]}

	elegidas := func(res []SkillResuelta) map[string]bool {
		out := map[string]bool{}
		for _, s := range SeleccionarCuerpos(res, 100) {
			if s.ConCuerpo {
				out[s.Name] = true
			}
		}
		return out
	}

	a, b := elegidas(base), elegidas(invertido)
	if len(a) != 1 || !a["alfa"] {
		t.Errorf("con empate de evidencia gana el nombre menor; elegidas: %v", a)
	}
	if len(a) != len(b) || !a["alfa"] || !b["alfa"] {
		t.Errorf("el orden de entrada cambió la selección: %v vs %v", a, b)
	}

	// Y el ORDEN DE SALIDA se preserva: la respuesta no se reordena por un detalle interno.
	sel := SeleccionarCuerpos(base, 100)
	if sel[0].Name != "zeta" || sel[1].Name != "alfa" {
		t.Errorf("la selección reordenó la respuesta: %s, %s", sel[0].Name, sel[1].Name)
	}
}

// El conjunto de skills activas NO cambia por los niveles: ResolveSkills tiene que seguir devolviendo
// exactamente lo mismo que antes de partirlo en dos. Es la red de regresión de T2.
func TestResolveSkillsNoCambioDeConjunto(t *testing.T) {
	r := arsenalDePrueba(t, arsenalConCuerpos)

	activas, err := r.ResolveSkills(ResolveRequest{ModifiedFiles: []string{"main.go"}})
	if err != nil {
		t.Fatalf("ResolveSkills: %v", err)
	}
	got := nombres(activas)
	for _, quiero := range []string{"por-archivo", "solo-comodin", "comodin-y-glob"} {
		if !got[quiero] {
			t.Errorf("falta %s en las activas: %v", quiero, got)
		}
	}
	if got["por-fase"] {
		t.Error("por-fase no declara globs: sin declarar la fase no puede activarse")
	}
}

func TestEsComodin(t *testing.T) {
	for _, c := range []struct {
		in   string
		want bool
	}{{"*", true}, {"**", true}, {" * ", true}, {"*.go", false}, {"", false}, {"main.go", false}} {
		if got := esComodin(c.in); got != c.want {
			t.Errorf("esComodin(%q) = %v, se esperaba %v", c.in, got, c.want)
		}
	}
}

func clavesDe(m map[string]SkillResuelta) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
