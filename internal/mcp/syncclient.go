package mcp

// syncclient.go implementa el CLIENTE de sync SALIENTE del cerebro híbrido (F2): empuja una
// fila del outbox al `musubi serve` central como un `tools/call` de `musubi_save_observation`
// remoto, por HTTP JSON-RPC. Es idempotente por id (id del request = obs_id → el UPSERT
// ON CONFLICT(id) del receptor da efecto exactly-once), y clasifica los fallos en TRANSITORIOS
// (reintentar con backoff) vs PERMANENTES (dead-letter). No suma dependencias: un solo POST,
// backoff a mano (D7), estilo internal/selfupdate/updater.go para el http.Client.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// errTransient marca un fallo reintentar-able (red/timeout/5xx/429): la fila vuelve a
// 'pending' con backoff. errPermanent marca un fallo que NO se reintenta (4xx de params,
// error JSON-RPC, config inválida): la fila va a dead-letter. El scheduler decide el mark
// según cuál sea (errors.Is).
var (
	errTransient = errors.New("fallo transitorio de sync")
	errPermanent = errors.New("fallo permanente de sync")
)

// SyncClient empuja filas del outbox al cerebro central. Se construye una vez desde SyncConfig
// (en cmd/musubi) y lo comparte el scheduler. El token se resuelve de la env var nombrada en
// la config EN CONSTRUCCIÓN (no se guarda el nombre, sólo el valor ya resuelto), y nunca se
// loguea. url es la base ya con el path /mcp.
type SyncClient struct {
	url   string
	token string
	http  *http.Client
}

// NewSyncClient construye el cliente desde la config. Resuelve el token desde os.Getenv(
// AuthTokenEnv). Rechaza (errPermanent) un CentralURL http:// cuando AllowInsecureToken es
// false, para no filtrar el token en texto plano (R9). Devuelve error si la URL es inválida.
func NewSyncClient(cfg config.SyncConfig) (*SyncClient, error) {
	base := strings.TrimRight(strings.TrimSpace(cfg.CentralURL), "/")
	if base == "" {
		return nil, fmt.Errorf("%w: central_url vacío", errPermanent)
	}
	isHTTPS := strings.HasPrefix(strings.ToLower(base), "https://")
	isHTTP := strings.HasPrefix(strings.ToLower(base), "http://")
	if !isHTTPS && !isHTTP {
		return nil, fmt.Errorf("%w: central_url debe ser http(s): %q", errPermanent, cfg.CentralURL)
	}
	if !isHTTPS && !cfg.AllowInsecureToken {
		return nil, fmt.Errorf("%w: central_url no es https y allow_insecure_token está desactivado; el token viajaría en texto plano", errPermanent)
	}
	token := ""
	if cfg.AuthTokenEnv != "" {
		token = os.Getenv(cfg.AuthTokenEnv)
	}
	timeout := cfg.RequestTimeoutSeconds
	if timeout <= 0 {
		timeout = 30
	}
	return &SyncClient{
		url:   base + "/mcp",
		token: token,
		http:  &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}, nil
}

// syncRPCRequest es el sobre JSON-RPC 2.0 que se emite por fila. id = obs_id (no notificación:
// exige respuesta, y ese id es la clave de idempotencia end-to-end).
type syncRPCRequest struct {
	JsonRpc string         `json:"jsonrpc"`
	ID      string         `json:"id"`
	Method  string         `json:"method"`
	Params  syncCallParams `json:"params"`
}

type syncCallParams struct {
	Name      string            `json:"name"`
	Arguments syncSaveArguments `json:"arguments"`
}

// syncSaveArguments son los argumentos de musubi_save_observation remoto. scope va SIEMPRE
// "shared": el receptor guarda la obs ya compartida. project_id es el del proyecto de ORIGEN:
// desde Track 16 F1 el central lo PRESERVA (atribución multi-tenant) en vez de estampar el suyo.
type syncSaveArguments struct {
	ID         string  `json:"id"`
	TopicKey   string  `json:"topic_key"`
	Content    string  `json:"content"`
	Importance float64 `json:"importance"`
	MemType    string  `json:"mem_type,omitempty"`
	ProjectID  string  `json:"project_id,omitempty"`
	Scope      string  `json:"scope"`
}

// syncRPCResponse es el sobre de respuesta. Éxito ⇔ HTTP 200 + result presente + error ausente.
type syncRPCResponse struct {
	JsonRpc string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcErrorBody   `json:"error,omitempty"`
}

type rpcErrorBody struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Push empuja un item del outbox al central. Devuelve nil en éxito (R10: HTTP 200 + result sin
// error). Ante fallo devuelve un error envuelto en errTransient (reintentar) o errPermanent
// (dead-letter), según classifyErr. El contexto acota la vida del request (además del timeout
// del http.Client).
func (c *SyncClient) Push(item memory.OutboxItem) error {
	reqBody := syncRPCRequest{
		JsonRpc: "2.0",
		ID:      item.ObsID,
		Method:  "tools/call",
		Params: syncCallParams{
			Name: "musubi_save_observation",
			Arguments: syncSaveArguments{
				ID:         item.ObsID,
				TopicKey:   item.TopicKey,
				Content:    item.Content,
				Importance: item.Importance,
				MemType:    item.MemType,
				ProjectID:  item.ProjectID,
				Scope:      memory.ScopeShared,
			},
		},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		// Un payload que no serializa es un defecto de datos, no algo que reintentar.
		return fmt.Errorf("%w: no se pudo serializar el request de sync: %v", errPermanent, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.http.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("%w: no se pudo construir el request de sync: %v", errPermanent, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		// Error de transporte (conexión rechazada, DNS, timeout del contexto): transitorio.
		return fmt.Errorf("%w: %v", errTransient, err)
	}
	defer resp.Body.Close()
	return classifyResponse(resp)
}

// syncPullArguments son los argumentos del musubi_sync_pull remoto (sync ENTRANTE C5.3b).
type syncPullArguments struct {
	AfterRowID int64 `json:"after_rowid"`
	Limit      int   `json:"limit"`
}

// pullPayload es el JSON que el tool devuelve DENTRO de content[0].text: el lote + el cursor.
type pullPayload struct {
	Items      []memory.SharedObs `json:"items"`
	NextCursor int64              `json:"next_cursor"`
}

// Pull baja un lote de la memoria 'shared' del proyecto DESDE el central (sync ENTRANTE, C5.3b): un
// tools/call remoto de musubi_sync_pull con el cursor afterRowID. Devuelve los items + el cursor
// siguiente. Cualquier fallo (red, HTTP, JSON-RPC) devuelve error: el scheduler entrante lo trata
// como transitorio (reintenta en el próximo tick) — es best-effort, no rompe nada.
func (c *SyncClient) Pull(afterRowID int64, limit int) ([]memory.SharedObs, int64, error) {
	reqBody := struct {
		JsonRpc string `json:"jsonrpc"`
		ID      string `json:"id"`
		Method  string `json:"method"`
		Params  struct {
			Name      string            `json:"name"`
			Arguments syncPullArguments `json:"arguments"`
		} `json:"params"`
	}{JsonRpc: "2.0", ID: "pull", Method: "tools/call"}
	reqBody.Params.Name = "musubi_sync_pull"
	reqBody.Params.Arguments = syncPullArguments{AfterRowID: afterRowID, Limit: limit}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, afterRowID, fmt.Errorf("%w: serializar pull: %v", errPermanent, err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.http.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return nil, afterRowID, fmt.Errorf("%w: construir pull: %v", errPermanent, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, afterRowID, fmt.Errorf("%w: %v", errTransient, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, afterRowID, fmt.Errorf("%w: pull HTTP %d", errTransient, resp.StatusCode)
	}
	var rpcResp syncRPCResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return nil, afterRowID, fmt.Errorf("%w: decodificar pull: %v", errTransient, err)
	}
	if rpcResp.Error != nil {
		return nil, afterRowID, fmt.Errorf("%w: pull JSON-RPC %d: %s", errPermanent, rpcResp.Error.Code, rpcResp.Error.Message)
	}
	// result = {content:[{type,text}]}; el text es el JSON {items, next_cursor}.
	var toolResult struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(rpcResp.Result, &toolResult); err != nil || len(toolResult.Content) == 0 {
		return nil, afterRowID, fmt.Errorf("%w: pull sin content parseable", errTransient)
	}
	var pl pullPayload
	if err := json.Unmarshal([]byte(toolResult.Content[0].Text), &pl); err != nil {
		return nil, afterRowID, fmt.Errorf("%w: pull payload inválido: %v", errPermanent, err)
	}
	return pl.Items, pl.NextCursor, nil
}

// classifyResponse traduce la respuesta HTTP+JSON-RPC a nil / errTransient / errPermanent
// (R10, D7). Éxito ⇔ 200 + result + sin error JSON-RPC. 5xx/429 → transitorio; otro no-2xx o
// error JSON-RPC → permanente (params inválidos / auth). Un body ilegible o no-JSON en un 200
// se trata como transitorio (el POST pudo llegar; conviene reintentar).
func classifyResponse(resp *http.Response) error {
	switch {
	case resp.StatusCode == http.StatusOK:
		var body syncRPCResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return fmt.Errorf("%w: respuesta 200 ilegible del central: %v", errTransient, err)
		}
		if body.Error != nil {
			// Cuota excedida (-32002): el central rate-limita; se libera al pasar la ventana ⇒
			// TRANSITORIO (Track 19). Antes caía en el "permanente" de abajo y el drain
			// DEAD-LETTEREABA memoria shared por un límite temporal — pérdida de durabilidad que
			// destapó la auditoría al encender la cuota por default (T18.5).
			if body.Error.Code == codeQuotaExceeded {
				return fmt.Errorf("%w: el central rate-limita por cuota: %s", errTransient, body.Error.Message)
			}
			// El central procesó pero rechazó: params inválidos / tool desconocida. No se
			// arregla reintentando lo mismo → permanente.
			return fmt.Errorf("%w: el central devolvió error JSON-RPC %d: %s", errPermanent, body.Error.Code, body.Error.Message)
		}
		if len(body.Result) == 0 {
			return fmt.Errorf("%w: respuesta 200 sin result ni error", errTransient)
		}
		return nil
	case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
		return fmt.Errorf("%w: el central devolvió HTTP %d", errTransient, resp.StatusCode)
	default:
		// 4xx (400/401/403/404, etc.): request mal formado o no autorizado → permanente.
		return fmt.Errorf("%w: el central devolvió HTTP %d", errPermanent, resp.StatusCode)
	}
}
