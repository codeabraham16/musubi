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

// Las guardas de estos guiones son de grep, así que un comentario que NOMBRA la cautela la
// satisface sin que la cautela exista. Pasó dos veces el 2026-09-05. `codigoDe` deja fuera las
// líneas de comentario —`#` sirve para bash Y para el PowerShell embebido— para que lo único que
// pueda satisfacer una guarda sea código.
func codigoDe(g string) string {
	var b strings.Builder
	for _, linea := range strings.Split(g, "\n") {
		if strings.HasPrefix(strings.TrimSpace(linea), "#") {
			continue
		}
		b.WriteString(linea)
		b.WriteByte('\n')
	}
	return b.String()
}

// Los bloques de PowerShell compartidos viven en `deploy/lib-agente-windows.sh` desde que
// `matar-zombis-agente.sh` los necesita iguales. Las guardas miran el conjunto —guion + lib—
// porque lo que custodian es la PROPIEDAD del despliegue, no en qué archivo quedó escrita.
func guionesDeWindows(t *testing.T) string {
	t.Helper()
	return leerDeploy(t, "actualizar-agente-windows.sh") + "\n" + leerDeploy(t, "lib-agente-windows.sh")
}

// Sabotaje que la hace fallar: volver a poner `Select-Object -First 1` para elegir la instalación.
func TestElActualizadorNoEligeLaInstalacionPorElPrimerProcesoQueAparezca(t *testing.T) {
	g := guionesDeWindows(t)

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
	g := guionesDeWindows(t)
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

// EL LANZADOR TAMBIÉN SE DESPLIEGA (A102).
//
// `agente.cmd` lo escribe SÓLO el instalador. El actualizador refrescaba el binario y
// `cambiar-agente.cmd` —con el argumento «el de la máquina puede ser viejo» escrito en su propio
// paso— y nunca el lanzador. Medido el 2026-09-05: `davantis-1` seguía con la forma vieja
// `set /p MUSUBI_DEVICE_TOKEN=<archivo`, que mete la credencial en el ENTORNO del proceso, corre
// una sola vez al arrancar —así que la rotación en caliente vence siempre— y lee sólo la PRIMERA
// línea del archivo, tirando el formato multi-token que existe para dar fallback a esa rotación.
// El arreglo estaba en el repo desde hacía tiempo y no había llegado a la máquina.
//
// Sabotaje que la hace fallar: sacar el paso que migra el lanzador.
func TestElActualizadorTambienRefrescaElLanzador(t *testing.T) {
	g := leerDeploy(t, "actualizar-agente-windows.sh")
	if !strings.Contains(g, "agente.cmd") {
		t.Fatal("el actualizador dejó de tocar `agente.cmd`: refresca el binario y el cambiador y\n" +
			"deja el lanzador viejo, que es donde vive la forma insegura del token (cabo A102)")
	}
	if !strings.Contains(g, "MUSUBI_DEVICE_TOKEN_FILE") {
		t.Fatal("el paso del lanzador ya no migra a `MUSUBI_DEVICE_TOKEN_FILE`: sin eso la credencial\n" +
			"sigue yendo por el entorno y la rotación en caliente no se puede completar")
	}
	// El respaldo importa tanto como la migración: se reescribe un archivo que el instalador
	// generó con valores propios de la máquina.
	if !strings.Contains(g, "antes-de-la-ruta") {
		t.Fatal("la migración del lanzador dejó de dejar respaldo antes de reescribirlo")
	}
}

// LA CONFIRMACIÓN FINAL NO PUEDE CONFORMARSE CON «HAY UN PROCESO VIVO».
//
// El 2026-09-05 la fila de `davantis-1` reportó durante horas una versión que NO ESTABA EN NINGÚN
// ARCHIVO: la escribía un proceso arrancado antes de que el binario se reemplazara, corriendo una
// imagen que ya no existía en disco. Un chequeo de «hay proceso vivo en esa carpeta» lo habría
// dado por bueno. La propiedad que sí distingue es local a la máquina y no necesita relojes
// sincronizados: el proceso tiene que haber arrancado DESPUÉS de que el archivo se escribiera.
//
// Sabotaje que la hace fallar: sacar la comparación de StartTime contra LastWriteTime.
func TestLaConfirmacionExigeUnProcesoMasNuevoQueElBinario(t *testing.T) {
	// SE MIRAN SÓLO LAS LÍNEAS DE CÓDIGO. La primera versión buscaba las frases en TODO el
	// archivo y `LastWriteTime` aparecía en el comentario que explica qué hace `$escrito`: el
	// sabotaje «que la hora salga del reloj y no del archivo» pasaba en verde, sostenido por la
	// documentación del arreglo. Es el mismo defecto que ya había roto la guarda de `-First 1`
	// esta misma mañana — la segunda vez en un día, y por eso ahora hay un helper.
	g := codigoDe(guionesDeWindows(t))
	for _, frase := range []string{
		"LastWriteTime",             // cuándo se escribió el binario
		"$_.StartTime -gt $escrito", // el proceso arrancó después
		"Get-FileHash $exe",         // y además es EL binario que se instaló
	} {
		if !strings.Contains(g, frase) {
			t.Fatalf("la confirmación final perdió `%s`: sin eso, un zombi corriendo la imagen vieja\n"+
				"la satisface, que es exactamente lo que confundió el diagnóstico del 2026-09-05", frase)
		}
	}
}

// LOS DOS GUIONES DE WINDOWS COMPARTEN LOS BLOQUES, NO UNA COPIA.
//
// Es el defecto dominante de este repo, y con estos mismos archivos ya pasó tres veces: la
// cautela escrita en uno y ausente —o distinta— en su hermano. `cambiar-agente.cmd` sabía de la
// app de escritorio y el actualizador no (A98); el actualizador refrescaba el cambiador y no el
// lanzador (A102); y la confirmación clasificaba procesos de una manera donde el cambiador los
// mataba de otra. Un `source` del mismo archivo hace imposible que uno se arregle sin el otro.
//
// Sabotaje que la hace fallar: pegarle a `matar-zombis-agente.sh` su propia copia del `RESOLVER=`
// en vez de cargar el lib.
func TestLosDosGuionesDeWindowsCarganElMismoLibYNoUnaCopia(t *testing.T) {
	for _, nombre := range []string{"actualizar-agente-windows.sh", "matar-zombis-agente.sh"} {
		g := leerDeploy(t, nombre)
		if !strings.Contains(g, "lib-agente-windows.sh") {
			t.Fatalf("%s no carga `lib-agente-windows.sh`: si tiene su propia copia de los bloques\n"+
				"de PowerShell, el próximo arreglo va a llegar a uno solo — que es el defecto que este\n"+
				"repo persigue desde A98", nombre)
		}
		if strings.Contains(g, "RESOLVER='") || strings.Contains(g, "CLASIFICAR='") {
			t.Fatalf("%s volvió a DEFINIR los bloques en vez de cargarlos del lib: dos copias divergen",
				nombre)
		}
	}
}

// LA CLASIFICACIÓN DE PROCESOS MIRA LAS DOS RUTAS, Y NO FILTRA POR NOMBRE DE PROCESO.
//
// La primera confirmación decía `Get-Process musubi | Where-Object { $_.Path -eq $exe }`, y por
// construcción no podía ver a la mitad de los zombis que decía custodiar:
//
//  1. El paso [2] del cambiador RENOMBRA el binario en uso, así que el agente anterior sobrevive
//     corriendo `musubi.exe.viejo`. `cambiar-agente.cmd` mata las DOS rutas —lo tiene escrito y
//     medido desde el 2026-09-02— y la confirmación miraba una.
//  2. El nombre de proceso de Windows es el del archivo sin su última extensión, así que ese
//     proceso se llama `musubi.exe` y NO `musubi`: `Get-Process musubi` no lo devuelve nunca.
//
// Sabotaje que la hace fallar: volver a filtrar con `Get-Process musubi`, o sacar `$viejo` de la
// condición de `$todos`.
func TestLaClasificacionVeTambienAlZombiDelBinarioRenombrado(t *testing.T) {
	lib := leerDeploy(t, "lib-agente-windows.sh")
	i := strings.Index(lib, "CLASIFICAR='")
	if i < 0 {
		t.Fatal("el lib ya no define `CLASIFICAR`: es el bloque que separa al agente nuevo del zombi")
	}
	clasif := lib[i:]

	if !strings.Contains(clasif, "$_.Path -eq $viejo") {
		t.Fatal("la clasificación dejó de mirar `musubi.exe.viejo`: el agente anterior sigue vivo con\n" +
			"ese nombre porque el paso [2] del cambiador renombra el binario EN USO, y así no se lo ve")
	}
	for _, linea := range strings.Split(clasif, "\n") {
		if strings.Contains(linea, "$todos") && strings.Contains(linea, "Get-Process musubi") {
			t.Fatalf("la clasificación volvió a filtrar por nombre de proceso:\n  %s\n"+
				"El proceso que corre `musubi.exe.viejo` se llama `musubi.exe`, no `musubi`, así que\n"+
				"`Get-Process musubi` no lo devuelve. Se enumera por RUTA.", strings.TrimSpace(linea))
		}
	}
}
