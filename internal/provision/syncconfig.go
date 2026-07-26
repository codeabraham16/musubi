package provision

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"musubi/internal/config"
)

// syncBlockRe captura el bloque `sync:` de nivel superior COMPLETO (la línea `sync:` más sus líneas
// indentadas) para poder REEMPLAZARLO en su lugar. No basta con detectar `^sync:` y anexar: el config
// que deja `ensureWorkspace` (config.Default().Marshal()) YA trae un bloque `sync:` con enabled:false,
// y anexar otro crearía una clave YAML duplicada (parseo fallido). Ver auditoría 2026-07-26 #2.
var syncBlockRe = regexp.MustCompile(`(?m)^sync:.*(?:\n[ \t]+.*)*\n?`)

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

	// "Ya configurado" NO es "existe un bloque sync:": el default trae uno con enabled:false. Se
	// consulta el estado REAL (parseado, con defaults aplicados): sólo si el sync ya está habilitado y
	// con central_url no vacío se lo deja en paz. Así provision funciona sobre un proyecto ya inicializado.
	if cfg, lerr := config.Load(projectDir); lerr == nil && cfg.Sync.Enabled && cfg.Sync.CentralURL != "" {
		return StepResult{Name: "sync-config", Status: StatusOK, Detail: "sync ya habilitado hacia " + cfg.Sync.CentralURL}
	}

	block := fmt.Sprintf("# Sync saliente del cerebro híbrido: sube solo la memoria 'shared' al cerebro central.\n"+
		"sync:\n"+
		"  enabled: true\n"+
		"  central_url: http://%s\n"+
		"  auth_token_env: %s\n"+
		"  drain_interval_seconds: 30\n"+
		"  allow_insecure_token: true  # http sobre el tailnet (WireGuard ya cifra el transporte)\n",
		brain, tokenEnv)

	if dryRun {
		return StepResult{Name: "sync-config", Status: StatusTodo, Detail: "habilitaría el sync saliente (auto-sube 'shared' al cerebro) en " + cfgPath}
	}

	if err := os.MkdirAll(filepath.Dir(cfgPath), 0o755); err != nil {
		return StepResult{Name: "sync-config", Status: StatusError, Detail: "no se pudo crear .musubi/: " + err.Error()}
	}

	// Si ya hay un bloque `sync:` (el deshabilitado del default), se REEMPLAZA en su lugar para no
	// duplicar la clave. Si no hay ninguno, se anexa al final.
	content := existing
	if syncBlockRe.Match(content) {
		content = syncBlockRe.ReplaceAll(content, []byte(block))
	} else {
		if len(bytes.TrimSpace(content)) == 0 {
			content = []byte("# Configuración de Musubi (bootstrap por `musubi provision`).\n")
		} else if !bytes.HasSuffix(content, []byte("\n")) {
			content = append(content, '\n')
		}
		content = append(content, '\n')
		content = append(content, []byte(block)...)
	}
	if err := os.WriteFile(cfgPath, content, 0o644); err != nil {
		return StepResult{Name: "sync-config", Status: StatusError, Detail: "no se pudo escribir " + cfgPath + ": " + err.Error()}
	}
	return StepResult{Name: "sync-config", Status: StatusDone, Detail: "sync saliente habilitado (auto-sube 'shared' al cerebro) en " + cfgPath}
}
