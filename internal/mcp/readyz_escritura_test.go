package mcp

// LA SONDA MIDE LA ESCRITURA (S1–S6).
//
// EL DEFECTO QUE ESTO FIJA. `/readyz` sondeaba con `GetMeta`, una lectura. El 2026-08-23 el cerebro
// central pasó ONCE HORAS sin poder guardar nada —`save_observation` colgaba 150 s, `memory_expand`
// y `token_list` 30 s— y `/readyz` devolvió 200 en 0,1 s todo ese tiempo, porque las lecturas
// andaban perfecto. La falla no se detectó por la sonda: se detectó porque alguien intentó guardar.
//
// Es el mismo patrón que el canario de `bench-scale`, del otro lado del espejo: allá un rojo
// permanente dejó de informar, acá un verde permanente nunca informó. La sonda tiene que medir LO
// QUE PUEDE FALLAR, y para un cerebro lo que puede fallar —y lo que importa— es aceptar memoria.

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"musubi/internal/embedding"
)

// relojFijo devuelve un `ahora` controlable, para medir esperas sin dormirlas.
func relojFijo(t *time.Time, mu *sync.Mutex) func() time.Time {
	return func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return *t
	}
}

// S1 — LA ESCRITURA QUE ANDA REPORTA LISTO.
func TestS1EscrituraSanaReportaListo(t *testing.T) {
	var sd sondaDeEscritura
	llamadas := 0
	ok, detalle := sd.verificar(func() error { llamadas++; return nil }, time.Second, time.Now)
	if !ok {
		t.Fatalf("una escritura sana debería reportar listo, y dijo: %s", detalle)
	}
	if llamadas != 1 {
		t.Fatalf("la sonda llamó %d veces a la escritura, esperaba 1", llamadas)
	}
}

// S2 — LA ESCRITURA QUE FALLA NO SE DISFRAZA DE SANA, y el motivo viaja en la respuesta.
//
// Que el detalle llegue no es cosmético: un 503 sin motivo obliga a reproducir el incidente para
// saber qué pasó, y estos incidentes se investigan DESPUÉS, cuando ya se reinició.
func TestS2EscrituraQueFallaSeDeclara(t *testing.T) {
	var sd sondaDeEscritura
	ok, detalle := sd.verificar(func() error { return errors.New("attempt to write a readonly database") }, time.Second, time.Now)
	if ok {
		t.Fatal("una escritura que devuelve error no puede reportar listo")
	}
	if !strings.Contains(detalle, "readonly database") {
		t.Fatalf("el detalle no dice qué falló: %q", detalle)
	}
}

// S3 — LA ESCRITURA QUE CUELGA. Éste es el incidente.
//
// No falla: no vuelve. Con el sondeo por lectura, esto era invisible; la sonda respondía 200 en
// 0,1 s mientras nadie podía guardar nada.
func TestS3EscrituraColgadaNoReportaListo(t *testing.T) {
	var sd sondaDeEscritura
	soltar := make(chan struct{})
	defer close(soltar)

	inicio := time.Now()
	ok, detalle := sd.verificar(func() error { <-soltar; return nil }, 80*time.Millisecond, time.Now)
	if ok {
		t.Fatal("una escritura que NO VUELVE reportó listo: la sonda está midiendo otra cosa (así se perdieron 11 h el 2026-08-23)")
	}
	if !strings.Contains(detalle, "no respondió") {
		t.Fatalf("el detalle no distingue un cuelgue de un error: %q", detalle)
	}
	// Y la sonda tiene que CORTAR, no colgarse con la escritura: si esperara para siempre, el 503
	// nunca llegaría y el cuelgue seguiría siendo invisible, sólo que un escalón más arriba.
	if d := time.Since(inicio); d > 2*time.Second {
		t.Fatalf("la sonda tardó %v: se colgó junto con la escritura en vez de acotarla", d)
	}
}

// S4 — UNA COLGADA NO SE MULTIPLICA POR CADA PEDIDO.
//
// Un monitor que pregunta cada 15 s haría 2.640 sondeos en once horas. Si cada uno lanzara su
// goroutine contra una base que no responde, la sonda pasaría de diagnosticar el problema a
// agravarlo — y sería un modo de falla NUEVO introducido por el arreglo.
func TestS4LaColgadaNoSeMultiplica(t *testing.T) {
	var sd sondaDeEscritura
	soltar := make(chan struct{})
	defer close(soltar)

	var mu sync.Mutex
	llamadas := 0
	escribir := func() error {
		mu.Lock()
		llamadas++
		mu.Unlock()
		<-soltar
		return nil
	}

	ahora := time.Now()
	var relojMu sync.Mutex
	reloj := relojFijo(&ahora, &relojMu)

	if ok, _ := sd.verificar(escribir, 50*time.Millisecond, reloj); ok {
		t.Fatal("el primer sondeo debería reportar no listo")
	}
	// El reloj avanza más allá del tope: los sondeos siguientes ven la colgada y responden solos.
	relojMu.Lock()
	ahora = ahora.Add(time.Minute)
	relojMu.Unlock()

	for i := 0; i < 20; i++ {
		ok, detalle := sd.verificar(escribir, 50*time.Millisecond, reloj)
		if ok {
			t.Fatalf("sondeo %d reportó listo con una escritura todavía colgada", i)
		}
		if !strings.Contains(detalle, "sin responder desde hace") {
			t.Fatalf("sondeo %d no reconoció la colgada previa: %q", i, detalle)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if llamadas != 1 {
		t.Fatalf("se lanzaron %d escrituras contra una base que no responde; tiene que ser 1", llamadas)
	}
}

// S5 — Y CUANDO LA COLGADA TERMINA, EL NODO VUELVE SOLO.
//
// Sin este caso, «arreglar» la sonda dejándola pegada en no-listo para siempre pasaría S3 y S4 con
// las mejores notas — y obligaría a reiniciar para volver a verde, que es exactamente el ritual que
// esto viene a evitar.
func TestS5CuandoLaEscrituraVuelveElNodoVuelve(t *testing.T) {
	var sd sondaDeEscritura
	soltar := make(chan struct{})

	ahora := time.Now()
	var relojMu sync.Mutex
	reloj := relojFijo(&ahora, &relojMu)
	const tope = 60 * time.Millisecond

	if ok, _ := sd.verificar(func() error { <-soltar; return nil }, tope, reloj); ok {
		t.Fatal("el primer sondeo debería reportar no listo")
	}
	close(soltar) // la base se destraba

	// EL RELOJ SE ADELANTA MÁS ALLÁ DEL TOPE, y no es un detalle: sin esto el caso se aprueba solo.
	// Con el reloj quieto, un sondeo que ve `enVuelo` todavía dentro del tope devuelve «lo último
	// medido» —que es un éxito, porque la escritura terminó bien— y el verde no diría nada sobre si
	// el estado se limpió. Pasado el tope ese atajo desaparece: la única forma de volver a listo es
	// que `enVuelo` se haya limpiado de verdad. (Se descubrió saboteando: quitar la limpieza dejaba
	// este caso en VERDE.)
	relojMu.Lock()
	ahora = ahora.Add(time.Minute)
	relojMu.Unlock()

	plazo := time.Now().Add(3 * time.Second)
	for {
		ok, detalle := sd.verificar(func() error { return nil }, tope, reloj)
		if ok {
			return
		}
		if time.Now().After(plazo) {
			t.Fatalf("la sonda quedó pegada en no-listo después de que la escritura se destrabó (%q): "+
				"volver a verde exigiría reiniciar, que es justo el ritual que esto viene a evitar", detalle)
		}
	}
}

// S6 — Y /readyz ESCRIBE DE VERDAD, no simula.
//
// S1–S5 miden la mecánica de la sonda con una función inyectada. Este caso cierra el hueco obvio:
// que el handler llame a una escritura de verdad contra la base. Se comprueba por el efecto —la
// fila de `meta` cambia entre dos llamadas—, no por el 200, que es justamente lo que mentía.
func TestS6ReadyzEscribeDeVerdad(t *testing.T) {
	// El servidor se arma acá en vez de usar newHTTPTestServer para quedarse con el engine: la
	// verificación es por EFECTO sobre la base, no por el código de estado — que es precisamente
	// lo que mentía.
	s := newTestServer(t, embedding.NoopProvider{})
	ts := httptest.NewServer(s.HTTPHandler(httpOptions{reqTimeout: 10 * time.Second, loopbackOnly: true}))
	defer ts.Close()

	sondear := func() {
		resp, err := http.Get(ts.URL + "/readyz")
		if err != nil {
			t.Fatalf("GET /readyz: %v", err)
		}
		cuerpo, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("readyz: status=%d cuerpo=%q", resp.StatusCode, cuerpo)
		}
	}

	sondear()
	primera, ok, err := s.engine.GetMeta(claveSondaReadyz)
	if err != nil {
		t.Fatalf("leer la marca de sondeo: %v", err)
	}
	if !ok {
		t.Fatal("/readyz devolvió 200 y no dejó ninguna marca de escritura: está sondeando con una lectura")
	}

	// El valor que escribe la sonda es un instante, así que dos sondeos separados dejan valores
	// distintos. Si el handler dejara de escribir, la marca se quedaría congelada.
	time.Sleep(2 * time.Millisecond)
	sondear()
	segunda, _, _ := s.engine.GetMeta(claveSondaReadyz)
	if primera == segunda {
		t.Fatalf("la marca de sondeo no cambió entre dos llamadas a /readyz (%q): el handler no está escribiendo", primera)
	}
}
