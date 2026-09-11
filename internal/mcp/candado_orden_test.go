package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// EL CANDADO ANTI FUERZA-BRUTA MIRA LA CREDENCIAL ANTES QUE LA IP — TAMBIÉN EN LAS PUERTAS DE
// FLOTA.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// ES EL HERMANO DE `TestUnaCredencialValidaEntraAunqueLaIPEsteCastigada`, que cubre `/mcp`.
//
// Aquel arregló A88 en la puerta de las PERSONAS: un cliente roto que compartía máquina con
// alguien le agotaba los intentos y lo dejaba afuera con su credencial válida en la mano. Las
// tres puertas de DISPOSITIVOS —latido, resultado y salud— se quedaron con el orden viejo:
// `limiter.locked(ip)` ANTES de resolver el token.
//
// Y ahí el daño es mayor que una persona esperando. Basta UN proceso mal configurado detrás de la
// misma IP —un agente con el token viejo, un script olvidado, una NAT compartida— para que TODAS
// las máquinas que salen por ahí dejen de poder latir. Una máquina que no late se ve exactamente
// igual que una máquina caída, así que el cerebro dispara `MaquinaCaida` sobre equipos encendidos
// y sanos, y `AgenteCaidoConMaquinaViva` no lo desambigua porque el tailnet sí las ve.
//
// MIRAR EL TOKEN PRIMERO NO LE REGALA NADA A QUIEN PRUEBA: resolverlo es un hash y una
// comparación, sin I/O que amplificar, y una credencial que ACIERTA no es un ataque por
// definición. El que falla sigue viendo 429 al agotar sus intentos.
// ────────────────────────────────────────────────────────────────────────────────────────────

func servidorDeFlotaConLatido(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	token := enrolarConExec(t, s, "casa", "pc-gio")
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second}))
	t.Cleanup(ts.Close)
	return ts, token
}

func latirCrudo(t *testing.T, ts, token string) int {
	t.Helper()
	cuerpo, err := json.Marshal(fleet.CuerpoLatido{Version: "0.140.0-prueba", Capver: 2})
	if err != nil {
		t.Fatal(err)
	}
	code, _ := postCon(t, ts+fleetHeartbeatPath, token, string(cuerpo))
	return code
}

func TestUnAgenteConTokenBuenoLateAunqueLaIPEsteCastigada(t *testing.T) {
	ts, token := servidorDeFlotaConLatido(t)

	// El vecino ruidoso agota los cinco intentos de la IP con un token muerto.
	visto429 := false
	for i := 0; i < 6; i++ {
		if latirCrudo(t, ts.URL, "msb_token_que_no_existe_en_ninguna_parte") == http.StatusTooManyRequests {
			visto429 = true
		}
	}
	if !visto429 {
		t.Fatal("el candado no llegó a cerrarse ni con seis intentos fallidos: sin castigo, esta " +
			"prueba no estaría midiendo nada")
	}

	// Y ahora el agente legítimo, desde la MISMA IP.
	if got := latirCrudo(t, ts.URL, token); got == http.StatusTooManyRequests {
		t.Fatal("el agente con el token BUENO recibió 429 porque un vecino de su IP gastó los " +
			"intentos.\n" +
			"  Esa máquina deja de latir, y una máquina que no late se ve igual que una caída: el " +
			"cerebro dispara MaquinaCaida sobre un equipo encendido y sano.\n" +
			"  El candado tiene que resolver la CREDENCIAL primero y castigar la IP sólo cuando falla.")
	} else if got != http.StatusOK {
		t.Fatalf("el latido legítimo devolvió %d", got)
	}
}

// LA OTRA DIRECCIÓN: el candado tiene que seguir cerrándose. Sin este caso, la guarda de arriba la
// satisface borrar el limiter — y entonces cualquiera prueba credenciales de dispositivo sin
// límite, contra la puerta que más expuesta está de toda la flota.
func TestElCandadoDeLaPuertaDeFlotaSigueCerrandose(t *testing.T) {
	ts, _ := servidorDeFlotaConLatido(t)
	for i := 0; i < 10; i++ {
		if latirCrudo(t, ts.URL, "msb_otro_token_que_no_existe") == http.StatusTooManyRequests {
			return
		}
	}
	t.Error("diez intentos con un token inválido desde la misma IP y nunca llegó el 429: el " +
		"candado no se cierra, así que alguien puede probar credenciales de dispositivo sin límite")
}

// Y LA GUARDA DE LA FORMA, para las puertas que las dos pruebas de arriba no ejercitan.
//
// Está anclada en el ORDEN de dos líneas que deciden, no en la presencia de un texto: una puerta
// nueva escrita con el orden viejo nace roja.
func TestNingunaPuertaConsultaElCandadoAntesDeMirarLaCredencial(t *testing.T) {
	archivos, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("no pude listar el paquete: %v", err)
	}
	resuelve := regexp.MustCompile(`DevicePorToken|registry\.resolve|validBearer`)

	revisados, puertas := 0, 0
	for _, e := range archivos {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		crudo, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatalf("no pude leer %s: %v", e.Name(), err)
		}
		revisados++
		// LOS COMENTARIOS SE BLANQUEAN, y lo aprendí acá mismo: el comentario que EXPLICA este
		// arreglo contiene el literal `limiter.locked(ip)` y está escrito ANTES de la resolución,
		// así que la primera versión de esta guarda se disparó sobre las tres puertas que acababa
		// de arreglar. Es exactamente la clase de defecto que este repo persigue —un texto que
		// nombra la cautela contando como la cautela— visto desde el otro lado: acá el comentario
		// no la satisfacía, la rompía.
		//
		// Se blanquean en vez de borrarse para no correr los índices que esta guarda compara.
		var limpio strings.Builder
		for _, linea := range strings.Split(string(crudo), "\n") {
			if strings.HasPrefix(strings.TrimSpace(linea), "//") {
				limpio.WriteByte('\n')
				continue
			}
			limpio.WriteString(linea)
			limpio.WriteByte('\n')
		}
		// Cada handler se mira por separado: buscar el primer `locked(` del ARCHIVO mezclaría
		// puertas distintas y daría un veredicto sobre un orden que no existe.
		for _, tramo := range strings.Split(limpio.String(), "func (s *McpServer) handler")[1:] {
			iLock := strings.Index(tramo, "limiter.locked(")
			if iLock < 0 {
				continue
			}
			puertas++
			loc := resuelve.FindStringIndex(tramo)
			if loc == nil {
				continue // no resuelve credenciales: no aplica
			}
			nombre := strings.Fields(tramo)[0]
			if iLock < loc[0] {
				t.Errorf("%s · %s consulta `limiter.locked(ip)` ANTES de resolver la credencial.\n"+
					"  Así, UN proceso mal configurado detrás de la misma IP deja afuera a TODAS las "+
					"máquinas que salen por ahí — y una máquina que no late se ve igual que una "+
					"caída.\n"+
					"  Resolvé primero; castigá la IP sólo cuando la credencial falla.", e.Name(), nombre)
			}
		}
	}
	if revisados < 20 || puertas < 3 {
		t.Fatalf("se revisaron %d archivos y %d puertas con candado, y son al menos 20 y 3: cambió "+
			"la forma del paquete y esta guarda está en verde sin mirar nada", revisados, puertas)
	}
}

// NINGUNA PUERTA QUE AUTENTIQUE PERSONAS RESUELVE EL BEARER POR SU CUENTA.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// SIETE PUERTAS NO TENÍAN CANDADO. `/metrics`, `/api/actores`, `/api/flota`, `/api/stream` y las
// TRES del relay de shell autenticaban contra el mismo registro que `/mcp` y no aplicaban ningún
// límite de intentos: eran la superficie para probar credenciales sin castigo, y una de ellas
// —el relay— es por donde viaja todo lo que una persona teclea adentro de una máquina.
//
// `principalDeRequest` llegó a documentar que resolvía «con la MISMA regla que /mcp y /metrics».
// Era una afirmación y no un hecho: reimplementaba la resolución sin el candado. Es el defecto
// dominante de este repo en su forma más pura — la cautela escrita en un camino, el comentario
// afirmando que está en todos, y el hermano sin ella.
//
// El arreglo no fue copiar el candado siete veces: escribir el orden ocho veces es escribirlo mal
// una vez. Todas pasan por `autenticarPersona`, y esta guarda impide que nazca la novena por
// fuera.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestNadieResuelveElBearerPorFueraDeLaPuertaComun(t *testing.T) {
	archivos, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("no pude listar el paquete: %v", err)
	}
	revisados, resoluciones := 0, 0
	for _, e := range archivos {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		crudo, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatalf("no pude leer %s: %v", e.Name(), err)
		}
		revisados++
		dentroDeLaPuerta := false
		for n, linea := range strings.Split(string(crudo), "\n") {
			desnuda := strings.TrimSpace(linea)
			if strings.HasPrefix(desnuda, "func ") {
				// LA ÚNICA EXCEPCIÓN ES LA PUERTA MISMA, y se reconoce por ser esa función y no
				// por su archivo: mover `autenticarPersona` de archivo no puede abrir el agujero.
				dentroDeLaPuerta = strings.HasPrefix(desnuda, "func autenticarPersona(")
			}
			if strings.HasPrefix(desnuda, "//") || !strings.Contains(desnuda, "registry.resolve(") {
				continue
			}
			resoluciones++
			if dentroDeLaPuerta {
				continue
			}
			t.Errorf("%s:%d resuelve el bearer por su cuenta:\n    %s\n"+
				"  Esa puerta se queda SIN el candado anti fuerza-bruta, y encima con el riesgo de "+
				"aplicarlo en el orden equivocado — que es como las tres puertas de flota quedaron "+
				"castigando la IP antes de mirar la credencial.\n"+
				"  Usá `autenticarPersona(opt, w, r)`.", e.Name(), n+1, desnuda)
		}
	}
	if revisados < 20 || resoluciones < 1 {
		t.Fatalf("se revisaron %d archivos y %d resoluciones de bearer: cambió la forma del paquete "+
			"y esta guarda está en verde sin mirar nada", revisados, resoluciones)
	}
}

// Y LA PUERTA COMÚN TIENE QUE LLEVAR EL CANDADO PUESTO EN PRODUCCIÓN.
//
// `opt.candado` nil significa SIN LÍMITE —el comportamiento que tenían esas siete puertas— y es
// aceptable sólo en una prueba que arme el handler a mano. Si `HTTPHandler` dejara de ponerlo, las
// ocho puertas volverían a no tener candado sin que nada se ponga rojo.
func TestElConstructorDelHandlerLePoneElCandadoALasOpciones(t *testing.T) {
	crudo, err := os.ReadFile("http.go")
	if err != nil {
		t.Fatalf("no pude leer http.go: %v", err)
	}
	var codigo strings.Builder
	for _, linea := range strings.Split(string(crudo), "\n") {
		if strings.HasPrefix(strings.TrimSpace(linea), "//") {
			codigo.WriteByte('\n')
			continue
		}
		codigo.WriteString(linea)
		codigo.WriteByte('\n')
	}
	if !strings.Contains(codigo.String(), "opt.candado = limiter") {
		t.Error("HTTPHandler ya no le pone el limitador a las opciones: `opt.candado` nil significa " +
			"SIN LÍMITE, así que las ocho puertas que pasan por `autenticarPersona` volverían a " +
			"aceptar intentos ilimitados de credencial — en silencio, y con todo en verde")
	}
}
