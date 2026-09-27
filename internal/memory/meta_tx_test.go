package memory

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"musubi/internal/config"
)

// sondaDelCandado abre la base con busy_timeout 0: su BEGIN IMMEDIATE no espera a nadie, así que
// dice en el acto si otro tiene el candado de escritura.
func sondaDelCandado(t *testing.T, dir string) func() bool {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.Join(dir, config.DirName, config.DBFile)+"?_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatalf("abrir la sonda: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return func() bool {
		t.Helper()
		ctx := context.Background()
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatalf("conexión de la sonda: %v", err)
		}
		defer conn.Close()
		if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
			if !esBaseBloqueada(err) {
				t.Fatalf("la sonda falló por otra cosa que el candado: %v", err)
			}
			return true
		}
		if _, err := conn.ExecContext(ctx, `ROLLBACK`); err != nil {
			t.Fatalf("soltar la sonda: %v", err)
		}
		return false
	}
}

// TestMetaEnTransaccionSostieneElCandado: mientras corre fn, nadie más puede escribir —el candado
// se toma antes de la primera lectura—, fn lee lo que ella misma escribió, y al volver queda escrito
// y el candado libre.
//
// Sabotaje: fn corre sin transacción.
// arnes: archivo="internal/memory/meta_tx.go"
// arnes: de="\ttx, err := e.db.Begin()\n"
// arnes: a="\tif true {\n\t\treturn fn(e)\n\t}\n\ttx, err := e.db.Begin()\n"
func TestMetaEnTransaccionSostieneElCandado(t *testing.T) {
	dir := t.TempDir()
	eng, err := NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer eng.Close()
	candadoTomado := sondaDelCandado(t, dir)
	if candadoTomado() {
		t.Fatal("CONTROL: con nadie escribiendo, la sonda tenía que poder tomar el candado")
	}

	var tomadoAntes, tomadoDespues bool
	var leido string
	err = eng.MetaEnTransaccion(func(tx MetaTx) error {
		tomadoAntes = candadoTomado()
		if err := tx.SetMeta("prueba_tx", "adentro"); err != nil {
			return err
		}
		tomadoDespues = candadoTomado()
		leido, _, _ = tx.GetMeta("prueba_tx")
		return nil
	})
	if err != nil {
		t.Fatalf("MetaEnTransaccion: %v", err)
	}
	if !tomadoAntes || !tomadoDespues {
		t.Errorf("otro podía escribir mientras corría fn (antes de su primera escritura: %v, después: %v)", !tomadoAntes, !tomadoDespues)
	}
	if leido != "adentro" {
		t.Errorf("fn no ve lo que ella misma escribió: leyó %q", leido)
	}
	if v, ok, _ := eng.GetMeta("prueba_tx"); !ok || v != "adentro" {
		t.Errorf("lo que escribió fn no quedó: %q, %v", v, ok)
	}
	if candadoTomado() {
		t.Error("al volver, el candado sigue tomado")
	}
}

// TestMetaEnTransaccionDeshaceSiFnFalla: si fn devuelve un error, MetaEnTransaccion lo devuelve y
// no queda nada de lo que fn había escrito.
//
// Sabotaje: confirmar aunque fn falle.
// arnes: archivo="internal/memory/meta_tx.go"
// arnes: de="\tif err := fn(metaEnTx{tx}); err != nil {\n\t\treturn err\n\t}\n"
// arnes: a="\tif err := fn(metaEnTx{tx}); err != nil {\n\t\t_ = tx.Commit()\n\t\treturn err\n\t}\n"
func TestMetaEnTransaccionDeshaceSiFnFalla(t *testing.T) {
	eng, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer eng.Close()

	falla := errors.New("fn falló")
	err = eng.MetaEnTransaccion(func(tx MetaTx) error {
		if err := tx.SetMeta("prueba_tx", "a medias"); err != nil {
			return err
		}
		return falla
	})
	if !errors.Is(err, falla) {
		t.Errorf("MetaEnTransaccion devolvió %v; tenía que devolver el error de fn", err)
	}
	if v, ok, _ := eng.GetMeta("prueba_tx"); ok {
		t.Errorf("fn falló y lo que escribió quedó: %q", v)
	}
}

// TestMetaEnTransaccionEnSoloLectura: un engine en sólo lectura no corre fn: devuelve
// ErrMetaSoloLectura y no escribe nada.
//
// Sabotaje: el engine en sólo lectura corre fn igual.
// arnes: archivo="internal/memory/meta_tx.go"
// arnes: de="\tif e.soloLectura {\n\t\treturn ErrMetaSoloLectura\n"
// arnes: a="\tif e.soloLectura && false {\n\t\treturn ErrMetaSoloLectura\n"
func TestMetaEnTransaccionEnSoloLectura(t *testing.T) {
	eng, err := NewDbEngineSoloLectura(baseMasNueva(t))
	if err != nil {
		t.Fatalf("NewDbEngineSoloLectura: %v", err)
	}
	defer eng.Close()

	corrio := false
	err = eng.MetaEnTransaccion(func(tx MetaTx) error {
		corrio = true
		return nil
	})
	if !errors.Is(err, ErrMetaSoloLectura) {
		t.Errorf("MetaEnTransaccion en sólo lectura devolvió %v; tenía que devolver ErrMetaSoloLectura", err)
	}
	if corrio {
		t.Error("un engine en sólo lectura corrió fn: fn existe para escribir")
	}
}
