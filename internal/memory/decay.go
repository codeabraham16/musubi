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
	// ReinforcementK es la fuerza del refuerzo de Ebbinghaus (B3): cada acceso alarga la
	// vida media efectiva de la recencia (memorias calientes se olvidan más lento). 0 =
	// vida media fija (comportamiento histórico). Ver effectiveHalfLife.
	ReinforcementK float64
}

// decayBatchSize es el tamaño de página del scan de olvido. Es var (no const) para que
// los tests puedan forzar múltiples páginas con pocos datos.
var decayBatchSize = 1000

// DecayResult resume una corrida de olvido.
type DecayResult struct {
	Scanned  int `json:"scanned"`
	Archived int `json:"archived"`
}

// effectiveHalfLife aplica el REFUERZO de Ebbinghaus (spacing effect): cada acceso (repaso)
// fortalece la memoria y alarga su vida media, de modo que las memorias "calientes"
// (frecuentemente accedidas) se olvidan más lento. effHL = halfLife * (1 + K*ln(1+access)).
// K=0 → vida media fija (comportamiento pre-B3). Clamp defensivo: K y access negativos no
// encogen la vida media (effHL >= halfLife), así el refuerzo nunca acelera el olvido.
func effectiveHalfLife(halfLifeDays float64, accessCount int, reinforcementK float64) float64 {
	if reinforcementK <= 0 || accessCount <= 0 {
		return halfLifeDays
	}
	return halfLifeDays * (1 + reinforcementK*math.Log(1+float64(accessCount)))
}

// salience combina importancia, frecuencia (log), recencia (decaimiento exponencial sobre la
// vida media EFECTIVA reforzada por acceso — Ebbinghaus, B3) y el peso del TIPO de memoria
// (typeWeight: episódico se enfría antes, procedural persiste; sin tipo = 1.0, neutro — B2).
// reinforcementK=0 y typeWeight=1.0 reproducen exactamente la fórmula previa. Determinista,
// sin LLM. La frecuencia (uso) y el refuerzo (rehearsal que ralentiza el olvido) son ejes
// distintos y se combinan a propósito.
func salience(importance float64, accessCount int, ageDays, halfLifeDays, typeWeight, reinforcementK float64) float64 {
	freq := 1 + math.Log(1+float64(accessCount))
	recency := math.Pow(0.5, ageDays/effectiveHalfLife(halfLifeDays, accessCount, reinforcementK))
	return importance * freq * recency * typeWeight
}

// edadDesde devuelve el instante desde el que el olvido y la cuota cuentan la edad de una nota: su
// último acceso, o su creación si nunca se accedió. Y si la nota BAJÓ del central (llegada es el
// created_at de su sello 'espejo' en el outbox) y llegó después, cuenta desde la llegada.
//
// ES LA GRACIA DESDE LA LLEGADA. Desde que la nota bajada guarda la fecha en que el central la
// recibió (fechaDeOrigen), una de hace tres meses entra con tres meses encima y sin un solo acceso:
// el primer mantenimiento de un cliente nuevo la archivaba antes de que nadie la pudiera usar. Con
// esto, lo que baja tiene la misma gracia que lo que se escribe acá, y después se enfría al mismo
// ritmo, contando desde que llegó.
//
// Toma el MÁS NUEVO de los dos instantes: una nota usada después de llegar cuenta desde el uso, y un
// sello anterior a la fecha de la nota no cambia nada. Si la fecha de la nota no se puede leer
// devuelve ok=false, como siempre, aunque el sello sí se lea.
//
// EN EL OLVIDO ESO SÓLO ARCHIVA MENOS; EN LA CUOTA, NO. Una edad menor sube la saliencia y demora la
// edad mínima, así que Decay nunca archiva una nota que antes no archivaba. La cuota, en cambio,
// desaloja un excedente FIJO: si una nota recién bajada queda en su gracia, su lugar lo ocupa la
// siguiente más fría, y ésa puede ser una nota local que nunca subió al central. Es lo decidido: la
// cuota cuenta la misma edad que el olvido.
//
// Y EL SELLO NO SIEMPRE ES UNA LLEGADA. Una nota propia cuyo envío murió ('dead') y que vuelve del
// central pasa a 'espejo' con la fecha de su encolado, porque el UPSERT del sello no toca created_at;
// y lo mismo quedó en las 'sent' que el sello de #656 pasó a 'espejo' hasta #693. Ahí la edad cuenta
// desde el encolado, y la nota sólo se archiva más tarde. Que una re-bajada NO renueve la llegada
// depende de ese mismo UPSERT: si algún día tocara created_at, lo que el central vuelve a entregar
// no se olvidaría mientras volviera a bajar antes de enfriarse.
func edadDesde(createdAt, lastAccess, llegada string) (time.Time, bool) {
	ts := lastAccess
	if strings.TrimSpace(ts) == "" {
		ts = createdAt
	}
	t, err := time.Parse(sqliteTimeLayout, ts)
	if err != nil {
		return time.Time{}, false
	}
	l, err := time.Parse(sqliteTimeLayout, llegada)
	if err == nil && l.After(t) {
		return l, true
	}
	return t, true
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
		// El sello 'espejo' dice cuándo llegó una nota bajada (ver edadDesde). outbox tiene
		// UNIQUE(obs_id): el JOIN trae a lo sumo una fila por nota y la paginación no cambia.
		rows, err := e.db.Query(`
			SELECT o.id, o.access_count, o.importance, COALESCE(o.created_at,''), COALESCE(o.last_accessed,''), COALESCE(o.mem_type,''),
			       COALESCE(esp.created_at,'')
			FROM observations o
			LEFT JOIN outbox esp ON esp.obs_id = o.id AND esp.status = 'espejo'
			WHERE o.archived = 0 AND o.id > ?
			ORDER BY o.id LIMIT ?
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
				memType, llegada      string
			)
			if err := rows.Scan(&id, &access, &importance, &createdAt, &lastAccess, &memType, &llegada); err != nil {
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

			t, ok := edadDesde(createdAt, lastAccess, llegada)
			if !ok {
				continue // sin timestamp parseable: no se archiva
			}
			ageDays := now.Sub(t).Hours() / 24
			if ageDays < opts.MinAgeDays {
				continue
			}
			if salience(importance, access, ageDays, opts.HalfLifeDays, memTypeSalienceWeight(memType), opts.ReinforcementK) < opts.MinSalience {
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
			e.index.RemoveBatch(toArchive)
		}
	}

	return DecayResult{Scanned: scanned, Archived: len(toArchive)}, nil
}
