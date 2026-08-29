package fleet

// aviso.go es la mitad del AGENTE del eje de consentimiento (A57): lo que hace falta para que
// `avisa` entregue de verdad y para que `pide` pueda preguntarle a alguien.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL EJE ESTABA COMPLETO DEL LADO DEL CEREBRO Y VACÍO DEL OTRO
//
// La política se puede fijar (`musubi_fleet_consent`), se guarda (v38), se resuelve por la más
// restrictiva y se aplica en el camino de pantalla. Lo que faltaba es que alguien, en la máquina
// destino, sepa dibujar algo. Mientras no exista:
//
//   - `puede_preguntar` es 0 para toda la flota, así que un `pide` se endurece a `prohibido`;
//   - un `avisa` abre la sesión y deja un WARN diciendo que el aviso NO se pudo entregar.
//
// Las dos cosas son honestas y ninguna alcanza.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// «SABER AVISAR» SE MIDE, NO SE SUPONE
//
// `PuedePreguntar` es una CAPACIDAD MEDIDA y no una configuración, y la diferencia es todo el
// punto: un servidor sin escritorio no tiene dónde dibujar un diálogo, y afirmarlo desde un
// archivo de configuración haría que `pide` prometa un permiso que nunca se va a pedir. El agente
// lo comprueba en SU máquina y lo reporta; el cerebro sólo lo guarda.
//
// Y se mide de las DOS mitades a la vez: que haya un escritorio Y que haya con qué dibujar. Una
// sola no alcanza — un Linux con `DISPLAY` puesto y sin `zenity` ni `notify-send` no puede
// avisarle a nadie, y decir que sí sería exactamente la promesa vacía que este eje evita.

import "time"

const (
	// AvisoTimeout es cuánto espera un `pide` la respuesta del usuario.
	//
	// DECIDIDO POR GIO EL 2026-08-29, y el número tiene los dos costos escritos: sesenta segundos
	// alcanzan para que alguien mire la pantalla y decida, y el operador que pide no queda
	// bloqueado un rato largo sin señal. Con treinta, «no llegué al teclado» se convierte en «me
	// negaron»; con ciento veinte, un flujo de trabajo espera dos minutos para descubrir que la
	// respuesta es no.
	AvisoTimeout = 60 * time.Second

	// AvisoTextoMax acota lo que se dibuja. El texto lo arma el cerebro y termina en un diálogo
	// del escritorio de otra persona: un mensaje de diez mil caracteres no es un aviso, es una
	// ventana que tapa la pantalla y que alguien va a cerrar sin leer.
	AvisoTextoMax = 300
)

// RespuestaAviso es cómo terminó una pregunta al usuario. Son CUATRO y no dos.
type RespuestaAviso string

const (
	// RespuestaConcedida y RespuestaNegada son las dos respuestas de una persona.
	RespuestaConcedida RespuestaAviso = "concedida"
	RespuestaNegada    RespuestaAviso = "negada"
	// RespuestaSinRespuesta es que venció el plazo sin que nadie contestara.
	//
	// SE NIEGA IGUAL —decidido por gio el 2026-08-29— pero NO se registra como una negativa, y
	// esa distinción es lo único que separa «me dijeron que no» de «no había nadie». El
	// comportamiento es el mismo; el diagnóstico, no: si el motivo es siempre `sin_respuesta`,
	// esa máquina no debería estar en `pide`, y sin este estado nadie se enteraría nunca.
	RespuestaSinRespuesta RespuestaAviso = "sin_respuesta"
	// RespuestaNoSePudo es que no había con qué preguntar. Distinta de las tres anteriores
	// porque no la produce el usuario ni el reloj sino la máquina, y su arreglo es otro.
	RespuestaNoSePudo RespuestaAviso = "no_se_pudo"
)

// Concede dice si esta respuesta abre la puerta. UNA SOLA de las cuatro lo hace.
//
// Se escribe como una lista blanca y no como `!= negada`: con la forma negativa, una respuesta
// nueva que alguien agregue mañana concedería acceso por omisión, que es exactamente al revés de
// como tiene que fallar este eje.
func (r RespuestaAviso) Concede() bool {
	return r == RespuestaConcedida
}

// Valida rechaza una respuesta que este binario no entiende. ENTRADA NO CONFIABLE: la manda el
// agente, y una respuesta desconocida no se puede interpretar de ninguna forma honesta — así que
// no se degrada a «negada» en silencio, se rechaza y el llamador decide.
func (r RespuestaAviso) Valida() bool {
	switch r {
	case RespuestaConcedida, RespuestaNegada, RespuestaSinRespuesta, RespuestaNoSePudo:
		return true
	}
	return false
}

// CapacidadDeAvisar es lo que el agente MIDIÓ en su máquina.
type CapacidadDeAvisar struct {
	// Puede es la respuesta corta: ¿hay dónde dibujar Y con qué?
	Puede bool
	// Herramienta es CON QUÉ (notify-send, zenity, osascript, powershell). Vacía cuando no puede.
	Herramienta string
	// Motivo dice POR QUÉ no puede, cuando no puede. Es el campo que evita el peor desenlace de
	// este eje: un `pide` endurecido a `prohibido` en toda la flota sin que nadie sepa si es
	// porque no hay escritorio, porque falta un paquete, o porque el agente corre como servicio.
	// Los tres se arreglan distinto.
	Motivo string
}
