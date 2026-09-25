//go:build unix

package main

// La rotación del token como root, del lado que sólo existe en Unix: los symlinks y el dueño de un
// archivo. En Windows ni os.Chown ni los bits de modo significan lo mismo.

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// EL MODO DEL TOKEN SE FIJA SOBRE EL TEMPORAL QUE SE ESCRIBIÓ, NO SOBRE LO QUE HAYA EN SU RUTA.
//
// Con el agente root y el token bajo el home de otro usuario —un drop-in mal ordenado, una vuelta
// atrás a medias—, ese usuario es dueño del directorio y puede cambiar el temporal por un symlink
// entre CreateTemp y el Chmod. os.Chmod sigue el symlink: root le aplicaba el modo del token a un
// archivo cualquiera del sistema. El hallazgo del revisor es el del Chown (la prueba de abajo); acá
// el cambio lo hace el doble de cambiarDueno, que corre justo antes del Chmod: el mismo hueco, sin root.
//
// Sabotaje que la hace fallar: fijar el modo por ruta.
// arnes: archivo="cmd/musubi/agent_token.go"
// arnes: de="tmp.Chmod(modo)"
// arnes: a="os.Chmod(nombre, modo)"
// arnes: colision_ok="TestColapsarElLlaveroNoAflojaElModoDelArchivo"
func TestElModoDelTokenNoSigueUnSymlinkPuestoEnLaRuta(t *testing.T) {
	origRoot, origDueno, origCambiar := esRootElAgente, duenoDeArchivo, cambiarDueno
	t.Cleanup(func() { esRootElAgente, duenoDeArchivo, cambiarDueno = origRoot, origDueno, origCambiar })

	dir := t.TempDir()
	ruta := filepath.Join(dir, "token")
	if err := os.WriteFile(ruta, []byte("viejo\nnuevo\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	victima := filepath.Join(dir, "victima")
	if err := os.WriteFile(victima, []byte("no es un token\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	esRootElAgente = func() bool { return true }
	duenoDeArchivo = func(string) (int, int, bool) { return 1000, 1000, true }
	cambiado := false
	cambiarDueno = func(f *os.File, _, _ int) error {
		// El dueño del directorio gana la carrera: el temporal pasa a ser un symlink a la víctima.
		if err := os.Remove(f.Name()); err != nil {
			return err
		}
		if err := os.Symlink(victima, f.Name()); err != nil {
			return err
		}
		cambiado = true
		return nil
	}

	if err := escribirTokens(ruta, []string{"nuevo"}); err != nil {
		t.Fatalf("escribirTokens: %v", err)
	}
	if !cambiado {
		t.Fatal("el doble de cambiarDueno no corrió: la prueba no puso el symlink y no midió nada")
	}
	fi, err := os.Lstat(victima)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o644 {
		t.Errorf("la víctima quedó %v (era 0644): el modo del token se aplicó por RUTA y siguió el symlink", got)
	}
}

// EL DUEÑO SE DEVUELVE SOBRE EL DESCRIPTOR: el cambiarDueno de producción no sigue un symlink.
//
// Como root se mide de verdad: la víctima es de root y el pedido es para 1000. Sin root —la CI— la
// víctima es un directorio del sistema que no es de quien corre, y un chown por ruta sobre ella da
// EPERM; sobre el descriptor, el temporal es propio y el chown a uno mismo pasa.
//
// Sabotaje que la hace fallar: devolver el dueño por ruta.
// arnes: archivo="cmd/musubi/agent_token.go"
// arnes: de="return f.Chown(uid, gid)"
// arnes: a="return os.Chown(f.Name(), uid, gid)"
func TestElDuenoDelTokenNoSigueUnSymlinkPuestoEnLaRuta(t *testing.T) {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, ".token-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })

	yo, grupo := os.Getuid(), os.Getgid()
	uid, gid := yo, grupo
	var victima string
	if yo == 0 {
		victima = filepath.Join(dir, "victima")
		if err := os.WriteFile(victima, nil, 0o644); err != nil {
			t.Fatal(err)
		}
		uid, gid = 1000, 1000
	} else {
		for _, d := range []string{"/", "/etc", "/usr", "/bin"} {
			if u, _, ok := duenoRealDeArchivo(d); ok && u != yo {
				victima = d
				break
			}
		}
		if victima == "" {
			t.Skip("no hay en esta máquina un directorio del sistema que no sea de quien corre")
		}
	}
	if err := os.Remove(f.Name()); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victima, f.Name()); err != nil {
		t.Fatal(err)
	}

	if err := cambiarDueno(f, uid, gid); err != nil {
		t.Fatalf("cambiarDueno falló (%v): siguió el symlink de la ruta hasta %s en vez de cambiar el archivo abierto", err, victima)
	}
	if u, _, ok := duenoRealDeArchivo(victima); !ok || (yo == 0 && u != 0) {
		t.Errorf("%s quedó de uid %d: el dueño del token se entregó por RUTA y siguió el symlink", victima, u)
	}
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if st, ok := fi.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != uid {
		t.Errorf("el temporal abierto no quedó de uid %d (%+v): el chown no llegó al archivo que se escribió", uid, fi.Sys())
	}
}
