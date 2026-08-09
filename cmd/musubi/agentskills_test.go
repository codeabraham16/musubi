package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/skills"
)

// Invariantes del spec «El arsenal se usa» (specs/arsenal-que-se-usa/).

// arsenalEnDisco escribe skills YAML en <root>/.musubi/skills y devuelve root.
func arsenalEnDisco(t *testing.T, yamls map[string]string) string {
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
	return root
}

func leerSkillMD(t *testing.T, root, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".claude", "skills", name, "SKILL.md"))
	if err != nil {
		t.Fatalf("leer SKILL.md de %s: %v", name, err)
	}
	return string(b)
}

// E1 — LA DESCRIPCIÓN EXPORTADA LLEVA EL «CUÁNDO».
//
// En SKILL.md la selección la hace el consumidor leyendo la description. Una que sólo dice QUÉ hace
// deja a la skill sin forma de ser elegida: el archivo existiría y no serviría de nada.
func TestE1LaDescripcionLlevaElCuando(t *testing.T) {
	sk := skills.Skill{
		Name: "plan-ahead", Description: "Planea antes de actuar.",
		Triggers: []string{"*"}, AppliesTo: []string{skills.FasePlanificar}, Rules: "r",
	}
	got := skills.DescripcionParaAgente(sk)
	if !strings.Contains(strings.ToLower(got), "cuando") {
		t.Fatalf("la descripción exportada tiene que decir cuándo usarla; quedó %q", got)
	}
	if !strings.Contains(got, "Planea antes de actuar.") {
		t.Errorf("el QUÉ original no puede perderse; quedó %q", got)
	}
}

// E2 — EL «CUÁNDO» SALE DE applies_to, Y SI NO DE always_because, Y SI NO DE LOS GLOBS.
//
// En ese orden: applies_to es vocabulario cerrado (traduce a una frase exacta), always_because es
// prosa que el autor escribió a propósito, los globs son el último recurso mecánico.
func TestE2ElOrdenDeLasFuentes(t *testing.T) {
	casos := []struct {
		nombre string
		sk     skills.Skill
		quiero string
	}{
		{"applies_to gana", skills.Skill{
			AppliesTo: []string{skills.TareaAuditar}, AlwaysBecause: "no me uses a mí", Triggers: []string{"*.go"},
		}, "auditar"},
		{"sin applies_to manda always_because", skills.Skill{
			AlwaysBecause: "el disparador es el momento", Triggers: []string{"*.go"},
		}, "el disparador es el momento"},
		{"último recurso: los globs", skills.Skill{
			Triggers: []string{"*.go", "*.ts"},
		}, "*.go"},
		{"sin nada, no se inventa", skills.Skill{Triggers: []string{"*"}}, ""},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := skills.CuandoUsarla(c.sk)
			if c.quiero == "" {
				if got != "" {
					t.Errorf("sin fuentes no se inventa nada, quedó %q", got)
				}
				return
			}
			if !strings.Contains(got, c.quiero) {
				t.Errorf("CuandoUsarla = %q, esperaba que contuviera %q", got, c.quiero)
			}
		})
	}
}

// E3 — UNA DESCRIPCIÓN QUE YA DICE CUÁNDO NO SE DUPLICA.
//
// go-hygiene empieza con «Usá cuando escribas o revises…». Anteponerle otro «Usá cuando…» daría una
// frase rota y —peor— sería el sistema pisando lo que una persona ya escribió bien.
func TestE3NoSeDuplicaElCuando(t *testing.T) {
	original := "Usá cuando escribas o revises cualquier archivo .go: errores, panic y go vet."
	got := skills.DescripcionParaAgente(skills.Skill{
		Name: "go-hygiene", Description: original, Triggers: []string{"*.go"},
	})
	if got != original {
		t.Errorf("una descripción que ya dice cuándo se devuelve intacta;\n  quedó:  %q\n  quería: %q", got, original)
	}
	if strings.Count(strings.ToLower(got), "usá cuando") != 1 {
		t.Error("quedaron dos cláusulas «usá cuando»: se duplicó")
	}
}

// E10 — EL CONECTOR DEPENDE DE LA FUENTE, Y LA FRASE TIENE QUE CERRAR.
//
// Lo encontró mirar la salida real, no un test: `always_because` es prosa escrita para explicarle a
// UNA PERSONA por qué la skill lleva '*' («gobierna el flujo completo de un cambio»), y anteponerle
// «Usá cuando» produce «Usá cuando gobierna el flujo completo», que está roto. Una descripción rota
// es exactamente lo único que decide si la skill se activa.
func TestE10ElConectorDependeDeLaFuente(t *testing.T) {
	// always_because ⇒ «Cuándo: …», nunca «Usá cuando …».
	got := skills.DescripcionParaAgente(skills.Skill{
		Name: "sdd-flow", Description: "Conduce features.",
		Triggers: []string{"*"}, AlwaysBecause: "gobierna el flujo completo de un cambio",
	})
	if strings.Contains(got, "Usá cuando gobierna") {
		t.Errorf("prosa de always_because pegada a «Usá cuando»: frase rota. Quedó %q", got)
	}
	if !strings.HasPrefix(got, "Cuándo: gobierna el flujo completo de un cambio.") {
		t.Errorf("esperaba el conector «Cuándo:»; quedó %q", got)
	}

	// applies_to ⇒ frases escritas PARA seguir a «Usá cuando».
	got = skills.DescripcionParaAgente(skills.Skill{
		Name: "plan-ahead", Description: "Planea.",
		Triggers: []string{"*"}, AppliesTo: []string{skills.FasePlanificar},
	})
	if !strings.HasPrefix(got, "Usá cuando estés planificando") {
		t.Errorf("con applies_to la frase compone con «Usá cuando»; quedó %q", got)
	}
}

// E4 — EL CUERPO VIAJA COMPLETO. Las rules SON la skill: un export que las recorte entrega un
// archivo con cara de funcionar.
func TestE4ElCuerpoViajaCompleto(t *testing.T) {
	reglas := "línea uno\nlínea dos con `código`\n\n- viñeta\n- otra"
	md, err := skills.ASkillMD(skills.Skill{
		Name: "x", Description: "usá cuando pruebes", Rules: reglas,
	}, "abc")
	if err != nil {
		t.Fatalf("ASkillMD: %v", err)
	}
	for _, frag := range strings.Split(reglas, "\n") {
		if frag == "" {
			continue
		}
		if !strings.Contains(md, frag) {
			t.Errorf("el cuerpo perdió %q", frag)
		}
	}
}

// E5 — EL NOMBRE Y LA RUTA SON LOS QUE EL AGENTE ESPERA: .claude/skills/<name>/SKILL.md.
// Verificado contra una skill que ya funciona en esta máquina, no contra la documentación.
func TestE5LaRutaEsLaQueElAgenteEspera(t *testing.T) {
	root := arsenalEnDisco(t, map[string]string{
		"mi-skill": "name: mi-skill\ndescription: usá cuando pruebes\ntriggers: ['*.go']\nrules: cuerpo\n",
	})
	if _, err := exportarSkillsAlAgente(root); err != nil {
		t.Fatalf("exportar: %v", err)
	}
	ruta := filepath.Join(root, ".claude", "skills", "mi-skill", "SKILL.md")
	if _, err := os.Stat(ruta); err != nil {
		t.Fatalf("esperaba %s: %v", ruta, err)
	}
	md := leerSkillMD(t, root, "mi-skill")
	if !strings.HasPrefix(md, "---\nname: mi-skill\n") {
		t.Errorf("el frontmatter tiene que abrir con --- y name; quedó:\n%s", md[:min(80, len(md))])
	}
}

// E6 — ES IDEMPOTENTE. Sin esto, cada setup reescribiría todo y ensuciaría cualquier diff o watcher.
func TestE6ExportarEsIdempotente(t *testing.T) {
	root := arsenalEnDisco(t, map[string]string{
		"a": "name: a\ndescription: usá cuando pruebes\ntriggers: ['*.go']\nrules: r\n",
	})
	if _, err := exportarSkillsAlAgente(root); err != nil {
		t.Fatalf("1ra: %v", err)
	}
	ruta := filepath.Join(root, ".claude", "skills", "a", "SKILL.md")
	info1, err := os.Stat(ruta)
	if err != nil {
		t.Fatal(err)
	}
	antes := leerSkillMD(t, root, "a")

	rep, err := exportarSkillsAlAgente(root)
	if err != nil {
		t.Fatalf("2da: %v", err)
	}
	if len(rep.Escritas) != 0 {
		t.Errorf("la 2da pasada no debería escribir nada, escribió %v", rep.Escritas)
	}
	info2, _ := os.Stat(ruta)
	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Error("el archivo se reescribió sin haber cambiado nada")
	}
	if leerSkillMD(t, root, "a") != antes {
		t.Error("el contenido cambió entre dos exports idénticos: no es determinista")
	}
}

// E7 — UN SKILL.md EDITADO A MANO SE PRESERVA. Regla de oro: ante la mínima duda, preservar.
func TestE7LoEditadoAManoSePreserva(t *testing.T) {
	root := arsenalEnDisco(t, map[string]string{
		"a": "name: a\ndescription: usá cuando pruebes\ntriggers: ['*.go']\nrules: original\n",
	})
	if _, err := exportarSkillsAlAgente(root); err != nil {
		t.Fatalf("export inicial: %v", err)
	}
	ruta := filepath.Join(root, ".claude", "skills", "a", "SKILL.md")
	editado := leerSkillMD(t, root, "a") + "\n\nESTO LO ESCRIBIÓ UNA PERSONA\n"
	if err := os.WriteFile(ruta, []byte(editado), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := exportarSkillsAlAgente(root)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if !strings.Contains(leerSkillMD(t, root, "a"), "ESTO LO ESCRIBIÓ UNA PERSONA") {
		t.Fatal("Musubi pisó una edición a mano")
	}
	var preservada bool
	for _, n := range rep.Preservadas {
		if n == "a" {
			preservada = true
		}
	}
	if !preservada {
		t.Error("preservar en silencio no alcanza: tiene que reportarse")
	}
}

// E8 — UN SKILL.md AJENO NO SE ADOPTA. Sin checksum de Musubi no es de Musubi: puede ser una skill
// que el usuario instaló a mano con el mismo nombre, y pisarla sería reclamar trabajo ajeno.
func TestE8LoAjenoNoSeAdopta(t *testing.T) {
	root := arsenalEnDisco(t, map[string]string{
		"a": "name: a\ndescription: usá cuando pruebes\ntriggers: ['*.go']\nrules: de-musubi\n",
	})
	dir := filepath.Join(root, ".claude", "skills", "a")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	ajena := "---\nname: a\ndescription: la instaló el usuario\n---\n\ncuerpo ajeno\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(ajena), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := exportarSkillsAlAgente(root); err != nil {
		t.Fatalf("export: %v", err)
	}
	if leerSkillMD(t, root, "a") != ajena {
		t.Error("un SKILL.md sin checksum de Musubi no se puede pisar ni adoptar")
	}
}

// E9 — UNA SKILL BORRADA DEL ORIGEN SE BORRA DEL EXPORT.
//
// Es el caso starter.yaml: una huérfana que nadie mantiene sigue costando contexto en cada turno.
// Sólo se retira lo que Musubi escribió y sigue INTACTO; lo editado a mano cae en E7 y sobrevive.
func TestE9LasHuerfanasSeRetiran(t *testing.T) {
	root := arsenalEnDisco(t, map[string]string{
		"queda":  "name: queda\ndescription: usá cuando pruebes\ntriggers: ['*.go']\nrules: r\n",
		"se-va":  "name: se-va\ndescription: usá cuando pruebes\ntriggers: ['*.go']\nrules: r\n",
		"tocada": "name: tocada\ndescription: usá cuando pruebes\ntriggers: ['*.go']\nrules: r\n",
	})
	if _, err := exportarSkillsAlAgente(root); err != nil {
		t.Fatalf("export inicial: %v", err)
	}
	// A 'tocada' la edita una persona; después desaparece del origen igual que 'se-va'.
	rutaTocada := filepath.Join(root, ".claude", "skills", "tocada", "SKILL.md")
	if err := os.WriteFile(rutaTocada, []byte(leerSkillMD(t, root, "tocada")+"\nMÍO\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"se-va", "tocada"} {
		if err := os.Remove(filepath.Join(root, ".musubi", "skills", n+".yaml")); err != nil {
			t.Fatal(err)
		}
	}

	rep, err := exportarSkillsAlAgente(root)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "se-va")); !os.IsNotExist(err) {
		t.Error("la huérfana intacta tenía que retirarse")
	}
	if len(rep.Retiradas) != 1 || rep.Retiradas[0] != "se-va" {
		t.Errorf("Retiradas = %v, esperaba [se-va]", rep.Retiradas)
	}
	if _, err := os.Stat(rutaTocada); err != nil {
		t.Error("una huérfana EDITADA A MANO no se borra: la regla de oro gana")
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "queda")); err != nil {
		t.Error("la que sigue viva no se puede tocar")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
