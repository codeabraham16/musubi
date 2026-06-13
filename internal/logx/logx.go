// Package logx provee un logger estructurado que escribe SIEMPRE a stderr.
// Nunca debe escribir a stdout: ese canal está reservado para el protocolo
// JSON-RPC del daemon MCP, y cualquier escritura espuria lo corrompería.
package logx

import (
	"log/slog"
	"os"
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// Warn registra un evento de nivel advertencia.
func Warn(msg string, args ...any) {
	logger.Warn(msg, args...)
}

// Info registra un evento informativo.
func Info(msg string, args ...any) {
	logger.Info(msg, args...)
}

// Error registra un error.
func Error(msg string, args ...any) {
	logger.Error(msg, args...)
}
