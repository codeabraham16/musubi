package mcp

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// LA SUBIDA VIAJA COMPRIMIDA, Y SYNC_VIAJES DICE CUÁNTO SE AHORRÓ.
//
// Push comprime el POST de una nota por encima de umbralCompresionSubida. Estas pruebas corren
// contra el handler HTTP REAL del central —el mismo readRequestBody que atiende en producción— y
// miran lo que se rompería sin que nadie lo viera: que el central guarde otra cosa que la nota, que
// la cabecera no viaje (el central leería el gzip como JSON roto y la nota iría a dead-letter), que
// una nota chica se comprima igual, y que sync_viajes confunda el cable con el crudo.

// postVisto es un POST tal como llegó al central.
type postVisto struct {
	encoding string
	cable    int // lo que cruzó la red
	crudo    int // el JSON que lee el handler, ya descomprimido
	args     syncSaveArguments
	rebotado bool
}

// centralQueMira pone delante del handler real un mirador que anota cada POST. A los ids de
// `rebotar` les contesta 503 las primeras N veces sin dejarlos pasar, que es lo que hace el proxy
// del tailnet cuando el central no atiende. Todo lo demás sigue al handler real con el cuerpo
// intacto, comprimido si llegó comprimido.
type centralQueMira struct {
	t       *testing.T
	real    http.Handler
	mu      sync.Mutex
	rebotar map[string]int
	vistos  map[string][]postVisto
}

func (c *centralQueMira) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	cable, err := io.ReadAll(r.Body)
	if err != nil {
		c.t.Errorf("leer el POST: %v", err)
		return
	}
	p := postVisto{encoding: r.Header.Get("Content-Encoding"), cable: len(cable)}
	cuerpo := cable
	if p.encoding == "gzip" {
		zr, zerr := gzip.NewReader(bytes.NewReader(cable))
		if zerr != nil {
			c.t.Errorf("el POST dijo gzip y no lo era: %v", zerr)
			return
		}
		if cuerpo, err = io.ReadAll(zr); err != nil {
			c.t.Errorf("descomprimir el POST: %v", err)
			return
		}
	}
	p.crudo = len(cuerpo)
	var req syncRPCRequest
	_ = json.Unmarshal(cuerpo, &req) // un cuerpo ilegible queda con id vacío: lo denuncia la prueba
	p.args = req.Params.Arguments

	c.mu.Lock()
	id := p.args.ID
	if c.rebotar[id] > 0 {
		c.rebotar[id]--
		p.rebotado = true
	}
	c.vistos[id] = append(c.vistos[id], p)
	c.mu.Unlock()

	if p.rebotado {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(cable))
	c.real.ServeHTTP(w, r)
}

// visto devuelve los POST que llegaron para un id, en orden.
func (c *centralQueMira) visto(id string) []postVisto {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]postVisto(nil), c.vistos[id]...)
}

// subidaContraCentralReal arma un central real detrás del mirador y un nodo cuyo sync le apunta.
func subidaContraCentralReal(t *testing.T, rebotar map[string]int) (nodo *McpServer, anota *engineQueAnotaViajes, mirador *centralQueMira, central *memory.DbEngine) {
	t.Helper()
	central, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { central.Close() })
	srv := NewMcpServer(central, t.TempDir(), embedding.NoopProvider{})
	mirador = &centralQueMira{t: t, real: srv.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second}),
		rebotar: rebotar, vistos: map[string][]postVisto{}}
	ts := httptest.NewServer(mirador)
	t.Cleanup(ts.Close)
	nodo, anota = serverQueAnotaViajes(t, ts.URL, false)
	return nodo, anota, mirador, central
}

// textoDeNota arma un contenido con la forma de una nota real: frases que se parecen sin repetirse,
// del largo de la nota media del central (~2,6 KB con 25 frases).
func textoDeNota(tema string, frases int) string {
	var b strings.Builder
	for i := 0; i < frases; i++ {
		fmt.Fprintf(&b, "%s, paso %d: el drain reclamó %d filas, el central tardó %d ms en embeber y quedaron %d pendientes. ",
			tema, i, (i*7)%50, 1500+i*37, (i*13)%9)
	}
	return b.String()
}

// guardarCompartidas guarda cada nota 'shared' en el nodo, que la encola en el outbox.
func guardarCompartidas(t *testing.T, nodo *McpServer, notas map[string]string) {
	t.Helper()
	for id, texto := range notas {
		if err := nodo.engine.SaveObservationTyped(id, "t/subida", texto, 1, "semantic", memory.ScopeShared, nil); err != nil {
			t.Fatalf("guardar %s: %v", id, err)
		}
	}
}

// TestLaSubidaComprimidaLlegaEnteraAlCentralReal: una nota del tamaño de la media viaja con
// Content-Encoding: gzip y el central guarda EXACTAMENTE su texto; una nota chica viaja en claro,
// sin cabecera. Las dos quedan enviadas.
//
// Sabotaje que la pone roja: que ninguna nota se comprima.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="if len(payload) > umbralCompresionSubida {"
// arnes: a="if false && len(payload) > umbralCompresionSubida {"
//
// Sabotaje que la pone roja: que el cuerpo viaje comprimido y la cabecera no.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="if viajaComprimida {"
// arnes: a="if false && viajaComprimida {"
//
// Sabotaje que la pone roja: comprimir también las notas chicas.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="const umbralCompresionSubida = 512"
// arnes: a="const umbralCompresionSubida = 0"
func TestLaSubidaComprimidaLlegaEnteraAlCentralReal(t *testing.T) {
	nodo, _, mirador, central := subidaContraCentralReal(t, nil)
	notas := map[string]string{
		"grande": textoDeNota("grande", 25),
		"chica":  "una nota corta que no vale la pena comprimir",
	}
	guardarCompartidas(t, nodo, notas)

	nodo.drainOutboxOnce(context.Background())

	grande, chica := mirador.visto("grande"), mirador.visto("chica")
	if len(grande) != 1 || len(chica) != 1 {
		t.Fatalf("esperaba un POST por nota; llegaron %d de la grande y %d de la chica (un cuerpo ilegible llega sin id)", len(grande), len(chica))
	}
	if g := grande[0]; g.encoding != "gzip" || g.cable >= g.crudo {
		t.Errorf("la nota grande (%d B de JSON) viajó con Content-Encoding %q y %d B en el cable; esperaba gzip y menos bytes que el JSON", g.crudo, g.encoding, g.cable)
	}
	if c := chica[0]; c.crudo > umbralCompresionSubida || c.encoding != "" || c.cable != c.crudo {
		t.Errorf("la nota chica (%d B de JSON, umbral %d) viajó con Content-Encoding %q y %d B en el cable; esperaba en claro", c.crudo, umbralCompresionSubida, c.encoding, c.cable)
	}

	// EL CENTRAL GUARDÓ LA NOTA, NO OTRA COSA. Es la mitad que la cabecera no prueba: un gzip
	// mal armado o un cuerpo cortado llegan con la cabecera puesta igual.
	obs, err := central.GetObservations([]string{"grande", "chica"})
	if err != nil {
		t.Fatal(err)
	}
	guardadas := map[string]string{}
	for _, o := range obs {
		guardadas[o.ID] = o.Content
	}
	for id, texto := range notas {
		if guardadas[id] != texto {
			t.Errorf("el central guardó para %q %d B que no son la nota (%d B)", id, len(guardadas[id]), len(texto))
		}
	}
	if p, sent, dead := statsOf(t, nodo); p != 0 || sent != 2 || dead != 0 {
		t.Errorf("outbox tras el drain: pending=%d sent=%d dead=%d; esperaba las dos enviadas", p, sent, dead)
	}
}

// TestLaSubidaComprimidaCuadraByteAByte: sync_viajes cuenta en el cable lo que CRUZÓ LA RED y en los
// crudos el JSON de la nota, y un POST rechazado suma su cable aparte. Se mide contra lo que el
// central recibió de verdad, incluido el reintento de la nota rebotada, que tiene que mandar
// exactamente los mismos bytes.
//
// Sabotaje que la pone roja: contar como crudo lo que viajó por el cable.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="c.trafico.subidaCrudos.Add(crudo)"
// arnes: a="c.trafico.subidaCrudos.Add(int64(len(payload)))\n\t_ = crudo"
//
// Sabotaje que la pone roja: contar un rechazo por su JSON y no por su cable.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="c.trafico.bytesRechazados.Add(int64(len(payload)))"
// arnes: a="c.trafico.bytesRechazados.Add(crudo)"
func TestLaSubidaComprimidaCuadraByteAByte(t *testing.T) {
	nodo, anota, mirador, central := subidaContraCentralReal(t, map[string]int{"rebotada": 1})
	guardarCompartidas(t, nodo, map[string]string{
		"grande":   textoDeNota("grande", 25),
		"rebotada": textoDeNota("rebotada", 25),
		"chica":    "una nota corta",
	})

	nodo.drainOutboxOnce(context.Background())

	grande, chica, rebotada := mirador.visto("grande"), mirador.visto("chica"), mirador.visto("rebotada")
	if len(grande) != 1 || len(chica) != 1 || len(rebotada) != 1 || !rebotada[0].rebotado {
		t.Fatalf("precondición: un POST por nota y la rebotada con 503; llegaron %d/%d/%d", len(grande), len(chica), len(rebotada))
	}
	if grande[0].encoding != "gzip" || rebotada[0].encoding != "gzip" {
		t.Fatalf("precondición: las dos grandes tenían que viajar comprimidas (%q, %q)", grande[0].encoding, rebotada[0].encoding)
	}
	viajes := anota.anotados(memory.ViajeSubida)
	if len(viajes) != 1 {
		t.Fatalf("un tick registró %d viajes de subida; esperaba uno", len(viajes))
	}
	v := viajes[0]
	quiero := memory.Viaje{
		Filas: 2, Posts: 3, Rechazados: 1,
		BytesCable:      int64(grande[0].cable + chica[0].cable),
		BytesCrudos:     int64(grande[0].crudo + chica[0].crudo),
		BytesRechazados: int64(rebotada[0].cable),
	}
	if v != quiero {
		t.Errorf("sync_viajes no cuadra con lo que recibió el central:\n  anotó  %+v\n  llegó  %+v", v, quiero)
	}

	// EL REINTENTO: la misma nota, armada con lo que el drain mandó, sale otra vez y ahora pasa.
	// Tiene que cruzar con los MISMOS bytes —gzip sin fecha en el encabezado— y contar como un POST
	// aceptado más, sin arrastrar nada del rechazo.
	a := rebotada[0].args
	antes := nodo.syncClient.foto()
	if err := nodo.syncClient.Push(memory.OutboxItem{ObsID: a.ID, TopicKey: a.TopicKey, Content: a.Content,
		Importance: a.Importance, MemType: a.MemType, ProjectID: a.ProjectID}); err != nil {
		t.Fatalf("el reintento de la rebotada falló: %v", err)
	}
	rebotada = mirador.visto("rebotada")
	if len(rebotada) != 2 || rebotada[1].rebotado {
		t.Fatalf("precondición: el reintento tenía que llegar al handler real; vistos %+v", rebotada)
	}
	if rebotada[1].cable != rebotada[0].cable || rebotada[1].crudo != rebotada[0].crudo {
		t.Errorf("el reintento cruzó %d B (%d de JSON) y el primer intento %d B (%d): la misma nota tiene que viajar igual",
			rebotada[1].cable, rebotada[1].crudo, rebotada[0].cable, rebotada[0].crudo)
	}
	reintento := nodo.syncClient.subidaDesde(antes)
	quiero = memory.Viaje{Posts: 1, BytesCable: int64(rebotada[1].cable), BytesCrudos: int64(rebotada[1].crudo)}
	if reintento != quiero {
		t.Errorf("el reintento no cuadra:\n  anotó  %+v\n  llegó  %+v", reintento, quiero)
	}
	if obs, err := central.GetObservations([]string{"rebotada"}); err != nil || len(obs) != 1 || obs[0].Content != a.Content {
		t.Errorf("el central no guardó la rebotada tras el reintento (err=%v, %d filas)", err, len(obs))
	}
}
