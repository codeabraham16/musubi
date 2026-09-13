package mcp

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// servidorQueFedera arma un server con la federación del grafo ENCENDIDA y apuntando a un central
// que no contesta hasta que se lo suelta.
//
// El gate de la federación son dos cosas —syncClient configurado Y team mode— y las dos tienen que
// estar: con cualquiera apagada el push es un no-op sin red, la sonda no mediría nada, y el verde no
// significaría absolutamente nada.
//
// EL TIMEOUT DEL CLIENTE ES MAYOR QUE LA PACIENCIA DE LA SONDA a propósito: con `newTestSyncClient`
// —2 s— el POST moriría antes que `esperaMax`, el candado se soltaría solo y el bloqueo nunca se
// declararía. La prueba igual podría ponerse roja, pero por otra aserción y cubriendo otra cosa.
func servidorQueFedera(t *testing.T, central *centralQueCuelga) *McpServer {
	t.Helper()
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	ts := httptest.NewServer(central)
	t.Cleanup(ts.Close)
	// EL ORDEN IMPORTA Y ES AL REVÉS: los cleanups corren en orden INVERSO al que se registran, así
	// que éste —registrado después— se ejecuta ANTES de `ts.Close`. Sin él, una prueba que muere en un
	// `t.Fatalf` deja el handler esperando para siempre, `httptest.Server.Close()` espera a los
	// handlers en vuelo, y el binario se cuelga hasta el timeout de `go test` — que NO imprime
	// `--- FAIL:`, así que el arnés no ve un rojo sino una corrida sin veredicto.
	t.Cleanup(central.liberar)

	t.Setenv("MUSUBI_TEST_TOKEN", "secreto-abc")
	cliente, err := NewSyncClient(config.SyncConfig{
		CentralURL:            ts.URL,
		AuthTokenEnv:          "MUSUBI_TEST_TOKEN",
		AllowInsecureToken:    true,
		RequestTimeoutSeconds: 120, // > esperaMax (30 s): el bloqueo tiene que sobrevivir a la sonda
	})
	if err != nil {
		t.Fatalf("NewSyncClient: %v", err)
	}
	s.syncClient = cliente
	// El otro medio gate. Se prende sobre el server ya armado —estamos en el mismo paquete— en vez de
	// reconstruirlo con WithMemory: menos andamiaje, y lo que se mide no es el constructor.
	s.memory.TeamMode = true
	return s
}

// sembrarGrafoPorEmpujar deja el grafo con algo NUEVO que federar.
//
// HACE FALTA Y NO ES ADORNO. Desde #505 el push no sale en cada corrida: `grafoPorEmpujar` compara la
// generación durable de la base contra la ya empujada, y sin nada nuevo el tick termina sin tocar la
// red. La primera versión de la prueba del tick indexaba un directorio VACÍO y por eso nunca llegaba
// al central: daba rojo por la aserción de viveza —«no llegó»— sin haber medido jamás el candado.
// Escribir un nodo avanza la generación, y recién ahí hay push que medir.
func sembrarGrafoPorEmpujar(t *testing.T, s *McpServer) {
	t.Helper()
	nodo := memory.GraphNode{Key: "x.go#func:X", Kind: "func", Name: "X", Path: "x.go", SrcFingerprint: "1"}
	if err := s.engine.ReplaceProjectGraphFrom("", []memory.GraphNode{nodo}, nil); err != nil {
		t.Fatalf("no se pudo sembrar el grafo: %v — sin esto el tick no empuja y no se mide nada", err)
	}
}

// TestIndexarElGrafoNoCongelaElServidor mide el EFECTO de `lockSelf` en musubi_codegraph_index.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA CUARTA DE LAS SIETE, Y LA QUE CRUZA DOS FRONTERAS
//
// Las tres conversiones anteriores cruzaban una sola: un POST al central. Ésta cruza dos, y la
// segunda no es red — `commitDeHEAD` lanza un PROCESO (`exec.CommandContext` sobre `git rev-parse`).
// La regla de server.go no distingue: ningún candado del despacho puede cruzar ninguna de las dos.
//
// No declara `readOnly` —indexa, o sea escribe—, así que lo que sostenía era el EXCLUSIVO: el
// servidor entero sin atender a nadie mientras esperaba al cerebro.
//
// MEDIDO ANTES DE ARREGLAR NADA, sobre la punta de main que ya traía #505: esta prueba fallaba en
// 30,12 s por la aserción de bloqueo. El defecto estaba entero; el arreglo la pone en 0,09 s.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL SELLO DEL HEAD ERA LA TRAMPA FINA
//
// `sellarHeadDelIndice` recibe QUIÉN escribe. La tool le pasaba `directo` —«yo ya tengo el candado
// del despacho»—, que era cierto sólo mientras el despachador se lo tomaba. Declarar `lockSelf` sin
// tocar eso habría dejado la escritura de la meta SIN NINGÚN candado: el arreglo del cuelgue pagado
// con una escritura suelta. Ahora pasa `s.withWriteLock`, igual que el scheduler.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL CONTROL ES `<-central.entro`, Y ACÁ NO ES OPCIONAL
//
// El push es BEST-EFFORT y se traga sus errores: si falla, la tool igual devuelve éxito y sólo marca
// `federated:false`. «La llamada volvió sin error» NO prueba que el push haya ocurrido — una versión
// con el gate apagado pasaría igual. Lo único que lo prueba es haber esperado a estar ADENTRO del
// POST antes de sondear.
//
// LA SONDA ES ESCRITORA a propósito: la única que se bloquea con CUALQUIER candado tomado.
//
// Sabotaje que la hace fallar: envolver en un `withWriteLock` la llamada al push que hace ESTA tool,
// con lo que la red vuelve a ocurrir con el candado del despacho tomado.
//
// POR QUÉ NO «SACARLE `lock: lockSelf`», QUE SERÍA EL OBVIO. Sin la declaración el despachador toma
// dispatchMu sobre el handler entero y adentro el handler pide el MISMO mutex en su withWriteLock.
// Eso es un DEADLOCK, no lentitud: la llamada no llega nunca a la red y la prueba cae por la
// aserción de VIVEZA, midiendo un cuelgue distinto del que dice medir. Se probó y se descartó
// leyendo la línea. (De paso: esa declaración ya no es una optimización — sacarla cuelga la tool.)
//
// Y POR QUÉ NO EL `PushGraph` DE `empujarFotoDelGrafo`, que fue el primer intento: esa línea es el
// CUELLO COMPARTIDO, y el censo denunció que ahí ya anclan dos guardas de #505
// (TestUnaFotoQueFallaNoMandaNadaAlCentral y TestUnPushFallidoDeLaToolLoReintentaElScheduler). Con
// varias directivas sobre el mismo literal, a lo sumo una mide comportamiento y las otras pueden
// estar cayendo por el daño al corpus. La salida no es declarar seis `colision_ok`, es que cada
// prueba saboteé SU PROPIO llamador — así el rojo no admite dos lecturas.
//
// El ancla es única (`Count == 1`), medida con los tabs a la vista después de correr gofmt.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="\tif attempted, ok := s.pushCodeGraphToCentral(ctx); attempted {"
// arnes: a="\ts.withWriteLock(func() { _, _ = s.pushCodeGraphToCentral(ctx) })\n\tif attempted, ok := s.pushCodeGraphToCentral(ctx); attempted {"
// arnes: prueba="TestIndexarElGrafoNoCongelaElServidor"
func TestIndexarElGrafoNoCongelaElServidor(t *testing.T) {
	central := &centralQueCuelga{entro: make(chan struct{}), soltar: make(chan struct{})}
	s := servidorQueFedera(t, central)

	indexado := make(chan *RpcError, 1)
	go func() {
		// llamarSinT y no call: esto corre en otra goroutine, y un `t.Fatal` desde ahí es un error
		// propio — la prueba seguiría corriendo con el resultado a medias.
		indexado <- llamarSinT(s, "musubi_codegraph_index", map[string]interface{}{})
	}()

	select {
	case <-central.entro:
	case <-time.After(esperaArranque):
		t.Fatal("musubi_codegraph_index no llegó al central: esta prueba no pudo empezar a medir, " +
			"así que su verde no significaría nada")
	}

	exigeQueUnEscritorSinEmbedderResponda(t, s,
		"musubi_codegraph_index está colgada empujando el grafo al central")

	// SOLTAR ES LO ÚNICO QUE TERMINA EL VIAJE, ahora que el cliente espera 120 s. Una prueba que
	// depende de un timeout para destrabarse mide el timeout, no el candado.
	central.liberar()
	select {
	case rpcErr := <-indexado:
		if rpcErr != nil {
			t.Fatalf("al soltar el central, el índice falló: %+v", rpcErr)
		}
	case <-time.After(esperaMax):
		t.Fatal("soltado el central, el índice igual no volvió: el candado quedó tomado por otra cosa")
	}
}

// TestElReindexadoDeFondoNoCongelaElServidor custodia el camino que la guarda estructural NO VE.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// ESTA MITAD NO ES MÍA: LA ARREGLÓ #505. ACÁ SE LE PONE LA GUARDA.
//
// `reindexCodeGraphOnce` hacía lo mismo que la tool —indexar y empujar el grafo por HTTP— con
// `dispatchMu.Lock()` sobre el cuerpo entero. Figuró como deuda declarada en #503 y #504, y #505 lo
// arregló: hoy el candado cubre sólo el tramo que escribe y la federación queda afuera.
//
// Lo que NO tenía era una prueba que lo fijara, y no la puede tener la guarda estructural: ésa cruza
// el grafo de llamadas contra las ENTRADAS DEL REGISTRO, y un ticker no tiene entrada. Es la forma de
// defecto que este repo llama dominante —la lección aplicada en N−1 de N caminos—, y sin esto una
// regresión en el tick sería invisible mientras el lado de la tool se ve impecable.
//
// LA GUARDA QUE YA EXISTÍA TAMPOCO ALCANZA. `TestUnaCorridaDelGrafoNoSeQuedaConElCandado` exige que
// la corrida no se QUEDE con el candado al terminar, y eso seguía siendo cierto con el defecto
// puesto: se soltaba, después del push. Nadie medía que no lo sostuviera DURANTE. Por eso la sonda
// corre con el central colgado y no después.
//
// Sabotaje que la hace fallar: tomar `dispatchMu` justo antes de la federación y sostenerlo con un
// `defer` hasta el final del tick. No es una envoltura artificial — es LITERALMENTE lo que este
// camino hacía antes de #505: el candado del despacho puesto durante el POST al central.
//
// CADA PRUEBA SABOTEA SU PROPIO LLAMADOR, y no una línea compartida. Hubo dos intentos previos y los
// dos los denunció el censo:
//
//	el `PushGraph` de empujarFotoDelGrafo ... ahí ya anclan DOS guardas de #505
//	la llamada `s.empujarGrafoSiHaceFalta` ... la directiva de
//	                                          TestUnCambioQueIndexoOtroDaemonLlegaAlCentral usa ese
//	                                          mismo texto SIN el tab inicial, así que su literal es
//	                                          subcadena del mío y cada mutación rompe el ancla ajena
//
// Con varias directivas sobre la misma línea, el rojo de cualquiera admite dos lecturas: midió el
// comportamiento, o se rompió el corpus. Por eso el ancla se movió dos veces en vez de declarar
// `colision_ok`: declarar es la salida cuando NO hay ancla limpia, no la primera opción.
//
// El ancla es única (`Count == 1`) y #505 no la toca.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="\ts.sellarHeadDelIndice(fallidos, s.withWriteLock)"
// arnes: a="\ts.sellarHeadDelIndice(fallidos, s.withWriteLock)\n\ts.dispatchMu.Lock()\n\tdefer s.dispatchMu.Unlock()"
// arnes: prueba="TestElReindexadoDeFondoNoCongelaElServidor"
func TestElReindexadoDeFondoNoCongelaElServidor(t *testing.T) {
	central := &centralQueCuelga{entro: make(chan struct{}), soltar: make(chan struct{})}
	s := servidorQueFedera(t, central)
	sembrarGrafoPorEmpujar(t, s)

	volvio := make(chan struct{})
	go func() {
		s.reindexCodeGraphOnce(context.Background())
		close(volvio)
	}()

	select {
	case <-central.entro:
	case <-time.After(esperaArranque):
		// DOS CAUSAS POSIBLES Y CONVIENE NOMBRAR LAS DOS: o el tick no tenía nada que federar —y
		// entonces esta prueba no midió nada, que es lo que pasaba antes de sembrar el grafo—, o hay
		// un candado tomado POR AFUERA que re-entra al de adentro del push, que no es lentitud sino
		// un deadlock y desde acá se ve igual.
		t.Fatal("el reindexado de fondo no llegó al central: o no tenía nada nuevo que empujar, o " +
			"hay un candado tomado por afuera que re-entra al de adentro (deadlock). En cualquier " +
			"caso esta prueba no pudo empezar a medir, así que su verde no significaría nada")
	}

	exigeQueUnEscritorSinEmbedderResponda(t, s,
		"el reindexado de fondo está colgado empujando el grafo al central")

	central.liberar()
	select {
	case <-volvio:
	case <-time.After(esperaMax):
		t.Fatal("soltado el central, el reindexado igual no volvió: el candado quedó tomado por otra cosa")
	}
}
