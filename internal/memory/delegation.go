package memory

import "fmt"

// delegation.go estima el AHORRO DE TOKENS por delegar en la pizarra (O2), fusionando
// el gobernador de tokens con la orquestación multi-agente. Es MODEL-FREE y explícito:
// Musubi no puede medir cuántos tokens consumió realmente un sub-agente, así que el
// ahorro es una ESTIMACIÓN con parámetros configurables, no una medición.
//
// La economía (alineada con el análisis de agent-teams-lite): el gran ahorro de
// delegar viene del AISLAMIENTO DE CONTEXTO — las lecturas y el razonamiento de cada
// unidad viven en el sub-agente y nunca entran al contexto del orquestador; solo
// vuelve el resultado compacto. Como ese resultado se paga igual inline o delegado,
// se cancela: el ahorro NETO es el contexto intermedio evitado menos el overhead fijo
// de lanzar el sub-agente, por unidad completada. De ahí que delegar rinda con MÁS
// unidades y sea contraproducente para tareas triviales.

// DelegationSavings es la estimación de ahorro de un batch delegado.
type DelegationSavings struct {
	UnitsDone            int    `json:"units_done"`
	OrchestratorTokens   int    `json:"orchestrator_tokens"`    // medido: tokens de los resultados que SÍ entraron al orquestador
	AvoidedContextTokens int    `json:"avoided_context_tokens"` // estimado: contexto intermedio que quedó en los sub-agentes
	DelegationOverhead   int    `json:"delegation_overhead"`    // estimado: costo fijo de lanzar los sub-agentes
	EstimatedSavings     int    `json:"estimated_savings"`      // avoided - overhead (puede ser negativo)
	PaidOff              bool   `json:"paid_off"`
	Note                 string `json:"note"`
}

// EstimateDelegationSavings calcula el ahorro estimado de un batch. avoidedPerUnit es
// el contexto intermedio evitado por unidad; overheadPerUnit el costo fijo de delegar
// una unidad. Solo cuentan las unidades DONE (una failed no aisló trabajo útil).
func EstimateDelegationSavings(b WorkBatch, avoidedPerUnit, overheadPerUnit int) DelegationSavings {
	if avoidedPerUnit < 0 {
		avoidedPerUnit = 0
	}
	if overheadPerUnit < 0 {
		overheadPerUnit = 0
	}
	done, orch := 0, 0
	for _, u := range b.Units {
		if u.Status == WorkDone {
			done++
			orch += EstimateTokens(u.Result)
		}
	}
	avoided := done * avoidedPerUnit
	overhead := done * overheadPerUnit
	savings := avoided - overhead

	ds := DelegationSavings{
		UnitsDone:            done,
		OrchestratorTokens:   orch,
		AvoidedContextTokens: avoided,
		DelegationOverhead:   overhead,
		EstimatedSavings:     savings,
		PaidOff:              savings > 0,
	}
	switch {
	case done == 0:
		ds.Note = "Ningún resultado completado todavía: sin ahorro que estimar."
	case savings > 0:
		ds.Note = fmt.Sprintf("Estimación model-free: delegar %d unidad(es) ahorró ~%d tokens de contexto "+
			"vs. hacerlo inline (contexto evitado %d − overhead %d). Solo el resultado (%d tokens) entró al orquestador.",
			done, savings, avoided, overhead, orch)
	default:
		ds.Note = fmt.Sprintf("Estimación model-free: con %d unidad(es) delegar NO rindió (~%d tokens): el overhead "+
			"de sub-agentes (%d) superó al contexto evitado (%d). Para tareas chicas conviene hacerlo inline.",
			done, savings, overhead, avoided)
	}
	return ds
}
