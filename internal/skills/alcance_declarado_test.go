package skills

import (
	"os"
	"path/filepath"
	"testing"
)

// Invariantes del spec «El porqué del `*` se vuelve alcance» (specs/alcance-declarado/).

// arsenalDePrueba escribe skills en <dir>/.musubi/skills y devuelve un Resolver.
// Sin capabilities: verifyCapabilities mira el PATH y no es lo que estas pruebas miden.
func arsenalDePrueba(t *testing.T, yamls map[string]string) *Resolver {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".musubi", "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	for name, body := range yamls {
		if err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(body), 0o644); err != nil {
			t.Fatalf("escribir %s: %v", name, err)
		}
	}
	return NewResolver(root)
}

func nombres(ss []Skill) map[string]bool {
	out := map[string]bool{}
	for _, s := range ss {
		out[s.Name] = true
	}
	return out
}

// arsenalMixto reproduce la forma real del arsenal: una skill por archivo, una por fase, una por
// tarea, y una que declara alcance y ADEMÁS conserva su comodín (el caso migrado).
var arsenalMixto = map[string]string{
	"por-archivo": "name: por-archivo\ndescription: usá cuando toques go\ntriggers: ['*.go']\nrules: r\n",
	"por-fase":    "name: por-fase\ndescription: planifica\ntriggers: []\napplies_to: ['phase:planning']\nrules: r\n",
	"por-tarea":   "name: por-tarea\ndescription: audita\ntriggers: []\napplies_to: ['task:audit']\nrules: r\n",
	"migrada":     "name: migrada\ndescription: revisa\ntriggers: ['*']\napplies_to: ['phase:reviewing']\nrules: r\n",
}

// A1 — DECLARAR UNA FASE ALCANZA PARA RESOLVER, sin un solo archivo.
//
// Es el corazón del spec: hoy esa llamada era un error de parámetros, y por eso una skill que se
// activa por la fase del trabajo no tenía cómo ser encontrada salvo declarando '*'.
func TestA1DeclararFaseResuelveSinArchivos(t *testing.T) {
	r := arsenalDePrueba(t, arsenalMixto)

	activas, err := r.ResolveSkills(ResolveRequest{Phase: FasePlanificar})
	if err != nil {
		t.Fatalf("ResolveSkills: %v", err)
	}
	got := nombres(activas)
	if !got["por-fase"] {
		t.Errorf("declarar %s tiene que activar por-fase; activas: %v", FasePlanificar, got)
	}
	if got["por-archivo"] {
		t.Error("sin archivos, una skill de globs no puede activarse")
	}
}

// A2 — SIN ARCHIVOS Y SIN DECLARACIÓN SIGUE SIENDO UNA PETICIÓN VACÍA.
//
// La validación no se afloja: se le agrega una segunda forma de satisfacerla. Una llamada que no
// dice NADA no tiene respuesta útil, y devolver el arsenal entero sería peor que fallar.
func TestA2PeticionVaciaSigueSiendoVacia(t *testing.T) {
	if (ResolveRequest{}).DeclaraAlgo() {
		t.Error("una petición sin archivos, sin fase y sin tarea no declara nada")
	}
	for _, r := range []ResolveRequest{
		{ModifiedFiles: []string{"main.go"}},
		{Phase: FasePlanificar},
		{Task: TareaAuditar},
	} {
		if !r.DeclaraAlgo() {
			t.Errorf("%+v sí declara algo", r)
		}
	}
}

// A3 — LOS GLOBS SIGUEN RESOLVIENDO IGUAL. Red de regresión: 5 de las 11 skills reales viven de sus
// globs y este spec no puede tocarlas.
func TestA3LosGlobsNoCambian(t *testing.T) {
	r := arsenalDePrueba(t, arsenalMixto)

	activas, err := r.ResolveSkills(ResolveRequest{ModifiedFiles: []string{"main.go"}})
	if err != nil {
		t.Fatalf("ResolveSkills: %v", err)
	}
	got := nombres(activas)
	if !got["por-archivo"] {
		t.Error("un .go tiene que activar la skill de '*.go'")
	}
	if got["por-fase"] || got["por-tarea"] {
		t.Errorf("sin declaración, las skills de alcance no deben activarse; activas: %v", got)
	}
}

// A4 — EL COMODÍN SIGUE SIENDO COMODÍN.
//
// Si migrar a applies_to le sacara el match por archivo, las 5 skills migradas dejarían de
// activarse mientras ningún llamador declare su fase — un arreglo que se ve como regresión.
func TestA4MigrarNoLeSacaElComodin(t *testing.T) {
	r := arsenalDePrueba(t, arsenalMixto)

	activas, err := r.ResolveSkills(ResolveRequest{ModifiedFiles: []string{"cualquier.txt"}})
	if err != nil {
		t.Fatalf("ResolveSkills: %v", err)
	}
	if !nombres(activas)["migrada"] {
		t.Error("una skill con '*' Y applies_to tiene que seguir matcheando por archivo")
	}
}

// A5 — SÓLO MATCHEA LO DECLARADO. Igualdad exacta: sin prefijos, sin heurística, sin distancia. Es
// lo que mantiene el matcher determinista y gratis.
func TestA5SoloMatcheaLoDeclarado(t *testing.T) {
	r := arsenalDePrueba(t, arsenalMixto)

	activas, err := r.ResolveSkills(ResolveRequest{Phase: FasePlanificar})
	if err != nil {
		t.Fatalf("ResolveSkills: %v", err)
	}
	if nombres(activas)["migrada"] {
		t.Error("declarar phase:planning no puede activar una skill de phase:reviewing")
	}
	if nombres(activas)["por-tarea"] {
		t.Error("una fase no puede activar una skill de tarea")
	}

	// Y un prefijo no alcanza: 'phase:plan' NO es 'phase:planning'.
	activas, err = r.ResolveSkills(ResolveRequest{Phase: "phase:plan"})
	if err != nil {
		t.Fatalf("ResolveSkills: %v", err)
	}
	if nombres(activas)["por-fase"] {
		t.Error("el match tiene que ser por igualdad exacta, no por prefijo")
	}
}

// A6 — UN VALOR FUERA DEL VOCABULARIO ES ERROR DE VALIDACIÓN.
//
// Vocabulario cerrado, como validOutcome del ledger: un typo se vería igual que una skill que nunca
// aplica —silencioso—, y por eso nadie lo encontraría.
func TestA6AlcanceDesconocidoEsError(t *testing.T) {
	rep := ValidateSkillQuality(Skill{
		Name:        "con-typo",
		Description: "usá cuando planifiques algo",
		Triggers:    []string{"*.go"},
		AppliesTo:   []string{"phase:planing"}, // typo deliberado
		Rules:       "r",
	})
	var visto bool
	for _, e := range rep.Errors {
		if e.Code == "applies_to_desconocido" {
			visto = true
		}
	}
	if !visto {
		t.Fatalf("un applies_to fuera del vocabulario tiene que ser ERROR; errores: %+v", rep.Errors)
	}
	if rep.OK() {
		t.Error("con un error, el reporte no puede dar OK")
	}

	// Y el vocabulario bueno no puede dar error.
	for _, v := range VocabularioDeAlcance() {
		rep := ValidateSkillQuality(Skill{
			Name: "ok", Description: "usá cuando corresponda", Triggers: []string{"*.go"},
			AppliesTo: []string{v}, Rules: "r",
		})
		for _, e := range rep.Errors {
			if e.Code == "applies_to_desconocido" {
				t.Errorf("%q está en el vocabulario y fue rechazado", v)
			}
		}
	}
}

// A7 — DECLARAR NO ROMPE A QUIEN NO DECLARA. Los dos caminos son independientes.
func TestA7LosDosCaminosSonIndependientes(t *testing.T) {
	r := arsenalDePrueba(t, arsenalMixto)

	// Una skill SIN applies_to nunca matchea por declaración...
	activas, err := r.ResolveSkills(ResolveRequest{Phase: FasePlanificar, Task: TareaAuditar})
	if err != nil {
		t.Fatalf("ResolveSkills: %v", err)
	}
	if nombres(activas)["por-archivo"] {
		t.Error("una skill sin applies_to no puede activarse por una declaración")
	}

	// ...y declarar no la excluye de matchear por sus globs.
	activas, err = r.ResolveSkills(ResolveRequest{ModifiedFiles: []string{"main.go"}, Phase: FasePlanificar})
	if err != nil {
		t.Fatalf("ResolveSkills: %v", err)
	}
	got := nombres(activas)
	if !got["por-archivo"] || !got["por-fase"] {
		t.Errorf("con archivos Y fase tienen que entrar las dos; activas: %v", got)
	}
}

// A8 — EL VALIDADOR NO APRUEBA POR SUBCADENA.
//
// Caso REAL: adversarial-review pasaba desc_no_trigger porque la lista busca "al " y su descripción
// dice «revisión advers-AL- estilo debate». Un token de 3 caracteres buscado adentro de las palabras
// aprueba casi cualquier texto en castellano, y dejaba mudo al warning en la skill que más lo
// necesitaba.
func TestA8ElValidadorNoApruebaPorSubcadena(t *testing.T) {
	sinCuando := ValidateSkillQuality(Skill{
		Name:        "adversarial-review",
		Description: "Revisión adversarial estilo debate: escépticos con lentes distintos refutan un cambio en rondas.",
		Triggers:    []string{"*"},
		Rules:       "r",
	})
	var avisado bool
	for _, w := range sinCuando.Warnings {
		if w.Code == "desc_no_trigger" {
			avisado = true
		}
	}
	if !avisado {
		t.Error("«adversarial estilo» no dice CUÁNDO usar la skill: el 'al ' de adversariAL no puede aprobarla")
	}

	// Y la frase legítima al inicio de palabra tiene que seguir aprobando.
	conCuando := ValidateSkillQuality(Skill{
		Name:        "go-hygiene",
		Description: "Usá cuando escribas o revises cualquier archivo .go: errores, panic y la puerta de go vet.",
		Triggers:    []string{"*.go"},
		Rules:       "r",
	})
	for _, w := range conCuando.Warnings {
		if w.Code == "desc_no_trigger" {
			t.Error("«Usá cuando …» sí dice cuándo: el arreglo no puede volverse un falso negativo")
		}
	}
}
