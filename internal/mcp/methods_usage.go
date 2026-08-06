package mcp

// methods_usage.go expone el LEDGER DE USO (F0 · track «Potencia medida») como tool MCP.
//
// SUS DOS CONSUMIDORES ESTÁN DECLARADOS, que es lo que la regla del track exige antes de escribir
// nada: (1) el agente, que puede preguntarse qué está usando de verdad en vez de suponerlo, y
// (2) el cuerpo, que en F5 va a tener el panel de uso. Una tool sin consumidor no se construye.

import (
	"context"
	"encoding/json"

	"musubi/internal/memory"
)

// usageReader lo implementa el motor real. Interfaz y no *memory.DbEngine porque el backend del
// server es memory.StorageBackend: si el backend de turno no sabe de ledger (los fakes de test),
// la tool responde que no hay datos en vez de romper.
type usageReader interface {
	ToolUsage(ctx context.Context, days int) ([]memory.ToolUsageRow, error)
}

// toolToolUsage responde qué tools se usaron, cuánto tardaron y cómo terminaron.
func (s *McpServer) toolToolUsage(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Days int `json:"days"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
		}
	}
	if args.Days <= 0 {
		args.Days = 30
	}

	lector, ok := s.engine.(usageReader)
	if !ok {
		return textResult("El motor de memoria de esta instancia no expone el ledger de uso."), nil
	}
	// scopedCtx acota la lectura al proyecto de la CREDENCIAL (Track 17). Sin esto, un miembro
	// del cerebro central vería qué herramientas usa otro equipo y con qué frecuencia — patrón
	// de trabajo ajeno, que es información de negocio. El barrido de aislamiento del repo lo
	// cazó antes del merge, con la fuga reproducida en un test.
	filas, err := lector.ToolUsage(s.scopedCtx(ctx), args.Days)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "error al leer el ledger de uso: %v", err)
	}
	if len(filas) == 0 && s.ledger == nil {
		return textResult("El ledger de uso está APAGADO en esta instancia (usage_ledger.enabled: false), " +
			"así que no hay historia que mostrar. Es un estado normal, no un error."), nil
	}
	return textResult(memory.FormatToolUsage(filas, args.Days)), nil
}
