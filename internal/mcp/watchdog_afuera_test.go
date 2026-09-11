package mcp

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// UN DEAD-MAN QUE CORRE EN LA MÁQUINA QUE VIGILA NO ES UN DEAD-MAN.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// `MusubiSiempreViva` late cada 5 minutos hacia un receptor externo, y su valor entero está en el
// SILENCIO: si deja de llegar, lo que se rompió es el sistema de alertas y todo lo demás está
// ciego. Eso sólo funciona si quien ESCUCHA está afuera de la máquina que puede morirse.
//
// Con el receptor adentro del mismo host, un corte de luz se lleva puestos al vigilado y al
// vigilante juntos: nadie late y nadie nota que nadie late. El propio `prometheus.yml` de este
// repo nombra ese patrón DOS VECES —«el watchdog vive adentro de la misma máquina que se
// apagó»— y hasta hoy no lo comprobaba nadie: era una advertencia escrita sin nada que la
// midiera.
//
// SE MIRA EL HOST Y NUNCA LA URL. `/etc/musubi/watchdog_url` contiene un uuid que ES la
// credencial del ping: quien lo tenga puede fingir el latido y apagar el dead-man.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestElVerificadorCompruebaQueElWatchdogEsteAfuera(t *testing.T) {
	guion := leerDeploy(t, "verificar-despliegue.sh")

	i := strings.Index(guion, `titulo "el watchdog externo, ¿está afuera?"`)
	if i < 0 {
		t.Fatal("el verificador no comprueba dónde vive el watchdog: un dead-man adentro de la " +
			"máquina que vigila se apaga junto con ella, y nadie nota que nadie late. El propio " +
			"prometheus.yml nombra ese patrón dos veces y no lo medía nada")
	}
	bloque := guion[i:]
	if j := strings.Index(bloque, `titulo "guiones derivados`); j > 0 {
		bloque = bloque[:j]
	}

	// LAS TRES RESPUESTAS NO SE CUENTAN ACÁ, SE EJERCITAN. Lo que había era un bucle que pedía
	// los literales «rojo », «ok » y «dudoso » adentro del bloque, y eso no custodiaba nada:
	// borrando ENTERO el `for palabra in $WD_YO` que decide, los tres veredictos seguían en su
	// lugar y esta guarda seguía verde —medido—. Quien lo comprueba de verdad es
	// TestElBloqueDelWatchdogDecideCorriendo, que corre el bloque con destinos de prueba.

	// Y LA URL NO SE IMPRIME NUNCA. El uuid ES la credencial del ping: publicarlo en el informe
	// —que se pega en un chat, se guarda en un journal y se lee en una pantalla compartida— es
	// entregarle a cualquiera la llave para fingir el latido y apagar el dead-man.
	if strings.Contains(bloque, "$WD_URL") || strings.Contains(bloque, "cat /etc/musubi/watchdog_url") {
		t.Error("la comprobación maneja la URL entera del watchdog: ese uuid es la credencial del " +
			"ping, y el informe se pega en chats y queda en journals. Sólo tiene que cruzar el HOST.")
	}
}

// bloqueDelWatchdog devuelve el bloque del watchdog LISTO PARA CORRER: sin comentarios (viene de
// `leerDeploy`) y sin las dos líneas que le preguntan al servidor por ssh, que la prueba reemplaza
// por valores propios.
//
// Se corta por los dos `titulo` que lo delimitan, que son CÓDIGO y no prosa: un comentario no
// puede moverlos ni fabricarlos.
func bloqueDelWatchdog(t *testing.T) string {
	t.Helper()
	guion := leerDeploy(t, "verificar-despliegue.sh")
	i := strings.Index(guion, `titulo "el watchdog externo, ¿está afuera?"`)
	j := strings.Index(guion, `titulo "guiones derivados`)
	if i < 0 || j <= i {
		t.Fatalf("no se pudo aislar el bloque del watchdog (i=%d, j=%d): o se renombró el título o "+
			"el bloque se movió, y esta guarda dejó de mirar lo que dice mirar", i, j)
	}
	// LAS LÍNEAS QUE HABLAN CON EL SERVIDOR SE SACAN POR LO QUE HACEN —llamar a `corre_alla`— y
	// NO POR EL NOMBRE DE LA VARIABLE QUE ASIGNAN. Antes decía `WD_HOST=` y `WD_YO=` a mano, y el
	// día que se agregó una tercera pregunta (`WD_DIR=`, la que averigua DÓNDE vive el archivo) el
	// extractor la dejó pasar: el bloque corría con `corre_alla` sin definir y la guarda medía un
	// error de bash en vez de la decisión. Derivado, una pregunta nueva se saca sola.
	//
	// De ahí que cada pregunta al servidor tenga que ser UNA asignación completa en una línea: un
	// `if` de tres líneas dejaría acá un `then` con el cuerpo borrado.
	var util []string
	for _, l := range strings.Split(guion[i:j], "\n") {
		if strings.Contains(l, "corre_alla") {
			continue
		}
		util = append(util, l)
	}
	return strings.Join(util, "\n")
}

// TestElBloqueDelWatchdogDecideCorriendo — la guarda que reemplaza a la de texto, y el caso que
// destapó al escribirla.
//
// POR QUÉ CORRER Y NO GREPEAR. La versión anterior pedía que los literales «rojo », «ok » y
// «dudoso » estuvieran en el bloque. Sabotaje medido: borrando ENTERO el
// `for palabra in $WD_YO; do [ "$palabra" = "$WD_HOST" ] && WD_ADENTRO=1; done` —o sea la única
// línea que COMPARA— los tres literales seguían ahí y el paquete entero quedaba en verde. Con ese
// sabotaje puesto, un watchdog apuntado al hostname propio del cerebro (no a `localhost`, que el
// `case` sí atrapa) se reporta `ok`. Es el dead-man adentro del cajón con una guarda que lo tapa.
//
// EL CASO QUE APARECIÓ AL EJERCITARLO, y que ninguna lectura había visto: con `WD_YO` VACÍO
// —el ssh no anduvo, o `hostname` no está— el `for` daba cero vueltas, `WD_ADENTRO` se quedaba en
// 0 y el bloque contestaba `ok`, «que no es esta máquina», sin haber comparado con nada. Medido
// sobre el commit anterior: destino ajeno + `WD_YO` vacío -> OK. O sea que el modo de falla que
// este bloque existe para cazar pasaba por bueno cada vez que fallaba el ssh.
func TestElBloqueDelWatchdogDecideCorriendo(t *testing.T) {
	guiones.Exigir(t, "corre el bloque del watchdog de deploy/verificar-despliegue.sh contra destinos de prueba, y ese guion es de un servidor Linux", "bash")

	bloque := bloqueDelWatchdog(t)

	for _, c := range []struct {
		nombre, host, yo, espera, porque string
	}{
		{"el destino es el hostname del propio cerebro", "musubi-server", "musubi-server musubi-server.tail 100.64.1.1", "ROJO",
			"es el caso que de verdad pasa: nadie escribe `localhost` en el watchdog_url, escribe el nombre de la máquina. Si esto no sale rojo, el dead-man vive adentro del cajón y el informe dice que está todo bien"},
		{"el destino es una IP de esta misma máquina", "100.64.1.1", "musubi-server 100.64.1.1", "ROJO",
			"la comparación tiene que ser contra TODO lo que la máquina dice de sí misma, no sólo contra su nombre corto"},
		{"el destino es localhost", "localhost", "musubi-server 100.64.1.1", "ROJO",
			"el `case` de los nombres de bucle tiene que seguir decidiendo sin preguntarle el nombre a nadie"},
		{"el destino es otra máquina", "healthchecks.io", "musubi-server 100.64.1.1", "VERDE",
			"sin el camino verde la comprobación no puede confirmar nada y sólo sabe acusar"},
		{"un vecino que EMPIEZA IGUAL no cuenta", "musubi-server2", "musubi-server 100.64.1.1", "VERDE",
			"la comparación es palabra por palabra a propósito: si fuera por prefijo o por `case` con `*`, un host ajeno con nombre parecido se reportaría como propio y el dead-man bueno se declararía malo"},
		{"no se supo cómo se llama esta máquina", "healthchecks.io", "", "DUDOSO",
			"«no pude comparar» no es «está afuera». Con `WD_YO` vacío esto contestaba `ok`: el ssh caído convertía la comprobación en un sello de aprobación"},
		{"no se pudo leer el destino", "", "musubi-server 100.64.1.1", "DUDOSO",
			"el archivo puede no existir, y entonces el dead-man ni siquiera está armado: eso no es «está bien»"},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			// LOS STUBS SE DERIVAN DEL GUION, NO SE ESCRIBEN A MANO, Y ESO ES LA MITAD DE ESTA
			// GUARDA. Acá decía `ok(){ echo OK; }` — y `ok` NO EXISTE en verificar-despliegue.sh.
			// El arnés FABRICABA la función que al guion le faltaba, así que el único camino que
			// contesta «está afuera» pasaba verde acá y moría con `ok: command not found` en
			// producción. Nadie lo vio durante semanas porque ese camino además era inalcanzable
			// por otro motivo (la ruta del watchdog estaba escrita a mano y nunca daba un host).
			//
			// Derivados, un stub no puede existir para una función que el guion no define: si el
			// bloque llama a algo inventado, bash lo dice y el `not found` de abajo lo caza.
			guion := stubsDeLosVeredictos(t) +
				"WD_HOST=" + shQuote(c.host) + "\nWD_YO=" + shQuote(c.yo) + "\n" + bloque
			salida, err := exec.Command("bash", "-c", guion).CombinedOutput()
			if err != nil {
				t.Fatalf("el bloque del watchdog no corrió: %v\n%s", err, salida)
			}
			if strings.Contains(string(salida), "command not found") {
				t.Fatalf("el bloque llamó a algo que el guion NO DEFINE — eso en el servidor es una "+
					"línea que no hace nada, y en el camino verde significa que la comprobación no "+
					"puede contestar «está bien»:\n%s", salida)
			}
			// El VEREDICTO es la última línea: `titulo` también imprime, porque se stubbea junto
			// con el resto en vez de silenciarlo a mano.
			lineas := strings.Fields(strings.TrimSpace(string(salida)))
			dicho := ""
			if len(lineas) > 0 {
				dicho = lineas[len(lineas)-1]
			}
			if dicho != c.espera {
				t.Errorf("con WD_HOST=%q y WD_YO=%q el bloque contestó %q y tiene que contestar %q.\n  %s\n  salida completa: %s",
					c.host, c.yo, dicho, c.espera, c.porque, salida)
			}
		})
	}
}

// stubsDeLosVeredictos arma un stub por CADA función que `verificar-despliegue.sh` define, cada uno
// imprimiendo su nombre en mayúsculas. Así la prueba mide CUÁL veredicto tomó el bloque, y —lo que
// importa más— NO puede darle al bloque una función que el guion no tenga: si el bloque llama a
// algo inventado, no hay stub que lo tape y bash lo delata.
//
// Es la diferencia entre un doble y un invento. Un doble reemplaza algo que existe; un invento
// completa lo que falta, y ahí el arnés deja de medir el guion y empieza a medirse a sí mismo.
func stubsDeLosVeredictos(t *testing.T) string {
	t.Helper()
	guion := leerDeploy(t, "verificar-despliegue.sh")
	def := regexp.MustCompile(`(?m)^([a-z_]+)\(\)`)
	nombres := def.FindAllStringSubmatch(guion, -1)
	if len(nombres) < 4 {
		t.Fatalf("se derivaron %d funciones de verificar-despliegue.sh y el bloque del watchdog usa "+
			"al menos cuatro (titulo, rojo, verde, dudoso): o cambió la forma de declararlas, o esta "+
			"guarda dejó de encontrarlas y estaría por inventarlas", len(nombres))
	}
	var b strings.Builder
	for _, m := range nombres {
		fmt.Fprintf(&b, "%s(){ echo %s; }\n", m[1], strings.ToUpper(m[1]))
	}
	return b.String()
}
