package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"musubi/internal/memory"
)

func TestSuggestedDivisorsUsaSugeridosYConservaFaltantes(t *testing.T) {
	// Divisor actual de referencia para los tipos sin sugerencia en el reporte.
	prose0, code0, json0 := memory.CurrentDivisors()

	rep := memory.CalibrationReport{
		PerKind: []memory.KindCalibration{
			{Kind: "prose", SuggestedDivisor: 3.7},
			{Kind: "code", SuggestedDivisor: 0}, // <=0 => se ignora, conserva el actual
		},
	}
	prose, code, jsn := suggestedDivisors(rep)
	if prose != 3.7 {
		t.Errorf("prose: esperaba 3.7 (sugerido), obtuve %v", prose)
	}
	if code != code0 {
		t.Errorf("code: esperaba conservar el actual %v, obtuve %v", code0, code)
	}
	if jsn != json0 {
		t.Errorf("json: esperaba conservar el actual %v, obtuve %v", json0, jsn)
	}
	_ = prose0
}

func TestGatherCalibrationTextsIncluyeCorpusBase(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	defer engine.Close()

	// DB vacía: deben venir al menos las muestras del corpus base (prosa/código/JSON).
	texts := gatherCalibrationTexts(engine, 12)
	if len(texts) < len(builtinCalibrationCorpus) {
		t.Fatalf("esperaba >= %d muestras del corpus base, obtuve %d", len(builtinCalibrationCorpus), len(texts))
	}
	for _, want := range builtinCalibrationCorpus {
		found := false
		for _, got := range texts {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("falta una muestra del corpus base en el resultado")
			break
		}
	}
}

func TestCountTokensRemoteParseaInputTokens(t *testing.T) {
	var gotKey, gotVersion string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("x-api-key")
		gotVersion = r.Header.Get("anthropic-version")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"input_tokens": 42})
	}))
	defer srv.Close()

	// countTokensRemote usa la URL global; la apuntamos al server de prueba.
	old := countTokensURL
	countTokensURL = srv.URL
	defer func() { countTokensURL = old }()

	n, err := countTokensRemote(srv.Client(), "sk-test", "claude-opus-4-8", "hola")
	if err != nil {
		t.Fatalf("countTokensRemote error: %v", err)
	}
	if n != 42 {
		t.Errorf("esperaba 42 tokens, obtuve %d", n)
	}
	if gotKey != "sk-test" {
		t.Errorf("x-api-key incorrecto: %q", gotKey)
	}
	if gotVersion != anthropicVersion {
		t.Errorf("anthropic-version incorrecto: %q", gotVersion)
	}
}

func TestCountTokensRemoteErrorHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"bad key"}}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	old := countTokensURL
	countTokensURL = srv.URL
	defer func() { countTokensURL = old }()

	if _, err := countTokensRemote(srv.Client(), "x", "m", "y"); err == nil {
		t.Fatal("esperaba error por status no-200")
	}
}
