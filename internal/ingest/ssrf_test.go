package ingest

import (
	"context"
	"net"
	"testing"
)

func TestIsBlockedIP(t *testing.T) {
	blocked := []string{
		"127.0.0.1", "::1", // loopback
		"10.0.0.5", "192.168.1.1", "172.16.0.1", // privadas
		"169.254.1.1", "169.254.169.254", // link-local + metadata
		"100.79.126.62", "100.64.0.0", "100.127.255.255", // CGNAT (tailnet)
		"0.0.0.0",            // unspecified
		"fd00::1", "fe80::1", // ULA + link-local v6
	}
	for _, s := range blocked {
		if !isBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s DEBERÍA estar bloqueada", s)
		}
	}
	public := []string{
		"1.1.1.1", "8.8.8.8", "142.250.72.14",
		"2606:4700:4700::1111",
		"100.63.255.255", "100.128.0.1", // justo fuera del rango CGNAT
	}
	for _, s := range public {
		if isBlockedIP(net.ParseIP(s)) {
			t.Errorf("%s NO debería estar bloqueada", s)
		}
	}
	if !isBlockedIP(nil) {
		t.Error("nil debe estar bloqueada")
	}
}

func TestAssertPublicHostRechazaInternos(t *testing.T) {
	ctx := context.Background()
	// Literales internos (sin DNS) → error.
	for _, h := range []string{"127.0.0.1", "10.1.2.3", "192.168.0.1", "169.254.169.254", "100.79.126.62", "127.0.0.1:7717"} {
		if err := assertPublicHost(ctx, h); err == nil {
			t.Errorf("assertPublicHost(%q) debería fallar", h)
		}
	}
	// IP pública literal → pasa (sin DNS).
	if err := assertPublicHost(ctx, "1.1.1.1"); err != nil {
		t.Errorf("1.1.1.1 debería pasar: %v", err)
	}
}

func TestRegistryGuardaSSRF(t *testing.T) {
	reg := NewRegistry(stubExtractor{"MEDIA"}, stubExtractor{"ARTICLE"})
	// Con la guarda, un destino interno se rechaza ANTES de tocar la red.
	if _, err := reg.Extract(context.Background(), "http://127.0.0.1:7717/mcp", Options{RestrictToPublic: true}); err == nil {
		t.Fatal("con RestrictToPublic, un destino interno debe ser rechazado")
	}
	// Sin la guarda (uso local de confianza), rutea normal (al stub de artículo).
	res, err := reg.Extract(context.Background(), "http://127.0.0.1:7717/mcp", Options{})
	if err != nil || res.Platform != "ARTICLE" {
		t.Fatalf("sin guarda debería rutear al stub: err=%v res=%+v", err, res)
	}
}
