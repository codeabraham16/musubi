package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"musubi/internal/cognition"
	"musubi/internal/embedding"
	"musubi/internal/logx"
	"musubi/internal/memory"
)

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
