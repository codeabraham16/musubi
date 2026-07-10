package mcp

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeRegAt escribe el YAML del registro con un mtime explícito (determinista, sin depender de
// la granularidad del reloj/FS).
func writeRegAt(t *testing.T, path, body string, mod time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mod, mod); err != nil {
		t.Fatal(err)
	}
}

// TestReloadableRegistryHotRevoke valida la revocación/alta EN CALIENTE (Track 18): editar
// principals.yaml surte efecto sin reiniciar. Se revoca a alice y se da de alta a bob; tras la
// recarga, el token de alice deja de autenticar y el de bob empieza.
func TestReloadableRegistryHotRevoke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "principals.yaml")
	aliceTok, bobTok := "alice-token", "bob-token"
	t0 := time.Unix(1_000_000, 0)

	body1 := "principals:\n  - name: alice\n    token_sha256: \"" + hashToken(aliceTok) + "\"\n    project_id: crm\n    role: writer\n"
	writeRegAt(t, path, body1, t0)

	reg, err := loadPrincipals(path, "")
	if err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(path)
	rr := newReloadableRegistry(path, "", reg, fi.ModTime())

	if _, ok := rr.resolve(aliceTok); !ok {
		t.Fatal("alice debía resolver inicialmente")
	}

	// Revocar alice + alta bob, con mtime posterior.
	body2 := "principals:\n  - name: bob\n    token_sha256: \"" + hashToken(bobTok) + "\"\n    project_id: web\n    role: reader\n"
	writeRegAt(t, path, body2, t0.Add(time.Minute))

	rr.reloadIfChanged()

	if _, ok := rr.resolve(aliceTok); ok {
		t.Error("alice debía quedar revocada tras la recarga en caliente")
	}
	if p, ok := rr.resolve(bobTok); !ok || p.Role != RoleReader {
		t.Errorf("bob debía resolver tras la recarga: %+v ok=%v", p, ok)
	}
}

// TestReloadableRegistryKeepsSnapshotOnBadReload valida el fail-safe: un archivo a medio editar
// (malformado, o con un reader sin project_id) NO deja al equipo afuera — se conserva el snapshot
// vigente hasta que el archivo vuelva a ser válido.
func TestReloadableRegistryKeepsSnapshotOnBadReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "principals.yaml")
	aliceTok := "alice-token"
	t0 := time.Unix(2_000_000, 0)

	body := "principals:\n  - name: alice\n    token_sha256: \"" + hashToken(aliceTok) + "\"\n    project_id: crm\n    role: writer\n"
	writeRegAt(t, path, body, t0)
	reg, err := loadPrincipals(path, "")
	if err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(path)
	rr := newReloadableRegistry(path, "", reg, fi.ModTime())

	// Escribir un YAML roto con mtime posterior.
	writeRegAt(t, path, "principals: [::::", t0.Add(time.Minute))
	rr.reloadIfChanged()

	// alice sigue autenticando (se conservó el snapshot; una recarga rota no revoca a nadie).
	if _, ok := rr.resolve(aliceTok); !ok {
		t.Error("una recarga fallida NO debía revocar el snapshot vigente")
	}
}
