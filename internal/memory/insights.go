package memory

import (
	"encoding/json"
	"fmt"
)

// insights.go implementa el resumen de "observabilidad activa" (Track 6 / T6.4): un único
// reporte model-free de lo que Musubi sabe de sí mismo — tamaño de la memoria, hotspots de
// errores no resueltos, decisiones de skills y salud del ciclo de mantenimiento. Todo es
// agregación SQL/aritmética determinista; sin LLM.

// insightsHotspotLimit acota cuántos archivos con más errores no resueltos se listan.
const insightsHotspotLimit = 10

// InsightsReport es el resumen agregado del estado de la memoria.
type InsightsReport struct {
	Observations     ObsStats       `json:"observations"`
	UnresolvedErrors int            `json:"unresolved_errors"`
	ErrorHotspots    []ErrorHotspot `json:"error_hotspots"`
	SkillDecisions   DecisionStats  `json:"skill_decisions"`
	LastMaintenance  string         `json:"last_maintenance,omitempty"`
	Health           string         `json:"health,omitempty"`
}

// ObsStats resume el tamaño de la memoria de observaciones.
type ObsStats struct {
	Total    int `json:"total"`
	Active   int `json:"active"`
	Archived int `json:"archived"`
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

// Insights agrega el estado de la memoria en un único reporte (T6.4). Read-only.
func (e *DbEngine) Insights() (InsightsReport, error) {
	var rep InsightsReport

	if err := e.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(CASE WHEN archived=1 THEN 1 ELSE 0 END),0) FROM observations`,
	).Scan(&rep.Observations.Total, &rep.Observations.Archived); err != nil {
		return rep, fmt.Errorf("insights: contar observaciones: %w", err)
	}
	rep.Observations.Active = rep.Observations.Total - rep.Observations.Archived

	if err := e.db.QueryRow(
		`SELECT COUNT(*) FROM telemetry_logs WHERE resolved=0`,
	).Scan(&rep.UnresolvedErrors); err != nil {
		return rep, fmt.Errorf("insights: contar errores: %w", err)
	}

	rows, err := e.db.Query(`
		SELECT file_path, COUNT(*) AS c FROM telemetry_logs WHERE resolved=0
		GROUP BY file_path ORDER BY c DESC, file_path ASC LIMIT ?`, insightsHotspotLimit)
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

	// Decisiones de skills por su decisión más reciente (last-write-wins, coherente con T6.1).
	decisions, err := e.GetSkillDecisions()
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
