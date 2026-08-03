package mcp

// Tests del hosting del auto-update del cuerpo (GET /body/<archivo>): sirve manifest +
// binarios desde bodyDir, sin auth, con guarda anti-traversal.

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"musubi/internal/embedding"
)

func newBodyHTTPServer(t *testing.T, bodyDir string) *httptest.Server {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, loopbackOnly: true, bodyDir: bodyDir}))
	t.Cleanup(ts.Close)
	return ts
}

func TestBodyUpdateServesFile(t *testing.T) {
	dir := t.TempDir()
	want := []byte(`{"latest":"0.99.1","url":"x","sha256":"y"}`)
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), want, 0o644); err != nil {
		t.Fatal(err)
	}
	ts := newBodyHTTPServer(t, dir)

	resp, err := http.Get(ts.URL + "/body/manifest.json") // sin auth
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, esperaba 200", resp.StatusCode)
	}
	got, _ := io.ReadAll(resp.Body)
	if string(got) != string(want) {
		t.Errorf("contenido = %q, esperaba %q", got, want)
	}
}

func TestBodyUpdateBlocksTraversal(t *testing.T) {
	dir := t.TempDir()
	// Un secreto FUERA de bodyDir, en el padre.
	secret := filepath.Join(filepath.Dir(dir), "secret.txt")
	_ = os.WriteFile(secret, []byte("TOP-SECRET"), 0o644)
	t.Cleanup(func() { _ = os.Remove(secret) })
	ts := newBodyHTTPServer(t, dir)

	for _, p := range []string{
		"/body/../secret.txt",
		"/body/..%2fsecret.txt",
		"/body/subdir/../../secret.txt",
	} {
		resp, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatalf("GET %s: %v", p, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK && string(body) == "TOP-SECRET" {
			t.Fatalf("TRAVERSAL: %s filtró el secreto de afuera de bodyDir", p)
		}
	}
}

func TestBodyUpdateMissingIs404(t *testing.T) {
	ts := newBodyHTTPServer(t, t.TempDir())
	resp, err := http.Get(ts.URL + "/body/no-existe.bin")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("archivo inexistente = %d, esperaba 404", resp.StatusCode)
	}
}

func TestBodyUpdatePostIs405(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "x"), []byte("y"), 0o644)
	ts := newBodyHTTPServer(t, dir)
	resp, err := http.Post(ts.URL+"/body/x", "application/octet-stream", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("POST = %d, esperaba 405", resp.StatusCode)
	}
}

func TestBodyUpdateDisabledWhenNoDir(t *testing.T) {
	ts := newBodyHTTPServer(t, "") // sin bodyDir → ruta no registrada
	resp, err := http.Get(ts.URL + "/body/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("con bodyDir vacío, /body/ = %d, esperaba 404 (ruta ausente)", resp.StatusCode)
	}
}
