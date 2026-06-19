package memory

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// decay.go implementa el olvido por SALIENCIA (auto-mantenimiento model-free):
// las memorias frías, poco usadas y poco importantes se archivan para que el
// recall siga filoso. El archivado es reversible (flag archived), no borra datos.

// Defaults del olvido.
const (
	defaultHalfLifeDays = 30.0
	defaultMinSalience  = 0.2
	defaultMinAgeDays   = 14.0
)

// sqliteTimeLayout es el formato de CURRENT_TIMESTAMP de SQLite (UTC).
const sqliteTimeLayout = "2006-01-02 15:04:05"

// DecayOptions configura el olvido. Los ceros usan los defaults.
type DecayOptions struct {
	HalfLifeDays float64 // vida media de la recencia (días)
	MinSalience  float64 // por debajo de esto, una memoria fría se archiva
	MinAgeDays   float64 // nunca archivar memorias más nuevas que esto
}

// DecayResult resume una corrida de olvido.
type DecayResult struct {
	Scanned  int `json:"scanned"`
	Archived int `json:"archived"`
}

// salience combina importancia, frecuencia (log) y recencia (decaimiento
// exponencial). Determinista, sin LLM.
func salience(importance float64, accessCount int, ageDays, halfLifeDays float64) float64 {
	freq := 1 + math.Log(1+float64(accessCount))
	recency := math.Pow(0.5, ageDays/halfLifeDays)
	return importance * freq * recency
}

// Decay archiva las observaciones frías cuya saliencia cae por debajo de
// MinSalience y que son más viejas que MinAgeDays.
func (e *DbEngine) Decay(opts DecayOptions) (DecayResult, error) {
	if opts.HalfLifeDays <= 0 {
		opts.HalfLifeDays = defaultHalfLifeDays
	}
	if opts.MinSalience <= 0 {
		opts.MinSalience = defaultMinSalience
	}
	if opts.MinAgeDays <= 0 {
		opts.MinAgeDays = defaultMinAgeDays
	}

	rows, err := e.db.Query(`
		SELECT id, access_count, importance, COALESCE(created_at,''), COALESCE(last_accessed,'')
		FROM observations WHERE archived = 0
	`)
	if err != nil {
		return DecayResult{}, fmt.Errorf("error al listar observaciones: %w", err)
	}

	now := time.Now().UTC()
	var toArchive []string
	scanned := 0
	for rows.Next() {
		var (
			id                    string
			access                int
			importance            float64
			createdAt, lastAccess string
		)
		if err := rows.Scan(&id, &access, &importance, &createdAt, &lastAccess); err != nil {
			rows.Close()
			return DecayResult{}, fmt.Errorf("error al escanear observación: %w", err)
		}
		scanned++

		ts := lastAccess
		if strings.TrimSpace(ts) == "" {
			ts = createdAt
		}
		t, perr := time.Parse(sqliteTimeLayout, ts)
		if perr != nil {
			continue // sin timestamp parseable: no se archiva
		}
		ageDays := now.Sub(t).Hours() / 24
		if ageDays < opts.MinAgeDays {
			continue
		}
		if salience(importance, access, ageDays, opts.HalfLifeDays) < opts.MinSalience {
			toArchive = append(toArchive, id)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return DecayResult{}, fmt.Errorf("error al iterar observaciones para decay: %w", err)
	}
	rows.Close()

	if len(toArchive) > 0 {
		placeholders := make([]string, len(toArchive))
		args := make([]interface{}, len(toArchive))
		for i, id := range toArchive {
			placeholders[i] = "?"
			args[i] = id
		}
		q := `UPDATE observations SET archived = 1 WHERE id IN (` + strings.Join(placeholders, ",") + `)`
		if _, err := e.db.Exec(q, args...); err != nil {
			return DecayResult{}, fmt.Errorf("error al archivar memorias frías: %w", err)
		}
		// Sacar del índice vectorial las que se archivaron (dejan de ser elegibles).
		// El re-filtro SQL ya garantiza correctness; esto mantiene afilado el recall.
		if e.index != nil {
			for _, id := range toArchive {
				e.index.Remove(id)
			}
		}
	}

	return DecayResult{Scanned: scanned, Archived: len(toArchive)}, nil
}
