package mcp

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"musubi/internal/logx"
)

// certificadoDePrueba deja un par PEM en `dir` con el vencimiento pedido y devuelve las dos rutas.
//
// SE GENERA UNO DE VERDAD y no se usa un fixture: lo que estas pruebas miden es que el par se
// CARGUE y que su `NotAfter` salga del certificado, y un fixture con fecha fija no puede ejercitar
// «se renovó y ahora vence más lejos» sin regenerarse igual.
func certificadoDePrueba(t *testing.T, dir string, vence time.Time, cn string) (string, string) {
	t.Helper()
	clave, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	plantilla := x509.Certificate{
		SerialNumber: big.NewInt(vence.UnixNano()),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    vence.Add(-90 * 24 * time.Hour),
		NotAfter:     vence,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{cn},
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, &plantilla, &plantilla, &clave.PublicKey, clave)
	if err != nil {
		t.Fatal(err)
	}
	derClave, err := x509.MarshalECPrivateKey(clave)
	if err != nil {
		t.Fatal(err)
	}
	rutaCert := filepath.Join(dir, "tls.crt")
	rutaKey := filepath.Join(dir, "tls.key")
	escribir := func(ruta, tipo string, bytes []byte, modo os.FileMode) {
		if err := os.WriteFile(ruta, pem.EncodeToMemory(&pem.Block{Type: tipo, Bytes: bytes}), modo); err != nil {
			t.Fatal(err)
		}
	}
	escribir(rutaCert, "CERTIFICATE", der, 0o644)
	escribir(rutaKey, "EC PRIVATE KEY", derClave, 0o600)
	return rutaCert, rutaKey
}

// TestElCertificadoRenovadoEnDiscoEMPIEZAaSERVIRSE — la propiedad entera de esta pieza.
//
// SIN ESTO, EL TIMER DE RENOVACIÓN ES TEATRO. `srv.ListenAndServeTLS(cert, key)` lee los dos
// archivos UNA SOLA VEZ al arrancar: renovar el par en disco deja el nuevo ahí quieto mientras el
// proceso sigue presentando el viejo, hasta que alguien reinicie el cerebro. Con un certificado de
// 90 días, eso se descubre el día 91.
//
// Sabotaje que la pone roja: en `GetCertificate`, devolver `c.par` sin mirar el sello.
func TestElCertificadoRenovadoEnDiscoEMPIEZAaSERVIRSE(t *testing.T) {
	dir := t.TempDir()
	viejo := time.Now().Add(3 * 24 * time.Hour).Truncate(time.Second)
	rutaCert, rutaKey := certificadoDePrueba(t, dir, viejo, "cerebro.tailnet.ts.net")

	c, err := nuevoCertificadoQueSeRelee(rutaCert, rutaKey)
	if err != nil {
		t.Fatalf("no se pudo cargar el par inicial: %v", err)
	}
	if queda, hay := c.venceEn(time.Now()); !hay || queda > 4*24*time.Hour {
		t.Fatalf("el par inicial no se leyó como se esperaba: queda=%v hay=%v", queda, hay)
	}

	// LA RENOVACIÓN: `tailscale cert` reescribe los dos archivos con un certificado nuevo.
	nuevo := time.Now().Add(90 * 24 * time.Hour).Truncate(time.Second)
	esperarMtimeDistinto(t, rutaCert)
	certificadoDePrueba(t, dir, nuevo, "cerebro.tailnet.ts.net")

	if _, err := c.GetCertificate(&tls.ClientHelloInfo{}); err != nil {
		t.Fatalf("el apretón de manos falló tras la renovación: %v", err)
	}
	queda, hay := c.venceEn(time.Now())
	if !hay {
		t.Fatal("tras la renovación no se pudo decir cuándo vence")
	}
	if queda < 80*24*time.Hour {
		t.Errorf("el certificado renovado NO se está sirviendo: quedan %v y el par nuevo vence en ~90 días.\n"+
			"  El proceso sigue presentando el viejo, así que el timer de renovación no cambia nada\n"+
			"  hasta el próximo reinicio — y un `openssl x509` sobre el ARCHIVO diría que está todo\n"+
			"  bien mientras los clientes reciben uno vencido.", queda)
	}
	if c.recargasFallidas() != 0 {
		t.Errorf("la recarga se contó como fallida (%d) y tenía que andar", c.recargasFallidas())
	}
}

// TestUnParAMedioEscribirNoTumbaElTLS — el caso que ocurre EN CADA renovación.
//
// `tailscale cert` escribe el `.crt` y el `.key` en DOS operaciones. Entre una y otra el par de
// disco no casa, y cualquier apretón de manos que caiga ahí encontraría un par inconsistente. Si
// esta pieza devolviera el error, el cerebro dejaría de servir TLS en una ventana que ocurre cada
// vez que se renueva — o sea que la renovación causaría el corte que vino a evitar.
//
// Y EL REINTENTO NO PUEDE SER MUDO: si el par nuevo NUNCA carga (permisos, una key de otro
// certificado), el cerebro se queda con el viejo hasta que vence. Por eso se cuenta y se loguea.
//
// Sabotaje que la pone roja: en la rama de error de `GetCertificate`, devolver `nil, err` en vez
// del par anterior; o sacar el `c.fallos++`.
func TestUnParAMedioEscribirNoTumbaElTLS(t *testing.T) {
	dir := t.TempDir()
	vence := time.Now().Add(30 * 24 * time.Hour).Truncate(time.Second)
	rutaCert, rutaKey := certificadoDePrueba(t, dir, vence, "cerebro.tailnet.ts.net")

	c, err := nuevoCertificadoQueSeRelee(rutaCert, rutaKey)
	if err != nil {
		t.Fatal(err)
	}

	// EL `.crt` DE LA RENOVACIÓN YA ESTÁ, EL `.key` TODAVÍA NO: se escribe el certificado nuevo
	// dejando la clave vieja, que es exactamente el estado intermedio de `tailscale cert`.
	otro := t.TempDir()
	certNuevo, _ := certificadoDePrueba(t, otro, time.Now().Add(90*24*time.Hour), "cerebro.tailnet.ts.net")
	crudo, err := os.ReadFile(certNuevo)
	if err != nil {
		t.Fatal(err)
	}
	esperarMtimeDistinto(t, rutaCert)
	if err := os.WriteFile(rutaCert, crudo, 0o644); err != nil {
		t.Fatal(err)
	}

	var log bytes.Buffer
	restaurar := logx.Capturar(&log)
	par, err := c.GetCertificate(&tls.ClientHelloInfo{})
	restaurar()

	if err != nil {
		t.Fatalf("un par a medio escribir tumbó el TLS: %v.\n"+
			"  Eso pasa en CADA renovación, así que la renovación causaría el corte que vino a evitar.", err)
	}
	if par == nil {
		t.Fatal("se devolvió un certificado nil: el apretón de manos falla igual que con un error")
	}
	// SE SIGUE SIRVIENDO EL VIEJO, que todavía es válido.
	if queda, hay := c.venceEn(time.Now()); !hay || queda > 31*24*time.Hour {
		t.Errorf("no se está sirviendo el certificado anterior: queda=%v hay=%v", queda, hay)
	}
	// Y EL RECHAZO SE VE.
	if c.recargasFallidas() != 1 {
		t.Errorf("las relecturas rechazadas son %d y tenía que ser 1.\n"+
			"  Sin este contador, el camino «el par nuevo nunca carga» no tiene ni una señal: el\n"+
			"  vencimiento avisaría recién al final, cuando ya no hay margen.", c.recargasFallidas())
	}
	if !strings.Contains(log.String(), nombreRecargasFallidas) {
		t.Errorf("el log del rechazo no nombra la serie que lo cuenta, así que quien lo lea no sabe "+
			"dónde mirar si se repite:\n%s", log.String())
	}

	// Y CUANDO LA CLAVE POR FIN LLEGA, entra sin reiniciar nada: el sello no se avanzó, así que se
	// reintenta solo. Sin eso, un rechazo dejaría el par nuevo afuera para siempre.
	certOtro, keyOtro := certificadoDePrueba(t, otro, time.Now().Add(90*24*time.Hour), "cerebro.tailnet.ts.net")
	for _, par := range [][2]string{{certOtro, rutaCert}, {keyOtro, rutaKey}} {
		crudo, err := os.ReadFile(par[0])
		if err != nil {
			t.Fatal(err)
		}
		esperarMtimeDistinto(t, par[1])
		if err := os.WriteFile(par[1], crudo, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := c.GetCertificate(&tls.ClientHelloInfo{}); err != nil {
		t.Fatal(err)
	}
	if queda, _ := c.venceEn(time.Now()); queda < 80*24*time.Hour {
		t.Errorf("tras completarse la renovación el par nuevo NO entró (queda %v): un rechazo "+
			"intermedio dejó el certificado nuevo afuera para siempre", queda)
	}
}

// esperarMtimeDistinto garantiza que la próxima escritura tenga un mtime distinto del actual.
//
// Sin esto la prueba depende de la resolución del reloj del filesystem: en uno con mtime de un
// segundo, escribir dos veces dentro del mismo segundo deja el mismo sello y la relectura no se
// dispara. Sería un parpadeo que aparece sólo en algunas máquinas, que es la peor clase.
func esperarMtimeDistinto(t *testing.T, ruta string) {
	t.Helper()
	fi, err := os.Stat(ruta)
	if err != nil {
		t.Fatal(err)
	}
	limite := time.Now().Add(3 * time.Second)
	for {
		if time.Now().After(fi.ModTime().Add(10 * time.Millisecond)) {
			// Se toca el archivo con una marca futura para no depender de esperar de verdad.
			futuro := fi.ModTime().Add(time.Second)
			if err := os.Chtimes(ruta, futuro, futuro); err == nil {
				return
			}
		}
		if time.Now().After(limite) {
			t.Fatal("no se pudo forzar un mtime distinto")
		}
		time.Sleep(2 * time.Millisecond)
	}
}
