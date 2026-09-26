package fleet

import (
	"sync"
	"unicode"
)

// blancosDeTrimSpace son TODOS los blancos que strings.TrimSpace recorta, cada uno como string: las
// runas para las que unicode.IsSpace dice que sí, recorridas enteras (de 0 a unicode.MaxRune).
//
// SE DERIVAN Y NO SE LISTAN, y la lista escrita a mano ya costó una ronda. cabezasDe y el `adelante`
// de formasDeLaCabeza llevaban cinco (espacio, tab, CR, LF y el espacio duro) y su doc prometía «los
// de strings.TrimSpace»: son 25. La segunda revisión de T7 lo midió con una segunda normalización en
// ArgvDeBitacora —`strings.Trim(…, " \t\r\n ")` sobre la primera parte no vacía—: el paquete
// entero quedaba en verde, y una sonda mostró la contraseña en claro con U+000B, U+000C, U+0085,
// U+1680, U+2000–U+200A, U+2028, U+2029, U+202F, U+205F y U+3000 adelante, formas que el agente
// despacha como pantalla porque LimpiarArgv sí las recorta.
//
// Es un HECHO DEL MUNDO —la propiedad White_Space de Unicode, tal como la implementa unicode.IsSpace,
// que es lo que llama TrimSpace—, así que el piso que lo acompaña (25, en
// TestLaCabezaDelArgvSeLeeComoLaDespachaElAgente) se clava y no se deriva de acá.
var blancosDeTrimSpace = sync.OnceValue(func() []string {
	var out []string
	for r := rune(0); r <= unicode.MaxRune; r++ {
		if unicode.IsSpace(r) {
			out = append(out, string(r))
		}
	}
	return out
})
