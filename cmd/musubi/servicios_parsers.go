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
		estado := estadoDeWindows(tomar(f, "state"), tomar(f, "exitcode"))
		arranque := strings.ToLower(tomar(f, "startmode"))
		// Automatic = alguien lo declaró. Lo demás sólo entra si está roto — un servicio Manual
		// que se murió importa igual, y uno Manual apagado es el ruido que este filtro evita.
		if !strings.HasPrefix(arranque, "auto") && estado != fleet.EstadoFallado {
			continue
		}
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
