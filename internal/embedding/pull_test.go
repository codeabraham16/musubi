package embedding

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestPullModel(t *testing.T) {
	content := []byte("tabla de embeddings de prueba")
	sum := sha256.Sum256(content)
	hexsum := hex.EncodeToString(sum[:])

	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	spec := ModelSpec{Name: "test", Files: []ModelFile{
		{Name: "model.safetensors", URL: srv.URL + "/model.safetensors", SHA256: hexsum, Size: int64(len(content))},
	}}

	dir := t.TempDir()
	if err := PullModel(dir, spec, srv.Client(), nil); err != nil {
		t.Fatalf("PullModel: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "model.safetensors"))
	if err != nil || string(got) != string(content) {
		t.Fatalf("archivo mal descargado: %q err=%v", got, err)
	}
	// No debe quedar el .part tras verificar.
	if _, err := os.Stat(filepath.Join(dir, "model.safetensors.part")); !os.IsNotExist(err) {
		t.Error(".part no debería quedar tras una descarga exitosa")
	}

	// Idempotente: una segunda pasada NO re-descarga (el checksum ya coincide).
	before := atomic.LoadInt32(&hits)
	if err := PullModel(dir, spec, srv.Client(), nil); err != nil {
		t.Fatalf("PullModel (2da): %v", err)
	}
	if atomic.LoadInt32(&hits) != before {
		t.Error("una segunda descarga no debería golpear la red si el checksum coincide")
	}
}

func TestPullModelRejectsBadChecksum(t *testing.T) {
	content := []byte("contenido")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()
	spec := ModelSpec{Name: "test", Files: []ModelFile{
		{Name: "x.bin", URL: srv.URL + "/x", SHA256: "00deadbeef", Size: int64(len(content))},
	}}
	dir := t.TempDir()
	if err := PullModel(dir, spec, srv.Client(), nil); err == nil {
		t.Fatal("un checksum incorrecto debería fallar (fail-closed)")
	}
	// El archivo final NO debe existir (sólo el .part queda para diagnóstico).
	if _, err := os.Stat(filepath.Join(dir, "x.bin")); !os.IsNotExist(err) {
		t.Error("no debería quedar el archivo final con checksum incorrecto")
	}
}

func TestPullModelRejectsBadSize(t *testing.T) {
	content := []byte("doce bytes!!")
	sum := sha256.Sum256(content)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()
	spec := ModelSpec{Name: "test", Files: []ModelFile{
		{Name: "x.bin", URL: srv.URL + "/x", SHA256: hex.EncodeToString(sum[:]), Size: 9999},
	}}
	if err := PullModel(t.TempDir(), spec, srv.Client(), nil); err == nil {
		t.Fatal("un tamaño inesperado debería fallar")
	}
}

func TestPullModelHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "no está", http.StatusNotFound)
	}))
	defer srv.Close()
	spec := ModelSpec{Name: "test", Files: []ModelFile{
		{Name: "x.bin", URL: srv.URL + "/x", SHA256: "abc", Size: 1},
	}}
	if err := PullModel(t.TempDir(), spec, srv.Client(), nil); err == nil {
		t.Fatal("un 404 debería fallar")
	}
}

// Con client nil, PullModel usa el cliente de fallback IPv4 y descarga igual (valida el
// cableado del path real de la CLI contra un server local).
func TestPullModelNilClientUsesFallback(t *testing.T) {
	content := []byte("contenido con cliente default")
	sum := sha256.Sum256(content)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()
	spec := ModelSpec{Name: "test", Files: []ModelFile{
		{Name: "x.bin", URL: srv.URL + "/x", SHA256: hex.EncodeToString(sum[:]), Size: int64(len(content))},
	}}
	if err := PullModel(t.TempDir(), spec, nil, nil); err != nil {
		t.Fatalf("PullModel con client nil (fallback): %v", err)
	}
}

// El cliente de fallback hace un GET normal sin regresiones (dial dual-stack a 127.0.0.1 OK).
func TestIPv4FallbackClientWorks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()
	resp, err := ipv4FallbackClient(30 * time.Second).Get(srv.URL)
	if err != nil {
		t.Fatalf("GET con cliente de fallback: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status inesperado: %d", resp.StatusCode)
	}
}

// isNetUnreachable reconoce el "IPv6 sin ruta" por errno y por texto, y NO dispara con
// errores comunes (para no reintentar de gusto).
func TestIsNetUnreachable(t *testing.T) {
	unreach := &net.OpError{Op: "dial", Net: "tcp", Err: os.NewSyscallError("connect", syscall.ENETUNREACH)}
	if !isNetUnreachable(unreach) {
		t.Error("ENETUNREACH debería contar como inalcanzable")
	}
	if !isNetUnreachable(errors.New("dial tcp [2600::1]:443: connect: network is unreachable")) {
		t.Error("el texto 'network is unreachable' debería contar (fallback)")
	}
	if isNetUnreachable(errors.New("boom")) {
		t.Error("un error cualquiera no debería contar como inalcanzable")
	}
}

// El registro trae la tabla multilingüe recomendada con checksum pinneado.
func TestKnownModelsHasMultilingual(t *testing.T) {
	spec, ok := KnownModels["potion-multilingual-128M"]
	if !ok {
		t.Fatal("falta potion-multilingual-128M en el registro")
	}
	if len(spec.Files) != 2 {
		t.Fatalf("esperaba 2 archivos (safetensors + tokenizer), obtuve %d", len(spec.Files))
	}
	for _, f := range spec.Files {
		if len(f.SHA256) != 64 {
			t.Errorf("%s: SHA-256 debería tener 64 hex, tiene %d", f.Name, len(f.SHA256))
		}
		if f.Size <= 0 {
			t.Errorf("%s: tamaño debería estar pinneado", f.Name)
		}
	}
}
