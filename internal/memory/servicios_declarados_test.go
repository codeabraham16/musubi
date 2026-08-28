package memory

// servicios_declarados_test.go custodia QUIÉN PUEDE SACAR UNA FILA DEL INVENTARIO DE SERVICIOS.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// EL DEFECTO QUE ESTAS PRUEBAS CIERRAN
//
// La poda por ausencia (PodarServiciosAusentes) corre en CADA latido y da de baja lo que la
// máquina dejó de reportar. Hasta la migración 37, la tabla no distinguía un servicio REPORTADO de
// uno DECLARADO A MANO — y lo declarado a mano es, por definición, lo que ninguna máquina va a
// reportar nunca: la tool dice, con todas las letras, que existe para «un Tier B que no enumera
// solo, un bot, un puente».
//
// O sea que el primer latido que trajera un enumerador de systemd se llevaba puesto, de un saque y
// en toda la flota a la vez, TODO lo declarado a mano. Y sin salida: la fila revocada sigue
// ocupando el único (project_id, device_id, name), así que redeclararla chocaba contra el índice
// con un «ya existe un servicio con ese nombre» que, desde donde mira quien opera, es falso.
//
// No explotaba todavía sólo porque el agente aún no enumera (A42). Por eso hacían falta pruebas y
// no un ticket: el día del despliegue nadie se iba a acordar.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"errors"
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// LA PODA POR AUSENCIA NO SE LLEVA LO QUE DECLARÓ UNA PERSONA.
//
// El escenario es el reportado, tal cual: un bot declarado a mano en una máquina que después
// aprende a enumerar sus units y reporta solamente `sshd.service`.
//
// Sabotaje que la hace fallar (VERIFICADO): sacarle el `AND declared = 0` al UPDATE de
// PodarServiciosAusentes. El bot desaparece del inventario en el primer latido.
func TestLaPodaPorAusenciaNoSeLlevaLoDeclaradoAMano(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "pc-gio")
	ahora := time.Now().UTC()

	// Lo que declaró una persona: ningún enumerador lo va a ver.
	if _, err := e.AltaServicio(fleet.Servicio{Nombre: "bot-telegram", DeviceID: d.ID, Clase: "docker"}); err != nil {
		t.Fatalf("declarar el bot: %v", err)
	}
	// Y lo que enumera la máquina.
	if _, _, err := e.ReportarServicios(d.ID, ahora, []fleet.ReporteServicio{
		{Nombre: "sshd.service", Salud: saludDePrueba(fleet.EstadoCorriendo)},
		{Nombre: "cron.service", Salud: saludDePrueba(fleet.EstadoCorriendo)},
	}); err != nil {
		t.Fatalf("reportar: %v", err)
	}

	// El latido siguiente ya no trae `cron.service` — y nunca trajo el bot.
	podados, err := e.PodarServiciosAusentes(d.ID, []string{"sshd.service"})
	if err != nil {
		t.Fatal(err)
	}
	if podados != 1 {
		t.Errorf("la poda dio de baja %d servicios, esperaba 1 (sólo `cron.service`, que sí lo enumeraba la máquina)", podados)
	}

	vivos := nombresDeServicios(t, e, d.ID)
	if !vivos["bot-telegram"] {
		t.Error("el latido se llevó puesto `bot-telegram`, que lo declaró una persona y ninguna máquina reporta: " +
			"la poda razona «dejó de aparecer, así que ya no está», y esa inferencia no vale sobre algo que nunca apareció")
	}
	if !vivos["sshd.service"] {
		t.Error("se podó `sshd.service`, que SÍ vino en el latido")
	}
	if vivos["cron.service"] {
		t.Error("no se podó `cron.service`, que la máquina dejó de reportar: la poda dejó de podar")
	}
}

// UN SERVICIO DECLARADO QUE LA MÁQUINA SÍ REPORTA SIGUE SIENDO DE QUIEN LO DECLARÓ.
//
// La procedencia es de la FILA, no de la última escritura. Si un reporte «adoptara» la fila,
// alcanzaría un latido con el nombre adentro para volver a dejarla a merced de la poda — y el
// agujero volvería por la puerta de atrás, disparado por la máquina misma.
//
// Sabotaje que la hace fallar: poner `declared = 0` en el UPDATE de ReportarServicios (o sumarlo
// al CASE de kind), y después podar sin ese nombre.
func TestUnServicioDeclaradoQueLaMaquinaReportaSigueSiendoDeclarado(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "pc-gio")
	ahora := time.Now().UTC()

	if _, err := e.AltaServicio(fleet.Servicio{Nombre: "postgres", DeviceID: d.ID}); err != nil {
		t.Fatal(err)
	}
	// La máquina lo reporta: se ACTUALIZA la fila que ya estaba, no se crea otra.
	nuevos, act, err := e.ReportarServicios(d.ID, ahora, []fleet.ReporteServicio{
		{Nombre: "postgres", Salud: saludDePrueba(fleet.EstadoCorriendo)},
	})
	if err != nil || nuevos != 0 || act != 1 {
		t.Fatalf("el reporte del declarado: nuevos=%d actualizados=%d err=%v (esperaba 0 y 1)", nuevos, act, err)
	}

	// Y ahora la máquina deja de reportarlo. Sigue siendo de quien lo declaró.
	if _, err := e.PodarServiciosAusentes(d.ID, []string{"otra-cosa.service"}); err != nil {
		t.Fatal(err)
	}
	if !nombresDeServicios(t, e, d.ID)["postgres"] {
		t.Error("un reporte de la máquina volvió podable un servicio DECLARADO: la procedencia es de la fila, " +
			"no de la última escritura, o alcanza un latido con el nombre adentro para reabrir el agujero")
	}
}

// VOLVER A DECLARAR UN SERVICIO DADO DE BAJA LO TRAE DE VUELTA, Y ÉSA ES LA SALIDA QUE NO HABÍA.
//
// La fila revocada sigue ocupando el único (project_id, device_id, name), así que sin esto
// redeclarar chocaba contra el índice y el mensaje decía «ya existe un servicio con ese nombre» —
// que para quien opera es falso, porque el listado por defecto no lo muestra— y encima no decía
// qué hacer. Revivir no contradice al agente: un REPORTE sigue sin resucitar nada.
//
// Sabotaje que la hace fallar: devolver ErrServicioDuplicado en cuanto el INSERT choca, sin mirar
// si la fila que ocupa el nombre está revocada.
func TestRedeclararUnServicioDadoDeBajaLoTraeDeVuelta(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "pc-gio")

	primero, err := e.AltaServicio(fleet.Servicio{Nombre: "bot-telegram", DeviceID: d.ID, Clase: "docker"})
	if err != nil {
		t.Fatal(err)
	}
	// Se da de baja la máquina entera (el camino real por el que un declarado se revoca hoy).
	if _, err := e.RevocarServiciosDeDevice(d.ID); err != nil {
		t.Fatal(err)
	}
	if nombresDeServicios(t, e, d.ID)["bot-telegram"] {
		t.Fatal("la baja no revocó nada: esta prueba no está probando la vuelta")
	}

	vuelto, err := e.AltaServicio(fleet.Servicio{Nombre: "bot-telegram", DeviceID: d.ID, Clase: "docker"})
	if err != nil {
		t.Fatalf("redeclarar un servicio dado de baja falló: %v — quien opera se queda sin ninguna salida, "+
			"porque la fila revocada sigue ocupando el nombre", err)
	}
	if vuelto.ID != primero.ID {
		t.Errorf("el servicio volvió con otro id (%q, era %q): para quien opera es el mismo de siempre y "+
			"cambiarlo rompe cualquier referencia vieja", vuelto.ID, primero.ID)
	}
	if !vuelto.Declarado || vuelto.Revocado {
		t.Errorf("volvió mal: declarado=%v revocado=%v", vuelto.Declarado, vuelto.Revocado)
	}
	if !nombresDeServicios(t, e, d.ID)["bot-telegram"] {
		t.Error("el servicio no volvió al inventario activo")
	}
	// Vuelve como NACE una declaración: sin salud vieja haciéndose pasar por presente.
	for _, sv := range serviciosDe(t, e, d.ID) {
		if sv.Nombre == "bot-telegram" && sv.Salud != nil {
			t.Error("volvió con la salud que tenía cuando lo revocaron: eso es de otra época y se lee como presente")
		}
	}

	// Y un duplicado DE VERDAD —la fila está viva— sigue rechazándose, con un mensaje que dice qué
	// hacer y no sólo qué pasó.
	_, err = e.AltaServicio(fleet.Servicio{Nombre: "bot-telegram", DeviceID: d.ID})
	if !errors.Is(err, fleet.ErrServicioDuplicado) {
		t.Fatalf("declarar dos veces uno vivo dio %v, esperaba ErrServicioDuplicado", err)
	}
	if !strings.Contains(err.Error(), "musubi_fleet_services") {
		t.Errorf("el mensaje del duplicado no dice qué hacer, sólo qué pasó: %v", err)
	}
}

// ── Ayudantes ───────────────────────────────────────────────────────────────────────────────

func serviciosDe(t *testing.T, e *DbEngine, deviceID string) []fleet.Servicio {
	t.Helper()
	svs, err := e.ServiciosDeDevice(deviceID)
	if err != nil {
		t.Fatal(err)
	}
	return svs
}

func nombresDeServicios(t *testing.T, e *DbEngine, deviceID string) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	for _, sv := range serviciosDe(t, e, deviceID) {
		out[sv.Nombre] = true
	}
	return out
}
