package fleet

// sysctlparse.go decodifica las estructuras binarias que devuelve sysctl en macOS.
//
// Está SIN build tag a propósito, por la misma razón que cpudelta.go: es la parte difícil del
// colector de darwin —aritmética de punto fijo y enteros little-endian— y así se prueba desde
// cualquier plataforma. Lo que queda detrás del build tag es leer bytes y pasarlos, que es lo
// único que no se puede cubrir sin un Mac.
//
// POR QUÉ SE PARSEAN BYTES A MANO. `syscall.SysctlTimeval` y `SysctlRaw` viven en
// golang.org/x/sys, no en la stdlib. Meter x/sys para dos números sería la 7ª dependencia
// directa de un repo que tiene 6. `syscall.Sysctl` sí está en stdlib y devuelve el buffer crudo
// como string (sólo descarta UN NUL final), así que alcanza para llegar a los mismos bytes.

import "encoding/binary"

// parseBoottime decodifica `kern.boottime`, que es un `struct timeval` — segundos y microsegundos
// del instante del ARRANQUE (no los segundos corridos desde entonces).
//
// Sólo se leen los 8 primeros bytes (tv_sec, int64 little-endian en las dos arquitecturas de Mac
// que existen). Se admite un buffer más corto que la estructura completa: `syscall.Sysctl`
// descarta un NUL final, y el tv_usec de un arranque puede terminar en cero.
func parseBoottime(b []byte) (segundos int64, ok bool) {
	if len(b) < 8 {
		return 0, false
	}
	s := int64(binary.LittleEndian.Uint64(b[:8]))
	// Un boottime plausible: después de 2001 y no en el futuro lejano. Un cero, o un número
	// absurdo, significa que el buffer no era lo que esperábamos — mejor no reportar uptime que
	// reportar uno inventado.
	if s < 978307200 || s > 4102444800 {
		return 0, false
	}
	return s, true
}

// parseLoadavg decodifica `vm.loadavg`, que es un `struct loadavg`:
//
//	struct loadavg { fixpt_t ldavg[3]; long fscale; }
//
// `fixpt_t` es un uint32 en PUNTO FIJO: el load real es ldavg[i] / fscale. Leer los uint32 como
// si fueran el número final daría cargas de varios miles — es el error clásico con esta
// estructura, y por eso esta función existe y tiene prueba.
func parseLoadavg(b []byte) (l1, l5, l15 float64, ok bool) {
	// 3 × uint32 + un long. El long son 8 bytes en las dos arquitecturas de Mac, pero se admite
	// también un buffer de 16 (fscale de 4) por si alguna vez cambia el empaquetado.
	if len(b) < 16 {
		return 0, 0, 0, false
	}
	var escala uint64
	if len(b) >= 20 {
		escala = binary.LittleEndian.Uint64(b[12:20])
	} else {
		escala = uint64(binary.LittleEndian.Uint32(b[12:16]))
	}
	if escala == 0 {
		return 0, 0, 0, false // dividir por la escala es todo el punto; sin ella no hay número
	}
	conv := func(off int) float64 {
		return float64(binary.LittleEndian.Uint32(b[off:off+4])) / float64(escala)
	}
	l1, l5, l15 = conv(0), conv(4), conv(8)
	// Una carga negativa es imposible y una de 10.000 significa que el buffer no era loadavg.
	if l1 < 0 || l5 < 0 || l15 < 0 || l1 > 10000 || l5 > 10000 || l15 > 10000 {
		return 0, 0, 0, false
	}
	return l1, l5, l15, true
}
