package memory

import (
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// shadow.go es el libro de EVIDENCIA del modo sombra: qué decidió el detector model-free y qué
// habría dicho el motor de cognición sobre el mismo par.
//
// La segunda lectura se descarta SIEMPRE. Este paquete ni siquiera sabe cómo se obtuvo —recibe un
// string ya resuelto— y no expone ninguna función que convierta un veredicto sombra en una
// relación. Esa ignorancia es la garantía: memoria no importa cognition, así que el libro mayor no
// tiene forma de recibir una escritura del LLM por este camino aunque alguien se distraiga.
//
// Lo que sí habilita es la pregunta que estaba trabada: de los pares que el detector auto-resolvió,
// ¿en cuántos el juez discrepa? Eso es lo que decide si 0,70 de léxico es el umbral correcto, y no
// se puede responder con la distribución de similitudes —que ya se midió sobre 77k pares— sino con
// pares etiquetados, que eran 8.

// ShadowVerdict es una fila de evidencia: las dos lecturas del mismo par, lado a lado.
type ShadowVerdict struct {
	RelationID string
	SourceID   string
	TargetID   string
	// HeurRelation y HeurStatus son la decisión que SÍ se aplicó.
	HeurRelation string
	HeurStatus   string
	// Lex y Cosine son las señales tal como estaban en ese momento. Se copian en vez de leerse
	// después desde observation_relations porque un re-juicio posterior las puede cambiar, y la
	// fila tiene que explicar por qué el detector decidió lo que decidió ESE día.
	Lex    *float64
	Cosine *float64
	// JudgeRelation es la lectura del motor, normalizada al mismo vocabulario. JudgeRaw guarda lo
	// que contestó sin tocar, porque el día que la normalización tenga un bug la única forma de
	// notarlo es tener el original.
	JudgeRelation string
	JudgeRaw      string
	JudgeModel    string
}

// SaveShadowVerdict registra una comparación. `agree` se computa acá y no lo pasa el caller: es
// una función de los dos campos que ya viajan, y calcularlo en el borde permitiría que dos
// llamadores lo definieran distinto y que la tabla dejara de ser comparable consigo misma.
func (e *DbEngine) SaveShadowVerdict(v ShadowVerdict) error {
	agree := 0
	if v.HeurRelation == v.JudgeRelation {
		agree = 1
	}
	_, err := e.db.Exec(`
		INSERT INTO shadow_verdicts
			(id, relation_id, source_id, target_id, heur_relation, heur_status,
			 lex_score, cosine_score, judge_relation, judge_raw, judge_model, agree)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), v.RelationID, v.SourceID, v.TargetID, v.HeurRelation, v.HeurStatus,
		floatOrNil(v.Lex), floatOrNil(v.Cosine), v.JudgeRelation, nullable(v.JudgeRaw), v.JudgeModel, agree)
	if err != nil {
		return fmt.Errorf("error al registrar el veredicto sombra de %q: %w", v.RelationID, err)
	}
	return nil
}

// ShadowPairTexts trae el contenido de las dos puntas de un par.
//
// IGNORA LA VISIBILIDAD, por la misma razón que el backfill del desglose: el target de un
// `supersedes` está supersedido por esa misma relación, así que exigir que sea visible dejaría
// afuera justo los pares que más importa medir. El nombre es específico —y no un lector general
// de observaciones que saltea filtros— para que quede claro dónde se puede usar.
func (e *DbEngine) ShadowPairTexts(srcID, tgtID string) (string, string, error) {
	var src, tgt string
	if err := e.db.QueryRow(`SELECT content FROM observations WHERE id = ?`, srcID).Scan(&src); err != nil {
		return "", "", fmt.Errorf("error al leer el contenido de %q: %w", srcID, err)
	}
	if err := e.db.QueryRow(`SELECT content FROM observations WHERE id = ?`, tgtID).Scan(&tgt); err != nil {
		return "", "", fmt.Errorf("error al leer el contenido de %q: %w", tgtID, err)
	}
	return src, tgt, nil
}

// ShadowAgreement es el resumen por tipo de veredicto model-free: cuántas veces el juez coincidió.
type ShadowAgreement struct {
	HeurRelation string  `json:"heur_relation"`
	Total        int     `json:"total"`
	Agreed       int     `json:"agreed"`
	Rate         float64 `json:"rate"`
	// LexMin y LexMax acotan el rango léxico observado para ese tipo. Es lo que convierte la tabla
	// en algo accionable: si los supersedes en los que el juez discrepa se apilan justo arriba del
	// umbral, el umbral está bajo.
	LexMin *float64 `json:"lex_min,omitempty"`
	LexMax *float64 `json:"lex_max,omitempty"`
}

// ShadowAgreementByRelation devuelve el acuerdo agrupado por el veredicto model-free.
func (e *DbEngine) ShadowAgreementByRelation() ([]ShadowAgreement, error) {
	rows, err := e.db.Query(`
		SELECT heur_relation, COUNT(1), SUM(agree), MIN(lex_score), MAX(lex_score)
		FROM shadow_verdicts GROUP BY heur_relation ORDER BY 2 DESC`)
	if err != nil {
		return nil, fmt.Errorf("error al resumir los veredictos sombra: %w", err)
	}
	defer rows.Close()

	out := []ShadowAgreement{}
	for rows.Next() {
		var a ShadowAgreement
		var lexMin, lexMax sql.NullFloat64
		if err := rows.Scan(&a.HeurRelation, &a.Total, &a.Agreed, &lexMin, &lexMax); err != nil {
			return nil, fmt.Errorf("error al escanear el resumen sombra: %w", err)
		}
		if a.Total > 0 {
			a.Rate = float64(a.Agreed) / float64(a.Total)
		}
		if lexMin.Valid {
			a.LexMin = &lexMin.Float64
		}
		if lexMax.Valid {
			a.LexMax = &lexMax.Float64
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
