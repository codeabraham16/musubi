package embedding

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"maps"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"musubi/internal/logx"
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
// escribe los sidecars. Devuelve el directorio y la referencia: el proveedor completo con el
// tokenizer del MAPA (conElMapa).
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
	return dir, conElMapa(t, sp, dir)
}

// conElMapa devuelve una copia de sp que tokeniza con el MAPA, armado de nuevo desde el
// tokenizer.json de dir. Es la referencia de toda comparación bit a bit: desde que el completo
// tokeniza con el índice, comparar la consulta liviana contra él sería comparar el índice consigo
// mismo, y un defecto del índice que rompiera los dos lados por igual daría verde.
func conElMapa(t *testing.T, sp *StaticProvider, dir string) *StaticProvider {
	t.Helper()
	tok, err := loadTokenizer(filepath.Join(dir, archivoTokenizer))
	if err != nil {
		t.Fatal(err)
	}
	if u, ok := tok.(*unigram); !ok || u.vocab == nil || u.piezas != nil {
		t.Fatalf("control: la referencia tiene que tokenizar con el mapa (%T)", tok)
	}
	ref := *sp
	ref.tok = tok
	return &ref
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
// ref es la referencia y prob el que se prueba.
func compararBitABit(t *testing.T, ref, prob Provider, textos []string) {
	t.Helper()
	ctx := context.Background()
	nr, np := nombreDe(ref), nombreDe(prob)
	if ref.Name() != prob.Name() || ref.Dimensions() != prob.Dimensions() {
		t.Fatalf("identidad distinta: %s %s/%d, %s %s/%d", nr, ref.Name(), ref.Dimensions(), np, prob.Name(), prob.Dimensions())
	}
	for i, tx := range textos {
		a, errA := ref.Embed(ctx, tx)
		b, errB := prob.Embed(ctx, tx)
		if errA != nil || errB != nil {
			t.Fatalf("texto %d: errores %s=%v, %s=%v", i, nr, errA, np, errB)
		}
		if len(a) != len(b) {
			t.Fatalf("texto %d (%q…): largo %d (%s) contra %d (%s)", i, recortar(tx), len(a), nr, len(b), np)
		}
		for j := range a {
			if math.Float32bits(a[j]) != math.Float32bits(b[j]) {
				t.Fatalf("texto %d (%q…): el vector difiere en la componente %d: %s da %v, %s da %v",
					i, recortar(tx), j, nr, a[j], np, b[j])
			}
		}
	}
}

// nombreDe nombra a un proveedor por lo que ES y no por la posición en que se lo pasó: con las
// etiquetas fijas «completo» y «liviano», una diferencia del completo con el índice se leía como
// una de la consulta liviana.
func nombreDe(p Provider) string {
	switch v := p.(type) {
	case *ConsultaLiviana:
		return "la consulta liviana"
	case *StaticProvider:
		return "el completo con " + comoTokeniza(v)
	}
	return fmt.Sprintf("%T", p)
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
// arnes: de="c.inicio+int64(distintas[k])*filaBytes"
// arnes: a="c.inicio+int64(distintas[k]+1)*filaBytes"
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
// lo tiene. Corre en el job recall-gate, que baja la tabla. Los dos que tokenizan con el índice —la
// consulta liviana y el completo— se comparan contra el completo con el MAPA.
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
	ref := conElMapa(t, sp, dir)
	compararBitABit(t, ref, liv, textosDePrueba())
	compararBitABit(t, ref, sp, textosDePrueba())
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
// y contra el mapa, sobre los 50 textos y sobre un texto real largo. Corre en recall-gate.
//
// El texto largo existe porque, con un índice que usar, el completo tokeniza con él TODO lo que
// embebe, notas de miles de caracteres incluidas, y los 50 textos son casi todos frases. Son los .go de este
// paquete: comentarios en español con tildes, comillas, rayas y código, recortados a 20.000 runas.
// Cambian con el código, y no importa: se comparan los dos caminos entre sí, no contra una
// referencia fija.
//
// Y después cada pieza del vocab, escrita como texto, por los dos caminos. Los textos de arriba
// casi no tienen piezas largas: con un índice que no miraba más allá de 16 runas, este barrido
// cuenta 1.196 piezas que se tokenizan distinto, y las tres comparaciones de arriba seguían
// verdes. El barrido sólo ve un índice que corta antes de maxRunes si alguna pieza de maxRunes
// runas se tokeniza como sí misma, así que eso también se exige: en POTION, la más larga lo hace.
//
// Las tres primeras directivas caen antes del barrido, en la referencia o en la cabecera, y siguen
// en rojo aunque el barrido no compare nada. La que lo vigila es la cuarta: con el barrido ahuecado
// queda verde.
//
// Sabotaje: cortar la búsqueda un paso antes de que el rango quede vacío.
// arnes: archivo="internal/embedding/indice_tokenizer.go"
// arnes: env="MUSUBI_SPM_TESTDATA"
// arnes: de="if lo >= hi {"
// arnes: a="if lo >= hi-1 {"
//
// Sabotaje: que el índice no vea la pieza más larga.
// arnes: archivo="internal/embedding/indice_tokenizer.go"
// arnes: env="MUSUBI_SPM_TESTDATA"
// arnes: de="u.maxRunes = int(l.u32())"
// arnes: a="u.maxRunes = int(l.u32()) - 1"
//
// Sabotaje: que el índice no traiga el score del unk.
// arnes: archivo="internal/embedding/indice_tokenizer.go"
// arnes: env="MUSUBI_SPM_TESTDATA"
// arnes: de="u.unkScore = math.Float64frombits(l.u64())"
// arnes: a="u.unkScore = math.Float64frombits(l.u64()) + 1"
//
// Sabotaje: que el índice no busque piezas de más de 185 runas, con la cabecera intacta.
// arnes: archivo="internal/embedding/indice_tokenizer.go"
// arnes: env="MUSUBI_SPM_TESTDATA"
// arnes: de="for l := 1; l <= len(runes); l++ {"
// arnes: a="for l := 1; l <= len(runes) && l <= 185; l++ {"
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
	// La cabecera, campo por campo. Un unkScore distinto cambia los ids sólo donde el texto tiene
	// runas sin pieza, y esto lo ve aunque ningún texto de abajo las tenga.
	if c, m := indexado, u; c.maxRunes != m.maxRunes || c.unkID != m.unkID ||
		c.unkScore != m.unkScore || c.repl != m.repl {
		t.Errorf("la cabecera del índice no es la del mapa (índice/mapa): "+
			"maxRunes %d/%d, unkID %d/%d, unkScore %v/%v, repl %q/%q",
			c.maxRunes, m.maxRunes, c.unkID, m.unkID, c.unkScore, m.unkScore, c.repl, m.repl)
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
	largo := textoRealLargo(t, 20_000)
	a, b := u.EncodeIDs(largo), indexado.EncodeIDs(largo)
	if !slices.Equal(a, b) {
		k := 0
		for k < len(a) && k < len(b) && a[k] == b[k] {
			k++
		}
		t.Errorf("texto real largo: los ids difieren desde la posición %d (mapa %d ids, índice %d)", k, len(a), len(b))
	}
	piezas := slices.Sorted(maps.Keys(u.vocab))
	distintas, propiaMasLarga := 0, 0
	for _, p := range piezas {
		tx := strings.ReplaceAll(p, u.repl, " ")
		a, b := u.EncodeIDs(tx), indexado.EncodeIDs(tx)
		if !slices.Equal(a, b) {
			if distintas < 5 {
				t.Errorf("pieza %q: mapa %v, índice %v", recortar(p), a, b)
			}
			distintas++
		}
		if len(a) == 1 && a[0] == u.vocab[p] {
			propiaMasLarga = max(propiaMasLarga, utf8.RuneCountInString(p))
		}
	}
	t.Logf("barrido: %d piezas del vocab, %d distintas, la más larga que se tokeniza como sí misma mide %d runas",
		len(piezas), distintas, propiaMasLarga)
	if distintas > 0 {
		t.Errorf("%d de %d piezas se tokenizan distinto por el índice", distintas, len(piezas))
	}
	if propiaMasLarga != u.maxRunes {
		t.Errorf("la pieza más larga que se tokeniza como sí misma mide %d runas y maxRunes es %d: "+
			"el barrido no vería un índice que corte antes de maxRunes", propiaMasLarga, u.maxRunes)
	}
}

// textoRealLargo concatena los .go de este paquete, en orden de nombre, y devuelve las primeras
// `runas` runas. Falla si no llega a ser largo o si es todo ASCII: un texto así no ejercitaría ni
// el largo ni las runas de varios bytes, que es para lo que existe.
func textoRealLargo(t *testing.T, runas int) string {
	t.Helper()
	archivos, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var r []rune
	for _, f := range archivos {
		c, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		r = append(r, []rune(string(c))...)
		if len(r) >= runas {
			break
		}
	}
	if len(r) > runas {
		r = r[:runas]
	}
	noASCII := 0
	for _, x := range r {
		if x > 0x7f {
			noASCII++
		}
	}
	if len(r) < runas*3/4 || noASCII < 100 {
		t.Fatalf("control: el texto real largo tiene %d runas y %d fuera de ASCII; no ejercita lo que tiene que ejercitar", len(r), noASCII)
	}
	return string(r)
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
// Y el aviso nombra la causa: el rename. Seguir de largo tampoco escribiría la identidad, porque
// la rama del índice que no se puede leer también corta; pero el aviso diría que hay un índice
// «recién escrito» ilegible, y ese índice no se escribió nunca. El aviso es lo único que separa
// las dos ramas, y por eso la prueba lo mira.
//
// Sabotaje: seguir de largo cuando el índice no se pudo escribir.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="if err != nil { // sin índice nuevo no hay identidad nueva"
// arnes: a="if err != nil && false { // sin índice nuevo no hay identidad nueva"
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
	rechazo := fmt.Errorf("simulado: el hook tiene %s abierto", archivoIndice)
	renombrar = func(desde, hasta string) error {
		if filepath.Base(hasta) == archivoIndice {
			return rechazo
		}
		return previo(desde, hasta)
	}
	t.Cleanup(func() { renombrar = previo })

	var log strings.Builder
	restaurar := logx.Capturar(&log)
	defer restaurar()
	sp, err := NewStaticProvider(dir)
	if err != nil {
		t.Fatalf("un sidecar que no se puede escribir no puede romper el proveedor completo: %v", err)
	}
	if !strings.Contains(log.String(), rechazo.Error()) {
		t.Fatalf("el aviso tiene que nombrar la causa (%v); avisó:\n%s", rechazo, log.String())
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
	compararBitABit(t, conElMapa(t, sp, dir), liv, textosDePrueba()[:10])
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
	compararBitABit(t, conElMapa(t, sp, dir), liv, textosDePrueba())
}

// TestUnIndiceDeOtroFormatoSeReescribe fija la promesa de formatoIndice: un tokenizer.idx de otro
// formato —el que deja un binario anterior o posterior, con su crc correcto y una identidad que lo
// nombra— no se interpreta: se trata como si no existiera, y el próximo NewStaticProvider lo
// reescribe. Interpretarlo daría los ids de una derivación que este binario no conoce, y desde que
// el completo tokeniza con el índice, ésos serían los vectores de TODO lo que se indexa.
//
// Sabotaje: interpretar un índice de otro formato.
// arnes: archivo="internal/embedding/indice_tokenizer.go"
// arnes: de="if f := binary.LittleEndian.Uint32(idx[len(magiaIndice):]); f != formatoIndice {"
// arnes: a="if f := binary.LittleEndian.Uint32(idx[len(magiaIndice):]); f != formatoIndice && false {"
func TestUnIndiceDeOtroFormatoSeReescribe(t *testing.T) {
	dir, ref := tablaDeJugueteConSidecars(t, 200, 8)
	rutaIdx := filepath.Join(dir, archivoIndice)
	idx, err := os.ReadFile(rutaIdx)
	if err != nil {
		t.Fatal(err)
	}
	// El índice de otro formato: la misma cabecera con el formato siguiente y el crc rehecho.
	otro := append([]byte(nil), idx...)
	binary.LittleEndian.PutUint32(otro[len(magiaIndice):], formatoIndice+1)
	binary.LittleEndian.PutUint32(otro[len(otro)-4:], crc32.Checksum(otro[:len(otro)-4], castagnoli))
	reemplazarIndice(t, dir, otro)
	if _, err := NewConsultaLiviana(dir); !errors.Is(err, ErrSinAtajo) {
		t.Fatalf("un índice de otro formato no tiene que servir: esperaba ErrSinAtajo, obtuve %v", err)
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
	compararBitABit(t, ref, liv, textosDePrueba()[:10])
	compararBitABit(t, ref, sp, textosDePrueba()[:10])
}

// reemplazarIndice pone `idx` como tokenizer.idx y rehace la identidad para que lo nombre (tamaño y
// crc32c), como lo dejaría un binario que lo escribió así: la identidad sigue vigente, y lo único
// que puede rechazar ese índice es leerlo.
func reemplazarIndice(t *testing.T, dir string, idx []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, archivoIndice), idx, 0o644); err != nil {
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
	id.Indice.Tamano = int64(len(idx))
	id.Indice.CRC32C = crc32.Checksum(idx, castagnoli)
	crudo, _ = json.MarshalIndent(id, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, archivoIdentidad), append(crudo, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// sinEscritura hace que crear un temporal falle en cualquier carpeta, como en una carpeta de
// tabla sin permiso de escritura, y cuenta cuántas veces se arma el índice.
func sinEscritura(t *testing.T) *int {
	t.Helper()
	armados := 0
	previoCrear, previoArmar := crearTemporal, armarIndice
	crearTemporal = func(string, string) (*os.File, error) {
		return nil, fmt.Errorf("simulado: la carpeta de la tabla no admite escritura")
	}
	armarIndice = func(u *unigram) ([]byte, error) {
		armados++
		return previoArmar(u)
	}
	t.Cleanup(func() { crearTemporal, armarIndice = previoCrear, previoArmar })
	return &armados
}

// TestSinEscrituraNoSeArmaElIndice: en una carpeta de tabla sin escritura (el usuario de un
// servicio sin permiso, una tabla en una ruta del sistema, un disco lleno), el proveedor completo
// NO arma el índice. Armarlo cuesta 0,7-1,1 s sobre POTION, y antes se armaba entero en cada
// construcción —capture lo construye en cada fin de turno— para después no poder escribirlo.
//
// Sabotaje: armar el índice antes de saber si se puede escribir.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="idx, err := escribirAtomico(filepath.Join(dir, archivoIndice), func() ([]byte, error) { return armarIndice(u) })"
// arnes: a="_, _ = armarIndice(u); idx, err := escribirAtomico(filepath.Join(dir, archivoIndice), func() ([]byte, error) { return armarIndice(u) })"
func TestSinEscrituraNoSeArmaElIndice(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tabla-juguete")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	escribirTablaDeJuguete(t, dir, 200, 8, 7)
	armados := sinEscritura(t)
	for i := 0; i < 2; i++ {
		if _, err := NewStaticProvider(dir); err != nil {
			t.Fatalf("una carpeta sin escritura no puede romper el proveedor completo: %v", err)
		}
	}
	if *armados != 0 {
		t.Fatalf("se armó el índice %d vez/veces en una carpeta donde no se podía escribir", *armados)
	}
	for _, f := range []string{archivoIndice, archivoIdentidad} {
		if _, err := os.Stat(filepath.Join(dir, f)); !os.IsNotExist(err) {
			t.Fatalf("%s no tenía que existir: %v", f, err)
		}
	}
	// Control: la costura cuenta de verdad cuando SÍ se puede escribir.
	crearTemporal = os.CreateTemp
	if _, err := NewStaticProvider(dir); err != nil {
		t.Fatal(err)
	}
	if *armados != 1 {
		t.Fatalf("control: con escritura el índice se tenía que armar una vez, y se armó %d", *armados)
	}
}

// TestElAvisoDeSinEscrituraSaleUnaVez: sin escritura, el aviso sale una vez por proceso y por
// carpeta, no una por construcción.
//
// Sabotaje: avisar en cada construcción.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="if _, ya := avisosSinAtajo.LoadOrStore(dir, true); ya {"
// arnes: a="if _, ya := avisosSinAtajo.LoadOrStore(dir, true); ya && false {"
func TestElAvisoDeSinEscrituraSaleUnaVez(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tabla-juguete")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	escribirTablaDeJuguete(t, dir, 200, 8, 7)
	sinEscritura(t)
	var log strings.Builder
	restaurar := logx.Capturar(&log)
	defer restaurar()
	for i := 0; i < 3; i++ {
		if _, err := NewStaticProvider(dir); err != nil {
			t.Fatal(err)
		}
	}
	if n := strings.Count(log.String(), "queda sin atajo"); n != 1 {
		t.Fatalf("el aviso salió %d veces en 3 construcciones (tiene que salir una):\n%s", n, log.String())
	}
}

// lecturasLentas hace que cada lectura de la tabla tarde `demora`, como un disco frío y ocupado.
// Se instala DESPUÉS de construir la consulta liviana, así afecta sólo a las filas.
func lecturasLentas(t *testing.T, demora time.Duration) {
	t.Helper()
	previo := abrirParaLeer
	abrirParaLeer = func(ruta string) (archivoDeLectura, error) {
		f, err := previo(ruta)
		if err != nil || filepath.Base(ruta) != archivoTabla {
			return f, err
		}
		return &archivoLento{archivoDeLectura: f, demora: demora}, nil
	}
	t.Cleanup(func() { abrirParaLeer = previo })
}

type archivoLento struct {
	archivoDeLectura
	demora time.Duration
}

func (a *archivoLento) ReadAt(p []byte, off int64) (int, error) {
	time.Sleep(a.demora)
	return a.archivoDeLectura.ReadAt(p, off)
}

// textoConMuchasFilas usa casi todas las piezas del vocab de juguete: son ~150 filas distintas.
func textoConMuchasFilas() string {
	return strings.Join(piezasDeJuguete()[3:], " ")
}

// TestConsultaLivianaRespetaElPlazo: el hook le pone un plazo al Embed (turnEmbedTimeout, 2 s) y
// confía en que el proveedor lo respete. Con la tabla fría, un prompt largo son cientos de
// lecturas; si el Embed no mirara el contexto, el plazo no cortaría nada y el hook se comería su
// techo de 10 s. También con el contexto ya cancelado, aunque el texto no tenga ninguna fila que
// leer: quien pidió el vector ya no lo espera.
//
// Sabotaje: no mirar el contexto entre una lectura y la siguiente.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="if err := ctx.Err(); err != nil { // el plazo corta entre una lectura y la siguiente"
// arnes: a="if err := ctx.Err(); err != nil && false { // el plazo corta entre una lectura y la siguiente"
//
// Sabotaje: no mirar el contexto al entrar.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="if err := ctx.Err(); err != nil { // quien pidió el vector ya no lo espera"
// arnes: a="if err := ctx.Err(); err != nil && false { // quien pidió el vector ya no lo espera"
func TestConsultaLivianaRespetaElPlazo(t *testing.T) {
	dir, _ := tablaDeJugueteConSidecars(t, 400, 16)
	liv, err := NewConsultaLiviana(dir)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("con el contexto ya cancelado", func(t *testing.T) {
		ctx, cancelar := context.WithCancel(context.Background())
		cancelar()
		for _, tx := range []string{"", "deploy la tabla del hook"} {
			if v, err := liv.Embed(ctx, tx); !errors.Is(err, context.Canceled) {
				t.Fatalf("Embed(%q) con el contexto cancelado devolvió err=%v y un vector de %d", tx, err, len(v))
			}
		}
	})

	t.Run("el plazo vence a mitad de las lecturas", func(t *testing.T) {
		lecturasLentas(t, 25*time.Millisecond) // ~150 filas: varios segundos sin el plazo
		ctx, cancelar := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancelar()
		t0 := time.Now()
		_, err := liv.Embed(ctx, textoConMuchasFilas())
		demora := time.Since(t0)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("con el plazo vencido el Embed devolvió err=%v después de %v", err, demora)
		}
		if demora > time.Second {
			t.Fatalf("el Embed cortó recién a los %v con un plazo de 100 ms", demora)
		}
	})
}

// TestConsultaLivianaLeeLasFilasEnParalelo: un prompt largo son cientos de filas distintas, y con
// la tabla fría cada una espera al disco. Medido en davantis-1 sobre POTION, leídas de a una tardaban
// 1,3-1,7 s con 7 KB de prompt y 2,7-2,9 s con 20 KB, contra una meta de p95 de 450 ms para el hook
// entero. Acá el disco lento lo simula una costura: ~150 filas a 25 ms cada una son ~3,8 s de a una,
// y en paralelo tienen que bajar de 1,5 s. Además: cada fila distinta se lee UNA sola vez, y el
// vector sigue siendo el de StaticProvider.
//
// Sabotaje: leer las filas de a una.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="lectoresDeFilas = 8"
// arnes: a="lectoresDeFilas = 1"
func TestConsultaLivianaLeeLasFilasEnParalelo(t *testing.T) {
	dir, sp := tablaDeJugueteConSidecars(t, 400, 16)
	liv, err := NewConsultaLiviana(dir)
	if err != nil {
		t.Fatal(err)
	}
	var c contador
	c.instalar(t)
	lecturasLentas(t, 25*time.Millisecond)
	texto := textoConMuchasFilas()
	t0 := time.Now()
	v, err := liv.Embed(context.Background(), texto)
	demora := time.Since(t0)
	if err != nil {
		t.Fatal(err)
	}
	distintas := map[int]bool{}
	for _, id := range liv.tok.EncodeIDs(texto) {
		distintas[id] = true
	}
	if len(distintas) < 100 {
		t.Fatalf("control: el texto usa sólo %d filas distintas, y la prueba necesita muchas", len(distintas))
	}
	if got, quiero := c.leidos[archivoTabla], int64(len(distintas))*16*4; got != quiero {
		t.Fatalf("se leyeron %d bytes de la tabla para %d filas distintas (tenían que ser %d: cada fila una vez)",
			got, len(distintas), quiero)
	}
	if demora > 1500*time.Millisecond {
		t.Fatalf("%d filas distintas a 25 ms cada una tardaron %v: se están leyendo de a una", len(distintas), demora)
	}
	ref, _ := sp.Embed(context.Background(), texto)
	for j := range ref {
		if math.Float32bits(ref[j]) != math.Float32bits(v[j]) {
			t.Fatalf("leídas en paralelo, el vector difiere del de StaticProvider en la componente %d", j)
		}
	}
	t.Logf("%d filas distintas con 25 ms por lectura: %v", len(distintas), demora)
}

// TestElCompletoSanaUnaTablaConMismoTamanoYFecha fija lo único que cubre el hueco que acepta la
// decisión 5 (la tabla se da por no cambiada mirando tamaño y fecha): si alguien reescribe la tabla
// con el MISMO tamaño y la MISMA fecha, la consulta liviana no lo ve, pero el primer proveedor
// completo que arranque sí, porque su «al día» compara el checksum de CONTENIDO que acaba de
// calcular, y reescribe la identidad. Sin eso, el hueco sería para siempre.
//
// Sabotaje: el «al día» sin comparar el checksum.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="err == nil && id.Checksum == checksum && id.Tabla == tabla && id.Tokenizer == tok {"
// arnes: a="err == nil && id.Tabla == tabla && id.Tokenizer == tok {"
func TestElCompletoSanaUnaTablaConMismoTamanoYFecha(t *testing.T) {
	dir, sp := tablaDeJugueteConSidecars(t, 200, 8)
	tab := filepath.Join(dir, archivoTabla)
	st, err := os.Stat(tab)
	if err != nil {
		t.Fatal(err)
	}
	crudo, err := os.ReadFile(tab)
	if err != nil {
		t.Fatal(err)
	}
	crudo[len(crudo)-2] ^= 0x40 // otro valor en la última fila: mismo tamaño
	if err := os.WriteFile(tab, crudo, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(tab, st.ModTime(), st.ModTime()); err != nil { // y la misma fecha
		t.Fatal(err)
	}
	liv, err := NewConsultaLiviana(dir)
	if err != nil || liv.Name() != sp.Name() {
		t.Fatalf("control: con el mismo tamaño y la misma fecha la identidad se da por buena (decisión 5): %v", err)
	}
	sp2, err := NewStaticProvider(dir)
	if err != nil {
		t.Fatal(err)
	}
	if sp2.Name() == sp.Name() {
		t.Fatal("control: el contenido cambió y el checksum del proveedor completo no")
	}
	liv2, err := NewConsultaLiviana(dir)
	if err != nil {
		t.Fatalf("después del arranque del proveedor completo tenía que haber atajo: %v", err)
	}
	if liv2.Name() != sp2.Name() {
		t.Fatalf("la identidad NO se sanó: la consulta liviana dice %s y la tabla es %s", liv2.Name(), sp2.Name())
	}
}

// swapAlCerrar reemplaza la tabla en disco apenas cargarTablaEnStreaming cierra el archivo: la
// tabla cambia ENTRE la carga y la escritura de los sidecars.
type swapAlCerrar struct {
	archivoDeLectura
	hacer func()
}

func (s *swapAlCerrar) Close() error {
	err := s.archivoDeLectura.Close()
	if s.hacer != nil {
		s.hacer()
		s.hacer = nil
	}
	return err
}

// TestLaTablaQueCambiaDuranteLaCargaNoSeFirmaConLaHuellaNueva fija por qué NewStaticProvider toma
// las huellas ANTES de leer la tabla. Si la tabla se reemplaza mientras se carga, el checksum y el
// índice son de la VIEJA; tomar la huella después le pegaría la fecha y el tamaño de la NUEVA, y la
// consulta liviana aceptaría esa identidad sobre la tabla nueva: vectores de una tabla con el nombre
// de otra, que es la corrupción de N1.
//
// Sabotaje: volver a tomar la huella de la tabla después de cargarla.
// arnes: archivo="internal/embedding/static.go"
// arnes: de="if u, ok := tok.(*unigram); ok && errTabla == nil && errTok == nil {"
// arnes: a="if u, ok := tok.(*unigram); ok && errTabla == nil && errTok == nil { huellaTabla, errTabla = huellaDe(filepath.Join(dir, archivoTabla))"
func TestLaTablaQueCambiaDuranteLaCargaNoSeFirmaConLaHuellaNueva(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tabla-juguete")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	escribirTablaDeJuguete(t, dir, 200, 8, 7)
	tab := filepath.Join(dir, archivoTabla)
	nueva, err := os.ReadFile(tab)
	if err != nil {
		t.Fatal(err)
	}
	for i := len(nueva) - 8*4*50; i < len(nueva); i++ { // otras 50 filas, mismo tamaño
		nueva[i] ^= 0x5a
	}
	previo := abrirParaLeer
	usado := false
	abrirParaLeer = func(ruta string) (archivoDeLectura, error) {
		f, err := previo(ruta)
		if err != nil || usado || filepath.Base(ruta) != archivoTabla {
			return f, err
		}
		usado = true
		return &swapAlCerrar{archivoDeLectura: f, hacer: func() {
			if err := os.WriteFile(tab, nueva, 0o644); err != nil {
				t.Errorf("no se pudo reemplazar la tabla: %v", err)
			}
			masTarde := time.Now().Add(time.Hour)
			_ = os.Chtimes(tab, masTarde, masTarde)
		}}, nil
	}
	t.Cleanup(func() { abrirParaLeer = previo })

	if _, err := NewStaticProvider(dir); err != nil { // carga la VIEJA; al cerrar, queda la nueva
		t.Fatal(err)
	}
	abrirParaLeer = previo
	if !usado {
		t.Fatal("control: la costura no reemplazó la tabla, así que esta prueba no mide nada")
	}
	if liv, err := NewConsultaLiviana(dir); err == nil {
		fresco, ferr := NewStaticProvider(dir)
		if ferr != nil {
			t.Fatal(ferr)
		}
		t.Fatalf("CORRUPCIÓN N1: la consulta liviana se construyó con la procedencia %s sobre una tabla que es %s",
			liv.Name(), fresco.Name())
	} else if !errors.Is(err, ErrSinAtajo) && !errors.Is(err, ErrIdentidadVencida) {
		t.Fatalf("sin atajo tiene que decirlo con ErrSinAtajo o ErrIdentidadVencida, no con %v", err)
	}
	// Y el arranque siguiente, ya sin cambios a mitad de carga, deja la identidad correcta.
	fresco, err := NewStaticProvider(dir)
	if err != nil {
		t.Fatal(err)
	}
	liv, err := NewConsultaLiviana(dir)
	if err != nil || liv.Name() != fresco.Name() {
		t.Fatalf("después de un arranque sano la identidad tenía que ser la de la tabla nueva: %v", err)
	}
}

// TestLosSidecarsQuedanLegiblesParaTodos: tokenizer.idx e identidad.json quedan en 0644, como la
// tabla. Con el 0600 de os.CreateTemp, en Linux otro usuario que use la misma tabla no los podría
// leer y los reescribiría como suyos. En Windows el modo sólo decide el atributo de sólo lectura, así
// que ahí la prueba mira que el modo se pida; donde el modo es real, mira además el archivo.
//
// Sabotaje: dejar el modo con que los crea os.CreateTemp.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="_ = cambiarModo(tmp, modoDeSidecar)"
// arnes: a="_ = modoDeSidecar"
func TestLosSidecarsQuedanLegiblesParaTodos(t *testing.T) {
	t.Run("un sistema de archivos sin modos no cuesta el sidecar", func(t *testing.T) {
		previo := cambiarModo
		cambiarModo = func(*os.File, os.FileMode) error { return fmt.Errorf("simulado: EPERM") }
		t.Cleanup(func() { cambiarModo = previo })
		dir, _ := tablaDeJugueteConSidecars(t, 200, 8)
		if _, err := NewConsultaLiviana(dir); err != nil {
			t.Fatalf("si el chmod falla, el sidecar tiene que quedar igual: %v", err)
		}
	})

	pedidos := map[string]os.FileMode{}
	var mu sync.Mutex
	previo := cambiarModo
	cambiarModo = func(f *os.File, m os.FileMode) error {
		base := filepath.Base(f.Name())
		mu.Lock()
		pedidos[base[:strings.Index(base, ".tmp-")]] = m
		mu.Unlock()
		return previo(f, m)
	}
	t.Cleanup(func() { cambiarModo = previo })

	dir, _ := tablaDeJugueteConSidecars(t, 200, 8)
	for _, f := range []string{archivoIndice, archivoIdentidad} {
		if m, ok := pedidos[f]; !ok || m != 0o644 {
			t.Fatalf("%s: el modo pedido fue %v (pedido=%v); tiene que ser 0644", f, m, ok)
		}
		if runtime.GOOS == "windows" {
			continue
		}
		st, err := os.Stat(filepath.Join(dir, f))
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o644 {
			t.Fatalf("%s quedó en %v; tiene que quedar en 0644 como la tabla", f, st.Mode().Perm())
		}
	}
}

// TestLosTemporalesHuerfanosSeBorran: un daemon que muere a mitad de escritura (se cierra la sesión
// que lo lanzó) deja su temporal; el próximo proveedor completo lo borra si es viejo, y deja el de
// un daemon que puede estar escribiendo ahora. El proveedor se construye con los sidecars AL DÍA,
// que es el caso de casi todos los arranques: ahí no se escribe nada, y la limpieza igual corre.
//
// Sabotaje: no borrar nunca un temporal.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="if info, err := e.Info(); err == nil && ahora.Sub(info.ModTime()) > edadDeHuerfano {"
// arnes: a="if info, err := e.Info(); err == nil && ahora.Sub(info.ModTime()) > edadDeHuerfano && false {"
//
// Sabotaje: no limpiar al construir el proveedor.
// arnes: archivo="internal/embedding/static.go"
// arnes: de="limpiarTemporalesHuerfanos(dir, time.Now())"
// arnes: a="_ = time.Now()"
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
		// Dos causas con el mismo síntoma, y se separan llamando a la limpieza a mano: si ella lo
		// borra, el que falló es el proveedor, que no la llamó.
		limpiarTemporalesHuerfanos(dir, time.Now())
		if _, err := os.Stat(viejo); os.IsNotExist(err) {
			t.Fatal("NewStaticProvider no limpió: la limpieza borra el temporal abandonado hace 3 h, pero con los sidecars al día el proveedor no la llamó")
		}
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
