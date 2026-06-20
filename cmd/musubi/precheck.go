package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/memory"
)

// precheck.go implementa 'musubi precheck --hook-mode': el hook PreToolUse atado a
// la tool Read. ANTES de que el agente lea un archivo, Musubi mira su memoria de
// código: si ya tiene un gist FRESCO lo inyecta (para no re-leer el archivo
// entero), si está desactualizado avisa, y si no hay gist y el archivo es grande
// recuerda guardarlo. Hace AUTOMÁTICO el uso de la memoria de código (recall sin
// que el agente tenga que acordarse; nudge de save). 100% model-free.

// umbralArchivoGrande es el tamaño (bytes) a partir del cual, si no hay gist,
// vale la pena recordar guardarlo. Por debajo, no molesta.
const umbralArchivoGrande = 1500

// codeStore es lo que el hook necesita del motor: leer la memoria de código y los errores
// conocidos (telemetría) del archivo que se va a leer.
type codeStore interface {
	GetCodeMemory(path string) (memory.CodeMemory, bool, error)
	GetUnresolvedTelemetryLogsForFiles(files []string) ([]memory.TelemetryLog, error)
}

// maxPrecheckTelemetry acota cuántos errores conocidos se surfacean por lectura, para no
// inundar el contexto del hook.
const maxPrecheckTelemetry = 3

// precheckInput es el subconjunto del JSON de stdin de PreToolUse que usamos.
type precheckInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
	SessionID string `json:"session_id"`
}

// precheckOutput arma el additionalContext del hook PreToolUse para una lectura.
// Devuelve "" (silencioso) si no aplica.
func precheckOutput(store codeStore, root string, stdin io.Reader) string {
	if store == nil {
		return ""
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return ""
	}
	var in precheckInput
	if err := json.Unmarshal(data, &in); err != nil {
		return ""
	}
	if in.ToolName != "Read" || in.ToolInput.FilePath == "" {
		return ""
	}

	path := in.ToolInput.FilePath
	key := memory.NormalizeCodePath(root, path)

	// Dos superficies que se combinan: la memoria de código (gist) y los errores conocidos
	// del archivo (telemetría, T6.3). Cualquiera puede estar vacía.
	parts := make([]string, 0, 2)
	if m := codeMemoryMessage(store, root, path, key); m != "" {
		parts = append(parts, m)
	}
	if m := telemetryMessage(store, key, path); m != "" {
		parts = append(parts, m)
	}
	if len(parts) == 0 {
		return ""
	}
	return preEnvelope(strings.Join(parts, "\n\n"))
}

// codeMemoryMessage arma el aviso de memoria de código para el archivo (gist fresco,
// desactualizado, o nudge de guardar si es grande y no hay). "" si no aplica.
func codeMemoryMessage(store codeStore, root, path, key string) string {
	cm, ok, err := store.GetCodeMemory(key)
	if err != nil {
		return ""
	}
	if !ok {
		if fileIsLarge(root, path) {
			return fmt.Sprintf("[Musubi — código] No hay gist de «%s». Tras leerlo, guardá uno con musubi_save_code (path, gist, symbols) para no re-leerlo entero en futuros turnos/sesiones.", key)
		}
		return ""
	}
	current, _ := memory.FileFingerprint(root, path)
	if current != "" && current == cm.Fingerprint {
		msg := fmt.Sprintf("[Musubi — código] Ya tenés un gist FRESCO de «%s»: %s", key, cm.Gist)
		if cm.Symbols != "" {
			msg += " | símbolos: " + cm.Symbols
		}
		msg += ". Si solo necesitás una parte, leé el rango puntual en vez del archivo entero (evitás re-pagar la lectura)."
		return msg
	}
	return fmt.Sprintf("[Musubi — código] Tenés un gist de «%s» pero el archivo CAMBIÓ desde entonces. Leé lo necesario y actualizá el gist con musubi_save_code.", key)
}

// telemetryMessage arma el aviso de errores conocidos NO resueltos del archivo (T6.3):
// Musubi recuerda proactivamente "este archivo ya te dio este error, este fue el fix"
// ANTES de que lo edites. "" si no hay. Acota a maxPrecheckTelemetry para no inundar.
func telemetryMessage(store codeStore, key, path string) string {
	logs, err := store.GetUnresolvedTelemetryLogsForFiles([]string{key, path})
	if err != nil || len(logs) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[Musubi — errores conocidos] Este archivo tiene %d error(es) sin resolver registrado(s):", len(logs))
	shown := logs
	if len(shown) > maxPrecheckTelemetry {
		shown = shown[:maxPrecheckTelemetry]
	}
	for _, l := range shown {
		fmt.Fprintf(&b, "\n- [id %d] %s", l.ID, l.ErrorMessage)
		if strings.TrimSpace(l.SuggestedPatch) != "" {
			fmt.Fprintf(&b, " → fix sugerido: %s", l.SuggestedPatch)
		}
	}
	if len(logs) > maxPrecheckTelemetry {
		fmt.Fprintf(&b, "\n(+%d más)", len(logs)-maxPrecheckTelemetry)
	}
	b.WriteString("\nSi lo resolviste, marcalo con musubi_resolve_telemetry {id}.")
	return b.String()
}

// fileIsLarge indica si el archivo supera el umbral (best-effort; false si no se
// puede stat-ear, para no molestar).
func fileIsLarge(root, path string) bool {
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, path)
	}
	fi, err := os.Stat(full)
	if err != nil {
		return false
	}
	return fi.Size() >= umbralArchivoGrande
}

// preEnvelope serializa el envelope de PreToolUse con additionalContext y
// permissionDecision=allow (no bloquea: solo aporta contexto).
func preEnvelope(ctx string) string {
	envelope := map[string]interface{}{
		"hookSpecificOutput": map[string]interface{}{
			"hookEventName":      "PreToolUse",
			"permissionDecision": "allow",
			"additionalContext":  ctx,
		},
	}
	datos, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi precheck: error al serializar: %v\n", err)
		return ""
	}
	return string(datos)
}

// runPrecheck implementa 'musubi precheck [--hook-mode]'. Sin --hook-mode es no-op.
// En hook-mode lee stdin, abre la memoria (best-effort) y escribe el envelope en
// stdout. Errores no fatales van a stderr y sale 0 para no romper la lectura.
func runPrecheck() {
	hookMode := false
	for _, arg := range os.Args[2:] {
		if arg == "--hook-mode" {
			hookMode = true
		}
	}
	if !hookMode {
		return
	}

	root := workspaceDir()
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi precheck: memoria no disponible: %v\n", err)
		os.Exit(0)
	}
	defer engine.Close()

	if out := precheckOutput(engine, root, os.Stdin); out != "" {
		fmt.Println(out)
	}
}
