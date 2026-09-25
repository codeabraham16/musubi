//go:build unix

package main

// La mitad de identidad_servicios.go que necesita el uid de verdad. Va aparte, como procvivo_*,
// porque syscall.Credential y syscall.Stat_t no existen en Windows: en blindaje.go o en agent.go
// —que no llevan build tag— el paquete dejaría de compilar en las otras dos plataformas de la CI.

import (
	"os"
	"syscall"
)

// atributosPara baja el hijo al uid, gid y grupos del dueño. nil cuando no hay que bajar: el hijo
// hereda al agente, que es lo que pasaba siempre.
//
// LOS GRUPOS VAN EXPLÍCITOS: sin ellos Go no llama a setgroups y el hijo conservaría los
// suplementarios de ROOT con el uid del dueño, una mezcla que no es de nadie.
func atributosPara(id identidadDeServicios) *syscall.SysProcAttr {
	if !id.Bajar {
		return nil
	}
	return &syscall.SysProcAttr{Credential: &syscall.Credential{
		Uid:    id.Uid,
		Gid:    id.Gid,
		Groups: id.Grupos,
	}}
}

// esDeUid dice si dir existe, es un directorio y su dueño es uid. Es lo que separa «el
// /run/user/<uid> de este usuario» de un directorio cualquiera con ese nombre.
func esDeUid(dir string, uid uint32) bool {
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	return ok && st.Uid == uid
}

// duenoRealDeArchivo devuelve el uid y el gid del archivo. ok=false si no existe o si el sistema
// no lo informa.
func duenoRealDeArchivo(ruta string) (uid, gid int, ok bool) {
	fi, err := os.Stat(ruta)
	if err != nil {
		return -1, -1, false
	}
	st, es := fi.Sys().(*syscall.Stat_t)
	if !es {
		return -1, -1, false
	}
	return int(st.Uid), int(st.Gid), true
}
