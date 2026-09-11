package mcp

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// ════════════════════════════════════════════════════════════════════════════════════════════
// `-d .git` ES FALSO EN UN WORKTREE, Y ACÁ EL WORKTREE ES EL CASO NORMAL
//
// Un guion que comprueba «¿esto es el repo?» con `[[ -d "$REPO/.git" ]]` se niega a correr desde
// cualquier worktree: ahí `.git` es un ARCHIVO que apunta al directorio real, no un directorio.
//
// NO ES UN BORDE TEÓRICO. Este repo tenía más de setenta worktrees el 2026-09-11 —es como trabajan
// las sesiones— así que el caso que el guion rechaza es el HABITUAL, no el raro. Y el modo de falla
// es de los que mandan a mirar donde no está el problema: `actualizar-agente-windows.sh` contestaba
// «no es el repo de Musubi» y su ayuda sugería que uno estaba en la máquina equivocada.
//
// LA FORMA CORRECTA YA VIVÍA EN EL REPO: `verificar-despliegue.sh` pregunta con
// `git -C "$REPO" rev-parse --git-dir`, que contesta bien en un checkout y en un worktree. Esta
// guarda existe para que la forma rota no vuelva a entrar por otro archivo.
//
// Sabotaje que la pone roja: volver a poner `[[ -d "$REPO/.git" ]]` en cualquier guion de deploy/.
// ════════════════════════════════════════════════════════════════════════════════════════════

// gitComoDirectorio caza la pregunta rota: un test de EXISTENCIA DE DIRECTORIO sobre `.git`.
//
// Se acepta `-e` y `-f` —los dos son ciertos en un worktree— y sólo se rechaza `-d`, que es el
// único que miente. Buscar «.git» a secas daría falsos positivos sobre cualquier ruta que lo
// nombre.
var gitComoDirectorio = regexp.MustCompile(`-d\s+"?\$?\{?[A-Za-z_][A-Za-z0-9_]*\}?/\.git"?`)

func TestNingunGuionPreguntaSiElRepoEsUnDirectorioGit(t *testing.T) {
	raiz := filepath.Join("..", "..")
	shs, _, cmds := archivosDeGuiones(t, raiz)

	revisados := 0
	for _, ruta := range append(append([]string{}, shs...), cmds...) {
		// `leerDeploy` viene con los comentarios BLANQUEADOS, así que la prosa que explica este
		// mismo defecto —la de arriba, y la del guion que se arregló— no puede satisfacer la
		// guarda. Es la regla de la casa y acá se nota: sin el filtro, este test se cazaría a sí
		// mismo por su propio comentario.
		crudo, err := leerArchivoDeDespliegue(filepath.Join(raiz, ruta))
		if err != nil {
			t.Fatalf("no se pudo leer %s: %v", ruta, err)
		}
		codigo := string(crudo)
		revisados++
		for n, linea := range strings.Split(codigo, "\n") {
			if !gitComoDirectorio.MatchString(linea) {
				continue
			}
			t.Errorf("%s:%d pregunta si `.git` es un DIRECTORIO, y en un worktree es un ARCHIVO.\n"+
				"  Ese guion se niega a correr desde cualquier worktree diciendo que no es el repo, y "+
				"acá el worktree es el caso normal: el 2026-09-11 había más de setenta.\n"+
				"  Peor, el mensaje manda a mirar la máquina o la ruta, o sea al lugar donde NO está "+
				"el problema.\n"+
				"  Usá `git -C \"$REPO\" rev-parse --git-dir >/dev/null 2>&1`, que es lo que ya hace "+
				"`verificar-despliegue.sh` y contesta bien en las dos formas de checkout.\n"+
				"  Línea: %s", ruta, n+1, strings.TrimSpace(linea))
		}
	}

	// EL PISO. Sin esto, el día que `archivosDeGuiones` cambie de forma o de raíz, esta prueba
	// pasaría en verde sin haber abierto un solo archivo — y ese verde diría «no hay problema» en
	// vez de «no miré», que es el defecto que este repo persigue en todos lados.
	if revisados < 20 {
		t.Fatalf("sólo se revisaron %d guiones y hay bastantes más: cambió dónde viven y esta guarda "+
			"está en verde sin mirar nada", revisados)
	}
}
