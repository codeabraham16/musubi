package mcp

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// LA FRESCURA DE `origin/main` SE MIDE EN EL ÁRBOL QUE FETCHEA, NO EN EL DE AL LADO.
//
// `verificar-despliegue.sh` avisa cuando la referencia contra la que compara es vieja, porque
// compararse contra un `origin/main` de la semana pasada es compararse contra un main viejo. La
// edad la sacaba del `FETCH_HEAD` del `--git-common-dir`.
//
// ESO ES CORRECTO EN UN CHECKOUT NORMAL —donde `--git-dir` y `--git-common-dir` son el MISMO
// directorio— Y MIENTE EN UN WORKTREE:
//
//	--git-dir        → .git/worktrees/<nombre>/FETCH_HEAD   ← lo escribe el fetch de ESE árbol
//	--git-common-dir → .git/FETCH_HEAD                      ← lo escribe el fetch de OTRO
//
// Medido el 2026-09-19, con el vigía corriendo desde un worktree que se pone al día cada 6 h: su
// propio fetch era de hacía minutos y el común tenía 60 h, así que el informe decía «la referencia
// se trajo hace 59 h», marcaba el eslabón como SIN VERIFICAR y salía con código 2. Cuatro corridas
// seguidas en rojo sobre un árbol que estaba exactamente en `origin/main` y limpio.
//
// Es la misma familia que el defecto que lo precedió —el vigía leyendo la rama que otro dejó
// puesta— y la misma pregunta: DE QUIÉN ES EL DATO QUE ESTOY MIRANDO.
//
// EL ARNÉS ARMA LA TOPOLOGÍA, NO LA SIMULA: crea un repo de origen, lo clona, le agrega un worktree
// y hace los fetch de verdad. Escribir los `FETCH_HEAD` a mano fijaría lo que yo CREO que git hace,
// que es justo lo que este defecto demostró que no se puede suponer.
//
// Sabotaje que la hace fallar: volver a mirar un solo directorio, que es el defecto original.
// arnes: archivo="deploy/edad-de-la-referencia.sh"
// arnes: de="rev-parse --git-dir 2>/dev/null"
// arnes: a="rev-parse --git-common-dir 2>/dev/null"
func TestLaFrescuraDeLaReferenciaSeMideEnElArbolQueFetchea(t *testing.T) {
	// El salteo de fuera de linux vive en UN solo lugar. Sin estas herramientas la guarda no
	// existe, y eso tiene que MORIR, no contestar `ok`.
	guiones.Exigir(t, "corre deploy/pruebas/frescura-de-la-referencia.sh, que arma un clon con "+
		"worktree y hace fetch de verdad", "bash", "git", "stat", "touch", "date")

	arnes := filepath.Join("..", "..", "deploy", "pruebas", "frescura-de-la-referencia.sh")
	raiz := filepath.Join("..", "..")
	salida, err := exec.Command("bash", arnes, raiz).CombinedOutput()
	if err != nil {
		t.Fatalf("el arnés de la frescura falló:\n%s", salida)
	}
	// CONTROL DE QUE EJERCITÓ LOS CUATRO CASOS. Un arnés que saliera en 0 sin haber armado el
	// worktree —o sin haber probado la otra dirección— diría «no hay peligro» cuando lo que hubo
	// fue «no lo provoqué». Los dos del medio son los que impiden que un guion que devuelve 0
	// siempre, o que inventa un 0 cuando no hay nada que medir, pase por sano.
	for _, senal := range []string{
		"topología armada: el worktree tiene su propio gitdir",
		"el fetch propio gana al común viejo",
		"con los dos viejos dice viejo",
		"sin FETCH_HEAD no inventa un cero",
		"el checkout normal sigue andando",
	} {
		if !strings.Contains(string(salida), senal) {
			t.Fatalf("el arnés terminó en 0 pero no dijo %q, así que no ejercitó lo que dice:\n%s", senal, salida)
		}
	}
}
