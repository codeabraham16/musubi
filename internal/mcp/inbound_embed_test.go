package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// centralConDosFilas es un central de mentira: la primera página trae c1 y c2, las siguientes vienen
// vacías. Es el mismo protocolo que sirve musubi_sync_pull, sin red real (NordVPN bloquea por ruta
// a los binarios de prueba recién compilados).
func centralConDosFilas(t *testing.T) *httptest.Server {
	t.Helper()
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Params struct {
				Arguments struct {
					AfterRowID int64 `json:"after_rowid"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		payload := `{"items":[],"next_cursor":` + strconv.FormatInt(req.Params.Arguments.AfterRowID, 10) + `}`
		if req.Params.Arguments.AfterRowID == 0 {
			payload = `{"items":[` +
				`{"rowid":3,"id":"c1","topic_key":"t/a","content":"alpha del central","importance":1,"mem_type":"semantic","author":"ana","project_id":"acme"},` +
				`{"rowid":5,"id":"c2","topic_key":"t/b","content":"beta del central","importance":1,"mem_type":"semantic","author":"juan","project_id":"acme"}` +
				`],"next_cursor":5}`
		}
		resp := `{"jsonrpc":"2.0","id":"pull","result":{"content":[{"type":"text","text":` + strconv.Quote(payload) + `}]}}`
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(resp))
	}))
	t.Cleanup(stub.Close)
	return stub
}

// conVectorTrasElDrain corre UN drain entrante con el embebedor dado y devuelve qué ids quedaron
// con vector de SU procedencia. La sincronización con la goroutine de fondo es
// engine.EsperarRelleno(), y recién después Close: Close CORTA el relleno (es un apagado), así que
// cerrar sin esperar mediría el corte y no la vectorización. Nada de sondear con sleeps. Después se reabre la base y se
// pregunta como lo haría el recall semántico (misma procedencia, regla de homogeneidad).
func conVectorTrasElDrain(t *testing.T, emb embedding.Provider, consulta []float32) map[string]bool {
	t.Helper()
	stub := centralConDosFilas(t)
	dir := memtest.DirSembrado(t)
	engine, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	s := NewMcpServer(engine, t.TempDir(), emb, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, stub.URL), config.SyncConfig{BatchSize: 200})

	s.drainInboundOnce(context.Background())

	// El drain tiene que haber ingerido igual, embebedor sano o no: el vector es un extra.
	if raw, ok, _ := engine.GetMeta("sync:inbound_cursor"); !ok || raw != "5" {
		t.Errorf("cursor entrante = %q (ok=%v), esperaba \"5\": el drain no ingirió", raw, ok)
	}
	engine.EsperarRelleno()
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}

	e2, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer e2.Close()
	e2.SetVectorModelID(emb.Name())
	res, err := e2.SearchObservations(context.Background(), consulta, 10)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, r := range res {
		got[r.ID] = true
	}
	return got
}

// Lo que baja del central queda con vector SIN esperar al próximo arranque del daemon.
//
// El hueco medido en davantis-1: IngestShared guarda sin vector y el relleno automático corría una
// sola vez, al arrancar; entre arranques lo bajado se acumulaba invisible para la búsqueda
// semántica (0 → 4 → 21). Esta prueba baja dos filas de un central falso y exige que las dos
// aparezcan en una búsqueda semántica de la procedencia del embebedor.
//
// Sabotaje que la hace fallar: sacar el disparo de vectorizarLoBajado del drain → c1 y c2 quedan
// sin vector y la búsqueda no las encuentra.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="\t\t\ts.vectorizarLoBajado()\n"
// arnes: a=""
func TestDrainInboundVectorizaLoQueBaja(t *testing.T) {
	vec := []float32{0.1, 0.2, 0.3}
	got := conVectorTrasElDrain(t, fakeEmbedder{vec: vec}, vec)
	for _, id := range []string{"c1", "c2"} {
		if !got[id] {
			t.Errorf("%s bajó del central pero no tiene vector de la procedencia \"fake\": sigue invisible para la búsqueda semántica (encontradas: %v)", id, got)
		}
	}
}

// embebedorContado cuenta las llamadas y delega. Existe para que las pruebas de degradación no
// pasen en vacío: «no hubo vectores y no se cayó nada» también es lo que se ve si el embebedor
// NUNCA se llamó, y entonces la prueba no probó la degradación sino la ausencia del arreglo.
type embebedorContado struct {
	embedding.Provider
	llamadas *atomic.Int32
}

func (c embebedorContado) Embed(ctx context.Context, s string) ([]float32, error) {
	c.llamadas.Add(1)
	return c.Provider.Embed(ctx, s)
}

// Un embebedor caído DEGRADA: el drain ingiere y avanza el cursor igual (lo asierta
// conVectorTrasElDrain), el proceso no se cae, y las filas quedan sin vector — pendientes, no
// perdidas: las retoma el próximo pull que ingiera al menos una fila, o el próximo arranque.
func TestDrainInboundConEmbebedorCaidoIngiereIgual(t *testing.T) {
	var n atomic.Int32
	got := conVectorTrasElDrain(t, embebedorContado{failingEmbedder{}, &n}, make([]float32, 8))
	if n.Load() == 0 {
		t.Fatal("el embebedor caído no se llamó nunca: esta prueba no ejerció la degradación")
	}
	if len(got) != 0 {
		t.Errorf("con el embebedor caído no debería haber vectores, hay %v", got)
	}
}

// panicEmbedder entra en pánico al embeber: un proveedor con un bug. Cuenta como Enabled.
type panicEmbedder struct{}

func (panicEmbedder) Embed(context.Context, string) ([]float32, error) {
	panic("embebedor roto (test)")
}
func (panicEmbedder) Dimensions() int { return 3 }
func (panicEmbedder) Name() string    { return "panic-test" }

// Un panic del embebedor en la goroutine de fondo mataría el PROCESO entero, no sólo el drain. Si
// esta prueba termina, el panic se contuvo; el contador descarta que termine porque nadie embebió.
//
// Es la prueba de punta a punta y no lleva directiva a propósito: sin el recover, el panic tumba
// el binario de prueba entero, no queda un solo `--- FAIL` que leer y el juez de sabotajes lo
// cuenta como «no se midió nada». El recover se mide en
// memory.TestLaPasadaDeRellenoContieneElPanicoDelEmbebedor, que corre la pasada en la goroutine de
// la prueba y ataja el panic con nombre.
func TestDrainInboundConEmbebedorEnPanicoNoTumbaNada(t *testing.T) {
	var n atomic.Int32
	got := conVectorTrasElDrain(t, embebedorContado{panicEmbedder{}, &n}, []float32{1, 0, 0})
	if n.Load() == 0 {
		t.Fatal("el embebedor en pánico no se llamó nunca: esta prueba no ejerció el recover")
	}
	if len(got) != 0 {
		t.Errorf("con el embebedor en pánico no debería haber vectores, hay %v", got)
	}
}

// engineQueFallaEn es un engine real al que IngestShared le falla para UNA id: el fallo de ingest a
// mitad de lote (un SQLITE_BUSY, una fila veneno) sin tener que provocarlo en SQLite.
type engineQueFallaEn struct {
	*memory.DbEngine
	id string
}

func (e engineQueFallaEn) IngestShared(o memory.SharedObs) (bool, error) {
	if o.ID == e.id {
		return false, errors.New("ingest roto a propósito (test)")
	}
	return e.DbEngine.IngestShared(o)
}

// Si el lote falla A MITAD, lo que sí se ingirió antes del fallo se vectoriza igual: el disparo va
// en un defer y cuenta las filas ingeridas, no los lotes completos.
//
// c1 entra, c2 falla: el cursor queda en c1 (el ingest no avanza más allá de lo contiguo) y c1
// tiene que salir del drain con vector, no esperar al próximo pull que baje algo.
//
// Sabotaje que la hace fallar: olvidar lo ingerido cuando el lote falla → el defer ve cero filas,
// no dispara la vectorización y c1 queda sin vector.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="\t\t\t\tfailed = true\n\t\t\t\tbreak\n"
// arnes: a="\t\t\t\tfailed = true\n\t\t\t\tingeridas = 0\n\t\t\t\tbreak\n"
func TestDrainInboundVectorizaLoIngeridoAunqueElLoteFalleAMitad(t *testing.T) {
	stub := centralConDosFilas(t)
	dir := memtest.DirSembrado(t)
	engine, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	vec := []float32{0.1, 0.2, 0.3}
	emb := fakeEmbedder{vec: vec}
	s := NewMcpServer(engineQueFallaEn{engine, "c2"}, t.TempDir(), emb, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, stub.URL), config.SyncConfig{BatchSize: 200})

	s.drainInboundOnce(context.Background())

	if raw, ok, _ := engine.GetMeta("sync:inbound_cursor"); !ok || raw != "3" {
		t.Fatalf("cursor entrante = %q (ok=%v), esperaba \"3\": el lote no falló a mitad como la prueba necesita", raw, ok)
	}
	engine.EsperarRelleno()
	if err := engine.Close(); err != nil {
		t.Fatal(err)
	}

	e2, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer e2.Close()
	e2.SetVectorModelID(emb.Name())
	res, err := e2.SearchObservations(context.Background(), vec, 10)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range res {
		if r.ID == "c1" {
			return
		}
	}
	t.Errorf("c1 se ingirió antes del fallo de c2 pero quedó sin vector: el lote que falla a mitad no disparó la vectorización (resultados: %d)", len(res))
}
