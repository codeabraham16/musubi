package mcp

import (
	"compress/gzip"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"musubi/internal/memory"
)

// sync_viajes.go junta lo que el sync mide de sí mismo: los bytes que el SyncClient hizo viajar y
// las líneas que musubi_sync_status arma con eso (ver memory/sync_viajes.go para la tabla).

// traficoDelSync acumula, por sentido, lo que el cliente hizo viajar desde que existe. Los drains
// no lo leen como total: toman una foto al empezar el tick y otra al terminar, y registran la
// DIFERENCIA. Así no hay que resetear nada —un reset compartido entre dos goroutines es un
// contador que se come lo del otro— y cada sentido lo escribe una sola goroutine por proceso (la
// subida la mueve drainOutboxOnce y la bajada drainInboundOnce).
//
// Cuenta sólo lo que VIAJÓ: en la subida, el cuerpo de un POST que el central contestó; en la
// bajada, el cuerpo de una página que llegó y se pudo leer. Un pedido sin respuesta no suma.
//
// Y separa lo que viajó CON FILAS de lo que no: el cable de un POST aceptado o de una página que
// trajo algo va a subidaCable/bajadaCable; el de un POST rechazado y el de una página vacía, a sus
// contadores propios. Es lo que deja que bytes_cable/filas sea el peso de una nota (ver
// memory.Viaje).
type traficoDelSync struct {
	subidaPosts, subidaCable, subidaCrudos atomic.Int64
	rechazados, bytesRechazados            atomic.Int64
	bajadaCable, bajadaCrudos              atomic.Int64
	vacias, bytesVacias                    atomic.Int64
}

// fotoDelTrafico es una lectura de los contadores, en la forma del viaje que se registra. Restar
// dos fotos da lo que movió el tick.
type fotoDelTrafico struct {
	subida, bajada memory.Viaje
}

// foto lee los contadores. Nil-safe: un server sin cliente de sync no movió nada.
func (c *SyncClient) foto() fotoDelTrafico {
	if c == nil {
		return fotoDelTrafico{}
	}
	t := &c.trafico
	return fotoDelTrafico{
		subida: memory.Viaje{
			Posts:           t.subidaPosts.Load(),
			BytesCable:      t.subidaCable.Load(),
			BytesCrudos:     t.subidaCrudos.Load(),
			Rechazados:      t.rechazados.Load(),
			BytesRechazados: t.bytesRechazados.Load(),
		},
		bajada: memory.Viaje{
			BytesCable:  t.bajadaCable.Load(),
			BytesCrudos: t.bajadaCrudos.Load(),
			Vacias:      t.vacias.Load(),
			BytesVacias: t.bytesVacias.Load(),
		},
	}
}

// menos resta b de a campo por campo: lo que movió el tick entre dos fotos.
func menos(a, b memory.Viaje) memory.Viaje {
	return memory.Viaje{
		Filas:           a.Filas - b.Filas,
		Posts:           a.Posts - b.Posts,
		BytesCable:      a.BytesCable - b.BytesCable,
		BytesCrudos:     a.BytesCrudos - b.BytesCrudos,
		Vacias:          a.Vacias - b.Vacias,
		BytesVacias:     a.BytesVacias - b.BytesVacias,
		Rechazados:      a.Rechazados - b.Rechazados,
		BytesRechazados: a.BytesRechazados - b.BytesRechazados,
		SinCambios:      a.SinCambios - b.SinCambios,
		Rebotes:         a.Rebotes - b.Rebotes,
		Choques:         a.Choques - b.Choques,
	}
}

// subidaDesde es el viaje de subida entre la foto `antes` y ahora. Filas lo pone el drain: sólo él
// sabe cuántas aceptó el central.
func (c *SyncClient) subidaDesde(antes fotoDelTrafico) memory.Viaje {
	return menos(c.foto().subida, antes.subida)
}

// bajadaDesde es lo que viajó en la bajada entre la foto `antes` y ahora: bytes y páginas vacías.
// Las páginas (Posts) y las filas las cuenta el drain, que es el que sabe cuáles llegaron bien.
func (c *SyncClient) bajadaDesde(antes fotoDelTrafico) memory.Viaje {
	return menos(c.foto().bajada, antes.bajada)
}

// lectorContado cuenta los bytes que se leyeron de un cuerpo de respuesta.
type lectorContado struct {
	r io.Reader
	n int64
}

func (l *lectorContado) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	l.n += int64(n)
	return n, err
}

// terminar lee lo que el decoder JSON dejó sin consumir (el salto de línea final, por ejemplo),
// que también viajó, y devuelve el total. Drenar el cuerpo además deja la conexión reusable.
func (l *lectorContado) terminar() int64 {
	_, _ = io.Copy(io.Discard, l)
	return l.n
}

// paginaEnClaro devuelve el cuerpo de una página de la bajada tal como se lee: descomprimido si
// viajó con gzip y, en las dos formas, ACOTADO a tope+1 bytes. El byte de más distingue «entró
// justo» de «se pasó», igual que en readRequestBody: si el decoder falla con más del tope leído, la
// página se cortó ahí.
//
// SIN TOPE, DESCOMPRIMIR ES UNA BOMBA. Medido en la revisión: ~64 KB de cable se expandían a 64 MiB
// y el decoder JSON los juntaba enteros antes de rechazar la página, con 448 MiB reservados. Con 1 MB
// de cable serían unos 7 GiB: el daemon de la laptop (7,6 GB) muere por memoria, y otra vez en cada
// tick. Hace falta un central comprometido o alguien que conteste en su lugar (una central_url
// http:// con allow_insecure_token), pero el central ya se cuida así en el sentido contrario, y la
// bajada se descomprime acá. En claro no hay amplificación —lo que se lee es lo que viajó—, pero
// una página de más del tope tampoco es una página real, y acotar las dos formas en el mismo lugar
// deja una sola cosa que vigilar.
//
// Vive al lado de lectorContado porque es la otra mitad de la medida: `cable` cuenta lo que viajó,
// y lo que devuelve esto es lo que se cuenta como crudos.
func paginaEnClaro(cable io.Reader, contentEncoding string, tope int64) (io.Reader, error) {
	claro := cable
	switch enc := strings.ToLower(strings.TrimSpace(contentEncoding)); enc {
	case "", "identity":
	case "gzip":
		zr, err := gzip.NewReader(cable)
		if err != nil {
			return nil, fmt.Errorf("%w: el central dijo gzip y la página no lo es: %v", errTransient, err)
		}
		claro = zr
	default:
		// Sólo se pidió gzip. Otra codificación no es algo que el decoder JSON pueda leer, y dejar
		// que lo intente daría un error de sintaxis que no nombra la causa.
		return nil, fmt.Errorf("%w: el central contestó con Content-Encoding %q, que no se pidió", errTransient, enc)
	}
	return io.LimitReader(claro, tope+1), nil
}

// bytesLegibles escribe un tamaño en B, KiB o MiB con un decimal.
func bytesLegibles(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// lineasDelViaje arma las tres líneas que musubi_sync_status suma al resumen del outbox.
//
// La de «bajada» es UNA sola a propósito: lleva el volumen (sync_viajes) y, al final, `edad`
// —cuándo bajó por última vez y cuándo se espera la próxima, ver edadDeLaBajada—, en vez de abrir
// otra línea que pueda contradecirla.
func lineasDelViaje(r memory.ResumenDelSync, teamMode bool, edad string) string {
	// El cable que se muestra es el de lo que viajó CON FILAS; lo rechazado y las páginas vacías van
	// entre paréntesis, con su cuenta y su peso, porque son otro costo (ver memory.Viaje).
	s := fmt.Sprintf("\nsubida: %d enviadas en 24 h y %d en 7 d · hoy %d filas en %d posts (%d rechazados, %s), %s en el cable (%s crudos) · 7 d %d filas en %d posts (%d rechazados), %s en el cable",
		r.EnviadasDia, r.EnviadasSemana,
		r.SubidaHoy.Filas, r.SubidaHoy.Posts, r.SubidaHoy.Rechazados, bytesLegibles(r.SubidaHoy.BytesRechazados),
		bytesLegibles(r.SubidaHoy.BytesCable), bytesLegibles(r.SubidaHoy.BytesCrudos),
		r.Subida7d.Filas, r.Subida7d.Posts, r.Subida7d.Rechazados, bytesLegibles(r.Subida7d.BytesCable))
	s += fmt.Sprintf("\nbajada: hoy %d filas en %d páginas (%d vacías, %s), %s en el cable · 7 d %d filas en %d páginas (%d vacías, %s), %s en el cable",
		r.BajadaHoy.Filas, r.BajadaHoy.Posts, r.BajadaHoy.Vacias, bytesLegibles(r.BajadaHoy.BytesVacias), bytesLegibles(r.BajadaHoy.BytesCable),
		r.Bajada7d.Filas, r.Bajada7d.Posts, r.Bajada7d.Vacias, bytesLegibles(r.Bajada7d.BytesVacias), bytesLegibles(r.Bajada7d.BytesCable))
	s += edad

	// POR QUÉ no viaja cada cosa, y no sólo cuántas. El motivo de las locales depende del proyecto:
	// el scope por defecto lo decide memory.team_mode, sin mirar la nota.
	motivoLocales := "el proyecto no está en team_mode"
	if teamMode {
		motivoLocales = "guardadas con scope local, o antes del team_mode"
	}
	s += fmt.Sprintf("\nno viajan: %d locales (%s), %d en cuarentena (propuestas de un LLM sin corroborar), %d de un LLM ya corroboradas que siguen locales (nada las promueve)",
		r.NoViajan.Locales, motivoLocales, r.NoViajan.EnCuarentena, r.NoViajan.LLMCorroboradasLocales)
	return s
}

// edadDeLaBajada completa la línea «bajada» de musubi_sync_status: cuándo bajó por última vez, si
// trajo algo y cuándo se espera la próxima.
//
// LEE SÓLO LA META (memory.MetaUltimaBajada), NUNCA EL ESTADO DE ESTE PROCESO. Sobre una base
// corren varios daemons y baja uno, el dueño del candado; el que contesta casi nunca es ése, y
// musubi_sync_status también lo sirve el central, que no baja nunca.
//
// Y NO PUEDE CONTRADECIR AL VOLUMEN, que sale de sync_viajes (r) y comparte transacción con la meta
// (memory.RegistrarBajada). Un binario de esta versión nunca deja una fuente sin la otra, así que
// si hay viajes de bajada sin meta —o más páginas que las que la meta anotó, un día posterior o el
// mismo día—, los bajó un binario anterior, que registra el viaje y no la edad. La línea dice eso y
// no «nunca» al lado de las filas bajadas hoy.
//
// EL ORDEN DE LECTURA ES PARTE DEL CONTRATO: el llamador toma `ahora` ANTES de leer sync_viajes, y
// la meta se lee acá, DESPUÉS. Así la meta es por lo menos tan nueva como los viajes que se leyeron,
// y un tick que se escribe entre las dos lecturas —o que cruza la medianoche UTC— no se confunde
// con un binario viejo: deja la meta con un día posterior al último leído, o con más páginas.
func (s *McpServer) edadDeLaBajada(r memory.ResumenDelSync, ahora time.Time) string {
	raw, hay, err := s.engine.GetMeta(memory.MetaUltimaBajada)
	if err != nil {
		return " · última: no se pudo leer (" + strconv.Quote(err.Error()) + ")"
	}
	// El tick es el de ESTE proceso, que sobre la misma base tiene la misma config que el dueño.
	return describirUltimaBajada(raw, hay, r, ahora, s.porQueNoBaja(), s.tickBajada())
}

// porQueNoBaja es por qué ESTE proceso no corre la bajada —la misma condición con la que se apaga
// RunInboundScheduler—, o "" si la corre, tenga o no el candado. Es configuración y no estado: no
// dice nada de cuándo bajó nadie, y edadDeLaBajada la usa sólo cuando no hay ningún registro.
func (s *McpServer) porQueNoBaja() string {
	switch {
	case s.syncClient == nil:
		return "no tiene cliente de sync"
	case !s.memory.TeamMode:
		return "el proyecto no está en team_mode"
	}
	return ""
}

// describirUltimaBajada es edadDeLaBajada sin la base: raw y hay son la meta tal como se leyó, r el
// volumen de sync_viajes, noBaja el motivo por el que este proceso no corre la bajada ("" si la
// corre) y tick el intervalo de la bajada, que es el margen antes de dar la próxima por vencida.
// Devuelve el final de la línea, que empieza con « · » y nunca trae un salto de línea.
func describirUltimaBajada(raw string, hay bool, r memory.ResumenDelSync, ahora time.Time, noBaja string, tick time.Duration) string {
	if !hay {
		switch {
		case r.UltimoDiaConBajada != "":
			// Hubo bajadas —cualquier día, no sólo en la semana— y ninguna anotó su edad: un binario
			// de esta versión no deja eso.
			return " · última: sin anotar (bajó un binario anterior a esta versión, que registra el viaje y no la edad)"
		case noBaja != "":
			// El central, o un nodo sin sync. «Nunca» sería cierto de ESTE proceso y se leería «tu
			// máquina nunca bajó», que es justo lo que no puede decir.
			return " · este proceso no baja (" + noBaja + "): la bajada de una máquina la informa el daemon de esa máquina"
		}
		return " · última: nunca"
	}
	u, err := memory.LeerUltimaBajada(raw)
	if err != nil {
		// Entre comillas y escapado: un valor cortado con un salto de línea partiría la línea en dos.
		return " · última: ilegible (" + strconv.Quote(recortarConMarca(raw, 40)) + ")"
	}
	que := "vacía"
	switch {
	case u.Filas == 1:
		que = "1 fila"
	case u.Filas > 1:
		que = strconv.FormatInt(u.Filas, 10) + " filas"
	case u.ConFilas == 1:
		// Trajo filas y no entró ninguna (una fila que esta base rechaza, o un SQLITE_BUSY), así que
		// el cursor no avanzó. sync_viajes la cuenta como página con filas: «vacía» lo contradiría,
		// y con la edad fresca se leería «no hay nada nuevo» con la bajada atascada.
		que = "1 página con filas y 0 ingeridas"
	case u.ConFilas > 1:
		que = strconv.FormatInt(u.ConFilas, 10) + " páginas con filas y 0 ingeridas"
	}
	edad := duracionLegible(max(ahora.Unix()-u.Unix, 0))
	if bajoDespuesSinAnotar(r, u) {
		return " · última anotada hace " + edad + " (" + que + "), pero después bajó un binario anterior a esta versión, que no la anota"
	}
	linea := " · última hace " + edad + " (" + que + "), "
	falta := u.ProximaUnix - ahora.Unix()
	switch {
	case falta >= 0:
		return linea + "próxima en ~" + duracionLegible(falta)
	case -falta <= int64(tick/time.Second):
		// UN TICK DE GRACIA. La próxima sale del FIN del Pull anterior y la meta nueva se escribe al
		// FIN del siguiente, así que con el dueño sano la próxima vence un rato cada vez que un Pull
		// tarda más que el anterior (varias páginas, una espera del busy_timeout) o el candado cambia
		// de dueño, que tickea en otra fase. Eso es ruido y no una alarma: la próxima es ahora.
		return linea + "próxima: ahora"
	}
	// Pasado el tick, lo que se sabe es que nadie anotó otra: nadie baja, sus Pull fallan, o uno
	// tarda más de un tick de lo que tardó el anterior. Se dice CUÁNTO DESPUÉS de la última se
	// esperaba y no hace cuánto: «última hace 3 d, la próxima se esperaba hace 2 d» por una
	// diferencia de 30 s se leía como un día entre las dos.
	return linea + "y no se anotó otra: la próxima se esperaba " + duracionLegible(u.ProximaUnix-u.Unix) + " después"
}

// bajoDespuesSinAnotar dice si sync_viajes tiene bajadas que la meta no anotó, o sea de un binario
// que no la anota. Se compara contra el ÚLTIMO día con bajadas y no contra las cubetas de hoy y de
// la semana, que dejaban afuera los días del medio: una meta de hace tres días con bajadas de ayer
// y ninguna hoy se leía «última hace 3 d».
//
// Hoy no es transicional: en davantis-1 se instalan binarios nuevos sin cerrar las sesiones, así que
// sobre la misma base bajan a la vez daemons que anotan la edad y otros que sólo registran el viaje.
func bajoDespuesSinAnotar(r memory.ResumenDelSync, u memory.UltimaBajada) bool {
	dia := time.Unix(u.Unix, 0).UTC().Format(time.DateOnly)
	if r.UltimoDiaConBajada != dia {
		// Un día POSTERIOR con bajadas las hizo alguien que no anota. Uno anterior —o ninguno— es
		// una meta más nueva que lo leído de sync_viajes: un tick que se anotó entre las dos lecturas.
		return r.UltimoDiaConBajada > dia
	}
	// El MISMO día: la meta anotó cuántas páginas tenía ese día contando las suyas, y si ahora hay
	// más, las sumó después alguien que no anota. Iguales es el caso sano; menos, otra vez un tick
	// que se anotó entre las dos lecturas.
	despues := r.PaginasDeEseDia - u.PaginasDelDia
	return despues > 0
}

// duracionLegible escribe unos segundos en s, min, h o d, sin decimales y REDONDEADOS a la unidad
// más cercana: la línea de la bajada quiere el orden de magnitud, no un cronómetro, y truncar mentía
// por casi una unidad entera (dos días y 23 horas salían «2 d»). La unidad se elige por el valor ya
// redondeado, así que 1 h 59 min 30 s sale «2 h» y no «120 min».
func duracionLegible(seg int64) string {
	if seg < 120 {
		return strconv.FormatInt(seg, 10) + " s"
	}
	if m := (seg + 30) / 60; m < 120 {
		return strconv.FormatInt(m, 10) + " min"
	}
	if h := (seg + 1800) / 3600; h < 48 {
		return strconv.FormatInt(h, 10) + " h"
	}
	return strconv.FormatInt((seg+43200)/86400, 10) + " d"
}
