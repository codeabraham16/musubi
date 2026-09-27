package memory

import (
	"strings"
	"testing"
	"time"
)

// TestLaBajadaSumaElViajeYLaEdadEnUnaTransaccion: RegistrarBajada escribe las dos fuentes de la
// línea «bajada» de musubi_sync_status —el viaje en sync_viajes y la edad en MetaUltimaBajada—
// juntas o ninguna, y el viaje cae en el día de la meta y no en el de date('now'): así un tick que
// cruza la medianoche UTC no deja la meta en un día y el viaje en el otro.
//
// Sabotaje: commitear el viaje antes de escribir la meta, o sea dos transacciones.
// arnes: archivo="internal/memory/sync_viajes.go"
// arnes: de="if err := sumarViaje(tx, dia, ViajeBajada, v); err != nil {"
// arnes: a="if err := sumarViaje(tx, dia, ViajeBajada, v); err != nil || tx.Commit() != nil {"
//
// Sabotaje: que el viaje vaya al día de ahora, el de date('now'), y no al de la meta.
// arnes: archivo="internal/memory/sync_viajes.go"
// arnes: de="dia := time.Unix(u.Unix, 0).UTC().Format(time.DateOnly)"
// arnes: a="dia := time.Now().UTC().Format(time.DateOnly)"
//
// Sabotaje: que la meta no anote las páginas que ese día tenía sync_viajes.
// arnes: archivo="internal/memory/sync_viajes.go"
// arnes: de="Scan(&u.PaginasDelDia)"
// arnes: a="Scan(new(int64))"
func TestLaBajadaSumaElViajeYLaEdadEnUnaTransaccion(t *testing.T) {
	haceDosDias := time.Now().Add(-48 * time.Hour)
	u := UltimaBajada{Unix: haceDosDias.Unix(), Filas: 2, ProximaUnix: haceDosDias.Unix() + 30}
	diaDeLaMeta := haceDosDias.UTC().Format(time.DateOnly)

	t.Run("las dos, en el día de la meta", func(t *testing.T) {
		e := newTestEngine(t)
		if err := e.RegistrarBajada(Viaje{Filas: 2, Posts: 1, BytesCable: 900, BytesCrudos: 900}, u); err != nil {
			t.Fatal(err)
		}
		// La meta lleva las páginas que el día tenía DESPUÉS de sumar las de este tick.
		quiero := u
		quiero.PaginasDelDia = 1
		if v, ok, err := e.GetMeta(MetaUltimaBajada); err != nil || !ok || v != quiero.Valor() {
			t.Errorf("la meta quedó %q (ok=%v, err=%v); esperaba %q", v, ok, err, quiero.Valor())
		}
		var dia string
		var filas, posts int64
		if err := e.db.QueryRow(`SELECT dia, filas, posts FROM sync_viajes WHERE sentido = 'bajada'`).Scan(&dia, &filas, &posts); err != nil {
			t.Fatalf("el viaje de la bajada no quedó en sync_viajes: %v", err)
		}
		if dia != diaDeLaMeta || filas != 2 || posts != 1 {
			t.Errorf("el viaje quedó en %s con filas=%d posts=%d; esperaba el día de la meta (%s) con filas=2 posts=1", dia, filas, posts, diaDeLaMeta)
		}
		// Y un segundo tick del mismo día anota las de los dos: es lo que deja ver, ese mismo día,
		// páginas que sumó alguien que no anota.
		if err := e.RegistrarBajada(Viaje{Posts: 2, Vacias: 2}, u); err != nil {
			t.Fatal(err)
		}
		quiero.PaginasDelDia = 3
		if v, _, err := e.GetMeta(MetaUltimaBajada); err != nil || v != quiero.Valor() {
			t.Errorf("tras el segundo tick del día la meta quedó %q (err=%v); esperaba %q", v, err, quiero.Valor())
		}
	})

	t.Run("si la meta no entra, el viaje tampoco", func(t *testing.T) {
		e := newTestEngine(t)
		if _, err := e.db.Exec(`CREATE TRIGGER meta_rota BEFORE INSERT ON meta WHEN NEW.key = '` + MetaUltimaBajada + `'
			BEGIN SELECT RAISE(ABORT, 'meta rota a propósito'); END`); err != nil {
			t.Fatal(err)
		}
		if err := e.RegistrarBajada(Viaje{Filas: 2, Posts: 1}, u); err == nil {
			t.Fatal("con la meta rota, RegistrarBajada tenía que devolver el error")
		}
		var n int
		if err := e.db.QueryRow(`SELECT COUNT(*) FROM sync_viajes`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("la meta no entró y el viaje sí (%d fila/s en sync_viajes): las dos fuentes quedaron desparejas", n)
		}
	})
}

// TestLaUltimaBajadaToleraLosCamposDeMas: la meta crece sólo agregando campos al final —el ritmo de
// la bajada va a sumar los suyos—, así que un binario lee los que conoce e ignora los que vengan
// después. Si un campo de más la volviera ilegible, la línea de un binario sobre una base que
// comparte con otro más nuevo diría «ilegible» en cada tick del nuevo.
//
// Sabotaje: exigir exactamente los campos que este binario conoce.
// arnes: archivo="internal/memory/bajada_lease.go"
// arnes: de="if len(partes) < len(n) {"
// arnes: a="if len(partes) != len(n) {"
func TestLaUltimaBajadaToleraLosCamposDeMas(t *testing.T) {
	u := UltimaBajada{Unix: 1758900000, Filas: 2, ProximaUnix: 1758900030, ConFilas: 1, PaginasDelDia: 40}
	if got, err := LeerUltimaBajada(u.Valor()); err != nil || got != u {
		t.Fatalf("la vuelta no cierra: %q se leyó %+v (err=%v); esperaba %+v", u.Valor(), got, err, u)
	}
	for _, deMas := range []string{"|7", "|7|algo que este binario todavía no conoce"} {
		if got, err := LeerUltimaBajada(u.Valor() + deMas); err != nil || got != u {
			t.Errorf("la meta %q tenía que leerse %+v ignorando lo de más; salió %+v (err=%v)", u.Valor()+deMas, u, got, err)
		}
	}
	// Uno de MENOS sí es ilegible: falta algo que este binario necesita para no mentir.
	corta := u.Valor()[:strings.LastIndex(u.Valor(), "|")]
	if got, err := LeerUltimaBajada(corta); err == nil {
		t.Errorf("la meta %q tiene un campo de menos y se leyó %+v", corta, got)
	}
}
