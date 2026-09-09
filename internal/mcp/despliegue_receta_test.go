package mcp

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// LA RECETA DE VUELTA ATRÁS QUE IMPRIME EL REDESPLIEGUE, EJERCITADA.
//
// El guión imprimía UNA sola receta a mano, y llevaba `cp -a $RESPALDO $BASE` siempre. La mayoría
// de los redespliegues NO migran —el del 2026-09-05 fue esquema 46 → 46— y ahí restaurar el
// snapshot es innecesario Y destructivo: descarta todo lo que el cerebro escribió desde el
// despliegue. Medido a los seis minutos de esa corrida, comparando el snapshot con la base viva:
// +57 tool_invocations, +5 device_commands, +1 screen_session. Una vuelta atrás a mano se hace
// HORAS después, que es cuando el número deja de ser 57.
//
// El guión ya tenía los dos datos que hacían falta ($VERSION_BASE y $ESQUEMA) y no los usaba.
//
// POR QUÉ ESTO ES UNA PRUEBA Y NO UNA REVISIÓN: la receta es la última cosa que alguien lee, en
// el peor momento, y la va a copiar tal cual. Una instrucción impresa se descubre estando mal el
// día que hay que usarla. El arnés EXTRAE la función del guión real, así que no puede quedarse
// verde sobre una copia.
//
// Sabotajes verificados que la ponen en rojo (los tres corren):
//   - volver a imprimir la línea de la base en las dos ramas: falla «no ofrece restaurar la base»;
//   - sacarla de las dos: falla «ofrece restaurar la base» en la rama que migró;
//   - olvidarse el `systemctl start` en una sola rama: falla el caso 3, que compara las dos
//     recetas línea por línea y exige que la ÚNICA diferencia sea la base.
//
// Y uno que el arnés ya encontró mientras se escribía: el aviso «no restaures» explicaba qué no
// hacer escribiendo el comando entre backticks. La prueba no podía distinguir la receta de su
// propia advertencia — y un humano apurado con una terminal tampoco. Ahora se dice en palabras.
func TestLaRecetaDeVueltaAtrasNoRestauraLaBaseSiNoHuboMigracion(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skipf("el guión de despliegue es de Linux y esta prueba corre bash; en %s no aplica", runtime.GOOS)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("sin bash en el PATH no se puede ejercitar el guión: %v", err)
	}
	arnes := filepath.Join("..", "..", "deploy", "pruebas", "receta-de-vuelta-atras.sh")
	guion := filepath.Join("..", "..", "deploy", "redesplegar-cerebro.sh")
	salida, err := exec.Command(bash, arnes, guion).CombinedOutput()
	if err != nil {
		t.Fatalf("la receta de vuelta atrás falló:\n%s", salida)
	}
	// CONTROL DE QUE EJERCITÓ ALGO: un arnés que no encuentra la función puede salir en 0 sin haber
	// probado nada, y ese verde es peor que un rojo.
	if !strings.Contains(string(salida), "distingue si la corrida migró") {
		t.Fatalf("el arnés terminó en 0 pero no dijo que ejercitó las dos ramas:\n%s", salida)
	}
}
