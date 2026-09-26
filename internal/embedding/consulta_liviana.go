package embedding

// consulta_liviana.go es el embebedor de UNA consulta sin cargar la tabla: el mismo vector que
// StaticProvider, bit a bit, pagando lo que cuesta embeber un texto y no lo que cuesta armar el
// proveedor.
//
// De dónde sale cada cosa:
//   - el tokenizer, de tokenizer.idx (indice_tokenizer.go), que se usa tal cual se lee;
//   - las filas de la tabla, con ReadAt sobre model.safetensors, sólo las de los tokens del texto;
//   - el nombre (el model_id, que es la procedencia del vector), de identidad.json.
//
// Los dos archivos de al lado —tokenizer.idx e identidad.json— los escribe NewStaticProvider cada
// vez que construye el proveedor completo (daemon, serve, embed pull, backfill) y faltan o están
// vencidos. Si no están, o si la tabla cambió desde que se escribieron, no hay atajo: el
// constructor devuelve error y quien lo pidió sigue sin vector, como hoy. Degradar, no romper.
//
// LA TABLA SE DA POR NO CAMBIADA MIRANDO TAMAÑO Y FECHA de model.safetensors y tokenizer.json, y
// no releyéndola: es la decisión del dueño (ola 2, decisión 5). Releer las 488 MB para validar es
// justo lo que este archivo existe para no hacer. Lo que eso no ve —una tabla reescrita con el
// mismo tamaño y la misma fecha— es el mismo hueco que tiene cualquier herramienta que decide por
// mtime, y no es la amenaza de N1 (re-destilar la tabla cambia la fecha).

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"musubi/internal/logx"
)

const (
	archivoTabla     = "model.safetensors"
	archivoTokenizer = "tokenizer.json"
	archivoIndice    = "tokenizer.idx"
	archivoIdentidad = "identidad.json"
	// formatoIdentidad se sube cuando cambia el significado de identidad.json.
	formatoIdentidad = 1
)

// ErrSinAtajo: no hay índice ni identidad utilizables al lado de la tabla (nunca se escribieron,
// son de otro formato, la tabla es WordPiece). No es una falla: el embebedor de consulta no está
// disponible y el caller sigue sin vector.
var ErrSinAtajo = errors.New("sin índice del tokenizer: el proveedor completo todavía no lo escribió")

// ErrIdentidadVencida: el índice y la identidad existen pero ya no describen la tabla que hay en
// disco (cambió el tamaño o la fecha de model.safetensors o de tokenizer.json, o el índice no es el
// que la identidad dice). Usarlos daría vectores de OTRA tabla con el nombre de ésta, que es la
// corrupción silenciosa que N1 prohíbe; por eso no se construye nada.
var ErrIdentidadVencida = errors.New("la identidad de la tabla está vencida: la tabla cambió desde que se escribió el índice")

// huellaArchivo es lo que identifica un archivo sin leerlo: tamaño y fecha de modificación.
type huellaArchivo struct {
	Tamano  int64 `json:"tamano"`
	MtimeNs int64 `json:"mtime_ns"`
}

func huellaDeInfo(st os.FileInfo) huellaArchivo {
	return huellaArchivo{Tamano: st.Size(), MtimeNs: st.ModTime().UnixNano()}
}

func huellaDe(ruta string) (huellaArchivo, error) {
	st, err := os.Stat(ruta)
	if err != nil {
		return huellaArchivo{}, err
	}
	return huellaDeInfo(st), nil
}

// coincide es LA comparación de la identidad, y la única: el constructor, el Embed y
// NewStaticProvider (para saber si tiene que reescribir) preguntan acá.
func (h huellaArchivo) coincide(st os.FileInfo) bool {
	return st.Size() == h.Tamano && st.ModTime().UnixNano() == h.MtimeNs
}

// identidadDeTabla es el contenido de identidad.json.
type identidadDeTabla struct {
	Formato int `json:"formato"`
	// Checksum es lo que va después de la @ en el model_id (checksumDeCRC). El basename NO se guarda:
	// el nombre se deriva del directorio actual con modelIDDe, igual que en NewStaticProvider, así
	// que una carpeta renombrada da el mismo nombre por los dos caminos.
	Checksum  string        `json:"checksum"`
	Tabla     huellaArchivo `json:"tabla"`
	Tokenizer huellaArchivo `json:"tokenizer"`
	// Indice ata la identidad al tokenizer.idx con el que se escribió. Sin esto, un índice que no
	// se pudo reemplazar (Windows no renombra encima de un archivo abierto) quedaría al lado de una
	// identidad nueva, y el tokenizer viejo produciría vectores con el nombre de la tabla nueva.
	Indice struct {
		Tamano int64  `json:"tamano"`
		CRC32C uint32 `json:"crc32c"`
	} `json:"indice"`
}

// modelIDDe arma el model_id de una tabla estática. Es la ÚNICA derivación: NewStaticProvider y la
// consulta liviana la llaman los dos, así el vector que se indexa y el que se consulta llevan la
// misma procedencia por construcción.
func modelIDDe(dir, checksum string) string {
	return "static:" + filepath.Base(filepath.Clean(dir)) + "@" + checksum
}

// archivoDeLectura es lo que el camino estático le pide a un archivo abierto. *os.File lo cumple.
type archivoDeLectura interface {
	io.Reader
	io.ReaderAt
	io.Closer
	Stat() (os.FileInfo, error)
}

// abrirParaLeer es la costura por la que pasan TODAS las lecturas de los archivos de la tabla, en
// los dos caminos (cargarTablaEnStreaming y la consulta liviana). Existe para las pruebas que
// fijan qué se abre y cuánto se lee: «no abre tokenizer.json» y «no lee la tabla». Producción no la
// toca.
var abrirParaLeer = func(ruta string) (archivoDeLectura, error) { return os.Open(ruta) }

// leerEntero lee un archivo completo por la costura, en un buffer del tamaño justo: io.ReadAll
// sobre los 14 MB del índice de POTION va duplicando el buffer y deja reservado el doble (medido:
// 33,6 MB asignados contra los 14 del archivo).
func leerEntero(ruta string) ([]byte, error) {
	f, err := abrirParaLeer(ruta)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, st.Size())
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, err
	}
	// Un archivo que creció entre el Stat y la lectura se lee corto; el que lo usa lo detecta (el
	// índice por su crc, la identidad por el JSON), y el que se achicó da ErrUnexpectedEOF arriba.
	return buf, nil
}

// cargarSidecars lee identidad.json y tokenizer.idx y comprueba que sigan describiendo la tabla
// que hay en disco. Es el único criterio de «los sidecars sirven»: lo usan la consulta liviana
// (para construirse) y NewStaticProvider (para no reescribirlos en cada arranque).
func cargarSidecars(dir string) (identidadDeTabla, []byte, error) {
	var id identidadDeTabla
	crudo, err := leerEntero(filepath.Join(dir, archivoIdentidad))
	if err != nil {
		return id, nil, fmt.Errorf("%w: %v", ErrSinAtajo, err)
	}
	if err := json.Unmarshal(crudo, &id); err != nil || id.Formato != formatoIdentidad || id.Checksum == "" {
		return id, nil, fmt.Errorf("%w: %s ilegible o de otro formato", ErrSinAtajo, archivoIdentidad)
	}
	for _, par := range []struct {
		archivo string
		huella  huellaArchivo
	}{{archivoTabla, id.Tabla}, {archivoTokenizer, id.Tokenizer}} {
		st, err := os.Stat(filepath.Join(dir, par.archivo))
		if err != nil {
			return id, nil, fmt.Errorf("%w: %v", ErrSinAtajo, err)
		}
		if !par.huella.coincide(st) {
			return id, nil, fmt.Errorf("%w: %s cambió (tamaño o fecha)", ErrIdentidadVencida, par.archivo)
		}
	}
	idx, err := leerEntero(filepath.Join(dir, archivoIndice))
	if err != nil {
		return id, nil, fmt.Errorf("%w: %v", ErrSinAtajo, err)
	}
	if int64(len(idx)) != id.Indice.Tamano || crc32.Checksum(idx, castagnoli) != id.Indice.CRC32C {
		return id, nil, fmt.Errorf("%w: %s no es el que la identidad describe", ErrIdentidadVencida, archivoIndice)
	}
	// Un índice de OTRO formato (el de un binario anterior o posterior) cuenta como ausente. Si
	// sólo se mirara que la identidad lo nombra, NewStaticProvider lo daría por «al día» y no lo
	// reescribiría nunca, mientras la consulta liviana lo rechaza: el atajo apagado para siempre.
	if err := cabeceraVigente(idx); err != nil {
		return id, nil, fmt.Errorf("%w: %v", ErrSinAtajo, err)
	}
	return id, idx, nil
}

// escribirSidecarsSiHaceFalta deja tokenizer.idx e identidad.json al día para la tabla que
// NewStaticProvider acaba de cargar. Es BEST-EFFORT: un directorio de sólo lectura, un disco lleno
// o un rename que Windows rechaza no le cuestan nada al proveedor completo; sólo dejan al camino
// liviano sin atajo hasta el próximo arranque.
//
// `tabla` y `tok` son las huellas tomadas ANTES de leer los archivos. Si al terminar de cargar ya
// no coinciden, alguien reescribió la tabla en el medio y lo que se cargó no se sabe de qué versión
// es: no se escribe nada. Tomarlas DESPUÉS sería peor: pegaría la huella del archivo nuevo al
// contenido viejo, y la consulta liviana lo aceptaría.
//
// EL ORDEN ES PARTE DEL CONTRATO: primero el índice, después la identidad que lo nombra. Si el
// índice no se pudo reemplazar, la identidad NO se escribe; y si la identidad falla, la vieja no
// nombra al índice nuevo. En los dos casos cargarSidecars rechaza, que es lo seguro.
func escribirSidecarsSiHaceFalta(dir string, u *unigram, checksum string, tabla, tok huellaArchivo) {
	limpiarTemporalesHuerfanos(dir, time.Now())
	if id, _, err := cargarSidecars(dir); err == nil && id.Checksum == checksum && id.Tabla == tabla && id.Tokenizer == tok {
		return // al día: nada que escribir
	}
	for _, par := range []struct {
		archivo string
		huella  huellaArchivo
	}{{archivoTabla, tabla}, {archivoTokenizer, tok}} {
		st, err := os.Stat(filepath.Join(dir, par.archivo))
		if err != nil || !par.huella.coincide(st) {
			logx.Warn("la tabla cambió mientras se cargaba: no se escribe el índice del tokenizer", "archivo", par.archivo)
			return
		}
	}
	idx, err := escribirIndiceTokenizer(u)
	if err != nil {
		logx.Warn("no se pudo armar el índice del tokenizer; la consulta liviana queda sin atajo", "error", err)
		return
	}
	if err := escribirAtomico(filepath.Join(dir, archivoIndice), idx); err != nil {
		logx.Warn("no se pudo escribir el índice del tokenizer; la consulta liviana queda sin atajo", "error", err)
		return
	}
	id := identidadDeTabla{Formato: formatoIdentidad, Checksum: checksum, Tabla: tabla, Tokenizer: tok}
	id.Indice.Tamano = int64(len(idx))
	id.Indice.CRC32C = crc32.Checksum(idx, castagnoli)
	crudo, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		return
	}
	if err := escribirAtomico(filepath.Join(dir, archivoIdentidad), append(crudo, '\n')); err != nil {
		logx.Warn("no se pudo escribir la identidad de la tabla; la consulta liviana queda sin atajo", "error", err)
		return
	}
	logx.Info("índice del tokenizer escrito: el embebedor de consulta ya no carga la tabla", "dir", dir)
}

// edadDeHuerfano es a partir de cuándo un temporal de sidecar se da por abandonado. Escribir el
// índice tarda menos de un segundo; una hora deja fuera de duda que no es el de otro daemon que
// está escribiendo ahora mismo.
const edadDeHuerfano = time.Hour

// limpiarTemporalesHuerfanos borra los temporales de sidecar que dejó un proceso MATADO a mitad de
// escritura: un daemon muere con la sesión que lo lanzó, y escribirAtomico no llega a borrar lo
// suyo. Sin esto, cada muerte a destiempo deja 14 MB de índice a medias en la carpeta de la tabla,
// para siempre. Se mira con ReadDir y prefijo, no con Glob, porque la ruta de la tabla es del
// usuario y un corchete en ella cambiaría el patrón.
func limpiarTemporalesHuerfanos(dir string, ahora time.Time) {
	entradas, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entradas {
		n := e.Name()
		if !strings.HasPrefix(n, archivoIndice+".tmp-") && !strings.HasPrefix(n, archivoIdentidad+".tmp-") {
			continue
		}
		if info, err := e.Info(); err == nil && ahora.Sub(info.ModTime()) > edadDeHuerfano {
			_ = os.Remove(filepath.Join(dir, n))
		}
	}
}

// renombrar es os.Rename; costura para probar el rename que falla (en Windows, encima de un
// archivo que otro proceso tiene abierto: Go abre sin FILE_SHARE_DELETE).
var renombrar = os.Rename

// escribirAtomico escribe en un temporal de nombre ÚNICO del mismo directorio y lo renombra encima.
// Nombre único porque varios daemons pueden arrancar a la vez y escribir el mismo sidecar: con un
// temporal fijo, uno truncaría el del otro a mitad de camino. Si el rename falla, el temporal se
// borra y el archivo de destino queda como estaba —entero, viejo o ausente, nunca a medias—.
func escribirAtomico(ruta string, datos []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(ruta), filepath.Base(ruta)+".tmp-"+strconv.Itoa(os.Getpid())+"-*")
	if err != nil {
		return err
	}
	nombre := tmp.Name()
	_, err = tmp.Write(datos)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = renombrar(nombre, ruta)
	}
	if err != nil {
		_ = os.Remove(nombre)
		return err
	}
	return nil
}

// ConsultaLiviana es el embebedor de consulta: implementa Provider sin tener la tabla en memoria.
// Se construye con NewProviderDeConsulta (que lo deja pasar desnudo, igual que a StaticProvider:
// ver needsGateway y newTroceado) o, directo, con NewConsultaLiviana.
type ConsultaLiviana struct {
	rutaTabla string
	huella    huellaArchivo // de model.safetensors, contra la que se compara en cada Embed
	tok       *unigram
	inicio    int64 // offset absoluto del blob de embeddings en model.safetensors
	filas     int
	dim       int
	modelID   string
}

// NewConsultaLiviana arma el embebedor de consulta sobre el directorio de una tabla estática. No
// abre tokenizer.json y de model.safetensors lee sólo el encabezado. Devuelve ErrSinAtajo si no hay
// sidecars utilizables y ErrIdentidadVencida si los hay pero son de otra versión de la tabla.
func NewConsultaLiviana(dir string) (*ConsultaLiviana, error) {
	if dir == "" {
		return nil, fmt.Errorf("static_path vacío: apuntá embedding.static_path a un directorio con model.safetensors + tokenizer.json")
	}
	id, idxCrudo, err := cargarSidecars(dir)
	if err != nil {
		return nil, err
	}
	tok, err := leerIndiceTokenizer(idxCrudo)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrSinAtajo, err)
	}
	c := &ConsultaLiviana{
		rutaTabla: filepath.Join(dir, archivoTabla),
		huella:    id.Tabla,
		tok:       tok,
		modelID:   modelIDDe(dir, id.Checksum),
	}
	f, err := c.abrirTabla()
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var largo [8]byte
	if _, err := f.ReadAt(largo[:], 0); err != nil {
		return nil, fmt.Errorf("safetensors demasiado corto: %w", err)
	}
	hlen := binary.LittleEndian.Uint64(largo[:])
	if hlen == 0 || hlen > topeDeHeader {
		return nil, fmt.Errorf("header safetensors inválido: declara %d bytes", hlen)
	}
	hdr := make([]byte, hlen)
	if _, err := f.ReadAt(hdr, 8); err != nil {
		return nil, fmt.Errorf("header safetensors truncado: %w", err)
	}
	inicio, fin, filas, dim, err := ubicarEmbeddings(hdr, 8+int64(hlen))
	if err != nil {
		return nil, err
	}
	if fin > c.huella.Tamano {
		return nil, fmt.Errorf("blob de embeddings truncado: termina en %d y el archivo tiene %d bytes", fin, c.huella.Tamano)
	}
	c.inicio, c.filas, c.dim = inicio, filas, dim
	return c, nil
}

// abrirTabla abre model.safetensors y comprueba, sobre el archivo YA ABIERTO, que siga siendo el de
// la identidad: entre el constructor y un Embed la tabla puede haberse reemplazado, y leer filas de
// la nueva con el nombre de la vieja es la corrupción que N1 prohíbe.
//
// La tabla se abre y se cierra en cada uso, y no queda abierta mientras viva el proveedor: en
// Windows un archivo abierto no se puede reemplazar, y `musubi embed pull` tiene que poder hacerlo.
func (c *ConsultaLiviana) abrirTabla() (archivoDeLectura, error) {
	f, err := abrirParaLeer(c.rutaTabla)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil || !c.huella.coincide(st) {
		_ = f.Close()
		return nil, fmt.Errorf("%w: %s cambió desde que se construyó el proveedor", ErrIdentidadVencida, archivoTabla)
	}
	return f, nil
}

func (c *ConsultaLiviana) Name() string    { return c.modelID }
func (c *ConsultaLiviana) Dimensions() int { return c.dim }

// Embed produce el MISMO vector que StaticProvider.Embed: el mismo tokenizer (sobre el índice), la
// misma media y la misma normalización (mediaNormalizada, que es una sola función para los dos).
// Lo único distinto es de dónde sale cada fila: ReadAt en vez de la tabla en memoria. Una fila
// repetida en el texto se lee una sola vez.
func (c *ConsultaLiviana) Embed(_ context.Context, text string) ([]float32, error) {
	ids := c.tok.EncodeIDs(text)
	var f archivoDeLectura
	defer func() {
		if f != nil {
			_ = f.Close()
		}
	}()
	leidas := make(map[int][]float32)
	filaBytes := int64(c.dim) * 4
	fila := func(id int) ([]float32, error) {
		if r, ok := leidas[id]; ok {
			return r, nil
		}
		if f == nil {
			var err error
			if f, err = c.abrirTabla(); err != nil {
				return nil, err
			}
		}
		buf := make([]byte, filaBytes)
		if _, err := f.ReadAt(buf, c.inicio+int64(id)*filaBytes); err != nil {
			return nil, fmt.Errorf("leyendo la fila %d de la tabla: %w", id, err)
		}
		r := make([]float32, c.dim)
		for j := range r {
			r[j] = math.Float32frombits(binary.LittleEndian.Uint32(buf[j*4:]))
		}
		leidas[id] = r
		return r, nil
	}
	return mediaNormalizada(ids, c.filas, c.dim, fila)
}
