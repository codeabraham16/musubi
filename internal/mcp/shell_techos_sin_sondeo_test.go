package mcp

import (
	"context"
	"errors"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// LOS TECHOS DE UNA SESIÓN DE SHELL NO PUEDEN COLGAR DEL SONDEO.
//
// `cerrarShellsVencidas` es lo ÚLTIMO que hace `barrerFlotaUnaVez`, y antes de llegar ahí el
// barrido tiene tres salidas tempranas que no tienen nada que ver con las shells:
//
//  1. el barrido anterior sigue corriendo (`flotaBusy`),
//  2. no se pudo leer la lista de proyectos con máquinas,
//  3. el contexto se canceló a mitad del recorrido de proyectos.
//
// En las tres, la función VUELVE y las sesiones vencidas siguen abiertas: un prompt en una
// máquina ajena que ya pasó su techo de vida o de inactividad, con su proceso remoto vivo.
//
// LA DIFERENCIA CON UN BARRIDO QUE SE ATRASA. Que un tick se saltee y el siguiente cierre la
// sesión es aceptable: el techo se aplica tarde. Lo que NO es aceptable es que la causa sea
// permanente — una base que no deja leer `devices`, o un barrido que quedó colgado contra una
// máquina que no responde — porque entonces ningún tick futuro llega a los techos tampoco, y el
// prompt queda abierto mientras dure la falla. Los techos existen para el caso en que del otro
// lado hay alguien de quien uno se está protegiendo (T5); apagarlos porque la SONDA anda mal es
// exactamente el acoplamiento que no puede existir.
//
// Esto es la misma familia que ya mordió a este repo: la guarda que está en N-1 de N caminos.

// motorSinListaDeProyectos es un almacén al que no se le puede preguntar qué proyectos tienen
// máquinas, y que por lo demás funciona.
//
// EMBEBE EL ALMACÉN REAL Y NO NIL, a propósito y al revés que `backendConListaIlegible`: lo que
// se mide acá es lo que pasa DESPUÉS de esa falla, así que `cerrarShellsVencidas` tiene que poder
// leer la sesión y cerrarla. Con un almacén nil, la prueba moriría de un panic y el defecto
// quedaría sin medir.
type motorSinListaDeProyectos struct {
	memory.StorageBackend
}

func (motorSinListaDeProyectos) ProyectosConDevices(int) ([]string, error) {
	return nil, errors.New("la tabla devices no se pudo leer")
}

// canalQueAnotaSuCierre es un canal de mentira que sólo registra si lo cerraron.
//
// No alcanza con mirar la fila: el canal es el proceso remoto: un `ssh -tt` vivo, con su shell del
// otro lado. Una fila cerrada con el canal abierto es la peor de las dos combinaciones, porque la
// bitácora dice que la sesión terminó y la puerta sigue abierta.
type canalQueAnotaSuCierre struct {
	fin     chan struct{}
	cerrado bool
}

func (c *canalQueAnotaSuCierre) Escribir([]byte) error              { return nil }
func (c *canalQueAnotaSuCierre) Leer(time.Duration) ([]byte, error) { return nil, nil }
func (c *canalQueAnotaSuCierre) Terminado() <-chan struct{}         { return c.fin }
func (c *canalQueAnotaSuCierre) Cerrar() error {
	c.cerrado = true
	return nil
}

// sesionVencidaRegistrada siembra una sesión que YA pasó su techo de vida, con su canal en el
// registro, y devuelve las dos puntas.
//
// La sesión se envejece por `Creada`, que es de donde sale `Vence`: son las dos horas de
// ShellVidaMax cumplidas, no una inactividad que alguien pueda discutir.
func sesionVencidaRegistrada(t *testing.T, s *McpServer) (string, *canalQueAnotaSuCierre) {
	t.Helper()
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID:  "dev-inventado",
		ProjectID: "casa",
		Principal: "op",
		Creada:    time.Now().Add(-3 * time.Hour),
	})
	if err != nil {
		t.Fatalf("no se pudo sembrar la sesión: %v", err)
	}
	if vencida, _ := ses.Vencida(time.Now()); !vencida {
		t.Fatalf("la sesión sembrada no está vencida (creada %s, vence %s): la prueba no mide nada",
			ses.Creada, ses.Vence)
	}
	canal := &canalQueAnotaSuCierre{fin: make(chan struct{})}
	s.shells.guardar(ses.ID, canal)
	return ses.ID, canal
}

// exigirCerrada falla si la sesión quedó abierta después del barrido.
func exigirCerrada(t *testing.T, s *McpServer, id string, canal *canalQueAnotaSuCierre, mundo string) {
	t.Helper()
	ses, existe, err := s.engine.SesionShellPorID(id)
	if err != nil || !existe {
		t.Fatalf("%s: no se pudo releer la sesión: existe=%v err=%v", mundo, existe, err)
	}
	if ses.Cerrada.IsZero() {
		t.Errorf("%s: la sesión venció hace una hora y el barrido la dejó ABIERTA en la bitácora. "+
			"Los techos de una shell los aplica el cerebro (T5); si cuelgan del sondeo, una falla "+
			"que no tiene nada que ver con las shells los apaga mientras dure.", mundo)
	}
	if !canal.cerrado {
		t.Errorf("%s: nadie cerró el canal de una sesión vencida: el proceso remoto sigue vivo y "+
			"el prompt sigue abierto en la máquina ajena.", mundo)
	}
}

// Sabotaje mecanizado: el cierre vuelve a depender de que no haya un barrido en vuelo, que es la
// forma mínima del defecto. El arreglo equivalente —la misma llamada como `defer` registrado
// antes de las salidas tempranas— NO se castiga.
//
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="\tif n := s.cerrarShellsVencidas(time.Now()); n > 0 {\n\t\tlogx.Info(\"flota: sesiones de shell cerradas por vencimiento\", \"sesiones\", n)\n\t}"
// arnes: a="\tif !s.flotaBusy.Load() {\n\t\tif n := s.cerrarShellsVencidas(time.Now()); n > 0 {\n\t\t\tlogx.Info(\"flota: sesiones de shell cerradas por vencimiento\", \"sesiones\", n)\n\t\t}\n\t}"
// arnes: arreglo_de="\tif n := s.cerrarShellsVencidas(time.Now()); n > 0 {\n\t\tlogx.Info(\"flota: sesiones de shell cerradas por vencimiento\", \"sesiones\", n)\n\t}"
// arnes: arreglo_a="\tdefer func() {\n\t\tif n := s.cerrarShellsVencidas(time.Now()); n > 0 {\n\t\t\tlogx.Info(\"flota: sesiones de shell cerradas por vencimiento\", \"sesiones\", n)\n\t\t}\n\t}()"
func TestLosTechosDeShellSeAplicanAunqueElSondeoNoCorra(t *testing.T) {
	// CONTROL — en el camino normal el barrido SÍ cierra la vencida. Sin este subtest, la guarda
	// la satisface borrar el barrido entero.
	t.Run("control: barrido normal", func(t *testing.T) {
		s := newTestServer(t, embedding.NoopProvider{})
		id, canal := sesionVencidaRegistrada(t, s)
		s.barrerFlotaUnaVez(t.Context())
		exigirCerrada(t, s, id, canal, "barrido normal")
	})

	// 1 — el barrido anterior sigue corriendo. Con 40 máquinas por SSH y una que no contesta, un
	// barrido puede quedarse colgado varios ticks; los techos de las shells no son suyos.
	t.Run("el barrido anterior sigue en vuelo", func(t *testing.T) {
		s := newTestServer(t, embedding.NoopProvider{})
		id, canal := sesionVencidaRegistrada(t, s)
		if !s.flotaBusy.CompareAndSwap(false, true) {
			t.Fatal("la bandera debería nacer libre")
		}
		s.barrerFlotaUnaVez(t.Context())
		exigirCerrada(t, s, id, canal, "barrido solapado")
	})

	// 2 — la lista de proyectos no se puede leer. Es la falla PERMANENTE: mientras dure, ningún
	// tick futuro llega a los techos tampoco.
	t.Run("la lista de proyectos es ilegible", func(t *testing.T) {
		s := newTestServer(t, embedding.NoopProvider{})
		id, canal := sesionVencidaRegistrada(t, s)
		s.engine = motorSinListaDeProyectos{StorageBackend: s.engine}
		s.barrerFlotaUnaVez(t.Context())
		exigirCerrada(t, s, id, canal, "lista de proyectos ilegible")
	})

	// 3 — el contexto se cancela a mitad del recorrido de proyectos, que es lo que pasa cuando el
	// cerebro se está apagando. Apagarse ordenadamente es JUSTO cuando conviene cerrar las
	// sesiones vencidas: lo que queda abierto al morir el proceso no lo cierra nadie hasta el
	// próximo arranque.
	//
	// HACE FALTA UNA MÁQUINA ENROLADA: sin un proyecto con máquinas el bucle no da ni una vuelta,
	// el `ctx.Done()` no se mira, y la prueba pasaría por no haber tocado el camino que mide.
	t.Run("el contexto se cancela a mitad", func(t *testing.T) {
		s := newTestServer(t, embedding.NoopProvider{})
		enrolarConShell(t, s, "casa", "nas")
		id, canal := sesionVencidaRegistrada(t, s)
		ctx, cancelar := context.WithCancel(t.Context())
		cancelar()
		s.barrerFlotaUnaVez(ctx)
		exigirCerrada(t, s, id, canal, "contexto cancelado")
	})
}
