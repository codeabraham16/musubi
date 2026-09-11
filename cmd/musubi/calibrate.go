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

	"musubi/internal/config"
	"musubi/internal/memory"
)

// calibrate.go implementa 'musubi calibrate': una calibración OPT-IN del estimador
// de tokens contra el endpoint count_tokens de Anthropic. Es lo ÚNICO en Musubi
// que hace red a Anthropic, requiere ANTHROPIC_API_KEY explícita y se ejecuta a
// mano: el server MCP sigue 100% offline y model-free. count_tokens cuenta tokens
// (no es inferencia de un LLM). Sin --apply solo diagnostica; con --apply persiste
// los divisores sugeridos y recomputa la columna tokens.

const (
	anthropicVersion      = "2023-06-01"
	defaultCalibrateModel = "claude-opus-4-8"
)

// countTokensURL es el endpoint de count_tokens. Es var (no const) para poder
// apuntarlo a un server de prueba en los tests.
var countTokensURL = "https://api.anthropic.com/v1/messages/count_tokens"

func runCalibrate(args []string) {
	apply := false
	model := defaultCalibrateModel
	limit := 12
	// A DÓNDE SE PREGUNTA Y DE DÓNDE SALE LA CREDENCIAL, los dos configurables.
	//
	// Estaban clavados en `api.anthropic.com` + `ANTHROPIC_API_KEY`, y eso obligaba a conseguir una
	// API key de consola aunque la instalación YA tuviera por dónde contar tokens. Medido en el
	// cerebro central: su pilar de cognición habla con un proxy LiteLLM
	// (`cognition.endpoint: http://127.0.0.1:4000/v1`, credencial en `LITELLM_MASTER_KEY`), ese
	// proxy EXPONE `/v1/messages/count_tokens` —contesta 401 y no 404— y acepta la credencial tanto
	// en `x-api-key` como en `Authorization: Bearer`, que es la cabecera que esto ya manda. O sea
	// que lo único que faltaba era poder apuntar a otra URL.
	//
	// No se cablea LiteLLM acá: se parametriza. Cualquier pasarela que hable el mismo endpoint
	// sirve, y la de Anthropic sigue siendo el default.
	endpoint := countTokensURL
	varClave := "ANTHROPIC_API_KEY"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--apply":
			apply = true
		case "--endpoint":
			if i+1 < len(args) {
				endpoint = args[i+1]
				i++
			}
		case "--key-env":
			if i+1 < len(args) {
				varClave = args[i+1]
				i++
			}
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

	// LA VARIABLE O EL ARCHIVO `<VAR>_FILE` (A89/A101). El cuarto sitio de la misma regla: acá el
	// `os.Getenv` pelado hacía que con `ANTHROPIC_API_KEY_FILE` puesto el mensaje de abajo dijera
	// «requiere ANTHROPIC_API_KEY» teniendo la credencial ahí al lado, sin leer. Es una
	// herramienta a mano y no rompe nada en producción, pero una regla que se aplica en unos
	// caminos y no en otros vuelve por el que quedó afuera.
	apiKey, err := config.SecretoDeEnv(varClave)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi calibrate: %v\n", err)
		os.Exit(1)
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		// EL MENSAJE NOMBRA LA VARIABLE QUE DE VERDAD SE MIRÓ, no una fija. Con `--key-env` puesto,
		// decir «requiere ANTHROPIC_API_KEY» mandaba a poner la credencial equivocada.
		fmt.Fprintf(os.Stderr, "musubi calibrate es OPT-IN: requiere %s (o %s_FILE).\n", varClave, varClave)
		fmt.Fprintln(os.Stderr, "Usa el endpoint count_tokens para medir la precisión del estimador.")
		fmt.Fprintln(os.Stderr, "Si tu instalación ya tiene una pasarela que lo expone (p. ej. un proxy LiteLLM),")
		fmt.Fprintln(os.Stderr, "apuntá ahí en vez de conseguir una API key nueva:")
		fmt.Fprintln(os.Stderr, "  musubi calibrate --endpoint http://127.0.0.1:4000/v1/messages/count_tokens \\")
		fmt.Fprintln(os.Stderr, "                   --key-env LITELLM_MASTER_KEY --model <alias-del-proxy>")
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
	// SE DICE CONTRA QUÉ SE MIDIÓ, endpoint incluido. EL CONTEO DE TOKENS ES ESPECÍFICO DEL MODELO:
	// desde Claude Opus 4.7 el tokenizador da ~30% más tokens que en los modelos anteriores.
	// Calibrar contra un alias de proxy que resuelve a otra familia deja los divisores mal por ese
	// margen, y el informe se vería igual de sano. Por eso el destino va impreso: sin él,
	// «calibrado» no dice contra qué.
	fmt.Printf("Calibrando con %d muestras contra %s (model=%s)...\n", len(texts), endpoint, model)
	if endpoint != countTokensURL {
		fmt.Printf("  OJO: el conteo sale del tokenizador que resuelva %q en esa pasarela.\n", model)
		fmt.Println("  Tiene que ser el MISMO modelo que consume esta memoria, o los divisores quedan sesgados.")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	var counts []memory.TextCount
	for _, txt := range texts {
		n, err := countTokensRemote(client, endpoint, apiKey, model, txt)
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
			// El portero NO va acá: vive en SampleContents, que es el punto de egreso y el único
			// lugar donde se puede probar contra una fila cruda. Ver el comentario de esa función.
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
func countTokensRemote(client *http.Client, endpoint, apiKey, model, text string) (int, error) {
	body, err := json.Marshal(map[string]interface{}{
		"model":    model,
		"messages": []map[string]string{{"role": "user", "content": text}},
	})
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
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
