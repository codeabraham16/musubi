package cerebro

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// elDobleDelProxy levanta un servidor TLS que EXIGE SNI, que es lo que hace `tailscale serve`.
//
// POR QUÉ ESTO REPRODUCE EL MUNDO REAL Y NO ES UN ESCENARIO INVENTADO: el proxy del tailnet tiene
// que elegir certificado por el nombre pedido, así que un handshake sin SNI no lo puede servir y
// lo corta. Y el cliente de Go NO manda SNI cuando el host de la URL es una IP —lo dice el RFC:
// una IP no es un nombre—, así que discar la IP sin declarar nada produce exactamente el corte
// que se midió contra producción el 2026-09-22: `remote error: tls: internal error`.
func elDobleDelProxy(t *testing.T, responde int, cuerpo string) *httptest.Server {
	t.Helper()
	s := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != RutaDeSalud {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(responde)
		_, _ = w.Write([]byte(cuerpo))
	}))
	s.TLS = &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetConfigForClient: func(hi *tls.ClientHelloInfo) (*tls.Config, error) {
			if strings.TrimSpace(hi.ServerName) == "" {
				// Sin SNI no hay con qué elegir certificado. Se corta, igual que el proxy.
				return nil, http.ErrAbortHandler
			}
			return nil, nil
		},
	}
	s.StartTLS()
	t.Cleanup(s.Close)
	return s
}

// transporteQueConfiaEnElDoble devuelve un Transport con la raíz de prueba y NADA más. Se clona
// porque `Cliente` le escribe el ServerName encima: sin clonar, una fila de la tabla le dejaría
// el nombre puesto a la siguiente y la prueba mediría el arrastre en vez del caso.
func transporteQueConfiaEnElDoble(t *testing.T, s *httptest.Server) *http.Transport {
	t.Helper()
	tr, ok := s.Client().Transport.(*http.Transport)
	if !ok {
		t.Fatalf("el cliente del doble no trae *http.Transport: %T", s.Client().Transport)
	}
	return tr.Clone()
}

// TestElSondeoLlegaPorIPPorqueDeclaraElNombre — las dos mitades del hallazgo del 2026-09-22.
//
// EL DEFECTO QUE ESTO FIJA. El manual de operación mandaba chequear el cerebro con una URL
// escrita a mano, y esa URL devolvía `000` contra un cerebro SANO. Dos cosas la habían dejado
// vieja a la vez: el puerto —desde #601 el `7717` es loopback y en claro, y el TLS lo termina
// `tailscale serve` en el `10000`— y el certificado, que lleva el NOMBRE del nodo como único SAN.
// Y el manual, en otra sección, manda usar IP porque el MagicDNS no resuelve confiable.
//
// LAS DOS REGLAS PARECÍAN INCOMPATIBLES Y NO LO SON, y ésa es la mitad que esta guarda custodia:
// se disca la IP —como manda la regla— y se verifica contra el NOMBRE. Sin declararlo, el proxy
// corta el handshake y el error vuelve como si fuera de red.
//
// LA FILA SIN NOMBRE NO ES DECORACIÓN: es el control negativo. Sin ella, un sondeo que ignorara
// el nombre igual daría verde contra cualquier servidor que no exija SNI, y la guarda estaría
// certificando que anda algo que no se probó.
//
// Sabotaje verificado que la pone roja: que el sondeo arme su cliente sin el nombre.
// arnes: archivo="internal/cerebro/alcance.go"
// arnes: de="\tcli := Cliente(strings.TrimSpace(nombre), espera, tr)"
// arnes: a="\tcli := Cliente(\"\", espera, tr)"
func TestElSondeoLlegaPorIPPorqueDeclaraElNombre(t *testing.T) {
	s := elDobleDelProxy(t, http.StatusOK, `{"status":"ready"}`)

	// La base disca la IP, que es lo que hacen las máquinas de la malla. `httptest` escucha en
	// 127.0.0.1, así que la URL que devuelve YA es una IP pelada: el caso es el real.
	base := s.URL
	if !HTTPSContraIPPelada(base) {
		t.Fatalf("el doble no quedó escuchando en una IP pelada (%s): la prueba estaría midiendo "+
			"el caso del NOMBRE, que es justo el que no hace falta arreglar", base)
	}

	// El certificado de httptest vale para `example.com`; ése es el nombre a declarar acá, y hace
	// el mismo papel que `<nodo>.tail<xxxx>.ts.net` en producción.
	const elNombreDelCertificado = "example.com"

	con := SondearCon(base, elNombreDelCertificado, 5*time.Second, transporteQueConfiaEnElDoble(t, s))
	if !con.Llego() {
		t.Fatalf("declarando el nombre NO llegó: codigo=%d err=%v\n"+
			"es el caso que hace andar a la flota entera: discar la IP y verificar contra el nombre",
			con.Codigo, con.Err)
	}
	if con.Cuerpo != `{"status":"ready"}` {
		t.Errorf("llegó pero el cuerpo es %q: el sondeo no está leyendo lo que contesta /readyz", con.Cuerpo)
	}
	if con.Causa != "" {
		t.Errorf("llegó y aun así inventó una causa (%q): una causa sobre un éxito se cree igual", con.Causa)
	}

	sin := SondearCon(base, "", 5*time.Second, transporteQueConfiaEnElDoble(t, s))
	if sin.Llego() {
		t.Fatal("SIN declarar el nombre llegó igual: el doble no está exigiendo SNI, así que la fila " +
			"de arriba no probó que el nombre se declare — probó que el servidor es permisivo")
	}
	if sin.Causa == "" {
		t.Error("no llegó y se quedó sin causa: éste es EXACTAMENTE el caso decidible de los " +
			"argumentos (https contra IP pelada, sin nombre), y si acá calla no sirve para nada")
	}
}

// TestLaCausaSeDecideDeLaEntradaYNoDelTextoDelError — la guarda de lo que la causa NO afirma.
//
// POR QUÉ IMPORTA MÁS LO QUE CALLA. La salida cómoda para clasificar un fallo de red es leer el
// texto del error —«connection refused», «no such host», «tls»— y eso es enumerar FORMAS: no
// converge, cambia con el sistema operativo y con la versión del stack, y el día que aparezca una
// forma nueva la función va a contestar con seguridad algo que no sabe. Este repo ya tiene esa
// lección con nombre y la pagó más de una vez.
//
// Así que la causa se decide de los ARGUMENTOS y de nada más, y las filas que esperan "" son la
// mitad que se rompe primero: son los mundos donde hay un error pero no hay nada que afirmar.
//
// LA SEGUNDA MITAD ES QUE LA CAUSA SEA ACCIONABLE. Una causa que describe el problema y no dice
// qué hacer manda a leer el código igual; ésta tiene que nombrar la variable que lo arregla.
func TestLaCausaSeDecideDeLaEntradaYNoDelTextoDelError(t *testing.T) {
	for _, c := range []struct {
		nombre  string
		base    string
		tlsName string
		decide  bool
		porque  string
	}{
		{"https a IP pelada sin nombre", "https://100.79.126.62:10000", "", true,
			"es el único caso que se puede afirmar sin mirar el error: el certificado del tailnet no lleva SAN de IP"},
		{"https a IP pelada CON nombre", "https://100.79.126.62:10000", "musubi-server.tail89e295.ts.net", false,
			"el nombre está declarado, así que el certificado no es la causa y afirmarlo mandaría a arreglar lo que ya está bien"},
		{"https al NOMBRE sin nombre declarado", "https://musubi-server.tail89e295.ts.net:10000", "", false,
			"la base ya trae el nombre: no hace falta declarar nada y el fallo es por otra cosa"},
		{"http en claro que no contesta", "http://100.79.126.62:7717", "", false,
			"es la receta vieja del manual y falla por connection refused; decidirlo exigiría leer el error, que es lo que no se hace"},
		{"IPv6 pelada entre corchetes", "https://[2001:db8::1]:10000", "", true,
			"una IPv6 es tan IP como una IPv4, y el corchete no puede hacerla pasar por nombre"},
		{"esquema en mayúsculas", "HTTPS://100.79.126.62:10000", "", true,
			"el esquema no distingue mayúsculas y escribirlo así no puede apagar la detección"},
		{"IP pelada sin puerto", "https://100.79.126.62", "", true,
			"el puerto es opcional en una URL y su ausencia no cambia que el destino sea una IP"},
		{"nombre que EMPIEZA con dígitos", "https://10gen.example.com:10000", "", false,
			"empezar con dígitos no lo hace una IP; si esto diera true, la causa acusaría a un nombre legítimo"},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			got := causaDecidible(c.base, c.tlsName)
			if (got != "") != c.decide {
				t.Fatalf("causaDecidible(%q, %q) = %q\nse esperaba decide=%v porque %s",
					c.base, c.tlsName, got, c.decide, c.porque)
			}
			if c.decide && !strings.Contains(got, EnvNombreTLS) {
				t.Errorf("la causa no nombra %s: describe el problema y no dice qué hacer, "+
					"así que manda a leer el código igual\ncausa: %s", EnvNombreTLS, got)
			}
		})
	}
}
