package mcp

import (
	"strings"
	"testing"
)

// LA PODA DECIDE POR CONTENIDO, NUNCA POR ANCESTRÍA.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// `git branch --merged` contesta por ANCESTRÍA: «¿este commit está en la historia de main?». Y
// este repo mergea con SQUASH, así que una rama cuyo contenido entró ENTERO a `main` sigue
// contestando «no mergeada» PARA SIEMPRE — su commit nunca va a ser ancestro de nada.
//
// Medido: `fix/powershell-escape-zombis` está idéntica en `origin/main` y `--merged` la reporta
// como pendiente. Con ese criterio las ramas se acumulan sin fin; y si alguien las borra igual
// «porque el contenido está», está decidiendo por corazonada sobre trabajo que puede no estar.
//
// LA PREGUNTA CORRECTA ES `git diff origin/main...RAMA` VACÍO: si no queda una sola línea que main
// no tenga, no hay nada que perder. Medido el 2026-09-11 sobre este repo: 49 ramas podables y UNA
// con trabajo propio — y esa una resultó ser un duplicado textual de dos arreglos que habían
// entrado por otros commits, cosa que `--merged` nunca habría podido decir.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestLaPodaDeRamasDecidePorContenidoYNoPorAncestria(t *testing.T) {
	guion := leerDeploy(t, "pruebas", "podar-ramas.sh")

	// EL CRITERIO. `diff ...` de tres puntos sobre la base.
	if !strings.Contains(guion, `git diff --stat "$BASE...$rama"`) {
		t.Error("la poda no decide con `git diff --stat \"$BASE...$rama\"`.\n" +
			"  Si decide por ancestría (`--merged`, `--contains`), toda rama squasheada figura " +
			"pendiente para siempre — este repo mergea con squash— y la poda no termina nunca.")
	}
	// Y EL QUE NO PUEDE USAR. Un `--merged` acá sería el defecto entero.
	for _, prohibido := range []string{"branch --merged", "branch --no-merged"} {
		if strings.Contains(guion, prohibido) {
			t.Errorf("la poda usa `%s`: eso decide por ANCESTRÍA, y con squash una rama ya integrada "+
				"figura pendiente para siempre", prohibido)
		}
	}

	// NO BORRA SIN RED. Borrar decenas de ramas es irreversible en la práctica —el reflog caduca—
	// y el repo tiene un cabo abierto que dice exactamente eso.
	iBundle := strings.Index(guion, "git bundle create")
	iBorrar := strings.Index(guion, "git branch -D")
	if iBundle < 0 {
		t.Error("la poda no crea un bundle antes de borrar: sin red no se salta, y el reflog caduca")
	} else if iBorrar > 0 && iBundle > iBorrar {
		t.Error("el bundle se crea DESPUÉS de borrar: para entonces ya no hay qué respaldar")
	}

	// Y NO BORRA POR DEFECTO. Un guion de poda que borra al invocarse es una trampa para quien lo
	// corre «a ver qué dice».
	if !strings.Contains(guion, `[ "$BORRAR" != "1" ]`) {
		t.Error("la poda no distingue el modo informe del modo borrar: un guion que borra al " +
			"invocarse es una trampa para quien lo corre para ver qué dice")
	}

	// UN ÁRBOL SUCIO NUNCA SE SACA. Este repo ya perdió tiempo con eso: cuatro worktrees sucios
	// con dos sabotajes, un scratch y trabajo real sin commitear.
	if !strings.Contains(guion, "status --short") {
		t.Error("la poda de worktrees no mira si el árbol está SUCIO: un árbol sucio puede tener " +
			"trabajo sin commitear, y sacarlo lo borra sin dejar rastro")
	}
}
