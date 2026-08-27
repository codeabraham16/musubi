package main

// flota.go es el PANEL DE FLOTA visto desde el navegador (S9): una tabla de máquinas con su
// estado, servida por el panel local y proxeada al cerebro central.
//
// POR QUÉ PROXEA EN VEZ DE QUE EL NAVEGADOR PIDA DIRECTO, y son las mismas tres razones que el
// censo de actores (actores.go) y el riel (livestream.go):
//   1. el bearer. Un fetch desde la página necesitaría el token EN la página: en el DOM, en el
//      historial y en cualquier extensión instalada;
//   2. CORS. El cerebro no publica cabeceras para un origen `127.0.0.1:7777`;
//   3. el panel YA tiene la credencial y la conexión.
//
// POR QUÉ NO HAY CAMINO LOCAL. La flota vive en la base del CEREBRO: es ahí donde se enrolan las
// máquinas y adonde latan. La base local de un puesto de trabajo no tiene ni una fila de
// `devices`, así que un camino local devolvería la lista vacía SIEMPRE — indistinguible de «no
// hay máquinas». Por eso el estado se declara en vez de servir un vacío que parece un dato.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// flotaRespuesta es lo que ve el navegador.
//
// `Estado` va SIEMPRE y es lo primero que mira el cliente. Una flota vacía puede significar CINCO
// cosas distintas —no hay central, el central no responde, la credencial no tiene concesiones de
// flota, no hay máquinas enroladas, o de verdad está todo bien y vacío— y las cinco se dibujan
// igual si lo único que viaja es la lista.
type flotaRespuesta struct {
	Estado  string           `json:"estado"`            // vivo | apagado | caido | sin_permiso | vacio
	Detalle string           `json:"detalle,omitempty"` // la frase accionable
	Destino string           `json:"destino,omitempty"`
	Equipos []map[string]any `json:"equipos,omitempty"`
	// SinPermiso dice cuántas máquinas quedaron fuera por la compuerta. Sin este número, una
	// tabla corta se lee como «hay pocas máquinas» en vez de «hay varias que no podés ver».
	SinPermiso int `json:"sin_permiso,omitempty"`
}

// handlerFlota arma la tabla combinando el inventario y las métricas del cerebro.
//
// Se piden LAS DOS porque responden preguntas distintas y la compuerta las gatea distinto:
// `fleet_list` dice qué máquinas hay (y qué podés sobre cada una), `fleet_metrics` dice cómo
// están (y sólo de las que tenés `metrics`). Una máquina puede aparecer en la primera y no en la
// segunda — eso NO es un error, es la compuerta, y la tabla lo dibuja como «sin métricas».
func handlerFlota(relay *relayVivo) http.HandlerFunc {
	cli := &http.Client{Timeout: 20 * time.Second}
	return func(w http.ResponseWriter, r *http.Request) {
		responder := func(f flotaRespuesta) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			_ = json.NewEncoder(w).Encode(f)
		}
		if relay == nil || relay.base == "" || relay.token == "" {
			responder(flotaRespuesta{
				Estado:  "apagado",
				Detalle: "sin enlace al cerebro central: la flota vive ahí, porque es donde se enrolan las máquinas y adonde latan. Configurá MUSUBI_BRAIN_URL y el token.",
			})
			return
		}

		inv, err := llamarToolDelCerebro(r, cli, relay, "musubi_fleet_list", map[string]any{})
		if err != nil {
			responder(flotaRespuesta{Estado: "caido", Destino: relay.host(), Detalle: err.Error()})
			return
		}
		// La compuerta de flota NO se deriva del rol: una credencial sin sección `fleet:` llama
		// bien y no ve nada. Distinguir ese caso del «no hay máquinas» es la razón de ser de
		// `sin_permiso`, y sin él alguien con el YAML mal configurado cree que no tiene flota.
		equipos := aFilas(inv["devices"])
		sinPermiso := aEntero(inv["sin_permiso"])
		if len(equipos) == 0 {
			estado, detalle := "vacio", "no hay máquinas enroladas en este proyecto. Se dan de alta con musubi_fleet_enroll."
			if sinPermiso > 0 {
				estado = "sin_permiso"
				detalle = fmt.Sprintf("hay %d máquina(s) que tu credencial no puede ver. Las capacidades de flota NO se derivan del rol: se declaran en la sección `fleet:` de principals.yaml.", sinPermiso)
			}
			responder(flotaRespuesta{Estado: estado, Destino: relay.host(), Detalle: detalle, SinPermiso: sinPermiso})
			return
		}

		// Las métricas son OPCIONALES: si fallan, la tabla se dibuja igual con el inventario.
		// Perder los números es molesto; perder la lista de máquinas es quedarse a oscuras.
		porNombre := map[string]map[string]any{}
		if met, err := llamarToolDelCerebro(r, cli, relay, "musubi_fleet_metrics", map[string]any{}); err == nil {
			for _, m := range aFilas(met["devices"]) {
				if n, _ := m["name"].(string); n != "" {
					porNombre[n] = m
				}
			}
		}
		for _, e := range equipos {
			n, _ := e["name"].(string)
			if m, hay := porNombre[n]; hay {
				for _, campo := range []string{"cpu_pct", "mem_pct", "disco_pct", "temp_c", "load1", "uptime_seg", "antiguedad_s", "num_cpu"} {
					e[campo] = m[campo]
				}
				e["con_metricas"] = true
			} else {
				// NO es un error: puede ser que la máquina nunca reportó, o que esta credencial
				// no tiene `metrics` sobre ella. La tabla lo dibuja distinto de un cero.
				e["con_metricas"] = false
			}
		}
		responder(flotaRespuesta{Estado: "vivo", Destino: relay.host(), Equipos: equipos, SinPermiso: sinPermiso})
	}
}

// llamarToolDelCerebro invoca una tool MCP en el central y devuelve su resultado ya deserializado.
//
// El panel usa LAS MISMAS TOOLS que cualquier otro cliente: no hay una segunda ruta de datos que
// pueda desincronizarse de la primera, ni un endpoint «para el panel» que se olvide de aplicar la
// compuerta. Lo que la tool no deja ver, el panel no lo ve.
func llamarToolDelCerebro(r *http.Request, cli *http.Client, relay *relayVivo, tool string, args map[string]any) (map[string]any, error) {
	sobre := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": tool, "arguments": args},
	}
	cuerpo, err := json.Marshal(sobre)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, relay.base+"/mcp", strings.NewReader(string(cuerpo)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+relay.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("el cerebro rechazó la credencial del panel (401): revisá el token")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("el cerebro respondió %d", resp.StatusCode)
	}
	// Tope de lectura: el cuerpo viene de otra máquina. Sin límite, un central que responda mal
	// puede hacer crecer este proceso sin techo.
	crudo, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}

	var sobreRPC struct {
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(crudo, &sobreRPC); err != nil {
		return nil, fmt.Errorf("el cerebro respondió algo que no es JSON-RPC: %w", err)
	}
	if sobreRPC.Error != nil {
		return nil, fmt.Errorf("%s: %s", tool, sobreRPC.Error.Message)
	}
	if len(sobreRPC.Result.Content) == 0 {
		return nil, fmt.Errorf("%s: respuesta vacía", tool)
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(sobreRPC.Result.Content[0].Text), &out); err != nil {
		return nil, fmt.Errorf("%s: el contenido no es JSON: %w", tool, err)
	}
	return out, nil
}

func aFilas(v any) []map[string]any {
	crudo, _ := v.([]any)
	out := make([]map[string]any, 0, len(crudo))
	for _, x := range crudo {
		if m, ok := x.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func aEntero(v any) int {
	f, _ := v.(float64)
	return int(f)
}
