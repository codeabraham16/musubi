package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"strings"
)

// persist.go arma las claves y el texto con que una ingesta se guarda en memoria (F1). NO toca el
// store (eso vive en cmd/musubi para no acoplar el paquete al motor): expone helpers puros y
// deterministas — la idempotencia sale de que el id derive del contenido/URL, no del reloj.

// trackingParams son los parámetros de query que NO identifican el recurso (analytics/tracking). Se
// quitan al canonicalizar para que dos URLs "iguales pero con distinto tracking" dedupliquen igual.
var trackingParams = map[string]bool{
	"utm_source": true, "utm_medium": true, "utm_campaign": true, "utm_term": true, "utm_content": true,
	"gclid": true, "fbclid": true, "igshid": true, "si": true, "ref": true, "ref_src": true,
	"spm": true, "mc_cid": true, "mc_eid": true, "feature": true, "s": true,
}

// CanonicalURL normaliza una URL para dedupe: baja el host, borra el fragmento y quita los
// parámetros de tracking conocidos, CONSERVando los significativos (p.ej. el ?v= de YouTube).
func CanonicalURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	u.Host = strings.ToLower(u.Host)
	u.Fragment = ""
	if u.RawQuery != "" {
		q := u.Query()
		for k := range q {
			if trackingParams[strings.ToLower(k)] {
				q.Del(k)
			}
		}
		u.RawQuery = q.Encode()
	}
	return u.String()
}

// PersistKey deriva el topic_key y el id DETERMINÍSTICO bajo el que se guarda un Result. Para video
// usa el id nativo de la plataforma (estable entre corridas y máquinas); para artículos, un hash de
// la URL canónica. Como el id es determinístico, re-ingerir la misma URL UPSERTEA la misma fila en
// vez de duplicar (idempotencia, R10).
func PersistKey(r Result) (topicKey, obsID string) {
	platform := r.Platform
	if platform == "" {
		platform = "web"
	}
	slug := r.ID
	if slug == "" {
		sum := sha256.Sum256([]byte(CanonicalURL(r.SourceURL)))
		slug = hex.EncodeToString(sum[:])[:12]
	}
	topicKey = "ingested/" + platform + "/" + slug
	sum := sha256.Sum256([]byte(topicKey))
	obsID = "ingest-" + hex.EncodeToString(sum[:])[:16]
	return topicKey, obsID
}

// RenderForMemory arma el texto a persistir: un encabezado con la fuente + metadata (para que el
// recall lo cite bien) y luego el texto extraído.
func RenderForMemory(r Result) string {
	var b strings.Builder
	title := r.Title
	if title == "" {
		title = r.SourceURL
	}
	b.WriteString(title + "\n")
	b.WriteString("Fuente: " + r.SourceURL + "\n")
	var meta []string
	if r.Author != "" {
		meta = append(meta, "por "+r.Author)
	}
	if r.PublishedAt != "" {
		meta = append(meta, r.PublishedAt)
	}
	if r.Platform != "" {
		meta = append(meta, r.Platform)
	}
	if len(meta) > 0 {
		b.WriteString(strings.Join(meta, " · ") + "\n")
	}
	b.WriteString("(ingerido vía `musubi ingest`; fuente del texto: " + r.TranscriptSource + ")\n\n")
	b.WriteString(r.Text)
	return b.String()
}
