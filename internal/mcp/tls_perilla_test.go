package mcp

import (
	"strings"
	"testing"
)

// LA PERILLA QUE CIERRA LA MIGRACIÓN A TLS NO VIVÍA EN NINGÚN LADO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// `MUSUBI_EXIGIR_TLS=1` convierte «el bearer viaja en texto plano» de una salvedad de POSTURA en
// una DIVERGENCIA: es el paso que hace que el verificador ponga en rojo un cerebro que quedó
// sirviendo HTTP en claro. O sea que es literalmente el último paso de la migración a TLS — el
// que comprueba que la migración quedó completa.
//
// Y SÓLO EXISTÍA EN EL README Y EN UNA INVOCACIÓN A MANO. La unidad que corre la comparación
// cuatro veces por día no la ponía, así que la exigencia no corría sola NUNCA. El día que alguien
// migrara el transporte, nada iba a comprobar que la migración se completó — y una migración a
// medias (el cerebro en TLS, un agente todavía en claro) se ve exactamente igual que una
// completa.
//
// ARRANCA EN 0 A PROPÓSITO. Hoy el cerebro sirve HTTP en claro —lo cifra el tailnet, no el
// transporte— y ponerla en 1 dejaría la unidad en rojo todos los días por una postura que está
// DECIDIDA. Lo que se arregla acá no es la postura: es que la perilla tenga UN SOLO lugar donde
// vive, para que cambiarla sea una línea y no acordarse de un comando.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestLaPerillaDelTLSViveEnLaUnidadYViajaEnElLatido(t *testing.T) {
	unidad := leerDeploy(t, "systemd", "musubi-comparar.service")

	// SE MIRA LA DIRECTIVA Y NO LA MENCIÓN: el bloque de prosa que explica la perilla la nombra
	// varias veces, y contar apariciones haría que la documentación de la perilla hiciera las
	// veces de la perilla. `leerDeploy` ya blanquea los comentarios; esto lo ancla además a la
	// forma que systemd entiende.
	if !strings.Contains(unidad, "Environment=MUSUBI_EXIGIR_TLS=") {
		t.Error("la unidad no declara `MUSUBI_EXIGIR_TLS`.\n" +
			"  Es el paso que CIERRA la migración a TLS —convierte el transporte en claro de una " +
			"salvedad de postura en una divergencia— y sin esta línea sólo existe en una invocación " +
			"a mano: la exigencia no corre sola nunca.\n" +
			"  El día que alguien migre el transporte, nada va a comprobar que la migración quedó " +
			"completa.")
	}

	// Y EL VALOR VIAJA EN EL LATIDO. Sin eso, «esta corrida exigió TLS» y «esta corrida lo aceptó
	// como está» son indistinguibles desde Prometheus: un verde del verificador no dice cuál de
	// las dos preguntas contestó.
	guion := leerDeploy(t, "comparar-y-latir.sh")
	if !strings.Contains(guion, "musubi_verificacion_tls_exigido") {
		t.Error("el latido no lleva `musubi_verificacion_tls_exigido`: desde Prometheus, una corrida " +
			"que exigió TLS y una que aceptó el transporte como está se ven idénticas — las dos " +
			"salen 0 cuando todo coincide")
	}
	if !strings.Contains(guion, "${MUSUBI_EXIGIR_TLS:-0}") {
		t.Error("el valor que viaja en el latido no sale de `MUSUBI_EXIGIR_TLS`: una copia tipeada " +
			"diría que se exigió TLS en corridas que no lo exigieron")
	}
}

// Y EL VERIFICADOR TIENE QUE SEGUIR TENIENDO LAS DOS RAMAS.
//
// Sin la rama `rojo`, poner la perilla en 1 no cambiaría nada y la migración se podría declarar
// cerrada sin estarlo. Sin la rama `tibio`, el estado de hoy —decidido y aceptado— se reportaría
// como divergencia y la unidad quedaría en rojo todos los días.
func TestElVerificadorDistingueExigirTLSDeAceptarElTransporte(t *testing.T) {
	guion := leerDeploy(t, "verificar-despliegue.sh")

	i := strings.Index(guion, `if [ "${MUSUBI_EXIGIR_TLS:-0}" = "1" ]; then`)
	if i < 0 {
		t.Fatal("el verificador ya no consulta `MUSUBI_EXIGIR_TLS`: la perilla dejó de tener efecto, " +
			"así que ponerla en 1 no cerraría nada")
	}
	bloque := guion[i:]
	if j := strings.Index(bloque, "\n  fi"); j > 0 {
		bloque = bloque[:j]
	}
	if !strings.Contains(bloque, "rojo ") {
		t.Error("con `MUSUBI_EXIGIR_TLS=1` el transporte en claro no se reporta como ROJO: la perilla " +
			"existiría sin cambiar nada, y la migración a TLS se podría declarar cerrada sin estarlo")
	}
	if !strings.Contains(bloque, "tibio ") {
		t.Error("sin la rama `tibio`, el estado de HOY —transporte en claro, decidido y aceptado— se " +
			"reportaría como divergencia: la unidad quedaría en rojo todos los días por una postura " +
			"que nadie va a cambiar hoy, y eso es cómo se apaga un canal")
	}
}
