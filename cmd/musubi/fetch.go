package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"
)

// maxFetchBytes acota la descarga (defensa ante una URL que apunte a algo enorme).
const maxFetchBytes = 512 << 20

// runFetch baja una URL del TAILNET (o loopback) y escribe sus bytes crudos a stdout.
// Es el transporte del auto-update del cuerpo (musubi-body): bajo redes particionadas
// por app (NordVPN split-tunnel) el cuerpo NO alcanza el tailnet por HTTP, pero `musubi`
// SÍ (está excluido), así que el cuerpo delega la descarga acá y lee stdout.
//
// Anti-SSRF: NO es un descargador de propósito general — sólo permite destinos en el
// rango tailnet (100.64.0.0/10) o loopback. Todo lo demás se rechaza.
func runFetch(args []string) {
	if len(args) < 1 || args[0] == "" {
		fmt.Fprintln(os.Stderr, "uso: musubi fetch <url>")
		os.Exit(1)
	}
	if err := fetch(args[0], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "fetch:", err)
		os.Exit(1)
	}
}

// fetch valida el destino, lo baja y lo escribe en out. Separado de runFetch para
// poder testearlo (out inyectable, sin tocar os.Stdout ni os.Exit).
func fetch(raw string, out io.Writer) error {
	if err := allowedFetchURL(raw); err != nil {
		return err
	}
	// Anti-SSRF en redirects: allowedFetchURL sólo valida la URL INICIAL. Sin esto un host
	// del tailnet podría redirigir (3xx) a una IP pública/link-local (metadata de cloud, LAN)
	// y `fetch` la seguiría, anulando la garantía "sólo tailnet". Revalidamos cada salto.
	client := &http.Client{
		Timeout: 120 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("demasiados redirects")
			}
			return allowedFetchURL(req.URL.String())
		},
	}
	resp, err := client.Get(raw)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if _, err := io.Copy(out, io.LimitReader(resp.Body, maxFetchBytes)); err != nil {
		return fmt.Errorf("descargando: %w", err)
	}
	return nil
}

// allowedFetchURL restringe fetch al tailnet/loopback (anti-SSRF). El host debe ser una
// IP en 100.64.0.0/10 (CGNAT, el rango del tailnet) o loopback. Se rechazan hostnames:
// bajo NordVPN MagicDNS no resuelve y la malla se direcciona por IP, así que exigir IP
// además cierra el vector de "hostname que resuelve a algo interno".
func allowedFetchURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("url inválida: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("esquema no permitido: %q", u.Scheme)
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("el host debe ser una IP del tailnet o loopback (no %q)", host)
	}
	if ip.IsLoopback() {
		return nil
	}
	_, tailnet, _ := net.ParseCIDR("100.64.0.0/10")
	if !tailnet.Contains(ip) {
		return fmt.Errorf("destino fuera del tailnet: %s", ip)
	}
	return nil
}
