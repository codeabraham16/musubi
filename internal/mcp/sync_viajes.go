package mcp

import (
	"fmt"
	"io"
	"sync/atomic"

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
type traficoDelSync struct {
	subidaPosts, subidaCable, subidaCrudos atomic.Int64
	bajadaCable, bajadaCrudos              atomic.Int64
}

// fotoDelTrafico es una lectura de los contadores. Restar dos fotos da lo que movió el tick.
type fotoDelTrafico struct {
	subidaPosts, subidaCable, subidaCrudos int64
	bajadaCable, bajadaCrudos              int64
}

// foto lee los contadores. Nil-safe: un server sin cliente de sync no movió nada.
func (c *SyncClient) foto() fotoDelTrafico {
	if c == nil {
		return fotoDelTrafico{}
	}
	return fotoDelTrafico{
		subidaPosts:  c.trafico.subidaPosts.Load(),
		subidaCable:  c.trafico.subidaCable.Load(),
		subidaCrudos: c.trafico.subidaCrudos.Load(),
		bajadaCable:  c.trafico.bajadaCable.Load(),
		bajadaCrudos: c.trafico.bajadaCrudos.Load(),
	}
}

// subidaDesde es el viaje de subida entre la foto `antes` y ahora. Filas lo pone el drain: sólo él
// sabe cuántas aceptó el central.
func (c *SyncClient) subidaDesde(antes fotoDelTrafico) memory.Viaje {
	d := c.foto()
	return memory.Viaje{
		Posts:       d.subidaPosts - antes.subidaPosts,
		BytesCable:  d.subidaCable - antes.subidaCable,
		BytesCrudos: d.subidaCrudos - antes.subidaCrudos,
	}
}

// bajadaDesde es lo que viajó en la bajada entre la foto `antes` y ahora. Las páginas y las filas
// las cuenta el drain, que es el que sabe cuáles llegaron bien.
func (c *SyncClient) bajadaDesde(antes fotoDelTrafico) (cable, crudos int64) {
	d := c.foto()
	return d.bajadaCable - antes.bajadaCable, d.bajadaCrudos - antes.bajadaCrudos
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
// La de «bajada» es UNA sola a propósito: el frente que mide la edad de la bajada le agrega «última
// hace…, próxima en…» a esta misma línea, en vez de abrir otra que pueda contradecirla.
func lineasDelViaje(r memory.ResumenDelSync, teamMode bool) string {
	s := fmt.Sprintf("\nsubida: %d enviadas en 24 h y %d en 7 d · hoy %d filas en %d posts, %s en el cable (%s crudos) · 7 d %d filas en %d posts, %s en el cable",
		r.EnviadasDia, r.EnviadasSemana,
		r.SubidaHoy.Filas, r.SubidaHoy.Posts, bytesLegibles(r.SubidaHoy.BytesCable), bytesLegibles(r.SubidaHoy.BytesCrudos),
		r.Subida7d.Filas, r.Subida7d.Posts, bytesLegibles(r.Subida7d.BytesCable))
	s += fmt.Sprintf("\nbajada: hoy %d filas en %d páginas, %s en el cable · 7 d %d filas en %d páginas, %s en el cable",
		r.BajadaHoy.Filas, r.BajadaHoy.Posts, bytesLegibles(r.BajadaHoy.BytesCable),
		r.Bajada7d.Filas, r.Bajada7d.Posts, bytesLegibles(r.Bajada7d.BytesCable))

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
