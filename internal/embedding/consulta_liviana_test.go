package embedding

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// --- la tabla Unigram de juguete ------------------------------------------------------------

// piezasDeJuguete es un vocab Unigram chico pero con lo que ejercita la búsqueda por prefijo:
// piezas que son prefijo de otras (▁a, ▁ab, ▁abc), hojas sin extensión, runas de varios bytes, una
// pieza REPETIDA («dup», que el mapa resuelve quedándose con la última) y scores empatados.
func piezasDeJuguete() []string {
	p := []string{"[PAD]", "[UNK]", "▁"}
	for r := 'a'; r <= 'z'; r++ {
		p = append(p, string(r))
	}
	for r := 'A'; r <= 'Z'; r++ {
		p = append(p, string(r))
	}
	for r := '0'; r <= '9'; r++ {
		p = append(p, string(r))
	}
	for _, r := range ".,:;-_[]()¿?¡!/=@'\"áéíóúñü" {
		p = append(p, string(r))
	}
	return append(p,
		"▁a", "▁ab", "▁abc", "ab", "abc", "bc", "▁de", "▁deploy", "deploy", "▁la", "▁tabla", "tabla",
		"▁información", "informa", "ción", "▁vos", "▁querés", "és", "▁el", "▁en", "en", "es", "▁es",
		"▁que", "que", "▁pro", "▁proveedor", "dor", "▁vector", "▁consulta", "▁hook", "▁turno",
		"▁memoria", "▁musubi", "ku", "▁REDACTED", "RED", "ACT", "ED", "▁AK", "IA", "dup", "dup",
		"▁日本", "日本語", "語", "▁ñandú", "ñ", "▁ti", "tipeo", "▁fich", "aje", "▁ter", "minal",
	)
}

// escribirTablaDeJuguete escribe un tokenizer.json Unigram (sin Precompiled: el charsmap real lo
// cubre la prueba con la tabla real) y un model.safetensors con `filas` filas de `dim` valores
// pseudoaleatorios. Las filas pueden ser MÁS que las piezas: la tabla no tiene por qué medir lo
// mismo que el vocab, y así una prueba puede tener una tabla grande con un vocab chico.
func escribirTablaDeJuguete(t *testing.T, dir string, filas, dim int, semilla int64, extra ...string) {
	t.Helper()
	piezas := append(piezasDeJuguete(), extra...)
	if filas < len(piezas) {
		filas = len(piezas)
	}
	vocab := make([]any, len(piezas))
	for k, p := range piezas {
		sc := -1.0 - 0.173*float64(k%37) - 0.01*float64(k)
		if k%5 == 0 {
			sc = -3.0 // empates a propósito
		}
		vocab[k] = []any{p, sc}
	}
	vocab[1] = []any{"[UNK]", 0.0}
	doc := map[string]any{
		"normalizer": map[string]any{
			"type": "Sequence",
			"normalizers": []any{
				map[string]any{"type": "Replace", "pattern": map[string]any{"String": " "}, "content": " "},
				map[string]any{"type": "Replace", "pattern": map[string]any{"Regex": " {2,}"}, "content": " "},
				map[string]any{"type": "Strip", "strip_left": true, "strip_right": true},
			},
		},
		"pre_tokenizer": map[string]any{"type": "Metaspace", "replacement": "▁", "prepend_scheme": "always", "split": false},
		"model":         map[string]any{"type": "Unigram", "unk_id": 1, "vocab": vocab},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, archivoTokenizer), b, 0o644); err != nil {
		t.Fatal(err)
	}
	rnd := rand.New(rand.NewSource(semilla))
	blob := make([]byte, filas*dim*4)
	for i := 0; i < filas*dim; i++ {
		binary.LittleEndian.PutUint32(blob[i*4:], math.Float32bits(rnd.Float32()*2-1))
	}
	// Un header que NO deja el blob alineado a 4: la lectura por filas no puede depender de eso.
	hdr, _ := json.Marshal(map[string]any{
		"__metadata__": map[string]any{"x": "y"},
		"embeddings":   map[string]any{"dtype": "F32", "shape": []int{filas, dim}, "data_offsets": []int{0, len(blob)}},
	})
	if (8+len(hdr))%4 == 0 {
		hdr = append(hdr, ' ')
	}
	out := make([]byte, 8, 8+len(hdr)+len(blob))
	binary.LittleEndian.PutUint64(out, uint64(len(hdr)))
	out = append(out, hdr...)
	out = append(out, blob...)
	if err := os.WriteFile(filepath.Join(dir, archivoTabla), out, 0o644); err != nil {
		t.Fatal(err)
	}
}

// tablaDeJugueteConSidecars arma la tabla y construye el StaticProvider sobre ella, que es lo que
// escribe los sidecars. Devuelve el directorio y el proveedor completo, que es la referencia.
func tablaDeJugueteConSidecars(t *testing.T, filas, dim int) (string, *StaticProvider) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tabla-juguete")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	escribirTablaDeJuguete(t, dir, filas, dim, 7)
	sp, err := NewStaticProvider(dir)
	if err != nil {
		t.Fatalf("NewStaticProvider: %v", err)
	}
	for _, f := range []string{archivoIndice, archivoIdentidad} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("NewStaticProvider no dejó %s al lado de la tabla: %v", f, err)
		}
	}
	return dir, sp
}

// textosDePrueba son los 50 textos contra los que se compara bit a bit: tipeos, voseo con tilde,
// emojis, vacío, espacios raros, runas sin pieza, un texto de más de trozoInicial bytes y uno con
// forma de secreto (armado en ejecución, para que el escáner de secretos no lo vea en el fuente).
func textosDePrueba() []string {
	secreto := "AKIA" + "1234567890ABCDEF"
	largo := strings.Repeat("el proveedor de la consulta embebe el turno del hook con información ", 110)
	t := []string{
		"", " ", "   ", "\t\n", "a", "ab", "abc", "▁", "[UNK]", "dup dup dup",
		"infromacion", "temrinal", "fichjae", "rasberry kisoko", "comadno", "busqeuda",
		"¿Vos querés que lo hagás?", "tenés razón, vení", "información", "información",
		"🚀 deploy 🔥", "👩‍💻 memoria", "日本語のテキスト", "ñandú", "Ｈｅｌｌｏ ﬁ ① ™",
		"deploy  la   tabla", " deploy ", "  espacios al borde  ",
		"MUSUBI musubi Musubi", "el hook del turno trae la memoria",
		"clave " + secreto + " en el log", "token=" + secreto,
		largo, largo[:trozoInicial+1],
		strings.Repeat("abc", 300), strings.Repeat("z", 1000),
		"12345 67890", "a.b,c:d;e-f_g", "[PAD] [UNK]", "\x00\x01 control",
		"\xff\xfe inválido", "mezcla ab ▁ab abc bc", "que es en el", "tipeo fichaje terminal",
		"vector consulta proveedor", "REDACTED ACT RED ED", "AKIA", "¡¿!?", "aje minal ti",
		"el vector de la consulta sin cargar la tabla",
	}
	if len(t) != 50 {
		panic(fmt.Sprintf("textosDePrueba tiene %d textos y la prueba promete 50", len(t)))
	}
	return t
}

// compararBitABit falla si los dos proveedores no dan exactamente los mismos bits para cada texto.
func compararBitABit(t *testing.T, ref, liv Provider, textos []string) {
	t.Helper()
	ctx := context.Background()
	if ref.Name() != liv.Name() || ref.Dimensions() != liv.Dimensions() {
		t.Fatalf("identidad distinta: completo %s/%d, liviano %s/%d", ref.Name(), ref.Dimensions(), liv.Name(), liv.Dimensions())
	}
	for i, tx := range textos {
		a, errA := ref.Embed(ctx, tx)
		b, errB := liv.Embed(ctx, tx)
		if errA != nil || errB != nil {
			t.Fatalf("texto %d: errores completo=%v liviano=%v", i, errA, errB)
		}
		if len(a) != len(b) {
			t.Fatalf("texto %d (%q…): largo %d contra %d", i, recortar(tx), len(a), len(b))
		}
		for j := range a {
			if math.Float32bits(a[j]) != math.Float32bits(b[j]) {
				t.Fatalf("texto %d (%q…): el vector difiere en la componente %d: completo %v, liviano %v",
					i, recortar(tx), j, a[j], b[j])
			}
		}
	}
}

func recortar(s string) string {
	if r := []rune(s); len(r) > 40 {
		return string(r[:40])
	}
	return s
}

// --- las pruebas ----------------------------------------------------------------------------

// TestConsultaLivianaEsBitExacta fija LA promesa del embebedor de consulta: el vector que da es el
// de StaticProvider, bit a bit, para los 50 textos de textosDePrueba. Con tolerancia cero: un
// vector de consulta que difiere del del índice se compara contra vectores incompatibles EN
// SILENCIO, y el ranking se corrompe sin un solo error.
//
// Sabotaje: leer la fila del token siguiente en vez de la del token.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="c.inicio+int64(id)*filaBytes"
// arnes: a="c.inicio+int64(id+1)*filaBytes"
func TestConsultaLivianaEsBitExacta(t *testing.T) {
	dir, sp := tablaDeJugueteConSidecars(t, 400, 16)
	liv, err := NewConsultaLiviana(dir)
	if err != nil {
		t.Fatalf("NewConsultaLiviana con los sidecars recién escritos: %v", err)
	}
	compararBitABit(t, sp, liv, textosDePrueba())
}

// TestConsultaLivianaEsBitExactaConLaTablaReal es la misma comparación sobre POTION multilingüe,
// que es la tabla que usa este repo y la única con el charsmap real (Precompiled): la de juguete no
// lo tiene. Corre en el job recall-gate, que baja la tabla.
//
// ESCRIBE tokenizer.idx e identidad.json AL LADO DE LA TABLA, porque NewStaticProvider los escribe
// siempre: son los mismos que escribiría el daemon. En una máquina de desarrollo, apuntá
// MUSUBI_POTION_DIR a una copia si no querés que aparezcan en tu tabla.
//
// Sabotaje: armar el tokenizer del índice sin su normalizer (se pierde el charsmap).
// arnes: archivo="internal/embedding/indice_tokenizer.go"
// arnes: env="MUSUBI_POTION_DIR"
// arnes: de="steps, err := parseNormalizer(norm)"
// arnes: a="steps, err := parseNormalizer(norm[:0])"
func TestConsultaLivianaEsBitExactaConLaTablaReal(t *testing.T) {
	dir := os.Getenv("MUSUBI_POTION_DIR")
	if dir == "" {
		t.Skip("sin MUSUBI_POTION_DIR: esta comparación necesita la tabla POTION real")
	}
	sp, err := NewStaticProvider(dir)
	if err != nil {
		t.Fatalf("NewStaticProvider(%s): %v", dir, err)
	}
	liv, err := NewConsultaLiviana(dir)
	if err != nil {
		t.Fatalf("NewConsultaLiviana(%s) después de NewStaticProvider: %v", dir, err)
	}
	compararBitABit(t, sp, liv, textosDePrueba())
}

// TestIndiceTokenizerIgualAlMapa compara los IDS —no los vectores— del tokenizer del índice contra
// los del mapa, sobre los 50 textos y 3.000 cadenas armadas al azar con las piezas del vocab, que
// es donde la búsqueda por prefijo se equivoca si se equivoca: prefijos de prefijos, hojas, la
// pieza repetida. Comparar ids y no vectores importa: dos tokenizaciones distintas pueden dar
// casi el mismo vector, y ésta no deja pasar ni una.
//
// Sabotaje: no aceptar una pieza cuando es la única que empieza con la clave.
// arnes: archivo="internal/embedding/indice_tokenizer.go"
// arnes: de="if len(p.pieza(lo)) == len(clave) {"
// arnes: a="if len(p.pieza(lo)) == len(clave) && hi-lo > 1 {"
func TestIndiceTokenizerIgualAlMapa(t *testing.T) {
	dir := t.TempDir()
	escribirTablaDeJuguete(t, dir, 0, 4, 1)
	mapa, err := loadTokenizer(filepath.Join(dir, archivoTokenizer))
	if err != nil {
		t.Fatal(err)
	}
	u := mapa.(*unigram)
	idx, err := escribirIndiceTokenizer(u)
	if err != nil {
		t.Fatal(err)
	}
	indexado, err := leerIndiceTokenizer(idx)
	if err != nil {
		t.Fatal(err)
	}
	textos := textosDePrueba()
	piezas := piezasDeJuguete()
	rnd := rand.New(rand.NewSource(3))
	for i := 0; i < 3000; i++ {
		var b strings.Builder
		for j := rnd.Intn(8); j >= 0; j-- {
			p := strings.TrimPrefix(piezas[rnd.Intn(len(piezas))], "▁")
			if rnd.Intn(4) == 0 {
				b.WriteString(" ")
			}
			b.WriteString(p)
		}
		textos = append(textos, b.String())
	}
	for _, tx := range textos {
		a, b := u.EncodeIDs(tx), indexado.EncodeIDs(tx)
		if fmt.Sprint(a) != fmt.Sprint(b) {
			t.Fatalf("EncodeIDs(%q): mapa %v, índice %v", tx, a, b)
		}
	}
}

// TestIndiceTokenizerBitExacto compara el tokenizer del índice armado desde el tokenizer.json REAL
// contra la referencia clavada en testdata/spm_potion_ids.json (la misma de TestUnigramRealBitExact)
// y contra el mapa, sobre los 50 textos. Corre en recall-gate.
//
// Sabotaje: cortar la búsqueda un paso antes de que el rango quede vacío.
// arnes: archivo="internal/embedding/indice_tokenizer.go"
// arnes: env="MUSUBI_SPM_TESTDATA"
// arnes: de="if lo >= hi {"
// arnes: a="if lo >= hi-1 {"
func TestIndiceTokenizerBitExacto(t *testing.T) {
	dir := os.Getenv("MUSUBI_SPM_TESTDATA")
	if dir == "" {
		t.Skip("MUSUBI_SPM_TESTDATA no seteado: se saltea el índice contra el tokenizer real de POTION")
	}
	mapa, err := loadTokenizer(filepath.Join(dir, archivoTokenizer))
	if err != nil {
		t.Fatalf("loadTokenizer(real): %v", err)
	}
	u, ok := mapa.(*unigram)
	if !ok {
		t.Fatalf("el tokenizer real no es Unigram: %T", mapa)
	}
	idx, err := escribirIndiceTokenizer(u)
	if err != nil {
		t.Fatal(err)
	}
	indexado, err := leerIndiceTokenizer(idx)
	if err != nil {
		t.Fatal(err)
	}
	ref, err := os.ReadFile("testdata/spm_potion_ids.json")
	if err != nil {
		t.Fatalf("referencia: %v", err)
	}
	var casos []struct {
		Text string `json:"text"`
		IDs  []int  `json:"ids"`
	}
	if err := json.Unmarshal(ref, &casos); err != nil {
		t.Fatal(err)
	}
	for _, c := range casos {
		if got := indexado.EncodeIDs(c.Text); fmt.Sprint(got) != fmt.Sprint(c.IDs) {
			t.Errorf("índice EncodeIDs(%q) = %v, la referencia dice %v", c.Text, got, c.IDs)
		}
	}
	for _, tx := range textosDePrueba() {
		if a, b := u.EncodeIDs(tx), indexado.EncodeIDs(tx); fmt.Sprint(a) != fmt.Sprint(b) {
			t.Errorf("EncodeIDs(%q): mapa %v, índice %v", recortar(tx), a, b)
		}
	}
}

// TestConsultaLivianaRechazaIdentidadVencida fija el invariante N1 en el camino liviano: si la
// tabla o el tokenizer cambiaron desde que se escribió la identidad, NO se construye un proveedor.
// Construirlo daría vectores de una tabla con el nombre de otra.
//
// Sabotaje: comparar sólo el tamaño y no la fecha.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="return st.Size() == h.Tamano && st.ModTime().UnixNano() == h.MtimeNs"
// arnes: a="return st.Size() == h.Tamano"
//
// Sabotaje: no atar la identidad al índice con el que se escribió.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="if int64(len(idx)) != id.Indice.Tamano || crc32.Checksum(idx, castagnoli) != id.Indice.CRC32C {"
// arnes: a="if false && (int64(len(idx)) != id.Indice.Tamano || crc32.Checksum(idx, castagnoli) != id.Indice.CRC32C) {"
func TestConsultaLivianaRechazaIdentidadVencida(t *testing.T) {
	masTarde := time.Now().Add(time.Hour)
	casos := []struct {
		nombre  string
		cambiar func(t *testing.T, dir string)
	}{
		{"la tabla cambió de fecha", func(t *testing.T, dir string) {
			if err := os.Chtimes(filepath.Join(dir, archivoTabla), masTarde, masTarde); err != nil {
				t.Fatal(err)
			}
		}},
		{"el tokenizer cambió de fecha", func(t *testing.T, dir string) {
			if err := os.Chtimes(filepath.Join(dir, archivoTokenizer), masTarde, masTarde); err != nil {
				t.Fatal(err)
			}
		}},
		{"la tabla cambió de tamaño", func(t *testing.T, dir string) {
			f, err := os.OpenFile(filepath.Join(dir, archivoTabla), os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = f.Write([]byte{0, 0, 0, 0})
			_ = f.Close()
		}},
		{"el índice es el de otra tabla", func(t *testing.T, dir string) {
			// Un tokenizer.idx que quedó de otra tabla (un rename que falló, una copia a mano):
			// mismo formato, válido, y con OTRO vocabulario.
			otra := t.TempDir()
			escribirTablaDeJuguete(t, otra, 0, 4, 2, "pieza-que-esta-no-tiene")
			mapa, err := loadTokenizer(filepath.Join(otra, archivoTokenizer))
			if err != nil {
				t.Fatal(err)
			}
			idx, err := escribirIndiceTokenizer(mapa.(*unigram))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, archivoIndice), idx, 0o644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			dir, _ := tablaDeJugueteConSidecars(t, 200, 8)
			if _, err := NewConsultaLiviana(dir); err != nil {
				t.Fatalf("control: con los sidecars al día tenía que construirse: %v", err)
			}
			c.cambiar(t, dir)
			liv, err := NewConsultaLiviana(dir)
			if !errors.Is(err, ErrIdentidadVencida) {
				t.Fatalf("esperaba ErrIdentidadVencida, obtuve err=%v (proveedor %v)", err, liv != nil)
			}
			if liv != nil {
				t.Fatal("con la identidad vencida no tiene que haber proveedor")
			}
		})
	}

	t.Run("sin sidecars no hay atajo", func(t *testing.T) {
		dir := t.TempDir()
		escribirTablaDeJuguete(t, dir, 0, 4, 1)
		if _, err := NewConsultaLiviana(dir); !errors.Is(err, ErrSinAtajo) {
			t.Fatalf("esperaba ErrSinAtajo, obtuve %v", err)
		}
	})

	t.Run("la tabla cambia entre el constructor y el Embed", func(t *testing.T) {
		dir, _ := tablaDeJugueteConSidecars(t, 200, 8)
		liv, err := NewConsultaLiviana(dir)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(filepath.Join(dir, archivoTabla), masTarde, masTarde); err != nil {
			t.Fatal(err)
		}
		if _, err := liv.Embed(context.Background(), "deploy la tabla"); !errors.Is(err, ErrIdentidadVencida) {
			t.Fatalf("un Embed sobre una tabla reemplazada tiene que fallar con ErrIdentidadVencida, obtuve %v", err)
		}
	})
}

// contador es la costura de apertura que registra qué archivos se abrieron y cuántos bytes se
// leyeron de cada uno.
type contador struct {
	mu       sync.Mutex
	abiertos []string
	leidos   map[string]int64
}

func (c *contador) instalar(t *testing.T) {
	t.Helper()
	c.leidos = map[string]int64{}
	previo := abrirParaLeer
	abrirParaLeer = func(ruta string) (archivoDeLectura, error) {
		f, err := previo(ruta)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.abiertos = append(c.abiertos, filepath.Base(ruta))
		c.mu.Unlock()
		return &archivoContado{archivoDeLectura: f, base: filepath.Base(ruta), c: c}, nil
	}
	t.Cleanup(func() { abrirParaLeer = previo })
}

func (c *contador) sumar(base string, n int) {
	c.mu.Lock()
	c.leidos[base] += int64(n)
	c.mu.Unlock()
}

type archivoContado struct {
	archivoDeLectura
	base string
	c    *contador
}

func (a *archivoContado) Read(p []byte) (int, error) {
	n, err := a.archivoDeLectura.Read(p)
	a.c.sumar(a.base, n)
	return n, err
}

func (a *archivoContado) ReadAt(p []byte, off int64) (int, error) {
	n, err := a.archivoDeLectura.ReadAt(p, off)
	a.c.sumar(a.base, n)
	return n, err
}

var _ io.ReaderAt = (*archivoContado)(nil)

// TestConsultaLivianaNoLeeLaTabla fija para qué existe el camino liviano: construirlo y embeber un
// prompt de 200 runas lee de model.safetensors menos de 5 MB, sobre una tabla de más de 12 MB. El
// proveedor completo lee la tabla entera; si el liviano la leyera, no ahorraría nada y seguiría
// dando el vector correcto, así que ninguna prueba de igualdad lo notaría.
//
// Sabotaje: que la consulta liviana cargue la tabla entera al abrirla.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="f, err := abrirParaLeer(c.rutaTabla)"
// arnes: a="f, err := abrirParaLeer(c.rutaTabla); _, _, _, _, _, _ = cargarTablaEnStreaming(c.rutaTabla)"
func TestConsultaLivianaNoLeeLaTabla(t *testing.T) {
	const filas, dim = 50_000, 64 // 12,8 MB de blob
	dir, sp := tablaDeJugueteConSidecars(t, filas, dim)
	var c contador
	c.instalar(t)
	liv, err := NewConsultaLiviana(dir)
	if err != nil {
		t.Fatal(err)
	}
	prompt := []rune(strings.Repeat("el vector de la consulta sin cargar la tabla, ", 6))[:200]
	v, err := liv.Embed(context.Background(), string(prompt))
	if err != nil {
		t.Fatal(err)
	}
	ref, _ := sp.Embed(context.Background(), string(prompt))
	if fmt.Sprint(v) != fmt.Sprint(ref) {
		t.Fatal("control: el vector liviano no es el del proveedor completo")
	}
	const tope = 5 << 20
	if got := c.leidos[archivoTabla]; got >= tope || got == 0 {
		t.Fatalf("la consulta liviana leyó %d bytes de %s (tope %d, y cero quiere decir que la costura no midió nada)",
			got, archivoTabla, tope)
	}
	t.Logf("leídos de %s: %d bytes (la tabla tiene %d)", archivoTabla, c.leidos[archivoTabla], filas*dim*4)
}

// TestConsultaLivianaNoAbreElTokenizerJSON fija la otra mitad del ahorro: decodificar
// tokenizer.json y armar su mapa es el 76 % del costo de construir el embebedor. El camino liviano
// saca TODO del índice —el charsmap, Metaspace, el unk— y no abre tokenizer.json ni una vez.
//
// Sabotaje: leer tokenizer.json al construir la consulta liviana.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="id, idxCrudo, err := cargarSidecars(dir)"
// arnes: a="id, idxCrudo, err := cargarSidecars(dir); _, _ = leerEntero(filepath.Join(dir, archivoTokenizer))"
func TestConsultaLivianaNoAbreElTokenizerJSON(t *testing.T) {
	dir, _ := tablaDeJugueteConSidecars(t, 200, 8)
	var c contador
	c.instalar(t)
	liv, err := NewConsultaLiviana(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := liv.Embed(context.Background(), "¿vos querés el vector de la consulta?"); err != nil {
		t.Fatal(err)
	}
	if len(c.abiertos) == 0 {
		t.Fatal("control: la costura no vio ninguna apertura, así que esta prueba no mide nada")
	}
	for _, a := range c.abiertos {
		if a == archivoTokenizer {
			t.Fatalf("el camino liviano abrió %s (aperturas: %v)", archivoTokenizer, c.abiertos)
		}
	}
}

// TestUnIndiceQueNoSePudoReemplazarNoDejaUnaIdentidadNueva: en Windows un rename encima de un
// archivo abierto por otro proceso falla (Go abre sin FILE_SHARE_DELETE), y el hook abre el
// tokenizer.idx en cada turno. Si ese rename falla después de re-destilar la tabla, la identidad
// NO se reescribe —queda la vieja, que ya no coincide con la tabla— y no queda ningún temporal.
//
// Sabotaje: seguir de largo cuando el índice no se pudo escribir.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="if err := escribirAtomico(filepath.Join(dir, archivoIndice), idx); err != nil {"
// arnes: a="if err := escribirAtomico(filepath.Join(dir, archivoIndice), idx); err != nil && false {"
func TestUnIndiceQueNoSePudoReemplazarNoDejaUnaIdentidadNueva(t *testing.T) {
	dir, _ := tablaDeJugueteConSidecars(t, 200, 8)
	identidadVieja, err := os.ReadFile(filepath.Join(dir, archivoIdentidad))
	if err != nil {
		t.Fatal(err)
	}
	// Re-destilar: otra tabla y otro tokenizer en el mismo lugar, con otra fecha.
	escribirTablaDeJuguete(t, dir, 300, 8, 99, "▁nueva", "pieza", "▁tab")
	masTarde := time.Now().Add(time.Hour)
	for _, f := range []string{archivoTabla, archivoTokenizer} {
		_ = os.Chtimes(filepath.Join(dir, f), masTarde, masTarde)
	}
	previo := renombrar
	renombrar = func(desde, hasta string) error {
		if filepath.Base(hasta) == archivoIndice {
			return fmt.Errorf("simulado: el hook tiene %s abierto", archivoIndice)
		}
		return previo(desde, hasta)
	}
	t.Cleanup(func() { renombrar = previo })

	sp, err := NewStaticProvider(dir)
	if err != nil {
		t.Fatalf("un sidecar que no se puede escribir no puede romper el proveedor completo: %v", err)
	}
	if identidadAhora, _ := os.ReadFile(filepath.Join(dir, archivoIdentidad)); string(identidadAhora) != string(identidadVieja) {
		t.Fatal("se reescribió identidad.json aunque el índice nuevo no se pudo poner: la identidad nombra un índice que no está")
	}
	entradas, _ := os.ReadDir(dir)
	for _, e := range entradas {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Fatalf("quedó un temporal: %s", e.Name())
		}
	}
	if _, err := NewConsultaLiviana(dir); !errors.Is(err, ErrIdentidadVencida) {
		t.Fatalf("con el índice viejo y la tabla nueva no hay atajo: esperaba ErrIdentidadVencida, obtuve %v", err)
	}
	// Y cuando el rename vuelve a andar, el próximo arranque lo arregla.
	renombrar = previo
	if _, err := NewStaticProvider(dir); err != nil {
		t.Fatal(err)
	}
	liv, err := NewConsultaLiviana(dir)
	if err != nil {
		t.Fatalf("después de un arranque sano tenía que haber atajo: %v", err)
	}
	compararBitABit(t, sp, liv, textosDePrueba()[:10])
}

// TestUnIndiceAbiertoNoRompeNada es la versión con el sistema operativo de verdad: el índice
// queda ABIERTO (como lo tiene un hook a mitad de turno) mientras la tabla se re-destila y el
// proveedor completo intenta reescribirlo. En Windows el rename falla; en Linux y macOS no. En
// los dos casos vale lo mismo: si hay atajo, da el vector de la tabla NUEVA.
func TestUnIndiceAbiertoNoRompeNada(t *testing.T) {
	dir, _ := tablaDeJugueteConSidecars(t, 200, 8)
	abierto, err := os.Open(filepath.Join(dir, archivoIndice))
	if err != nil {
		t.Fatal(err)
	}
	escribirTablaDeJuguete(t, dir, 300, 8, 42, "▁nueva", "pieza", "▁tab")
	masTarde := time.Now().Add(time.Hour)
	for _, f := range []string{archivoTabla, archivoTokenizer} {
		_ = os.Chtimes(filepath.Join(dir, f), masTarde, masTarde)
	}
	sp, err := NewStaticProvider(dir)
	_ = abierto.Close()
	if err != nil {
		t.Fatal(err)
	}
	liv, err := NewConsultaLiviana(dir)
	if err != nil {
		if !errors.Is(err, ErrIdentidadVencida) && !errors.Is(err, ErrSinAtajo) {
			t.Fatalf("sin atajo tiene que decirlo con ErrIdentidadVencida o ErrSinAtajo, no con %v", err)
		}
		t.Logf("sin atajo mientras el índice estaba abierto (esperable en Windows): %v", err)
		return
	}
	compararBitABit(t, sp, liv, textosDePrueba())
}

// TestUnIndiceDeOtroFormatoSeReescribe fija la promesa de formatoIndice: un tokenizer.idx de otro
// formato —el que deja un binario anterior o posterior, con su crc correcto y una identidad que lo
// nombra— se trata como si no existiera, y el próximo NewStaticProvider lo reescribe. Si el «al
// día» sólo mirara que la identidad lo nombra, el atajo quedaría apagado para siempre en toda
// máquina que haya corrido el otro binario, y el hook volvería a léxico sin avisar.
//
// Sabotaje: dar por bueno un índice de otro formato en el «al día».
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="if err := cabeceraVigente(idx); err != nil {"
// arnes: a="if err := cabeceraVigente(idx); err != nil && false {"
func TestUnIndiceDeOtroFormatoSeReescribe(t *testing.T) {
	dir, _ := tablaDeJugueteConSidecars(t, 200, 8)
	rutaIdx := filepath.Join(dir, archivoIndice)
	idx, err := os.ReadFile(rutaIdx)
	if err != nil {
		t.Fatal(err)
	}
	// El índice de otro formato: la misma cabecera con el formato siguiente y el crc rehecho.
	otro := append([]byte(nil), idx...)
	binary.LittleEndian.PutUint32(otro[len(magiaIndice):], formatoIndice+1)
	binary.LittleEndian.PutUint32(otro[len(otro)-4:], crc32.Checksum(otro[:len(otro)-4], castagnoli))
	if err := os.WriteFile(rutaIdx, otro, 0o644); err != nil {
		t.Fatal(err)
	}
	var id identidadDeTabla
	crudo, err := os.ReadFile(filepath.Join(dir, archivoIdentidad))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(crudo, &id); err != nil {
		t.Fatal(err)
	}
	id.Indice.Tamano = int64(len(otro))
	id.Indice.CRC32C = crc32.Checksum(otro, castagnoli)
	crudo, _ = json.MarshalIndent(id, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, archivoIdentidad), append(crudo, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewConsultaLiviana(dir); err == nil {
		t.Fatal("control: un índice de otro formato no tenía que servir")
	}

	sp, err := NewStaticProvider(dir) // el arranque siguiente de un daemon con ESTE binario
	if err != nil {
		t.Fatal(err)
	}
	ahora, _ := os.ReadFile(rutaIdx)
	if len(ahora) < largoCabecera || binary.LittleEndian.Uint32(ahora[len(magiaIndice):]) != formatoIndice {
		t.Fatalf("el índice de otro formato sigue en disco después de un NewStaticProvider: el atajo queda apagado")
	}
	liv, err := NewConsultaLiviana(dir)
	if err != nil {
		t.Fatalf("después de reescribir tenía que haber atajo: %v", err)
	}
	compararBitABit(t, sp, liv, textosDePrueba()[:10])
}

// TestLosTemporalesHuerfanosSeBorran: un daemon que muere a mitad de escritura (se cierra la sesión
// que lo lanzó) deja su temporal; el próximo proveedor completo lo borra si es viejo, y deja el de
// un daemon que puede estar escribiendo ahora.
//
// Sabotaje: no borrar nunca un temporal.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="if info, err := e.Info(); err == nil && ahora.Sub(info.ModTime()) > edadDeHuerfano {"
// arnes: a="if info, err := e.Info(); err == nil && ahora.Sub(info.ModTime()) > edadDeHuerfano && false {"
func TestLosTemporalesHuerfanosSeBorran(t *testing.T) {
	dir, _ := tablaDeJugueteConSidecars(t, 200, 8)
	viejo := filepath.Join(dir, archivoIndice+".tmp-999-123")
	nuevo := filepath.Join(dir, archivoIdentidad+".tmp-998-456")
	ajeno := filepath.Join(dir, "notas.tmp-1")
	for _, f := range []string{viejo, nuevo, ajeno} {
		if err := os.WriteFile(f, []byte("a medias"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hace := time.Now().Add(-3 * time.Hour)
	for _, f := range []string{viejo, ajeno} {
		if err := os.Chtimes(f, hace, hace); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := NewStaticProvider(dir); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(viejo); !os.IsNotExist(err) {
		t.Fatalf("el temporal abandonado hace 3 h sigue ahí (err=%v)", err)
	}
	for _, f := range []string{nuevo, ajeno} {
		if _, err := os.Stat(f); err != nil {
			t.Fatalf("se borró %s, que no es un temporal abandonado de un sidecar: %v", filepath.Base(f), err)
		}
	}
}

// TestLosSidecarsNoSeReescribenSiEstanAlDia: el daemon construye el proveedor completo en cada
// arranque, y capture en cada fin de turno. Reescribir 15 MB de índice cada vez sería gastar disco
// para nada y, en Windows, pelearse con el hook que lo tiene abierto.
func TestLosSidecarsNoSeReescribenSiEstanAlDia(t *testing.T) {
	dir, _ := tablaDeJugueteConSidecars(t, 200, 8)
	antes, err := os.Stat(filepath.Join(dir, archivoIndice))
	if err != nil {
		t.Fatal(err)
	}
	previo := renombrar
	renombrar = func(desde, hasta string) error {
		t.Errorf("con los sidecars al día no tenía que renombrarse nada, y se renombró %s", filepath.Base(hasta))
		return previo(desde, hasta)
	}
	t.Cleanup(func() { renombrar = previo })
	if _, err := NewStaticProvider(dir); err != nil {
		t.Fatal(err)
	}
	despues, _ := os.Stat(filepath.Join(dir, archivoIndice))
	if !despues.ModTime().Equal(antes.ModTime()) {
		t.Fatal("el índice se reescribió aunque estaba al día")
	}
}
