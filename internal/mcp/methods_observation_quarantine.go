package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"musubi/internal/memory"
)

// Murallas 2 y 3 del blindaje del cerebro (F4): las dos puertas de la cuarentena de escritura.
//
// Modeladas sobre toolProposeFacts, que ya resuelve exactamente este problema del lado del grafo
// de hechos. Mismo patrón de `model` → 'caller' por default, misma atribución por credencial vía
// writeOriginFor, misma redacción forzada al borde compartido.

// toolProposeObservation escribe una observación EN CUARENTENA con procedencia de modelo.
//
// Q2 ES ESTRUCTURAL: el schema NO expone `provenance` ni `quarantined`. No es que se ignoren si
// los mandan — no existen. Un modelo no puede declararse 'human' porque la puerta por la que
// escribe no tiene esa perilla. Misma decisión que en F1, donde el portero de privacidad se cableó
// dentro del único constructor para que no hubiera forma de construir un motor sin él.
func (s *McpServer) toolProposeObservation(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		TopicKey   string   `json:"topic_key"`
		Content    string   `json:"content"`
		Model      string   `json:"model"`
		Confidence *float64 `json:"confidence"`
		MemType    string   `json:"mem_type"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.TopicKey) == "" {
		return nil, rpcErrorf(codeInvalidParams, "topic_key es obligatorio")
	}
	if strings.TrimSpace(args.Content) == "" {
		return nil, rpcErrorf(codeInvalidParams, "content es obligatorio")
	}

	// Confianza: puntero para distinguir "no la mandaron" de "mandaron 0". Sin ese distingo, un
	// caller que afirma confianza CERO (no le creo nada a esto) quedaría indistinguible de uno que
	// se olvidó del campo, y el default lo subiría a 0.5 en silencio.
	confidence := 0.5
	if args.Confidence != nil {
		confidence = *args.Confidence
	}

	// Atribución por credencial (Track 17), igual que save_observation y propose_facts.
	origin, okOrigin := writeOriginFor(principalFrom(ctx), "")
	if !okOrigin {
		return nil, rpcErrorf(codeUnauthorized, "escritura sin proyecto: esta credencial no tiene project_id propio y no se declaró ninguno; una fila sin atribuir la ven TODOS los tenants")
	}

	id, err := s.engine.ProposeObservation(origin, authorFrom(principalFrom(ctx)), args.TopicKey, args.Content, args.Model, confidence, args.MemType, nil)
	if err != nil {
		// La confianza fuera de rango es error DEL CALLER, no del servidor: se rechaza en la
		// frontera en vez de recortarse, así que corresponde codeInvalidParams.
		if errors.Is(err, memory.ErrInvalidConfidence) {
			return nil, rpcErrorf(codeInvalidParams, "%v", err)
		}
		return nil, rpcErrorf(codeInternalError, "error al proponer observación: %v", err)
	}

	model := strings.TrimSpace(args.Model)
	if model == "" {
		model = "caller"
	}
	return textResult(fmt.Sprintf(
		"Observación PROPUESTA en cuarentena (id: %s, procedencia: llm:%s, confianza: %.2f).\n"+
			"NO aparece en recall, NO se promueve a 'shared' y NO viaja al cerebro central. "+
			"Para volverla visible, corroborála con musubi_corroborate — el sello de procedencia se conserva.",
		id, model, confidence)), nil
}

// toolCorroborate saca una observación de cuarentena. Es la ÚNICA puerta de salida (Q4).
//
// CONSERVA la procedencia a propósito: corroborar no convierte una inferencia en una nota humana,
// sólo la hace visible. Si esto pisara el sello, la Muralla 3 duraría hasta la primera
// corroboración.
func (s *McpServer) toolCorroborate(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.ID) == "" {
		return nil, rpcErrorf(codeInvalidParams, "id es obligatorio")
	}

	// Variante Ctx: lleva adentro la guarda multi-tenant, la misma que promote. Sin ella,
	// conocer un id ajeno alcanzaría para volver visible memoria de otro proyecto.
	if err := s.engine.CorroborateObservationCtx(s.scopedCtx(ctx), args.ID); err != nil {
		if errors.Is(err, memory.ErrObservationNotFound) {
			return nil, rpcErrorf(codeInvalidParams, "no existe una observación con id %q", args.ID)
		}
		if errors.Is(err, memory.ErrCrossTenant) {
			return nil, rpcErrorf(codeUnauthorized, "%v", err)
		}
		// No estar en cuarentena es un error y NO un no-op: un corroborate por el id equivocado
		// tiene que notarse, en vez de reportar éxito y dejar la observación real todavía invisible.
		if errors.Is(err, memory.ErrNotQuarantined) {
			return nil, rpcErrorf(codeInvalidParams, "la observación %q no está en cuarentena (no hay nada que corroborar)", args.ID)
		}
		return nil, rpcErrorf(codeInternalError, "error al corroborar la observación: %v", err)
	}

	prov, conf, _, err := s.engine.ObservationStamp(args.ID)
	if err != nil {
		return textResult("Observación corroborada (id: " + args.ID + "); ya aparece en el recall."), nil
	}
	return textResult(fmt.Sprintf(
		"Observación corroborada (id: %s); ya aparece en el recall.\n"+
			"Conserva su sello: procedencia %s, confianza %.2f. Corroborar la hace visible, no la vuelve un hecho humano.",
		args.ID, prov, conf)), nil
}
