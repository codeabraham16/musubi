package cognition

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

// defaultCognitionTimeout es el timeout por llamada si la config no fija RequestTimeoutSeconds.
// Generoso a propósito: la cognición a-demanda tolera latencia (el agente eligió esperar), a
// diferencia del recall model-free que es el camino caliente.
const defaultCognitionTimeout = 120 * time.Second

// OpenAICompatProvider es el motor de cognición que habla con un endpoint OpenAI-compatible de
// CHAT (POST {endpoint}/chat/completions). Sirve para cualquier backend que exponga ese esquema:
// el endpoint cabinlab/litellm que respalda la suscripción Max por el Agent SDK (tailnet, loopback),
// LiteLLM, vLLM, LocalAI, etc. La autenticación es Bearer con la master key del proxy (NUNCA la
// credencial cruda de la suscripción: esa vive dentro del proxy). El pilar SIGUE model-free por
// default; este motor es el acelerador OPT-IN que las tools a-demanda consumen.
type OpenAICompatProvider struct {
	endpoint string
	model    string
	apiKey   string
	client   *http.Client
}

// NewOpenAICompatProvider crea el motor. endpoint es la base OpenAI-compatible (ej.
// http://127.0.0.1:4000/v1); model es el alias que expone el proxy (ej. "sonnet"); apiKey es la
// master key del proxy (puede quedar vacía para backends locales sin auth). timeout<=0 ⇒ default.
func NewOpenAICompatProvider(endpoint, model, apiKey string, timeout time.Duration) *OpenAICompatProvider {
	if timeout <= 0 {
		timeout = defaultCognitionTimeout
	}
	return &OpenAICompatProvider{
		endpoint: endpoint,
		model:    model,
		apiKey:   apiKey,
		client:   &http.Client{Timeout: timeout},
	}
}

// Name devuelve la PROCEDENCIA incluyendo el modelo concreto ("llm:<model>"), homogéneo con el
// contrato de procedencia del pilar (una propuesta/juicio queda atribuible a un modelo puntual).
func (o *OpenAICompatProvider) Name() string {
	if strings.TrimSpace(o.model) == "" {
		return "llm"
	}
	return "llm:" + o.model
}

// Ask envía system+user como un chat de dos mensajes y devuelve el texto de la respuesta.
func (o *OpenAICompatProvider) Ask(ctx context.Context, system, user string) (string, error) {
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	msgs := make([]message, 0, 2)
	if strings.TrimSpace(system) != "" {
		msgs = append(msgs, message{Role: "system", Content: system})
	}
	msgs = append(msgs, message{Role: "user", Content: user})

	reqBody, err := json.Marshal(map[string]interface{}{
		"model":    o.model,
		"messages": msgs,
	})
	if err != nil {
		return "", fmt.Errorf("error al serializar pedido de cognición: %w", err)
	}

	url := strings.TrimRight(o.endpoint, "/") + "/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("error al construir pedido de cognición: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	resp, err := o.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("error al llamar al motor de cognición en %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// El esquema OpenAI devuelve {"error":{"message":...}}; extraemos el mensaje si está.
		var apiErr struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Error.Message != "" {
			return "", fmt.Errorf("el motor de cognición devolvió status %d: %s", resp.StatusCode, apiErr.Error.Message)
		}
		return "", fmt.Errorf("el motor de cognición devolvió status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("error al decodificar la respuesta del motor de cognición: %w", err)
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("el motor de cognición no devolvió ninguna respuesta (¿modelo %q válido en el endpoint?)", o.model)
	}
	return strings.TrimSpace(out.Choices[0].Message.Content), nil
}
