package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/detector"
)

// metaEnMemoria es la meta de la base, sin base: lo único que el refresco le pide.
type metaEnMemoria map[string]string

func (m metaEnMemoria) GetMeta(k string) (string, bool, error) { v, ok := m[k]; return v, ok, nil }
func (m metaEnMemoria) SetMeta(k, v string) error              { m[k] = v; return nil }

// proyectoConSkills deja un proyecto que YA eligió tener manuales de Musubi (.musubi/skills/ existe).
func proyectoConSkills(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, config.DirName, config.SkillsDir), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// refrescar corre el refresco como lo corre el arranque: con el stack detectado del proyecto.
func refrescar(t *testing.T, root string, store almacenDeManuales) ([]string, error) {
	t.Helper()
	stack, _ := detector.DetectStack(root)
	return refrescarSkillsSiHaceFalta(root, store, stack)
}

func skillMDs(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.Walk(filepath.Join(root, filepath.FromSlash(dirSkillsAgente)), func(p string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && info.Name() == "SKILL.md" {
			out = append(out, p)
		}
		return nil
	})
	return out
}

func yamlDe(root, nombre string) string {
	return filepath.Join(root, config.DirName, config.SkillsDir, nombre+".yaml")
}

// Un repo que nunca eligió tener manuales de Musubi no los recibe por arrancar una sesión: el hook
// puede correr en cualquier repo, y crearlos ahí sería imponerlos sin permiso.
func TestSkillsFrescasNoImponeManualesDondeNoLosHay(t *testing.T) {
	root := t.TempDir()
	if _, err := refrescar(t, root, metaEnMemoria{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, config.DirName)); !os.IsNotExist(err) {
		t.Fatal("no tenía que crear .musubi/ en un repo que no lo tenía")
	}
}

// EL DEFECTO: los manuales sólo se escribían con `musubi setup`. Ahora el primer arranque los escribe,
// los exporta al agente, y guarda la huella EN LA BASE —no en un archivo del árbol, que dejaba cada
// build del repo marcado como sucio—.
func TestSkillsFrescasElArranqueLosEscribe(t *testing.T) {
	root := proyectoConSkills(t)
	store := metaEnMemoria{}
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	if len(skillMDs(t, root)) == 0 {
		t.Fatal("el arranque tenía que dejar los manuales en .claude/skills/")
	}
	if store[metaHuellaSkills] == "" {
		t.Fatal("tenía que guardar la huella en la meta")
	}
	entradas, _ := os.ReadDir(filepath.Join(root, config.DirName))
	for _, e := range entradas {
		if !e.IsDir() {
			t.Errorf("el refresco dejó un archivo suelto en .musubi/: %s", e.Name())
		}
	}
}

// Con la huella igual NO se reescribe nada: corre en cada arranque y tiene que ser barato. Se prueba
// borrando un SKILL.md exportado (que NO entra en la huella): si el refresco corriera igual, lo
// volvería a escribir.
//
// Sabotaje que la pone roja: no mirar la huella guardada.
// arnes: archivo="cmd/musubi/skills_frescas.go"
// arnes: de="ok && previa == huellaSkills(root, stack) {"
// arnes: a="ok && previa == huellaSkills(root, stack) && false {"
func TestSkillsFrescasConLaMismaHuellaNoHaceNada(t *testing.T) {
	root := proyectoConSkills(t)
	store := metaEnMemoria{}
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	mds := skillMDs(t, root)
	if len(mds) == 0 {
		t.Fatal("setup: no se exportó nada")
	}
	if err := os.Remove(mds[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mds[0]); !os.IsNotExist(err) {
		t.Fatal("con la huella igual no tenía que tocar nada, y reescribió el manual borrado")
	}
}

// La huella mide QUÉ se escribiría, no QUIÉN: otro build con los mismos manuales no reescribe nada, y
// uno con otro texto sí, aunque tenga la misma versión y el mismo commit (dos builds sucios del mismo
// commit, o cualquiera desde un worktree anidado, que estampa el commit del repo padre).
//
// Sabotaje que la pone roja: volver a meter la versión del binario en la huella.
// arnes: archivo="cmd/musubi/skills_frescas.go"
// arnes: de="\th := sha256.New()\n\tfor _, sk := range cognitiveSkills(stack) {"
// arnes: a="\th := sha256.New()\n\tfmt.Fprintf(h, \"binario:%s\\n\", version)\n\tfor _, sk := range cognitiveSkills(stack) {"
func TestLaHuellaNoDependeDeQuienLaEscribe(t *testing.T) {
	root := proyectoConSkills(t)
	stack, _ := detector.DetectStack(root)
	antes := version
	t.Cleanup(func() { version = antes })
	version = "0.1.0-uno"
	h1 := huellaSkills(root, stack)
	version = "0.2.0-otro"
	if h2 := huellaSkills(root, stack); h1 != h2 {
		t.Fatalf("la huella cambió con la versión del binario sin que cambie nada de lo que escribiría: %s != %s", h1, h2)
	}
}

// Una skill que llega por OTRO camino —el arsenal que baja del central— también se exporta: el
// listado de .musubi/skills/ es parte de la huella.
func TestSkillsFrescasUnaSkillNuevaSeExporta(t *testing.T) {
	root := proyectoConSkills(t)
	store := metaEnMemoria{}
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	nueva := "name: llegada-del-central\ndescription: una skill que bajó del arsenal compartido y no vino con el binario\nrules: |\n  Hacé lo que dice esta skill.\n"
	if err := os.WriteFile(yamlDe(root, "llegada-del-central"), []byte(nueva), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	md, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(dirSkillsAgente), "llegada-del-central", "SKILL.md"))
	if err != nil {
		t.Fatalf("la skill nueva tenía que llegar al agente: %v", err)
	}
	if !strings.Contains(string(md), "Hacé lo que dice esta skill.") {
		t.Errorf("la skill llegó sin su cuerpo:\n%s", md)
	}
}

// Si el refresco falla a mitad, la huella NO se guarda: la sesión siguiente reintenta. Guardarla igual
// marcaría como hecho algo que quedó a medias, y no se volvería a intentar hasta el binario siguiente.
func TestSkillsFrescasUnFalloNoGuardaLaHuella(t *testing.T) {
	root := proyectoConSkills(t)
	// .claude/skills como ARCHIVO: exportar no puede crear el directorio.
	if err := os.MkdirAll(filepath.Join(root, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(dirSkillsAgente)), []byte("estorbo"), 0o644); err != nil {
		t.Fatal(err)
	}
	store := metaEnMemoria{}
	if _, err := refrescar(t, root, store); err == nil {
		t.Fatal("con .claude/skills bloqueado el refresco tenía que fallar")
	}
	if _, hay := store[metaHuellaSkills]; hay {
		t.Fatal("tras un fallo NO se guarda la huella, o la sesión siguiente no reintenta")
	}
}

// Lo editado a mano se preserva también por este camino (la preservación ya la probaba escribirSkillMD;
// acá se prueba que el refresco automático la usa y no la saltea).
func TestSkillsFrescasPreservaLoEditadoAMano(t *testing.T) {
	root := proyectoConSkills(t)
	store := metaEnMemoria{}
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	md := skillMDs(t, root)[0]
	editado := "# Esto lo escribió una persona\n\nY Musubi no lo puede pisar.\n"
	if err := os.WriteFile(md, []byte(editado), 0o644); err != nil {
		t.Fatal(err)
	}
	store[metaHuellaSkills] = "la-de-otro-binario" // fuerza el refresco
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(md); string(got) != editado {
		t.Fatalf("el refresco pisó un manual editado a mano:\n%s", got)
	}
}

// Un manual cognitivo que el usuario BORRÓ no resucita en el arranque siguiente —antes volvía sólo con
// `musubi setup`, y con el refresco automático volvía solo—; uno que el binario trae por PRIMERA vez,
// en cambio, sí se crea.
//
// Sabotaje que la pone roja: crear todo lo que falte, como setup.
// arnes: archivo="cmd/musubi/skills_frescas.go"
// arnes: de="escribirCognitivas(root, stack, func(nombre string) bool { return !conocidas[nombre] })"
// arnes: a="escribirCognitivas(root, stack, func(nombre string) bool { return true || !conocidas[nombre] })"
func TestSkillsFrescasUnManualBorradoNoResucita(t *testing.T) {
	root := proyectoConSkills(t)
	store := metaEnMemoria{}
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	borrado := yamlDe(root, "plan-ahead")
	if _, err := os.Stat(borrado); err != nil {
		t.Fatalf("premisa: el bundle no trajo plan-ahead.yaml: %v", err)
	}
	if err := os.Remove(borrado); err != nil {
		t.Fatal(err)
	}
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(borrado); err == nil {
		t.Fatal("el usuario borró plan-ahead.yaml y el arranque siguiente lo volvió a escribir")
	}
}

// Sabotaje que la pone roja: no recordar qué manuales se escribieron alguna vez.
// arnes: archivo="cmd/musubi/skills_frescas.go"
// arnes: de="\tguardarCognitivasConocidas(store, conocidas)\n"
// arnes: a="\t_ = guardarCognitivasConocidas\n"
func TestSkillsFrescasUnManualNuevoDelBinarioSeCrea(t *testing.T) {
	root := proyectoConSkills(t)
	store := metaEnMemoria{}
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(store[metaSkillsCognitivasConocidas], "plan-ahead") {
		t.Fatalf("el refresco tenía que recordar los manuales que escribió: %q", store[metaSkillsCognitivasConocidas])
	}
	// Como si plan-ahead lo trajera por primera vez un binario nuevo: no está ni en disco ni en la historia.
	if err := os.Remove(yamlDe(root, "plan-ahead")); err != nil {
		t.Fatal(err)
	}
	store[metaSkillsCognitivasConocidas] = strings.ReplaceAll(store[metaSkillsCognitivasConocidas], `"plan-ahead",`, "")
	store[metaSkillsCognitivasConocidas] = strings.ReplaceAll(store[metaSkillsCognitivasConocidas], `,"plan-ahead"`, "")
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(yamlDe(root, "plan-ahead")); err != nil {
		t.Fatalf("un manual que el binario trae por primera vez tenía que crearse: %v", err)
	}
}

// Un binario MÁS VIEJO que el que escribió los manuales no los toca: con dos versiones conviviendo
// sobre el mismo proyecto, cada una reescribía lo de la otra en cada arranque.
//
// Sabotaje que la pone roja: no mirar quién los escribió.
// arnes: archivo="cmd/musubi/skills_frescas.go"
// arnes: de="ok && escritaPorUnoMasNuevo(registrada, version) {"
// arnes: a="ok && escritaPorUnoMasNuevo(registrada, version) && false {"
func TestSkillsFrescasUnBinarioViejoNoTocaLoDeUnoNuevo(t *testing.T) {
	root := proyectoConSkills(t)
	store := metaEnMemoria{}
	antes := version
	t.Cleanup(func() { version = antes })
	version = "0.150.0-main.aaaaaaa"
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	md := skillMDs(t, root)[0]
	if err := os.Remove(md); err != nil {
		t.Fatal(err)
	}
	store[metaHuellaSkills] = "otra" // algo cambió: un binario igual o más nuevo refrescaría
	version = "0.140.0-main.bbbbbbb"
	if _, err := refrescar(t, root, store); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(md); err == nil {
		t.Fatal("un binario más viejo reescribió lo que había dejado uno más nuevo")
	}
	// Un build sin versión (`dev`) no se compara: no se inventa un orden.
	if escritaPorUnoMasNuevo("0.150.0-main.aaaaaaa", "dev") || escritaPorUnoMasNuevo("dev", "0.1.0") {
		t.Error("con una versión sin núcleo no hay orden que comparar")
	}
}

// La escritura atómica no deja temporales: si quedaran, un .tmp suelto por cada manual refrescado
// se iría acumulando en .claude/skills/.
func TestEscribirArchivoAtomicoNoDejaTemporales(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "SKILL.md")
	for _, contenido := range []string{"primera versión\n", "segunda versión\n"} {
		if err := escribirArchivoAtomico(ruta, []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(ruta); string(got) != contenido {
			t.Fatalf("quería %q, quedó %q", contenido, got)
		}
	}
	entradas, _ := os.ReadDir(dir)
	for _, e := range entradas {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("quedó un temporal: %s", e.Name())
		}
	}
	if len(entradas) != 1 {
		t.Errorf("tenía que quedar un solo archivo, hay %d", len(entradas))
	}
}

// Si la escritura falla, el temporal se limpia. En el caso feliz el rename lo consume y no queda nada
// que probar; lo que importa es el fallo. Acá el destino es un DIRECTORIO, así que el rename no puede
// completarse.
func TestEscribirArchivoAtomicoLimpiaElTemporalSiFalla(t *testing.T) {
	dir := t.TempDir()
	destino := filepath.Join(dir, "soy-un-directorio")
	if err := os.MkdirAll(filepath.Join(destino, "adentro"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := escribirArchivoAtomico(destino, []byte("x"), 0o644); err == nil {
		t.Fatal("renombrar sobre un directorio no vacío tenía que fallar")
	}
	entradas, _ := os.ReadDir(dir)
	for _, e := range entradas {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("el fallo dejó un temporal suelto: %s", e.Name())
		}
	}
}

// El DISPARADOR: el hook de arranque (`musubi detect --hook-mode` → detectOutput) de verdad refresca,
// con una base de verdad. Sin esto el refresco podía quedar escrito, probado y desconectado —el modo
// de falla de siempre—, y ninguna de las pruebas de arriba lo vería.
func TestElHookDeArranqueRefrescaLosManuales(t *testing.T) {
	root := proyectoConSkills(t)
	if _, err := detectOutput(root, true, "sesion-de-prueba"); err != nil {
		t.Fatal(err)
	}
	if len(skillMDs(t, root)) == 0 {
		t.Fatal("el hook de arranque no dejó los manuales en .claude/skills/")
	}
}
