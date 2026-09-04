package fleet

// exposicion.go lee el FORMATO DE EXPOSICIÓN de Prometheus y lo convierte en una Muestra.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// PARA QUÉ EXISTE
//
// Tier B es «sin binario en el device, por su protocolo nativo». Hasta acá el único protocolo
// era SSH, y eso deja afuera a toda una clase de máquinas que Musubi tiene que poder mirar: las
// que no dan shell y sí publican sus vitales. Una base gestionada en la nube es el caso exacto —
// no hay dónde correr un agente, no hay ssh, y hay un endpoint con `node_exporter` adentro.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EL PARSEO ES ACÁ Y NO UNA DEPENDENCIA
//
// Existe una librería oficial para esto y no entra: el repo tiene SEIS dependencias directas y
// eso es una restricción deliberada. Lo que se necesita de verdad son ocho familias de métricas
// y sus etiquetas, y eso es un recorrido de texto. Traer el parser completo del formato —con sus
// exemplars, sus histogramas nativos y su modelo de tipos— por ocho gauges sería pagar todo para
// usar una esquina.
//
// Lo que SÍ se respeta del formato: los comentarios (`#`), las etiquetas con comillas escapadas,
// la notación científica (`7.994146816e+09` es lo que manda un endpoint real, no `7994146816`) y
// el timestamp opcional después del valor.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA REGLA QUE MANDA SOBRE TODO: AUSENTE NO ES CERO
//
// Es la misma disciplina que el resto del track y acá es más fácil de romper, porque un mapa de
// float64 devuelve 0 para lo que no está y ese 0 es indistinguible de un cero medido. Un endpoint
// sin `node_boot_time_seconds` —hay varios, incluido el que motivó esto— daría uptime 0 y el panel
// diría «esta máquina arrancó recién», para siempre. Por eso cada lectura viaja con su
// PRESENCIA, y quien la consume decide si un ausente es un nil o un campo que no se escribe.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ExposicionMax acota lo que se acepta de un endpoint ajeno. El de la base que motivó esto pesa
// 515 KiB; el techo deja lugar de sobra y no permite que un endpoint hostil —o roto— haga que el
// cerebro se coma la memoria del host.
const ExposicionMax = 8 << 20

// MontajeRaiz es el punto de montaje por defecto. Se puede pedir otro porque «el disco que
// importa» no siempre es la raíz: en una base gestionada, `/` es el sistema operativo (76 GB, casi
// vacío) y `/data` es el volumen que se llena. Mirar la raíz ahí sería mirar el disco equivocado
// y no enterarse nunca.
const MontajeRaiz = "/"

// LecturaExposicion es lo que se pudo sacar del texto, CON su presencia.
//
// El mapa es privado a propósito: si fuera público, un llamador podría leer `l.vals["x"]` y
// recibir el 0 de Go creyendo que midió algo. La única puerta es Num, que devuelve dos valores.
type LecturaExposicion struct {
	vals map[string]float64
	// cpus son los valores distintos de la etiqueta `cpu`. Se cuentan y no se suman: NumCPU es
	// cuántos núcleos hay, y sumar los contadores de tiempo daría un número enorme sin sentido.
	cpus map[string]bool
}

// Num devuelve un valor y si de verdad estaba. Nunca devuelve un cero fabricado.
func (l LecturaExposicion) Num(clave string) (float64, bool) {
	v, ok := l.vals[clave]
	return v, ok
}

// NumCPU es cuántos núcleos distintos declaró el endpoint. 0 = no lo dijo.
func (l LecturaExposicion) NumCPU() int { return len(l.cpus) }

// Las claves de la lectura. Son constantes y no cadenas sueltas porque un typo en una de ellas no
// rompe nada visible: el valor no aparece, el campo queda en «no medido», y el panel muestra un
// hueco que se lee como «esta máquina no lo expone».
const (
	ExpMemTotal      = "mem_total"
	ExpMemDisponible = "mem_disponible"
	ExpMemLibre      = "mem_libre"
	ExpSwapTotal     = "swap_total"
	ExpSwapLibre     = "swap_libre"
	ExpDiscoTotal    = "disco_total"
	ExpDiscoLibre    = "disco_libre"
	ExpDiscoDispo    = "disco_disponible"
	ExpLoad1         = "load1"
	ExpLoad5         = "load5"
	ExpLoad15        = "load15"
	ExpCPUIdle       = "cpu_idle"
	ExpCPUTotal      = "cpu_total"
	ExpArranque      = "arranque"
	ExpAhora         = "ahora"
)

// familiaSimple mapea las métricas de un solo valor a su clave.
var familiaSimple = map[string]string{
	"node_memory_MemTotal_bytes":     ExpMemTotal,
	"node_memory_MemAvailable_bytes": ExpMemDisponible,
	"node_memory_MemFree_bytes":      ExpMemLibre,
	"node_memory_SwapTotal_bytes":    ExpSwapTotal,
	"node_memory_SwapFree_bytes":     ExpSwapLibre,
	"node_load1":                     ExpLoad1,
	"node_load5":                     ExpLoad5,
	"node_load15":                    ExpLoad15,
	"node_boot_time_seconds":         ExpArranque,
	"node_time_seconds":              ExpAhora,
}

// familiaDisco mapea las del sistema de archivos, que además necesitan el punto de montaje.
var familiaDisco = map[string]string{
	"node_filesystem_size_bytes":  ExpDiscoTotal,
	"node_filesystem_free_bytes":  ExpDiscoLibre,
	"node_filesystem_avail_bytes": ExpDiscoDispo,
}

// ParsearExposicion recorre el texto UNA vez y se queda con lo que necesita.
//
// Devuelve ok=false cuando del otro lado NO hay vitales de host. La validación es SEMÁNTICA y no
// de formato, igual que la de /proc: un endpoint puede ser un formato de exposición perfecto y
// exponer únicamente métricas de aplicación. Aceptarlo daría una Muestra de ceros que el panel
// dibujaría como una máquina con 0 de RAM, y ése es el modo de fallo que este track persigue.
// El mínimo es la memoria total: no hay host que no la sepa decir.
func ParsearExposicion(texto, montaje string) (LecturaExposicion, bool) {
	if montaje == "" {
		montaje = MontajeRaiz
	}
	l := LecturaExposicion{vals: map[string]float64{}, cpus: map[string]bool{}}

	for _, linea := range strings.Split(texto, "\n") {
		linea = strings.TrimSpace(linea)
		// El `#` sólo es comentario AL PRINCIPIO. Adentro de una etiqueta es un carácter común, y
		// un `strings.Contains` acá se comería líneas legítimas.
		if linea == "" || strings.HasPrefix(linea, "#") {
			continue
		}
		nombre, etiquetas, valorTxt, ok := partirLineaExposicion(linea)
		if !ok {
			continue
		}

		if clave, hay := familiaSimple[nombre]; hay {
			if v, err := strconv.ParseFloat(valorTxt, 64); err == nil {
				l.vals[clave] = v
			}
			continue
		}
		if clave, hay := familiaDisco[nombre]; hay {
			// LA PRIMERA QUE COINCIDE GANA, y no la última. Un endpoint puede publicar el mismo
			// montaje dos veces (un bind mount, un overlay); quedarse con la última haría que el
			// número dependa del orden en que el exporter recorrió /proc/mounts, que no está
			// garantizado y cambia entre corridas.
			if _, ya := l.vals[clave]; ya {
				continue
			}
			if valorDeEtiqueta(etiquetas, "mountpoint") != montaje {
				continue
			}
			if v, err := strconv.ParseFloat(valorTxt, 64); err == nil {
				l.vals[clave] = v
			}
			continue
		}
		if nombre == "node_cpu_seconds_total" {
			v, err := strconv.ParseFloat(valorTxt, 64)
			if err != nil {
				continue
			}
			// Se ACUMULA sobre todos los núcleos y todos los modos: es exactamente lo que hace
			// /proc/stat en su primera línea, y el contador de deltas espera esa forma.
			l.vals[ExpCPUTotal] += v
			if valorDeEtiqueta(etiquetas, "mode") == "idle" {
				l.vals[ExpCPUIdle] += v
			}
			if c := valorDeEtiqueta(etiquetas, "cpu"); c != "" {
				l.cpus[c] = true
			}
		}
	}

	// LA COMPUERTA PIDE EL PAR, NO EL TOTAL SOLO, Y TIENE QUE COINCIDIR CON Muestra.Valida.
	//
	// La primera versión pedía nada más `MemTotal`, y el desacuerdo con la regla de los pares del
	// dominio se veía así: un endpoint que publica el total y no el disponible pasaba la
	// compuerta, armaba una Muestra, y recién ahí la rechazaban con «la muestra no es creíble».
	// El mensaje es cierto y manda a mirar el lugar equivocado — suena a dato corrupto cuando lo
	// que pasa es que ese endpoint no alcanza para medir un host. Dos guardas sobre lo mismo que
	// no se enteran una de la otra terminan discutiendo en el mensaje de error.
	_, hayTotal := l.Num(ExpMemTotal)
	_, hayDispo := l.Num(ExpMemDisponible)
	if !hayTotal || !hayDispo {
		return LecturaExposicion{}, false
	}
	return l, true
}

// partirLineaExposicion separa `nombre{etiquetas} valor [timestamp]`.
//
// El timestamp opcional del formato es la trampa: está DESPUÉS del valor, así que quedarse con el
// último campo devolvería una marca de tiempo en milisegundos donde va la memoria. Se toma el
// PRIMER campo tras las etiquetas, que es lo que dice la especificación.
func partirLineaExposicion(linea string) (nombre, etiquetas, valor string, ok bool) {
	var resto string
	if i := strings.IndexByte(linea, '{'); i >= 0 {
		j := cierreDeEtiquetas(linea, i)
		if j < 0 {
			return "", "", "", false
		}
		nombre = linea[:i]
		etiquetas = linea[i+1 : j]
		resto = linea[j+1:]
	} else {
		i := strings.IndexAny(linea, " \t")
		if i < 0 {
			return "", "", "", false
		}
		nombre = linea[:i]
		resto = linea[i:]
	}
	campos := strings.Fields(resto)
	if len(campos) == 0 {
		return "", "", "", false
	}
	return nombre, etiquetas, campos[0], true
}

// cierreDeEtiquetas encuentra la `}` que cierra, respetando las comillas.
//
// No alcanza con IndexByte('}'): una etiqueta puede CONTENER una llave —`device_error="}"` es
// legal— y cortar ahí partiría la línea en el lugar equivocado, con el resto interpretado como
// valor. Lo mismo con la comilla escapada `\"`, que no cierra la cadena.
func cierreDeEtiquetas(s string, desde int) int {
	dentro := false
	for i := desde + 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			if dentro {
				i++ // se saltea lo escapado, sea lo que sea
			}
		case '"':
			dentro = !dentro
		case '}':
			if !dentro {
				return i
			}
		}
	}
	return -1
}

// valorDeEtiqueta saca `clave="valor"` del bloque de etiquetas.
//
// Compara el nombre COMPLETO y no por prefijo: `mode` y `modo_extendido` empiezan igual, y buscar
// `mode="` encontraría a la segunda si el exporter la publicara antes. El síntoma sería un
// porcentaje de CPU plausible calculado con el contador equivocado.
func valorDeEtiqueta(etiquetas, clave string) string {
	i := 0
	for i < len(etiquetas) {
		for i < len(etiquetas) && (etiquetas[i] == ' ' || etiquetas[i] == ',') {
			i++
		}
		ini := i
		for i < len(etiquetas) && etiquetas[i] != '=' {
			i++
		}
		if i >= len(etiquetas) {
			return ""
		}
		nombre := strings.TrimSpace(etiquetas[ini:i])
		i++ // el =
		if i >= len(etiquetas) || etiquetas[i] != '"' {
			return ""
		}
		i++ // la comilla de apertura
		var b strings.Builder
		for i < len(etiquetas) && etiquetas[i] != '"' {
			if etiquetas[i] == '\\' && i+1 < len(etiquetas) {
				i++
				switch etiquetas[i] {
				case 'n':
					b.WriteByte('\n')
				default:
					b.WriteByte(etiquetas[i])
				}
			} else {
				b.WriteByte(etiquetas[i])
			}
			i++
		}
		i++ // la comilla de cierre
		if nombre == clave {
			return b.String()
		}
	}
	return ""
}

// MuestraDesdeExposicion arma la Muestra. `cpu` lleva el estado entre sondeos; con nil sale sin
// porcentaje, que es lo honesto en la primera lectura.
//
// Cada campo se escribe SÓLO si su fuente estaba. Lo que no vino queda en el cero de Go para los
// que el dominio ya interpreta como «no medido» (UptimeSeg, NumProcesos) y en nil para los que
// viajan como puntero.
func MuestraDesdeExposicion(l LecturaExposicion, cpu *ContadorCPUExportado, ahora time.Time) Muestra {
	m := Muestra{Tomada: ahora.UTC(), NumCPU: l.NumCPU()}

	// EL TOTAL SE ESCRIBE SÓLO SI TAMBIÉN SE PUEDE ESCRIBIR EL USADO. Es la regla de los pares
	// del dominio respetada en el ORIGEN en vez de producir una Muestra que Valida va a rechazar:
	// un total con el usado en cero se lee como 0 % de memoria en uso, y ese es el número que
	// alguien mira para decidir que no hay problema.
	//
	// USADA SALE DE MemAvailable Y NO DE MemFree, igual que en el colector de Linux: la diferencia
	// son varios GB de page cache, que está usada por el kernel y disponible para una aplicación.
	// Reportar `total - free` como usada daría cualquier Linux sano al 95 %.
	if total, hay := l.Num(ExpMemTotal); hay {
		if disp, hayD := l.Num(ExpMemDisponible); hayD && disp <= total {
			m.MemTotal = uint64(total)
			m.MemUsada = uint64(total - disp)
		}
	}
	if libre, hay := l.Num(ExpMemLibre); hay {
		v := uint64(libre)
		m.MemLibre = &v
	}
	if total, hay := l.Num(ExpSwapTotal); hay {
		m.SwapTotal = uint64(total)
		if libre, hay := l.Num(ExpSwapLibre); hay && libre <= total {
			m.SwapUsada = uint64(total - libre)
		}
	}

	// Misma regla de los pares, y por el mismo motivo: un disco total con el usado en cero se
	// dibuja como un disco vacío.
	//
	// USADO = total - LIBRE, y disponible es otra cosa. Es la regla de las tres columnas de `df`
	// que el dominio ya documenta: la reserva del 5 % para root no es ni usado ni disponible, así
	// que calcular el usado con `avail` lo inflaría por esa reserva.
	if total, hay := l.Num(ExpDiscoTotal); hay {
		if libre, hayL := l.Num(ExpDiscoLibre); hayL && libre <= total {
			m.DiscoTotal = uint64(total)
			m.DiscoUsado = uint64(total - libre)
			// El disponible sólo tiene sentido junto al total: solo, es un número sin escala.
			if dispo, hayD := l.Num(ExpDiscoDispo); hayD && dispo <= total {
				m.DiscoDisponible = uint64(dispo)
			}
		}
	}

	if v, hay := l.Num(ExpLoad1); hay {
		x := v
		m.Load1 = &x
	}
	if v, hay := l.Num(ExpLoad5); hay {
		x := v
		m.Load5 = &x
	}
	if v, hay := l.Num(ExpLoad15); hay {
		x := v
		m.Load15 = &x
	}

	// El uptime NECESITA LAS DOS: sin `node_time_seconds` no se puede usar el reloj del cerebro
	// en su lugar. Los relojes de dos máquinas difieren, y en una nube gestionada la diferencia
	// puede ser de minutos: el uptime saldría con un error del tamaño de esa deriva, o negativo.
	arranque, hayA := l.Num(ExpArranque)
	ahoraRemoto, hayB := l.Num(ExpAhora)
	if hayA && hayB && ahoraRemoto > arranque {
		m.UptimeSeg = uint64(ahoraRemoto - arranque)
	}

	// El porcentaje sale del par (ocupado, total) en CENTÉSIMAS DE SEGUNDO. No es una constante
	// arbitraria: /proc/stat cuenta en jiffies de 1/100 s, así que multiplicar por 100 pone estos
	// segundos en la misma unidad que la otra fuente del mismo contador, y la aritmética de
	// cpudelta.go se reusa tal cual en vez de tener una segunda copia con otra escala.
	//
	// MUCHOS ENDPOINTS GESTIONADOS CACHEAN SU RESPUESTA, y eso es lo primero que sorprende acá.
	// Medido contra el endpoint real que motivó esto: el contador refresca cada ~62 s (medido: 62, 66, 62). Dos
	// sondeos dentro de la misma ventana de caché ven el mismo total, y cpudelta devuelve nil
	// porque no hay contra qué restar — correcto, y significa que el intervalo de sondeo tiene
	// que SUPERAR ese caché o el porcentaje sale null siempre.
	//
	// Null y no cero, que es la diferencia que importa: el colector que esto reemplaza hacía
	// `[ "$DT" -gt 0 ] && CPU=…` con CPU inicializada en 0, así que cada vez que el endpoint no
	// había refrescado reportaba 0 % — una base ociosa, dibujada con confianza.
	if cpu != nil {
		idle, hayI := l.Num(ExpCPUIdle)
		total, hayT := l.Num(ExpCPUTotal)
		if hayI && hayT && total >= idle {
			m.CPUPct = cpu.Delta(uint64((total-idle)*100), uint64(total*100))
		}
	}
	return m
}

// ── El viaje hasta el endpoint ──────────────────────────────────────────────────────────────

// DestinoExposicion es todo lo que hace falta para raspar un endpoint.
//
// La AUTORIZACIÓN llega ya resuelta: acá adentro no se lee ninguna variable de entorno ni ningún
// archivo. Es a propósito y no es ceremonia — así este paquete se puede probar sin poner un
// secreto en ningún lado, y el único lugar donde una credencial se materializa es el que la
// configura, que es donde se puede auditar.
type DestinoExposicion struct {
	URL string
	// Autorizacion es el valor CRUDO del header (`Bearer abc…`, `Basic …`). Vacío = sin header.
	Autorizacion string
	// Montaje es el sistema de archivos que interesa. Vacío = la raíz.
	Montaje string
}

// clienteExposicion es `var` para que las pruebas lo apunten a un servidor de prueba.
//
// NO SIGUE REDIRECCIONES, y esa es una decisión de seguridad y no de comodidad. Un endpoint que
// contesta 302 hacia otro host haría que el cliente repita el pedido allá — y aunque Go quita los
// headers sensibles al cambiar de dominio, «aunque» no es una garantía que uno quiera sobre una
// credencial. Peor: un redirect a una dirección interna convierte este raspado en un SSRF con
// nuestras propias credenciales. Un endpoint de métricas no tiene por qué redirigir; si lo hace,
// que se note.
var clienteExposicion = &http.Client{
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("el endpoint redirige y esto no sigue redirecciones: apuntá a la URL final")
	},
}

// TomarMuestraDeExposicion trae el texto del endpoint y lo convierte en Muestra.
func TomarMuestraDeExposicion(d DestinoExposicion, cpu *ContadorCPUExportado, timeout time.Duration) (Muestra, error) {
	destino := strings.TrimSpace(d.URL)
	if destino == "" {
		return Muestra{}, errors.New("el dispositivo no tiene URL de métricas: declarala en la configuración de exposición")
	}
	u, err := url.Parse(destino)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Muestra{}, errors.New("la URL de métricas no es http(s) con host: " + primeraLinea(destino))
	}
	if timeout <= 0 || timeout > ComandoTimeoutMax {
		timeout = ComandoTimeoutDefault
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, destino, nil)
	if err != nil {
		return Muestra{}, errors.New("no se pudo armar el pedido a " + u.Host)
	}
	if d.Autorizacion != "" {
		req.Header.Set("Authorization", d.Autorizacion)
	}
	// El formato de exposición de texto, explícito. Sin esto, un endpoint que también habla
	// protobuf puede elegir el binario y el parseo de arriba vería basura — y la vería como «este
	// host no publica memoria», que se lee como un endpoint equivocado y no como un formato mal
	// negociado.
	req.Header.Set("Accept", "text/plain;version=0.0.4,*/*;q=0.1")

	resp, err := clienteExposicion.Do(req)
	if err != nil {
		// EL ERROR DE net/http LLEVA LA URL ENTERA ADENTRO, y esta URL puede traer un token en la
		// query. Este texto termina en la respuesta de una tool, así que se dice el host y el
		// motivo, nunca el pedido crudo.
		return Muestra{}, fmt.Errorf("no se pudo consultar %s: %s", u.Host, motivoDeRed(err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 401 y 403 se nombran aparte porque mandan a mirar un lugar distinto: la credencial, no
		// la red ni el endpoint.
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return Muestra{}, fmt.Errorf("%s rechazó la credencial (HTTP %d): revisá la variable de entorno declarada para este dispositivo", u.Host, resp.StatusCode)
		default:
			return Muestra{}, fmt.Errorf("%s contestó HTTP %d", u.Host, resp.StatusCode)
		}
	}

	// EL LÍMITE ES +1 PARA PODER DISTINGUIR «entró justo» de «se cortó». Leer exactamente el techo
	// y parsear lo que entró daría una Muestra a partir de un texto truncado: las familias que
	// quedaron afuera se leerían como «este host no las expone», que es una mentira con forma de
	// dato válido.
	cuerpo, err := io.ReadAll(io.LimitReader(resp.Body, ExposicionMax+1))
	if err != nil {
		return Muestra{}, fmt.Errorf("no se pudo leer la respuesta de %s: %s", u.Host, motivoDeRed(err))
	}
	if len(cuerpo) > ExposicionMax {
		return Muestra{}, fmt.Errorf("%s devolvió más de %d bytes de métricas: no se parsea a medias", u.Host, ExposicionMax)
	}

	l, ok := ParsearExposicion(string(cuerpo), d.Montaje)
	if !ok {
		return Muestra{}, fmt.Errorf("%s responde pero no publica vitales de host: no se puede medir por este camino", u.Host)
	}
	m := MuestraDesdeExposicion(l, cpu, time.Now())
	if err := m.Valida(); err != nil {
		// Viene de una máquina que no controlamos: entrada no confiable. Se rechaza entera en vez
		// de «corregirla», igual que la de un agente o la de un sondeo por SSH.
		return Muestra{}, errors.New("la muestra no es creíble: " + err.Error())
	}
	return m, nil
}

// motivoDeRed acorta el error de red a algo que se pueda leer, SIN la URL.
func motivoDeRed(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return "se venció el tiempo de espera"
	}
	var uerr *url.Error
	if errors.As(err, &uerr) && uerr.Err != nil {
		return primeraLinea(uerr.Err.Error())
	}
	return primeraLinea(err.Error())
}
