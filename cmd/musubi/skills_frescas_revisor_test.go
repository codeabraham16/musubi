package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"musubi/internal/config"
	"musubi/internal/detector"
	"musubi/internal/guiones"
	"musubi/internal/skills"
)

// Regresiones de la revisión adversarial de este cambio (tres lentes), adaptadas a la huella en la base.

// Un nombre de skill con ".." escapaba de .claude/skills: el arranque escribía un SKILL.md FUERA del
// proyecto, porque LoadSkills sólo exige name != "" y el export hace Join(destino, sk.Name).
//
// Sabotaje que la pone roja: exportar sin mirar el nombre.
// arnes: archivo="cmd/musubi/agentskills.go"
// arnes: de="\t\tif !skills.NombreSeguro(sk.Name) {"
// arnes: a="\t\tif !skills.NombreSeguro(sk.Name) && false {"
func TestSkillsFrescasNombreConPuntosNoEscapaDeClaudeSkills(t *testing.T) {
	root := proyectoConSkills(t)
	yml := "name: ../../../fuera-de-root\ndescription: una skill cuyo nombre escala directorios\nrules: |\n  x\n"
	if err := os.WriteFile(yamlDe(root, "rara"), []byte(yml), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _ = refrescar(t, root, metaEnMemoria{})
	if fuera := filepath.Join(filepath.Dir(root), "fuera-de-root", "SKILL.md"); fileExists(fuera) {
		t.Fatalf("el arranque de sesión escribió un SKILL.md FUERA del proyecto: %s", fuera)
	}
	rep, err := exportarSkillsAlAgente(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Omitidas) != 1 || rep.Omitidas[0] != "../../../fuera-de-root" {
		t.Errorf("el export tiene que decir qué no exportó y por qué: omitidas=%q", rep.Omitidas)
	}
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// Una skill que llega a .musubi/skills/ DESPUÉS de que el export la leyó pero ANTES de guardar la
// huella quedaba registrada como procesada y no se exportaba nunca. Ahora la huella se toma antes del
// export: lo que llega en esa ventana la cambia y se exporta en la sesión siguiente.
//
// Sabotaje que la pone roja: tomar la huella después del export.
// arnes: archivo="cmd/musubi/skills_frescas.go"
// arnes: de="\tescritas = append(escritas, rep.Escritas...)\n"
// arnes: a="\tescritas = append(escritas, rep.Escritas...)\n\thuella = huellaSkills(root, stack)\n"
func TestSkillsFrescasSkillQueLlegaDuranteElRefrescoSeExportaDespues(t *testing.T) {
	root := proyectoConSkills(t)
	// 300 skills de relleno: el export tarda y la ventana LoadSkills → fin se ensancha.
	for i := 0; i < 300; i++ {
		yml := fmt.Sprintf("name: relleno-%03d\ndescription: relleno para ensanchar la ventana\nrules: |\n  x\n", i)
		if err := os.WriteFile(yamlDe(root, fmt.Sprintf("relleno-%03d", i)), []byte(yml), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	claudeSkills := filepath.Join(root, filepath.FromSlash(dirSkillsAgente))
	tarde := yamlDe(root, "llego-tarde")
	hecho := make(chan struct{})
	go func() {
		defer close(hecho)
		limite := time.Now().Add(20 * time.Second)
		for time.Now().Before(limite) {
			// MkdirAll(.claude/skills) corre DESPUÉS de LoadSkills: si ya existe, el listado ya se leyó.
			if _, err := os.Stat(claudeSkills); err == nil {
				_ = os.WriteFile(tarde, []byte("name: llego-tarde\ndescription: la instaló otra terminal en ese instante\nrules: |\n  x\n"), 0o644)
				return
			}
		}
	}()
	store := metaEnMemoria{}
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	<-hecho
	if _, err := refrescar(t, root, store); err != nil { // la sesión siguiente
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(claudeSkills, "llego-tarde", "SKILL.md")); err != nil {
		t.Fatalf("la skill que llegó durante el refresco quedó sin exportar y la huella la da por procesada: %v", err)
	}
}

// El stack decide el texto de los manuales cognitivos (sus triggers): un proyecto que pasa a tener Go
// tiene que ver los manuales con los triggers de Go, sin esperar a otro binario.
//
// Sabotaje que la pone roja: sacar los manuales cognitivos de la huella.
// arnes: archivo="cmd/musubi/skills_frescas.go"
// arnes: de="\t\tfmt.Fprintf(h, \"cognitiva:%s|%s|%t\\n\", sk.Name, sum, err != nil)"
// arnes: a="\t\t_, _ = sum, err"
func TestSkillsFrescasUnCambioDeStackRefrescaLosManuales(t *testing.T) {
	root := proyectoConSkills(t)
	store := metaEnMemoria{}
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module ejemplo\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(yamlDe(root, "analyze-project"))
	if !strings.Contains(string(got), "*.go") {
		t.Fatalf("el stack cambió a Go y el manual siguió con los triggers viejos:\n%s", got)
	}
}

// Con otra huella registrada (la de un binario con otro texto de manual), el manual manejado e intacto
// se reescribe con el texto de este binario, aunque los dos compartan versión y commit.
func TestSkillsFrescasOtroTextoDeManualSeReescribe(t *testing.T) {
	root := proyectoConSkills(t)
	store := metaEnMemoria{}
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	ruta := yamlDe(root, "analyze-project")
	actual, _ := os.ReadFile(ruta)
	var sk = cognitiveSkillsPorNombre(t, root, "analyze-project")
	sk.Description = "TEXTO DEL BINARIO A"
	sum, err := skillContentChecksum(sk)
	if err != nil {
		t.Fatal(err)
	}
	sk.ManagedChecksum = sum
	escribirYAML(t, ruta, sk)
	store[metaHuellaSkills] = "la-huella-del-binario-A"
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(ruta); string(stripCR(got)) != string(stripCR(actual)) {
		t.Fatalf("el manual quedó con el texto del otro binario:\n%s", got)
	}
}

// El refresco del arranque no puede ensuciar el árbol del repo en el que corre: `deploy/construir.sh`
// estampa `-sucio` con cualquier línea de `git status --porcelain`. Con el .gitignore REAL de este repo,
// lo que el refresco escribe tiene que quedar ignorado.
//
// Sabotaje que la pone roja: volver a guardar la huella en un archivo de .musubi/.
// arnes: archivo="cmd/musubi/skills_frescas.go"
// arnes: de="\tif err := store.SetMeta(metaHuellaSkills, huella); err != nil {"
// arnes: a="\t_ = os.WriteFile(filepath.Join(root, config.DirName, \"skills-huella\"), []byte(huella), 0o644)\n\tif err := store.SetMeta(metaHuellaSkills, huella); err != nil {"
func TestSkillsFrescasNoEnsucianElArbolDelRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("sin git en el PATH")
	}
	gitignore, err := os.ReadFile(filepath.Join("..", "..", ".gitignore"))
	if err != nil {
		t.Fatalf("leer el .gitignore del repo: %v", err)
	}
	root := proyectoConSkills(t)
	gitSinHook(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), gitignore, 0o644); err != nil {
		t.Fatal(err)
	}
	if antes := gitSinHook(t, root, "status", "--porcelain", "--untracked-files=all", "--", config.DirName, ".claude"); strings.TrimSpace(antes) != "" {
		t.Fatalf("premisa: el árbol ya estaba sucio:\n%s", antes)
	}
	if _, err := refrescar(t, root, metaEnMemoria{}); err != nil {
		t.Fatal(err)
	}
	if despues := gitSinHook(t, root, "status", "--porcelain", "--untracked-files=all", "--", config.DirName, ".claude"); strings.TrimSpace(despues) != "" {
		t.Fatalf("el refresco de arranque dejó el árbol sucio (construir.sh estamparía -sucio):\n%s", despues)
	}
}

// gitSinHook corre git sin heredar GIT_DIR/GIT_WORK_TREE del proceso (un hook pre-push que corre las
// pruebas los exporta, y git miraría el repo equivocado).
func gitSinHook(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := guiones.Herramienta(t, "git", append([]string{"-C", dir}, args...)...)
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, "GIT_") {
			cmd.Env = append(cmd.Env, kv)
		}
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// En Windows, os.Rename sobre un destino que otro proceso Go tiene abierto (os.Open no pide
// FILE_SHARE_DELETE) falla con «Access is denied»; os.WriteFile, lo que había antes, no fallaba.
//
// Sabotaje que la pone roja: sin el último recurso de escribir en el lugar.
// arnes: archivo="cmd/musubi/catalog.go"
// arnes: de="\t\tif bloqueoDeWindows(err) {\n\t\t\tif werr := os.WriteFile(path, data, perm); werr == nil {"
// arnes: a="\t\tif bloqueoDeWindows(err) && false {\n\t\t\tif werr := os.WriteFile(path, data, perm); werr == nil {"
func TestEscribirArchivoAtomicoConElDestinoAbiertoEnWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("sólo Windows")
	}
	ruta := filepath.Join(t.TempDir(), "analyze-project.yaml")
	if err := os.WriteFile(ruta, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(ruta) // otra terminal leyendo el manual en ese instante
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := escribirArchivoAtomico(ruta, []byte("v3\n"), 0o644); err != nil {
		t.Fatalf("la escritura atómica falla donde os.WriteFile andaba: %v", err)
	}
	if got, _ := os.ReadFile(ruta); string(got) != "v3\n" {
		t.Fatalf("quedó %q", got)
	}
}

// Un manual que es un symlink sigue siéndolo: se escribe junto a su objetivo. Renombrar sobre el
// enlace lo reemplazaba por un archivo común y el objetivo quedaba viejo.
//
// Sabotaje que la pone roja: no resolver el enlace.
// arnes: archivo="cmd/musubi/catalog.go"
// arnes: de="if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 {"
// arnes: a="if fi, err := os.Lstat(path); err == nil && fi.Mode()&os.ModeSymlink != 0 && false {"
func TestEscribirArchivoAtomicoRespetaUnSymlink(t *testing.T) {
	dir := t.TempDir()
	objetivo := filepath.Join(dir, "compartido.yaml")
	if err := os.WriteFile(objetivo, []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	enlace := filepath.Join(dir, "enlace.yaml")
	if err := os.Symlink(objetivo, enlace); err != nil {
		t.Skipf("sin permiso para crear symlinks: %v", err)
	}
	if err := escribirArchivoAtomico(enlace, []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Lstat(enlace)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("el enlace quedó reemplazado por un archivo común")
	}
	if got, _ := os.ReadFile(objetivo); string(got) != "v2\n" {
		t.Fatalf("el objetivo del enlace no se actualizó: %q", got)
	}
}

func cognitiveSkillsPorNombre(t *testing.T, root, nombre string) skills.Skill {
	t.Helper()
	stack, _ := detector.DetectStack(root)
	for _, sk := range cognitiveSkills(stack) {
		if sk.Name == nombre {
			return sk
		}
	}
	t.Fatalf("no encontré %s en el bundle cognitivo", nombre)
	return skills.Skill{}
}

func escribirYAML(t *testing.T, ruta string, sk skills.Skill) {
	t.Helper()
	data, err := yaml.Marshal(sk)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
