package main

// Lo de NTFS que las pruebas de `musubi sesiones` necesitan HACER para medirse: crear una junction y
// dejar un archivo abierto como lo dejaría otro proceso. El comando no lo llama.
//
// VIVE FUERA DE LOS `_test.go` A PROPÓSITO. Toda prueba de este repo tiene que compilar en linux
// (internal/mcp/pruebas_que_no_corren_test.go: una prueba que no compila en CI «contesta ok igual»),
// así que lo que va detrás del tag es sólo QUÉ SE EJECUTA, como en internal/fleet/tempwindows.go.
// Las pruebas viven en sesionesntfs_test.go y se saltean, diciéndolo, fuera de Windows. Y no se
// lanza `cmd /c mklink /J`: ninguna prueba lanza un proceso por fuera de internal/guiones, y `cmd`
// es una shell.

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"syscall"
	"unicode/utf16"
)

// crearJunction crea `enlace` como junction a `destino`, en el orden de os.Symlink. Es lo que hace
// `mklink /J`: una carpeta vacía con un punto de reparación MOUNT_POINT que apunta a
// `\??\<destino absoluto>`. A diferencia de un enlace simbólico, no pide privilegios.
func crearJunction(destino, enlace string) error {
	const (
		tagJunction          = 0xA0000003 // IO_REPARSE_TAG_MOUNT_POINT
		fsctlSetReparsePoint = 0x000900A4 // FSCTL_SET_REPARSE_POINT
	)
	abs, err := filepath.Abs(destino)
	if err != nil {
		return err
	}
	if err := os.Mkdir(enlace, 0o755); err != nil {
		return err
	}
	sustituto := utf16.Encode([]rune(`\??\` + abs))
	impreso := utf16.Encode([]rune(abs))
	nombres := append(append(append(sustituto, 0), impreso...), 0)
	var buf bytes.Buffer
	for _, v := range []any{
		uint32(tagJunction),
		uint16(8 + 2*len(nombres)),   // ReparseDataLength: los cuatro campos que siguen y los nombres
		uint16(0),                    // Reserved
		uint16(0),                    // SubstituteNameOffset
		uint16(2 * len(sustituto)),   // SubstituteNameLength, sin el NUL
		uint16(2*len(sustituto) + 2), // PrintNameOffset, después del NUL
		uint16(2 * len(impreso)),     // PrintNameLength
		nombres,
	} {
		if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
			return err
		}
	}
	p, err := syscall.UTF16PtrFromString(enlace)
	if err != nil {
		return err
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_EXISTING,
		syscall.FILE_FLAG_OPEN_REPARSE_POINT|syscall.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer syscall.CloseHandle(h)
	datos := buf.Bytes()
	var n uint32
	return syscall.DeviceIoControl(h, fsctlSetReparsePoint, &datos[0], uint32(len(datos)), nil, 0, &n, nil)
}

// abrirComoOtroProceso deja `ruta` abierta hasta que se llame a la función que devuelve. dejaBorrar
// dice si comparte el borrado (FILE_SHARE_DELETE), que es como abre libuv —o sea, Claude Code—: con
// él, Windows deja borrar el archivo mientras está abierto; sin él, no.
func abrirComoOtroProceso(ruta string, dejaBorrar bool) (func(), error) {
	p, err := syscall.UTF16PtrFromString(ruta)
	if err != nil {
		return nil, err
	}
	share := uint32(syscall.FILE_SHARE_READ | syscall.FILE_SHARE_WRITE)
	if dejaBorrar {
		share |= syscall.FILE_SHARE_DELETE
	}
	h, err := syscall.CreateFile(p, syscall.GENERIC_READ, share, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, err
	}
	return func() { _ = syscall.CloseHandle(h) }, nil
}
