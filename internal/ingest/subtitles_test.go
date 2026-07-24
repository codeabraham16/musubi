package ingest

import "testing"

func TestCleanSubtitlesLimpiaYDeduplica(t *testing.T) {
	raw := "WEBVTT\n" +
		"Kind: captions\n" +
		"Language: es\n" +
		"\n" +
		"00:00:00.000 --> 00:00:02.000\n" +
		"Hola a todos\n" +
		"\n" +
		"00:00:02.000 --> 00:00:04.000\n" +
		"Hola a todos\n" + // rodante duplicada → se dedupe
		"\n" +
		"00:00:04.000 --> 00:00:06.000 align:start position:0%\n" +
		"<00:00:04.500><c>bienvenidos</c> al canal\n" + // tags inline → se quitan
		"\n" +
		"00:00:06.000 --> 00:00:08.000\n" +
		"[Música]\n" // anotación → se descarta a vacío

	got := CleanSubtitles(raw)
	want := "Hola a todos bienvenidos al canal"
	if got != want {
		t.Fatalf("CleanSubtitles\n got=%q\nwant=%q", got, want)
	}
}

func TestCleanSubtitlesVacio(t *testing.T) {
	if got := CleanSubtitles("WEBVTT\n\n"); got != "" {
		t.Fatalf("esperaba vacío, got=%q", got)
	}
}
