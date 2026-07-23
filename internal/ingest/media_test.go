package ingest

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
)

// stubFetcher simula yt-dlp sin red.
type stubFetcher struct {
	meta      ytMeta
	vtt       string
	lang      string
	err       error
	audioPath string
	audioErr  error
}

func (s stubFetcher) fetch(context.Context, string, Options) (ytMeta, string, string, error) {
	return s.meta, s.vtt, s.lang, s.err
}

func (s stubFetcher) fetchAudio(context.Context, string, Options) (string, func(), error) {
	return s.audioPath, func() {}, s.audioErr
}

// mockTranscriber simula whisper.cpp sin binario.
type mockTranscriber struct {
	avail bool
	text  string
	lang  string
	err   error
}

func (m mockTranscriber) Available() bool { return m.avail }
func (m mockTranscriber) Transcribe(context.Context, string, []string) (string, string, error) {
	return m.text, m.lang, m.err
}

const vttSample = "WEBVTT\n\n00:00:00.000 --> 00:00:02.000\nhola mundo\n\n" +
	"00:00:02.000 --> 00:00:04.000\nhola mundo\nsegunda linea\n"

func TestMediaExtractorConSubtitulos(t *testing.T) {
	m := &MediaExtractor{Fetcher: stubFetcher{
		meta: ytMeta{ID: "abc", Title: "Un video", Uploader: "Canal X", UploadDate: "20260723", Duration: 65, Language: "es"},
		vtt:  vttSample, lang: "es",
	}}
	res, err := m.Extract(context.Background(), "https://www.youtube.com/watch?v=abc", Options{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if res.TranscriptSource != SourceCaptions {
		t.Fatalf("esperaba captions, got %q (note=%q)", res.TranscriptSource, res.Note)
	}
	if res.Text != "hola mundo segunda linea" {
		t.Fatalf("texto inesperado: %q", res.Text)
	}
	if res.Platform != "youtube" || res.Kind != KindVideo || res.Author != "Canal X" {
		t.Fatalf("metadata inesperada: %+v", res)
	}
	if res.PublishedAt != "2026-07-23" || res.DurationSeconds != 65 {
		t.Fatalf("fecha/duración inesperadas: %+v", res)
	}
}

func TestMediaExtractorSinSubtitulosDegrada(t *testing.T) {
	m := &MediaExtractor{Fetcher: stubFetcher{meta: ytMeta{Title: "Sin subs"}, vtt: ""}}
	res, err := m.Extract(context.Background(), "https://www.tiktok.com/@x/video/1", Options{})
	if err != nil {
		t.Fatalf("Extract no debería dar error duro: %v", err)
	}
	if res.TranscriptSource != SourceNone || res.Note == "" {
		t.Fatalf("esperaba none + note; got %+v", res)
	}
	if !strings.Contains(res.Note, "whisper.cpp") {
		t.Fatalf("sin whisper, el aviso debería sugerir instalarlo; note=%q", res.Note)
	}
}

func TestMediaWhisperTranscribeSinSubtitulos(t *testing.T) {
	m := &MediaExtractor{
		Fetcher:     stubFetcher{meta: ytMeta{Title: "Reel sin subs"}, vtt: "", audioPath: "/tmp/audio.wav"},
		Transcriber: mockTranscriber{avail: true, text: "esto lo dijo el audio", lang: "es"},
	}
	res, err := m.Extract(context.Background(), "https://www.instagram.com/reel/xyz/", Options{})
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if res.TranscriptSource != SourceWhisper {
		t.Fatalf("esperaba whisper, got %q (note=%q)", res.TranscriptSource, res.Note)
	}
	if res.Text != "esto lo dijo el audio" {
		t.Fatalf("texto de whisper inesperado: %q", res.Text)
	}
	if res.Note != "" {
		t.Fatalf("con transcripción exitosa no debería quedar aviso; note=%q", res.Note)
	}
}

func TestMediaWhisperNoPisaSubtitulos(t *testing.T) {
	// Con subtítulos, la cascada NO debe tocar Whisper (barato primero).
	m := &MediaExtractor{
		Fetcher:     stubFetcher{meta: ytMeta{Title: "Con subs"}, vtt: vttSample, lang: "es"},
		Transcriber: mockTranscriber{avail: true, text: "NO DEBERÍA USARSE"},
	}
	res, _ := m.Extract(context.Background(), "https://youtu.be/abc", Options{})
	if res.TranscriptSource != SourceCaptions {
		t.Fatalf("con subtítulos debía quedarse en captions, got %q", res.TranscriptSource)
	}
	if strings.Contains(res.Text, "NO DEBERÍA") {
		t.Fatalf("Whisper pisó los subtítulos: %q", res.Text)
	}
}

func TestMediaWhisperFalloDegrada(t *testing.T) {
	m := &MediaExtractor{
		Fetcher:     stubFetcher{vtt: "", audioErr: errors.New("no bajó audio")},
		Transcriber: mockTranscriber{avail: true, text: "irrelevante"},
	}
	res, _ := m.Extract(context.Background(), "https://youtu.be/x", Options{})
	if res.TranscriptSource != SourceNone {
		t.Fatalf("si falla la descarga de audio debe degradar a none; got %q", res.TranscriptSource)
	}
}

func TestMediaExtractorErrorDaNoteAccionable(t *testing.T) {
	m := &MediaExtractor{Fetcher: stubFetcher{err: errors.New("ERROR: Sign in to confirm you're not a bot")}}
	res, err := m.Extract(context.Background(), "https://www.instagram.com/reel/xyz/", Options{})
	if err != nil {
		t.Fatalf("no debería propagar error duro: %v", err)
	}
	if res.TranscriptSource != SourceNone {
		t.Fatalf("esperaba none; got %q", res.TranscriptSource)
	}
	if !strings.Contains(res.Note, "cookies-from-browser") {
		t.Fatalf("el aviso debería sugerir cookies; note=%q", res.Note)
	}
}

func TestMediaMatchYPlatform(t *testing.T) {
	m := &MediaExtractor{}
	yes, _ := url.Parse("https://youtu.be/abc")
	no, _ := url.Parse("https://blog.example.com/post")
	if !m.Match(yes) {
		t.Fatal("debería matchear youtu.be")
	}
	if m.Match(no) {
		t.Fatal("no debería matchear un blog")
	}
	if platformForHost("www.instagram.com") != "instagram" {
		t.Fatal("instagram mal clasificado")
	}
	if platformForHost("example.com") != "" {
		t.Fatal("un host cualquiera no es media")
	}
}
