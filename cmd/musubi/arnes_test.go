package main

import (
	"strings"
	"testing"
)

// Banco de `musubi arnes`. Cada test declara UN invariante, nombrado M<n>.
//
// El invariante que sostiene la fase entera es M1: que un cero de gate y un cero de debate
// NO den el mismo veredicto. Si los diera, el comando serviría para reportar el fracaso y no
// para diagnosticarlo — y agregar la séptima pieza a algo que nadie iba a usar seguiría siendo
// una salida razonable.

// --- M1 🔴: los dos ceros son distintos ------------------------------------------------

func TestM1UnCeroDeGateNoEsLoMismoQueUnCeroDeUso(t *testing.T) {
	apagado, _ := veredictoArnes(MedicionArnes{TokensGate: 0, LlamadasDebate: 0})
	ignorado, diag := veredictoArnes(MedicionArnes{TokensGate: 4200, LlamadasDebate: 0})

	if apagado == ignorado {
		t.Fatalf("«nunca se inyectó» y «se inyectó y nadie lo siguió» piden acciones OPUESTAS —arreglar el "+
			"mecanismo contra dejar de agregarle piezas— y no pueden dar el mismo veredicto (%q)", apagado)
	}
	if apagado != "APAGADO" || ignorado != "IGNORADO" {
		t.Errorf("veredictos: apagado=%q ignorado=%q", apagado, ignorado)
	}
	// Y el caso IGNORADO tiene que traer el criterio escrito de antemano, que es lo que evita
	// explicar el cero a posteriori.
	if !strings.Contains(diag, "DEJAR DE AGREGARLE PIEZAS") {
		t.Errorf("el diagnóstico de IGNORADO tiene que decir que se pare, no que se siga; obtuve %q", diag)
	}
}

// --- M2: el diagnóstico del APAGADO manda a mirar el cableado, no el texto ---------------

func TestM2ApagadoMandaAMirarElCableado(t *testing.T) {
	_, diag := veredictoArnes(MedicionArnes{})
	// Si el bloque nunca se inyectó, reescribir la skill es trabajo perdido: no llegó a leerse.
	for _, must := range []string{"UserPromptSubmit", "MUSUBI_REVIEW_GATE", "cableado"} {
		if !strings.Contains(diag, must) {
			t.Errorf("el diagnóstico de APAGADO debe mencionar %q: el problema es de cableado y no de texto; obtuve %q", must, diag)
		}
	}
}

// --- M3: abrir sin cerrar tiene su propio veredicto --------------------------------------

func TestM3AbrirSinCerrarNoEsEncendido(t *testing.T) {
	v, diag := veredictoArnes(MedicionArnes{TokensGate: 100, LlamadasDebate: 5, Abiertos: 3, Cerrados: 0})
	if v == "ENCENDIDO" {
		t.Error("tres debates abiertos y ninguno recontado no es un arnés encendido: es un panel que nadie cerró")
	}
	if v != "A MEDIO CAMINO" || !strings.Contains(diag, "tally") {
		t.Errorf("tiene que nombrar la acción que falta (action=tally); obtuve %q / %q", v, diag)
	}
}

// --- M4: el camino sano existe ------------------------------------------------------------

func TestM4ElArnesEncendidoSeReconoce(t *testing.T) {
	v, diag := veredictoArnes(MedicionArnes{TokensGate: 5000, LlamadasDebate: 12, Abiertos: 1, Cerrados: 7})
	if v != "ENCENDIDO" {
		t.Errorf("con gate, uso y debates que cierran el veredicto es ENCENDIDO, obtuve %q", v)
	}
	// Y aun encendido, lo que sigue pendiente se dice: el K de F4 es un juicio.
	if !strings.Contains(diag, "juicio") {
		t.Errorf("aun encendido hay que declarar que el K sigue sin calibrar; obtuve %q", diag)
	}
}

// --- M5: uso sin gate es su propio caso, y no se lee como éxito ---------------------------

func TestM5UsarSinGateNoCuentaComoGateEncendido(t *testing.T) {
	v, _ := veredictoArnes(MedicionArnes{TokensGate: 0, LlamadasDebate: 9, Cerrados: 4})
	if v == "ENCENDIDO" {
		t.Error("que alguien invoque la revisión por su cuenta no significa que el gate funcione: " +
			"leerlo como éxito escondería que el mecanismo sigue sin correr")
	}
	if v != "SE USA SIN EL GATE" {
		t.Errorf("obtuve %q", v)
	}
}

// --- M6: las vueltas salen del topic, y sólo de los que siguen la convención ---------------

func TestM6LasVueltasSalenDeLosTopicsDeF4(t *testing.T) {
	topics := []string{
		"el gate de revisión · vuelta 1/3 · abiertos: seguridad#1 · previo: -",
		"el gate de revisión · vuelta 2/3 · abiertos: seguridad#1 · previo: d1",
		"¿monolito o microservicios?", // de antes de F4: no es una cadena de corrección
		"otra cosa · vuelta 3 / 3 · abiertos: - · previo: d2",
	}
	v := vueltasDeTopics(topics)
	if len(v) != 3 {
		t.Fatalf("sólo cuentan los topics con la convención «vuelta k/K»: quería 3, obtuve %d (%v)", len(v), v)
	}
	if v[0] != 1 || v[2] != 3 {
		t.Errorf("las vueltas tienen que salir ordenadas: %v", v)
	}
	// Un topic sin la convención no puede contarse como «vuelta 1»: inflaría la medición con
	// debates que no son cadenas de corrección, y el K se calibraría contra ruido.
	if len(vueltasDeTopics([]string{"¿monolito o microservicios?"})) != 0 {
		t.Error("un debate de antes de F4 no es una cadena de corrección")
	}
}

// --- M7: sin cadenas, el comando lo DICE en vez de callarse --------------------------------

func TestM7SinCadenasSeDiceQueElKSigueSiendoUnJuicio(t *testing.T) {
	s := resumenVueltas(nil)
	if !strings.Contains(s, "juicio") {
		t.Errorf("sin datos, el K=3 sigue siendo un juicio y hay que decirlo en vez de mostrar un cero mudo; obtuve %q", s)
	}
}

// --- M8: un cero se dice con letras --------------------------------------------------------

func TestM8ElCeroSeDiceConLetras(t *testing.T) {
	// El 0 es EL dato de este comando, y un 0 en una columna se lee por arriba.
	if s := conCero(0, "llamadas"); !strings.Contains(s, "CERO") {
		t.Errorf("el cero tiene que gritar; obtuve %q", s)
	}
	if s := conCero(7, "llamadas"); strings.Contains(s, "CERO") {
		t.Errorf("un número normal no; obtuve %q", s)
	}
}

// --- M9 🔴: el TERCER cero — el gate corrió y no tuvo nada que avisar ---------------------
//
// Este test existe porque el comando falló su propia prueba en producción. Corrí `musubi arnes`
// contra el repo de verdad, el árbol estaba limpio, el gate midió y no habló —comportamiento
// correcto— y el veredicto dijo APAGADO: «el problema es de cableado». Mandaba a arreglar algo
// que funcionaba.
//
// Es exactamente la confusión que este comando existe para evitar, una capa más abajo: un cero
// que sirve a la vez de valor de fallo y de valor tranquilizador. Por eso el gate ahora cuenta
// las veces que MIDIÓ, no sólo las que habló.
func TestM9ElGateQueCorrioYCalloNoEsElGateQueNuncaCorrio(t *testing.T) {
	nuncaCorrio, _ := veredictoArnes(MedicionArnes{Evaluaciones: 0, TokensGate: 0, LlamadasDebate: 0})
	corrioYCallo, diag := veredictoArnes(MedicionArnes{Evaluaciones: 42, TokensGate: 0, LlamadasDebate: 0})

	if nuncaCorrio == corrioYCallo {
		t.Fatalf("«nunca midió» y «midió 42 veces y no tuvo nada que decir» piden acciones OPUESTAS "+
			"—arreglar el cableado contra esperar— y no pueden dar el mismo veredicto (%q)", nuncaCorrio)
	}
	if nuncaCorrio != "APAGADO" {
		t.Errorf("sin una sola medición el veredicto es APAGADO, obtuve %q", nuncaCorrio)
	}
	if corrioYCallo != "EN REPOSO" {
		t.Errorf("con mediciones y sin avisos el veredicto es EN REPOSO, obtuve %q", corrioYCallo)
	}
	// Y no puede sugerir que el cableado esté mal, que es lo que hacía.
	if strings.Contains(diag, "cableado, no de texto") {
		t.Errorf("EN REPOSO no manda a revisar el cableado: funciona. Obtuve %q", diag)
	}
	if !strings.Contains(diag, "42") {
		t.Errorf("el diagnóstico tiene que decir CUÁNTAS veces corrió, o el «en reposo» no se puede auditar; obtuve %q", diag)
	}
}
