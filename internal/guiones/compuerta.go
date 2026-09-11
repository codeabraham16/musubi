// Package guiones es LA ÚNICA COMPUERTA de las pruebas que EJECUTAN un guion de shell de este
// repo. No tiene otra función y no debería tener otra.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ EXISTE: EL ARNÉS ES DE UNIX Y LO QUE MIDE SON INSTALADORES DE SERVIDOR LINUX
//
// Las pruebas de `internal/mcp/despliegue_*_test.go` no grepean los guiones de `deploy/`: los
// CORREN, con un `curl` de mentira en el PATH, y miran si el archivo apareció en el destino. Eso
// las hace valiosas —un grep lo satisface un comentario— y también las ata al sistema operativo:
//
//   - el arnés escribe stubs ejecutables (`0o755` + shebang) y los pone primero en `$PATH`
//     separando con `:`;
//   - el prólogo es bash con `set -euo pipefail`, arrays y `[[ =~ ]]`;
//   - los guiones llaman `sha256sum`, `install`, `mktemp`, `awk`, `tr`, `unzip`, `tar`, `find`,
//     `uname -m` — coreutils GNU;
//   - y lo que instalan son unidades systemd de un servidor Linux, con `useradd` y `/etc`.
//
// MEDIDO EN CI (jobs test-cross), y por eso esto existe:
//
//   - Windows: el guion salió con `exit status 22` —el código de curl para «el servidor devolvió
//     un error HTTP»— después de imprimir la URL real del release. O sea: el shell de Windows NO
//     tomó el stub del PATH y el arnés SALIÓ A INTERNET. Un `PATH=C:\...\stubs:"$PATH"` se parte
//     en dos por el `:` del nombre de unidad, y un archivo `curl` sin `.exe` ni bit de ejecución
//     no es un ejecutable ahí.
//   - macOS: el runner es arm64, `uname -m` contesta `arm64`, y el guion del relay muere en
//     «arquitectura no soportada por los binarios oficiales» ANTES de llegar al checksum. El
//     arnés veía «no se instaló» y lo leía como «la verificación funcionó».
//
// EL SEGUNDO CASO ES EL PELIGROSO Y ES LA RAZÓN DE ESTA COMPUERTA: una prueba que se pone roja —o
// verde— por un motivo que NO es el que dice custodiar es un falso positivo, y un falso positivo
// termina apagado. Acotar el alcance a donde el guion corre de verdad no baja el listón: el job
// `test` de CI, el que gatea el merge, es linux y ahí corren TODAS, enteras.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LO QUE ESTA COMPUERTA NO PUEDE HACER, POR CONSTRUCCIÓN
//
//  1. NO PUEDE SALTEAR EN LINUX. La rama de `t.Skipf` es inalcanzable cuando GOOS es linux: no es
//     una promesa escrita, es el `if` de abajo. Y no se le cree al `if`: `TestLaCompuertaNuncaSaltea…`
//     la CORRE de verdad y comprueba que la línea siguiente se ejecutó.
//  2. NO PUEDE TAPAR UN «NO PUDE MEDIR». En linux, una herramienta que falta es `t.Fatal`, no
//     `t.Skip`. Ésa es la trampa que ya estaba puesta en cuatro pruebas de este repo
//     (`if _, err := exec.LookPath("bash"); err != nil { t.Skip(...) }`): sin bash, `go test`
//     contestaba `ok` y la guarda no existía.
//  3. NO PUEDE USARSE PARA OTRA COSA. `TestElSalteoEstaAcotadoALasPruebasQueCorrenUnaShell` arma
//     el grafo de llamadas de cada paquete de prueba del repo y exige la equivalencia en los DOS
//     sentidos: quien llama a esta compuerta tiene que ejecutar una shell, y quien ejecuta una
//     shell tiene que llamar a esta compuerta.
//
// Y EL ARCHIVO NO LLEVA SUFIJO DE PLATAFORMA NI `//go:build`: compila en todas y decide en
// runtime. Este repo ya se comió que `despliegue_agente_windows_test.go` no compilara en Linux
// mientras `go test` decía `ok` — la ausencia de una prueba se ve idéntica a su éxito.
package guiones

import (
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// ElSistemaDondeCorren es el GOOS en el que los guiones de `deploy/` se ejecutan de verdad, que es
// el único en el que estas pruebas miden lo que dicen medir.
const ElSistemaDondeCorren = "linux"

// Exigir frena la prueba si esta máquina no es donde corren los guiones, y comprueba que las
// herramientas Unix que el arnés va a usar estén.
//
//   - En linux NUNCA saltea. Si falta una herramienta, MUERE: eso es «no pude medir», y un verde
//     ahí sería la mentira exacta que estas pruebas existen para cazar.
//   - Fuera de linux saltea diciendo el GOOS, el GOARCH y el motivo concreto de quien la llamó.
//   - Un uso sin motivo o sin herramientas es un error EN TODAS las plataformas: esta compuerta no
//     es un `t.Skip` de propósito general y no se la puede usar como tal ni por accidente.
func Exigir(t *testing.T, motivo string, herramientas ...string) {
	t.Helper()

	// Los dos controles de USO van antes que nada y no miran el GOOS: si alguien llama a esto
	// para apagar una prueba que no corre ningún guion, tiene que romperse donde lo intentó y no
	// quedar en un `skip` cómodo en la plataforma que le molestaba.
	if len(strings.Fields(motivo)) < 4 {
		t.Fatalf("guiones.Exigir se llamó con el motivo %q: hace falta una frase que diga QUÉ guion "+
			"se ejecuta y por qué sólo aplica en %s. Un salteo sin motivo es un salteo que nadie "+
			"revisa.", motivo, ElSistemaDondeCorren)
	}
	if len(herramientas) == 0 {
		t.Fatal("guiones.Exigir se llamó sin nombrar una sola herramienta Unix. Esta compuerta NO es " +
			"un t.Skip de propósito general: existe para las pruebas que EJECUTAN un guion de shell, " +
			"y ésas siempre necesitan por lo menos `bash`. Si lo que querés es apagar otra cosa, esto " +
			"no es lo que buscás.")
	}

	if runtime.GOOS == ElSistemaDondeCorren {
		var faltan []string
		for _, h := range herramientas {
			if _, err := exec.LookPath(h); err != nil {
				faltan = append(faltan, h)
			}
		}
		if len(faltan) > 0 {
			t.Fatalf("en %s falta(n) %s en el PATH, así que este arnés NO PUEDE EJERCITAR el guion.\n"+
				"  Esto es un FALLO y no un salteo a propósito: %s es donde el guion corre de verdad, "+
				"y «no pude medir» no puede contestar lo mismo que «medí y está bien».\n"+
				"  Instalalas (en Debian/Ubuntu casi todas vienen en coreutils; `unzip` y `tar` son "+
				"paquetes propios) y volvé a correr.",
				runtime.GOOS, strings.Join(faltan, ", "), ElSistemaDondeCorren)
		}
		// Único camino de vuelta en linux. No hay `t.Skip` después de acá.
		return
	}

	t.Skipf("SALTEADA EN %s/%s — esto sólo se mide en %s.\n"+
		"  MOTIVO: %s\n"+
		"  EL ARNÉS ES DE UNIX: escribe stubs ejecutables (shebang + bit de ejecución), los antepone "+
		"al PATH separando con `:`, corre bash con `set -euo pipefail` y usa %s.\n"+
		"  EN WINDOWS eso no se sostiene —medido: el `:` de `C:\\` parte el PATH, el stub sin `.exe` "+
		"no se ejecuta, y el guion SALIÓ A LA URL REAL del release (exit 22 de curl)—; en macOS el "+
		"runner es arm64 y el guion del relay muere en la arquitectura antes de llegar al checksum.\n"+
		"  DÓNDE SÍ CORRE, ENTERA Y SIEMPRE: el job `test` de CI, que es ubuntu y es el que gatea el "+
		"merge. Acá no se pierde cobertura de la propiedad; se deja de medirla donde el resultado "+
		"habla del runner y no del guion.",
		runtime.GOOS, runtime.GOARCH, ElSistemaDondeCorren, motivo, strings.Join(herramientas, ", "))
}
