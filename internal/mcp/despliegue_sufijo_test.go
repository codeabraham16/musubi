package mcp

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// UN BINARIO QUE SE LLEVA CÓDIGO SIN COMMITEAR TIENE QUE DECIRLO EN SU PROPIA VERSIÓN.
//
// El contrato está escrito en la cabecera de `deploy/construir.sh`: un build sucio no se prohíbe
// —a veces hace falta probar algo rápido— se DECLARA, porque no se puede reconstruir después: el
// commit que anuncia NO es el código que corre.
//
// Ese contrato se apoyaba en `git diff --quiet && git diff --cached --quiet`, y **`git diff` no ve
// los archivos SIN TRACKEAR**. `go build` compila todo el `.go` del directorio, rastreado o no.
// Así que el caso PEOR —código que se despliega y no está en NINGÚN commit, el único que no se
// puede auditar ni reproducir— era exactamente el que el guion daba por limpio.
//
// Medido el 2026-09-05 en un clon aislado, con un `cmd/musubi/zz_chivato.go` sin trackear:
//
//	git status --short           ->  ?? cmd/musubi/zz_chivato.go
//	el chequeo viejo             ->  LIMPIO
//	versión estampada            ->  0.131.0-flota.ac75fec, SIN -sucio
//	sha256                       ->  CAMBIÓ: el archivo sin trackear viajó adentro del binario
//	vcs.modified que embute Go   ->  true
//
// Y en ese momento la frase «sin sufijo -sucio, o sea que no se lleva trabajo de nadie» ya estaba
// escrita en el registro y en un mensaje a gio, sobre un binario que se iba a desplegar. La
// conclusión resultó cierta por suerte —el árbol estaba limpio— y no por el razonamiento.
//
// ─────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ NO ES UN GREP, Y POR QUÉ LA PRUEBA VIVE EN UN ARNÉS
//
// Un grep de `--porcelain` lo satisfacería un comentario, que es el defecto dominante de este repo
// y ya pasó nueve veces en un día. Y además prohibiría la solución CORRECTA: hoy el sufijo no se
// calcula dos veces, se CORRIGE contra el `vcs.modified` que Go embute al compilar, así que la
// palabra «untracked» no aparece en ninguna parte del guion. Una guarda de texto habría puesto en
// rojo el arreglo bueno — la falla 7 de `deploy/pruebas/sabotaje.sh`.
//
// El arnés mide CONDUCTA en un clon —no en un worktree, donde Go no estampa `vcs` en absoluto y el
// módulo queda en `(devel)`, o sea que el sello no dice «limpio»: no dice NADA— y comprueba las dos
// direcciones más la premisa: que el archivo sin trackear de verdad VIAJE (los dos sha tienen que
// diferir). Sin eso mediría la etiqueta y no el peligro.
//
// Sabotajes verificados: con la conjetura vieja Y el relink correctivo anulado, el arnés sale 1 con
// «UN BINARIO QUE SE LLEVA CÓDIGO SIN COMMITEAR NO SALIÓ MARCADO». Y uno que NO lo pone en rojo, a
// propósito: marcar SIEMPRE queda curado por el relink, porque el sufijo es una lectura del sello y
// no una afirmación — dos capas, y el arnés lo deja escrito para que ese verde no se lea como hueco.
func TestUnBinarioConCodigoSinCommitearLoDiceEnSuVersion(t *testing.T) {
	// Los dos `t.Skipf` de `exec.LookPath` que había acá salteaban TAMBIÉN EN LINUX: sin bash o sin
	// go esta guarda no existía y `go test` contestaba `ok`. La compuerta los convierte en un fallo.
	guiones.Exigir(t, "corre deploy/pruebas/sufijo-sucio.sh, que ejercita el guion de "+
		"construcción de un servidor Linux y compila con go", "bash", "go", "awk", "sed", "grep")

	arnes := filepath.Join("..", "..", "deploy", "pruebas", "sufijo-sucio.sh")
	raiz := filepath.Join("..", "..")
	salida, err := exec.Command("bash", arnes, raiz).CombinedOutput()
	if err != nil {
		t.Fatalf("el arnés del sufijo falló:\n%s", salida)
	}
	// CONTROL DE QUE EJERCITÓ ALGO, y en particular la PREMISA: un arnés que no logre que el archivo
	// sin trackear cambie el binario puede salir en 0 sin haber medido nada, y ese verde diría
	// «no hay peligro» cuando lo que hubo fue «no lo provoqué».
	for _, senal := range []string{"el archivo sin trackear VIAJÓ", "(marcado)", "sin marca, como debe"} {
		if !strings.Contains(string(salida), senal) {
			t.Fatalf("el arnés terminó en 0 pero no dijo %q, así que no ejercitó lo que dice:\n%s", senal, salida)
		}
	}
}
