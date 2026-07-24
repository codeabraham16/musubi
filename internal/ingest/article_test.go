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
