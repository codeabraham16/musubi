package mcp

// methods_cronologia.go expone la línea de tiempo de UNA máquina: qué le hizo Musubi dentro de
// una ventana, cruzando los tres planos en una sola lista. Fase 5 · S11.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ ES UNA TOOL NUEVA Y NO UN PARÁMETRO DE LAS QUE YA HAY
//
// Las tres bitácoras existentes contestan «lo último que pasó en este plano». Ésta contesta «qué
// pasó en esta máquina», que es la pregunta que alguien hace de verdad cuando algo anda mal — y
// que hoy se responde llamando a tres tools y ordenando a mano, o sea que no se responde.
//
// La diferencia no es de comodidad: al cruzarlas a mano se pierde justo lo que importa, que es
// ver que la sesión de shell de las 14:02 y el comando de las 14:03 son la misma historia.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA COMPUERTA ES POR HECHO, Y ES EL CORAZÓN DE ESTE ARCHIVO
//
// Las tres fuentes tienen tres compuertas distintas: `exec` los comandos, `screen:view` las
// pantallas, `shell` las shells. Compuertar la lista entera con UNA capacidad —cualquiera— falla
// en una de las dos direcciones: con la más laxa le muestra a alguien con `exec` quién tuvo un
// prompt; con la más estricta le esconde sus propios comandos a quien puede correrlos.
//
// Así que cada hecho trae su capacidad (fleet.CapDeHecho) y se pregunta por separado. Un hecho
// cuyo tipo no está clasificado NO SE MUESTRA A NADIE y se cuenta aparte: el default es no
// mostrar, para que una operación interna nueva no aparezca en la lista de todo el que pueda
// ejecutar antes de que alguien haya decidido quién puede verla.

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"musubi/internal/fleet"
)

const (
	// cronologiaTopeDefault y cronologiaTopeMax acotan cuántos hechos vuelven. Más chico que el
	// tope de las bitácoras a propósito: una línea de tiempo se LEE, y doscientas líneas no se
	// leen — se escanean, que es otra cosa.
	cronologiaTopeDefault = 50
	cronologiaTopeMax     = 300
)

// toolFleetCronologia arma la línea de tiempo de una máquina.
func (s *McpServer) toolFleetCronologia(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	p := principalFrom(ctx)
	var args struct {
		Device  string  `json:"device"`
		Project string  `json:"project"`
		Desde   string  `json:"desde"`
		Hasta   string  `json:"hasta"`
		Horas   float64 `json:"horas"`
		Limite  int     `json:"limite"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "argumentos inválidos: %v", err)
		}
	}
	nombre := strings.TrimSpace(args.Device)
	if nombre == "" {
		return nil, rpcErrorf(codeInvalidParams, "falta `device`: la cronología es de UNA máquina. Sin eso sería la bitácora, que ya existe")
	}

	// EL LAZO POR PROYECTO, no `fleetReadScopeFor` a secas. El principal del panel es `read: all`
	// SIN proyecto propio, así que resolver el alcance con el atajo devuelve vacío y la tool
	// contesta «no se pudo determinar el proyecto» estando todo bien. Ese bug ya se pagó en
	// cuatro tools de lectura; escribir la quinta con el atajo sería reproducirlo a sabiendas.
	proyectos, _ := s.proyectosParaLeer(p, args.Project)
	if len(proyectos) == 0 {
		return nil, rpcErrorf(codeInvalidParams, "no se pudo determinar el proyecto: declaralo en `project`")
	}
	var (
		device   fleet.Device
		proyecto string
		hallados int
	)
	for _, pr := range proyectos {
		d, existe, err := s.engine.DevicePorNombre(pr, nombre)
		if err != nil {
			return nil, rpcErrorf(codeInternalError, "%v", err)
		}
		if existe {
			hallados++
			device, proyecto = d, pr
		}
	}
	// DOS MÁQUINAS CON EL MISMO NOMBRE EN DOS TENANTS NO SE DESEMPATAN SOLAS. El nombre sólo es
	// único dentro de su proyecto, así que elegir una —la primera, la última— le devolvería a
	// quien ve varios tenants la cronología de una máquina que no es la que preguntó, sin decirlo.
	if hallados > 1 {
		return nil, rpcErrorf(codeInvalidParams,
			"hay %d máquinas llamadas %q en los proyectos que alcanzás: declará `project` para elegir cuál", hallados, nombre)
	}
	if hallados == 0 {
		return nil, rpcErrorf(codeUnauthorized,
			"no hay ninguna máquina %q en los proyectos que alcanzás", nombre)
	}

	// ¿Puede ver ALGÚN plano de esta máquina? Sin esto la respuesta sería una lista vacía con un
	// contador de ocultos, que se lee como «no pasó nada» justo cuando lo que pasa es que no
	// tenés permiso para verlo.
	//
	// NO es una fuga de existencia: la tenencia ya filtró arriba, y el inventario
	// (musubi_fleet_list) muestra la máquina sin exigir capacidad ninguna. Lo que se protege acá
	// es la CONFUSIÓN, no el secreto.
	if !algunPlanoVisible(p, device) {
		return nil, rpcErrorf(codeUnauthorized,
			"tu credencial no puede ver ningún plano de %q: la cronología pide al menos una de `exec`, `screen:view` o `shell` sobre esa máquina (ver la sección `fleet:` de principals.yaml)", nombre)
	}

	ahora := time.Now().UTC()
	ventana, rpcErr := ventanaDeArgs(args.Desde, args.Hasta, args.Horas, ahora)
	if rpcErr != nil {
		return nil, rpcErr
	}
	// SE NORMALIZA ACÁ Y NO SÓLO ADENTRO DE LA CONSULTA porque esta ventana también se DEVUELVE.
	// Contestar con la ventana pedida mientras se aplica otra es la clase de mentira chica que
	// hace irreproducible una investigación: alguien copia el `desde` de la respuesta, lo vuelve
	// a pedir, y le vuelven hechos distintos.
	ventana = ventana.Normalizada()

	tope := cronologiaTopeDefault
	if args.Limite > 0 {
		tope = args.Limite
	}
	if tope > cronologiaTopeMax {
		tope = cronologiaTopeMax
	}

	hechos, truncado, err := s.engine.CronologiaDeDevice(proyecto, device.ID, ventana, tope, ahora)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "%v", err)
	}

	filas := make([]map[string]interface{}, 0, len(hechos))
	ocultos, sinClasificar := 0, 0
	for _, h := range hechos {
		necesaria, clasificado := fleet.CapDeHecho(h.Tipo)
		if !clasificado {
			// NO es «oculto por permiso» y por eso se cuenta aparte: nadie lo puede ver, ni
			// siquiera quien tenga todo. Mezclarlos en un contador diría «pedile permiso a
			// alguien» sobre algo que ningún permiso destraba.
			sinClasificar++
			continue
		}
		// PuedeVerHistorialDeDevice y no PuedeSobreDevice: una máquina revocada conserva su
		// cronología y quien podía verla mientras vivía la sigue viendo (A51). La revocación es
		// el kill-switch para OPERAR, no para auditar.
		if !PuedeVerHistorialDeDevice(p, device, necesaria) {
			ocultos++
			continue
		}
		filas = append(filas, filaDeHecho(h))
	}

	return jsonResult(map[string]interface{}{
		"device":  device.Name,
		"project": proyecto,
		"ventana": map[string]interface{}{
			"desde":       ventana.Desde.UTC().Format(time.RFC3339),
			"hasta":       ventana.Hasta.UTC().Format(time.RFC3339),
			"duracion_hs": ventana.Duracion().Hours(),
		},
		"hechos": filas,
		"total":  len(filas),
		// Los tres contadores dicen COSAS DISTINTAS y por eso son tres. `truncado` es «hubo más
		// adentro de la ventana»; `ocultos_por_permiso` es «hubo más y no los podés ver»;
		// `sin_clasificar` es «hubo más y esta versión no sabe qué son». Un solo número los
		// confundiría, y cada uno se arregla distinto: agrandá la ventana, pedí permiso, o
		// actualizá el cerebro.
		"truncado":            truncado,
		"ocultos_por_permiso": ocultos,
		"sin_clasificar":      sinClasificar,
		// LO QUE ESTA LISTA NO VIO, dicho en la misma respuesta. Una cronología vacía se lee como
		// «no pasó nada en esa máquina» cuando lo que quiere decir es «no pasó nada DE LO QUE YO
		// MIRO», y esa diferencia es la que hace que alguien deje de buscar la causa real.
		"no_visto": fleet.HuecosDeLaCronologia(),
	})
}

// algunPlanoVisible dice si esta credencial alcanza a ver al menos un plano de esta máquina.
//
// Se recorre lo que la cronología PUEDE mostrar y no la lista entera de capacidades: tener
// `metrics` sobre una máquina no hace visible ni un solo hecho, así que dar por buena la consulta
// devolvería siempre una lista vacía.
func algunPlanoVisible(p *Principal, d fleet.Device) bool {
	for _, t := range fleet.TiposDeHecho {
		necesaria, clasificado := fleet.CapDeHecho(t)
		if clasificado && PuedeVerHistorialDeDevice(p, d, necesaria) {
			return true
		}
	}
	return false
}

// ventanaDeArgs traduce los tres argumentos posibles a UNA ventana, con los defaults del dominio.
//
// Las tres formas existen porque las tres preguntas existen: «las últimas 6 horas» se pide con
// `horas`, «el martes» con `desde` y `hasta`, y «desde que lo reinicié» con `desde` solo. Ninguna
// se deduce de las otras, y obligar a escribir las dos puntas para preguntar «¿qué pasó recién?»
// es cómo una herramienta deja de usarse.
func ventanaDeArgs(desdeTxt, hastaTxt string, horas float64, ahora time.Time) (fleet.Ventana, *RpcError) {
	parsear := func(campo, txt string) (time.Time, *RpcError) {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(txt))
		if err != nil {
			return time.Time{}, rpcErrorf(codeInvalidParams,
				"`%s` no es una fecha RFC3339 (ej: 2026-08-25T14:00:00Z): %v", campo, err)
		}
		return t.UTC(), nil
	}
	desdeTxt, hastaTxt = strings.TrimSpace(desdeTxt), strings.TrimSpace(hastaTxt)

	// `horas` con `desde` o `hasta` es una contradicción y NO se resuelve eligiendo uno: quien
	// mandó las dos cosas cree que pidió algo distinto de lo que sea que elijamos, y la respuesta
	// se vería correcta.
	if horas > 0 && (desdeTxt != "" || hastaTxt != "") {
		return fleet.Ventana{}, rpcErrorf(codeInvalidParams,
			"`horas` y `desde`/`hasta` son dos formas de pedir la misma ventana: mandá una sola")
	}

	switch {
	case desdeTxt != "" && hastaTxt != "":
		desde, err := parsear("desde", desdeTxt)
		if err != nil {
			return fleet.Ventana{}, err
		}
		hasta, err := parsear("hasta", hastaTxt)
		if err != nil {
			return fleet.Ventana{}, err
		}
		v := fleet.Ventana{Desde: desde, Hasta: hasta}
		if e := v.Valida(); e != nil {
			return fleet.Ventana{}, rpcErrorf(codeInvalidParams, "%v", e)
		}
		return v, nil
	case desdeTxt != "":
		desde, err := parsear("desde", desdeTxt)
		if err != nil {
			return fleet.Ventana{}, err
		}
		v := fleet.Ventana{Desde: desde, Hasta: ahora}
		if e := v.Valida(); e != nil {
			return fleet.Ventana{}, rpcErrorf(codeInvalidParams, "%v", e)
		}
		return v, nil
	case hastaTxt != "":
		// `hasta` sin `desde` es la ventana default TERMINANDO ahí. Es lo que pide quien
		// investiga hacia atrás desde el momento del incidente.
		hasta, err := parsear("hasta", hastaTxt)
		if err != nil {
			return fleet.Ventana{}, err
		}
		return fleet.VentanaHasta(hasta, fleet.VentanaDefault), nil
	case horas > 0:
		return fleet.VentanaHasta(ahora, time.Duration(horas*float64(time.Hour))), nil
	}
	return fleet.VentanaHasta(ahora, fleet.VentanaDefault), nil
}

// filaDeHecho serializa un hecho. Es UNA sola función porque los cinco tipos comparten forma:
// dos formatos distintos según el tipo obligarían a cada consumidor a ramificar, y el que se
// olvide de ramificar dibuja mal justo el tipo que menos aparece.
func filaDeHecho(h fleet.Hecho) map[string]interface{} {
	fila := map[string]interface{}{
		"cuando":     h.Cuando.UTC().Format(time.RFC3339),
		"tipo":       string(h.Tipo),
		"plano":      string(h.Plano),
		"principal":  h.Principal,
		"referencia": h.Referencia,
		"estado":     h.Estado,
	}
	// El argv YA viene sin secreto (fleet.ArgvDeBitacora, aplicado al construir el hecho). Va
	// null y no `[]` cuando no aplica: una lista vacía se leería como «corrió algo sin
	// argumentos», que es un hecho distinto de «esto no es un comando».
	if len(h.Argv) > 0 {
		fila["argv"] = h.Argv
	} else {
		fila["argv"] = nil
	}
	// `termino` y `duracion_seg` viajan en null cuando no terminó. Un 0 se dibuja como «duró
	// nada» y lo que pasa es que sigue en curso — el mismo cero mentiroso que persigue todo el
	// track, esta vez en el eje del tiempo.
	if h.Termino.IsZero() {
		fila["termino"] = nil
	} else {
		fila["termino"] = h.Termino.UTC().Format(time.RFC3339)
	}
	if d, hay := h.Duracion(); hay {
		fila["duracion_seg"] = int64(d.Seconds())
	} else {
		fila["duracion_seg"] = nil
	}
	return fila
}
