package mcp

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"musubi/internal/logx"
)

// ════════════════════════════════════════════════════════════════════════════════════════════
// EL CERTIFICADO SE RELEE, Y SIN ESTO UN TIMER DE RENOVACIÓN NO SERVIRÍA PARA NADA
//
// `tailscale cert` emite un certificado válido 90 días y se renueva volviéndolo a correr. Pero
// `srv.ListenAndServeTLS(cert, key)` LEE LOS DOS ARCHIVOS UNA SOLA VEZ, al arrancar: renovarlos en
// disco deja el par nuevo ahí quieto mientras el proceso sigue presentando el viejo, hasta que
// alguien reinicie el cerebro.
//
// Eso convierte el timer semanal en teatro, y de la peor forma: un `openssl x509` sobre el ARCHIVO
// contestaría «faltan 89 días» mientras los clientes reciben un certificado vencido. Productor y
// parser sin nadie en el medio, otra vez, y acá el que paga es el que disca.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LO QUE ESTA PIEZA GARANTIZA, Y LO QUE A PROPÓSITO NO
//
//  1. Relee cuando el par CAMBIÓ en disco (mtime+tamaño de los dos archivos), no en cada apretón
//     de manos: un `os.Stat` por conexión es barato; un `LoadX509KeyPair` no.
//  2. SI LA RELECTURA FALLA, SIGUE SIRVIENDO EL VIEJO. No es tolerancia: es que `tailscale cert`
//     escribe el `.crt` y el `.key` en DOS operaciones, y entre una y otra el par de disco no
//     casa. Tirar el certificado ahí dejaría al cerebro sin servir TLS por una ventana de
//     milisegundos que ocurre cada renovación. El viejo todavía es válido; se reintenta solo.
//  3. EL FALLO SE VE. Cada rechazo se loguea con el error y con el nombre de la serie que lo
//     cuenta, y `musubi_tls_certificate_reload_failures` sube. Un reintento silencioso sería el
//     modo de falla de siempre: el archivo nuevo nunca entra y nadie se entera hasta que vence.
//  4. LA FECHA QUE SE PUBLICA ES LA DEL CERTIFICADO QUE SE ESTÁ SIRVIENDO, no la del archivo. Si
//     la relectura viene fallando, la serie sigue mostrando el vencimiento VIEJO y cuenta hacia
//     atrás hasta disparar — que es exactamente lo que hay que ver.
//
// NO renueva nada: eso es `deploy/renovar-tls.sh`, que corre en el servidor del cerebro. Las dos
// piezas están a propósito en manos distintas — quien renueva no es quien mide.
// ════════════════════════════════════════════════════════════════════════════════════════════

// certificadoQueSeRelee entrega el par TLS y lo vuelve a leer del disco cuando cambió.
type certificadoQueSeRelee struct {
	rutaCert, rutaKey string

	mu     sync.RWMutex
	sello  string           // mtime+tamaño de los DOS archivos, tal como estaban al cargar
	par    *tls.Certificate // lo que se está sirviendo AHORA
	vence  time.Time        // NotAfter del `par` de arriba; cero si no se pudo parsear
	fallos int              // relecturas rechazadas desde que arrancó el proceso
}

// nuevoCertificadoQueSeRelee carga el par por primera vez. Un fallo acá SÍ es fatal: arrancar sin
// certificado y quedarse esperando a que aparezca sería un cerebro que dice estar sirviendo TLS y
// rechaza cada conexión, que es peor que no arrancar.
func nuevoCertificadoQueSeRelee(rutaCert, rutaKey string) (*certificadoQueSeRelee, error) {
	c := &certificadoQueSeRelee{rutaCert: rutaCert, rutaKey: rutaKey}
	par, vence, err := cargarPar(rutaCert, rutaKey)
	if err != nil {
		return nil, fmt.Errorf("no se pudo cargar el certificado TLS (%s / %s): %w", rutaCert, rutaKey, err)
	}
	c.par, c.vence, c.sello = par, vence, selloDeLosDos(rutaCert, rutaKey)
	return c, nil
}

// GetCertificate es el gancho de `tls.Config`. Se llama una vez por apretón de manos.
func (c *certificadoQueSeRelee) GetCertificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	sello := selloDeLosDos(c.rutaCert, c.rutaKey)

	c.mu.RLock()
	igual := sello == c.sello || sello == ""
	par := c.par
	c.mu.RUnlock()
	if igual {
		// `sello == ""` es «no pude mirar el disco». No es «no cambió», pero la respuesta
		// operativa es la misma —seguir sirviendo lo que hay— y forzar una relectura que
		// tampoco va a poder leer no mejora nada.
		return par, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// Otro apretón de manos pudo haber recargado mientras esperábamos el candado.
	if sello == c.sello {
		return c.par, nil
	}
	nuevo, vence, err := cargarPar(c.rutaCert, c.rutaKey)
	if err != nil {
		// EL PAR A MEDIO ESCRIBIR ES EL CASO NORMAL, NO EL RARO: `tailscale cert` escribe los dos
		// archivos en dos operaciones. Se sigue con el viejo y se reintenta en el apretón que
		// viene; el sello NO se avanza, justamente para que se reintente.
		c.fallos++
		logx.Error("musubi: el certificado TLS cambió en disco y NO se pudo cargar; se sigue sirviendo el anterior",
			"cert", c.rutaCert, "error", err, "fallos", c.fallos,
			"serie", nombreRecargasFallidas)
		return c.par, nil
	}
	anterior := c.vence
	c.par, c.vence, c.sello = nuevo, vence, sello
	logx.Info("musubi: certificado TLS recargado en caliente",
		"cert", c.rutaCert, "vencia", anterior.Format(time.RFC3339), "vence", vence.Format(time.RFC3339))
	return c.par, nil
}

// venceEn devuelve cuánto le queda al certificado QUE SE ESTÁ SIRVIENDO, y si se pudo saber.
//
// El segundo valor no es cortesía: un certificado cuyo `NotAfter` no se pudo parsear tiene que
// dejar la serie AUSENTE, no en 0. Un 0 se leería como «vence ahora mismo» y dispararía la alerta
// por el motivo equivocado; la ausencia se lee como lo que es.
func (c *certificadoQueSeRelee) venceEn(ahora time.Time) (time.Duration, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.vence.IsZero() {
		return 0, false
	}
	return c.vence.Sub(ahora), true
}

// recargasFallidas devuelve cuántas relecturas se rechazaron desde que arrancó el proceso.
func (c *certificadoQueSeRelee) recargasFallidas() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.fallos
}

// cargarPar lee el par y saca el `NotAfter` de la hoja.
//
// `tls.LoadX509KeyPair` NO deja la hoja parseada en `Leaf` (la descarta para ahorrar memoria), así
// que se vuelve a parsear el DER que sí quedó. Sin esto no habría de dónde sacar la fecha, y la
// alternativa —leer el archivo otra vez con openssl— mediría el disco y no lo que se sirve, que
// es el defecto que esta pieza entera existe para evitar.
func cargarPar(rutaCert, rutaKey string) (*tls.Certificate, time.Time, error) {
	par, err := tls.LoadX509KeyPair(rutaCert, rutaKey)
	if err != nil {
		return nil, time.Time{}, err
	}
	if len(par.Certificate) == 0 {
		return nil, time.Time{}, fmt.Errorf("el par cargó pero no trae ninguna hoja")
	}
	hoja, err := x509.ParseCertificate(par.Certificate[0])
	if err != nil {
		// El par SIRVE aunque la hoja no se pueda parsear: el apretón de manos no necesita que
		// nosotros la entendamos. Lo que se pierde es la fecha, y eso se dice devolviendo el cero
		// —que `venceEn` traduce a «no sé»— en vez de inventar una.
		logx.Warn("musubi: el certificado TLS se cargó pero su hoja no se pudo parsear; no se va a poder publicar su vencimiento",
			"cert", rutaCert, "error", err)
		return &par, time.Time{}, nil
	}
	par.Leaf = hoja
	return &par, hoja.NotAfter, nil
}

// selloDeLosDos describe el estado en disco del par. Devuelve "" si alguno no se pudo mirar.
//
// SE MIRAN LOS DOS ARCHIVOS y no sólo el `.crt`: una renovación escribe los dos, y si el sello
// dependiera de uno solo, el instante en que el `.crt` ya cambió y el `.key` todavía no quedaría
// indistinguible del estado final.
func selloDeLosDos(rutaCert, rutaKey string) string {
	var partes []string
	for _, r := range []string{rutaCert, rutaKey} {
		fi, err := os.Stat(r)
		if err != nil {
			return ""
		}
		partes = append(partes, fmt.Sprintf("%d:%d", fi.ModTime().UnixNano(), fi.Size()))
	}
	return strings.Join(partes, "|")
}

// ── LAS SERIES ──────────────────────────────────────────────────────────────────────────────

// nombreVenceCertificado dice cuántos segundos le quedan al certificado QUE SE ESTÁ SIRVIENDO.
//
// AUSENTE cuando el cerebro no sirve TLS, y eso es correcto y no un silencio: hoy no hay
// certificado que vencer. El día que la migración se haga, la serie aparece sola.
const nombreVenceCertificado = "musubi_tls_certificate_expiry_seconds"

// nombreRecargasFallidas cuenta las relecturas rechazadas desde que arrancó el proceso.
//
// EXISTE PORQUE EL REINTENTO ES SILENCIOSO POR DISEÑO. Seguir sirviendo el certificado viejo ante
// un par a medio escribir es lo correcto, pero si el par nuevo NUNCA carga —permisos, una key de
// otro certificado, un disco lleno a mitad de la escritura— el cerebro se queda con el viejo hasta
// que vence. Sin este contador, ese camino no tiene ni una señal: el vencimiento avisaría recién
// al final, cuando ya no hay margen para arreglarlo.
const nombreRecargasFallidas = "musubi_tls_certificate_reload_failures"

// renderCertificadoTLS emite las dos series del certificado. No emite NADA si no hay TLS.
func (s *McpServer) renderCertificadoTLS(b *strings.Builder, ahora time.Time) {
	if s.certTLS == nil {
		return
	}
	fmt.Fprintf(b, "# HELP %s Segundos que le quedan al certificado TLS QUE EL CEREBRO ESTÁ SIRVIENDO —no el del archivo en disco: si una renovación dejó el par nuevo sin poder cargarse, esta serie sigue contando el viejo, que es lo que ven los clientes. NEGATIVO = ya venció. AUSENTE cuando el cerebro no sirve TLS, que hoy es el caso.\n# TYPE %s gauge\n",
		nombreVenceCertificado, nombreVenceCertificado)
	if queda, hay := s.certTLS.venceEn(ahora); hay {
		// SÓLO SI SE PUDO MEDIR. Un 0 acá significaría «vence en este instante» y dispararía la
		// alerta por el motivo equivocado; la ausencia dice «no sé», que es lo que pasa.
		fmt.Fprintf(b, "%s %d\n", nombreVenceCertificado, int64(queda.Seconds()))
	}
	fmt.Fprintf(b, "# HELP %s Relecturas del certificado TLS que se rechazaron desde que arrancó el proceso. Seguir con el viejo ante un par a medio escribir es correcto —`tailscale cert` escribe .crt y .key en dos operaciones—, pero si el par nuevo nunca carga el cerebro se queda con el viejo hasta que vence. Un valor que CRECE entre scrapes es eso.\n# TYPE %s counter\n",
		nombreRecargasFallidas, nombreRecargasFallidas)
	fmt.Fprintf(b, "%s %d\n", nombreRecargasFallidas, s.certTLS.recargasFallidas())
}
