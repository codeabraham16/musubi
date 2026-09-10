package mcp

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
// POR QUÉ ESTA GUARDA SE REESCRIBIÓ ENTERA (2026-09-10, el mismo día)
//
// La primera versión miraba SÓLO `deploy/*.sh`, y sólo los literales de bash que abrían con la
// secuencia `"'`. Le faltaba justo lo que había al lado, y se midió con sabotajes reales:
//
//  1. ERA CIEGA A `deploy/lib-agente-windows.sh`. Ahí viven `RESOLVER` y `CLASIFICAR`, que abren
//     con `NOMBRE='` y no con `"'`. Se les plantó el `\"` exacto adentro y `go test` dio PASS.
//     No es un archivo más: `RESOLVER` se antepone a las OCHO llamadas a `ps1` de los dos
//     guiones, así que un `\"` ahí rompe TODOS los comandos remotos de Windows a la vez.
//  2. ERA CIEGA A LOS `.ps1`. Los descartaba por extensión, y en `agente-windows.ps1:222` había
//     un `\"` VIVO: el instalador imprimía `curl.exe -sS -o NUL -w \%{http_code}\ ...` como el
//     comando a copiar para distinguir «es la red» de «es un filtro por proceso».
//  3. SE PONÍA ROJA EN FALSO SOBRE BASH SANO. `"'` también aparece al CERRAR una cadena de bash:
//     un `trap 'rm -rf "$TMP"' EXIT` la hacía abrir un «bloque de PowerShell» de mil caracteres
//     de bash. Una guarda que grita en falso se termina apagando.
//  4. SU CONTROL DE COBERTURA ERA AIRE. Exigía «al menos 10 bloques» y contaba 17, pero CINCO
//     salían de `install-musubi-brain.sh` y `verificar-cobertura.sh`, que no tienen una sola
//     línea de PowerShell, y los reales venían partidos en pedazos por las interpolaciones.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// CÓMO SE MIDE AHORA
//
// PowerShell vive en este repo, fuera del código Go, en cuatro formas — y las cuatro se miran,
// en `deploy/` Y en `scripts/`, porque el hermano de `deploy/*.ps1` es `scripts/install.ps1`:
//
//	a) los `.sh` que lo GENERAN en literales de bash que abren con `"'` (`ps1 "$RESOLVER"'...'`);
//	b) los `.sh` que lo GENERAN en literales que abren con `NOMBRE='` (`RESOLVER=`, `CLASIFICAR=`);
//	c) los `.ps1`, que son PowerShell de punta a punta;
//	d) los `.cmd`, en el argumento de `powershell -NoProfile -Command "..."`.
//
// Y LA DISTINCIÓN BASH/POWERSHELL NO SE HACE POR OLFATO. Se lexea bash de verdad —comillas
// simples, dobles, `\`, `#`, y `$( )` anidado, que es lo que le desordenaba el estado a la
// primera versión— así que la comilla simple que CIERRA el `trap` se reconoce como cierre y no
// como apertura. Después, sobre el texto ya extraído, se lexea PowerShell —comillas simples,
// dobles con escape de BACKTICK y `""`, comentarios `#` y `<# #>`, here-strings `@" "@`— y el
// hallazgo es la comilla que cierra una cadena doble teniendo una BARRA pegada a la izquierda.
// Ese es el carácter que DECIDE dónde termina la cadena; no es un grep del archivo.
//
// LA CLASE QUE NO SE MIRA, DICHA EN VOZ ALTA: el PowerShell que arman los `.go` y le pasan a
// `exec.Command("powershell", "-Command", ...)` — `cmd/musubi/install.go:141`,
// `internal/provision/network.go:122`, y los `*_windows.go`. Ahí `\"` vuelve a significar otra
// cosa (en una cadena Go entre comillas es el escape CORRECTO y produce `"`; el defecto sería un
// `\"` adentro de una cadena Go entre backticks). Hoy no hay ni un `\"` en ninguno de esos
// archivos —se midió—, pero la guarda no los cubre: hace falta un extractor propio.
//
// SABOTAJES CORRIDOS CONTRA ESTA VERSIÓN, a mano y restaurados con `cp` (no con `git checkout`,
// que no conoce un archivo nuevo):
//   - ROJO: `\"` en `RESOLVER`, en `deploy/lib-agente-windows.sh:60` (clase b). La guarda anterior
//     daba PASS sobre este mismo sabotaje.
//   - ROJO: `\"` en el bloque de `deploy/matar-zombis-agente.sh:140` (clase a) — el bug original.
//   - ROJO: `\"` en `deploy/agente-windows.ps1:222` (clase c) — el defecto que estaba VIVO.
//   - ROJO: `\"` en `scripts/install.ps1:24` (clase c, el hermano que no vive en `deploy/`).
//   - ROJO: `\\\"` en `deploy/cambiar-agente.cmd:69` (clase d), que es lo que le llega a
//     PowerShell como `\"`. Ojo: un `\"` escrito ahí NO es el defecto, y la guarda no se pone
//     roja por él — ver `comandosPowerShellDeCmd`.
//   - VERDE: el `trap 'rm -rf "$TMP"' EXIT` de `deploy/verificar-cobertura.sh`, con un `echo
//     "... \"$TMP\" ..."` plantado detrás que es bash CORRECTO (`bash -n` limpio, imprime bien).
//     La versión anterior se ponía ROJA acá.
//
// Y LOS CONTROLES TAMBIÉN SE PROBARON ROJOS, que es lo que separa «medí y está bien» de «no medí»:
//   - renombrar `lib-agente-windows.sh` → falla por las dos puntas (falta en el inventario, y el
//     archivo nuevo tiene PowerShell y no está en la tabla);
//   - cambiar `CLASIFICAR='` por `CLASIFICAR=$'` → falla con «se extrajeron 1 y se esperaban 2»;
//   - sacar el único `.cmd` → falla con «no encontré archivos que mirar».
func TestNingunPowerShellDeDeployEscapaComillasConBarra(t *testing.T) {
	// SE MIRAN `deploy/` Y `scripts/`. El hermano de `deploy/*.ps1` es `scripts/install.ps1`, que
	// vive un directorio más arriba y que la primera versión de esta guarda no habría visto ni
	// aunque hubiera dejado de descartar los `.ps1`: leía UN solo directorio.
	raiz := filepath.Join("..", "..")
	carpetas := []string{"deploy", "scripts"}

	shs, ps1s, cmds := archivosConPowerShell(t, raiz, carpetas)

	// EL PRIMER CONTROL ES QUE EL GLOB HAYA MATCHEADO. Un cero acá significa «no pude medir»,
	// no «no hay nada malo», y son cosas opuestas: si el árbol se mueve, o el test corre desde
	// otro directorio, o alguien renombra `deploy/`, la guarda tiene que gritar y no pasar.
	if len(shs) == 0 || len(ps1s) == 0 || len(cmds) == 0 {
		t.Fatalf("no encontré archivos que mirar bajo %v: %d .sh, %d .ps1, %d .cmd. "+
			"Un cero acá es «no pude medir», no «está todo bien»", carpetas, len(shs), len(ps1s), len(cmds))
	}

	// Las variables de bash que GUARDAN PowerShell (`RESOLVER`, `CLASIFICAR`, `VEREDICTO`).
	// Hacen falta porque un literal como el de `VEREDICTO="$RESOLVER$CLASIFICAR"'...'` es la COLA
	// de un bloque: por sí solo no tiene ni un cmdlet, y sin esto se lo tomaría por texto de bash.
	// Se propaga hasta punto fijo y ENTRE ARCHIVOS: `RESOLVER` se define en `lib-agente-windows.sh`
	// y se usa en los otros dos, que lo cargan con `source`.
	fuentes := map[string]string{}
	for _, r := range append(append([]string{}, shs...), append(ps1s, cmds...)...) {
		fuentes[r] = leerArchivo(t, filepath.Join(raiz, r))
	}
	variablesPS := variablesDeBashQueGuardanPowerShell(shs, fuentes)

	// unidades[ruta] = cuántas piezas de PowerShell REALES se miraron en ese archivo.
	unidades := map[string]int{}

	// (a) y (b) — los `.sh` que generan PowerShell.
	for _, ruta := range shs {
		src := fuentes[ruta]
		for _, lit := range literalesDeBash(src) {
			if !literalEsPowerShell(lit, variablesPS) {
				continue
			}
			unidades[ruta]++
			hallazgos, _ := escanearPowerShell(lit.texto)
			for _, h := range hallazgos {
				informar(t, ruta, lit.lineaEnArchivo(src, h.desplazamiento), h.recorte)
			}
		}
	}

	// (c) — los `.ps1` son PowerShell de punta a punta.
	for _, ruta := range ps1s {
		src := fuentes[ruta]
		hallazgos, cadenas := escanearPowerShell(src)
		// La unidad acá es la CADENA DOBLE mirada, que es donde el defecto puede estar. Un `.ps1`
		// sin ninguna cadena doble sería un lexer roto, no un archivo limpio.
		unidades[ruta] = cadenas
		for _, h := range hallazgos {
			informar(t, ruta, 1+strings.Count(src[:h.desplazamiento], "\n"), h.recorte)
		}
	}

	// (d) — los `.cmd`, en el argumento de `powershell -Command "..."`.
	for _, ruta := range cmds {
		src := fuentes[ruta]
		for _, inv := range comandosPowerShellDeCmd(src) {
			unidades[ruta]++
			hallazgos, _ := escanearPowerShell(inv.texto)
			for _, h := range hallazgos {
				informar(t, ruta, inv.linea, h.recorte)
			}
		}
	}

	verificarInventario(t, raiz, shs, ps1s, cmds, unidades)
}

func informar(t *testing.T, ruta string, linea int, recorte string) {
	t.Helper()
	t.Errorf("%s:%d — PowerShell que escapa una comilla con BARRA (`\\\"`).\n\n"+
		"  …%s…\n\n"+
		"En una cadena de PowerShell con comillas dobles el escape es el BACKTICK; la barra no "+
		"escapa nada, así que esa comilla CIERRA la cadena y lo que sigue queda suelto. Si esto "+
		"está en un bloque que se manda entero, el bloque DEJA DE PARSEAR —PowerShell parsea antes "+
		"de ejecutar—, así que también se rompen las ramas que sí se iban a correr; si está en un "+
		"`.ps1`, la línea imprime o ejecuta algo distinto de lo escrito.\n"+
		"Para una comilla literal adentro de la cadena, DOBLALA: `\"\"` (o poné un backtick).\n"+
		"Ojo: `\\\"` SÍ es correcto en las cadenas de BASH de estos mismos archivos (los `echo`, el "+
		"JSON de `musubi_fleet_log`), y por eso esta guarda lexea bash primero y mira sólo lo que "+
		"termina siendo PowerShell.",
		ruta, linea, recorte)
}

// ────────────────────────────────────────────────────────────────────────────────────────────
// EL INVENTARIO. Es el control de cobertura, y cuenta piezas REALES.
//
// El de la versión anterior («al menos 10 bloques en total») se satisfacía con basura: cinco de
// sus diecisiete «bloques» salían de dos archivos sin una línea de PowerShell. Un número global
// acolchado no distingue «miré los ocho bloques de Windows» de «miré cuatro y cuatro fragmentos
// de un `trap`». Éste nombra archivo por archivo lo que hay que encontrar, y falla si un archivo
// del inventario no está en el disco (renombrado o borrado) o si rinde menos de lo esperado
// (la extracción dejó de reconocerlo).
//
// Y hay un control al revés, que es el que evita repetir el defecto que originó esta reescritura:
// un archivo de `deploy/` que TENGA PowerShell y NO esté en esta tabla hace fallar la prueba. Así
// el próximo `.sh` o `.ps1` que aparezca entra por la puerta y no por la ventana.
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
	{"deploy/agente-windows.ps1", 40, "el instalador; hoy tiene 76 cadenas dobles"},
	{"deploy/confiar-editor-windows.ps1", 30, "hoy tiene 66 cadenas dobles"},
	{"deploy/connect-brain-windows.ps1", 25, "hoy tiene 54 cadenas dobles"},
	{"deploy/diagnostico-cortes-windows.ps1", 40, "hoy tiene 81 cadenas dobles"},
	{"deploy/cambiar-agente.cmd", 1,
		"el `powershell -NoProfile -Command` que mata los procesos por RUTA exacta (:69)"},
	// EL HERMANO QUE NO VIVE EN `deploy/`. Lo encontró el control al revés de acá abajo, no yo:
	// al empezar a mirar `scripts/` la prueba se puso roja diciendo «este `.ps1` no está en la
	// tabla». Es el instalador que se baja de la web, o sea el PowerShell que más gente corre.
	{"scripts/install.ps1", 12, "el instalador público; hoy tiene 23 cadenas dobles"},
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
	for _, ruta := range append(append([]string{}, shs...), cmds...) {
		if unidades[ruta] == 0 {
			continue
		}
		if !esperado[ruta] {
			t.Errorf("de %s se extrajeron %d piezas de PowerShell y el archivo no está en "+
				"`inventarioPowerShell`. Agregalo con su mínimo", ruta, unidades[ruta])
		}
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

// archivosConPowerShell devuelve las rutas RELATIVAS A LA RAÍZ del repo, que son las que nombra
// el inventario: así una tabla que dice `deploy/lib-agente-windows.sh` no puede confundirse con
// un archivo del mismo nombre en otra carpeta.
func archivosConPowerShell(t *testing.T, raiz string, carpetas []string) (shs, ps1s, cmds []string) {
	t.Helper()
	for _, carpeta := range carpetas {
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
			rel = filepath.ToSlash(rel)
			switch strings.ToLower(filepath.Ext(ruta)) {
			case ".sh":
				shs = append(shs, rel)
			case ".ps1":
				ps1s = append(ps1s, rel)
			case ".cmd":
				cmds = append(cmds, rel)
			}
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

// escanearPowerShell devuelve las comillas que cierran una cadena doble teniendo una barra
// pegada a la izquierda, y cuántas cadenas dobles miró (que es la unidad de cobertura).
//
// Sigue las reglas REALES de PowerShell, que son las que deciden dónde termina una cadena:
// el escape es el BACKTICK, un `""` adentro de una cadena doble es una comilla literal, las
// cadenas simples no interpolan, `#` y `<# #>` son comentarios, y `@" "@` / `@' '@` son
// here-strings que terminan sólo con la marca al principio de una línea.
func escanearPowerShell(src string) (hallazgos []hallazgoPS, cadenasDobles int) {
	n := len(src)
	for i := 0; i < n; {
		c := src[i]
		switch {
		case c == '`':
			i += 2
		case strings.HasPrefix(src[i:], "<#"):
			j := strings.Index(src[i+2:], "#>")
			if j < 0 {
				return
			}
			i += 2 + j + 2
		case c == '#' && (i == 0 || strings.ContainsRune(" \t\r\n;{(,|", rune(src[i-1]))):
			j := strings.IndexByte(src[i:], '\n')
			if j < 0 {
				return
			}
			i += j + 1
		case c == '@' && i+1 < n && (src[i+1] == '"' || src[i+1] == '\'') &&
			(i == 0 || strings.ContainsRune(" \t\r\n=(,", rune(src[i-1]))):
			fin := "\n\"@"
			if src[i+1] == '\'' {
				fin = "\n'@"
			}
			j := strings.Index(src[i+2:], fin)
			if j < 0 {
				return
			}
			i += 2 + j + len(fin)
		case c == '\'':
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
		default:
			i++
		}
	}
	return
}

// ────────────────────────────────────────────────────────────────────────────────────────────
// EL LEXER DE BASH

type trozoBash struct {
	enArchivo int // desplazamiento en bytes dentro del archivo
	texto     string
}

type literalBash struct {
	prefijo string      // la palabra pegada a la izquierda de la comilla que lo abre
	trozos  []trozoBash // partido por las interpolaciones `'"$VAR"'`
	texto   string      // los trozos, con la interpolación sustituida por una marca sin comillas
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

// literalesDeBash lexea bash y devuelve los literales `'...'` que abren pegados a un `"` o a un
// `=` — las dos formas con las que este repo escribe PowerShell adentro de un guion:
//
//	ps1 "$RESOLVER"'$n = Join-Path $d "musubi-nuevo.exe" ...'      (abre con `"'`)
//	RESOLVER='$ErrorActionPreference = "SilentlyContinue" ...'     (abre con `='`)
//
// EL ESTADO DE COMILLAS SE SIGUE DE VERDAD, y ésa es la diferencia con la primera versión de esta
// guarda. Sin eso, el `"'` de un `trap 'rm -rf "$TMP"' EXIT` —donde esa comilla simple CIERRA, no
// abre— se lee como el principio de un bloque de PowerShell y la guarda se pone roja sobre bash
// correcto. Se maneja también `$( )` anidado, que es donde el intento anterior se perdía: en
// `llamar "$(ps1 "$RESOLVER"'...'")"` la sustitución reabre el nivel de comillas.
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
			lit := literalBash{prefijo: prefijo}
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
			if strings.HasSuffix(prefijo, `"`) || strings.HasSuffix(prefijo, "=") {
				out = append(out, lit)
			}
			i = sig
		default:
			i++
		}
	}
	return out
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

func literalEsPowerShell(l literalBash, variablesPS map[string]bool) bool {
	if textoEsPowerShell(l.texto) {
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
	for vuelta := 0; vuelta < 8; vuelta++ {
		antes := len(vars)
		for _, ruta := range shs {
			for _, lit := range literalesDeBash(fuentes[ruta]) {
				if !literalEsPowerShell(lit, vars) {
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

// ────────────────────────────────────────────────────────────────────────────────────────────
// LOS `.cmd`

type invocacionCmd struct {
	linea int
	texto string
}

var powershellEnCmd = regexp.MustCompile(`(?i)\bpowershell(\.exe)?\b`)

// comandosPowerShellDeCmd devuelve, de cada `powershell ... -Command "..."`, EL TEXTO QUE LE
// LLEGA A POWERSHELL — no el que está escrito en el `.cmd`. Es la cuarta forma en que PowerShell
// vive en `deploy/`: hoy sólo `cambiar-agente.cmd:69`, el que mata los procesos del agente por
// RUTA exacta para no llevarse puesta la app de escritorio. Hoy no tiene ningún hueco vivo; se
// mira para que el día que lo tenga no sea una sorpresa.
//
// Y ACÁ `\"` NO SIGNIFICA LO MISMO QUE EN LAS OTRAS TRES CLASES, así que copiar la regla de al
// lado habría dado una guarda que grita en falso. `powershell.exe` es un programa normal: su
// línea de comandos la parte `CommandLineToArgvW`, donde `\"` SÍ es una comilla escapada y `\\`
// una barra. O sea que un `\"` escrito en el `.cmd` le llega a PowerShell como `"` —correcto— y
// lo que sí es un defecto es `\\\"`, que le llega como `\"` y ahí sí rompe la cadena. Por eso se
// deshace esa capa ANTES de lexear PowerShell: la pregunta es siempre la misma —qué comilla
// decide dónde termina la cadena— pero hay que hacérsela al texto que la responde.
//
// El `^` de los `^|` no se toca: `cmd` lo conserva adentro de comillas y, sea o no un problema
// aparte, no cambia dónde empiezan ni terminan las cadenas.
func comandosPowerShellDeCmd(src string) []invocacionCmd {
	var out []invocacionCmd
	for i, l := range strings.Split(src, "\n") {
		if !powershellEnCmd.MatchString(l) {
			continue
		}
		k := strings.Index(strings.ToLower(l), "-command")
		if k < 0 {
			continue
		}
		abre := strings.IndexByte(l[k:], '"')
		if abre < 0 {
			continue
		}
		var b strings.Builder
		j := k + abre + 1
		for j < len(l) {
			if l[j] == '\\' {
				barras := 0
				for j+barras < len(l) && l[j+barras] == '\\' {
					barras++
				}
				if j+barras < len(l) && l[j+barras] == '"' {
					b.WriteString(strings.Repeat(`\`, barras/2))
					if barras%2 == 1 { // impar: la comilla queda literal
						b.WriteByte('"')
						j += barras + 1
						continue
					}
					j += barras // par: la comilla es delimitador, la ve el paso de abajo
					continue
				}
				b.WriteString(strings.Repeat(`\`, barras))
				j += barras
				continue
			}
			if l[j] == '"' { // cierra el argumento de -Command
				break
			}
			b.WriteByte(l[j])
			j++
		}
		out = append(out, invocacionCmd{linea: i + 1, texto: b.String()})
	}
	return out
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
