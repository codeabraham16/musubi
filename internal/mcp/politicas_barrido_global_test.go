package mcp

// politicas_barrido_global_test.go — A132: el `puede_actuar` del inventario contra el estado GLOBAL
// del barrido, o sea si corre y sobre qué tenants.
//
// Las filas que comparan el inventario contra el scheduler y el barrido REALES viven en la tabla de
// TestLaPoliticaActuaDondeSuPrincipalPodriaYElInventarioLoDice, que exige una fila por freno. Acá
// quedan los bordes que no son una fila de esa tabla —la lista ilegible, el orden entre frenos, el
// costo sin políticas y el aviso del recorte— y los ayudantes que usan las dos.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/logx"
	"musubi/internal/memory"
)

// sondeoCortoDePrueba es el intervalo del scheduler encendido en las pruebas: corto para no esperar
// los 5 min del default, y no tanto como para que un tick se pise con el anterior y el barrido
// saltee ticks avisándolo.
const sondeoCortoDePrueba = 100 * time.Millisecond

// esperaDelScheduler es cuánto se le da al scheduler REAL para actuar, o para volver solo si está
// apagado. Holgada a propósito: con -race y la máquina cargada un tick puede tardar, y esperar de
// más sólo lo paga la fila que falla.
const esperaDelScheduler = 10 * time.Second

// barridoDelInventario lee el estado del barrido como lo lee musubi_fleet_list, con barridoVigente:
// una prueba que le pregunta directo a porQueNoActuaria no puede inventarlo, o compararía el
// indicador contra un barrido que el cerebro no tiene.
func barridoDelInventario(t *testing.T, s *McpServer) barridoDePoliticas {
	t.Helper()
	b, err := s.barridoVigente()
	if err != nil {
		t.Fatalf("barridoVigente: %v", err)
	}
	return b
}

// tenantsAlrededorDeCasa da de alta `antes` tenants que ordenan ANTES que «casa» y `despues` que
// ordenan DESPUÉS (la lista del barrido va por project_id), con una máquina activa cada uno:
//   - antes = proyectosParaVigilar: casa queda la 65ª, afuera del tope;
//   - antes = proyectosParaVigilar-1: casa entra justo; con despues = 0 el tope no recorta a nadie,
//     y con despues = 1 recorta al tenant de atrás y casa sigue adentro. Son dos celdas distintas, y
//     un indicador o un barrido que tratara «hubo recorte» como «casa quedó afuera» sólo cae en la
//     segunda (revisión de A132).
//
// Su muestra (muestraDePrueba, 37,5 % de RAM) no cumple la condición de la política de referencia,
// así que el barrido las recorre sin actuar ni contar nada sobre ellas.
//
// PISO: dónde quedó casa se mide con la consulta cruda de la base, no con proyectosDelBarrido, que es
// justo lo que las pruebas comparan: un fixture que no la deja donde dice mediría otra cosa.
func tenantsAlrededorDeCasa(t *testing.T, s *McpServer, antes, despues int, ahora time.Time) {
	t.Helper()
	for i := 0; i < antes; i++ {
		maquinaConMuestra(t, s, fmt.Sprintf("a-%03d", i), "server", *muestraDePrueba(), ahora)
	}
	for i := 0; i < despues; i++ {
		maquinaConMuestra(t, s, fmt.Sprintf("z-%03d", i), "server", *muestraDePrueba(), ahora)
	}
	todos, err := s.engine.ProyectosConDevices(antes + despues + 10)
	if err != nil {
		t.Fatalf("ProyectosConDevices: %v", err)
	}
	if len(todos) != antes+1+despues || todos[antes] != "casa" {
		t.Fatalf("el fixture tenía que dejar %d tenants con casa en el lugar %d de la lista por project_id, y la lista es %v",
			antes+1+despues, antes+1, todos)
	}
}

// almacenQueCuentaBarridos cuenta, sin romperla, cada lectura de la lista de tenants. Cada barrido
// la lee dos veces —la sonda al empezar y la vida de red después—, así que sirve de reloj de
// barridos sin tocar producción. Es atómico porque lo incrementa la goroutine del scheduler y lo lee
// la de la prueba.
type almacenQueCuentaBarridos struct {
	memory.StorageBackend
	lecturas *atomic.Int64
}

func (a almacenQueCuentaBarridos) ProyectosConDevices(tope int) ([]string, error) {
	a.lecturas.Add(1)
	return a.StorageBackend.ProyectosConDevices(tope)
}

// comoCorrioElScheduler es lo que correrElSchedulerReal vio del scheduler de producción.
type comoCorrioElScheduler struct {
	// volvioSolo: RunFlotaScheduler volvió sin que se cancelara el contexto. Es lo que tiene que
	// hacer con el barrido apagado, y sólo entonces.
	volvioSolo bool
	// siguioBarriendo: después del primer comando de política EMPEZÓ otro barrido, con el
	// scheduler todavía vivo. Un scheduler que barre una vez y se queda quieto también «actuó».
	siguioBarriendo bool
}

// correrElSchedulerReal arranca RunFlotaScheduler —el de producción, con el intervalo que dejó
// ConfigurarFlota— y lo mira hasta que pase una de tres cosas: vuelve solo, empieza otro barrido
// después de encolar un comando de política, o se agota esperaDelScheduler. Después lo cancela y
// ESPERA a que termine, también si la prueba muere a la mitad: un tick que siguiera vivo escribiría
// en una base que el cleanup ya cerró.
//
// POR QUÉ DEVUELVE CÓMO CORRIÓ, Y NO SÓLO SI ACTUÓ (revisión de A132). La versión anterior volvía
// igual por las tres puertas, y las filas del barrido sólo miraban la cola. Así, un scheduler que con
// el barrido apagado siguiera corriendo con el intervalo por default —actuaría a los 5 min, con el
// inventario diciendo `barrido_apagado`— agotaba la espera sin encolar nada y quedaba en verde; y
// uno con un Timer en vez de un Ticker actuaba una vez y no volvía a barrer nunca, también en verde.
//
// LO QUE NO PUEDE VER, dicho: un scheduler que vuelva solo pero deje OTRA goroutine barriendo con su
// propio reloj. Cerrarlo pediría un ticker inyectable, y no es la forma natural de equivocarse acá:
// la natural es la condición de apagado, que sí se mide.
func correrElSchedulerReal(t *testing.T, s *McpServer, antes int) comoCorrioElScheduler {
	t.Helper()
	lecturas := &atomic.Int64{}
	original := s.engine
	s.engine = almacenQueCuentaBarridos{StorageBackend: original, lecturas: lecturas}
	ctx, cancelar := context.WithCancel(context.Background())
	termino := make(chan struct{})
	go func() {
		defer close(termino)
		s.RunFlotaScheduler(ctx)
	}()
	defer func() {
		cancelar()
		<-termino
		s.engine = original
	}()
	var como comoCorrioElScheduler
	alActuar := int64(-1) // lecturas de la lista cuando apareció el primer comando
	limite := time.After(esperaDelScheduler)
	for {
		select {
		case <-termino:
			como.volvioSolo = true
			return como
		case <-limite:
			return como
		case <-time.After(20 * time.Millisecond):
			if alActuar < 0 && comandosDePolitica(t, s) > antes {
				alActuar = lecturas.Load()
			}
			// +2 garantiza una lectura de un barrido POSTERIOR: del que estaba en curso al actuar
			// puede faltar, como mucho, la de la vida de red.
			if alActuar >= 0 && lecturas.Load() >= alActuar+2 {
				como.siguioBarriendo = true
				return como
			}
		}
	}
}

// almacenSinListaDelBarrido es la base de siempre con UNA sola consulta rota: la lista de tenants
// con máquinas, que es de donde sale el tope del barrido. Cuenta cuántas veces se la preguntaron,
// para que la prueba pueda exigir que los dos lados pasaron de verdad por la rama de error.
type almacenSinListaDelBarrido struct {
	memory.StorageBackend
	consultas *int
}

func (a almacenSinListaDelBarrido) ProyectosConDevices(int) ([]string, error) {
	*a.consultas++
	return nil, errors.New("simulado: la lista de tenants con máquinas no contestó")
}

// A132 · CON LA LISTA DEL BARRIDO ILEGIBLE, EL INVENTARIO NO INVENTA UN VEREDICTO.
//
// barridoVigente pregunta con la MISMA lista que usa el barrido (proyectosDelBarrido). Si esa lista
// no se puede leer, el barrido no corre sobre nadie: barrerFlotaUnaVez vuelve antes de aplicar
// políticas. El inventario, con la misma falla, no tiene nada honesto que publicar como
// `puede_actuar`: «true» sería el defecto de A132 escondido en la rama de error, que es la que nadie
// prueba, y «false» con `fuera_del_barrido` mandaría a buscar un tope recortado cuando lo que pasa es
// que la base no contestó. La tool devuelve error y dice qué no pudo leer.
//
// Sabotaje: tragarse el error y seguir con el valor cero de barridoDePoliticas, que marca a toda
// máquina `fuera_del_barrido`: la tool devuelve un inventario con un veredicto inventado.
// arnes: archivo="internal/mcp/methods_fleet.go"
// arnes: de="\t\t\treturn nil, rpcErrorf(codeInternalError, \"no se pudo leer qué proyectos barre el cerebro"
// arnes: a="\t\t\t_ = rpcErrorf(codeInternalError, \"no se pudo leer qué proyectos barre el cerebro"
func TestConLaListaDelBarridoIlegibleElInventarioNoDaUnVeredicto(t *testing.T) {
	s, d := maquinaConPolitica(t, []string{"metrics", "exec"})
	s.buscarPrincipal = registroDePrueba(autoHeal())
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(95, ahora), ahora) // 95 % de RAM: la condición se cumple

	consultas := 0
	s.engine = almacenSinListaDelBarrido{StorageBackend: s.engine, consultas: &consultas}

	// 1. EL INVENTARIO: un error que diga qué no pudo leer, y ningún veredicto.
	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_list", map[string]any{})
	delInventario := consultas
	if delInventario == 0 {
		t.Fatalf("musubi_fleet_list no consultó la lista del barrido: el inventario no pasó por la rama de error y esta prueba no la mide")
	}
	if e == nil {
		t.Fatalf("con la lista del barrido ilegible, musubi_fleet_list devolvió un inventario: cualquier `puede_actuar` "+
			"ahí es inventado, porque con esa misma falla el barrido no corre sobre nadie.\n%v", res)
	}
	if e.Code != codeInternalError || !strings.Contains(e.Message, "qué proyectos barre") {
		t.Errorf("el error no dice qué no pudo leer (code=%d, %q): quien lo ve tiene que saber que falló la lista del "+
			"barrido y no la del inventario", e.Code, e.Message)
	}

	// 2. EL BARRIDO, con la misma consulta rota: no actúa.
	antes := comandosDePolitica(t, s)
	s.barrerFlotaUnaVez(context.Background())
	if consultas == delInventario {
		t.Fatalf("barrerFlotaUnaVez no consultó la lista: el barrido no pasó por la rama de error y esta prueba no la mide")
	}
	if n := comandosDePolitica(t, s) - antes; n != 0 {
		t.Errorf("con la lista ilegible el barrido encoló %d comando(s) de política: no tenía tenants que barrer", n)
	}
}

// A132 · EL FRENO DEL BARRIDO VA ANTES QUE LA VENTANA.
//
// Con el barrido apagado —o el tenant fuera del tope— Y la máquina en ventana, la política no actúa
// por las dos cosas, y `puede_actuar` es `false` se ordenen como se ordenen. Lo que cambia es
// `inerte_por`, que es a dónde manda a mirar el panel: informar `mantenimiento` manda a cerrar una
// ventana que no destraba nada, porque cerrada la ventana el barrido sigue sin pasar. El orden es el
// del barrido: RunFlotaScheduler vuelve antes de todo, y el tope recorta los tenants antes de que
// aplicarPoliticas consulte las ventanas.
//
// Sabotaje: que porQueNoActuaria deje de anteponer el barrido. Los dos casos pasan a decir
// `mantenimiento`.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tif global := barrido.frenoSobre(d.ProjectID); global != sinFreno {\n\t\treturn global\n\t}\n"
// arnes: a=""
func TestElFrenoDelBarridoVaAntesQueLaVentana(t *testing.T) {
	casos := []struct {
		caso  string
		armar func(t *testing.T, s *McpServer, ahora time.Time)
		freno frenoDePolitica
	}{
		{"barrido apagado y máquina en ventana", func(t *testing.T, s *McpServer, _ time.Time) {
			if err := s.ConfigurarFlota(config.FleetConfig{ProbeMinutes: -1, Policies: []config.PolicyConfig{politicaDeMemoria()}}); err != nil {
				t.Fatalf("ConfigurarFlota: %v", err)
			}
		}, frenoBarridoApagado},
		{"tenant fuera del tope y máquina en ventana", func(t *testing.T, s *McpServer, ahora time.Time) {
			tenantsAlrededorDeCasa(t, s, proyectosParaVigilar, 0, ahora)
		}, frenoFueraDelBarrido},
	}
	for _, c := range casos {
		t.Run(c.caso, func(t *testing.T) {
			s, d := maquinaConPolitica(t, []string{"metrics", "exec"})
			s.buscarPrincipal = registroDePrueba(autoHeal())
			ahora := time.Now()
			c.armar(t, s, ahora)
			if _, err := s.engine.AbrirMantenimiento(fleet.Mantenimiento{
				DeviceID: d.ID, ProjectID: d.ProjectID, Principal: "gio",
				Desde: ahora.Add(-time.Minute), Hasta: ahora.Add(time.Hour), Motivo: "migración de postgres",
			}); err != nil {
				t.Fatalf("AbrirMantenimiento: %v", err)
			}
			latir(t, s, d.ID, muestraSana(95, ahora), ahora)

			puede, inertePor, visible := politicaEnElInventario(t, s)
			if !visible {
				t.Fatal("el detalle de la política no viajó a una credencial con exec:*: no hay `inerte_por` que comparar")
			}
			if puede {
				t.Fatalf("con %s el inventario dice `puede_actuar: true`: esta prueba mide el orden de dos frenos y ninguno frenó", c.caso)
			}
			if inertePor != string(c.freno) {
				t.Errorf("con %s el inventario dice `inerte_por: %q` y tenía que decir %q: el panel manda a arreglar "+
					"algo que no destraba nada", c.caso, inertePor, c.freno)
			}
		})
	}
}

// almacenQueCuentaLaLista cuenta las consultas a la lista de tenants con máquinas, sin romperla.
type almacenQueCuentaLaLista struct {
	memory.StorageBackend
	consultas *int
}

func (a almacenQueCuentaLaLista) ProyectosConDevices(tope int) ([]string, error) {
	*a.consultas++
	return a.StorageBackend.ProyectosConDevices(tope)
}

// A132 · SIN POLÍTICAS, EL INVENTARIO NO PREGUNTA POR EL BARRIDO.
//
// El estado del barrido sólo sirve para decir si una política actuaría: sin políticas no hay nada
// que decidir, y el inventario no suma una consulta por pedido para nada. Es la misma regla que ya
// seguían las ventanas de mantenimiento. Y con una política la pregunta tiene que hacerse: ése es el
// piso que prueba que el contador ve la consulta del inventario.
//
// Sabotaje: leer el estado de las políticas aunque no haya ninguna.
// arnes: archivo="internal/mcp/methods_fleet.go"
// arnes: de="\tif len(s.politicas) > 0 {\n"
// arnes: a="\tif true {\n"
func TestSinPoliticasElInventarioNoPreguntaPorElBarrido(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	// COMO ARRANCA UN CEREBRO DE VERDAD: `musubi serve` corre ConfigurarFlota siempre, con el sondeo
	// en su default aunque no haya ninguna política. Sin esto el intervalo queda en 0, barridoVigente
	// vuelve por barridoApagado antes de mirar la lista, y esta prueba no medía nada: su sabotaje
	// quedaba en verde (lo cazó `arnes -correr`).
	if err := s.ConfigurarFlota(config.FleetConfig{}); err != nil {
		t.Fatalf("ConfigurarFlota sin políticas: %v", err)
	}
	if s.barridoApagado() {
		t.Fatal("con la configuración por default el barrido figura apagado: barridoVigente no llegaría a la lista y esta prueba no mediría nada")
	}
	enrolarDePrueba(t, s, "casa", "pc-gio")
	consultas := 0
	s.engine = almacenQueCuentaLaLista{StorageBackend: s.engine, consultas: &consultas}

	if _, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_list", map[string]any{}); e != nil {
		t.Fatalf("fleet_list: %+v", e)
	}
	if consultas != 0 {
		t.Errorf("sin ninguna política configurada, musubi_fleet_list consultó %d vez/veces la lista del barrido: "+
			"no hay nada que decidir con ella", consultas)
	}

	// PISO: con una política el inventario SÍ pregunta. Sin esto, un contador que no viera la
	// consulta dejaría la aserción de arriba en verde sin medir nada.
	if err := s.ConfigurarFlota(config.FleetConfig{Policies: []config.PolicyConfig{politicaDeMemoria()}}); err != nil {
		t.Fatalf("ConfigurarFlota: %v", err)
	}
	if _, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_list", map[string]any{}); e != nil {
		t.Fatalf("fleet_list con una política: %+v", e)
	}
	if consultas == 0 {
		t.Fatal("con una política configurada, musubi_fleet_list no consultó la lista del barrido: el contador no ve " +
			"la consulta, y la aserción sin políticas no mide nada")
	}
}

// A132 · EL INVENTARIO NO DISPARA EL AVISO DEL RECORTE.
//
// El aviso `barrido_truncado:*` dice «hay tenants que un barrido dejó afuera», una vez por episodio,
// y lo da el barrido que recorta (proyectosAVigilar). El inventario lee la MISMA lista para decir
// `fuera_del_barrido`, pero no barre nada: si pasara por el envoltorio que avisa, cada vista del
// panel anotaría un episodio propio —un `barrido_truncado:inventario` que no corresponde a ningún
// barrido— y el log contaría recortes que nadie hizo.
//
// Sabotaje: que barridoVigente lea la lista por el envoltorio que avisa.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\tproyectos, _, err := s.proyectosDelBarrido()\n"
// arnes: a="\tproyectos, err := s.proyectosAVigilar(\"inventario\")\n"
func TestElInventarioNoDisparaElAvisoDelRecorte(t *testing.T) {
	s, d := maquinaConPolitica(t, []string{"metrics", "exec"})
	s.buscarPrincipal = registroDePrueba(autoHeal())
	ahora := time.Now()
	tenantsAlrededorDeCasa(t, s, proyectosParaVigilar, 0, ahora) // casa, la 65ª: el tope recorta
	latir(t, s, d.ID, muestraSana(95, ahora), ahora)

	// El log se captura para la prueba entera y se restaura con defer: un Fatal a la mitad no puede
	// dejar el logger global escribiendo en un buffer muerto para las pruebas que siguen.
	var log bytes.Buffer
	defer logx.Capturar(&log)()
	_, inertePor, _ := politicaEnElInventario(t, s)
	delInventario := log.String()
	if inertePor != string(frenoFueraDelBarrido) {
		t.Fatalf("casa es el tenant 65 y el inventario dice `inerte_por: %q`: esta prueba necesita el recorte puesto "+
			"para medir su aviso", inertePor)
	}
	if hayAvisoConPrefijo(s, "barrido_truncado:") {
		t.Errorf("leer el inventario dejó anotado un aviso `barrido_truncado:`: el inventario no barre, y un episodio " +
			"de recorte anotado por una vista del panel no corresponde a ningún barrido")
	}
	if strings.Contains(delInventario, "NO se vigilan") {
		t.Errorf("leer el inventario logueó el aviso del recorte, que es del barrido:\n%s", delInventario)
	}

	// PISO: el barrido real sí lo dice. Sin esto, un aviso que ya no existiera dejaría las dos
	// aserciones de arriba en verde sin medir nada.
	s.barrerFlotaUnaVez(context.Background())
	if !hayAvisoConPrefijo(s, "barrido_truncado:") {
		t.Fatal("el barrido real recortó y no anotó su aviso: esta prueba no tiene contra qué medir que el inventario no lo dispare")
	}
}

// A132 · UN TENANT QUE VUELVE A ENTRAR AL TOPE SE VE, Y ACTÚA — SOBRE EL MISMO SERVIDOR.
//
// Las filas del tope de la tabla arman cada celda en un servidor nuevo, así que no pueden ver una
// respuesta que se quede VIEJA: un inventario que guardara la lista la primera vez que la calcula
// diría `fuera_del_barrido` para siempre de un tenant que ya volvió a entrar, mientras el barrido lo
// repara. Acá casa empieza afuera, se revoca uno de los de adelante, y los dos lados tienen que
// enterarse en el mismo servidor.
//
// Sabotaje: que barridoVigente recuerde la primera lista que calculó (acá, en un mapa del servidor).
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\treturn barridoDePoliticas{proyectos: barridos}, nil\n"
// arnes: a="\tif v, ok := s.avisosDados.Load(\"memo:barrido\"); ok {\n\t\treturn v.(barridoDePoliticas), nil\n\t}\n\ts.avisosDados.Store(\"memo:barrido\", barridoDePoliticas{proyectos: barridos})\n\treturn barridoDePoliticas{proyectos: barridos}, nil\n"
func TestUnTenantQueVuelveAEntrarAlTopeSeVeYActua(t *testing.T) {
	s, d := maquinaConPolitica(t, []string{"metrics", "exec"})
	s.buscarPrincipal = registroDePrueba(autoHeal())
	ahora := time.Now()
	tenantsAlrededorDeCasa(t, s, proyectosParaVigilar, 0, ahora) // casa, la 65ª
	latir(t, s, d.ID, muestraSana(95, ahora), ahora)             // 95 % de RAM: la condición se cumple

	// 1. AFUERA: el inventario lo dice y el barrido no la visita.
	puede, inertePor, _ := politicaEnElInventario(t, s)
	antes := comandosDePolitica(t, s)
	s.barrerFlotaUnaVez(context.Background())
	if puede || inertePor != string(frenoFueraDelBarrido) || comandosDePolitica(t, s) != antes {
		t.Fatalf("con casa como tenant 65: puede_actuar=%v inerte_por=%q y comandos %d→%d; esta prueba necesita "+
			"arrancar con casa afuera del tope", puede, inertePor, antes, comandosDePolitica(t, s))
	}

	// 2. VUELVE A ENTRAR: se revoca uno de los tenants de adelante.
	if ok, err := s.engine.RevocarDevice("a-000", "server"); err != nil || !ok {
		t.Fatalf("RevocarDevice(a-000): ok=%v err=%v", ok, err)
	}
	todos, err := s.engine.ProyectosConDevices(proyectosParaVigilar + 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != proyectosParaVigilar || todos[proyectosParaVigilar-1] != "casa" {
		t.Fatalf("revocado a-000, casa tenía que quedar como tenant %d de %d y la lista es %v", proyectosParaVigilar, proyectosParaVigilar, todos)
	}

	// 3. LOS DOS LADOS SE ENTERAN, sobre el mismo servidor.
	puede, inertePor, _ = politicaEnElInventario(t, s)
	antes = comandosDePolitica(t, s)
	s.barrerFlotaUnaVez(context.Background())
	actuo := comandosDePolitica(t, s) > antes
	if !actuo {
		t.Errorf("casa volvió a entrar al tope y el barrido real no actuó sobre ella")
	}
	if puede != actuo {
		t.Errorf("casa volvió a entrar al tope: el inventario dice `puede_actuar: %v` (inerte_por %q) y el barrido "+
			"actuó=%v; una respuesta vieja deja a una máquina reparada figurando como inerte", puede, inertePor, actuo)
	}
}

// A132 · EL VALOR CERO DEL BARRIDO NO DICE QUE UNA POLÍTICA ACTUARÍA.
//
// barridoDePoliticas sólo lo arma barridoVigente. Su doc promete que el valor cero —el de un
// llamador que se olvide de pedirlo— dice `fuera_del_barrido` de todas: se equivoca para el lado de
// «inerte», que se ve, y no para el de «actuaría», que es la alarma apagada de A132. Hoy ningún
// camino de producción le pasa el valor cero; esta prueba es la que hace cierta la promesa del doc.
//
// Sabotaje: que el valor cero diga que no frena nada.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="func (b barridoDePoliticas) frenoSobre(proyecto string) frenoDePolitica {\n"
// arnes: a="func (b barridoDePoliticas) frenoSobre(proyecto string) frenoDePolitica {\n\tif b.proyectos == nil && !b.apagado {\n\t\treturn sinFreno\n\t}\n"
func TestElValorCeroDelBarridoNoDiceQueActuaria(t *testing.T) {
	var cero barridoDePoliticas
	for _, proyecto := range []string{"casa", "", "otro"} {
		if got := cero.frenoSobre(proyecto); got != frenoFueraDelBarrido {
			t.Errorf("el valor cero de barridoDePoliticas dice %q del proyecto %q: tenía que decir %q, el lado que se ve", got, proyecto, frenoFueraDelBarrido)
		}
	}
}

// A132 · EL SERVIDOR DE VERDAD ARRANCA EL BARRIDO QUE EL INVENTARIO DA POR CORRIENDO.
//
// barridoVigente dice «apagado» con barridoApagado, que es la condición con la que RunFlotaScheduler
// vuelve sin barrer. Eso supone que alguien LO ARRANCA, y lo que cumple esa suposición es UNA línea
// de ListenAndServeHTTP: `go s.RunFlotaScheduler(ctx)`. Si se perdiera en un refactor del arranque,
// el inventario seguiría diciendo `puede_actuar: true` —el barrido figura encendido— y no actuaría
// ninguna política. (Las shells vencidas ya no cuelgan de esta línea: desde A136 las cierra su
// propio vigía, que custodia TestElServidorAplicaLosTechosDeShellConOSinBarrido.) Es la forma
// que ya custodia TestRevocarSurteEfectoSinReiniciarElServidor con el watch del registro, una línea
// más arriba.
//
// Se arranca el servidor entero —loopback, puerto efímero, principals.yaml en disco— con una
// política cuya condición se cumple y un sondeo de 100 ms, y se exige que la política actúe.
//
// Sabotaje: no lanzar el barrido al arrancar.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\tgo s.RunFlotaScheduler(ctx)\n"
// arnes: a=""
func TestElServidorArrancaElBarridoQueElInventarioDaPorCorriendo(t *testing.T) {
	s, d := maquinaConPolitica(t, []string{"metrics", "exec"})
	if err := s.ConfigurarFlota(config.FleetConfig{ProbeMinutes: sondeoCortoDePrueba.Minutes(), Policies: []config.PolicyConfig{politicaDeMemoria()}}); err != nil {
		t.Fatalf("ConfigurarFlota: %v", err)
	}
	principals := registroEnDisco(t, []Principal{autoHeal()})
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(95, ahora), ahora) // 95 % de RAM: la condición se cumple

	ctx, cancelar := context.WithCancel(context.Background())
	termino := make(chan error, 1)
	go func() {
		termino <- s.ListenAndServeHTTP(ctx, config.ServiceConfig{Addr: "127.0.0.1:0", PrincipalsFile: principals, RequestTimeoutSeconds: 10})
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
		// ListenAndServeHTTP no espera a la goroutine del barrido. Antes de que el cleanup de
		// newTestServer cierre la base se deja pasar más de un tick y se espera a que flotaBusy baje.
		// NO es una garantía, y queda dicho (segunda vuelta de la revisión de A132): tras el cancel,
		// el select de RunFlotaScheduler todavía puede elegir un tick pendiente, y un barrido que
		// arranque después de esta espera no se ve. Desde A136 el tramo es más angosto —el barrido ya
		// no cierra shells, que era lo que escribía ANTES de levantar flotaBusy—, pero sigue abierto.
		// El que llegue tarde recibe «sql: database is closed» y lo loguea; no rompe esta prueba.
		// Cerrarlo del todo pide que ListenAndServeHTTP espere también al barrido, que tarda lo que
		// tarden sus sondas SSH (al vigía de las shells sí lo espera).
		time.Sleep(3 * sondeoCortoDePrueba)
		for fin := time.Now().Add(esperaDelScheduler); s.flotaBusy.Load() && time.Now().Before(fin); {
			time.Sleep(10 * time.Millisecond)
		}
	})

	limite := time.Now().Add(esperaDelScheduler)
	for comandosDePolitica(t, s) == 0 {
		if time.Now().After(limite) {
			t.Fatalf("el servidor lleva %s arriba con un sondeo de %s y una política cuya condición se cumple, y no "+
				"actuó: nadie arrancó el barrido, y el inventario lo sigue dando por corriendo", esperaDelScheduler, sondeoCortoDePrueba)
		}
		select {
		case err := <-termino:
			termino <- err // lo devuelve para que el cleanup no espere uno que ya llegó
			t.Fatalf("ListenAndServeHTTP terminó antes de tiempo: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
	}
}
