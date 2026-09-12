package embedding

import (
	"testing"

	"golang.org/x/text/unicode/norm"
)

// TestLaNormalizacionUnicodeEstaFijada CLAVA el comportamiento de la normalizacion Unicode del que
// dependen los DOS tokenizers, para que un cambio en golang.org/x/text se vea como un test ROJO en
// vez de como vectores distintos que nadie nota.
//
// EL AGUJERO QUE VIENE A TAPAR, Y ES DE LOS SILENCIOSOS. El `model_id` que se estampa en cada
// embedding se deriva del CONTENIDO de model.safetensors + tokenizer.json (ver checksumDeCRC): NO
// lleva nada del codigo del tokenizer. Y el tokenizer normaliza con golang.org/x/text — NFD para
// el strip-accents del WordPiece (static.go, wpStripAccents) y NFC antes del charsmap del Unigram
// (spm.go, precompiledStep.apply).
//
// O sea que si una version nueva de x/text cambiara sus tablas de normalizacion:
//
//	texto -> otros tokens -> OTRO VECTOR ... con el MISMO model_id
//
// La regla de homogeneidad (operations.go) filtra por model_id, asi que seguiria comparando por
// coseno los vectores viejos contra los nuevos como si fueran de la misma procedencia. Es
// EXACTAMENTE la corrupcion silenciosa de ranking que N1 vino a cerrar, entrando por una puerta que
// N1 no mira: N1 vigila que cambie la TABLA, no que cambie el CODIGO que la consulta.
//
// POR QUE NO ALCANZABA CON LO QUE YA HABIA. `TestUnigramRealBitExact` valida bit-exacto contra el
// tokenizer real, y es la guarda fuerte — pero SE SALTEA SIN `MUSUBI_SPM_TESTDATA`, o sea que en CI
// no corre (el asset son 18 MB y no se commitea). De las que si corren, `TestUnigram` no tiene NI UN
// caso con acentos, y `TestStaticWordPiece` tiene uno solo ("Cafe" con tilde). Medido: subir x/text
// de 0.41.0 a 0.42.0 NO cambia nada — la huella de tokenizar 32 frases dificiles con la tabla real
// da identica en las dos. El problema no es ese bump: es que el proximo no tendria quien lo vea.
//
// LOS VALORES ESPERADOS SON HECHOS DEL MUNDO, NO DERIVADOS NUESTROS, y por eso van clavados a mano:
// que U+00FF descomponga en "y" + dieresis combinante lo dice Unicode, no este repo. Derivarlos
// llamando a la misma funcion que se quiere custodiar dejaria la guarda midiendose a si misma.
// Fueron MEDIDOS contra x/text v0.41.0, no escritos de memoria.
//
// UN CASO ME AGARRO A MI MIENTRAS ESCRIBIA ESTA GUARDA, y por eso se queda. El hangul
// precompuesto U+D55C U+AE00 pasa por wpStripAccents y sale DESCOMPUESTO en jamo (U+1112 U+1161
// U+11AB...), porque los jamo no son marcas Mn y el filtro no los toca. Las dos formas SE VEN
// EXACTAMENTE IGUAL en pantalla, asi que copie el valor equivocado de mi propia sonda y el test
// nacio rojo. Una diferencia de normalizacion puede ser invisible al ojo y total para el
// tokenizador: eso es precisamente lo que esta guarda existe para ver.
//
// Un caso tiene historia: `\u00ff` -> "y" es el byte 0xFF que una auditoria de agosto dejo escrito
// como NO-hallazgo tras medirlo ("es la regla funcionando, no una perdida"). Aca deja de ser una
// nota y pasa a ser un caso que se corre.
func TestLaNormalizacionUnicodeEstaFijada(t *testing.T) {
	casos := []struct {
		nombre, entrada, strip, nfc, nfd string
	}{
		{"acento latino simple", "Caf\u00e9", "Cafe", "Caf\u00e9", "Cafe\u0301"},
		{"voseo rioplatense", "guard\u00e1", "guarda", "guard\u00e1", "guarda\u0301"},
		{"enie y tilde juntas", "\u00f1and\u00fa", "nandu", "\u00f1and\u00fa", "n\u0303andu\u0301"},
		{"ya descompuesto en la entrada", "e\u0301", "e", "\u00e9", "e\u0301"},
		{"anillo combinante", "A\u030a", "A", "\u00c5", "A\u030a"},
		{"I con punto (turco)", "\u0130stanbul", "Istanbul", "\u0130stanbul", "I\u0307stanbul"},
		{"el byte 0xFF de la traduccion", "\u00ff", "y", "\u00ff", "y\u0308"},
		{"s larga con dos diacriticos", "\u1e9b\u0323", "\u017f", "\u1e9b\u0323", "\u017f\u0323\u0307"},
		{"virama devanagari", "\u0915\u094d\u0937\u093f", "\u0915\u0937\u093f", "\u0915\u094d\u0937\u093f", "\u0915\u094d\u0937\u093f"},
		{"hangul: el strip devuelve JAMO y se ve IGUAL", "\ud55c\uae00", "\u1112\u1161\u11ab\u1100\u1173\u11af", "\ud55c\uae00", "\u1112\u1161\u11ab\u1100\u1173\u11af"},
		{"ligadura fi (NFC no la parte)", "\ufb01", "\ufb01", "\ufb01", "\ufb01"},
		{"digrafo croata en titulo", "\u01c5", "\u01c5", "\u01c5", "\u01c5"},
		{"numero romano", "\u2167", "\u2167", "\u2167", "\u2167"},
		{"ancho completo", "\uff21\uff22\uff23", "\uff21\uff22\uff23", "\uff21\uff22\uff23", "\uff21\uff22\uff23"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := wpStripAccents(c.entrada); got != c.strip {
				t.Errorf("wpStripAccents(%q) = %q, esperaba %q\n"+
					"  Si esto cambio por subir golang.org/x/text: los vectores nuevos difieren de los "+
					"guardados y comparten model_id, asi que la busqueda los compara igual. Hay que "+
					"re-embeber, o el ranking se degrada sin un solo error.", c.entrada, got, c.strip)
			}
			if got := norm.NFC.String(c.entrada); got != c.nfc {
				t.Errorf("norm.NFC(%q) = %q, esperaba %q — lo usa el charsmap del Unigram (spm.go)", c.entrada, got, c.nfc)
			}
			if got := norm.NFD.String(c.entrada); got != c.nfd {
				t.Errorf("norm.NFD(%q) = %q, esperaba %q — lo usa el strip-accents del WordPiece (static.go)", c.entrada, got, c.nfd)
			}
		})
	}
}
