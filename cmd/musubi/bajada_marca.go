package main

import (
	"strconv"
	"strings"
	"time"

	"musubi/internal/memory"
)

// umbralMarcaDeBajada es cuánto tiene que tener la marca de actividad vigente para que el turno la
// vuelva a escribir: con una más nueva, el turno no escribe nada.
//
// 15 s ES MEDIO TICK DE LA BAJADA (30 s en todos los configs que escribe el alta). Con turnos
// seguidos la marca cambia igual dentro de cada tick —se reescribe a lo sumo 15 s después del
// último cambio más lo que tarde el turno siguiente—, así que con actividad continua el dueño del
// candado sigue bajando en cada tick. Lo que ahorra son las ráfagas: los avisos del sistema y los
// «sigue» llegan de a varios por minuto, y cada uno sería una escritura más sobre una base que
// comparten varios daemons con _txlock=immediate. El costo: un turno que llega menos de 15 s después
// de una marca que el dueño ya leyó no lo despierta de nuevo, y la próxima bajada llega hasta un tick
// más tarde. Con un tick menor que 30 s el umbral pesa más: la actividad continua baja cada dos ticks.
const umbralMarcaDeBajada = 15 * time.Second

// marcarActividadParaLaBajada anota en la base que alguien está trabajando (memory.MetaDespertarBajada),
// para que el dueño del candado de la bajada —casi nunca el daemon de esta terminal— vuelva a pedirle
// al central en su próximo tick en vez de esperar el tope del espaciado (ver internal/mcp/bajada_ritmo.go).
//
// LA LLAMA turnOutputConTareas CON TODO PROMPT NO VACÍO, antes de la compuerta: un «sigue» o un
// <task-notification> no son consultas, pero en esos turnos la persona está trabajando, y una marca
// detrás de la compuerta dormiría la bajada justo entonces. Best-effort: el turno no depende de esto.
func marcarActividadParaLaBajada(store turnStore) {
	marcarActividadParaLaBajadaEn(store, time.Now())
}

// marcarActividadParaLaBajadaEn es marcarActividadParaLaBajada con el reloj afuera, para las pruebas.
// Una marca que no se entiende, o que está en el futuro, se reescribe: no puede trabar la señal.
func marcarActividadParaLaBajadaEn(store turnStore, ahora time.Time) {
	if raw, ok, err := store.GetMeta(memory.MetaDespertarBajada); err == nil && ok {
		if t, perr := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); perr == nil {
			if edad := ahora.Unix() - t; edad >= 0 && edad < int64(umbralMarcaDeBajada/time.Second) {
				return
			}
		}
	}
	_ = store.SetMeta(memory.MetaDespertarBajada, strconv.FormatInt(ahora.Unix(), 10))
}
