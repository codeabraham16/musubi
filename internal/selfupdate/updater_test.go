package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestUpdaterLatestAndDownload(t *testing.T) {
	binary := []byte("binario nuevo de musubi")
	sum := sha256.Sum256(binary)
	shaLine := hex.EncodeToString(sum[:]) + "  Musubi.exe"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/acme/musubi/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"tag_name":"v0.2.5"}`))
		case "/acme/musubi/releases/download/v0.2.5/Musubi.exe":
			w.Write(binary)
		case "/acme/musubi/releases/download/v0.2.5/Musubi.exe.sha256":
			w.Write([]byte(shaLine))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	u := New("acme", "musubi")
	u.APIBase = srv.URL
	u.DLBase = srv.URL
	u.HTTP = srv.Client()

	ctx := context.Background()
	tag, err := u.LatestVersion(ctx)
	if err != nil || tag != "v0.2.5" {
		t.Fatalf("LatestVersion = %q,%v quiero v0.2.5", tag, err)
	}

	data, err := u.Download(ctx, tag, "Musubi.exe")
	if err != nil || string(data) != string(binary) {
		t.Fatalf("Download error o contenido distinto: %v", err)
	}

	sha, err := u.Download(ctx, tag, "Musubi.exe.sha256")
	if err != nil {
		t.Fatalf("Download sha error: %v", err)
	}
	if err := VerifyChecksum(data, string(sha)); err != nil {
		t.Errorf("checksum del binario descargado debería validar: %v", err)
	}
}

func TestApplyReplacesBinary(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "musubi.exe")
	if err := os.WriteFile(exe, []byte("binario viejo"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := Apply(exe, []byte("binario nuevo")); err != nil {
		t.Fatalf("Apply error: %v", err)
	}

	got, err := os.ReadFile(exe)
	if err != nil {
		t.Fatalf("no se pudo leer el exe tras Apply: %v", err)
	}
	if string(got) != "binario nuevo" {
		t.Errorf("esperaba binario reemplazado, obtuve %q", string(got))
	}
}
