package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/mcp"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// TestLaNotaBajadaConservaSuEdad: el camino entero de una nota vieja. Un central de verdad (el
// dispatcher real detrás de un httptest) tiene una nota creada hace 90 días; el cliente de sync real
// la baja, IngestShared la guarda, y acá conserva los 90 días en vez de nacer con la fecha de la
// bajada. Después el hook del turno corre con la base local y la viñeta de esa nota queda en el log
// de la prueba, literal.
//
// Esta prueba fija DOS cosas: (1) la fecha GUARDADA —que la nota bajada conserve sus 90 días en vez
// de nacer con la fecha de la bajada— y (2) que esos 90 días lleguen hasta la viñeta del hook como
// « · hace 3m». La (2) une al productor (soloFecha, internal/memory, que recorta created_at a
// «2006-01-02») con el parser (gistAge, cmd/musubi) por el camino real central→sync→base→hook.
// Antes gistAge leía sólo RFC3339 y la base guarda «2006-01-02 15:04:05», así que el sufijo salía
// vacío en TODAS las notas (0 de 7.177 viñetas en siete transcripts reales); desde que gistAge lee
// los layouts de la base y el día a secas, la viñeta vuelve a decir la edad.
//
// Sabotaje: la fecha no sale en el JSON del central.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="json:\"created_at,omitempty\""
// arnes: a="json:\"-\""
// arnes: colision_ok="TestElPullDelCentralLlevaLaFecha"
func TestLaNotaBajadaConservaSuEdad(t *testing.T) {
	const layout = "2006-01-02 15:04:05"
	const id = "kiosko-f18"
	origen := time.Now().UTC().Add(-90 * 24 * time.Hour).Format(layout)

	central, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { central.Close() })
	central.SetProjectID("")
	if err := central.SaveObservationTypedFrom("", "gio", id, "altura/kiosko",
		"el kiosko del fichaje F18 queda en blanco cuando la raspberry pierde el WiFi", 1, "semantic", "shared", nil); err != nil {
		t.Fatal(err)
	}
	if err := central.SetObservationCreatedAt(id, origen); err != nil {
		t.Fatal(err)
	}
	servidor := mcp.NewMcpServer(central, t.TempDir(), embedding.NoopProvider{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req mcp.JsonRpcRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp, _ := servidor.Dispatch(r.Context(), req)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	cli, err := mcp.NewSyncClient(config.SyncConfig{CentralURL: srv.URL, AllowInsecureToken: true, RequestTimeoutSeconds: 5})
	if err != nil {
		t.Fatal(err)
	}
	items, _, _, err := cli.Pull(0, 50)
	if err != nil || len(items) != 1 {
		t.Fatalf("pull: %v (%d filas)", err, len(items))
	}
	local, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { local.Close() })
	if _, err := local.IngestShared(items[0]); err != nil {
		t.Fatal(err)
	}

	obs, err := local.GetObservations([]string{id})
	if err != nil || len(obs) != 1 {
		t.Fatalf("leer la nota bajada: %v (%d filas)", err, len(obs))
	}
	if obs[0].CreatedAt != origen {
		t.Errorf("la nota bajada quedó con created_at %q; nació en el central el %q (hace 90 días)", obs[0].CreatedAt, origen)
	}

	_, salida := hookAdditionalContext(t, turnoReal(t, local, "s-edad", "¿por qué el kiosko del fichaje F18 queda en blanco?"))
	var vineta string
	for _, l := range strings.Split(salida, "\n") {
		if strings.Contains(l, "[id:"+id+"]") {
			vineta = l
		}
	}
	if vineta == "" {
		t.Fatalf("el hook del turno no trajo la nota bajada; la prueba no mide la viñeta:\n%s", salida)
	}
	// gistAge YA lee el layout de la base y la fecha a secas que empaqueta soloFecha: el Logf de
	// antes es ahora ESTA aserción. La nota nació hace 90 días → la viñeta la cierra « · hace 3m»
	// entre el gist y el [id:…]. Es el ÚNICO test que une al productor (soloFecha, internal/memory)
	// con el parser (gistAge, acá) por el camino real; si cualquiera cambia de forma, el sufijo
	// vuelve a salir vacío y la prueba cae.
	if !strings.Contains(vineta, " · hace 3m") {
		t.Errorf("la viñeta de una nota de hace 90 días debería cerrar con « · hace 3m»; salió: %q", vineta)
	}
}
