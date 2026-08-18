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

// Embed delega en el lote de UNO. No es comodidad: mantiene UN SOLO camino de red, así que el
// truncado, el manejo de status y el parseo no pueden divergir entre el caso simple y el lote —
// que es exactamente cómo dos rutas hermanas se separan con el tiempo sin que nadie lo note.
func (o *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	out, err := o.EmbedBatch(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(out) != 1 || len(out[0]) == 0 {
		return nil, fmt.Errorf("ollama devolvió un embedding vacío (¿modelo %q instalado?)", o.model)
	}
	return out[0], nil
}

// EmbedBatch embebe VARIOS textos en UNA llamada.
//
// CUÁNTO RINDE, MEDIDO Y SIN INFLAR: 1,37× con bge-m3 en el server (917 → 670 ms por texto). El
// tiempo TOTAL crece casi lineal con el tamaño del lote, así que el modelo en CPU NO paraleliza el
// cómputo — lo único que el lote ahorra es la ida y vuelta HTTP y el arranque por pedido.
// `/api/embed` ya aceptaba un array en `input` y ya devolvía `embeddings` como array; sólo faltaba
// usarlo. La tabla completa está en embed_backfill.go, junto al tamaño de lote que salió de ella.
//
// El orden de `embeddings` es el de `input`, y eso es lo que permite aparear por índice. Que
// vinieron TODOS lo verifica embedding.EmbedBatch, en un solo lugar.
func (o *OllamaProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	// truncate:true ⇒ Ollama recorta el input al contexto del modelo en vez de fallar con 500 "input
	// length exceeds the context length" (el corpus tiene memorias/dossiers que superan el contexto de
	// bge-m3). Robusto y model-free: Ollama trunca al límite EXACTO del modelo, sin que el server tenga
	// que adivinar un tope de caracteres/tokens.
	reqBody, err := json.Marshal(map[string]interface{}{
		"model":    o.model,
		"input":    texts,
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

	// /api/embed responde {"embeddings": [[...], [...]]} en el MISMO orden que `input`. Se devuelve
	// el lote ENTERO, sin recortar ni juzgar cuántos vinieron: quien verifica que la cuenta cierre
	// es embedding.EmbedBatch, en un solo lugar, para que ningún proveedor pueda olvidárselo.
	var out struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("error al decodificar respuesta de Ollama: %w", err)
	}
	return out.Embeddings, nil
}
