package main

// servicios_usuario.go es la fuente `systemctl --user`: las units que escribió el DUEÑO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EXISTE, MEDIDO EL 2026-09-24 EN musubi-server
//
// Los tres puentes de WhatsApp, el gateway de Telegram, el CRM y los oneshots de Core01 son units
// `--user` del usuario `musubi`, y el agente sólo le preguntaba al manager del SISTEMA. No estaban
// en el inventario, así que ninguna alerta podía verlos: `core01-ensayo-local` (el simulacro de
// restauración de Core01) estaba FAILED y no figuraba en ninguna alerta.
//
// SIN BUILD TAG, como servicios_parsers.go: lo único que depende de Linux es la llamada desde
// servicios_linux.go. Todo lo demás se prueba desde cualquier máquina.

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"musubi/internal/fleet"
)

// estadoDelBus es lo que se sabe del manager de usuario ANTES de preguntarle nada.
type estadoDelBus int

const (
	// runtimeAusente: no hay /run/user/<uid>. El usuario no tiene manager (ni sesión ni linger), o
	// el agente enumera un mundo sin runtime (root sin MUSUBI_AGENTE_USUARIO). Es una fuente que NO
	// ESTÁ, igual que un podman sin instalar: no es una falla.
	runtimeAusente estadoDelBus = iota
	// busAusente: el runtime existe y no tiene ni systemd/private ni bus. Es el arranque en el que
	// el agente llegó antes que user@<uid>.service.
	busAusente
	busListo
)

// estadoDelBusDeUsuario mira el runtime de la identidad. `systemctl --user`, corriendo con el uid
// del dueño, se conecta primero a <runtime>/systemd/private y cae al bus de D-Bus si no está.
func estadoDelBusDeUsuario(runtime string) estadoDelBus {
	if runtime == "" {
		return runtimeAusente
	}
	if fi, err := os.Stat(runtime); err != nil || !fi.IsDir() {
		return runtimeAusente
	}
	for _, sock := range []string{path.Join(runtime, "systemd", "private"), path.Join(runtime, "bus")} {
		if _, err := os.Stat(sock); err == nil {
			return busListo
		}
	}
	return busAusente
}

// enumerarUnitsDeUsuario devuelve las units del dueño. El home y el runtime salen de la IDENTIDAD,
// nunca del proceso: en un agente root el HOME es /root y /run/user/0 no es de nadie, y leerlos acá
// dejaba la fuente vacía EN SILENCIO.
//
// LOS TRES DESENLACES, y el del medio es el que importa:
//
//	sin runtime             → nil, nil: la fuente no está, y el inventario sigue completo
//	runtime sin bus         → ERROR: se aborta el inventario de este latido
//	systemctl falla         → ERROR: completo o nada, igual que las otras fuentes
//
// «Runtime sin bus» NO es «fuente ausente», aunque lo parezca. Pasa en el arranque, cuando el
// agente llega antes que user@1000.service: si se tomara como ausente, el primer inventario saldría
// sin las usuario:*, el cerebro las podaría, y volverían en el latido siguiente — alertas que se
// abren y se cierran solas. Abortar da, en cambio, un ServicioSinNoticias transitorio y ninguna baja.
func enumerarUnitsDeUsuario(id identidadDeServicios, bus func(runtime string) estadoDelBus, ahora time.Time) ([]fleet.ReporteServicio, error) {
	switch bus(id.Runtime) {
	case runtimeAusente:
		return nil, nil
	case busAusente:
		return nil, fmt.Errorf("el manager de usuario de %s no contesta: %s existe y no tiene "+
			"systemd/private ni bus (¿el agente arrancó antes que user@%d.service?)", id.Nombre, id.Runtime, id.Uid)
	}
	args := append([]string{"--user", "show", "*.service", "--no-pager"},
		"--property="+strings.Join(propiedadesPedidas, ","))
	salida, hay, err := enumerarFuenteComo(id, "systemctl", args...)
	if err != nil {
		return nil, fmt.Errorf("units --user de %s: %w", id.Nombre, err)
	}
	if !hay {
		return nil, nil
	}
	return parsearSystemctlShowDeUsuario(salida, ahora, id.Home), nil
}
