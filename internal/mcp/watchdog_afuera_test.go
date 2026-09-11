package mcp

import (
	"os/exec"
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
	var util []string
	for _, l := range strings.Split(guion[i:j], "\n") {
		// Las dos que hablan con el servidor por ssh: la prueba pone esos valores a mano.
		if strings.HasPrefix(l, "WD_HOST=") || strings.HasPrefix(l, "WD_YO=") {
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
		{"el destino es otra máquina", "healthchecks.io", "musubi-server 100.64.1.1", "OK",
			"sin el camino verde la comprobación no puede confirmar nada y sólo sabe acusar"},
		{"un vecino que EMPIEZA IGUAL no cuenta", "musubi-server2", "musubi-server 100.64.1.1", "OK",
			"la comparación es palabra por palabra a propósito: si fuera por prefijo o por `case` con `*`, un host ajeno con nombre parecido se reportaría como propio y el dead-man bueno se declararía malo"},
		{"no se supo cómo se llama esta máquina", "healthchecks.io", "", "DUDOSO",
			"«no pude comparar» no es «está afuera». Con `WD_YO` vacío esto contestaba `ok`: el ssh caído convertía la comprobación en un sello de aprobación"},
		{"no se pudo leer el destino", "", "musubi-server 100.64.1.1", "DUDOSO",
			"el archivo puede no existir, y entonces el dead-man ni siquiera está armado: eso no es «está bien»"},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			// Los tres veredictos se reemplazan por stubs que imprimen su nombre: así se mide CUÁL
			// se tomó, y no si la palabra aparece en algún lado del archivo.
			guion := "rojo(){ echo ROJO; }\nok(){ echo OK; }\ndudoso(){ echo DUDOSO; }\ntitulo(){ :; }\n" +
				"WD_HOST=" + shQuote(c.host) + "\nWD_YO=" + shQuote(c.yo) + "\n" + bloque
			salida, err := exec.Command("bash", "-c", guion).CombinedOutput()
			if err != nil {
				t.Fatalf("el bloque del watchdog no corrió: %v\n%s", err, salida)
			}
			dicho := strings.TrimSpace(string(salida))
			if dicho != c.espera {
				t.Errorf("con WD_HOST=%q y WD_YO=%q el bloque contestó %q y tiene que contestar %q.\n  %s",
					c.host, c.yo, dicho, c.espera, c.porque)
			}
		})
	}
}
