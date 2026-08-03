package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAllowedFetchURL(t *testing.T) {
	ok := []string{
		"http://127.0.0.1:8080/x",
		"http://100.79.126.62:7717/body/manifest.json", // IP tailnet (el central)
		"https://100.64.0.1/y",
	}
	for _, u := range ok {
		if err := allowedFetchURL(u); err != nil {
			t.Errorf("debería permitir %q: %v", u, err)
		}
	}
	bad := []string{
		"http://8.8.8.8/x",        // internet público
		"http://example.com/x",    // hostname (no IP)
		"ftp://100.64.0.1/x",      // esquema no http
		"http://192.168.0.10/x",   // LAN, no tailnet
		"file:///etc/passwd",      // no http
		"http://[::1]enmascarado", // inválida
	}
	for _, u := range bad {
		if err := allowedFetchURL(u); err == nil {
			t.Errorf("debería RECHAZAR %q", u)
		}
	}
}

func TestFetchLoopback(t *testing.T) {
	payload := []byte("bytes-del-central-v0.99.1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	// httptest sirve en 127.0.0.1 → permitido por la allowlist.
	var buf bytes.Buffer
	if err := fetch(srv.URL+"/body/bin", &buf); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if !bytes.Equal(buf.Bytes(), payload) {
		t.Errorf("fetch trajo %q, esperaba %q", buf.Bytes(), payload)
	}
}

func TestFetchRejectsBeforeNetwork(t *testing.T) {
	// Un destino no permitido falla SIN salir a la red.
	var buf bytes.Buffer
	err := fetch("http://8.8.8.8/x", &buf)
	if err == nil || !strings.Contains(err.Error(), "tailnet") {
		t.Fatalf("debería rechazar por allowlist, dio: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("no debería haber escrito nada")
	}
}
