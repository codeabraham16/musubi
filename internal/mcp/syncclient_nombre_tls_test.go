package mcp

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/cerebro"
	"musubi/internal/config"
	"musubi/internal/memory"
)

// elNombreDelCerebro es el único SAN del certificado de `tailscale cert` del cerebro en :10000.
// No trae SAN de IP, y ése es todo el problema.
const elNombreDelCerebro = "musubi-server.tail89e295.ts.net"

// cfgSyncPorIP es el config que queda cuando central_url se escribe con la IP del tailnet, que es
// lo que hay que hacer porque con NordVPN el MagicDNS no resuelve.
func cfgSyncPorIP(url, nombre string) config.SyncConfig {
	return config.SyncConfig{Enabled: true, CentralURL: url, TLSServerName: nombre}
}

// TestElSyncDeclaraElNombreTLSDeSuConfig — la clave `sync.tls_server_name` llega al ServerName.
//
// EL DEFECTO. El sync saca su URL SÓLO de `.musubi/config.yaml`, y el nombre sólo podía venir de
// MUSUBI_BRAIN_TLS_NAME, una variable que el daemon hereda o no según quién lo lanzó. Sin nombre,
// discar la IP no manda SNI y `tailscale serve` corta el handshake: error de red, transitorio, y
// las filas 'shared' pending para siempre.
//
// Custodia también las otras dos mitades, que son las que un arreglo apurado rompe: sin nombre en
// ningún lado el cliente sigue SIN Transport propio (el default del stdlib sigue siendo el
// default), y la clave del config le GANA a la variable, porque es la que vive al lado de la URL.
//
// Sabotaje que la hace fallar: que el constructor ignore la clave del config.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="\tn := strings.TrimSpace(cfg.TLSServerName)"
// arnes: a="\tn := strings.TrimSpace(\"\")"
func TestElSyncDeclaraElNombreTLSDeSuConfig(t *testing.T) {
	t.Setenv(cerebro.EnvNombreTLS, "")

	serverName := func(t *testing.T, cfg config.SyncConfig) (string, bool) {
		t.Helper()
		c, err := NewSyncClient(cfg)
		if err != nil {
			t.Fatalf("NewSyncClient: %v", err)
		}
		if c.http.Transport == nil {
			return "", false
		}
		tr, ok := c.http.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("el Transport no es *http.Transport: %T", c.http.Transport)
		}
		if tr.TLSClientConfig == nil {
			t.Fatal("hay Transport propio pero sin TLSClientConfig: el nombre no llegó a ninguna parte")
		}
		if tr.TLSClientConfig.InsecureSkipVerify {
			t.Fatal("InsecureSkipVerify PRENDIDO: declarar el nombre no puede apagar la verificación")
		}
		if tr.TLSClientConfig.MinVersion != tls.VersionTLS12 {
			t.Errorf("MinVersion = %d, esperaba TLS 1.2", tr.TLSClientConfig.MinVersion)
		}
		return tr.TLSClientConfig.ServerName, true
	}

	t.Run("la clave del config llega al ServerName", func(t *testing.T) {
		got, ok := serverName(t, cfgSyncPorIP("https://100.79.126.62:10000", "  "+elNombreDelCerebro+" "))
		if !ok {
			t.Fatal("con sync.tls_server_name puesto el cliente salió SIN Transport propio: el sync va a " +
				"discar la IP sin SNI y tailscale serve le corta el handshake")
		}
		if got != elNombreDelCerebro {
			t.Errorf("ServerName = %q, esperaba %q", got, elNombreDelCerebro)
		}
	})

	t.Run("sin nombre en ningún lado no hay Transport propio", func(t *testing.T) {
		if _, ok := serverName(t, cfgSyncPorIP("https://100.79.126.62:10000", "")); ok {
			t.Error("sin nombre declarado el cliente trae Transport propio: el default del stdlib dejó de ser el default")
		}
	})

	t.Run("la clave del config le gana a la variable", func(t *testing.T) {
		t.Setenv(cerebro.EnvNombreTLS, "otro-nodo.tail89e295.ts.net")
		got, _ := serverName(t, cfgSyncPorIP("https://100.79.126.62:10000", elNombreDelCerebro))
		if got != elNombreDelCerebro {
			t.Errorf("ServerName = %q: la variable pisó a la clave que vive al lado de la URL", got)
		}
	})

	t.Run("sin clave sigue valiendo la variable", func(t *testing.T) {
		t.Setenv(cerebro.EnvNombreTLS, elNombreDelCerebro)
		got, ok := serverName(t, cfgSyncPorIP("https://100.79.126.62:10000", ""))
		if !ok || got != elNombreDelCerebro {
			t.Errorf("ServerName = %q (Transport propio: %v): quien ya exportaba MUSUBI_BRAIN_TLS_NAME se "+
				"quedó sin nombre", got, ok)
		}
	})
}

// ═════════════════════════════════════════════════════════════════════════════════════════════
// EL HANDSHAKE DE PUNTA A PUNTA: UNA PRUEBA CON LA FORMA DE PRODUCCIÓN Y UNA POR CADA CAUSA
//
// En producción el sync por IP muere por DOS causas independientes, y cualquiera de las dos
// alcanza:
//
//  1. EL SERVIDOR CORTA SIN SNI. `tailscale serve` elige el certificado por el SNI; a un
//     ClientHello sin SNI le contesta con la alerta 80 («remote error: tls: internal error») y el
//     cliente nunca llega a ver un certificado.
//  2. EL CERTIFICADO NO TIENE SAN DE IP. Aunque el servidor lo entregara, el cliente lo verifica
//     contra la IP que discó, y el de `tailscale cert` sólo nombra al nodo.
//
// El arreglo —declarar el ServerName— cubre las dos a la vez: manda el SNI y verifica contra el
// nombre. La primera versión de estas pruebas medía las dos JUNTAS con un solo doble, y un revisor
// le desactivó la guarda de SNI (`if false && h.ServerName != …`) y la prueba siguió en VERDE: sin
// ServerName el cliente fallaba igual por la causa 2, así que el «sin la clave tiene que fallar»
// se cumplía por la razón que no se estaba mirando. Un rojo que tiene dos motivos posibles no
// prueba ninguno de los dos. Por eso cada causa tiene ahora su prueba, con la OTRA apagada: el
// doble de la causa 1 lleva un certificado que SÍ valida por IP, y el de la causa 2 entrega su
// certificado a cualquier ClientHello.

// TestElSyncLlegaAlCentralPorIPConElNombreDeSuConfig — la forma de producción, las dos causas juntas.
//
// El doble hace lo que hace `tailscale serve` en :10000: tiene UN certificado, emitido sólo para el
// nombre y sin SAN de IP, y a un ClientHello sin SNI le corta el handshake con «internal error».
// Se disca 127.0.0.1, que para Go es tan IP como la del tailnet: no manda SNI salvo que el cliente
// declare el ServerName.
//
// Sin la clave la entrega tiene que caer transitoria —que es la forma del defecto en producción:
// pending para siempre— y con el MISMO texto que se midió contra el cerebro. El texto no es
// decorativo: con las dos causas puestas el servidor corta ANTES de que el cliente vea un
// certificado, así que «tls: internal error» es la causa 1 hablando. Qué causa sostiene sola ese
// rojo lo dicen las dos pruebas de abajo, no ésta.
//
// Sabotaje que la hace fallar: que la clave del config se valide y después se descarte.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="\treturn n, nil\n}"
// arnes: a="\treturn cerebro.NombreTLS(), nil\n}"
func TestElSyncLlegaAlCentralPorIPConElNombreDeSuConfig(t *testing.T) {
	t.Setenv(cerebro.EnvNombreTLS, "")

	cert, raices := certificadoParaElNombre(t, elNombreDelCerebro)
	d := levantarElDoble(t, cert, true)

	con, sin := entregarConYSinLaClave(t, d, raices)
	if con.err != nil {
		t.Fatalf("con sync.tls_server_name puesto la entrega por IP falló: %v\n  SNI visto por el "+
			"servidor: %q. Sin SNI, tailscale serve corta el handshake y la fila queda pending para "+
			"siempre.", con.err, con.snis)
	}
	if len(con.snis) == 0 || con.snis[0] != elNombreDelCerebro {
		t.Errorf("el servidor vio SNI %q, esperaba %q", con.snis, elNombreDelCerebro)
	}
	if sin.err == nil {
		t.Fatal("SIN nombre la entrega por IP pasó igual: el doble no reproduce a tailscale serve y el " +
			"verde de arriba no prueba nada")
	}
	if !errors.Is(sin.err, errTransient) {
		t.Errorf("sin nombre el fallo no es transitorio (%v): el doble dejó de reproducir la forma del defecto", sin.err)
	}
	if !strings.Contains(sin.err.Error(), "tls: internal error") {
		t.Errorf("sin nombre el fallo no es el que se midió contra el cerebro («remote error: tls: internal "+
			"error»): %v", sin.err)
	}
	if len(sin.snis) == 0 || sin.snis[0] != "" {
		t.Errorf("sin nombre el servidor vio SNI %q, esperaba un ClientHello sin SNI", sin.snis)
	}
}

// TestSinNombreElServidorCortaElHandshakePorFaltaDeSNI — la causa 1, SOLA.
//
// El certificado de este doble SÍ trae SAN de IP (127.0.0.1): la causa 2 está apagada a propósito,
// y un cliente sin ServerName podría verificarlo perfectamente. Lo único que queda para hacer
// fallar la entrega sin la clave es que el servidor exija el SNI. Si esa guarda del doble se apaga,
// la entrega sin la clave PASA y esta prueba se pone roja: es la que el revisor no encontró.
//
// Y la mitad CON la clave dice que el arreglo cubre esta causa por sí solo: el nombre viaja en el
// ClientHello y el servidor lo ve.
//
// Sabotaje que la hace fallar: apagar la guarda de SNI del doble.
// arnes: archivo="internal/mcp/syncclient_nombre_tls_test.go"
// arnes: de="\t\t\tif h.ServerName != elNombreDelCerebro {"
// arnes: a="\t\t\tif false && h.ServerName != elNombreDelCerebro {"
func TestSinNombreElServidorCortaElHandshakePorFaltaDeSNI(t *testing.T) {
	t.Setenv(cerebro.EnvNombreTLS, "")

	cert, raices := certificadoParaElNombre(t, elNombreDelCerebro, net.IPv4(127, 0, 0, 1))
	d := levantarElDoble(t, cert, true)

	con, sin := entregarConYSinLaClave(t, d, raices)
	if con.err != nil {
		t.Fatalf("con la clave la entrega falló contra un doble que sólo exige SNI: %v (SNI visto %q)", con.err, con.snis)
	}
	if len(con.snis) == 0 || con.snis[0] != elNombreDelCerebro {
		t.Errorf("con la clave el servidor vio SNI %q, esperaba %q: el nombre no viajó en el ClientHello",
			con.snis, elNombreDelCerebro)
	}
	if sin.err == nil {
		t.Fatal("SIN la clave la entrega pasó: el doble no exige SNI, así que nada de esta prueba mide la " +
			"causa 1 (el servidor que corta el handshake sin SNI)")
	}
	if !strings.Contains(sin.err.Error(), "tls: internal error") {
		t.Errorf("sin la clave falló, pero no porque el servidor cortara el handshake: %v", sin.err)
	}
	if !errors.Is(sin.err, errTransient) {
		t.Errorf("sin la clave el fallo no es transitorio (%v)", sin.err)
	}
	if len(sin.snis) == 0 || sin.snis[0] != "" {
		t.Errorf("sin la clave el servidor vio SNI %q, esperaba un ClientHello sin SNI", sin.snis)
	}
}

// TestSinNombreElClienteRechazaElCertificadoPorFaltaDeSANDeIP — la causa 2, SOLA.
//
// Este doble NO exige SNI: le entrega su certificado a cualquier ClientHello, así que la causa 1
// está apagada. El certificado es el de `tailscale cert`, con el nombre y ninguna IP, y lo único que
// queda para hacer fallar la entrega sin la clave es que el CLIENTE lo verifique contra la IP que
// discó. Si el certificado gana un SAN de IP, la entrega sin la clave PASA y esta prueba se pone
// roja.
//
// La mitad CON la clave dice que el arreglo cubre también esta causa: con el ServerName declarado
// el cliente verifica contra el nombre, no contra la IP, y la verificación sigue ENTERA (la raíz de
// prueba es la única en la que se confía).
//
// Sabotaje que la hace fallar: darle al certificado del cerebro un SAN de IP.
// arnes: archivo="internal/mcp/syncclient_nombre_tls_test.go"
// arnes: de="\tcertSinIP, raices := certificadoParaElNombre(t, elNombreDelCerebro)\n"
// arnes: a="\tcertSinIP, raices := certificadoParaElNombre(t, elNombreDelCerebro, net.IPv4(127, 0, 0, 1))\n"
func TestSinNombreElClienteRechazaElCertificadoPorFaltaDeSANDeIP(t *testing.T) {
	t.Setenv(cerebro.EnvNombreTLS, "")

	certSinIP, raices := certificadoParaElNombre(t, elNombreDelCerebro)
	d := levantarElDoble(t, certSinIP, false)

	con, sin := entregarConYSinLaClave(t, d, raices)
	if con.err != nil {
		t.Fatalf("con la clave la entrega falló contra un doble que no exige SNI: %v", con.err)
	}
	if sin.err == nil {
		t.Fatal("SIN la clave la entrega pasó: el cliente aceptó por IP un certificado que tendría que " +
			"nombrar sólo al nodo, así que nada de esta prueba mide la causa 2 (el SAN de IP que falta)")
	}
	if !strings.Contains(sin.err.Error(), "doesn't contain any IP SANs") {
		t.Errorf("sin la clave falló, pero no porque el certificado no tenga SAN de IP: %v", sin.err)
	}
	if strings.Contains(sin.err.Error(), "tls: internal error") {
		t.Errorf("sin la clave el servidor cortó el handshake, y este doble no exige SNI: la causa 1 se "+
			"coló en la prueba de la causa 2: %v", sin.err)
	}
	if !errors.Is(sin.err, errTransient) {
		t.Errorf("sin la clave el fallo no es transitorio (%v)", sin.err)
	}
	// El servidor entregó el certificado igual: el ClientHello sin SNI llegó y no se cortó.
	if len(sin.snis) == 0 || sin.snis[0] != "" {
		t.Errorf("sin la clave el servidor vio SNI %q, esperaba un ClientHello sin SNI", sin.snis)
	}
}

// TestElSyncRechazaUnNombreTLSConEsquemaOPuerto — un nombre mal escrito falla al ARRANCAR.
//
// `tls_server_name: https://nodo…:10000` es el error natural de quien copia la URL. Llegaría al
// handshake como un nombre que ningún certificado tiene, fallaría como error de red —transitorio—
// y la fila se reintentaría para siempre, con la configuración «puesta». Rechazado en el
// constructor, el daemon lo dice una vez al arrancar y el sync ni se enciende.
//
// Sabotaje que la hace fallar: sacar la validación.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="\tif strings.ContainsAny(n, \":/ \\t\") {"
// arnes: a="\tif false && strings.ContainsAny(n, \":/ \\t\") {"
func TestElSyncRechazaUnNombreTLSConEsquemaOPuerto(t *testing.T) {
	t.Setenv(cerebro.EnvNombreTLS, "")
	for _, malo := range []string{
		"https://" + elNombreDelCerebro,
		elNombreDelCerebro + ":10000",
		elNombreDelCerebro + "/mcp",
		"musubi server",
	} {
		_, err := NewSyncClient(cfgSyncPorIP("https://100.79.126.62:10000", malo))
		if err == nil {
			t.Errorf("tls_server_name %q se aceptó: va a fallar en cada handshake como error de red, "+
				"para siempre", malo)
			continue
		}
		if !errors.Is(err, errPermanent) || !strings.Contains(err.Error(), "tls_server_name") {
			t.Errorf("tls_server_name %q: el error tiene que ser permanente y nombrar la clave, dijo: %v", malo, err)
		}
	}
}

// TestElSyncRechazaUnNombreTLSMalEscritoEnLaVariable — el respaldo pasa por la misma aduana.
//
// Sin la clave del config, el nombre sale de MUSUBI_BRAIN_TLS_NAME. La primera versión validaba
// sólo la clave, y el valor de la variable llegaba al handshake tal cual: un `https://nodo…` en el
// entorno reproducía el defecto entero —error de red, transitorio, pending para siempre— con la
// validación «puesta». El error tiene que nombrar a la VARIABLE: mandar a corregir la clave cuando
// lo malo es el entorno es mandar a editar el archivo equivocado.
//
// Y la otra mitad: con la clave puesta la variable no llega al handshake del sync, así que su valor
// no lo frena. Validar lo que no se usa convertiría el entorno sucio de OTRO cliente en un sync
// apagado.
//
// Sabotaje que la hace fallar: que el respaldo de la variable vuelva a salir sin validar.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="\t\torigen, n = cerebro.EnvNombreTLS, cerebro.NombreTLS()\n"
// arnes: a="\t\treturn cerebro.NombreTLS(), nil\n"
func TestElSyncRechazaUnNombreTLSMalEscritoEnLaVariable(t *testing.T) {
	for _, malo := range []string{
		"https://" + elNombreDelCerebro,
		elNombreDelCerebro + ":10000",
		elNombreDelCerebro + "/mcp",
		"musubi server",
	} {
		t.Setenv(cerebro.EnvNombreTLS, malo)
		_, err := NewSyncClient(cfgSyncPorIP("https://100.79.126.62:10000", ""))
		if err == nil {
			t.Errorf("%s=%q se aceptó sin tls_server_name en el config: llega al handshake y falla como "+
				"error de red, para siempre", cerebro.EnvNombreTLS, malo)
			continue
		}
		if !errors.Is(err, errPermanent) || !strings.Contains(err.Error(), cerebro.EnvNombreTLS) {
			t.Errorf("%s=%q: el error tiene que ser permanente y nombrar a la VARIABLE, dijo: %v",
				cerebro.EnvNombreTLS, malo, err)
		}
	}

	t.Setenv(cerebro.EnvNombreTLS, "https://"+elNombreDelCerebro)
	if _, err := NewSyncClient(cfgSyncPorIP("https://100.79.126.62:10000", elNombreDelCerebro)); err != nil {
		t.Errorf("con la clave puesta la variable no llega al handshake del sync, y aun así lo frenó: %v", err)
	}
}

// dobleDelCerebro es un servidor HTTPS en 127.0.0.1 que anota el SNI de cada ClientHello.
type dobleDelCerebro struct {
	url  string
	mu   sync.Mutex
	snis []string
}

// vistos devuelve los SNI anotados desde la última llamada y vacía la lista.
func (d *dobleDelCerebro) vistos() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := d.snis
	d.snis = nil
	return out
}

// levantarElDoble arranca el doble con UN certificado. Con `exigeSNI` se porta como `tailscale
// serve`: a un ClientHello cuyo SNI no es el nombre del cerebro le corta el handshake, y el cliente
// ve la alerta 80 («remote error: tls: internal error»). Sin `exigeSNI` le entrega el certificado a
// cualquiera, que es lo que hace falta para medir la causa del SAN de IP sin la otra encima.
func levantarElDoble(t *testing.T, cert tls.Certificate, exigeSNI bool) *dobleDelCerebro {
	t.Helper()
	d := &dobleDelCerebro{}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"obs-tls","result":{"content":[{"type":"text","text":"ok"}]}}`))
	}))
	srv.TLS = &tls.Config{
		// SIN Certificates a propósito: con la lista vacía el servidor le pregunta SIEMPRE a
		// GetCertificate, también cuando el ClientHello no trae SNI. Con un certificado en la lista,
		// un hello sin SNI se lo llevaría igual y la guarda de abajo no existiría.
		GetCertificate: func(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
			d.mu.Lock()
			d.snis = append(d.snis, h.ServerName)
			d.mu.Unlock()
			if !exigeSNI {
				return &cert, nil
			}
			// LA GUARDA DE SNI DEL DOBLE. Apagarla es el sabotaje de
			// TestSinNombreElServidorCortaElHandshakePorFaltaDeSNI.
			if h.ServerName != elNombreDelCerebro {
				return nil, fmt.Errorf("no hay certificado para el SNI %q", h.ServerName)
			}
			return &cert, nil
		},
		MinVersion: tls.VersionTLS12,
	}
	// Los ClientHello rechazados de las mitades «sin la clave» son ESPERADOS: su «TLS handshake
	// error» no es ruido que valga la pena leer en la salida de la suite.
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	// StartTLS completa Certificates con el suyo cuando la lista viene vacía, y eso desarmaría el
	// doble: se arranca a mano sobre un listener TLS propio.
	srv.Listener = tls.NewListener(srv.Listener, srv.TLS)
	srv.Start()
	t.Cleanup(srv.Close)
	d.url = "https://" + srv.Listener.Addr().String()
	if !strings.HasPrefix(d.url, "https://127.0.0.1:") {
		t.Fatalf("el doble no escucha en una IP: %s", d.url)
	}
	return d
}

// entrega es lo que pasó con UNA entrega: el error de Push y los SNI que vio el servidor.
type entrega struct {
	err  error
	snis []string
}

// entregarConYSinLaClave empuja la misma fila dos veces contra el doble: una con
// `sync.tls_server_name` puesta y otra sin ella. Los dos clientes salen de NewSyncClient, que es lo
// que se mide; la prueba sólo les agrega la raíz de confianza del certificado de prueba.
func entregarConYSinLaClave(t *testing.T, d *dobleDelCerebro, raices *x509.CertPool) (con, sin entrega) {
	t.Helper()
	item := memory.OutboxItem{ObsID: "obs-tls", TopicKey: "t", Content: "c", Importance: 0.5}
	empujar := func(nombre string) entrega {
		c, err := NewSyncClient(cfgSyncPorIP(d.url, nombre))
		if err != nil {
			t.Fatalf("NewSyncClient(tls_server_name=%q): %v", nombre, err)
		}
		confiarEnLaRaizDePrueba(c, raices)
		d.vistos()
		err = c.Push(item)
		return entrega{err: err, snis: d.vistos()}
	}
	return empujar(elNombreDelCerebro), empujar("")
}

// certificadoParaElNombre emite un certificado autofirmado para `nombre` y, si se piden, para las
// `ips` —el de `tailscale cert` NO trae ninguna—, y devuelve el pool que lo reconoce como raíz.
func certificadoParaElNombre(t *testing.T, nombre string, ips ...net.IP) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	clave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	plantilla := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: nombre},
		DNSNames:              []string{nombre},
		IPAddresses:           ips,
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &plantilla, &plantilla, &clave.PublicKey, clave)
	if err != nil {
		t.Fatal(err)
	}
	hoja, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	if len(hoja.IPAddresses) != len(ips) {
		t.Fatalf("pedí %d SAN de IP y el certificado trae %v: el doble no es el que la prueba cree", len(ips), hoja.IPAddresses)
	}
	raices := x509.NewCertPool()
	raices.AddCert(hoja)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: clave}, raices
}

// confiarEnLaRaizDePrueba le agrega al cliente YA CONSTRUIDO la raíz del certificado de prueba, y
// nada más: el ServerName (o su ausencia) queda como lo dejó NewSyncClient, que es lo que se mide.
// Sin Transport propio se clona el default, igual que haría el stdlib.
func confiarEnLaRaizDePrueba(c *SyncClient, raices *x509.CertPool) {
	tr, _ := c.http.Transport.(*http.Transport)
	if tr == nil {
		tr = http.DefaultTransport.(*http.Transport).Clone()
		c.http.Transport = tr
	}
	if tr.TLSClientConfig == nil {
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	tr.TLSClientConfig.RootCAs = raices
}
