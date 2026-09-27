package mcp

import (
	"fmt"
	"io"
	"strconv"
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
// si hay viajes de bajada sin meta —o viajes de un día posterior al de la meta—, los bajó un
// binario anterior, que registra el viaje y no la edad. La línea dice eso y no «nunca» al lado de
// las filas bajadas hoy.
//
// EL ORDEN DE LECTURA ES PARTE DEL CONTRATO: el llamador toma `ahora` ANTES de leer sync_viajes, y
// la meta se lee acá, DESPUÉS. Así la meta es por lo menos tan nueva como los viajes que se leyeron,
// y un tick que se escribe entre las dos lecturas —o que cruza la medianoche UTC— no se confunde
// con un binario viejo.
func (s *McpServer) edadDeLaBajada(r memory.ResumenDelSync, ahora time.Time) string {
	raw, hay, err := s.engine.GetMeta(memory.MetaUltimaBajada)
	if err != nil {
		return " · última: no se pudo leer (" + strconv.Quote(err.Error()) + ")"
	}
	return describirUltimaBajada(raw, hay, r, ahora, s.porQueNoBaja())
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
// volumen de sync_viajes y noBaja el motivo por el que este proceso no corre la bajada ("" si la
// corre). Devuelve el final de la línea, que empieza con « · » y nunca trae un salto de línea.
func describirUltimaBajada(raw string, hay bool, r memory.ResumenDelSync, ahora time.Time, noBaja string) string {
	if !hay {
		switch {
		case r.Bajada7d.Posts > 0:
			// Hubo bajadas y ninguna anotó su edad: un binario de esta versión no deja eso.
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
	}
	edad := duracionLegible(max(ahora.Unix()-u.Unix, 0))
	if bajoDespuesSinAnotar(r, u, ahora) {
		return " · última anotada hace " + edad + " (" + que + "), pero después bajó un binario anterior a esta versión, que no la anota"
	}
	linea := " · última hace " + edad + " (" + que + "), "
	falta := u.ProximaUnix - ahora.Unix()
	if falta < 0 {
		// Nadie anotó otra bajada desde entonces: no hay un proceso bajando, o sus Pull fallan.
		return linea + "la próxima se esperaba hace " + duracionLegible(-falta)
	}
	return linea + "próxima en ~" + duracionLegible(falta)
}

// bajoDespuesSinAnotar dice si sync_viajes tiene bajadas de un día POSTERIOR al de la meta, o sea
// de un binario que no la anota. Los viajes van por día UTC y se comparan por día: hay viajes hoy y
// la meta es de un día anterior, o hay viajes en la semana y la meta es de antes de la semana.
// Dentro del mismo día no se distingue; ahí la meta envejece y la línea dice «la próxima se
// esperaba hace…».
func bajoDespuesSinAnotar(r memory.ResumenDelSync, u memory.UltimaBajada, ahora time.Time) bool {
	dia := time.Unix(u.Unix, 0).UTC().Format(time.DateOnly)
	hoy := ahora.UTC()
	if r.BajadaHoy.Posts > 0 && dia < hoy.Format(time.DateOnly) {
		return true
	}
	return r.Bajada7d.Posts > 0 && dia < hoy.AddDate(0, 0, -6).Format(time.DateOnly)
}

// duracionLegible escribe unos segundos en s, min, h o d, sin decimales: la línea de la bajada
// quiere el orden de magnitud, no un cronómetro.
func duracionLegible(seg int64) string {
	switch {
	case seg < 120:
		return strconv.FormatInt(seg, 10) + " s"
	case seg < 2*3600:
		return strconv.FormatInt(seg/60, 10) + " min"
	case seg < 48*3600:
		return strconv.FormatInt(seg/3600, 10) + " h"
	default:
		return strconv.FormatInt(seg/86400, 10) + " d"
	}
}
