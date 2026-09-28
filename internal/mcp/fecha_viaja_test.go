package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// La fecha de creación viaja en la respuesta de musubi_sync_pull, y los dos lados conviven con la
// otra versión: un central viejo no la manda y un cliente viejo no la lee.

// TestElPullDelCentralLlevaLaFecha: el JSON que sirve musubi_sync_pull trae `created_at` con la
// fecha tal como la guarda el central, y lo omite en una fila sin fecha (esa fila sale igual que
// de un central viejo). Y un cliente VIEJO, que decodifica con la SharedObs de antes, lee la misma
// página sin error.
//
// No lleva sabotaje la mitad del cliente viejo, y es a propósito: ese código ya está compilado en
// las máquinas y nada de esta rama lo puede cambiar. Lo que fija es que el central sólo AGREGA.
//
// Sabotaje: la fecha no sale en el JSON.
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="json:\"created_at,omitempty\""
// arnes: a="json:\"-\""
// arnes: colision_ok="TestLaNotaBajadaConservaSuEdad"
func TestElPullDelCentralLlevaLaFecha(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	for id, fecha := range map[string]string{"con-fecha": "2026-06-29 10:11:12", "sin-fecha": ""} {
		if err := engine.SaveObservationTypedFrom("acme", "ana", id, "t/fecha", "la nota "+id, 1, "semantic", "shared", nil); err != nil {
			t.Fatal(err)
		}
		if err := engine.SetObservationCreatedAt(id, fecha); err != nil {
			t.Fatal(err)
		}
	}

	args, _ := json.Marshal(map[string]any{"after_rowid": 0, "limit": 50})
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_sync_pull", Arguments: args})
	out, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), &Principal{Name: "ana", Role: RoleWriter, ProjectID: "acme"}), params)
	if rpcErr != nil {
		t.Fatalf("pull: %+v", rpcErr)
	}
	texto := out.(CallToolResponse).Content[0].Text

	var crudo struct {
		Items []map[string]json.RawMessage `json:"items"`
	}
	if err := json.Unmarshal([]byte(texto), &crudo); err != nil {
		t.Fatalf("la respuesta no es JSON: %v", err)
	}
	fechas := map[string]string{}
	for _, it := range crudo.Items {
		var id string
		_ = json.Unmarshal(it["id"], &id)
		fechas[id] = string(it["created_at"])
	}
	if fechas["con-fecha"] != `"2026-06-29 10:11:12"` {
		t.Errorf("created_at de con-fecha salió como %s; esperaba \"2026-06-29 10:11:12\". Respuesta: %s", fechas["con-fecha"], texto)
	}
	if f, esta := fechas["sin-fecha"]; !esta || f != "" {
		t.Errorf("sin-fecha: esperaba la fila SIN la clave created_at, salió %q (presente=%v)", f, esta)
	}

	// El cliente viejo: SharedObs sin CreatedAt, decodificada como la decodifica Pull.
	type sharedObsViejo struct {
		RowID      int64   `json:"rowid"`
		ID         string  `json:"id"`
		TopicKey   string  `json:"topic_key"`
		Content    string  `json:"content"`
		Importance float64 `json:"importance"`
		MemType    string  `json:"mem_type"`
		Author     string  `json:"author"`
		ProjectID  string  `json:"project_id"`
	}
	var viejo struct {
		Items      []sharedObsViejo `json:"items"`
		NextCursor int64            `json:"next_cursor"`
		Alcance    string           `json:"alcance"`
	}
	if err := json.Unmarshal([]byte(texto), &viejo); err != nil {
		t.Fatalf("un cliente viejo no puede leer la página del central nuevo: %v", err)
	}
	if len(viejo.Items) != 2 || viejo.NextCursor == 0 {
		t.Fatalf("el cliente viejo leyó %+v", viejo)
	}
	for _, it := range viejo.Items {
		if it.Content != "la nota "+it.ID || it.Author != "ana" || it.ProjectID != "acme" || it.TopicKey != "t/fecha" {
			t.Errorf("el cliente viejo leyó mal una fila: %+v", it)
		}
	}
}

// TestUnClienteNuevoConUnCentralViejo: el drain real contra un central que no manda la fecha (el
// de hoy, hasta que se despliegue) guarda la de la bajada, como siempre; contra uno que la manda,
// guarda la de origen.
//
// Sabotaje: pasar la fecha sin el default (con un central viejo la fila quedaba con created_at NULL).
// arnes: archivo="internal/memory/inboundsync.go"
// arnes: de="COALESCE(?, CURRENT_TIMESTAMP)"
// arnes: a="?"
// arnes: colision_ok="TestUnaFechaQueNoSirveCaeAlDefault"
func TestUnClienteNuevoConUnCentralViejo(t *testing.T) {
	fechaDe := func(t *testing.T, eng *engineQueAnotaViajes, id string) string {
		t.Helper()
		obs, err := eng.StorageBackend.(*memory.DbEngine).GetObservations([]string{id})
		if err != nil || len(obs) != 1 {
			t.Fatalf("leer %s: %v (%d filas)", id, err, len(obs))
		}
		return obs[0].CreatedAt
	}

	viejo := centralQueDevuelve(memory.SharedObs{RowID: 1, ID: "del-viejo", TopicKey: "t/fecha", Content: "de un central que no manda la fecha", Importance: 1, MemType: "semantic", Author: "gio", ProjectID: "acme"})
	defer viejo.Close()
	s, eng := serverQueAnotaViajes(t, viejo.URL, true)
	antes := time.Now().UTC().Truncate(time.Second)
	s.drainInboundOnce(context.Background())
	despues := time.Now().UTC().Add(time.Second)
	got := fechaDe(t, eng, "del-viejo")
	if f, err := time.Parse("2006-01-02 15:04:05", got); err != nil || f.Before(antes) || f.After(despues) {
		t.Errorf("con un central viejo la fila quedó con created_at %q; esperaba la fecha de la bajada (%s…%s)", got, antes, despues)
	}

	nuevo := centralQueDevuelve(memory.SharedObs{RowID: 1, ID: "del-nuevo", TopicKey: "t/fecha", Content: "de un central que manda la fecha", Importance: 1, MemType: "semantic", Author: "gio", ProjectID: "acme", CreatedAt: "2026-06-29 10:11:12"})
	defer nuevo.Close()
	s2, eng2 := serverQueAnotaViajes(t, nuevo.URL, true)
	s2.drainInboundOnce(context.Background())
	if got := fechaDe(t, eng2, "del-nuevo"); got != "2026-06-29 10:11:12" {
		t.Errorf("con un central nuevo la fila quedó con created_at %q; esperaba la de origen, 2026-06-29 10:11:12", got)
	}
}
