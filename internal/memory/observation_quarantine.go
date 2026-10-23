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
//
// EL PREFIJO APARECE EXACTAMENTE UNA VEZ. `llm:llm:claude-opus-5` se lee razonable y no lo es:
// nombra como MODELO a algo que ya es una procedencia, así que el sello deja de decir qué modelo
// escribió — que es lo único que el sello existe para decir. Antes pasaba porque acá sólo se
// exigía que después de `llm:` quedara algo no vacío, y con esa regla el valor era válido por
// construcción. Hay una fila así en el libro mayor (topic `cuerpo/11-banco-hermetica`,
// 2026-09-10 12:11:23); así se descubrió.
//
// Los dos puntos SÍ son legítimos dentro del modelo (`llm:ollama:qwen`, `llm:groq/llama-3.3`): lo
// que se rechaza es repetir el prefijo, no el separador.
func validProvenance(p string) bool {
	switch {
	case p == provenanceHuman, p == provenanceDeterministic:
		return true
	case strings.HasPrefix(p, provenanceLLMPrefix):
		modelo := strings.TrimSpace(strings.TrimPrefix(p, provenanceLLMPrefix))
		// 'llm:' pelado no alcanza: sin el modelo el sello no es auditable. Y un modelo que
		// vuelve a empezar con 'llm:' es el prefijo duplicado, no un modelo.
		return modelo != "" && !strings.HasPrefix(modelo, provenanceLLMPrefix)
	default:
		return false
	}
}

// HonrarSello decide si el receptor de un push aplica el sello del payload como stamp. Devuelve
// true SÓLO si la procedencia es válida Y NO es 'human'. Es la regla no-human del borde de sync:
//
//   - un 'llm:*' o 'deterministic' entrante se HONRA (se escribe tal cual): el nodo origen ya lo
//     selló en su columna por la puerta Q2, y perderlo acá sería el lavado que todo esto evita;
//   - un 'human' entrante se IGNORA y cae al default human/1.0: 'human' es siempre por-default,
//     nunca por-claim, así un payload no puede afirmarse 'human' con una confidence a medida.
//
// El sello sólo puede BAJAR o MANTENER la autoridad del contenido, nunca SUBIRLA: un 'human'
// forjado cae al default y no gana nada, así que honrar el sello entrante no abre un agujero
// nuevo. La procedencia es metadato de CONTENIDO (qué clase de proceso lo generó), no un eje de
// atribución —ese lo gobierna writeOriginFor para project_id, independiente—, así que honrarla
// tampoco toca el aislamiento de tenant en la escritura.
func HonrarSello(provenance string) bool {
	return validProvenance(provenance) && provenance != provenanceHuman
}

// SelloFueraDeTaxonomia es la contracara de HonrarSello para el TERCER caso del receptor: un sello
// PRESENTE que no es ni 'human' ni una procedencia válida. El receptor lo RECHAZA en vez de lavarlo
// a 'human': un 'llm:groq/llama-3.3' truncado a 'll' que cayera al default humano quedaría estampado
// 'human', es decir contenido de un modelo lavado como decisión de una persona — justo el lavado que
// este borde existe para impedir. Un sello vacío ('') NO cuenta acá: es la AUSENCIA de sello, que
// cae al default human/1.0 sin ser un error (un emisor viejo que no manda el campo, R9).
func SelloFueraDeTaxonomia(provenance string) bool {
	return provenance != "" && !validProvenance(provenance)
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
	// El caller manda el MODELO, no la procedencia armada. Un 'llm:claude-opus-5' acá produciría
	// 'llm:llm:claude-opus-5', y se RECHAZA en vez de recortarse por la misma razón que la
	// confianza fuera de rango: recortar en silencio convierte el error de quien llama en un dato
	// plausible y equivocado guardado para siempre. El error nombra la equivocación del caller —
	// "ya trae el prefijo" — en vez de un 'procedencia inválida' que lo dejaría adivinando.
	if strings.HasPrefix(model, provenanceLLMPrefix) {
		return "", fmt.Errorf("%w: el modelo ya trae el prefijo %q (%q). Mandá sólo el modelo", ErrInvalidProvenance, provenanceLLMPrefix, model)
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
	var existe int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observations WHERE id = ?`, id).Scan(&existe); err != nil {
		return fmt.Errorf("error al leer el estado de %s: %w", id, err)
	}
	if existe == 0 {
		return fmt.Errorf("%w: %s", ErrObservationNotFound, id)
	}
	// LA GUARDA ES EL WHERE, igual que en DescartarPropuesta: sólo una propuesta VIVA en cuarentena
	// sale a la luz. Una ya DESCARTADA (archived=1) o REEMPLAZADA (superseded_by) no se corrobora.
	//
	// EL archived=0 IMPORTA MÁS DESDE QUE CORROBORAR PROMUEVE (team mode, toolCorroborate). Antes,
	// corroborar una propuesta descartada sólo la volvía quarantined=0 y el daño quedaba LOCAL e
	// invisible (archived=1 la esconde del recall). Ahora la promoción la marcaría 'shared' y la
	// encolaría: una nota que alguien DESCARTÓ viajaría al cerebro central y a las otras máquinas
	// como nota viva (archived no viaja en el outbox). La carrera corroborar/descartar sobre el
	// tablero musubi/cuarentena es el escenario real. El bump de sync_seq se conserva: la fila pasa
	// a ser visible y, si además fuera shared, debe re-entregarse al pull entrante.
	res, err := e.db.Exec(
		`UPDATE observations
		 SET quarantined = 0,
		     sync_seq = (SELECT IFNULL(MAX(sync_seq),0) FROM observations) + 1
		 WHERE id = ? AND quarantined = 1 AND archived = 0 AND superseded_by IS NULL`, id)
	if err != nil {
		return fmt.Errorf("error al corroborar la observación %s: %w", id, err)
	}
	if n, err := res.RowsAffected(); err == nil && n == 0 {
		return fmt.Errorf("%w: %s", ErrNotQuarantined, id)
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
