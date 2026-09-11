package mcp

import (
	"os/exec"
	"strings"
	"testing"

	"musubi/internal/guiones"
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
	// LAS DOS RAMAS NO SE CUENTAN ACÁ, SE EJERCITAN. Lo que había era pedir los literales «rojo »
	// y «tibio » adentro del bloque, y eso deja sin custodiar lo único que importa: CUÁL va con
	// CUÁL. Sabotaje medido: intercambiando los dos veredictos —de modo que con la perilla en 1 el
	// transporte en claro pase a `tibio` y sin exigirla pase a `rojo`, o sea exactamente al revés
	// de lo que la perilla significa— esta guarda quedaba VERDE. Quien lo comprueba es
	// TestLaPerillaDeTLSDecideCorriendo.
	_ = bloque
}

// TestLaPerillaDeTLSDecideCorriendo — las cinco posturas, corriendo el bloque de verdad.
//
// POR QUÉ NO ALCANZA CON PREGUNTAR SI LOS VEREDICTOS ESTÁN. Una guarda que pide que «rojo» y
// «tibio» aparezcan en el bloque se satisface con los dos veredictos intercambiados, que es el
// único error que alguien va a cometer acá: nadie borra una rama, la gente invierte una condición.
// Y el costo de esa inversión es el peor de los dos lados a la vez — la migración a TLS se podría
// declarar cerrada sin estarlo, y mientras tanto la unidad quedaría en rojo todos los días por una
// postura que está decidida, que es cómo se apaga un canal.
func TestLaPerillaDeTLSDecideCorriendo(t *testing.T) {
	guiones.Exigir(t, "corre el bloque de postura TLS de deploy/verificar-despliegue.sh contra configuraciones de prueba, y ese guion es de un servidor Linux", "bash", "grep")

	guion := leerDeploy(t, "verificar-despliegue.sh")
	i := strings.Index(guion, `if [ -z "$POSTURA_TLS" ]; then`)
	if i < 0 {
		t.Fatal("no se encontró el arranque del bloque de postura TLS: o se renombró la variable o " +
			"el bloque se movió, y esta guarda dejó de mirar lo que dice mirar")
	}
	resto := guion[i:]
	j := strings.Index(resto, "\nfi\n")
	if j < 0 {
		t.Fatal("no se encontró el `fi` que cierra el bloque de postura TLS")
	}
	bloque := resto[:j+4]

	for _, c := range []struct {
		nombre, postura, perilla, espera, porque string
	}{
		{"no se pudo leer la config", "", "0", "DUDOSO",
			"no saber qué sirve el cerebro no es «sirve TLS» ni «no sirve»: es no saber, y se arregla mirando"},
		{"hay certificado configurado", "tls_cert_file:/etc/musubi/cert.pem", "0", "VERDE",
			"con certificado el bearer viaja cifrado de extremo a extremo, y eso es el objetivo de la migración"},
		{"claro, con la perilla APAGADA", "allow_insecure_token:true", "0", "TIBIO",
			"el estado de HOY está decidido y aceptado: reportarlo como divergencia dejaría la unidad en rojo todos los días por algo que nadie va a cambiar hoy"},
		{"claro, con la perilla PRENDIDA", "allow_insecure_token:true", "1", "ROJO",
			"ésta es la razón de ser de la perilla: prenderla convierte «el bearer viaja en claro» de salvedad de postura en DIVERGENCIA. Si acá no sale rojo, la perilla no cambia nada y la migración se puede declarar cerrada sin estarlo"},
		{"claro desactivado sin certificado", "allow_insecure_token:false", "1", "VERDE",
			"la rama de cierre tiene que seguir existiendo: sin ella el bloque sólo sabría acusar"},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			completo := "rojo(){ echo ROJO; }\nverde(){ echo VERDE; }\ntibio(){ echo TIBIO; }\ndudoso(){ echo DUDOSO; }\n" +
				"POSTURA_TLS=" + shQuote(c.postura) + "\nCFG_REMOTO=/etc/musubi/config.yaml\n" +
				"MUSUBI_EXIGIR_TLS=" + shQuote(c.perilla) + "\n" + bloque
			salida, err := exec.Command("bash", "-c", completo).CombinedOutput()
			if err != nil {
				t.Fatalf("el bloque de postura TLS no corrió: %v\n%s", err, salida)
			}
			dicho := strings.TrimSpace(string(salida))
			if dicho != c.espera {
				t.Errorf("con POSTURA_TLS=%q y MUSUBI_EXIGIR_TLS=%q el bloque contestó %q y tiene que contestar %q.\n  %s",
					c.postura, c.perilla, dicho, c.espera, c.porque)
			}
		})
	}
}
