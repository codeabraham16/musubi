package memory

import (
	"fmt"
	"sort"
)

// consolidate.go fusiona observaciones casi-duplicadas (auto-mantenimiento
// model-free) para que la memoria no crezca con repeticiones. Usa la similitud
// de trigramas (similarity.go); sin LLM.

// defaultDedupThreshold es el umbral de similitud por defecto para fusionar.
const defaultDedupThreshold = 0.85

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
		SELECT id, content, access_count, importance, COALESCE(created_at,'')
		FROM observations WHERE archived = 0 AND superseded_by IS NULL
	`)
	if err != nil {
		return ConsolidateResult{}, fmt.Errorf("error al listar observaciones: %w", err)
	}
	var all []consObs
	for rows.Next() {
		var o consObs
		if err := rows.Scan(&o.id, &o.content, &o.access, &o.importance, &o.createdAt); err != nil {
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
	byNorm := map[string]int{}     // contenido normalizado -> índice de canónico
	inverted := map[string][]int{} // trigrama -> índices de canónicos que lo contienen
	var removed []string           // ids de duplicados archivados (para sacar del índice vectorial)
	merged := 0

	for _, o := range all {
		norm := normalizeForSim(o.content)
		tg := trigrams(norm)

		matchIdx := -1
		if ki, ok := byNorm[norm]; ok {
			matchIdx = ki // igualdad exacta tras normalizar (Similarity == 1.0)
		} else {
			overlap := map[int]int{}
			for g := range tg {
				for _, ki := range inverted[g] {
					overlap[ki]++
				}
			}
			for ki, ov := range overlap {
				denom := len(tg) + len(keptTg[ki]) - ov // |A| + |B| - |A∩B|
				if denom <= 0 {
					continue
				}
				if float64(ov)/float64(denom) >= threshold {
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
			if _, ok := byNorm[norm]; !ok {
				byNorm[norm] = ki
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

	// Sacar los duplicados borrados del índice vectorial (post-commit).
	if e.index != nil {
		for _, id := range removed {
			e.index.Remove(id)
		}
	}

	return ConsolidateResult{Scanned: len(all), Merged: merged}, nil
}
