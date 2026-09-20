package mcp

import (
	"regexp"
	"strings"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────────────────────
// A129 · LA DIRECCIÓN DEL CEREBRO NO SE ESCRIBE A MANO, Y EL NOMBRE TLS NO TIENE HERMANOS SOLOS
// ─────────────────────────────────────────────────────────────────────────────────────────────

// laURLEscritaAMano reconoce una dirección de cerebro CLAVADA en un guion: `set MUSUBI_BRAIN_URL=`
// seguido de algo que tiene esquema. Lo que se permite es el valor DERIVADO —una variable— porque
// entonces hay una sola dirección en juego y no dos que se contradicen.
var laURLEscritaAMano = regexp.MustCompile(`(?i)set\s+MUSUBI_BRAIN_URL\s*=\s*[a-z]+://`)

// TestLaDireccionDelCerebroNoSeEscribeAManoEnLosGuionesDeWindows — el defecto que medí el
// 2026-09-20 y que A129 no listaba.
//
// `cambiar-agente.cmd` —el guion que actualiza el agente de una Windows— llevaba la dirección del
// cerebro clavada con su IP y su puerto, y con ella probaba el binario nuevo antes de aceptarlo.
// Era una COPIA de lo que vive en `agente.cmd`, que es el archivo que la Tarea Programada usa de
// verdad. El día que el cerebro pase a `https://…:10000` (que es el punto entero de A129), esa
// prueba iba a seguir discando el puerto viejo, fallar, y disparar el ROLLBACK acusando al binario
// nuevo de no latir. El binario estaría sano y nadie miraría ahí.
//
// Es la forma que este repo ya tiene con nombre —un derivado escrito a mano es una copia, y dos
// copias se contradicen el día que alguien cambia una—, aplicada a una dirección de red.
//
// LA PREGUNTA CONVERGE porque no hay que decidir QUÉ dirección es la buena: el valor o tiene
// esquema (`http://`, `https://`) y entonces es un literal escrito a mano, o sale de una variable
// y entonces se deriva. No hay tercera forma que valga la pena distinguir.
//
// Sabotaje verificado que la pone roja: devolverle a `cambiar-agente.cmd` su dirección clavada.
// arnes: archivo="deploy/cambiar-agente.cmd"
// arnes: de="set MUSUBI_BRAIN_URL=\nfor /f"
// arnes: a="set MUSUBI_BRAIN_URL=http://100.79.126.62:7717\nfor /f"
func TestLaDireccionDelCerebroNoSeEscribeAManoEnLosGuionesDeWindows(t *testing.T) {
	// El instalador ENTREGA la dirección (la recibe por parámetro y la escribe en el lanzador);
	// el cambiador la CONSUME y por eso tiene que derivarla. Los dos se miran igual: lo que se
	// prohíbe es el literal, venga de donde venga.
	casos := []struct {
		archivo string
		texto   string
	}{
		{"cambiar-agente.cmd", codigoDeCmd(leerDeploy(t, "cambiar-agente.cmd"))},
		{"agente-windows.ps1", codigoDe(leerDeploy(t, "agente-windows.ps1"))},
	}

	mirados := 0
	for _, c := range casos {
		if len(strings.TrimSpace(c.texto)) < 500 {
			t.Fatalf("%s vino con %d bytes de código: o no se leyó, o el filtro de comentarios se "+
				"comió el archivo. Un cero de hallazgos sobre un archivo vacío no es «está limpio»",
				c.archivo, len(c.texto))
		}
		mirados++
		for i, linea := range strings.Split(c.texto, "\n") {
			if !laURLEscritaAMano.MatchString(linea) {
				continue
			}
			t.Errorf("%s línea %d: la dirección del cerebro está ESCRITA A MANO.\n"+
				"    %s\n"+
				"  Es una copia de la que vive en `agente.cmd`, que es la que la Tarea Programada usa\n"+
				"  de verdad, y dos copias de una dirección se contradicen el día que una cambia. El día\n"+
				"  que el cerebro pase a https (A129), este guion va a seguir discando el puerto viejo,\n"+
				"  fallar, y —si es el cambiador— disparar el rollback acusando al binario nuevo. El\n"+
				"  binario va a estar sano.\n"+
				"  Arreglo en un `.cmd`: derivar el entorno del lanzador, que está al lado —\n"+
				"    for /f \"usebackq delims=\" %%L in (`findstr /b /c:\"set MUSUBI_\" \"%%DIR%%agente.cmd\"`) do %%L\n"+
				"  y fallar en voz alta si no quedó definida, porque «no pude leer el lanzador» y «el\n"+
				"  agente no late» son cosas distintas.\n"+
				"  Arreglo en el instalador: la dirección entra por parámetro, no se clava.",
				c.archivo, i+1, strings.TrimSpace(linea))
		}
	}
	if mirados != len(casos) {
		t.Fatalf("se miraron %d de %d guiones", mirados, len(casos))
	}
}

// TestElInstaladorDeWindowsNoDejaSoloAlNombreTLS — el hermano, medido donde SE APLICA.
//
// EL DEFECTO: `MUSUBI_BRAIN_TLS_NAME` existe porque el certificado del tailnet no tiene SAN de IP
// y estas máquinas discan la IP (con NordVPN el MagicDNS no resuelve). El instalador fija la
// dirección del cerebro en DOS lugares —el entorno con el que prueba el latido antes de crear la
// tarea, y el `agente.cmd` que la tarea va a usar— y hasta el 2026-09-20 no llevaba el nombre TLS
// a NINGUNO. Las dos mitades fallan distinto y las dos son caras:
//
//   - sin el nombre en el LANZADOR, una reinstalación revierte la migración a HTTPS en silencio:
//     el agente arranca sin declarar el ServerName, deja de latir, y desde el cerebro se ve
//     idéntico a una máquina apagada;
//   - sin el nombre en la PRUEBA DE LATIDO, el instalador aborta ANTES de crear la tarea, con un
//     mensaje que habla del certificado y no del instalador.
//
// LA PRIMERA VERSIÓN DE ESTA GUARDA SE COMIÓ SU PROPIO SABOTAJE, y por eso está escrita así.
// Contaba las líneas que MENCIONAN `MUSUBI_BRAIN_TLS_NAME` y exigía tantas como las que fijan la
// URL. Pero en PowerShell el nombre se arma en una variable (`$lineaTls = "…set
// MUSUBI_BRAIN_TLS_NAME=…"`) y después se INTERPOLA en el heredoc del lanzador. Sacar la
// interpolación —que es el defecto exacto, el lanzador se queda sin el nombre— no toca la línea
// que lo define, así que el conteo no se movía y la guarda quedaba VERDE. Es la forma que este
// repo ya tiene con nombre: preguntar por un texto que está donde no decide.
//
// AHORA SE PREGUNTA DONDE DECIDE: cada línea que compone la dirección del cerebro tiene que llevar
// el nombre TLS con ella, sea nombrándolo directo o a través de una variable que lo lleve. Un solo
// salto de resolución, que es lo que este guion usa; si alguien encadena dos, la guarda lo va a
// acusar en vez de dejarlo pasar, que es la dirección correcta para equivocarse.
//
// Sabotaje verificado que la pone roja: sacar `$lineaTls` del heredoc del lanzador, que deja el
// nombre en la prueba de latido y lo pierde en la tarea — la mitad que se paga en la próxima
// reinstalación, en silencio.
// arnes: archivo="deploy/agente-windows.ps1"
// arnes: de="set MUSUBI_BRAIN_URL=$BrainUrl$lineaTls$lineaAlcance"
// arnes: a="set MUSUBI_BRAIN_URL=$BrainUrl$lineaAlcance"
func TestElInstaladorDeWindowsNoDejaSoloAlNombreTLS(t *testing.T) {
	g := codigoDe(leerDeploy(t, "agente-windows.ps1"))
	if len(strings.TrimSpace(g)) < 500 {
		t.Fatalf("agente-windows.ps1 vino con %d bytes de código: no se leyó, y todo lo que sigue "+
			"daría verde sin haber mirado nada", len(g))
	}
	lineas := strings.Split(g, "\n")

	fijaURL := regexp.MustCompile(`(?i)(set|\$env:)\s*MUSUBI_BRAIN_URL\s*=`)
	const laVariableDelNombre = "MUSUBI_BRAIN_TLS_NAME"

	// LAS VARIABLES QUE LLEVAN EL NOMBRE. Una asignación `$x = "...MUSUBI_BRAIN_TLS_NAME=..."`
	// convierte a `$x` en portadora: interpolarla en otro lado ES llevar el nombre.
	portadoras := map[string]bool{}
	asigna := regexp.MustCompile(`\$([A-Za-z_][A-Za-z0-9_]*)\s*=`)
	for _, l := range lineas {
		if !strings.Contains(l, laVariableDelNombre) {
			continue
		}
		for _, m := range asigna.FindAllStringSubmatch(l, -1) {
			portadoras[m[1]] = true
		}
	}

	lleva := func(desde, hasta int) bool {
		for i := desde; i <= hasta && i < len(lineas); i++ {
			if i < 0 {
				continue
			}
			l := lineas[i]
			if strings.Contains(l, laVariableDelNombre) {
				return true
			}
			for v := range portadoras {
				if strings.Contains(l, "$"+v) {
					return true
				}
			}
		}
		return false
	}

	var sitios, huerfanos []string
	for i, l := range lineas {
		if !fijaURL.MatchString(l) {
			continue
		}
		sitios = append(sitios, itoaL(i+1)+": "+strings.TrimSpace(l))
		// La misma línea (el heredoc interpola) o la de al lado (el entorno se fija en dos líneas).
		if !lleva(i, i+2) {
			huerfanos = append(huerfanos, itoaL(i+1)+": "+strings.TrimSpace(l))
		}
	}

	// EL CONTROL VA PRIMERO: si el reconocedor se rompiera, no habría sitios, no habría huérfanos,
	// y esto pasaría en verde sin haber comprobado nada.
	if len(sitios) < 2 {
		t.Fatalf("se encontraron %d sitios que fijan MUSUBI_BRAIN_URL y tiene que haber al menos DOS "+
			"(el entorno de la prueba de latido y el `agente.cmd` que escribe el instalador). El "+
			"reconocedor dejó de ver el archivo: un cero acá no es «no hay», es «no medí».",
			len(sitios))
	}
	// Y EL SEGUNDO CONTROL, que es el que la primera versión no tenía: que la resolución de
	// variables de verdad esté resolviendo algo. Sin portadoras, `lleva` se degrada a buscar el
	// texto suelto y la guarda vuelve a ser la que se comió su sabotaje.
	if len(portadoras) == 0 {
		t.Fatal("no se reconoció NINGUNA variable que lleve el nombre TLS, y este guion arma una " +
			"(`$lineaTls`). O el guion cambió de forma, o la resolución se rompió — y sin ella esta " +
			"guarda mira el texto donde se escribe en vez de donde se aplica, que es exactamente el " +
			"defecto por el que quedó verde la primera vez.")
	}
	t.Logf("%d sitio(s) que fijan la dirección del cerebro, %d variable(s) portadoras del nombre TLS",
		len(sitios), len(portadoras))

	for _, x := range huerfanos {
		t.Errorf("LA DIRECCIÓN DEL CEREBRO SIN EL NOMBRE TLS AL LADO, en agente-windows.ps1 %s\n"+
			"  Ese sitio compone la URL del cerebro y no lleva `%s` ni una variable que lo lleve.\n"+
			"  Si es el LANZADOR: una reinstalación revierte la migración a HTTPS en silencio — el\n"+
			"  agente arranca sin declarar el ServerName, deja de latir, y desde el cerebro se ve\n"+
			"  idéntico a una máquina apagada.\n"+
			"  Si es la PRUEBA DE LATIDO: el instalador aborta antes de crear la tarea, con un mensaje\n"+
			"  que habla del certificado y no del instalador.\n"+
			"  Arreglo: el parámetro `-TlsName` viaja a los dos — `$env:MUSUBI_BRAIN_TLS_NAME` antes\n"+
			"  del `agent --once`, y `$lineaTls` interpolada adentro del heredoc del lanzador.",
			x, laVariableDelNombre)
	}
}

func itoaL(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
