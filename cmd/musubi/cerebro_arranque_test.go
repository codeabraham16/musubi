package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Invariantes del spec «El canal arranca o falla rápido» (specs/cerebro-arranca-o-falla-rapido/).
//
// EL INCIDENTE QUE LOS ORIGINA: con el cerebro central caído por un corte de luz, a otra máquina del
// equipo NO le arrancaba Claude Code entero. Cronometrado: initialize + notifications/initialized +
// tools/list, a ~21 s cada una (el timeout de conexión del SO), daban 63 s contra los 60 s que el
// host da para inicializar un MCP server.

// ipQueNoResponde es una dirección de TEST-NET-2 (RFC 5737): existe para documentación y NADIE la
// rutea, así que un intento de conexión se queda esperando — que es exactamente el escenario del
// central apagado. No se usa un puerto cerrado de localhost porque ahí el SO contesta RST al
// instante y el test mediría otra cosa.
const ipQueNoResponde = "198.51.100.1:7717"

// C1 — EL DIAL TIENE SU PROPIO TIMEOUT, Y MANDA.
//
// Es el corazón del arreglo: sin esto el tiempo de un intento fallido lo elige el sistema operativo
// (~21 s en Windows) y nadie lo pactó.
func TestC1ElDialTieneSuPropioTimeout(t *testing.T) {
	// timeout de request GRANDE, dial CHICO: tiene que ganar el chico.
	client := clienteCerebro(60, 1)

	arranque := time.Now()
	_, err := forward(client, "http://"+ipQueNoResponde+"/mcp", "tok", []byte(`{"jsonrpc":"2.0","id":1}`))
	tardo := time.Since(arranque)

	if err == nil {
		t.Fatal("una IP que no rutea no puede devolver éxito")
	}
	// 6 s es holgado para 1 s de dial y sigue MUY lejos de los 21 s del SO: si el dial no mandara,
	// esto no entraría ni de casualidad.
	if tardo > 6*time.Second {
		t.Errorf("el intento tardó %v: el timeout de dial no está mandando (el del SO son ~21 s)", tardo.Round(time.Millisecond))
	}
}

// C2 — EL ARRANQUE COMPLETO ENTRA EN EL PRESUPUESTO DEL HOST.
//
// No alcanza con que UNA request sea rápida: el incidente lo produjo la SUMA de las tres del
// arranque. Esta prueba mide la secuencia entera contra el número real que dio el host: 60 s.
func TestC2ElArranqueEntraEnElPresupuesto(t *testing.T) {
	const presupuestoHost = 60 * time.Second
	client := clienteCerebro(60, defaultDialTimeoutSeg)
	endpoint := "http://" + ipQueNoResponde + "/mcp"

	arranque := time.Now()
	for _, payload := range [][]byte{
		[]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`),
		[]byte(`{"jsonrpc":"2.0","method":"notifications/initialized"}`),
		[]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`),
	} {
		if _, err := forward(client, endpoint, "tok", payload); err == nil {
			t.Fatal("con el central caído ninguna request puede tener éxito")
		}
	}
	tardo := time.Since(arranque)

	// La mitad del presupuesto: margen de sobra para un runner lento, y aun así imposible de pasar
	// con los ~63 s que daba el comportamiento viejo.
	if tardo > presupuestoHost/2 {
		t.Errorf("el arranque tardó %v de un presupuesto de %v: con el central caído la sesión entera no levanta",
			tardo.Round(time.Millisecond), presupuestoHost)
	}
	t.Logf("arranque con el central caído: %v (presupuesto del host: %v)", tardo.Round(time.Millisecond), presupuestoHost)
}

// C3 — UN CENTRAL VIVO PERO LENTO NO SE CORTA POR EL DIAL.
//
// El dial corto acota cuánto se espera para saber si el otro extremo ESTÁ; una vez conectado, el que
// manda es el timeout de REQUEST. Confundirlos rompería a un central sano que tarda en responder un
// tools/list grande.
func TestC3UnCentralLentoNoSeCortaPorElDial(t *testing.T) {
	lento := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1500 * time.Millisecond) // más que el dial, menos que el request
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer lento.Close()

	client := clienteCerebro(10, 1) // dial 1 s, request 10 s
	resp, err := forward(client, lento.URL+"/mcp", "tok", []byte(`{"jsonrpc":"2.0","id":1}`))
	if err != nil {
		t.Fatalf("un central vivo que tarda 1,5 s NO puede cortarse por un dial de 1 s: %v", err)
	}
	if len(resp) == 0 {
		t.Error("respuesta vacía")
	}
}

// C4 — UN DIAL <= 0 CAE AL DEFAULT, NO A «SIN LÍMITE».
//
// `net.Dialer{Timeout: 0}` significa SIN TIMEOUT, o sea justo el comportamiento que causó el
// incidente. Un cero tiene que ser el default seguro y no la puerta de atrás que lo reintroduce.
func TestC4DialCeroCaeAlDefaultNoASinLimite(t *testing.T) {
	for _, v := range []int{0, -1} {
		client := clienteCerebro(60, v)
		tr, ok := client.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("esperaba un *http.Transport, hay %T", client.Transport)
		}
		// Se verifica el EFECTO —cuánto tarda en rendirse— y no el campo: el campo se puede leer bien
		// y estar cableado a otro dialer.
		arranque := time.Now()
		_, err := tr.DialContext(t.Context(), "tcp", ipQueNoResponde)
		tardo := time.Since(arranque)
		if err == nil {
			t.Fatal("una IP que no rutea no puede conectar")
		}
		if tardo > time.Duration(defaultDialTimeoutSeg+3)*time.Second {
			t.Errorf("con dial=%d tardó %v: un cero se convirtió en «sin límite»", v, tardo.Round(time.Millisecond))
		}
	}
}

// C5 — EL TIMEOUT DE REQUEST SIGUE EXISTIENDO. Separar los dos no puede haber borrado el que ya
// estaba: un central que acepta la conexión y después se queda mudo tiene que cortarse igual.
func TestC5ElTimeoutDeRequestSigueVivo(t *testing.T) {
	mudo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer mudo.Close()
	go func() {
		for {
			c, err := mudo.Accept()
			if err != nil {
				return
			}
			_ = c // acepta y nunca contesta
		}
	}()

	client := clienteCerebro(1, 5) // request 1 s, dial 5 s: acá manda el de request
	arranque := time.Now()
	_, err = forward(client, "http://"+mudo.Addr().String()+"/mcp", "tok", []byte(`{"jsonrpc":"2.0","id":1}`))
	tardo := time.Since(arranque)
	if err == nil {
		t.Fatal("un central mudo no puede devolver éxito")
	}
	if tardo > 5*time.Second {
		t.Errorf("tardó %v: el timeout de request de 1 s no cortó", tardo.Round(time.Millisecond))
	}
}
