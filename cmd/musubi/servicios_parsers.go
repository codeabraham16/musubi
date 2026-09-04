package main

// servicios_parsers.go tiene el parseo de las TRES plataformas, SIN build tag.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ NO VIVE CADA UNO EN SU ARCHIVO DE PLATAFORMA
//
// Porque la suite corre en Linux. Con el parser de Windows detrás de `//go:build windows`, la
// única parte del enumerador de Windows que se puede equivocar —convertir la salida del SCM en
// reportes— sería justo la que ninguna prueba puede mirar. Ya pasó una vez en este repo, con las
// rutas de RustDesk: el fallo era exclusivo de Windows, y la guarda que lo habría atrapado no se
// podía escribir desde acá.
//
// Detrás del build tag queda SÓLO lo que de verdad depende del sistema: qué comando se ejecuta.
//
// LO DE LINUX LLEGÓ TARDE, Y SE NOTÓ EN CI. Este archivo decía «las TRES plataformas» desde el
// día uno, pero systemd y los contenedores se habían quedado en servicios_linux.go. Como la
// suite corre en Linux, nada lo delató: servicios_test.go —que no tiene build tag— alcanzaba a
// estadoDeSystemd igual. Recién la primera corrida de `test-cross` lo dijo, y no como un fallo
// de portabilidad sino peor: `vet.exe: servicios_test.go:96: undefined: estadoDeSystemd`. El
// paquete cmd/musubi NO COMPILABA en Windows ni en macOS, así que en esas dos plataformas no se
// verificaba nada — ni estos parsers, ni el resto del binario.

import (
	"encoding/csv"
	"strconv"
	"strings"
	"time"

	"musubi/internal/fleet"
)

func parsearServiciosWindows(salida string, ahora time.Time) []fleet.ReporteServicio {
	r := csv.NewReader(strings.NewReader(strings.ReplaceAll(salida, "\r\n", "\n")))
	r.FieldsPerRecord = -1
	filas, err := r.ReadAll()
	if err != nil || len(filas) < 2 {
		return nil
	}
	col := map[string]int{}
	for i, h := range filas[0] {
		col[strings.ToLower(strings.TrimSpace(h))] = i
	}
	tomar := func(f []string, nombre string) string {
		if i, ok := col[nombre]; ok && i < len(f) {
			return strings.TrimSpace(f[i])
		}
		return ""
	}

	var rs []fleet.ReporteServicio
	for _, f := range filas[1:] {
		nombre := tomar(f, "name")
		if nombre == "" {
			continue
		}
		// SÓLO LOS `Automatic`, Y EL FILTRO VA ANTES DE MIRAR EL CÓDIGO DE SALIDA.
		//
		// Ponerlo después costó 75 alarmas falsas —cinco veces peor que el problema que este
		// archivo vino a arreglar— y la causa es una sola línea de datos: un servicio Manual
		// apagado reporta `ExitCode=1077`, que es «nunca se intentó arrancar desde el boot». En
		// un Automatic eso ES una falla; en un Manual es lo NORMAL. Windows trae cientos, y
		// dejarlos entrar como `fallado` llenó el canal.
		//
		// El código de salida sólo responde una pregunta —«¿se apagó porque terminó o porque
		// murió?»— y esa pregunta sólo tiene sentido sobre algo que alguien declaró que corra.
		arranque := strings.ToLower(tomar(f, "startmode"))
		if !strings.HasPrefix(arranque, "auto") {
			continue
		}
		estado := estadoDeWindows(tomar(f, "state"), tomar(f, "exitcode"))
		rs = append(rs, fleet.ReporteServicio{
			Nombre: nombre,
			Clase:  "windows",
			Salud: fleet.SaludServicio{
				Tomada: ahora,
				Estado: estado,
				// PID, Reinicios y Desde quedan en nil: el SCM no los da por acá y un cero
				// sería una afirmación falsa.
			},
		})
	}
	return rs
}

func estadoDeWindows(estado, exitCode string) fleet.EstadoServicio {
	corriendo := strings.EqualFold(strings.TrimSpace(estado), "running")
	// EL CÓDIGO DE SALIDA MANDA SOBRE EL ESTADO, y sólo cuando NO está corriendo.
	//
	// Un servicio vivo puede arrastrar el ExitCode de una caída anterior de la que ya se
	// recuperó; mirarlo ahí lo dibujaría fallado mientras funciona. Apagado, en cambio, el
	// código es exactamente la pregunta que importa: ¿se apagó porque terminó, o porque murió?
	if !corriendo {
		if c := strings.TrimSpace(exitCode); c != "" && c != "0" {
			return fleet.EstadoFallado
		}
	}
	switch strings.ToLower(strings.TrimSpace(estado)) {
	case "running":
		return fleet.EstadoCorriendo
	case "stopped", "paused":
		// Apagado y con salida limpia: OCIOSO, no caído. Es la distinción entera de A70.
		return fleet.EstadoOcioso
	default:
		return fleet.EstadoDesconocido
	}
}

func parsearLaunchctl(salida string, ahora time.Time) []fleet.ReporteServicio {
	var rs []fleet.ReporteServicio
	for i, l := range strings.Split(strings.ReplaceAll(salida, "\r\n", "\n"), "\n") {
		if i == 0 || strings.TrimSpace(l) == "" {
			continue // la primera línea es el encabezado
		}
		campos := strings.Fields(l)
		if len(campos) < 3 {
			continue
		}
		etiqueta := campos[2]
		if strings.HasPrefix(etiqueta, "com.apple.") {
			continue
		}
		salud := fleet.SaludServicio{Tomada: ahora, Estado: fleet.EstadoDetenido}
		if pid, err := strconv.Atoi(campos[0]); err == nil && pid > 0 {
			salud.PID = &pid
			salud.Estado = fleet.EstadoCorriendo
		}
		if code, err := strconv.Atoi(campos[1]); err == nil && code != 0 {
			salud.Estado = fleet.EstadoFallado
			salud.Detalle = "salida=" + campos[1]
		}
		rs = append(rs, fleet.ReporteServicio{Nombre: etiqueta, Clase: "launchd", Salud: salud})
	}
	return rs
}

// parsearSystemctlShow convierte la salida de bloques en reportes.
//
// SE QUEDA CON LO QUE ALGUIEN DECIDIÓ QUE CORRA, MÁS LO ROTO. Una unit deshabilitada e inactiva
// es ruido —hay cientos—; una habilitada y detenida es exactamente la fila que uno quiere ver.
func parsearSystemctlShow(salida string, ahora time.Time) []fleet.ReporteServicio {
	var rs []fleet.ReporteServicio
	for _, bloque := range strings.Split(strings.ReplaceAll(salida, "\r\n", "\n"), "\n\n") {
		p := map[string]string{}
		for _, l := range strings.Split(bloque, "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(l), "="); ok {
				p[k] = v
			}
		}
		id := p["Id"]
		if id == "" {
			continue
		}
		habilitada := strings.HasPrefix(p["UnitFileState"], "enabled")
		fallada := p["ActiveState"] == "failed"
		if !habilitada && !fallada {
			continue
		}
		salud := fleet.SaludServicio{
			Tomada:  ahora,
			Estado:  estadoDeSystemd(p["ActiveState"], p["SubState"]),
			Detalle: detalleDeSystemd(p),
		}
		if pid, err := strconv.Atoi(p["MainPID"]); err == nil && pid > 0 {
			salud.PID = &pid
		}
		if n, err := strconv.Atoi(p["NRestarts"]); err == nil {
			salud.Reinicios = &n
		}
		if t := fechaDeSystemd(p["ActiveEnterTimestamp"]); t != nil {
			salud.Desde = t
		}
		rs = append(rs, fleet.ReporteServicio{
			Nombre: strings.TrimSuffix(id, ".service"),
			Clase:  "systemd",
			Salud:  salud,
		})
	}
	return rs
}

// estadoDeSystemd traduce el par (ActiveState, SubState) al vocabulario del dominio.
//
// `activating` y `deactivating` NO son «corriendo»: son estados de transición, y llamarlos
// corriendo haría que un servicio que se reinicia en loop se vea sano en el panel.
func estadoDeSystemd(activo, sub string) fleet.EstadoServicio {
	switch activo {
	case "active":
		if sub == "running" || sub == "exited" {
			return fleet.EstadoCorriendo
		}
		return fleet.EstadoDesconocido
	case "failed":
		return fleet.EstadoFallado
	case "inactive":
		return fleet.EstadoDetenido
	case "activating", "deactivating", "reloading":
		return fleet.EstadoDesconocido
	default:
		return fleet.EstadoDesconocido
	}
}

// detalleDeSystemd arma el texto corto que ve el operador. `Result` es lo que dice POR QUÉ murió
// (`exit-code`, `signal`, `oom-kill`), que es la mitad del diagnóstico.
func detalleDeSystemd(p map[string]string) string {
	res := p["Result"]
	if res == "" || res == "success" {
		return ""
	}
	return "result=" + res
}

// fechaDeSystemd parsea el formato de systemd. Devuelve nil si no se entiende o si viene vacío:
// systemd manda `ActiveEnterTimestamp=` a secas para lo que nunca arrancó, y eso NO es la época
// Unix — una fecha inventada es peor que ninguna.
func fechaDeSystemd(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" || s == "n/a" {
		return nil
	}
	for _, f := range []string{"Mon 2006-01-02 15:04:05 MST", "Mon 2006-01-02 15:04:05 -0700"} {
		if t, err := time.Parse(f, s); err == nil {
			return &t
		}
	}
	return nil
}
