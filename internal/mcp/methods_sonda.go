package mcp

// methods_sonda.go es «ir a buscar»: el cerebro sale a medir los dispositivos que NO corren un
// agente (Tier B por SSH, Tier C por ADB). Track «Control de flota», S7b/S8.
//
// LA SEPARACIÓN ES LA MISMA QUE EN TODO EL RESTO DEL PROYECTO: `musubi_fleet_probe` va a buscar,
// `musubi_fleet_metrics` lee lo último que se trajo. Igual que el ledger es la historia y el feed
// en vivo es el presente; igual que Musubi guarda la última muestra y Prometheus la serie.
//
// Mezclarlas —que `metrics` salga a la red cuando le falta un dato— haría que una lectura barata
// se vuelva impredecible: a veces microsegundos, a veces treinta segundos y un timeout.

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"musubi/internal/fleet"
)

// contadoresRemotos guarda el estado de CPU POR DISPOSITIVO entre sondeos.
//
// Hace falta porque el porcentaje de CPU es una DERIVADA: sin la lectura anterior no hay número.
// En Tier A ese estado vive en el agente; acá no hay agente, así que lo lleva el cerebro.
//
// Con mutex porque dos sondeos concurrentes del mismo dispositivo son posibles —dos personas
// mirando el mismo router— y `contadorCPU` no es seguro para uso concurrente.
type contadoresRemotos struct {
	mu  sync.Mutex
	por map[string]*fleet.ContadorCPUExportado
}

func (c *contadoresRemotos) para(deviceID string) *fleet.ContadorCPUExportado {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.por == nil {
		c.por = make(map[string]*fleet.ContadorCPUExportado)
	}
	if _, hay := c.por[deviceID]; !hay {
		c.por[deviceID] = &fleet.ContadorCPUExportado{}
	}
	return c.por[deviceID]
}

// sondaMaxDispositivos acota cuántos se sondean en una llamada.
//
// Cada uno es una conexión de red y un fork+exec; sin tope, una llamada sobre un proyecto grande
// se pasa del deadline del transporte y el llamador recibe un timeout en vez de resultados.
const sondaMaxDispositivos = 20

// sondaTimeoutPorDispositivo es cuánto se espera a cada uno. Corto: un dispositivo que no
// responde en 15 s no va a responder, y hacer esperar a los demás por él es peor que saltearlo.
const sondaTimeoutPorDispositivo = 15 * time.Second

// toolFleetProbe sale a medir los dispositivos sin agente y guarda lo que trae.
func (s *McpServer) toolFleetProbe(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Project string `json:"project"`
		Device  string `json:"device"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	proyecto := fleetReadScopeFor(p, args.Project)
	if proyecto == "" {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto: declaralo en `project`")
	}
	devices, err := s.engine.ListarDevices(proyecto, false)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}

	ahora := time.Now()
	filas := make([]map[string]interface{}, 0, len(devices))
	var sinPermiso, sinAgente, truncados int

	for _, d := range devices {
		if nombre := args.Device; nombre != "" && d.Name != nombre {
			continue
		}
		// Tier A tiene agente: sondearlo sería ir a buscar lo que ya viene solo, por un camino
		// que además no existe (nada le entra a esa máquina). Se saltea y se dice.
		if d.Tier == fleet.TierAgente {
			sinAgente++
			continue
		}
		// La compuerta, por máquina: sondear es MEDIR, así que exige `metrics`.
		if !PuedeSobreDevice(p, d, fleet.CapMetrics) {
			sinPermiso++
			continue
		}
		if len(filas) >= sondaMaxDispositivos {
			truncados++
			continue
		}
		filas = append(filas, s.sondearUno(d, ahora))
	}

	res := map[string]interface{}{"project_id": proyecto, "sondeados": len(filas), "resultados": filas}
	if sinPermiso > 0 {
		res["sin_permiso"] = sinPermiso
	}
	if sinAgente > 0 {
		res["con_agente_propio"] = sinAgente
		res["nota"] = "los dispositivos de Tier A reportan solos con `musubi agent`: no se sondean desde acá"
	}
	// Nunca se trunca en silencio.
	if truncados > 0 {
		res["no_sondeados_por_tope"] = truncados
	}
	return jsonResult(res)
}

// sondearUno mide un dispositivo y guarda el resultado.
func (s *McpServer) sondearUno(d fleet.Device, ahora time.Time) map[string]interface{} {
	fila := map[string]interface{}{"device": d.Name, "tier": string(d.Tier)}

	// EL TECHO DE iOS SE DICE ANTES DE INTENTAR NADA. Un error de adb mandaría a alguien a
	// depurar el cable cuando el problema es que la plataforma no lo permite.
	if fleet.EsIOS(d.OS) {
		fila["ok"] = false
		fila["transporte"] = "ninguno"
		fila["error"] = fleet.ErrIOSNoSeMide.Error()
		return fila
	}

	// EL TRANSPORTE LO ELIGE LO QUE SE DECLARÓ, NO EL TIER SOLO. Un Tier B es «por su protocolo
	// nativo», y hay más de uno: una base gestionada en la nube no da shell y sí publica sus
	// vitales en un endpoint. Que esté en `flota-exposicion.yaml` ES la declaración de por dónde
	// se llega a ésta.
	//
	// UN ARCHIVO DE CONFIGURACIÓN ROTO NO PUEDE TUMBAR A LAS MÁQUINAS QUE NO ESTÁN EN ÉL.
	//
	// La primera versión devolvía el error como el resultado del sondeo, y eso convertía una coma
	// de más en `flota-exposicion.yaml` en «ninguna máquina de la flota se pudo medir» — routers
	// por SSH incluidos, que no tienen nada que ver con ese archivo. Un YAML que no parsea no
	// permite saber quién estaba declarado adentro, así que la única salida honesta es sondear a
	// todos por su transporte por defecto y LLEVAR EL AVISO PEGADO a cada fila.
	//
	// No se pierde nada: la máquina que sí necesitaba la exposición va a fallar su sondeo por SSH
	// con su propio mensaje —«no tiene dirección»—, y el aviso que explica por qué está en la
	// misma fila, no en un log que nadie mira.
	destino, porExposicion, errCfg := s.destinoDeExposicion(d.Name)
	if errCfg != nil {
		fila["aviso_configuracion"] = errCfg.Error()
	}

	var m fleet.Muestra
	var err error
	switch {
	case porExposicion:
		fila["transporte"] = string(fleet.TransporteExposicion)
		m, err = fleet.TomarMuestraDeExposicion(destino, s.cpuRemotos.para(d.ID), sondaTimeoutPorDispositivo)
	default:
		transporte := fleet.TransporteSSH
		if d.Tier == fleet.TierMovil {
			transporte = fleet.TransporteADB
		}
		fila["transporte"] = string(transporte)
		m, err = fleet.TomarMuestraRemota(d.Address, transporte, s.cpuRemotos.para(d.ID), sondaTimeoutPorDispositivo)
	}
	if err != nil {
		fila["ok"] = false
		fila["error"] = err.Error()
		// NO se estampa señal de vida: no se llegó. Estamparla haría que un dispositivo
		// inalcanzable figure vivo para siempre.
		return fila
	}
	if err := m.Valida(); err != nil {
		// La muestra vino de un dispositivo que no controlamos: es entrada no confiable. Se
		// rechaza entera en vez de «corregirla», igual que con la de un agente.
		fila["ok"] = false
		fila["error"] = "la muestra no es creíble: " + err.Error()
		return fila
	}

	texto, err := m.Serializar()
	if err != nil {
		fila["ok"] = false
		fila["error"] = err.Error()
		return fila
	}
	// El MISMO UPDATE que estampa la señal de vida guarda la muestra: acá el sondeo exitoso ES
	// la prueba de que se llegó.
	if _, err := s.engine.LatirDevice(d.ID, ahora, texto); err != nil {
		fila["ok"] = false
		fila["error"] = err.Error()
		return fila
	}
	fila["ok"] = true
	fila["cpu_pct"] = m.CPUPct // null en el primer sondeo: la derivada necesita una lectura previa
	fila["mem_pct"] = fleet.PctUsado(m.MemUsada, m.MemTotal)
	fila["mem_libre"] = m.MemLibre                    // nil ⇒ null: no todo sistema expone MemFree
	fila["num_procesos"] = enteroONull(m.NumProcesos) // 0 ⇒ null, con el mismo helper que la tool
	fila["disco_pct"] = fleet.PctUsado(m.DiscoUsado, m.DiscoTotal)
	fila["uptime_seg"] = m.UptimeSeg
	return fila
}
