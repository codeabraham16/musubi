package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// maxDownloadBytes es el tope de tamaño de un asset descargado (backstop anti-DoS
// de memoria). 512 MiB cubre con holgura cualquier binario de Musubi.
const maxDownloadBytes = 512 << 20

// Updater consulta releases en GitHub y descarga assets. Las bases son
// configurables para poder testear con httptest.
type Updater struct {
	Owner   string
	Repo    string
	APIBase string // ej. https://api.github.com
	DLBase  string // ej. https://github.com
	HTTP    *http.Client
}

// New crea un Updater con los endpoints públicos de GitHub.
func New(owner, repo string) *Updater {
	return &Updater{
		Owner:   owner,
		Repo:    repo,
		APIBase: "https://api.github.com",
		DLBase:  "https://github.com",
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
}

// LatestVersion devuelve el tag del último release (ej. "v0.2.5").
func (u *Updater) LatestVersion(ctx context.Context) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", u.APIBase, u.Owner, u.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("error al construir pedido a GitHub: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := u.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("error al consultar el último release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub devolvió status %d al consultar releases", resp.StatusCode)
	}

	var out struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("error al decodificar respuesta de GitHub: %w", err)
	}
	if out.TagName == "" {
		return "", fmt.Errorf("el release no trae tag_name")
	}
	return out.TagName, nil
}

// Download baja un asset del release tag dado.
func (u *Updater) Download(ctx context.Context, tag, asset string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s/%s/releases/download/%s/%s", u.DLBase, u.Owner, u.Repo, tag, asset)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error al construir descarga: %w", err)
	}
	resp, err := u.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error al descargar %s: %w", asset, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d al descargar %s", resp.StatusCode, asset)
	}
	// Tope de tamaño: backstop ante un server/MITM que intente agotar memoria.
	return io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes))
}

// Apply reemplaza el ejecutable en exePath por newBinary. En Windows no se puede
// sobrescribir un .exe en uso, pero sí renombrarlo: se mueve el actual a ".old",
// se coloca el nuevo, y se intenta limpiar el viejo (best-effort). El binario
// nuevo toma efecto en la próxima ejecución.
func Apply(exePath string, newBinary []byte) error {
	tmp := exePath + ".new"
	old := exePath + ".old"

	if err := os.WriteFile(tmp, newBinary, 0o755); err != nil {
		return fmt.Errorf("error al escribir el binario nuevo: %w", err)
	}

	_ = os.Remove(old) // limpiar un .old previo si quedó

	if err := os.Rename(exePath, old); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("error al apartar el binario actual: %w", err)
	}
	if err := os.Rename(tmp, exePath); err != nil {
		// Intentar restaurar el original.
		_ = os.Rename(old, exePath)
		os.Remove(tmp)
		return fmt.Errorf("error al colocar el binario nuevo: %w", err)
	}

	_ = os.Remove(old) // best-effort; en Windows puede fallar si está en uso
	return nil
}
