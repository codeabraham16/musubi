package mcp

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/memory"
)

// filtrarFotoAlCommit recorta la foto del grafo a lo que el COMMIT de la publicación contiene: se
// quedan los nodos, las aristas y los gists de los archivos rastreados en ese commit, y sale todo
// lo demás.
//
// ⚠️ POR QUÉ. El índice lee el DISCO, y el central guarda la foto con la etiqueta de un commit.
// origenDelGrafo ya frena lo modificado y lo sin trackear (primerArchivoIndexableSinCommitear),
// pero `git status` no lista los IGNORADOS, y lo ignorado se indexa igual: walkSourceTree no lee
// .gitignore. Medido el 2026-09-24: cmd/musubi/assets/boceto/boceto-g.js —un bundle que el
// .gitignore de su carpeta deja afuera— le sumaba 1.222 nodos al mapa del central con la etiqueta
// eb2cdb7, un commit que no lo contiene. La laptop, con el mismo commit y sin el bundle, publicaba
// otro mapa, y el central se quedaba con el del último que empujaba.
//
// El grafo LOCAL no se toca: sirve para el trabajo en curso, y ahí un boceto o un generado es tan
// código como lo demás. Lo que se recorta es lo que se PUBLICA.
//
// La lista sale de `git ls-tree` sobre el commit de la etiqueta, no de `git ls-files`: ls-files lee
// el ÍNDICE de git, que puede traer algo agregado y sin commitear, y lo que el mapa promete es el
// commit. Corre con `-C projectPath`, así que si el proyecto es un subdirectorio del repo, las rutas
// salen relativas a él, igual que las del grafo. Una foto SIN etiqueta (sin sello todavía) se
// recorta contra HEAD.
//
// Las aristas salen si alguna de sus puntas salió: una arista hacia un nodo que no viajó queda
// colgada en el central. Los nodos sin archivo (los paquetes importados) se quedan, porque no son
// de ningún archivo.
//
// SI GIT NO PUEDE CONTESTAR, NO SE PUBLICA. Devolver la foto entera sería publicar el disco con la
// etiqueta del commit, que es justo lo que esto cierra. La única foto que viaja sin recortar es la
// de un proyecto que no es un repo git: sin commit no hay etiqueta, y no hay contra qué recortar.
func (s *McpServer) filtrarFotoAlCommit(foto memory.FotoDelGrafo, pub memory.PublicacionDelGrafo) (memory.FotoDelGrafo, error) {
	commit := pub.Head
	if commit == "" {
		if !hayRepoGit(s.projectPath) {
			return foto, nil
		}
		commit = "HEAD"
	}
	salida, rc := s.gitDelArbol("ls-tree", "-r", "-z", "--name-only", commit)
	if rc != 0 {
		return memory.FotoDelGrafo{}, fmt.Errorf("git no pudo listar los archivos de %s (código %d): sin esa lista el mapa describiría el disco y no el commit, y no se publica", abreviar(commit), rc)
	}
	rastreados := map[string]bool{}
	for _, ruta := range strings.Split(salida, "\x00") {
		if ruta != "" {
			rastreados[ruta] = true
		}
	}
	enElCommit := func(ruta string) bool { return rastreados[memory.NormalizeCodePath(s.projectPath, ruta)] }
	return recortarFotoA(foto, enElCommit), nil
}

// recortarFotoA deja en la foto sólo lo de los archivos para los que enElCommit dice que sí. La
// generación no cambia: la que se marca como empujada sigue siendo la de la foto leída.
func recortarFotoA(foto memory.FotoDelGrafo, enElCommit func(ruta string) bool) memory.FotoDelGrafo {
	salieron := map[string]bool{}
	foto.Nodes = quedarseCon(foto.Nodes, func(n memory.GraphNode) bool {
		if n.Path == "" || enElCommit(n.Path) {
			return true
		}
		salieron[n.Key] = true
		return false
	})
	foto.Edges = quedarseCon(foto.Edges, func(e memory.GraphEdge) bool {
		return !salieron[e.FromKey] && !salieron[e.ToKey]
	})
	foto.Gists = quedarseCon(foto.Gists, func(g memory.CodeMemory) bool { return enElCommit(g.Path) })
	return foto
}

// quedarseCon filtra sin cambiar si la lista es nil o vacía: en el push de gists son dos mensajes
// distintos (nil ⇒ «no hablo de gists», vacía ⇒ «no tengo ninguno»; ver toolCodegraphPush).
func quedarseCon[T any](xs []T, queda func(T) bool) []T {
	if xs == nil {
		return nil
	}
	out := make([]T, 0, len(xs))
	for _, x := range xs {
		if queda(x) {
			out = append(out, x)
		}
	}
	return out
}

// hayRepoGit dice si dir está adentro de un repo git, buscando lo mismo que busca git para
// descubrirlo: un `.git` —directorio, o archivo en un worktree— en dir o en alguno de sus padres.
// Hace falta saberlo SIN git, porque un git que falla y un directorio que no es un repo contestan
// con el mismo código de salida.
func hayRepoGit(dir string) bool {
	d := filepath.Clean(dir)
	if abs, err := filepath.Abs(d); err == nil {
		d = abs // con una ruta relativa, subir por los padres terminaría en «.»
	}
	for {
		if _, err := os.Stat(filepath.Join(d, ".git")); err == nil {
			return true
		}
		padre := filepath.Dir(d)
		if padre == d {
			return false
		}
		d = padre
	}
}
