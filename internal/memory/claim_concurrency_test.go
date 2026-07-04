package memory

import (
	"fmt"
	"sync"
	"testing"
)

// TestClaimWorkUnitConcurrentNoDoubleClaim estresa la ATOMICIDAD del claim de la pizarra bajo
// carrera REAL (goroutines concurrentes), no secuencialmente: N agentes compiten por reclamar M
// unidades del mismo batch. La atomicidad se apoya en el UPDATE...RETURNING guardado por el
// write-lock de SQLite (busy_timeout hace que los escritores esperen, no fallen). Invariantes:
// ninguna unidad la reclaman dos agentes, y se reclaman EXACTAMENTE las M unidades. Correr con
// `go test -race` para detectar data races.
func TestClaimWorkUnitConcurrentNoDoubleClaim(t *testing.T) {
	e := newTestEngine(t)

	const units = 20
	specs := make([]WorkUnitSpec, units)
	for i := range specs {
		specs[i] = WorkUnitSpec{Title: fmt.Sprintf("u%d", i), Spec: "hacer algo"}
	}
	batch, err := e.CreateWorkBatch("b", specs)
	if err != nil {
		t.Fatalf("CreateWorkBatch: %v", err)
	}

	const workers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	claimedBy := make(map[string]string) // unitID -> agente que la reclamó
	dupes := 0

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(agent string) {
			defer wg.Done()
			for {
				u, ok, err := e.ClaimWorkUnit(batch.BatchID, agent, 300, 5)
				if err != nil {
					t.Errorf("claim de %s falló: %v", agent, err) // busy_timeout ⇒ no debería
					return
				}
				if !ok {
					return // batch agotado: no queda nada que reclamar
				}
				mu.Lock()
				if prev, seen := claimedBy[u.ID]; seen {
					dupes++
					t.Errorf("unidad %s reclamada por %s y %s (doble claim)", u.ID, prev, agent)
				}
				claimedBy[u.ID] = agent
				mu.Unlock()
			}
		}(fmt.Sprintf("a%d", w))
	}
	wg.Wait()

	if dupes != 0 {
		t.Errorf("hubo %d claims duplicados", dupes)
	}
	if len(claimedBy) != units {
		t.Errorf("esperaba %d unidades reclamadas exactamente una vez, obtuve %d", units, len(claimedBy))
	}
}
