package fleet

// servicio.go es QUÉ CORRE ADENTRO de una máquina de la flota: una unit de systemd, un servicio
// de Windows, un contenedor. Dominio puro, como muestra.go — describe la cosa y su
// serialización, y no sabe leerla de ningún supervisor. La persistencia vive en internal/memory
// y el transporte en internal/mcp.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// «SERVICIO» YA SIGNIFICA OTRA COSA EN ESTE MISMO SERVIDOR, Y HAY QUE DECIRLO ACÁ
//
// La sección «Flota» del CRM inventaría BOTS, PUENTES Y SERVICIOS PUBLICADOS A MANO: alguien
// corre `flota publicar` y la fila aparece. Ésta es la otra flota: MÁQUINAS QUE SE MIDEN SOLAS, y
// un fleet.Servicio es una unidad que corre EN una de esas máquinas y hereda su tenencia.
//
// Comparten el nombre y no comparten nada más (registro B17). Sin este párrafo, alguien va a
// mirar una creyendo que es la otra — y la primera vez que eso pase va a ser mientras busca por
// qué algo dejó de andar.
// ────────────────────────────────────────────────────────────────────────────────────────────
//
// EL PRINCIPIO QUE GOBIERNA ESTE ARCHIVO ES EL MISMO QUE EL DE LA MUESTRA: lo que no se pudo
// medir viaja como nil, nunca como cero, y `desconocido` NO es `detenido`. Una máquina que no
// pudo enumerar sus servicios no está afirmando que el postgres esté caído.

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// EstadoServicio son los cuatro estados posibles, y el cuarto es el que importa.
//
// `desconocido` NO es `detenido`. Una máquina que no pudo enumerar sus servicios —el agente
// arrancó a medias, systemd no contestó, el usuario no tiene permiso— no está diciendo que el
// postgres esté caído. Confundirlos es el mismo modo de falla que el resto del track viene
// evitando desde el primer slice (el 0 % de CPU que se cree y no se arregla), y es el que
// despierta a alguien a las 4 de la mañana por nada.
type EstadoServicio string

const (
	EstadoCorriendo   EstadoServicio = "corriendo"
	EstadoDetenido    EstadoServicio = "detenido"
	EstadoFallado     EstadoServicio = "fallado"
	EstadoDesconocido EstadoServicio = "desconocido"
)

// Techos. Todos existen por la misma razón que MuestraMaxBytes: lo que entra por la puerta del
// dispositivo viene de la superficie MÁS expuesta de la flota, y un campo sin acotar es un DoS
// con forma de inventario.
const (
	// SaludMaxBytes es el tope del JSON de UNA salud.
	SaludMaxBytes = 2 << 10
	// ServiciosPorLatido es cuántos servicios acepta un latido. 64 alcanza para cualquier host
	// real (un servidor cargado tiene ~40 units interesantes) y acota el cuerpo.
	ServiciosPorLatido = 64
	// InventarioCada es cada cuánto el agente REENVÍA el inventario aunque no haya cambiado.
	//
	// ────────────────────────────────────────────────────────────────────────────────────────
	// VIVE ACÁ, EN EL DOMINIO, Y NO EN EL AGENTE, PORQUE EL CEREBRO LA NECESITA IGUAL
	//
	// El agente manda el inventario cuando CAMBIÓ, más este piso. El cerebro decide si un
	// servicio está FRESCO. Si cada lado elige su número por su cuenta, se separan — y se
	// separaron: la primera versión usaba el umbral de «en línea» del dispositivo (90 s) contra
	// un piso de 5 minutos, así que TODO servicio se leía viejo para siempre. Un `fresco: false`
	// permanente no es una alarma, es ruido de fondo que enseña a ignorar la columna.
	//
	// Las dos puntas salen de acá, y hay una prueba que exige que el umbral del cerebro sea
	// MAYOR que este piso.
	InventarioCada = 5 * time.Minute
	// UmbralInventario es cuánto aguanta el cerebro sin noticias antes de marcar un servicio
	// como no fresco. Es DOS veces el piso a propósito: un reenvío perdido —un latido que no
	// llegó, un reinicio del agente— no puede marcar la flota entera como vieja.
	UmbralInventario = 2 * InventarioCada

	// NombreServicioMax y DetalleServicioMax se cuentan en RUNAS, no en bytes: el nombre de una
	// unit y el `Result=` de systemd pueden traer acentos, y cortar a la mitad de un carácter
	// multibyte deja basura en una columna que después se dibuja.
	NombreServicioMax  = 128
	DetalleServicioMax = 200
)

var (
	ErrServicioAjeno     = errors.New("ese servicio no es de esta máquina")
	ErrServicioDuplicado = errors.New("ya existe un servicio con ese nombre en esa máquina")
	ErrClaseDesconocida  = errors.New("clase de servicio desconocida")
	ErrSinNombreServicio = errors.New("el servicio necesita un nombre")
)

// clasesConocidas es el enum ACOTADO de qué supervisor lo corre. Vacío es legítimo y significa
// «no se declaró»: un Tier B que alguien inventaría a mano no siempre sabe decirlo.
//
// Es un enum y no texto libre por la misma razón que las tags SÍ son texto libre: la clase va a
// terminar agrupando y filtrando, y un campo que agrupa con valores libres agrupa mal el día que
// alguien escriba «Systemd» con mayúscula.
// PODMAN FALTABA, Y SE VIO AL DESPLEGAR. El servidor real corre 18 contenedores con podman
// rootless; el agente los enumeraba bien y el cerebro les vaciaba la clase en silencio, porque
// `podman` no estaba en este mapa. El resultado era peor que un error: 18 filas correctas con
// una columna en blanco, indistinguibles de las que de verdad no saben decir quién las corre.
//
// Se agrega también `launchd`, que el enumerador de macOS ya emite.
//
// Y NO se agrega nada más. La primera versión de este arreglo metió `kubernetes` por las dudas
// —«si un día alguien lo reporta»— y dos pruebas existentes se pusieron rojas, porque usaban esa
// clase justamente como su ejemplo de una desconocida. Tenían razón: el enum se ensancha cuando
// un enumerador REAL emite el valor, no antes. Una clase que nadie produce es una entrada que
// sólo sirve para que el enum deje de rechazar algo.
var clasesConocidas = map[string]bool{
	"": true, "systemd": true, "windows": true,
	"docker": true, "podman": true, "launchd": true,
}

// SaludServicio es el PRESENTE de un servicio, serializado a JSON en UNA columna. Mismo idioma
// del «no sé» que fleet.Muestra: lo que no se pudo medir es un puntero nil, nunca un cero.
type SaludServicio struct {
	Tomada time.Time      `json:"tomada"`
	Estado EstadoServicio `json:"estado"`
	// Desde es cuándo entró en ese estado. nil = la máquina no lo sabe: systemd lo da
	// (ActiveEnterTimestamp), el SCM de Windows no siempre.
	Desde *time.Time `json:"desde,omitempty"`
	// PID es puntero porque un servicio detenido NO tiene pid, y un 0 se lee como «pid 0», que
	// además existe.
	PID *int `json:"pid"`
	// Reinicios es el contador del supervisor (NRestarts en systemd). Es lo que distingue «anda»
	// de «anda a los tumbos», y por eso vale la pena aunque sea opcional: un servicio que se
	// reinició 400 veces está corriendo en este instante y no está sano.
	Reinicios *int `json:"reinicios"`
	// Detalle es texto de la máquina (el `Result=` de systemd, el mensaje del SCM). ENTRADA NO
	// CONFIABLE: se acota a DetalleServicioMax runas al recibir y se escapa al dibujar.
	Detalle string `json:"detalle,omitempty"`
}

// Servicio es el servicio tal como lo conoce el registro.
//
// Lo que NO tiene, y es deliberado: NO hay campo `sano` ni `activo` ni `up`. El frescor se DERIVA
// al leer (ver Fresco), por la misma razón que `online` se deriva en Device: un booleano guardado
// se queda en true para siempre cuando la cosa muere de golpe, que es justo cuando querés saber
// que se cayó.
type Servicio struct {
	ID        string // lo asigna el CEREBRO, igual que el de un Device
	Nombre    string // unit / nombre de servicio; único DENTRO de su máquina
	ProjectID string // sale del DEVICE, nunca del cuerpo del pedido
	DeviceID  string
	Clase     string // systemd | windows | docker | "" (no declarada)

	Registrado    time.Time
	UltimoReporte time.Time // cero = nunca reportó: DECLARADO y todavía sin muestras
	Salud         *SaludServicio
	Revocado      bool

	// Declarado es QUIÉN puso esta fila acá: true = una persona, con
	// musubi_fleet_service_declare; false = la máquina, enumerándose en un latido.
	//
	// ────────────────────────────────────────────────────────────────────────────────────────
	// NO ES DECORATIVO: ES LO QUE DECIDE QUIÉN PUEDE BORRARLA
	//
	// La poda por ausencia da de baja lo que la máquina dejó de reportar. Sin este campo, esa
	// poda se lleva puesto lo que se declaró A MANO — que por definición es lo que ninguna
	// máquina va a reportar nunca: el bot de un Tier B, un puente, un contenedor en un host que
	// no enumera. La tool existe literalmente para eso, así que el primer latido con un
	// enumerador de systemd vaciaría, de una, todo lo declarado en toda la flota. Y sin vuelta
	// atrás visible: la fila revocada sigue ocupando el único (project_id, device_id, name).
	//
	// Que sea del ROW y no de la última escritura es a propósito. Un servicio declarado a mano
	// que después la máquina SÍ reporta sigue siendo de quien lo declaró: su salud se actualiza
	// como la de cualquier otro, pero quien lo saca del inventario es una persona, nunca un
	// silencio. «Dejó de aparecer en el latido» es una afirmación que sólo tiene sentido sobre lo
	// que el latido trajo alguna vez por su cuenta.
	// ────────────────────────────────────────────────────────────────────────────────────────
	Declarado bool
}

// ReporteServicio es lo que una MÁQUINA reporta de un servicio suyo.
//
// No trae ni id ni project_id ni device: los tres salen del token y de la fila del device. Que no
// tenga POR DÓNDE pasarlos es la garantía (B4/D5), no la disciplina — igual que cuerpoLatido.
//
// Los tags están en CASTELLANO como todo el cuerpo del latido, y eso no es sólo estilo:
// agent_test.go barre el JSON entero buscando la subcadena `"name"`, así que un campo `nombre`
// pasa y uno `name` no.
type ReporteServicio struct {
	Nombre string        `json:"nombre"`
	Clase  string        `json:"clase,omitempty"`
	Salud  SaludServicio `json:"salud"`
}

// Valida chequea que una salud REPORTADA sea creíble antes de guardarla. Mismo trato que
// Muestra.Valida: el agente es un cliente y su reporte es entrada no confiable aunque su
// credencial sea válida.
//
// Un reporte inválido se SALTEA —no tumba a los demás ni al latido—, así que las reglas de acá
// son baratas de a una y hay que elegirlas con el mismo cuidado: cada aserción de más es un
// servicio que se deja de ver.
func (s SaludServicio) Valida() error {
	switch s.Estado {
	case EstadoCorriendo, EstadoDetenido, EstadoFallado, EstadoDesconocido:
	default:
		return fmt.Errorf("estado de servicio desconocido: %q (esperaba corriendo, detenido, fallado o desconocido)", s.Estado)
	}
	if s.Tomada.IsZero() {
		return errors.New("la salud no dice cuándo se tomó: sin `tomada` no hay forma de saber si es de hace un minuto o de hace una semana")
	}
	if s.PID != nil && *s.PID <= 0 {
		return fmt.Errorf("pid inválido: %d (un servicio sin pid manda null, no 0)", *s.PID)
	}
	if s.Reinicios != nil && *s.Reinicios < 0 {
		return fmt.Errorf("reinicios negativo: %d", *s.Reinicios)
	}
	return nil
}

// Serializar lleva la salud a JSON para guardarla en la fila del servicio.
func (s SaludServicio) Serializar() (string, error) {
	b, err := json.Marshal(s)
	if err != nil {
		return "", fmt.Errorf("no se pudo serializar la salud del servicio: %w", err)
	}
	return string(b), nil
}

// SaludDesdeTexto revierte Serializar. Un texto vacío devuelve (nil, nil): «este servicio está
// declarado y todavía nadie lo midió», que NO es un error sino el estado inicial de todo servicio
// dado de alta a mano. Es la misma asimetría que MuestraDesdeTexto.
func SaludDesdeTexto(txt string) (*SaludServicio, error) {
	if strings.TrimSpace(txt) == "" {
		return nil, nil
	}
	var s SaludServicio
	if err := json.Unmarshal([]byte(txt), &s); err != nil {
		return nil, fmt.Errorf("salud de servicio guardada ilegible: %w", err)
	}
	return &s, nil
}

// NombreDeServicioValido acota el nombre de una unit. 1..NombreServicioMax runas y NINGÚN
// carácter de control: el nombre lo reporta la máquina y termina en una tabla HTML y en un
// mensaje de error, y un `\n` en el medio parte una línea de log en dos.
func NombreDeServicioValido(n string) bool {
	n = strings.TrimSpace(n)
	if n == "" {
		return false
	}
	runas := 0
	for _, r := range n {
		if unicode.IsControl(r) {
			return false
		}
		runas++
	}
	return runas <= NombreServicioMax
}

// ClaseValida dice si la clase está en el enum acotado. La cadena vacía SÍ es válida: significa
// «no se declaró», que es distinto de una clase inventada.
func ClaseValida(c string) bool { return clasesConocidas[strings.ToLower(strings.TrimSpace(c))] }

// ValidarAltaServicio chequea lo que tiene que ser cierto ANTES de que el servicio exista.
// Fail-closed, igual que ValidarAlta: ante la duda, no se da de alta.
func ValidarAltaServicio(s Servicio) error {
	if !NombreDeServicioValido(s.Nombre) {
		return fmt.Errorf("%w (o el nombre pasa de %d runas o trae caracteres de control)", ErrSinNombreServicio, NombreServicioMax)
	}
	// A6 llegando hasta acá: una fila sin proyecto se ve desde TODOS los tenants. En un servicio
	// el proyecto no lo declara nadie —se copia del device— así que llegar sin él significa que
	// el device no se resolvió, y eso es un error del llamador, no del usuario.
	if strings.TrimSpace(s.ProjectID) == "" {
		return fmt.Errorf("%w (servicio %q)", ErrSinProyecto, s.Nombre)
	}
	if strings.TrimSpace(s.DeviceID) == "" {
		return fmt.Errorf("el servicio %q no dice en qué máquina corre: un servicio sin device no tiene de dónde heredar la tenencia", s.Nombre)
	}
	if !ClaseValida(s.Clase) {
		return fmt.Errorf("%w: %q (esperaba systemd, windows, docker o vacío)", ErrClaseDesconocida, s.Clase)
	}
	return nil
}

// RecortarReporte acota lo que viene de la máquina. NO rechaza: recorta.
//
// La asimetría es a propósito. Un nombre de 300 runas es una unit con un nombre largo, no un
// ataque, y perder el servicio entero por eso sería peor que mostrarlo cortado; una salud con un
// estado inventado, en cambio, sí se rechaza en Valida, porque un estado que no entendemos no se
// puede dibujar de ninguna forma honesta.
func RecortarReporte(r ReporteServicio) ReporteServicio {
	r.Nombre = recortarRunas(strings.TrimSpace(r.Nombre), NombreServicioMax)
	r.Clase = strings.ToLower(strings.TrimSpace(r.Clase))
	if !ClaseValida(r.Clase) {
		// Una clase que este binario no conoce se trata como AUSENTE, no como un rechazo: es la
		// misma lectura tolerante que CapsDesdeTexto hace con las capacidades. Perder la etiqueta
		// del supervisor es barato; perder el servicio, no.
		r.Clase = ""
	}
	r.Salud.Detalle = recortarRunas(strings.TrimSpace(r.Salud.Detalle), DetalleServicioMax)
	return r
}

// recortarRunas corta por RUNAS y no por bytes: el nombre de una unit puede traer acentos, y
// cortar a la mitad de un carácter multibyte deja basura en una celda que después se dibuja.
func recortarRunas(s string, max int) string {
	if max <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= max {
		return s
	}
	return string(rs[:max])
}

// Fresco DERIVA si el reporte es reciente. Es el gemelo de Device.EnLinea y existe por el mismo
// motivo: un servicio con un reporte de hace dos días NO está «corriendo», está sin noticias, y
// son cosas distintas.
//
// El umbral lo elige QUIEN PREGUNTA, igual que en EnLinea, y por eso no hay default acá.
//
// La guarda de UltimoReporte cero cubre al servicio DECLARADO Y NUNCA MEDIDO —el que alguien
// enroló a mano y todavía nadie reportó—: sin ella, un `ahora` cero (un reloj inyectado sin
// inicializar) lo daría por fresco.
func (s Servicio) Fresco(ahora time.Time, umbral time.Duration) bool {
	if s.Revocado || s.UltimoReporte.IsZero() || umbral <= 0 {
		return false
	}
	return ahora.Sub(s.UltimoReporte) <= umbral
}

// EstadoActual es lo que se INFORMA de un servicio, y es la función que sostiene el invariante
// que le da sentido al slice: sin salud, el estado es `desconocido` — JAMÁS `detenido`.
//
// Está acá, en el dominio, y no en la fila de la tool ni en el panel, para que los tres
// consumidores no puedan discrepar. El día que alguien escriba un tercer consumidor y decida por
// su cuenta qué significa «sin salud», la mitad de la flota va a decir una cosa y la otra mitad
// otra.
func (s Servicio) EstadoActual() EstadoServicio {
	if s.Salud == nil {
		return EstadoDesconocido
	}
	return s.Salud.Estado
}
