package mcp

import (
	"context"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// LOS TECHOS DE UNA SESIÓN DE SHELL NO PUEDEN COLGAR DEL SONDEO.
//
// Los techos los aplica EL CEREBRO y no la máquina remota (T5): si dependieran del otro lado, una
// máquina comprometida se los saltearía. Y un techo que se apaga por algo que no tiene nada que ver
// con las shells no es un techo.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA HISTORIA, EN DOS VUELTAS
//
// Primero `cerrarShellsVencidas` era lo ÚLTIMO de `barrerFlotaUnaVez`, detrás de tres salidas
// tempranas: un barrido anterior en vuelo (`flotaBusy`), la lista de proyectos ilegible, el
// contexto cancelado. Se la subió al principio del barrido, y esta prueba cubría esos mundos.
//
// Después A136 midió el mundo que ninguna posición dentro del barrido cubre: que el barrido NO
// CORRA. Con `fleet.probe_minutes` negativo RunFlotaScheduler vuelve al instante, y ninguna sesión
// vencida se cerraba nunca. El 410 de autorizarShell rechaza cada pedido, pero no mata el canal, y
// en una Tier B el canal es el `ssh` que sostiene el cerebro: el proceso remoto seguía vivo y la
// bitácora mostraba la sesión «activa». Desde A136 las cierra su propio vigía, RunTechosDeShell, y
// esta prueba lo corre DE VERDAD en cada uno de esos mundos.
//
// LA SESIÓN SE SIEMBRA DESPUÉS DE LA PRIMERA PASADA. El vigía hace una al arrancar; si la sesión
// ya existiera, la cerraría esa pasada, y la prueba no distinguiría un vigía que sigue vigilando de
// uno que mira una vez y se va. Se espera a que termine la pasada inicial —cuenta las llamadas a
// CerrarSesionesShellVencidas, que corre una vez por pasada— y recién ahí se siembra: sólo un TICK
// la puede cerrar.
//
// Y SON DOS SESIONES, UNA DETRÁS DE LA OTRA: la segunda se siembra recién cuando la primera cerró,
// así que la tiene que cerrar OTRO tick. Con una sola, pasaban un vigía que se va después de su
// primer tick y uno armado con un Timer en vez de un Ticker (lo midió la revisión adversaria): en
// producción, los dos cuidan el primer minuto después de arrancar y nunca más.

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

// motorQueCuentaPasadas cuenta las pasadas del vigía: cada una llama UNA vez a
// CerrarSesionesShellVencidas. Cuenta DESPUÉS de delegar, para que quien espera el conteo no lea
// la base en el medio de la escritura (la lección de `engineQueCuentaCierres`).
type motorQueCuentaPasadas struct {
	memory.StorageBackend
	pasadas *atomic.Int64
}

func (m motorQueCuentaPasadas) CerrarSesionesShellVencidas(ahora time.Time) (int64, error) {
	n, err := m.StorageBackend.CerrarSesionesShellVencidas(ahora)
	m.pasadas.Add(1)
	return n, err
}

// canalQueAnotaSuCierre es un canal de mentira que sólo registra si lo cerraron.
//
// No alcanza con mirar la fila: el canal es el proceso remoto, un `ssh -tt` vivo con su shell del
// otro lado. Una fila cerrada con el canal abierto es la peor de las dos combinaciones, porque la
// bitácora dice que la sesión terminó y la puerta sigue abierta. Es atómico porque lo cierra la
// goroutine del vigía y lo lee la de la prueba.
//
// Anota también CUÁNDO, con el reloj de la prueba: la fila se cierra con la hora del vigía, y ésa
// no sirve para juzgar al reloj del vigía. Se anota antes de `cerrado`, así que quien ve el canal
// cerrado ve también la hora.
type canalQueAnotaSuCierre struct {
	fin       chan struct{}
	cerrado   atomic.Bool
	cerradoEn atomic.Int64 // UnixNano del primer Cerrar
}

func (c *canalQueAnotaSuCierre) Escribir([]byte) error              { return nil }
func (c *canalQueAnotaSuCierre) Leer(time.Duration) ([]byte, error) { return nil, nil }
func (c *canalQueAnotaSuCierre) Terminado() <-chan struct{}         { return c.fin }
func (c *canalQueAnotaSuCierre) Cerrar() error {
	c.cerradoEn.CompareAndSwap(0, time.Now().UnixNano())
	c.cerrado.Store(true)
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

// shellCerrada contesta si la sesión quedó cerrada del todo: el canal y la fila. cerrarShell cierra
// primero el canal y después la fila, así que esperar sólo al canal leería la fila en el medio.
func shellCerrada(s *McpServer, id string, canal *canalQueAnotaSuCierre) bool {
	ses, existe, err := s.engine.SesionShellPorID(id)
	return err == nil && existe && !ses.Cerrada.IsZero() && canal.cerrado.Load()
}

// exigirCerrada falla si la sesión quedó abierta.
func exigirCerrada(t *testing.T, s *McpServer, id string, canal *canalQueAnotaSuCierre, mundo string) {
	t.Helper()
	ses, existe, err := s.engine.SesionShellPorID(id)
	if err != nil || !existe {
		t.Fatalf("%s: no se pudo releer la sesión: existe=%v err=%v", mundo, existe, err)
	}
	if ses.Cerrada.IsZero() {
		t.Errorf("%s: la sesión venció hace una hora y quedó ABIERTA en la bitácora. Los techos de "+
			"una shell los aplica el cerebro (T5); si cuelgan de algo que no tiene nada que ver con "+
			"las shells, ese algo los apaga mientras dure.", mundo)
	}
	if !canal.cerrado.Load() {
		t.Errorf("%s: nadie cerró el canal de una sesión vencida: el proceso remoto sigue vivo y "+
			"el prompt sigue abierto en la máquina ajena.", mundo)
	}
}

// esperarQueSeCumpla pregunta cada 10 ms hasta que la condición se cumple o vence el plazo, y dice
// cuál de las dos pasó.
func esperarQueSeCumpla(plazo time.Duration, cond func() bool) bool {
	limite := time.Now().Add(plazo)
	for {
		if cond() {
			return true
		}
		if time.Now().After(limite) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// cadenciaDePruebaDeTechos es la cadencia del vigía en las pruebas: un minuto por tick haría que
// nadie corriera esta suite.
const cadenciaDePruebaDeTechos = 20 * time.Millisecond

// correrElVigiaDeShells arranca el vigía de PRODUCCIÓN (RunTechosDeShell) con la cadencia de
// prueba, sobre un motor que cuenta sus pasadas, y espera la pasada inicial. Devuelve el contador y
// cómo pararlo; parar también corre en el cleanup, cancela, espera a que la goroutine termine y
// recién ahí devuelve la cadencia.
//
// PARAR PREGUNTA PRIMERO SI EL VIGÍA SIGUE VIVO. Nadie le cancela el contexto antes de eso, así
// que si ya volvió, volvió SOLO: dejó de vigilar, y lo que venza después queda abierto. Es el ciclo
// de vida afirmado, no deducido de que algo no pasó en un plazo.
func correrElVigiaDeShells(t *testing.T, s *McpServer, mundo string) (*atomic.Int64, func()) {
	t.Helper()
	anterior := cadenciaTechosDeShell
	cadenciaTechosDeShell = cadenciaDePruebaDeTechos
	pasadas := &atomic.Int64{}
	s.engine = motorQueCuentaPasadas{StorageBackend: s.engine, pasadas: pasadas}
	ctx, cancelar := context.WithCancel(context.Background())
	termino := make(chan struct{})
	go func() {
		defer close(termino)
		s.RunTechosDeShell(ctx)
	}()
	var una sync.Once
	yaDicho := false // el piso ya dijo que el vigía volvió solo: parar no lo repite
	parar := func() {
		una.Do(func() {
			select {
			case <-termino:
				if !yaDicho {
					t.Errorf("%s: el vigía de las shells VOLVIÓ SOLO, sin que nadie le cancelara el contexto: "+
						"dejó de vigilar, y toda sesión que venza después queda abierta con su proceso remoto vivo", mundo)
				}
			default:
			}
			cancelar()
			<-termino
			cadenciaTechosDeShell = anterior
		})
	}
	t.Cleanup(parar)
	// PISO: la pasada al arrancar. Sin ella, sembrar «después de la primera pasada» no significa nada.
	//
	// EL ROJO DICE LO QUE SE VIO, NO UNA CAUSA. Lo que se cuenta es la llamada al cierre de las filas,
	// y una pasada que vuelve antes de llegar ahí —por un barrido en vuelo, por la lista de proyectos,
	// porque no hay canales vivos— también deja el conteo en cero: «no hizo su pasada» culpaba a la
	// causa equivocada (lo midió la segunda vuelta de la revisión adversaria).
	if !esperarQueSeCumpla(5*time.Second, func() bool { return pasadas.Load() >= 1 }) {
		select {
		case <-termino:
			yaDicho = true
			t.Fatalf("%s: el vigía de las shells VOLVIÓ SOLO sin llamar ni una vez al cierre de las filas vencidas: "+
				"no vigiló nada", mundo)
		default:
		}
		t.Fatalf("%s: el vigía de las shells sigue vivo y en 5 s nadie llamó al cierre de las filas vencidas, que "+
			"cada pasada completa llama una vez: o no pasa, o sus pasadas vuelven antes de llegar ahí", mundo)
	}
	return pasadas, parar
}

// Sabotaje: que el vigía se vaya si el barrido está apagado, que es el acoplamiento que A136 vino a
// sacar. Cae el mundo del barrido apagado: no hay ni la pasada al arrancar.
// arnes: archivo="internal/mcp/shell_relay.go"
// arnes: de="func (s *McpServer) RunTechosDeShell(ctx context.Context) {\n"
// arnes: a="func (s *McpServer) RunTechosDeShell(ctx context.Context) {\n\tif s.barridoApagado() {\n\t\treturn\n\t}\n"
//
// Sabotaje: que la pasada se saltee mientras un barrido está en vuelo. Cae el mundo del barrido en
// vuelo, que puede durar lo que tarde una máquina que no contesta.
// arnes: archivo="internal/mcp/shell_relay.go"
// arnes: de="func (s *McpServer) aplicarTechosDeShell() {\n"
// arnes: a="func (s *McpServer) aplicarTechosDeShell() {\n\tif s.flotaBusy.Load() {\n\t\treturn\n\t}\n"
//
// Sabotaje: que la pasada dependa de poder leer la lista de proyectos. Cae el mundo de la lista
// ilegible, la falla que DURA mientras la base no deje leer `devices`.
// arnes: archivo="internal/mcp/shell_relay.go"
// arnes: de="func (s *McpServer) aplicarTechosDeShell() {\n"
// arnes: a="func (s *McpServer) aplicarTechosDeShell() {\n\tif _, _, err := s.proyectosDelBarrido(); err != nil {\n\t\treturn\n\t}\n"
//
// Sabotaje: que el vigía mire una vez y se vaya, sin pasadas en el tick. Cae ya el control: la
// sesión sembrada después de la primera pasada no la cierra nadie.
// arnes: archivo="internal/mcp/shell_relay.go"
// arnes: de="\t\tcase <-t.C:\n\t\t\ts.aplicarTechosDeShell()\n"
// arnes: a="\t\tcase <-t.C:\n"
// arnes: colision_ok="TestElServidorEsperaAlVigiaDeShellsAlApagarse"
//
// Sabotaje: que el `for` dé una sola vuelta, así que el vigía vuelve después de su primer tick. La
// primera sesión se cierra igual; cae ya el control, porque el vigía volvió solo.
// arnes: archivo="internal/mcp/shell_relay.go"
// arnes: de="\t\t}\n\t}\n}\n\n// aplicarTechosDeShell"
// arnes: a="\t\t}\n\t\treturn\n\t}\n}\n\n// aplicarTechosDeShell"
//
// Sabotaje: un Timer en vez de un Ticker. Suena una vez y el vigía queda vivo y sordo; la primera
// sesión se cierra igual, y cae ya el control, con la segunda.
// arnes: archivo="internal/mcp/shell_relay.go"
// arnes: de="\tt := time.NewTicker(cadenciaTechosDeShell)\n"
// arnes: a="\tt := time.NewTimer(cadenciaTechosDeShell)\n"
// arnes: colision_ok="TestElVigiaPasaAlRitmoDeSuCadencia"
func TestLosTechosDeShellSeAplicanAunqueElSondeoNoCorra(t *testing.T) {
	mundos := []struct {
		nombre string
		armar  func(t *testing.T, s *McpServer)
	}{
		// CONTROL: sin él, la guarda la satisface un vigía que no cierra nada en ningún mundo.
		{"control: todo normal", func(*testing.T, *McpServer) {}},
		// A136: el barrido no corre nunca.
		{"el barrido está apagado", func(t *testing.T, s *McpServer) {
			if err := s.ConfigurarFlota(config.FleetConfig{ProbeMinutes: -1}); err != nil {
				t.Fatalf("ConfigurarFlota: %v", err)
			}
			if !s.barridoApagado() {
				t.Fatal("con probe_minutes negativo el barrido no figura apagado: este mundo no es el que dice")
			}
		}},
		// Un barrido colgado contra una máquina que no contesta puede durar varios ticks.
		{"el barrido anterior sigue en vuelo", func(t *testing.T, s *McpServer) {
			if !s.flotaBusy.CompareAndSwap(false, true) {
				t.Fatal("la bandera debería nacer libre")
			}
		}},
		// La falla PERMANENTE: mientras dure, ningún barrido llega a nada.
		{"la lista de proyectos es ilegible", func(t *testing.T, s *McpServer) {
			s.engine = motorSinListaDeProyectos{StorageBackend: s.engine}
		}},
	}
	for _, m := range mundos {
		t.Run(m.nombre, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			// CADA MUNDO ARRANCA COMO EL CEREBRO REAL, con el barrido ENCENDIDO: `musubi serve` corre
			// ConfigurarFlota siempre. Sin esto el intervalo queda en 0 y los cuatro mundos son, además,
			// «barrido apagado»: el sabotaje de ese acoplamiento caía primero en el control, y ningún
			// mundo medía una sola cosa (lo mostró el motivo de `arnes -correr`). Sólo el mundo del
			// apagado lo apaga.
			if err := s.ConfigurarFlota(config.FleetConfig{}); err != nil {
				t.Fatalf("ConfigurarFlota: %v", err)
			}
			m.armar(t, s)
			if m.nombre != "el barrido está apagado" && s.barridoApagado() {
				t.Fatalf("%s: el barrido figura apagado en un mundo que no es el del apagado: el mundo no mide lo que dice", m.nombre)
			}
			pasadas, parar := correrElVigiaDeShells(t, s, m.nombre)
			for _, cual := range []string{"primera", "segunda"} {
				id, canal := sesionVencidaRegistrada(t, s)
				desde := pasadas.Load()
				if esperarQueSeCumpla(5*time.Second, func() bool { return shellCerrada(s, id, canal) }) {
					continue
				}
				parar()
				if pasadas.Load() == desde {
					t.Errorf("%s: el vigía no volvió a pasar después de sembrar la %s sesión: dejó de vigilar", m.nombre, cual)
				}
				exigirCerrada(t, s, id, canal, m.nombre+", "+cual+" sesión")
				return
			}
			parar()
		})
	}
}

// A136 · EL SERVIDOR DE VERDAD CIERRA LAS SHELLS VENCIDAS, CON EL BARRIDO APAGADO Y CON EL BARRIDO
// ENCENDIDO.
//
// El vigía lo lanza UNA línea de ListenAndServeHTTP. Acá se arranca el servidor entero —loopback,
// puerto efímero— en los dos mundos, y se exige que cierre una sesión vencida que aparece mientras
// corre. Nace vencida hace una hora, así que esto no mide el reloj del vigía: eso lo mide
// TestElVigiaCierraLaSesionCuandoVenceYNoAntes.
//
// EL MUNDO ENCENDIDO NO ES DE RELLENO. Desde A136 el barrido ya no cierra shells, así que un
// servidor que lanzara el vigía sólo con el barrido apagado —como si el vigía fuera el reemplazo del
// barrido y no el dueño de los techos— dejaría abiertas las shells de todo cerebro que corre con el
// default, que son todos. Con el mundo apagado solo, eso pasaba en verde (lo midió la revisión
// adversaria).
//
// Sabotaje: no lanzar el vigía al arrancar.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\t\ts.RunTechosDeShell(ctxTechos)\n"
// arnes: a="\t\t_ = ctxTechos\n"
//
// Sabotaje: lanzarlo sólo con el barrido apagado. Cae el mundo encendido.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\t\tdefer close(techosListos)\n"
// arnes: a="\t\tdefer close(techosListos)\n\t\tif !s.barridoApagado() {\n\t\t\treturn\n\t\t}\n"
func TestElServidorAplicaLosTechosDeShellConOSinBarrido(t *testing.T) {
	mundos := []struct {
		nombre  string
		probe   float64
		apagado bool
	}{
		// A136: el barrido no corre nunca.
		{"el barrido está apagado", -1, true},
		// El default de `musubi serve`: el barrido corre cada 5 min, y su primer tick no llega
		// dentro de la prueba, así que acá tampoco hay barrido que cierre nada.
		{"el barrido está encendido", 0, false},
	}
	for _, m := range mundos {
		t.Run(m.nombre, func(t *testing.T) {
			anterior := cadenciaTechosDeShell
			cadenciaTechosDeShell = cadenciaDePruebaDeTechos
			// Se devuelve DESPUÉS de apagar el servidor: las limpiezas corren al revés.
			t.Cleanup(func() { cadenciaTechosDeShell = anterior })

			s := newTestServer(t, embedding.NoopProvider{})
			if err := s.ConfigurarFlota(config.FleetConfig{ProbeMinutes: m.probe}); err != nil {
				t.Fatalf("ConfigurarFlota: %v", err)
			}
			if s.barridoApagado() != m.apagado {
				t.Fatalf("%s: barridoApagado()=%v, y este mundo no es el que dice", m.nombre, s.barridoApagado())
			}
			pasadas := &atomic.Int64{}
			s.engine = motorQueCuentaPasadas{StorageBackend: s.engine, pasadas: pasadas}

			ctx, cancelar := context.WithCancel(context.Background())
			termino := make(chan error, 1)
			go func() {
				termino <- s.ListenAndServeHTTP(ctx, config.ServiceConfig{Addr: "127.0.0.1:0", RequestTimeoutSeconds: 10})
			}()
			t.Cleanup(func() {
				cancelar()
				select {
				case err := <-termino:
					if err != nil {
						t.Errorf("ListenAndServeHTTP terminó con error: %v", err)
					}
				case <-time.After(10 * time.Second):
					t.Error("ListenAndServeHTTP no terminó al cancelar el contexto")
				}
			})

			if !esperarQueSeCumpla(10*time.Second, func() bool { return pasadas.Load() >= 1 }) {
				t.Fatalf("%s: el servidor lleva 10 s arriba y nadie llamó al cierre de las filas vencidas, que cada "+
					"pasada completa del vigía llama una vez: o el servidor no lanzó al vigía, o sus pasadas vuelven "+
					"antes de llegar ahí. Una sesión vencida queda abierta con su proceso remoto vivo", m.nombre)
			}
			id, canal := sesionVencidaRegistrada(t, s)
			if !esperarQueSeCumpla(10*time.Second, func() bool { return shellCerrada(s, id, canal) }) {
				t.Errorf("%s: con el servidor arriba, una sesión vencida sigue abierta después de 10 s", m.nombre)
			}
			exigirCerrada(t, s, id, canal, "servidor, "+m.nombre)
		})
	}
}

// motorQueFrenaLaPasada frena UNA pasada del vigía hasta que la prueba la suelte, y avisa cuando
// entró. Elige cuál por el número de llamada a CerrarSesionesShellVencidas, que corre una vez por
// pasada: la 1 es la pasada al arrancar, y la 2 la del primer tick.
type motorQueFrenaLaPasada struct {
	memory.StorageBackend
	cual    int64
	llamada *atomic.Int64
	entro   chan struct{}
	soltar  chan struct{}
}

func (m motorQueFrenaLaPasada) CerrarSesionesShellVencidas(ahora time.Time) (int64, error) {
	if m.llamada.Add(1) == m.cual {
		close(m.entro)
		<-m.soltar
	}
	return m.StorageBackend.CerrarSesionesShellVencidas(ahora)
}

// A136 · EL SERVIDOR NO VUELVE CON UNA PASADA DEL VIGÍA EN CURSO, NI CUANDO LO APAGAN NI CUANDO SE
// CAE SOLO.
//
// Quien llama a ListenAndServeHTTP puede cerrar la base apenas vuelve: las pruebas lo hacen siempre,
// y `musubi serve` cuando lo apagan (si vuelve con error, sale con os.Exit). El vigía escribe en
// ella, así que el servidor lo espera: si volviera antes, la pasada en curso terminaría contra una
// base cerrada, y lo que quisiera cerrar quedaría abierto con un WARN de auditoría sobre una
// bitácora que estaba bien. Se frena una pasada a propósito y se exige que el servidor no vuelva
// hasta soltarla, y que después vuelva.
//
// POR LAS DOS SALIDAS. Al servidor lo apagan —una señal cancela su contexto— o se cae solo: acá el
// puerto está ocupado, y nadie cancela nada. Con la segunda sin medir pasaban un servidor que en esa
// salida volvía sin esperar al vigía, y uno que lo esperaba sin haberlo parado: el vigía corría con
// el contexto del servidor, que nadie cancela, y la espera no volvía nunca (lo midió la revisión
// adversaria).
//
// Y POR LAS DOS PASADAS: la del arranque y la de un tick. Frenando sólo la primera, una pasada de
// tick lanzada en su propia goroutine dejaba al servidor volver con ella en curso, y la prueba
// seguía verde (lo midió la segunda vuelta de la revisión). La de un tick se mide sólo cuando lo
// apagan: el que se cae solo vuelve antes del primer tick, y lo espera por la misma función.
//
// Sabotaje: cancelar el vigía sin esperarlo.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\t\tcancelarTechos()\n\t\t<-techosListos\n"
// arnes: a="\t\tcancelarTechos()\n"
//
// Sabotaje: esperarlo sólo cuando lo apagan. Cae el mundo del servidor que se cae solo: vuelve con
// la pasada en curso.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\t\tesperarTechos()\n\t\tif errors.Is(err, http.ErrServerClosed) {\n"
// arnes: a="\t\tif errors.Is(err, http.ErrServerClosed) {\n"
//
// Sabotaje: que el vigía corra con el contexto del servidor, sin uno propio que parar. Cae el mundo
// del servidor que se cae solo: nadie cancela ese contexto, y el servidor se cuelga esperándolo.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\tctxTechos, cancelarTechos := context.WithCancel(ctx)\n"
// arnes: a="\tctxTechos, cancelarTechos := ctx, func() {}\n"
//
// Sabotaje: que la pasada de un tick corra en su propia goroutine. El vigía vuelve sin esperarla, y
// cae el mundo de la pasada de un tick: la del arranque sigue siendo sincrónica.
// arnes: archivo="internal/mcp/shell_relay.go"
// arnes: de="\t\tcase <-t.C:\n\t\t\ts.aplicarTechosDeShell()\n"
// arnes: a="\t\tcase <-t.C:\n\t\t\tgo s.aplicarTechosDeShell()\n"
// arnes: colision_ok="TestLosTechosDeShellSeAplicanAunqueElSondeoNoCorra"
func TestElServidorEsperaAlVigiaDeShellsAlApagarse(t *testing.T) {
	mundos := []struct {
		nombre string
		seCae  bool
		frena  int64  // qué llamada al cierre de las filas se frena: la 1 es la pasada al arrancar
		pasada string // cuál es, para el rojo
	}{
		{"lo apagan en la pasada al arrancar", false, 1, "al arrancar"},
		{"se cae solo en la pasada al arrancar", true, 1, "al arrancar"},
		{"lo apagan en la pasada de un tick", false, 2, "de un tick"},
	}
	for _, m := range mundos {
		t.Run(m.nombre, func(t *testing.T) {
			anterior := cadenciaTechosDeShell
			cadenciaTechosDeShell = cadenciaDePruebaDeTechos
			// Se devuelve DESPUÉS de apagar el servidor: las limpiezas corren al revés.
			t.Cleanup(func() { cadenciaTechosDeShell = anterior })

			s := newTestServer(t, embedding.NoopProvider{})
			if err := s.ConfigurarFlota(config.FleetConfig{}); err != nil {
				t.Fatalf("ConfigurarFlota: %v", err)
			}
			freno := motorQueFrenaLaPasada{StorageBackend: s.engine, cual: m.frena, llamada: &atomic.Int64{},
				entro: make(chan struct{}), soltar: make(chan struct{})}
			s.engine = freno
			soltar := sync.OnceFunc(func() { close(freno.soltar) })

			addr := "127.0.0.1:0"
			if m.seCae {
				ocupado, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatalf("no se pudo ocupar un puerto: %v", err)
				}
				t.Cleanup(func() { _ = ocupado.Close() })
				addr = ocupado.Addr().String()
				// PISO: el puerto está ocupado de verdad. Si no, el servidor no se cae, y este mundo
				// sería el de siempre con un contexto que nadie cancela.
				if otro, err := net.Listen("tcp", addr); err == nil {
					_ = otro.Close()
					t.Fatalf("%s: el puerto %s se pudo volver a tomar: el servidor no se caería, y este mundo no es el que dice", m.nombre, addr)
				}
			}

			ctx, cancelar := context.WithCancel(context.Background())
			termino := make(chan error, 1)
			go func() {
				termino <- s.ListenAndServeHTTP(ctx, config.ServiceConfig{Addr: addr, RequestTimeoutSeconds: 10})
			}()
			var volvio atomic.Bool
			t.Cleanup(func() {
				soltar()
				cancelar()
				if !volvio.Load() {
					select {
					case <-termino:
					case <-time.After(10 * time.Second):
						t.Error("ListenAndServeHTTP no terminó ni cancelándole el contexto")
					}
				}
			})

			// PISO: la pasada elegida entró y quedó frenada.
			select {
			case <-freno.entro:
			case <-time.After(10 * time.Second):
				t.Fatalf("%s: en 10 s no llegó la llamada %d al cierre de las filas vencidas, que es donde se frena la "+
					"pasada %s: no hay pasada en curso que esperar, y este mundo no es el que dice", m.nombre, m.frena, m.pasada)
			}
			if !m.seCae {
				cancelar()
			}
			select {
			case err := <-termino:
				volvio.Store(true)
				t.Fatalf("%s: ListenAndServeHTTP volvió (err=%v) con la pasada %s del vigía todavía en curso: quien "+
					"llama puede cerrar la base apenas esto vuelve, y la pasada escribe en ella", m.nombre, err, m.pasada)
			case <-time.After(300 * time.Millisecond):
			}
			soltar()
			select {
			case err := <-termino:
				volvio.Store(true)
				if m.seCae && err == nil {
					t.Fatalf("%s: ListenAndServeHTTP volvió sin error con el puerto ocupado: no se cayó, y este mundo no es el que dice", m.nombre)
				}
				if !m.seCae && err != nil {
					t.Errorf("%s: ListenAndServeHTTP terminó con error: %v", m.nombre, err)
				}
			case <-time.After(10 * time.Second):
				t.Fatalf("%s: ListenAndServeHTTP no volvió en 10 s después de soltar la pasada del vigía: se quedó "+
					"esperando a un vigía que nadie paró", m.nombre)
			}
		})
	}
}

// A136 · LA PASADA AL ARRANCAR CIERRA LAS FILAS QUE DEJÓ UN REINICIO.
//
// Un reinicio del cerebro mata los canales —son procesos suyos— pero deja las filas abiertas, y la
// bitácora las sigue mostrando «activas». Esas filas no tienen canal en el registro, así que el
// recorrido de las vivas no las ve: las cierra CerrarSesionesShellVencidas, y la primera vez que
// corre es la pasada al arrancar. Se siembra una fila vencida SIN canal, que es el mundo de un
// reinicio, y se corre el vigía con una cadencia de una hora: en los 5 s de espera, sólo la pasada
// al arrancar la puede cerrar. Y se cuentan las llamadas al cierre de las filas, para que el rojo
// diga cuál de las dos cosas faltó: la pasada, o que la pasada vea vencida la fila.
//
// Sabotaje: que el cierre de las filas pregunte con una hora que no vence nada. Las sesiones con
// canal se siguen cerrando, así que ninguna otra prueba lo ve (lo midió la revisión adversaria).
// arnes: archivo="internal/mcp/shell_relay.go"
// arnes: de="\tif _, err := s.engine.CerrarSesionesShellVencidas(ahora); err != nil {\n"
// arnes: a="\tif _, err := s.engine.CerrarSesionesShellVencidas(time.Time{}); err != nil {\n"
//
// Sabotaje: sin pasada al arrancar. Las filas de un reinicio esperan el primer tick, que en
// producción llega un minuto después.
// arnes: archivo="internal/mcp/shell_relay.go"
// arnes: de="\ts.aplicarTechosDeShell()\n\tt := time.New"
// arnes: a="\tt := time.New"
func TestLaPasadaAlArrancarCierraLasFilasDeUnReinicio(t *testing.T) {
	anterior := cadenciaTechosDeShell
	cadenciaTechosDeShell = time.Hour
	// Se devuelve DESPUÉS de parar al vigía: las limpiezas corren al revés.
	t.Cleanup(func() { cadenciaTechosDeShell = anterior })

	s := newTestServer(t, embedding.NoopProvider{})
	if err := s.ConfigurarFlota(config.FleetConfig{}); err != nil {
		t.Fatalf("ConfigurarFlota: %v", err)
	}
	pasadas := &atomic.Int64{}
	s.engine = motorQueCuentaPasadas{StorageBackend: s.engine, pasadas: pasadas}
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID:  "dev-inventado",
		ProjectID: "casa",
		Principal: "op",
		Creada:    time.Now().Add(-3 * time.Hour),
	})
	if err != nil {
		t.Fatalf("no se pudo sembrar la fila: %v", err)
	}
	// PISOS: la fila está vencida, y no hay un solo canal en el registro.
	if vencida, _ := ses.Vencida(time.Now()); !vencida {
		t.Fatalf("la fila sembrada no está vencida (creada %s, vence %s): la prueba no mide nada", ses.Creada, ses.Vence)
	}
	if vivas := s.shells.vivas(); len(vivas) != 0 {
		t.Fatalf("hay %d canales en el registro: esto no es el mundo de un reinicio", len(vivas))
	}

	ctx, cancelar := context.WithCancel(context.Background())
	termino := make(chan struct{})
	go func() {
		defer close(termino)
		s.RunTechosDeShell(ctx)
	}()
	t.Cleanup(func() {
		cancelar()
		<-termino
	})

	cerrada := esperarQueSeCumpla(5*time.Second, func() bool {
		r, existe, err := s.engine.SesionShellPorID(ses.ID)
		return err == nil && existe && !r.Cerrada.IsZero()
	})
	switch {
	case cerrada:
	case pasadas.Load() == 0:
		t.Error("el vigía arrancó y en 5 s nadie llamó al cierre de las filas: o no hubo pasada al arrancar, o " +
			"la pasada se saltea las filas. La que dejó un reinicio sigue abierta, y la bitácora la muestra «activa»")
	default:
		t.Error("la pasada al arrancar corrió y la fila vencida que dejó un reinicio sigue abierta: el cierre de " +
			"las filas no la ve vencida, y la bitácora la sigue mostrando «activa»")
	}
}

// A136 · EL BARRIDO DE FLOTA NO CIERRA SHELLS: UN DUEÑO SOLO.
//
// Las cierra el vigía, siempre. Si el barrido también las cerrara, en el default —el barrido corre
// cada 5 min— un vigía roto no se notaría: el barrido lo taparía, y A136 volvería entero el día que
// alguien apague el barrido, que es justo cuando no queda nadie más para cerrarlas. Se corre UN
// barrido de verdad, sincrónico, con una sesión vencida en el registro, y se exige que la deje como
// estaba: el canal abierto, la fila abierta y ni una llamada al cierre de las filas vencidas.
//
// EL BARRIDO RECORRE UNA MÁQUINA. Sin ninguna, el bucle de proyectos no da una vuelta, y un cierre
// metido ADENTRO del bucle —justo donde alguien pondría uno «por proyecto»— pasaba los dos paquetes
// en verde (lo midió la segunda vuelta de la revisión adversaria). Acá casa tiene una máquina con la
// memoria al 95 % y una política que salta al 90: que la política actúe prueba que el barrido pasó
// por adentro del bucle.
//
// EL ROJO DICE DÓNDE APARECIÓ EL SEGUNDO DUEÑO: antes o después de que la política de casa encolara
// su comando. Los dos sabotajes de abajo caen en esa aserción, y con un mensaje que sólo contaba las
// llamadas caían igual: el arnés los marcaba como un sabotaje contado dos veces, y el rojo no decía
// cuál de los dos acoples había vuelto.
//
// Sabotaje: que el barrido vuelva a cerrar las shells vencidas al empezar, como antes de A136. Todo
// lo demás sigue verde (lo midió la revisión adversaria).
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="func (s *McpServer) barrerFlotaUnaVez(ctx context.Context) {\n"
// arnes: a="func (s *McpServer) barrerFlotaUnaVez(ctx context.Context) {\n\ts.cerrarShellsVencidas(time.Now())\n"
//
// Sabotaje: que las cierre adentro del bucle de proyectos, después de aplicar las políticas. Con un
// barrido sin máquinas pasaba.
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="\t\tacciones += s.aplicarPoliticas(proy, time.Now())\n"
// arnes: a="\t\tacciones += s.aplicarPoliticas(proy, time.Now())\n\t\ts.cerrarShellsVencidas(time.Now())\n"
func TestElBarridoDeFlotaNoCierraShells(t *testing.T) {
	s, d := maquinaConPolitica(t, []string{"metrics", "exec"})
	s.buscarPrincipal = registroDePrueba(autoHeal())
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(95, ahora), ahora) // 95 % de RAM: la política se cumple
	cierres := motorQueUbicaElCierre{
		StorageBackend: s.engine,
		encolo:         &atomic.Bool{}, antesDeEncolar: &atomic.Int64{}, despuesDeEncolar: &atomic.Int64{},
	}
	s.engine = cierres
	id, canal := sesionVencidaRegistrada(t, s)

	antes := comandosDePolitica(t, s)
	s.barrerFlotaUnaVez(context.Background())

	// PISOS: el barrido pasó por adentro del bucle de casa, y llegó hasta el final, que es la poda.
	// Uno que no diera ninguna vuelta, o que volviera antes, dejaría las shells en paz sin probar nada.
	if comandosDePolitica(t, s) == antes {
		t.Fatal("el barrido no encoló el comando de la política de casa, que se cumple: no pasó por adentro del " +
			"bucle de proyectos, y la prueba no ve lo que se cierre ahí")
	}
	if s.ultimaPoda.IsZero() {
		t.Fatal("el barrido no llegó a la poda: volvió antes, y la prueba no ve lo que se cierre al final")
	}
	switch a, d := cierres.antesDeEncolar.Load(), cierres.despuesDeEncolar.Load(); {
	case a > 0 && d > 0:
		t.Errorf("el barrido llamó %d vez/veces al cierre de las filas vencidas antes de que la política de casa "+
			"encolara su comando, y %d después: las shells tienen dos dueños", a, d)
	case a > 0:
		t.Errorf("el barrido llamó %d vez/veces al cierre de las filas vencidas antes de que la política de casa "+
			"encolara su comando: las shells tienen dos dueños", a)
	case d > 0:
		t.Errorf("el barrido llamó %d vez/veces al cierre de las filas vencidas después de que la política de casa "+
			"encoló su comando: las shells tienen dos dueños", d)
	}
	if canal.cerrado.Load() {
		t.Error("el barrido cerró el canal de una sesión vencida: las shells tienen dos dueños")
	}
	if ses, existe, err := s.engine.SesionShellPorID(id); err != nil || !existe || !ses.Cerrada.IsZero() {
		t.Errorf("el barrido tocó la fila de una sesión vencida (existe=%v, err=%v): las shells tienen dos dueños", existe, err)
	}
}

// motorQueUbicaElCierre cuenta las llamadas al cierre de las filas vencidas, separadas por si
// llegaron antes o después de que se encolara el primer comando de una política. Sólo lo usa
// TestElBarridoDeFlotaNoCierraShells, para que su rojo diga en qué tramo del barrido apareció el
// cierre.
type motorQueUbicaElCierre struct {
	memory.StorageBackend
	encolo           *atomic.Bool
	antesDeEncolar   *atomic.Int64
	despuesDeEncolar *atomic.Int64
}

func (m motorQueUbicaElCierre) EncolarComando(c fleet.Comando) (fleet.Comando, error) {
	cmd, err := m.StorageBackend.EncolarComando(c)
	if err == nil && c.Origen == fleet.OrigenPolitica {
		m.encolo.Store(true)
	}
	return cmd, err
}

func (m motorQueUbicaElCierre) CerrarSesionesShellVencidas(ahora time.Time) (int64, error) {
	if m.encolo.Load() {
		m.despuesDeEncolar.Add(1)
	} else {
		m.antesDeEncolar.Add(1)
	}
	return m.StorageBackend.CerrarSesionesShellVencidas(ahora)
}

// cadenciaTechosDeShellDeProduccion es la cadencia con la que arranca el paquete, antes de que
// ninguna prueba la toque: la que corre en producción.
var cadenciaTechosDeShellDeProduccion = cadenciaTechosDeShell

// A136 · EL VIGÍA PASA MUCHO MÁS SEGUIDO QUE EL TECHO MÁS CORTO.
//
// Lo que tarda el vigía en volver a pasar se le SUMA a cada techo: con una cadencia de una hora, los
// 15 min de inactividad serían, en los hechos, hasta 75. Todas las demás pruebas achican la cadencia
// para no esperar, así que ninguna ve la de producción. La cota sale del techo más corto y no de un
// número escrito acá: si el techo cambia, la cota lo sigue. Un décimo del techo es el atraso que se
// tolera.
//
// ACOTA LA VARIABLE, NO EL RELOJ. Que el reloj del vigía sea esta variable lo exige
// TestElVigiaPasaAlRitmoDeSuCadencia: sin ella, un Ticker armado con otro número dejaba esta prueba
// en regla y al vigía pasando cuando quisiera.
//
// Sabotaje: una cadencia de dos horas. Todo lo demás sigue verde.
// arnes: archivo="internal/mcp/shell_relay.go"
// arnes: de="var cadenciaTechosDeShell = time.Minute\n"
// arnes: a="var cadenciaTechosDeShell = 2 * time.Hour\n"
func TestElVigiaDeShellsPasaMuchoMasSeguidoQueElTechoMasCorto(t *testing.T) {
	techo := min(fleet.ShellInactividadMax, fleet.ShellVidaMax)
	if cadenciaTechosDeShellDeProduccion <= 0 {
		t.Fatalf("la cadencia del vigía es %s: time.NewTicker entra en pánico y el servidor no arranca", cadenciaTechosDeShellDeProduccion)
	}
	if cota := techo / 10; cadenciaTechosDeShellDeProduccion > cota {
		t.Errorf("el vigía de las shells pasa cada %s y el techo más corto es de %s: una sesión vencida puede "+
			"sobrevivir hasta %s, más de un décimo del techo (%s)", cadenciaTechosDeShellDeProduccion, techo,
			techo+cadenciaTechosDeShellDeProduccion, cota)
	}
}

// A136 · EL VIGÍA PASA AL RITMO DE SU CADENCIA.
//
// La prueba de arriba acota la VARIABLE; ésta exige que el reloj del vigía sea ella. Las pruebas de
// conducta no lo distinguían: esperan hasta 5 s a que una sesión se cierre, así que un Ticker con la
// cadencia por 60 —1,2 s con la de prueba, una hora en producción— las pasaba todas (lo midió la
// segunda vuelta de la revisión adversaria). Acá se cuentan las pasadas del vigía en 25 cadencias,
// después de la del arranque: se esperan unas 25 y se exigen 8. El margen es para una máquina
// cargada. Un reloj 3,6 veces más lento que la cadencia, o más, da a lo sumo 7 y cae siempre; uno
// entre 3,1 y 3,6 veces puede pasar, según en qué fase caiga la ventana.
//
// Sabotaje: el reloj del vigía con la cadencia por 60.
// arnes: archivo="internal/mcp/shell_relay.go"
// arnes: de="\tt := time.NewTicker(cadenciaTechosDeShell)\n"
// arnes: a="\tt := time.NewTicker(cadenciaTechosDeShell * 60)\n"
// arnes: colision_ok="TestLosTechosDeShellSeAplicanAunqueElSondeoNoCorra"
func TestElVigiaPasaAlRitmoDeSuCadencia(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pasadas, parar := correrElVigiaDeShells(t, s, "ritmo del vigía")
	defer parar()

	desde := pasadas.Load()
	ventana := 25 * cadenciaDePruebaDeTechos
	time.Sleep(ventana)
	if n := pasadas.Load() - desde; n < 8 {
		t.Errorf("con una cadencia de %s, el vigía llamó %d vez/veces al cierre de las filas en %s, y se esperaban "+
			"unas 25: su reloj no es la cadencia, así que lo que acota TestElVigiaDeShellsPasaMuchoMasSeguidoQueElTechoMasCorto "+
			"no es lo que tarda en volver a pasar", cadenciaDePruebaDeTechos, n, ventana)
	}
}

// A136 · EL VIGÍA CIERRA LA SESIÓN CUANDO VENCE, Y NO ANTES.
//
// Todas las demás sesiones de estas pruebas nacen vencidas hace horas, así que las cerraba igual un
// vigía que mirara otra hora que la de la pasada: una atrasada, una tomada al arrancar. Ninguna
// prueba distinguía el reloj (lo midió la revisión adversaria, con uno atrasado una hora). Ésta
// siembra una sesión VIVA que vence por inactividad un par de segundos después, y exige las dos
// mitades del techo: que el vigía la cierre cuando vence, y que no la cierre antes, porque cortarle
// la shell a quien todavía tiene tiempo también es romper el techo.
//
// EL VENCIMIENTO SE LEE DE LA BASE, NO DE LO SEMBRADO: la fila guarda los segundos enteros, así que
// la sesión vence hasta un segundo antes de lo que dice la cuenta. Y el «antes» se mide con el
// reloj de la prueba, que anota el canal: la fila se cierra con la hora del vigía, que es justo la
// que está en duda.
//
// Sabotaje: el vigía mira la hora con una hora de atraso. Lo que venza mientras corre no lo ve
// vencer nunca, y la sesión queda abierta.
// arnes: archivo="internal/mcp/shell_relay.go"
// arnes: de="\tif n := s.cerrarShellsVencidas(time.Now()); n > 0 {\n"
// arnes: a="\tif n := s.cerrarShellsVencidas(time.Now().Add(-time.Hour)); n > 0 {\n"
//
// Sabotaje: la pasada juzga con una hora de adelanto. Cierra en su primera pasada una sesión a la
// que todavía le quedaban un par de segundos (y cerraría hasta una recién abierta).
// arnes: archivo="internal/mcp/shell_relay.go"
// arnes: de="\t\tif vencida, motivo := ses.Vencida(ahora); vencida {\n"
// arnes: a="\t\tif vencida, motivo := ses.Vencida(ahora.Add(time.Hour)); vencida {\n"
func TestElVigiaCierraLaSesionCuandoVenceYNoAntes(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	pasadas, parar := correrElVigiaDeShells(t, s, "reloj del vigía")
	defer parar()

	sembrada, err := s.engine.AbrirSesionShell(fleet.SesionShell{
		DeviceID:  "dev-inventado",
		ProjectID: "casa",
		Principal: "op",
		Creada:    time.Now().Add(-fleet.ShellInactividadMax + 2*time.Second),
	})
	if err != nil {
		t.Fatalf("no se pudo sembrar la sesión: %v", err)
	}
	canal := &canalQueAnotaSuCierre{fin: make(chan struct{})}
	s.shells.guardar(sembrada.ID, canal)
	ses, existe, err := s.engine.SesionShellPorID(sembrada.ID)
	if err != nil || !existe {
		t.Fatalf("no se pudo releer la sesión sembrada: existe=%v err=%v", existe, err)
	}
	vence := ses.UltimoTrafico.Add(fleet.ShellInactividadMax)
	// PISO: nace viva. A una sesión ya vencida la cierra cualquier reloj, y la prueba no mediría nada.
	if vencida, _ := ses.Vencida(time.Now()); vencida {
		t.Fatalf("la sesión sembrada ya nace vencida (vence %s): la prueba no mide el reloj del vigía",
			vence.Format(time.RFC3339Nano))
	}

	// EL ROJO DICE CUÁNTAS VECES PASÓ EL VIGÍA, Y NO POR QUÉ NO LA CERRÓ: un vigía que dejó de pasar
	// también la deja abierta, y «no mira la hora de cada pasada» culpaba ahí a la causa equivocada (lo
	// midió la segunda vuelta de la revisión adversaria). Si volvió solo, además lo dice parar.
	if !esperarQueSeCumpla(time.Until(vence)+5*time.Second, func() bool { return shellCerrada(s, ses.ID, canal) }) {
		t.Fatalf("la sesión venció a las %s y 5 s después sigue abierta. El vigía, con una cadencia de %s, "+
			"llamó %d vez/veces al cierre de las filas desde que arrancó: ninguna de esas pasadas la cerró",
			vence.Format(time.RFC3339), cadenciaDePruebaDeTechos, pasadas.Load())
	}
	if cerro := time.Unix(0, canal.cerradoEn.Load()); cerro.Before(vence) {
		t.Errorf("el vigía cerró la sesión %s ANTES de que venciera: le cortó la shell a alguien a quien "+
			"el techo todavía le daba tiempo", vence.Sub(cerro).Round(time.Millisecond))
	}
}
