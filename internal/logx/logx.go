// Package logx provee un logger estructurado que escribe SIEMPRE a stderr.
// Nunca debe escribir a stdout: ese canal está reservado para el protocolo
// JSON-RPC del daemon MCP, y cualquier escritura espuria lo corrompería.
package logx

import (
	"io"
	"log/slog"
	"os"
	"sync/atomic"
)

// EL LOGGER VIVE EN UN atomic.Pointer Y NO EN UNA VARIABLE SUELTA por la costura de pruebas de
// más abajo: `Capturar` lo cambia mientras el resto del proceso puede estar logueando desde otra
// goroutine (el empuje OTLP y el barrido de flota corren en las suyas). Con una variable pelada
// eso es una carrera de manual, y la pagaríamos justo en las pruebas que existen para atrapar
// fallas silenciosas. El costo es una carga atómica por línea de log.
var logger atomic.Pointer[slog.Logger]

func init() {
	logger.Store(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
}

// Warn registra un evento de nivel advertencia.
func Warn(msg string, args ...any) {
	logger.Load().Warn(msg, args...)
}

// Info registra un evento informativo.
func Info(msg string, args ...any) {
	logger.Load().Info(msg, args...)
}

// Error registra un error.
func Error(msg string, args ...any) {
	logger.Load().Error(msg, args...)
}

// Capturar redirige el logger a w y devuelve la función que lo restaura. ES SÓLO PARA PRUEBAS.
//
// POR QUÉ EXISTE: hay avisos cuyo ÚNICO efecto observable es la línea de log, y eso no es un
// descuido sino una decisión. El empuje OTLP que se queda mudo porque alguien le sacó la concesión
// `metrics` de principals.yaml (A50) NO cuenta un fallo a propósito: `musubi_push_failures_total`
// significa «no llegó a destino», y acá ni se intentó llegar; ensuciarlo rompería la regla que
// distingue «se cayó» de «nunca anduvo». Así que el aviso es todo lo que hay.
//
// Sin esta costura, una prueba de ese aviso sólo puede mirar la contabilidad interna del «avisar
// una vez» — y entonces borrar la línea de log deja la suite en VERDE con el operador a ciegas,
// que es exactamente la clase de bug que este track persigue.
//
// No relaja el invariante del paquete: el logger de producción se arma en init() contra os.Stderr
// y ningún camino de producción llama acá. Quien la use tiene que restaurar con defer.
func Capturar(w io.Writer) (restaurar func()) {
	previo := logger.Load()
	logger.Store(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})))
	return func() { logger.Store(previo) }
}
