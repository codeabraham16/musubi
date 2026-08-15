package ingest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// fakeDoer devuelve una respuesta canned sin tocar la red.
type fakeDoer struct {
	body   string
	status int
}

func (f fakeDoer) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: f.status,
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Header:     make(http.Header),
	}, nil
}

const fixtureHTML = `<!DOCTYPE html>
<html lang="es">
<head>
  <title>El patrón orquestador — blog</title>
  <meta property="og:title" content="El patrón orquestador">
  <meta name="author" content="Ada Lovelace">
</head>
<body>
  <nav>Inicio · Sobre · Contacto</nav>
  <article>
    <h1>El patrón orquestador</h1>
    <p>El patrón orquestador-minion separa el razonamiento de la ejecución para no perder decisiones
    cuando la ventana de contexto se compacta. Un modelo fuerte piensa y delega; los subagentes
    ejecutan con contexto limpio y devuelven un artefacto concreto.</p>
    <p>La ventaja principal es que el orquestador nunca compacta su hilo, así que conserva las
    decisiones tomadas por el humano en lugar de dejar que el modelo decida solo qué borrar. Esto
    reduce las alucinaciones y baja el costo en tokens de forma notable.</p>
  </article>
  <footer>Copyright 2026</footer>
</body>
</html>`

func TestArticleExtractorSacaTextoYMetadata(t *testing.T) {
	ex := &ArticleExtractor{Client: fakeDoer{body: fixtureHTML, status: 200}}
	res, err := ex.Extract(context.Background(), "https://blog.example.com/orquestador", Options{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if res.Kind != KindArticle || res.TranscriptSource != SourceReadability {
		t.Fatalf("kind/source inesperados: %+v", res)
	}
	if !strings.Contains(res.Text, "orquestador-minion separa el razonamiento") {
		t.Fatalf("no extrajo el cuerpo del artículo; text=%q", res.Text)
	}
	if strings.Contains(res.Text, "Copyright 2026") || strings.Contains(res.Text, "Inicio · Sobre") {
		t.Fatalf("no descartó el boilerplate (nav/footer); text=%q", res.Text)
	}
	if res.Title == "" {
		t.Fatalf("esperaba un título; res=%+v", res)
	}
}

func TestArticleExtractorHTTPError(t *testing.T) {
	ex := &ArticleExtractor{Client: fakeDoer{body: "nope", status: 404}}
	if _, err := ex.Extract(context.Background(), "https://x.example/y", Options{}); err == nil {
		t.Fatal("esperaba error en HTTP 404")
	}
}

// capturingDoer guarda la request para poder mirarle las cabeceras.
type capturingDoer struct {
	req    *http.Request
	status int
}

func (c *capturingDoer) Do(r *http.Request) (*http.Response, error) {
	c.req = r
	st := c.status
	if st == 0 {
		st = 200
	}
	return &http.Response{
		StatusCode: st,
		Body:       io.NopCloser(strings.NewReader(fixtureHTML)),
		Header:     make(http.Header),
	}, nil
}

// El UA decía ser un navegador en el comentario y era lo contrario en el valor: el token
// `compatible;` + nombre de producto + `+URL` es la convención de DECLARARSE CRAWLER (formato
// Googlebot), justo lo que hacía que algunos sitios devolvieran la página vacía. Este test fija
// que el código y su comentario no vuelvan a divergir.
func TestArticleExtractorMandaUAdeNavegador(t *testing.T) {
	d := &capturingDoer{}
	ex := &ArticleExtractor{Client: d}
	if _, err := ex.Extract(context.Background(), "https://blog.example.com/x", Options{}); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	ua := d.req.Header.Get("User-Agent")
	if ua == "" {
		t.Fatal("no mandó User-Agent")
	}
	for _, marca := range []string{"Musubi", "compatible;", "+http"} {
		if strings.Contains(ua, marca) {
			t.Errorf("el UA se declara automatismo (contiene %q): %s", marca, ua)
		}
	}
	if !strings.Contains(ua, "Chrome/") || !strings.Contains(ua, "AppleWebKit/") {
		t.Errorf("el UA no parece de un navegador real: %s", ua)
	}
}

// Un bloqueo y un enlace roto NO son lo mismo para quien decide si reintentar. Antes los dos
// salían con el mismo texto ("la página respondió HTTP %d") y el agente no tenía cómo
// distinguirlos: reintentaba el que nunca iba a andar y abandonaba el que sí.
func TestArticleExtractorDistingueBloqueoDeNoExiste(t *testing.T) {
	msg := func(status int) string {
		ex := &ArticleExtractor{Client: fakeDoer{body: "x", status: status}}
		_, err := ex.Extract(context.Background(), "https://x.example/y", Options{})
		if err == nil {
			t.Fatalf("HTTP %d: esperaba error", status)
		}
		return err.Error()
	}

	bloqueo, noExiste, tasa, servidor := msg(403), msg(404), msg(429), msg(503)

	if bloqueo == noExiste {
		t.Errorf("403 y 404 dan el mismo error: %q", bloqueo)
	}
	// Cada uno tiene que traer su pista accionable, que es el punto del cambio.
	if !strings.Contains(bloqueo, "Reintentar igual no cambia nada") {
		t.Errorf("el 403 no dice que reintentar no sirve: %q", bloqueo)
	}
	if !strings.Contains(tasa, "Esperar") {
		t.Errorf("el 429 no dice que hay que esperar: %q", tasa)
	}
	if !strings.Contains(noExiste, "Revisar la URL") {
		t.Errorf("el 404 no manda a revisar la URL: %q", noExiste)
	}
	if !strings.Contains(servidor, "transitoria") {
		t.Errorf("el 5xx no se marca como transitorio: %q", servidor)
	}
	// Y todos tienen que nombrar la URL: sin eso, con varias ingestas en vuelo no se sabe cuál falló.
	for _, m := range []string{bloqueo, noExiste, tasa, servidor} {
		if !strings.Contains(m, "https://x.example/y") {
			t.Errorf("el error no nombra la URL: %q", m)
		}
	}
}

// countingRC cuenta cuántos bytes se consumieron del body subyacente.
type countingRC struct {
	r io.Reader
	n *int64
}

func (c countingRC) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	*c.n += int64(n)
	return n, err
}
func (countingRC) Close() error { return nil }

type countingDoer struct {
	body io.Reader
	read *int64
}

func (d countingDoer) Do(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: 200, Body: countingRC{r: d.body, n: d.read}, Header: make(http.Header)}, nil
}

// REGRESIÓN (auditoría 2026-07-26, #14): article.Extract pasaba resp.Body directo a trafilatura SIN
// tope de tamaño. Una URL pública que streamea GBs => OOM/DoS en el cerebro central always-on. El fix
// envuelve el body en io.LimitReader(maxArticleBytes): acá probamos que NO se consumen más bytes que el tope.
func TestArticleExtractorCapsBodySize(t *testing.T) {
	var read int64
	big := strings.NewReader(fixtureHTML + strings.Repeat("x", int(maxArticleBytes)+(2<<20)))
	ex := &ArticleExtractor{Client: countingDoer{body: big, read: &read}}
	_, _ = ex.Extract(context.Background(), "https://blog.example.com/big", Options{})
	if read > maxArticleBytes+4096 {
		t.Fatalf("el body no se acotó: se leyeron %d bytes (tope %d)", read, int64(maxArticleBytes))
	}
}
