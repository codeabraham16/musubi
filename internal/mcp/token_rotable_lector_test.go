package mcp

import (
	"strings"
	"testing"
)

// `token_rotable` SE EMITÍA Y NO LA LEÍA NADIE.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// La serie existe desde A88 y ninguna regla la consumía. Y lo único que lo decía estaba adentro de
// una fila del registro marcada como cerrada — o sea que borrar esa fila, que es lo correcto,
// habría destruido el único registro del cabo.
//
// NO ES UNA MOLESTIA: un 0 significa que la rotación de esa máquina NO SE PUEDE COMPLETAR NUNCA.
// La credencial llegó por variable de entorno, un proceso no puede reescribir su propio entorno,
// así que el token nuevo llega, el agente no lo puede guardar, y a las 24 h la rotación se
// abandona sola. En cada intento, para siempre.
//
// Y el costo no termina ahí: con la credencial en el entorno la lee cualquier proceso del mismo
// usuario, y sobrevive a que alguien arregle el archivo. Es el mecanismo exacto de A88 mirado
// desde el otro lado.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestLaSerieDeCredencialRotableTieneQuienLaLea(t *testing.T) {
	reglas := strings.Join(strings.Fields(leerDeploy(t, "musubi-alerts-flota.yml")), " ")

	if !strings.Contains(reglas, "musubi_fleet_device_token_rotable == 0") {
		t.Error("ninguna alerta lee `musubi_fleet_device_token_rotable == 0`.\n" +
			"  Un 0 ahí no es una molestia: es que la rotación de esa máquina NO SE PUEDE COMPLETAR " +
			"NUNCA —un proceso no puede reescribir su propio entorno— así que cada intento se " +
			"abandona solo a las 24 h, para siempre.\n" +
			"  Y mientras tanto la credencial vive en el entorno, donde la lee cualquier proceso del " +
			"mismo usuario.")
	}

	// Y NO PUEDE LEERSE COMO `!= 1` NI COMO `< 1`. La serie está AUSENTE cuando el agente no lo
	// reporta (binario viejo), y en PromQL un `!=` sobre una serie ausente no produce nada — pero
	// escribirlo así invita a leerlo como «todo lo que no sea 1», que incluiría el «no sé». La
	// forma que dice lo que se quiere decir es `== 0`: la máquina AFIRMÓ que no puede rotar.
	if strings.Contains(reglas, "musubi_fleet_device_token_rotable != 1") ||
		strings.Contains(reglas, "musubi_fleet_device_token_rotable < 1") {
		t.Error("la alerta lee `token_rotable` con `!= 1` o `< 1`: esa forma se lee como «todo lo que " +
			"no sea rotable», que incluiría el «no sé» de un agente viejo. La máquina tiene que " +
			"HABER AFIRMADO que no puede rotar: `== 0`.")
	}
}
