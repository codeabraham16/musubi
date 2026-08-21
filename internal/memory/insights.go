package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// insights.go implementa el resumen de "observabilidad activa" (Track 6 / T6.4): un único
// reporte model-free de lo que Musubi sabe de sí mismo — tamaño de la memoria, hotspots de
// errores no resueltos, decisiones de skills y salud del ciclo de mantenimiento. Todo es
// agregación SQL/aritmética determinista; sin LLM.

// insightsHotspotLimit acota cuántos archivos con más errores no resueltos se listan.
const insightsHotspotLimit = 10

// InsightsReport es el resumen agregado del estado de la memoria.
type InsightsReport struct {
	Observations     ObsStats         `json:"observations"`
	Utilization      UtilizationStats `json:"utilization"`
	UnresolvedErrors int              `json:"unresolved_errors"`
	ErrorHotspots    []ErrorHotspot   `json:"error_hotspots"`
	SkillDecisions   DecisionStats    `json:"skill_decisions"`
	LastMaintenance  string           `json:"last_maintenance,omitempty"`
	Health           string           `json:"health,omitempty"`
}

// UtilizationStats mide cuánta de la memoria RECUPERABLE se usó de verdad (Musubi Renaissance · F4 —
// "que todo lo ingerido se use, sin cabos sueltos"). recall_rate = recalled / active. never_recalled es
// la señal cruda de cabos sueltos; NO implica huérfano por sí sola (una obs recién guardada tiene 0
// accesos legítimamente — se interpreta junto con la edad). Model-free: agregación SQL determinista.
type UtilizationStats struct {
	Active        int     `json:"active"`         // observaciones recuperables (visibles) del scope
	Recalled      int     `json:"recalled"`       // de esas, cuántas se recuperaron alguna vez (access_count>0)
	NeverRecalled int     `json:"never_recalled"` // nunca recuperadas (candidatas a cabo suelto)
	RecallRate    float64 `json:"recall_rate"`    // recalled / active (0 si no hay activas)
}

// ObsStats resume el tamaño de la memoria de observaciones.
//
// Active y Visible NO son lo mismo, y la diferencia es la que hacía que el dashboard
// afirmara dos poblaciones distintas en la misma pantalla:
//
//	Active  = Total - Archived          -> mide "no archivada", un solo eje
//	Visible = visibleObsPredicate       -> archived=0 AND superseded_by IS NULL AND quarantined=0
//
// Visible es la cantidad RECUPERABLE: lo que el recall devuelve y lo que el grafo dibuja.
// En el cerebro central la brecha era de 64 notas superadas o en cuarentena que Active
// contaba como memoria viva y ningún camino de lectura devuelve nunca.
//
// Active se deja como está a propósito, con su fórmula vieja: alimenta readiness y
// consumidores externos, y cambiarle el VALOR a un campo publicado es un break silencioso.
// Se agrega Visible al lado y se hace que la UI lea el bueno.
type ObsStats struct {
	Total    int `json:"total"`
	Active   int `json:"active"`
	Archived int `json:"archived"`
	Visible  int `json:"visible"`
}

// ErrorHotspot es un archivo con errores no resueltos y su cantidad.
type ErrorHotspot struct {
	FilePath string `json:"file_path"`
	Count    int    `json:"count"`
}

// DecisionStats cuenta las skills por su decisión MÁS RECIENTE (last-write-wins).
type DecisionStats struct {
	Accepted int `json:"accepted"`
	Rejected int `json:"rejected"`
}

// Insights agrega el estado de la memoria en el espacio FEDERADO (histórico). Fino wrapper
// sobre InsightsCtx con contexto vacío. Read-only.
func (e *DbEngine) Insights() (InsightsReport, error) {
	return e.InsightsCtx(context.Background())
}

// InsightsCtx agrega el estado de la memoria acotando TODO al proyecto del contexto (Track 17 +
// Track 18): sin esto un principal veía el tamaño de la memoria, los hotspots de errores y las
// decisiones de skills de TODOS los proyectos (fuga de volumen/metadata ajena). Desde Track 18
// telemetry_logs y skill_decisions tienen project_id (migración v15), así que los tres se acotan
// con el mismo scopeClause; sólo lo sin atribuir (project_id vacío) queda visible para todos. Ctx federado
// (stdio/admin) ⇒ counts globales (histórico bit-a-bit). Read-only.
func (e *DbEngine) InsightsCtx(ctx context.Context) (InsightsReport, error) {
	var rep InsightsReport

	// Scope de proyecto (Track 17/18): se aplica a observations, telemetry_logs y skill_decisions.
	scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("")
	where := ""
	if scopeSQL != "" {
		where = " WHERE" + strings.TrimPrefix(scopeSQL, " AND")
	}
	if err := e.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN archived=1 THEN 1 ELSE 0 END),0) FROM observations`+where,
		scopeArgs...,
	).Scan(&rep.Observations.Total, &rep.Observations.Archived); err != nil {
		return rep, fmt.Errorf("insights: contar observaciones: %w", err)
	}
	rep.Observations.Active = rep.Observations.Total - rep.Observations.Archived

	// Utilización del acervo (F4): de las observaciones RECUPERABLES (visibles), cuántas se recuperaron
	// alguna vez. never_recalled es la señal de cabos sueltos. Mismo scope de proyecto que el resto.
	var util UtilizationStats
	if err := e.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN access_count>0 THEN 1 ELSE 0 END),0)
		 FROM observations WHERE `+visibleObsPredicate+scopeSQL,
		scopeArgs...,
	).Scan(&util.Active, &util.Recalled); err != nil {
		return rep, fmt.Errorf("insights: utilización: %w", err)
	}
	util.NeverRecalled = util.Active - util.Recalled
	if util.Active > 0 {
		util.RecallRate = float64(util.Recalled) / float64(util.Active)
	}
	rep.Utilization = util
	// El conteo VISIBLE ya está calculado acá arriba con el predicado canónico: se copia a
	// ObsStats en vez de correr un segundo COUNT idéntico. Que salgan de la misma query es
	// además lo que garantiza que los dos campos no puedan divergir.
	rep.Observations.Visible = util.Active

	if err := e.db.QueryRow(
		`SELECT COUNT(*) FROM telemetry_logs WHERE resolved=0`+scopeSQL,
		scopeArgs...,
	).Scan(&rep.UnresolvedErrors); err != nil {
		return rep, fmt.Errorf("insights: contar errores: %w", err)
	}

	hotspotArgs := append(append([]interface{}{}, scopeArgs...), insightsHotspotLimit)
	rows, err := e.db.Query(`
		SELECT file_path, COUNT(*) AS c FROM telemetry_logs WHERE resolved=0`+scopeSQL+`
		GROUP BY file_path ORDER BY c DESC, file_path ASC LIMIT ?`, hotspotArgs...)
	if err != nil {
		return rep, fmt.Errorf("insights: hotspots: %w", err)
	}
	for rows.Next() {
		var h ErrorHotspot
		if err := rows.Scan(&h.FilePath, &h.Count); err != nil {
			rows.Close()
			return rep, fmt.Errorf("insights: escanear hotspot: %w", err)
		}
		rep.ErrorHotspots = append(rep.ErrorHotspots, h)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return rep, fmt.Errorf("insights: iterar hotspots: %w", err)
	}
	rows.Close()

	// Decisiones de skills por su decisión más reciente (last-write-wins, coherente con T6.1),
	// acotadas al proyecto del contexto (Track 18).
	decisions, err := e.GetSkillDecisionsCtx(ctx)
	if err != nil {
		return rep, fmt.Errorf("insights: decisiones: %w", err)
	}
	latest := make(map[string]string, len(decisions))
	for _, d := range decisions {
		latest[d.SkillID] = d.Decision
	}
	for _, dec := range latest {
		switch dec {
		case "accepted":
			rep.SkillDecisions.Accepted++
		case "rejected":
			rep.SkillDecisions.Rejected++
		}
	}

	rep.LastMaintenance, _, _ = e.GetMeta(metaLastMaintenance)
	if raw, ok, _ := e.GetMeta(MetaLastHealth); ok && raw != "" {
		var dr DiagnoseReport
		if json.Unmarshal([]byte(raw), &dr) == nil {
			rep.Health = dr.Status
		}
	}
	return rep, nil
}
