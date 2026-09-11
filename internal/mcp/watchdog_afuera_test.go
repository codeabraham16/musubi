package mcp

import (
	"strings"
	"testing"
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

	// LAS TRES RESPUESTAS. «afuera», «adentro» y «no pude preguntar» se arreglan distinto, y la
	// última es la que un verificador tiende a confundir con un verde.
	for _, veredicto := range []struct{ fn, porque string }{
		{"rojo ", "un watchdog adentro del cajón es una divergencia real contra lo que el despliegue declara querer, no una salvedad"},
		{"ok ", "sin el camino verde, la comprobación no puede confirmar nada y sólo sabe acusar"},
		{"dudoso ", "si el archivo no se puede leer, eso NO es «está bien»: puede no existir —y entonces el dead-man ni siquiera está armado—"},
	} {
		if !strings.Contains(bloque, veredicto.fn) {
			t.Errorf("la comprobación del watchdog no tiene el camino `%s`: %s", strings.TrimSpace(veredicto.fn), veredicto.porque)
		}
	}

	// Y LA URL NO SE IMPRIME NUNCA. El uuid ES la credencial del ping: publicarlo en el informe
	// —que se pega en un chat, se guarda en un journal y se lee en una pantalla compartida— es
	// entregarle a cualquiera la llave para fingir el latido y apagar el dead-man.
	if strings.Contains(bloque, "$WD_URL") || strings.Contains(bloque, "cat /etc/musubi/watchdog_url") {
		t.Error("la comprobación maneja la URL entera del watchdog: ese uuid es la credencial del " +
			"ping, y el informe se pega en chats y queda en journals. Sólo tiene que cruzar el HOST.")
	}
}
