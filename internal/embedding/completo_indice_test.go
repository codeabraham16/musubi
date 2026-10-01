package embedding

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// completo_indice_test.go fija que el proveedor COMPLETO tokeniza con tokenizer.idx: lo toma del
// disco cuando está al día, y si no, usa el que acaba de escribir. Las comparaciones bit a bit son
// contra el mapa (conElMapa), que es como tokenizaba antes y la referencia de lo que tiene que dar.

// contarDeserializaciones envuelve la costura deserializarTokenizer y cuenta cuántas veces se
// deserializa tokenizer.json. Se cuentan deserializaciones y no aperturas: el completo LEE el
// archivo siempre, porque el checksum de contenido lo necesita.
func contarDeserializaciones(t *testing.T) *int {
	t.Helper()
	n := 0
	previo := deserializarTokenizer
	deserializarTokenizer = func(b []byte) (tokenizer, error) {
		n++
		return previo(b)
	}
	t.Cleanup(func() { deserializarTokenizer = previo })
	return &n
}

// delIndice dice si el proveedor tokeniza con el índice (las piezas ordenadas) y no retiene el
// mapa: el mapa de POTION son ~40 MB de heap vivo que tienen que quedar para el recolector.
func delIndice(sp *StaticProvider) bool {
	u, ok := sp.tok.(*unigram)
	return ok && u.piezas != nil && u.vocab == nil
}

// comoTokeniza nombra el camino para los mensajes: el índice y el mapa son el mismo tipo, así que
// un %T no los distingue.
func comoTokeniza(sp *StaticProvider) string {
	u, ok := sp.tok.(*unigram)
	switch {
	case !ok:
		return fmt.Sprintf("%T", sp.tok)
	case u.piezas != nil && u.vocab != nil:
		return fmt.Sprintf("el índice, pero retiene el mapa (%d piezas)", len(u.vocab))
	case u.vocab != nil:
		return fmt.Sprintf("el mapa (%d piezas)", len(u.vocab))
	case u.piezas != nil:
		return "el índice"
	}
	return "un unigram sin mapa ni índice"
}

// delMapa dice si el proveedor tokeniza con el mapa de tokenizer.json, como antes del índice.
func delMapa(sp *StaticProvider) bool {
	u, ok := sp.tok.(*unigram)
	return ok && u.vocab != nil && u.piezas == nil
}

// contarArmados envuelve la costura armarIndice y cuenta cuántas veces se arma el índice.
func contarArmados(t *testing.T) *int {
	t.Helper()
	n := 0
	previo := armarIndice
	armarIndice = func(u *unigram) ([]byte, error) {
		n++
		return previo(u)
	}
	t.Cleanup(func() { armarIndice = previo })
	return &n
}

// TestElCompletoConElIndiceAlDiaNoDeserializaElJSON fija el ahorro del arranque: con los sidecars
// al día, NewStaticProvider toma el tokenizer de tokenizer.idx y NO deserializa tokenizer.json,
// que sobre POTION es ~1 s de armar un mapa de 500.353 piezas. El daemon, serve y capture
// construyen el proveedor con los sidecars al día casi siempre. El model_id y el vector tienen que
// ser los del mapa, bit a bit.
//
// Sabotaje: no usar nunca el índice al día.
// arnes: archivo="internal/embedding/static.go"
// arnes: de="if u := indiceAlDia(dir, checksum, huellaTabla, huellaTok); u != nil {"
// arnes: a="if u := indiceAlDia(dir, checksum, huellaTabla, huellaTok); u != nil && false {"
func TestElCompletoConElIndiceAlDiaNoDeserializaElJSON(t *testing.T) {
	dir, ref := tablaDeJugueteConSidecars(t, 400, 16)
	n := contarDeserializaciones(t)
	sp, err := NewStaticProvider(dir)
	if err != nil {
		t.Fatal(err)
	}
	if *n != 0 {
		t.Fatalf("con los sidecars al día se deserializó tokenizer.json %d vez/veces", *n)
	}
	if !delIndice(sp) {
		t.Fatalf("con los sidecars al día el completo tenía que tokenizar con el índice, y tokeniza con %s", comoTokeniza(sp))
	}
	compararBitABit(t, ref, sp, textosDePrueba())
}

// TestElCompletoUsaElIndiceQueAcabaDeEscribir: en el primer arranque no hay sidecars, así que el
// completo arma el mapa y escribe el índice. Desde ahí tokeniza con el índice que acaba de
// escribir, y el mapa queda para el recolector: el ahorro no espera al arranque siguiente.
//
// Sabotaje: seguir con el mapa después de escribir el índice.
// arnes: archivo="internal/embedding/static.go"
// arnes: de="if escrito := escribirSidecarsSiHaceFalta(dir, u, checksum, huellaTabla, huellaTok); escrito != nil {"
// arnes: a="if escrito := escribirSidecarsSiHaceFalta(dir, u, checksum, huellaTabla, huellaTok); escrito != nil && false {"
func TestElCompletoUsaElIndiceQueAcabaDeEscribir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tabla-juguete")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	escribirTablaDeJuguete(t, dir, 400, 16, 7)
	n := contarDeserializaciones(t)
	sp, err := NewStaticProvider(dir)
	if err != nil {
		t.Fatal(err)
	}
	if *n != 1 {
		t.Fatalf("control: sin sidecars el completo tenía que deserializar tokenizer.json una vez, y lo hizo %d", *n)
	}
	for _, f := range []string{archivoIndice, archivoIdentidad} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("control: el primer arranque no escribió %s: %v", f, err)
		}
	}
	if !delIndice(sp) {
		t.Fatalf("después de escribir el índice el completo sigue tokenizando con %s", comoTokeniza(sp))
	}
	compararBitABit(t, conElMapa(t, sp, dir), sp, textosDePrueba())
}

// TestUnIndiceIlegibleConIdentidadVigenteSeReescribe fija qué quiere decir «al día»: que el índice
// se puede USAR, no que la identidad lo nombra. Un tokenizer.idx con identidad vigente y crc
// correcto que leerIndiceTokenizer rechaza —acá, con maxRunes en cero— se reescribe en el próximo
// arranque. Si «al día» fuera sólo la identidad, ese índice no se reescribiría nunca: la consulta
// liviana quedaría sin atajo para siempre y el completo pagaría el mapa en cada arranque.
//
// Sabotaje: dar por al día lo que la identidad acepta, sin leer el índice.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="if al := indiceAlDia(dir, checksum, tabla, tok); al != nil {"
// arnes: a="if id, _, err := cargarSidecars(dir); err == nil && id.Checksum == checksum { return nil }; if al := indiceAlDia(dir, checksum, tabla, tok); al != nil {"
func TestUnIndiceIlegibleConIdentidadVigenteSeReescribe(t *testing.T) {
	dir, ref := tablaDeJugueteConSidecars(t, 200, 8)
	rutaIdx := filepath.Join(dir, archivoIndice)
	idx, err := os.ReadFile(rutaIdx)
	if err != nil {
		t.Fatal(err)
	}
	malo := append([]byte(nil), idx...)
	binary.LittleEndian.PutUint32(malo[largoCabecera+4:], 0) // maxRunes
	binary.LittleEndian.PutUint32(malo[len(malo)-4:], crc32.Checksum(malo[:len(malo)-4], castagnoli))
	reemplazarIndice(t, dir, malo)
	if _, err := NewConsultaLiviana(dir); !errors.Is(err, ErrSinAtajo) {
		t.Fatalf("control: un índice con maxRunes en cero no tenía que servir; esperaba ErrSinAtajo, obtuve %v", err)
	}

	sp, err := NewStaticProvider(dir)
	if err != nil {
		t.Fatal(err)
	}
	ahora, err := os.ReadFile(rutaIdx)
	if err != nil {
		t.Fatal(err)
	}
	if len(ahora) < largoCabecera+8 || binary.LittleEndian.Uint32(ahora[largoCabecera+4:]) == 0 {
		t.Fatal("el índice ilegible sigue en disco después de un NewStaticProvider: el atajo queda apagado")
	}
	if !delIndice(sp) {
		t.Fatalf("después de reescribir el índice el completo tenía que tokenizar con él, y tokeniza con %s", comoTokeniza(sp))
	}
	liv, err := NewConsultaLiviana(dir)
	if err != nil {
		t.Fatalf("después de reescribir tenía que haber atajo: %v", err)
	}
	compararBitABit(t, ref, liv, textosDePrueba()[:10])
	compararBitABit(t, ref, sp, textosDePrueba()[:10])
}

// TestConQueTokenizaElCompleto es la matriz de NewStaticProvider. Para cada forma en que puede
// terminar la escritura de los sidecars fija cuatro cosas: con qué tokeniza el completo, cuántas
// veces armó el índice ESTE proceso, si queda una identidad en disco y si la consulta liviana
// arranca después. En todas las filas el vector es bit a bit el del mapa, y el arranque siguiente,
// ya sin la falla, deja todo al día.
//
// Los comentarios del código no repiten esta lista: el de NewStaticProvider dice la regla y apunta
// acá, y los de consulta_liviana.go e indice_tokenizer.go apuntan a NewStaticProvider. Copiada en
// cada comentario, la lista se desfasó del código en más de un lugar.
//
// Sabotaje: soltar el índice recién escrito cuando lo único que falla es la identidad.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="return escrito // el índice quedó escrito"
// arnes: a="return nil // el índice quedó escrito"
//
// Sabotaje: no tomar el índice que otro proceso dejó al día mientras éste armaba el mapa.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="return al // al día: nada que escribir"
// arnes: a="_ = al // al día: nada que escribir"
//
// Sabotaje: armar el índice en memoria cuando no quedó ninguno que usar.
// arnes: archivo="internal/embedding/static.go"
// arnes: de="return &StaticProvider{"
// arnes: a="if u, ok := tok.(*unigram); ok && u.piezas == nil { if b, e := armarIndice(u); e == nil { if w, e := leerIndiceTokenizer(b); e == nil { tok = w } } }; return &StaticProvider{"
//
// Sabotaje: escribir la identidad de un índice recién escrito que no se puede leer.
// arnes: archivo="internal/embedding/consulta_liviana.go"
// arnes: de="return nil // ilegible: sin identidad"
// arnes: a="_ = 0 // ilegible: sin identidad"
func TestConQueTokenizaElCompleto(t *testing.T) {
	yaAlDia := func(t *testing.T, dir string) func() {
		if _, err := NewStaticProvider(dir); err != nil {
			t.Fatal(err)
		}
		return nil
	}
	filas := []struct {
		nombre    string
		montar    func(t *testing.T, dir string) (curar func())
		conIndice bool
		armados   int
		identidad bool
		atajo     bool
	}{
		{"primer arranque", nil, true, 1, true, true},
		{"los sidecars ya estaban al día", yaAlDia, true, 0, true, true},
		{"otro proceso los deja al día mientras éste arma el mapa", otroProcesoLosEscribe, true, 0, true, true},
		{"la carpeta no admite escritura", sinTemporal, false, 0, false, false},
		{"no se pudieron tomar las huellas", sinHuellaDeLaTabla, false, 0, false, false},
		{"la tabla cambia durante la carga", laTablaCambiaAlCargar, false, 0, false, false},
		{"el índice armado no se puede guardar", falloElRename(archivoIndice), false, 1, false, false},
		{"el índice escrito no se puede leer", indiceIlegibleAlEscribir, false, 1, false, false},
		{"falla sólo la identidad", falloElRename(archivoIdentidad), true, 1, false, false},
	}
	for _, f := range filas {
		t.Run(f.nombre, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "tabla-juguete")
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			escribirTablaDeJuguete(t, dir, 400, 16, 7)
			armados := contarArmados(t)
			curar := func() {}
			if f.montar != nil {
				if c := f.montar(t, dir); c != nil {
					curar = c
				}
			}
			*armados = 0
			sp, err := NewStaticProvider(dir)
			curar()
			if err != nil {
				t.Fatalf("una falla de los sidecars no puede romper el proveedor completo: %v", err)
			}
			bien, quiero := delMapa(sp), "el mapa"
			if f.conIndice {
				bien, quiero = delIndice(sp), "el índice"
			}
			if !bien {
				t.Errorf("tenía que tokenizar con %s, y tokeniza con %s", quiero, comoTokeniza(sp))
			}
			if *armados != f.armados {
				t.Errorf("este proceso armó el índice %d vez/veces, y tenían que ser %d", *armados, f.armados)
			}
			if _, err := os.Stat(filepath.Join(dir, archivoIdentidad)); (err == nil) != f.identidad {
				t.Errorf("identidad.json en disco: %v, y tenía que ser %v (%v)", err == nil, f.identidad, err)
			}
			ref := conElMapa(t, sp, dir)
			compararBitABit(t, ref, sp, textosDePrueba())
			liv, err := NewConsultaLiviana(dir)
			switch {
			case f.atajo && err != nil:
				t.Errorf("después de este arranque tenía que haber atajo: %v", err)
			case f.atajo:
				compararBitABit(t, ref, liv, textosDePrueba()[:10])
			case err == nil:
				t.Errorf("sin sidecars al día no tenía que haber atajo, y la consulta liviana arrancó "+
					"como %s", liv.Name())
			case !errors.Is(err, ErrSinAtajo) && !errors.Is(err, ErrIdentidadVencida):
				t.Errorf("sin atajo tiene que decirlo con ErrSinAtajo o ErrIdentidadVencida: %v", err)
			}

			// El arranque siguiente, ya sin la falla.
			sp2, err := NewStaticProvider(dir)
			if err != nil {
				t.Fatal(err)
			}
			if !delIndice(sp2) {
				t.Errorf("el arranque siguiente tenía que tokenizar con el índice, y tokeniza con %s",
					comoTokeniza(sp2))
			}
			liv2, err := NewConsultaLiviana(dir)
			if err != nil {
				t.Fatalf("después del arranque siguiente tenía que haber atajo: %v", err)
			}
			ref2 := conElMapa(t, sp2, dir)
			compararBitABit(t, ref2, sp2, textosDePrueba()[:10])
			compararBitABit(t, ref2, liv2, textosDePrueba()[:10])
		})
	}
}

// Las fallas de TestConQueTokenizaElCompleto. Cada una se monta antes del NewStaticProvider y
// devuelve con qué curarla después; la cura también queda en t.Cleanup por si la fila corta antes.

// otroProcesoLosEscribe: entre la pregunta del camino rápido y la escritura, otro proceso deja los
// sidecars al día. Se simula con un NewStaticProvider adentro de la deserialización del mapa, con
// armarIndice desviada para que ese armado no se cuente como de ESTE proceso.
func otroProcesoLosEscribe(t *testing.T, dir string) func() {
	previo := deserializarTokenizer
	otro := false
	deserializarTokenizer = func(b []byte) (tokenizer, error) {
		if !otro {
			otro = true
			contado := armarIndice
			armarIndice = escribirIndiceTokenizer
			_, err := NewStaticProvider(dir)
			armarIndice = contado
			if err != nil {
				t.Errorf("el otro proceso: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, archivoIndice)); err != nil {
				t.Errorf("control: el otro proceso no dejó %s: %v", archivoIndice, err)
			}
		}
		return previo(b)
	}
	t.Cleanup(func() { deserializarTokenizer = previo })
	return func() {
		deserializarTokenizer = previo
		if !otro {
			t.Error("control: el otro proceso no llegó a correr, así que la fila no mide nada")
		}
	}
}

// sinTemporal: la carpeta de la tabla no admite escritura.
func sinTemporal(t *testing.T, _ string) func() {
	previo := crearTemporal
	crearTemporal = func(string, string) (*os.File, error) {
		return nil, fmt.Errorf("simulado: la carpeta de la tabla no admite escritura")
	}
	curar := func() { crearTemporal = previo }
	t.Cleanup(curar)
	return curar
}

// sinHuellaDeLaTabla: model.safetensors no está donde huellaDe lo busca, pero la carga lo lee igual
// por la costura. Lo único que falta son las huellas.
func sinHuellaDeLaTabla(t *testing.T, dir string) func() {
	tab := filepath.Join(dir, archivoTabla)
	aparte := tab + ".aparte"
	if err := os.Rename(tab, aparte); err != nil {
		t.Fatal(err)
	}
	previo := abrirParaLeer
	abrirParaLeer = func(ruta string) (archivoDeLectura, error) {
		if ruta == tab {
			ruta = aparte
		}
		return previo(ruta)
	}
	curado := false
	curar := func() {
		if curado {
			return
		}
		curado = true
		abrirParaLeer = previo
		if err := os.Rename(aparte, tab); err != nil {
			t.Errorf("no se pudo devolver la tabla a su lugar: %v", err)
		}
	}
	t.Cleanup(curar)
	return curar
}

// laTablaCambiaAlCargar reemplaza model.safetensors apenas cargarTablaEnStreaming lo cierra: las
// últimas filas cambian, el tamaño es el mismo y la fecha, una hora más tarde.
func laTablaCambiaAlCargar(t *testing.T, dir string) func() {
	tab := filepath.Join(dir, archivoTabla)
	nueva, err := os.ReadFile(tab)
	if err != nil {
		t.Fatal(err)
	}
	for i := len(nueva) - 256; i < len(nueva); i++ {
		nueva[i] ^= 0x5a
	}
	previo := abrirParaLeer
	usado := false
	abrirParaLeer = func(ruta string) (archivoDeLectura, error) {
		f, err := previo(ruta)
		if err != nil || usado || ruta != tab {
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
	return func() {
		abrirParaLeer = previo
		if !usado {
			t.Error("control: la tabla no cambió durante la carga, así que la fila no mide nada")
		}
	}
}

// falloElRename hace fallar el rename que pone `archivo` en su lugar: en Windows, el de un archivo
// que otro proceso tiene abierto.
func falloElRename(archivo string) func(*testing.T, string) func() {
	return func(t *testing.T, _ string) func() {
		previo := renombrar
		renombrar = func(desde, hasta string) error {
			if filepath.Base(hasta) == archivo {
				return fmt.Errorf("simulado: no se pudo poner %s en su lugar", archivo)
			}
			return previo(desde, hasta)
		}
		curar := func() { renombrar = previo }
		t.Cleanup(curar)
		return curar
	}
}

// indiceIlegibleAlEscribir: el índice se arma —y se cuenta—, pero lo que llega al disco no se puede
// leer.
func indiceIlegibleAlEscribir(t *testing.T, _ string) func() {
	previo := armarIndice
	armarIndice = func(u *unigram) ([]byte, error) {
		if _, err := previo(u); err != nil {
			return nil, err
		}
		return []byte("no es un índice"), nil
	}
	curar := func() { armarIndice = previo }
	t.Cleanup(curar)
	return curar
}

// huellasDelIndiceReal ata cada formatoIndice a los bytes del índice armado desde el tokenizer.json
// de POTION que baja `embed pull`: el sha256 de escribirIndiceTokenizer sobre ese asset.
//
// NO ES UN DERIVADO, ES UN COMPROMISO: «el formato 1 son estos bytes». Desde que el completo
// tokeniza con el índice, confía en un tokenizer.idx que pudo escribir OTRO binario. Si ese índice
// está roto, lo rechazan el crc y los offsets; si está entero pero se derivó distinto, lo único que
// lo delata es el formato. Si la derivación cambia (unkPenalty, maxRunes, los ids, el orden de las
// piezas) sin que suba el formato, los índices viejos en disco siguen valiendo con la derivación
// vieja y el completo y la consulta liviana dan otros vectores que un binario recién instalado, con
// el mismo model_id.
//
// UNA ENTRADA NO SE EDITA. Si los bytes cambian, sube formatoIndice y se AGREGA la huella nueva
// bajo el formato nuevo; la del viejo queda como registro de qué escribió cada binario.
var huellasDelIndiceReal = map[uint32]string{
	1: "27b490b8fd6f9da65bf47a5bae72ca46ba449246377f0ded669731c42ac6995c",
}

// TestElIndiceRealEstaAtadoASuFormato es la guarda de huellasDelIndiceReal. Corre en recall-gate.
//
// EL CONTROL VA PRIMERO y separa dos causas que dan el mismo síntoma: el sha256 del tokenizer.json
// tiene que ser el que KnownModels pinea para `embed pull`. Si no lo es, cambió el ASSET y esta
// guarda no tiene nada que decir; si lo es y la huella del índice cambió, cambió la DERIVACIÓN.
//
// Sabotaje: penalizar distinto al unk, que cambia unkScore y nada más.
// arnes: archivo="internal/embedding/spm.go"
// arnes: env="MUSUBI_SPM_TESTDATA"
// arnes: de="const unkPenalty = 10.0"
// arnes: a="const unkPenalty = 10.5"
func TestElIndiceRealEstaAtadoASuFormato(t *testing.T) {
	dir := os.Getenv("MUSUBI_SPM_TESTDATA")
	if dir == "" {
		t.Skip("MUSUBI_SPM_TESTDATA no seteado: se saltea la huella del índice contra el tokenizer real de POTION")
	}
	crudo, err := os.ReadFile(filepath.Join(dir, archivoTokenizer))
	if err != nil {
		t.Fatal(err)
	}
	pineado := ""
	for _, f := range KnownModels["potion-multilingual-128M"].Files {
		if f.Name == archivoTokenizer {
			pineado = f.SHA256
		}
	}
	if pineado == "" {
		t.Fatalf("control: KnownModels no pinea %s de potion-multilingual-128M", archivoTokenizer)
	}
	if got := sha256Hex(crudo); got != pineado {
		t.Fatalf("control: %s no es el de POTION que baja `embed pull` (sha256 %s, KnownModels pinea %s). "+
			"Esta guarda fija la derivación del índice sobre ESE asset", filepath.Join(dir, archivoTokenizer), got, pineado)
	}
	tok, err := loadTokenizerBytes(crudo)
	if err != nil {
		t.Fatal(err)
	}
	u, ok := tok.(*unigram)
	if !ok {
		t.Fatalf("el tokenizer real no es Unigram: %T", tok)
	}
	idx, err := escribirIndiceTokenizer(u)
	if err != nil {
		t.Fatal(err)
	}
	fijada, ok := huellasDelIndiceReal[formatoIndice]
	if !ok {
		t.Fatalf("formatoIndice %d no tiene huella en huellasDelIndiceReal: agregala con el sha256 que da esta prueba, %s",
			formatoIndice, sha256Hex(idx))
	}
	if got := sha256Hex(idx); got != fijada {
		t.Errorf(`LOS BYTES DEL ÍNDICE CAMBIARON Y formatoIndice NO SUBIÓ.
  sha256 armado hoy        : %s
  sha256 fijado (formato %d): %s

El asset es el mismo (el control de arriba pasó), así que cambió la DERIVACIÓN del índice. Los
tokenizer.idx que ya están en disco, escritos por binarios anteriores, siguen diciendo «formato %d»
y el completo los va a usar con la derivación VIEJA: vectores distintos con el mismo model_id.

Subí formatoIndice en indice_tokenizer.go y AGREGÁ la huella nueva bajo el formato nuevo en
huellasDelIndiceReal. No edites la del formato %d.`, got, formatoIndice, fijada, formatoIndice, formatoIndice)
	}
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
