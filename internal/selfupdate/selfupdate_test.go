package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestNormalizeVersion(t *testing.T) {
	cases := map[string]string{
		"v0.2.5":  "0.2.5",
		"0.2.5":   "0.2.5",
		"  v1.0 ": "1.0",
		"V2.3.4":  "2.3.4",
	}
	for in, want := range cases {
		if got := NormalizeVersion(in); got != want {
			t.Errorf("NormalizeVersion(%q) = %q, quiero %q", in, got, want)
		}
	}
}

func TestNeedsUpdate(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"v0.2.4", "v0.2.5", true},
		{"0.2.5", "v0.2.5", false},  // misma versión (normalizada)
		{"v0.2.5", "v0.2.4", false}, // latest no es mayor lexicográfico -> no actualizar
		{"dev", "v0.2.5", false},    // build de desarrollo: no molestar
		{"v0.2.4", "", false},       // latest desconocido
	}
	for _, c := range cases {
		if got := NeedsUpdate(c.current, c.latest); got != c.want {
			t.Errorf("NeedsUpdate(%q,%q) = %v, quiero %v", c.current, c.latest, got, c.want)
		}
	}
}

func TestAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch, want string
	}{
		{"windows", "amd64", "Musubi.exe"},
		{"windows", "arm64", "Musubi-arm64.exe"},
		{"linux", "amd64", "musubi-linux-amd64"},
		{"linux", "arm64", "musubi-linux-arm64"},
		{"darwin", "amd64", "musubi-darwin-amd64"},
		{"darwin", "arm64", "musubi-darwin-arm64"},
	}
	for _, c := range cases {
		got, err := AssetName(c.goos, c.goarch)
		if err != nil || got != c.want {
			t.Errorf("AssetName(%s,%s) = %q,%v quiero %q", c.goos, c.goarch, got, err, c.want)
		}
	}
	if _, err := AssetName("plan9", "amd64"); err == nil {
		t.Error("plataforma no soportada debería dar error")
	}
}

func TestVerifyChecksum(t *testing.T) {
	data := []byte("contenido binario de prueba")
	sum := sha256.Sum256(data)
	hexsum := hex.EncodeToString(sum[:])

	// Formato de sha256sum: "<hash>  <archivo>".
	if err := VerifyChecksum(data, hexsum+"  Musubi.exe"); err != nil {
		t.Errorf("checksum válido no debería fallar: %v", err)
	}
	// Solo el hash también vale.
	if err := VerifyChecksum(data, hexsum); err != nil {
		t.Errorf("checksum (solo hash) no debería fallar: %v", err)
	}
	// Hash incorrecto.
	if err := VerifyChecksum(data, "deadbeef  Musubi.exe"); err == nil {
		t.Error("checksum incorrecto debería fallar")
	}
}
