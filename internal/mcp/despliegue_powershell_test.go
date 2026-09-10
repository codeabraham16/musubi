package mcp

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// EN UNA CADENA DE POWERSHELL, `\"` NO ESCAPA NADA — Y EL MISMO PAR ES CORRECTO EN BASH.
//
// LO QUE PASÓ, MEDIDO EN `davantis-1` EL 2026-09-10. `matar-zombis-agente.sh` llevaba esto adentro
// de un bloque de PowerShell:
//
//	"... (schtasks /run /tn \"Musubi Agente de Flota\"). Procesos: " + $detalle
//
// En una cadena de PowerShell con comillas dobles el carácter de escape es el BACKTICK, no la
// barra invertida. Así que esa comilla CIERRA la cadena y lo que sigue queda suelto:
//
//   - FullyQualifiedErrorId : UnexpectedToken) [], ParentContainsErrorRecordException
//     ✗ no se mató nada
//
// Y LO QUE LO HACE PEOR ES DÓNDE ESTABA: en la rama `$nuevos.Count -eq 0`, que NO SE EJECUTÓ —la
// máquina sí tenía el proceso nuevo—. PowerShell PARSEA EL BLOQUE ENTERO antes de correr una sola
// línea, así que un error de sintaxis en una rama muerta se llevó puesta la rama viva.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// REESCRITURA 1 (2026-09-10, el mismo día). La primera versión miraba SÓLO `deploy/*.sh`, y sólo
// los literales de bash que abrían con la secuencia `"'`:
//
//  1. ERA CIEGA A `deploy/lib-agente-windows.sh` (`RESOLVER=` y `CLASIFICAR=`, que abren con
//     `NOMBRE='`). `RESOLVER` se antepone a las OCHO llamadas a `ps1` de los guiones de Windows.
//  2. ERA CIEGA A LOS `.ps1`, y en `agente-windows.ps1:222` había un `\"` VIVO.
//  3. SE PONÍA ROJA EN FALSO sobre un `trap 'rm -rf "$TMP"' EXIT` de bash sano.
//  4. SU CONTROL DE COBERTURA ERA AIRE: «al menos 10 bloques», y cinco de los diecisiete que
//     contaba salían de archivos sin una línea de PowerShell.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// REESCRITURA 2 (2026-09-10, ronda adversaria). La reescritura 1 ARREGLABA EL DEFECTO pero seguía
// preguntando por LA FORMA con la que el defecto se había escrito aquella vez. Un saboteador
// reprodujo el mismo defecto de cinco maneras y las cinco quedaron en VERDE:
//
//   - `install.bat:38` en la RAÍZ del repo — el `.bat` de doble clic que baja y corre el
//     instalador público — tiene EXACTAMENTE la misma construcción que `cambiar-agente.cmd:69`,
//     y el barrido miraba `deploy/` y `scripts/` nada más.
//   - `ps1 'BLOQUE'` con la comilla simple SUELTA: sin `"` ni `=` pegados a la izquierda el
//     extractor de bash tiraba el literal. Miraba EL PREFIJO, que no decide nada.
//   - el PowerShell que arman los `.go` y le pasan a `exec.Command("powershell", "-Command", ps)`.
//     La reescritura 1 lo declaró sin cubrir; adentro de un raw string de Go (backticks) el `\"`
//     llega LITERAL a PowerShell y es el defecto exacto.
//   - la condición muerta (`... -or $true`) que se sacó de `agente-windows.ps1` volvió a ponerse
//     y nada la vio. Una rama incondicional en un instalador es un defecto propio.
//   - la invocación de PowerShell partida con el continuador `^` de cmd, y `-c` como alias de
//     `-Command`: el extractor de `.cmd` buscaba el texto `-command` línea por línea.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// CÓMO SE MIDE AHORA. La pregunta es siempre la misma —QUÉ COMILLA DECIDE DÓNDE TERMINA LA
// CADENA QUE VE POWERSHELL— y por eso lo primero es siempre RECONSTRUIR ESE TEXTO. Ninguna clase
// se reconoce por un prefijo, un nombre de archivo ni una palabra suelta:
//
//	a) `.sh` — se lexea bash de verdad (comillas simples, dobles, `\`, `#`, `$( )` anidado) y se
//	   toma TODO literal `'...'`. Que sea PowerShell lo decide su CONTENIDO (un `Verbo-Sustantivo`,
//	   `$ErrorActionPreference`, `-ErrorAction`), o que se lo pase a un HELPER que termina
//	   invocando `powershell` —los `ps1(){ ... }` se descubren leyendo el archivo, no por nombre—,
//	   o que se le pegue una variable de bash que ya se sabe que guarda PowerShell (`RESOLVER`).
//	b) `.ps1` — PowerShell de punta a punta.
//	c) `.cmd` y `.bat`, en `deploy/`, en `scripts/` Y EN LA RAÍZ del repo. Se pegan las líneas que
//	   terminan en `^` (el continuador de cmd), se parte la línea con las reglas REALES de
//	   `CommandLineToArgvW` —que es quien decide qué le llega a `powershell.exe`— y se toma el
//	   argumento que sigue a cualquier abreviatura de `-Command` (`-c`, `-com`, `/command`, …), o
//	   el primer posicional si no hay bandera, que es el default de `powershell.exe`.
//	d) `.go` — se parsea con `go/ast`, se buscan las llamadas que tienen un argumento literal
//	   `powershell`/`powershell.exe`, y se RESUELVE el argumento del `-Command` hasta su texto:
//	   consts, variables locales, concatenaciones, `fmt.Sprintf`, y funciones envoltorio
//	   (`runPowerShell(script string)`) siguiendo el parámetro hasta sus llamadores.
//
// Y EN LAS CUATRO SE DESHACE LA CAPA DE ESCAPE DE CADA UNA ANTES DE PREGUNTAR, porque `\"` no
// significa lo mismo en todas:
//
//   - en bash y en un `.ps1`, `\"` llega tal cual a PowerShell → ES el defecto;
//   - en un `.cmd`/`.bat`, `CommandLineToArgvW` convierte `\"` en `"` → NO es defecto; el defecto
//     es `\\\"`, que llega como `\"`;
//   - en un `.go`, una cadena entre comillas convierte `\"` en `"` → NO es defecto; el defecto es
//     un `\"` adentro de un RAW STRING (backticks), donde llega literal, o un `\\\"` adentro de
//     una cadena normal. `strconv.Unquote` deshace exactamente esa capa, así que las dos se
//     distinguen sin adivinar y la guarda no grita en falso.
//
// SEGUNDO DEFECTO, EL MISMO ARCHIVO: LA RAMA INCONDICIONAL. `agente-windows.ps1` tenía
//
//	if ($LASTEXITCODE -ne 0 -and $error[0] -match "forbidden|10013" -or $true)
//
// que es siempre verdadera (`-and` liga más fuerte que `-or`). No era una redundancia inofensiva:
// LEÍA como un filtro y no filtraba nada, en el instalador que corre un cliente. Se saca la
// condición y se guarda que no vuelva: de CADA `if`/`elseif` de CADA pieza de PowerShell de las
// cuatro clases se evalúa la condición —partida por `-or` y `-and`, con la precedencia real— y se
// falla si es constante, verdadera o falsa. No se busca el texto `-or $true`: se busca una
// condición que no decide nada, que es lo que el defecto ES.
//
// SABOTAJES CORRIDOS CONTRA ESTA VERSIÓN, a mano y restaurados con `cp` (no con `git checkout`,
// que no conoce un archivo nuevo): ver el mensaje del commit. Los cinco de arriba dan ROJO, más
// los cinco de la reescritura 1, y el `trap` de bash sigue en verde.
func TestNingunPowerShellDeDeployEscapaComillasConBarra(t *testing.T) {
	raiz := filepath.Join("..", "..")

	shs, ps1s, cmds := archivosDeGuiones(t, raiz)
	gos := archivosGo(t, raiz)

	// EL PRIMER CONTROL ES QUE EL BARRIDO HAYA ENCONTRADO ALGO. Un cero acá significa «no pude
	// medir», no «no hay nada malo», y son cosas opuestas: si el árbol se mueve, o el test corre
	// desde otro directorio, o alguien renombra `deploy/`, la guarda tiene que gritar y no pasar.
	if len(shs) == 0 || len(ps1s) == 0 || len(cmds) == 0 || len(gos) == 0 {
		t.Fatalf("no encontré archivos que mirar bajo %v: %d .sh, %d .ps1, %d .cmd/.bat, %d .go. "+
			"Un cero acá es «no pude medir», no «está todo bien»",
			carpetasDeGuiones, len(shs), len(ps1s), len(cmds), len(gos))
	}

	fuentes := map[string]string{}
	for _, r := range append(append([]string{}, shs...), append(ps1s, cmds...)...) {
		fuentes[r] = leerArchivo(t, filepath.Join(raiz, r))
	}

	// Las variables de bash que GUARDAN PowerShell (`RESOLVER`, `CLASIFICAR`, `VEREDICTO`) y los
	// helpers que se lo PASAN a `powershell` (`ps1(){ ... }`). Los dos se descubren leyendo los
	// archivos: ni una lista de nombres ni un prefijo.
	variablesPS := variablesDeBashQueGuardanPowerShell(shs, fuentes)
	helpers := map[string]bool{}
	for _, ruta := range shs {
		for h := range funcionesDeBashQueInvocanPowerShell(fuentes[ruta]) {
			helpers[h] = true
		}
	}

	// unidades[ruta] = cuántas piezas de PowerShell REALES se miraron en ese archivo.
	unidades := map[string]int{}

	// (a) — los `.sh` que generan PowerShell.
	for _, ruta := range shs {
		src := fuentes[ruta]
		for _, lit := range literalesDeBash(src) {
			if !literalEsPowerShell(lit, variablesPS, helpers) {
				continue
			}
			unidades[ruta]++
			revisarPowerShell(t, ruta, lit.texto, func(off int) int { return lit.lineaEnArchivo(src, off) })
		}
	}

	// (b) — los `.ps1` son PowerShell de punta a punta.
	for _, ruta := range ps1s {
		src := fuentes[ruta]
		// La unidad acá es la CADENA DOBLE mirada, que es donde el defecto puede estar. Un `.ps1`
		// sin ninguna cadena doble sería un lexer roto, no un archivo limpio.
		unidades[ruta] = revisarPowerShell(t, ruta, src, func(off int) int {
			return 1 + strings.Count(src[:off], "\n")
		})
	}

	// (c) — los `.cmd` y los `.bat`, en el argumento del `-Command` de `powershell`.
	for _, ruta := range cmds {
		src := fuentes[ruta]
		for _, inv := range comandosPowerShellDeCmd(src) {
			unidades[ruta]++
			revisarPowerShell(t, ruta, inv.texto, func(int) int { return inv.linea })
		}
	}

	// (d) — el PowerShell que arman los `.go`.
	for _, pz := range piezasDePowerShellEnGo(t, raiz, gos) {
		unidades[pz.ruta]++
		revisarPowerShell(t, pz.ruta, pz.texto, func(int) int { return pz.linea })
	}

	verificarInventario(t, raiz, shs, ps1s, cmds, unidades)
}

// revisarPowerShell le hace a UN texto de PowerShell las dos preguntas, y devuelve cuántas
// cadenas dobles miró (la unidad de cobertura). `linea` traduce un desplazamiento del texto a la
// línea del archivo: cada clase sabe cómo hacerlo y ninguna adivina.
func revisarPowerShell(t *testing.T, ruta, ps string, linea func(off int) int) int {
	t.Helper()
	hallazgos, cadenas, mascara := escanearPowerShell(ps)
	for _, h := range hallazgos {
		informarBarra(t, ruta, linea(h.desplazamiento), h.recorte)
	}
	for _, c := range condicionesConstantes(ps, mascara) {
		informarCondicion(t, ruta, linea(c.desplazamiento), c.texto, c.veredicto)
	}
	return cadenas
}

func informarBarra(t *testing.T, ruta string, linea int, recorte string) {
	t.Helper()
	t.Errorf("%s:%d — PowerShell que escapa una comilla con BARRA (`\\\"`).\n\n"+
		"  …%s…\n\n"+
		"En una cadena de PowerShell con comillas dobles el escape es el BACKTICK; la barra no "+
		"escapa nada, así que esa comilla CIERRA la cadena y lo que sigue queda suelto. Si esto "+
		"está en un bloque que se manda entero, el bloque DEJA DE PARSEAR —PowerShell parsea antes "+
		"de ejecutar—, así que también se rompen las ramas que sí se iban a correr; si está en un "+
		"`.ps1`, la línea imprime o ejecuta algo distinto de lo escrito.\n"+
		"Para una comilla literal adentro de la cadena, DOBLALA: `\"\"` (o poné un backtick).\n"+
		"Ojo: esto es lo que LE LLEGA A POWERSHELL, ya deshecha la capa de escape de cada formato "+
		"(bash, `CommandLineToArgvW` en los `.cmd`/`.bat`, `strconv.Unquote` en los `.go`). Un "+
		"`\\\"` escrito en un `.cmd` o en una cadena Go normal NO es esto y no pone roja la guarda.",
		ruta, linea, recorte)
}

func informarCondicion(t *testing.T, ruta string, linea int, cond, veredicto string) {
	t.Helper()
	t.Errorf("%s:%d — condición de PowerShell CONSTANTE: %s.\n\n"+
		"  if (%s)\n\n"+
		"Una condición que no decide nada es un defecto propio, no una redundancia: LEE como un "+
		"filtro y no filtra. Así estuvo `agente-windows.ps1` hasta el 2026-09-10 con\n"+
		"  if ($LASTEXITCODE -ne 0 -and $error[0] -match \"forbidden|10013\" -or $true)\n"+
		"—`-and` liga más fuerte que `-or`, así que la condición entera era `(...) -or $true`—, en "+
		"el instalador que corre un cliente.\n"+
		"Si la rama tiene que correr siempre, SACÁ EL `if` y dejá el cuerpo. Si tiene que filtrar, "+
		"escribí el filtro que de verdad filtra: borrar sólo el `-or $true` habría APAGADO el "+
		"consejo, que no es lo mismo que dejar el filtro que aparenta.",
		ruta, linea, veredicto, strings.Join(strings.Fields(cond), " "))
}

// ────────────────────────────────────────────────────────────────────────────────────────────
// EL INVENTARIO. Es el control de cobertura, y cuenta piezas REALES.
//
// El de la primera versión («al menos 10 bloques en total») se satisfacía con basura: cinco de
// sus diecisiete «bloques» salían de dos archivos sin una línea de PowerShell. Un número global
// acolchado no distingue «miré los ocho bloques de Windows» de «miré cuatro y cuatro fragmentos
// de un `trap`». Éste nombra archivo por archivo lo que hay que encontrar, y falla si un archivo
// del inventario no está en el disco (renombrado o borrado) o si rinde menos de lo esperado
// (la extracción dejó de reconocerlo).
//
// Y hay un control al revés, que es el que evita repetir el defecto que originó cada reescritura:
// un archivo que TENGA PowerShell y NO esté en esta tabla hace fallar la prueba. Así el próximo
// `.sh`, `.ps1`, `.bat` o `.go` que aparezca entra por la puerta y no por la ventana.
var inventarioPowerShell = []struct {
	ruta   string
	minimo int
	porQue string
}{
	{"deploy/lib-agente-windows.sh", 2,
		"RESOLVER y CLASIFICAR, que abren con `NOMBRE='` y se anteponen a las 8 llamadas a `ps1`"},
	{"deploy/actualizar-agente-windows.sh", 5,
		"los 5 `llamar \"$(ps1 ...)\"`: bajar el binario, refrescar el cambiador, migrar el lanzador, lanzar el cambiador, confirmar el agente vivo"},
	{"deploy/matar-zombis-agente.sh", 2,
		"el VEREDICTO y el bloque que mata a los zombis (donde estaba el bug del 2026-09-10)"},
	{"deploy/agente-windows.ps1", 40, "el instalador; hoy tiene 74 cadenas dobles"},
	{"deploy/confiar-editor-windows.ps1", 30, "hoy tiene 66 cadenas dobles"},
	{"deploy/connect-brain-windows.ps1", 25, "hoy tiene 54 cadenas dobles"},
	{"deploy/diagnostico-cortes-windows.ps1", 40, "hoy tiene 81 cadenas dobles"},
	{"deploy/cambiar-agente.cmd", 1,
		"el `powershell -NoProfile -Command` que mata los procesos por RUTA exacta (:69)"},
	// EL HERMANO QUE NO VIVE EN `deploy/`. Lo encontró el control al revés de acá abajo, no yo:
	// al empezar a mirar `scripts/` la prueba se puso roja diciendo «este `.ps1` no está en la
	// tabla». Es el instalador que se baja de la web, o sea el PowerShell que más gente corre.
	{"scripts/install.ps1", 12, "el instalador público; hoy tiene 23 cadenas dobles"},
	// LOS HERMANOS DE LA RAÍZ. `install.bat` es el `.bat` de doble clic que arranca el instalador
	// público, y lleva LA MISMA construcción que `cambiar-agente.cmd:69`. Se le plantó el defecto
	// y la reescritura 1 dio PASS: miraba `deploy/` y `scripts/`, y este vive un piso más arriba.
	{"install.bat", 1,
		"el `powershell -NoProfile -ExecutionPolicy Bypass -Command \"irm ... | iex\"` de doble clic (:38)"},
	// `musubi-setup.bat` HOY NO INVOCA POWERSHELL: lo nombra en un `echo` que le dice al usuario
	// dónde está el instalador. Entra igual al barrido —el día que lo invoque, se mira sin que
	// nadie tenga que acordarse— y entra al inventario con mínimo 0 para que un renombre o un
	// borrado no pase inadvertido. El 0 acá está MEDIDO y dicho, que es lo contrario de un hueco.
	{"musubi-setup.bat", 0,
		"hoy sólo nombra `scripts/install.ps1` en un echo; no invoca PowerShell (medido)"},
	// LA CLASE QUE LA REESCRITURA 1 DECLARÓ SIN CUBRIR, y que el saboteador usó: el PowerShell que
	// arman los `.go`. `internal/fleet/aviso.go` NO está acá a propósito: nombra `powershell` en
	// un comentario y no invoca nada — el avisador de Windows vive en `cmd/musubi/avisador_windows.go`.
	{"cmd/musubi/install.go", 1,
		"el `$d=...; if ($p -notlike ...)` que agrega el PATH y MUSUBI_BIN, en un RAW STRING (:141)"},
	{"internal/provision/network.go", 2,
		"las reglas de firewall: `runPowerShell(script string)` es envoltorio, así que se sigue el parámetro hasta sus llamadores"},
	{"cmd/musubi/servicios_windows.go", 1, "el `Get-CimInstance Win32_Service | ConvertTo-Csv` (:43)"},
	{"cmd/musubi/avisador_windows.go", 2, "los dos MessageBox: el aviso y la pregunta (:85 y :99)"},
	{"internal/fleet/colector_windows.go", 1, "la temperatura por `MSAcpi_ThermalZoneTemperature` (:198)"},
}

func verificarInventario(t *testing.T, raiz string, shs, ps1s, cmds []string, unidades map[string]int) {
	t.Helper()

	esperado := map[string]bool{}
	for _, e := range inventarioPowerShell {
		esperado[e.ruta] = true
		ruta := filepath.Join(raiz, e.ruta)
		if _, err := os.Stat(ruta); err != nil {
			t.Errorf("el inventario nombra %s y no está en el disco (%v). O se renombró y hay que "+
				"actualizar la tabla, o se borró: en los dos casos esta guarda dejó de mirar «%s»",
				e.ruta, err, e.porQue)
			continue
		}
		if n := unidades[e.ruta]; n < e.minimo {
			t.Errorf("de %s se extrajeron %d piezas de PowerShell y se esperaban al menos %d (%s). "+
				"Probablemente cambió cómo se escribe el PowerShell ahí y la extracción dejó de "+
				"reconocerlo: la guarda estaría en verde SIN HABER MIRADO ese archivo",
				e.ruta, n, e.minimo, e.porQue)
		}
	}

	// El control al revés: nada con PowerShell puede quedar fuera del inventario.
	for _, ruta := range ps1s {
		if !esperado[ruta] {
			t.Errorf("%s es un `.ps1` —o sea PowerShell entero— y no está en `inventarioPowerShell`. "+
				"Agregalo con su mínimo: si no, esta guarda lo mira hoy y nadie se entera el día que "+
				"la extracción deje de verlo", ruta)
		}
	}
	for ruta, n := range unidades {
		if n == 0 || esperado[ruta] {
			continue
		}
		t.Errorf("de %s se extrajeron %d piezas de PowerShell y el archivo no está en "+
			"`inventarioPowerShell`. Agregalo con su mínimo", ruta, n)
	}

	if t.Failed() {
		return
	}
	total := 0
	nombres := make([]string, 0, len(unidades))
	for ruta, n := range unidades {
		total += n
		nombres = append(nombres, fmt.Sprintf("%s=%d", filepath.Base(ruta), n))
	}
	sort.Strings(nombres)
	t.Logf("PowerShell mirado: %d piezas · %s", total, strings.Join(nombres, " "))
}

// ────────────────────────────────────────────────────────────────────────────────────────────
// QUÉ ARCHIVOS SE MIRAN

// carpetasDeGuiones se recorren enteras; ADEMÁS se miran los archivos sueltos de la RAÍZ del
// repo, que es donde viven `install.bat` y `musubi-setup.bat` — los dos de doble clic, o sea los
// que corre alguien que no es de acá.
var carpetasDeGuiones = []string{"deploy", "scripts", "."}

// archivosDeGuiones devuelve las rutas RELATIVAS A LA RAÍZ del repo, que son las que nombra el
// inventario: así una tabla que dice `deploy/lib-agente-windows.sh` no puede confundirse con un
// archivo del mismo nombre en otra carpeta.
func archivosDeGuiones(t *testing.T, raiz string) (shs, ps1s, cmds []string) {
	t.Helper()
	clasificar := func(rel string) {
		switch strings.ToLower(filepath.Ext(rel)) {
		case ".sh":
			shs = append(shs, rel)
		case ".ps1":
			ps1s = append(ps1s, rel)
		case ".cmd", ".bat":
			cmds = append(cmds, rel)
		}
	}
	for _, carpeta := range carpetasDeGuiones {
		if carpeta == "." {
			// La raíz NO se recorre en profundidad —abajo cuelga el repo entero— pero sí se
			// miran sus archivos sueltos.
			entradas, err := os.ReadDir(raiz)
			if err != nil {
				t.Fatalf("no pude leer la raíz %s: %v — no medí nada", raiz, err)
			}
			for _, e := range entradas {
				if !e.IsDir() {
					clasificar(e.Name())
				}
			}
			continue
		}
		err := filepath.WalkDir(filepath.Join(raiz, carpeta), func(ruta string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			rel, err := filepath.Rel(raiz, ruta)
			if err != nil {
				return err
			}
			clasificar(filepath.ToSlash(rel))
			return nil
		})
		if err != nil {
			t.Fatalf("no pude recorrer %s: %v — no medí nada", carpeta, err)
		}
	}
	sort.Strings(shs)
	sort.Strings(ps1s)
	sort.Strings(cmds)
	return
}

var carpetasQueNoSonCodigo = map[string]bool{
	".git": true, "vendor": true, "node_modules": true, ".claude": true, "dist": true,
}

func archivosGo(t *testing.T, raiz string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(raiz, func(ruta string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if carpetasQueNoSonCodigo[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		rel, err := filepath.Rel(raiz, ruta)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("no pude recorrer los `.go` desde %s: %v — no medí nada", raiz, err)
	}
	sort.Strings(out)
	return out
}

func leerArchivo(t *testing.T, ruta string) string {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("no pude leer %s: %v — no medí nada", ruta, err)
	}
	return string(b)
}

// ────────────────────────────────────────────────────────────────────────────────────────────
// EL LEXER DE POWERSHELL

type hallazgoPS struct {
	desplazamiento int // dónde está la BARRA, en bytes desde el principio del texto escaneado
	recorte        string
}

// escanearPowerShell devuelve (1) las comillas que cierran una cadena doble teniendo una barra
// pegada a la izquierda, (2) cuántas cadenas dobles miró —la unidad de cobertura— y (3) una
// MÁSCARA del mismo largo con las cadenas, los here-strings y los comentarios reemplazados por
// espacios: o sea, lo que es CÓDIGO y no texto. La máscara es lo que después deja preguntar por
// un `if` sin confundirlo con la palabra «if» adentro de un mensaje.
//
// Sigue las reglas REALES de PowerShell, que son las que deciden dónde termina una cadena:
// el escape es el BACKTICK, un `""` adentro de una cadena doble es una comilla literal, las
// cadenas simples no interpolan, `#` y `<# #>` son comentarios, y `@" "@` / `@' '@` son
// here-strings que terminan sólo con la marca al principio de una línea.
func escanearPowerShell(src string) (hallazgos []hallazgoPS, cadenasDobles int, mascara string) {
	n := len(src)
	m := []byte(src)
	tapar := func(desde, hasta int) {
		if hasta > n {
			hasta = n
		}
		for k := desde; k < hasta; k++ {
			if m[k] != '\n' {
				m[k] = ' '
			}
		}
	}
	defer func() { mascara = string(m) }()

	for i := 0; i < n; {
		c := src[i]
		switch {
		case c == '`':
			tapar(i, i+2)
			i += 2
		case strings.HasPrefix(src[i:], "<#"):
			j := strings.Index(src[i+2:], "#>")
			if j < 0 {
				tapar(i, n)
				return
			}
			tapar(i, i+2+j+2)
			i += 2 + j + 2
		case c == '#' && (i == 0 || strings.ContainsRune(" \t\r\n;{(,|", rune(src[i-1]))):
			j := strings.IndexByte(src[i:], '\n')
			if j < 0 {
				tapar(i, n)
				return
			}
			tapar(i, i+j)
			i += j + 1
		case c == '@' && i+1 < n && (src[i+1] == '"' || src[i+1] == '\'') &&
			(i == 0 || strings.ContainsRune(" \t\r\n=(,", rune(src[i-1]))):
			fin := "\n\"@"
			if src[i+1] == '\'' {
				fin = "\n'@"
			}
			j := strings.Index(src[i+2:], fin)
			if j < 0 {
				tapar(i, n)
				return
			}
			tapar(i, i+2+j+len(fin))
			i += 2 + j + len(fin)
		case c == '\'':
			ini := i
			i++
			for i < n {
				if src[i] == '\'' {
					if i+1 < n && src[i+1] == '\'' { // `''` es una comilla literal
						i += 2
						continue
					}
					i++
					break
				}
				i++
			}
			tapar(ini, i)
		case c == '"':
			ini := i
			cadenasDobles++
			i++
			for i < n {
				ch := src[i]
				if ch == '`' {
					i += 2
					continue
				}
				if ch == '"' {
					if i+1 < n && src[i+1] == '"' { // `""` es una comilla literal
						i += 2
						continue
					}
					// ACÁ ESTÁ LA DECISIÓN. Esta comilla CIERRA la cadena. Si tiene una barra
					// pegada a la izquierda, quien la escribió creía que la estaba escapando.
					if i > ini+1 && src[i-1] == '\\' {
						hallazgos = append(hallazgos, hallazgoPS{
							desplazamiento: i - 1,
							recorte:        recorteDelBloquePS(src, i-1),
						})
					}
					i++
					break
				}
				i++
			}
			tapar(ini, i)
		default:
			i++
		}
	}
	return
}

// ────────────────────────────────────────────────────────────────────────────────────────────
// LAS CONDICIONES QUE NO DECIDEN NADA

type condicionPS struct {
	desplazamiento int
	texto          string
	veredicto      string
}

// `while ($true)` es un bucle legítimo y no entra: acá se miran `if` y `elseif`, que son los que
// prometen filtrar.
var ifDePowerShell = regexp.MustCompile(`(?i)(^|[^\w$-])(elseif|if)[ \t]*\(`)

// condicionesConstantes evalúa la condición de cada `if`/`elseif` y devuelve las que valen
// siempre lo mismo. Se trabaja sobre la MÁSCARA —o sea sobre el código, sin las cadenas ni los
// comentarios— así que ni un `if (` escrito adentro de un mensaje se cuenta, ni un `$true`
// adentro de una cadena hace pasar por constante a una condición que no lo es.
func condicionesConstantes(src, mascara string) []condicionPS {
	var out []condicionPS
	for _, m := range ifDePowerShell.FindAllStringSubmatchIndex(mascara, -1) {
		abre := m[1] - 1 // el `(` es el último carácter del match
		if abre < 0 || abre >= len(mascara) || mascara[abre] != '(' {
			continue
		}
		prof, cierra := 0, -1
		for k := abre; k < len(mascara); k++ {
			switch mascara[k] {
			case '(':
				prof++
			case ')':
				prof--
				if prof == 0 {
					cierra = k
				}
			}
			if cierra >= 0 {
				break
			}
		}
		if cierra < 0 {
			continue
		}
		v, ok := veredictoDeCondicion(mascara[abre+1 : cierra])
		if !ok {
			continue
		}
		out = append(out, condicionPS{desplazamiento: m[2], texto: src[abre+1 : cierra], veredicto: v})
	}
	return out
}

// veredictoDeCondicion aplica la precedencia REAL de PowerShell: `-and` liga más fuerte que
// `-or`. Ésa es la regla que convirtió a `A -and B -or $true` en «siempre verdadera» sin que se
// notara. No se busca ningún texto: se parte la expresión y se evalúa lo que se puede.
func veredictoDeCondicion(cond string) (string, bool) {
	partes := partirEnNivelCero(cond, "-or")
	todasFalsas := true
	for _, p := range partes {
		v, ok := veredictoDeConjuncion(p)
		if ok && v {
			return "siempre VERDADERA", true
		}
		if !ok || v {
			todasFalsas = false
		}
	}
	if todasFalsas && len(partes) > 0 {
		return "siempre FALSA (la rama está muerta)", true
	}
	return "", false
}

func veredictoDeConjuncion(cond string) (bool, bool) {
	partes := partirEnNivelCero(cond, "-and")
	todasVerdaderas := true
	for _, p := range partes {
		v, ok := valorConstante(p)
		if ok && !v {
			return false, true
		}
		if !ok {
			todasVerdaderas = false
		}
	}
	if todasVerdaderas && len(partes) > 0 {
		return true, true
	}
	return false, false
}

// valorConstante reconoce los operandos que no dependen de nada. Deliberadamente corta: si no
// está segura, dice «no sé» y la condición no se reporta. Un falso positivo acá apagaría la
// guarda entera.
func valorConstante(s string) (bool, bool) {
	s = strings.TrimSpace(s)
	for strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") && parentesisBalanceados(s[1:len(s)-1]) {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	if resto, corto := cortarPrefijoDePalabra(s, "-not"); corto {
		v, ok := valorConstante(resto)
		return !v, ok
	}
	if resto, corto := cortarPrefijoDePalabra(s, "!"); corto {
		v, ok := valorConstante(resto)
		return !v, ok
	}
	switch strings.ToLower(s) {
	case "$true", "1":
		return true, true
	case "$false", "0":
		return false, true
	}
	return false, false
}

func cortarPrefijoDePalabra(s, pref string) (string, bool) {
	if !strings.HasPrefix(strings.ToLower(s), pref) {
		return s, false
	}
	resto := s[len(pref):]
	if pref != "!" && resto != "" && !strings.ContainsRune(" \t(", rune(resto[0])) {
		return s, false
	}
	return strings.TrimSpace(resto), true
}

func parentesisBalanceados(s string) bool {
	prof := 0
	for _, c := range s {
		switch c {
		case '(':
			prof++
		case ')':
			prof--
			if prof < 0 {
				return false
			}
		}
	}
	return prof == 0
}

// partirEnNivelCero parte por un operador de PowerShell (`-or`, `-and`) sólo fuera de paréntesis.
func partirEnNivelCero(s, op string) []string {
	var out []string
	prof, ini := 0, 0
	bajo := strings.ToLower(s)
	for i := 0; i < len(s); {
		switch s[i] {
		case '(':
			prof++
		case ')':
			prof--
		}
		if prof == 0 && strings.HasPrefix(bajo[i:], op) &&
			(i == 0 || strings.ContainsRune(" \t\r\n)", rune(s[i-1]))) {
			fin := i + len(op)
			if fin >= len(s) || strings.ContainsRune(" \t\r\n(", rune(s[fin])) {
				out = append(out, s[ini:i])
				i = fin
				ini = fin
				continue
			}
		}
		i++
	}
	return append(out, s[ini:])
}

// ────────────────────────────────────────────────────────────────────────────────────────────
// EL LEXER DE BASH

type trozoBash struct {
	enArchivo int // desplazamiento en bytes dentro del archivo
	texto     string
}

type literalBash struct {
	prefijo   string      // la palabra pegada a la izquierda de la comilla que lo abre
	invocante string      // la palabra ANTERIOR — `ps1` en `ps1 '...'`, `$(ps1 '...'`, …
	trozos    []trozoBash // partido por las interpolaciones `'"$VAR"'`
	texto     string      // los trozos, con la interpolación sustituida por una marca sin comillas
}

// marcaDeInterpolacion ocupa el lugar de un `'"$VAR"'` en el texto del bloque. No lleva comillas
// ni barras: no puede inventar ni tapar un hallazgo.
const marcaDeInterpolacion = "INTERPOLACIONDEBASH"

// lineaEnArchivo traduce un desplazamiento dentro de `texto` a la línea del archivo.
func (l literalBash) lineaEnArchivo(src string, off int) int {
	if len(l.trozos) == 0 {
		return 0
	}
	acum := 0
	for _, tr := range l.trozos {
		if off <= acum+len(tr.texto) {
			return 1 + strings.Count(src[:tr.enArchivo+(off-acum)], "\n")
		}
		acum += len(tr.texto) + len(marcaDeInterpolacion)
	}
	ult := l.trozos[len(l.trozos)-1]
	return 1 + strings.Count(src[:ult.enArchivo+len(ult.texto)], "\n")
}

// literalesDeBash lexea bash y devuelve TODOS los literales `'...'`, con la palabra pegada a la
// izquierda (`prefijo`) y la palabra anterior (`invocante`) como datos — no como filtro.
//
// LA REESCRITURA 1 FILTRABA ACÁ POR EL PREFIJO: sólo se quedaba con los que abrían pegados a un
// `"` o a un `=`, porque así estaban escritos los bloques que conocía:
//
//	ps1 "$RESOLVER"'$n = Join-Path $d "musubi-nuevo.exe" ...'      (abre con `"'`)
//	RESOLVER='$ErrorActionPreference = "SilentlyContinue" ...'     (abre con `='`)
//
// Y EL PREFIJO NO DECIDE NADA. Se plantó un bloque con la comilla suelta —`ps1 'BLOQUE'`, el
// mismo helper, sin `RESOLVER` ni `NOMBRE=` delante— en `matar-zombis-agente.sh:111` y la guarda
// dio PASS. Agregar «la comilla suelta» a la lista de prefijos habría dejado pasar el siguiente.
// Ahora salen todos y quien decide es `literalEsPowerShell`, que mira EL CONTENIDO y A DÓNDE VA.
//
// EL ESTADO DE COMILLAS SE SIGUE DE VERDAD, y ésa es la diferencia con la primera versión. Sin
// eso, el `"'` de un `trap 'rm -rf "$TMP"' EXIT` —donde esa comilla simple CIERRA, no abre— se
// lee como el principio de un bloque de PowerShell y la guarda se pone roja sobre bash correcto.
// Se maneja también `$( )` anidado: en `llamar "$(ps1 "$RESOLVER"'...'")"` la sustitución reabre
// el nivel de comillas.
func literalesDeBash(src string) []literalBash {
	var out []literalBash
	n := len(src)
	dobles := false
	var pila []bool // niveles de `$( )`, con el estado de comillas de afuera
	for i := 0; i < n; {
		c := src[i]
		if c == '\\' {
			i += 2
			continue
		}
		if dobles {
			switch {
			case strings.HasPrefix(src[i:], "$("):
				pila = append(pila, dobles)
				dobles = false
				i += 2
			case c == '"':
				dobles = false
				i++
			default:
				i++
			}
			continue
		}
		switch {
		case c == '#' && (i == 0 || strings.ContainsRune(" \t\n;&|(", rune(src[i-1]))):
			j := strings.IndexByte(src[i:], '\n')
			if j < 0 {
				return out
			}
			i += j + 1
		case c == '"':
			dobles = true
			i++
		case c == ')' && len(pila) > 0:
			dobles = pila[len(pila)-1]
			pila = pila[:len(pila)-1]
			i++
		case strings.HasPrefix(src[i:], "$("):
			pila = append(pila, false)
			i += 2
		case c == '\'':
			k := i - 1
			for k >= 0 && !strings.ContainsRune(" \t\n", rune(src[k])) {
				k--
			}
			prefijo := src[k+1 : i]

			ini := i + 1
			j := strings.IndexByte(src[ini:], '\'')
			if j < 0 {
				return out
			}
			lit := literalBash{prefijo: prefijo, invocante: palabraAnterior(src, k)}
			lit.trozos = append(lit.trozos, trozoBash{enArchivo: ini, texto: src[ini : ini+j]})
			sig := ini + j + 1
			// `'"$VAR"'` — la interpolación cierra el literal y lo vuelve a abrir. Es EL MISMO
			// bloque de PowerShell: si no se pegan, un bloque se cuenta tres veces (y así el
			// control de cobertura anterior llegaba a diecisiete «bloques» teniendo ocho).
			for sig < n && src[sig] == '"' {
				m := sig + 1
				for m < n {
					if src[m] == '\\' {
						m += 2
						continue
					}
					if src[m] == '"' {
						break
					}
					m++
				}
				if m >= n {
					break
				}
				if m+1 < n && src[m+1] == '\'' {
					ini2 := m + 2
					j2 := strings.IndexByte(src[ini2:], '\'')
					if j2 < 0 {
						break
					}
					lit.trozos = append(lit.trozos, trozoBash{enArchivo: ini2, texto: src[ini2 : ini2+j2]})
					sig = ini2 + j2 + 1
					continue
				}
				sig = m + 1
				break
			}
			partes := make([]string, 0, len(lit.trozos))
			for _, tr := range lit.trozos {
				partes = append(partes, tr.texto)
			}
			lit.texto = strings.Join(partes, marcaDeInterpolacion)
			out = append(out, lit)
			i = sig
		default:
			i++
		}
	}
	return out
}

// palabraAnterior devuelve la palabra que está antes del prefijo pegado, sin los caracteres con
// los que bash la puede abrir (`$(`, `(`, “ ` “, `|`, `;`, `&`). En `llamar "$(ps1 'BLOQUE'` la
// palabra anterior a `'BLOQUE'` es `ps1`.
func palabraAnterior(src string, hasta int) string {
	k := hasta
	for k >= 0 && strings.ContainsRune(" \t", rune(src[k])) {
		k--
	}
	fin := k + 1
	for k >= 0 && !strings.ContainsRune(" \t\n", rune(src[k])) {
		k--
	}
	if fin <= k+1 {
		return ""
	}
	// `llamar "$(ps1 '...'` — la palabra anterior viene con todo lo que bash le pega adelante.
	return strings.TrimLeft(src[k+1:fin], "$(`|;&{\"'")
}

// cmdletDePowerShell reconoce un `Verbo-Sustantivo` de PowerShell, que es la marca que ningún
// texto de bash de este repo tiene por casualidad.
var cmdletDePowerShell = regexp.MustCompile(
	`\b(Get|Set|New|Remove|Move|Copy|Start|Stop|Test|Join|Split|Where|ForEach|Select|Sort|Write|Out|Invoke|Add|Wait|Resolve|Measure|Compare|Import|Export|Register|Unregister|Enable|Disable|Convert)-[A-Z][A-Za-z]+\b`)

func textoEsPowerShell(s string) bool {
	return cmdletDePowerShell.MatchString(s) ||
		strings.Contains(s, "$ErrorActionPreference") ||
		strings.Contains(s, "$LASTEXITCODE") ||
		strings.Contains(s, "-ErrorAction")
}

// literalEsPowerShell decide con LO QUE EL LITERAL HACE, nunca con el prefijo.
//
// Y LA PREGUNTA POR EL CONTENIDO SE LE HACE AL CÓDIGO, NO A LOS COMENTARIOS. Sin eso, el bloque
// de Python que `esperar_comando` le pasa a `python3 -c` entraba como «PowerShell» porque adentro
// tiene un comentario que dice «un `Start-Process` que no encuentra el .exe». Un comentario no
// ejecuta nada: se enmascara (junto con las cadenas) antes de preguntar. Es el mismo error que
// esta guarda ya pagó dos veces en otras: la guarda satisfecha por el texto de al lado.
func literalEsPowerShell(l literalBash, variablesPS, helpers map[string]bool) bool {
	if _, _, codigo := escanearPowerShell(l.texto); textoEsPowerShell(codigo) {
		return true
	}
	// A DÓNDE VA. `ps1 'BLOQUE'` es PowerShell aunque el bloque no tenga ni un cmdlet, porque
	// `ps1` termina en `["powershell","-NoProfile","-Command", <el argumento>]`.
	if helpers[l.invocante] {
		return true
	}
	// La COLA de un bloque puede no tener ni un cmdlet: `VEREDICTO="$RESOLVER$CLASIFICAR"'...'`
	// es PowerShell porque se pega a dos variables que lo son.
	for v := range variablesPS {
		if strings.Contains(l.prefijo, "$"+v) {
			return true
		}
	}
	return false
}

var asignacionDeBash = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)=[^= \t]*$`)

// variablesDeBashQueGuardanPowerShell propaga hasta punto fijo, y ENTRE ARCHIVOS: `RESOLVER` y
// `CLASIFICAR` se definen en `lib-agente-windows.sh` y se usan en los dos guiones que hacen
// `source` de él, así que mirar un archivo por vez no alcanza.
func variablesDeBashQueGuardanPowerShell(shs []string, fuentes map[string]string) map[string]bool {
	vars := map[string]bool{}
	sinHelpers := map[string]bool{}
	for vuelta := 0; vuelta < 8; vuelta++ {
		antes := len(vars)
		for _, ruta := range shs {
			for _, lit := range literalesDeBash(fuentes[ruta]) {
				if !literalEsPowerShell(lit, vars, sinHelpers) {
					continue
				}
				if m := asignacionDeBash.FindStringSubmatch(lit.prefijo); m != nil {
					vars[m[1]] = true
				}
			}
		}
		if len(vars) == antes {
			break
		}
	}
	return vars
}

var definicionDeFuncionBash = regexp.MustCompile(`(?m)^[ \t]*(?:function[ \t]+)?([A-Za-z_][A-Za-z0-9_]*)[ \t]*\(\)[ \t]*\{`)
var mencionaPowerShell = regexp.MustCompile(`(?i)\bpowershell(\.exe)?\b`)

var argumentoPosicionalDeBash = regexp.MustCompile(`\$\{?1\}?|\$@|argv\[1\]`)

// funcionesDeBashQueInvocanPowerShell descubre, LEYENDO EL ARCHIVO, qué funciones terminan
// pasándole su argumento a `powershell`. En este repo es `ps1(){ python3 -c '... json.dumps(
// ["powershell","-NoProfile","-Command",sys.argv[1]]) ...' "$1"; }`, definida por separado en dos
// guiones. Nadie la nombra en esta guarda: si mañana se llama distinto, o aparece otra, se
// descubre igual. Una lista de nombres siempre le falta el próximo.
//
// HACEN FALTA LAS DOS COSAS —nombrar a `powershell` Y usar el argumento— y el nombre tiene que
// estar en CÓDIGO, no en un comentario. Con la sola mención alcanzaba para que `esperar_comando`
// pasara por helper: adentro tiene un comentario que dice «un PowerShell que rompe por sintaxis».
func funcionesDeBashQueInvocanPowerShell(src string) map[string]bool {
	out := map[string]bool{}
	lineas := strings.Split(src, "\n")
	for i, l := range lineas {
		m := definicionDeFuncionBash.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		cuerpo := l
		prof := strings.Count(l, "{") - strings.Count(l, "}")
		for j := i + 1; j < len(lineas) && prof > 0 && j-i < 60; j++ {
			cuerpo += "\n" + lineas[j]
			prof += strings.Count(lineas[j], "{") - strings.Count(lineas[j], "}")
		}
		if mencionaPowerShell.MatchString(sinComentarios(cuerpo)) &&
			argumentoPosicionalDeBash.MatchString(cuerpo) {
			out[m[1]] = true
		}
	}
	return out
}

// sinComentarios saca lo que va de un `#` al final de la línea. Es la misma marca de comentario
// en bash, en Python y en PowerShell, que son los tres lenguajes que conviven adentro de estos
// guiones.
func sinComentarios(s string) string {
	lineas := strings.Split(s, "\n")
	for i, l := range lineas {
		for k := 0; k < len(l); k++ {
			if l[k] == '#' && (k == 0 || strings.ContainsRune(" \t", rune(l[k-1]))) {
				lineas[i] = l[:k]
				break
			}
		}
	}
	return strings.Join(lineas, "\n")
}

// ────────────────────────────────────────────────────────────────────────────────────────────
// LOS `.cmd` Y LOS `.bat`

type invocacionCmd struct {
	linea int
	texto string
}

// comandosPowerShellDeCmd devuelve, de cada invocación de `powershell`, EL TEXTO QUE LE LLEGA A
// POWERSHELL — no el que está escrito en el archivo.
//
// LA REESCRITURA 1 BUSCABA EL TEXTO `-command` LÍNEA POR LÍNEA, y las dos formas que eso no ve se
// plantaron y quedaron en verde:
//
//   - la invocación PARTIDA con `^`, el continuador de línea de cmd (`powershell -NoProfile ^` /
//     `  -Command "..."`): ninguna de las dos líneas tiene las dos cosas;
//   - `-c`, que PowerShell acepta como abreviatura de `-Command` (y también `/c`, `-com`, …).
//
// Así que ahora se pegan las líneas continuadas y DESPUÉS se parte la línea con las reglas reales
// de `CommandLineToArgvW`, que es quien decide qué argumento recibe `powershell.exe`. La bandera
// se reconoce por ser un PREFIJO de `-command` —que es la regla de PowerShell, no una lista—, y
// si no hay bandera se toma el primer posicional, que es el default de `powershell.exe`.
//
// Y ACÁ `\"` NO SIGNIFICA LO MISMO QUE EN LAS OTRAS CLASES. `powershell.exe` es un programa
// normal: en `CommandLineToArgvW` un `\"` SÍ es una comilla escapada y `\\` una barra. O sea que
// un `\"` escrito en el `.cmd` le llega a PowerShell como `"` —correcto— y lo que sí es un
// defecto es `\\\"`, que le llega como `\"`. Por eso se deshace esa capa ANTES de lexear
// PowerShell: copiar la regla de al lado habría dado una guarda que grita en falso.
//
// El `^` de los `^|` de adentro de las comillas no se toca: no cambia dónde empiezan ni terminan
// las cadenas.
// Y LA PALABRA `powershell` TIENE QUE ESTAR EN POSICIÓN DE COMANDO. Buscarla en cualquier lado
// hacía que `REM ... PowerShell 5.1 y cmd.exe ...` —un comentario de `cambiar-agente.cmd:11`—
// entrara como una pieza de PowerShell de más. Un comentario no ejecuta nada.
func comandosPowerShellDeCmd(src string) []invocacionCmd {
	var out []invocacionCmd
	for _, lg := range lineasLogicasDeCmd(src) {
		for _, seg := range segmentosDeCmd(lg.texto) {
			args := argvDeWindows(seg)
			for len(args) > 0 && esArranqueDeCmd(args[0]) {
				args = args[1:]
			}
			if len(args) == 0 || !esPowerShellElPrograma(args[0]) {
				continue
			}
			if ps, ok := guionDeArgv(args[1:]); ok {
				out = append(out, invocacionCmd{linea: lg.linea, texto: ps})
			}
		}
	}
	return out
}

// esPowerShellElPrograma mira el PROGRAMA que se ejecuta, con ruta y extensión: en cmd tanto
// `powershell` como `%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe` son lo mismo.
func esPowerShellElPrograma(arg string) bool {
	arg = strings.ToLower(arg)
	if i := strings.LastIndexAny(arg, `\/`); i >= 0 {
		arg = arg[i+1:]
	}
	return arg == "powershell" || arg == "powershell.exe" || arg == "pwsh" || arg == "pwsh.exe"
}

func esArranqueDeCmd(arg string) bool {
	a := strings.ToLower(arg)
	return a == "start" || a == "call" || a == "@call"
}

// segmentosDeCmd parte una línea lógica en los comandos que cmd va a correr: `&`, `&&`, `|` y
// `||` FUERA de comillas separan; adentro de comillas no (por eso el `^|` de `cambiar-agente.cmd`
// no parte nada), y un `^` afuera escapa al carácter que sigue. Los comentarios (`REM`, `::`) no
// son comandos y se descartan acá.
func segmentosDeCmd(l string) []string {
	var out []string
	enComillas := false
	ini := 0
	agregar := func(s string) {
		s = strings.TrimLeft(s, " \t@(")
		bajo := strings.ToLower(s)
		if strings.HasPrefix(bajo, "::") || bajo == "rem" || strings.HasPrefix(bajo, "rem ") ||
			strings.HasPrefix(bajo, "rem\t") {
			return
		}
		if s != "" {
			out = append(out, s)
		}
	}
	for i := 0; i < len(l); i++ {
		switch {
		case l[i] == '"':
			enComillas = !enComillas
		case !enComillas && l[i] == '^':
			i++ // el `^` escapa al que sigue
		case !enComillas && (l[i] == '&' || l[i] == '|'):
			agregar(l[ini:i])
			if i+1 < len(l) && l[i+1] == l[i] {
				i++
			}
			ini = i + 1
		}
	}
	agregar(l[ini:])
	return out
}

type lineaLogicaCmd struct {
	linea int
	texto string
}

// lineasLogicasDeCmd pega las líneas que terminan en `^`, que es como cmd continúa una línea.
// Un `^^` al final es un circunflejo literal y NO continúa.
func lineasLogicasDeCmd(src string) []lineaLogicaCmd {
	var out []lineaLogicaCmd
	lineas := strings.Split(src, "\n")
	for i := 0; i < len(lineas); i++ {
		texto := strings.TrimRight(lineas[i], "\r")
		inicio := i + 1
		for continuaEnCmd(texto) && i+1 < len(lineas) {
			i++
			texto = strings.TrimSuffix(strings.TrimRight(texto, " \t"), "^") + " " +
				strings.TrimSpace(strings.TrimRight(lineas[i], "\r"))
		}
		out = append(out, lineaLogicaCmd{linea: inicio, texto: texto})
	}
	return out
}

func continuaEnCmd(l string) bool {
	l = strings.TrimRight(l, " \t")
	if !strings.HasSuffix(l, "^") {
		return false
	}
	circunflejos := 0
	for k := len(l) - 1; k >= 0 && l[k] == '^'; k-- {
		circunflejos++
	}
	return circunflejos%2 == 1
}

// banderaDeComando reconoce cualquier abreviatura de `-Command`, que es como PowerShell resuelve
// sus parámetros: `-c`, `-com`, `-Command`, y lo mismo con `/`.
func banderaDeComando(arg string) bool {
	if len(arg) < 2 || (arg[0] != '-' && arg[0] != '/') {
		return false
	}
	nombre := strings.ToLower(arg[1:])
	return nombre != "" && strings.HasPrefix("command", nombre)
}

// banderasConValor son las de `powershell.exe` que se comen el argumento siguiente. Hacen falta
// para saber cuál es el primer POSICIONAL, que es el guion cuando no hay `-Command`.
var banderasConValor = map[string]bool{
	"executionpolicy": true, "file": true, "inputformat": true, "outputformat": true,
	"windowstyle": true, "version": true, "configurationname": true, "encodedcommand": true,
	"ec": true, "psconsolefile": true,
}

func guionDeArgv(args []string) (string, bool) {
	saltar := false
	posicional := ""
	for i, a := range args {
		if saltar {
			saltar = false
			continue
		}
		if a != "" && (a[0] == '-' || a[0] == '/') {
			if banderaDeComando(a) {
				if i+1 < len(args) {
					// EL ARGUMENTO, no lo que queda de la línea: lo que viene después es la
					// redirección de cmd (`>> "%LOG%" 2>&1`), que no es parte del guion.
					return args[i+1], true
				}
				return "", false
			}
			if banderasConValor[strings.ToLower(a[1:])] {
				saltar = true
			}
			continue
		}
		if posicional == "" {
			posicional = a
		}
	}
	// Sin bandera, `powershell.exe` toma lo que queda como `-Command`. Un `>` o un `2>&1` de cmd
	// ya no es un argumento del programa, así que un posicional que empiece ahí no cuenta.
	if posicional != "" && !strings.HasPrefix(posicional, ">") && !strings.HasPrefix(posicional, "2>") {
		return posicional, true
	}
	return "", false
}

// argvDeWindows parte una línea de comandos con las reglas de `CommandLineToArgvW`: las comillas
// delimitan, `\\` es una barra si viene antes de una comilla, `\"` es una comilla literal y `""`
// adentro de comillas también. Corta en el primer redirector de cmd que esté FUERA de comillas,
// porque a partir de ahí ya no es la línea de comandos del programa.
func argvDeWindows(s string) []string {
	var args []string
	var b strings.Builder
	enComillas, hayArg := false, false
	i := 0
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\\':
			barras := 0
			for i+barras < len(s) && s[i+barras] == '\\' {
				barras++
			}
			if i+barras < len(s) && s[i+barras] == '"' {
				b.WriteString(strings.Repeat(`\`, barras/2))
				hayArg = true
				if barras%2 == 1 { // impar: la comilla queda literal
					b.WriteByte('"')
					i += barras + 1
					continue
				}
				i += barras // par: la comilla es delimitador, la ve el paso de abajo
				continue
			}
			b.WriteString(strings.Repeat(`\`, barras))
			hayArg = true
			i += barras
		case c == '"':
			hayArg = true
			if enComillas && i+1 < len(s) && s[i+1] == '"' {
				b.WriteByte('"')
				i += 2
				continue
			}
			enComillas = !enComillas
			i++
		case !enComillas && (c == ' ' || c == '\t'):
			if hayArg {
				args = append(args, b.String())
				b.Reset()
				hayArg = false
			}
			i++
		case !enComillas && (c == '&' || c == '|'):
			// Fin de este comando: lo que sigue es otro comando de cmd, no un argumento.
			if hayArg {
				args = append(args, b.String())
			}
			return args
		default:
			b.WriteByte(c)
			hayArg = true
			i++
		}
	}
	if hayArg {
		args = append(args, b.String())
	}
	return args
}

// ────────────────────────────────────────────────────────────────────────────────────────────
// EL POWERSHELL QUE ARMAN LOS `.go`
//
// LA CLASE QUE LA REESCRITURA 1 DECLARÓ SIN CUBRIR, y que el saboteador usó: se plantó un `\"`
// adentro del RAW STRING de `cmd/musubi/install.go:141` —el que agrega el PATH y `MUSUBI_BIN` al
// instalar en Windows— y la guarda dio PASS.
//
// ACÁ HAY DOS CASOS Y CONFUNDIRLOS ES GRITAR EN FALSO:
//
//	exec.Command("powershell", "-Command", `... -notlike "*\"$d\"*" ...`)   ← DEFECTO: llega `\"`
//	exec.Command("powershell", "-Command", "... Write-Host \"hola\" ...")   ← CORRECTO: llega `"`
//
// No se distinguen a ojo ni con un grep: se distinguen DESHACIENDO LA CAPA, que es lo que hace
// `strconv.Unquote` sobre el literal tal como lo devuelve `go/ast`. Después de eso la pregunta
// vuelve a ser la de siempre —qué comilla cierra la cadena de PowerShell— y la responde el mismo
// lexer que las otras clases. Un `\\\"` adentro de una cadena Go normal también sale ROJO, y
// corresponde: deshecha la capa, llega `\"`.
//
// Y EL ANCLAJE NO ES «una cadena que parece PowerShell». Es LA LLAMADA: un `CallExpr` con un
// argumento literal `powershell`/`powershell.exe`, del que se toma el argumento que sigue a la
// bandera `-Command` y se lo RESUELVE hasta su texto (consts, variables locales, `+`,
// `fmt.Sprintf`, y funciones envoltorio como `runPowerShell(script string)`, siguiendo el
// parámetro hasta sus llamadores hasta punto fijo).

type piezaGo struct {
	ruta  string
	linea int
	texto string
}

// marcaDeInterpolacionGo ocupa el lugar de lo que no se puede resolver (una llamada, un
// parámetro). No lleva comillas ni barras: no puede inventar ni tapar un hallazgo.
const marcaDeInterpolacionGo = "INTERPOLACIONDEGO"

var nombreDePowerShell = regexp.MustCompile(`(?i)^powershell(\.exe)?$`)

func piezasDePowerShellEnGo(t *testing.T, raiz string, gos []string) []piezaGo {
	t.Helper()
	fset := token.NewFileSet()
	archivos := map[string]*ast.File{}
	for _, rel := range gos {
		f, err := parser.ParseFile(fset, filepath.Join(raiz, rel), nil, 0)
		if err != nil {
			t.Fatalf("no pude parsear %s: %v — no medí los `.go`", rel, err)
		}
		archivos[rel] = f
	}

	// envoltorios[func][índice de parámetro] — funciones que le pasan ese parámetro a PowerShell.
	envoltorios := map[string]map[int]bool{}
	vistas := map[token.Pos]piezaGo{}

	for vuelta := 0; vuelta < 8; vuelta++ {
		antesEnv, antesPz := largoEnvoltorios(envoltorios), len(vistas)
		for _, rel := range gos {
			f := archivos[rel]
			ast.Inspect(f, func(nodo ast.Node) bool {
				fd, ok := nodo.(*ast.FuncDecl)
				if !ok {
					return true
				}
				ast.Inspect(fd, func(n ast.Node) bool {
					llamada, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					for _, arg := range guionesDeLaLlamada(llamada, envoltorios) {
						if idx, ok := indiceDeParametro(fd, arg); ok {
							nombre := fd.Name.Name
							if envoltorios[nombre] == nil {
								envoltorios[nombre] = map[int]bool{}
							}
							envoltorios[nombre][idx] = true
							continue
						}
						texto, hay := resolverCadenaGo(arg, 0)
						if !hay {
							continue
						}
						p := fset.Position(arg.Pos())
						vistas[arg.Pos()] = piezaGo{ruta: rel, linea: p.Line, texto: texto}
					}
					return true
				})
				return false
			})
		}
		if largoEnvoltorios(envoltorios) == antesEnv && len(vistas) == antesPz {
			break
		}
	}

	out := make([]piezaGo, 0, len(vistas))
	for _, p := range vistas {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ruta != out[j].ruta {
			return out[i].ruta < out[j].ruta
		}
		return out[i].linea < out[j].linea
	})
	return out
}

func largoEnvoltorios(e map[string]map[int]bool) int {
	n := 0
	for _, m := range e {
		n += len(m)
	}
	return n
}

// guionesDeLaLlamada devuelve las expresiones de esta llamada que terminan en PowerShell: el
// argumento del `-Command` cuando algún argumento es el literal `powershell`, y el argumento que
// ocupa el lugar del parámetro de un envoltorio ya conocido.
func guionesDeLaLlamada(c *ast.CallExpr, envoltorios map[string]map[int]bool) []ast.Expr {
	var out []ast.Expr
	esPS := false
	for _, a := range c.Args {
		if s, ok := literalDeCadena(a); ok && nombreDePowerShell.MatchString(s) {
			esPS = true
		}
	}
	if esPS {
		for i, a := range c.Args {
			s, ok := literalDeCadena(a)
			if !ok || !banderaDeComando(s) {
				continue
			}
			if i+1 < len(c.Args) {
				out = append(out, c.Args[i+1])
			}
		}
	}
	if nombre := nombreDeLaFuncion(c.Fun); nombre != "" {
		for idx := range envoltorios[nombre] {
			if idx < len(c.Args) {
				out = append(out, c.Args[idx])
			}
		}
	}
	return out
}

func nombreDeLaFuncion(f ast.Expr) string {
	switch v := f.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return v.Sel.Name
	}
	return ""
}

// indiceDeParametro dice si `e` es exactamente un parámetro de `fd`, y cuál. Es lo que convierte
// a `runPowerShell(script string)` en un envoltorio y hace que se sigan sus llamadores.
func indiceDeParametro(fd *ast.FuncDecl, e ast.Expr) (int, bool) {
	id, ok := e.(*ast.Ident)
	if !ok || fd.Type.Params == nil {
		return 0, false
	}
	idx := 0
	for _, campo := range fd.Type.Params.List {
		for _, n := range campo.Names {
			if n.Name == id.Name && id.Obj != nil && id.Obj.Decl == campo {
				return idx, true
			}
			idx++
		}
		if len(campo.Names) == 0 {
			idx++
		}
	}
	return 0, false
}

func literalDeCadena(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return s, true
}

// resolverCadenaGo reconstruye EL TEXTO QUE LE LLEGA A POWERSHELL. `strconv.Unquote` deshace la
// capa de escape de Go, que es justo la que distingue el defecto (`\"` en un raw string) de lo
// correcto (`\"` en una cadena normal). Lo que no se puede resolver se reemplaza por una marca
// sin comillas ni barras. Devuelve `false` si no se resolvió NADA: una pieza toda marca no es una
// pieza mirada, y contarla sería decir «medí» sin haber medido.
func resolverCadenaGo(e ast.Expr, prof int) (string, bool) {
	if prof > 12 {
		return marcaDeInterpolacionGo, false
	}
	switch v := e.(type) {
	case *ast.BasicLit:
		if s, ok := literalDeCadena(v); ok {
			return s, true
		}
	case *ast.ParenExpr:
		return resolverCadenaGo(v.X, prof+1)
	case *ast.BinaryExpr:
		if v.Op == token.ADD {
			izq, a := resolverCadenaGo(v.X, prof+1)
			der, b := resolverCadenaGo(v.Y, prof+1)
			return izq + der, a || b
		}
	case *ast.CallExpr:
		// `fmt.Sprintf(formato, ...)` — el formato es el que trae las comillas; los `%s` no.
		if nombreDeLaFuncion(v.Fun) == "Sprintf" && len(v.Args) > 0 {
			return resolverCadenaGo(v.Args[0], prof+1)
		}
	case *ast.Ident:
		if v.Obj == nil {
			break
		}
		switch d := v.Obj.Decl.(type) {
		case *ast.ValueSpec:
			for i, n := range d.Names {
				if n.Name == v.Name && i < len(d.Values) {
					return resolverCadenaGo(d.Values[i], prof+1)
				}
			}
		case *ast.AssignStmt:
			for i, l := range d.Lhs {
				id, ok := l.(*ast.Ident)
				if ok && id.Name == v.Name && i < len(d.Rhs) {
					return resolverCadenaGo(d.Rhs[i], prof+1)
				}
			}
		}
	}
	return marcaDeInterpolacionGo, false
}

func recorteDelBloquePS(s string, en int) string {
	ini := en - 55
	if ini < 0 {
		ini = 0
	}
	fin := en + 25
	if fin > len(s) {
		fin = len(s)
	}
	return strings.ReplaceAll(s[ini:fin], "\n", "⏎")
}
