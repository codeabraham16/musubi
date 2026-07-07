// Package provision implementa `musubi provision`: llevar una máquina "desde 0" a estar
// UNIDA al cerebro central de Musubi (memoria híbrida local+central). El corazón es un
// preflight de red VPN-agnóstico que diagnostica el entorno ANTES de mutar nada, de modo
// que cualquier máquina con un bloqueo tipo-VPN lo detecte y reciba guía accionable en vez
// de fallar en silencio. Esta es la Fase 1 (core: unir al cerebro) del track PC-provisioning;
// porta a Go la lógica probada en los scripts deploy/connect-brain-* (PR #120).
package provision

import "fmt"

// NetworkMode clasifica el entorno de red desde la perspectiva del PROCESO musubi (que es,
// además, el sync-client del cerebro híbrido). Se deduce del cruce de dos alcances: a un
// destino público de control y al cerebro en el tailnet. Es VPN-agnóstico: describe el
// síntoma observado, no un producto.
type NetworkMode int

const (
	// ModeClean — público y tailnet responden: red sin bloqueos. Sigue.
	ModeClean NetworkMode = iota
	// ModeSplitExcluded — tailnet OK, público NO: este proceso va DIRECTO y solo ve la malla
	// (típico de una VPN con split-tunnel que excluye a musubi del túnel). El cerebro es
	// alcanzable → es un modo ESPERADO, no un error. (Es el caso de kernelos-pc.)
	ModeSplitExcluded
	// ModeTunneled — público OK, tailnet NO: este proceso está atrapado en el túnel de una VPN
	// que tapa el rango del tailnet. Falta excluir el runtime o abrir 100.64.0.0/10. Frena el
	// self-check y reporta el paso faltante. (El riesgo que podría aparecer en otra PC.)
	ModeTunneled
	// ModeIsolated — ni público ni tailnet: sin conectividad útil.
	ModeIsolated
)

// tailnetCIDR es el rango CGNAT de Tailscale que hay que dejar pasar.
const tailnetCIDR = "100.64.0.0/10"

func (m NetworkMode) String() string {
	switch m {
	case ModeClean:
		return "Clean"
	case ModeSplitExcluded:
		return "SplitExcluded"
	case ModeTunneled:
		return "Tunneled"
	case ModeIsolated:
		return "Isolated"
	default:
		return "Unknown"
	}
}

// Reachability es el resultado crudo del sondeo: qué alcanzó el proceso musubi.
type Reachability struct {
	PublicOK  bool // ¿respondió el destino público de control?
	TailnetOK bool // ¿respondió el cerebro en el tailnet?
}

// Classify mapea el cruce público×tailnet al modo de red. Función PURA (el núcleo testeable
// del preflight): no toca red ni estado.
func Classify(r Reachability) NetworkMode {
	switch {
	case r.PublicOK && r.TailnetOK:
		return ModeClean
	case !r.PublicOK && r.TailnetOK:
		return ModeSplitExcluded
	case r.PublicOK && !r.TailnetOK:
		return ModeTunneled
	default:
		return ModeIsolated
	}
}

// ok indica si el modo permite continuar el provisioning hasta el self-check (el cerebro es
// alcanzable). Clean y SplitExcluded siguen; Tunneled e Isolated frenan con guía.
func (m NetworkMode) ok() bool { return m == ModeClean || m == ModeSplitExcluded }

// Guidance devuelve una explicación accionable EN PROSA para el modo detectado, sin nombrar
// ningún producto de VPN concreto. brain es el destino del cerebro (para citarlo en la guía).
func Guidance(m NetworkMode, brain string) string {
	switch m {
	case ModeClean:
		return "Red limpia: la malla y el internet público responden."
	case ModeSplitExcluded:
		return "Este proceso llega a la malla pero no al internet público — típico de una VPN " +
			"con split-tunnel que deja a musubi FUERA del túnel. El cerebro es alcanzable, así que " +
			"es el modo esperado en esta máquina; sigo."
	case ModeTunneled:
		return fmt.Sprintf("Este proceso llega al internet público pero NO a la malla (%s) — una VPN "+
			"está enrutando este proceso por el túnel y tapando el tailnet. Falta abrir el rango del "+
			"tailnet %s o excluir el runtime del túnel. Revisá el paso de firewall de abajo; no corro "+
			"el self-check hasta resolverlo.", brain, tailnetCIDR)
	case ModeIsolated:
		return fmt.Sprintf("No hay alcance ni a la malla (%s) ni al internet público. Verificá la "+
			"conectividad de red y que Tailscale esté conectado.", brain)
	default:
		return "Modo de red desconocido."
	}
}

// Preflight ejecuta las sondas vía el Prober y clasifica el resultado. Devuelve la
// Reachability cruda y el modo, para que el orquestador arme el Report.
func Preflight(p Prober, brain string) (Reachability, NetworkMode) {
	r := Reachability{
		PublicOK:  p.PublicReachable(),
		TailnetOK: p.TailnetReachable(brain),
	}
	return r, Classify(r)
}
