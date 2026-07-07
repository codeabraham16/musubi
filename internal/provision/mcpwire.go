package provision

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"musubi/internal/bootstrap"
)

// wireResult describe qué pasó con el .mcp.json del proyecto.
type wireResult struct {
	path    string
	changed bool // ¿el contenido quedó distinto de lo que había? (idempotencia)
}

// wireMCPJSON cablea (idempotente) el .mcp.json de projectDir con DOS entradas: la LOCAL
// ("musubi", stdio al binario, forma portable ${MUSUBI_BIN}) y la del CEREBRO
// ("musubi-cerebro", http al brain, con el bearer por referencia a tokenEnv). Preserva
// cualquier otra entrada/clave existente. El secreto NUNCA toca el archivo (va por ${VAR}).
// Con dryRun no escribe: solo calcula si cambiaría.
func wireMCPJSON(projectDir, brain, tokenEnv, exePath string, dryRun bool) (wireResult, error) {
	res := wireResult{path: filepath.Join(projectDir, ".mcp.json")}

	existing, err := os.ReadFile(res.path)
	if err != nil && !os.IsNotExist(err) {
		return res, fmt.Errorf("no se pudo leer %s: %w", res.path, err)
	}

	// Entrada LOCAL (portable): el command se resuelve por MUSUBI_BIN con la ruta actual de
	// fallback; el daemon toma la raíz del proyecto de CLAUDE_PROJECT_DIR (igual que `setup`).
	local := bootstrap.MCPServerEntry{
		Command: "${MUSUBI_BIN:-" + exePath + "}",
		Args:    []string{"daemon"},
	}
	merged, err := bootstrap.MergeMCPServer(existing, "musubi", local)
	if err != nil {
		return res, err
	}

	// Entrada CEREBRO (remota http): bearer por referencia a la env var (nunca el secreto).
	remote := bootstrap.RemoteEntry("http://"+brain+"/mcp", tokenEnv)
	merged, err = bootstrap.MergeRemoteMCPServer(merged, "musubi-cerebro", remote)
	if err != nil {
		return res, err
	}

	res.changed = !bytes.Equal(bytes.TrimSpace(existing), bytes.TrimSpace(merged))
	if dryRun || !res.changed {
		return res, nil
	}

	if dir := filepath.Dir(res.path); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return res, fmt.Errorf("no se pudo crear %s: %w", dir, err)
		}
	}
	if err := os.WriteFile(res.path, merged, 0o644); err != nil {
		return res, fmt.Errorf("no se pudo escribir %s: %w", res.path, err)
	}
	return res, nil
}
