package mcp

import (
	"context"
	"encoding/json"
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

// TestLaBajadaEntranteTraeElABDelCentralYLoMergea cierra el cable del A/B (R9 + decisión D4): un tick
// de bajada entrante que sale a la red —dueño del candado, team mode— NO sólo baja observaciones;
// también pide el snapshot del A/B del decididor (musubi_sync_pull_ab) y lo mergea en la base local.
//
// Es la ÚNICA prueba que ejerce la llamada drain-time bajarDecididorAB. Las otras (bajada_ritmo,
// inbound_candado_por_pagina) la TOLERAN respondiéndola vacía para que no perturbe sus cuentas, pero
// ninguna afirma que SALE: sin esta, borrar la llamada dejaba el denominador sin viajar —el lift
// inmedible en producción— y todo el verde seguía. El central devuelve una fila (acme, on,
// oportunidades=10, exigencias=4) y se comprueba que DecididorAB local la ve: así se prueba el pull Y
// el merge por el camino real del scheduler, no por una llamada directa a MergeDecididorABDesdeCentral
// (eso ya lo cubre decididor_ab_test).
//
// Sabotaje: borrar la llamada ⇒ 0 pulls del A/B y la fila no aparece local ⇒ rojo.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="\ts.bajarDecididorAB()\n"
// arnes: a=""
func TestLaBajadaEntranteTraeElABDelCentralYLoMergea(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	var abPulls atomic.Int64
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		if req.Params.Name == "musubi_sync_pull_ab" {
			abPulls.Add(1)
			fila := `{"rows":[{"project_id":"acme","arm":"on","oportunidades":10,"exigencias":4,"cargadas":0}]}`
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"pull-ab","result":{"content":[{"type":"text","text":` + strconv.Quote(fila) + `}]}}`))
			return
		}
		// Observaciones: una página vacía, para que el tick sea una sola bajada limpia que igual sale
		// a la red (salioALaRed) y llegue hasta el canal lateral del A/B.
		payload := `{"items":[],"next_cursor":0}`
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"pull","result":{"content":[{"type":"text","text":` + strconv.Quote(payload) + `}]}}`))
	}))
	defer stub.Close()

	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, stub.URL), config.SyncConfig{BatchSize: 50, DrainIntervalSeconds: 30})

	s.drainInboundOnce(context.Background())

	if n := abPulls.Load(); n != 1 {
		t.Fatalf("un tick que sale a la red tenía que pedir el A/B del decididor una vez, pidió %d", n)
	}
	// El merge tiene que haber poblado la base local: antes del cable, esta fila nacía sólo en el
	// central y nunca llegaba acá (el límite deferido de decididor_ab.go).
	rows, err := engine.DecididorAB(context.Background())
	if err != nil {
		t.Fatalf("DecididorAB local: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("el snapshot del central tenía que dejar 1 fila en la base local, quedaron %d: %+v", len(rows), rows)
	}
	if r := rows[0]; r.ProjectID != "acme" || r.Arm != memory.BrazoOn || r.Oportunidades != 10 || r.Exigencias != 4 {
		t.Errorf("la fila mergeada esperaba (acme, on, op=10, ex=4), quedó %+v", r)
	}
}

// TestLaBajadaEntranteToleraQueElCentralNoExpongaElAB cierra R8 (best-effort): un central VIEJO que no
// expone musubi_sync_pull_ab —o un fallo de red/RPC— NO debe cortar la bajada de observaciones ni hacer
// paniquear el tick. El stub contesta el pull del A/B con un error JSON-RPC (method not found); la
// bajada de observaciones igual tiene que correr (su página se sirve), el tick tiene que terminar sin
// pánico, y la base local queda SIN filas del A/B (el pull falló ⇒ no hubo merge).
//
// Es la contracara de TestLaBajadaEntranteTraeElABDelCentralYLoMergea: aquella prueba que el canal SALE;
// ésta, que cuando el otro lado no está, el canal lateral degrada con gracia y no se lleva puesta la
// bajada de obs. drainInboundOnce corre SÍNCRONO acá, así que un pánico en el canal del A/B crashearía
// esta prueba: ése es exactamente el sabotaje.
//
// Sabotaje: que el fallo del pull del A/B deje de ser best-effort (paniquear en vez de logear+return)
// ⇒ drainInboundOnce paniquea ⇒ este test crashea ⇒ rojo.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="logx.Error(\"inbound: no se pudo bajar el A/B del decididor del central (best-effort; no corta la bajada de obs)\", \"error\", err)\n\t\treturn"
// arnes: a="panic(\"sabotaje R8: el fallo del pull del A/B dejo de ser best-effort\")"
func TestLaBajadaEntranteToleraQueElCentralNoExpongaElAB(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	var abPulls, obsPulls atomic.Int64
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		if req.Params.Name == "musubi_sync_pull_ab" {
			abPulls.Add(1)
			// Un central viejo que no expone la tool: error JSON-RPC (method not found).
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"pull-ab","error":{"code":-32601,"message":"method not found"}}`))
			return
		}
		obsPulls.Add(1)
		payload := `{"items":[],"next_cursor":0}`
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"pull","result":{"content":[{"type":"text","text":` + strconv.Quote(payload) + `}]}}`))
	}))
	defer stub.Close()

	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, stub.URL), config.SyncConfig{BatchSize: 50, DrainIntervalSeconds: 30})

	// Si el fallo del A/B NO fuera best-effort (propagara o paniqueara), este llamado crashearía el test.
	s.drainInboundOnce(context.Background())

	if n := abPulls.Load(); n != 1 {
		t.Fatalf("el tick tenía que INTENTAR el pull del A/B una vez, intentó %d", n)
	}
	if n := obsPulls.Load(); n < 1 {
		t.Fatalf("el fallo del A/B no debía impedir la bajada de observaciones; obsPulls=%d", n)
	}
	// El pull falló ⇒ no hubo merge ⇒ la base local queda sin filas del A/B.
	rows, err := engine.DecididorAB(context.Background())
	if err != nil {
		t.Fatalf("DecididorAB local: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("un pull fallido no debía poblar la base local, quedaron %d filas: %+v", len(rows), rows)
	}
}

// TestElPullFallidoDelABNoTocaLaContabilidadDeLaBajada cierra la cláusula de R8 que el test de
// tolerancia deja suelta: el fallo del pull del A/B NO DEBE PERTURBAR LA CONTABILIDAD de la bajada de
// observaciones (sync_viajes / ritmo). TestLaBajadaEntranteToleraQueElCentralNoExpongaElAB prueba que
// el fallo no CORTA la bajada (sigue corriendo, no paniquea), pero no mira sync_viajes: un fallo que
// anotara un viaje fantasma en su rama de error movería el ritmo —calibrado con filas de observación—
// y quedaba verde. Acá el pull del A/B falla igual que allá, y se afirma que ese tick anotó UNA sola
// página de bajada sin filas (la del obs vacío), sin el viaje de más.
//
// Se mide por DELTA alrededor del tick (ResumenDelSync antes/después), así el assert no depende de si
// la base sembrada ya tenía viajes: el tick legítimo suma +1 página y +0 filas; un viaje fantasma en la
// rama de error sumaría otra página (o filas) y rompería el delta.
//
// Sabotaje: anotar un viaje fantasma en la rama de error del pull del A/B ⇒ sync_viajes se mueve ⇒ rojo.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="\t\tlogx.Error(\"inbound: no se pudo bajar el A/B del decididor del central (best-effort; no corta la bajada de obs)\", \"error\", err)\n\t\treturn"
// arnes: a="\t\tlogx.Error(\"inbound: no se pudo bajar el A/B del decididor del central (best-effort; no corta la bajada de obs)\", \"error\", err)\n\t\ts.anotarUltimaBajada(memory.Viaje{Posts: 1, Filas: 7})\n\t\treturn"
func TestElPullFallidoDelABNoTocaLaContabilidadDeLaBajada(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Params struct {
				Name string `json:"name"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		if req.Params.Name == "musubi_sync_pull_ab" {
			// El pull del A/B falla (central viejo): su rama de error es la que no debe tocar la contabilidad.
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"pull-ab","error":{"code":-32601,"message":"method not found"}}`))
			return
		}
		// Una sola página de obs vacía: la bajada la anota como 1 página, 0 filas.
		payload := `{"items":[],"next_cursor":0}`
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"pull","result":{"content":[{"type":"text","text":` + strconv.Quote(payload) + `}]}}`))
	}))
	defer stub.Close()

	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, stub.URL), config.SyncConfig{BatchSize: 50, DrainIntervalSeconds: 30})

	antes, err := engine.ResumenDelSync(context.Background())
	if err != nil {
		t.Fatalf("ResumenDelSync (antes): %v", err)
	}

	s.drainInboundOnce(context.Background())

	despues, err := engine.ResumenDelSync(context.Background())
	if err != nil {
		t.Fatalf("ResumenDelSync (después): %v", err)
	}

	// El tick legítimo es UNA página de obs vacía: +1 post, +0 filas. El fallo del A/B no puede haber
	// sumado su propio viaje (lo que movería el ritmo, calibrado con filas de observación).
	dPosts := despues.BajadaHoy.Posts - antes.BajadaHoy.Posts
	dFilas := despues.BajadaHoy.Filas - antes.BajadaHoy.Filas
	if dPosts != 1 || dFilas != 0 {
		t.Fatalf("el fallo del A/B no debía tocar la contabilidad de la bajada; esperaba Δ{Posts:1, Filas:0}, obtuve Δ{Posts:%d, Filas:%d}", dPosts, dFilas)
	}
}

// TestElPullDelABVaSinCursorNiPaginacion fija R2: el A/B se baja en UNA sola llamada, SIN cursor ni
// paginación (la tabla tiene a lo sumo dos filas por proyecto). A diferencia de musubi_sync_pull —que
// manda after_rowid/limit y pagina—, PullDecididorAB tiene que mandar argumentos VACÍOS. Ningún otro
// test mira el request del cliente: sin esto, agregarle un cursor para "paginar" la tabla pasaría toda
// la suite en verde y rompería en silencio el contrato de un solo viaje.
//
// El stub captura params.arguments del tools/call y cuenta sus claves; para musubi_sync_pull_ab tiene
// que haber CERO. Se llama PullDecididorAB directo (no por el scheduler) para observar el request crudo.
//
// Sabotaje: mandar un cursor en PullDecididorAB (after_rowid/limit) ⇒ params.arguments con 2 claves ⇒ rojo.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="c.callCentral(\"pull-ab\", \"musubi_sync_pull_ab\", map[string]any{})"
// arnes: a="c.callCentral(\"pull-ab\", \"musubi_sync_pull_ab\", map[string]any{\"after_rowid\": int64(5), \"limit\": 1})"
func TestElPullDelABVaSinCursorNiPaginacion(t *testing.T) {
	var llamadas, claves atomic.Int64
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Params struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			} `json:"params"`
		}
		_ = json.Unmarshal(body, &req)
		if req.Params.Name == "musubi_sync_pull_ab" {
			llamadas.Add(1)
			var args map[string]any
			_ = json.Unmarshal(req.Params.Arguments, &args)
			claves.Store(int64(len(args)))
		}
		fila := `{"rows":[]}`
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"pull-ab","result":{"content":[{"type":"text","text":` + strconv.Quote(fila) + `}]}}`))
	}))
	defer stub.Close()

	if _, err := newTestSyncClient(t, stub.URL).PullDecididorAB(); err != nil {
		t.Fatalf("PullDecididorAB: %v", err)
	}

	if n := llamadas.Load(); n != 1 {
		t.Fatalf("PullDecididorAB tenía que pegarle a musubi_sync_pull_ab una vez, fueron %d", n)
	}
	if n := claves.Load(); n != 0 {
		t.Fatalf("el pull del A/B va SIN cursor ni paginación (R2): params.arguments tenía que ir vacío, trajo %d clave(s)", n)
	}
}
