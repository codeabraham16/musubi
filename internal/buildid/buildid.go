// Package buildid deriva la identidad de ESTE binario y la deja como un solo objeto, para que
// cualquier extremo del protocolo pueda decir qué build tiene enfrente.
//
// EL AGUJERO QUE TAPA. Hasta acá ningún camino del protocolo publicaba versión de ninguna clase:
// el handshake MCP contestaba el literal "1.0.0" y `musubi_whoami` tampoco la llevaba. Medido el
// 2026-09-07 hay TRES builds hablándose en la malla al mismo tiempo —0.133.0-main en esta PC,
// 0.131.0-flota en el cerebro central y 0.130.0-flota en el daemon del gateway— y cuatro tools
// con JSON distinto entre dos de ellos. El agente veía dos descripciones de la misma tool y nada
// le decía cuál era la vieja. Ese es el síntoma; la causa es que la identidad no viajaba.
//
// LA REGLA DE LA PIEZA: lo que se puede DERIVAR no se tipea. El commit y el estado del árbol los
// graba el toolchain y salen de runtime/debug; el esquema sale de la última migración que este
// binario conoce; el catálogo se cuenta y se hashea sobre el registro real. Lo único escrito a
// mano es Capver, y es a propósito: declara SEMÁNTICA, que ningún dato del binario puede inferir.
//
// LO QUE NO ESTÁ, Y POR QUÉ. No hay `branch` ni `canal`. Go no los graba en el build info, así
// que incluirlos exigiría tipearlos por ldflags —exactamente lo que esta pieza viene a eliminar—
// y un campo tipeado que nadie actualiza miente peor que un campo ausente. Cuando `construir.sh`
// compone la versión, la rama y el commit ya van embebidos ahí (0.133.0-main.55eae4d).
package buildid

import (
	"runtime"
	"runtime/debug"

	"musubi/internal/memory"
)

// Capver es la versión de CAPACIDADES del protocolo entre una máquina y el cerebro central.
// Es un entero monótono y es lo ÚNICO de la identidad que se escribe a mano, porque declara qué
// puede esperar un extremo del otro y eso no se deduce de ningún dato del binario.
//
// Sube cuando cambia el CONTRATO, no cuando cambia la versión del producto: dos builds con
// versiones distintas pueden compartir capver, y ése es justamente el punto —permite desplegar
// sin romper. Un par que no manda capver se lee como 0, que significa «no declara», no «versión
// cero».
//
// Bitácora — una línea por bump, y las viejas no se borran:
//
//	1 · 2026-09-07 · La identidad de build viaja en el handshake MCP. Antes de esto el protocolo
//	    no publicaba versión de ninguna clase, así que 1 es el primer valor con el que un extremo
//	    puede afirmar algo sobre el otro.
//	2 · 2026-09-09 · El latido lleva tres campos nuevos: `servicios_omitidos` (cuántos servicios
//	    no entraron en la lista), `servicios_error` (por qué la máquina no pudo enumerar) y
//	    `token_fuente` (de dónde sacó su credencial). Los tres entraron en `663d5a0` SIN tocar
//	    esta constante, y el costo fue concreto: `davantis-1` y `musubi-server` declaraban los dos
//	    capver=1 hablando contratos distintos, así que el cerebro no podía saber que el `""` de
//	    `servicios_error` de uno significaba «no pude enumerar: no» y el del otro «mi binario no
//	    sabe decírtelo». `MaquinaNoPuedeEnumerar` quedaba VERDE por ignorancia, y como dispara con
//	    `== 1`, el falso 0 es una alerta perdida y no una falsa.
//	    `CapverMin` NO se mueve: un agente que declare 1 sigue siendo atendido, que es para lo que
//	    existe la banda. Lo custodia `TestElCapverSubeCuandoSeMueveElContratoDelLatido`.
const Capver = 2

// CapverConInventarioExplicado es el capver desde el cual un agente SABE contar lo que no mandó y
// SABE decir por qué no pudo enumerar — o sea desde el cual `servicios_omitidos` y
// `servicios_error` significan algo cuando llegan vacíos.
//
// EXISTE PORQUE UN CERO Y UN «NO SÉ DECIRTELO» SE VEÍAN IGUAL. Un agente anterior a 2 no manda
// esos campos, así que el cerebro los lee como 0 y `""` — exactamente lo mismo que un agente
// nuevo que enumeró bien y no recortó nada. El exportador usa esta constante para OMITIR las
// series de esas máquinas en vez de publicar un 0, que es la regla que ya gobierna el resto del
// exportador: un dato ausente no es un cero.
//
// NO ES `Capver` A SECAS, y la diferencia importa el día del próximo bump: cuando Capver pase a 3
// esta constante se queda en 2, porque un agente en 2 SIGUE sabiendo contestar estas dos
// preguntas. Lo que se fija acá es desde cuándo existe la CAPACIDAD, no cuál es el último
// contrato.
const CapverConInventarioExplicado = 2

// CapverMin es el capver MÁS VIEJO que este binario todavía atiende. Junto con Capver forma la
// BANDA `[CapverMin, Capver]`: un par que declara un capver dentro de la banda puede hablar con
// éste; uno afuera, no, y hay que decirle cuál de los dos lados le falta.
//
// POR QUÉ UNA BANDA Y NO UN NÚMERO. Con un solo valor, cualquier cambio de contrato obliga a
// actualizar toda la malla el mismo día — que es exactamente el cutover duro que el piso de
// lectura acaba de sacar del esquema. La banda dice hasta dónde atrás se sigue atendiendo, y por
// eso se puede desplegar el central primero y las máquinas después.
//
// SUBE, NUNCA BAJA, Y SUBIRLA ES RETIRAR SOPORTE. El día que una máquina vieja deje de poder
// hablar es el día que alguien mueve esta constante, y por eso lleva su propia bitácora: el
// cambio tiene que ser una decisión con fecha, no un efecto lateral de haber tocado Capver.
//
// Bitácora — una línea por bump, y las viejas no se borran:
//
//	1 · 2026-09-07 · Nace con la banda. Igual a Capver porque no hay ningún capver anterior que
//	    retirar: 1 es el primer valor que existió.
const CapverMin = 1

// EnLaBanda dice si `capver` cae dentro de lo que ESTE binario atiende.
//
// Un 0 NO está en la banda, y es a propósito: significa «no declara», que es lo que manda un par
// anterior a que el capver existiera. Tratarlo como 1 sería inventarle una capacidad que nadie
// afirmó — la misma regla que gobierna `PuedePreguntar` en el latido, donde el nil se distingue
// del false explícito.
func EnLaBanda(capver int) bool { return capver >= CapverMin && capver <= Capver }

// Identity es lo que un extremo publica sobre sí mismo. Cruza el borde MCP y la red, así que
// lleva tags JSON explícitos: sin ellos Go emitiría "Version"/"Schema" en mayúscula y un receptor
// que parsea en minúscula NO falla, guarda un objeto con todos los campos vacíos y la falla se
// lee como un dato. Ya pasó en este repo con skills.Skill.
type Identity struct {
	// Version es la del producto, inyectada al compilar con -ldflags "-X main.version=<tag>".
	// En un build local queda "dev". Es el único campo que el binario no puede derivar de sí
	// mismo, y por eso entra por parámetro en vez de leerse de una constante de este paquete.
	Version string `json:"version"`

	// Commit y Dirty los graba el toolchain al compilar. Un binario hecho con `go run`, o el
	// binario de un test, no los trae: quedan vacíos, que es la respuesta honesta —«no se
	// sabe»— y no un valor inventado que después alguien cite como si fuera medición.
	Commit string `json:"commit"`
	Dirty  bool   `json:"dirty"`

	// Schema es la migración más alta que este binario conoce. Es el número que decide si puede
	// abrir una base sin abortar, así que es el dato duro de la compatibilidad.
	Schema int `json:"schema"`

	Capver int `json:"capver"`

	// ToolsCount y CatalogSHA describen el catálogo que este binario EXPONDRÍA en tools/list,
	// respetando la misma regla de visibilidad que tools/list aplica. Sirven para detectar deriva
	// entre dos extremos sin bajarse los dos catálogos y compararlos a mano.
	ToolsCount int    `json:"tools_count"`
	CatalogSHA string `json:"catalog_sha256"`

	Go string `json:"go"`
}

// Derive arma la identidad. El catálogo entra por parámetro y no se calcula acá a propósito:
// vive en el paquete mcp, y que buildid lo importara crearía un ciclo —mcp necesita publicar
// una Identity en el handshake—. Los dos llamadores lo obtienen del mismo lugar (mcp), así que
// no hay dos cuentas distintas del mismo catálogo.
func Derive(version string, toolsCount int, catalogSHA string) Identity {
	id := Identity{
		Version:    version,
		Schema:     memory.EsquemaEsperado(),
		Capver:     Capver,
		ToolsCount: toolsCount,
		CatalogSHA: catalogSHA,
		Go:         runtime.Version(),
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, ajuste := range bi.Settings {
			switch ajuste.Key {
			case "vcs.revision":
				id.Commit = ajuste.Value
			case "vcs.modified":
				id.Dirty = ajuste.Value == "true"
			}
		}
	}
	return id
}
