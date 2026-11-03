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
	// HeurRelation y HeurStatus son la decisión DEL DETECTOR MODEL-FREE, y el nombre es el contrato:
	// toda la tabla existe para contestar «¿el umbral del heurístico está bien puesto?», así que
	// estas dos columnas son de PROCEDENCIA y sólo admiten lo que decidió el heurístico.
	//
	// Decía «la decisión que SÍ se aplicó», que era lo mismo mientras el detector siempre escribiera
	// lo que decidía. Desde que la guarda del upsert conserva los veredictos ajenos, las dos
	// lecturas DIVERGEN: lo que se aplica puede ser de una persona. Quien llena esta fila filtra por
	// procedencia antes de encolar (ver encolarSombra, en internal/mcp/shadow.go); si acá entrara un
	// veredicto humano, el acuerdo medido dejaría de ser heurístico-vs-juez sin que nada lo avise.
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

// ShadowCoverageReport dice SOBRE QUÉ PARTE DEL CORPUS puede hablar la tabla del modo sombra, que
// es justo lo que la tabla sola no puede decir: agrupa por `heur_relation` las filas que EXISTEN,
// así que una tasa de acuerdo del 95 % se lee igual si salió de 20 pares o de 20 sobre 1.100
// candidatos.
//
// Corrido el 2026-11-02 en este repo: de 1.138 relaciones, 901 son re-detectables, 897 de ésas
// tienen firma ajena y sólo 4 son medibles. El recorte no es de configuración, y por eso el reporte
// trae el motivo de cada recorte: las dos únicas filas que el detector firmó solo son `supersedes`
// que YA se aplicaron, así que su target quedó invisible y salieron del conjunto por construcción.
//
// Las cifras de este comentario son una FOTO y envejecen —se movieron el mismo día en que se
// midieron, porque arbitrar un par le cambia la firma—. El número de hoy lo da el comando; lo que
// no envejece es la forma del hallazgo: el recorte es la enorme mayoría y tiene dos motivos
// distintos que se arreglan distinto.
//
// NO SALE DE CONTADORES, Y ÉSA ES LA DECISIÓN. El worker de la sombra ya cuenta `ajenos` y
// `tirados`, pero son campos en memoria que salen únicamente en la línea de log del apagado: cuando
// el proceso muere, el denominador deja de existir. Y el recorte no es una propiedad de una corrida
// del worker: es una propiedad del CORPUS. Derivarlo por SQL lo vuelve durable, siempre al día, sin
// esquema nuevo y sin una sola escritura en el camino de guardado.
type ShadowCoverageReport struct {
	// Relaciones son todas las filas del libro de relaciones, medibles o no.
	Relaciones int `json:"relaciones"`
	// Redetectables son las que la detección puede volver a visitar —las dos puntas visibles—, o
	// sea la cota superior de lo que la sombra podría llegar a medir alguna vez.
	Redetectables int `json:"redetectables"`
	// Medibles son las re-detectables cuyo veredicto es del detector (una propuesta `pending` o su
	// propia firma), que es exactamente lo que encolarSombra no descarta.
	Medibles int `json:"medibles"`
	// FirmaAjena son las re-detectables que la guarda del upsert protegió. Medirlas anotaría el
	// veredicto de una persona en una columna de PROCEDENCIA del heurístico, y el desacuerdo medido
	// dejaría de ser heurístico-vs-juez para ser persona-vs-juez, que es otra pregunta.
	FirmaAjena int `json:"firma_ajena"`
	// PuntaNoVisible son las que la detección no vuelve a visitar nunca, porque una de las dos
	// observaciones está archivada, superseded o en cuarentena.
	//
	// NO ES CORPUS PODRIDO Y CONVIENE NO LEERLO ASÍ: medido el 2026-11-02, de las 237 de este repo
	// hay 197 con una punta supersedida, y de los 66 pares `supersedes` hay 64 cuyo target está
	// supersedido POR SU PROPIO SOURCE —los otros 2 por otra observación, y NINGUNO sin aplicar—.
	// El par ya cumplió su función: es trabajo cerrado, no basura.
	// Es el mismo motivo por el que ShadowPairTexts ignora la visibilidad a propósito.
	PuntaNoVisible int `json:"punta_no_visible"`
	// AutoAplicadas es el SUBCONJUNTO de PuntaNoVisible que explica por qué la vía «firmada por el
	// detector» tiende a cero sola: son relaciones que el detector resolvió solo y que YA surtieron
	// efecto, con el target supersedido por su propio source. Aplicar un `supersedes` deja invisible
	// una de las dos puntas, así que la auto-resolución exitosa se autoexcluye de la re-detección
	// POR CONSTRUCCIÓN, no por un error.
	//
	// NO SUMA en las dos identidades de abajo: es un subconjunto, y está acá porque sin él un «0
	// medibles» manda a buscar el problema donde no está. El repo ya había medido esto por otro
	// lado: relations_backfill.go documenta que exigir visibilidad «excluiría TODA auto-resolución
	// exitosa», y que en el cerebro central eso se comió los 50 `supersedes` de 73 pares de señal.
	// Por eso `conflicts backfill` —que calibra el umbral con la señal léxica— a propósito NO exige
	// visibilidad: la señal de estos pares sigue siendo legible aunque la sombra no los alcance.
	//
	// La huella es el EFECTO (`target.superseded_by = source_id`) y no la etiqueta `supersedes`: una
	// fila re-arbitrada después seguiría teniendo el supersedido aplicado, y lo que saca al par del
	// conjunto es el efecto.
	AutoAplicadas int `json:"auto_aplicadas"`
	// Medidas son las filas que la sombra efectivamente escribió. Con Medibles al lado, un cero deja
	// de ser ambiguo: dice si falta encender algo o si no había nada que medir.
	Medidas int `json:"medidas"`
}

// ShadowCoverage deriva la cobertura del estado ACTUAL del corpus.
//
// El criterio de «re-detectable» es visibleObsPredicateDe y NO una copia escrita a mano, por el
// motivo que documenta scope.go: el único sitio del repo que lo escribió a mano salió mal —le
// faltaba `quarantined = 0`—, y la centralidad que entra al recall se calculaba sobre un grafo que
// incluía observaciones en cuarentena. obsrank.go ya resuelve este mismo caso, las dos puntas de
// una arista, con este helper.
//
// Y el criterio de «medible» se arma desde las constantes del paquete en vez de literales en el
// SQL, que es el patrón que setConservandoVeredicto estableció en este mismo expediente y por la
// misma razón: si la firma del detector cambia, la cobertura cambia con ella en vez de declarar una
// cobertura que no es la que encolarSombra aplica. Son texto del propio código, nunca entrada de
// nadie.
//
// LA ARITMÉTICA CIERRA POR CONSTRUCCIÓN, y por eso PuntaNoVisible sale de una RESTA y no de una
// segunda consulta con NOT EXISTS: una consulta aparte podría devolver un número que no cuadra con
// los otros tres y nadie se enteraría. Medibles+FirmaAjena == Redetectables, y
// Redetectables+PuntaNoVisible == Relaciones. El JOIN es por clave primaria, así que lo único que
// la resta absorbe además de la invisibilidad es una relación cuya punta ya no exista —medido hoy:
// cero, pero el borrado en duro de observaciones es una receta documentada de este proyecto—, y una
// punta que no existe tampoco es re-detectable: el balde sigue siendo el correcto.
func (e *DbEngine) ShadowCoverage() (ShadowCoverageReport, error) {
	var r ShadowCoverageReport
	medible := `(r.status = '` + RelStatusPending + `' OR COALESCE(r.resolved_by,'') = '` + RelResolvedByHeuristic + `')`
	// COALESCE sobre los dos SUM porque con cero filas re-detectables devuelven NULL, no 0, y el
	// Scan a un int fallaría con un corpus chico o recién nacido en vez de informar la verdad: que
	// no hay nada medible.
	err := e.db.QueryRow(`
		SELECT
			(SELECT COUNT(1) FROM observation_relations),
			COUNT(1),
			COALESCE(SUM(CASE WHEN `+medible+` THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN `+medible+` THEN 0 ELSE 1 END), 0),
			(SELECT COUNT(1) FROM shadow_verdicts),
			(SELECT COUNT(1) FROM observation_relations r2
			   JOIN observations b2 ON b2.id = r2.target_id
			  WHERE COALESCE(r2.resolved_by,'') = '`+RelResolvedByHeuristic+`'
			    AND b2.superseded_by = r2.source_id)
		FROM observation_relations r
		JOIN observations a ON a.id = r.source_id
		JOIN observations b ON b.id = r.target_id
		WHERE `+visibleObsPredicateDe("a")+` AND `+visibleObsPredicateDe("b")).
		Scan(&r.Relaciones, &r.Redetectables, &r.Medibles, &r.FirmaAjena, &r.Medidas, &r.AutoAplicadas)
	if err != nil {
		return r, fmt.Errorf("error al derivar la cobertura del modo sombra: %w", err)
	}
	r.PuntaNoVisible = r.Relaciones - r.Redetectables
	return r, nil
}
