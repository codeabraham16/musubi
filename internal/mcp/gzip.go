package mcp

import (
	"bytes"
	"compress/gzip"
	"io"
	"sync"
)

// gzip.go es la compresión que comparten las dos puntas del sync: la subida (SyncClient.Push y
// PushGraphDe) y las respuestas del central (writeHTTPJSON). Vive aparte, y una sola vez, porque la
// usan el cliente y el servidor: los dos PR que comprimen, la subida y la bajada, la habían escrito
// cada uno con otra firma, en el mismo paquete.

// escritoresGzip recicla los compresores. Un gzip.Writer nuevo reserva sus tablas en la primera
// escritura: medido, sin el pool cada cuerpo comprimido reservaba ~800 KB para el recolector, y con
// él, ~40 KB. El pool comparte el COMPRESOR, nunca el destino: cada llamada escribe en su propio
// buffer, y compartirlo mezclaría el cuerpo de una respuesta con el de otra.
var escritoresGzip = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}

// comprimirGzip comprime un cuerpo entero en memoria, al nivel por defecto. La salida es
// determinista —el encabezado va sin fecha ni nombre—, así que un reintento manda exactamente los
// mismos bytes. Con un error decide el que llama: la subida manda la nota en claro y el central
// contesta en claro, que siempre es válido.
func comprimirGzip(cuerpo []byte) ([]byte, error) {
	zw := escritoresGzip.Get().(*gzip.Writer)
	defer escritoresGzip.Put(zw)
	var buf bytes.Buffer
	zw.Reset(&buf)
	if _, err := zw.Write(cuerpo); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
