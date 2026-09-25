//go:build unix

package main

// Las dos piezas que tocan el uid de verdad. Corren sólo en Unix: syscall.Credential y
// syscall.Stat_t no existen en Windows.

import (
	"os"
	"path/filepath"
	"testing"
)

// EL HIJO BAJA AL UID, AL GID Y A LOS GRUPOS DEL DUEÑO, Y SIN BAJAR NO SE TOCA NADA.
//
// Sabotaje que la hace fallar: no bajar nunca (el hijo corre como root y lee el store rootful).
// arnes: archivo="cmd/musubi/identidad_servicios_unix.go"
// arnes: de="\tif !id.Bajar {\n\t\treturn nil\n\t}"
// arnes: a="\tif true {\n\t\treturn nil\n\t}"
// Sabotaje que la hace fallar: bajar sin los grupos (el hijo conserva los suplementarios de root).
// arnes: archivo="cmd/musubi/identidad_servicios_unix.go"
// arnes: de="\t\tGroups: id.Grupos,\n"
// arnes: a="\t\tGroups: nil,\n"
func TestAtributosParaBajanAlUidDelDueno(t *testing.T) {
	id := identidadDeServicios{Nombre: "musubi", Uid: 1000, Gid: 1000, Grupos: []uint32{1000, 10}, Bajar: true}
	a := atributosPara(id)
	if a == nil || a.Credential == nil {
		t.Fatal("con Bajar no se pidió credencial: el hijo corre como root y podman lee el store rootful")
	}
	c := a.Credential
	if c.Uid != 1000 || c.Gid != 1000 {
		t.Errorf("credencial uid=%d gid=%d; quería 1000/1000", c.Uid, c.Gid)
	}
	if len(c.Groups) != 2 || c.Groups[0] != 1000 || c.Groups[1] != 10 {
		t.Errorf("grupos %v; sin ellos Go no llama a setgroups y el hijo conserva los de root", c.Groups)
	}
	if atributosPara(identidadDeServicios{Nombre: "root"}) != nil {
		t.Error("sin Bajar se pidieron atributos: el hijo tiene que heredar al agente tal cual")
	}
}

// esDeUid SEPARA EL RUNTIME DEL DUEÑO DE UN DIRECTORIO CUALQUIERA CON ESE NOMBRE.
//
// Sabotaje que la hace fallar: no mirar el dueño del directorio.
// arnes: archivo="cmd/musubi/identidad_servicios_unix.go"
// arnes: de="\treturn ok && st.Uid == uid\n"
// arnes: a="\treturn ok && st != nil\n"
func TestEsDeUidMiraElDuenoYQueSeaDirectorio(t *testing.T) {
	dir := t.TempDir()
	yo := uint32(os.Getuid())
	if !esDeUid(dir, yo) {
		t.Fatalf("%s es mío (uid %d) y esDeUid dijo que no", dir, yo)
	}
	if esDeUid(dir, yo+1) {
		t.Errorf("%s es de %d y esDeUid dijo que es de %d", dir, yo, yo+1)
	}
	archivo := filepath.Join(dir, "no-es-dir")
	if err := os.WriteFile(archivo, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if esDeUid(archivo, yo) {
		t.Error("un archivo pasó por runtime")
	}
	if esDeUid(filepath.Join(dir, "no-existe"), yo) {
		t.Error("un directorio inexistente pasó por runtime")
	}
	if uid, _, ok := duenoRealDeArchivo(archivo); !ok || uid != int(yo) {
		t.Errorf("duenoRealDeArchivo = %d, %v; quería %d", uid, ok, yo)
	}
}
