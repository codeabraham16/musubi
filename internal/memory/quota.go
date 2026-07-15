package memory

import (
	"container/heap"
	"fmt"
	"strings"
	"time"
)

// quota.go acota el crecimiento por CUOTA, no por tiempo: mantiene el set ACTIVO de cada
// tenant (project_id) por debajo de un techo de N observaciones, archivando las más frías
// (menor saliencia) hasta volver bajo el techo. Complementa el olvido (Decay), que archiva
// por un umbral ABSOLUTO de saliencia: un tenant de alto ingest cuyas memorias nunca bajan
// del umbral crecería sin límite igual. La cuota es el bound que SIEMPRE aplica.
//
// POR TENANT y no global: en el cerebro central multi-tenant, una cuota global dejaría que un
// proyecto ruidoso desalojara la memoria de otro (daño cross-tenant, e injusto). Cada
// project_id se acota por separado; un nodo local de un solo proyecto tiene un único bucket
// (project_id vacío = su propio bucket, no se mezcla con los atribuidos).
//
// Evicción = ARCHIVAR (reversible, flag archived), igual que el olvido: la purga por edad
// (PurgeArchived) hace el borrado duro después, con su período de gracia. Así la cuota nunca
// borra memoria de forma irreversible por sí sola.

// QuotaOptions configura la cuota de crecimiento.
type QuotaOptions struct {
	// MaxActivePerProject es el techo de observaciones ACTIVAS por project_id. <= 0 desactiva
	// la cuota (comportamiento histórico: sólo el olvido por umbral acota el set activo).
	MaxActivePerProject int
	// ProtectImportance protege de la evicción a las observaciones con importance >= a este
	// valor (conocimiento deliberado), igual que en el olvido. Cuentan para la cuota pero no
	// se archivan: si un tenant supera el techo sólo con memoria protegida, la cuota respeta
	// la protección y NO fuerza el desalojo (best-effort bajo protecciones).
	ProtectImportance float64
	// MinAgeDays nunca evicciona memorias más nuevas que esto (período de gracia): una ráfaga
	// de ingest reciente no se auto-archiva antes de tener chance de ser útil. 0 = sin gracia.
	MinAgeDays float64
	// HalfLifeDays y ReinforcementK parametrizan la saliencia (misma fórmula que el olvido) con
	// la que se rankea qué evictar: se archiva de MENOR a mayor saliencia (lo más frío primero).
	HalfLifeDays   float64
	ReinforcementK float64
}

// EnforceQuota archiva, por cada tenant que supera MaxActivePerProject, las observaciones
// activas MÁS FRÍAS (menor saliencia) hasta volver bajo el techo. Devuelve cuántas archivó.
// Respeta las mismas protecciones que el olvido (importancia deliberada, período de gracia) y
// NUNCA evicciona memoria no sincronizada (fila de outbox no 'sent'): archivarla podría dejarla
// varada sin llegar nunca al central. En el cerebro central (nodo terminal, sin outbox saliente)
// esa cláusula no excluye nada; en un cliente, protege lo que aún no viajó.
func (e *DbEngine) EnforceQuota(opts QuotaOptions) (int, error) {
	if opts.MaxActivePerProject <= 0 {
		return 0, nil
	}
	if opts.HalfLifeDays <= 0 {
		opts.HalfLifeDays = defaultHalfLifeDays
	}

	// Proyectos cuyo set activo supera el techo. El GROUP BY/HAVING corre en SQL (barato) para
	// no traer NADA de los proyectos que están dentro de la cuota — el caso común.
	rows, err := e.db.Query(`
		SELECT COALESCE(project_id,''), COUNT(*)
		FROM observations
		WHERE archived = 0
		GROUP BY COALESCE(project_id,'')
		HAVING COUNT(*) > ?`, opts.MaxActivePerProject)
	if err != nil {
		return 0, fmt.Errorf("error al listar proyectos sobre la cuota: %w", err)
	}
	type overCap struct {
		project string
		count   int
	}
	var over []overCap
	for rows.Next() {
		var oc overCap
		if err := rows.Scan(&oc.project, &oc.count); err != nil {
			rows.Close()
			return 0, fmt.Errorf("error al escanear proyecto sobre la cuota: %w", err)
		}
		over = append(over, oc)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, fmt.Errorf("error al iterar proyectos sobre la cuota: %w", err)
	}
	rows.Close()

	now := time.Now().UTC()
	var toArchive []string
	for _, oc := range over {
		excess := oc.count - opts.MaxActivePerProject
		ids, err := e.coldestEvictable(oc.project, excess, opts, now)
		if err != nil {
			return 0, err
		}
		toArchive = append(toArchive, ids...)
	}

	if len(toArchive) == 0 {
		return 0, nil
	}

	// Archivar en tandas (respetar el tope de parámetros enlazados): la primera enforcement
	// sobre una base grande puede evictar muchas de una. archived_at = ahora para que la
	// ventana de gracia de la purga cuente DESDE el archivado. Mismo mecanismo que el olvido.
	for _, chunk := range chunkStrings(toArchive, maxSQLParams) {
		placeholders := make([]string, len(chunk))
		args := make([]interface{}, len(chunk))
		for i, id := range chunk {
			placeholders[i] = "?"
			args[i] = id
		}
		q := `UPDATE observations SET archived = 1, archived_at = CURRENT_TIMESTAMP WHERE id IN (` + strings.Join(placeholders, ",") + `)`
		if _, err := e.db.Exec(q, args...); err != nil {
			return 0, fmt.Errorf("error al archivar por cuota: %w", err)
		}
	}
	// Sacar del índice vectorial las evictadas (dejan de ser elegibles para el recall).
	if e.index != nil {
		e.index.RemoveBatch(toArchive)
	}

	return len(toArchive), nil
}

// coldestEvictable devuelve los ids de las `excess` observaciones activas MÁS FRÍAS (menor
// saliencia) del proyecto que son elegibles para evicción (no protegidas por importancia, más
// viejas que la gracia, y sincronizadas). Escanea en streaming con un max-heap ACOTADO a
// `excess`: memoria O(excess), no O(activas del proyecto) — no re-materializamos el corpus.
func (e *DbEngine) coldestEvictable(project string, excess int, opts QuotaOptions, now time.Time) ([]string, error) {
	if excess <= 0 {
		return nil, nil
	}
	// Candidatas: activas del proyecto y SINCRONIZADAS (sin fila de outbox pendiente/no-'sent').
	// La saliencia y la edad se computan en Go con la MISMA fórmula del olvido, para que "frío"
	// signifique exactamente lo mismo en ambos y no haya divergencia float Go/SQLite.
	rows, err := e.db.Query(`
		SELECT o.id, o.access_count, o.importance, COALESCE(o.created_at,''), COALESCE(o.last_accessed,''), COALESCE(o.mem_type,'')
		FROM observations o
		WHERE o.archived = 0 AND COALESCE(o.project_id,'') = ?
		  AND NOT EXISTS (SELECT 1 FROM outbox b WHERE b.obs_id = o.id AND b.status != 'sent')`,
		project)
	if err != nil {
		return nil, fmt.Errorf("error al listar candidatas a evicción de %q: %w", project, err)
	}
	defer rows.Close()

	h := &coldHeap{}
	heap.Init(h)
	for rows.Next() {
		var (
			id                    string
			access                int
			importance            float64
			createdAt, lastAccess string
			memType               string
		)
		if err := rows.Scan(&id, &access, &importance, &createdAt, &lastAccess, &memType); err != nil {
			return nil, fmt.Errorf("error al escanear candidata a evicción: %w", err)
		}
		// Protección por importancia: el conocimiento deliberado cuenta para la cuota pero no
		// se evicta (idéntico al olvido).
		if opts.ProtectImportance > 0 && importance >= opts.ProtectImportance {
			continue
		}
		ts := lastAccess
		if strings.TrimSpace(ts) == "" {
			ts = createdAt
		}
		t, perr := time.Parse(sqliteTimeLayout, ts)
		if perr != nil {
			continue // sin timestamp parseable: no se evicta (no podemos datarla)
		}
		ageDays := now.Sub(t).Hours() / 24
		if ageDays < opts.MinAgeDays {
			continue // dentro del período de gracia
		}
		sal := salience(importance, access, ageDays, opts.HalfLifeDays, memTypeSalienceWeight(memType), opts.ReinforcementK)
		heap.Push(h, evictCand{id: id, salience: sal})
		// Acotar a `excess`: al pasarse, sacar la raíz = la de MAYOR saliencia entre las
		// retenidas (la menos merecedora de evicción). Quedan las `excess` más frías.
		if h.Len() > excess {
			heap.Pop(h)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error al iterar candidatas a evicción: %w", err)
	}

	ids := make([]string, h.Len())
	for i := range ids {
		ids[i] = (*h)[i].id
	}
	return ids, nil
}

// evictCand es un candidato a evicción con su saliencia ya calculada.
type evictCand struct {
	id       string
	salience float64
}

// coldHeap es un MAX-heap por (saliencia, id): container/heap pone en la raíz el elemento
// "menor" según Less, y acá definimos "menor" = MAYOR saliencia, de modo que la raíz sea el
// candidato MENOS merecedor de evicción. Manteniéndolo acotado a `excess` (Pop de la raíz al
// pasarse), al final quedan las `excess` observaciones de MENOR saliencia (las más frías). El
// desempate por id hace el resultado determinista (clave para tests estables).
type coldHeap []evictCand

func (h coldHeap) Len() int { return len(h) }
func (h coldHeap) Less(i, j int) bool {
	if h[i].salience != h[j].salience {
		return h[i].salience > h[j].salience
	}
	return h[i].id > h[j].id
}
func (h coldHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *coldHeap) Push(x any)   { *h = append(*h, x.(evictCand)) }
func (h *coldHeap) Pop() any {
	old := *h
	n := len(old)
	it := old[n-1]
	*h = old[:n-1]
	return it
}
