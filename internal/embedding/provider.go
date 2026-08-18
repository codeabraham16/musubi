// Package embedding genera vectores de embedding a partir de texto.
// La capa MCP usa un Provider para que los agentes guarden/busquen con TEXTO,
// no con vectores crudos.
package embedding

import (
	"context"
	"errors"
	"fmt"
)

// ErrEmbeddingDisabled se devuelve cuando no hay un proveedor de embeddings configurado.
var ErrEmbeddingDisabled = errors.New("embeddings deshabilitados: configura embedding.provider en .musubi/config.yaml (ej. ollama) o usá la búsqueda por palabra clave")

// Provider genera embeddings para texto.
type Provider interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	Dimensions() int
	Name() string
}

// BatchProvider lo implementa el proveedor que sabe embeber VARIOS textos en UNA llamada.
//
// Es una interfaz aparte y OPCIONAL a propósito: agregarle el método a Provider obligaría a los
// seis implementadores —incluidos los espías de los tests— a escribir un lote que la mayoría no
// tiene. Quien no la implementa sigue andando por el bucle de EmbedBatch, más lento y correcto.
type BatchProvider interface {
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
}

// EmbedBatch embebe varios textos: usa el lote NATIVO si el proveedor lo tiene, y cae a un bucle
// si no. Es el único punto por el que se pide un lote, así que la garantía de abajo vale siempre.
//
// ⚠️ LA GARANTÍA QUE NO SE PUEDE PERDER: devuelve EXACTAMENTE len(texts) vectores, en el MISMO
// orden. El caller aparea out[i] con texts[i] para escribir en la base, así que un lote que
// devuelve de menos no produce un error visible: CORRE LOS VECTORES UNA POSICIÓN y le escribe a
// cada observación el embedding de otra. La memoria queda semánticamente barajada, y nada falla.
// Por eso el chequeo es un error duro y no un warning, y por eso vive acá y no en cada caller.
//
// Un vector VACÍO en una posición sí es válido: significa «este texto no se pudo embeber»
// (proveedor apagado, texto vacío), y el caller lo saltea contando el skip.
func EmbedBatch(ctx context.Context, p Provider, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if bp, ok := p.(BatchProvider); ok {
		out, err := bp.EmbedBatch(ctx, texts)
		if err != nil {
			return nil, err
		}
		if len(out) != len(texts) {
			return nil, fmt.Errorf("el proveedor %s devolvió %d vectores para %d textos: se aborta el lote antes de aparear vectores con observaciones equivocadas",
				p.Name(), len(out), len(texts))
		}
		return out, nil
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := p.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
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
