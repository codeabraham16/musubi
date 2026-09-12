package embedding

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"testing"
	"time"
)

// ¿PUEDE EL HOOK POR TURNO PAGAR LA SEÑAL VECTORIAL?
//
// El hook `musubi turn --hook-mode` es un PROCESO EFÍMERO —uno nuevo por prompt— con techo de 10 s.
// Hoy la señal vectorial NO se le entrega: cmd/musubi/embed.go apaga el embebedor cuando construirlo
// es caro (embedderCaroDeConstruir). La señal vale la pena —medido aparte: nDCG@1 0,294 → 0,353—
// así que la pregunta es si hay alguna forma de entregarla bajo el techo.
//
// Este archivo NO es una guarda de rendimiento: los tiempos dependen de la máquina y no se asertan.
// Es el INSTRUMENTO que contestó la pregunta, guardado para que la próxima corrida no empiece de
// cero. Lo único que sí se aserta es una invariante de CORRECCIÓN (ver el archivo _unix).
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// CÓMO CORRER ESTO, Y POR QUÉ IMPORTA
//
// UNA MEDICIÓN POR PROCESO. `go test -run TestMedirElArranque...` SOLA, no toda la tanda junta.
// Cada una de estas pruebas deja entre 0,5 y 1,3 GB tocados, así que corridas en el mismo binario
// se contaminan entre sí y los números dejan de querer decir algo. No es teoría: corriendo las
// cinco juntas, la comparación del tokenizer (abajo) DIO VUELTA EL SIGNO —de −35 % a +41 %— y el
// desglose de etapas pasó de 4,9 s a 121 s. Dos veces en esta misma tanda estuve por publicar una
// conclusión que era un artefacto de mi propia medición.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// MEDIDO EL 2026-09-11, tabla potion-multilingual-128M (488 MB + 17 MB de tokenizer.json),
// un proceso fresco por medición, en una máquina de 7,6 GB con 8,5 GB YA en swap.
//
//	etapa                          hoy                    con la tabla mapeada
//	─────────────────────────────  ─────────────────────  ──────────────────────
//	ReadFile(model.safetensors)     1019 ms                  0,06 ms
//	parseStaticTable (copia f32)     534 ms                  0,5  ms
//	ReadFile(tokenizer.json)          26 ms                   26 ms
//	loadTokenizerBytes               983 ms                  983 ms
//	identidad de la tabla             86 ms                 2000-3035 ms  ← ojo
//	─────────────────────────────  ─────────────────────  ──────────────────────
//	NewStaticProvider, punta a punta 3298 ms / 1325 MB RSS   1013 ms / 188 MB RSS
//
// CUATRO COSAS QUE SALIERON DISTINTAS DE LO ESPERADO:
//
//  1. EL CHECKSUM SE DA VUELTA AL MAPEAR. Sobre un buffer ya residente cuesta 86 ms; sobre un mmap
//     cuesta 2-3 SEGUNDOS y se lleva entre 292 y 452 MB de RSS, porque recorrer las 488 MB FUERZA a
//     traer cada página. O sea que «mapear la tabla en vez de leerla», hecho solo, deja el arranque
//     PEOR que hoy. El mmap sólo rinde si la identidad deja de tocar la tabla entera — una
//     identidad muestreada (256 ventanas de 4 KB estriadas) cuesta 18-29 ms y +27 MB.
//
//  2. CON ESO ARREGLADO, LA TABLA DEJA DE IMPORTAR Y EL TOKENIZER PASA A SER TODO. La tabla cae a
//     0,6 ms; el tokenizer queda en ~1009 ms, que es el 97 % del piso. Nadie lo tenía señalado, y
//     su costo NO está en parsear de más (ver la prueba de abajo): está en construir el mapa de
//     500.353 entradas. Bajarlo pide un caché binario del tokenizer ya armado, que es obra aparte.
//
//  3. EL RECURSO ESCASO NO ES EL CPU, ES LA MEMORIA. El arranque de hoy pide 1325 MB de RSS —la
//     tabla entra DOS veces, los bytes crudos y la copia a []float32— y en esta máquina esa plata
//     no está. La ganancia que importa no es 3298 ms → 1013 ms: es 1325 MB → 188 MB.
//
//  4. LOS ABSOLUTOS NO SON TRANSPORTABLES. El mismo ReadFile midió 1019 ms con la máquina holgada
//     y 3356 ms con ella cargada. Lo que se sostiene son las RAZONES entre etapas y los órdenes de
//     magnitud, no los milisegundos.
//
// El piso, 1013 ms, ENTRA bajo el techo de 10 s. Eso es lo que este archivo establece y antes no
// se sabía: el camino es viable, y hay que empezarlo por el tokenizer, no por la tabla.
func TestMedirElArranqueDelProveedorEstatico(t *testing.T) {
	dir := dirDeTablaReal(t)

	// GC APAGADO A PROPÓSITO. Con el GC prendido, el marcado concurrente de un heap de ~1,3 GB
	// corre DEBAJO de la etapa que se está midiendo y le carga tiempo que no es suyo: la primera
	// corrida de esta medición le adjudicó 1712 ms al checksum, y aislado son 53-86 ms. El
	// comentario de staticTableChecksum (que dice ~39 ms) tenía razón; la medición estaba mal.
	defer debug.SetGCPercent(debug.SetGCPercent(-1))

	t0 := time.Now()
	tableRaw, err := os.ReadFile(filepath.Join(dir, "model.safetensors"))
	if err != nil {
		t.Fatal(err)
	}
	dRead := time.Since(t0)

	t1 := time.Now()
	_, rows, dim, err := parseStaticTable(tableRaw)
	if err != nil {
		t.Fatal(err)
	}
	dParse := time.Since(t1)

	t2 := time.Now()
	tokRaw, err := os.ReadFile(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		t.Fatal(err)
	}
	dReadTok := time.Since(t2)

	t3 := time.Now()
	if _, err := loadTokenizerBytes(tokRaw); err != nil {
		t.Fatal(err)
	}
	dTok := time.Since(t3)

	t4 := time.Now()
	id := staticTableChecksum(tableRaw, tokRaw)
	dSum := time.Since(t4)

	total := dRead + dParse + dReadTok + dTok + dSum
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	t.Logf("tabla %d x %d (%d MB) · tokenizer %d MB · id=%s", rows, dim, len(tableRaw)>>20, len(tokRaw)>>20, id)
	for _, e := range []struct {
		nombre string
		d      time.Duration
	}{
		{"ReadFile(model.safetensors)", dRead},
		{"parseStaticTable", dParse},
		{"ReadFile(tokenizer.json)", dReadTok},
		{"loadTokenizerBytes", dTok},
		{"staticTableChecksum", dSum},
	} {
		t.Logf("  %-28s %8.1f ms  %5.1f %%", e.nombre, msDe(e.d), 100*float64(e.d)/float64(total))
	}
	t.Logf("  %-28s %8.1f ms · heap vivo %d MB", "TOTAL", msDe(total), ms.HeapAlloc>>20)
}

// TestElDobleParseoDelTokenizerNoEsElProblema conserva una hipótesis MÍA REFUTADA, porque la
// refutación es el resultado y sin ella el próximo la vuelve a intentar.
//
// loadTokenizerBytes deserializa el tokenizer.json ENTERO DOS VECES: una para sacar `model.type`
// (con Model como struct{Type string}) y otra, en la rama Unigram, para el vocabulario. Con 17 MB
// y 500.353 piezas eso se lee como desperdicio evidente, y el arreglo parece obvio: tomar `model`
// como json.RawMessage en la primera pasada y deserializar sólo ese raw en la segunda.
//
// MEDIDO EN TRES PROCESOS FRESCOS, EL «ARREGLO» ES UN 34-38 % MÁS LENTO (383-390 ms → 520-530 ms),
// y coincide con lo que dice contar el trabajo: hoy son DOS recorridos del documento más una
// materialización del vocab; con RawMessage son TRES recorridos, más una copia de 17 MB, más la
// misma materialización. Dos motivos concretos:
//
//   - json.RawMessage COPIA. El objeto `model` son casi los 17 MB enteros, así que la primera
//     pasada deja de descartarlos y pasa a guardarlos.
//   - Sacar el type de ese raw cuesta OTRA pasada sobre los 17 MB, mientras que hoy el
//     decodificador de Go SALTEA los campos que no le pedís sin materializarlos, y eso ya es más
//     barato que copiarlos.
//
// (Una corrida de las cinco pruebas EN EL MISMO PROCESO dio +41 %, o sea el signo al revés. Ésa
// es la corrida mala: ver el encabezado del archivo. La conclusión es la de los procesos frescos,
// que además es la que predice contar los recorridos.)
//
// El costo del tokenizer no está en parsear de más: está en construir el mapa de 500 K entradas y
// los scores. Bajarlo pide un CACHÉ BINARIO del tokenizer ya construido, que es obra aparte.
func TestElDobleParseoDelTokenizerNoEsElProblema(t *testing.T) {
	dir := dirDeTablaReal(t)
	b, err := os.ReadFile(filepath.Join(dir, "tokenizer.json"))
	if err != nil {
		t.Fatal(err)
	}

	var hoy struct {
		Normalizer   json.RawMessage `json:"normalizer"`
		PreTokenizer tkMetaspace     `json:"pre_tokenizer"`
		Model        struct {
			Type string `json:"type"`
		} `json:"model"`
	}
	t0 := time.Now()
	if err := json.Unmarshal(b, &hoy); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Model tkUnigramModel `json:"model"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	dHoy := time.Since(t0)

	var conRaw struct {
		Normalizer   json.RawMessage `json:"normalizer"`
		PreTokenizer tkMetaspace     `json:"pre_tokenizer"`
		Model        json.RawMessage `json:"model"`
	}
	t1 := time.Now()
	if err := json.Unmarshal(b, &conRaw); err != nil {
		t.Fatal(err)
	}
	var tipo struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(conRaw.Model, &tipo); err != nil {
		t.Fatal(err)
	}
	var modelo tkUnigramModel
	if err := json.Unmarshal(conRaw.Model, &modelo); err != nil {
		t.Fatal(err)
	}
	dRaw := time.Since(t1)

	if len(doc.Model.Vocab) != len(modelo.Vocab) {
		t.Fatalf("los dos caminos no leyeron el mismo vocabulario: %d vs %d", len(doc.Model.Vocab), len(modelo.Vocab))
	}
	t.Logf("tokenizer.json %d MB · %d piezas · type=%q", len(b)>>20, len(modelo.Vocab), tipo.Type)
	t.Logf("  como está hoy (dos unmarshal del documento): %8.1f ms", msDe(dHoy))
	t.Logf("  con model como RawMessage (el «arreglo»):    %8.1f ms", msDe(dRaw))
	t.Logf("  diferencia: %+.1f ms (%+.0f %%) — negativo = el «arreglo» pierde", msDe(dHoy-dRaw), 100*float64(dHoy-dRaw)/float64(dHoy))
}

// dirDeTablaReal devuelve el directorio de la tabla POTION real, o saltea. Estas mediciones no
// pueden correr contra un fixture sintético: lo que miden es el costo de UNA TABLA DE 488 MB.
func dirDeTablaReal(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("MUSUBI_POTION_DIR")
	if dir == "" {
		t.Skip("sin MUSUBI_POTION_DIR: esta medición necesita la tabla POTION real")
	}
	if _, err := os.Stat(filepath.Join(dir, "model.safetensors")); err != nil {
		t.Skipf("MUSUBI_POTION_DIR sin model.safetensors: %v", err)
	}
	return dir
}

// msDe convierte una duración a milisegundos con decimales, para los t.Logf de este archivo.
func msDe(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
