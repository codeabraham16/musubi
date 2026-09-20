package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/guiones"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// scheduler_grafo_al_dia_test.go custodia que el grafo de código se mantenga al día SOLO: que no
// dependa de que el daemon sobreviva un intervalo entero, que un tick sin cambios no congele las
// tools ni mande el grafo entero por la red, y que el sello del commit acompañe al reindexado.

// proyectoGoSinIndexar arma un repo mínimo con un paquete en disco y NINGÚN índice.
func proyectoGoSinIndexar(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n")
	writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\nfunc Alpha() { beta() }\n\nfunc beta() {}\n")
	return dir
}

// archivosEnElGrafo cuenta los archivos que el grafo conoce.
func archivosEnElGrafo(t *testing.T, s *McpServer) int {
	t.Helper()
	fps, err := s.engine.GraphFileFingerprintsCtx(s.scopedCtx(context.Background()))
	if err != nil {
		t.Fatalf("GraphFileFingerprintsCtx: %v", err)
	}
	return len(fps)
}

// esperarIndice sondea hasta que el grafo tenga archivos o venza el plazo. Sondeo acotado y no una
// espera ciega: si el invariante se rompe, la prueba FALLA a los `plazo`, no se cuelga.
func esperarIndice(t *testing.T, s *McpServer, plazo time.Duration) bool {
	t.Helper()
	limite := time.Now().Add(plazo)
	for time.Now().Before(limite) {
		if archivosEnElGrafo(t, s) > 0 {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

// lanzarScheduler corre el cuerpo del scheduler en una goroutine y devuelve con qué pararlo. El
// cleanup espera a que la goroutine salga, así ninguna prueba deja un scheduler escribiendo en la
// base de la siguiente.
func lanzarScheduler(t *testing.T, s *McpServer, interval time.Duration, despuesDe <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	listo := make(chan struct{})
	go func() {
		defer close(listo)
		s.correrSchedulerDelGrafo(ctx, interval, despuesDe, 0)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-listo:
		case <-time.After(10 * time.Second):
			t.Error("el scheduler no salió tras cancelar el contexto")
		}
	})
}

// A1 — EL PRIMER REINDEXADO NO ESPERA UN INTERVALO ENTERO.
//
// Con el ticker pelado, el primer índice llegaba recién a las 6 h, y un daemon de sesión rara vez
// vive tanto. Con un intervalo de UNA HORA, el grafo tiene que aparecer en segundos: si aparece, fue
// la corrida de arranque, porque el tick no llegó.
func TestElGrafoSeIndexaAlArrancarSinEsperarElPrimerTick(t *testing.T) {
	s := newTestServerWithPath(t, proyectoGoSinIndexar(t))
	if archivosEnElGrafo(t, s) != 0 {
		t.Fatal("el andamio ya venía indexado: la prueba no mediría la corrida de arranque")
	}

	lanzarScheduler(t, s, time.Hour, nil)

	if !esperarIndice(t, s, 10*time.Second) {
		t.Fatal("con interval=1h el grafo no se indexó en 10 s: no hubo corrida de arranque y el primer índice esperaría el tick")
	}
}

// A2 — LA CORRIDA DE ARRANQUE ESPERA AL MANTENIMIENTO DE ARRANQUE.
//
// Los dos escriben, y el mantenimiento puede traer un VACUUM. Mientras `despuesDe` siga abierto no
// se indexa nada; al cerrarse, sí.
func TestLaCorridaDeArranqueDelGrafoEsperaAlMantenimiento(t *testing.T) {
	s := newTestServerWithPath(t, proyectoGoSinIndexar(t))
	mantenimiento := make(chan struct{})

	lanzarScheduler(t, s, time.Hour, mantenimiento)

	// 500 ms es de sobra para indexar un paquete de dos funciones (A1 lo hace en milisegundos).
	time.Sleep(500 * time.Millisecond)
	if n := archivosEnElGrafo(t, s); n != 0 {
		t.Fatalf("el grafo se indexó (%d archivos) con el mantenimiento de arranque todavía corriendo", n)
	}
	close(mantenimiento)
	if !esperarIndice(t, s, 10*time.Second) {
		t.Fatal("terminado el mantenimiento de arranque, la corrida del grafo no llegó en 10 s")
	}
}

// A3 — LA ESPERA DE ARRANQUE ES ACOTADA Y ALEATORIA.
//
// Acotada: nunca llega al tope ni a medio intervalo (si no, el primer tick le ganaría). Aleatoria:
// dos daemons arrancados juntos sobre la misma base tienen que sortear esperas DISTINTAS; una
// constante los haría chocar igual, sólo que más tarde.
func TestLaEsperaDeArranqueDelGrafoEsAcotadaYVaria(t *testing.T) {
	for _, interval := range []time.Duration{6 * time.Hour, time.Hour, 10 * time.Second} {
		tope := min(topeEsperaArranqueGrafo, interval/2)
		distintas := map[time.Duration]bool{}
		for i := 0; i < 200; i++ {
			e := esperaDeArranqueDelGrafo(interval)
			if e < 0 || e >= tope {
				t.Fatalf("interval=%v: espera %v fuera de [0, %v)", interval, e, tope)
			}
			distintas[e] = true
		}
		if len(distintas) < 2 {
			t.Errorf("interval=%v: 200 sorteos dieron %d valor distinto: dos daemons arrancarían juntos", interval, len(distintas))
		}
	}
	if e := esperaDeArranqueDelGrafo(time.Nanosecond); e != 0 {
		t.Errorf("con un intervalo sin lugar para esperar, la espera tiene que ser 0, obtuve %v", e)
	}
}

// centralQueCuentaPushes es un central de mentira que cuenta los push del grafo y registra si el
// candado del despacho estaba LIBRE en el momento en que llegó cada uno.
type centralQueCuentaPushes struct {
	pushes            atomic.Int64
	conCandado        atomic.Int64
	fallarLosPrimeros atomic.Int64 // cuántos push contesta con 500 antes de aceptar
	servidor          atomic.Pointer[McpServer]
	urlDelStub        string
}

func nuevoCentralQueCuentaPushes(t *testing.T) *centralQueCuentaPushes {
	t.Helper()
	c := &centralQueCuentaPushes{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c.pushes.Add(1)
		// TryLock responde «¿alguien lo tiene?» sin bloquearse: si el scheduler empujara con el
		// candado tomado, acá falla. Si no, se suelta enseguida.
		if s := c.servidor.Load(); s != nil {
			if s.dispatchMu.TryLock() {
				s.dispatchMu.Unlock()
			} else {
				c.conCandado.Add(1)
			}
		}
		if c.fallarLosPrimeros.Add(-1) >= 0 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"codegraph-push","result":{}}`))
	}))
	t.Cleanup(ts.Close)
	c.urlDelStub = ts.URL
	return c
}

// servidorFederado arma un server con sync + team mode contra el central de mentira.
func servidorFederado(t *testing.T, dir string, central *centralQueCuentaPushes) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	s := NewMcpServer(engine, dir, embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: true}))
	s.SetSyncClient(newTestSyncClient(t, central.urlDelStub), config.SyncConfig{BatchSize: 200})
	central.servidor.Store(s)
	return s
}

// B1 — UN TICK SIN CAMBIOS NO MANDA EL GRAFO ENTERO POR LA RED.
//
// El push es de reemplazo y lleva TODO (en Musubi, 11.415 nodos y 27.350 aristas). Antes salía en
// cada tick. Ahora: el primer tick indexa un paquete nuevo ⇒ empuja; el segundo no encuentra nada
// ⇒ no empuja; tocar un archivo ⇒ vuelve a empujar.
func TestElSchedulerSoloEmpujaElGrafoSiCambioAlgo(t *testing.T) {
	central := nuevoCentralQueCuentaPushes(t)
	dir := proyectoGoSinIndexar(t)
	s := servidorFederado(t, dir, central)

	s.reindexCodeGraphOnce(context.Background())
	if n := central.pushes.Load(); n != 1 {
		t.Fatalf("el primer tick indexó un paquete nuevo y tenía que empujar UNA vez, empujó %d", n)
	}

	s.reindexCodeGraphOnce(context.Background())
	if n := central.pushes.Load(); n != 1 {
		t.Fatalf("un tick SIN CAMBIOS empujó el grafo entero otra vez (pushes=%d, esperaba 1)", n)
	}

	writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\nfunc Alpha() { beta() }\n\nfunc beta() {}\n\nfunc Gamma() {}\n")
	s.reindexCodeGraphOnce(context.Background())
	if n := central.pushes.Load(); n != 2 {
		t.Fatalf("un archivo cambió y el tick tenía que volver a empujar (pushes=%d, esperaba 2)", n)
	}
}

// B1b — UN PUSH QUE FALLÓ SE REINTENTA AUNQUE EL CÓDIGO NO CAMBIE.
//
// Con «sólo si cambió algo» a secas, un central caído en el tick que traía el cambio dejaría al
// central con la foto vieja hasta el PRÓXIMO cambio de código, que puede tardar días. El reintento
// dura lo que tarde en salir bien, y después vuelve el silencio.
func TestElSchedulerReintentaUnPushQueFallo(t *testing.T) {
	central := nuevoCentralQueCuentaPushes(t)
	central.fallarLosPrimeros.Store(1)
	s := servidorFederado(t, proyectoGoSinIndexar(t), central)

	s.reindexCodeGraphOnce(context.Background()) // cambio + central caído
	if n := central.pushes.Load(); n != 1 {
		t.Fatalf("el primer tick tenía que intentar UN push, intentó %d", n)
	}
	s.reindexCodeGraphOnce(context.Background()) // sin cambios, pero el anterior falló
	if n := central.pushes.Load(); n != 2 {
		t.Fatalf("el push anterior falló y el tick sin cambios no lo reintentó (pushes=%d, esperaba 2)", n)
	}
	s.reindexCodeGraphOnce(context.Background()) // sin cambios y el anterior salió bien
	if n := central.pushes.Load(); n != 2 {
		t.Fatalf("el reintento ya salió bien y el tick siguiente volvió a empujar (pushes=%d, esperaba 2)", n)
	}
}

// B1c — LO QUE REFRESCÓ musubi_save_code TAMBIÉN SE FEDERA.
//
// save_code re-deriva el grafo del paquete como efecto del guardado y deja el fingerprint al día,
// así que el incremental del scheduler ve ese paquete LIMPIO. Con «empujar sólo si el incremental
// cambió algo» a secas, ese cambio no llegaba nunca al central. Lo destapó la refutación de este
// mismo cambio: el push incondicional de cada tick era lo único que lo federaba.
func TestLoQueRefrescoSaveCodeTambienSeFedera(t *testing.T) {
	central := nuevoCentralQueCuentaPushes(t)
	dir := proyectoGoSinIndexar(t)
	s := servidorFederado(t, dir, central)

	s.reindexCodeGraphOnce(context.Background())
	if n := central.pushes.Load(); n != 1 {
		t.Fatalf("el primer tick tenía que empujar una vez, empujó %d", n)
	}

	writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\nfunc Alpha() { beta() }\n\nfunc beta() {}\n\nfunc Gamma() {}\n")
	mustCall(t, s, "musubi_save_code", map[string]interface{}{"path": "pkg/a.go", "gist": "paquete de prueba con Gamma"})
	if plan, err := s.planearIncremental(context.Background()); err != nil || !plan.sinCambios() {
		t.Fatalf("save_code tenía que dejar el paquete al día (si no, la prueba no mide este caso): plan=%+v err=%v", plan, err)
	}

	s.reindexCodeGraphOnce(context.Background())
	if n := central.pushes.Load(); n != 2 {
		t.Fatalf("save_code cambió el grafo y el tick no lo federó (pushes=%d, esperaba 2): el central se queda sin Gamma", n)
	}
	s.reindexCodeGraphOnce(context.Background())
	if n := central.pushes.Load(); n != 2 {
		t.Fatalf("ya federado, el tick siguiente volvió a empujar (pushes=%d, esperaba 2)", n)
	}
}

// B2 — EL ENVÍO POR RED NO VIAJA CON EL CANDADO DEL DESPACHO.
//
// NINGÚN CANDADO DEL DESPACHO PUEDE CRUZAR UNA LLAMADA DE RED: con el central lento, cada tool del
// daemon esperaría al central. El stub mira el candado en el instante en que le llega el push.
func TestElPushDelSchedulerNoViajaConElCandadoDelDespacho(t *testing.T) {
	central := nuevoCentralQueCuentaPushes(t)
	s := servidorFederado(t, proyectoGoSinIndexar(t), central)

	s.reindexCodeGraphOnce(context.Background())

	if central.pushes.Load() == 0 {
		t.Fatal("no llegó ningún push: la prueba no ejercitó el envío")
	}
	if n := central.conCandado.Load(); n != 0 {
		t.Errorf("%d push(es) llegaron al central con dispatchMu tomado: todas las tools esperan a la red", n)
	}
}

// B0 — UN TICK SIN CAMBIOS NO TOMA EL CANDADO DEL DESPACHO.
//
// Antes la pregunta «¿cambió algo?» se hacía con el candado EXCLUSIVO: medido el 2026-09-13, una
// corrida sin cambios lo sostuvo 5,3 s. Acá una tool lectora está en vuelo (RLock tomado) y el tick
// sin cambios tiene que terminar igual: si pidiera el Lock, esperaría a que la lectora suelte.
func TestUnTickSinCambiosNoTomaElCandadoDelDespacho(t *testing.T) {
	s := newTestServerWithPath(t, proyectoGoSinIndexar(t)) // sin git: el sello no escribe nada
	s.reindexCodeGraphOnce(context.Background())           // primer tick: indexa (con candado, es lo esperado)
	if archivosEnElGrafo(t, s) == 0 {
		t.Fatal("el primer tick no indexó: la prueba no llegaría al caso sin cambios")
	}

	s.dispatchMu.RLock() // una tool lectora en vuelo
	listo := make(chan struct{})
	go func() {
		defer close(listo)
		s.reindexCodeGraphOnce(context.Background())
	}()
	select {
	case <-listo:
		s.dispatchMu.RUnlock()
	case <-time.After(5 * time.Second):
		// Se suelta ANTES de fallar, para que la goroutine termine y la prueba no deje nada colgado.
		s.dispatchMu.RUnlock()
		<-listo
		t.Fatal("un tick SIN CAMBIOS esperó a que una tool lectora soltara el candado: el tick pide dispatchMu aunque no tenga nada que escribir")
	}
}

// B3 — EL SCHEDULER TAMBIÉN SELLA EL COMMIT DEL ÍNDICE.
//
// Medido el 2026-09-13: el sello seguía en un commit de una semana antes aunque el scheduler había
// re-indexado 722 archivos dos días atrás, porque sólo la tool sellaba. Un miss repetía ese commit
// viejo como «el árbol del índice».
func TestElSchedulerSellaElCommitDelIndice(t *testing.T) {
	if !hayGit() {
		t.Fatal("sin git en el PATH: este repo se clona con git y CI lo tiene en las tres plataformas")
	}
	dir := proyectoGoSinIndexar(t)
	gitInit(t, dir)
	s := newTestServerWithPath(t, dir)

	s.reindexCodeGraphOnce(context.Background())

	quiere := commitCorto(t, dir)
	got, ok, _ := s.engine.GetMeta(memory.MetaCodegraphHead)
	if !ok || got != quiere {
		t.Fatalf("tras un tick del scheduler, codegraph_head tenía que ser %q, es %q (presente=%v)", quiere, got, ok)
	}
}

// B4 — CON DIRECTORIOS FALLIDOS EL SELLO NO AVANZA, y es la regla que comparten tool y scheduler.
//
// Hacer fallar un directorio de verdad pide permisos del sistema de archivos que salen distintos en
// cada plataforma (ver debeSellarDerivador), así que la regla se afirma directo.
func TestElSelloDelCommitNoAvanzaConFallidos(t *testing.T) {
	if !hayGit() {
		t.Fatal("sin git en el PATH: este repo se clona con git y CI lo tiene en las tres plataformas")
	}
	dir := proyectoGoSinIndexar(t)
	gitInit(t, dir)
	s := newTestServerWithPath(t, dir)

	if head := s.sellarHeadDelIndice(1, directo); head != "" {
		t.Errorf("con un directorio fallido no se sella, y selló %q", head)
	}
	if got, ok, _ := s.engine.GetMeta(memory.MetaCodegraphHead); ok && got != "" {
		t.Fatalf("con un directorio fallido codegraph_head quedó escrito: %q", got)
	}
	// La contracara, para que la prueba no pase con una función que no sella nunca.
	if head := s.sellarHeadDelIndice(0, directo); head != commitCorto(t, dir) {
		t.Errorf("sin fallidos tenía que sellar %q, selló %q", commitCorto(t, dir), head)
	}
}

func commitCorto(t *testing.T, dir string) string {
	t.Helper()
	out, err := guiones.Herramienta(t, "git", "-C", dir, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	return strings.TrimSpace(string(out))
}
