package fleet

// Pruebas del eje de AVISO (A57): lo que el agente necesita para que `avisa` entregue y `pide`
// pueda preguntar.

import (
	"strings"
	"testing"
	"time"
)

// UNA SOLA DE LAS CUATRO RESPUESTAS ABRE LA PUERTA, y se escribe como lista blanca.
//
// Con la forma negativa —`!= negada`— una respuesta nueva que alguien agregue mañana concedería
// acceso POR OMISIÓN, que es exactamente al revés de como este eje tiene que fallar. Y hay dos
// respuestas que no son ni sí ni no: el plazo vencido y «no había con qué preguntar».
//
// Sabotaje que la hace fallar: implementar Concede como `r != RespuestaNegada`.
func TestSoloLaRespuestaConcedidaAbreLaPuerta(t *testing.T) {
	if !RespuestaConcedida.Concede() {
		t.Error("una respuesta concedida no abre: el eje quedaría inutilizable")
	}
	for _, r := range []RespuestaAviso{RespuestaNegada, RespuestaSinRespuesta, RespuestaNoSePudo} {
		if r.Concede() {
			t.Errorf("%q abre la puerta y no debería", r)
		}
	}
	// Y una respuesta que este binario NO conoce tampoco: es el caso que atrapa a la forma
	// negativa, y el que un agente comprometido usaría.
	if RespuestaAviso("cualquier-cosa").Concede() {
		t.Error("una respuesta desconocida abrió la puerta: con una lista negra, cualquier " +
			"agente que mande basura entra")
	}
}

// «SIN RESPUESTA» NO ES «NEGADA», aunque las dos cierren.
//
// El comportamiento es el mismo —decidido por gio: el silencio no es permiso— y el DIAGNÓSTICO no.
// Si el motivo es siempre «nadie contestó», esa máquina no debería estar en `pide`, y sin este
// estado separado nadie se enteraría nunca.
//
// Sabotaje que la hace fallar: hacer que RespuestaSinRespuesta sea un alias de RespuestaNegada.
func TestSinRespuestaSeDistingueDeUnaNegativa(t *testing.T) {
	if RespuestaSinRespuesta == RespuestaNegada {
		t.Fatal("«nadie contestó» y «me dijeron que no» son el mismo valor: el diagnóstico se " +
			"pierde, y con él la única forma de saber que una máquina no debería estar en `pide`")
	}
	if RespuestaSinRespuesta.Concede() || RespuestaNegada.Concede() {
		t.Error("alguna de las dos abre la puerta: las dos tienen que cerrar")
	}
	// Y «no se pudo» es una tercera cosa: no la produce ni el usuario ni el reloj sino la
	// máquina, y su arreglo es otro.
	if RespuestaNoSePudo == RespuestaSinRespuesta {
		t.Error("«no había con qué preguntar» y «nadie contestó» son el mismo valor: el primero " +
			"se arregla instalando algo, el segundo no")
	}
}

// UNA RESPUESTA QUE ESTE BINARIO NO ENTIENDE NO SE DEGRADA A «NEGADA» EN SILENCIO. La manda el
// agente, que es entrada no confiable; interpretarla como negativa parece seguro y esconde que un
// agente está hablando un protocolo que este cerebro no conoce.
//
// Sabotaje que la hace fallar: que Valida devuelva true siempre.
func TestUnaRespuestaDesconocidaNoSeInterpretaSola(t *testing.T) {
	for _, r := range []RespuestaAviso{RespuestaConcedida, RespuestaNegada, RespuestaSinRespuesta, RespuestaNoSePudo} {
		if !r.Valida() {
			t.Errorf("%q es una respuesta legítima y se rechazó", r)
		}
	}
	for _, r := range []RespuestaAviso{"", "si", "SÍ", "concedida ", "negado"} {
		if RespuestaAviso(r).Valida() {
			t.Errorf("%q se aceptó como respuesta válida", r)
		}
	}
}

// EL PLAZO ES EL QUE SE DECIDIÓ, y está en el dominio y no repartido por los llamadores.
//
// Sabotaje que la hace fallar: cambiar AvisoTimeout, o dejar que un llamador use su propio número.
func TestElPlazoDelDialogoEsElDecidido(t *testing.T) {
	if AvisoTimeout != 60*time.Second {
		t.Errorf("AvisoTimeout = %v; se decidió 60 s el 2026-08-29, con los dos costos escritos: "+
			"con 30 «no llegué al teclado» se lee como «me negaron», y con 120 un flujo de "+
			"trabajo espera dos minutos para descubrir que la respuesta es no", AvisoTimeout)
	}
	// El techo del texto también: lo que se dibuja va a la pantalla de otra persona.
	if AvisoTextoMax <= 0 || AvisoTextoMax > 1000 {
		t.Errorf("AvisoTextoMax = %d: un aviso sin techo es una ventana que tapa la pantalla y "+
			"que alguien cierra sin leer", AvisoTextoMax)
	}
}

// LA CAPACIDAD LLEVA SU MOTIVO CUANDO NO PUEDE, y eso no es cosmética.
//
// El desenlace natural de este eje sin agente es que `pide` se endurezca a `prohibido` en toda la
// flota. Si eso pasa y lo único que hay es un `false`, nadie sabe si es porque no hay escritorio,
// porque falta un paquete, o porque el agente corre como servicio — y los tres se arreglan
// distinto.
//
// Sabotaje que la hace fallar: sacarle el campo Motivo a CapacidadDeAvisar.
func TestLaCapacidadDeAvisarLlevaSuMotivo(t *testing.T) {
	c := CapacidadDeAvisar{Motivo: "el agente no está en una sesión gráfica"}
	if c.Puede {
		t.Error("una capacidad con motivo de fallo se declaró capaz")
	}
	if strings.TrimSpace(c.Motivo) == "" {
		t.Error("el motivo no sobrevive: un `prohibido` en toda la flota sin explicación no se arregla")
	}
	// Y cuando SÍ puede, dice con qué: es lo que permite distinguir una máquina con zenity de una
	// con sólo notify-send, que pueden avisar pero no preguntar.
	ok := CapacidadDeAvisar{Puede: true, Herramienta: "zenity"}
	if ok.Herramienta == "" {
		t.Error("una capacidad afirmativa no dice con qué herramienta")
	}
}
