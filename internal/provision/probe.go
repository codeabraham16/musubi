package provision

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"musubi/internal/cerebro"
)

const (
	// publicControlAddr es un destino público ESTABLE por IP literal (Cloudflare, anycast),
	// para sondear "¿hay internet público?" sin depender del DNS (que una VPN puede pisar).
	publicControlAddr = "1.1.1.1:443"

	dialTimeout = 3 * time.Second
	httpTimeout = 6 * time.Second
)

// netProber sondea desde el PROPIO proceso musubi (que es el sync-client): lo que él alcanza
// es lo que importa para el cerebro. Un simple TCP connect basta para clasificar el modo.
type netProber struct{}

func (netProber) PublicReachable() bool { return dialOK(publicControlAddr) }
func (netProber) TailnetReachable(brain string) bool {
	_, hostPort, ok := direccionDelCerebro(brain)
	if !ok {
		return false
	}
	return dialOK(hostPort)
}

func dialOK(addr string) bool {
	c, err := net.DialTimeout("tcp", addr, dialTimeout)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// direccionDelCerebro separa las dos cosas que `Options.Brain` tiene que poder ser, porque hasta
// hoy sólo podía ser una.
//
// EL ESQUEMA ESTABA ESCRITO A MÁQUINA en CUATRO sitios, así que este paquete no podía expresar
// HTTPS aunque el cerebro lo sirviera: `Reach`, `Auth` y `wireMCPJSON` con la forma
// `"http://" + brain`, y `ensureSyncConfig` con `http://%s` adentro de un formato más grande.
//
// EL CUARTO SE ESCAPÓ DE LA PRIMERA PASADA, y vale más que el arreglo: se buscó por FORMA
// —un grep del literal `"http://"`— y el que faltaba no tiene esa forma. Enumerar las maneras
// de escribir algo no converge nunca; la pregunta que sí converge es «¿quién CONSUME la
// dirección del cerebro?», y ésa se contesta siguiendo los llamadores de Options.Brain. No es un ServerName que falta: es un
// esquema que no se puede decir. El día que el cerebro deje de escuchar en claro, el self-check
// falla con «no se alcanzó /readyz» —un error que acusa a la red— y el `.mcp.json` que genera
// `provision` sale con una URL en claro clavada adentro, en cada máquina nueva.
//
// `host:port` pelado sigue significando lo de siempre (http://), así que ningún llamador de hoy
// cambia de comportamiento. Lo que se agrega es poder escribir `https://host:port`.
//
// Devuelve la base CON esquema —para todo lo que hable HTTP—, el `host:port` pelado, que es lo
// único que entiende un dial TCP, y si la entrada era USABLE.
//
// EL TERCER VALOR NO ES DECORACIÓN. Sin él, un esquema que no es http(s) se devolvía tal cual y
// los llamadores lo PERSISTÍAN: `wireMCPJSON` lo escribía en el .mcp.json y `ensureSyncConfig`
// en el config.yaml, así que el error aparecía mucho después, en otra máquina y sin la causa a
// la vista. Un valor que no se puede usar se rechaza ANTES de escribirlo en disco.
func direccionDelCerebro(brain string) (base, hostPort string, ok bool) {
	brain = strings.TrimSpace(brain)
	brain = strings.TrimRight(brain, "/")
	if brain == "" {
		return "", "", false
	}
	if i := strings.Index(brain, "://"); i >= 0 {
		esquema := strings.ToLower(brain[:i])
		resto := brain[i+3:]
		if resto == "" {
			return "", "", false
		}
		// Un esquema que no es http(s) no se adivina ni se corrige: se rechaza.
		if esquema != "http" && esquema != "https" {
			return "", "", false
		}
		return esquema + "://" + resto, resto, true
	}
	return "http://" + brain, brain, true
}

// elCertificadoNoVaAServirParaUnaIP avisa de un pie de plomo que no es un error de este paquete y
// que igual conviene decir en voz alta: una base `https://` contra una IP PELADA no la puede
// validar ningún cliente que no declare el ServerName a mano, porque el certificado del tailnet
// lleva sólo el nombre como SAN y ninguna IP. El .mcp.json que escribe `provision` lo consume el
// host MCP, que NO declara ServerName — así que ahí eso no anda, y el self-check de este paquete
// tampoco lo ve, porque él sí sale por otro camino.
func elCertificadoNoVaAServirParaUnaIP(base string) bool {
	if !strings.HasPrefix(strings.ToLower(base), "https://") {
		return false
	}
	hostPort := base[len("https://"):]
	host := hostPort
	if i := strings.LastIndex(hostPort, ":"); i >= 0 && !strings.Contains(hostPort[i+1:], "]") {
		host = hostPort[:i]
	}
	host = strings.Trim(host, "[]")
	return net.ParseIP(host) != nil
}

// httpVerifier hace el self-check real contra el cerebro con el stack HTTP de musubi (el mismo
// que usa el sync saliente): reach por /readyz y auth por tools/list.
//
// HASTA EL 2026-09-20 ESE COMENTARIO ERA FALSO: los dos clientes se armaban a mano y con el
// esquema clavado, así que no era «el mismo stack» ni por el cliente ni por la URL. Ahora los
// dos salen de internal/cerebro, igual que el del sync.
type httpVerifier struct{}

func (httpVerifier) Reach(brain string) (bool, string) {
	base, _, ok := direccionDelCerebro(brain)
	if !ok {
		return false, "la dirección del cerebro no es usable: " + brain
	}
	client := cerebro.Cliente(cerebro.NombreTLS(), httpTimeout, nil)
	resp, err := client.Get(base + "/readyz")
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
	base, _, ok := direccionDelCerebro(brain)
	if !ok {
		return false, "la dirección del cerebro no es usable: " + brain
	}
	client := cerebro.Cliente(cerebro.NombreTLS(), httpTimeout, nil)
	req, err := http.NewRequest(http.MethodPost, base+"/mcp",
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
