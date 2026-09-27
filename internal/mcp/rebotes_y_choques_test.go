package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"musubi/internal/memory"
)

// centralQueDevuelve contesta musubi_sync_pull con estos items en CADA pedido, sin mirar el cursor.
func centralQueDevuelve(items ...memory.SharedObs) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, _ := json.Marshal(map[string]interface{}{"items": items, "next_cursor": int64(len(items))})
		resp, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": "pull",
			"result": map[string]interface{}{"content": []map[string]string{{"type": "text", "text": string(payload)}}}})
		_, _ = w.Write(resp)
	}))
}

// TestLaBajadaCuentaRebotesYChoques: lo que IngestShared conservó llega al viaje del tick y de ahí a
// sync_viajes, cada uno en su columna. Una página trae el rebote de una nota propia que se editó
// después de salir y una versión ajena de otra nota que acá esperaba para salir: rebotes=1,
// choques=1, y las dos ediciones locales intactas.
//
// Sabotaje: no sumar el rebote al viaje.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="\t\t\t\tviaje.Rebotes++\n"
// arnes: a="\t\t\t\tviaje.Rebotes += 0\n"
//
// Sabotaje: no sumar el choque al viaje.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="\t\t\t\tviaje.Choques++\n"
// arnes: a="\t\t\t\tviaje.Choques += 0\n"
func TestLaBajadaCuentaRebotesYChoques(t *testing.T) {
	const salio, editada, deAca = "lo que salio de aca", "lo que se edito despues de salir", "escrita aca, esperando salir"
	guardar := func(s *McpServer, id, contenido string) {
		t.Helper()
		if err := s.engine.SaveObservationTyped(id, "t/x", contenido, 1, "semantic", memory.ScopeShared, nil); err != nil {
			t.Fatal(err)
		}
	}
	// El central devuelve lo que se le subió de «propia», y otra versión de «ajena».
	central := centralQueDevuelve(
		memory.SharedObs{RowID: 1, ID: "propia", TopicKey: "t/x", Content: salio, Importance: 1, MemType: "semantic", Author: "davantis-mando-admin", ProjectID: "acme"},
		memory.SharedObs{RowID: 2, ID: "ajena", TopicKey: "t/x", Content: "la version de gio", Importance: 1, MemType: "semantic", Author: "gio", ProjectID: "acme"},
	)
	defer central.Close()
	s, eng := serverQueAnotaViajes(t, central.URL, true)

	guardar(s, "propia", salio)
	reclamadas, err := s.engine.ClaimOutboxBatch(50, 60)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.engine.MarkOutboxSent("propia", hashReclamado(t, reclamadas, "propia")); err != nil {
		t.Fatal(err)
	}
	guardar(s, "propia", editada)
	guardar(s, "ajena", deAca)

	s.drainInboundOnce(context.Background())

	viajes := eng.anotados(memory.ViajeBajada)
	if len(viajes) != 1 || viajes[0].Rebotes != 1 || viajes[0].Choques != 1 || viajes[0].Filas != 2 {
		t.Errorf("el viaje de la bajada fue %+v; esperaba uno con filas=2 rebotes=1 choques=1", viajes)
	}
	r, err := s.engine.ResumenDelSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if r.BajadaHoy.Rebotes != 1 || r.BajadaHoy.Choques != 1 {
		t.Errorf("sync_viajes de hoy: rebotes=%d choques=%d; esperaba 1 y 1", r.BajadaHoy.Rebotes, r.BajadaHoy.Choques)
	}
	obs, _, err := s.engine.GetObservationsBudget([]string{"propia", "ajena"}, 0)
	if err != nil || len(obs) != 2 {
		t.Fatalf("leer las dos notas locales: %v (%d filas)", err, len(obs))
	}
	for _, o := range obs {
		if want := map[string]string{"propia": editada, "ajena": deAca}[o.ID]; o.Content != want {
			t.Errorf("EDICIÓN PERDIDA: %s quedó %q; esperaba %q", o.ID, o.Content, want)
		}
	}
}
