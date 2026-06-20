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
	// ProtectImportance protege del olvido a las observaciones con importance >= a este
	// valor (conocimiento deliberado). 0 = sin protección.
	ProtectImportance float64
}

// decayBatchSize es el tamaño de página del scan de olvido. Es var (no const) para que
// los tests puedan forzar múltiples páginas con pocos datos.
var decayBatchSize = 1000

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

	// Scan paginado por keyset (id > lastID): acota la memoria en bases grandes en vez de
	// cargar TODO el set activo de una. La saliencia se computa en Go con la MISMA fórmula
	// de siempre (no se mueve a SQL): así el conjunto archivado es idéntico al histórico,
	// sin riesgo de regresión por diferencias de float/timestamps entre Go y SQLite.
	now := time.Now().UTC()
	var toArchive []string
	scanned := 0
	lastID := ""
	for {
		rows, err := e.db.Query(`
			SELECT id, access_count, importance, COALESCE(created_at,''), COALESCE(last_accessed,'')
			FROM observations WHERE archived = 0 AND id > ?
			ORDER BY id LIMIT ?
		`, lastID, decayBatchSize)
		if err != nil {
			return DecayResult{}, fmt.Errorf("error al listar observaciones: %w", err)
		}
		batch := 0
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
			lastID = id
			batch++
			scanned++

			// Protección por importancia: el conocimiento deliberado no se auto-archiva.
			if opts.ProtectImportance > 0 && importance >= opts.ProtectImportance {
				continue
			}

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
		if batch < decayBatchSize {
			break // última página
		}
	}

	if len(toArchive) > 0 {
		// Trocear el IN(...) para respetar el tope de parámetros enlazados: un primer
		// mantenimiento sobre una base grande puede archivar miles de filas de una sola
		// pasada. Se marca archived_at = ahora para que la ventana de retención de la
		// purga cuente DESDE el archivado (período de gracia), no desde el último acceso.
		for _, chunk := range chunkStrings(toArchive, maxSQLParams) {
			placeholders := make([]string, len(chunk))
			args := make([]interface{}, len(chunk))
			for i, id := range chunk {
				placeholders[i] = "?"
				args[i] = id
			}
			q := `UPDATE observations SET archived = 1, archived_at = CURRENT_TIMESTAMP WHERE id IN (` + strings.Join(placeholders, ",") + `)`
			if _, err := e.db.Exec(q, args...); err != nil {
				return DecayResult{}, fmt.Errorf("error al archivar memorias frías: %w", err)
			}
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
