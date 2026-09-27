package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// centralConMemoria es un central de juguete que GUARDA lo que le suben y lo devuelve en la bajada,
// con un cursor que sube en cada guardado como el sync_seq del de verdad. Es lo mínimo para que
// exista el rebote: lo que esta máquina sube vuelve en el pull siguiente.
type centralConMemoria struct {
	mu        sync.Mutex
	seq       int64
	notas     map[string]memory.SharedObs
	guardados []string // el contenido de cada save recibido, en orden
	// mientrasViaja, si no es nil, corre ADENTRO del save, antes de contestar: lo que pasa en la
	// máquina mientras el push está en vuelo.
	mientrasViaja func(content string)
}

func newCentralConMemoria() *centralConMemoria {
	return &centralConMemoria{notas: map[string]memory.SharedObs{}}
}

func (c *centralConMemoria) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		} `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch req.Params.Name {
	case "musubi_save_observation":
		var a struct {
			ID         string  `json:"id"`
			TopicKey   string  `json:"topic_key"`
			Content    string  `json:"content"`
			Importance float64 `json:"importance"`
			MemType    string  `json:"mem_type"`
			ProjectID  string  `json:"project_id"`
		}
		_ = json.Unmarshal(req.Params.Arguments, &a)
		c.mu.Lock()
		c.seq++
		c.notas[a.ID] = memory.SharedObs{RowID: c.seq, ID: a.ID, TopicKey: a.TopicKey, Content: a.Content,
			Importance: a.Importance, MemType: a.MemType, Author: "davantis-mando-admin", ProjectID: a.ProjectID}
		c.guardados = append(c.guardados, a.Content)
		enVuelo := c.mientrasViaja
		c.mu.Unlock()
		if enVuelo != nil {
			enVuelo(a.Content)
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"x","result":{"content":[{"type":"text","text":"ok"}]}}`))
	case "musubi_sync_pull":
		var a struct {
			AfterRowID int64 `json:"after_rowid"`
		}
		_ = json.Unmarshal(req.Params.Arguments, &a)
		c.mu.Lock()
		items := []memory.SharedObs{}
		next := a.AfterRowID
		for _, o := range c.notas {
			if o.RowID > a.AfterRowID {
				items = append(items, o)
				if o.RowID > next {
					next = o.RowID
				}
			}
		}
		c.mu.Unlock()
		sort.Slice(items, func(i, j int) bool { return items[i].RowID < items[j].RowID })
		payload, _ := json.Marshal(map[string]interface{}{"items": items, "next_cursor": next})
		resp, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": "pull",
			"result": map[string]interface{}{"content": []map[string]string{{"type": "text", "text": string(payload)}}}})
		_, _ = w.Write(resp)
	default:
		http.Error(w, "tool desconocida: "+req.Params.Name, http.StatusBadRequest)
	}
}

// TestUnaEdicionEnVueloLlegaAlCentral es el caso real entero, con DOS PROCESOS sobre una misma base
// —el daemon que drena y el hook que guarda— y un central que devuelve lo que se le sube:
//
//  1. el daemon reclama v1 y la empuja;
//  2. mientras viaja, el otro proceso guarda v2;
//  3. el central acepta v1;
//  4. la bajada trae v1 de vuelta: el rebote;
//  5. el tick siguiente de subida, y la bajada de lo que haya subido.
//
// En main, (3) dejaba la fila 'sent' con v2 adentro y (4) le pisaba el contenido: el central se
// quedaba con v1, esta máquina también, y v2 no existía en ningún lado. Ahora v2 llega al central, y
// el viaje de la bajada cuenta un rebote y ningún choque.
//
// Sabotaje: que el drain no le pase a la marca el hash de lo que empujó.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="s.engine.MarkOutboxSent(item.ObsID, item.Hash)"
// arnes: a="s.engine.MarkOutboxSent(item.ObsID, \"\")"
func TestUnaEdicionEnVueloLlegaAlCentral(t *testing.T) {
	const v1, v2 = "la nota tal como salio al central", "la nota editada mientras la otra viajaba"
	dir := memtest.DirSembrado(t)
	abrir := func() *memory.DbEngine {
		e, err := memory.NewDbEngine(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { e.Close() })
		return e
	}
	daemon, hook := abrir(), abrir()

	central := newCentralConMemoria()
	srv := httptest.NewServer(central)
	defer srv.Close()
	s := NewMcpServer(daemon, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, srv.URL), config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, BackoffBaseSeconds: 5, BackoffMaxSeconds: 300})
	ctx := context.Background()

	if err := hook.SaveObservationTyped("nota-1", "t/x", v1, 1, "semantic", memory.ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	central.mu.Lock()
	central.mientrasViaja = func(content string) {
		if content != v1 {
			return
		}
		if err := hook.SaveObservationTyped("nota-1", "t/x", v2, 1, "semantic", memory.ScopeShared, nil); err != nil {
			t.Errorf("la edición en vuelo no se pudo guardar: %v", err)
		}
	}
	central.mu.Unlock()

	s.drainOutboxOnce(ctx) // (1)-(3): sube v1 y, mientras viaja, el hook guarda v2
	central.mu.Lock()
	central.mientrasViaja = nil
	central.mu.Unlock()
	s.drainInboundOnce(ctx) // (4): baja v1
	s.drainOutboxOnce(ctx)  // (5)
	s.drainInboundOnce(ctx)

	central.mu.Lock()
	enElCentral := central.notas["nota-1"].Content
	guardados := append([]string(nil), central.guardados...)
	central.mu.Unlock()
	obs, err := daemon.GetObservations([]string{"nota-1"})
	if err != nil || len(obs) != 1 {
		t.Fatalf("leer la nota local: %v (%d filas)", err, len(obs))
	}
	pending, sent, dead, err := daemon.OutboxStats()
	if err != nil {
		t.Fatal(err)
	}
	r, err := daemon.ResumenDelSync(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if enElCentral != v2 {
		t.Errorf("EDICIÓN PERDIDA: el central terminó con %q; esperaba v2", enElCentral)
	}
	if obs[0].Content != v2 {
		t.Errorf("EDICIÓN PERDIDA: esta máquina terminó con %q; esperaba v2", obs[0].Content)
	}
	if pending != 0 || sent != 1 || dead != 0 {
		t.Errorf("outbox pending=%d sent=%d dead=%d; esperaba 0/1/0 (v2 entregada y nada colgado)", pending, sent, dead)
	}
	if r.BajadaHoy.Rebotes != 1 || r.BajadaHoy.Choques != 0 {
		t.Errorf("sync_viajes cuenta rebotes=%d choques=%d; esperaba 1 y 0 (v1 volvió mientras v2 esperaba, y no la escribió nadie más)",
			r.BajadaHoy.Rebotes, r.BajadaHoy.Choques)
	}
	// Lo que pasó, también en verde (con -v): es la medición del caso, no sólo su veredicto.
	t.Logf("saves que recibió el central, en orden: %q", guardados)
	t.Logf("central=%q · local=%q · outbox pending=%d sent=%d dead=%d · bajada de hoy: filas=%d rebotes=%d choques=%d",
		enElCentral, obs[0].Content, pending, sent, dead, r.BajadaHoy.Filas, r.BajadaHoy.Rebotes, r.BajadaHoy.Choques)
}
