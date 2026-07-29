package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"musubi/internal/cognition"
	"musubi/internal/embedding"
	"musubi/internal/logx"
	"musubi/internal/memory"
)

// defaultRerankTopK es cuántos candidatos ve el juez read-time si la config no lo fija. Acotado a
// propósito: el juez es caro (latencia + rate-limit), sólo mira el tope.
const defaultRerankTopK = 12

// rerankCache memoiza el orden que dictó el juez para un (query + set de ids) — read-time selectivo
// y cacheado (guardrail de la auditoría: estira la suscripción y protege el rate-limit compartido).
var (
	rerankCacheMu sync.Mutex
	rerankCache   = map[string][]string{}
)

const rerankCacheCap = 512

// askGroundingBudget es el techo de tokens de memoria que se recupera para fundamentar la
// respuesta. Generoso pero acotado: es el CONTEXTO que ve el LLM, no el camino caliente del recall.
const askGroundingBudget = 6000

// askTimeout es el backstop de la llamada al motor (además del timeout del propio cliente HTTP del
// provider). La cognición a-demanda tolera latencia; el agente eligió invocarla y esperar.
const askTimeout = 150 * time.Second

// toolAsk es la COGNICIÓN A-DEMANDA del 3er pilar (F3.5b): responde una pregunta en lenguaje
// natural FUNDAMENTÁNDOSE en la memoria recuperada (RAG-synthesis). A diferencia de musubi_recall
// (que devuelve gists crudos, model-free), acá el motor LLM SINTETIZA una respuesta y cita las
// memorias que la respaldan. Es OPT-IN: sin motor configurado (NoopProvider) devuelve un error
// explícito y el binario sigue model-free. NO escribe al libro mayor: es de sólo lectura.
func (s *McpServer) toolAsk(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Question    string `json:"question"`
		TokenBudget int    `json:"token_budget"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Question) == "" {
		return nil, rpcErrorf(codeInvalidParams, "question es obligatorio")
	}
	// Opt-in: si el pilar está apagado, fallar explícito (no degradar en silencio). El caller puede
	// caer a musubi_recall (model-free) que siempre está disponible.
	if !cognition.Enabled(s.cognition) {
		return nil, rpcErrorf(codeInvalidParams, "cognición no disponible: musubi_ask necesita un motor (cognition.provider en .musubi/config.yaml). Usá musubi_recall para el recall model-free.")
	}

	// 1) Grounding: recuperar la memoria relevante (mismo camino model-free que musubi_recall,
	// incluído el embedder híbrido si está y el aislamiento por proyecto derivado del principal).
	budget := askGroundingBudget
	if args.TokenBudget > 0 && args.TokenBudget <= maxRecallBudget {
		budget = args.TokenBudget
	}
	opts := memory.RecallOptions{
		TokenBudget:     budget,
		CandidatePool:   s.memory.CandidatePool,
		GistMaxTokens:   s.memory.GistMaxTokens,
		GraphCentrality: s.memory.RecallGraphCentrality,
		Cooccurrence:    s.memory.RecallCooccurrence,
		Stemming:        s.memory.RecallStemming,
		VectorFloor:     s.memory.VectorFloor,
		MMRLambda:       s.memory.MMRLambda,
	}
	opts.ProjectScope, opts.Federate = recallScopeFor(principalFrom(ctx))
	if embedding.Enabled(s.embedder) {
		embCtx, embCancel := context.WithTimeout(ctx, 30*time.Second)
		if vec, eerr := s.embedder.Embed(embCtx, args.Question); eerr != nil {
			logx.Error("ask: no se pudo embeber la pregunta, sigo solo con léxico", "error", eerr)
		} else {
			opts.QueryVector = vec
		}
		embCancel()
	}
	res, err := s.engine.Recall(ctx, args.Question, opts)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error en el recall de grounding: %v", err)
	}
	// Sin memoria relevante NO llamamos al LLM (evita alucinación y gasta una llamada en vano).
	if len(res.Items) == 0 {
		return jsonResult(map[string]interface{}{
			"answer":  "No encontré memoria relevante para esa pregunta.",
			"sources": []string{},
			"model":   s.cognition.Name(),
		})
	}

	// 2) Construir el prompt fundamentado. El system exige citar ids y admitir lo que no sabe.
	var b strings.Builder
	for _, it := range res.Items {
		age := ""
		if it.CreatedAt != "" {
			age = " · " + it.CreatedAt
		}
		fmt.Fprintf(&b, "[%s] (%s%s)\n%s\n\n", it.ID, it.TopicKey, age, it.Gist)
	}
	system := "Sos el asistente de cognición de Musubi. Respondé la PREGUNTA usando ÚNICAMENTE la MEMORIA provista. " +
		"Citá entre corchetes el id [id] de cada memoria que respalde una afirmación. " +
		"Si la memoria no alcanza para responder, DECILO explícitamente — no inventes ni completes con conocimiento externo. " +
		"Ojo con la edad de cada memoria: una nota vieja puede estar desactualizada. Sé conciso y directo."
	user := "PREGUNTA:\n" + args.Question + "\n\nMEMORIA RELEVANTE:\n" + b.String()

	// 3) Llamar al motor (a-demanda, con backstop de timeout).
	askCtx, cancel := context.WithTimeout(ctx, askTimeout)
	defer cancel()
	answer, err := s.cognition.Ask(askCtx, system, user)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "el motor de cognición falló: %v", err)
	}
	// sources = SÓLO las memorias que la respuesta realmente citó (no las ~N del grounding), para que
	// el caller pueda verificar la atribución sin ruido. grounded_on deja transparente cuántas se
	// consideraron.
	return jsonResult(map[string]interface{}{
		"answer":      answer,
		"sources":     citedSources(answer, res.Items),
		"grounded_on": len(res.Items),
		"model":       s.cognition.Name(),
	})
}

// citedSources devuelve los ids de las memorias del grounding que la respuesta CITÓ, ya sea por su id
// completo o por el prefijo de 8 hex de un uuid ([822784c1]) — la forma abreviada que suelen usar los
// LLM. Preserva el orden del recall y deduplica.
func citedSources(answer string, items []memory.RecallItem) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 8)
	for _, it := range items {
		if seen[it.ID] {
			continue
		}
		cited := strings.Contains(answer, it.ID)
		if !cited {
			// Prefijo de uuid: 8 hex seguidos de '-'. Sólo entonces el prefijo es distintivo.
			if i := strings.IndexByte(it.ID, '-'); i == 8 && isHex8(it.ID[:8]) && strings.Contains(answer, it.ID[:8]) {
				cited = true
			}
		}
		if cited {
			seen[it.ID] = true
			out = append(out, it.ID)
		}
	}
	return out
}

// isHex8 indica si s son 8 dígitos hexadecimales (el primer segmento de un uuid).
func isHex8(s string) bool {
	if len(s) != 8 {
		return false
	}
	for i := 0; i < 8; i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// rerankIfEnabled aplica el JUEZ DE PERTINENCIA read-time (F3.5c) SÓLO si está activado en la config
// y hay motor. Re-ordena los primeros TopK candidatos por relevancia a la consulta (no descarta) y
// deja el resto intacto al final. Es OPT-IN, selectivo, cacheado y BEST-EFFORT: ante cualquier fallo,
// timeout o parseo malo se devuelve el orden model-free sin tocar — el recall nunca se rompe ni se
// bloquea por el LLM. Con la flag apagada (default) es un no-op y el recall queda 100% model-free.
func (s *McpServer) rerankIfEnabled(ctx context.Context, query string, res memory.RecallResult) memory.RecallResult {
	if !s.cognitionCfg.ReadTimeRerank || !cognition.Enabled(s.cognition) || len(res.Items) < 2 {
		return res
	}
	topK := s.cognitionCfg.ReadTimeRerankTopK
	if topK <= 0 {
		topK = defaultRerankTopK
	}
	n := len(res.Items)
	if n > topK {
		n = topK
	}
	head := res.Items[:n]

	ids := make([]string, len(head))
	for i, it := range head {
		ids[i] = it.ID
	}
	key := query + "\x00" + strings.Join(ids, ",")

	order := rerankCacheGet(key)
	if order == nil {
		var b strings.Builder
		for _, it := range head {
			fmt.Fprintf(&b, "[%s] %s\n", it.ID, it.Gist)
		}
		system := "Sos un juez de pertinencia. Dada una CONSULTA y MEMORIAS candidatas, devolvé SÓLO un array JSON " +
			"con TODOS los ids ordenados de MÁS a MENOS relevante para la consulta. Nada de prosa ni explicación, " +
			"sólo el JSON. Ejemplo de formato: [\"id-a\",\"id-b\",\"id-c\"]."
		user := "CONSULTA: " + query + "\n\nMEMORIAS:\n" + b.String()

		rctx, cancel := context.WithTimeout(ctx, askTimeout)
		ans, err := s.cognition.Ask(rctx, system, user)
		cancel()
		if err != nil {
			logx.Error("rerank read-time: el motor falló, mantengo el orden model-free", "error", err)
			return res
		}
		order = parseIDArray(ans)
		if len(order) == 0 {
			logx.Error("rerank read-time: no pude parsear ids de la respuesta, mantengo el orden model-free")
			return res
		}
		rerankCachePut(key, order)
	}

	res.Items = append(reorderByIDs(head, order), res.Items[n:]...)
	return res
}

// parseIDArray extrae el array JSON de ids de la respuesta del juez (tolera prosa alrededor: toma
// del primer '[' al último ']'). Devuelve nil si no hay un array de strings parseable.
func parseIDArray(answer string) []string {
	i := strings.IndexByte(answer, '[')
	j := strings.LastIndexByte(answer, ']')
	if i < 0 || j <= i {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(answer[i:j+1]), &ids); err != nil {
		return nil
	}
	out := ids[:0]
	for _, id := range ids {
		if strings.TrimSpace(id) != "" {
			out = append(out, id)
		}
	}
	return out
}

// reorderByIDs reordena items según la secuencia de ids del juez; los ids desconocidos se ignoran y
// los items NO mencionados se preservan al final en su orden original (nunca se pierde una memoria).
func reorderByIDs(items []memory.RecallItem, order []string) []memory.RecallItem {
	byID := make(map[string]memory.RecallItem, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	out := make([]memory.RecallItem, 0, len(items))
	placed := make(map[string]bool, len(items))
	for _, id := range order {
		if it, ok := byID[id]; ok && !placed[id] {
			out = append(out, it)
			placed[id] = true
		}
	}
	for _, it := range items { // los que el juez no mencionó, en su orden original
		if !placed[it.ID] {
			out = append(out, it)
		}
	}
	return out
}

func rerankCacheGet(key string) []string {
	rerankCacheMu.Lock()
	defer rerankCacheMu.Unlock()
	return rerankCache[key]
}

func rerankCachePut(key string, order []string) {
	rerankCacheMu.Lock()
	defer rerankCacheMu.Unlock()
	if len(rerankCache) >= rerankCacheCap { // evicción tonta: vaciar al llegar al tope (read-time, barato)
		rerankCache = map[string][]string{}
	}
	rerankCache[key] = order
}
