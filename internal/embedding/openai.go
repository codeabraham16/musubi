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

// Valores por defecto del proveedor OpenAI-compatible.
const (
	defaultOpenAIBaseURL = "https://api.openai.com/v1"
	defaultOpenAIModel   = "text-embedding-3-small"
)

// OpenAIProvider genera embeddings vía la API de OpenAI o cualquier servidor
// compatible con su esquema (LM Studio, vLLM, LocalAI, Together, etc.).
// Endpoint: POST {base_url}/embeddings  body {"model","input"[,"dimensions"]}
// -> {"data":[{"embedding":[...]}]}. La autenticación es Bearer con la API key.
type OpenAIProvider struct {
	baseURL string
	model   string
	apiKey  string
	dim     int
	client  *http.Client
}

// NewOpenAIProvider crea un proveedor OpenAI-compatible. baseURL y model caen a
// los valores por defecto de OpenAI si vienen vacíos; apiKey puede estar vacía
// para servidores locales compatibles que no exigen autenticación.
func NewOpenAIProvider(baseURL, model, apiKey string, dim int) *OpenAIProvider {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = defaultOpenAIBaseURL
	}
	if strings.TrimSpace(model) == "" {
		model = defaultOpenAIModel
	}
	return &OpenAIProvider{
		baseURL: baseURL,
		model:   model,
		apiKey:  apiKey,
		dim:     dim,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Name devuelve la PROCEDENCIA del vector INCLUYENDO el modelo concreto ("openai:<model>"), no
// sólo el provider (T17.3). Sin el modelo, dos modelos OpenAI-compatibles de igual dimensión
// (p.ej. text-embedding-3-small vs un modelo local a 1536) compartían el model_id "openai" y se
// MEZCLABAN en silencio en la búsqueda por coseno; con el modelo en la procedencia, la regla de
// homogeneidad las separa. NewOpenAIProvider siempre setea un modelo (default), así que el
// fallback es defensivo.
func (o *OpenAIProvider) Name() string {
	if strings.TrimSpace(o.model) == "" {
		return "openai"
	}
	return "openai:" + o.model
}
func (o *OpenAIProvider) Dimensions() int { return o.dim }

func (o *OpenAIProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	payload := map[string]interface{}{
		"model": o.model,
		"input": text,
	}
	// Los modelos v3 (text-embedding-3-*) permiten truncar la dimensión del
	// vector con el parámetro "dimensions". Solo lo enviamos si está configurado.
	if o.dim > 0 {
		payload["dimensions"] = o.dim
	}
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("error al serializar pedido a OpenAI: %w", err)
	}

	url := strings.TrimRight(o.baseURL, "/") + "/embeddings"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("error al construir pedido a OpenAI: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("error al llamar a OpenAI en %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// La API devuelve {"error":{"message":...}}; extraemos el mensaje si está.
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("OpenAI devolvió status %d: %s", resp.StatusCode, apiErr.Error.Message)
		}
		return nil, fmt.Errorf("OpenAI devolvió status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("error al decodificar respuesta de OpenAI: %w", err)
	}
	if len(out.Data) == 0 || len(out.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("OpenAI devolvió un embedding vacío (¿modelo %q válido?)", o.model)
	}
	return out.Data[0].Embedding, nil
}
