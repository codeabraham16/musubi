package mcp

// methods_contexto.go es la segunda mitad de la fase 5: el plano de FLOTA preguntándole al plano
// de MEMORIA. La cronología dice qué HIZO Musubi en una máquina; esto dice qué SABÍA.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LOS TÉRMINOS SON INFORMACIÓN, Y ÉSA ES LA TRAMPA DE SEGURIDAD DE ESTE SLICE
//
// La lista de términos que se devuelve —el nombre de la máquina y sus servicios— parece metadato
// de la consulta y no lo es: decirle a alguien «busqué `postgres` en esta máquina» le está
// diciendo que ahí corre un postgres. Eso es exactamente lo que `musubi_fleet_services` compuerta
// con `metrics`.
//
// Así que los términos de servicio **sólo se arman si la credencial puede ver los servicios**, y
// cuando no puede, la respuesta lo DICE (`servicios_ocultos`) en vez de devolver una lista corta
// que se lee como «esta máquina no corre nada».
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA MEMORIA SE LEE EN EL PROYECTO DE LA MÁQUINA, NO EN EL ALCANCE DE QUIEN PREGUNTA
//
// Un principal `read: all` alcanza la memoria de todos los tenants. Si la búsqueda usara SU
// alcance, el contexto de una máquina de `altura` traería notas de `crm` que no tienen nada que
// ver — no es una fuga (puede verlas igual por otra puerta) pero es una RESPUESTA FALSA: enlaza
// como contexto de esa máquina algo que pertenece a otro mundo.
//
// El scope se fija al proyecto del DEVICE, que además siempre es igual o más angosto que lo que
// la credencial ya podía ver.

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// toolFleetContexto cruza la actividad de una máquina con lo que la memoria y el código saben.
func (s *McpServer) toolFleetContexto(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Device  string  `json:"device"`
		Project string  `json:"project"`
		Desde   string  `json:"desde"`
		Hasta   string  `json:"hasta"`
		Horas   float64 `json:"horas"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	nombre := strings.TrimSpace(args.Device)
	if nombre == "" {
		return nil, rpcErrorf(codeInvalidParams, "falta `device`: el contexto es de UNA máquina")
	}
	device, proyecto, rpcErr := s.resolverDeviceUnico(p, nombre, args.Project)
	if rpcErr != nil {
		return nil, rpcErr
	}
	ahora := time.Now().UTC()
	ventana, rpcErr := ventanaDeArgs(args.Desde, args.Hasta, args.Horas, ahora)
	if rpcErr != nil {
		return nil, rpcErr
	}
	ventana = ventana.Normalizada()

	// ── 1. La actividad, con la MISMA compuerta que la cronología ────────────────────────────
	//
	// Va primero porque su contador de ocultos es parte de la honestidad de la respuesta: un
	// contexto que dice «no pasó nada» sin aclarar que hubo doce hechos que no podés ver es la
	// misma mentira que perseguía S13, un nivel más arriba.
	hechos, truncado, err := s.engine.CronologiaDeDevice(proyecto, device.ID, ventana, cronologiaTopeMax, ahora)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	visibles, ocultos, sinClasificar := hechosVisiblesPara(p, device, hechos)
	porPlano := map[string]int{}
	for _, h := range visibles {
		porPlano[string(h.Tipo)]++
	}

	// ── 2. Los términos, compuertados por lo que la credencial PUEDE VER ─────────────────────
	var nombresDeServicio []string
	serviciosOcultos := false
	if PuedeVerHistorialDeDevice(p, device, fleet.CapMetrics) {
		servicios, err := s.engine.ListarServicios(proyecto, device.ID, false)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "%v", err)
		}
		for _, sv := range servicios {
			nombresDeServicio = append(nombresDeServicio, sv.Nombre)
		}
	} else {
		// NO se devuelve una lista corta y ya: sin esta marca, una credencial sin `metrics`
		// recibiría exactamente la misma forma de respuesta que una máquina sin servicios, y las
		// dos cosas se leen igual y significan lo contrario.
		serviciosOcultos = true
	}
	terminos := fleet.TerminosDeContexto(device.Name, nombresDeServicio)

	// ── 3. La memoria, en el proyecto de la MÁQUINA ──────────────────────────────────────────
	memCtx := memory.WithProjectScope(ctx, memory.ProjectScope{ProjectID: proyecto, Federate: false})

	type hallazgo struct {
		obs     memory.Observation
		enlace  fleet.Enlace
		termino string
	}
	porID := map[string]hallazgo{}
	// Por término primero: si una misma nota entra por las dos vías, el enlace que se conserva
	// es el FUERTE. Al revés, un acierto de término quedaría rotulado «coincidió en el tiempo» y
	// perdería justo el peso que lo hace útil.
	for _, t := range terminos {
		obs, err := s.engine.SearchObservationsFTS(memCtx, t.Texto, fleet.ContextoTopeMemoria)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "%v", err)
		}
		for _, o := range obs {
			if _, ya := porID[o.ID]; !ya {
				porID[o.ID] = hallazgo{obs: o, enlace: fleet.EnlacePorTermino, termino: t.Texto}
			}
		}
	}
	enVentana, err := s.engine.ObservacionesEnVentana(memCtx, ventana, fleet.ContextoTopeMemoria)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	for _, o := range enVentana {
		if _, ya := porID[o.ID]; !ya {
			porID[o.ID] = hallazgo{obs: o, enlace: fleet.EnlacePorVentana}
		}
	}

	filasMem := make([]map[string]interface{}, 0, len(porID))
	for _, h := range porID {
		fila := map[string]interface{}{
			"id":        h.obs.ID,
			"topic_key": h.obs.TopicKey,
			"cuando":    h.obs.CreatedAt,
			// EL ENLACE ES EL CAMPO QUE MÁS IMPORTA de toda la respuesta: dice si esto es
			// evidencia o si sólo pasó el mismo día.
			"enlace":    string(h.enlace),
			"contenido": s.redactIfForced(recortarConMarca(h.obs.Content, fleet.ContenidoMax)),
		}
		// El término viaja SÓLO cuando enlazó por término. Ponerlo vacío en los otros invitaría a
		// leer `""` como «no se sabe qué término», cuando lo cierto es que no hubo ninguno.
		if h.enlace == fleet.EnlacePorTermino {
			fila["termino"] = h.termino
		}
		filasMem = append(filasMem, fila)
	}

	// ── 4. El código tocado en la ventana ────────────────────────────────────────────────────
	archivos, err := s.engine.CodigoTocadoEnVentana(memCtx, ventana, fleet.ContextoTopeCodigo)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}
	filasCod := make([]map[string]interface{}, 0, len(archivos))
	for _, a := range archivos {
		fila := map[string]interface{}{"path": a.Path}
		// null y no una fecha inventada: una hora ilegible no se rellena con el cero de Go, que
		// se dibujaría como el año 1.
		if a.Cuando.IsZero() {
			fila["cuando"] = nil
		} else {
			fila["cuando"] = a.Cuando.UTC().Format(time.RFC3339)
		}
		filasCod = append(filasCod, fila)
	}

	filasTerm := make([]map[string]interface{}, 0, len(terminos))
	for _, t := range terminos {
		filasTerm = append(filasTerm, map[string]interface{}{"texto": t.Texto, "de": string(t.De)})
	}

	return jsonResult(map[string]interface{}{
		"device":  device.Name,
		"project": proyecto,
		"ventana": map[string]interface{}{
			"desde":       ventana.Desde.UTC().Format(time.RFC3339),
			"hasta":       ventana.Hasta.UTC().Format(time.RFC3339),
			"duracion_hs": ventana.Duracion().Hours(),
		},
		// LOS TÉRMINOS SE DECLARAN, y no es adorno: son la única forma de que quien lee juzgue si
		// un enlace vale. Una herramienta que busca por su cuenta y no dice qué buscó obliga a
		// creerle.
		"terminos":          filasTerm,
		"servicios_ocultos": serviciosOcultos,
		"actividad": map[string]interface{}{
			"por_tipo":            porPlano,
			"total":               len(visibles),
			"truncado":            truncado,
			"ocultos_por_permiso": ocultos,
			"sin_clasificar":      sinClasificar,
		},
		"memoria": filasMem,
		"codigo":  filasCod,
		"no_visto": append(fleet.HuecosDelContexto(),
			"El `cuando` de un archivo es cuándo Musubi RE-RESUMIÓ ese archivo, no la fecha de un commit: Musubi no lee el historial de git."),
	})
}

// recortarConMarca acota un texto DEJANDO LA MARCA, y corta por RUNAS.
//
// Es deliberadamente distinta de `recortar` (fleet_http.go), que corta por bytes y sin marca: esa
// normaliza un campo para que entre en una celda de tabla, ésta muestra PROSA a una persona. Sin
// la marca, una nota cortada se lee como una nota corta y alguien concluye sobre media frase
// creyendo que la leyó entera; y cortando por bytes, un acento en el límite queda partido.
func recortarConMarca(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "… [recortado]"
}

// resolverDeviceUnico busca una máquina por nombre en los proyectos que la credencial alcanza.
//
// Está extraída porque la usan las DOS tools de fase 5 y porque su parte delicada —que dos
// máquinas homónimas en dos tenants NO se desempaten solas— es fácil de escribir mal una segunda
// vez: elegir «la primera» le devolvería a quien ve varios tenants la historia de una máquina que
// no es la que preguntó, sin decirlo.
func (s *McpServer) resolverDeviceUnico(p *Principal, nombre, declarado string) (fleet.Device, string, *RpcError) {
	// EL LAZO POR PROYECTO, no `fleetReadScopeFor` a secas: el principal del panel es `read: all`
	// SIN proyecto propio, y el atajo devuelve vacío. Ese bug ya se pagó en cuatro tools.
	proyectos, _ := s.proyectosParaLeer(p, declarado)
	if len(proyectos) == 0 {
		return fleet.Device{}, "", rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto: declaralo en `project`")
	}
	var (
		device   fleet.Device
		proyecto string
		hallados int
	)
	for _, pr := range proyectos {
		d, existe, err := s.engine.DevicePorNombre(pr, nombre)
		if err != nil {
			return fleet.Device{}, "", rpcErrorf(codeInternalError, "%v", err)
		}
		if existe {
			hallados++
			device, proyecto = d, pr
		}
	}
	if hallados > 1 {
		return fleet.Device{}, "", rpcErrorf(codeInvalidParams,
			"hay %d máquinas llamadas %q en los proyectos que alcanzás: declará `project` para elegir cuál", hallados, nombre)
	}
	if hallados == 0 {
		return fleet.Device{}, "", rpcErrorf(codeUnauthorized,
			"no hay ninguna máquina %q en los proyectos que alcanzás", nombre)
	}
	return device, proyecto, nil
}
