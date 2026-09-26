package memory

import (
	"context"
	"fmt"
	"strings"
)

// sync_viajes.go es el INSTRUMENTO del sync: lo que movió cada tick, sumado por día y sentido
// (migración v58). Existe porque hasta acá la única forma de saber cuántos bytes costaba subir una
// nota era simular el cable sobre una copia de la base, y la única forma de saber cuántas notas
// habían salido era un contador de estado que el pull pisaba (ver IngestShared).
//
// Lo escriben los drains de internal/mcp al final de cada tick, y SÓLO si el tick salió a la red:
// un tick que no tenía nada que mandar, o que no pudo hablar con el central, no escribe nada. Sin
// esa regla, cada tick vacío sería una escritura más sobre una base que comparten varios procesos,
// y la columna posts contaría intentos que nunca viajaron.

// Sentidos de un viaje. Son los mismos dos valores que admite el CHECK de la tabla.
const (
	ViajeSubida = "subida"
	ViajeBajada = "bajada"
)

// Viaje es lo que un tick del sync movió en un sentido. Se suma por día (UTC) en sync_viajes.
//
//   - Filas: en la subida, las filas que el central aceptó; en la bajada, las que se ingirieron acá.
//   - Posts: en la subida, los POST que el central CONTESTÓ (aceptados o rechazados: un rechazo
//     también le costó al central, y posts > filas es como se ve una tormenta de reintentos); en la
//     bajada, las páginas que llegaron bien. Un pedido que no llegó a tener respuesta no cuenta.
//   - BytesCable / BytesCrudos: el cuerpo que viajó, tal como fue por el cable y descomprimido. Hoy
//     no se comprime en ningún sentido, así que valen lo mismo; los PR de compresión los separan.
//   - SinCambios, Rebotes, Choques: nacen en cero. Los llenan los PR siguientes de la ola (el
//     central que no re-guarda lo que ya tiene, y el pull que distingue un rebote propio de un
//     choque con otra máquina). Están en la tabla desde ya para no pedir otra migración.
type Viaje struct {
	Filas       int64 `json:"filas"`
	Posts       int64 `json:"posts"`
	BytesCable  int64 `json:"bytes_cable"`
	BytesCrudos int64 `json:"bytes_crudos"`
	SinCambios  int64 `json:"sin_cambios"`
	Rebotes     int64 `json:"rebotes"`
	Choques     int64 `json:"choques"`
}

// RegistrarViaje SUMA el viaje de un tick a la fila del día (UTC) y sentido. Un solo UPSERT, así
// que dos procesos que registran a la vez no se pisan: cada uno suma lo suyo.
//
// Un sentido desconocido es un error y no un default: la tabla lo rechazaría igual por el CHECK,
// pero con un mensaje de SQLite que no dice quién lo mandó.
func (e *DbEngine) RegistrarViaje(sentido string, v Viaje) error {
	if sentido != ViajeSubida && sentido != ViajeBajada {
		return fmt.Errorf("sentido de viaje desconocido %q: es %q o %q", sentido, ViajeSubida, ViajeBajada)
	}
	if _, err := e.db.Exec(`
		INSERT INTO sync_viajes (dia, sentido, filas, posts, bytes_cable, bytes_crudos, sin_cambios, rebotes, choques)
		VALUES (date('now'), ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(dia, sentido) DO UPDATE SET
			filas        = sync_viajes.filas        + excluded.filas,
			posts        = sync_viajes.posts        + excluded.posts,
			bytes_cable  = sync_viajes.bytes_cable  + excluded.bytes_cable,
			bytes_crudos = sync_viajes.bytes_crudos + excluded.bytes_crudos,
			sin_cambios  = sync_viajes.sin_cambios  + excluded.sin_cambios,
			rebotes      = sync_viajes.rebotes      + excluded.rebotes,
			choques      = sync_viajes.choques      + excluded.choques`,
		sentido, v.Filas, v.Posts, v.BytesCable, v.BytesCrudos, v.SinCambios, v.Rebotes, v.Choques); err != nil {
		return fmt.Errorf("error al registrar el viaje de %s: %w", sentido, err)
	}
	return nil
}

// NoViajan cuenta lo que existe acá y nunca va a salir, separado por el motivo. Las tres
// categorías son DISJUNTAS: una nota cae en una sola, para que la suma se pueda leer.
type NoViajan struct {
	// Locales: visibles, con scope 'local' y no escritas por un LLM. Salen de un proyecto que no
	// está en team_mode (el scope se decide por el proyecto, sin mirar la nota) o de un guardado
	// que pidió 'local' explícito.
	Locales int `json:"locales"`
	// EnCuarentena: propuestas de un LLM que nadie corroboró. No son visibles ni viajan.
	EnCuarentena int `json:"en_cuarentena"`
	// LLMCorroboradasLocales: escritas por un LLM, ya corroboradas, y todavía 'local'. Corroborar
	// las hace visibles pero no las promueve, y nada más las promueve.
	LLMCorroboradasLocales int `json:"llm_corroboradas_locales"`
}

// ResumenDelSync es lo que musubi_sync_status muestra además de la salud del outbox.
type ResumenDelSync struct {
	// EnviadasDia / EnviadasSemana: filas cuya última entrega (sent_at) cae en las últimas 24 h /
	// 7 días. Cuenta NOTAS, no entregas: una nota enviada dos veces en el día cuenta una.
	EnviadasDia    int `json:"enviadas_24h"`
	EnviadasSemana int `json:"enviadas_7d"`
	// Los viajes van por DÍA (UTC) porque así se guardan: «hoy» es la fila de hoy y «7 d» son las
	// siete filas que terminan hoy, no una ventana móvil de 168 horas.
	SubidaHoy Viaje    `json:"subida_hoy"`
	Subida7d  Viaje    `json:"subida_7d"`
	BajadaHoy Viaje    `json:"bajada_hoy"`
	Bajada7d  Viaje    `json:"bajada_7d"`
	NoViajan  NoViajan `json:"no_viajan"`
}

// ResumenDelSync junta los contadores nuevos del sync. Sólo lee.
//
// LO QUE MIRA OBSERVACIONES VA ACOTADO AL PROYECTO DEL ctx (scopeClause), porque musubi_sync_status
// también lo sirve el central: sin el recorte, una credencial de un proyecto leería cuántas notas
// locales o en cuarentena tiene OTRO. Un conteo es información del vecino igual que su texto. En un
// daemon local no hay credencial y el alcance es el de siempre: toda la base. sync_viajes no tiene
// proyecto —es del nodo— y va sin recortar.
func (e *DbEngine) ResumenDelSync(ctx context.Context) (ResumenDelSync, error) {
	var r ResumenDelSync
	sc := projectScopeFrom(ctx)
	scopeO, argsO := sc.scopeClause("o")
	if err := e.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN b.sent_at >= datetime('now','-1 day') THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN b.sent_at >= datetime('now','-7 day') THEN 1 ELSE 0 END), 0)
		FROM outbox b JOIN observations o ON o.id = b.obs_id
		WHERE b.sent_at IS NOT NULL`+scopeO, argsO...).Scan(&r.EnviadasDia, &r.EnviadasSemana); err != nil {
		return r, fmt.Errorf("error al contar las enviadas: %w", err)
	}

	sumar := func(sentido, desde string, v *Viaje) error {
		return e.db.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(filas),0), COALESCE(SUM(posts),0), COALESCE(SUM(bytes_cable),0),
			       COALESCE(SUM(bytes_crudos),0), COALESCE(SUM(sin_cambios),0), COALESCE(SUM(rebotes),0),
			       COALESCE(SUM(choques),0)
			FROM sync_viajes WHERE sentido = ? AND dia >= date('now', ?)`, sentido, desde).
			Scan(&v.Filas, &v.Posts, &v.BytesCable, &v.BytesCrudos, &v.SinCambios, &v.Rebotes, &v.Choques)
	}
	for _, c := range []struct {
		sentido, desde string
		v              *Viaje
	}{
		{ViajeSubida, "+0 day", &r.SubidaHoy},
		{ViajeSubida, "-6 day", &r.Subida7d},
		{ViajeBajada, "+0 day", &r.BajadaHoy},
		{ViajeBajada, "-6 day", &r.Bajada7d},
	} {
		if err := sumar(c.sentido, c.desde, c.v); err != nil {
			return r, fmt.Errorf("error al sumar los viajes de %s: %w", c.sentido, err)
		}
	}

	// Las tres categorías son disjuntas por construcción: la cuarentena exige quarantined=1 y las
	// otras dos van con el predicado de visibilidad, que exige quarantined=0; y entre las dos
	// visibles decide la procedencia.
	esLLM := "provenance LIKE '" + strings.ReplaceAll(provenanceLLMPrefix, "'", "''") + "%'"
	scope, args := sc.scopeClause("")
	if err := e.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN `+visibleObsPredicate+` AND scope = 'local' AND NOT (`+esLLM+`) THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN quarantined = 1 AND archived = 0 AND superseded_by IS NULL THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN `+visibleObsPredicate+` AND scope = 'local' AND `+esLLM+` THEN 1 ELSE 0 END), 0)
		FROM observations WHERE 1 = 1`+scope, args...).Scan(&r.NoViajan.Locales, &r.NoViajan.EnCuarentena, &r.NoViajan.LLMCorroboradasLocales); err != nil {
		return r, fmt.Errorf("error al contar lo que no viaja: %w", err)
	}
	return r, nil
}
