package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"musubi/internal/ingest"
	"musubi/internal/memory"
)

// methods_ingest.go implementa musubi_ingest_url: ingiere un link (video/red/artículo) y devuelve su
// texto; con save=true lo persiste como observación durable (idempotente por URL/id). SOLO se
// registra en el daemon LOCAL (WithLocalTools). En el central sería un fetcher de URLs ARBITRARIAS
// del lado del server (SSRF) + un spawner de subprocesos (yt-dlp) expuesto a varios principales —
// superficie que deliberadamente no abrimos en infra compartida. Ver SDD sdd/ingesta-de-links (T11).

func (s *McpServer) toolIngestURL(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		URL                string `json:"url"`
		Save               bool   `json:"save"`
		As                 string `json:"as"`
		Lang               string `json:"lang"`
		CookiesFromBrowser string `json:"cookies_from_browser"`
		CookiesFile        string `json:"cookies_file"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.URL) == "" {
		return nil, rpcErrorf(codeInvalidParams, "url es obligatorio")
	}

	// Motores: yt-dlp si está (ruta video/redes) + go-trafilatura embebido (artículos, siempre).
	var media ingest.Extractor
	if bin := ingest.FindYtDlp(); bin != "" {
		media = ingest.NewMediaExtractor(bin)
	}
	reg := ingest.NewRegistry(media, ingest.NewArticleExtractor())

	opts := ingest.Options{
		ForceKind:          ingestForceKind(args.As),
		CookiesFromBrowser: args.CookiesFromBrowser,
		CookiesFile:        args.CookiesFile,
	}
	if strings.TrimSpace(args.Lang) != "" {
		opts.Langs = ingestSplitLangs(args.Lang)
	}

	ectx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	res, err := reg.Extract(ectx, args.URL, opts)
	if err != nil {
		return nil, rpcErrorf(codeInvalidParams, "no pude ingerir la URL: %v", err)
	}

	// save=true persiste (idempotente): mismo id determinístico ⇒ UPSERT, no duplica. Atribución
	// por credencial (origin/author sellados server-side), igual que musubi_save_observation.
	if args.Save && strings.TrimSpace(res.Text) != "" {
		origin, okOrigin := writeOriginFor(principalFrom(ctx), "")
		if !okOrigin {
			return nil, rpcErrorf(codeUnauthorized, "escritura sin proyecto: esta credencial no tiene project_id propio")
		}
		author := authorFrom(principalFrom(ctx))
		topicKey, obsID := ingest.PersistKey(res)
		content := ingest.RenderForMemory(res)
		emb := s.embedIfEnabled(content)
		if serr := s.engine.SaveObservationTypedFrom(origin, author, obsID, topicKey, content, 1.0, "semantic", s.defaultScope(), emb); serr != nil {
			if errors.Is(serr, memory.ErrCrossTenant) {
				return nil, rpcErrorf(codeUnauthorized, "%v — usá un id nuevo", serr)
			}
			return nil, rpcErrorf(codeInternalError, "no pude guardar la ingesta: %v", serr)
		}
		res.SavedID = obsID
	}
	return jsonResult(res)
}

// localToolEntries son las tools que SOLO se registran en el daemon local (ver WithLocalTools).
func (s *McpServer) localToolEntries() []toolEntry {
	return []toolEntry{
		{
			Tool: Tool{
				Name:        "musubi_ingest_url",
				Description: "Ingiere un link (video de YouTube/redes vía yt-dlp, o cualquier artículo web vía go-trafilatura embebido) y devuelve su texto + metadata (título/autor/fecha/idioma/duración). Model-free: para video usa los subtítulos existentes (sin transcripción de audio en esta fase). Con save=true persiste el texto como memoria durable bajo ingested/<plataforma>/<id>, de forma IDEMPOTENTE (re-ingerir la misma URL no duplica). Por defecto solo devuelve el texto. Sólo disponible en el daemon local (no en el cerebro central). Degrada blando: si falta yt-dlp o la plataforma bloquea (IG/FB/X piden cookies), devuelve un aviso en 'note', no un error.",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"url":                  {Type: "string", Description: "La URL a ingerir (video, red social o artículo)"},
						"save":                 {Type: "boolean", Description: "Si es true, guarda el texto en la memoria del cerebro (idempotente); default false (solo devolver)"},
						"as":                   {Type: "string", Description: "Forzar la ruta: 'article' o 'video'; opcional, default auto por el host"},
						"lang":                 {Type: "string", Description: "Idioma(s) de subtítulos preferidos, coma-separados (ej. 'es,en'); opcional"},
						"cookies_from_browser": {Type: "string", Description: "Navegador del que tomar cookies (chrome, firefox…) para plataformas que piden sesión (IG/FB/X); opcional"},
						"cookies_file":         {Type: "string", Description: "Ruta a un archivo de cookies Netscape para yt-dlp; opcional"},
					},
					Required: []string{"url"},
				},
			},
			handler: s.toolIngestURL,
		},
	}
}

func ingestForceKind(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "article", "articulo", "artículo", "web", "page":
		return ingest.KindArticle
	case "video", "media":
		return ingest.KindVideo
	default:
		return ""
	}
}

func ingestSplitLangs(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
