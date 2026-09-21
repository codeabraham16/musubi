package mcp

import (
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/guiones"
)

// ─────────────────────────────────────────────────────────────────────────────────────────────
// EL VERIFICADOR NUNCA TOCABA EL PROXY: LEÍA CONFIG Y DECLARABA POSTURA
// ─────────────────────────────────────────────────────────────────────────────────────────────

// TestElVerificadorSondeaElMcpPorElProxy — la guarda de que el informe MIDE ALCANCE y no sólo lee
// configuración.
//
// EL DEFECTO, MEDIDO EN PRODUCCIÓN EL 2026-09-20. Se ató el cerebro a `127.0.0.1` para cerrar el
// puerto en claro, y `/mcp` POR EL PROXY empezó a contestar 403: atar el bind ENCIENDE la defensa
// anti DNS-rebinding, que exige un `Host` loopback, y `tailscale serve` preserva el del tailnet. La
// flota siguió viva —el latido no pasa por /mcp— pero el sync de memoria de todos los clientes MCP
// se cortó.
//
// Y ESTE INFORME LO HABRÍA CANTADO VERDE. Su rama buena es exactamente «hay un proxy TLS por
// delante Y el cerebro escucha sólo en loopback», que es lo que el config decía. La postura era
// correcta sobre el papel y el camino real estaba cortado: leer configuración no es medir alcance.
//
// LA SONDA VA SIN CREDENCIAL A PROPÓSITO, y por eso puede correr donde el verificador ya corre:
// un 401 dice «la puerta vive y la custodia el bearer», un 403 dice «la compuerta se está comiendo
// al proxy». La diferencia entre esos dos códigos es todo el hallazgo.
//
// Sabotaje verificado: que el 403 deje de ser rojo (el arnés lo caza por dos lados — la fila del
// 403 y el control de despacho degenerado); y renombrar la función, que hace que el arnés ABORTE
// con 2 en vez de pasar en verde sobre nada.
// arnes: archivo="deploy/verificar-despliegue.sh"
// arnes: de="    403)     rojo \"el /mcp del cerebro contesta 403 POR EL PROXY"
// arnes: a="    403)     verde \"el /mcp del cerebro contesta 403 POR EL PROXY"
func TestElVerificadorSondeaElMcpPorElProxy(t *testing.T) {
	// Sin bash esta guarda no existiría y `go test` contestaría `ok`. La compuerta lo convierte en
	// un fallo en vez de un salteo silencioso.
	compuerta := guiones.Exigir(t, "corre deploy/pruebas/sonda-por-el-proxy.sh, que ejercita el "+
		"veredicto de la sonda del verificador sin red ni servidor", "bash", "awk")

	arnes := filepath.Join("..", "..", "deploy", "pruebas", "sonda-por-el-proxy.sh")
	verif := filepath.Join("..", "..", "deploy", "verificar-despliegue.sh")
	salida, err := compuerta.Comando("bash", arnes, verif).CombinedOutput()
	if err != nil {
		t.Fatalf("el arnés de la sonda por el proxy falló:\n%s", salida)
	}

	// CONTROL DE QUE EJERCITÓ ALGO. Un arnés que no encuentre la función sale con 2 —y eso ya es un
	// fallo de arriba—, pero uno que la encuentre y no ejercite los caminos que importan saldría en
	// 0 sin medir nada. Se exigen las dos filas que llevan el hallazgo: el 401 que es la respuesta
	// buena, y el 403 que es el fallo que este informe no podía ver.
	for _, senal := range []string{"HTTP 401 → verde", "HTTP 403 → rojo", "el despacho sabe decir «rojo»"} {
		if !strings.Contains(string(salida), senal) {
			t.Fatalf("el arnés terminó en 0 pero no dijo %q, así que no ejercitó lo que dice:\n%s", senal, salida)
		}
	}
}

// TestLaSondaDelProxyNoSeEscribeAManoEnElVerificador — el frente se DERIVA, no se clava.
//
// El nombre del tailnet (`musubi-server.tail89e295.ts.net:10000`) vive en `tailscale serve` y en el
// certificado. Escribirlo a mano en el verificador sería una tercera copia que envejece sola el día
// que el nodo cambie de nombre — y este repo ya pagó eso: `deploy/cambiar-agente.cmd` tenía la URL
// del cerebro clavada y atar el cerebro a loopback rompía el mecanismo de actualización entero.
//
// Sabotaje: clavar el nombre en vez de derivarlo del JSON de `tailscale serve`.
// arnes: archivo="deploy/verificar-despliegue.sh"
// arnes: de="  FRENTE_TLS=\"$(corre_alla 'tailscale serve status --json 2>/dev/null' | tr -d ' \\n' \\"
// arnes: a="  FRENTE_TLS=\"musubi-server.tail89e295.ts.net:10000\" # $(corre_alla 'tailscale serve status --json 2>/dev/null' | tr -d ' \\n' \\"
func TestLaSondaDelProxyNoSeEscribeAManoEnElVerificador(t *testing.T) {
	crudo := leerDeploy(t, "verificar-despliegue.sh")

	// La pregunta es por el NOMBRE DEL NODO escrito a mano, no por la palabra «tailnet»: el guion
	// habla del tailnet en su prosa y contar menciones haría que la documentación hiciera las veces
	// del código.
	if strings.Contains(crudo, ".ts.net:") {
		t.Error("el verificador lleva un nombre de tailnet escrito a mano.\n" +
			"  El frente del proxy tiene que DERIVARSE del JSON de `tailscale serve status`, que es\n" +
			"  donde vive de verdad: una copia tipeada envejece sola el día que el nodo se renombre,\n" +
			"  y entonces la sonda mide un destino que ya no existe y su rojo acusa al cerebro.")
	}
	if !strings.Contains(crudo, "veredicto_sonda_mcp") {
		t.Error("el verificador no tiene `veredicto_sonda_mcp`: sin esa función, el informe vuelve a " +
			"declarar la postura leyendo el config y sin tocar el camino que usan los clientes — que " +
			"es exactamente cómo el 403 del 2026-09-20 le pasó por al lado")
	}
}
