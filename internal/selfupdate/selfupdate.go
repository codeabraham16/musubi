// Package selfupdate permite que el binario de Musubi consulte el último release
// en GitHub, verifique su checksum y se auto-reemplace. Pure-Go, sin dependencias.
package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// NormalizeVersion saca espacios y el prefijo 'v'/'V' de una versión.
func NormalizeVersion(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 0 && (s[0] == 'v' || s[0] == 'V') {
		s = s[1:]
	}
	return s
}

// NeedsUpdate indica si latest es estrictamente más nueva que current. Devuelve
// false si latest está vacío o si current es un build de desarrollo ("dev").
func NeedsUpdate(current, latest string) bool {
	if strings.TrimSpace(latest) == "" {
		return false
	}
	nc := NormalizeVersion(current)
	if nc == "" || nc == "dev" {
		return false
	}
	return compareVersions(NormalizeVersion(latest), nc) > 0
}

// compareVersions compara dos versiones punteadas (ej. "0.2.5"): -1, 0 o 1.
func compareVersions(a, b string) int {
	pa := strings.Split(a, ".")
	pb := strings.Split(b, ".")
	n := len(pa)
	if len(pb) > n {
		n = len(pb)
	}
	for i := 0; i < n; i++ {
		va, vb := 0, 0
		if i < len(pa) {
			va, _ = strconv.Atoi(pa[i])
		}
		if i < len(pb) {
			vb, _ = strconv.Atoi(pb[i])
		}
		if va != vb {
			if va < vb {
				return -1
			}
			return 1
		}
	}
	return 0
}

// AssetName devuelve el nombre del asset de release para la plataforma dada,
// alineado con la matriz de .github/workflows/release.yml.
func AssetName(goos, goarch string) (string, error) {
	switch goos {
	case "windows":
		if goarch == "arm64" {
			return "Musubi-arm64.exe", nil
		}
		return "Musubi.exe", nil
	case "linux":
		return "musubi-linux-" + goarch, nil
	case "darwin":
		return "musubi-darwin-" + goarch, nil
	default:
		return "", fmt.Errorf("plataforma no soportada para auto-update: %s/%s", goos, goarch)
	}
}

// VerifyChecksum verifica que el sha256 de data coincida con shaLine, que puede
// ser solo el hash o el formato de sha256sum ("<hash>  <archivo>").
func VerifyChecksum(data []byte, shaLine string) error {
	fields := strings.Fields(shaLine)
	if len(fields) == 0 {
		return fmt.Errorf("checksum vacío")
	}
	expected := strings.ToLower(fields[0])
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != expected {
		return fmt.Errorf("checksum no coincide: esperado %s, obtenido %s", expected, got)
	}
	return nil
}
