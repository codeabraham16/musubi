// Package embedding genera vectores de embedding a partir de texto.
// La capa MCP usa un Provider para que los agentes guarden/busquen con TEXTO,
// no con vectores crudos.
package embedding

import (
	"context"
	"errors"
)

// ErrEmbeddingDisabled se devuelve cuando no hay un proveedor de embeddings configurado.
var ErrEmbeddingDisabled = errors.New("embeddings deshabilitados: configura embedding.provider en .musubi/config.yaml (ej. ollama) o usá la búsqueda por palabra clave")

// Provider genera embeddings para texto.
type Provider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimensions() int
	Name() string
}

// NoopProvider es el proveedor por defecto: no genera embeddings.
// Hace que la búsqueda semántica falle de forma explícita en lugar de
// devolver resultados vacíos en silencio.
type NoopProvider struct{}

func (NoopProvider) Embed(context.Context, string) ([]float32, error) {
	return nil, ErrEmbeddingDisabled
}

func (NoopProvider) Dimensions() int { return 0 }

func (NoopProvider) Name() string { return "none" }

// Enabled indica si el proveedor genera embeddings reales.
func Enabled(p Provider) bool {
	_, isNoop := p.(NoopProvider)
	return !isNoop
}
