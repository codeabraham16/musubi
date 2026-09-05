package mcp

import (
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// LA PODA DE LOS PUNTOS DE RETORNO (A87), EJERCITADA DE VERDAD Y NO LEÍDA.
//
// `redesplegar-cerebro.sh` creaba dos archivos por corrida —el snapshot `pre-redespliegue-*.db` y
// el binario `musubi.antes-de-*`— y no borraba ninguno. Su propio aviso decía «borralos recién
// cuando estés seguro», que es un paso a mano: medido el 2026-09-04, 33 snapshots (4,8 GB) y 26
// binarios (833 MB) en el servidor, el más viejo del 28-08.
//
// Esta prueba corre el guión de shell que ejercita la función REAL contra archivos de verdad. Una
// guarda de TEXTO (buscar `sort` en el archivo) diría que la línea está y no que la poda hace lo
// que dice; el orden por nombre-vs-fecha es justo el caso donde las dos cosas se separan.
//
// Sabotajes verificados que la ponen en rojo (los dos COMPILAN, que es lo que los hace valer):
//   - cambiar el `| sort` por un orden por mtime (`-printf '%T@ %f\n' | sort -n`): borra los tres
//     equivocados, y además mete el punto de retorno de la corrida actual en la lista de borrado;
//   - quitar la guarda `[[ "$dir/$f" == "$actual" ]]`: con retención 0 se lleva la vuelta atrás.
func TestLaPodaDePuntosDeRetornoHaceLoQueDice(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("el guión de despliegue es de Linux y esta prueba corre bash; en %s no aplica", runtime.GOOS)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("sin bash en el PATH no se puede ejercitar el guión: %v", err)
	}
	arnes := filepath.Join("..", "..", "deploy", "pruebas", "poda-puntos-de-retorno.sh")
	guion := filepath.Join("..", "..", "deploy", "redesplegar-cerebro.sh")
	salida, err := exec.Command(bash, arnes, guion).CombinedOutput()
	if err != nil {
		t.Fatalf("la poda de puntos de retorno falló:\n%s", salida)
	}
	// CONTROL DE QUE EJERCITÓ ALGO: un arnés que no encuentra la función podría salir en 0 sin
	// haber probado nada, y este Fatal es más fácil de leer que un verde vacío.
	if !strings.Contains(string(salida), "los 4 casos en verde") {
		t.Fatalf("el arnés terminó en 0 pero no dijo que corrió los 4 casos:\n%s", salida)
	}
}

var (
	// Un artefacto POR CORRIDA es una variable cuyo valor lleva el sello de la corrida.
	artefactoPorCorrida = regexp.MustCompile(`^([A-Z_][A-Z0-9_]*)="[^"]*\$SELLO[^"]*"`)
	// El cuarto argumento de la llamada a la poda es el artefacto de ESTA corrida.
	llamadaDePoda = regexp.MustCompile(`podar_puntos_de_retorno\s+.*"\$([A-Z_][A-Z0-9_]*)"\s*$`)
)

// TODO LO QUE EL REDESPLIEGUE CREA POR CORRIDA TIENE PODA, y uno nuevo obliga a decidir.
//
// Es la forma de A76 aplicada acá: la poda se escribió para los DOS artefactos que hay hoy, y si
// mañana el guión aparta un tercero —otro binario, otro volcado— la poda no lo va a alcanzar y
// nada lo iba a decir. El acumulado no se nota nunca: se nota el día que el disco se llena, o el
// día que un secreto sobrevive en una copia que ninguna retención barre (A81).
//
// Sabotaje que la hace fallar: agregar `OTRO_VOLCADO="$HOME_CEREBRO/algo-$SELLO.db"` sin sumarle
// su llamada a `podar_puntos_de_retorno`.
func TestTodoArtefactoPorCorridaDelRedespliegueSePoda(t *testing.T) {
	guion := leerDeploy(t, "redesplegar-cerebro.sh")

	creados := map[string]int{} // variable → línea donde se crea
	podados := map[string]bool{}
	for i, linea := range strings.Split(guion, "\n") {
		if m := artefactoPorCorrida.FindStringSubmatch(strings.TrimSpace(linea)); m != nil {
			creados[m[1]] = i + 1
		}
		if m := llamadaDePoda.FindStringSubmatch(linea); m != nil {
			podados[m[1]] = true
		}
	}

	// CONTROL DE QUE MIRÓ ALGO: si el guión cambiara de forma —otro nombre de sello, comillas
	// simples— los dos regex dejarían de matchear y esta prueba pasaría en verde sin haber
	// encontrado un solo artefacto. Hoy son dos: RESPALDO y BIN_VIEJO.
	if len(creados) < 2 {
		t.Fatalf("se reconocieron %d artefactos por corrida y son al menos 2 (RESPALDO y BIN_VIEJO): "+
			"cambió la forma del guión y esta guarda dejó de mirar", len(creados))
	}

	for nombre, linea := range creados {
		if !podados[nombre] {
			t.Errorf("redesplegar-cerebro.sh crea %s en la línea %d (lleva $SELLO, así que es uno por "+
				"corrida) y NADIE lo poda.\n"+
				"  Sumale una llamada a `podar_puntos_de_retorno`, o —si de verdad tiene que quedarse "+
				"para siempre— escribí por qué al lado, porque hoy se lee como un olvido.", nombre, linea)
		}
	}
	// Y al revés: una poda sobre algo que el guión ya no crea es una limpieza que sobrevivió a su
	// motivo, y la próxima persona la va a leer como vigente.
	for nombre := range podados {
		if _, ok := creados[nombre]; !ok {
			t.Errorf("se poda %s y el guión ya no lo crea con el sello de la corrida: sacá la llamada, "+
				"o la poda queda diciendo que cuida algo que no existe", nombre)
		}
	}
}
