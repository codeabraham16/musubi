package provision

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// syncKeyRe detecta un bloque `sync:` de nivel superior ya presente en el config.yaml.
var syncKeyRe = regexp.MustCompile(`(?m)^sync:`)

// ensureSyncConfig deja el bloque `sync:` en el .musubi/config.yaml del proyecto para que el
// daemon LOCAL suba solo la memoria `shared` al cerebro central (outbox de F2). Sin esto, el
// `.mcp.json` conecta pero el auto-sync local→central queda apagado (era el hueco que hacía
// falta un paso manual). Idempotente: si el bloque ya está, no lo pisa. Incluye
// `allow_insecure_token: true` porque el central es http:// sobre el tailnet (WireGuard ya
// cifra el transporte) y el cliente de sync es fail-closed sin ese opt-in.
func ensureSyncConfig(projectDir, brain, tokenEnv string, dryRun bool) StepResult {
	cfgPath := filepath.Join(projectDir, ".musubi", "config.yaml")

	existing, err := os.ReadFile(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		return StepResult{Name: "sync-config", Status: StatusError, Detail: "no se pudo leer " + cfgPath + ": " + err.Error()}
	}
	if syncKeyRe.Match(existing) {
		return StepResult{Name: "sync-config", Status: StatusOK, Detail: "bloque sync: ya presente en .musubi/config.yaml"}
	}

	block := fmt.Sprintf("\n# Sync saliente del cerebro híbrido: sube solo la memoria 'shared' al cerebro central.\n"+
		"sync:\n"+
		"  enabled: true\n"+
		"  central_url: http://%s\n"+
		"  auth_token_env: %s\n"+
		"  drain_interval_seconds: 30\n"+
		"  allow_insecure_token: true  # http sobre el tailnet (WireGuard ya cifra el transporte)\n",
		brain, tokenEnv)

	if dryRun {
		return StepResult{Name: "sync-config", Status: StatusTodo, Detail: "agregaría el bloque sync: (auto-sube 'shared' al cerebro) a " + cfgPath}
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return StepResult{Name: "sync-config", Status: StatusError, Detail: "no se pudo crear .musubi/: " + err.Error()}
	}
	content := existing
	if len(bytes.TrimSpace(content)) == 0 {
		content = []byte("# Configuración de Musubi (bootstrap por `musubi provision`).\n")
	} else if !bytes.HasSuffix(content, []byte("\n")) {
		content = append(content, '\n')
	}
	content = append(content, []byte(block)...)
	if err := os.WriteFile(cfgPath, content, 0o644); err != nil {
		return StepResult{Name: "sync-config", Status: StatusError, Detail: "no se pudo escribir " + cfgPath + ": " + err.Error()}
	}
	return StepResult{Name: "sync-config", Status: StatusDone, Detail: "bloque sync: agregado (auto-sube 'shared' al cerebro) en " + cfgPath}
}
