package memory

import (
	"fmt"
	"sort"
	"strings"
)

// consolidate.go fusiona observaciones casi-duplicadas (auto-mantenimiento
// model-free) para que la memoria no crezca con repeticiones. Usa la similitud
// de trigramas (similarity.go); sin LLM.

// defaultDedupThreshold es el umbral de similitud por defecto para fusionar.
const defaultDedupThreshold = 0.85

// negationTokens son negadores explícitos (es/en) para el guard de POLARIDAD de la
// consolidación. Model-free y deliberadamente ACOTADO: cierra el falso positivo medido
// (auditoría) donde una observación larga negada con un token corto —"no usa Postgres"
// vs "usa Postgres"— empata en trigramas (sim ~0.89 > 0.85) y se fusionaría, ocultando del
// recall la polaridad opuesta. NO pretende resolver antónimos ("seguro"/"inseguro") ni
// negación implícita: eso es el techo semántico model-free (Track 15), delegado al caller/LLM.
var negationTokens = map[string]bool{
	"no": true, "nunca": true, "jamás": true, "jamas": true, "tampoco": true, "ni": true,
	"not": true, "never": true, "neither": true, "nor": true, "cannot": true,
}

// negationCount cuenta tokens de negación explícita en un texto YA normalizado
// (normalizeForSim: minúsculas + espacios colapsados). Cuenta palabras completas, así que
// "no-sql" (un token) no dispara: solo el "no" suelto.
func negationCount(normalized string) int {
	n := 0
	for _, tok := range strings.Fields(normalized) {
		if negationTokens[tok] {
			n++
		}
	}
	return n
}

// ConsolidateResult resume una corrida de consolidación.
type ConsolidateResult struct {
	Scanned int `json:"scanned"`
	Merged  int `json:"merged"`
}

type consObs struct {
	id         string
	content    string
	createdAt  string
	access     int
	importance float64
	projectID  string // atribución (Track 17): la consolidación NO fusiona a través de tenants
}

// Consolidate fusiona observaciones cuya similitud supere threshold. El más
// "fuerte" (más accesos, luego más importante, luego más nuevo) queda como
// canónico; los duplicados se ARCHIVAN (soft-delete reversible) acumulando sus accesos
// y la importancia máxima en el canónico.
func (e *DbEngine) Consolidate(threshold float64) (ConsolidateResult, error) {
	if threshold <= 0 {
		threshold = defaultDedupThreshold
	}

	// Solo memorias VIVAS: excluir archivadas y superseded (coherente con recall,
	// prime, context y conflicts). No tocar una observación ya oculta del recall.
	rows, err := e.db.Query(`
		SELECT id, content, access_count, importance, COALESCE(created_at,''), COALESCE(project_id,'')
		FROM observations WHERE ` + visibleObsPredicate + `
	`)
	if err != nil {
		return ConsolidateResult{}, fmt.Errorf("error al listar observaciones: %w", err)
	}
	var all []consObs
	for rows.Next() {
		var o consObs
		if err := rows.Scan(&o.id, &o.content, &o.access, &o.importance, &o.createdAt, &o.projectID); err != nil {
			rows.Close()
			return ConsolidateResult{}, fmt.Errorf("error al escanear observación: %w", err)
		}
		all = append(all, o)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return ConsolidateResult{}, fmt.Errorf("error al iterar observaciones para consolidar: %w", err)
	}
	rows.Close()

	// Procesar de más fuerte a más débil para que el canónico sea el mejor.
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].access != all[j].access {
			return all[i].access > all[j].access
		}
		if all[i].importance != all[j].importance {
			return all[i].importance > all[j].importance
		}
		return all[i].createdAt > all[j].createdAt
	})

	tx, err := e.db.Begin()
	if err != nil {
		return ConsolidateResult{}, fmt.Errorf("error al iniciar transacción: %w", err)
	}
	defer tx.Rollback()

	// Bloqueo por trigramas para evitar el O(n²): en vez de comparar cada observación
	// contra TODOS los canónicos, se indexan los trigramas de los canónicos y solo se
	// computa el Jaccard exacto contra los que comparten al menos un trigrama. Saltear
	// los de overlap 0 NO cambia el resultado: su Jaccard es 0 < threshold. `byNorm`
	// cubre el atajo de igualdad exacta tras normalizar (incluye textos de <3 runas y
	// vacíos, que pueden no tener trigramas). Mismo criterio de match que el original
	// (el canónico más fuerte, es decir el de menor índice, gana).
	var kept []consObs
	keptTg := []map[string]bool{}  // trigramas de cada canónico (paralelo a kept)
	byNorm := map[string]int{}     // (project_id, contenido normalizado) -> índice de canónico
	inverted := map[string][]int{} // trigrama -> índices de canónicos que lo contienen (GLOBAL)
	var removed []string           // ids de duplicados archivados (para sacar del índice vectorial)
	merged := 0

	// overlap[ki] = #trigramas compartidos con el canónico ki, para la observación actual.
	// Como los ki son densos (0..len(kept)-1), usamos un slice REUTILIZADO entre
	// observaciones en vez de un map[int]int nuevo por iteración: el `++` pasa de un
	// mapassign con hash a un índice de array. `touched` lista los ki tocados para
	// resetear a 0 en O(tocados) sin barrer todo el slice. Mismo resultado exacto que el
	// map original (es solo la estructura de conteo), pero elimina el churn de mapas que
	// dominaba la consolidación a escala (T7.1).
	var overlap []int
	var touched []int

	for _, o := range all {
		norm := normalizeForSim(o.content)
		tg := trigrams(norm)
		oNeg := negationCount(norm) // guard de polaridad: un candidato con distinta cuenta de negación no es duplicado
		// AISLAMIENTO POR TENANT (Track 17): la consolidación NO fusiona a través de proyectos —
		// igual que EnforceQuota agrupa por project_id. La clave de igualdad-exacta incluye el
		// project_id (el NUL nunca aparece en texto normalizado), y el match por trigramas exige
		// mismo proyecto abajo. Con un solo bucket (mono-tenant o federado local '') el
		// comportamiento es bit-idéntico al previo.
		normKey := o.projectID + "\x00" + norm

		matchIdx := -1
		if ki, ok := byNorm[normKey]; ok {
			matchIdx = ki // igualdad exacta tras normalizar, dentro del mismo tenant (Similarity == 1.0)
		} else {
			if len(overlap) < len(kept) {
				overlap = append(overlap, make([]int, len(kept)-len(overlap))...)
			}
			touched = touched[:0]
			for g := range tg {
				for _, ki := range inverted[g] {
					if overlap[ki] == 0 {
						touched = append(touched, ki)
					}
					overlap[ki]++
				}
			}
			for _, ki := range touched {
				ov := overlap[ki]
				overlap[ki] = 0                         // reset para la próxima observación
				denom := len(tg) + len(keptTg[ki]) - ov // |A| + |B| - |A∩B|
				if denom <= 0 {
					continue
				}
				// Sólo fusiona dentro del mismo tenant (aislamiento): un candidato de otro
				// project_id nunca es "duplicado" aunque el texto empate por trigramas. Y guard de
				// POLARIDAD: si difieren en la cuenta de negadores explícitos, no son duplicados
				// (evita fusionar "usa X" con "no usa X", que empatan en trigramas — auditoría).
				if float64(ov)/float64(denom) >= threshold && kept[ki].projectID == o.projectID &&
					negationCount(normalizeForSim(kept[ki].content)) == oNeg {
					if matchIdx == -1 || ki < matchIdx {
						matchIdx = ki
					}
				}
			}
		}

		if matchIdx == -1 {
			ki := len(kept)
			kept = append(kept, o)
			keptTg = append(keptTg, tg)
			if _, ok := byNorm[normKey]; !ok {
				byNorm[normKey] = ki
			}
			for g := range tg {
				inverted[g] = append(inverted[g], ki)
			}
			continue
		}

		k := &kept[matchIdx]
		k.access += o.access
		if o.importance > k.importance {
			k.importance = o.importance
		}
		if _, err := tx.Exec(`UPDATE observations SET access_count=?, importance=? WHERE id=?`,
			k.access, k.importance, k.id); err != nil {
			return ConsolidateResult{}, fmt.Errorf("error al actualizar canónico: %w", err)
		}
		// Soft-delete reversible (T5.5): en vez de borrar físicamente el duplicado, se
		// archiva y se apunta al canónico. Queda oculto del recall (archived + superseded)
		// pero recuperable; el borrado definitivo lo hace PurgeArchived tras el período de
		// gracia de retención, que limpia relaciones y embeddings. Así una fusión por falso
		// positivo de trigramas no pierde datos. archived_at = ahora arranca la ventana de
		// gracia desde el archivado.
		if _, err := tx.Exec(`UPDATE observations SET archived=1, archived_at=CURRENT_TIMESTAMP, superseded_by=? WHERE id=?`, k.id, o.id); err != nil {
			return ConsolidateResult{}, fmt.Errorf("error al archivar duplicado: %w", err)
		}
		// Re-apuntar los punteros superseded_by que apuntaban al duplicado hacia el canónico
		// vivo (aplana la cadena; el duplicado ya quedó apuntando a k.id arriba).
		if _, err := tx.Exec(`UPDATE observations SET superseded_by=? WHERE superseded_by=?`, k.id, o.id); err != nil {
			return ConsolidateResult{}, fmt.Errorf("error al re-apuntar punteros superseded_by: %w", err)
		}
		removed = append(removed, o.id)
		merged++
	}

	if err := tx.Commit(); err != nil {
		return ConsolidateResult{}, fmt.Errorf("error al commitear consolidación: %w", err)
	}

	// Sacar los duplicados borrados del índice vectorial (post-commit), en lote: un solo
	// Lock y una pasada por celda, no un Remove por id (evita el O(n²) del mantenimiento).
	if e.index != nil {
		e.index.RemoveBatch(removed)
	}

	return ConsolidateResult{Scanned: len(all), Merged: merged}, nil
}
