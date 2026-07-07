package provision

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultBrain es el cerebro central por defecto (IP del tailnet, NO MagicDNS: con una
	// VPN activa el DNS de la malla no resuelve).
	DefaultBrain = "100.79.126.62:7717"
	// publicControlAddr es un destino público ESTABLE por IP literal (Cloudflare, anycast),
	// para sondear "¿hay internet público?" sin depender del DNS (que una VPN puede pisar).
	publicControlAddr = "1.1.1.1:443"

	dialTimeout = 3 * time.Second
	httpTimeout = 6 * time.Second
)

// netProber sondea desde el PROPIO proceso musubi (que es el sync-client): lo que él alcanza
// es lo que importa para el cerebro. Un simple TCP connect basta para clasificar el modo.
type netProber struct{}

func (netProber) PublicReachable() bool              { return dialOK(publicControlAddr) }
func (netProber) TailnetReachable(brain string) bool { return dialOK(brain) }

func dialOK(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// httpVerifier hace el self-check real contra el cerebro con el stack HTTP de musubi (el mismo
// que usa el sync saliente): reach por /readyz y auth por tools/list.
type httpVerifier struct{}

func (httpVerifier) Reach(brain string) (bool, string) {
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Get("http://" + brain + "/readyz")
	if err != nil {
		return false, "no se alcanzó /readyz: " + err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
	if resp.StatusCode == http.StatusOK {
		return true, "cerebro alcanzable (/readyz → " + strings.TrimSpace(string(body)) + ")"
	}
	return false, fmt.Sprintf("/readyz devolvió %d", resp.StatusCode)
}

func (httpVerifier) Auth(brain, token string) (bool, string) {
	client := &http.Client{Timeout: httpTimeout}
	req, err := http.NewRequest(http.MethodPost, "http://"+brain+"/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if err != nil {
		return false, err.Error()
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return false, "auth falló: " + err.Error()
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusOK && strings.Contains(string(body), `"tools"`) {
		return true, "autenticación OK (el cerebro devuelve el catálogo de tools)"
	}
	return false, fmt.Sprintf("auth rechazada por el cerebro (HTTP %d)", resp.StatusCode)
}
