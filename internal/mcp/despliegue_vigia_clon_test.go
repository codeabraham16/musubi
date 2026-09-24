package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// LO QUE CUSTODIA: que la receta que el vigía le IMPRIME al operador, cuando su checkout no se
// puede poner al día solo, mande a hacer un CLON y no un worktree de git.
//
// EL DEFECTO QUE LA MOTIVA NO ES HIPOTÉTICO. El 2026-09-22 el checkout del vigía en `davantis`
// era un worktree, alguien barrió worktrees viejos, y `musubi-comparar.service` salió con
// `203/EXEC` en las CUATRO corridas siguientes: 18 horas sin comparar el repo contra producción.
// Lo delató `ComparacionRepoServidorSinCorrer`, no una prueba.
//
// Y ARREGLAR EL COMENTARIO DE LA UNIDAD NO ARREGLÓ ESTO. El aviso quedó escrito en
// `deploy/systemd/musubi-comparar.service` mientras este guion, veinte líneas más allá, le seguía
// imprimiendo al operador `git worktree add --detach <ruta> origin/main`. La prosa de un archivo
// no gobierna la receta de otro: lo que el operador obedece es lo que le sale por pantalla.
//
// SE MIRA DONDE DECIDE, NO DONDE SE CUENTA. Esta guarda no barre el repo por la cadena
// «worktree add»: la busca SÓLO en las líneas de salida de la rama `elif [ -n "$_rama" ]`, que es
// el código que dispara cuando el checkout no sirve. Dos motivos, medidos los dos:
//
//   - El comentario de la unidad NOMBRA `git worktree add` a propósito, para contar qué falló.
//     Una guarda por cadena se pondría roja sobre la prosa del propio arreglo.
//   - `deploy/actualizar-agente-windows.sh` y `deploy/pruebas/frescura-de-la-referencia.sh` usan
//     `worktree add` con razón: uno arma un árbol efímero para compilar, la otra arma a propósito
//     la topología de worktree que existe para probar. Prohibirlo en el repo entero sería falso.
//
// LAS DOS DIRECCIONES, y la segunda es la que impide el arreglo cómodo: borrar la receta entera
// haría desaparecer el `worktree add` y esta guarda pasaría en verde sobre un vigía que ya no le
// dice al operador qué hacer. Por eso también se exige que la receta nombre `git clone`.
//
// Sabotaje: devolverle a la receta la forma vieja —`git worktree add`—, que es literalmente lo
// que decía hasta el 2026-09-22 y lo que dejó al vigía 18 horas mudo.
// arnes: archivo="deploy/comparar-y-latir.sh"
// arnes: de="git clone <url-del-remoto> <ruta> && git -C <ruta> checkout --detach origin/main"
// arnes: a="git worktree add --detach <ruta> origin/main"
func TestLaRecetaDelVigiaMandaAClonarYNoAHacerUnWorktree(t *testing.T) {
	rel := filepath.Join("deploy", "comparar-y-latir.sh")
	crudo, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("leer %s: %v", rel, err)
	}
	lineas := strings.Split(string(crudo), "\n")

	// EL ANCLA ES CÓDIGO, NO PROSA: la rama que corre cuando el checkout tiene una rama puesta y
	// por eso no se puede mover. Si alguien la reescribe, esta guarda falla por no encontrarla
	// —que es lo correcto— en vez de pasar en verde mirando otra cosa.
	reDisparo := regexp.MustCompile(`^\s*elif\s+\[\s+-n\s+"\$_rama"\s+\]`)
	reCierre := regexp.MustCompile(`^\s*(elif|else|fi)\b`)
	// Una línea de salida es la que EMPIEZA con `printf`/`echo`. Con eso, una receta comentada
	// —`# printf '… worktree add …'`— no cuenta, que es lo correcto: el operador no la obedece.
	reSalida := regexp.MustCompile(`^\s*(printf|echo)\b`)

	inicio := -1
	for n, l := range lineas {
		if reDisparo.MatchString(l) {
			inicio = n
			break
		}
	}
	if inicio < 0 {
		t.Fatalf("no encontré en %s la rama `elif [ -n \"$_rama\" ]`, que es la que imprime la "+
			"receta del vigía. O se reescribió el bloque —y entonces hay que re-anclar esta "+
			"guarda— o se borró, y entonces el vigía dejó de decirle al operador qué hacer con un "+
			"checkout que no se puede poner al día.", rel)
	}

	type salida struct {
		n     int
		texto string
	}
	var receta []salida
	for n := inicio + 1; n < len(lineas); n++ {
		if reCierre.MatchString(lineas[n]) {
			break
		}
		if reSalida.MatchString(lineas[n]) {
			receta = append(receta, salida{n + 1, lineas[n]})
		}
	}
	if len(receta) < 2 {
		t.Fatalf("la receta del vigía quedó en %d línea(s) de salida en %s: con eso esta guarda no "+
			"está leyendo la receta, y pasaría en verde sin haber mirado nada.", len(receta), rel)
	}

	reWorktree := regexp.MustCompile(`(?i)worktree\s+add`)
	for _, s := range receta {
		if m := reWorktree.FindString(s.texto); m != "" {
			t.Errorf("%s:%d le dice al operador que arme un worktree («%s»):\n  %s\n"+
				"Es la receta que dejó al vigía 18 h mudo el 2026-09-22. Un worktree se lo lleva "+
				"cualquier limpieza, y entonces systemd falla con `203/EXEC` ANTES de correr una "+
				"sola línea del guion: ninguna verificación de adentro puede avisar.\n"+
				"Arreglo: `git clone <url> <ruta> && git -C <ruta> checkout --detach origin/main`.",
				rel, s.n, m, strings.TrimSpace(s.texto))
		}
	}

	var cuerpo []string
	for _, s := range receta {
		cuerpo = append(cuerpo, strings.TrimSpace(s.texto))
	}
	if !regexp.MustCompile(`(?i)git\s+clone`).MatchString(strings.Join(cuerpo, "\n")) {
		t.Errorf("la receta del vigía en %s ya no nombra `git clone`.\n"+
			"Esta mitad existe porque la otra sola se satisface BORRANDO: sin `worktree add` y sin "+
			"receta, la guarda pasaría en verde sobre un vigía que dejó de decirle al operador "+
			"cómo armar un checkout que se pueda poner al día.\n"+
			"Las %d líneas leídas fueron:\n  %s", rel, len(receta), strings.Join(cuerpo, "\n  "))
	}

	t.Logf("%d líneas de receta leídas en %s:%d…%d", len(receta), rel, receta[0].n, receta[len(receta)-1].n)
}
