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

// defaultOllamaBaseURL es el base_url por defecto que config.Default() asigna.
// El factory lo usa para detectar que un usuario de "openai" no cambió el base_url.
const defaultOllamaBaseURL = "http://localhost:11434"

// OllamaProvider genera embeddings llamando a una instancia local de Ollama.
// Endpoint: POST {base_url}/api/embed  body {"model","input","truncate":true} -> {"embeddings":[[...]]}
// (se usa /api/embed, no el deprecado /api/embeddings, para poder pedir truncate: ante un texto más
// largo que el contexto del modelo Ollama lo RECORTA en vez de devolver 500).
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

// Name devuelve la PROCEDENCIA del vector INCLUYENDO el modelo concreto ("ollama:<model>"), no
// sólo el provider (T17.3). Sin el modelo, dos tablas distintas de Ollama de igual dimensión
// (p.ej. nomic-embed-text vs mxbai-embed-large a 768) compartían el model_id "ollama" y se
// MEZCLABAN en silencio en la búsqueda por coseno; con el modelo en la procedencia, la regla de
// homogeneidad las separa. Modelo vacío ⇒ "ollama" (defensivo; en la práctica config exige uno).
func (o *OllamaProvider) Name() string {
	if strings.TrimSpace(o.model) == "" {
		return "ollama"
	}
	return "ollama:" + o.model
}
func (o *OllamaProvider) Dimensions() int { return o.dim }

func (o *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	// truncate:true ⇒ Ollama recorta el input al contexto del modelo en vez de fallar con 500 "input
	// length exceeds the context length" (el corpus tiene memorias/dossiers que superan el contexto de
	// bge-m3). Robusto y model-free: Ollama trunca al límite EXACTO del modelo, sin que el server tenga
	// que adivinar un tope de caracteres/tokens.
	reqBody, err := json.Marshal(map[string]interface{}{
		"model":    o.model,
		"input":    text,
		"truncate": true,
	})
	if err != nil {
		return nil, fmt.Errorf("error al serializar pedido a Ollama: %w", err)
	}

	url := strings.TrimRight(o.baseURL, "/") + "/api/embed"
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
		return nil, fmt.Errorf("ollama devolvió status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// /api/embed devuelve un LOTE: {"embeddings": [[...]]}. Con un único input, el vector es el [0].
	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("error al decodificar respuesta de Ollama: %w", err)
	}
	if len(out.Embeddings) == 0 || len(out.Embeddings[0]) == 0 {
		return nil, fmt.Errorf("ollama devolvió un embedding vacío (¿modelo %q instalado?)", o.model)
	}
	return out.Embeddings[0], nil
}
