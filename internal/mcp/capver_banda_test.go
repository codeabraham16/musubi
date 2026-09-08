package mcp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"musubi/internal/buildid"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// La BANDA de capver: hasta dónde atrás este cerebro sigue atendiendo.
//
// El capver ya viajaba en el handshake MCP desde que existe `buildid`, y NADIE LO LEÍA. Es el modo
// de falla que este repo persigue —se construye y no se enciende— y estas pruebas son lo que hace
// que el dato tenga un consumidor.

// latirConCapver hace latir a una máquina declarando el contrato que habla, y devuelve la
// respuesta ya parseada.
func latirConCapver(t *testing.T, ts, token string, capver int) fleet.RespuestaLatido {
	t.Helper()
	cuerpo, err := json.Marshal(fleet.CuerpoLatido{Version: "0.137.0-prueba", Capver: capver})
	if err != nil {
		t.Fatal(err)
	}
	code, body := postCon(t, ts+fleetHeartbeatPath, token, string(cuerpo))
	if code != http.StatusOK {
		t.Fatalf("el latido con capver=%d falló: %d %s", capver, code, body)
	}
	var r fleet.RespuestaLatido
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("parsear la respuesta: %v (%s)", err, body)
	}
	return r
}

// B1 — UN CAPVER DENTRO DE LA BANDA NO GENERA RUIDO, Y QUEDA GUARDADO.
//
// Las dos mitades importan. Si avisara siempre, la nota se volvería ruido de fondo y nadie la
// leería el día que diga algo. Y si no se guardara, la pregunta «¿qué máquinas no pueden hablar mi
// protocolo?» habría que contestarla esperando el próximo latido de cada una.
func TestB1ElCapverEnBandaNoAvisaYSeGuarda(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc")

	r := latirConCapver(t, ts.URL, tok, buildid.Capver)
	if r.Protocolo != "" {
		t.Errorf("un capver DENTRO de la banda generó aviso: %q", r.Protocolo)
	}

	ds, err := s.engine.ListarDevices("casa", true)
	if err != nil {
		t.Fatalf("listar: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("esperaba 1 máquina, hay %d", len(ds))
	}
	if ds[0].Capver != buildid.Capver {
		t.Errorf("la máquina quedó con capver=%d, declaró %d", ds[0].Capver, buildid.Capver)
	}
}

// B2 — UN CAPVER FUERA DE LA BANDA SE CONTESTA, Y LA RESPUESTA DICE LA BANDA.
//
// Decir sólo «no puedo» obliga a quien administra esa máquina a ir a buscar contra qué comparar.
// La banda entera en el mensaje es la diferencia entre un rechazo y una instrucción.
func TestB2ElCapverFueraDeBandaSeContestaConLaBanda(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc")

	// Uno POR ENCIMA: un agente más nuevo que este cerebro. Es el caso que va a existir mientras
	// se despliega la malla, y el que no se puede tratar como «actualizá el agente».
	r := latirConCapver(t, ts.URL, tok, buildid.Capver+7)
	if r.Protocolo == "" {
		t.Fatal("un capver fuera de la banda no generó ningún aviso")
	}
	// La banda se DERIVA de las constantes, no se escribe a mano: si mañana CapverMin sube, esta
	// prueba sigue midiendo lo mismo en vez de quedarse pidiendo un texto que ya no existe.
	banda := fmt.Sprintf("%d..%d", buildid.CapverMin, buildid.Capver)
	if !strings.Contains(r.Protocolo, banda) {
		t.Errorf("el aviso no dice cuál es la banda (%s): %q", banda, r.Protocolo)
	}
	if !strings.Contains(r.Protocolo, fmt.Sprintf("%d", buildid.Capver+7)) {
		t.Errorf("el aviso no dice qué capver se recibió: %q", r.Protocolo)
	}
}

// B3 — «NO DECLARA» NO ES «DECLARA CERO», Y SE CONTESTA DISTINTO.
//
// Un agente anterior a esta pieza no manda el campo y llega como 0. Tratarlo igual que un capver
// numérico fuera de banda le diría a alguien que su agente «habla un contrato viejo» cuando lo que
// pasa es que no habla ninguno — y el remedio es el mismo pero el diagnóstico no, que es lo que
// alguien va a leer a las tres de la mañana.
func TestB3ElAgenteQueNoDeclaraSeDistingueDelQueDeclaraViejo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc")

	sinDeclarar := latirConCapver(t, ts.URL, tok, 0)
	if sinDeclarar.Protocolo == "" {
		t.Fatal("un agente que no declara capver no recibió aviso: un 0 no es una afirmación")
	}
	if !strings.Contains(sinDeclarar.Protocolo, "no declara") {
		t.Errorf("el aviso del que NO declara no lo dice: %q", sinDeclarar.Protocolo)
	}

	fuera := latirConCapver(t, ts.URL, tok, buildid.Capver+7)
	if fuera.Protocolo == sinDeclarar.Protocolo {
		t.Errorf("«no declara» y «declara algo fuera de banda» dan el MISMO texto: %q", fuera.Protocolo)
	}
}

// B4 — EL LATIDO NO SE RECHAZA POR EL CONTRATO.
//
// Es la decisión de diseño de la pieza, y su razón es medible: una máquina rechazada deja de
// latir, y dejar de latir se ve EXACTAMENTE IGUAL que estar apagada. El problema quedaría
// invisible justo para quien puede arreglarlo.
func TestB4UnCapverFueraDeBandaNoTiraALaMaquinaDeLaFlota(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	ts := servidorHTTP(t, s)
	tok := enrolarDePrueba(t, s, "casa", "pc")

	r := latirConCapver(t, ts.URL, tok, buildid.Capver+7)
	if !r.OK {
		t.Fatal("el latido se rechazó por el capver: la máquina desaparece de la flota y eso se lee igual que «apagada»")
	}
	if r.Device != "pc" {
		t.Errorf("la respuesta no reconoce a la máquina: %+v", r)
	}
	ds, err := s.engine.ListarDevices("casa", true)
	if err != nil || len(ds) != 1 {
		t.Fatalf("la máquina no quedó en la flota: %d (%v)", len(ds), err)
	}
	if ds[0].LastSeen.IsZero() {
		t.Error("la máquina no registró señal de vida pese a haber latido")
	}
}

// B5 — EL CAPVER NO SE REESCRIBE SI NO CAMBIÓ.
//
// El capver de una máquina cambia cuando alguien la actualiza a un build con otro contrato, o sea
// casi nunca. Un UPDATE por latido en una flota de 2000 máquinas cada 30 s es un fsync 67 veces
// por segundo para no cambiar nada — el gasto que la Ola 0 sacó del camino caliente, y que entra
// de vuelta por la puerta de atrás si esta guarda no existe.
//
// SE MIDE LA ESCRITURA, NO EL VALOR. Comparar el capver antes y después sería medir el proxy: un
// UPDATE que reasigna la fila a sí misma deja el valor idéntico y cuesta exactamente lo mismo
// —página sucia, frame de WAL y fsync— que uno que la cambia. El espía de escrituras es el que
// distingue las dos cosas.
func TestB5ElCapverNoSeReescribeSiNoCambio(t *testing.T) {
	s, ts, tok, espia := servidorConEspiaDeEscrituras(t)
	_ = s

	// Primer latido: el capver pasa de 0 a 1, así que SÍ tiene que escribirse.
	latirConCapver(t, ts.URL, tok, buildid.Capver)
	if !contiene(espia.vistas(), "ActualizarCapver") {
		t.Fatalf("el primer capver declarado no se guardó: %v", espia.vistas())
	}

	// Segundo latido idéntico: ni una escritura de capver.
	espia.resetear()
	latirConCapver(t, ts.URL, tok, buildid.Capver)
	if contiene(espia.vistas(), "ActualizarCapver") {
		t.Errorf("un latido con el MISMO capver volvió a escribir: %v — a 2000 máquinas cada 30 s "+
			"eso es un fsync 67 veces por segundo para no cambiar nada", espia.vistas())
	}

	// Y uno DISTINTO vuelve a escribir: sin esta mitad, «no escribir nunca» pasaría la anterior
	// con las mejores notas y el dato quedaría congelado en el primero que llegó.
	espia.resetear()
	latirConCapver(t, ts.URL, tok, buildid.Capver+7)
	if !contiene(espia.vistas(), "ActualizarCapver") {
		t.Errorf("un capver NUEVO no se guardó: %v", espia.vistas())
	}
}
