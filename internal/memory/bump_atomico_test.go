package memory

import (
	"context"
	"sync"
	"testing"
)

// G10 del spec «El motor no traba la casa» (specs/motor-sin-candado/).
//
// POR QUÉ ESTA PRUEBA EXISTE. El candado EXCLUSIVO que musubi_recall tomaba en el dispatcher se
// justificaba con «bumpea contadores de acceso», y el comentario del candado dice proteger
// «lost-updates de read-modify-write». Pero el bump es una sola sentencia:
//
//	UPDATE observations SET last_accessed = CURRENT_TIMESTAMP, access_count = access_count + 1 ...
//
// que SQLite ya serializa. Sacar el candado exclusivo del recall se apoya en ese hecho, así que
// acá se fija como CONTRATO en vez de dejarlo como suposición de quien leyó la query. Si alguien
// convierte el bump en un read-modify-write de Go (leer, sumar en memoria, escribir), los recalls
// concurrentes empiezan a perder incrementos y esta prueba se pone roja.
func TestG10RecallsConcurrentesNoPierdenAccesos(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer e.Close()

	const id = "obs-bump-atomico"
	if err := e.SaveObservation(id, "candado/bump", "el bump de accesos es atomico en SQL y no necesita candado de Go", nil); err != nil {
		t.Fatalf("SaveObservation: %v", err)
	}

	const n = 32
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Sin NoBump: justamente queremos que cada recall incremente.
			if _, err := e.Recall(context.Background(), "bump accesos atomico candado", RecallOptions{TokenBudget: 500}); err != nil {
				t.Errorf("Recall concurrente falló: %v", err)
			}
		}()
	}
	wg.Wait()

	var accesos int
	if err := e.db.QueryRow(`SELECT access_count FROM observations WHERE id = ?`, id).Scan(&accesos); err != nil {
		t.Fatalf("no pude leer access_count: %v", err)
	}
	if accesos != n {
		t.Fatalf("esperaba access_count=%d tras %d recalls concurrentes, obtuve %d: se perdieron %d incrementos — el bump dejó de ser atómico",
			n, n, accesos, n-accesos)
	}
}
