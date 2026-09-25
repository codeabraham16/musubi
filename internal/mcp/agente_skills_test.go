package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory/memtest"
	"musubi/internal/skills"
)

// sembrarSkill escribe una skill en .musubi/skills y, si exportada, su SKILL.md en el formato del
// agente. El contenido del SKILL.md no importa acá: el mapa sólo pregunta si existe.
func sembrarSkill(t *testing.T, root, nombre string, alcances []string, exportada bool) {
	t.Helper()
	dir := filepath.Join(root, ".musubi", "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	yaml := "name: " + nombre + "\ndescription: prueba\ntriggers: ['*']\napplies_to: [" + strings.Join(alcances, ", ") + "]\nrules: hacé algo\n"
	if err := os.WriteFile(filepath.Join(dir, nombre+".yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	if !exportada {
		return
	}
	ruta := skills.RutaSkillAgente(root, nombre)
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, []byte("---\nname: "+nombre+"\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// renglonDe devuelve el renglón del mapa que corresponde a un alcance, o "".
func renglonDe(texto, alcance string) string {
	frase := "cuando " + skills.FraseDeAlcance(alcance) + " →"
	for _, l := range strings.Split(texto, "\n") {
		if strings.Contains(l, frase) {
			return l
		}
	}
	return ""
}

// EL MAPA NOMBRA SÓLO SKILLS QUE EL AGENTE PUEDE CARGAR, Y DE CADA ALCANCE LA MÁS ESPECÍFICA.
//
// Las dos reglas cuidan lo mismo desde lados distintos: que el renglón mande al agente a algo que
// existe y que es lo que corresponde. Una skill sin SKILL.md no se puede cargar con la tool Skill
// —es un callejón, como una tool dormida—; y una skill que declara varios alcances (sdd-flow) no
// puede tapar a la que declara uno solo (plan-ahead) en el renglón de ese uno.
//
// LOS NOMBRES ESTÁN ELEGIDOS POR SU ORDEN ALFABÉTICO, que es el orden en que el resolvedor las lee:
// en auditar la amplia (aflujo-a) llega ANTES que la específica, y en planificar DESPUÉS (zflujo-p).
// Son los dos caminos del código —reemplazar lo que había y descartar lo que llega— y cada uno tiene
// su sabotaje. Con un solo orden, uno de los dos quedaba sin medir.
//
// Sabotaje que la hace fallar: nombrar también las skills que no están exportadas.
// arnes: archivo="internal/mcp/agente.go"
// arnes: de="\t\t\tif _, err := os.Stat(skills.RutaSkillAgente(s.projectPath, r.Name)); err != nil {"
// arnes: a="\t\t\tif _, err := os.Stat(skills.RutaSkillAgente(s.projectPath, r.Name)); false && err != nil {"
//
// Sabotaje que la hace fallar: sumar la que llega después aunque sea más amplia.
// arnes: archivo="internal/mcp/agente.go"
// arnes: de="\t\t\tcase n == mejor:"
// arnes: a="\t\t\tcase true:"
//
// Sabotaje que la hace fallar: no reemplazar la amplia cuando llega una más específica.
// arnes: archivo="internal/mcp/agente.go"
// arnes: de="\t\t\tcase mejor == 0 || n < mejor:"
// arnes: a="\t\t\tcase mejor == 0:"
func TestElMapaNombraSkillsCargablesYLasMasEspecificas(t *testing.T) {
	root := t.TempDir()
	sembrarSkill(t, root, "aflujo-a", []string{skills.TareaAuditar, skills.TareaOrquestar}, true)
	sembrarSkill(t, root, "auditar-x", []string{skills.TareaAuditar}, true)
	sembrarSkill(t, root, "planear-y", []string{skills.FasePlanificar}, true)
	sembrarSkill(t, root, "zflujo-p", []string{skills.FasePlanificar, skills.FaseImplementar}, true)
	sembrarSkill(t, root, "revisar-z", []string{skills.FaseRevisar}, false) // sin SKILL.md

	s := NewMcpServer(memtest.NuevoEngine(t, t.TempDir()), root, embedding.NoopProvider{}, WithInstruccionesParaElAgente())
	texto := s.instruccionesParaElAgente()
	if !strings.Contains(texto, encabezadoDelMapa) {
		t.Fatalf("las instrucciones no traen el mapa de skills:\n%s", texto)
	}

	quiere := map[string]string{
		skills.TareaAuditar:    "auditar-x", // la amplia llegó antes y la específica la reemplaza
		skills.FasePlanificar:  "planear-y", // la amplia llega después y se descarta
		skills.TareaOrquestar:  "aflujo-a",  // donde la amplia es la única, queda
		skills.FaseImplementar: "zflujo-p",
	}
	for alcance, skill := range quiere {
		if l := renglonDe(texto, alcance); !strings.HasSuffix(l, "→ "+skill) {
			t.Errorf("el renglón de %s tiene que nombrar SÓLO %s: %q", alcance, skill, l)
		}
	}
	if strings.Contains(texto, "revisar-z") {
		t.Errorf("el mapa nombra revisar-z, que no tiene SKILL.md: el agente no la puede cargar")
	}
	if l := renglonDe(texto, skills.FaseRevisar); l != "" {
		t.Errorf("la única skill de revisar no está exportada y el mapa tiene su renglón: %q", l)
	}
}

// SIN SKILLS, LAS INSTRUCCIONES QUEDAN COMO ANTES. Un proyecto sin .musubi/skills no recibe un
// encabezado de mapa vacío que le prometa skills que no hay.
func TestSinSkillsNoHayMapa(t *testing.T) {
	s := servidorQueLeHablaAlAgente(t)
	if texto := s.instruccionesParaElAgente(); texto != instruccionesAgente {
		t.Errorf("sin skills las instrucciones cambiaron:\n%s", texto)
	}
}

// EL MAPA NO PASA EL TOPE, Y LO QUE NO ENTRA NO SE MANDA A MEDIAS.
//
// Pasarse no falla en ningún lado: Claude Code corta y sigue, y el corte cae a la mitad de un
// renglón. Con cincuenta renglones largos, el texto tiene que caber y terminar en un renglón entero.
//
// Sabotaje que la hace fallar: agregar renglones sin mirar el tope.
// arnes: archivo="internal/mcp/agente.go"
// arnes: de="\t\tif largoParaElCliente(candidato) > topeInstrucciones {"
// arnes: a="\t\tif false {"
func TestElMapaNoPasaElTope(t *testing.T) {
	var renglones []string
	for i := 0; i < 50; i++ {
		renglones = append(renglones, "- cuando estés haciendo algo muy específico número "+strings.Repeat("x", 30)+" → una-skill-con-nombre-largo")
	}
	texto := conMapaDeSkills(instruccionesAgente, renglones)
	if n := largoParaElCliente(texto); n > topeInstrucciones {
		t.Fatalf("el texto mide %d y el tope es %d: Claude Code le corta el final", n, topeInstrucciones)
	}
	lineas := strings.Split(texto, "\n")
	if ultima := lineas[len(lineas)-1]; ultima != renglones[0] {
		// todos los renglones son iguales: el último tiene que ser uno entero
		t.Errorf("el texto termina en un renglón incompleto: %q", ultima)
	}
	if !strings.HasPrefix(texto, instruccionesAgente) {
		t.Error("el mapa desplazó a las instrucciones de base: lo que no entra es el mapa, no la memoria")
	}
}
