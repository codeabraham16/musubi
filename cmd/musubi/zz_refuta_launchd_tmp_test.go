package main

import (
	"testing"
	"time"

	"musubi/internal/fleet"
)

func TestZZRefutaLaunchdTmp(t *testing.T) {
	ahora := time.Now()
	casos := []string{
		"PID\tStatus\tLabel\n431\t-9\tcom.ejemplo.agente\n",
		"PID\tStatus\tLabel\n431\t1\tcom.ejemplo.agente\n",
		"PID\tStatus\tLabel\n431\t0\tcom.ejemplo.agente\n",
		"PID\tStatus\tLabel\n-\t-9\tcom.ejemplo.agente\n",
	}
	for _, s := range casos {
		for _, r := range parsearLaunchctl(s, ahora) {
			pid := -1
			if r.Salud.PID != nil {
				pid = *r.Salud.PID
			}
			t.Logf("launchd in=%q -> estado=%q pid=%d detalle=%q caido=%v prio=%d",
				s, r.Salud.Estado, pid, r.Salud.Detalle,
				fleet.EstadoCuentaComoCaido(r.Salud.Estado), prioridadDeReporte(r))
		}
	}
	win := "\"Name\",\"State\",\"StartMode\",\"ExitCode\"\n\"vivo\",\"Running\",\"Auto\",\"1067\"\n"
	for _, r := range parsearServiciosWindows(win, ahora) {
		t.Logf("windows vivo+1067 -> estado=%q detalle=%q caido=%v prio=%d",
			r.Salud.Estado, r.Salud.Detalle, fleet.EstadoCuentaComoCaido(r.Salud.Estado), prioridadDeReporte(r))
	}
	sysd := "Id=vivo.service\nActiveState=active\nSubState=running\nUnitFileState=enabled\nMainPID=431\nNRestarts=13\nResult=exit-code"
	for _, r := range parsearSystemctlShow(sysd, ahora) {
		t.Logf("systemd activo+Result=exit-code -> estado=%q detalle=%q caido=%v",
			r.Salud.Estado, r.Salud.Detalle, fleet.EstadoCuentaComoCaido(r.Salud.Estado))
	}
}
