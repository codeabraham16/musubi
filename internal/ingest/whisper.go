package ingest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// whisper.go implementa la transcripción LOCAL de audio (F2): cuando un video no tiene subtítulos,
// se baja el audio y se transcribe con whisper.cpp (modelo GGML, CPU, offline, sin costo). Es la
// ÚNICA parte con modelo y es OPCIONAL: si no está configurado, la cascada corta en "sin subtítulos".
// Consistente con la identidad model-free (Musubi nunca llama a un ASR/LLM externo): whisper es un
// binario local, misma categoría que el StaticProvider de embeddings.

// Transcriber transcribe un archivo de audio a texto.
type Transcriber interface {
	Available() bool
	Transcribe(ctx context.Context, audioPath string, langs []string) (text, lang string, err error)
}

// WhisperCpp corre el binario de whisper.cpp (whisper-cli / main) sobre un wav 16kHz mono.
type WhisperCpp struct {
	Bin   string // ruta al binario whisper.cpp
	Model string // ruta al modelo GGML (.bin)
}

// FindWhisper resuelve el transcriptor desde la config explícita (env) o el PATH. Se prefiere env
// porque el modelo GGML no tiene una ubicación estándar: MUSUBI_WHISPER_BIN y MUSUBI_WHISPER_MODEL
// (los deja `musubi provision`). Sin binario o sin modelo ⇒ Available()=false (degradación).
func FindWhisper() *WhisperCpp {
	bin := strings.TrimSpace(os.Getenv("MUSUBI_WHISPER_BIN"))
	if bin == "" {
		for _, name := range []string{"whisper-cli", "whisper", "main"} {
			if p, err := exec.LookPath(name); err == nil {
				bin = p
				break
			}
		}
	}
	model := strings.TrimSpace(os.Getenv("MUSUBI_WHISPER_MODEL"))
	return &WhisperCpp{Bin: bin, Model: model}
}

func (w *WhisperCpp) Available() bool {
	if w == nil || w.Bin == "" || w.Model == "" {
		return false
	}
	if _, err := os.Stat(w.Model); err != nil {
		return false
	}
	return true
}

// Transcribe corre whisper.cpp sobre audioPath y devuelve el texto. Usa -otxt (escribe <base>.txt),
// -nt (sin timestamps) y -l <lang> (o 'auto' si no se especifica).
func (w *WhisperCpp) Transcribe(ctx context.Context, audioPath string, langs []string) (string, string, error) {
	lang := "auto"
	if len(langs) > 0 && strings.TrimSpace(langs[0]) != "" {
		lang = strings.TrimSpace(langs[0])
	}
	outBase := audioPath // whisper.cpp con -of <base> escribe <base>.txt
	args := []string{"-m", w.Model, "-f", audioPath, "-l", lang, "-otxt", "-nt", "-of", outBase}
	cmd := exec.CommandContext(ctx, w.Bin, args...)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", "", fmt.Errorf("whisper.cpp falló: %s", strings.TrimSpace(stderr.String()))
	}
	raw, err := os.ReadFile(outBase + ".txt")
	if err != nil {
		return "", "", fmt.Errorf("whisper.cpp no dejó salida: %w", err)
	}
	// El .txt de whisper trae una línea por segmento; las unimos en un párrafo.
	text := strings.Join(strings.Fields(strings.ReplaceAll(string(raw), "\n", " ")), " ")
	return text, lang, nil
}
