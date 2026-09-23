package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

// proyectoConSkills deja un proyecto que YA eligió tener manuales de Musubi (.musubi/skills/ existe).
func proyectoConSkills(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, config.DirName, config.SkillsDir), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
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

// Un repo que nunca eligió tener manuales de Musubi no los recibe por arrancar una sesión: el hook
// puede correr en cualquier repo, y crearlos ahí sería imponerlos sin permiso.
func TestSkillsFrescasNoImponeManualesDondeNoLosHay(t *testing.T) {
	root := t.TempDir()
	if _, err := refrescarSkillsSiHaceFalta(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, config.DirName)); !os.IsNotExist(err) {
		t.Fatal("no tenía que crear .musubi/ en un repo que no lo tenía")
	}
}

// EL DEFECTO: los manuales sólo se escribían con `musubi setup`. Ahora el primer arranque los escribe
// y los exporta al agente.
func TestSkillsFrescasElArranqueLosEscribe(t *testing.T) {
	root := proyectoConSkills(t)
	if _, err := refrescarSkillsSiHaceFalta(root); err != nil {
		t.Fatal(err)
	}
	if len(skillMDs(t, root)) == 0 {
		t.Fatal("el arranque tenía que dejar los manuales en .claude/skills/")
	}
	if _, err := os.Stat(filepath.Join(root, config.DirName, archivoHuellaSkills)); err != nil {
		t.Fatalf("tenía que guardar la huella: %v", err)
	}
}

// Con la huella igual NO se reescribe nada: corre en cada arranque y tiene que ser barato. Se prueba
// borrando un SKILL.md exportado (que NO entra en la huella): si el refresco corriera igual, lo
// volvería a escribir.
func TestSkillsFrescasConLaMismaHuellaNoHaceNada(t *testing.T) {
	root := proyectoConSkills(t)
	if _, err := refrescarSkillsSiHaceFalta(root); err != nil {
		t.Fatal(err)
	}
	mds := skillMDs(t, root)
	if len(mds) == 0 {
		t.Fatal("setup: no se exportó nada")
	}
	if err := os.Remove(mds[0]); err != nil {
		t.Fatal(err)
	}
	if _, err := refrescarSkillsSiHaceFalta(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mds[0]); !os.IsNotExist(err) {
		t.Fatal("con la huella igual no tenía que tocar nada, y reescribió el manual borrado")
	}
}

// Un binario NUEVO reescribe. `version` vale "dev" en todo build local, así que la huella incluye el
// commit del build: con la versión sola, el refresco no se dispararía nunca donde más se compila. Acá
// se simula el binario nuevo cambiando `version`, que es la parte de la huella que una prueba puede mover.
func TestSkillsFrescasUnBinarioNuevoReescribe(t *testing.T) {
	root := proyectoConSkills(t)
	if _, err := refrescarSkillsSiHaceFalta(root); err != nil {
		t.Fatal(err)
	}
	mds := skillMDs(t, root)
	if err := os.Remove(mds[0]); err != nil {
		t.Fatal(err)
	}
	antes := version
	version = "binario-nuevo-de-prueba"
	t.Cleanup(func() { version = antes })
	if _, err := refrescarSkillsSiHaceFalta(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mds[0]); err != nil {
		t.Fatalf("con un binario nuevo tenía que volver a escribir el manual: %v", err)
	}
}

// Una skill que llega por OTRO camino —el arsenal que baja del central— también se exporta: el
// listado de .musubi/skills/ es parte de la huella.
func TestSkillsFrescasUnaSkillNuevaSeExporta(t *testing.T) {
	root := proyectoConSkills(t)
	if _, err := refrescarSkillsSiHaceFalta(root); err != nil {
		t.Fatal(err)
	}
	nueva := "name: llegada-del-central\ndescription: una skill que bajó del arsenal compartido y no vino con el binario\ninstructions: |\n  Hacé lo que dice esta skill.\n"
	if err := os.WriteFile(filepath.Join(root, config.DirName, config.SkillsDir, "llegada-del-central.yaml"), []byte(nueva), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := refrescarSkillsSiHaceFalta(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(dirSkillsAgente), "llegada-del-central", "SKILL.md")); err != nil {
		t.Fatalf("la skill nueva tenía que llegar al agente: %v", err)
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
	if _, err := refrescarSkillsSiHaceFalta(root); err == nil {
		t.Fatal("con .claude/skills bloqueado el refresco tenía que fallar")
	}
	if _, err := os.Stat(filepath.Join(root, config.DirName, archivoHuellaSkills)); !os.IsNotExist(err) {
		t.Fatal("tras un fallo NO se guarda la huella, o la sesión siguiente no reintenta")
	}
}

// Lo editado a mano se preserva también por este camino nuevo (la preservación ya la probaba
// escribirSkillMD; acá se prueba que el refresco automático la usa y no la saltea).
func TestSkillsFrescasPreservaLoEditadoAMano(t *testing.T) {
	root := proyectoConSkills(t)
	if _, err := refrescarSkillsSiHaceFalta(root); err != nil {
		t.Fatal(err)
	}
	md := skillMDs(t, root)[0]
	editado := "# Esto lo escribió una persona\n\nY Musubi no lo puede pisar.\n"
	if err := os.WriteFile(md, []byte(editado), 0o644); err != nil {
		t.Fatal(err)
	}
	antes := version
	version = "otro-binario-de-prueba" // fuerza el refresco
	t.Cleanup(func() { version = antes })
	if _, err := refrescarSkillsSiHaceFalta(root); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(md); string(got) != editado {
		t.Fatalf("el refresco pisó un manual editado a mano:\n%s", got)
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

// El commit del build entra en la huella, y es la razón de no usar la versión sola: en un build local
// `version` vale "dev" SIEMPRE, así que con la versión sola dos compilaciones distintas darían la misma
// huella y el refresco no se dispararía nunca en la máquina donde más se compila.
func TestSkillsFrescasUnCommitNuevoReescribeAunqueLaVersionSeaDev(t *testing.T) {
	antesV, antesID := version, identidadDelBuild
	version = "dev"
	identidadDelBuild = func() string { return "commit-uno|false" }
	t.Cleanup(func() { version, identidadDelBuild = antesV, antesID })

	root := proyectoConSkills(t)
	if _, err := refrescarSkillsSiHaceFalta(root); err != nil {
		t.Fatal(err)
	}
	md := skillMDs(t, root)[0]
	if err := os.Remove(md); err != nil {
		t.Fatal(err)
	}
	identidadDelBuild = func() string { return "commit-dos|false" } // mismo "dev", otro commit
	if _, err := refrescarSkillsSiHaceFalta(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(md); err != nil {
		t.Fatalf("con otro commit y la misma versión \"dev\" tenía que reescribir: %v", err)
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
