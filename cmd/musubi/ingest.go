package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/ingest"
	"musubi/internal/memory"
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
		save        = fs.Bool("save", false, "persistir el texto en la memoria del cerebro (default: solo mostrar)")
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

	// --save (R6): persistir el texto en memoria. "las dos cosas": por defecto solo se muestra; con
	// --save además se guarda como conocimiento durable del cerebro (idempotente por URL/id).
	if *save {
		if strings.TrimSpace(res.Text) == "" {
			fmt.Fprintln(os.Stderr, "ingest: no hay texto para guardar (--save omitido)")
		} else if id, serr := saveIngest(res); serr != nil {
			fmt.Fprintf(os.Stderr, "ingest: no pude guardar en memoria: %v\n", serr)
		} else {
			res.SavedID = id
		}
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

// saveIngest persiste el Result en la memoria local del cerebro y devuelve el id de la observación.
// El id/topic_key son DETERMINÍSTICOS (ver ingest.PersistKey): re-ingerir la misma URL UPSERTEA la
// misma fila en vez de duplicar (idempotencia, R10). Estampa el embedding si la semántica está
// encendida, igual que la captura de commits, para que participe del recall semántico.
func saveIngest(res ingest.Result) (string, error) {
	root := workspaceDir()
	if err := ensureWorkspace(root); err != nil {
		return "", err
	}
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		return "", err
	}
	defer engine.Close()

	topicKey, obsID := ingest.PersistKey(res)
	content := ingest.RenderForMemory(res)

	var vec []float32
	cfg, _ := config.Load(root)
	embedder := resolveEmbedder(cfg, root)
	if embedding.Enabled(embedder) {
		engine.SetVectorModelID(embedder.Name())
		ectx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if v, eerr := embedder.Embed(ectx, content); eerr == nil {
			vec = v
		}
	}

	// Scope: local por defecto; en team mode viaja al cerebro central (como la captura de commits).
	scope := memory.ScopeLocal
	if cfg.Memory.TeamMode {
		scope = memory.ScopeShared
	}
	if err := engine.SaveObservationTyped(obsID, topicKey, content, 1.0, "semantic", scope, vec); err != nil {
		return "", err
	}
	return obsID, nil
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
	if w := ingest.FindWhisper(); w.Available() {
		fmt.Printf("  %-16s %s\n", "audio→texto", cGreen("whisper.cpp — "+w.Bin))
	} else {
		fmt.Printf("  %-16s %s\n", "audio→texto", cYellow("whisper.cpp NO configurado (definí MUSUBI_WHISPER_BIN y MUSUBI_WHISPER_MODEL)"))
	}
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
	if r.SavedID != "" {
		fmt.Println(cGreen("✔ guardado en memoria: " + r.SavedID))
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
