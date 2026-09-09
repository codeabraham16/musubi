package mcp

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TODA VERSIÓN QUE `construir.sh` PUEDA EMITIR TIENE QUE PARSEARLA NUESTRO PROPIO PARSER.
//
// EL INCIDENTE, MEDIDO. `construir.sh` arma `<VERSION>[-<track>].<commit>[-sucio]`. Con el track
// VACÍO el guión desaparece y quedan CUATRO componentes: `0.139.6.7e2d211`. `fleet.NucleoDeVersion`
// corta en el primer `-`, parte por `.` y exige tres, así que devuelve ok=false. Y
// `fleet.VersionDelAgenteDifiere` le pregunta AL CEREBRO PRIMERO: si la versión del cerebro no
// parsea contesta `comparable=false` para TODAS las máquinas, o sea que
// `musubi_fleet_device_agent_stale` deja de emitirse para la flota entera.
//
// El 2026-09-09 un redespliegue del cerebro con el primer argumento omitido lo provocó:
//
//	20:30 UTC · 3 series      20:39 UTC · redespliegue con `0.139.6.7e2d211`      21:00 UTC · 0 series
//
// El binario reemplazado era `0.139.3-main.326e411` y parseaba. La diferencia entera fue un
// argumento que el guión aceptaba omitir.
//
// POR QUÉ LA PRUEBA QUE YA EXISTÍA NO LO CAZÓ, que es lo que hace falta entender para que esta
// sirva. `internal/fleet/version_test.go` YA fija `{"0.130.0.1", "", false} // cuatro componentes`.
// Está verde y es correcta. Lo que no hace es CONECTAR esa forma con que es la salida de nuestro
// propio guión: la trata como entrada basura venida de afuera. Productor y parser sin nadie en el
// medio — la misma forma que `Capver` y `CuerpoLatido`, encontrada el mismo día.
//
// Por eso esta guarda no agrega más casos basura a mano: ENUMERA lo que `construir.sh` puede
// emitir —sin track, con track, árbol limpio y sucio— y exige que el parser los acepte todos.
//
// SE MIDE CONDUCTA Y NO TEXTO, y acá importa doble: un `grep` de `ETIQUETA` sobre el guión lo
// satisfaría un comentario, y encima seguiría verde el día que el formato de la versión cambie
// por otro lado. El arnés CORRE `construir.sh` en un clon y le pasa las versiones resultantes al
// `NucleoDeVersion` de verdad.
//
// SABOTAJES CORRIDOS, LOS TRES:
//
//   - ROJO (exit 1): devolverle al track su valor por defecto vacío. El arnés reprodujo la forma
//     exacta del incidente, `0.139.7.3de5306`.
//   - ROJO (exit 1): dejar el track obligatorio pero pegarlo con `.` en vez de `-`
//     (`0.139.7.arnes.646d661`). Es el que importa: prueba que la guarda mira el PARSEO y no
//     solamente que el guión pida un argumento.
//   - EXIT 2, no verde: hacer que el verificador conteste `true` a todo. El arnés lo detecta por
//     su control —la forma mala conocida tiene que seguir rechazada— y sale por «falló la
//     medición», que es distinto de «falló el guión». Sin ese control su verde no valdría nada.
//
// Y UN DEFECTO PROPIO QUE APARECIÓ CORRIENDO EL SEGUNDO, dejado escrito porque es de la familia
// que este repo persigue: el verificador separaba sus campos con TAB, y bash COLAPSA las corridas
// de caracteres IFS que son espacio en blanco —tab incluido, aunque `IFS` sea sólo tab—. Con el
// núcleo vacío, que es justo el caso que falla, los campos se corrían y el lector terminaba
// mirando `ok` en la variable del núcleo: el sabotaje salía rojo POR CASUALIDAD y el mensaje
// informaba un campo que no era. Se cambió el separador a `|`, que no colapsa.
func TestLaVersionQueEmiteConstruirEsSiempreParseable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("el guión de construcción es de Linux y este arnés corre bash; en %s no aplica", runtime.GOOS)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("sin bash en el PATH no se puede ejercitar el guión: %v", err)
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("sin go en el PATH el arnés no puede compilar: %v", err)
	}

	arnes := filepath.Join("..", "..", "deploy", "pruebas", "version-parseable.sh")
	raiz := filepath.Join("..", "..")
	salida, err := exec.Command(bash, arnes, raiz).CombinedOutput()
	if err != nil {
		t.Fatalf("el arnés de la versión falló:\n%s", salida)
	}
	// CONTROL DE QUE EJERCITÓ LAS TRES COSAS. Un arnés que saliera en 0 sin haber corrido los
	// builds —o sin haber probado su propio control— diría «no hay peligro» cuando lo que hubo
	// fue «no lo provoqué». Es la falla que `sufijo-sucio.sh` ya documenta en su hermana.
	for _, senal := range []string{
		"sin track → el guión se niega",
		"(parseable)",
		"la forma mala conocida",
	} {
		if !strings.Contains(string(salida), senal) {
			t.Fatalf("el arnés terminó en 0 pero no dijo %q, así que no ejercitó lo que dice:\n%s", senal, salida)
		}
	}
}
