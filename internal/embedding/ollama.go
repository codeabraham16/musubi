package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OllamaProvider genera embeddings llamando a una instancia local de Ollama.
// Endpoint: POST {base_url}/api/embeddings  body {"model","prompt"} -> {"embedding":[...]}
type OllamaProvider struct {
	baseURL string
	model   string
	dim     int
	client  *http.Client
}

// NewOllamaProvider crea un proveedor Ollama. dim es la dimensión esperada del modelo.
func NewOllamaProvider(baseURL, model string, dim int) *OllamaProvider {
	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		dim:     dim,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (o *OllamaProvider) Name() string    { return "ollama" }
func (o *OllamaProvider) Dimensions() int { return o.dim }

func (o *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody, err := json.Marshal(map[string]string{
		"model":  o.model,
		"prompt": text,
	})
	if err != nil {
		return nil, fmt.Errorf("error al serializar pedido a Ollama: %w", err)
	}

	url := strings.TrimRight(o.baseURL, "/") + "/api/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("error al construir pedido a Ollama: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error al llamar a Ollama en %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama devolvió status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("error al decodificar respuesta de Ollama: %w", err)
	}
	if len(out.Embedding) == 0 {
		return nil, fmt.Errorf("Ollama devolvió un embedding vacío (¿modelo %q instalado?)", o.model)
	}
	return out.Embedding, nil
}
