package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

// codepath.go: helpers puros (sin estado del motor) para la memoria de código.
// Normalizan la clave de path y computan el fingerprint del contenido de un
// archivo. Los comparten la capa MCP (tools) y el hook PreToolUse (precheck), de
// modo que lo que guarda una tool sea encontrable por el hook.

// NormalizeCodePath devuelve una clave de path estable: relativa a root cuando el
// path cae dentro de root, limpia, y con separadores "/" (consistente en todo SO).
func NormalizeCodePath(root, path string) string {
	p := path
	if filepath.IsAbs(p) {
		if rel, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(rel, "..") {
			p = rel
		}
	}
	return filepath.ToSlash(filepath.Clean(p))
}

// FileFingerprint devuelve el sha256 (hex) del contenido actual del archivo,
// resolviendo un path relativo contra root. Es la señal de frescura de la memoria
// de código: si cambia el contenido, cambia el fingerprint.
func FileFingerprint(root, path string) (string, error) {
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, path)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
