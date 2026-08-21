package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"musubi/internal/cognition"
	"musubi/internal/memory"
)

// methods_dedup.go implementa musubi_sharpen: el AFILADOR del acervo de diseño (pilar 'Musubi
// Renaissance'), el gemelo del destilador. El molino (musubi_distill) LLENA el acervo; el afilador lo
// AFILA: junta las tarjetas que dicen la misma lección con otras palabras. El destilador produce gemelas
// inevitablemente —destila artículos que se solapan, y hasta la misma lección en inglés y en castellano—,
// así que sin este pase el acervo engorda con casi-duplicados que reparten el calor del recall y ensucian
// el brief.
//
// Por qué NO alcanza el Consolidate que ya existe: ése fusiona por TRIGRAMAS (parecido léxico). Las
// gemelas del acervo son SEMÁNTICAS: "contraste mínimo 4.5:1" y "el texto necesita 4.5:1 de contraste"
// comparten la idea, no las letras (coseno ~0.85, trigramas muy por debajo del umbral). Este pase halla
// candidatas por COSENO y deja el veredicto final a un JUEZ LLM.
//
// Disciplina del pilar (igual que el destilador): OFFLINE (LLM, jamás en el camino caliente), OPT-IN
// (sin motor, falla explícito), ADMIN (escribe en el acervo compartido) y CONSERVADOR — el juez fusiona
// SÓLO si archivar una tarjeta no pierde conocimiento, ante la duda conserva, y toda fusión es un
// soft-delete REVERSIBLE (ArchiveAsDuplicate calca a Consolidate). Un par que el juez decide conservar
// (KEEP) se marca `not_duplicate` para no volver a gastarle una llamada al motor. Se corre de a tandas.

const (
	// dedupScope es el tenant del acervo (el mismo que destila y lee musubi_design).
	dedupScope = designCorpusScope
	// dedupCardPrefix son las tarjetas curadas: el afilador SÓLO junta tarjetas entre sí, nunca blobs.
	dedupCardPrefix = distillCardPrefix
	// dedupDefaultFloor es el piso de coseno para proponer un par al juez. Medido en el acervo: las
	// gemelas reales viven en 0.82-0.89 (los blobs distintos de verdad quedan debajo). El piso es un
	// FILTRO GRUESO barato; la precisión la pone el juez LLM, así que conviene un piso algo bajo (más
	// pares a juzgar) antes que perderse gemelas legítimas.
	dedupDefaultFloor = 0.84
	// dedupDefaultPairs / dedupMaxPairs acotan cuántos pares juzga una tanda. Cada par es una llamada al
	// motor; la tanda chica mantiene la latencia y la cuota manejables y se corre en bucle.
	dedupDefaultPairs = 6
	dedupMaxPairs     = 25
	// dedupAuthor es quien firma el marcador `not_duplicate`: procedencia legible del pase (el afilador).
	dedupAuthor     = "afilador"
	dedupKeepMarker = memory.RelNotDuplicate
	// dedupMerge / dedupKeep son los dos veredictos del juez.
	dedupMerge = "MERGE"
	dedupKeep  = "KEEP"
)

// dedupPairResult reporta qué pasó con UN par de la tanda.
type dedupPairResult struct {
	A       string  `json:"a"`
	B       string  `json:"b"`
	TopicA  string  `json:"topic_a"`
	TopicB  string  `json:"topic_b"`
	Cosine  float64 `json:"cosine"`
	Verdict string  `json:"verdict,omitempty"` // MERGE | KEEP (vacío en dry-run)
	Action  string  `json:"action"`            // merged | kept | dry_run | error: ...
}

// dedupReport es la respuesta de musubi_sharpen.
type dedupReport struct {
	Scanned int               `json:"scanned"` // pares candidatos por coseno en esta tanda
	Merged  int               `json:"merged"`  // pares fusionados (una tarjeta archivada)
	Kept    int               `json:"kept"`    // pares que el juez conservó (marcados not_duplicate)
	Motor   string            `json:"motor"`   // el modelo que juzgó
	Note    string            `json:"note,omitempty"`
	Pairs   []dedupPairResult `json:"pairs"`
}

func (s *McpServer) toolSharpen(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	// Escribe en el acervo COMPARTIDO (archiva tarjetas): sólo admin, igual que maintain y distill.
	if !principalFrom(ctx).isAdmin() {
		return nil, rpcErrorf(codeUnauthorized, "musubi_sharpen es una operación de mantenimiento del acervo: requiere un principal admin")
	}
	// Opt-in: el veredicto lo da un juez LLM. Sin motor, falla explícito (no degrada en silencio).
	if !cognition.Enabled(s.cognition) {
		return nil, rpcErrorf(codeInvalidParams, "cognición no disponible: musubi_sharpen usa un juez LLM offline y necesita un motor (cognition.provider en .musubi/config.yaml)")
	}
	var args struct {
		Pairs  int     `json:"pairs"`
		Floor  float64 `json:"floor"`
		DryRun bool    `json:"dry_run"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	rep, err := s.runDedupBatch(ctx, args.Floor, args.Pairs, args.DryRun)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	return jsonResult(rep)
}

// runDedupBatch es el NÚCLEO del afilador, compartido por la tool musubi_sharpen y el scheduler
// (sharpenBatchOnce). NO hace control de acceso ni parseo de args —eso es del caller— y asume que el
// motor está disponible. Acota su sección crítica: lee candidatos bajo RLock, JUZGA cada par con el LLM
// AFUERA del candado, y escribe (archivar o marcar not_duplicate) bajo Lock. Sólo devuelve error ante un
// fallo de LECTURA; los fallos por par van en el reporte y no abortan la tanda. dryRun lista sin juzgar.
func (s *McpServer) runDedupBatch(ctx context.Context, floor float64, maxPairs int, dryRun bool) (dedupReport, error) {
	if floor <= 0 {
		floor = dedupDefaultFloor
	}
	if maxPairs <= 0 {
		maxPairs = dedupDefaultPairs
	}
	if maxPairs > dedupMaxPairs {
		maxPairs = dedupMaxPairs
	}
	report := dedupReport{Motor: s.cognition.Name(), Pairs: []dedupPairResult{}}

	// 1) Candidatos por coseno (read, bajo RLock acotado). Model-free salvo el coseno del embedder.
	var cands []memory.SemDupCandidate
	var readErr error
	s.withReadLock(func() {
		cands, readErr = s.engine.SemanticDuplicateCandidates(dedupScope, dedupCardPrefix, floor, maxPairs)
	})
	if readErr != nil {
		return report, fmt.Errorf("no pude listar candidatos a duplicado: %w", readErr)
	}
	report.Scanned = len(cands)
	if len(cands) == 0 {
		report.Note = "no hay pares de tarjetas sobre el piso de coseno sin juzgar: el acervo está sin gemelas detectables."
		return report, nil
	}

	// dry-run: informa qué se juzgaría, sin tocar el motor ni escribir.
	if dryRun {
		for _, c := range cands {
			report.Pairs = append(report.Pairs, dedupPairResult{
				A: c.A, B: c.B, TopicA: c.TopicA, TopicB: c.TopicB, Cosine: c.Cosine, Action: "dry_run",
			})
		}
		report.Note = fmt.Sprintf("dry-run: %d pares candidatos (no se juzgó ni escribió nada).", len(cands))
		return report, nil
	}

	// 2) Por cada par: juzgar con el motor (SIN candado) → aplicar el veredicto (write-lock).
	// gone son las tarjetas ya archivadas por un par ANTERIOR de esta misma tanda: en un cluster denso
	// (A~B~C~D) una tarjeta aparece en varios pares, y una vez fusionada su par deja de existir. Saltear
	// esos pares ANTES de juzgarlos evita gastar una llamada al juez (y cuota de motor) en un no-op y no
	// falsear el reporte; se re-evalúan en la próxima tanda contra el canónico vivo.
	gone := map[string]bool{}
	for _, c := range cands {
		if gone[c.A] || gone[c.B] {
			report.Pairs = append(report.Pairs, dedupPairResult{A: c.A, B: c.B, TopicA: c.TopicA, TopicB: c.TopicB, Cosine: c.Cosine, Action: "skipped: una de las dos ya se fusionó en esta tanda"})
			continue
		}
		if !s.hayPresupuestoDeMotor(ctx) {
			if s.metrics != nil {
				s.metrics.motorDenied.Add(1)
			}
			report.Note = fmt.Sprintf("presupuesto del motor agotado tras juzgar %d pares (máx %d llamadas/hora por principal); corré de nuevo más tarde.", report.Merged+report.Kept, s.motorQuota.max)
			break
		}
		verdict := s.dedupJudge(ctx, c)
		res := dedupPairResult{A: c.A, B: c.B, TopicA: c.TopicA, TopicB: c.TopicB, Cosine: c.Cosine, Verdict: verdict}

		if verdict == dedupMerge {
			var archived bool
			var werr error
			s.withWriteLock(func() {
				archived, werr = s.engine.ArchiveAsDuplicate(dedupScope, c.B, c.A)
			})
			switch {
			case werr != nil:
				res.Action = "error: " + werr.Error()
			case archived:
				report.Merged++
				gone[c.B] = true // la perdedora ya no existe: cualquier otro par que la toque se saltea
				res.Action = "merged"
			default:
				// no-op idempotente: el perdedor ya estaba archivado (defensivo — el guard de gone de
				// arriba ya cubre el caso normal dentro de la tanda).
				res.Action = "skipped: la más débil ya estaba archivada"
			}
			report.Pairs = append(report.Pairs, res)
			continue
		}

		// KEEP: marca not_duplicate para no volver a juzgar el par (ahorra cuota de motor).
		var werr error
		s.withWriteLock(func() {
			_, werr = s.engine.UpsertObsRelation(memory.ObsRelation{
				SourceID: c.A, TargetID: c.B, Relation: dedupKeepMarker,
				Status: memory.RelStatusResolved, ResolvedBy: dedupAuthor,
				Reason: "el juez las consideró facetas distintas, no duplicados", Confidence: 1.0,
			})
		})
		if werr != nil {
			res.Action = "error: " + werr.Error()
		} else {
			report.Kept++
			res.Action = "kept"
		}
		report.Pairs = append(report.Pairs, res)
	}

	if report.Note == "" {
		report.Note = fmt.Sprintf("juzgué %d pares: fusioné %d, conservé %d.", report.Merged+report.Kept, report.Merged, report.Kept)
	}
	return report, nil
}

// dedupJudge pregunta al motor si dos tarjetas gemelas son redundantes (MERGE) o facetas distintas
// (KEEP). FAIL-SAFE: ante un fallo del motor devuelve KEEP —nunca se fusiona por un hipo del endpoint—.
func (s *McpServer) dedupJudge(ctx context.Context, c memory.SemDupCandidate) string {
	user := fmt.Sprintf("TARJETA A (tema %s):\n%s\n\nTARJETA B (tema %s):\n%s", c.TopicA, c.ContentA, c.TopicB, c.ContentB)
	jctx, cancel := context.WithTimeout(ctx, askTimeout)
	defer cancel()
	answer, err := s.cognition.Ask(jctx, sharpenSystemPrompt, user)
	if err != nil {
		return dedupKeep
	}
	return parseDedupVerdict(answer)
}

// parseDedupVerdict extrae MERGE|KEEP de la respuesta del juez. Prefiere el JSON estricto
// {"verdict":"..."}; si no, cae a la última palabra-veredicto del texto. Default KEEP (fail-safe): ante
// cualquier ambigüedad NO se fusiona.
func parseDedupVerdict(answer string) string {
	if i := strings.IndexByte(answer, '{'); i >= 0 {
		if j := strings.LastIndexByte(answer, '}'); j > i {
			var v struct {
				Verdict string `json:"verdict"`
			}
			if err := json.Unmarshal([]byte(answer[i:j+1]), &v); err == nil {
				switch strings.ToUpper(strings.TrimSpace(v.Verdict)) {
				case dedupMerge:
					return dedupMerge
				case dedupKeep:
					return dedupKeep
				}
			}
		}
	}
	up := strings.ToUpper(answer)
	if strings.LastIndex(up, dedupMerge) > strings.LastIndex(up, dedupKeep) {
		return dedupMerge
	}
	return dedupKeep
}

const sharpenSystemPrompt = `Sos el AFILADOR del acervo de diseño de Musubi (pilar 'Musubi Renaissance'). Recibís DOS tarjetas de conocimiento de diseño que un detector marcó como PARECIDAS por su vector. Tu único trabajo: decidir si son LA MISMA lección accionable —tan redundantes que archivar una NO pierde nada— o si cubren facetas DISTINTAS que conviene conservar por separado.

CRITERIO:
- MERGE sólo si una es redundante con la otra: mismo principio, misma acción, sin matiz propio que se perdería al archivar una. Ejemplos claros de MERGE: la misma regla escrita en inglés y en castellano; dos redacciones de "contraste mínimo 4.5:1".
- KEEP si aportan algo distinto: distinto disparador, distinto valor, distinta faceta, un ejemplo o una excepción que la otra no tiene. Ante CUALQUIER duda, KEEP — perder una fusión es reversible, perder conocimiento no.

SALIDA: SÓLO un objeto JSON, sin prosa antes ni después: {"verdict":"MERGE"} o {"verdict":"KEEP"}.`

// sharpenToolEntry registra musubi_sharpen. Va en el central (donde vive el acervo `musubi-design` y hay
// motor); en stdio local sin motor, falla explícito (opt-in). NO es readOnly: archiva tarjetas + escribe
// marcadores. Es lockSelf porque hace I/O externa (juez LLM) por cada par — sostener el candado del
// despacho durante minutos congelaría el servidor (igual que el destilador).
func (s *McpServer) sharpenToolEntry() toolEntry {
	return toolEntry{
		Tool: Tool{
			Name:        "musubi_sharpen",
			Description: "AFILADOR del acervo de diseño (pilar 'Musubi Renaissance'), el gemelo del destilador: pase OFFLINE que junta las tarjetas `design-corpus/*` que dicen la MISMA lección con otras palabras (gemelas por COSENO de embeddings, que el Consolidate por trigramas no ve). Halla pares sobre un piso de coseno y un JUEZ LLM decide, par por par, si son redundantes (MERGE: archiva la más débil, conservando accesos e importancia en la más fuerte — soft-delete REVERSIBLE) o facetas distintas (KEEP: las marca `not_duplicate` para no volver a juzgarlas). CONSERVADOR: ante la duda, conserva. Requiere admin y un motor de cognición (opt-in: sin motor, falla explícito). Procesa de a tandas (default 6 pares, máx 25); corré en bucle. Pasá `dry_run:true` para ver los pares candidatos sin juzgar ni escribir, `pairs` para el tamaño de la tanda, o `floor` para el piso de coseno (default 0.84).",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"pairs":   {Type: "number", Description: "Cuántos pares juzgar en esta tanda (default 6, máximo 25). Cada par es una llamada al juez LLM."},
					"floor":   {Type: "number", Description: "Piso de coseno para proponer un par al juez (default 0.84). Más bajo = más pares candidatos."},
					"dry_run": {Type: "boolean", Description: "Si es true, lista los pares candidatos por coseno SIN llamar al juez ni archivar nada."},
				},
			},
		},
		handler: s.toolSharpen,
		lock:    lockSelf,
	}
}
