package repovivo

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// obsConRemoto arma un observador sobre un repo en vivo con remoto bare "central".
func obsConRemoto(t *testing.T, intervalo time.Duration) (o *observador, f *fotografo, dir, bare string) {
	t.Helper()
	f, dir, remoto, bare := repoConRemoto(t)
	tr, err := nuevoTransporte(f.git, remoto, refTest)
	if err != nil {
		t.Fatal(err)
	}
	return nuevoObservador(f, tr, intervalo), f, dir, bare
}

func TestObservadorPublicaFotoNueva(t *testing.T) {
	o, _, _, bare := obsConRemoto(t, time.Second)
	pub, err := o.tick(context.Background())
	if err != nil {
		t.Fatalf("tick: %v", err)
	}
	if !pub {
		t.Error("el primer tick debía publicar una foto")
	}
	if shaEnBare(t, bare, refTest) == "" {
		t.Error("la ref remota quedó vacía: no se publicó nada")
	}
}

// TestObservadorDedupeNoPublicaSinCambios: si el working tree no cambió, el segundo tick NO crea
// commit ni empuja. Es el invariante que evita el push espurio por tick.
func TestObservadorDedupeNoPublicaSinCambios(t *testing.T) {
	o, _, _, bare := obsConRemoto(t, time.Second)
	if _, err := o.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	sha1 := shaEnBare(t, bare, refTest)

	pub, err := o.tick(context.Background())
	if err != nil {
		t.Fatalf("segundo tick: %v", err)
	}
	if pub {
		t.Error("el segundo tick sin cambios NO debía publicar")
	}
	if got := shaEnBare(t, bare, refTest); got != sha1 {
		t.Errorf("la ref remota se movió sin cambios en el working tree: %q → %q", sha1, got)
	}
}

// TestObservadorPublicaTrasCambio: cuando el working tree cambia, el tick siguiente publica una
// foto nueva (sha distinto).
func TestObservadorPublicaTrasCambio(t *testing.T) {
	o, _, dir, bare := obsConRemoto(t, time.Second)
	if _, err := o.tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	sha1 := shaEnBare(t, bare, refTest)

	if err := os.WriteFile(filepath.Join(dir, "nuevo.txt"), []byte("contenido nuevo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pub, err := o.tick(context.Background())
	if err != nil {
		t.Fatalf("tick tras cambio: %v", err)
	}
	if !pub {
		t.Error("el tick tras un cambio debía publicar")
	}
	if got := shaEnBare(t, bare, refTest); got == sha1 {
		t.Error("la ref remota no se movió pese al cambio en el working tree")
	}
}

// TestObservadorSalteaOperacionEnVuelo: con una operación git a medio camino (MERGE_HEAD), el tick
// se saltea en silencio (R7): ni error, ni publicación.
func TestObservadorSalteaOperacionEnVuelo(t *testing.T) {
	o, f, _, bare := obsConRemoto(t, time.Second)
	if err := os.WriteFile(filepath.Join(f.gitDir, "MERGE_HEAD"), []byte("deadbeef\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pub, err := o.tick(context.Background())
	if err != nil {
		t.Errorf("una operación en vuelo no es error, debía saltearse: %v", err)
	}
	if pub {
		t.Error("con operación en vuelo no se debía publicar")
	}
	if got := shaEnBare(t, bare, refTest); got != "" {
		t.Errorf("se publicó pese a la operación en vuelo: %q", got)
	}
}

// TestObservadorReintentaTrasFalloDeRed: si el push falla (remoto inalcanzable), ultimoTree NO
// avanza, así que el tick siguiente —ya con el remoto disponible— reintenta y publica.
func TestObservadorReintentaTrasFalloDeRed(t *testing.T) {
	dir := repoEnVivo(t)
	f, err := nuevoFotografo(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	bare := filepath.Join(t.TempDir(), "central.git") // todavía NO inicializado → push falla
	runGit(t, dir, "remote", "add", "central", bare)
	tr, _ := nuevoTransporte(f.git, "central", refTest)
	o := nuevoObservador(f, tr, time.Second)

	pub, err := o.tick(context.Background())
	if err == nil {
		t.Error("el tick con el remoto caído debía devolver error")
	}
	if pub {
		t.Error("no se pudo publicar: pub debía ser false")
	}
	if o.ultimoTree != "" {
		t.Error("ultimoTree no debía avanzar tras un push fallido (si no, no reintenta)")
	}

	runGit(t, "", "init", "--bare", "-q", bare) // el remoto ahora existe
	pub, err = o.tick(context.Background())
	if err != nil {
		t.Fatalf("el reintento debía publicar: %v", err)
	}
	if !pub {
		t.Error("el reintento debía publicar la foto")
	}
	if o.ultimoTree == "" {
		t.Error("ultimoTree debía avanzar tras el push exitoso")
	}
	if shaEnBare(t, bare, refTest) == "" {
		t.Error("la ref remota quedó vacía tras el reintento")
	}
}

// TestObservadorCorrerLimpioBajoRace corre el bucle de verdad (con -race) hasta que el ctx muere,
// mientras el working tree cambia: no debe haber data race ni panic, y debe publicar algo.
func TestObservadorCorrerLimpioBajoRace(t *testing.T) {
	o, _, dir, bare := obsConRemoto(t, 2*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); o.correr(ctx) }()

	for i := 0; i < 20; i++ {
		_ = os.WriteFile(filepath.Join(dir, "cambia.txt"), []byte(time.Duration(i).String()), 0o644)
		time.Sleep(3 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("correr() no terminó tras cancelar el ctx")
	}
	if shaEnBare(t, bare, refTest) == "" {
		t.Error("el bucle no publicó ninguna foto en toda la corrida")
	}
}

// TestTransportePublicarConcurrente ejerce el candado: dos publicaciones simultáneas por el mismo
// transporte no se corrompen (bajo -race) y la ref queda en una de las dos fotos, no a medias.
func TestTransportePublicarConcurrente(t *testing.T) {
	f, dir, remoto, bare := repoConRemoto(t)
	tr, _ := nuevoTransporte(f.git, remoto, refTest)
	sha1 := fotoTras(t, f, dir, "a.txt", "uno\n")
	sha2 := fotoTras(t, f, dir, "a.txt", "dos\n")
	if sha1 == sha2 {
		t.Fatal("el test necesita dos fotos distintas")
	}

	var wg sync.WaitGroup
	for _, sha := range []string{sha1, sha2} {
		wg.Add(1)
		go func(s string) { defer wg.Done(); _ = tr.publicar(context.Background(), s) }(sha)
	}
	wg.Wait()

	got := shaEnBare(t, bare, refTest)
	if got != sha1 && got != sha2 {
		t.Errorf("la ref quedó en %q, que no es ninguna de las dos fotos (%q, %q)", got, sha1, sha2)
	}
}

// TestBackoffRed fija el freno exponencial de B4c de forma determinista (reloj inyectado, sin dormir
// de verdad): sin fallos nunca frena; el 1er fallo frena `base`; cada fallo siguiente DUPLICA la
// espera hasta un TOPE; y un éxito la resetea. Sin estas propiedades, un remoto caído haría que el
// observador reintentara el push tick a tick (martilleo de red + un commit huérfano por tick).
func TestBackoffRed(t *testing.T) {
	t0 := time.Unix(1_000_000, 0)
	b := nuevoBackoffRed(5*time.Second, 1*time.Minute)

	if !b.puedeIntentar(t0) {
		t.Fatal("sin fallos debería poder intentar siempre")
	}

	b.fallo(t0) // 1er fallo: frena `base` (5s)
	if b.puedeIntentar(t0.Add(4 * time.Second)) {
		t.Error("a los 4s (< 5s de base) debería seguir frenado")
	}
	if !b.puedeIntentar(t0.Add(5 * time.Second)) {
		t.Error("a los 5s debería liberar")
	}

	t1 := t0.Add(5 * time.Second)
	b.fallo(t1) // 2º fallo: duplica a 10s
	if b.puedeIntentar(t1.Add(9 * time.Second)) {
		t.Error("a los 9s (< 10s, ya duplicado) debería seguir frenado")
	}
	if !b.puedeIntentar(t1.Add(10 * time.Second)) {
		t.Error("a los 10s debería liberar")
	}

	for i := 0; i < 20; i++ { // muchos fallos: debe clavarse en el tope, no crecer sin límite
		b.fallo(t1)
	}
	if b.espera != time.Minute {
		t.Errorf("la espera no respetó el tope de 1min: %v", b.espera)
	}

	b.exito() // éxito: resetea
	if b.espera != 0 {
		t.Errorf("tras éxito la espera debía volver a 0, quedó %v", b.espera)
	}
	if !b.puedeIntentar(t1) {
		t.Error("tras un éxito no debería quedar freno activo")
	}
}
