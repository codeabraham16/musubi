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
// (se usa /api/embed, no el deprecado /api/embeddings, porque acepta un array en `input` y por
// tanto el lote). Ojo con la creencia vieja de que `truncate` resuelve el texto largo: NO lo hace
// —ver el detalle medido en EmbedBatch—; de eso se ocupa el troceo de trozos.go.
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
		// Sin tope en el cliente: el plazo lo pone cada pedido según cuánto texto lleva (plazoPara).
		// Un Timeout acá se aplicaría por igual al pedido de un renglón y al de un dossier, que es
		// justo el defecto que se está sacando. Ningún pedido sale sin vencimiento: EmbedBatch le
		// pone uno al contexto cuando el caller no trajo el suyo.
		client: &http.Client{},
	}
}

// plazoPara calcula cuánto esperar por un pedido, a partir de cuánto texto lleva.
//
// LOS NÚMEROS SALEN DE MEDIR CONTRA EL EMBEBEDOR REAL (bge-m3 en el server, 2026-08-18), no de
// elegir uno cómodo. El tiempo es lineal en caracteres, ~0,5–0,8 ms cada uno:
//
//	 1.000 caracteres ->  0,8 s
//	16.000            ->  8,1 s
//	48.000            -> 27,8 s
//	96.000            -> 65,5 s
//
// El margen es de ~3× sobre lo medido, porque el server es compartido y el modelo puede estar
// frío. La base cubre el arranque del pedido y NO baja de los 30 s que había: un plazo nuevo no
// puede volverse más estricto que el viejo para los pedidos que ya andaban bien.
//
// El tope existe para que un embebedor colgado se note. Sin él, el plazo crece con el texto y una
// corrida grande podría esperar indefinidamente sin que nadie sospeche.
func plazoPara(texts []string) time.Duration {
	total := 0
	for _, t := range texts {
		total += len(t)
	}
	plazo := plazoBase + time.Duration(total)*plazoPorCaracter
	if plazo > plazoMaximo {
		return plazoMaximo
	}
	return plazo
}

const (
	plazoBase = 30 * time.Second
	// 2 ms por carácter ≈ 3× el peor caso medido (0,682 ms/car en el pedido de 96.000).
	// ⚠️ La unidad importa y es fácil de errar: son MILIsegundos por carácter. Con microsegundos
	// el plazo de un pedido de 96.000 caracteres crecería 0,24 s en vez de 192 s — o sea que
	// quedaría igual que el fijo de antes, y el arreglo sería puro comentario. Hay un test que
	// compara contra los segundos MEDIDOS justamente para atajar eso.
	plazoPorCaracter = 2 * time.Millisecond
	plazoMaximo      = 10 * time.Minute
)

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
	// ⚠️ `truncate:true` NO ALCANZA, Y LA NOTA VIEJA DECÍA QUE SÍ. Se creía que Ollama recortaba al
	// contexto del modelo y por eso el texto largo era problema resuelto. Medido contra el ollama
	// del central (bge-m3, 2026-08-18): hay una FRANJA de largos que devuelve 400 igual —con
	// `true`, con `false`, sin el campo y con `num_ctx` explícito— y por encima de ella el recorte
	// ocurre EN SILENCIO, dejando un vector del primer pedazo presentado como si fuera del
	// documento entero. Quien protege de verdad es el troceo de trozos.go; esto queda como red.
	reqBody, err := json.Marshal(map[string]interface{}{
		"model":    o.model,
		"input":    texts,
		"truncate": true,
	})
	if err != nil {
		return nil, fmt.Errorf("error al serializar pedido a Ollama: %w", err)
	}

	// EL PLAZO SE CALCULA CON LO QUE SE PIDE, NO SE FIJA DE ANTEMANO. Antes eran 30 s para
	// cualquier pedido, y el costo del embebedor es lineal en caracteres: 30 s alcanzan para unos
	// 50.000, así que un lote de 16 textos de 3.000 quedaba al filo y uno de 6.000 se pasaba
	// siempre. Un plazo que no mira el tamaño no es un plazo, es una lotería.
	// Si el caller ya puso su propio vencimiento, manda el suyo: acá no se afloja lo que otro apretó.
	if _, hay := ctx.Deadline(); !hay {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, plazoPara(texts))
		defer cancel()
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
