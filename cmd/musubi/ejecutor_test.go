package main

import (
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// Un comando que falla es un RESULTADO, no un fallo del agente. La distinción vive en el par
// (ExitCode, Error) y confundirla haría que un `grep` que no encuentra nada —exit 1, normal— se
// lea como una máquina rota.
//
// Sabotaje que la hace fallar: poner el exit code distinto de cero en `Error`.
func TestUnExitDistintoDeCeroEsResultadoNoError(t *testing.T) {
	casos := []struct {
		nombre     string
		argv       []string
		quieroExit *int
		quieroErr  bool
	}{
		{"comando que anda", []string{"true"}, iptr(0), false},
		{"comando que falla", []string{"false"}, iptr(1), false},
		{"exit code propio", []string{"sh", "-c", "exit 47"}, iptr(47), false},
		{"ejecutable inexistente", []string{"no-existe-este-binario-jamas"}, nil, true},
		{"argv vacío", nil, nil, true},
	}
	for _, c := range casos {
		res := ejecutar(comandoRecibido{ID: "x", Argv: c.argv, TimeoutSeg: 5}, "", "")
		if c.quieroErr {
			if res.Error == "" {
				t.Errorf("%s: esperaba un error de CANAL, no lo hubo", c.nombre)
			}
			if res.ExitCode != nil {
				t.Errorf("%s: un fallo de canal no debería traer exit code (%d)", c.nombre, *res.ExitCode)
			}
			continue
		}
		if res.Error != "" {
			t.Errorf("%s: un comando que corrió no debería dar error de canal: %s", c.nombre, res.Error)
		}
		if res.ExitCode == nil {
			t.Errorf("%s: falta el exit code", c.nombre)
		} else if *res.ExitCode != *c.quieroExit {
			t.Errorf("%s: exit %d, esperaba %d", c.nombre, *res.ExitCode, *c.quieroExit)
		}
	}
}

// F7 — NO hay shell implícito. `echo $HOME` con argv debe imprimir la cadena literal, no
// expandirla: si expandiera, el comando registrado en la bitácora no sería el comando corrido.
//
// Sabotaje: pasar el argv por `sh -c`.
func TestNoHayShellImplicito(t *testing.T) {
	res := ejecutar(comandoRecibido{ID: "x", Argv: []string{"echo", "$HOME y *"}, TimeoutSeg: 5}, "", "")
	if res.Error != "" {
		t.Fatalf("error inesperado: %s", res.Error)
	}
	got := strings.TrimSpace(res.Stdout)
	if got != "$HOME y *" {
		t.Errorf("la salida fue %q: el argv pasó por una shell y expandió variables/globs", got)
	}
	// Y con shell EXPLÍCITA sí expande: la capacidad está, sólo hay que pedirla.
	res2 := ejecutar(comandoRecibido{ID: "y", Argv: []string{"sh", "-c", "echo $((2+3))"}, TimeoutSeg: 5}, "", "")
	if strings.TrimSpace(res2.Stdout) != "5" {
		t.Errorf("con shell explícita debería expandir, obtuve %q", res2.Stdout)
	}
}

// F8 — el timeout MATA el comando y lo dice.
// Sabotaje: usar exec.Command en vez de exec.CommandContext → el proceso sobrevive al agente.
func TestElTimeoutMataElComando(t *testing.T) {
	arranque := time.Now()
	res := ejecutar(comandoRecibido{ID: "x", Argv: []string{"sleep", "30"}, TimeoutSeg: 1}, "", "")
	tardo := time.Since(arranque)

	if tardo > 5*time.Second {
		t.Errorf("el comando tardó %s: el timeout no lo mató", tardo)
	}
	if res.Error == "" || !strings.Contains(res.Error, "timeout") {
		t.Errorf("el resultado no explica que venció el timeout: %+v", res)
	}
	if res.ExitCode != nil {
		t.Errorf("un comando matado por timeout no debería traer exit code (%d)", *res.ExitCode)
	}
}

// F9 — la salida se acota Y deja la marca. Un `cat` sobre un log de 4 GB no puede llenar la RAM
// de la máquina que este agente existe para cuidar.
//
// Sabotaje: usar un bytes.Buffer sin tope.
func TestLaSalidaSeAcotaEnElAgenteYDejaLaMarca(t *testing.T) {
	// Genera bastante más que el tope.
	res := ejecutar(comandoRecibido{
		ID: "x", Argv: []string{"sh", "-c", "yes AAAAAAAAAAAAAAAA | head -c 500000"}, TimeoutSeg: 20}, "", "")
	if res.Error != "" {
		t.Fatalf("error inesperado: %s", res.Error)
	}
	if len(res.Stdout) > fleet.SalidaMaxBytes+len(fleet.AvisoTruncado) {
		t.Errorf("la salida no se acotó: %d bytes (tope %d)", len(res.Stdout), fleet.SalidaMaxBytes)
	}
	if !strings.Contains(res.Stdout, "truncada") {
		t.Error("se truncó SIN marca: quien lea el log saca conclusiones de datos que no están")
	}
	// Y el comando NO murió por culpa del buffer: siguió hasta terminar bien.
	if res.ExitCode == nil || *res.ExitCode != 0 {
		t.Errorf("el buffer acotado mató al comando: %+v", res)
	}
}

// El buffer acotado nunca reporta escrituras cortas: si lo hiciera, el proceso hijo recibiría
// EPIPE y moriría por culpa del agente, cuando lo único que pasó es que su salida es larga.
func TestElBufferAcotadoNoRompeAlProcesoHijo(t *testing.T) {
	b := &bufferAcotado{tope: 10}
	n, err := b.Write([]byte(strings.Repeat("x", 100)))
	if err != nil {
		t.Fatalf("el buffer devolvió error: %v", err)
	}
	if n != 100 {
		t.Errorf("reportó %d de 100 bytes escritos: el hijo recibiría EPIPE y moriría", n)
	}
	if !strings.Contains(b.texto(), "truncada") {
		t.Error("descartó sin dejar la marca")
	}
	// Y una escritura que entra completa no marca nada.
	b2 := &bufferAcotado{tope: 10}
	b2.Write([]byte("corto"))
	if strings.Contains(b2.texto(), "truncada") {
		t.Error("marcó truncado sin haber truncado")
	}
}

// El comando no hereda stdin: uno que espera una entrada que nunca va a llegar tiene que leer
// EOF y terminar, no bloquear hasta el timeout.
func TestElComandoNoHeredaStdin(t *testing.T) {
	arranque := time.Now()
	res := ejecutar(comandoRecibido{ID: "x", Argv: []string{"cat"}, TimeoutSeg: 10}, "", "")
	if tardo := time.Since(arranque); tardo > 3*time.Second {
		t.Errorf("`cat` sin stdin tardó %s: heredó una entrada que nunca cierra", tardo)
	}
	if res.ExitCode == nil || *res.ExitCode != 0 {
		t.Errorf("`cat` con stdin cerrado debería salir 0: %+v", res)
	}
}

func iptr(v int) *int { return &v }

// EL TIMEOUT TIENE QUE LIBERAR AL AGENTE, aunque el comando deje un hijo en background.
//
// Lo encontró una prueba del runner de SSH, que tiene el mismo patrón. `CommandContext` mata al
// proceso al vencer, pero `Run` vuelve cuando se cierran las TUBERÍAS — y un hijo en background
// se las lleva abiertas. Medido en el otro ejecutor: un timeout de 1 s tardaba 30.
//
// Acá el costo es peor: el agente atiende los comandos SECUENCIALMENTE, así que uno colgado deja
// a la máquina sin atender nada más, y el cerebro la ve latiendo pero muda.
//
// Sabotaje que la hace fallar: quitar `cmd.WaitDelay`.
func TestUnHijoEnBackgroundNoDerrotaElTimeout(t *testing.T) {
	arranque := time.Now()
	res := ejecutar(comandoRecibido{
		ID: "x", Argv: []string{"sh", "-c", "sleep 30 & echo lanzado"}, TimeoutSeg: 1}, "", "")
	tardo := time.Since(arranque)

	if tardo > 8*time.Second {
		t.Fatalf("el ejecutor tardó %s con un timeout de 1s: un hijo en background se llevó las "+
			"tuberías y `Run` siguió esperando. El agente queda colgado y no atiende nada más.", tardo)
	}
	// El comando en sí terminó bien y rápido; lo que quedó es el hijo.
	t.Logf("liberó en %s · exit=%v error=%q", tardo, res.ExitCode, res.Error)
}
