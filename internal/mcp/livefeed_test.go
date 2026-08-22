package mcp

// Invariantes del FEED EN VIVO (livefeed.go). Lo que se prueba acá no es que "ande", es que no
// pueda hacer las tres cosas que un feed en vivo hace mal por default: frenar el camino caliente,
// mentir sobre lo que se perdió, y dejar que un tenant vea el trabajo de otro.

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
)

// L-VIVO-1 · publish JAMÁS bloquea, ni con un suscriptor que no lee nunca.
//
// Es el invariante que sostiene todo lo demás: publish corre adentro de registrarUso, o sea en el
// camino de salida de TODA tool. Un envío bloqueante acá convierte una pestaña en segundo plano
// —que el navegador congela— en un cuelgue del servidor entero.
//
// SABOTAJE QUE TIENE QUE ROMPERLO: cambiar el `select` con `default` de publish por un envío
// directo `s.ch <- e`. Con eso el test no falla: se cuelga hasta el timeout de go test, que es
// exactamente la falla que describe.
func TestFeedVivoNoBloqueaConSuscriptorMuerto(t *testing.T) {
	f := newLiveFeed()
	_, _, _ = f.subscribe("", false) // se suscribe y NUNCA lee

	listo := make(chan struct{})
	go func() {
		for i := 0; i < liveSubBuf*4; i++ {
			f.publish(LiveEvent{Tool: "musubi_recall", Kind: KindTrabajo})
		}
		close(listo)
	}()

	select {
	case <-listo:
	case <-time.After(5 * time.Second):
		t.Fatal("publish se bloqueó con un suscriptor que no lee: el camino caliente de toda tool quedaría colgado")
	}
}

// L-VIVO-2 · lo descartado se AVISA. Un feed que pierde eventos en silencio le hace creer al que
// mira que vio todo, y eso es peor que no tener feed.
func TestFeedVivoAvisaLoQueDescarto(t *testing.T) {
	f := newLiveFeed()
	_, ch, _ := f.subscribe("", false)

	// Llenar el buffer y pasarse: lo que sobra se descarta contándolo.
	const extra = 50
	for i := 0; i < liveSubBuf+extra; i++ {
		f.publish(LiveEvent{Tool: "musubi_recall", Kind: KindTrabajo})
	}
	// Drenar lo que sí entró.
	for i := 0; i < liveSubBuf; i++ {
		<-ch
	}
	// El próximo que entre tiene que traer la cuenta de lo perdido.
	f.publish(LiveEvent{Tool: "musubi_save_observation", Kind: KindTrabajo})
	select {
	case ev := <-ch:
		if ev.Perdidos != extra {
			t.Fatalf("perdidos = %d, esperaba %d: la cuenta de lo descartado no llegó al que mira", ev.Perdidos, extra)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó el evento posterior al descarte")
	}
}

// L-VIVO-3 · AISLAMIENTO POR PROYECTO. Un suscriptor acotado a lo suyo no ve ni un evento de otro
// proyecto — ni en el backlog ni en el stream.
//
// No es paranoia de laboratorio: el evento lleva `principal` y `project`, así que sin este filtro
// un miembro vería EN TIEMPO REAL a qué hora trabaja otro equipo, con qué herramientas y a qué
// ritmo. Es la misma clase de fuga que el barrido de aislamiento del repo caza en las lecturas.
func TestFeedVivoAislaPorProyecto(t *testing.T) {
	f := newLiveFeed()
	// Historia previa: hay eventos de los dos proyectos ANTES de que nadie se suscriba.
	f.publish(LiveEvent{Tool: "musubi_recall", Project: "crm", Kind: KindTrabajo})
	f.publish(LiveEvent{Tool: "musubi_recall", Project: "altura", Kind: KindTrabajo})

	_, ch, backlog := f.subscribe("crm", true)
	for _, ev := range backlog {
		if ev.Project != "crm" {
			t.Fatalf("el backlog filtró mal: llegó un evento de %q a un suscriptor de crm", ev.Project)
		}
	}
	if len(backlog) != 1 {
		t.Fatalf("backlog = %d eventos, esperaba 1 (el de crm)", len(backlog))
	}

	// Y en vivo, lo mismo.
	f.publish(LiveEvent{Tool: "musubi_save_observation", Project: "altura", Kind: KindTrabajo})
	f.publish(LiveEvent{Tool: "musubi_save_observation", Project: "crm", Kind: KindTrabajo})
	select {
	case ev := <-ch:
		if ev.Project != "crm" {
			t.Fatalf("llegó en vivo un evento de %q a un suscriptor de crm", ev.Project)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó el evento propio")
	}
	select {
	case ev := <-ch:
		t.Fatalf("llegó un evento de más (%q/%q): el de altura no debería haber pasado", ev.Project, ev.Tool)
	case <-time.After(150 * time.Millisecond):
	}
}

// L-VIVO-4 · la costura de subscribe no pierde ni duplica: lo que ya pasó va en el backlog, lo que
// viene después va por el canal, y nada aparece en los dos.
func TestFeedVivoSinHuecoNiDuplicadoAlSuscribirse(t *testing.T) {
	f := newLiveFeed()
	f.publish(LiveEvent{Tool: "antes"})
	_, ch, backlog := f.subscribe("", false)
	f.publish(LiveEvent{Tool: "despues"})

	if len(backlog) != 1 || backlog[0].Tool != "antes" {
		t.Fatalf("backlog = %+v, esperaba sólo el evento previo", backlog)
	}
	select {
	case ev := <-ch:
		if ev.Tool != "despues" {
			t.Fatalf("por el canal llegó %q; 'antes' ya venía en el backlog y no puede repetirse", ev.Tool)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no llegó el evento posterior a la suscripción")
	}
}

// L-VIVO-5 · el struct del evento NO TIENE DÓNDE PONER CONTENIDO. Es el mismo invariante L1 del
// ledger, pero acá importa más: un feed en vivo es la superficie más fácil de dejar abierta sin
// querer, y `save_observation` recibe justo lo que el portero de privacidad existe para proteger.
//
// El test enumera los campos por reflexión en vez de mirar un JSON de ejemplo: así, agregar un
// campo `Args` o `Result` al struct rompe ACÁ, en el commit que lo agrega, y no en una auditoría
// seis meses después.
func TestLiveEventNoTieneDondePonerContenido(t *testing.T) {
	permitidos := []string{"At", "DurationMs", "Kind", "Outcome", "Perdidos", "Principal", "Project", "Seq", "Tool"}
	var got []string
	tp := reflect.TypeOf(LiveEvent{})
	for i := 0; i < tp.NumField(); i++ {
		got = append(got, tp.Field(i).Name)
	}
	sort.Strings(got)
	if !reflect.DeepEqual(got, permitidos) {
		t.Fatalf("los campos de LiveEvent cambiaron.\n  ahora:    %v\n  permitido: %v\n"+
			"Si el campo nuevo puede llevar argumentos, resultados o mensajes de error, NO va: el feed "+
			"viaja a un navegador y sería una segunda copia de la memoria sensible sin ninguna de sus "+
			"murallas. Si es metadato inocuo, agregalo a la lista con su razón.", got, permitidos)
	}
}

// L-VIVO-6 · una tool desconocida nace VISIBLE (trabajo), no escondida.
//
// La lista es de sondeo y el default es trabajo, a propósito: al revés, una tool nueva nacería
// invisible en el panel, que es la peor falla posible para algo cuyo único trabajo es mostrar lo
// que pasa.
func TestClasificarToolPorDefectoEsTrabajo(t *testing.T) {
	if k := clasificarTool("musubi_tool_que_no_existe_todavia"); k != KindTrabajo {
		t.Fatalf("tool desconocida clasificada como %q; tiene que nacer visible (%q)", k, KindTrabajo)
	}
	if k := clasificarTool("musubi_sync_pull"); k != KindSondeo {
		t.Fatalf("musubi_sync_pull clasificada como %q; es el 76%% del tráfico del central y es sondeo", k)
	}
	if k := clasificarTool("musubi_save_observation"); k != KindTrabajo {
		t.Fatalf("musubi_save_observation clasificada como %q; es trabajo", k)
	}
}

// L-VIVO-7 · apagar la HISTORIA no apaga el PRESENTE: con el ledger de uso deshabilitado, el feed
// sigue publicando. Antes de que publicarUso saliera del guard de `s.ledger == nil`, poner
// `usage_ledger.enabled: false` dejaba el panel mudo sin que nada lo dijera.
func TestFeedVivoPublicaAunqueElLedgerEsteApagado(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{}) // sin WithUsageLedger ⇒ s.ledger == nil
	if s.ledger != nil {
		t.Fatal("el server de test no debería tener ledger; el test perdió su sentido")
	}
	_, ch, _ := s.live.subscribe("", false)
	s.registrarUso(context.Background(), "musubi_recall", "ok", 7*time.Millisecond)

	select {
	case ev := <-ch:
		if ev.Tool != "musubi_recall" || ev.Outcome != "ok" {
			t.Fatalf("evento inesperado: %+v", ev)
		}
		if ev.DurationMs != 7 {
			t.Fatalf("ms = %v, esperaba 7", ev.DurationMs)
		}
		if ev.At == "" {
			t.Fatal("el evento salió sin hora; la hora real es justamente lo que la base no puede dar")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("el feed no publicó con el ledger apagado")
	}
}

// L-VIVO-8 · /api/stream exige credencial con la misma regla que /metrics.
func TestStreamExigeCredencial(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	reg := &PrincipalRegistry{principals: []Principal{
		{Name: "cabina", Role: RoleReader, Read: ReadAll, Write: WriteNone, hash: hashToken("buen-token")},
	}}
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, registry: reg}))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/stream")
	if err != nil {
		t.Fatalf("GET /api/stream: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("/api/stream sin credencial dio %d, esperaba 401", resp.StatusCode)
	}
}

// L-VIVO-9 · el camino completo por HTTP: backlog primero, después los eventos en vivo.
func TestStreamEmiteBacklogYLuegoEnVivo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, loopbackOnly: true}))
	defer ts.Close()

	// Un evento ANTES de conectarse: tiene que venir en el backlog.
	s.live.publish(LiveEvent{Tool: "musubi_ingest_url", Outcome: "ok", Kind: KindTrabajo})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/stream", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/stream: %v", err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("Content-Type = %q, esperaba text/event-stream", ct)
	}

	sc := bufio.NewScanner(resp.Body)
	leerFrame := func() (evento, data string) {
		t.Helper()
		for sc.Scan() {
			l := sc.Text()
			switch {
			case strings.HasPrefix(l, "event: "):
				evento = strings.TrimPrefix(l, "event: ")
			case strings.HasPrefix(l, "data: "):
				data = strings.TrimPrefix(l, "data: ")
			case l == "" && evento != "":
				return evento, data
			}
		}
		t.Fatalf("el stream se cortó antes de completar un frame (err=%v)", sc.Err())
		return "", ""
	}

	ev, data := leerFrame()
	if ev != "backlog" {
		t.Fatalf("primer frame = %q, esperaba backlog", ev)
	}
	var back []LiveEvent
	if err := json.Unmarshal([]byte(data), &back); err != nil {
		t.Fatalf("backlog no parsea: %v (%s)", err, data)
	}
	if len(back) != 1 || back[0].Tool != "musubi_ingest_url" {
		t.Fatalf("backlog = %+v, esperaba el evento previo", back)
	}

	// Ya conectados: lo que se publique ahora tiene que llegar en vivo.
	s.live.publish(LiveEvent{Tool: "musubi_judge", Outcome: "ok", Kind: KindTrabajo})
	ev2, data2 := leerFrame()
	if ev2 != "uso" {
		t.Fatalf("segundo frame = %q, esperaba uso", ev2)
	}
	var vivo LiveEvent
	if err := json.Unmarshal([]byte(data2), &vivo); err != nil {
		t.Fatalf("evento no parsea: %v (%s)", err, data2)
	}
	if vivo.Tool != "musubi_judge" {
		t.Fatalf("llegó %q, esperaba musubi_judge", vivo.Tool)
	}
}

// L-VIVO-10 · ?kind=trabajo saca el sondeo del cable, no sólo de la vista.
//
// Importa por bytes, no por estética: medido sobre 24 h, el sondeo es el 99,92% de las
// invocaciones locales y el 76% de las del central. Filtrarlo en el navegador significaría
// mandar todo eso por el tailnet para tirarlo al llegar.
func TestStreamPuedeDejarFueraElSondeo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, loopbackOnly: true}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/stream?kind=trabajo", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/stream: %v", err)
	}
	defer resp.Body.Close()

	sc := bufio.NewScanner(resp.Body)
	leerFrame := func() (evento, data string) {
		t.Helper()
		for sc.Scan() {
			l := sc.Text()
			switch {
			case strings.HasPrefix(l, "event: "):
				evento = strings.TrimPrefix(l, "event: ")
			case strings.HasPrefix(l, "data: "):
				data = strings.TrimPrefix(l, "data: ")
			case l == "" && evento != "":
				return evento, data
			}
		}
		t.Fatalf("el stream se cortó (err=%v)", sc.Err())
		return "", ""
	}
	if ev, _ := leerFrame(); ev != "backlog" {
		t.Fatalf("primer frame = %q, esperaba backlog", ev)
	}

	// Sondeo primero, trabajo después: si el filtro no existiera, el primero en llegar sería el
	// sondeo y el test lo cazaría.
	s.live.publish(LiveEvent{Tool: "musubi_sync_pull", Kind: KindSondeo})
	s.live.publish(LiveEvent{Tool: "musubi_recall", Kind: KindTrabajo})

	_, data := leerFrame()
	var ev LiveEvent
	if err := json.Unmarshal([]byte(data), &ev); err != nil {
		t.Fatalf("evento no parsea: %v", err)
	}
	if ev.Tool != "musubi_recall" {
		t.Fatalf("llegó %q; con kind=trabajo el sondeo no tiene que salir al cable", ev.Tool)
	}
}
