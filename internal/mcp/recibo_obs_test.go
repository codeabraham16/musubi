package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// recibo_obs_test.go custodia el RECIBO del push de observaciones (E3): el 200 ya no alcanza. El
// central, cuando el cliente le pide recibo (manda huella), contesta {stored, hash, transformed}; el
// cliente lo lee y decide si cerró en nil, si avisa una transformación legítima, o si dead-lettea. La
// asimetría que gobierna TODO el archivo: tirar memoria es irreversible, reintentar es barato — así
// que sólo se dead-lettea ante señal CLARA (un drop, o una corrupción que dice ser verbatim).

// TestPushLeeElReciboDeLaObs recorre las cinco formas de respuesta que el cliente tiene que saber
// distinguir. Mira SÓLO la reacción del cliente: el central es un stub que devuelve el recibo tal
// cual se lo pongo, así cada rama del switch de Push queda cubierta aislada.
//
// Sabotaje que la pone roja: desconectar la lectura del recibo (el 200 vuelve a alcanzar).
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="if rec, ok := leerReciboDeObs(result); ok {"
// arnes: a="if rec, ok := leerReciboDeObs(result); ok && false {"
//
// Sabotaje que la pone roja: tratar una redacción legítima como corrupción (romper la rama
// transformed, con la que la nota redactada NO se dead-lettea).
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="case rec.Transformed:"
// arnes: a="case rec.Transformed && false:"
func TestPushLeeElReciboDeLaObs(t *testing.T) {
	var texto atomic.Value
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// resultadoDelPush envuelve el texto en result.content[0].text, que es como el save real
		// devuelve su JSON; el `id` del sobre lo ignora el cliente, así que da igual que diga "push".
		resultadoDelPush(w, texto.Load().(string))
	}))
	t.Cleanup(ts.Close)
	c := newTestSyncClient(t, ts.URL)

	item := testItem()
	// H es la huella que el cliente manda y contra la que compara: ContentHash del contenido crudo.
	H := memory.ContentHash(item.Content)

	casos := []struct {
		nombre, recibo, frag string
		cuadra               bool // true ⇒ Push cierra en nil; false ⇒ dead-letter permanente
	}{
		{
			nombre: "verbatim: lo guardado es lo mandado",
			recibo: fmt.Sprintf(`{"mensaje":"ok","stored":true,"hash":%q,"transformed":false}`, H),
			cuadra: true,
		},
		{
			nombre: "transformada: el central redactó, la nota viva es la suya, NO se dead-lettea",
			recibo: `{"mensaje":"ok","stored":true,"hash":"h-redactada-por-el-central","transformed":true}`,
			cuadra: true,
		},
		{
			nombre: "central anterior al recibo: texto plano, no es un recibo ⇒ se confía en el 200",
			recibo: "Observación guardada con éxito (id: obs-123).",
			cuadra: true,
		},
		{
			nombre: "drop silencioso: 200 pero la fila no quedó bajo ese id",
			recibo: `{"mensaje":"ok","stored":false,"hash":"","transformed":false}`,
			frag:   "no persistió",
			cuadra: false,
		},
		{
			nombre: "corrupción: dice verbatim y aun así el hash no es el que mandé",
			recibo: `{"mensaje":"ok","stored":true,"hash":"h-que-nunca-mande","transformed":false}`,
			frag:   "hash no coincide",
			cuadra: false,
		},
	}
	for _, caso := range casos {
		texto.Store(caso.recibo)
		err := c.Push(item)
		switch {
		case caso.cuadra && err != nil:
			t.Errorf("%s: tenía que cerrar en nil y dio %v", caso.nombre, err)
		case !caso.cuadra && !errors.Is(err, errReciboNoCuadra):
			t.Errorf("%s: tenía que rechazar con errReciboNoCuadra, dio %v", caso.nombre, err)
		case !caso.cuadra && !errors.Is(err, errPermanent):
			t.Errorf("%s: el rechazo tiene que ser PERMANENTE (dead-letter recuperable), dio %v", caso.nombre, err)
		case !caso.cuadra && !strings.Contains(err.Error(), caso.frag):
			t.Errorf("%s: el error tiene que decir %q, dijo %v", caso.nombre, caso.frag, err)
		}
	}
}

// TestPushMandaLaHuellaDeLaObs ancla que la huella VIAJA en el pedido (RE1): es la presencia de la
// huella lo que le pide recibo al central, y su valor es ContentHash del contenido crudo, que el
// central recomputa para comparar. El payload de testItem pesa < umbralCompresionSubida (512), así
// que viaja sin comprimir y el cuerpo se lee crudo.
//
// Sabotaje que la pone roja: no mandar la huella.
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="Huella:     huella,"
// arnes: a="Huella:     \"\","
func TestPushMandaLaHuellaDeLaObs(t *testing.T) {
	item := testItem()
	H := memory.ContentHash(item.Content)
	var vista atomic.Value
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Params struct {
				Arguments struct {
					Huella string `json:"huella"`
				} `json:"arguments"`
			} `json:"params"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		vista.Store(req.Params.Arguments.Huella)
		// Contesto verbatim para que Push cierre en nil: el foco del test es lo que VIAJÓ.
		resultadoDelPush(w, fmt.Sprintf(`{"mensaje":"ok","stored":true,"hash":%q,"transformed":false}`, H))
	}))
	t.Cleanup(ts.Close)
	c := newTestSyncClient(t, ts.URL)

	if err := c.Push(item); err != nil {
		t.Fatalf("Push con recibo verbatim tenía que cerrar en nil, dio %v", err)
	}
	got, _ := vista.Load().(string)
	if got == "" || got != H {
		t.Fatalf("el push no llevó la huella del contenido: mandó %q, esperaba %q", got, H)
	}
}

// TestElReciboDeLaObsConfirmaContraLaBaseYMarcaLaRedaccion mira el OTRO lado: que el central arme el
// recibo leyendo la fila ALMACENADA (read-back por id), no el valor de retorno del save. Y la
// decisión más delicada (D3): `transformed` se compara contra el contenido CRUDO que recibió, nunca
// contra el ya redactado — comparar contra el redactado haría que una redacción legítima pareciera
// verbatim-pero-con-hash-distinto, o sea CORRUPCIÓN, y el cliente la mandaría a dead-letter: el
// pecado capital que este recibo existe para evitar.
//
// Sabotaje que la pone roja: comparar `transformed` contra el contenido YA redactado (D3 roto).
// Apunta a la rama de UPSERT POR id (la del `args.ID`), porque esta prueba —y el cliente de sync
// real— SIEMPRE mandan id, así que el handler toma ese camino y nunca la rama auto-UUID de al lado.
// arnes: archivo="internal/mcp/methods.go"
// arnes: de="res = s.resultadoDeGuardado(args.Huella, args.ID, args.Content, \"Observación guardada con éxito (id: \"+args.ID+\").\"+s.detectAndSurface(args.ID))"
// arnes: a="res = s.resultadoDeGuardado(args.Huella, args.ID, content, \"Observación guardada con éxito (id: \"+args.ID+\").\"+s.detectAndSurface(args.ID))"
func TestElReciboDeLaObsConfirmaContraLaBaseYMarcaLaRedaccion(t *testing.T) {
	// forceRedact=true ⇒ el server es infra compartida: redacta los secretos y fuerza scope=shared.
	// Con eso, el MISMO camino de guardado deja la nota limpia tal cual (transformed=false) y la nota
	// con secreto redactada (transformed=true): el bit depende sólo de si el contenido cambió.
	reciboDelHandler := func(t *testing.T, id, content string, conHuella bool) (reciboDeObs, bool) {
		t.Helper()
		s := newTestServer(t, embedding.NoopProvider{})
		s.pozoCompartido = true
		args := map[string]any{"id": id, "topic_key": "cfg/x", "content": content, "scope": "shared"}
		if conHuella {
			args["huella"] = memory.ContentHash(content)
		}
		raw, _ := json.Marshal(args)
		res, rpcErr := s.toolSaveObservation(context.Background(), raw)
		if rpcErr != nil {
			t.Fatalf("save: %+v", rpcErr)
		}
		envoltura, _ := json.Marshal(res)
		return leerReciboDeObs(envoltura)
	}

	t.Run("verbatim ⇒ stored, transformed=false, hash el de lo guardado", func(t *testing.T) {
		const content = "una nota reusable sin ningún secreto"
		rec, ok := reciboDelHandler(t, "obs-limpia", content, true)
		if !ok {
			t.Fatal("con huella el handler tiene que devolver un recibo legible")
		}
		if rec.Stored == nil || !*rec.Stored {
			t.Errorf("stored tiene que ser true para una nota recién guardada, dio %v", rec.Stored)
		}
		if rec.Transformed {
			t.Errorf("sin redacción el contenido no cambió: transformed tiene que ser false")
		}
		if want := memory.ContentHash(content); rec.Hash != want {
			t.Errorf("hash=%q, esperaba el de la fila guardada %q", rec.Hash, want)
		}
	})

	t.Run("redacción legítima ⇒ stored, transformed=true (no es un drop ni corrupción)", func(t *testing.T) {
		const secreto = "AKIA1234567890ABCDEF" // matchea aws-access-key, se redacta con forceRedact
		const content = "config " + secreto + " fin"
		rec, ok := reciboDelHandler(t, "obs-con-secreto", content, true)
		if !ok {
			t.Fatal("con huella el handler tiene que devolver un recibo legible")
		}
		if rec.Stored == nil || !*rec.Stored {
			t.Errorf("una redacción NO es un drop: stored tiene que ser true, dio %v", rec.Stored)
		}
		if !rec.Transformed {
			t.Errorf("el central redactó el secreto: transformed TIENE que ser true, o el cliente dead-lettea una nota viva")
		}
		if rec.Hash == memory.ContentHash(content) {
			t.Errorf("el hash del recibo tiene que ser el de lo GUARDADO (redactado), no el del crudo con el secreto")
		}
	})

	t.Run("sin huella ⇒ texto plano, no recibo (opt-in D4: camino interactivo intacto)", func(t *testing.T) {
		_, ok := reciboDelHandler(t, "obs-sin-huella", "una nota cualquiera", false)
		if ok {
			t.Errorf("sin huella la respuesta tiene que quedar byte-idéntica a la de antes del recibo: texto plano, no recibo")
		}
	})
}

// TestUnaRedaccionDelCentralNoMandaLaNotaADeadLetter cierra el lazo de punta a punta, con el cliente
// real contra un central real: una nota con un secreto se empuja, el central la redacta, y el push
// cierra en nil —NO dead-letter—. Es el capstone del pecado capital: la foto de los dos lados juntos.
func TestUnaRedaccionDelCentralNoMandaLaNotaADeadLetter(t *testing.T) {
	central := newTestServer(t, embedding.NoopProvider{})
	central.pozoCompartido = true
	ts := httptest.NewServer(central.HTTPHandler(httpOptions{reqTimeout: 30 * time.Second}))
	t.Cleanup(ts.Close)
	c := newTestSyncClient(t, ts.URL)

	const secreto = "AKIA1234567890ABCDEF"
	item := testItem()
	item.ObsID = "obs-con-secreto"
	item.Content = "config " + secreto + " fin"

	if err := c.Push(item); err != nil {
		t.Fatalf("una redacción legítima del central NO puede ir a dead-letter; Push dio %v", err)
	}
	// Y lo que quedó en el central es la versión redactada, no el crudo con el secreto.
	hash, found, err := central.engine.ContentHashDe(item.ObsID)
	if err != nil || !found {
		t.Fatalf("la nota no quedó en el central: found=%v err=%v", found, err)
	}
	if hash == memory.ContentHash(item.Content) {
		t.Errorf("el central guardó el crudo con el secreto en vez de una redacción (hash idéntico al del contenido mandado)")
	}
}

// TestUnContenidoConUTF8InvalidoNoVaADeadLetter: un Content con bytes UTF-8 inválidos (que el
// transporte JSON coacciona a U+FFFD) NO puede ir a dead-letter. No es un secreto ni corrupción: es
// una nota viva no-representable en JSON (p.ej. un mensaje de commit, que git no garantiza en UTF-8,
// capturado en team mode). La huella se calcula sobre lo que VIAJA, así que coincide con lo que el
// central recomputa sobre el contenido recibido, y el push cierra en nil. Sin el arreglo, el cliente
// hashea los bytes crudos, el central hashea U+FFFD, transformed=false, hash≠huella ⇒ default ⇒
// dead-letter permanente (requeue reenvía los mismos bytes y reproduce el mismatch).
//
// Sabotaje que la pone roja: calcular la huella sobre los bytes crudos (revertir el arreglo).
// arnes: archivo="internal/mcp/syncclient.go"
// arnes: de="huella := memory.ContentHash(contenidoQueViaja(item.Content))"
// arnes: a="huella := memory.ContentHash(item.Content)"
func TestUnContenidoConUTF8InvalidoNoVaADeadLetter(t *testing.T) {
	central := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(central.HTTPHandler(httpOptions{reqTimeout: 30 * time.Second}))
	t.Cleanup(ts.Close)
	c := newTestSyncClient(t, ts.URL)

	item := testItem()
	item.ObsID = "obs-utf8-invalido"
	item.Content = "nota con un byte invalido \xff y sigue"

	if err := c.Push(item); err != nil {
		t.Fatalf("un contenido no-representable en JSON (UTF-8 inválido) NO es corrupción: Push tenía que cerrar en nil, dio %v", err)
	}
	// Y la fila quedó: lo que el central guardó es la forma que viaja (U+FFFD), y la huella coincide.
	hash, found, err := central.engine.ContentHashDe(item.ObsID)
	if err != nil || !found {
		t.Fatalf("la nota con UTF-8 inválido no quedó en el central: found=%v err=%v", found, err)
	}
	if want := memory.ContentHash(contenidoQueViaja(item.Content)); hash != want {
		t.Errorf("el central guardó un hash %q que no es el de la forma que viaja %q", hash, want)
	}
}
