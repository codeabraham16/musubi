package main

// La rotación del token cuando el agente corre como root (decisión del dueño, 2026-09-24).

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// UN AGENTE ROOT QUE ROTA NO LE ROBA EL TOKEN AL DUEÑO.
//
// El colapso del llavero reescribe el archivo con temporal + rename, y el temporal lo crea el
// proceso: como root nace root:root. En /etc/musubi-agente eso es lo correcto; en la ruta de
// antes, bajo el home de musubi, dejaba un token que la vuelta atrás —el agente otra vez como
// musubi— ya no puede leer. Se le devuelve el dueño AL TEMPORAL, antes del rename: si falla, el
// archivo queda como estaba, con los dos tokens, que es el desenlace ya declarado no fatal.
//
// LOS DOS SABOTAJES MIDEN COSAS DISTINTAS: el primero deja el token de root (no se pide nada) y el
// segundo lo pide tarde, con el temporal ya cerrado y el destino ya reemplazado. Corridos los dos.
// Desde que el dueño se devuelve sobre el descriptor (2026-09-25) ya no pisan la misma línea.
//
// Sabotaje que la hace fallar: no devolver nunca el dueño.
// arnes: archivo="cmd/musubi/agent_token.go"
// arnes: de="ok && esRootElAgente()"
// arnes: a="ok && false"
// Sabotaje que la hace fallar: devolverlo DESPUÉS del rename (diferido al final, con el temporal ya
// cerrado y el destino ya reemplazado).
// arnes: archivo="cmd/musubi/agent_token.go"
// arnes: de="\t\tif err := cambiarDueno(tmp, uid, gid); err != nil {"
// arnes: a="\t\tdefer cambiarDueno(tmp, uid, gid)\n\t\tif err := error(nil); err != nil {"
func TestRotarComoRootConservaElDuenoDelToken(t *testing.T) {
	origRoot, origDueno, origCambiar := esRootElAgente, duenoDeArchivo, cambiarDueno
	t.Cleanup(func() { esRootElAgente, duenoDeArchivo, cambiarDueno = origRoot, origDueno, origCambiar })

	preparar := func(t *testing.T) (string, *credencial) {
		ruta := filepath.Join(t.TempDir(), "token")
		if err := os.WriteFile(ruta, []byte("viejo\nnuevo\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return ruta, &credencial{ruta: ruta, tokens: []string{"viejo", "nuevo"}, i: 1, probado: []bool{true, true}}
	}
	leer := func(t *testing.T, ruta string) string {
		b, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatal(err)
		}
		return strings.TrimSpace(string(b))
	}

	t.Run("root sobre un token de musubi: se le devuelve al temporal, antes del rename", func(t *testing.T) {
		ruta, c := preparar(t)
		esRootElAgente = func() bool { return true }
		duenoDeArchivo = func(string) (int, int, bool) { return 1000, 1000, true }
		type pedido struct {
			nombre       string
			uid, gid     int
			destinoAntes string
			contenido    string
			abierto      bool
		}
		var pedidos []pedido
		cambiarDueno = func(f *os.File, uid, gid int) error {
			tmp, _ := os.ReadFile(f.Name())
			_, errAbierto := f.Stat()
			pedidos = append(pedidos, pedido{f.Name(), uid, gid, leer(t, ruta), strings.TrimSpace(string(tmp)), errAbierto == nil})
			return nil
		}
		if err := c.Funciono(); err != nil {
			t.Fatalf("Funciono: %v", err)
		}
		if len(pedidos) != 1 {
			t.Fatalf("se pidió cambiar el dueño %d vez/veces; quería 1: el token nuevo nace root:root", len(pedidos))
		}
		p := pedidos[0]
		if p.uid != 1000 || p.gid != 1000 {
			t.Errorf("se le dio el token a %d:%d; su dueño era 1000:1000", p.uid, p.gid)
		}
		if p.nombre == ruta || filepath.Dir(p.nombre) != filepath.Dir(ruta) {
			t.Errorf("el dueño se cambió sobre %q: tiene que ser el TEMPORAL, en el mismo directorio", p.nombre)
		}
		if p.destinoAntes != "viejo\nnuevo" || p.contenido != "nuevo" {
			t.Errorf("al cambiar el dueño el destino tenía %q y el temporal %q: el cambio tiene que ir "+
				"ANTES del rename, sobre el temporal ya escrito", p.destinoAntes, p.contenido)
		}
		if !p.abierto {
			t.Errorf("el dueño se pidió sobre un descriptor cerrado: tiene que ir sobre el temporal ABIERTO, " +
				"que es el archivo que se escribió y no lo que haya en su ruta")
		}
		if got := leer(t, ruta); got != "nuevo" {
			t.Errorf("después de colapsar el archivo tiene %q", got)
		}
	})

	t.Run("si no se puede devolver, no se reemplaza", func(t *testing.T) {
		ruta, c := preparar(t)
		esRootElAgente = func() bool { return true }
		duenoDeArchivo = func(string) (int, int, bool) { return 1000, 1000, true }
		cambiarDueno = func(*os.File, int, int) error { return errors.New("operation not permitted") }
		err := c.Funciono()
		if err == nil || !strings.Contains(err.Error(), "dueño") {
			t.Fatalf("Funciono = %v; tenía que decir que no pudo devolverle el token a su dueño", err)
		}
		if got := leer(t, ruta); got != "viejo\nnuevo" {
			t.Errorf("el archivo quedó con %q: sin poder devolver el dueño NO se reemplaza", got)
		}
		restos, _ := filepath.Glob(filepath.Join(filepath.Dir(ruta), ".token-*"))
		if len(restos) != 0 {
			t.Errorf("quedaron temporales: %v", restos)
		}
	})

	t.Run("token de root o agente sin privilegios: no se toca el dueño", func(t *testing.T) {
		for _, c := range []struct {
			nombre  string
			root    bool
			uid     int
			conoces bool
		}{
			{"token de root en /etc", true, 0, true},
			{"agente como musubi", false, 1000, true},
			{"sistema que no informa dueño", true, -1, false},
		} {
			ruta, cred := preparar(t)
			esRootElAgente = func() bool { return c.root }
			duenoDeArchivo = func(string) (int, int, bool) { return c.uid, c.uid, c.conoces }
			llamado := false
			cambiarDueno = func(*os.File, int, int) error { llamado = true; return nil }
			if err := cred.Funciono(); err != nil {
				t.Fatalf("%s: %v", c.nombre, err)
			}
			if llamado {
				t.Errorf("%s: se cambió el dueño y no correspondía", c.nombre)
			}
			if got := leer(t, ruta); got != "nuevo" {
				t.Errorf("%s: el archivo quedó con %q", c.nombre, got)
			}
		}
	})
}
