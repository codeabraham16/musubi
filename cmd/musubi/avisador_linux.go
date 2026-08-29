//go:build linux

package main

// El avisador en Linux. Ver avisador.go para el porqué; acá está el CÓMO de este sistema.

import (
	"context"
	"os/exec"

	"musubi/internal/fleet"
)

// detectarAvisador mide las dos mitades: sesión gráfica Y herramienta.
//
// EL ORDEN DE LOS DOS CHEQUEOS DECIDE EL MENSAJE, y por eso importa. Se mira primero el
// escritorio: un servidor sin sesión gráfica no se arregla instalando zenity, y decirle a alguien
// que le falta un paquete cuando el problema es que el agente corre como servicio de sistema lo
// manda a instalar algo que no va a cambiar nada.
func detectarAvisador() fleet.CapacidadDeAvisar {
	hay, tipo := hayEscritorio()
	if !hay {
		return fleet.CapacidadDeAvisar{Motivo: "el agente no está en una sesión gráfica " +
			"(ni DISPLAY ni WAYLAND_DISPLAY): corre como servicio de sistema, o esta máquina no " +
			"tiene escritorio. Instalar un paquete no lo arregla"}
	}
	// zenity PRIMERO aunque notify-send sea más común: notify-send NO puede preguntar, sólo
	// avisar. Elegirlo dejaría a la máquina declarando que puede y devolviendo `no_se_pudo` en el
	// primer `pide` — la promesa vacía otra vez, sólo que un paso más adelante.
	if h := primeroDisponible("zenity", "kdialog"); h != "" {
		return fleet.CapacidadDeAvisar{Puede: true, Herramienta: h}
	}
	if h := primeroDisponible("notify-send"); h != "" {
		// Alcanza para `avisa` y NO para `pide`. Se declara que NO puede, a propósito: la
		// capacidad que el eje mide es la de PREGUNTAR, y media capacidad reportada como entera
		// es lo que endurecería mal un `pide`.
		return fleet.CapacidadDeAvisar{Motivo: "hay sesión " + tipo + " y sólo `notify-send`, " +
			"que avisa pero no pregunta: instalá zenity o kdialog para poder usar `pide`"}
	}
	return fleet.CapacidadDeAvisar{Motivo: "hay sesión " + tipo + " y ninguna herramienta de " +
		"diálogo: instalá zenity, kdialog o al menos notify-send"}
}

// correrAvisador muestra el mensaje SIN esperar respuesta.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// UN AVISO QUE ESPERA UN CLIC NO ES UN AVISO, ES UNA PREGUNTA SIN OPCIONES
//
// `zenity --warning` BLOQUEA hasta que alguien aprieta OK. Usarlo tal cual dejaba al agente
// esperando los diez segundos del timeout en CADA aviso — y el agente atiende los comandos EN
// SERIE, así que esa máquina se queda sin atender nada más: ni exec, ni pantalla, ni shell. El
// cerebro la vería latiendo y muda, que es el peor estado porque parece sano.
//
// Por eso se prefiere `notify-send`, que es una notificación de escritorio y vuelve en el acto,
// aunque para MEDIR la capacidad no alcance (no sabe preguntar). Son dos preguntas distintas:
// «¿con qué aviso?» y «¿con qué pregunto?», y la herramienta buena para una es mala para la otra.
//
// El zenity de respaldo lleva `--timeout`: si no hay notify-send, el aviso se cierra solo en vez
// de quedarse esperando a alguien que quizás no está.
func correrAvisador(ctx context.Context, c fleet.CapacidadDeAvisar, texto string) error {
	if h := primeroDisponible("notify-send"); h != "" {
		// `-u critical` para que no se lo trague el modo «no molestar»: el aviso dice que alguien
		// está entrando a esta máquina, y es exactamente el que no hay que silenciar.
		return exec.CommandContext(ctx, h, "-u", "critical", "-a", "Musubi", "Musubi", texto).Run()
	}
	if c.Herramienta == "kdialog" {
		// `--passivepopup` ya es no bloqueante; el 10 es cuántos segundos queda en pantalla.
		return exec.CommandContext(ctx, "kdialog", "--passivepopup", texto, "10").Run()
	}
	// `--warning` y no `--info`: el aviso dice que alguien está entrando a esta máquina, y el
	// ícono es la mitad del mensaje para quien lo ve de reojo.
	return exec.CommandContext(ctx, "zenity", "--warning", "--timeout=10", "--title=Musubi",
		"--text="+texto).Run()
}

// correrPregunta muestra la pregunta y espera. El CÓDIGO DE SALIDA es la respuesta.
//
// EL CERO NO ES «ANDUVO», ES «DIJO QUE SÍ». En zenity y kdialog, exit 0 = aceptó y exit 1 =
// canceló, así que la convención de shell —cero es éxito— acá significa otra cosa. Leerlo como
// «el comando funcionó» concedería acceso cada vez que la ventana se abre.
func correrPregunta(ctx context.Context, c fleet.CapacidadDeAvisar, texto string) fleet.RespuestaAviso {
	var cmd *exec.Cmd
	switch c.Herramienta {
	case "kdialog":
		cmd = exec.CommandContext(ctx, "kdialog", "--warningyesno", texto, "--title", "Musubi")
	default:
		cmd = exec.CommandContext(ctx, "zenity", "--question", "--title=Musubi", "--text="+texto,
			"--ok-label=Permitir", "--cancel-label=No permitir")
	}
	if err := cmd.Run(); err != nil {
		// Cualquier salida distinta de cero es una NEGATIVA. Incluye el «no permitir», la ventana
		// cerrada con la X, y la herramienta que no arrancó — y las tres se tratan igual a
		// propósito: ante la duda, este eje no abre. El caso del plazo vencido lo separa el
		// llamador, que mira el contexto.
		return fleet.RespuestaNegada
	}
	return fleet.RespuestaConcedida
}
