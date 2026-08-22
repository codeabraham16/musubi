package mcp

// spool.go es la salida del FEED EN VIVO para los daemons que NO sirven HTTP.
//
// EL HUECO QUE CIERRA. `liveFeed` reparte los eventos por un canal en proceso, y el único que los
// expone hacia afuera es `ListenAndServeHTTP` — o sea `musubi serve`, el central. Un daemon stdio
// (el que usa cada sesión de un agente contra la memoria local) publica sus eventos a un feed que
// NADIE escucha: emisor construido, receptor apagado. El trabajo local no se veía en ningún lado.
//
// POR QUÉ UN ARCHIVO POR PROCESO Y NO UNO SOLO. Medido en la máquina de la sala de mando: hay
// **7 daemons stdio vivos a la vez**. Siete escritores concurrentes sobre un mismo archivo es
// contención de escritura y —peor— líneas entrelazadas, que se leen como un evento corrupto y se
// descartan sin que nadie se entere. Con un archivo por PID cada proceso es dueño del suyo: no
// coordina con nadie, acota el propio, y lo borra al salir.
//
// POR QUÉ NO SIRVE LEER EL LEDGER, que es la pregunta obvia: está contestado arriba de todo en
// livefeed.go y las tres razones siguen valiendo (buffer de 10 s, `created_at` = hora del INSERT,
// resolución de 1 segundo). El ledger es la HISTORIA; esto es el PRESENTE.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

// spoolTope es cuánto puede pesar el archivo de UN proceso antes de truncarse.
//
// Un feed no necesita historia —para eso está el ledger—, así que el tope no busca guardar sino
// no crecer. 1 MB son ~6.000 eventos: mucho más de lo que un panel muestra, y nada para el disco.
// Sin tope, en esta misma máquina el sondeo de un día (109.687 invocaciones) escribiría ~18 MB por
// daemon, y hay siete.
const spoolTope = 1 << 20

// spoolLocal escribe los eventos de ESTE proceso a su propio archivo.
type spoolLocal struct {
	mu   sync.Mutex
	ruta string
	f    *os.File
	n    int64 // bytes escritos desde el último truncado
	tope int64
	roto bool // ya falló una vez: se deja de intentar, en silencio
}

// nuevoSpool abre (o crea) el archivo de este proceso. Devuelve nil si no se puede: el llamador no
// tiene que chequear nada, porque todos los métodos toleran el receptor nil.
func nuevoSpool(dir string, pid int, tope int64) *spoolLocal {
	if dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil
	}
	ruta := filepath.Join(dir, spoolNombre(pid))
	f, err := os.OpenFile(ruta, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil
	}
	if tope <= 0 {
		tope = spoolTope
	}
	return &spoolLocal{ruta: ruta, f: f, tope: tope}
}

func spoolNombre(pid int) string { return strconv.Itoa(pid) + ".jsonl" }

// escribir agrega un evento. NO devuelve error a propósito (invariante A5): corre en el camino de
// salida de TODA tool, y una tool que empieza a fallar porque su telemetría falla es peor que no
// tener telemetría. Ante cualquier problema se marca roto y se calla para siempre.
func (s *spoolLocal) escribir(ev LiveEvent) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.roto || s.f == nil {
		return
	}

	b, err := json.Marshal(ev)
	if err != nil {
		s.roto = true
		return
	}
	b = append(b, '\n')

	// El truncado va ANTES de escribir, no después: así el archivo nunca supera el tope, en vez de
	// superarlo y volver. Y se hace con Truncate+Seek sobre el mismo descriptor —no reabriendo—
	// porque el lector puede tener el archivo abierto y reabrir invita a una carrera de nombres.
	if s.n+int64(len(b)) > s.tope {
		if err := s.f.Truncate(0); err != nil {
			s.roto = true
			return
		}
		if _, err := s.f.Seek(0, 0); err != nil {
			s.roto = true
			return
		}
		s.n = 0
	}

	k, err := s.f.Write(b)
	if err != nil {
		s.roto = true
		return
	}
	s.n += int64(k)
}

// cerrar borra el archivo propio. Morir de golpe también existe, así que el lector poda por su
// cuenta (ver A7); esto es sólo la salida limpia.
func (s *spoolLocal) cerrar() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f != nil {
		_ = s.f.Close()
		s.f = nil
	}
	_ = os.Remove(s.ruta)
	s.roto = true
}
