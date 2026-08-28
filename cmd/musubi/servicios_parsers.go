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

func parsearGetService(salida string, ahora time.Time) []fleet.ReporteServicio {
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
		estado := estadoDeWindows(tomar(f, "status"))
		arranque := strings.ToLower(tomar(f, "starttype"))
		// Automatic = alguien lo declaró. Lo demás sólo entra si está roto.
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

// estadoDeWindows traduce el estado del SCM.
//
// Los estados de transición (StartPending, StopPending, ...) NO son «corriendo»: un servicio
// atascado en StartPending lleva minutos sin arrancar y llamarlo corriendo lo esconde.
func estadoDeWindows(s string) fleet.EstadoServicio {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "running":
		return fleet.EstadoCorriendo
	case "stopped":
		return fleet.EstadoDetenido
	case "paused":
		return fleet.EstadoDetenido
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
