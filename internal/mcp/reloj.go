package mcp

// reloj.go guarda, por máquina, cuánto se corrió su reloj (A133). El agente manda en cada latido
// la hora de su reloj de pared al enviarlo (`enviado_ms`); el cerebro le resta la suya al
// recibirlo, y el resultado queda acá hasta que el exportador lo publica.

import "time"

// relojVigencia es cuánto vale una medición sin un latido que la renueve.
//
// ES EL UMBRAL DE «EN LÍNEA», Y NO UN NÚMERO PROPIO: una máquina que dejó de latir figura caída a
// los 90 s, y desde ahí nadie volvió a mirar su reloj. Publicar el último desfase conocido diría
// «este reloj se corrió tanto» sobre algo que nadie confirmó, con la misma cara que una medición
// fresca. Está atado a la constante para que, si cambia el intervalo del latido, esto lo siga sin
// que nadie tenga que acordarse.
const relojVigencia = umbralEnLineaDefault

type medicionDeReloj struct {
	desfase time.Duration
	cuando  time.Time
}

// registrarReloj anota el desfase del reloj de una máquina a partir de un latido ACEPTADO.
//
// `enviadoMs` es lo que dijo el agente (0 si no dijo nada) y `llegada`, la hora del cerebro al
// entrar al handler. El desfase es enviado − llegada, CON SIGNO: positivo es un reloj ADELANTADO.
// El viaje de red lo corre hacia abajo —el latido tarda en llegar—, milisegundos por el tailnet.
//
// SIN HORA BORRA lo que hubiera: un agente viejo, un cuerpo ilegible o un 0 no son un reloj en
// hora, son un reloj que nadie midió. Conservar la medición anterior la publicaría como si fuera
// fresca, que es el mismo congelamiento que evita vidaDeRed.
//
// LA RESTA ES ENTRE time.Time Y NO ENTRE ENTEROS: `Sub` satura en ±292 años en vez de desbordar,
// y un reloj absurdo es exactamente lo que esto existe para ver. Restar los milisegundos a mano
// desborda con un `enviado_ms` extremo y da vuelta el signo.
//
// SIN ORDEN ENTRE LATIDOS, a propósito: un agente late en serie, así que dos latidos simultáneos
// de la misma máquina sólo existen con dos agentes, y eso ya tiene su propia alerta
// (`DosAgentesSobreLaMismaMaquina`). Lo peor que puede pasar es quedarse con la medición del
// latido anterior.
func (s *McpServer) registrarReloj(deviceID string, enviadoMs int64, llegada time.Time) {
	if enviadoMs == 0 {
		s.relojes.Delete(deviceID)
		return
	}
	s.relojes.Store(deviceID, medicionDeReloj{
		desfase: time.UnixMilli(enviadoMs).Sub(llegada),
		cuando:  llegada,
	})
}

// relojDe devuelve el desfase medido del reloj de una máquina, si sigue vigente.
func (s *McpServer) relojDe(deviceID string, ahora time.Time) (time.Duration, bool) {
	v, hay := s.relojes.Load(deviceID)
	if !hay {
		return 0, false
	}
	m, ok := v.(medicionDeReloj)
	if !ok || ahora.Sub(m.cuando) > relojVigencia {
		return 0, false
	}
	return m.desfase, true
}
