package embedding

import (
	"encoding/binary"
	"math"
	"testing"
)

// TestConvertirTrozoArrastraElFloat32Partido — la rama que NINGUNA prueba alcanzaba.
//
// LO QUE SE MIDIÓ, Y ES EL MOTIVO DE ESTE ARCHIVO. El 2026-09-12 se puso un `panic` en la primera
// línea de `if nSobra > 0` (convertirTrozo) y corrió el paquete `internal/embedding` ENTERO, con la
// tabla real de 488 MB: 17,6 s, VERDE. La rama no se alcanza NUNCA. Incluida la subprueba que se
// llama, textualmente, «varios trozos, PARTIENDO un float32 entre trozos».
//
// POR QUÉ NO SE ALCANZABA, y es más interesante que un descuido. `trozoDeCarga` es 1 MiB, múltiplo
// de 4 a propósito, así que un trozo entero nunca parte un float32. Lo que SÍ puede partirlo es una
// LECTURA CORTA: `f.Read(buf)` puede devolver menos de lo pedido, y ahí `len(datos)` deja de ser
// múltiplo de 4. Con un `*os.File` sobre un archivo local eso no pasa, y los fixtures de
// `TestLaCargaEnStreamingDaLosMismosValoresQueLeerElArchivoEntero` son más chicos que un trozo: el
// blob llega ENTERO de una sola vez.
//
// Y EL «CONTROL POSITIVO» DE AQUELLA PRUEBA MEDÍA UNA PROPIEDAD QUE NO DECIDE NADA. Asserta que el
// blob arranque en un offset con resto 2 DENTRO DEL ARCHIVO. Ese número no entra en ninguna
// decisión del conversor: lo que decide si un float32 queda partido es `len(datos) % 4`, y `datos`
// sale del recorte del trozo. El control estaba escrito para distinguir «ejercito la rama difícil»
// de «no la ejercito», y no distinguía ninguna de las dos.
//
// ESTA PRUEBA VA DIRECTO AL CONVERSOR, que es donde la decisión vive, y le da trozos que SÍ parten.
// No se puede llegar por el camino del archivo sin fabricar una lectura corta; la unidad sí es
// alcanzable, y es la que tiene el arrastre.
func TestConvertirTrozoArrastraElFloat32Partido(t *testing.T) {
	valores := []float32{1, -2.5, 3.25, 4e10, -0.0001, 65535.5, 7, 8}
	crudo := make([]byte, 4*len(valores))
	for i, v := range valores {
		binary.LittleEndian.PutUint32(crudo[i*4:], math.Float32bits(v))
	}

	// Cada corte parte un float32 por un lugar distinto: 1, 2 y 3 bytes de sobra, más un corte
	// que deja UN SOLO byte suelto (el caso en que ni siquiera alcanza para completar el valor y
	// hay que arrastrar de nuevo).
	cortes := [][]int{
		{5, 27},          // resto 1 en el primer trozo
		{6, 26},          // resto 2
		{7, 25},          // resto 3
		{3, 4, 25},       // el segundo trozo NO alcanza a completar: se arrastra dos veces
		{1, 1, 1, 1, 28}, // de a un byte al principio
	}

	for _, corte := range cortes {
		t.Run(nombreDelCorte(corte), func(t *testing.T) {
			tabla := make([]float32, len(valores))
			var sobra [4]byte
			nSobra, escritos, desde := 0, 0, 0
			partio := false
			for _, n := range corte {
				if desde+n > len(crudo) {
					n = len(crudo) - desde
				}
				datos := crudo[desde : desde+n]
				desde += n
				antes := nSobra
				escritos, nSobra = convertirTrozo(datos, tabla, escritos, sobra[:], nSobra)
				if antes > 0 {
					partio = true
				}
			}
			// Lo que quede sin consumir, en un último trozo.
			if desde < len(crudo) {
				antes := nSobra
				escritos, nSobra = convertirTrozo(crudo[desde:], tabla, escritos, sobra[:], nSobra)
				if antes > 0 {
					partio = true
				}
			}

			// EL CONTROL, Y ES EL QUE FALTABA EN LA PRUEBA VIEJA: que el corte de verdad haya
			// ejercitado el arrastre. Sin esto, un corte mal elegido caería en la rama fácil y este
			// caso estaría en verde sin haber tocado lo que dice tocar — que es exactamente lo que
			// pasaba antes.
			if !partio {
				t.Fatalf("el corte %v NO partió ningún float32: este caso no ejercita el arrastre, "+
					"que es lo único que vino a probar", corte)
			}
			if nSobra != 0 {
				t.Errorf("quedaron %d bytes sin consumir: el arrastre perdió un float32 por el camino", nSobra)
			}
			if escritos != len(valores) {
				t.Fatalf("se escribieron %d valores de %d", escritos, len(valores))
			}
			for i := range valores {
				if math.Float32bits(tabla[i]) != math.Float32bits(valores[i]) {
					t.Errorf("valor %d: %v != %v (bits %08x != %08x) — el arrastre entre trozos "+
						"corrompió el float32 partido",
						i, tabla[i], valores[i], math.Float32bits(tabla[i]), math.Float32bits(valores[i]))
				}
			}
		})
	}
}

func nombreDelCorte(corte []int) string {
	s := "corte"
	for _, n := range corte {
		s += "-" + itoa(n)
	}
	return s
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
