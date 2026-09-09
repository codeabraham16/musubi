package mcp

import (
	"sync"
	"time"

	"musubi/internal/logx"
)

// Motivos de un rechazo de autenticación. Son DOS y no uno porque distinguen dos mundos distintos:
// «no mandaste credencial» casi siempre es un cliente propio mal configurado, y «mandaste una que
// no conozco» es una credencial vieja, vencida o alguien probando. Confundirlos manda a diagnosticar
// una intrusión cuando lo que hay es un daemon al que le sacaron la variable de entorno.
const (
	motivoSinCredencial         = "sin_credencial"
	motivoCredencialDesconocida = "credencial_desconocida"
)

// registroDeAuth escribe QUIÉN está fallando la autenticación, con freno.
//
// POR QUÉ EXISTE: el 2026-09-05 el cerebro acumuló más de ochocientos 401 en unas horas y no había
// forma de saber de dónde salían. El contador `musubi_http_requests_total{result="unauthorized"}`
// no lleva etiqueta de IP —y no puede llevarla, sería cardinalidad sin techo— y el journal no
// registraba nada. Hubo que muestrear `ss` en las dos puntas para nombrar al culpable, que resultó
// ser un daemon local sin credencial. Una tormenta de auth que el propio sistema no puede atribuir
// es indiagnosticable desde adentro. Ver el cabo A88.
//
// EL FRENO IMPORTA TANTO COMO EL REGISTRO: sin él, quien prueba tokens en un bucle escribe el
// journal del servidor a la velocidad que quiera, que es un ataque distinto y gratis. Se emite una
// línea por IP por ventana, y la línea dice cuántas se callaron — así el freno no esconde el
// volumen, que es justo el dato que hace falta para saber si es un cliente roto o una tormenta.
type registroDeAuth struct {
	mu       sync.Mutex
	ultimo   map[string]time.Time
	callados map[string]int
	ventana  time.Duration
	// maxIPs es el techo del mapa: sin él, un atacante que rota IPs de origen hace crecer la
	// memoria del proceso sin límite. Al pasarse se poda lo viejo, y si aun así no alcanza se
	// vacía entero: perder el freno un momento es mejor que perder el servidor.
	maxIPs int
}

func nuevoRegistroDeAuth(ventana time.Duration, maxIPs int) *registroDeAuth {
	if ventana <= 0 {
		ventana = time.Minute
	}
	if maxIPs <= 0 {
		maxIPs = 1024
	}
	return &registroDeAuth{
		ultimo:   make(map[string]time.Time),
		callados: make(map[string]int),
		ventana:  ventana,
		maxIPs:   maxIPs,
	}
}

// fallo registra un rechazo. Devuelve true si además lo escribió en el log (para poder probarlo sin
// leer stderr). La PRIMERA falla de una IP se escribe siempre: el valor de este registro está en
// avisar temprano, no en resumir tarde.
func (r *registroDeAuth) fallo(ahora time.Time, ip, motivo, ruta, agente string) bool {
	if r == nil || ip == "" {
		return false
	}
	r.mu.Lock()
	ultimo, visto := r.ultimo[ip]
	if visto && ahora.Sub(ultimo) < r.ventana {
		r.callados[ip]++
		r.mu.Unlock()
		return false
	}
	callados := r.callados[ip]
	delete(r.callados, ip)
	r.ultimo[ip] = ahora
	r.podarSiHaceFalta(ahora)
	r.mu.Unlock()

	args := []any{"ip", ip, "motivo", motivo, "ruta", ruta}
	if agente != "" {
		args = append(args, "agente", agente)
	}
	if callados > 0 {
		args = append(args, "callados_desde_el_ultimo_aviso", callados)
	}
	logx.Warn("auth: credencial rechazada", args...)
	return true
}

// podarSiHaceFalta se llama con el candado tomado.
func (r *registroDeAuth) podarSiHaceFalta(ahora time.Time) {
	if len(r.ultimo) <= r.maxIPs {
		return
	}
	for ip, t := range r.ultimo {
		if ahora.Sub(t) >= r.ventana {
			delete(r.ultimo, ip)
			delete(r.callados, ip)
		}
	}
	if len(r.ultimo) > r.maxIPs {
		r.ultimo = make(map[string]time.Time)
		r.callados = make(map[string]int)
	}
}
