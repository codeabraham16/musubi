package memory

import "testing"

// Un dueño con '|' —el separador del valor «dueño|vence»— trababa el candado para siempre: se leía
// otro dueño (no podía renovar ni soltar) y un vencimiento que como texto nunca queda atrás de una
// fecha. Ahora se rechaza con error, y un valor así que ya estuviera guardado no traba a nadie.
//
// Sabotaje que la pone roja: volver a aceptar el dueño con separador.
// arnes: archivo="internal/memory/bajada_lease.go"
// arnes: de="\tif strings.ContainsRune(dueno, '|') {"
// arnes: a="\tif strings.ContainsRune(dueno, '|') && false {"
func TestReclamarBajadaDuenoConSeparadorNoTrabaElCandado(t *testing.T) {
	e := newTestEngine(t)
	if ok, err := e.ReclamarBajada("a|b", 120); err == nil || ok {
		t.Fatalf("un dueño con '|' tenía que rechazarse con error: ok=%v err=%v", ok, err)
	}
	if err := e.SoltarBajada("a|b"); err == nil {
		t.Error("soltar con un dueño con '|' tenía que rechazarse con error")
	}
	// Aunque un valor así ya estuviera guardado (por un binario que lo aceptaba), vencido lo toma otro.
	if _, err := e.db.Exec(`INSERT INTO meta (key, value) VALUES (?, 'a|b|2000-01-01 00:00:00')
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, metaBajadaLease); err != nil {
		t.Fatal(err)
	}
	if ok, err := e.ReclamarBajada("otro", 120); err != nil || !ok {
		t.Fatalf("con un valor viejo trabado, otro dueño no pudo tomar el candado: ok=%v err=%v", ok, err)
	}
}
