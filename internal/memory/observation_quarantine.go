package memory

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Murallas 2 y 3 del blindaje del cerebro (F4): cuarentena de escritura y procedencia.
//
// EL PROBLEMA QUE CIERRA. `musubi_ask` sintetiza texto con un LLM. Nada impedía tomar esa
// respuesta y guardarla con `musubi_save_observation`: quedaba en el libro mayor
// indistinguible de una nota verificada a mano. El grafo de hechos ya tenía esta guarda
// (`relations.source = 'llm-extract:<model>'`, migración v20); el libro mayor no.
//
// OJO — `author` NO sirve para esto. Es la atribución por credencial del Track C5: dice QUÉ
// persona o máquina escribió, y un agente-LLM y una persona escriben con la misma credencial.
// Acá se responde otra pregunta: QUÉ CLASE DE PROCESO generó el contenido.

// Procedencia: taxonomía CERRADA. Un valor fuera del conjunto es un error, no un default
// silencioso — misma regla que el `mode` desconocido del gateway de F1, que apaga el pilar
// entero en vez de adivinar.
const (
	// provenanceHuman: lo decidió una persona (posiblemente tipeado por un agente en su
	// nombre). Es el default del esquema y el sello de las filas anteriores a F4.
	provenanceHuman = "human"
	// provenanceDeterministic: lo derivó código sin modelo (hooks de captura, detect_changes,
	// indexadores). Verificable re-corriendo el mismo código.
	provenanceDeterministic = "deterministic"
	// provenanceLLMPrefix: lo generó un modelo. Siempre lleva el modelo pegado
	// ('llm:groq/llama-3.3'), porque "lo dijo un LLM" sin decir cuál no es auditable.
	provenanceLLMPrefix = "llm:"
)

var (
	// ErrInvalidConfidence — la confianza cayó fuera de [0,1]. Se RECHAZA en vez de recortar:
	// recortar en silencio convierte el error de quien llama en un dato plausible y equivocado
	// guardado para siempre.
	ErrInvalidConfidence = errors.New("confidence fuera de rango: debe estar en [0,1]")
	// ErrInvalidProvenance — sello fuera de la taxonomía.
	ErrInvalidProvenance = errors.New("procedencia inválida")
	// ErrNotQuarantined — se intentó corroborar algo que no está en cuarentena. Es un error y
	// no un no-op para que un corroborate por el id equivocado se note en vez de pasar como
	// éxito y dejar la observación real todavía invisible.
	ErrNotQuarantined = errors.New("la observación no está en cuarentena")
	// ErrQuarantined — se intentó promover a 'shared' algo en cuarentena (Q6).
	ErrQuarantined = errors.New("la observación está en cuarentena")
)

// obsStamp es el sello que saveObservation escribe en el INSERT. nil ⇒ defaults del esquema.
type obsStamp struct {
	provenance  string
	confidence  float64
	quarantined bool
}

// validProvenance valida contra la taxonomía cerrada.
func validProvenance(p string) bool {
	switch {
	case p == provenanceHuman, p == provenanceDeterministic:
		return true
	case strings.HasPrefix(p, provenanceLLMPrefix):
		// 'llm:' pelado no alcanza: sin el modelo el sello no es auditable.
		return strings.TrimSpace(strings.TrimPrefix(p, provenanceLLMPrefix)) != ""
	default:
		return false
	}
}

// ProposeObservation escribe una observación EN CUARENTENA con procedencia de modelo. Es la
// puerta de escritura para todo lo que produjo un LLM.
//
// Q2 ES ESTRUCTURAL, NO DECLARATIVO: esta función no recibe ni procedencia ni bandera de
// cuarentena — las escribe ella. Un modelo no puede declararse 'human' porque la puerta por la
// que escribe no tiene esa perilla. Es la misma decisión que en F1 hizo imposible construir un
// motor de cognición sin portero: el sello es POR DÓNDE ENTRASTE, no qué dijiste que sos.
//
// SIN DEDUP, a propósito. El resto de los caminos de guardado deduplican por content_hash, pero
// acá eso sería un agujero: si el texto propuesto coincide con una observación autoritativa
// existente, el dedup devolvería ese id y la propuesta quedaría "confirmada" sin que nadie la
// haya corroborado. Una propuesta es un artefacto distinto de una nota corroborada, aunque digan
// lo mismo.
//
// Scope siempre 'local': una fila en cuarentena no puede ser candidata al central (Q6).
func (e *DbEngine) ProposeObservation(originProjectID, author, topicKey, content, model string, confidence float64, memType string, embedding []float32) (string, error) {
	if confidence < 0 || confidence > 1 {
		return "", fmt.Errorf("%w: %v", ErrInvalidConfidence, confidence)
	}
	model = strings.TrimSpace(model)
	if model == "" {
		// Mismo default que toolProposeFacts: el caller es el que aportó el texto.
		model = "caller"
	}
	stamp := &obsStamp{
		provenance:  provenanceLLMPrefix + model,
		confidence:  confidence,
		quarantined: true,
	}
	id := uuid.NewString()
	if err := e.saveObservation(id, topicKey, content, 1.0, true, memType, ScopeLocal, originProjectID, author, nil, embedding, stamp); err != nil {
		return "", err
	}
	return id, nil
}

// CorroborateObservation saca una observación de cuarentena. Es la ÚNICA puerta de salida (Q4):
// nada sale solo por antigüedad, por accesos, por decaimiento ni porque el recall la haya rozado.
//
// CONSERVA EL SELLO DE PROCEDENCIA a propósito. Corroborar no convierte una inferencia en una
// nota humana; sólo la hace visible. Un 'llm:groq/llama-3.3' corroborado sigue diciendo de dónde
// salió, y el recall lo sigue marcando (Q3). Si esto pisara la procedencia, la muralla 3 duraría
// exactamente hasta la primera corroboración.
func (e *DbEngine) CorroborateObservation(id string) error {
	var quarantined int
	err := e.db.QueryRow(`SELECT quarantined FROM observations WHERE id = ?`, id).Scan(&quarantined)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("%w: %s", ErrObservationNotFound, id)
	}
	if err != nil {
		return fmt.Errorf("error al leer el estado de cuarentena de %s: %w", id, err)
	}
	if quarantined == 0 {
		return fmt.Errorf("%w: %s", ErrNotQuarantined, id)
	}
	// Se bumpea sync_seq porque la fila pasa a ser visible: sin eso, una observación corroborada
	// que además sea shared no se re-entregaría al pull entrante.
	if _, err := e.db.Exec(
		`UPDATE observations
		 SET quarantined = 0,
		     sync_seq = (SELECT IFNULL(MAX(sync_seq),0) FROM observations) + 1
		 WHERE id = ?`, id); err != nil {
		return fmt.Errorf("error al corroborar la observación %s: %w", id, err)
	}
	return nil
}

// CorroborateObservationCtx es CorroborateObservation acotada al proyecto de la credencial, con
// la MISMA guarda que PromoteObservationCtx: sin ella, conocer un id ajeno alcanzaría para volver
// visible memoria de OTRO tenant. Es el gemelo exacto del hallazgo #11 de la auditoría 2026-07-26,
// que es el que obligó a acotar promote.
func (e *DbEngine) CorroborateObservationCtx(ctx context.Context, id string) error {
	ok, err := e.obsInScope(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: no se puede corroborar %s desde otro proyecto", ErrCrossTenant, id)
	}
	return e.CorroborateObservation(id)
}

// IsQuarantined indica si una observación está en cuarentena. Lo usan las guardas de promoción
// y los tests de los invariantes.
func (e *DbEngine) IsQuarantined(id string) (bool, error) {
	var q int
	err := e.db.QueryRow(`SELECT quarantined FROM observations WHERE id = ?`, id).Scan(&q)
	if errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("%w: %s", ErrObservationNotFound, id)
	}
	if err != nil {
		return false, fmt.Errorf("error al leer el estado de cuarentena de %s: %w", id, err)
	}
	return q == 1, nil
}

// ObservationStamp devuelve el sello de una observación (procedencia y confianza).
func (e *DbEngine) ObservationStamp(id string) (provenance string, confidence float64, quarantined bool, err error) {
	var q int
	row := e.db.QueryRow(
		`SELECT COALESCE(provenance,''), COALESCE(confidence,1.0), quarantined FROM observations WHERE id = ?`, id)
	if err := row.Scan(&provenance, &confidence, &q); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", 0, false, fmt.Errorf("%w: %s", ErrObservationNotFound, id)
		}
		return "", 0, false, fmt.Errorf("error al leer el sello de %s: %w", id, err)
	}
	return provenance, confidence, q == 1, nil
}
