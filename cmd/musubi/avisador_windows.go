//go:build windows

package main

// El avisador en Windows, y acá vive la trampa más grande de este archivo.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// UN SERVICIO DE WINDOWS NO PUEDE DIBUJARLE A NADIE, Y NO DA ERROR
//
// Desde Vista, los servicios corren en la SESIÓN 0, aislada de la del usuario. Un MessageBox
// lanzado desde ahí se crea sin problema, no falla, y NADIE LO VE: queda en un escritorio que no
// se muestra. El proceso se queda esperando un clic que nunca llega hasta que vence el plazo.
//
// Eso es exactamente el modo de falla que este eje existe para evitar, y con la forma peor: un
// `pide` que parece funcionar, tarda sesenta segundos y termina en «nadie contestó» — cuando la
// verdad es que no había dónde preguntar. Por eso la detección mira la SESIÓN y no si el binario
// existe.
//
// En la flota de hoy el agente de Windows corre por Tarea Programada con `-AtLogOn`, o sea DENTRO
// de la sesión del usuario. Ahí sí se ve. Un agente instalado como servicio, no — y este archivo
// lo distingue en vez de suponerlo.

import (
	"context"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"musubi/internal/fleet"
)

// STDLIB PURA, IGUAL QUE EL COLECTOR DE WINDOWS. `golang.org/x/sys/windows` trae
// ProcessIdToSessionId envuelto y listo — y usarla la promovería de INDIRECTA a DIRECTA, o sea la
// séptima dependencia de un repo que tiene seis por decisión. `syscall.NewLazyDLL` resuelve el
// símbolo en tiempo de ejecución y no cuesta nada: es el mismo patrón que colector_windows.go usa
// para GetSystemTimes y GlobalMemoryStatusEx.
var (
	kernel32Aviso            = syscall.NewLazyDLL("kernel32.dll")
	procProcessIdToSessionId = kernel32Aviso.NewProc("ProcessIdToSessionId")
)

// detectarAvisador comprueba que este proceso esté en una sesión INTERACTIVA.
//
// `WTSGetActiveConsoleSessionId` da la sesión que está en la consola física; la del proceso sale
// de su token. Si no coinciden —o si la del proceso es 0— no hay a quién dibujarle.
func detectarAvisador() fleet.CapacidadDeAvisar {
	var sesionProceso uint32
	// El valor de retorno es un BOOL: 0 = falló. `err` de Call() SIEMPRE trae algo (el último
	// error del hilo, que puede ser de una llamada anterior y perfectamente ser "operación
	// completada con éxito"), así que mirarlo en vez del retorno da falsos negativos — el error
	// clásico de este patrón, y por el que colector_windows.go también mira el retorno.
	ok, _, err := procProcessIdToSessionId.Call(
		uintptr(syscall.Getpid()), uintptr(unsafe.Pointer(&sesionProceso)))
	if ok == 0 {
		return fleet.CapacidadDeAvisar{Motivo: "no se pudo determinar la sesión del proceso: " + err.Error()}
	}
	if sesionProceso == 0 {
		return fleet.CapacidadDeAvisar{Motivo: "el agente corre en la SESIÓN 0 (como servicio de " +
			"Windows), que está aislada del escritorio del usuario: un diálogo lanzado desde ahí " +
			"no falla y tampoco lo ve nadie. Corré el agente como Tarea Programada con -AtLogOn"}
	}
	if _, err := exec.LookPath("powershell.exe"); err != nil {
		return fleet.CapacidadDeAvisar{Motivo: "no hay powershell.exe en el PATH"}
	}
	return fleet.CapacidadDeAvisar{Puede: true, Herramienta: "powershell"}
}

// escaparPS tapa las comillas simples para interpolar en una cadena de PowerShell.
//
// El texto lo arma el cerebro y se interpola en un guion que se va a EJECUTAR. En PowerShell una
// comilla simple se escapa duplicándola; sin esto, el texto cierra la cadena y lo que sigue corre
// como código en la máquina de otra persona.
func escaparPS(s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	// Los saltos de línea se vuelven espacios: el guion viaja en UNA línea de -Command, y un
	// salto lo partiría en dos sentencias.
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.ReplaceAll(s, "\n", " ")
}

const preludioPS = "Add-Type -AssemblyName System.Windows.Forms | Out-Null; "

func correrAvisador(ctx context.Context, _ fleet.CapacidadDeAvisar, texto string) error {
	guion := preludioPS + "[System.Windows.Forms.MessageBox]::Show('" + escaparPS(texto) +
		"','Musubi',[System.Windows.Forms.MessageBoxButtons]::OK," +
		"[System.Windows.Forms.MessageBoxIcon]::Warning) | Out-Null"
	return exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", guion).Run()
}

func correrPregunta(ctx context.Context, _ fleet.CapacidadDeAvisar, texto string) fleet.RespuestaAviso {
	// SE IMPRIME LA RESPUESTA Y NO SE MIRA EL CÓDIGO DE SALIDA: PowerShell sale con 0 tanto si el
	// usuario apretó Sí como si apretó No. Quedarse con el exit code concedería acceso siempre
	// que la ventana llegara a abrirse.
	//
	// `DefaultDesktopOnly` no se usa a propósito: en la sesión 0 hace que la ventana aparezca en
	// el escritorio por defecto —donde tampoco la ve nadie— y disimularía justamente el caso que
	// la detección de arriba existe para atrapar.
	guion := preludioPS + "$r = [System.Windows.Forms.MessageBox]::Show('" + escaparPS(texto) +
		"','Musubi',[System.Windows.Forms.MessageBoxButtons]::YesNo," +
		"[System.Windows.Forms.MessageBoxIcon]::Warning); Write-Output $r"
	salida, err := exec.CommandContext(ctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", guion).Output()
	if err != nil {
		return fleet.RespuestaNegada
	}
	if strings.EqualFold(strings.TrimSpace(string(salida)), "Yes") {
		return fleet.RespuestaConcedida
	}
	return fleet.RespuestaNegada
}
