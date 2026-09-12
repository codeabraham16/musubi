package embedding

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// safetensorsDePrueba fabrica un model.safetensors válido con `filas`x`dim` floats deterministas.
//
// `partePorMedio` elige DÓNDE arranca el blob: el header se rellena con espacios hasta que el
// offset quede alineado a 4 (los trozos de lectura caen justo) o en resto 2 (un float32 queda
// PARTIDO entre dos trozos). Ese corte es el único camino no trivial de convertirTrozo.
//
// El relleno se CALCULA y no se pasa a mano: escrito como un número fijo, el largo del header base
// —que depende de cuántos dígitos tengan filas y dim— decidía la alineación por accidente, y los
// casos «alineado» no estaban alineados. Lo cazó el control de este mismo test.
func safetensorsDePrueba(t *testing.T, dir string, filas, dim int, partePorMedio bool) {
	t.Helper()
	cuerpo := fmt.Sprintf(`{"embeddings":{"dtype":"F32","shape":[%d,%d],"data_offsets":[0,%d]}`, filas, dim, filas*dim*4)
	quiero := 0
	if partePorMedio {
		quiero = 2
	}
	// offset del blob = 8 (el largo) + len(cuerpo) + relleno + 1 (la llave que cierra)
	relleno := ((quiero-(8+len(cuerpo)+1))%4 + 4) % 4
	hdr := cuerpo + strings.Repeat(" ", relleno) + "}"

	blob := make([]byte, filas*dim*4)
	for i := range filas * dim {
		// Valores con exponente y mantisa variados, para que un byte mal puesto se note.
		v := float32(i%997)*0.5 - float32(i%13)*1e-3
		binary.LittleEndian.PutUint32(blob[i*4:], math.Float32bits(v))
	}
	var largo [8]byte
	binary.LittleEndian.PutUint64(largo[:], uint64(len(hdr)))
	if err := os.WriteFile(filepath.Join(dir, "model.safetensors"), append(append(largo[:], hdr...), blob...), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestLaCargaEnStreamingDaLosMismosValoresQueLeerElArchivoEntero es LA guarda del cambio: la carga
// en streaming existe sólo para no tener los bytes crudos y su conversión vivos a la vez, y eso no
// vale nada si produce OTROS vectores. Compara contra `os.ReadFile` + `parseStaticTable`, que es la
// implementación que venía andando, y exige igualdad BIT A BIT.
//
// Los tamaños no son arbitrarios. `relleno` mueve el arranque del blob para cubrir los dos casos:
// alineado a 4 (los trozos caen justo) y desalineado (un float32 queda PARTIDO entre dos trozos, y
// hay que arrastrar los bytes sobrantes). El segundo es el único camino no trivial del conversor y
// sin él esta guarda estaría mirando la mitad fácil.
func TestLaCargaEnStreamingDaLosMismosValoresQueLeerElArchivoEntero(t *testing.T) {
	casos := []struct {
		nombre        string
		filas, dim    int
		relleno       int
		partePorMedio bool
	}{
		{"chico y alineado", 4, 8, 0, false},
		{"chico y desalineado", 4, 8, 2, true},
		{"varios trozos, alineado", 90000, 4, 0, false},
		{"varios trozos, PARTIENDO un float32 entre trozos", 90000, 4, 2, true},
		{"una sola fila", 1, 256, 1, true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			dir := t.TempDir()
			safetensorsDePrueba(t, dir, c.filas, c.dim, c.partePorMedio)
			ruta := filepath.Join(dir, "model.safetensors")

			crudo, err := os.ReadFile(ruta)
			if err != nil {
				t.Fatal(err)
			}
			esperada, filasRef, dimRef, err := parseStaticTable(crudo)
			if err != nil {
				t.Fatalf("el camino de referencia no pudo leer el fixture: %v", err)
			}

			// CONTROL DE QUE EL FIXTURE HACE LO QUE DICE: que el blob arranque (des)alineado
			// como el caso pretende. Sin esto, un cambio en el header dejaría todos los casos
			// cayendo en la rama fácil y la guarda seguiría verde sin mirar lo que dice mirar.
			hlen := binary.LittleEndian.Uint64(crudo[:8])
			inicioBlob := 8 + int(hlen)
			if parte := inicioBlob%4 != 0; parte != c.partePorMedio {
				t.Fatalf("el fixture no ejercita lo que declara: el blob arranca en %d (resto %d) y el caso dice partePorMedio=%v",
					inicioBlob, inicioBlob%4, c.partePorMedio)
			}

			tabla, filas, dim, crc, leidos, err := cargarTablaEnStreaming(ruta)
			if err != nil {
				t.Fatalf("la carga en streaming falló: %v", err)
			}
			if filas != filasRef || dim != dimRef {
				t.Fatalf("forma distinta: streaming %dx%d, referencia %dx%d", filas, dim, filasRef, dimRef)
			}
			if leidos != int64(len(crudo)) {
				t.Errorf("contó %d bytes leídos y el archivo tiene %d", leidos, len(crudo))
			}
			if len(tabla) != len(esperada) {
				t.Fatalf("largos distintos: streaming %d, referencia %d", len(tabla), len(esperada))
			}
			for i := range esperada {
				if math.Float32bits(tabla[i]) != math.Float32bits(esperada[i]) {
					t.Fatalf("difieren en el índice %d (fila %d): streaming %v, referencia %v",
						i, i/dim, tabla[i], esperada[i])
				}
			}

			// Y LA IDENTIDAD, que es lo que decide si los vectores viejos se siguen reconociendo.
			tokRaw := []byte(`{"model":{"type":"WordPiece"}}`)
			porStreaming := checksumDeCRC(crc, leidos, crc32.Checksum(tokRaw, castagnoli), int64(len(tokRaw)))
			porBuffer := staticTableChecksum(crudo, tokRaw)
			if porStreaming != porBuffer {
				t.Fatalf("la identidad NO coincide: streaming %s, buffer %s — los vectores ya escritos dejarían de reconocerse",
					porStreaming, porBuffer)
			}
		})
	}
}

// TestLaCargaEnStreamingRechazaUnArchivoRoto: la versión vieja validaba los offsets contra el
// buffer entero (lo tenía). La nueva no lo tiene, así que las mismas roturas tienen que seguir
// saliendo como ERROR y no como una tabla a medio llenar.
func TestLaCargaEnStreamingRechazaUnArchivoRoto(t *testing.T) {
	t.Run("blob truncado", func(t *testing.T) {
		dir := t.TempDir()
		safetensorsDePrueba(t, dir, 100, 8, false)
		ruta := filepath.Join(dir, "model.safetensors")
		crudo, err := os.ReadFile(ruta)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(ruta, crudo[:len(crudo)-40], 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, _, _, _, err := cargarTablaEnStreaming(ruta); err == nil {
			t.Fatal("un blob truncado tiene que fallar, no devolver una tabla a medio llenar")
		}
	})

	t.Run("header que declara un tamaño absurdo", func(t *testing.T) {
		dir := t.TempDir()
		ruta := filepath.Join(dir, "model.safetensors")
		var largo [8]byte
		binary.LittleEndian.PutUint64(largo[:], 1<<62)
		if err := os.WriteFile(ruta, append(largo[:], []byte("{}")...), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, _, _, _, err := cargarTablaEnStreaming(ruta); err == nil {
			t.Fatal("un header de 2^62 bytes tiene que fallar ANTES de pedirle esa memoria al sistema")
		}
	})

	t.Run("forma que no cuadra con el blob", func(t *testing.T) {
		dir := t.TempDir()
		ruta := filepath.Join(dir, "model.safetensors")
		hdr := `{"embeddings":{"dtype":"F32","shape":[10,10],"data_offsets":[0,16]}}`
		var largo [8]byte
		binary.LittleEndian.PutUint64(largo[:], uint64(len(hdr)))
		cuerpo := append(append(largo[:], hdr...), make([]byte, 16)...)
		if err := os.WriteFile(ruta, cuerpo, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, _, _, _, err := cargarTablaEnStreaming(ruta); err == nil {
			t.Fatal("100 floats declarados en 16 bytes tiene que fallar")
		}
	})

	t.Run("sin el tensor embeddings", func(t *testing.T) {
		dir := t.TempDir()
		ruta := filepath.Join(dir, "model.safetensors")
		hdr := `{"otra_cosa":{"dtype":"F32","shape":[2,2],"data_offsets":[0,16]}}`
		var largo [8]byte
		binary.LittleEndian.PutUint64(largo[:], uint64(len(hdr)))
		if err := os.WriteFile(ruta, append(append(largo[:], hdr...), make([]byte, 16)...), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, _, _, _, _, err := cargarTablaEnStreaming(ruta); err == nil {
			t.Fatal("un safetensors sin el tensor \"embeddings\" tiene que fallar")
		}
	})
}

// TestElArranqueNoTieneLaTablaDosVecesEnMemoria mide la razón de ser del cambio sobre la TABLA REAL:
// que la identidad salga idéntica a la del camino viejo, y que el pico de memoria baje.
//
// El pico se REPORTA y no se aserta: depende de la máquina y de cuánto le dejó el GC, y una guarda
// sobre milisegundos o megabytes ajenos es una guarda que se pone roja en el runner de otro. Lo que
// sí se aserta es la identidad, que no depende de nada externo.
func TestElArranqueNoTieneLaTablaDosVecesEnMemoria(t *testing.T) {
	dir := dirDeTablaReal(t)
	ruta := filepath.Join(dir, "model.safetensors")

	// (1) EL CAMINO NUEVO, en un proceso con el heap limpio.
	runtime.GC()
	var antes runtime.MemStats
	runtime.ReadMemStats(&antes)
	tabla, filas, dim, crc, leidos, err := cargarTablaEnStreaming(ruta)
	if err != nil {
		t.Fatal(err)
	}
	var despues runtime.MemStats
	runtime.ReadMemStats(&despues)
	picoNuevo := despues.TotalAlloc - antes.TotalAlloc
	t.Logf("STREAMING: tabla %dx%d · %d bytes leídos · asignado en total %d MB",
		filas, dim, leidos, picoNuevo>>20)

	// (2) LA IDENTIDAD, contra el camino viejo sobre el archivo entero. Éste es el aserto.
	crudo, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatal(err)
	}
	tokRaw, err := os.ReadFile(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		t.Fatal(err)
	}
	porBuffer := staticTableChecksum(crudo, tokRaw)
	porStreaming := checksumDeCRC(crc, leidos, crc32.Checksum(tokRaw, castagnoli), int64(len(tokRaw)))
	if porStreaming != porBuffer {
		t.Fatalf("LA IDENTIDAD CAMBIÓ: streaming %s, buffer %s. Todos los vectores ya escritos "+
			"dejarían de reconocerse y el recall semántico saldría vacío en silencio", porStreaming, porBuffer)
	}
	t.Logf("identidad idéntica por los dos caminos: %s", porStreaming)

	// (3) Y LOS VALORES, sobre la tabla real y entera.
	esperada, _, _, err := parseStaticTable(crudo)
	if err != nil {
		t.Fatal(err)
	}
	if len(tabla) != len(esperada) {
		t.Fatalf("largos distintos: %d vs %d", len(tabla), len(esperada))
	}
	for i := range esperada {
		if math.Float32bits(tabla[i]) != math.Float32bits(esperada[i]) {
			t.Fatalf("difieren en el índice %d (fila %d)", i, i/dim)
		}
	}
	t.Logf("%d valores idénticos entre los dos caminos", len(esperada))
}
