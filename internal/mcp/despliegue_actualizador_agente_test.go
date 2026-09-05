package mcp

// despliegue_actualizador_agente_test.go — las guardas de A98.
//
// OJO CON EL NOMBRE DEL ARCHIVO. La primera versión se llamó `despliegue_agente_windows_test.go`
// y Go la IGNORÓ ENTERA: un archivo terminado en `_windows_test.go` lleva una restricción de
// build implícita por GOOS y sólo compila en Windows. `go test` contestó `ok` y `go vet` pasó; las
// cuatro pruebas no existían. Es el mismo defecto que este archivo custodia, una vuelta más
// adentro: algo que no falla y afirma lo que no es. Cualquier `_linux`, `_darwin`, `_windows`,
// `_amd64`… antes del `_test.go` hace lo mismo.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL CABO: EL ACTUALIZADOR ELEGÍA LA INSTALACIÓN EQUIVOCADA, Y EL CAMBIADOR OBEDECÍA IGUAL
//
// `deploy/actualizar-agente-windows.sh` resolvía la carpeta con
// `Get-Process musubi | Select-Object -First 1`. En `davantis-1` conviven DOCE procesos
// `musubi.exe` en DOS carpetas —el agente de flota y la app de escritorio, que levanta sus propios
// `daemon` y `cerebro`— y el 2026-09-05 eligió la equivocada: la actualización entera fue a la app.
//
// Y lo peor no fue la selección: `cambiar-agente.cmd` para la tarea POR NOMBRE GLOBAL, que es
// única por máquina. Una copia del cambiador en cualquier carpeta PARA AL AGENTE REAL y después
// cambia un binario que no es el suyo. Eso pasó. Lo único que evitó el destrozo fue una
// casualidad: esa carpeta no tenía `device.token` y el paso [4] no pudo probar el binario nuevo.
//
// POR QUÉ ESTAS PRUEBAS SON DE GREP Y NO DE COMPORTAMIENTO: son dos guiones —bash y cmd— que se
// ejecutan en una Windows remota. No hay forma de correrlos acá. Pero la propiedad que importa SÍ
// se puede custodiar desde el repo, y es la que se perdió: que la elección sea por `device.token`
// y no por «el primer proceso», y que el candado del cambiador esté ANTES de tocar la tarea. Sus
// dos hermanos ya tenían pruebas que nombran sus sabotajes; este guion no tenía ninguna, y es el
// que causó el incidente.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"strings"
	"testing"
)

// Usa el `leerDeploy` variádico de despliegue_alertas_test.go: duplicarlo sería, otra vez, la
// misma cautela escrita dos veces y divergiendo.

// Sabotaje que la hace fallar: volver a poner `Select-Object -First 1` para elegir la instalación.
func TestElActualizadorNoEligeLaInstalacionPorElPrimerProcesoQueAparezca(t *testing.T) {
	g := leerDeploy(t, "actualizar-agente-windows.sh")

	// SE MIRA LÍNEA POR LÍNEA Y SE SALTEAN LOS COMENTARIOS. La primera versión de esta prueba
	// buscaba las dos cadenas en TODO el archivo y se disparaba con el comentario que explica el
	// cabo — o sea que la documentación del arreglo rompía la guarda del arreglo. Lo que importa
	// es la TUBERÍA: enumerar procesos y quedarse con el primero, en la misma línea.
	for i, linea := range strings.Split(g, "\n") {
		desnuda := strings.TrimSpace(linea)
		if strings.HasPrefix(desnuda, "#") {
			continue
		}
		if strings.Contains(linea, "Get-Process musubi") && strings.Contains(linea, "Select-Object -First 1") {
			t.Fatalf("línea %d: el guion volvió a elegir la instalación con "+
				"`Get-Process musubi | Select-Object -First 1`.\n"+
				"En una máquina con la app de escritorio Y el agente de flota corriendo eso elige la que\n"+
				"aparezca primero — y el 2026-09-05 eligió la app: la actualización entera fue a la carpeta\n"+
				"equivocada y el agente ni se enteró. Elegí por device.token (cabo A98).", i+1)
		}
	}
	if !strings.Contains(g, "device.token") {
		t.Fatal("el guion ya no elige por `device.token`: sin ese discriminante no hay forma de saber\n" +
			"cuál de las instalaciones es el agente de flota")
	}
}

// La otra mitad de A98: elegir bien no alcanza si ante la duda se elige igual.
//
// Sabotaje que la hace fallar: sacar cualquiera de los dos `exit 1` del resolver.
func TestElActualizadorSeParaSiNoPuedeDecidirCualEsElAgente(t *testing.T) {
	g := leerDeploy(t, "actualizar-agente-windows.sh")
	for _, frase := range []string{
		"$conToken.Count -eq 0", // ninguna candidata tiene la credencial
		"$conToken.Count -gt 1", // dos instalaciones con credencial: no se elige a ciegas
	} {
		if !strings.Contains(g, frase) {
			t.Fatalf("falta la guarda `%s`: ante 0 o ante 2 candidatas el guion tiene que PARAR.\n"+
				"Elegir a ciegas es exactamente lo que produjo el cabo A98", frase)
		}
	}
}

// EL CANDADO VA DONDE ESTÁ LA ACCIÓN, NO SÓLO DONDE ESTÁ LA DECISIÓN.
//
// `schtasks /end /tn "Musubi Agente de Flota"` golpea al agente real desde CUALQUIER carpeta,
// porque el nombre de la tarea es único por máquina. Si el candado quedara después, una copia del
// cambiador en la carpeta equivocada ya habría parado al agente antes de darse cuenta.
//
// Sabotaje que la hace fallar: mover el `if not exist "%DIR%\device.token"` debajo del `schtasks
// /end`, o sacarlo.
func TestElCambiadorNoTocaLaTareaAntesDeConfirmarQueEsLaCarpetaDelAgente(t *testing.T) {
	c := leerDeploy(t, "cambiar-agente.cmd")

	guarda := strings.Index(c, `if not exist "%DIR%\device.token"`)
	if guarda < 0 {
		t.Fatal("el cambiador perdió su candado: sin comprobar `device.token` en SU carpeta puede\n" +
			"parar la tarea del agente real desde cualquier lado. Pasó el 2026-09-05 (cabo A98)")
	}
	fin := strings.Index(c, "schtasks /end")
	if fin < 0 {
		t.Fatal("no encuentro el `schtasks /end` del paso [1]: ¿cambió el cambiador?")
	}
	if guarda > fin {
		t.Fatal("el candado de `device.token` quedó DESPUÉS del `schtasks /end`.\n" +
			"Para entonces el agente real ya está parado, que es justo el daño que el candado evita")
	}
}

// El cambiador es ASCII puro a propósito (lo dice su propia cabecera): cmd.exe y PowerShell 5.1
// con UTF-8 sin BOM ya rompieron una vez. Un comentario nuevo con acentos lo vuelve a romper.
func TestElCambiadorSigueSiendoAsciiPuro(t *testing.T) {
	for i, r := range leerDeploy(t, "cambiar-agente.cmd") {
		if r > 127 {
			t.Fatalf("byte no-ASCII en la posición %d (%q): el cambiador declara ASCII puro porque\n"+
				"cmd.exe ya rompió una vez con UTF-8 sin BOM", i, r)
		}
	}
}

// EL VERDE FINAL NO LO PUEDE DAR LA VERSIÓN SOLA.
//
// El paso [4] de `cambiar-agente.cmd` corre `musubi.exe agent --once` para PROBAR el binario
// nuevo, y ese latido de prueba ya escribe la versión nueva en la fila del cerebro. Si el paso [5]
// falla —`schtasks /run` no arranca la tarea— la máquina queda SIN AGENTE y el guion la declararía
// actualizada igual: el verde lo daría un campo que la propia prueba escribe.
//
// Sabotaje que la hace fallar: volver a poner `[[ "$V" == "$VERSION" ]] && { ok ...; exit 0; }`
// sin la confirmación en la máquina.
func TestElActualizadorNoCantaVictoriaSoloPorLaVersionReportada(t *testing.T) {
	g := leerDeploy(t, "actualizar-agente-windows.sh")

	confirma := strings.Index(g, "confirmando EN LA MÁQUINA")
	if confirma < 0 {
		t.Fatal("el guion perdió la confirmación en la máquina: sin ella, un `schtasks /run` que no\n" +
			"arranca deja la máquina sin agente y el guion la declara actualizada, porque el latido\n" +
			"de PRUEBA del paso [4] ya escribió la versión nueva (cabo A98)")
	}
	victoria := strings.Index(g, `ok "ACTUALIZADA`)
	if victoria < 0 {
		t.Fatal("no encuentro el mensaje de éxito: ¿cambió el guion?")
	}
	if victoria < confirma {
		t.Fatal("el guion canta victoria ANTES de confirmar que quedó un agente vivo en la máquina")
	}
}
