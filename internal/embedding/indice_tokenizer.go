package embedding

// indice_tokenizer.go escribe y lee tokenizer.idx: el tokenizer Unigram YA ARMADO, en un formato
// binario que se usa tal cual se lee, sin construir nada.
//
// POR QUÉ EXISTE. Construir el embebedor estático cuesta ~1,25 s en esta PC, y el 76 % es el
// tokenizer: deserializar las 500.353 piezas de tokenizer.json y armar con ellas un mapa (medido:
// loadTokenizerBytes 0,94-0,98 s; el ReadFile, 6 ms). Un proceso efímero —el hook por turno, uno
// nuevo por prompt— no puede pagar eso. El índice reemplaza el mapa por las piezas ORDENADAS, y la
// búsqueda de «qué piezas empiezan acá» pasa a ser una búsqueda binaria que se angosta runa a runa.
// No hay nada que construir: se lee el archivo y se busca sobre sus bytes.
//
// LO QUE LLEVA ADEMÁS DEL VOCABULARIO, y es lo que hace que el camino liviano no abra NUNCA
// tokenizer.json: el normalizer entero (el charsmap precompilado incluido, 237.539 bytes en POTION),
// el símbolo de Metaspace, el unk_id, el score del unk y el largo máximo de pieza. Si alguna de esas
// cosas se sacara de tokenizer.json, se volvería a pagar la decodificación del JSON.
//
// SÓLO UNIGRAM. Con WordPiece no hay atajo: sus tablas son las inglesas, que no son las que usa este
// repo, y su vocabulario se recorre de otra forma.
//
// Formato (todo little-endian; lo escribe escribirIndiceTokenizer, lo lee leerIndiceTokenizer):
//
//	magia "MSBTKIDX" · formato u32 · tipo u32 (1 = Unigram)
//	unkID u32 · maxRunes u32 · unkScore f64
//	repl: largo u32 + bytes · normalizer (JSON tal cual): largo u32 + bytes
//	n u32 · offsets (n+1)×u32 · ids n×u32 · scores n×f64 · blob de las piezas ordenadas
//	crc32c u32 de todo lo anterior

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"math"
	"sort"
	"unicode/utf8"
)

const (
	magiaIndice = "MSBTKIDX"
	// formatoIndice se sube cuando cambia CUALQUIER cosa del formato. Un índice de otro formato no
	// se interpreta: se lo trata como si no existiera, y NewStaticProvider lo reescribe. Lo que
	// hace cierta esa promesa es que cargarSidecars exige cabeceraVigente: sin eso, el completo
	// daría por «al día» un índice que la consulta liviana rechaza, y el atajo quedaría apagado.
	formatoIndice   = 1
	tipoIndiceUnigr = 1
)

// errIndiceInvalido envuelve todo lo que hace que un tokenizer.idx no se pueda usar: truncado,
// corrupto, de otro formato. El caller lo trata igual que «no hay índice».
var errIndiceInvalido = errors.New("tokenizer.idx inválido")

// escribirIndiceTokenizer serializa un Unigram YA CONSTRUIDO desde tokenizer.json. Se arma desde
// el tokenizer en memoria y no releyendo el JSON, así el índice lleva exactamente lo que el
// StaticProvider usa: si el mapa tuviera dos entradas con la misma pieza, gana la que ganó en el
// mapa (la última), porque es la que está en él.
func escribirIndiceTokenizer(u *unigram) ([]byte, error) {
	if u.vocab == nil || u.piezas != nil {
		return nil, fmt.Errorf("el índice se escribe desde un tokenizer cargado de tokenizer.json")
	}
	piezas := make([]string, 0, len(u.vocab))
	for p := range u.vocab {
		piezas = append(piezas, p)
	}
	sort.Strings(piezas) // orden de BYTES, que es el que usa la búsqueda
	var blobLen int
	for _, p := range piezas {
		blobLen += len(p)
	}
	if uint64(len(piezas)) >= math.MaxUint32 || uint64(blobLen) >= math.MaxUint32 {
		return nil, fmt.Errorf("vocabulario demasiado grande para el índice: %d piezas, %d bytes", len(piezas), blobLen)
	}

	var b bytes.Buffer
	b.Grow(64 + len(u.repl) + len(u.normCrudo) + (len(piezas)+1)*4 + len(piezas)*12 + blobLen + 4)
	u32 := func(v uint32) { _ = binary.Write(&b, binary.LittleEndian, v) }
	b.WriteString(magiaIndice)
	u32(formatoIndice)
	u32(tipoIndiceUnigr)
	u32(uint32(u.unkID))
	u32(uint32(u.maxRunes))
	_ = binary.Write(&b, binary.LittleEndian, math.Float64bits(u.unkScore))
	u32(uint32(len(u.repl)))
	b.WriteString(u.repl)
	u32(uint32(len(u.normCrudo)))
	b.Write(u.normCrudo)
	u32(uint32(len(piezas)))
	off := uint32(0)
	u32(off)
	for _, p := range piezas {
		off += uint32(len(p))
		u32(off)
	}
	for _, p := range piezas {
		u32(uint32(u.vocab[p]))
	}
	for _, p := range piezas {
		_ = binary.Write(&b, binary.LittleEndian, math.Float64bits(u.scores[u.vocab[p]]))
	}
	for _, p := range piezas {
		b.WriteString(p)
	}
	u32(crc32.Checksum(b.Bytes(), castagnoli))
	return b.Bytes(), nil
}

// leerIndiceTokenizer arma un Unigram sobre los bytes de un tokenizer.idx, sin copiar el vocab:
// las piezas, ids y scores se leen de `crudo` cada vez que la búsqueda los necesita.
func leerIndiceTokenizer(crudo []byte) (*unigram, error) {
	if len(crudo) < len(magiaIndice)+4 {
		return nil, fmt.Errorf("%w: %d bytes", errIndiceInvalido, len(crudo))
	}
	cuerpo := crudo[:len(crudo)-4]
	if crc32.Checksum(cuerpo, castagnoli) != binary.LittleEndian.Uint32(crudo[len(crudo)-4:]) {
		return nil, fmt.Errorf("%w: el crc no coincide (truncado o corrupto)", errIndiceInvalido)
	}
	if err := cabeceraVigente(cuerpo); err != nil {
		return nil, err
	}
	l := lectorIndice{b: cuerpo, pos: largoCabecera}
	u := &unigram{}
	u.unkID = int(l.u32())
	u.maxRunes = int(l.u32())
	u.unkScore = math.Float64frombits(l.u64())
	u.repl = string(l.bytes(int(l.u32())))
	norm := l.bytes(int(l.u32()))
	n := int(l.u32())
	p := &piezasOrdenadas{n: n}
	p.offs = l.bytes((n + 1) * 4)
	p.ids = l.bytes(n * 4)
	p.scores = l.bytes(n * 8)
	if l.err != nil {
		return nil, fmt.Errorf("%w: %v", errIndiceInvalido, l.err)
	}
	p.blob = l.b[l.pos:]
	// Los offsets tienen que crecer y terminar justo en el final del blob: si no, una pieza
	// cortaría fuera del archivo. El crc ya dice que el archivo es el que se escribió; esto dice
	// que lo que se escribió tiene sentido, y cuesta un recorrido de 500.353 enteros.
	prev := uint32(0)
	for k := 0; k <= n; k++ {
		o := binary.LittleEndian.Uint32(p.offs[4*k:])
		if o < prev || int(o) > len(p.blob) {
			return nil, fmt.Errorf("%w: offset %d fuera de orden o de rango", errIndiceInvalido, k)
		}
		prev = o
	}
	if int(prev) != len(p.blob) || u.maxRunes < 1 {
		return nil, fmt.Errorf("%w: el blob no cierra con los offsets", errIndiceInvalido)
	}
	steps, err := parseNormalizer(norm)
	if err != nil {
		return nil, fmt.Errorf("%w: normalizer: %v", errIndiceInvalido, err)
	}
	u.steps = steps
	u.piezas = p
	return u, nil
}

// largoCabecera es lo que ocupan la magia, el formato y el tipo al principio del índice.
const largoCabecera = len(magiaIndice) + 8

// cabeceraVigente dice si el índice empieza con la cabecera que ESTE binario sabe leer: la magia,
// el formato y el tipo. Es una sola función porque son dos los que preguntan —leerIndiceTokenizer
// para interpretarlo y cargarSidecars para darlo por «al día»— y tienen que contestar lo mismo.
func cabeceraVigente(idx []byte) error {
	if len(idx) < largoCabecera || string(idx[:len(magiaIndice)]) != magiaIndice {
		return fmt.Errorf("%w: no es un índice de tokenizer", errIndiceInvalido)
	}
	if f := binary.LittleEndian.Uint32(idx[len(magiaIndice):]); f != formatoIndice {
		return fmt.Errorf("%w: formato %d, este binario lee el %d", errIndiceInvalido, f, formatoIndice)
	}
	if tipo := binary.LittleEndian.Uint32(idx[len(magiaIndice)+4:]); tipo != tipoIndiceUnigr {
		return fmt.Errorf("%w: tipo de tokenizer %d", errIndiceInvalido, tipo)
	}
	return nil
}

// lectorIndice lee campos consecutivos y recuerda el primer error, así el parseo no se llena de
// chequeos: un campo que se sale del buffer deja `err` puesto y todo lo que sigue devuelve ceros.
type lectorIndice struct {
	b   []byte
	pos int
	err error
}

func (l *lectorIndice) bytes(n int) []byte {
	if l.err != nil {
		return nil
	}
	if n < 0 || n > len(l.b)-l.pos {
		l.err = fmt.Errorf("campo de %d bytes en la posición %d de %d", n, l.pos, len(l.b))
		return nil
	}
	out := l.b[l.pos : l.pos+n]
	l.pos += n
	return out
}

func (l *lectorIndice) u32() uint32 {
	if b := l.bytes(4); b != nil {
		return binary.LittleEndian.Uint32(b)
	}
	return 0
}

func (l *lectorIndice) u64() uint64 {
	if b := l.bytes(8); b != nil {
		return binary.LittleEndian.Uint64(b)
	}
	return 0
}

// piezasOrdenadas es el vocabulario del índice: n piezas ordenadas por bytes, cada una con su id
// y su score, leídas directo de los bytes del archivo.
type piezasOrdenadas struct {
	n      int
	offs   []byte // (n+1) u32: la pieza k es blob[offs[k]:offs[k+1]]
	ids    []byte // n u32
	scores []byte // n f64
	blob   []byte
}

func (p *piezasOrdenadas) pieza(k int) []byte {
	return p.blob[binary.LittleEndian.Uint32(p.offs[4*k:]):binary.LittleEndian.Uint32(p.offs[4*k+4:])]
}

func (p *piezasOrdenadas) id(k int) int {
	return int(binary.LittleEndian.Uint32(p.ids[4*k:]))
}

func (p *piezasOrdenadas) score(k int) float64 {
	return math.Float64frombits(binary.LittleEndian.Uint64(p.scores[8*k:]))
}

// coincidencias contesta lo mismo que el mapa —cada pieza que es exactamente runes[:l], en orden
// creciente de l— con una búsqueda binaria que se ANGOSTA: las piezas que empiezan con runes[:l+1]
// son un subrango de las que empiezan con runes[:l], así que cada runa busca dentro del rango de la
// anterior, y apenas el rango queda vacío no hay pieza más larga que pueda coincidir.
//
// Dentro del rango de las que empiezan con la clave, la clave misma —si es una pieza— es la
// PRIMERA, porque es la más corta de todas las que la tienen de prefijo.
//
// La clave se arma con utf8.AppendRune runa por runa, que codifica igual que string(runes[:l]) —
// incluida la runa inválida, que las dos escriben como U+FFFD—.
func (p *piezasOrdenadas) coincidencias(runes []rune, dst []coincidencia, clave []byte) ([]coincidencia, []byte) {
	lo, hi := 0, p.n
	clave = clave[:0]
	for l := 1; l <= len(runes); l++ {
		clave = utf8.AppendRune(clave, runes[l-1])
		// lo: la primera pieza de [lo, hi) que es >= clave.
		a, z := lo, hi
		for a < z {
			m := int(uint(a+z) >> 1)
			if bytes.Compare(p.pieza(m), clave) < 0 {
				a = m + 1
			} else {
				z = m
			}
		}
		lo = a
		// hi: la primera pieza de [lo, hi) que ya NO empieza con clave. Es monótono: una pieza
		// >= clave que no la tiene de prefijo difiere de ella en un byte mayor, y entonces es mayor
		// que toda pieza que sí la tiene.
		a, z = lo, hi
		for a < z {
			m := int(uint(a+z) >> 1)
			if bytes.HasPrefix(p.pieza(m), clave) {
				a = m + 1
			} else {
				z = m
			}
		}
		hi = a
		if lo >= hi {
			break
		}
		if len(p.pieza(lo)) == len(clave) {
			dst = append(dst, coincidencia{largo: l, id: p.id(lo), score: p.score(lo)})
		}
	}
	return dst, clave
}
