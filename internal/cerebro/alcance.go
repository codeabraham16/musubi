package cerebro

import (
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// ═════════════════════════════════════════════════════════════════════════════════════════════
// LA RECETA PARA CHEQUEAR EL CEREBRO SE MIDE, NO SE RECITA
//
// El manual de operación llevaba escrito, a mano, el comando con el que se comprueba que el
// cerebro está vivo. Medido el 2026-09-22, ese comando devolvía `000` contra un cerebro SANO:
//
//	http://100.79.126.62:7717/readyz          -> connection refused
//	https://musubi-server.tail…ts.net:10000/…  -> 200 en 196 ms
//	https://100.79.126.62:10000/readyz         -> remote error: tls: internal error
//	https://100.79.126.62:10000/readyz + SNI   -> 200 en 203 ms
//
// Las tres cosas que esa tabla enseña, y que ninguna receta escrita a mano conserva:
//
//   - EL PUERTO SE MUDÓ. Desde #601 el cerebro escucha SÓLO en `127.0.0.1:7717` y el TLS lo
//     termina `tailscale serve` por delante, en el `10000`. La receta seguía nombrando el 7717,
//     que es el puerto en claro, y por eso el chequeo fallaba contra un cerebro perfecto.
//   - `000` NO ES UN DIAGNÓSTICO. Es lo que imprime curl cuando no hubo respuesta, y aplasta en
//     un solo número tres mundos distintos: no resuelve, no escucha, o el handshake se cortó. El
//     error de Go —`remote error: tls: internal error`— ya dice cuál de los tres, y es del
//     SERVIDOR: `tailscale serve` no puede elegir certificado sin SNI y corta.
//   - Y LA SALIDA NO ERA CAMBIAR DE IP A NOMBRE. El manual también manda usar IP porque el
//     MagicDNS no resuelve confiable. Las dos reglas parecían incompatibles y no lo son: se disca
//     la IP y se verifica contra el nombre, que es exactamente para lo que existe `ServerName`.
//
// POR QUÉ ESTO ES CÓDIGO Y NO UN PÁRRAFO MEJOR ESCRITO: un literal envejece en silencio y una
// medición no. La receta anterior no estaba mal escrita —estaba VIEJA—, y el día que envejeció
// nadie se enteró, porque un `000` se lee como «el cerebro está caído» y no como «me quedé con la
// dirección de antes». Un comando que sondea vuelve a preguntar cada vez que se lo corre.
// ═════════════════════════════════════════════════════════════════════════════════════════════

// RutaDeSalud es lo que se sondea para saber si el cerebro contesta. `/readyz` y no `/health`
// porque desde el 2026-08-23 `/readyz` sondea la ESCRITURA: con la base trabada, `/health`
// contestaba 200 igual.
const RutaDeSalud = "/readyz"

// Alcance es lo que se midió al sondear el cerebro. Es un HECHO con su hora, no un veredicto:
// quien lo lee decide qué hacer.
type Alcance struct {
	Base   string        // la base sondeada, tal como se la pasaron
	Codigo int           // el HTTP que contestó; 0 si no hubo respuesta
	Demora time.Duration // cuánto tardó, haya llegado o no
	Cuerpo string        // el principio del cuerpo, recortado
	Err    error         // el error crudo del stack HTTP, si lo hubo
	Causa  string        // la causa, SÓLO si se la puede decidir; vacía si no (ver `causaDecidible`)
}

// Llego es la única pregunta que este tipo contesta con un sí o un no.
func (a Alcance) Llego() bool { return a.Codigo == http.StatusOK }

// Sondear le pide `/readyz` al cerebro y devuelve lo que midió. `nombre` es el ServerName contra
// el que verificar el certificado —vacío si la base ya trae el nombre—, y sale de donde ya salía:
// `sync.tls_server_name` o, si no está, la variable que lee NombreTLS.
func Sondear(base, nombre string, espera time.Duration) Alcance {
	return SondearCon(base, nombre, espera, nil)
}

// SondearCon es Sondear con el Transport puesto por quien llama. Existe por la misma razón que el
// `tr` de Cliente —una prueba necesita declarar su raíz de confianza— y mantiene la misma forma,
// así que el ServerName se sigue aplicando DESPUÉS y no hay manera de pisarlo sin querer.
func SondearCon(base, nombre string, espera time.Duration, tr *http.Transport) Alcance {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	a := Alcance{Base: base}

	cli := Cliente(strings.TrimSpace(nombre), espera, tr)
	arranque := time.Now()
	resp, err := cli.Get(base + RutaDeSalud)
	a.Demora = time.Since(arranque)

	if err != nil {
		a.Err = err
		a.Causa = causaDecidible(base, nombre)
		return a
	}
	defer resp.Body.Close()

	cuerpo, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	a.Codigo = resp.StatusCode
	a.Cuerpo = strings.TrimSpace(string(cuerpo))
	return a
}

// causaDecidible devuelve la causa SÓLO cuando sale de los datos de entrada, y "" cuando no.
//
// ACÁ ESTÁ LA DECISIÓN DE DISEÑO, y es la que hace que esto no se pudra. La tentación es clasificar
// el fallo leyendo el texto del error —«connection refused», «no such host», «tls»— y ésa es una
// enumeración de FORMAS: no converge nunca, depende del sistema operativo y del idioma del stack, y
// el día que una forma nueva aparezca esta función va a contestar con seguridad lo que no sabe.
//
// La única causa que se puede DECIDIR sin mirar el error se decide de los argumentos: una base
// `https://` contra una IP PELADA y sin nombre declarado no puede validar nunca, porque el
// certificado de `tailscale cert` lleva el nombre del nodo como único SAN y ninguna IP. Eso es una
// propiedad de la entrada, no una lectura del síntoma, y por eso se puede afirmar.
//
// Para todo lo demás se devuelve "" y el error crudo viaja intacto. Un diagnóstico que falta se
// nota; uno inventado se cree.
func causaDecidible(base, nombre string) string {
	if HTTPSContraIPPelada(base) && strings.TrimSpace(nombre) == "" {
		return "la base es https contra una IP pelada y no se declaró ningún nombre: el certificado " +
			"del tailnet lleva el NOMBRE del nodo como único SAN y ninguna IP, así que el handshake " +
			"no puede validar. La salida no es cambiar la IP por el nombre —el MagicDNS no resuelve " +
			"confiable— sino discar la IP y verificar contra el nombre: " + EnvNombreTLS +
			"=<nodo>.tail<xxxx>.ts.net, o `sync.tls_server_name` en .musubi/config.yaml"
	}
	return ""
}

// HTTPSContraIPPelada dice si una base va a discar una IP por https sin que nadie declare contra
// qué nombre verificar. Es un pie de plomo que no es un error de quien lo escribe y que igual
// conviene decir en voz alta, porque el síntoma —un error de certificado, o directamente un corte
// del handshake del lado del servidor— no habla de la causa.
//
// VIVE ACÁ Y NO EN `internal/provision` PORQUE ES DONDE VIVE EL ARREGLO. El predicado nació ahí,
// al lado del `.mcp.json` que `provision` escribe, y quedó invisible para todo lo demás: el
// paquete que sabe declarar el ServerName es éste, así que el que sabe detectar que falta también.
// Una segunda copia escrita a mano habría sido una copia, con su propio criterio de qué es una IP.
func HTTPSContraIPPelada(base string) bool {
	base = strings.TrimSpace(base)
	if !strings.HasPrefix(strings.ToLower(base), "https://") {
		return false
	}
	hostPort := base[len("https://"):]
	// Se corta el puerto por el ÚLTIMO `:`, salvo que lo que sigue cierre un corchete: en
	// `[::1]:10000` el último `:` de adentro no es el separador.
	host := hostPort
	if i := strings.LastIndex(hostPort, ":"); i >= 0 && !strings.Contains(hostPort[i+1:], "]") {
		host = hostPort[:i]
	}
	host = strings.Trim(host, "[]")
	return net.ParseIP(host) != nil
}
