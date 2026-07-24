// Package ingest le da a Musubi la capacidad de INGERIR un link (video, red social, artículo) y
// convertirlo en texto utilizable — para leerlo al vuelo y/o guardarlo en memoria como conocimiento
// durable del cerebro. La cascada es model-free por defecto: primero el texto barato y sin modelo
// (artículo con go-trafilatura in-process; subtítulos existentes vía yt-dlp). La transcripción de
// audio (Whisper, modelo LOCAL opcional) es una fase posterior; el binario base nunca llama a un
// LLM/ASR externo ni de pago. Ver SDD sdd/ingesta-de-links.
package ingest

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// TranscriptSource registra de dónde salió el texto de un Result.
const (
	SourceCaptions    = "captions"    // subtítulos existentes del video
	SourceWhisper     = "whisper"     // transcripción local del audio (fase F2)
	SourceReadability = "readability" // texto principal de un artículo
	SourceNone        = "none"        // no se pudo extraer texto (degradación/bloqueo)
)

// Kind clasifica el contenido ingerido.
const (
	KindArticle = "article"
	KindVideo   = "video"
	KindSocial  = "social"
)

// DefaultLangs es la preferencia de idioma de subtítulos cuando el caller no especifica.
var DefaultLangs = []string{"es", "en"}

// Result es el producto de ingerir una URL. Serializable a JSON para la salida --json y para
// persistir en memoria (F1).
type Result struct {
	SourceURL        string `json:"source_url"`
	Platform         string `json:"platform"`
	ID               string `json:"id,omitempty"` // id nativo de la plataforma (ej. video_id de YouTube); "" para artículos
	Kind             string `json:"kind"`
	Title            string `json:"title,omitempty"`
	Author           string `json:"author,omitempty"`
	PublishedAt      string `json:"published_at,omitempty"`
	Lang             string `json:"lang,omitempty"`
	DurationSeconds  int    `json:"duration_seconds,omitempty"`
	Text             string `json:"text"`
	TranscriptSource string `json:"transcript_source"`
	Note             string `json:"note,omitempty"`     // aviso accionable (degradación, bloqueo por cookies, etc.)
	SavedID          string `json:"saved_id,omitempty"` // id de la observación si se persistió (--save)
}

// Options modula una ingesta.
type Options struct {
	Langs              []string // preferencia de idioma para subtítulos (en orden)
	CookiesFromBrowser string   // navegador para --cookies-from-browser (IG/FB/X)
	CookiesFile        string   // archivo de cookies Netscape
	ForceKind          string   // KindArticle|KindVideo para forzar la ruta (R2); "" = auto
}

// Extractor extrae contenido de las URLs que sabe manejar.
type Extractor interface {
	Name() string
	Match(u *url.URL) bool
	Extract(ctx context.Context, rawURL string, opts Options) (Result, error)
}

// Registry elige el extractor adecuado para una URL y delega. media puede ser nil cuando yt-dlp no
// está instalado; article es el fallback universal y siempre está presente (go-trafilatura va dentro
// del binario).
type Registry struct {
	media   Extractor
	article Extractor
}

// NewRegistry arma el registro. media puede ser nil (degradación elegante, R7).
func NewRegistry(media, article Extractor) *Registry {
	return &Registry{media: media, article: article}
}

// Extract clasifica la URL (R2) y delega en el extractor correcto. La degradación es BLANDA: si la
// URL es de video pero falta yt-dlp, devuelve un Result con TranscriptSource=none y un aviso
// accionable en Note, no un error duro (R7).
func (r *Registry) Extract(ctx context.Context, rawURL string, opts Options) (Result, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Scheme == "" || u.Host == "" {
		return Result{}, fmt.Errorf("URL inválida: %q", rawURL)
	}

	wantMedia := opts.ForceKind == KindVideo || (opts.ForceKind == "" && IsMediaHost(u))
	if opts.ForceKind == KindArticle {
		wantMedia = false
	}

	if wantMedia {
		if r.media == nil {
			return Result{
				SourceURL:        rawURL,
				Platform:         platformForHost(u.Host),
				Kind:             KindVideo,
				TranscriptSource: SourceNone,
				Note:             "esta URL es de video pero yt-dlp no está instalado — instalalo con `uv tool install yt-dlp` o `musubi provision`",
			}, nil
		}
		return r.media.Extract(ctx, rawURL, opts)
	}
	return r.article.Extract(ctx, rawURL, opts)
}

// firstNonEmpty devuelve el primer string no vacío (tras trim).
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
