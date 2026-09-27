package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// sync_viajes.go es el INSTRUMENTO del sync: lo que movió cada tick, sumado por día y sentido
// (migración v58). Existe porque hasta acá la única forma de saber cuántos bytes costaba subir una
// nota era simular el cable sobre una copia de la base, y la única forma de saber cuántas notas
// habían salido era un contador de estado que el pull pisaba (ver IngestShared).
//
// Lo escriben los drains de internal/mcp al final de cada tick, y SÓLO si el tick salió a la red.
// La subida no escribe si no mandó nada o si ningún POST tuvo respuesta. La bajada escribe una vez
// por tick en que algún Pull volvió bien, AUNQUE la página venga vacía —así cuenta los pulls—, y
// no escribe en un tick salteado: sin el candado, cediendo tras un fallo, o con el Pull fallido.
// Sin esa regla cada tick salteado sería una escritura más sobre una base que comparten varios
// procesos, y la columna posts contaría intentos que nunca viajaron.

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
//     bajada, TODAS las páginas que llegaron bien, vacías o no: es el contador de pulls. Un pedido
//     que no llegó a tener respuesta no cuenta.
//   - BytesCable / BytesCrudos: el cuerpo de lo que viajó CON FILAS —los POST aceptados, las
//     páginas que trajeron algo—, tal como fue por el cable y descomprimido. Así
//     SUM(bytes_cable)/SUM(filas) es el peso de una nota que viaja, y no el ritmo de los sondeos.
//     Hoy no se comprime en ningún sentido y valen lo mismo; los PR de compresión los separan.
//   - Vacias / BytesVacias: las páginas de la bajada que volvieron bien SIN filas y lo que pesaron.
//     Es el costo fijo del sondeo (~135 B por página, miles por día y por base) y se mide aparte:
//     mezclado con lo anterior, el «peso por nota» medía el ritmo de pulls (medido en la revisión:
//     21.263 B por nota contra una nota real de 2.903 B) y el frente que baja ese ritmo no tendría
//     con qué verse.
//   - Rechazados / BytesRechazados: los POST de la subida que el central contestó sin aceptar la
//     nota (4xx, 5xx, un error JSON-RPC) y el cuerpo que viajó igual. Mismo motivo: un corte del
//     central reintentado N veces no es peso de nota.
//   - Rebotes, Choques: sólo en la bajada, filas (ya contadas en Filas) que encontraron acá una
//     edición local sin salir con otro contenido y NO la pisaron. Rebote si el contenido que bajó es
//     lo que esta máquina entregó por última vez; choque si no. Choques no es cota de nada: cuenta de
//     más la re-entrega de una versión de base ajena, y de menos un cambio ajeno de sólo metadatos,
//     que cae en rebotes (ver memory.Ingesta).
//   - SinCambios: nace en cero. Lo llena un PR siguiente de la ola (el central que no re-guarda lo
//     que ya tiene). Está en la tabla desde ya para no pedir otra migración.
type Viaje struct {
	Filas           int64 `json:"filas"`
	Posts           int64 `json:"posts"`
	BytesCable      int64 `json:"bytes_cable"`
	BytesCrudos     int64 `json:"bytes_crudos"`
	Vacias          int64 `json:"vacias"`
	BytesVacias     int64 `json:"bytes_vacias"`
	Rechazados      int64 `json:"rechazados"`
	BytesRechazados int64 `json:"bytes_rechazados"`
	SinCambios      int64 `json:"sin_cambios"`
	Rebotes         int64 `json:"rebotes"`
	Choques         int64 `json:"choques"`
}

// RegistrarViaje SUMA el viaje de un tick a la fila del día (UTC) y sentido. Un solo UPSERT, así
// que dos procesos que registran a la vez no se pisan: cada uno suma lo suyo.
//
// Un sentido desconocido es un error y no un default: la tabla lo rechazaría igual por el CHECK,
// pero con un mensaje de SQLite que no dice quién lo mandó.
func (e *DbEngine) RegistrarViaje(sentido string, v Viaje) error {
	return sumarViaje(e.db, "", sentido, v)
}

// RegistrarBajada es RegistrarViaje para un tick de BAJADA que salió a la red, y además anota la
// última bajada (MetaUltimaBajada) en la MISMA transacción. La llama el dueño del candado desde el
// defer de drainInboundOnce, y lo que escribe es lo que muestra la línea «bajada» de
// musubi_sync_status: el volumen sale de sync_viajes y la edad, de la meta.
//
// VAN JUNTAS POR DOS RAZONES, y cada una alcanzaba sola:
//   - Es UN commit por tick y no dos. La base la comparten varios daemons que escriben (seis en
//     davantis-1), el DSN lleva `_txlock=immediate` y el WAL corre con `synchronous` en FULL: cada
//     commit toma el candado de escritura y hace un fsync (ver latido.go, que juntó lo del latido
//     por lo mismo). La meta suma una fila a una transacción que ya existía, no una transacción.
//   - Las dos fuentes no quedan desparejas por un fallo a mitad de camino: si una se escribe, la
//     otra también. Por eso musubi_sync_status puede leer «hay viajes de bajada y ninguna meta»
//     como lo que es —los bajó un binario anterior, que registra el viaje y no la edad— en vez de
//     afirmar «nunca» con filas bajadas hoy.
//
// Y EL DÍA DEL VIAJE SALE DEL INSTANTE DE LA META, no de date('now'): así los dos caen en el mismo
// día UTC aunque el tick cruce la medianoche. Con dos relojes, un viaje de las 00:00:00 quedaría en
// un día y su meta de las 23:59:59 en el anterior, y el estado leería «bajó hoy un binario que no
// anota la edad» durante un tick por día.
//
// Las páginas de ese día se leen DESPUÉS de sumar las del tick y en la misma transacción
// (UltimaBajada.PaginasDelDia): con eso el estado ve a un binario que bajó sin anotar el mismo día
// de la meta, que por día no se distingue.
func (e *DbEngine) RegistrarBajada(v Viaje, u UltimaBajada) error {
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al abrir la transacción de la bajada: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	dia := time.Unix(u.Unix, 0).UTC().Format(time.DateOnly)
	if err := sumarViaje(tx, dia, ViajeBajada, v); err != nil {
		return err
	}
	if err := tx.QueryRow(`SELECT posts FROM sync_viajes WHERE dia = ? AND sentido = ?`, dia, ViajeBajada).Scan(&u.PaginasDelDia); err != nil {
		return fmt.Errorf("error al leer las páginas del día de la bajada: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO meta (key, value, updated_at) VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		MetaUltimaBajada, u.Valor()); err != nil {
		return fmt.Errorf("error al anotar la última bajada: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al commitear la bajada: %w", err)
	}
	return nil
}

// sumarViaje es el UPSERT de sync_viajes, sobre la base o sobre una transacción. dia es el día UTC
// en la forma de date(); vacío es el de SQLite ahora mismo, que es lo que usa RegistrarViaje.
func sumarViaje(x execQuerier, dia, sentido string, v Viaje) error {
	if sentido != ViajeSubida && sentido != ViajeBajada {
		return fmt.Errorf("sentido de viaje desconocido %q: es %q o %q", sentido, ViajeSubida, ViajeBajada)
	}
	if _, err := x.Exec(`
		INSERT INTO sync_viajes (dia, sentido, filas, posts, bytes_cable, bytes_crudos, vacias, bytes_vacias,
			rechazados, bytes_rechazados, sin_cambios, rebotes, choques)
		VALUES (COALESCE(NULLIF(?, ''), date('now')), ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(dia, sentido) DO UPDATE SET
			filas            = sync_viajes.filas            + excluded.filas,
			posts            = sync_viajes.posts            + excluded.posts,
			bytes_cable      = sync_viajes.bytes_cable      + excluded.bytes_cable,
			bytes_crudos     = sync_viajes.bytes_crudos     + excluded.bytes_crudos,
			vacias           = sync_viajes.vacias           + excluded.vacias,
			bytes_vacias     = sync_viajes.bytes_vacias     + excluded.bytes_vacias,
			rechazados       = sync_viajes.rechazados       + excluded.rechazados,
			bytes_rechazados = sync_viajes.bytes_rechazados + excluded.bytes_rechazados,
			sin_cambios      = sync_viajes.sin_cambios      + excluded.sin_cambios,
			rebotes          = sync_viajes.rebotes          + excluded.rebotes,
			choques          = sync_viajes.choques          + excluded.choques`,
		dia, sentido, v.Filas, v.Posts, v.BytesCable, v.BytesCrudos, v.Vacias, v.BytesVacias,
		v.Rechazados, v.BytesRechazados, v.SinCambios, v.Rebotes, v.Choques); err != nil {
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
	// UltimoDiaConBajada y PaginasDeEseDia son el último día (UTC) con páginas de bajada en
	// sync_viajes y cuántas tuvo, SIN la ventana de la semana. La edad de la bajada los compara con
	// la meta (MetaUltimaBajada) para ver si después bajó alguien que no la anota. No van en el
	// JSON, que lleva el volumen: la edad va sólo en el texto de musubi_sync_status.
	UltimoDiaConBajada string `json:"-"`
	PaginasDeEseDia    int64  `json:"-"`
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
			       COALESCE(SUM(bytes_crudos),0), COALESCE(SUM(vacias),0), COALESCE(SUM(bytes_vacias),0),
			       COALESCE(SUM(rechazados),0), COALESCE(SUM(bytes_rechazados),0),
			       COALESCE(SUM(sin_cambios),0), COALESCE(SUM(rebotes),0), COALESCE(SUM(choques),0)
			FROM sync_viajes WHERE sentido = ? AND dia >= date('now', ?)`, sentido, desde).
			Scan(&v.Filas, &v.Posts, &v.BytesCable, &v.BytesCrudos, &v.Vacias, &v.BytesVacias,
				&v.Rechazados, &v.BytesRechazados, &v.SinCambios, &v.Rebotes, &v.Choques)
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
	// Sin fila no es un error: esta base nunca registró una bajada (el central, o un nodo nuevo).
	switch err := e.db.QueryRowContext(ctx, `SELECT dia, posts FROM sync_viajes
		WHERE sentido = ? AND posts > 0 ORDER BY dia DESC LIMIT 1`, ViajeBajada).Scan(&r.UltimoDiaConBajada, &r.PaginasDeEseDia); {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return r, fmt.Errorf("error al leer el último día con bajadas: %w", err)
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
