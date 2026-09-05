package mcp

// despliegue_cerebro_test.go custodia el script que redespliega el cerebro.
//
// Es el despliegue más peligroso del repo y por un motivo que no es obvio: en el servidor real,
// `musubi-brain.service` y `musubi-agente.service` COMPARTEN EJECUTABLE, y `applyMigrations` es
// fail-closed — un binario viejo se NIEGA a abrir una base que migró uno nuevo. Eso convierte
// cada redespliegue en una puerta de una sola dirección, y hace que volver atrás el binario NO
// alcance: hay que volver atrás también la base.
//
// Las tres guardas de acá son las cosas cuyo incumplimiento no rompe nada visible el día del
// despliegue y dejan al operador sin salida el día que algo sale mal.

import (
	"os"
	"strings"
	"testing"
)

func leerRedespliegue(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../../deploy/redesplegar-cerebro.sh")
	if err != nil {
		t.Fatalf("falta el script de redespliegue del cerebro: %v", err)
	}
	return string(b)
}

// TestLaVueltaAtrasRestauraLaBaseYNoSoloElBinario — la guarda que justifica el archivo.
//
// Restaurar sólo el binario deja al cerebro viejo mirando una base con esquema nuevo, y
// `applyMigrations` se niega a abrirla: el «rollback» termina con el servicio caído y sin base a
// la que volver. Es el peor resultado posible, y se ve exactamente igual que un rollback bien
// hecho hasta que el servicio no levanta.
//
// Sabotaje que la hace fallar: sacar la línea que copia el respaldo sobre la base en volver_atras.
func TestLaVueltaAtrasRestauraLaBaseYNoSoloElBinario(t *testing.T) {
	texto := leerRedespliegue(t)
	i := strings.Index(texto, "volver_atras(){")
	if i < 0 {
		t.Fatal("el script no tiene función de vuelta atrás: un despliegue de una sola dirección sin rollback no se puede correr")
	}
	cuerpo := texto[i:]
	if j := strings.Index(cuerpo, "\n}\n"); j > 0 {
		cuerpo = cuerpo[:j]
	}
	for _, quiero := range []struct{ frag, porque string }{
		{`"$BIN_VIEJO" "$DESTINO"`, "no restaura el binario viejo"},
		{`"$RESPALDO" "$BASE"`, "no restaura la BASE, y el esquema no vuelve solo: el cerebro viejo se negaría a abrirla"},
		{`rm -f "$BASE-wal"`, "no borra el WAL de la base migrada, que sobreviviría a la restauración y la contaminaría"},
	} {
		if !strings.Contains(cuerpo, quiero.frag) {
			t.Errorf("la vuelta atrás %s", quiero.porque)
		}
	}
}

// TestElRespaldoSeSacaANTESDeTocarNada.
//
// Un respaldo posterior al cambio no es un respaldo. Y tiene que usar la API de SQLite, no `cp`:
// el cerebro está escribiendo, y un `cp` en caliente deja una base a medio escribir que parece
// buena hasta el día que se la necesita.
//
// Sabotaje que la hace fallar: cambiar el respaldo por `cp -a "$BASE" "$RESPALDO"`.
func TestElRespaldoSeSacaANTESDeTocarNada(t *testing.T) {
	texto := leerRedespliegue(t)
	posRespaldo := strings.Index(texto, "s.backup(d)")
	posCambio := strings.Index(texto, `install -m 0755 -o root`)
	if posRespaldo < 0 {
		t.Fatal("el respaldo no usa la API de backup de SQLite: un `cp` en caliente puede dejar una base a medio escribir")
	}
	if posCambio < 0 {
		t.Fatal("no se encontró el reemplazo del binario; el patrón cambió")
	}
	if posRespaldo > posCambio {
		t.Error("el respaldo se saca DESPUÉS de reemplazar el binario: eso no es un punto de retorno")
	}
	if !strings.Contains(texto, "SIN RESPALDO NO SE SIGUE") {
		t.Error("el script no aborta si el respaldo falla: seguiría hacia una puerta de una sola dirección sin llave")
	}
}

// TestSeVerificaElINODOYNoElIsActive.
//
// Ya pasó en este servidor: `systemctl is-active` decía `active` mientras el proceso corría un
// binario BORRADO (`/proc/PID/exe -> ...(deleted)`), o sea que el reemplazo no había tomado
// efecto y todo se veía bien.
//
// Sabotaje que la hace fallar: reemplazar la comparación de inodos por un `systemctl is-active`.
func TestSeVerificaElINODOYNoElIsActive(t *testing.T) {
	texto := leerRedespliegue(t)
	if !strings.Contains(texto, `/proc/$PID/exe`) {
		t.Fatal("el script no mira /proc/$PID/exe: `is-active` no distingue un proceso que corre un binario BORRADO")
	}
	// Y no alcanza con que los inodos se lean: la comparación tiene que ser una COMPUERTA que
	// dispara la vuelta atrás. La primera versión de esta prueba sólo buscaba los nombres de las
	// variables, y pasaba en verde con el `is-active` de vuelta — porque los nombres seguían
	// apareciendo en la línea de comparación. Una guarda que busca cadenas y no efecto no protege.
	comparacion := false
	for _, l := range strings.Split(texto, "\n") {
		if strings.Contains(l, "INODO_PROC") && strings.Contains(l, "INODO_DISCO") && strings.Contains(l, "volver_atras") {
			comparacion = true
		}
	}
	if !comparacion {
		t.Error("los inodos se leen pero no gatean nada: hace falta una línea que los compare Y llame a volver_atras cuando difieren")
	}
	// Y el sha256 del binario es obligatorio: una descarga truncada devuelve éxito igual.
	if !strings.Contains(texto, "SHA_ESPERADO") || !strings.Contains(texto, "sha256sum") {
		t.Error("el script no exige el sha256 del binario: ya se desplegó una descarga truncada que reportó éxito")
	}
}
