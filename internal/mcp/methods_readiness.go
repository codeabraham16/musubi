package mcp

// methods_readiness.go expone el ESTADO DE MADUREZ MEDIDO (F3 · doctrina de loop-engineering).
//
// Las evaluaciones de madurez que se usan en la industria son cuestionarios: alguien marca
// casilleros sobre su propio equipo y sale un nivel. Miden la intención, no lo que pasó, y por eso
// el equipo que dice revisar todo y el que efectivamente revisa sacan el mismo puntaje. Musubi
// tiene lo que un cuestionario no tiene —un ledger de invocaciones, una cola de contradicciones, un
// grafo de código y el estado de su memoria— así que puede responder la misma pregunta sin
// preguntarle nada a nadie.
//
// SUS CONSUMIDORES ESTÁN DECLARADOS, que es lo que el repo exige antes de sumar una tool: (1) la
// cabina —el CRM y el tablero del cuerpo— que muestra el estado de cada proyecto del cerebro
// central y necesita un número comparable entre ellos; y (2) el agente que onboardea un proyecto
// nuevo, que hoy no tiene forma de contestar «¿esto ya sirve?» sin revisar cinco tools a mano.
// Es read-only, así que la cabina (write=none) puede llamarla.

import (
	"context"
	"encoding/json"

	"musubi/internal/memory"
)

// readinessReader lo implementa el motor real. Interfaz y no *memory.DbEngine por el mismo motivo
// que el ledger: si el backend de turno no lo sabe calcular, la tool lo dice en vez de romper.
type readinessReader interface {
	Readiness(ctx context.Context, days int) (memory.ReadinessReport, error)
}

// toolReadiness responde qué tan lista está esta instalación, según lo que hizo.
func (s *McpServer) toolReadiness(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Days int `json:"days"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
		}
	}

	lector, ok := s.engine.(readinessReader)
	if !ok {
		return textResult("El motor de memoria de esta instancia no sabe calcular el estado de madurez."), nil
	}
	// scopedCtx acota TODO al proyecto de la credencial (Track 17/18). Sin esto el puntaje de un
	// proyecto se calcularía con el uso, los conflictos y el grafo de los demás — y en un cerebro
	// central eso además filtra el patrón de trabajo ajeno, que es información de negocio.
	rep, err := lector.Readiness(s.scopedCtx(ctx), args.Days)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo calcular el estado de madurez: %v", err)
	}
	return jsonResult(rep)
}
