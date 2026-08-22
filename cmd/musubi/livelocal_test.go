package main

// Invariantes del LECTOR del riel local (livelocal.go). Ver specs/riel-local/spec.md.
//
// Los archivos se escriben a mano en vez de usar el spool de internal/mcp: así se prueba el
// CONTRATO —una línea JSON por evento— y no la implementación del escritor. Si algún día el
// escritor cambia de forma, este test es el que tiene que gritar.

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"musubi/internal/mcp"
)

func escribirSpool(t *testing.T, dir string, pid int, evs ...mcp.LiveEvent) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, ev := range evs {
		b, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		sb.Write(b)
		sb.WriteByte('\n')
	}
	ruta := filepath.Join(dir, strconv.Itoa(pid)+".jsonl")
	f, err := os.OpenFile(ruta, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.WriteString(sb.String()); err != nil {
		t.Fatal(err)
	}
}

func tools(lineas [][]byte) []string {
	var out []string
	for _, l := range lineas {
		var ev mcp.LiveEvent
		if json.Unmarshal(l, &ev) == nil {
			out = append(out, ev.Tool)
		}
	}
	return out
}

// A1 · UN EVENTO LOCAL LLEGA SIN PASAR POR EL LEDGER, Y CON SU HORA REAL.
//
// Es la razón de ser de todo esto. El ledger vuelca cada 10 s y estampa la hora del INSERT con
// resolución de 1 segundo: dos tools separadas por 20 ms le salen simultáneas. Acá tienen que
// llegar con marcas distintas, porque la hora la pone quien ejecuta, no quien guarda.
func TestLectorEntregaEventosConSuHoraReal(t *testing.T) {
	dir := t.TempDir()
	l := nuevoLectorSpool(dir)

	t0 := time.Date(2026, 8, 22, 15, 0, 0, 0, time.UTC)
	escribirSpool(t, dir, 100,
		mcp.LiveEvent{Tool: "musubi_recall", At: t0.Format("2006-01-02T15:04:05.000Z07:00"), Origen: "local"},
		mcp.LiveEvent{Tool: "musubi_save_observation", At: t0.Add(20 * time.Millisecond).Format("2006-01-02T15:04:05.000Z07:00"), Origen: "local"},
	)

	got := l.leerNuevos()
	if len(got) != 2 {
		t.Fatalf("se escribieron 2 eventos, el lector entregó %d", len(got))
	}
	var a, b mcp.LiveEvent
	_ = json.Unmarshal(got[0], &a)
	_ = json.Unmarshal(got[1], &b)
	if a.At == b.At {
		t.Fatalf("los dos eventos llegaron con la MISMA marca (%s): eso es lo que hace el ledger y "+
			"es justo lo que este camino existe para evitar", a.At)
	}
	if a.Origen != "local" {
		t.Fatalf("el evento perdió su procedencia: %+v", a)
	}

	// Segunda pasada sin escrituras nuevas: no puede repetir nada.
	if again := l.leerNuevos(); len(again) != 0 {
		t.Fatalf("el lector repitió %d eventos ya entregados", len(again))
	}
}

// A3 · EL PANEL QUE ARRANCA TARDE VE LO QUE YA ESTABA.
//
// El daemon vive antes que el panel — de hecho es lo normal: los daemons los levanta el agente al
// abrir el proyecto y el panel se abre cuando a alguien se le ocurre. Un lector que empezara en el
// final del archivo mostraría una pantalla vacía sobre una máquina que estuvo trabajando.
func TestLectorQueArrancaTardeVeLoAnterior(t *testing.T) {
	dir := t.TempDir()
	escribirSpool(t, dir, 200, mcp.LiveEvent{Tool: "musubi_doctor", Origen: "local"})

	l := nuevoLectorSpool(dir) // recién ahora
	if got := tools(l.leerNuevos()); len(got) != 1 || got[0] != "musubi_doctor" {
		t.Fatalf("el lector no vio lo anterior a su arranque: %v", got)
	}
}

// EL ESCRITOR TRUNCA AL LLEGAR A SU TOPE (spool.go, A6) — y el lector tiene que darse cuenta.
//
// Sin esto, un daemon deja de verse justo cuando MÁS escribe, que es el peor momento posible: el
// offset del lector queda más allá del fin del archivo nuevo y no vuelve a leer nada nunca.
func TestLectorSeRecuperaDeUnTruncado(t *testing.T) {
	dir := t.TempDir()
	l := nuevoLectorSpool(dir)
	ruta := filepath.Join(dir, "300.jsonl")

	escribirSpool(t, dir, 300, mcp.LiveEvent{Tool: "antes-del-truncado", Origen: "local"})
	if got := tools(l.leerNuevos()); len(got) != 1 {
		t.Fatalf("primera pasada: %v", got)
	}

	if err := os.Truncate(ruta, 0); err != nil { // el escritor llegó a su tope
		t.Fatal(err)
	}
	escribirSpool(t, dir, 300, mcp.LiveEvent{Tool: "despues-del-truncado", Origen: "local"})

	got := tools(l.leerNuevos())
	if len(got) != 1 || got[0] != "despues-del-truncado" {
		t.Fatalf("tras el truncado el lector entregó %v; se quedó mirando un offset que ya no existe", got)
	}
}

// UNA LÍNEA A MEDIO ESCRIBIR NO SE ENTREGA.
//
// El escritor puede quedar a mitad de una línea justo cuando el lector pasa. Media línea no rompe
// nada ruidosamente: el navegador la descarta en silencio, que es la peor forma de perder un
// evento. Se deja para la próxima pasada, cuando esté completa.
func TestLectorNoEntregaLineasAMedias(t *testing.T) {
	dir := t.TempDir()
	l := nuevoLectorSpool(dir)
	ruta := filepath.Join(dir, "400.jsonl")

	escribirSpool(t, dir, 400, mcp.LiveEvent{Tool: "completa", Origen: "local"})
	f, err := os.OpenFile(ruta, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"tool":"a-mit`); err != nil { // sin cerrar ni salto de línea
		t.Fatal(err)
	}
	_ = f.Close()

	if got := tools(l.leerNuevos()); len(got) != 1 || got[0] != "completa" {
		t.Fatalf("entregó una línea incompleta: %v", got)
	}

	// Y cuando se completa, aparece — no se perdió por haberla visto a medias.
	f, _ = os.OpenFile(ruta, os.O_WRONLY|os.O_APPEND, 0o644)
	_, _ = f.WriteString("ades\"}\n")
	_ = f.Close()
	if got := tools(l.leerNuevos()); len(got) != 1 || got[0] != "a-mitades" {
		t.Fatalf("la línea completada nunca llegó: %v", got)
	}
}

// A7 · SE PODA LO DE LOS MUERTOS, Y SÓLO ESO.
//
// Sin poda, cada daemon que muere de golpe deja un archivo que el panel relee para siempre — que
// es exactamente la forma del bug de los `bridge -watch` huérfanos que medimos el mismo día que se
// escribió esto. Y con una poda demasiado suelta pasa lo contrario: se borra el archivo de un
// daemon VIVO y su trabajo deja de verse. Por eso hacen falta las dos condiciones.
func TestPodarSacaLosMuertosYNoLosVivos(t *testing.T) {
	dir := t.TempDir()
	l := nuevoLectorSpool(dir)

	const pidMuerto = 999999 // fuera del rango de PID de cualquier sistema razonable
	pidVivo := os.Getpid()
	escribirSpool(t, dir, pidMuerto, mcp.LiveEvent{Tool: "de-un-muerto"})
	escribirSpool(t, dir, pidVivo, mcp.LiveEvent{Tool: "de-uno-vivo"})
	// Un archivo que no es nuestro: no se toca aunque esté viejo.
	ajeno := filepath.Join(dir, "notas.txt")
	if err := os.WriteFile(ajeno, []byte("hola"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Recién escritos: la gracia los protege a TODOS, incluso al del muerto.
	if n := l.podar(time.Now()); n != 0 {
		t.Fatalf("podó %d archivos recién escritos; la gracia existe para que un PID reciclado no "+
			"borre el archivo de un daemon vivo", n)
	}

	// Pasada la gracia, cae el del muerto y sólo ése.
	if n := l.podar(time.Now().Add(2 * graciaPoda)); n != 1 {
		t.Fatalf("podó %d archivos, se esperaba 1 (el del PID muerto)", n)
	}
	if _, err := os.Stat(filepath.Join(dir, strconv.Itoa(pidMuerto)+".jsonl")); !os.IsNotExist(err) {
		t.Fatal("el archivo del proceso muerto sigue ahí")
	}
	if _, err := os.Stat(filepath.Join(dir, strconv.Itoa(pidVivo)+".jsonl")); err != nil {
		t.Fatalf("se podó el archivo de un proceso VIVO: %v", err)
	}
	if _, err := os.Stat(ajeno); err != nil {
		t.Fatalf("se borró un archivo que no es nuestro: %v", err)
	}
}

// El lector no puede romperse por lo que no está: sin directorio, sin permisos, sin nada.
func TestLectorToleraLoQueNoExiste(t *testing.T) {
	l := nuevoLectorSpool(filepath.Join(t.TempDir(), "no-existe"))
	if got := l.leerNuevos(); got != nil {
		t.Fatalf("sobre un directorio inexistente devolvió %v", got)
	}
	if n := l.podar(time.Now()); n != 0 {
		t.Fatalf("podó %d sobre un directorio inexistente", n)
	}
	if nuevoLectorSpool("") != nil {
		t.Fatal("sin directorio no debería haber lector")
	}
	var nada *lectorSpool
	_ = nada.leerNuevos()
	_ = nada.podar(time.Now())
}

// El bombeo publica en el riel de verdad, y TERMINA cuando se lo para.
//
// Lo segundo importa: cada panel arranca esta goroutine, y una que no muere deja un proceso
// leyendo disco para siempre. Es la misma familia de bug que los `-watch` huerfanos, y no
// vamos a construir su gemelo el mismo dia que lo limpiamos.
func TestSeguirSpoolPublicaEnElRielYPara(t *testing.T) {
	dir := t.TempDir()
	escribirSpool(t, dir, 500, mcp.LiveEvent{Tool: "musubi_judge", Origen: "local"})

	r := nuevoRelay("", "") // sin central: el riel local tiene que andar igual
	ts := httptest.NewServer(r.handlerStream())
	defer ts.Close()

	stop := make(chan struct{})
	murio := make(chan struct{})
	go func() { seguirSpoolLocal(stop, nuevoLectorSpool(dir), r); close(murio) }()

	todo := strings.Join(leerFrames(t, ts.URL, 3, 4*time.Second), "\n")
	if !strings.Contains(todo, "musubi_judge") || !strings.Contains(todo, `"origen":"local"`) {
		close(stop)
		t.Fatalf("el evento local no llego al riel; llego:\n%s", todo)
	}

	close(stop)
	select {
	case <-murio:
	case <-time.After(3 * time.Second):
		t.Fatal("seguirSpoolLocal no termino al pararlo: quedaria leyendo disco para siempre")
	}
}
