package mcp

// vidared.go es el borde: le pregunta al tailnet quién está en línea y lo deja en memoria para
// que el exportador lo publique. El porqué del eje entero vive en internal/fleet/vidared.go.

import (
	"context"
	"encoding/json"
	"os/exec"
	"sync/atomic"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// vidaRedTimeout acota la consulta. `tailscale status` lee un socket local y contesta en
// milisegundos; si tarda más que esto, algo está mal y hacer esperar al barrido de la flota por
// una señal auxiliar es peor que no tenerla.
const vidaRedTimeout = 5 * time.Second

// vidaRedVigencia es cuánto vale una medición antes de considerarse vieja.
//
// Una medición vieja NO se emite. Un cerebro cuyo `tailscale` dejó de contestar seguiría
// publicando el último 1 conocido, y ese 1 diría «la máquina está viva» sobre algo que nadie
// volvió a mirar — el mismo congelamiento que hace que Prometheus no borre las series de una
// máquina muerta, sólo que fabricado por nosotros.
const vidaRedVigencia = 5 * time.Minute

type medicionDeRed struct {
	estado fleet.VidaDeRed
	cuando time.Time
}

// vidaRedDeshabilitada recuerda que no hay `tailscale` en esta máquina, para no intentar un
// fork+exec por tick durante días. Se decide UNA vez por vida del proceso: si alguien instala
// tailscale después, reiniciar el cerebro es barato y el estado intermedio sería peor.
var vidaRedDeshabilitada atomic.Bool

// tailnetPares es el punto de sustitución de las pruebas. Es una variable y no un parámetro
// porque el llamador es el scheduler, que no tiene por qué saber de dónde sale esto.
var tailnetPares = paresDelTailnet

// medirVidaDeRed actualiza, para los devices que se le pasen, si el tailnet los ve.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// SE PREGUNTA UNA SOLA VEZ PARA TODA LA FLOTA, Y SÓLO POR LAS QUE NO ESTÁN LATIENDO
//
// `tailscale status --json` trae TODOS los pares en una llamada, así que el costo no crece con
// la flota — al revés que un ping por máquina, que a 300 máquinas serían 300 forks por tick.
//
// Y sólo se pregunta por las que ya figuran caídas: en una máquina que late, la respuesta no
// cambia ninguna decisión. El trabajo de esta sonda es proporcional al problema, que es la única
// forma en que una señal auxiliar se puede permitir correr siempre.
func (s *McpServer) medirVidaDeRed(ctx context.Context, caidos []fleet.Device, ahora time.Time) {
	if len(caidos) == 0 || vidaRedDeshabilitada.Load() {
		return
	}
	pares, err := tailnetPares(ctx)
	if err != nil {
		// Sin `tailscale` no hay eje: se apaga y se dice UNA vez. Lo que NO se hace es emitir
		// ceros, que dirían «ninguna de estas máquinas está en la red».
		vidaRedDeshabilitada.Store(true)
		logx.Info("flota: no se puede consultar el tailnet, así que no se va a poder distinguir «máquina apagada» de «agente caído»",
			"motivo", err)
		return
	}
	for _, d := range caidos {
		v := fleet.VidaDeRedDe(d.Name, pares)
		if v == fleet.VidaNoMedida {
			// NO MEDIDA BORRA lo que hubiera: dejar la medición anterior publicaría un dato que
			// ya nadie confirmó, con la misma cara que uno fresco.
			s.vidaDeRed.Delete(d.ID)
			continue
		}
		s.vidaDeRed.Store(d.ID, medicionDeRed{estado: v, cuando: ahora})
	}
}

// vidaDeRedDe devuelve lo medido para una máquina, si sigue vigente.
func (s *McpServer) vidaDeRedDe(deviceID string, ahora time.Time) (fleet.VidaDeRed, bool) {
	v, hay := s.vidaDeRed.Load(deviceID)
	if !hay {
		return fleet.VidaNoMedida, false
	}
	m, ok := v.(medicionDeRed)
	if !ok || ahora.Sub(m.cuando) > vidaRedVigencia {
		return fleet.VidaNoMedida, false
	}
	return m.estado, true
}

// olvidarVidaDeRed saca la medición cuando la máquina vuelve a latir: a partir de ahí la
// pregunta no tiene sentido y la serie tiene que desaparecer, no quedarse en 1 para siempre.
func (s *McpServer) olvidarVidaDeRed(deviceID string) { s.vidaDeRed.Delete(deviceID) }

// ── El borde: hablar con `tailscale` ─────────────────────────────────────────────────────────

// paresDelTailnet corre `tailscale status --json` y traduce lo que hace falta.
//
// SE USA `status` Y NO `ping`, y la diferencia importa. `ping` mide el camino de datos en vivo,
// pero cuesta un fork por máquina; `status` trae todos los pares de una y lee un socket local.
// El costo de `status` es que su `Online` viene del plano de control y puede atrasarse unos
// segundos — irrelevante cuando la alerta que lo consume tiene un `for:` de minutos.
//
// Y SE LEE CON UN STRUCT PROPIO, no con un map genérico: la salida de una herramienta externa
// cambia entre versiones, y un campo que desaparece tiene que dar «no medida» —un cero de Go—
// en vez de un pánico o un valor inventado.
func paresDelTailnet(ctx context.Context) ([]fleet.ParDeTailnet, error) {
	bin, err := exec.LookPath("tailscale")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, vidaRedTimeout)
	defer cancel()
	salida, err := exec.CommandContext(ctx, bin, "status", "--json").Output()
	if err != nil {
		return nil, err
	}
	var doc struct {
		Peer map[string]struct {
			HostName string `json:"HostName"`
			DNSName  string `json:"DNSName"`
			Online   bool   `json:"Online"`
		} `json:"Peer"`
	}
	if err := json.Unmarshal(salida, &doc); err != nil {
		return nil, err
	}
	pares := make([]fleet.ParDeTailnet, 0, len(doc.Peer))
	for _, p := range doc.Peer {
		pares = append(pares, fleet.ParDeTailnet{Nombre: p.HostName, DNS: p.DNSName, EnLinea: p.Online})
	}
	return pares, nil
}
