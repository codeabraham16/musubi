package embedding

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestElTopeDelHeaderRechazaUnLargoAbsurdo — la cota que NADIE custodiaba.
//
// LO QUE SE MIDIÓ, Y ES EL MOTIVO DE ESTE ARCHIVO. El 2026-09-12 se aflojó `topeDeHeader` de
// 8 MiB a 8 TiB —cinco órdenes de magnitud— y el paquete `internal/embedding` ENTERO quedó en
// VERDE con la tabla real de 488 MB. La constante existe para que un safetensors corrupto que
// declare un header gigante no haga que `make([]byte, hlen)` pida esa cantidad, y no la medía nadie.
//
// EL DISCRIMINADOR ES CUÁL ERROR SALE, NO QUE SALGA UN ERROR. Sin la cota, el `make` de 1 GiB
// ANDA —Go reserva memoria virtual y la toca recién al escribirla—, el `io.ReadFull` posterior
// falla, y la carga devuelve «header safetensors truncado». O sea que un test que sólo pidiera
// «error» quedaría VERDE con la cota borrada, habiendo reservado el gigabyte. Por eso se exige la
// frase que SÓLO puede escribir la cota.
func TestElTopeDelHeaderRechazaUnLargoAbsurdo(t *testing.T) {
	// EL CONTROL, PRIMERO: un archivo sano se carga. Sin esto, esta prueba pasaría en verde con la
	// carga rota del todo, que es el modo en que una guarda de rechazo deja de medir nada.
	dir := t.TempDir()
	safetensorsDePrueba(t, dir, 4, 8, false)
	if _, _, _, _, _, err := cargarTablaEnStreaming(filepath.Join(dir, "model.safetensors")); err != nil {
		t.Fatalf("control: un safetensors sano no cargó (%v); esta prueba estaría midiendo el "+
			"rechazo sobre una función que ya no funciona", err)
	}

	// 1 GiB es un HECHO DEL MUNDO y no se deriva de `topeDeHeader`: el header real de POTION son
	// 80 bytes, y ningún safetensors legítimo declara un JSON de mil megas. Derivarlo de la
	// constante que custodia —`topeDeHeader + 1`— volvería espejo a la guarda: aflojar la constante
	// aflojaría la prueba y las dos dirían que sí.
	const absurdo = 1 << 30

	roto := filepath.Join(t.TempDir(), "model.safetensors")
	cuerpo := make([]byte, 8+64)
	binary.LittleEndian.PutUint64(cuerpo[:8], absurdo)
	if err := os.WriteFile(roto, cuerpo, 0o600); err != nil {
		t.Fatal(err)
	}

	_, _, _, _, _, err := cargarTablaEnStreaming(roto)
	if err == nil {
		t.Fatal("un header que declara 1 GiB se aceptó")
	}
	if !strings.Contains(err.Error(), "header safetensors inválido") {
		t.Errorf(`el rechazo NO vino de la cota, vino de más abajo: %v

Eso significa que se reservaron los %d bytes y recién después falló la lectura. La cota
`+"`topeDeHeader`"+` es lo único entre un archivo corrupto y un `+"`make`"+` de ese tamaño, y este
mensaje dice que no actuó.`, err, absurdo)
	}

	// SEGUNDA RED, con otro vocabulario: la magnitud de la constante misma. La de arriba pregunta
	// por el COMPORTAMIENTO y deja una ventana —aflojar de 8 MiB a 64 MiB no la mueve—; ésta
	// pregunta por el NÚMERO y no sabe nada de la carga. Ninguna de las dos cubre sola las dos
	// formas de romperlo, y no se conocen entre ellas.
	//
	// Los dos extremos son hechos del mundo, no derivados: por arriba, ningún header de
	// safetensors llega a 64 MiB (el de POTION son 80 bytes); por abajo, achicarlo de 4 KiB
	// empezaría a rechazar archivos legítimos con muchos tensores.
	if topeDeHeader > 64<<20 {
		t.Errorf("topeDeHeader vale %d (%d MiB) y ningún header de safetensors llega a 64 MiB: "+
			"la cota dejó de acotar", topeDeHeader, topeDeHeader>>20)
	}
	if topeDeHeader < 4<<10 {
		t.Errorf("topeDeHeader vale %d y eso rechaza headers legítimos: un safetensors con muchos "+
			"tensores pasa los 4 KiB de JSON sin ser sospechoso", topeDeHeader)
	}
}

// TestElHeaderNoSeDaVueltaAlConvertirlo — el desborde de `parseStaticTable`, que era un PÁNICO.
//
// `hdrEnd := 8 + int(hlen)` con `hlen` de 2^63 o más se da vuelta: `int(0xFFFFFFFFFFFFFFFF)` es -1.
// `hdrEnd` quedaba negativo, la comparación `hdrEnd > len(raw)` lo dejaba pasar, y el
// `raw[8:hdrEnd]` de abajo largaba `slice bounds out of range`. Medido el 2026-09-12:
//
//	hlen=0x8000000000000001 → PÁNICO: slice bounds out of range [:-9223372036854775799]
//	hlen=0xffffffffffffffff → PÁNICO: slice bounds out of range [8:7]
//	hlen=0x4000000000000000 → rechazado bien (no se da vuelta)
//
// HOY NO LO ALCANZA PRODUCCIÓN y conviene decirlo en vez de inflarlo: `parseStaticTable` no tiene
// NI UN llamador fuera de las pruebas —el camino vivo es `cargarTablaEnStreaming`, que sí tiene
// `topeDeHeader`—. Es la implementación de REFERENCIA contra la que se compara la de streaming.
// Pero una referencia que se cae con una entrada que la real rechaza es una trampa puesta: el día
// que alguien la vuelva a cablear (el camino de mmap es el candidato), la cota no viaja con ella.
func TestElHeaderNoSeDaVueltaAlConvertirlo(t *testing.T) {
	// El control: un header válido se parsea. Si esto se rompe, lo de abajo mide el rechazo sobre
	// una función que ya no parsea nada y todos los casos pasan por el motivo equivocado.
	dir := t.TempDir()
	safetensorsDePrueba(t, dir, 4, 8, false)
	sano, err := os.ReadFile(filepath.Join(dir, "model.safetensors"))
	if err != nil {
		t.Fatal(err)
	}
	if _, filas, dim, err := parseStaticTable(sano); err != nil || filas != 4 || dim != 8 {
		t.Fatalf("control: un safetensors sano no parseó (filas=%d dim=%d err=%v)", filas, dim, err)
	}

	casos := []struct {
		nombre string
		hlen   uint64
	}{
		{"2^63: el primero que se da vuelta", 1 << 63},
		{"2^63+1: da vuelta a un negativo grande", 1<<63 + 1},
		{"2^64-1: da vuelta a -1, y hdrEnd queda en 7", ^uint64(0)},
		{"2^62: NO se da vuelta, tiene que rechazarse igual", 1 << 62},
		{"len(raw): justo uno de más", uint64(len(sano) - 8 + 1)},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			raw := make([]byte, len(sano))
			copy(raw, sano)
			binary.LittleEndian.PutUint64(raw[:8], c.hlen)

			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("hlen=%#x hizo PANIQUEAR a parseStaticTable: %v\n"+
						"El largo del header lo elige el ARCHIVO. Un pánico acá es un archivo "+
						"corrupto llevándose al que lo lea.", c.hlen, r)
				}
			}()
			if _, _, _, err := parseStaticTable(raw); err == nil {
				t.Errorf("hlen=%#x (int = %d) se aceptó y el archivo tiene %d bytes",
					c.hlen, int(c.hlen), len(raw))
			}
		})
	}
}
