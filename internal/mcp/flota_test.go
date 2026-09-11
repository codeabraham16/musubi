package mcp

// Los invariantes de FLOTA EN VIVO (specs/flota-en-vivo/spec.md), cada uno con su sabotaje
// anotado. La disciplina SDD: cada test se vio ROJO flipeando el invariante que declara — el
// flip exacto está en el comentario de cada uno, para poder repetir la verificación.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/memory"
)

func servidorFlota(t *testing.T) (*McpServer, *httptest.Server) {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	reg := &PrincipalRegistry{principals: []Principal{
		{Name: "cabina-flota", ProjectID: "musubi", Role: RoleReader, Read: ReadAll, Write: WriteNone, hash: hashToken("token-flota")},
	}}
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, registry: reg}))
	t.Cleanup(ts.Close)
	return s, ts
}

func postFlota(t *testing.T, url, token string, body []byte) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url+"/api/flota", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /api/flota: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

// recogerEventos drena el canal del suscriptor hasta juntar n eventos o vencer el plazo.
func recogerEventos(t *testing.T, ch <-chan LiveEvent, n int) []LiveEvent {
	t.Helper()
	var out []LiveEvent
	plazo := time.After(2 * time.Second)
	for len(out) < n {
		select {
		case ev := <-ch:
			out = append(out, ev)
		case <-plazo:
			t.Fatalf("esperaba %d eventos, llegaron %d", n, len(out))
		}
	}
	return out
}

// I1 — SÓLO EL TRABAJO VIAJA (y sólo el propio). El sondeo es el 99,92 % del tráfico local y no
// cruza la red ni una vez; y un evento que no sea de origen local (p. ej. "flota" relayado)
// jamás se re-reenvía — el freno estructural anti-loop.
// Sabotaje visto rojo: esDeFlota ⇒ `return ev.Tool != ""`.
func TestFlotaSoloElTrabajoViaja(t *testing.T) {
	casos := []struct {
		ev     LiveEvent
		quiero bool
	}{
		{LiveEvent{Tool: "musubi_recall", Kind: KindTrabajo, Origen: "local"}, true},
		{LiveEvent{Tool: "musubi_sync_pull", Kind: KindSondeo, Origen: "local"}, false},
		{LiveEvent{Tool: "musubi_recall", Kind: KindTrabajo, Origen: origenFlota}, false},
		{LiveEvent{Tool: "musubi_recall", Kind: KindTrabajo, Origen: ""}, false},
		{LiveEvent{Tool: "", Kind: KindTrabajo, Origen: "local"}, false},
	}
	for _, c := range casos {
		if got := esDeFlota(c.ev); got != c.quiero {
			t.Errorf("esDeFlota(%+v) = %v, quiero %v", c.ev, got, c.quiero)
		}
	}
}

// I2 — LA IDENTIDAD LA SELLA EL SERVER. El principal y el project del evento publicado salen del
// TOKEN autenticado; el body ni siquiera tiene dónde declararlos (I4 rechaza el intento).
// Sabotaje visto rojo: en handlerFlota, no estampar principal (dejar ev.Principal vacío).
func TestFlotaIdentidadLaSellaElServer(t *testing.T) {
	s, ts := servidorFlota(t)
	id, ch, _ := s.live.subscribe("", false)
	defer s.live.unsubscribe(id)

	body, _ := json.Marshal([]flotaEventoEntrante{{
		At: time.Now().Format("2006-01-02T15:04:05.000Z07:00"), Tool: "musubi_recall", Outcome: "ok", DurationMs: 12.5,
	}})
	resp := postFlota(t, ts.URL, "token-flota", body)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, quiero 202", resp.StatusCode)
	}
	evs := recogerEventos(t, ch, 1)
	if evs[0].Principal != "cabina-flota" || evs[0].Project != "musubi" {
		t.Fatalf("el evento no lleva la identidad del token: %+v", evs[0])
	}
	if evs[0].Origen != origenFlota {
		t.Fatalf("origen = %q, quiero %q", evs[0].Origen, origenFlota)
	}
}

// Y sin token no entra nadie: la telemetría de la flota es exactamente el tipo de superficie
// que se deja abierta por accidente.
func TestFlotaSinCredencialNoEntra(t *testing.T) {
	_, ts := servidorFlota(t)
	body, _ := json.Marshal([]flotaEventoEntrante{{Tool: "musubi_recall"}})
	if resp := postFlota(t, ts.URL, "", body); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("sin token: status = %d, quiero 401", resp.StatusCode)
	}
	if resp := postFlota(t, ts.URL, "token-falso", body); resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("token inválido: status = %d, quiero 401", resp.StatusCode)
	}
}

// I3 — EL RECEPTOR ACOTA. Un batch más grande que el tope rebota entero con 400 y no publica
// nada. Sabotaje visto rojo: quitar el chequeo de flotaBatchTope.
func TestFlotaElReceptorAcota(t *testing.T) {
	s, ts := servidorFlota(t)
	id, ch, _ := s.live.subscribe("", false)
	defer s.live.unsubscribe(id)

	grandes := make([]flotaEventoEntrante, flotaBatchTope+1)
	for i := range grandes {
		grandes[i] = flotaEventoEntrante{Tool: "musubi_recall"}
	}
	body, _ := json.Marshal(grandes)
	if resp := postFlota(t, ts.URL, "token-flota", body); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("batch de %d: status = %d, quiero 400", len(grandes), resp.StatusCode)
	}
	select {
	case ev := <-ch:
		t.Fatalf("un batch rechazado publicó igual: %+v", ev)
	case <-time.After(150 * time.Millisecond):
	}
}

// I4 — SIN CONTENIDO, POR CONSTRUCCIÓN. El decode es estricto: un campo extra (content, args,
// principal — lo que sea) rechaza el batch entero. Es la mitad receptora del invariante L1 del
// feed. Sabotaje visto rojo: quitar DisallowUnknownFields.
func TestFlotaSinContenidoPorConstruccion(t *testing.T) {
	_, ts := servidorFlota(t)
	body := []byte(`[{"tool":"musubi_recall","outcome":"ok","ms":3,"content":"el secreto que no debe viajar"}]`)
	resp := postFlota(t, ts.URL, "token-flota", body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("evento con campo extra: status = %d, quiero 400", resp.StatusCode)
	}
}

// I5 — EL KIND LO DECIDE EL SERVER, y la forma del tool también. Un sondeo no se vuelve trabajo
// por venir de afuera (clasificarTool se recomputa), y un nombre de tool que no tiene forma de
// tool se descarta contado, no publicado.
// Sabotaje visto rojo: estampar Kind: KindTrabajo fijo en el receptor.
func TestFlotaElKindLoDecideElServer(t *testing.T) {
	s, ts := servidorFlota(t)
	id, ch, _ := s.live.subscribe("", false)
	defer s.live.unsubscribe(id)

	body, _ := json.Marshal([]flotaEventoEntrante{
		{Tool: "musubi_sync_pull", Outcome: "ok"},
		{Tool: "musubi_recall", Outcome: "raro"},
		{Tool: "DROP TABLE observations", Outcome: "ok"},
	})
	resp := postFlota(t, ts.URL, "token-flota", body)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, quiero 202", resp.StatusCode)
	}
	var rep map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&rep); err != nil {
		t.Fatalf("respuesta: %v", err)
	}
	if rep["aceptados"] != 2 || rep["saltados"] != 1 {
		t.Fatalf("reparto = %v, quiero 2 aceptados y 1 saltado", rep)
	}
	evs := recogerEventos(t, ch, 2)
	porTool := map[string]LiveEvent{}
	for _, ev := range evs {
		porTool[ev.Tool] = ev
	}
	if ev, ok := porTool["musubi_sync_pull"]; !ok || ev.Kind != KindSondeo {
		t.Fatalf("sync_pull publicado como %+v, quiero kind sondeo", porTool["musubi_sync_pull"])
	}
	if ev, ok := porTool["musubi_recall"]; !ok || ev.Kind != KindTrabajo || ev.Outcome != "error" {
		t.Fatalf("recall publicado como %+v, quiero kind trabajo y outcome normalizado a error", porTool["musubi_recall"])
	}
	if _, ok := porTool["DROP TABLE observations"]; ok {
		t.Fatal("un nombre sin forma de tool se publicó al DOM de todos los paneles")
	}
}

// El gate de config: la frontera del opt-in es la del sync, y false lo apaga explícito.
func TestFlotaVivoActivoSigueLaFronteraDelSync(t *testing.T) {
	off := false
	casos := []struct {
		cfg    config.SyncConfig
		quiero bool
	}{
		{config.SyncConfig{Enabled: true, CentralURL: "https://c"}, true},
		{config.SyncConfig{Enabled: true, CentralURL: "https://c", FlotaVivo: &off}, false},
		{config.SyncConfig{Enabled: false, CentralURL: "https://c"}, false},
		{config.SyncConfig{Enabled: true, CentralURL: "  "}, false},
	}
	for i, c := range casos {
		if got := c.cfg.FlotaVivoActivo(); got != c.quiero {
			t.Errorf("caso %d: FlotaVivoActivo() = %v, quiero %v", i, got, c.quiero)
		}
	}
}

// PushFlota apunta al endpoint correcto derivándolo de la URL del sync.
func TestFlotaPushApuntaAlEndpoint(t *testing.T) {
	var camino string
	var cuerpo []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		camino = r.URL.Path
		cuerpo, _ = json.Marshal(r.Header.Get("Authorization") != "")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	t.Setenv("MUSUBI_TEST_FLOTA_TOKEN", "tok")
	cl, err := NewSyncClient(config.SyncConfig{
		Enabled: true, CentralURL: srv.URL, AuthTokenEnv: "MUSUBI_TEST_FLOTA_TOKEN", AllowInsecureToken: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := cl.PushFlota(t.Context(), []LiveEvent{{Tool: "musubi_recall", Kind: KindTrabajo}}); err != nil {
		t.Fatalf("PushFlota: %v", err)
	}
	if camino != "/api/flota" {
		t.Fatalf("camino = %q, quiero /api/flota", camino)
	}
	if !strings.Contains(string(cuerpo), "true") {
		t.Fatal("el POST viajó sin Authorization")
	}
}

// EL ROUND-TRIP DE VERDAD: el remitente REAL contra el receptor REAL. Los otros tests construyen
// el body con `flotaEventoEntrante` —la struct del receptor— así que las dos mitades nunca se
// probaron una contra la otra: es «el test espera el proxy, no la cosa», y tapó que PushFlota
// serializa `LiveEvent` entero (seq, kind, origen) mientras el receptor decodifica estricto sin
// esos campos. Resultado: TODO batch rebotaba con 400 y la feature no entregaba un solo evento,
// con el único síntoma de una línea de log con freno de un minuto.
func TestFlotaRoundTripRemitenteContraReceptor(t *testing.T) {
	s, ts := servidorFlota(t)
	id, ch, _ := s.live.subscribe("", false)
	defer s.live.unsubscribe(id)

	t.Setenv("MUSUBI_TEST_FLOTA_RT", "token-flota")
	cl, err := NewSyncClient(config.SyncConfig{
		Enabled: true, CentralURL: ts.URL, AuthTokenEnv: "MUSUBI_TEST_FLOTA_RT", AllowInsecureToken: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Un evento tal cual sale del feed local: con seq, kind y origen puestos por publicarUso.
	lote := []LiveEvent{{
		Seq: 7, At: time.Now().Format("2006-01-02T15:04:05.000Z07:00"),
		Tool: "musubi_recall", Outcome: "ok", DurationMs: 12.5,
		Kind: KindTrabajo, Origen: "local", Principal: "mentira", Project: "ajeno",
	}}
	if err := cl.PushFlota(t.Context(), lote); err != nil {
		t.Fatalf("el remitente real no pudo entregarle al receptor real: %v", err)
	}
	evs := recogerEventos(t, ch, 1)
	if evs[0].Tool != "musubi_recall" {
		t.Fatalf("tool = %q", evs[0].Tool)
	}
	// Y el re-sellado sigue en pie aunque el body venga con identidad declarada (I2/I5).
	if evs[0].Principal != "cabina-flota" || evs[0].Project != "musubi" {
		t.Fatalf("la identidad del body se coló: %+v", evs[0])
	}
	if evs[0].Origen != origenFlota {
		t.Fatalf("origen = %q, quiero %q", evs[0].Origen, origenFlota)
	}
}

// TestFlotaVaciaContestaInventarioVacioYNoError fija la distinción que faltaba: «no pude resolver
// el proyecto» y «no hay ninguna máquina» son dos estados distintos, y el segundo tiene respuesta.
//
// LOS NÚMEROS QUE LO ORIGINAN: musubi_fleet_list falla 19 de 40 veces en esta instalación (47%), y
// aporta el 48% de TODOS los errores del ledger. Las 19 son este caso — stdio local sin `project`
// contra una tabla `devices` vacía. El mismo estado del mundo contestaba ok/total:0 si nombrabas el
// proyecto y ERROR si no.
//
// Y la guarda que debía cubrirlo no lo hacía: TestUnPanelSinProyectoPropioVeTodoLoQueSuCredencialConcede
// enrola DOS máquinas antes de medir, así que sólo prueba el caso CON flota. Cero pruebas del caso
// vacío en todo internal/mcp.
func TestFlotaVaciaContestaInventarioVacioYNoError(t *testing.T) {
	s := newTestServer(t, nil) // base nueva: cero devices, que es el estado bajo prueba

	casos := []struct {
		nombre string
		llamar func() (interface{}, *RpcError)
	}{
		{"fleet_list", func() (interface{}, *RpcError) {
			return s.toolFleetList(context.Background(), json.RawMessage(`{}`))
		}},
		{"fleet_metrics", func() (interface{}, *RpcError) {
			return s.toolFleetMetrics(context.Background(), json.RawMessage(`{}`))
		}},
		{"fleet_services", func() (interface{}, *RpcError) {
			return s.toolFleetServices(context.Background(), json.RawMessage(`{}`))
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			res, rpcErr := c.llamar()
			if rpcErr != nil {
				t.Fatalf("con la flota vacía %s devolvió error (%d: %s); tenía que devolver un inventario vacío",
					c.nombre, rpcErr.Code, rpcErr.Message)
			}
			if res == nil {
				t.Fatal("devolvió nil sin error")
			}
		})
	}
}

// backendConListaIlegible es un almacén al que NO se le puede preguntar qué proyectos tienen
// máquinas. Embebe StorageBackend en nil a propósito: si una tool tocara cualquier OTRO método
// del almacén, paniquea, y así la prueba no puede pasar por un camino que no es el que mide.
type backendConListaIlegible struct {
	memory.StorageBackend
}

func (backendConListaIlegible) ProyectosConDevices(int) ([]string, error) {
	return nil, errors.New("la tabla devices no se pudo leer")
}

// TestUnaListaDeProyectosILEGIBLENoSeContestaComoInventarioVacio es el hermano exacto de
// TestFlotaVaciaContestaInventarioVacioYNoError, y existe porque el defecto que cubre NO ESTABA EN
// NINGUNA DE LAS DOS RAMAS QUE LO CREARON — nació del merge.
//
// De un lado, `proyectosVisibles` aprendió a decir «no pude leer la lista» (el tercer valor
// `ilegible`, que en el export sale por la serie `unreadable`). Del otro, esta rama convirtió
// «cero proyectos» en «inventario vacío» para que el panel dejara de leerse como caído. Cada
// cambio es correcto solo. Juntos con un resolve ingenuo —`return ps, tr, true`— una base
// ILEGIBLE contesta `{"total": 0, "devices": []}`: un cero que significa «no sé» servido como si
// fuera una medición, que es exactamente lo que el arreglo del export fue a sacar del otro lado.
//
// La diferencia importa porque las dos respuestas mandan a mirar lugares opuestos: «no hay
// máquinas» manda a enrolar, «no puedo leer la lista» manda a mirar la base.
//
// SABOTAJE QUE LA HACE FALLAR, verificado y corrido: en `proyectosParaLeer`, descartar el valor
// —`ps, tr, _ := proyectosVisibles(...)` y `return ps, tr, true`—, que es literalmente lo que
// escribe un resolve ingenuo del conflicto. Los tres subtests pasan a recibir
// `{"total": 0, "devices": []}` en verde. (Dejar el `!ilegible` por `true` a secas NO sirve de
// sabotaje: el compilador lo ataja con «declared and not used», así que no es la forma en que
// este defecto puede volver.)
func TestUnaListaDeProyectosILEGIBLENoSeContestaComoInventarioVacio(t *testing.T) {
	s := newTestServer(t, nil)
	s.engine = backendConListaIlegible{}

	casos := []struct {
		nombre string
		llamar func() (interface{}, *RpcError)
	}{
		{"fleet_list", func() (interface{}, *RpcError) {
			return s.toolFleetList(context.Background(), json.RawMessage(`{}`))
		}},
		{"fleet_metrics", func() (interface{}, *RpcError) {
			return s.toolFleetMetrics(context.Background(), json.RawMessage(`{}`))
		}},
		{"fleet_services", func() (interface{}, *RpcError) {
			return s.toolFleetServices(context.Background(), json.RawMessage(`{}`))
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			res, rpcErr := c.llamar()
			if rpcErr == nil {
				t.Fatalf("con la lista de proyectos ILEGIBLE %s contestó %v sin error: "+
					"un inventario en cero que en realidad es «no sé» se lee como flota apagada", c.nombre, res)
			}
		})
	}
}

// backendConListaRecortada finge más proyectos con máquinas de los que entran en un barrido, y
// una única máquina con el nombre buscado adentro del recorte. Es el estado en que `hallados`
// vale 1 y NO se puede probar que sea única: la segunda homónima no se descartó, se quedó afuera.
type backendConListaRecortada struct {
	memory.StorageBackend
	proyectoConLaMaquina string // "" ⇒ ninguno la tiene, y el barrido igual fue parcial
	nombre               string
}

func (b backendConListaRecortada) ProyectosConDevices(tope int) ([]string, error) {
	// `proyectosVisibles` pide tope+1 justamente para poder DETECTAR el recorte. Devolver los
	// tope+1 es lo que enciende `truncado`.
	ps := make([]string, 0, tope)
	for i := 0; i < tope; i++ {
		ps = append(ps, fmt.Sprintf("proyecto-%02d", i))
	}
	return ps, nil
}

func (b backendConListaRecortada) DevicePorNombre(projectID, name string) (fleet.Device, bool, error) {
	if projectID == b.proyectoConLaMaquina && name == b.nombre {
		return fleet.Device{Name: name, ProjectID: projectID}, true, nil
	}
	return fleet.Device{}, false, nil
}

// TestUnaBusquedaPARCIALNoDesempataSolaNiDeclaraInexistente cubre el último de los cinco
// llamadores de `proyectosParaLeer`, que era el único que TIRABA `truncado`. N-1 de N.
//
// Los dos daños son distintos y los dos importan:
//
//  1. `hallados == 0` sobre una lista recortada contestaba «no hay ninguna máquina %q en los
//     proyectos que alcanzás» — un falso negativo que AFIRMA haber buscado en todo lo alcanzable,
//     y manda a enrolar una máquina que ya existe.
//
//  2. `hallados == 1` sobre una lista recortada devolvía ese device en silencio. Es exactamente
//     lo que esta función se extrajo para impedir —que dos homónimas en dos tenants no se
//     desempaten solas— entrando por la otra puerta: la segunda no se descartó, se quedó afuera
//     del barrido, y `hallados` cayó de 2 a 1.
//
// HOY ES LATENTE y se dice: hace falta un principal `read: all` con más de 64 proyectos con
// máquinas (`proyectosParaExportar`). Se arregla igual porque este repo YA se comió un techo de
// 64 que truncaba en silencio —el latido y su inventario de servicios— y la forma es idéntica.
//
// SABOTAJE QUE LA HACE FALLAR: volver la línea a `proyectos, _, vacioLegitimo :=` y sacar los dos
// bloques de `truncado`. Verificado: el subtest de la homónima pasa a recibir un device sin error.
func TestUnaBusquedaPARCIALNoDesempataSolaNiDeclaraInexistente(t *testing.T) {
	casos := []struct {
		nombre   string
		proyecto string
		espera   string
	}{
		{"una sola hallada no es una única", "proyecto-07", "NO puedo probar que sea la única"},
		{"ninguna hallada no es inexistente", "", "la búsqueda fue PARCIAL"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s := newTestServer(t, nil)
			s.engine = backendConListaRecortada{proyectoConLaMaquina: c.proyecto, nombre: "pc-gio"}

			d, pr, rpcErr := s.resolverDeviceUnico(nil, "pc-gio", "")
			if rpcErr == nil {
				t.Fatalf("con la lista de proyectos RECORTADA devolvió la máquina %q del proyecto %q sin error: "+
					"una búsqueda parcial no prueba unicidad, y contestar es elegir por el que preguntó", d.Name, pr)
			}
			if !strings.Contains(rpcErr.Message, c.espera) {
				t.Errorf("el error no dice que la búsqueda fue parcial.\n  esperaba que contuviera: %q\n  obtuve: %q", c.espera, rpcErr.Message)
			}
		})
	}
}
