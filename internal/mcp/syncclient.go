package mcp

// syncclient.go implementa el CLIENTE de sync SALIENTE del cerebro híbrido (F2): empuja una
// fila del outbox al `musubi serve` central como un `tools/call` de `musubi_save_observation`
// remoto, por HTTP JSON-RPC. Es idempotente por id (id del request = obs_id → el UPSERT
// ON CONFLICT(id) del receptor da efecto exactly-once), y clasifica los fallos en TRANSITORIOS
// (reintentar con backoff) vs PERMANENTES (dead-letter). No suma dependencias: un solo POST,
// backoff a mano (D7), estilo internal/selfupdate/updater.go para el http.Client.

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"musubi/internal/config"
	"musubi/internal/logx"
	"musubi/internal/memory"
)

// umbralCompresionPush es a partir de cuántos bytes el push del grafo viaja comprimido. Está bien
// por debajo del tope del central (4 MiB) para dejar margen, y bien por encima de lo que pesa el
// grafo de un proyecto chico, que es justo el que puede estar hablándole a un central viejo.
const umbralCompresionPush = 1 << 20 // 1 MiB

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

// NewSyncClient construye el cliente desde la config. Resuelve el token con
// config.SecretoDeEnv(AuthTokenEnv), que acepta la variable directa o el archivo `<VAR>_FILE`.
// Rechaza (errPermanent) un CentralURL http:// cuando AllowInsecureToken es false, para no filtrar
// el token en texto plano (R9), y también un auth_token_env declarado que no resuelve a nada.
// Devuelve error si la URL es inválida.
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
		v, err := config.SecretoDeEnv(cfg.AuthTokenEnv)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", errPermanent, err)
		}
		// FALLAR CERRADO, igual que nuevoEmpujadorOTLP. Sin esta guarda el cliente se construía
		// con token vacío, salía a la red SIN cabecera Authorization y el central contestaba 401
		// en cada drain — para siempre, con la configuración «puesta» y nadie mirando. El
		// 2026-09-05 eso corrió durante horas a razón de un 401 cada 30 s (cabo A89).
		if v == "" {
			return nil, fmt.Errorf("%w: sync.auth_token_env nombra a %s y no hay valor; el drain saldría sin credencial y el central contestaría 401 para siempre. Exportála, o sacá auth_token_env si el central no autentica",
				errPermanent, config.NombresDeSecreto(cfg.AuthTokenEnv))
		}
		token = v
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

// PushGraph empuja el grafo de código COMPLETO de un proyecto al central (Track 20 · F6): un
// tools/call remoto de musubi_codegraph_push, espejo de Push. NO manda project_id: el central lo
// atribuye por el principal del token (un write=own lo ignoraría igual, y no asertar proyecto es lo
// más seguro). Devuelve error clasificado (transitorio/permanente) como el resto del sync; el caller
// (pushCodeGraphToCentral) lo trata best-effort y no rompe el index.
//
// Lleva nodos, aristas y GISTS. Los gists se sumaron el 2026-08-12: hasta entonces el push
// federaba sólo la estructura y el central quedaba con `code_memory` en CERO (medido: 4.862 nodos
// contra 0 gists), así que `musubi_recall_code` contra el cerebro compartido no tenía nada que
// devolver. El campo va SIEMPRE, aunque esté vacío — es lo que le dice al receptor "reemplazá los
// míos"; un central viejo ignora la clave desconocida y se comporta como antes.
func (c *SyncClient) PushGraph(nodes []memory.GraphNode, edges []memory.GraphEdge, gists []memory.CodeMemory) error {
	if gists == nil {
		gists = []memory.CodeMemory{}
	}
	reqBody := struct {
		JsonRpc string `json:"jsonrpc"`
		ID      string `json:"id"`
		Method  string `json:"method"`
		Params  struct {
			Name      string `json:"name"`
			Arguments struct {
				Nodes []memory.GraphNode  `json:"nodes"`
				Edges []memory.GraphEdge  `json:"edges"`
				Gists []memory.CodeMemory `json:"gists"`
			} `json:"arguments"`
		} `json:"params"`
	}{JsonRpc: "2.0", ID: "codegraph-push", Method: "tools/call"}
	reqBody.Params.Name = "musubi_codegraph_push"
	reqBody.Params.Arguments.Nodes = nodes
	reqBody.Params.Arguments.Edges = edges
	reqBody.Params.Arguments.Gists = gists

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("%w: serializar push del grafo: %v", errPermanent, err)
	}

	// El grafo entero va en UN solo POST, y el central topea el body en 4 MiB. Un proyecto
	// mediano ya se pasa: medido el 2026-08-14 contra el repo de Musubi, el central rechazaba
	// con -32700 y el push moría en silencio (best-effort ⇒ sólo `federated:false`).
	//
	// Se comprime SÓLO por encima del umbral, no siempre, y eso es deliberado: un central viejo
	// no entiende Content-Encoding: gzip, así que comprimir de entrada rompería los pushes chicos
	// que hoy SÍ funcionan. Por debajo del umbral el comportamiento queda idéntico al de antes;
	// por encima, hoy no funciona nada, así que no hay nada que romper. Efecto lateral: el
	// central hay que actualizarlo ANTES que los clientes que lo empujan.
	crudo := len(payload)
	comprimido := false
	if crudo > umbralCompresionPush {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if _, werr := zw.Write(payload); werr != nil {
			return fmt.Errorf("%w: comprimir push del grafo: %v", errPermanent, werr)
		}
		if cerr := zw.Close(); cerr != nil {
			return fmt.Errorf("%w: cerrar el gzip del push: %v", errPermanent, cerr)
		}
		payload = buf.Bytes()
		comprimido = true
		logx.Info("federación del grafo: payload comprimido", "crudo_bytes", crudo, "gzip_bytes", len(payload),
			"nodos", len(nodes), "aristas", len(edges), "gists", len(gists))
	}
	// Si NI COMPRIMIDO entra, el arreglo ya no es este: hay que trocear el push. Se dice acá y con
	// los dos números, para que el próximo que lo vea no tenga que reproducirlo para enterarse.
	if len(payload) > maxRequestBody {
		return fmt.Errorf("%w: el grafo no entra en un POST (%d bytes crudos, %d comprimidos, tope %d) — hace falta trocear el push",
			errPermanent, crudo, len(payload), maxRequestBody)
	}

	ctx, cancel := context.WithTimeout(context.Background(), c.http.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("%w: construir push del grafo: %v", errPermanent, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if comprimido {
		req.Header.Set("Content-Encoding", "gzip")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
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
		// Misma disciplina que classifyResponse: permanente SÓLO si el central RECHAZÓ el pedido
		// (params / tool / credencial). Un fallo INTERNO suyo (-32603: SQLITE_BUSY, disco) o la
		// cuota se reintentan — antes cortaban el pull y la máquina se quedaba sin bajar memoria.
		kind := errTransient
		if permanentRPCCodes[rpcResp.Error.Code] {
			kind = errPermanent
		}
		return nil, afterRowID, fmt.Errorf("%w: pull JSON-RPC %d: %s", kind, rpcResp.Error.Code, rpcResp.Error.Message)
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

// permanentRPCCodes enumera los errores del central que NO se arreglan reintentando el MISMO
// payload: la request está mal formada, la tool no existe, o la credencial no alcanza para esa
// escritura. Todo lo demás es TRANSITORIO — en particular -32603 (fallo INTERNO del central:
// SQLITE_BUSY por contención, disco, etc.), que es un fallo del servidor, no del pedido.
//
// La lista es de PERMANENTES, no de transitorios, y esa forma es el punto. Antes era al revés
// —TODO permanente salvo la cuota— y eso hacía que un SQLITE_BUSY del central mandara la
// observación a DEAD-LETTER: memoria perdida en silencio, sin reintentar una sola vez. Y salta
// justo en el sync inicial grande de una máquina nueva, que es cuando más contención hay y cuando
// menos perdonable es perder memoria.
//
// La asimetría manda: reintentar de más es barato y ACOTADO (el outbox corta solo al llegar a
// max_attempts); tirar la memoria es irreversible. Ante un error que no conocemos, se reintenta.
//
// La cuota (-32002) ya se había carveado a mano por esta misma razón (Track 19), caso por caso.
// Esto arregla la FORMA, no un caso más: cualquier código nuevo del central nace transitorio.
var permanentRPCCodes = map[int]bool{
	codeParseError:     true,
	codeInvalidRequest: true,
	codeMethodNotFound: true,
	codeInvalidParams:  true,
	// -32001: rol insuficiente, o la id pertenece a otro tenant (ErrCrossTenant). En ambos casos
	// reenviar lo mismo no cambia nada: el caller tiene que corregir el payload o la credencial.
	codeUnauthorized: true,
}

// classifyResponse traduce la respuesta HTTP+JSON-RPC a nil / errTransient / errPermanent
// (R10, D7). Éxito ⇔ 200 + result + sin error JSON-RPC. 5xx/429 → transitorio; otro no-2xx →
// permanente. Un error JSON-RPC es permanente SÓLO si está en permanentRPCCodes (ver arriba).
// Un body ilegible o no-JSON en un 200 se trata como transitorio (el POST pudo llegar).
func classifyResponse(resp *http.Response) error {
	switch {
	case resp.StatusCode == http.StatusOK:
		var body syncRPCResponse
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			return fmt.Errorf("%w: respuesta 200 ilegible del central: %v", errTransient, err)
		}
		if body.Error != nil {
			if permanentRPCCodes[body.Error.Code] {
				return fmt.Errorf("%w: el central RECHAZÓ la entrega (JSON-RPC %d): %s",
					errPermanent, body.Error.Code, body.Error.Message)
			}
			// Fallo del central procesando un pedido válido (incl. -32603 SQLITE_BUSY y -32002
			// cuota): se libera solo. Reintentar con backoff; el outbox corta a max_attempts.
			return fmt.Errorf("%w: el central FALLÓ al procesar (JSON-RPC %d): %s",
				errTransient, body.Error.Code, body.Error.Message)
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
