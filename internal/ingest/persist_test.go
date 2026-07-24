package ingest

import (
	"strings"
	"testing"
)

func TestCanonicalURLQuitaTracking(t *testing.T) {
	got := CanonicalURL("https://www.YouTube.com/watch?v=abc123&utm_source=news&si=xyz&feature=share")
	// Conserva v=, baja el host, quita utm_source/si/feature.
	if !strings.Contains(got, "v=abc123") {
		t.Fatalf("debía conservar v=; got %q", got)
	}
	if strings.Contains(got, "utm_source") || strings.Contains(got, "si=") || strings.Contains(got, "feature") {
		t.Fatalf("no quitó el tracking; got %q", got)
	}
	if strings.Contains(got, "YouTube") {
		t.Fatalf("no bajó el host; got %q", got)
	}
}

func TestPersistKeyIdempotente(t *testing.T) {
	// Mismo video, dos URLs con distinto tracking → mismo topic_key e id (idempotencia, R10).
	a := Result{Platform: "youtube", ID: "abc123", SourceURL: "https://youtube.com/watch?v=abc123"}
	b := Result{Platform: "youtube", ID: "abc123", SourceURL: "https://www.youtube.com/watch?v=abc123&utm_source=x"}
	ka, ida := PersistKey(a)
	kb, idb := PersistKey(b)
	if ka != kb || ida != idb {
		t.Fatalf("mismo video debía dar misma clave: (%s,%s) vs (%s,%s)", ka, ida, kb, idb)
	}
	if ka != "ingested/youtube/abc123" {
		t.Fatalf("topic_key inesperado: %s", ka)
	}
	if !strings.HasPrefix(ida, "ingest-") {
		t.Fatalf("id con prefijo inesperado: %s", ida)
	}
}

func TestPersistKeyArticuloUsaHashDeURL(t *testing.T) {
	// Sin ID nativo (artículo): el slug sale del hash de la URL canónica, estable entre corridas.
	r := Result{Platform: "article", SourceURL: "https://blog.example.com/post?utm_medium=rss"}
	k1, id1 := PersistKey(r)
	r2 := Result{Platform: "article", SourceURL: "https://blog.example.com/post"}
	k2, id2 := PersistKey(r2)
	if k1 != k2 || id1 != id2 {
		t.Fatalf("la misma nota con/sin tracking debía deduplicar: %s/%s vs %s/%s", k1, id1, k2, id2)
	}
	if !strings.HasPrefix(k1, "ingested/article/") {
		t.Fatalf("topic_key inesperado: %s", k1)
	}
}

func TestRenderForMemoryLlevaEncabezado(t *testing.T) {
	r := Result{Title: "Un título", SourceURL: "https://x/y", Author: "Ada", PublishedAt: "2026-07-23",
		Platform: "youtube", TranscriptSource: SourceCaptions, Text: "el cuerpo real"}
	out := RenderForMemory(r)
	for _, want := range []string{"Un título", "Fuente: https://x/y", "por Ada", "2026-07-23", "el cuerpo real"} {
		if !strings.Contains(out, want) {
			t.Fatalf("el render no contiene %q\n%s", want, out)
		}
	}
}
