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

// TestElSyncLlegaAlCentralPorIPConElNombreDeSuConfig — el handshake de punta a punta.
//
// El doble hace lo que hace `tailscale serve` en :10000: tiene UN certificado, emitido sólo para el
// nombre y sin SAN de IP, y a un ClientHello sin SNI le corta el handshake con «internal error».
// Se disca 127.0.0.1, que para Go es tan IP como la del tailnet: no manda SNI salvo que el cliente
// declare el ServerName.
//
// LAS DOS MITADES SE MIDEN, porque un verde solo no distingue «el nombre anda» de «el doble no
// exige nada». Sin la clave, la misma entrega tiene que caer como transitoria —que es exactamente
// la forma del defecto en producción: pending para siempre— y el doble tiene que haber visto el
// ClientHello sin SNI.
//
// Sabotaje que la hace fallar: que la clave del config se valide y después se descarte.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="\treturn n, nil\n}"
// arnes: a="\treturn cerebro.NombreTLS(), nil\n}"
func TestElSyncLlegaAlCentralPorIPConElNombreDeSuConfig(t *testing.T) {
	t.Setenv(cerebro.EnvNombreTLS, "")

	cert, raices := certificadoSoloParaElNombre(t, elNombreDelCerebro)

	var mu sync.Mutex
	var snis []string
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"obs-tls","result":{"content":[{"type":"text","text":"ok"}]}}`))
	}))
	srv.TLS = &tls.Config{
		// SIN Certificates a propósito: con la lista vacía el servidor le pregunta SIEMPRE a
		// GetCertificate, también cuando el ClientHello no trae SNI. Con un certificado en la lista,
		// un hello sin SNI se lo llevaría igual y el doble dejaría de parecerse a tailscale serve.
		GetCertificate: func(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
			mu.Lock()
			snis = append(snis, h.ServerName)
			mu.Unlock()
			if h.ServerName != elNombreDelCerebro {
				return nil, fmt.Errorf("no hay certificado para el SNI %q", h.ServerName)
			}
			return &cert, nil
		},
		MinVersion: tls.VersionTLS12,
	}
	// El ClientHello sin SNI de la segunda mitad es ESPERADO: su «TLS handshake error» no es ruido
	// que valga la pena leer en la salida de la suite.
	srv.Config.ErrorLog = log.New(io.Discard, "", 0)
	// StartTLS completa Certificates con el suyo cuando la lista viene vacía, y eso desarmaría el
	// doble: se arranca a mano sobre un listener TLS propio.
	srv.Listener = tls.NewListener(srv.Listener, srv.TLS)
	srv.Start()
	defer srv.Close()
	url := "https://" + srv.Listener.Addr().String()
	if !strings.HasPrefix(url, "https://127.0.0.1:") {
		t.Fatalf("el doble no escucha en una IP: %s", url)
	}

	vistos := func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := append([]string(nil), snis...)
		snis = nil
		return out
	}
	item := memory.OutboxItem{ObsID: "obs-tls", TopicKey: "t", Content: "c", Importance: 0.5}

	// ── CON LA CLAVE ─────────────────────────────────────────────────────────────────────────
	con, err := NewSyncClient(cfgSyncPorIP(url, elNombreDelCerebro))
	if err != nil {
		t.Fatalf("NewSyncClient: %v", err)
	}
	confiarEnLaRaizDePrueba(con, raices)
	if err := con.Push(item); err != nil {
		t.Fatalf("con sync.tls_server_name puesto la entrega por IP falló: %v\n  SNI visto por el "+
			"servidor: %q. Sin SNI, tailscale serve corta el handshake y la fila queda pending para "+
			"siempre.", err, vistos())
	}
	if v := vistos(); len(v) == 0 || v[0] != elNombreDelCerebro {
		t.Errorf("el servidor vio SNI %q, esperaba %q", v, elNombreDelCerebro)
	}

	// ── SIN LA CLAVE: el control que prueba que el doble exige el nombre ─────────────────────
	sin, err := NewSyncClient(cfgSyncPorIP(url, ""))
	if err != nil {
		t.Fatalf("NewSyncClient: %v", err)
	}
	confiarEnLaRaizDePrueba(sin, raices)
	err = sin.Push(item)
	if err == nil {
		t.Fatal("SIN nombre la entrega por IP pasó igual: el doble no exige SNI y el verde de arriba no " +
			"prueba nada")
	}
	if !errors.Is(err, errTransient) {
		t.Errorf("sin nombre el fallo no es transitorio (%v): el doble dejó de reproducir la forma del defecto", err)
	}
	if v := vistos(); len(v) == 0 || v[0] != "" {
		t.Errorf("sin nombre el servidor vio SNI %q, esperaba un ClientHello sin SNI", v)
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

// certificadoSoloParaElNombre emite un certificado autofirmado cuyo ÚNICO SAN es `nombre`, igual
// que el de `tailscale cert`, y devuelve el pool que lo reconoce como raíz.
func certificadoSoloParaElNombre(t *testing.T, nombre string) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	clave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	plantilla := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: nombre},
		DNSNames:              []string{nombre},
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
	if len(hoja.IPAddresses) != 0 {
		t.Fatalf("el certificado de prueba trae SAN de IP %v: el test dejaría de reproducir el defecto", hoja.IPAddresses)
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
