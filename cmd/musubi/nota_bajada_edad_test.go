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
// Lo que NO fija, y por qué: que la viñeta diga «· hace 3m». Hoy la viñeta no dice NINGUNA edad,
// ni la verdadera ni «· hoy»: gistAge sólo lee RFC3339 y la base guarda «2006-01-02 15:04:05», así
// que el sufijo sale vacío en todas las notas (0 de 7.177 viñetas en siete transcripts reales). Eso
// es un defecto aparte, del formateador, y queda anotado para su propio cambio. Lo que esta prueba
// sí fija es la edad que la viñeta va a decir cuando gistAge lea el formato de la base: la calcula
// con el mismo gistAge, sobre la fecha guardada.
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
	guardada, err := time.Parse(layout, obs[0].CreatedAt)
	if err != nil {
		t.Fatalf("la fecha guardada no tiene el formato de la base: %q", obs[0].CreatedAt)
	}
	if edad := gistAge(guardada.Format(time.RFC3339)); edad != " · hace 3m" {
		t.Errorf("con la fecha guardada, la viñeta diría %q; una nota de hace 90 días dice « · hace 3m»", edad)
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
	t.Logf("viñeta literal del hook: %s", vineta)
}
