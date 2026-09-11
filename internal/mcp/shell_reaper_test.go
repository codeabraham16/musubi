package mcp

import (
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// EL GOROUTINE QUE CIERRA LA FILA CUANDO LA SHELL REMOTA MUERE TIENE QUE CALLARSE CUANDO LA
// MATAMOS NOSOTROS.
//
// LO QUE PASÓ, MEDIDO EN CI EL 2026-09-10 (macOS). `TestAbrirDosVecesDevuelveLaMismaSesion` falló
// sin fallar ninguna aserción:
//
//	testing.go:1464: TempDir RemoveAll cleanup: unlinkat …/001/.musubi: directory not empty
//
// La pila del culpable, tomada con `runtime.Stack` justo en el instante en que el cuerpo del test
// volvía, lo dijo entero: el goroutine de `abrirShellConSesion` seguía vivo y estaba adentro de
// `_sqlite3InitOne` — o sea ABRIENDO UNA CONEXIÓN NUEVA. Una conexión nueva a SQLite CREA
// `memory.db-wal` y `memory.db-shm`. El `RemoveAll` del TempDir ya había borrado los hijos de
// `.musubi` y estaba por hacer el rmdir; los archivos reaparecieron en el medio.
//
// POR QUÉ ESE GOROUTINE ESTABA ESCRIBIENDO. `Terminado()` se cierra por dos motivos que el canal
// no distingue: la shell remota se murió, o `cerrarShell` llamó a `canal.Cerrar()`. En el segundo
// caso la fila YA ESTÁ CERRADA, con el estado y el motivo que eligió quien decidió cerrarla, y el
// goroutine sólo iba a repetir un UPDATE que `WHERE cerrada IS NULL` convierte en no-op.
//
// NO ES «UN TEST QUE PARPADEA». La misma carrera, del otro lado, es un apagado ordenado: se
// cierran las shells, se cierra el engine, y el goroutine llega tarde con su UPDATE. Ahí falla, y
// `cerrarShell` loguea «shell: no se pudo cerrar la fila de la sesión» — un WARN de auditoría que
// manda a alguien a revisar una bitácora que está perfecta. Ese WARN está en el log de CI, varias
// veces, emitido por tests que ya habían terminado.

// engineQueCuentaCierres cuenta los llamados a CerrarSesionShell sin cambiar ninguna semántica.
//
// SE CUENTA EL LLAMADO Y NO SE MIRA LA FILA, a propósito. La fila NO sirve de testigo: el UPDATE
// lleva `WHERE cerrada IS NULL`, así que la segunda escritura no deja rastro ninguno y una
// aserción sobre el estado guardado pasaría en verde con el defecto puesto — sería exactamente la
// guarda hueca que este repo ya pagó siete veces. Lo que hay que observar es el LLAMADO.
type engineQueCuentaCierres struct {
	memory.StorageBackend
	cierres atomic.Int32
}

func (e *engineQueCuentaCierres) CerrarSesionShell(id string, estado fleet.EstadoShell, motivo string, ahora time.Time) error {
	e.cierres.Add(1)
	return e.StorageBackend.CerrarSesionShell(id, estado, motivo, ahora)
}

// vigiasVivos cuenta los goroutines de `abrirShellConSesion` que hay AHORA en todo el proceso.
//
// SE CUENTA Y NO SE EXIGE CERO, y eso no es prolijidad: medido, este paquete deja vigías colgados
// para siempre. Varias pruebas abren una shell y no la cierran nunca, así que su goroutine se
// queda en `chan receive` hasta que termina el binario de test. Una espera que exigiera cero
// pasaría a depender de qué otras pruebas corrieron antes — verde o rojo según el `-run`, que es
// la peor clase de prueba.
func vigiasVivos() int {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	return strings.Count(string(buf[:n]), "abrirShellConSesion.func1")
}

// esperarVigiaTerminado espera a que MI vigía salga, partiendo de cuántos había antes.
//
// Es una espera POR ESTADO y no un sleep: si ya terminó vuelve enseguida, y si quedara colgado la
// prueba falla por timeout en vez de pasar por haber esperado poco. Los colgados de otras pruebas
// no se mueven, así que la única baja posible es la propia.
func esperarVigiaTerminado(t *testing.T, antes int) {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for {
		if vigiasVivos() < antes {
			return
		}
		if time.Now().After(limite) {
			t.Fatalf("el vigía de esta sesión no salió en 5 s: había %d y hay %d. Las dos lecturas "+
				"posibles son que quedó colgado o que nunca existió; cuál de las dos lo dice "+
				"TestSiLaShellRemotaSeMuereSolaElVigiaCierraLaFila, que mide lo segundo",
				antes, vigiasVivos())
		}
		time.Sleep(2 * time.Millisecond)
	}
}

// esperarCierreDelEngine espera a que el engine reciba su primer cierre.
//
// Va por el CONTADOR y no por los goroutines porque el caso que la usa —la shell que se muere
// sola— puede terminar antes de que la prueba llegue a tomar una línea de base.
func esperarCierreDelEngine(t *testing.T, contado *engineQueCuentaCierres) {
	t.Helper()
	limite := time.Now().Add(5 * time.Second)
	for contado.cierres.Load() == 0 {
		if time.Now().After(limite) {
			t.Fatal("la shell remota murió y 5 s después nadie le cerró la fila")
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func servidorConEngineContado(t *testing.T) (*McpServer, *engineQueCuentaCierres) {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	contado := &engineQueCuentaCierres{StorageBackend: s.engine}
	s.engine = contado
	return s, contado
}

// DIRECCIÓN 1 — cuando el cierre lo pedimos nosotros, el vigía no vuelve a tocar el engine.
//
// Sabotaje que la pone roja: sacar la consulta a `s.shells.buscar` del goroutine de
// `abrirShellConSesion`. Quedan dos llamados en vez de uno.
func TestElVigiaNoReescribeLaFilaQueYaCerramosNosotros(t *testing.T) {
	s, contado := servidorConEngineContado(t)
	enrolarConShell(t, s, "casa", "nas")
	restaurar := fleet.SSHFalsoParaTest(t, "sleep 30")
	defer restaurar()

	r, e := callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell", map[string]any{"device": "nas"})
	if e != nil {
		t.Fatalf("apertura: %+v", e)
	}
	id, _ := jsonOf(t, r)["session_id"].(string)

	contado.cierres.Store(0)
	antes := vigiasVivos()
	// Un motivo que NO es el del vigía: es el caso real de `cerrarShellsVencidas`, donde el
	// cerebro mata la sesión por techo y el motivo que importa es el suyo.
	s.cerrarShell(id, fleet.ShellVencida, "pasó el techo de la sesión", time.Now())
	esperarVigiaTerminado(t, antes)

	if n := contado.cierres.Load(); n != 1 {
		t.Errorf("el cierre se le pidió al engine %d veces y tenía que ser 1: el vigía despertó por "+
			"el `Cerrar()` que hicimos nosotros y salió a escribir una fila que ya estaba cerrada. "+
			"Ese segundo llamado es el que, en un apagado, llega con el engine cerrado y loguea un "+
			"WARN de auditoría falso — y en CI abre una conexión SQLite mientras borran el directorio.", n)
	}
}

// DIRECCIÓN 2 — y sin embargo, cuando la shell remota se muere sola, la fila SÍ tiene que cerrarse.
//
// SIN ESTE CASO LA GUARDA DE ARRIBA LA SATISFACE BORRAR EL GOROUTINE ENTERO, que es el arreglo
// equivocado: la bitácora quedaría con sesiones «activas» de máquinas donde alguien tecleó `exit`
// hace horas, que es justo lo que ese vigía vino a evitar.
func TestSiLaShellRemotaSeMuereSolaElVigiaCierraLaFila(t *testing.T) {
	s, contado := servidorConEngineContado(t)
	enrolarConShell(t, s, "casa", "nas")
	// El `ssh` falso se muere solo, sin que nadie de este lado pida nada: es `exit` tecleado del
	// otro lado, o la red que se cayó.
	restaurar := fleet.SSHFalsoParaTest(t, "exit 0")
	defer restaurar()

	r, e := callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell", map[string]any{"device": "nas"})
	if e != nil {
		t.Fatalf("apertura: %+v", e)
	}
	id, _ := jsonOf(t, r)["session_id"].(string)

	contado.cierres.Store(0)
	esperarCierreDelEngine(t, contado)

	if n := contado.cierres.Load(); n != 1 {
		t.Fatalf("la shell remota murió y el engine recibió %d cierres: tenía que recibir 1", n)
	}
	ses, existe, err := s.engine.SesionShellPorID(id)
	if err != nil || !existe {
		t.Fatalf("la sesión %q no se pudo releer: existe=%v err=%v", id, existe, err)
	}
	if ses.Viva(time.Now()) {
		t.Errorf("la sesión sigue figurando viva después de que la shell remota terminó: la bitácora " +
			"mostraría a alguien adentro de una máquina de la que ya salió")
	}
	if !strings.Contains(ses.Error, "remota") {
		t.Errorf("el motivo guardado no dice que murió del otro lado: %q", ses.Error)
	}
}

// DIRECCIÓN 3 — Y LO QUE SOSTIENE A LAS DOS: `cerrarShell` desregistra ANTES de cerrar.
//
// Las dos pruebas de arriba se apoyan en un orden que ninguna de las dos mide. El vigía sabe que
// «lo cerramos nosotros» PORQUE al despertar el id ya no está en el registro; si `cerrarShell`
// cerrara el canal primero y desregistrara después, el vigía podría despertar en el medio, verse
// vigente, y volver el defecto entero.
//
// LO MEDÍ: sabotear ese orden deja las dos pruebas de arriba EN VERDE, 20 corridas seguidas. Con
// un `ssh` real, matar el proceso y esperar el `Wait()` tarda tanto que el `quitar` gana siempre
// — o sea que el verde de arriba no dice nada sobre el orden, sólo que la carrera es lenta. Un
// día que no lo sea, vuelve.
//
// Por eso esta prueba NO abre ninguna shell ni corre ningún goroutine: pone un canal espía en el
// registro y le pregunta, DESDE ADENTRO de `Cerrar()`, si el id todavía figura. Sin carrera que
// pueda salir para el otro lado.
type canalEspia struct {
	reg              *registroDeShells
	id               string
	fin              chan struct{}
	seguiaRegistrado bool
	cerrado          bool
}

func (c *canalEspia) Escribir([]byte) error              { return nil }
func (c *canalEspia) Leer(time.Duration) ([]byte, error) { return nil, nil }
func (c *canalEspia) Terminado() <-chan struct{}         { return c.fin }
func (c *canalEspia) Cerrar() error {
	// LA PREGUNTA, hecha en el único instante en que la respuesta significa algo.
	_, c.seguiaRegistrado = c.reg.buscar(c.id)
	c.cerrado = true
	return nil
}

func TestCerrarShellDesregistraAntesDeCerrarElCanal(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	espia := &canalEspia{reg: &s.shells, id: "sesion-inventada", fin: make(chan struct{})}
	s.shells.guardar(espia.id, espia)

	s.cerrarShell(espia.id, fleet.ShellCerrada, "da igual: no hay fila", time.Now())

	if !espia.cerrado {
		t.Fatal("cerrarShell no cerró el canal: sin eso el proceso remoto queda vivo y nadie lo audita")
	}
	if espia.seguiaRegistrado {
		t.Error("cuando cerrarShell cerró el canal, el id TODAVÍA estaba en el registro. " +
			"El vigía de abrirShellConSesion usa exactamente esa consulta para saber si el cierre " +
			"lo pidió alguien de este lado; despertando en esta ventana se ve vigente, sale a " +
			"escribir una fila que ya está cerrada, y vuelve la escritura al engine desde un " +
			"goroutine que ya nadie espera.")
	}
}
