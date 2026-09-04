package mcp

import (
	"time"
)

// ════════════════════════════════════════════════════════════════════════════════════════════
// EL TOKEN NUEVO DE UNA ROTACIÓN VIVE EN MEMORIA, NUNCA EN LA BASE
//
// Es la decisión central de este tramo, y sale de un choque entre dos cosas que las dos son
// ciertas:
//
//   · El agente puede FALLAR AL GUARDARLO —disco lleno, permisos, el proceso muere justo—, así
//     que mandarlo una sola vez convierte ese fallo en una rotación que nadie puede completar y
//     nadie sabe por qué. Repetirlo en cada latido lo arregla.
//   · Y para repetirlo hay que TENERLO, o sea guardarlo en claro. Pero el invariante del sistema
//     es que en reposo hay hashes y no credenciales, en los dos almacenes: la base guarda
//     `token_sha256`, y un volcado de la base no puede ser un llavero. Es exactamente lo que
//     costó A74 con la contraseña de pantalla.
//
// La salida es la misma que la de A74: el secreto vive en un mapa en memoria del cerebro y la
// base sólo guarda su hash. Lo que se paga es explícito: SI EL CEREBRO SE REINICIA, LA ROTACIÓN
// SE PIERDE. Y ese costo es aceptable porque el token VIEJO sigue valiendo — la máquina no queda
// afuera, la rotación simplemente no se completó y se vuelve a pedir. Un reinicio del cerebro no
// puede dejar una flota sin credenciales.
//
// El scheduler abandona las vencidas, así que la fila tampoco queda con una rotación fantasma.
// ════════════════════════════════════════════════════════════════════════════════════════════

// rotacionPendiente es el token en claro de una rotación abierta, más cuándo deja de ofrecerse.
type rotacionPendiente struct {
	token string
	vence time.Time
}

// recordarRotacion guarda el token nuevo para poder repetirlo en cada latido hasta que el agente
// lo use.
func (s *McpServer) recordarRotacion(deviceID, token string, vence time.Time) {
	s.rotaciones.Store(deviceID, rotacionPendiente{token: token, vence: vence})
}

// tokenDeRotacionPendiente devuelve el token nuevo si hay una rotación viva para ese device.
//
// Una vencida se OLVIDA acá mismo en vez de seguir ofreciéndose: el plazo lo fija quien abre la
// rotación, y seguir mandando un token después de su plazo sería contradecir en silencio lo que
// esa persona declaró.
func (s *McpServer) tokenDeRotacionPendiente(deviceID string) (string, bool) {
	v, hay := s.rotaciones.Load(deviceID)
	if !hay {
		return "", false
	}
	r, ok := v.(rotacionPendiente)
	if !ok {
		return "", false
	}
	if !time.Now().Before(r.vence) {
		s.rotaciones.Delete(deviceID)
		return "", false
	}
	return r.token, true
}

// olvidarRotacion borra el token de memoria. Se llama cuando la rotación se completó: a partir de
// ahí el secreto no tiene ninguna razón para seguir existiendo en ningún lado.
func (s *McpServer) olvidarRotacion(deviceID string) {
	s.rotaciones.Delete(deviceID)
}
