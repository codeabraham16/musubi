package fleet

// shell_agente.go es el canal de una shell interactiva en una máquina CON AGENTE (Tier A).
// Track «Control de flota», S5c.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// LA DIFERENCIA CON TIER B, EN UNA LÍNEA: acá el cerebro no puede ir a buscar nada.
//
// A un Tier B se le entra por SSH y el pty lo pone el sshd de la otra punta. A un Tier A no le
// entra NADIE: es una máquina detrás de un NAT, sin puertos abiertos, que sólo sabe SALIR hacia
// el cerebro. Es toda la razón de ser del Tier A.
//
// Así que el canal es un ENCUENTRO: el cerebro deja la sesión esperando, el agente se entera por
// la cola de comandos y se conecta desde su lado. Este tipo es el punto de encuentro — dos
// buffers, uno por sentido, con la misma contrapresión que el resto.
// ────────────────────────────────────────────────────────────────────────────────────────────

import (
	"sync"
	"time"
)

// CanalAgente es un CanalInteractivo cuyo otro extremo es un agente que se conecta solo.
//
// Los dos buffers NO son simétricos en su uso y conviene tener claro quién es quién:
//
//	haciaLaMaquina  — lo TECLEA la persona, lo LEE el agente
//	haciaLaPersona  — lo IMPRIME la máquina, lo LEE la persona
//
// La confusión de nombres es la trampa más fácil de este archivo: «entrada» y «salida» significan
// lo contrario según de qué lado se pare uno. Por eso los campos se llaman por su DESTINO, que es
// lo único que no cambia según quién mire.
type CanalAgente struct {
	haciaLaMaquina *bufferInteractivo
	haciaLaPersona *bufferInteractivo

	unaVez sync.Once
	fin    chan struct{}

	// enganchado se cierra cuando el agente aparece. Sirve para distinguir «la máquina no
	// contestó todavía» de «la máquina contestó y no imprime nada», que son lo mismo mirando el
	// buffer y cosas muy distintas para quien está esperando un prompt.
	mu           sync.Mutex
	engancho     chan struct{}
	yaEnganchado bool
}

// NuevoCanalAgente crea el punto de encuentro, todavía sin agente del otro lado.
func NuevoCanalAgente() *CanalAgente {
	return &CanalAgente{
		haciaLaMaquina: nuevoBufferInteractivo(),
		haciaLaPersona: nuevoBufferInteractivo(),
		fin:            make(chan struct{}),
		engancho:       make(chan struct{}),
	}
}

// ── El lado de la PERSONA (lo usa el relay HTTP, igual que con un canal SSH) ────────────────

// Escribir manda lo tecleado hacia la máquina.
func (c *CanalAgente) Escribir(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	if !c.haciaLaMaquina.escribir(p) {
		return ErrCanalCerrado
	}
	return nil
}

// Leer devuelve lo que la máquina imprimió.
func (c *CanalAgente) Leer(espera time.Duration) ([]byte, error) {
	return c.haciaLaPersona.leer(espera)
}

func (c *CanalAgente) Terminado() <-chan struct{} { return c.fin }

// Cerrar mata el encuentro por los dos lados. Idempotente.
func (c *CanalAgente) Cerrar() error {
	c.unaVez.Do(func() {
		c.haciaLaMaquina.cerrar()
		c.haciaLaPersona.cerrar()
		close(c.fin)
	})
	return nil
}

// ── El lado del AGENTE ──────────────────────────────────────────────────────────────────────

// Enganchar anuncia que el agente llegó. Idempotente: un agente que se reconecta tras un hipo de
// red no tiene que fallar.
func (c *CanalAgente) Enganchar() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.yaEnganchado {
		c.yaEnganchado = true
		close(c.engancho)
	}
}

// Enganchado dice si el agente ya apareció.
func (c *CanalAgente) Enganchado() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.yaEnganchado
}

// EsperarAgente bloquea hasta que el agente aparezca, se cierre el canal, o venza la espera.
//
// Existe para que quien abre la shell reciba un prompt y no una pantalla en blanco: un Tier A se
// entera de que lo llamaron en su próximo latido, así que hay una demora REAL de hasta un
// intervalo de latido. Decirlo es mejor que dejar a alguien mirando un cursor quieto sin saber si
// se colgó.
func (c *CanalAgente) EsperarAgente(espera time.Duration) bool {
	select {
	case <-c.engancho:
		return true
	case <-c.fin:
		return false
	case <-time.After(espera):
		return false
	}
}

// LeerDeLaPersona lo usa el AGENTE para recoger lo tecleado.
func (c *CanalAgente) LeerDeLaPersona(espera time.Duration) ([]byte, error) {
	return c.haciaLaMaquina.leer(espera)
}

// EscribirALaPersona lo usa el AGENTE para entregar lo que imprimió el pty.
func (c *CanalAgente) EscribirALaPersona(p []byte) error {
	if len(p) == 0 {
		return nil
	}
	if !c.haciaLaPersona.escribir(p) {
		return ErrCanalCerrado
	}
	return nil
}
