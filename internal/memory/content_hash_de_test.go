package memory

import "testing"

// TestContentHashDeLeeLaFilaOEntiendeQueNoHay fija el contrato del accesor que el recibo del push
// usa para confirmar persistencia CONTRA la base: una fila existente devuelve su content_hash
// almacenado con found=true, y un id sin fila devuelve found=false SIN error (no hay fila ≠ fallo).
func TestContentHashDeLeeLaFilaOEntiendeQueNoHay(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer e.Close()

	const id = "obs-recibo-1"
	const contenido = "una nota reusable para el recibo del push"
	if err := e.SaveObservation(id, "sync/recibo", contenido, nil); err != nil {
		t.Fatalf("SaveObservation: %v", err)
	}

	hash, found, err := e.ContentHashDe(id)
	if err != nil {
		t.Fatalf("ContentHashDe(existente): %v", err)
	}
	if !found {
		t.Fatal("found=false para una observación recién guardada")
	}
	// Lee la COLUMNA content_hash, que es ContentHash(contenido) para un guardado local sin redacción.
	if want := ContentHash(contenido); hash != want {
		t.Fatalf("hash=%q, esperaba %q (el content_hash de la fila)", hash, want)
	}

	// Un id que no existe: found=false, sin error. Es la señal de DROP que el cliente dead-lettea.
	if gotHash, found, err := e.ContentHashDe("no-existe"); err != nil || found || gotHash != "" {
		t.Fatalf("ContentHashDe(inexistente) = (%q, %v, %v); esperaba (\"\", false, nil)", gotHash, found, err)
	}
}
