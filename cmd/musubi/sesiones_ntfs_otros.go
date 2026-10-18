//go:build !windows

package main

import "errors"

// Fuera de Windows no hay NTFS que medir: las pruebas que usan esto se saltean diciéndolo. Ver
// sesiones_ntfs_windows.go.

func crearJunction(destino, enlace string) error { return errors.ErrUnsupported }

func abrirComoOtroProceso(ruta string, dejaBorrar bool) (func(), error) {
	return nil, errors.ErrUnsupported
}
