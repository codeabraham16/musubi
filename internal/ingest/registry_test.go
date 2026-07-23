package ingest

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

// stubExtractor marca en el Result qué extractor corrió (vía Platform).
type stubExtractor struct{ tag string }

func (s stubExtractor) Name() string        { return s.tag }
func (s stubExtractor) Match(*url.URL) bool { return true }
func (s stubExtractor) Extract(_ context.Context, rawURL string, _ Options) (Result, error) {
	return Result{SourceURL: rawURL, Platform: s.tag}, nil
}

func TestRegistryRuteaVideoAMedia(t *testing.T) {
	reg := NewRegistry(stubExtractor{"MEDIA"}, stubExtractor{"ARTICLE"})
	res, err := reg.Extract(context.Background(), "https://www.youtube.com/watch?v=x", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Platform != "MEDIA" {
		t.Fatalf("una URL de YouTube debía ir a media, fue a %q", res.Platform)
	}
}

func TestRegistryRuteaArticuloPorDefecto(t *testing.T) {
	reg := NewRegistry(stubExtractor{"MEDIA"}, stubExtractor{"ARTICLE"})
	res, _ := reg.Extract(context.Background(), "https://blog.example.com/post", Options{})
	if res.Platform != "ARTICLE" {
		t.Fatalf("un blog debía ir a article, fue a %q", res.Platform)
	}
}

func TestRegistryForceArticleSobreVideo(t *testing.T) {
	reg := NewRegistry(stubExtractor{"MEDIA"}, stubExtractor{"ARTICLE"})
	res, _ := reg.Extract(context.Background(), "https://www.youtube.com/watch?v=x", Options{ForceKind: KindArticle})
	if res.Platform != "ARTICLE" {
		t.Fatalf("--as=article debía forzar article, fue a %q", res.Platform)
	}
}

func TestRegistryDegradaSinYtDlp(t *testing.T) {
	reg := NewRegistry(nil, stubExtractor{"ARTICLE"}) // media nil = yt-dlp ausente
	res, err := reg.Extract(context.Background(), "https://www.youtube.com/watch?v=x", Options{})
	if err != nil {
		t.Fatalf("degradación no debe ser error duro: %v", err)
	}
	if res.TranscriptSource != SourceNone || !strings.Contains(res.Note, "yt-dlp") {
		t.Fatalf("esperaba degradación con aviso de yt-dlp; got %+v", res)
	}
}

func TestRegistryURLInvalida(t *testing.T) {
	reg := NewRegistry(nil, stubExtractor{"ARTICLE"})
	if _, err := reg.Extract(context.Background(), "no-es-una-url", Options{}); err == nil {
		t.Fatal("esperaba error con URL inválida")
	}
}
