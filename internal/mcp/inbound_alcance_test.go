package mcp

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// EL CURSOR DE BAJADA SE REINICIA CUANDO CAMBIA EL RECORTE DEL CENTRAL.
//
// El filtro de proyecto del central no oculta filas: las SALTA. Comparte el WHERE con `sync_seq > ?`,
// el LIMIT se aplica después, y el `next_cursor` sale de las filas ya filtradas — así que lo
// descartado queda DEBAJO del cursor sin haberse entregado. Mientras la credencial no cambia eso no
// molesta; cuando se ENSANCHA, el filtro desaparece pero el cursor ya está arriba y esa historia no
// volvía nunca (medido el 2026-09-24: 64 filas inalcanzables en davantis-1, 980 en la laptop).
//
// El arreglo es que el central DECLARE su recorte en cada pull y el cliente reinicie el cursor cuando
// cambia. Estos tests cubren las dos mitades: la decisión (pura) y la reparación (de punta a punta).

func TestAlcanceCambio(t *testing.T) {
	casos := []struct {
		nombre, nuevo, visto string
		quiero               bool
		porque               string
	}{
		{
			"central viejo que no manda el campo", "", "proyecto=musubi", false,
			"tratar «no sé» como «cambió» reiniciaría el corpus entero en CADA tick",
		},
		{
			"central viejo y base sin alcance", "", "", false,
			"sin dato de ninguno de los dos lados no hay nada que decidir",
		},
		{
			"base que venía sincronizando sin la clave", "federado", "", true,
			"su cursor lo avanzó un recorte desconocido: puede tener huecos, hay que reparar",
		},
		{
			"la credencial se ensanchó", "federado", "proyecto=musubi", true,
			"el filtro desapareció y lo que había salteado quedó debajo del cursor",
		},
		{
			"la credencial se acotó", "proyecto=musubi", "federado", true,
			"el recorte es otro, y el cursor dejó de significar lo mismo",
		},
		{
			"el mismo recorte", "proyecto=musubi", "proyecto=musubi", false,
			"nada cambió: reiniciar acá sería re-bajar el corpus por gusto",
		},
	}
	for _, c := range casos {
		if got := alcanceCambio(c.nuevo, c.visto); got != c.quiero {
			t.Errorf("%s: alcanceCambio(%q, %q) = %v, esperaba %v — %s",
				c.nombre, c.nuevo, c.visto, got, c.quiero, c.porque)
		}
	}
}

// TestReiniciarBajadaEscribeLasDosONinguna cubre la atomicidad, que no es decorativa: con el cursor en
// cero y el alcance sin registrar, el próximo tick volvería a detectar un cambio y reiniciaría otra
// vez — un bucle que re-baja el corpus completo en cada tick.
func TestReiniciarBajadaEscribeLasDosONinguna(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	if err := engine.AvanzarCursorBajada(metaInboundCursor, 10990); err != nil {
		t.Fatal(err)
	}
	if err := engine.ReiniciarBajadaPorAlcance(metaInboundCursor, metaInboundAlcance, "federado"); err != nil {
		t.Fatalf("ReiniciarBajadaPorAlcance: %v", err)
	}

	if raw, ok, _ := engine.GetMeta(metaInboundCursor); !ok || raw != "0" {
		t.Errorf("cursor = %q (ok=%v), esperaba \"0\"", raw, ok)
	}
	if raw, ok, _ := engine.GetMeta(metaInboundAlcance); !ok || raw != "federado" {
		t.Errorf("alcance = %q (ok=%v), esperaba \"federado\"", raw, ok)
	}

	// Y la contraprueba de que NO se rompió la monotonía que protege al cursor de los ticks lentos:
	// avanzar sigue siendo la única vía que sube, y sigue negándose a bajar.
	if err := engine.AvanzarCursorBajada(metaInboundCursor, 500); err != nil {
		t.Fatal(err)
	}
	if err := engine.AvanzarCursorBajada(metaInboundCursor, 400); err != nil {
		t.Fatal(err)
	}
	if raw, _, _ := engine.GetMeta(metaInboundCursor); raw != "500" {
		t.Errorf("AvanzarCursorBajada retrocedió a %q: la monotonía se rompió", raw)
	}
}

// centralQueCambiaDeAlcance devuelve un central de prueba que sirve DOS filas con `sync_seq`
// entrelazado —la ajena (5) ANTES de la propia (10)— y que empieza acotado al proyecto «propio».
// Mientras está acotado esconde la ajena pero devuelve la propia, que es exactamente cómo el cursor
// se sube por encima de una fila que nunca entregó. Al poner *federado en true empieza a servir las
// dos y a declararse federado.
func centralQueCambiaDeAlcance(t *testing.T, federado *bool) *httptest.Server {
	t.Helper()
	const ajena = `{"rowid":5,"id":"ajena","topic_key":"t/x","content":"esto es de otro proyecto","importance":1,"mem_type":"semantic","author":"otro","project_id":"ajeno"}`
	const propia = `{"rowid":10,"id":"propia","topic_key":"t/x","content":"esto es de mi proyecto","importance":1,"mem_type":"semantic","author":"yo","project_id":"propio"}`

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Params struct {
				Arguments struct {
					AfterRowID int64 `json:"after_rowid"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		desde := req.Params.Arguments.AfterRowID

		alcance := "proyecto=propio"
		var filas []string
		if *federado {
			alcance = "federado"
			if desde < 5 {
				filas = append(filas, ajena)
			}
			if desde < 10 {
				filas = append(filas, propia)
			}
		} else if desde < 10 {
			filas = append(filas, propia) // la ajena NO se entrega: el filtro la SALTA
		}

		next := desde
		if len(filas) > 0 {
			next = 10
		}
		payload := `{"items":[` + join(filas) + `],"next_cursor":` + strconv.FormatInt(next, 10) +
			`,"alcance":` + strconv.Quote(alcance) + `}`
		resp := `{"jsonrpc":"2.0","id":"pull","result":{"content":[{"type":"text","text":` + strconv.Quote(payload) + `}]}}`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(resp))
	}))
}

func join(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += ","
		}
		out += x
	}
	return out
}

// TestEnsancharElAlcanceRecuperaLoSalteado es la prueba de la reparación, y es el test que faltaba en
// todo este track: con el cursor arriba por un pull acotado, ensanchar la credencial tiene que traer
// la fila que el filtro había salteado.
func TestEnsancharElAlcanceRecuperaLoSalteado(t *testing.T) {
	federado := false
	stub := centralQueCambiaDeAlcance(t, &federado)
	defer stub.Close()

	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, stub.URL), config.SyncConfig{BatchSize: 200})

	// FASE 1 — acotado. Baja la propia y el cursor se sube a 10, por encima de la ajena (5).
	s.drainInboundOnce(context.Background())

	if !existeObs(t, engine, "propia") {
		t.Fatal("fase 1: la fila del propio proyecto tenía que bajar")
	}
	if existeObs(t, engine, "ajena") {
		t.Fatal("fase 1: la fila ajena NO debía bajar con el alcance acotado")
	}
	if raw, _, _ := engine.GetMeta(metaInboundCursor); raw != "10" {
		t.Fatalf("fase 1: cursor = %q, esperaba \"10\" (por encima de la ajena, que es 5)", raw)
	}
	if raw, _, _ := engine.GetMeta(metaInboundAlcance); raw != "proyecto=propio" {
		t.Fatalf("fase 1: alcance registrado = %q, esperaba \"proyecto=propio\"", raw)
	}

	// FASE 2 — la credencial se ensancha. Sin el reinicio, la ajena (5) quedaría debajo del cursor
	// (10) para siempre: es el defecto que este arreglo cierra.
	federado = true
	s.drainInboundOnce(context.Background())

	if !existeObs(t, engine, "ajena") {
		raw, _, _ := engine.GetMeta(metaInboundCursor)
		t.Errorf("la fila salteada NO se recuperó al ensanchar el alcance (cursor=%q): el reinicio no ocurrió", raw)
	}
	if raw, _, _ := engine.GetMeta(metaInboundAlcance); raw != "federado" {
		t.Errorf("alcance registrado = %q, esperaba \"federado\"", raw)
	}
	if raw, _, _ := engine.GetMeta(metaInboundCursor); raw != "10" {
		t.Errorf("cursor = %q tras rebobinar y volver a bajar, esperaba \"10\"", raw)
	}

	// FASE 3 — el alcance ya no cambia, así que un tick más NO vuelve a rebobinar. Sin esta mitad, el
	// arreglo sería un bucle que re-baja el corpus entero en cada tick.
	s.drainInboundOnce(context.Background())
	if raw, _, _ := engine.GetMeta(metaInboundCursor); raw != "10" {
		t.Errorf("cursor = %q tras un tick sin cambio de alcance: rebobinó de más", raw)
	}
}

func existeObs(t *testing.T, e *memory.DbEngine, id string) bool {
	t.Helper()
	items, err := e.ListSharedForPull(context.Background(), 0, 200)
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range items {
		if o.ID == id {
			return true
		}
	}
	return false
}

// TestToolSyncPullDeclaraSuAlcance cubre la mitad del CENTRAL, y no es un test de más: el sabotaje
// destapó que nadie la cuidaba. TestEnsancharElAlcanceRecuperaLoSalteado usa un central de prueba que
// inventa su propio `alcance`, así que borrar el campo del `toolSyncPull` REAL lo dejaba verde — una
// guarda definida y desconectada. Este test llama a la tool de verdad, por el mismo camino que un
// cliente remoto, y mira lo que contesta.
//
// ⚠️ LOS TRES CASOS TIENEN QUE DAR LO QUE DA scopeClause, no lo que dice la credencial. Un principal
// federado y uno con proyecto VACÍO filtran IGUAL (scope.go: `if sc.Federate || sc.ProjectID == ""`),
// así que los dos son "federado". Distinguirlos haría rebobinar el cursor —y re-bajar el corpus
// entero— cada vez que se alterne entre dos credenciales que recortan lo mismo.
func TestToolSyncPullDeclaraSuAlcance(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	alcanceCon := func(p *Principal) string {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"after_rowid": 0, "limit": 10})
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_sync_pull", Arguments: raw})
		ctx := context.Background()
		if p != nil {
			ctx = withPrincipal(ctx, p)
		}
		out, rpcErr := s.handleToolsCall(ctx, params)
		if rpcErr != nil {
			t.Fatalf("sync_pull: %+v", rpcErr)
		}
		var pl struct {
			Alcance string `json:"alcance"`
		}
		if err := json.Unmarshal([]byte(out.(CallToolResponse).Content[0].Text), &pl); err != nil {
			t.Fatalf("parse del payload del pull: %v", err)
		}
		return pl.Alcance
	}

	casos := []struct {
		nombre    string
		principal *Principal
		quiero    string
	}{
		{"writer acotado a un proyecto", &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"}, "proyecto=crm"},
		{"otro proyecto da OTRA huella", &Principal{Name: "bob", Role: RoleWriter, ProjectID: "altura"}, "proyecto=altura"},
		{"admin (read:all) federa", &Principal{Name: "root", Role: RoleAdmin}, "federado"},
		{"reader sin proyecto NO filtra, así que es el mismo alcance", &Principal{Name: "ana", Role: RoleReader}, "federado"},
		{"stdio local sin principal", nil, "federado"},
	}
	for _, c := range casos {
		if got := alcanceCon(c.principal); got != c.quiero {
			t.Errorf("%s: alcance = %q, esperaba %q", c.nombre, got, c.quiero)
		}
	}
}
