package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"musubi/internal/ingest"
)

// ingest.go implementa `musubi ingest <url>`: convierte un link (video, red social, artículo) en
// texto, para leerlo al vuelo (y, en F1, guardarlo en memoria). Model-free por defecto. Ver el SDD
// sdd/ingesta-de-links.

func runIngest(args []string) {
	fs := flag.NewFlagSet("ingest", flag.ContinueOnError)
	var (
		asKind      = fs.String("as", "", "forzar la ruta: article|video (default: auto por el host)")
		lang        = fs.String("lang", "", "idioma(s) de subtítulos preferidos, coma-separados (ej. es,en)")
		asJSON      = fs.Bool("json", false, "salida en JSON")
		doctor      = fs.Bool("doctor", false, "reporta los motores de ingesta detectados y sale")
		cookiesFrom = fs.String("cookies-from-browser", "", "navegador para tomar cookies (chrome, firefox…) — para IG/FB/X")
		cookiesFile = fs.String("cookies-file", "", "archivo de cookies Netscape para yt-dlp")
	)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Uso: musubi ingest [--as article|video] [--lang es,en] [--json] [--cookies-from-browser chrome] <url>")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return
	}

	ytdlp := ingest.FindYtDlp()
	if *doctor {
		reportIngestEngines(ytdlp)
		return
	}

	rest := fs.Args()
	if len(rest) == 0 {
		fs.Usage()
		os.Exit(1)
	}
	rawURL := rest[0]

	var media ingest.Extractor
	if ytdlp != "" {
		media = ingest.NewMediaExtractor(ytdlp)
	}
	reg := ingest.NewRegistry(media, ingest.NewArticleExtractor())

	opts := ingest.Options{
		ForceKind:          normalizeKind(*asKind),
		CookiesFromBrowser: *cookiesFrom,
		CookiesFile:        *cookiesFile,
	}
	if strings.TrimSpace(*lang) != "" {
		opts.Langs = splitCommaTrim(*lang)
	}

	// Presupuesto amplio: la ruta artículo es de segundos; una descarga de subtítulos puede tardar.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	res, err := reg.Extract(ctx, rawURL, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ingest: %v\n", err)
		os.Exit(1)
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
		return
	}
	printIngestHuman(res)
}

// normalizeKind mapea el valor de --as al Kind interno; "" si no se forzó.
func normalizeKind(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "article", "articulo", "artículo", "web", "page":
		return ingest.KindArticle
	case "video", "media":
		return ingest.KindVideo
	default:
		return ""
	}
}

func splitCommaTrim(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// reportIngestEngines imprime qué motores de ingesta hay disponibles (R7).
func reportIngestEngines(ytdlp string) {
	fmt.Println(cBold("Motores de ingesta:"))
	fmt.Printf("  %-16s %s\n", "artículos", cGreen("go-trafilatura (embebido, siempre disponible)"))
	if ytdlp != "" {
		fmt.Printf("  %-16s %s\n", "video/redes", cGreen("yt-dlp — "+ytdlp))
	} else {
		fmt.Printf("  %-16s %s\n", "video/redes", cYellow("yt-dlp NO instalado (instalá con `uv tool install yt-dlp`)"))
	}
	fmt.Printf("  %-16s %s\n", "audio→texto", cYellow("whisper.cpp — pendiente (fase F2)"))
}

// printIngestHuman imprime un resumen legible + el texto extraído.
func printIngestHuman(r ingest.Result) {
	fmt.Println(cBold(firstNonEmptyStr(r.Title, r.SourceURL)))
	meta := []string{r.Platform, r.Kind}
	if r.Author != "" {
		meta = append(meta, r.Author)
	}
	if r.PublishedAt != "" {
		meta = append(meta, r.PublishedAt)
	}
	if r.DurationSeconds > 0 {
		meta = append(meta, fmt.Sprintf("%dm%02ds", r.DurationSeconds/60, r.DurationSeconds%60))
	}
	fmt.Println(cDim(strings.Join(meta, " · ")))
	fmt.Println(cDim("fuente del texto: " + r.TranscriptSource))
	if r.Note != "" {
		fmt.Println(cYellow("⚠ " + r.Note))
	}
	if r.Text != "" {
		words := len(strings.Fields(r.Text))
		fmt.Printf("%s\n\n", cDim(fmt.Sprintf("— %d palabras —", words)))
		fmt.Println(r.Text)
	}
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
