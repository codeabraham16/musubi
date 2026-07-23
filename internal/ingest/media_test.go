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
	meta ytMeta
	vtt  string
	lang string
	err  error
}

func (s stubFetcher) fetch(context.Context, string, Options) (ytMeta, string, string, error) {
	return s.meta, s.vtt, s.lang, s.err
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
	if !strings.Contains(res.Note, "Whisper") {
		t.Fatalf("el aviso debería mencionar Whisper; note=%q", res.Note)
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
