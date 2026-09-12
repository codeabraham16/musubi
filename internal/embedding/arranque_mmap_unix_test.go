//go:build unix

package embedding

// LA MITAD UNIX DE LA MEDICIÓN DE arranque_real_test.go. Va aparte y con etiqueta `unix` porque
// syscall.Mmap NO EXISTE en Windows: dejarlo en el archivo portable rompería la compilación de
// test-cross (windows-latest), y ponerlo en un `*_windows_test.go` lo dejaría sin correr en
// ningún lado. Una implementación de verdad necesita las dos mitades (unix + CreateFileMapping);
// acá alcanza con una, porque lo que se está midiendo es SI EL CAMINO RINDE, no entregarlo.

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

// TestLaTablaMapeadaDaLosMismosVectores es la ÚNICA ASERCIÓN de esta tanda, y no es de tiempo: es
// de corrección. Todo el camino del mmap se apoya en que el blob de un safetensors F32 se puede
// ALIASAR como []float32 sin convertir elemento por elemento — que es de donde salen los 534 ms
// que hoy se van en parseStaticTable. Eso vale sólo si el orden de bytes de la máquina coincide
// con el del archivo (little-endian), y es justo el tipo de supuesto que compila, pasa, y devuelve
// vectores basura en silencio.
//
// La guarda compara los DOS caminos sobre la MISMA tabla y exige igualdad BIT A BIT.
//
// ES UNA GUARDA SOBRE UNA TÉCNICA, NO SOBRE UN LLAMADOR: hoy nadie en producción aliasa la tabla,
// y ubicarBlobDeEmbeddings vive en este mismo archivo. Lo que fija es que el atajo ES LEGÍTIMO
// antes de que alguien lo implemente, junto con sus DOS precondiciones —little-endian y blob
// alineado a 4— que es lo que un `unsafe.Slice` no te pregunta.
func TestLaTablaMapeadaDaLosMismosVectores(t *testing.T) {
	dir := dirDeTablaReal(t)
	if !esLittleEndian() {
		t.Skip("máquina big-endian: el aliasado directo no aplica y haría falta convertir")
	}

	leida, err := os.ReadFile(filepath.Join(dir, "model.safetensors"))
	if err != nil {
		t.Fatal(err)
	}
	porCopia, filas, dim, err := parseStaticTable(leida)
	if err != nil {
		t.Fatal(err)
	}

	mapeada, cerrar := mapearTabla(t, filepath.Join(dir, "model.safetensors"))
	defer cerrar()
	inicio, fin, filas2, dim2, err := ubicarBlobDeEmbeddings(mapeada)
	if err != nil {
		t.Fatal(err)
	}
	if filas2 != filas || dim2 != dim {
		t.Fatalf("las dos lecturas no coinciden en forma: %dx%d vs %dx%d", filas, dim, filas2, dim2)
	}
	blob := mapeada[inicio:fin]
	// LA SEGUNDA PRECONDICIÓN, Y NO ES TEÓRICA. unsafe.Slice a *float32 sobre una dirección que no
	// esté alineada a 4 es comportamiento indefinido: en x86 anda igual, en ARM puede romper — y
	// test-cross corre macos-latest, que es ARM. El offset del blob sale de 8 + largo del header
	// JSON + data_offsets[0], y NADA en el formato safetensors obliga a que eso sea múltiplo de 4:
	// en la tabla de POTION da 88 por cómo la escribió HuggingFace, no por garantía del formato.
	// Una implementación de verdad tiene que CHEQUEARLO y caer al camino de copia si no da.
	if inicio%4 != 0 {
		t.Fatalf("el blob arranca en el offset %d, que no está alineado a 4: aliasarlo como "+
			"[]float32 sería comportamiento indefinido y hay que caer al camino de copia", inicio)
	}
	if desalineado := uintptr(unsafe.Pointer(&blob[0])) % 4; desalineado != 0 {
		t.Fatalf("la dirección del blob mapeado no está alineada a 4 (resto %d)", desalineado)
	}
	porAlias := unsafe.Slice((*float32)(unsafe.Pointer(&blob[0])), filas*dim)

	if len(porAlias) != len(porCopia) {
		t.Fatalf("largos distintos: alias %d, copia %d", len(porAlias), len(porCopia))
	}
	for i := range porCopia {
		if porAlias[i] != porCopia[i] {
			t.Fatalf("la tabla aliasada difiere de la convertida en el índice %d (fila %d): %v vs %v",
				i, i/dim, porAlias[i], porCopia[i])
		}
	}
	t.Logf("%d valores idénticos entre la tabla convertida y la aliasada (%d filas x %d) · blob en el offset %d",
		len(porCopia), filas, dim, inicio)
}

// TestMedirElPisoDelArranqueConMmap mide el MEJOR CASO POSIBLE de este camino: tabla mapeada,
// aliasada sin copia, y SIN checksum de la tabla entera. Es el piso contra el que hay que
// comparar el techo de 10 s del hook. No aserta tiempos.
func TestMedirElPisoDelArranqueConMmap(t *testing.T) {
	dir := dirDeTablaReal(t)
	if !esLittleEndian() {
		t.Skip("máquina big-endian")
	}
	t.Logf("RSS al arrancar: %d MB", rssEnMB())

	t0 := time.Now()
	mapeada, cerrar := mapearTabla(t, filepath.Join(dir, "model.safetensors"))
	defer cerrar()
	inicio, fin, filas, dim, err := ubicarBlobDeEmbeddings(mapeada)
	if err != nil {
		t.Fatal(err)
	}
	blob := mapeada[inicio:fin]
	tabla := unsafe.Slice((*float32)(unsafe.Pointer(&blob[0])), filas*dim)
	dTabla := time.Since(t0)
	t.Logf("[1] tabla mapeada + aliasada:  %8.1f ms · RSS %d MB", msDe(dTabla), rssEnMB())

	t1 := time.Now()
	tokRaw, err := os.ReadFile(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		t.Fatal(err)
	}
	dLeerTok := time.Since(t1)
	t2 := time.Now()
	tok, err := loadTokenizerBytes(tokRaw)
	if err != nil {
		t.Fatal(err)
	}
	dTok := time.Since(t2)
	t.Logf("[2] tokenizer: leer %6.1f ms + construir %6.1f ms · RSS %d MB", msDe(dLeerTok), msDe(dTok), rssEnMB())

	p := &StaticProvider{table: tabla, rows: filas, dim: dim, tok: tok, modelID: "static:medicion"}
	t3 := time.Now()
	v, err := p.Embed(t.Context(), "por qué la cola de conflictos no se resuelve sola")
	if err != nil {
		t.Fatal(err)
	}
	dEmbed := time.Since(t3)
	t.Logf("[3] Embed de una consulta:     %8.1f ms (dim %d) · RSS %d MB", msDe(dEmbed), len(v), rssEnMB())
	t.Logf("PISO: %.1f ms · RSS %d MB  (hoy: ~3300 ms · ~1325 MB)", msDe(dTabla+dLeerTok+dTok+dEmbed), rssEnMB())
}

// TestMedirLoQueCuestaLaIdentidadSobreUnMmap es la medición que decide el camino, y la que da
// vuelta el diagnóstico que estaba escrito en cmd/musubi/embed.go.
//
// staticTableChecksum existe por N1: re-destilar la tabla in-place tiene que cambiar el model_id,
// o los vectores viejos siguen pareciendo compatibles y el ranking se corrompe EN SILENCIO. Cuesta
// ~86 ms sobre un buffer YA RESIDENTE en el heap. Sobre un mmap cuesta ~35 veces más, porque
// recorrer las 488 MB obliga al kernel a traer cada página: el checksum se come solo TODA la
// ganancia del mapeo.
//
// La alternativa medida acá es una identidad MUESTREADA (estriada sobre el archivo). Ojo con el
// RSS: 256 muestras de 4 KB son 1 MB leído y mueven 27 MB de RSS, porque el readahead del kernel
// trae una ventana entera por cada página tocada.
func TestMedirLoQueCuestaLaIdentidadSobreUnMmap(t *testing.T) {
	dir := dirDeTablaReal(t)
	mapeada, cerrar := mapearTabla(t, filepath.Join(dir, "model.safetensors"))
	defer cerrar()
	t.Logf("RSS con el mmap hecho y sin tocar nada: %d MB", rssEnMB())

	for _, muestras := range []int{256, 1024} {
		paso := len(mapeada) / muestras
		antes := rssEnMB()
		t0 := time.Now()
		h := crc32.New(castagnoli)
		for off := 0; off+4096 <= len(mapeada); off += paso {
			_, _ = h.Write(mapeada[off : off+4096])
		}
		t.Logf("identidad muestreada: %5d x 4 KB (%d KB) %8.1f ms · crc=%08x · RSS %d → %d MB",
			muestras, muestras*4, msDe(time.Since(t0)), h.Sum32(), antes, rssEnMB())
	}

	antes := rssEnMB()
	t1 := time.Now()
	c := crc32.Checksum(mapeada, castagnoli)
	t.Logf("identidad COMPLETA (la de hoy):       488 MB      %8.1f ms · crc=%08x · RSS %d → %d MB",
		msDe(time.Since(t1)), c, antes, rssEnMB())
}

// mapearTabla mapea el archivo en solo-lectura y devuelve los bytes y cómo soltarlos.
func mapearTabla(t *testing.T, ruta string) ([]byte, func()) {
	t.Helper()
	f, err := os.Open(ruta)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	b, err := syscall.Mmap(int(f.Fd()), 0, int(fi.Size()), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	return b, func() {
		_ = syscall.Munmap(b)
		_ = f.Close()
	}
}

// ubicarBlobDeEmbeddings es parseStaticTable SIN LA COPIA: lee el header del safetensors y
// devuelve los límites del blob f32 y su forma, sin tocar ni un byte del blob. Ésa es justamente
// la diferencia que se está midiendo — parseStaticTable convierte los 128 millones de f32 uno por
// uno (534 ms) y, sobre un mmap, además forzaría a traer las 488 MB enteras.
func ubicarBlobDeEmbeddings(raw []byte) (inicio, fin, filas, dim int, err error) {
	if len(raw) < 8 {
		return 0, 0, 0, 0, fmt.Errorf("safetensors demasiado corto")
	}
	hlen := binary.LittleEndian.Uint64(raw[:8])
	hdrEnd := 8 + int(hlen)
	if hlen == 0 || hdrEnd > len(raw) {
		return 0, 0, 0, 0, fmt.Errorf("header safetensors inválido")
	}
	var hdr map[string]json.RawMessage
	if err := json.Unmarshal(raw[8:hdrEnd], &hdr); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("header JSON: %w", err)
	}
	crudo, ok := hdr["embeddings"]
	if !ok {
		return 0, 0, 0, 0, fmt.Errorf("safetensors sin tensor \"embeddings\"")
	}
	var ti stTensor
	if err := json.Unmarshal(crudo, &ti); err != nil {
		return 0, 0, 0, 0, fmt.Errorf("tensor embeddings: %w", err)
	}
	if ti.Dtype != "F32" || len(ti.Shape) != 2 || len(ti.DataOffsets) != 2 {
		return 0, 0, 0, 0, fmt.Errorf("esperaba embeddings F32 2D, obtuve %s %v", ti.Dtype, ti.Shape)
	}
	filas, dim = ti.Shape[0], ti.Shape[1]
	inicio, fin = hdrEnd+int(ti.DataOffsets[0]), hdrEnd+int(ti.DataOffsets[1])
	if inicio < 0 || fin > len(raw) || fin-inicio != filas*dim*4 {
		return 0, 0, 0, 0, fmt.Errorf("blob de embeddings inconsistente")
	}
	return inicio, fin, filas, dim, nil
}

func esLittleEndian() bool {
	var x uint16 = 1
	return *(*byte)(unsafe.Pointer(&x)) == 1
}

// rssEnMB lee VmRSS de /proc/self/status. Devuelve -1 donde no exista (macOS).
func rssEnMB() int {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return -1
	}
	for _, l := range strings.Split(string(b), "\n") {
		if rest, ok := strings.CutPrefix(l, "VmRSS:"); ok {
			campos := strings.Fields(rest)
			if len(campos) == 0 {
				return -1
			}
			kb, err := strconv.Atoi(campos[0])
			if err != nil {
				return -1
			}
			return kb / 1024
		}
	}
	return -1
}
