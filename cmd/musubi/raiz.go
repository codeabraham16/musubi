package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"musubi/internal/config"
)

// raiz.go — EN QUÉ PROYECTO TRABAJA UN PROCESO QUE ARRANCA SOLO, O QUE NO HAY NINGUNO.
//
// Los procesos que nadie lanza a mano —el daemon del MCP y los cuatro hooks (detect, turn,
// precheck, capture)— los arranca el agente, en la carpeta donde se abrió la sesión. Hasta ahora
// tomaban esa carpeta tal cual (workspaceDir: MUSUBI_HOME, después CLAUDE_PROJECT_DIR, después el
// cwd) y cuatro de los cinco CREABAN `.musubi/` si no estaba. Mientras Musubi se cableaba repo por
// repo eso casi no se veía: sólo arrancaba donde alguien había corrido `musubi setup`. Con Musubi
// registrado a nivel usuario arranca en CUALQUIER carpeta, y crear ahí es plantar memorias
// fantasma: así nació `~/.musubi` (A96), con sólo correr un comando desde el home.
//
// La regla, en orden:
//
//  1. MUSUBI_HOME manda, como siempre: es una decisión explícita de alguien.
//  2. El ancestro más cercano que YA TIENE memoria (.musubi/config.yaml). Cubre abrir la sesión en
//     una subcarpeta y los worktrees que viven adentro del repo (.claude/worktrees/…): usan la
//     memoria del proyecto, no una propia.
//  3. Un worktree enlazado que vive AFUERA del repo usa la memoria del repo principal, si la tiene.
//  4. La raíz de un repo git sin memoria: Musubi se ACTIVA ahí. Es el «por defecto»: el agente
//     tiene Musubi en cualquier proyecto sin que nadie configure nada.
//  5. Nada de lo anterior: no hay proyecto, y el proceso queda INERTE. No crea nada.
//
// Y EL HOME NUNCA ES UN PROYECTO, en ninguno de los pasos: ni su `.musubi` (la sombra) ni un repo
// git en el home (los dotfiles). Un `.musubi/` que sólo trae el config.example.yaml versionado
// tampoco cuenta como memoria: lo que la define es el config.yaml.

// raizResuelta es la decisión, con su porqué.
type raizResuelta struct {
	// Dir es la raíz del proyecto. "" ⇒ no hay proyecto y el proceso queda inerte.
	Dir string
	// Motivo dice de dónde salió Dir, o por qué no hay ninguno. Viaja al agente en el modo inerte.
	Motivo string
	// Activar dice que Dir todavía no tiene memoria y que el daemon tiene que crearla. Los hooks
	// NUNCA la crean: si el daemon todavía no activó el proyecto, se callan.
	Activar bool
}

// raizDelProceso resuelve la raíz de un proceso que arrancó solo (daemon o hook).
func raizDelProceso() raizResuelta {
	if home := os.Getenv("MUSUBI_HOME"); home != "" {
		return raizResuelta{Dir: home, Motivo: "MUSUBI_HOME", Activar: !tieneMemoria(home)}
	}
	base := os.Getenv("CLAUDE_PROJECT_DIR")
	if base == "" {
		base, _ = os.Getwd()
	}
	home, _ := os.UserHomeDir()
	return resolverRaiz(base, home)
}

// resolverRaiz es la regla, con sus dos entradas explícitas para poder probarla sin tocar el
// entorno del proceso.
func resolverRaiz(base, home string) raizResuelta {
	if base == "" {
		return raizResuelta{Motivo: "no sé en qué carpeta arrancó la sesión"}
	}
	if abs, err := filepath.Abs(base); err == nil {
		base = abs
	}
	for d := base; ; {
		if !mismoDir(d, home) && tieneMemoria(d) {
			return raizResuelta{Dir: d, Motivo: "el proyecto con memoria que contiene la carpeta de la sesión"}
		}
		padre := filepath.Dir(d)
		if padre == d {
			break
		}
		d = padre
	}
	top, ok := raizGit(base)
	if !ok {
		return raizResuelta{Motivo: "no es un repo git ni está adentro de un proyecto con memoria"}
	}
	if mismoDir(top, home) {
		return raizResuelta{Motivo: "la carpeta personal no es un proyecto"}
	}
	if principal, ok := principalDelWorktree(top); ok && !mismoDir(principal, home) && tieneMemoria(principal) {
		return raizResuelta{Dir: principal, Motivo: "el repo principal de este worktree"}
	}
	return raizResuelta{Dir: top, Motivo: "un repo git que todavía no tenía memoria", Activar: true}
}

// tieneMemoria dice si dir es un proyecto de Musubi: tiene su config.yaml. Un `.musubi/` sin él
// —el que trae sólo el config.example.yaml versionado— no lo es.
func tieneMemoria(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, config.DirName, config.ConfigFile))
	return err == nil && !st.IsDir()
}

// raizGit devuelve la raíz del repo git que contiene dir: la carpeta más cercana hacia arriba que
// tiene un `.git` (directorio en un clon, archivo en un worktree enlazado). Sin ejecutar git: esto
// corre en cada hook, con un techo de 10 s.
func raizGit(dir string) (string, bool) {
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return d, true
		}
		padre := filepath.Dir(d)
		if padre == d {
			return "", false
		}
		d = padre
	}
}

// principalDelWorktree devuelve la raíz del repo principal si top es un worktree ENLAZADO. En uno
// así `.git` es un archivo «gitdir: <repo>/.git/worktrees/<nombre>», y esa carpeta tiene un
// `commondir` que apunta al `.git` compartido; la raíz del principal es la carpeta que lo contiene.
func principalDelWorktree(top string) (string, bool) {
	crudo, err := os.ReadFile(filepath.Join(top, ".git"))
	if err != nil {
		return "", false // un directorio (clon normal) o nada: no es un worktree enlazado
	}
	linea := strings.TrimSpace(string(crudo))
	if !strings.HasPrefix(linea, "gitdir:") {
		return "", false
	}
	gitdir := strings.TrimSpace(strings.TrimPrefix(linea, "gitdir:"))
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(top, gitdir)
	}
	comun, err := os.ReadFile(filepath.Join(gitdir, "commondir"))
	if err != nil {
		return "", false
	}
	dirComun := strings.TrimSpace(string(comun))
	if !filepath.IsAbs(dirComun) {
		dirComun = filepath.Join(gitdir, dirComun)
	}
	dirComun = filepath.Clean(dirComun)
	if filepath.Base(dirComun) != ".git" {
		return "", false // un repo bare: no hay carpeta de trabajo principal
	}
	return filepath.Dir(dirComun), true
}

// mismoDir compara dos rutas como carpetas. En Windows sin distinguir mayúsculas, como el disco.
func mismoDir(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// activarMemoria crea la memoria de un proyecto que todavía no la tenía: el `.musubi/` con su
// config.yaml por defecto (ensureWorkspace) y, SÓLO si la carpeta la creamos nosotros, un
// `.gitignore` adentro que la ignora entera. Así activarse solo no ensucia el `git status` de un
// repo ajeno ni toca su `.gitignore`; y un `.musubi/` que ya existía —con archivos versionados, como
// las skills de Altura— no recibe uno que los escondería.
func activarMemoria(root string) error {
	dir := filepath.Join(root, config.DirName)
	_, errAntes := os.Stat(dir)
	if err := ensureWorkspace(root); err != nil {
		return err
	}
	if os.IsNotExist(errAntes) {
		contenido := "# Lo creó Musubi al activarse solo en este repo: todo lo de acá es estado local.\n*\n"
		if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(contenido), 0o644); err != nil {
			return err
		}
	}
	return nil
}
