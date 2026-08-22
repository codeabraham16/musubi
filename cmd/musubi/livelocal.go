package main

// livelocal.go es la mitad LECTORA del riel local: sigue los archivos que dejan los daemons stdio
// (ver internal/mcp/spool.go) y los mete en el mismo riel por donde ya entra el central.
//
// POR QUÉ ENTRA AL MISMO RIEL Y NO A UNO NUEVO. `relayVivo` ya tiene lo difícil resuelto: fan-out a
// N pestañas, ring para que una pestaña que abre tarde no arranque en blanco, y descarte por
// suscriptor lento. Un riel local paralelo sería una segunda copia de todo eso, y dos copias se
// desincronizan. Acá sólo se publica en el que hay.
//
// POR QUÉ SE PODA. Un daemon que muere de golpe no borra su archivo, y sin poda el panel lo
// releería para siempre. Esa es exactamente la forma del bug de los `bridge -watch` huérfanos que
// medimos hoy —procesos muertos cuyos restos siguen contando— así que el mismo día que lo
// diagnosticamos no vamos a construir su gemelo.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// lecturaLocalCada es cada cuánto se mira el directorio. 250 ms es imperceptible para el ojo y son
// 4 stat por segundo sobre un puñado de archivos: nada. Bajarlo no haría el riel más "vivo" —el
// evento ya está escrito cuando se lo lee— y subirlo se empezaría a notar como retraso.
const lecturaLocalCada = 250 * time.Millisecond

// graciaPoda es cuánto tiene que llevar quieto un archivo antes de que se lo considere candidato a
// poda. Existe para no borrar jamás el archivo de un daemon vivo por una lectura de PID confusa:
// los PID de Windows se reciclan, así que la muerte del proceso NO alcanza sola como criterio.
const graciaPoda = 2 * time.Minute

// lectorSpool recuerda por dónde iba en cada archivo.
type lectorSpool struct {
	dir string
	pos map[string]int64
	// seq es la ultima marca entregada de cada archivo. Es lo que hace segura la relectura
	// desde cero cuando el escritor trunco: sin esto, un reinicio de offset re-entregaria todo
	// lo que el archivo todavia tenga y el panel mostraria el pasado como si fuera presente.
	seq map[string]int64
}

func nuevoLectorSpool(dir string) *lectorSpool {
	if dir == "" {
		return nil
	}
	return &lectorSpool{dir: dir, pos: map[string]int64{}, seq: map[string]int64{}}
}

// leerNuevos devuelve las líneas aparecidas desde la última pasada, ya como JSON crudo: el riel no
// interpreta los eventos, igual que el relay del central.
func (l *lectorSpool) leerNuevos() [][]byte {
	if l == nil {
		return nil
	}
	entradas, err := os.ReadDir(l.dir)
	if err != nil {
		return nil // el directorio todavía no existe: no hay daemons, no es un error
	}
	var out [][]byte
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		out = append(out, l.leerArchivo(filepath.Join(l.dir, e.Name()), e.Name())...)
	}
	return out
}

func (l *lectorSpool) leerArchivo(ruta, nombre string) [][]byte {
	fragmento, hayCorte := l.trozoNuevo(ruta, nombre, l.pos[nombre])
	if !hayCorte {
		return nil // nada nuevo, o una linea a medio escribir que se deja para la proxima pasada
	}
	out := l.parsear(fragmento, nombre)

	// EL ESCRITOR TRUNCO Y EL ARCHIVO YA VOLVIO A CRECER. Comparar el tamano contra el offset NO
	// alcanza para detectarlo: solo lo delata mientras lo nuevo sea mas corto que lo viejo, y en
	// cuanto lo supera el lector queda leyendo desde el medio de una linea para siempre. Lo que sí
	// lo delata es esto: se leyeron bytes con al menos un salto de linea y NINGUNO produjo un
	// evento nuevo. Ahi se relee desde cero, y el filtro por `seq` evita re-entregar lo ya visto.
	if len(out) == 0 {
		l.pos[nombre] = 0
		fragmento, hayCorte = l.trozoNuevo(ruta, nombre, 0)
		if !hayCorte {
			return nil
		}
		out = l.parsear(fragmento, nombre)
	}
	return out
}

// trozoNuevo devuelve los bytes desde `desde` hasta el ULTIMO salto de linea, y avanza el offset.
// Solo se consumen lineas completas: entregar media linea es perderla en silencio, porque el
// navegador la descarta sin decir nada.
func (l *lectorSpool) trozoNuevo(ruta, nombre string, desde int64) ([]byte, bool) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return nil, false
	}
	if fi.Size() < desde { // el caso facil del truncado: el archivo se achico
		desde = 0
	}
	if fi.Size() <= desde {
		return nil, false
	}
	if _, err := f.Seek(desde, 0); err != nil {
		return nil, false
	}
	b := make([]byte, fi.Size()-desde)
	n, _ := f.Read(b)
	b = b[:n]

	corte := strings.LastIndexByte(string(b), '\n')
	if corte < 0 {
		return nil, false
	}
	l.pos[nombre] = desde + int64(corte) + 1
	return b[:corte], true
}

// parsear queda con las lineas validas que ademas son NUEVAS para este archivo.
func (l *lectorSpool) parsear(b []byte, nombre string) [][]byte {
	var out [][]byte
	for _, linea := range strings.Split(string(b), "\n") {
		linea = strings.TrimSpace(linea)
		if linea == "" || !json.Valid([]byte(linea)) {
			continue // una linea rota se descarta, no tumba la pasada
		}
		var ev struct {
			Seq int64 `json:"seq"`
		}
		if json.Unmarshal([]byte(linea), &ev) == nil && ev.Seq > 0 {
			if ev.Seq <= l.seq[nombre] {
				continue // ya entregada: pasa cuando se releyo desde cero tras un truncado
			}
			l.seq[nombre] = ev.Seq
		}
		out = append(out, []byte(linea))
	}
	return out
}

// podar saca los archivos de daemons que ya no existen. Dos condiciones a la vez, y las dos hacen
// falta: el proceso tiene que estar muerto Y el archivo tiene que llevar un rato quieto. Con sólo
// la primera, un PID reciclado borraría el archivo de un daemon vivo.
func (l *lectorSpool) podar(ahora time.Time) int {
	if l == nil {
		return 0
	}
	entradas, err := os.ReadDir(l.dir)
	if err != nil {
		return 0
	}
	podados := 0
	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSuffix(e.Name(), ".jsonl"))
		if err != nil {
			continue // no es un archivo nuestro; no se toca
		}
		fi, err := e.Info()
		if err != nil || ahora.Sub(fi.ModTime()) < graciaPoda {
			continue
		}
		if procesoVivo(pid) {
			continue
		}
		if os.Remove(filepath.Join(l.dir, e.Name())) == nil {
			// Las DOS: si un PID se recicla, el archivo nuevo tiene que arrancar limpio y no filtrado
			// por las marcas del proceso anterior.
			delete(l.pos, e.Name())
			delete(l.seq, e.Name())
			podados++
		}
	}
	return podados
}

// seguirSpoolLocal bombea el spool al riel hasta que ctx se cancele. Bloquea: va en una goroutine.
func seguirSpoolLocal(stop <-chan struct{}, l *lectorSpool, r *relayVivo) {
	if l == nil || r == nil {
		return
	}
	tick := time.NewTicker(lecturaLocalCada)
	defer tick.Stop()
	poda := time.NewTicker(graciaPoda)
	defer poda.Stop()
	for {
		select {
		case <-stop:
			return
		case <-tick.C:
			for _, linea := range l.leerNuevos() {
				r.publicar(frame{evento: "uso", data: linea})
			}
		case <-poda.C:
			l.podar(time.Now())
		}
	}
}
