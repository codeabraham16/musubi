package ingest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// media.go implementa la ruta de video/redes vía yt-dlp (shell-out). F0: SOLO subtítulos existentes,
// sin Whisper. Si el video no tiene subtítulos, devuelve la metadata con TranscriptSource=none y un
// aviso (la transcripción de audio llega en F2). yt-dlp se detecta en runtime; si falta, el Registry
// nunca construye este extractor (degradación, R7).

// ytMeta es el subconjunto del info.json de yt-dlp que usamos.
type ytMeta struct {
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	Uploader   string  `json:"uploader"`
	Channel    string  `json:"channel"`
	UploadDate string  `json:"upload_date"` // YYYYMMDD
	Duration   float64 `json:"duration"`
	Extractor  string  `json:"extractor_key"`
	Language   string  `json:"language"`
}

// ytFetcher trae metadata + subtítulos (fetch) o el audio (fetchAudio) de una URL. Se inyecta para
// testear sin yt-dlp. fetchAudio devuelve la ruta a un wav y un cleanup para borrar el temporal.
type ytFetcher interface {
	fetch(ctx context.Context, rawURL string, opts Options) (meta ytMeta, vtt string, subLang string, err error)
	fetchAudio(ctx context.Context, rawURL string, opts Options) (audioPath string, cleanup func(), err error)
}

// MediaExtractor implementa Extractor para plataformas de video/redes. Transcriber es opcional
// (whisper.cpp): si está disponible y el video no tiene subtítulos, se transcribe el audio (F2).
type MediaExtractor struct {
	Fetcher     ytFetcher
	Transcriber Transcriber
}

// NewMediaExtractor arma el extractor con el fetcher real de yt-dlp apuntando a bin, y detecta
// whisper.cpp para el fallback de transcripción (nil-safe: si no está, la cascada corta en captions).
func NewMediaExtractor(bin string) *MediaExtractor {
	return &MediaExtractor{Fetcher: &ytDlpFetcher{bin: bin}, Transcriber: FindWhisper()}
}

// FindYtDlp busca el binario de yt-dlp: primero en el PATH, luego en ~/.local/bin (donde lo deja
// `uv tool install yt-dlp` / pip --user, que no siempre está en el PATH). Devuelve "" si no está.
func FindYtDlp() string {
	if p, err := exec.LookPath("yt-dlp"); err == nil {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	for _, name := range []string{"yt-dlp", "yt-dlp.exe"} {
		cand := filepath.Join(home, ".local", "bin", name)
		if _, err := os.Stat(cand); err == nil {
			return cand
		}
	}
	return ""
}

func (m *MediaExtractor) Name() string          { return "media(yt-dlp)" }
func (m *MediaExtractor) Match(u *url.URL) bool { return IsMediaHost(u) }

// Extract trae los subtítulos y arma el Result. Un fallo de yt-dlp (privado, bloqueado, sin
// cookies) se degrada BLANDO: Result con Note accionable, no error duro (R4.3/R8).
func (m *MediaExtractor) Extract(ctx context.Context, rawURL string, opts Options) (Result, error) {
	if len(opts.Langs) == 0 {
		opts.Langs = DefaultLangs
	}
	meta, vtt, subLang, err := m.Fetcher.fetch(ctx, rawURL, opts)
	if err != nil {
		return Result{
			SourceURL:        rawURL,
			Platform:         platformForHost(hostOf(rawURL)),
			Kind:             KindVideo,
			TranscriptSource: SourceNone,
			Note:             mediaErrorNote(err),
		}, nil
	}
	res := buildMediaResult(rawURL, meta, vtt, subLang)
	if res.TranscriptSource != SourceNone {
		return res, nil // ya hay subtítulos: no toca Whisper (cascada barata primero)
	}
	// Cascada F2: sin subtítulos → si whisper.cpp está disponible, bajar el audio y transcribir local.
	if m.Transcriber != nil && m.Transcriber.Available() {
		if txt, lang := m.transcribe(ctx, rawURL, opts); txt != "" {
			res.Text = txt
			res.TranscriptSource = SourceWhisper
			res.Note = ""
			if res.Lang == "" {
				res.Lang = lang
			}
			return res, nil
		}
		res.Note = "el video no tiene subtítulos y la transcripción de audio falló"
		return res, nil
	}
	res.Note = "el video no tiene subtítulos; instalá whisper.cpp (MUSUBI_WHISPER_BIN/MODEL) para transcribir el audio"
	return res, nil
}

// transcribe baja el audio del video y lo pasa por el Transcriber. Best-effort: cualquier fallo
// (descarga o whisper) devuelve "" y la cascada degrada con aviso.
func (m *MediaExtractor) transcribe(ctx context.Context, rawURL string, opts Options) (text, lang string) {
	audioPath, cleanup, err := m.Fetcher.fetchAudio(ctx, rawURL, opts)
	if err != nil {
		return "", ""
	}
	defer cleanup()
	txt, lg, terr := m.Transcriber.Transcribe(ctx, audioPath, opts.Langs)
	if terr != nil {
		return "", ""
	}
	return strings.TrimSpace(txt), lg
}

// buildMediaResult arma el Result a partir de la metadata y el VTT. Es puro y testeable.
func buildMediaResult(rawURL string, m ytMeta, vtt, subLang string) Result {
	platform := platformForHost(hostOf(rawURL))
	if platform == "" {
		platform = strings.ToLower(strings.TrimSpace(m.Extractor))
	}
	r := Result{
		SourceURL:       rawURL,
		Platform:        platform,
		ID:              strings.TrimSpace(m.ID),
		Kind:            KindVideo,
		Title:           strings.TrimSpace(m.Title),
		Author:          firstNonEmpty(m.Uploader, m.Channel),
		Lang:            firstNonEmpty(subLang, m.Language),
		DurationSeconds: int(m.Duration),
	}
	if len(m.UploadDate) == 8 {
		r.PublishedAt = m.UploadDate[:4] + "-" + m.UploadDate[4:6] + "-" + m.UploadDate[6:]
	}
	if txt := CleanSubtitles(vtt); txt != "" {
		r.Text = txt
		r.TranscriptSource = SourceCaptions
	} else {
		r.TranscriptSource = SourceNone // el aviso lo pone Extract según haya o no whisper
	}
	return r
}

// mediaErrorNote traduce un error de yt-dlp a un aviso accionable. Detecta los bloqueos típicos de
// IG/FB/TikTok/X (login/cookies) para sugerir la solución (R8/E4) en vez de escupir un stacktrace.
func mediaErrorNote(err error) string {
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "sign in") || strings.Contains(s, "log in") || strings.Contains(s, "login") ||
		strings.Contains(s, "cookies") || strings.Contains(s, "rate-limit") || strings.Contains(s, "not available"):
		return "la plataforma pidió sesión: corré esto desde tu PC/laptop con --cookies-from-browser <navegador> (donde estás logueado)"
	case strings.Contains(s, "private") || strings.Contains(s, "unavailable"):
		return "el contenido es privado o no está disponible"
	case strings.Contains(s, "executable file not found") || strings.Contains(s, "no such file"):
		return "yt-dlp no está disponible — instalalo con `uv tool install yt-dlp`"
	default:
		return "no pude bajar el video: " + strings.TrimSpace(err.Error())
	}
}

// ---- fetcher real (yt-dlp por shell-out) ----

type ytDlpFetcher struct{ bin string }

// fetch corre yt-dlp una vez: baja el info.json + los subtítulos (auto y manuales) a un dir temporal,
// y devuelve la metadata parseada + el mejor VTT según la preferencia de idioma.
func (f *ytDlpFetcher) fetch(ctx context.Context, rawURL string, opts Options) (ytMeta, string, string, error) {
	tmp, err := os.MkdirTemp("", "musubi-ingest-*")
	if err != nil {
		return ytMeta{}, "", "", err
	}
	defer os.RemoveAll(tmp)

	args := []string{
		"--skip-download",
		"--write-info-json",
		"--write-subs", "--write-auto-subs",
		"--sub-langs", strings.Join(opts.Langs, ","),
		"--sub-format", "vtt",
		"--no-warnings", "--ignore-config", "--no-playlist",
		"-o", filepath.Join(tmp, "item.%(ext)s"),
	}
	if opts.CookiesFromBrowser != "" {
		args = append(args, "--cookies-from-browser", opts.CookiesFromBrowser)
	}
	if opts.CookiesFile != "" {
		args = append(args, "--cookies", opts.CookiesFile)
	}
	args = append(args, rawURL)

	cmd := exec.CommandContext(ctx, f.bin, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	runErr := cmd.Run()

	// BEST-EFFORT: yt-dlp sale con código ≠0 si ALGÚN idioma pedido falla (p.ej. HTTP 429 al pedir
	// el 2º subtítulo), aunque otro haya bajado bien y ya esté en disco. Leemos lo que quedó ANTES de
	// tratar el exit code como fallo: si recuperamos subtítulos o al menos la metadata, seguimos.
	meta, metaErr := readInfoJSON(tmp)
	vtt, lang := pickBestSubtitle(tmp, opts.Langs)
	if vtt == "" && metaErr != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" && runErr != nil {
			msg = runErr.Error()
		}
		if msg == "" {
			msg = "yt-dlp no devolvió datos"
		}
		return ytMeta{}, "", "", fmt.Errorf("%s", msg)
	}
	return meta, vtt, lang, nil
}

// fetchAudio baja SOLO el audio del video a un wav 16kHz mono (lo que espera whisper.cpp) en un dir
// temporal. Devuelve la ruta del wav + un cleanup para borrar el temporal. Requiere ffmpeg (yt-dlp lo
// usa para extraer/convertir el audio).
func (f *ytDlpFetcher) fetchAudio(ctx context.Context, rawURL string, opts Options) (string, func(), error) {
	noop := func() {}
	tmp, err := os.MkdirTemp("", "musubi-audio-*")
	if err != nil {
		return "", noop, err
	}
	cleanup := func() { os.RemoveAll(tmp) }
	args := []string{
		"--no-playlist", "--no-warnings", "--ignore-config",
		"-x", "--audio-format", "wav",
		"--postprocessor-args", "ffmpeg:-ar 16000 -ac 1",
		"-o", filepath.Join(tmp, "audio.%(ext)s"),
	}
	if opts.CookiesFromBrowser != "" {
		args = append(args, "--cookies-from-browser", opts.CookiesFromBrowser)
	}
	if opts.CookiesFile != "" {
		args = append(args, "--cookies", opts.CookiesFile)
	}
	args = append(args, rawURL)

	cmd := exec.CommandContext(ctx, f.bin, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return "", noop, fmt.Errorf("%s", strings.TrimSpace(stderr.String()))
	}
	wavs, _ := filepath.Glob(filepath.Join(tmp, "*.wav"))
	if len(wavs) == 0 {
		cleanup()
		return "", noop, fmt.Errorf("yt-dlp no dejó audio wav (¿falta ffmpeg?)")
	}
	return wavs[0], cleanup, nil
}

// readInfoJSON encuentra y parsea el *.info.json del dir.
func readInfoJSON(dir string) (ytMeta, error) {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.info.json"))
	if len(matches) == 0 {
		return ytMeta{}, fmt.Errorf("sin info.json")
	}
	raw, err := os.ReadFile(matches[0])
	if err != nil {
		return ytMeta{}, err
	}
	var m ytMeta
	if err := json.Unmarshal(raw, &m); err != nil {
		return ytMeta{}, err
	}
	return m, nil
}

// pickBestSubtitle elige el .vtt del dir según la preferencia de idioma; si no matchea ninguno,
// devuelve el primero que haya. Devuelve el contenido y el código de idioma detectado del nombre.
func pickBestSubtitle(dir string, langs []string) (content, lang string) {
	matches, _ := filepath.Glob(filepath.Join(dir, "*.vtt"))
	if len(matches) == 0 {
		return "", ""
	}
	pick := func(path string) (string, string) {
		b, err := os.ReadFile(path)
		if err != nil {
			return "", ""
		}
		return string(b), subLangOfFile(path)
	}
	// Preferencia por idioma pedido (prefijo: es matchea es, es-ES, es-419).
	for _, want := range langs {
		want = strings.ToLower(want)
		for _, path := range matches {
			if strings.HasPrefix(subLangOfFile(path), want) {
				return pick(path)
			}
		}
	}
	return pick(matches[0])
}

// subLangOfFile saca el código de idioma de "item.es.vtt" -> "es", "item.en-US.vtt" -> "en-us".
func subLangOfFile(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".vtt")
	if i := strings.LastIndexByte(base, '.'); i >= 0 {
		return strings.ToLower(base[i+1:])
	}
	return ""
}
