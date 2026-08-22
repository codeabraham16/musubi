package mcp

// Invariantes del VERTEDERO del feed local (spool.go). Cada uno está en specs/riel-local/spec.md.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func leerLineas(t *testing.T, ruta string) []LiveEvent {
	t.Helper()
	b, err := os.ReadFile(ruta)
	if err != nil {
		t.Fatalf("leer %s: %v", ruta, err)
	}
	var out []LiveEvent
	for _, l := range strings.Split(string(b), "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		var ev LiveEvent
		if err := json.Unmarshal([]byte(l), &ev); err != nil {
			t.Fatalf("línea no parseable (%v): %q", err, l)
		}
		out = append(out, ev)
	}
	return out
}

// A2 · CON VARIOS ESCRITORES A LA VEZ NO SE PIERDE NI SE MEZCLA NADA.
//
// Medido en la máquina real: hay 7 daemons stdio vivos al mismo tiempo. Cada uno escribe a SU
// archivo, así que entre procesos no hay contención — pero dentro de un proceso varias goroutines
// despachan tools a la vez, y ahí el candado es lo único que lo ordena.
//
// LO QUE ESTE TEST GUARDA Y LO QUE NO — medido, no supuesto. Guarda que no se pierda ningún
// evento y que ninguna línea salga rota. NO guarda el candado: saboteé `escribir` quitándole el
// mutex y corrí esto 20 veces, y pasó las 20, incluso forzando el truncado —tres pasos, no uno—
// en medio de ocho goroutines. Sin `-race` la carrera no se manifiesta a esta escala.
//
// Al candado lo custodia la CI, que corre con `-race`. Y eso no es una esperanza: esta misma
// mañana `-race` atrapó en CI una carrera de datos de un fixture de test que acá había pasado
// verde. Dejar escrito qué NO cubre un test es parte del test — si no, la próxima persona confía
// en una valla que no existe.
func TestSpoolAguantaEscritoresConcurrentes(t *testing.T) {
	const goroutines, cadaUna = 8, 200

	enParalelo := func(s *spoolLocal) {
		var wg sync.WaitGroup
		for g := 0; g < goroutines; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < cadaUna; i++ {
					s.escribir(LiveEvent{Tool: "musubi_recall", Outcome: "ok", Kind: KindTrabajo, Origen: "local"})
				}
			}()
		}
		wg.Wait()
	}

	t.Run("sin truncar no se pierde ninguno", func(t *testing.T) {
		dir := t.TempDir()
		s := nuevoSpool(dir, 4242, 0)
		if s == nil {
			t.Fatal("nuevoSpool devolvió nil sobre un TempDir")
		}
		defer s.cerrar()
		enParalelo(s)

		evs := leerLineas(t, filepath.Join(dir, "4242.jsonl"))
		if len(evs) != goroutines*cadaUna {
			t.Fatalf("se escribieron %d eventos, se leyeron %d", goroutines*cadaUna, len(evs))
		}
	})

	t.Run("truncando a mitad de la concurrencia no se rompe ninguna linea", func(t *testing.T) {
		dir := t.TempDir()
		// Tope chico a propósito: obliga a truncar decenas de veces MIENTRAS ocho goroutines
		// escriben. Es el único momento en que el spool hace más de una operación por evento.
		s := nuevoSpool(dir, 4243, 2048)
		if s == nil {
			t.Fatal("nuevoSpool devolvió nil")
		}
		defer s.cerrar()
		enParalelo(s)

		// leerLineas falla el test ante cualquier línea que no parsee: eso ES la aserción.
		evs := leerLineas(t, filepath.Join(dir, "4243.jsonl"))
		if len(evs) == 0 {
			t.Fatal("no quedó ningún evento legible")
		}
		for i, ev := range evs {
			if ev.Tool != "musubi_recall" || ev.Origen != "local" {
				t.Fatalf("evento %d salió corrupto: %+v", i, ev)
			}
		}
	})
}

// A5 · UN SPOOL QUE FALLA NO HACE FALLAR LA TOOL.
//
// `escribir` corre en el camino de salida de TODA tool. Si pudiera romper, una tool empezaría a
// fallar porque su telemetría falla — que es peor que no tener telemetría. Se le cierra el archivo
// por debajo, que es lo que pasa con un disco lleno o un directorio que alguien borró.
func TestSpoolRotoNoRompeAlLlamador(t *testing.T) {
	dir := t.TempDir()
	s := nuevoSpool(dir, 7, 0)
	if s == nil {
		t.Fatal("nuevoSpool devolvió nil")
	}
	s.escribir(LiveEvent{Tool: "antes"})

	// El sabotaje que el mundo real hace solo: el descriptor deja de servir.
	s.mu.Lock()
	_ = s.f.Close()
	s.mu.Unlock()

	// Ninguna de estas puede entrar en pánico ni bloquear.
	for i := 0; i < 50; i++ {
		s.escribir(LiveEvent{Tool: "despues"})
	}
	s.cerrar()
	s.escribir(LiveEvent{Tool: "despues-de-cerrar"}) // sobre un spool cerrado tampoco

	var nilSpool *spoolLocal
	nilSpool.escribir(LiveEvent{Tool: "sobre-nil"}) // ni sobre uno nil
	nilSpool.cerrar()
}

// A6 · EL SPOOL NO CRECE SIN FIN.
//
// Un feed no necesita historia —para eso está el ledger—, y en esta máquina el sondeo de un día
// (109.687 invocaciones) escribiría ~18 MB por daemon, con siete daemons vivos. El tope se
// comprueba ANTES de escribir, así que el archivo nunca lo supera: si se comprobara después, lo
// superaría en cada vuelta y volvería.
func TestSpoolNoCreceSinFin(t *testing.T) {
	dir := t.TempDir()
	const tope = 4096
	s := nuevoSpool(dir, 99, tope)
	if s == nil {
		t.Fatal("nuevoSpool devolvió nil")
	}
	defer s.cerrar()

	ruta := filepath.Join(dir, "99.jsonl")
	mayor := int64(0)
	for i := 0; i < 3000; i++ {
		s.escribir(LiveEvent{Tool: "musubi_sync_status", Outcome: "ok", Kind: KindSondeo, Origen: "local"})
		if fi, err := os.Stat(ruta); err == nil && fi.Size() > mayor {
			mayor = fi.Size()
		}
	}
	if mayor > tope {
		t.Fatalf("el archivo llegó a %d bytes con un tope de %d", mayor, tope)
	}
	// Y sigue sirviendo después de truncar: lo último escrito tiene que estar.
	if evs := leerLineas(t, ruta); len(evs) == 0 {
		t.Fatal("después de truncar el archivo quedó vacío: el feed dejaría de mostrar el presente")
	}
}

// A7 · UN DAEMON QUE SALE LIMPIO NO DEJA BASURA.
//
// La otra mitad —morir de golpe— la cubre la poda del lector, porque acá no hay nada que hacer.
func TestSpoolSeBorraAlCerrar(t *testing.T) {
	dir := t.TempDir()
	s := nuevoSpool(dir, 1234, 0)
	if s == nil {
		t.Fatal("nuevoSpool devolvió nil")
	}
	ruta := filepath.Join(dir, "1234.jsonl")
	s.escribir(LiveEvent{Tool: "musubi_doctor"})
	if _, err := os.Stat(ruta); err != nil {
		t.Fatalf("el archivo tendría que existir mientras el daemon vive: %v", err)
	}
	s.cerrar()
	if _, err := os.Stat(ruta); !os.IsNotExist(err) {
		t.Fatalf("cerrar() dejó el archivo en pie (err=%v)", err)
	}
}
