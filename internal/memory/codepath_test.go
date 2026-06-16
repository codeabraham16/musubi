package memory

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeCodePath(t *testing.T) {
	root := t.TempDir() // raíz absoluta real del SO
	cases := []struct{ in, want string }{
		{filepath.Join(root, "internal", "foo.go"), "internal/foo.go"}, // absoluto bajo root -> relativo
		{filepath.FromSlash("internal/foo.go"), "internal/foo.go"},     // relativo -> limpio
		{filepath.FromSlash("./a/../b/c.go"), "b/c.go"},                // limpieza
	}
	for _, c := range cases {
		if got := NormalizeCodePath(root, c.in); got != c.want {
			t.Errorf("NormalizeCodePath(%q) = %q, quiero %q", c.in, got, c.want)
		}
	}
}

func TestFileFingerprint(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "f.txt")
	if err := os.WriteFile(p, []byte("hola mundo"), 0644); err != nil {
		t.Fatal(err)
	}
	fp1, err := FileFingerprint(root, "f.txt") // path relativo a root
	if err != nil || fp1 == "" {
		t.Fatalf("fingerprint relativo: %q err=%v", fp1, err)
	}
	fp2, err := FileFingerprint(root, p) // path absoluto -> mismo hash
	if err != nil || fp2 != fp1 {
		t.Errorf("abs y rel deben coincidir: %q vs %q (err=%v)", fp2, fp1, err)
	}
	if _, err := FileFingerprint(root, "no-existe.txt"); err == nil {
		t.Error("un archivo inexistente debe devolver error")
	}
}
