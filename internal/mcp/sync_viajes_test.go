package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// LOS DRAINS REGISTRAN EL VIAJE DEL TICK, Y SÓLO CUANDO EL TICK SALIÓ A LA RED.
//
// sync_viajes es el instrumento de la ola 2: bytes, posts y filas por día y sentido. Tiene dos
// maneras de mentir y las dos se prueban acá: no registrar lo que viajó (el contador queda en cero
// con el sync andando) y registrar lo que no viajó (cada tick vacío o fallido escribe una fila, que
// además es una escritura por tick sobre una base compartida por varios procesos).

// viajeAnotado es una llamada a RegistrarViaje vista por el engine.
type viajeAnotado struct {
	sentido string
	v       memory.Viaje
}

// engineQueAnotaViajes envuelve un engine real y anota cada RegistrarViaje antes de delegarlo. Así
// la prueba distingue «no se llamó» de «se llamó con ceros», que en la tabla se leen igual.
type engineQueAnotaViajes struct {
	memory.StorageBackend
	mu     sync.Mutex
	viajes []viajeAnotado
}

func (e *engineQueAnotaViajes) RegistrarViaje(sentido string, v memory.Viaje) error {
	e.mu.Lock()
	e.viajes = append(e.viajes, viajeAnotado{sentido, v})
	e.mu.Unlock()
	return e.StorageBackend.RegistrarViaje(sentido, v)
}

func (e *engineQueAnotaViajes) anotados(sentido string) []memory.Viaje {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []memory.Viaje
	for _, a := range e.viajes {
		if a.sentido == sentido {
			out = append(out, a.v)
		}
	}
	return out
}

// serverQueAnotaViajes arma un server sobre un engine real envuelto, con el sync apuntando a url.
func serverQueAnotaViajes(t *testing.T, url string, teamMode bool) (*McpServer, *engineQueAnotaViajes) {
	t.Helper()
	real, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { real.Close() })
	eng := &engineQueAnotaViajes{StorageBackend: real}
	s := NewMcpServer(eng, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: teamMode}))
	s.SetSyncClient(newTestSyncClient(t, url), config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, BackoffBaseSeconds: 5, BackoffMaxSeconds: 300})
	return s, eng
}

// TestSyncStatusCuentaLoDeHoy: tres filas que el central acepta dejan un viaje de subida con tres
// filas, tres posts y bytes en el cable, y musubi_sync_status dice «3 enviadas en 24 h».
//
// Sabotaje: que drainOutboxOnce no registre el viaje.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="s.engine.RegistrarViaje(memory.ViajeSubida, v)"
// arnes: a="error(nil)"
func TestSyncStatusCuentaLoDeHoy(t *testing.T) {
	stub := httptest.NewServer(newCentralStub())
	defer stub.Close()
	s, eng := serverQueAnotaViajes(t, stub.URL, false)

	for _, id := range []string{"n1", "n2", "n3"} {
		saveShared(t, s, id)
	}
	s.drainOutboxOnce(context.Background())

	r, err := s.engine.ResumenDelSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := r.SubidaHoy; got.Filas != 3 || got.Posts != 3 || got.BytesCable <= 0 || got.BytesCable != got.BytesCrudos {
		t.Errorf("subida de hoy en sync_viajes = %+v; esperaba filas=3 posts=3 y bytes > 0 (cable = crudos, sin comprimir)", got)
	}
	if n := len(eng.anotados(memory.ViajeSubida)); n != 1 {
		t.Errorf("un tick registró %d viajes de subida; esperaba uno solo", n)
	}

	res, e := call(t, s, "musubi_sync_status", map[string]interface{}{})
	if e != nil {
		t.Fatalf("sync_status: %+v", e)
	}
	txt := res.(CallToolResponse).Content[0].Text
	for _, want := range []string{"3 enviadas en 24 h", "hoy 3 filas en 3 posts", "\nbajada: ", "\nno viajan: ", `"enviadas_24h":3`, `"pending":0`} {
		if !strings.Contains(txt, want) {
			t.Errorf("musubi_sync_status no dice %q:\n%s", want, txt)
		}
	}
}

// TestUnaSubidaQueNoSalioNoRegistraViaje: un tick sin nada que mandar y un tick con el central
// inalcanzable no escriben en sync_viajes. Un central que CONTESTA, aunque rechace, sí cuenta: el
// POST viajó y le costó.
//
// Sabotaje: registrar aunque no haya salido ningún POST.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="if v.Posts == 0 {"
// arnes: a="if false && v.Posts == 0 {"
func TestUnaSubidaQueNoSalioNoRegistraViaje(t *testing.T) {
	t.Run("tick sin nada pendiente", func(t *testing.T) {
		stub := httptest.NewServer(newCentralStub())
		defer stub.Close()
		s, eng := serverQueAnotaViajes(t, stub.URL, false)
		s.drainOutboxOnce(context.Background())
		if got := eng.anotados(memory.ViajeSubida); len(got) != 0 {
			t.Errorf("un tick vacío registró viajes: %+v", got)
		}
	})

	t.Run("central inalcanzable", func(t *testing.T) {
		caido := httptest.NewServer(http.NotFoundHandler())
		url := caido.URL
		caido.Close() // la dirección queda sin nadie escuchando: el POST no llega a tener respuesta
		s, eng := serverQueAnotaViajes(t, url, false)
		saveShared(t, s, "no-sale")
		s.drainOutboxOnce(context.Background())
		if p, _, _ := statsOf(t, s); p != 1 {
			t.Fatalf("precondición: la fila tenía que seguir pendiente, pending=%d", p)
		}
		if got := eng.anotados(memory.ViajeSubida); len(got) != 0 {
			t.Errorf("un tick que no llegó al central registró viajes: %+v", got)
		}
	})

	t.Run("central que rechaza sí cuenta el post", func(t *testing.T) {
		stub := newCentralStub()
		stub.status = http.StatusInternalServerError
		srv := httptest.NewServer(stub)
		defer srv.Close()
		s, eng := serverQueAnotaViajes(t, srv.URL, false)
		saveShared(t, s, "rechazada")
		s.drainOutboxOnce(context.Background())
		got := eng.anotados(memory.ViajeSubida)
		if len(got) != 1 || got[0].Posts != 1 || got[0].Filas != 0 || got[0].Rechazados != 1 ||
			got[0].BytesRechazados <= 0 || got[0].BytesCable != 0 {
			t.Errorf("un POST contestado con 500 tenía que contar posts=1 filas=0, 1 rechazado con sus bytes y 0 B de cable, vino %+v", got)
		}
	})
}

// centralQueSirvePull arma un central que contesta musubi_sync_pull con `n` items en la primera
// página y vacío después.
func centralQueSirvePull(n int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var items []string
		for i := 1; i <= n; i++ {
			items = append(items, `{"rowid":`+strconv.Itoa(i)+`,"id":"c`+strconv.Itoa(i)+`","topic_key":"t/a","content":"del central `+strconv.Itoa(i)+`","importance":1,"mem_type":"semantic","author":"ana","project_id":"acme"}`)
		}
		payload := `{"items":[` + strings.Join(items, ",") + `],"next_cursor":` + strconv.Itoa(n) + `}`
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"pull","result":{"content":[{"type":"text","text":` + strconv.Quote(payload) + `}]}}`))
	}))
}

// TestLaBajadaRegistraSuViaje: una página que llega bien deja un viaje de bajada con sus filas, la
// página y los bytes.
//
// Sabotaje: que drainInboundOnce no registre el viaje.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="s.engine.RegistrarViaje(memory.ViajeBajada, viaje)"
// arnes: a="error(nil)"
func TestLaBajadaRegistraSuViaje(t *testing.T) {
	central := centralQueSirvePull(2)
	defer central.Close()
	s, eng := serverQueAnotaViajes(t, central.URL, true)

	s.drainInboundOnce(context.Background())

	r, err := s.engine.ResumenDelSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := r.BajadaHoy; got.Filas != 2 || got.Posts != 1 || got.BytesCable <= 0 || got.BytesCable != got.BytesCrudos {
		t.Errorf("bajada de hoy en sync_viajes = %+v; esperaba filas=2 posts=1 y bytes > 0", got)
	}
	if n := len(eng.anotados(memory.ViajeBajada)); n != 1 {
		t.Errorf("un tick registró %d viajes de bajada; esperaba uno solo", n)
	}
}

// TestUnaBajadaQueNoSalioNoRegistraViaje: los ticks que no bajaron nada —el Pull falló, otro
// proceso tiene el candado, este proceso está cediendo tras un fallo— no escriben en sync_viajes.
// Es el contrato con el frente que ajusta el ritmo de la bajada: cada tick salteado sin escritura.
//
// Sabotaje: registrar el viaje en todos los ticks, hayan salido o no.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="if salioALaRed {"
// arnes: a="if true || salioALaRed {"
func TestUnaBajadaQueNoSalioNoRegistraViaje(t *testing.T) {
	t.Run("el pull falla", func(t *testing.T) {
		caido := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer caido.Close()
		s, eng := serverQueAnotaViajes(t, caido.URL, true)
		s.drainInboundOnce(context.Background())
		if got := eng.anotados(memory.ViajeBajada); len(got) != 0 {
			t.Errorf("un tick con el Pull fallido registró viajes: %+v", got)
		}
	})

	t.Run("otro proceso tiene el candado", func(t *testing.T) {
		central := centralQueSirvePull(2)
		defer central.Close()
		s, eng := serverQueAnotaViajes(t, central.URL, true)
		if ok, err := s.engine.ReclamarBajada("otro-proceso", 120); err != nil || !ok {
			t.Fatalf("precondición: otro proceso tenía que tomar el candado (ok=%v, err=%v)", ok, err)
		}
		s.drainInboundOnce(context.Background())
		if got := eng.anotados(memory.ViajeBajada); len(got) != 0 {
			t.Errorf("un tick sin el candado registró viajes: %+v", got)
		}
	})

	t.Run("cediendo tras un fallo", func(t *testing.T) {
		central := centralQueSirvePull(2)
		defer central.Close()
		s, eng := serverQueAnotaViajes(t, central.URL, true)
		s.bajadaCedidaHasta.Store(time.Now().Add(time.Minute).UnixNano())
		s.drainInboundOnce(context.Background())
		if got := eng.anotados(memory.ViajeBajada); len(got) != 0 {
			t.Errorf("un tick que cedía la bajada registró viajes: %+v", got)
		}
	})
}

// textoDeSyncStatusComo llama a musubi_sync_status con la credencial p y devuelve el texto entero.
func textoDeSyncStatusComo(t *testing.T, s *McpServer, p *Principal) string {
	t.Helper()
	params, _ := json.Marshal(CallToolRequest{Name: "musubi_sync_status", Arguments: json.RawMessage(`{}`)})
	out, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
	if rpcErr != nil {
		t.Fatalf("sync_status como %s: %+v", p.Name, rpcErr)
	}
	return out.(CallToolResponse).Content[0].Text
}

// syncStatusComo llama a musubi_sync_status con la credencial p y devuelve el resumen de viajes del
// JSON (la última línea del texto).
func syncStatusComo(t *testing.T, s *McpServer, p *Principal) memory.ResumenDelSync {
	t.Helper()
	txt := textoDeSyncStatusComo(t, s, p)
	var cuerpo struct {
		Viajes memory.ResumenDelSync `json:"viajes"`
	}
	if err := json.Unmarshal([]byte(txt[strings.LastIndex(txt, "\n")+1:]), &cuerpo); err != nil {
		t.Fatalf("decodear el JSON de sync_status: %v\n%s", err, txt)
	}
	return cuerpo.Viajes
}

// TestSyncStatusAcotadoAlProyecto: el central también sirve musubi_sync_status, y lo que cuenta
// sobre observaciones —enviadas, locales, en cuarentena— es información del proyecto dueño. Una
// credencial de otro proyecto ve ceros; el admin federado ve el dato (control positivo: sin él, una
// tool que devolviera ceros para todos pasaría con honores).
//
// Sabotaje: contar con el ctx pelado, sin el alcance de la credencial.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="s.engine.ResumenDelSync(sctx)"
// arnes: a="s.engine.ResumenDelSync(ctx)"
func TestSyncStatusAcotadoAlProyecto(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	if err := engine.SaveObservationTypedFrom("web", "", "web-local", "web/t", "VICTIM local de web", 1, "semantic", memory.ScopeLocal, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ProposeObservation("web", "", "web/t", "VICTIM propuesta de web", "modelo-x", 0.5, "semantic", nil); err != nil {
		t.Fatal(err)
	}
	if err := engine.SaveObservationTypedFrom("web", "", "web-shared", "web/t", "VICTIM compartida de web", 1, "semantic", memory.ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.ClaimOutboxBatch(50, 60); err != nil {
		t.Fatal(err)
	}
	if err := engine.MarkOutboxSent("web-shared"); err != nil {
		t.Fatal(err)
	}

	vecino := syncStatusComo(t, s, &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"})
	if vecino.EnviadasDia != 0 || vecino.NoViajan != (memory.NoViajan{}) {
		t.Errorf("FUGA cross-tenant: crm ve los conteos de web: enviadas=%d no_viajan=%+v", vecino.EnviadasDia, vecino.NoViajan)
	}
	federado := syncStatusComo(t, s, &Principal{Name: "root", Role: RoleAdmin})
	if federado.EnviadasDia != 1 || federado.NoViajan.Locales != 1 || federado.NoViajan.EnCuarentena != 1 {
		t.Errorf("el admin federado tenía que ver lo de web (seed roto o recorte de más): enviadas=%d no_viajan=%+v",
			federado.EnviadasDia, federado.NoViajan)
	}
}

// TestSyncStatusNoMuestraElOutboxAjeno: lo que musubi_sync_status cuenta del OUTBOX —pendientes,
// enviadas, dead-letter, espejo, la antigüedad y el TEXTO del último error— también es del proyecto
// dueño de cada nota. Una credencial de otro proyecto ve ceros y ningún error ajeno; el admin
// federado ve el dato. Hoy es latente (el central no tiene outbox), pero la tool se declara aislada.
//
// Sabotaje: leer el outbox con el ctx pelado.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="s.engine.OutboxHealthCtx(sctx)"
// arnes: a="s.engine.OutboxHealthCtx(ctx)"
func TestSyncStatusNoMuestraElOutboxAjeno(t *testing.T) {
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	for _, id := range []string{"web-enviada", "web-muerta"} {
		if err := engine.SaveObservationTypedFrom("web", "", id, "web/t", "VICTIM "+id, 1, "semantic", memory.ScopeShared, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := engine.ClaimOutboxBatch(50, 60); err != nil {
		t.Fatal(err)
	}
	if err := engine.MarkOutboxSent("web-enviada"); err != nil {
		t.Fatal(err)
	}
	if err := engine.MarkOutboxDead("web-muerta", "rechazo permanente de la nota VICTIM de web"); err != nil {
		t.Fatal(err)
	}

	type outbox struct {
		Pending, Sent, Dead, Espejo int
		OldestPendingAgeSec         int64  `json:"oldest_pending_age_seconds"`
		LastError                   string `json:"last_error"`
	}
	leer := func(txt string) outbox {
		var o outbox
		if err := json.Unmarshal([]byte(txt[strings.LastIndex(txt, "\n")+1:]), &o); err != nil {
			t.Fatalf("decodear el JSON de sync_status: %v\n%s", err, txt)
		}
		return o
	}

	txt := textoDeSyncStatusComo(t, s, &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"})
	if o := leer(txt); o != (outbox{}) || strings.Contains(txt, "VICTIM") {
		t.Errorf("FUGA cross-tenant: crm ve el outbox de web: %+v\n%s", o, txt)
	}
	fed := leer(textoDeSyncStatusComo(t, s, &Principal{Name: "root", Role: RoleAdmin}))
	if fed.Sent != 1 || fed.Dead != 1 || !strings.Contains(fed.LastError, "VICTIM") {
		t.Errorf("el admin federado tenía que ver el outbox de web (seed roto o recorte de más): %+v", fed)
	}
}

// TestLaBajadaPorNotaNoCuentaElSondeo: contra el central REAL (su handler HTTP), treinta ticks sin
// novedades y uno con una nota. Las páginas vacías —el régimen de un nodo quieto, miles por día y
// por base— suman a posts (el contador de pulls) y a vacias/bytes_vacias, pero NO al cable: así
// SUM(bytes_cable)/SUM(filas), la métrica (1) del plan, da el peso de la nota y no el del sondeo.
// Con el cable mezclado, la revisión midió 7.088 B «por nota» para una nota de 2.903 B.
//
// Sabotaje: que las páginas vacías vuelvan a sumar al cable.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="if len(pl.Items) == 0 {"
// arnes: a="if false && len(pl.Items) == 0 {"
func TestLaBajadaPorNotaNoCuentaElSondeo(t *testing.T) {
	centralEng, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	defer centralEng.Close()
	central := NewMcpServer(centralEng, t.TempDir(), embedding.NoopProvider{})
	ts := httptest.NewServer(central.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second}))
	defer ts.Close()
	cli, eng := serverQueAnotaViajes(t, ts.URL, true)

	const ticksVacios = 30
	for i := 0; i < ticksVacios; i++ {
		cli.drainInboundOnce(context.Background())
	}
	// Una nota del tamaño medio del central (2.617 B de content, medido en producción).
	if err := centralEng.SaveObservationTyped("nota-real", "t/x", strings.Repeat("palabra ", 327), 1, "semantic", memory.ScopeShared, nil); err != nil {
		t.Fatal(err)
	}
	cli.drainInboundOnce(context.Background())

	vs := eng.anotados(memory.ViajeBajada)
	if len(vs) != ticksVacios+1 {
		t.Fatalf("precondición: cada tick que volvió bien registra UN viaje; esperaba %d, hubo %d", ticksVacios+1, len(vs))
	}
	nota := vs[len(vs)-1]
	if nota.Filas != 1 || nota.Vacias != 0 || nota.BytesCable <= 0 {
		t.Fatalf("precondición: la nota tenía que bajar en una página con datos: %+v", nota)
	}
	r, err := cli.engine.ResumenDelSync(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	b := r.BajadaHoy
	if b.Filas != 1 || b.BytesCable != nota.BytesCable || b.BytesCrudos != nota.BytesCrudos {
		t.Errorf("DILUIDA: la métrica (1) da %d B por nota y la página de la nota pesó %d B; el resto es sondeo (%+v)",
			b.BytesCable/max(b.Filas, 1), nota.BytesCable, b)
	}
	if b.Vacias != b.Posts-1 || b.Vacias < ticksVacios || b.BytesVacias <= 0 {
		t.Errorf("las páginas vacías no quedaron medidas aparte, o dejaron de contar como pulls: %+v", b)
	}
	t.Logf("métrica (1) de la bajada = %d B por nota; el sondeo, aparte: %d páginas vacías, %d B (%.0f B cada una), en %d pulls",
		b.BytesCable/max(b.Filas, 1), b.Vacias, b.BytesVacias, float64(b.BytesVacias)/float64(max(b.Vacias, 1)), b.Posts)
}
