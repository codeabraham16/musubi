package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// prime.go implementa el "memory priming" del arranque: un recall SIN query que
// devuelve los gists de las observaciones de mayor SALIENCIA (importancia ×
// frecuencia × recencia) dentro de un presupuesto de tokens. Es lo que permite
// que cada sesión arranque "acordándose" del proyecto. 100% model-free y de solo
// lectura (no toca stats de acceso: el priming es pasivo, no un recall activo).

const defaultPrimeBudget = 300

// PrimeContext devuelve los gists más salientes que entren en budget tokens,
// rankeados por saliencia y a lo sumo uno por topic_key. No recibe query: es
// contexto general del proyecto.
// Es PrimeContextCtx sin alcance: todo el acervo, como fue siempre.
func (e *DbEngine) PrimeContext(budget int) (RecallResult, error) {
	return e.PrimeContextCtx(context.Background(), budget)
}

// PrimeContextCtx es PrimeContext acotado al ProjectScope del ctx (WithProjectScope): con un
// proyecto, sólo lo propio y lo sin atribuir entran al ranking de saliencia, con el MISMO criterio
// que la muralla (scopeClause). Sin scope en el ctx, o federado, es todo el acervo.
//
// Lo usa el hook de arranque en los modos «aparte» y «aislado» (AlcanceDelTurno.ScopeDelPriming).
func (e *DbEngine) PrimeContextCtx(ctx context.Context, budget int) (RecallResult, error) {
	if budget <= 0 {
		budget = defaultPrimeBudget
	}

	clause, args := projectScopeFrom(ctx).scopeClause("o")
	rows, err := e.db.QueryContext(ctx, `
		SELECT o.id, o.topic_key, COALESCE(o.gist,''), o.content, COALESCE(o.content_hash,''), o.tokens,
		       COALESCE(o.created_at,''), COALESCE(o.last_accessed,''), o.access_count, o.importance, COALESCE(o.project_id,''), COALESCE(o.author,''), COALESCE(o.provenance,'human')
		FROM observations o
		WHERE `+visibleObsPredicate+clause+`
	`, args...)
	if err != nil {
		return RecallResult{}, fmt.Errorf("error al listar observaciones para priming: %w", err)
	}
	cands, err := scanCandidates(rows)
	rows.Close()
	if err != nil {
		return RecallResult{}, err
	}

	if len(cands) == 0 {
		return RecallResult{Budget: budget, Items: []RecallItem{}}, nil
	}

	// Rankear por saliencia (determinista, sin LLM) y empaquetar con el núcleo
	// compartido packByBudget (mismo estimador y lógica de presupuesto que el recall).
	now := time.Now().UTC()
	ranked := make([]scoredCandidate, len(cands))
	for i, c := range cands {
		ranked[i] = scoredCandidate{candidate: c, score: candidateSalience(c, now)}
	}
	sort.SliceStable(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	// UNA NOTA POR TEMA: de cada topic_key entra sólo la primera del ranking, que es la más saliente.
	// Medido el 2026-09-28 sobre una copia de la base de davantis-1, con el binario de main: de las 15
	// notas del bloque, 5 eran de project/brain-dashboard-webgl y 3 de ola1/prender-lo-construido, o
	// sea 9 temas en 15 lugares. Las del dashboard son de julio: las sostienen arriba su importancia y
	// el recall, que las sigue trayendo, y no notas nuevas del tema. El presupuesto del arranque se iba
	// en contar el mismo tema cinco veces.
	//
	// Un topic vacío NO se deduplica: no dice de qué habla la nota, así que dos notas sin tema no son
	// el mismo tema y entran las dos. «Vacío» es lo mismo que para el MCP, que rechaza un topic_key
	// en blanco: después de strings.TrimSpace.
	//
	// Va ANTES de packByBudget, y eso decide el borde del presupuesto: si la nota más saliente de un
	// tema no cabe en lo que queda, el tema queda afuera y su lugar lo toma la próxima nota de OTRO
	// tema que quepa (el `continue` de empaquetar). La segunda del mismo tema no entra: sería una
	// versión menos saliente del tema, elegida sólo por ser más corta. Y hacerlo adentro tocaría
	// empaquetar, que es el núcleo que el priming comparte con el recall por turno.
	//
	// Sin vectores ni MMR, a propósito: sobre las 2.412 candidatas de ese arranque, diversificar por
	// similitud pediría los vectores de todas, cada vez.
	visto := make(map[string]bool, len(ranked))
	unaPorTema := ranked[:0]
	for _, c := range ranked {
		tema := strings.TrimSpace(c.topicKey)
		if tema != "" {
			if visto[tema] {
				continue
			}
			visto[tema] = true
		}
		unaPorTema = append(unaPorTema, c)
	}

	return packByBudget(unaPorTema, budget, defaultGistMaxTokens, reparto{}), nil
}

// candidateSalience calcula la saliencia de un candidato usando la edad derivada
// de last_accessed (o created_at). Reusa la función salience del olvido para que
// priming y decay coincidan en el criterio de "qué memoria importa".
func candidateSalience(c candidate, now time.Time) float64 {
	imp := c.importance
	if imp <= 0 {
		imp = 1.0
	}
	ts := effectiveRecency(c)
	// typeWeight neutro (1.0) en el priming: el peso por mem_type modula el OLVIDO (decay),
	// no el ranking de arranque; el pool de candidatos aún no carga mem_type (eso tocaría el
	// hot path de recall, fuera del alcance de B2).
	t, err := time.Parse(sqliteTimeLayout, ts)
	if err != nil {
		// Sin timestamp parseable: edad 0 (recencia máxima), no penalizar.
		return salience(imp, c.accessCount, 0, defaultHalfLifeDays, 1.0, 0)
	}
	ageDays := now.Sub(t).Hours() / 24
	return salience(imp, c.accessCount, ageDays, defaultHalfLifeDays, 1.0, 0)
}
