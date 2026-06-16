package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"musubi/internal/memory"
)

// calibrate.go implementa 'musubi calibrate': una calibración OPT-IN del estimador
// de tokens contra el endpoint count_tokens de Anthropic. Es lo ÚNICO en Musubi
// que hace red a Anthropic, requiere ANTHROPIC_API_KEY explícita y se ejecuta a
// mano: el server MCP sigue 100% offline y model-free. count_tokens cuenta tokens
// (no es inferencia de un LLM). Sin --apply solo diagnostica; con --apply persiste
// los divisores sugeridos y recomputa la columna tokens.

const (
	countTokensURL        = "https://api.anthropic.com/v1/messages/count_tokens"
	anthropicVersion      = "2023-06-01"
	defaultCalibrateModel = "claude-opus-4-8"
)

func runCalibrate(args []string) {
	apply := false
	model := defaultCalibrateModel
	limit := 12
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--apply":
			apply = true
		case "--model":
			if i+1 < len(args) {
				model = args[i+1]
				i++
			}
		case "--limit":
			if i+1 < len(args) {
				if n, err := strconv.Atoi(args[i+1]); err == nil {
					limit = n
				}
				i++
			}
		}
	}

	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "musubi calibrate es OPT-IN: requiere ANTHROPIC_API_KEY.")
		fmt.Fprintln(os.Stderr, "Usa el endpoint count_tokens de Anthropic para medir la precisión del estimador.")
		fmt.Fprintln(os.Stderr, "El server MCP sigue offline/model-free; esto es solo una herramienta manual.")
		os.Exit(1)
	}

	root := workspaceDir()
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error al abrir la memoria: %v\n", err)
		os.Exit(1)
	}
	defer engine.Close()

	texts := gatherCalibrationTexts(engine, limit)
	fmt.Printf("Calibrando con %d muestras contra count_tokens (model=%s)...\n", len(texts), model)

	client := &http.Client{Timeout: 30 * time.Second}
	var counts []memory.TextCount
	for _, txt := range texts {
		n, err := countTokensRemote(client, apiKey, model, txt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ! muestra omitida: %v\n", err)
			continue
		}
		counts = append(counts, memory.TextCount{Text: txt, Actual: n})
	}
	if len(counts) == 0 {
		fmt.Fprintln(os.Stderr, "No se pudo medir ninguna muestra (revisá la API key / red).")
		os.Exit(1)
	}

	rep := memory.BuildCalibrationReport(counts)
	printCalibrationReport(rep)

	if !apply {
		fmt.Println("\n(diagnóstico) Usá 'musubi calibrate --apply' para persistir los divisores sugeridos y recomputar.")
		return
	}

	prose, code, jsn := suggestedDivisors(rep)
	if err := engine.SaveDivisors(prose, code, jsn); err != nil {
		fmt.Fprintf(os.Stderr, "Error al guardar divisores: %v\n", err)
		os.Exit(1)
	}
	memory.ConfigureDivisors(prose, code, jsn)
	if err := engine.RecomputeTokens(); err != nil {
		fmt.Fprintf(os.Stderr, "Error al recomputar tokens: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nAplicado: prose=%.2f code=%.2f json=%.2f. Columna 'tokens' recomputada.\n", prose, code, jsn)
}

// gatherCalibrationTexts arma el conjunto de muestras: un corpus base que cubre
// los tres tipos (asegura cobertura de código/JSON) + hasta limit contenidos de
// la memoria del proyecto (más representativos de su prosa real).
func gatherCalibrationTexts(engine *memory.DbEngine, limit int) []string {
	texts := append([]string{}, builtinCalibrationCorpus...)
	if contents, err := engine.SampleContents(limit); err == nil {
		for _, c := range contents {
			if len(strings.TrimSpace(c)) >= 40 { // descartar muestras muy cortas (overhead domina)
				texts = append(texts, c)
			}
		}
	}
	return texts
}

// builtinCalibrationCorpus cubre prosa, código y JSON para que la calibración
// tenga señal en los tres tipos aunque la memoria sea casi toda prosa.
var builtinCalibrationCorpus = []string{
	"Este es un párrafo de prosa en español, con varias oraciones de longitud razonable que sirven para medir cuántos tokens consume el texto natural frente a la estimación heurística.",
	"The quick brown fox jumps over the lazy dog, and then writes a moderately long sentence in English to exercise the prose estimator with natural language tokens.",
	"func Fibonacci(n int) int {\n\tif n < 2 {\n\t\treturn n\n\t}\n\treturn Fibonacci(n-1) + Fibonacci(n-2)\n}",
	"for (let i = 0; i < items.length; i++) { total += items[i].price * items[i].quantity; if (total > limit) break; }",
	`{"name":"musubi","version":"0.8.0","tools":["recall","expand","tokens"],"config":{"budget":400,"delta":true}}`,
	`[{"id":"a","tokens":12,"hash":"abc123"},{"id":"b","tokens":34,"hash":"def456"},{"id":"c","tokens":7,"hash":"ghi789"}]`,
}

// suggestedDivisors extrae los divisores sugeridos por tipo del reporte; si falta
// algún tipo, conserva el divisor actual de ese tipo.
func suggestedDivisors(rep memory.CalibrationReport) (prose, code, jsn float64) {
	prose, code, jsn = memory.CurrentDivisors()
	for _, k := range rep.PerKind {
		if k.SuggestedDivisor <= 0 {
			continue
		}
		switch k.Kind {
		case "prose":
			prose = k.SuggestedDivisor
		case "code":
			code = k.SuggestedDivisor
		case "json":
			jsn = k.SuggestedDivisor
		}
	}
	return prose, code, jsn
}

func printCalibrationReport(rep memory.CalibrationReport) {
	fmt.Printf("\n%-7s %8s %10s %8s %10s %10s\n", "tipo", "muestras", "estimado", "real", "error%", "divisor→sug")
	for _, k := range rep.PerKind {
		fmt.Printf("%-7s %8d %10d %8d %9.1f%% %5.2f→%.2f\n",
			k.Kind, k.Samples, k.EstimatedTokens, k.ActualTokens, k.ErrorPct, k.CurrentDivisor, k.SuggestedDivisor)
	}
	fmt.Printf("\nError global del estimador actual: %.1f%%\n", rep.OverallErrorPct)
}

// countTokensRemote llama al endpoint count_tokens de Anthropic (raw net/http,
// sin SDK: respeta el invariante de no agregar dependencias) y devuelve el conteo
// real de tokens del texto.
func countTokensRemote(client *http.Client, apiKey, model, text string) (int, error) {
	body, err := json.Marshal(map[string]interface{}{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": text}},
	})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, countTokensURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("content-type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("count_tokens HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var out struct {
		InputTokens int `json:"input_tokens"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return 0, fmt.Errorf("respuesta inválida de count_tokens: %w", err)
	}
	return out.InputTokens, nil
}
