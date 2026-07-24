package ingest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWhisperAvailable(t *testing.T) {
	if (&WhisperCpp{}).Available() {
		t.Fatal("sin bin ni model NO debe estar disponible")
	}
	if (&WhisperCpp{Bin: "whisper-cli"}).Available() {
		t.Fatal("sin model NO debe estar disponible")
	}
	if (&WhisperCpp{Bin: "whisper-cli", Model: filepath.Join(t.TempDir(), "no-existe.bin")}).Available() {
		t.Fatal("con un model inexistente NO debe estar disponible")
	}
	model := filepath.Join(t.TempDir(), "ggml.bin")
	if err := os.WriteFile(model, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !(&WhisperCpp{Bin: "whisper-cli", Model: model}).Available() {
		t.Fatal("con bin + model existente DEBE estar disponible")
	}
	var nilw *WhisperCpp
	if nilw.Available() {
		t.Fatal("un *WhisperCpp nil no debe estar disponible")
	}
}
