package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// trigger_honesto_test.go sella el contrato del spec «trigger-honesto» del lado del paquete
// skills: el campo always_because y el detector de wildcard.
//
// El invariante que más importa es G6. WildcardUnjustified es la ÚNICA definición de «'*' sin
// declarar», y la consumen dos lugares con severidades OPUESTAS: warning en el score, error en la
// puerta de promoción. Si se duplicara la lógica, el día que una se corrija la otra seguiría
// juzgando distinto la misma skill — y el síntoma sería «la advertencia decía que estaba bien pero
// el central la rechazó», que es exactamente el tipo de contradicción que nadie sabe depurar.

// razonValido es una justificación que supera el piso de MinAlwaysBecauseChars.
const razonValido = "se activa por tipo de tarea (orquestar), no por archivo"

// skillCon arma una skill mínima que pasa el gate, con los triggers y la razón que se le pidan.
func skillCon(triggers []string, razon string) Skill {
	return Skill{
		Name:          "hacer-algo",
		Description:   "Hace algo util. Use when se necesita hacer algo.",
		Triggers:      triggers,
		Rules:         "Hacer la cosa bien.\n\n```sh\nhacer --bien\n```",
		AlwaysBecause: razon,
	}
}

// warnings devuelve los códigos de warning emitidos, como conjunto.
func warnings(s Skill) map[string]bool {
	out := map[string]bool{}
	for _, w := range ValidateSkillQuality(s).Warnings {
		out[w.Code] = true
	}
	return out
}

// TestG1ElCampoSobreviveAlDisco — si always_because no se persiste, todo lo demás es teatro:
// la razón se declararía una vez y desaparecería al releer la skill.
func TestG1ElCampoSobreviveAlDisco(t *testing.T) {
	dir := t.TempDir()
	skillsDir := filepath.Join(dir, ".musubi", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	yaml := "name: orquestar\n" +
		"description: Coordina agentes. Use when se orquesta trabajo en paralelo.\n" +
		"triggers:\n  - \"*\"\n" +
		"always_because: " + razonValido + "\n" +
		"rules: |\n  Dividir el trabajo y verificar cada parte.\n"
	if err := os.WriteFile(filepath.Join(skillsDir, "orquestar.yaml"), []byte(yaml), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cargadas, err := NewResolver(dir).LoadSkills()
	if err != nil {
		t.Fatalf("LoadSkills: %v", err)
	}
	if len(cargadas) != 1 {
		t.Fatalf("esperaba 1 skill, obtuve %d", len(cargadas))
	}
	if cargadas[0].AlwaysBecause != razonValido {
		t.Fatalf("always_because no sobrevivió al disco: %q", cargadas[0].AlwaysBecause)
	}
}

// TestG3DeclararElMotivoApagaLaAdvertencia — el punto entero del campo. Si el warning siguiera
// encendido, declarar la razón sería burocracia sin efecto y nadie lo haría.
func TestG3DeclararElMotivoApagaLaAdvertencia(t *testing.T) {
	sin := warnings(skillCon([]string{"*"}, ""))
	if !sin["triggers_over_broad"] {
		t.Fatalf("un '*' sin declarar DEBE advertir; warnings=%v", sin)
	}
	con := warnings(skillCon([]string{"*"}, razonValido))
	if con["triggers_over_broad"] {
		t.Errorf("declarar el motivo debe apagar triggers_over_broad; warnings=%v", con)
	}
	// Y el score tiene que reflejarlo: si la penalización quedara igual, el campo no sirve
	// para nada en la práctica.
	if a, b := ValidateSkillQuality(skillCon([]string{"*"}, razonValido)).Score,
		ValidateSkillQuality(skillCon([]string{"*"}, "")).Score; a <= b {
		t.Errorf("declarar el motivo debe SUBIR el score: con=%d sin=%d", a, b)
	}
}

// TestG4UnaJustificacionDeRellenoNoAlcanza — un campo que se satisface con dos palabras no
// cambia ninguna conducta; sólo agrega un trámite.
func TestG4UnaJustificacionDeRellenoNoAlcanza(t *testing.T) {
	for _, relleno := range []string{"porque si", "", "   ", "aplica"} {
		if !WildcardUnjustified(skillCon([]string{"*"}, relleno)) {
			t.Errorf("%q (%d chars) no debería alcanzar como justificación", relleno, len([]rune(relleno)))
		}
	}
	if WildcardUnjustified(skillCon([]string{"*"}, razonValido)) {
		t.Errorf("una razón de %d caracteres SÍ debería alcanzar", len([]rune(razonValido)))
	}
}

// TestG5ElCasoEnmascaradoSeNombraAparte — ["*", "*.go"] es PEOR que ["*"]: los específicos hacen
// parecer que la skill está acotada cuando el '*' ya la activó en todo. La versión anterior del
// detector (allWildcard) se lo perdía entero.
func TestG5ElCasoEnmascaradoSeNombraAparte(t *testing.T) {
	mezclado := warnings(skillCon([]string{"*", "*.go"}, ""))
	if !mezclado["triggers_wildcard_masks_specific"] {
		t.Fatalf("un '*' mezclado con específicos debe advertirse aparte; warnings=%v", mezclado)
	}
	// Y la razón NO lo excusa: justifica el '*', no vuelve honesto al adorno.
	conRazon := warnings(skillCon([]string{"*", "*.go"}, razonValido))
	if !conRazon["triggers_wildcard_masks_specific"] {
		t.Errorf("always_because no debe apagar el caso enmascarado; warnings=%v", conRazon)
	}
	// Control: el caso honesto (sólo '*') NO emite este código.
	if warnings(skillCon([]string{"*"}, ""))["triggers_wildcard_masks_specific"] {
		t.Errorf("['*'] a secas no está enmascarando nada")
	}
}

// TestG6UnPredicadoDosConsumidores — el predicado exportado y el warning tienen que coincidir
// SIEMPRE. Es la prueba de que hay una sola definición y no dos que puedan divergir.
func TestG6UnPredicadoDosConsumidores(t *testing.T) {
	casos := []struct {
		triggers []string
		razon    string
	}{
		{[]string{"*"}, ""},
		{[]string{"*"}, razonValido},
		{[]string{"*"}, "corto"},
		{[]string{"*.go"}, ""},
		{[]string{"*.go"}, razonValido},
		{[]string{"*", "*.go"}, ""},
		{[]string{"*", "*.go"}, razonValido},
		{nil, ""},
	}
	for _, c := range casos {
		sk := skillCon(c.triggers, c.razon)
		if got, want := warnings(sk)["triggers_over_broad"], WildcardUnjustified(sk); got != want {
			t.Errorf("triggers=%v razon=%q: warning=%v pero WildcardUnjustified=%v — las dos vistas divergieron",
				c.triggers, c.razon, got, want)
		}
	}
}

// TestG10LoAcotadoNuncaSeToca — es la mayoría del arsenal y no debe pagar por un problema que
// no tiene. Control de todo lo de arriba: sin esto, romper el detector para que dispare siempre
// dejaría los demás tests en verde.
func TestG10LoAcotadoNuncaSeToca(t *testing.T) {
	w := warnings(skillCon([]string{"*.go", "*.mod"}, ""))
	if w["triggers_over_broad"] || w["triggers_wildcard_masks_specific"] {
		t.Fatalf("una skill acotada no debe ver ninguno de los dos chequeos; warnings=%v", w)
	}
	if WildcardUnjustified(skillCon([]string{"*.go"}, "")) {
		t.Errorf("['*.go'] sin razón no es un wildcard injustificado")
	}
	// Y un '*.go' no es un '*' aunque lo contenga como subcadena.
	if anyWildcard([]string{"*.go", "**/*.ts"}) {
		t.Errorf("anyWildcard no debe matchear globs que sólo EMPIEZAN con '*'")
	}
	// Espacios alrededor sí cuentan: " * " es un '*' escrito con descuido, no otro glob.
	if !anyWildcard([]string{" * "}) {
		t.Errorf("un '*' con espacios sigue siendo un '*'")
	}
}

// TestElMensajeDiceComoArreglarlo — un hallazgo sin fix accionable convierte la herramienta en
// adivinanza, que es el defecto que la Fase B arregló en install_skill.
func TestElMensajeDiceComoArreglarlo(t *testing.T) {
	for _, w := range ValidateSkillQuality(skillCon([]string{"*"}, "")).Warnings {
		if w.Code != "triggers_over_broad" {
			continue
		}
		if !strings.Contains(w.Fix, "always_because") {
			t.Fatalf("el fix debe nombrar el campo que resuelve el hallazgo: %q", w.Fix)
		}
		return
	}
	t.Fatal("no se emitió triggers_over_broad")
}
