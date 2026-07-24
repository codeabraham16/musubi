package ingest

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/markusmobius/go-trafilatura"
)

// article.go extrae el texto principal de cualquier página web con go-trafilatura (Go puro,
// in-process, sin binarios externos). Es el fallback UNIVERSAL de la ingesta: Match siempre da true.
// 100% model-free.

// httpDoer permite inyectar el cliente HTTP en tests (sin red).
type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// ArticleExtractor implementa Extractor para artículos/páginas web.
type ArticleExtractor struct {
	Client httpDoer
}

// NewArticleExtractor arma el extractor con un cliente HTTP con timeout razonable.
func NewArticleExtractor() *ArticleExtractor {
	return &ArticleExtractor{Client: &http.Client{Timeout: 20 * time.Second}}
}

func (a *ArticleExtractor) Name() string        { return "article" }
func (a *ArticleExtractor) Match(*url.URL) bool { return true } // fallback universal

// Extract baja la página y devuelve su texto principal + metadata (title/author/fecha/idioma).
func (a *ArticleExtractor) Extract(ctx context.Context, rawURL string, opts Options) (Result, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return Result{}, fmt.Errorf("URL inválida: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Result{}, err
	}
	// Un UA de navegador evita que algunos sitios devuelvan una página vacía a los bots.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Musubi-Ingest/1.0; +https://musubi)")
	// En infra compartida usamos un cliente SSRF-safe (dialer que rechaza IPs internas incluso tras un
	// redirect / DNS-rebinding). En local (CLI) se usa el cliente inyectado (testeable).
	client := a.Client
	if opts.RestrictToPublic {
		client = safeHTTPClient(20 * time.Second)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("no pude bajar %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return Result{}, fmt.Errorf("la página respondió HTTP %d", resp.StatusCode)
	}

	res, err := trafilatura.Extract(resp.Body, trafilatura.Options{
		OriginalURL:     u,
		EnableFallback:  true, // usa Readability/DomDistiller si trafilatura no encuentra el cuerpo
		ExcludeComments: true,
	})
	if err != nil {
		return Result{}, fmt.Errorf("no pude extraer el texto: %w", err)
	}

	out := Result{
		SourceURL:        rawURL,
		Platform:         "article",
		Kind:             KindArticle,
		Text:             strings.TrimSpace(res.ContentText),
		TranscriptSource: SourceReadability,
	}
	m := res.Metadata
	out.Title = strings.TrimSpace(m.Title)
	out.Author = strings.TrimSpace(m.Author)
	out.Lang = strings.TrimSpace(m.Language)
	if !m.Date.IsZero() {
		out.PublishedAt = m.Date.Format("2006-01-02")
	}
	if out.Text == "" {
		out.TranscriptSource = SourceNone
		out.Note = "no se encontró texto principal (¿página muy dependiente de JS o sin cuerpo de artículo?)"
	}
	return out, nil
}
