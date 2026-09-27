package mcp

import (
	"compress/gzip"
	"fmt"
	"io"
	"strings"
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
// La de «bajada» es UNA sola a propósito: el frente que mide la edad de la bajada le agrega «última
// hace…, próxima en…» a esta misma línea, en vez de abrir otra que pueda contradecirla.
func lineasDelViaje(r memory.ResumenDelSync, teamMode bool) string {
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
