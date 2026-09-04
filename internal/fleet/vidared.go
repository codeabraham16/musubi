package fleet

// vidared.go es LO QUE EL CEREBRO PUEDE DECIR DE UNA MÁQUINA SIN EL AGENTE.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL PROBLEMA: `up == 0` DICE DOS COSAS DISTINTAS Y MANDA AL LUGAR EQUIVOCADO
//
// `musubi_fleet_device_up` se deriva del último latido. Vale 0 cuando la máquina está apagada
// **y también** cuando la máquina está perfecta y el agente no está corriendo. Son dos problemas
// distintos, se arreglan en dos lugares distintos, y hasta hoy `MaquinaCaida` disparaba igual
// para los dos: la alerta manda a mirar el hardware cuando lo que hay que mirar es una tarea
// programada.
//
// No es hipotético y ya costó caro dos veces. `gio` figuró caída TRES DÍAS mientras respondía al
// ping en 145 ms: el agente arranca AL INICIAR SESIÓN y nadie había iniciado sesión. Y el
// 2026-09-04, con `davantis-1` en medio de una investigación de quince cortes de energía, la
// máquina estaba ENCENDIDA —`tailscale ping` contestaba en 55 ms— y lo único muerto era el
// agente. Sin este eje, esa hora se gasta mirando la fuente de alimentación.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// TRES ESTADOS, Y EL TERCERO ES EL QUE HACE QUE ESTO SIRVA
//
// «No pude preguntar» NO es «no está». Un cerebro sin acceso al tailnet, una máquina que no está
// en el tailnet, o dos pares que dicen llamarse igual, todos terminan en NO MEDIDA — y una
// máquina no medida NO emite serie, en vez de emitir un 0.
//
// Es la misma regla que el resto de este dominio sostiene para la telemetría: un cero inventado
// es indistinguible de uno medido, y acá el cero significaría «la máquina no está», que es
// exactamente la conclusión que mandaría a alguien a revisar el hardware de una máquina sana.

import "strings"

// VidaDeRed es lo que se pudo averiguar de una máquina por fuera del agente.
type VidaDeRed int

const (
	// VidaNoMedida: no se pudo preguntar, o la respuesta fue ambigua. NO SE EMITE SERIE.
	VidaNoMedida VidaDeRed = iota
	// VidaAusente: se preguntó y la máquina no está en la red.
	VidaAusente
	// VidaPresente: se preguntó y la máquina está en la red. Si además `up == 0`, lo que está
	// caído es el AGENTE y no la máquina.
	VidaPresente
)

// ParDeTailnet es lo mínimo que hace falta de un par del tailnet para resolver esto.
//
// Es un tipo propio y no la estructura que devuelve `tailscale status --json` a propósito: el
// dominio no tiene por qué saber qué forma tiene la salida de una herramienta externa, y esa
// forma cambia entre versiones. La traducción vive en el borde, que es donde se paga un cambio.
type ParDeTailnet struct {
	Nombre  string // HostName
	DNS     string // DNSName, con o sin punto final
	EnLinea bool
}

// VidaDeRedDe resuelve el estado de UNA máquina contra la lista de pares del tailnet.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// CERO COINCIDENCIAS Y DOS COINCIDENCIAS DAN LO MISMO: NO MEDIDA
//
// Con CERO, la máquina puede estar encendida en otra red: afirmar que «no está» sería inventar.
// Con DOS, alguien tiene dos máquinas que dicen llamarse igual, y elegir una es una moneda al
// aire — la misma decisión que A13 tomó para el id de pantalla, y por el mismo motivo: acertar
// por casualidad es peor que no contestar, porque enseña a confiar.
//
// El nombre del par lo REPORTAN las máquinas al tailnet, así que es entrada no confiable. Lo
// único que se hace con ella es comparar; nunca se interpola en un comando ni en una consulta.
func VidaDeRedDe(nombreDevice string, pares []ParDeTailnet) VidaDeRed {
	buscado := normalizarNombreDeRed(nombreDevice)
	if buscado == "" {
		return VidaNoMedida
	}
	encontrado := VidaNoMedida
	n := 0
	for _, p := range pares {
		if normalizarNombreDeRed(p.Nombre) != buscado && primerLabel(p.DNS) != buscado {
			continue
		}
		n++
		if n > 1 {
			return VidaNoMedida // ambiguo: dos pares dicen ser la misma máquina
		}
		if p.EnLinea {
			encontrado = VidaPresente
		} else {
			encontrado = VidaAusente
		}
	}
	return encontrado
}

// normalizarNombreDeRed empareja lo que el tailnet llama distinto que Musubi: mayúsculas y
// espacios al borde. NO toca los guiones ni los puntos internos — «srv-01» y «srv01» son dos
// máquinas, y colapsarlas sería fabricar la ambigüedad que la función de arriba rechaza.
func normalizarNombreDeRed(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// primerLabel se queda con la primera etiqueta de un DNS: `pc.tail1234.ts.net.` → `pc`.
func primerLabel(dns string) string {
	d := normalizarNombreDeRed(dns)
	if i := strings.IndexByte(d, '.'); i >= 0 {
		d = d[:i]
	}
	return d
}
