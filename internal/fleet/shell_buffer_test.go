package fleet

// Pruebas del buffer interactivo (S5b). Es una pieza chica donde se decide qué pasa cuando la
// máquina imprime más rápido de lo que el cliente consume, y las tres respuestas posibles llevan
// a lugares muy distintos.

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

// NO SE PIERDE UN BYTE, AUNQUE EL ESCRITOR VAYA MUCHO MÁS RÁPIDO.
//
// La alternativa tentadora es un ring buffer que descarta lo viejo: no se cae nada y la terminal
// se GARBLEA — faltan bytes en el medio de una secuencia de escape y lo que se ve deja de tener
// sentido. Es peor que un error, porque parece que funciona.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA PRIMERA VERSIÓN DE ESTA PRUEBA NO SERVÍA. Escribía rápido y leía en un bucle apretado, así
// que el buffer casi nunca llegaba a llenarse — y el camino del descarte, que es el ÚNICO que
// pierde bytes, no se ejecutaba. Con el sabotaje puesto (ring buffer) pasaba igual.
//
// El lector tiene que ser LENTO A PROPÓSITO. Es además el caso real: el cliente consume a la
// velocidad de la red y la máquina imprime a la velocidad del disco.
// ────────────────────────────────────────────────────────────────────────────────────────────
//
// Sabotaje que la hace fallar: descartar lo viejo al llenarse en vez de bloquear.
func TestElBufferNoPierdeNiUnByteAunqueElEscritorCorraMasRapido(t *testing.T) {
	b := nuevoBufferInteractivo()
	patron := []byte("0123456789abcdef")
	// Cuatro veces la capacidad: el buffer se llena varias veces y el escritor tiene que frenarse
	// varias veces. Con un ring buffer, tres cuartas partes se perderían.
	vueltas := (bufferMaxBytes * 4) / len(patron)
	quiero := vueltas * len(patron)

	listo := make(chan struct{})
	go func() {
		for i := 0; i < vueltas; i++ {
			if !b.escribir(patron) {
				break
			}
		}
		b.cerrar()
		close(listo)
	}()

	var recibido bytes.Buffer
	for {
		p, err := b.leer(2 * time.Second)
		recibido.Write(p)
		if errors.Is(err, ErrCanalCerrado) {
			break
		}
		if err != nil {
			t.Fatalf("leer: %v", err)
		}
		// EL LECTOR LENTO. Sin esta pausa el buffer no se llena y la prueba no prueba nada.
		time.Sleep(15 * time.Millisecond)
	}
	<-listo

	if recibido.Len() != quiero {
		t.Fatalf("se escribieron %d bytes y llegaron %d: se perdieron %d en el camino",
			quiero, recibido.Len(), quiero-recibido.Len())
	}
	// Y llegaron EN ORDEN: un buffer que corrompe el orden garglea igual que uno que descarta.
	got := recibido.Bytes()
	for i := 0; i+len(patron) <= len(got); i += len(patron) {
		if !bytes.Equal(got[i:i+len(patron)], patron) {
			t.Fatalf("los bytes llegaron desordenados en el offset %d", i)
		}
	}
}

// LA CONTRAPRESIÓN EXISTE DE VERDAD: con el buffer lleno, el escritor SE FRENA.
//
// Es la mitad que la prueba de arriba no distingue de un buffer sin límite — que tampoco pierde
// bytes, y se come un `cat /dev/urandom` en la RAM del cerebro hasta tumbarlo.
//
// Sabotaje que la hace fallar: quitar el `for len(b.datos) >= bufferMaxBytes` de escribir.
func TestElEscritorSeFrenaConElBufferLlenoYSiguePorSiSoloAlDrenarse(t *testing.T) {
	b := nuevoBufferInteractivo()
	bloque := make([]byte, bufferMaxBytes)

	if !b.escribir(bloque) {
		t.Fatal("la primera escritura tendría que entrar")
	}
	// La segunda encuentra el buffer lleno y tiene que quedarse esperando.
	paso := make(chan bool, 1)
	go func() { paso <- b.escribir([]byte("una gota más")) }()

	select {
	case <-paso:
		t.Fatal("el escritor NO se frenó con el buffer lleno: sin contrapresión, un `cat` grande se acumula sin techo en la memoria del cerebro")
	case <-time.After(150 * time.Millisecond):
		// bien: sigue esperando
	}

	// Al drenar, el escritor tiene que continuar SOLO. Una contrapresión que no se suelta es un
	// cuelgue, no un freno.
	if _, err := b.leer(time.Second); err != nil {
		t.Fatalf("leer: %v", err)
	}
	select {
	case ok := <-paso:
		if !ok {
			t.Fatal("el escritor volvió con error tras drenarse el buffer")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("el escritor quedó colgado tras drenarse el buffer: la contrapresión se convirtió en un cuelgue")
	}
}

// Cerrar tiene que DESPERTAR al escritor frenado. Sin esto, una sesión que vence con el buffer
// lleno deja un goroutine esperando para siempre — una fuga por cada sesión que muera así.
//
// Sabotaje que la hace fallar: quitar el `lugar.Broadcast()` de cerrar.
func TestCerrarDespiertaAlEscritorFrenadoYNoDejaFugas(t *testing.T) {
	b := nuevoBufferInteractivo()
	if !b.escribir(make([]byte, bufferMaxBytes)) {
		t.Fatal("la primera escritura tendría que entrar")
	}
	paso := make(chan bool, 1)
	go func() { paso <- b.escribir([]byte("x")) }()
	time.Sleep(80 * time.Millisecond) // que llegue a frenarse

	b.cerrar()
	select {
	case ok := <-paso:
		if ok {
			t.Error("escribir devolvió éxito sobre un buffer cerrado: esos bytes no los va a leer nadie")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("el escritor quedó colgado tras cerrar: es una goroutine filtrada por cada sesión que muera con el buffer lleno")
	}
}

// Una terminal QUIETA es su estado normal: la lectura vence sin salida y eso NO es un error.
// Confundirlo con un fallo haría que el cliente cerrara la sesión cada vez que nadie teclea.
//
// Sabotaje que la hace fallar: devolver un error al vencer la espera.
func TestUnaTerminalQuietaNoEsUnError(t *testing.T) {
	b := nuevoBufferInteractivo()
	inicio := time.Now()
	p, err := b.leer(120 * time.Millisecond)
	if err != nil {
		t.Fatalf("una terminal sin salida devolvió error: %v", err)
	}
	if len(p) != 0 {
		t.Fatalf("devolvió %d bytes de la nada", len(p))
	}
	if d := time.Since(inicio); d < 100*time.Millisecond {
		t.Errorf("volvió en %s sin esperar: sin la espera, el cliente haría un request por milisegundo", d)
	}
}

// Y la lectura devuelve TODO lo acumulado de una vez. Entregar de a tramos fijos convertiría un
// redibujado de pantalla en veinte viajes de red.
func TestLaLecturaDevuelveTodoLoAcumuladoDeUnaVez(t *testing.T) {
	b := nuevoBufferInteractivo()
	for _, s := range []string{"uno", "dos", "tres"} {
		b.escribir([]byte(s))
	}
	p, err := b.leer(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if string(p) != "unodostres" {
		t.Errorf("leer devolvió %q; esperaba las tres escrituras juntas", p)
	}
}

// Cerrado y vacío ⇒ ErrCanalCerrado, que es un final NORMAL (alguien tecleó `exit`) y no un
// fallo. Tiene nombre propio para que el cliente pueda distinguirlo de «todavía no hay salida».
func TestUnCanalCerradoSeDistingueDeUnoQuietoPeroVivo(t *testing.T) {
	b := nuevoBufferInteractivo()
	b.escribir([]byte("adiós"))
	b.cerrar()

	// Lo que quedó en el buffer se entrega IGUAL: las últimas líneas antes de que la shell
	// muriera son justamente las que se quieren ver.
	p, err := b.leer(time.Second)
	if string(p) != "adiós" {
		t.Errorf("se perdieron los bytes pendientes al cerrar: %q", p)
	}
	if err != nil && !errors.Is(err, ErrCanalCerrado) {
		t.Errorf("error inesperado: %v", err)
	}
	// Y recién con el buffer vacío se anuncia el final.
	if _, err := b.leer(time.Second); !errors.Is(err, ErrCanalCerrado) {
		t.Errorf("un canal cerrado y vacío devolvió %v; esperaba ErrCanalCerrado", err)
	}
}
