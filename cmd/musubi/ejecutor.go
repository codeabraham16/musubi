package main

// ejecutor.go es la parte del agente que EJECUTA lo que el cerebro le pide. Track «Control de
// flota», S5.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LO QUE ESTE ARCHIVO NO HACE, Y ES EL DISEÑO
//
//   - NO interpreta una shell. Recibe un argv y lo ejecuta tal cual (`exec.Command(argv[0],
//     argv[1:]...)`). Quien quiera una shell la pide explícita: ["sh", "-c", "..."]. No es por
//     inyección —quien llegó hasta acá ya está autorizado a ejecutar—, es porque una cadena que
//     a veces pasa por shell y a veces no hace que el mismo comando de mantenimiento haga cosas
//     distintas en dos máquinas, y porque el comando REGISTRADO tiene que ser el comando CORRIDO.
//   - NO decide si puede. Eso ya lo decidió el cerebro con la compuerta de S3 antes de encolar.
//   - NO guarda nada localmente. El resultado va al cerebro y ahí queda la bitácora.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"musubi/internal/fleet"
)

// comandoRecibido es un pedido tal como llega en la respuesta del latido.
//
// ES UN ALIAS DEL TIPO DEL CONTRATO, no una copia. Era una copia byte a byte de
// fleet.ComandoParaElAgente, y ése es el patrón que dejó dos campos de la respuesta sin receptor
// —el porqué está en internal/fleet/protocolo.go—. El alias conserva el nombre local, que en este
// paquete se lee mejor (acá el comando se RECIBE), sin que haya dos declaraciones que mantener.
type comandoRecibido = fleet.ComandoParaElAgente

// resultadoDeComando es lo que se le reporta al cerebro.
type resultadoDeComando struct {
	ComandoID string `json:"command_id"`
	ExitCode  *int   `json:"exit_code"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Error     string `json:"error"`
}

// ejecutar corre un comando y devuelve el resultado. NUNCA devuelve error al llamador: un
// comando que falla es un RESULTADO, no un fallo del agente.
//
// La distinción vive en el par (ExitCode, Error):
//   - ExitCode no nil  ⇒ el proceso corrió y terminó. 0 o 47, es su resultado.
//   - Error no vacío   ⇒ falló el CANAL: no se pudo lanzar el ejecutable, venció el timeout.
//
// Confundirlos haría que un `grep` que no encuentra nada (exit 1, perfectamente normal) se lea
// como una máquina rota.
// `base` y `token` son CÓMO VOLVER AL CEREBRO, y sólo los usa la shell interactiva (S5c): a
// diferencia de un comando común —que corre, termina y se reporta— una sesión de shell abre sus
// propios canales de vuelta mientras dura. El resto de los comandos los ignora.
func ejecutar(c comandoRecibido, base, token string) resultadoDeComando {
	res := resultadoDeComando{ComandoID: c.ID}

	argv := fleet.LimpiarArgv(c.Argv)
	if len(argv) == 0 {
		res.Error = "comando vacío"
		return res
	}
	// Las operaciones INTERNAS del canal se interceptan antes de intentar lanzarlas como
	// binario. `musubi:pantalla` no es un ejecutable del host y nunca debe llegar a exec.Command:
	// si llegara, el error diría «no such file» y —peor— el mensaje podría arrastrar la
	// contraseña que va en el argv.
	if strings.HasPrefix(argv[0], "musubi:") {
		if argv[0] == "musubi:pantalla" {
			return aplicarSesionPantalla(comandoRecibido{ID: c.ID, Argv: argv, TimeoutSeg: c.TimeoutSeg})
		}
		if argv[0] == comandoShellAgente {
			// La shell interactiva (S5c). BLOQUEA todo lo que dure la sesión, y eso es correcto:
			// el resultado que se reporta al final es «la sesión terminó», que es lo que la
			// bitácora tiene que registrar. El timeout del comando NO la acota — la acotan los
			// techos del cerebro y, como red de seguridad, el techo local del propio agente.
			return atenderShellDelCerebro(c.ID, base, token, argv)
		}
		if argv[0] == comandoAvisarAgente {
			return atenderAviso(c.ID, argv)
		}
		if argv[0] == comandoPreguntarAgente {
			return atenderPregunta(c.ID, argv)
		}
		res.Error = "operación interna desconocida: " + argv[0]
		return res
	}

	timeout := time.Duration(c.TimeoutSeg) * time.Second
	if timeout <= 0 || timeout > fleet.ComandoTimeoutMax {
		timeout = fleet.ComandoTimeoutDefault
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	// WaitDelay: sin esto el timeout no libera al agente.
	//
	// CommandContext mata al proceso al vencer, pero `Run` vuelve cuando se cierran las TUBERÍAS
	// —y un comando que dejó un hijo en background (`sh -c "algo &"`) se las lleva abiertas—.
	// Acá el costo es peor que en el runner de SSH: el agente atiende los comandos de forma
	// SECUENCIAL, así que uno solo colgado deja a la máquina sin atender nada más, y el cerebro
	// la ve latiendo pero muda.
	//
	// Lo encontró una prueba del runner de SSH, que tiene exactamente el mismo patrón.
	cmd.WaitDelay = 2 * time.Second
	// El proceso NO hereda stdin. Un comando que espera una entrada que nunca va a llegar
	// bloquearía hasta el timeout; con stdin cerrado, lee EOF y termina solo.
	cmd.Stdin = nil

	// Buffers ACOTADOS: un `cat` sobre un log de 4 GB no puede llenar la RAM de la máquina que
	// este agente existe para cuidar. El corte se hace acá, en el productor, no sólo en el
	// cerebro — mandar 4 GB por la red para que el otro lado los descarte sería absurdo.
	var so, se bufferAcotado
	so.tope, se.tope = fleet.SalidaMaxBytes, fleet.SalidaMaxBytes
	cmd.Stdout, cmd.Stderr = &so, &se

	err := cmd.Run()
	res.Stdout, res.Stderr = so.texto(), se.texto()

	switch {
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		// CommandContext ya mató al proceso. Se dice el timeout aplicado para que quien lea la
		// bitácora sepa si el comando era lento o el margen era corto.
		res.Error = fmt.Sprintf("el comando excedió su timeout de %s y fue terminado", timeout)
	case err == nil:
		cero := 0
		res.ExitCode = &cero
	default:
		var salida *exec.ExitError
		if errors.As(err, &salida) {
			code := salida.ExitCode()
			res.ExitCode = &code
		} else {
			// No se pudo lanzar: ejecutable inexistente, sin permisos, cwd que no existe.
			res.Error = err.Error()
		}
	}
	return res
}

// bufferAcotado acumula hasta `tope` bytes y descarta el resto, dejando la marca.
//
// Descarta en vez de fallar a propósito: un comando cuya salida se pasa de largo igual corrió, y
// su exit code sigue siendo información. Cortar la ejecución por eso sería perder el dato útil
// por proteger la memoria, cuando se pueden las dos cosas.
type bufferAcotado struct {
	buf      bytes.Buffer
	tope     int
	descarto bool
}

func (b *bufferAcotado) Write(p []byte) (int, error) {
	if libre := b.tope - b.buf.Len(); libre > 0 {
		if len(p) <= libre {
			b.buf.Write(p)
		} else {
			b.buf.Write(p[:libre])
			b.descarto = true
		}
	} else if len(p) > 0 {
		b.descarto = true
	}
	// SIEMPRE se reporta que se escribió todo. Devolver menos haría que el proceso hijo reciba
	// un error de escritura (EPIPE) y muera por culpa del agente, cuando lo único que pasó es
	// que su salida es larga.
	return len(p), nil
}

func (b *bufferAcotado) texto() string {
	if b.descarto {
		return b.buf.String() + fleet.AvisoTruncado
	}
	return b.buf.String()
}

// reportar manda el resultado al cerebro. Best-effort: si falla, se pierde el resultado pero NO
// el registro del pedido — la bitácora ya tiene la fila desde que se encoló (F1), y el comando
// queda visible como `entregado` sin terminar, que es información honesta.
func reportar(urlBase, token string, r resultadoDeComando) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, rutaResultado(urlBase), bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := clienteLatido.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("el cerebro respondió %d al resultado", resp.StatusCode)
	}
	return nil
}
