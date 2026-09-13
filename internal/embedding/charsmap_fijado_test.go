package embedding

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestLaTokenizacionDelCharsmapEstaFijada — el agujero que dejaba abierto `TestUnigramRealBitExact`.
//
// LO QUE PASÓ, Y ES EL MOTIVO DE ESTE ARCHIVO. Una auditoría adversaria del 2026-09-12 rompió el
// trie darts del charsmap precompilado con un typo de UN BIT —`dartsHasLeaf` de `(u>>8)&1` a
// `(u>>9)&1`, que es exactamente el error que se comete decodificando darts-clone— y TODO quedó en
// verde: `TestUnigramRealBitExact`, `TestLaNormalizacionUnicodeEstaFijada`, el paquete
// `internal/embedding` ENTERO con la tabla real (21 s), y hasta el gate de recall R@10 con POTION.
//
// Y NO ERA UN FALSO POSITIVO: medido, la tokenización CAMBIA. Huella de 14 frases difíciles,
// c7bf6e144231ada3 con el bit roto contra 7126b392b5d20d14 con el código sano.
//
// LA CAUSA es el corpus, no el mecanismo: las 12 entradas de `testdata/spm_potion_ids.json` son
// todas ASCII/Latin-1 YA NORMALIZADAS, así que ninguna atraviesa el charsmap precompilado
// (`precompiledStep.apply` → NFC → el trie). Una guarda bit-exacta sobre un corpus que no toca el
// camino no custodia ese camino, por más real que sea el asset.
//
// QUÉ CLASE DE GUARDA ES ÉSTA, Y NO ES LA MISMA QUE SU HERMANA. `TestUnigramRealBitExact` compara
// contra IDs de REFERENCIA: afirma que tokenizamos IGUAL QUE el tokenizer de verdad. Ésta NO puede
// afirmar eso —no hay referencia externa para estas frases— y no lo pretende: la huella se midió
// con NUESTRA implementación y lo que custodia es que NO CAMBIE. Es un detector de cambio, no de
// corrección: si hoy estuviéramos tokenizando mal algo de esto, esta guarda clava el error.
// Se dice acá para que nadie la lea como lo que no es.
//
// Y ESO ALCANZA PARA LA AMENAZA QUE IMPORTA, que es la que N1 no cubre: el `model_id` se deriva de
// los BYTES del asset y no del CÓDIGO que los lee, así que un bump de `golang.org/x/text`, un
// cambio de tabla del toolchain o un refactor propio mueven los vectores SIN mover la procedencia,
// y los viejos se comparan por coseno contra los nuevos como si fueran del mismo modelo.
//
// MECANIZADA. El sabotaje es el typo de UN BIT que la parió. El arreglo es la MISMA línea
// reescrita de forma equivalente —extraer el bit 8 con máscara en vez de con corrimiento—: una
// huella que se pusiera roja ahí estaría fijando la FORMA del decodificador y no su salida, que
// es justo lo que esta guarda no debe hacer. Las dos direcciones, medidas el 2026-09-12:
// con (u>>9)&1 la huella pasa a c7bf6e144231ada3 y la prueba falla; con (u&0x100)>>8 sigue
// dando 7126b392b5d20d14 y pasa.
//
// OJO, Y ES CONDICIÓN PARA QUE ESTE VEREDICTO VALGA: sin MUSUBI_SPM_TESTDATA esta prueba hace
// t.Skip y el paquete contesta `ok` con exit 0. Un arnés que lea el rojo por «--- FAIL» cuenta
// eso como VERDE, o sea como guarda hueca, que es un hallazgo FALSO. Medido acá mismo.
// Sabotaje que la pone roja: dar vuelta un bit en `dartsHasLeaf`, de `(u>>8)&1` a `(u>>9)&1`.
// arnes: archivo="internal/embedding/spm.go"
// arnes: env="MUSUBI_SPM_TESTDATA"
// arnes: de="return (u>>8)&1 == 1"
// arnes: a="return (u>>9)&1 == 1"
// arnes: arreglo_de="return (u>>8)&1 == 1"
// arnes: arreglo_a="return (u&0x100)>>8 == 1"
func TestLaTokenizacionDelCharsmapEstaFijada(t *testing.T) {
	dir := os.Getenv("MUSUBI_SPM_TESTDATA")
	if dir == "" {
		t.Skip("MUSUBI_SPM_TESTDATA no seteado: se saltea la huella del charsmap")
	}
	tok, err := loadTokenizer(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		t.Fatalf("loadTokenizer(real): %v", err)
	}

	// CADA CASO ESTÁ ACÁ PORQUE ATRAVIESA EL CHARSMAP, no por variedad decorativa: descompuestos
	// que el NFC recompone, ancho completo y kana de media anchura que el NFKC pliega, ligaduras,
	// hangul precompuesto, la İ turca, y números romanos y símbolos CJK de compatibilidad.
	casos := []string{
		"café niño",    // descompuestos: NFC los recompone
		"ＦＵＬＬＷＩＤＴＨ",      // ancho completo → ASCII
		"ﬁligrana ﬂor",   // ligaduras fi/fl
		"한글 조합",          // hangul precompuesto
		"İstanbul ırmak", // İ turca y ı sin punto
		"Привет мир",     // cirílico
		"مرحبا بالعالم",  // árabe (RTL)
		"カタカナ ひらがな",      // kana
		"ｱｲｳｴｵ",          // kana de media anchura → ancho completo
		"élève",        // dos combinantes distintas
		"Ⅻ ⅷ",            // números romanos de compatibilidad
		"㈱㌔㍿",            // símbolos CJK cuadrados
		"ﬅ ﬆ",            // ligaduras raras
		"ᾀ ᾳ",            // griego politónico con iota suscrita
	}

	h := sha256.New()
	for _, c := range casos {
		fmt.Fprintf(h, "%q=%v\n", c, tok.EncodeIDs(c))
	}
	huella := hex.EncodeToString(h.Sum(nil))[:16]

	// MEDIDA el 2026-09-12 contra el tokenizer real de potion-multilingual-128M, con x/text
	// v0.42.0. No está derivada de nada de este repo: es el valor que salió de correr esto.
	const esperada = "7126b392b5d20d14"
	if huella != esperada {
		t.Errorf(`LA TOKENIZACIÓN DEL CHARSMAP CAMBIÓ.
  huella medida : %s
  huella fijada : %s

Algo movió cómo se tokeniza el texto que pasa por el charsmap precompilado, y el `+"`model_id`"+` NO
lo refleja: se deriva de los BYTES del asset, no del código que los lee. O sea que los vectores
viejos se van a comparar por coseno contra los nuevos como si fueran del mismo modelo.

Candidatos, en orden: un bump de golang.org/x/text (la normalización), un cambio de tabla Unicode
del toolchain de Go, o un refactor del Unigram/charsmap en este repo.

SI EL CAMBIO ES DELIBERADO Y CORRECTO: actualizá la huella acá Y re-embebé la base
(`+"`musubi embed backfill --all`"+`), porque los vectores ya guardados quedaron con otra
tokenización.`, huella, esperada)
	}

	// EL CONTROL: que estos casos de verdad produzcan tokens. Una huella sobre catorce listas
	// vacías también sería estable, y estaría custodiando la nada.
	vacios := 0
	for _, c := range casos {
		if len(tok.EncodeIDs(c)) == 0 {
			vacios++
			t.Errorf("el caso %q no produjo NI UN token: no ejercita nada", c)
		}
	}
	t.Logf("huella=%s sobre %d casos, %d vacíos", huella, len(casos), vacios)
}
