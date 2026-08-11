package mcp

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// Invariantes del spec «El motor no traba la casa» (specs/motor-sin-candado/).
//
// EL PUNTO DE TODO ESTE ARCHIVO: ningún candado del despacho puede sostenerse durante una llamada
// de red. Medido en el central antes del arreglo, un musubi_ask real tardó 25 s con el candado
// EXCLUSIVO tomado — nadie más pudo entrar en ese rato.
//
// POR QUÉ LOS FALSOS BLOQUEAN EN VEZ DE DORMIR: una prueba escrita con time.Sleep pasa IGUAL con el
// candado puesto (la otra tool simplemente espera y termina "a tiempo" si el sleep es corto). Con
// un falso que se cuelga hasta que la prueba lo suelta, el verde SÓLO es posible si la otra tool
// corrió de verdad en paralelo.

// LOS DOS PRESUPUESTOS SON GENEROSOS A PROPÓSITO, y eso NO debilita las pruebas.
//
// Bajo el defecto que persiguen, la sonda queda bloqueada hasta que la prueba suelte el motor —o
// sea, indefinidamente—, así que el tamaño del timeout no cambia si el fallo se detecta: cambia
// sólo cuánto tarda en detectarse. Apretarlos, en cambio, sí produce falsos rojos: en un runner de
// CI cargado este paquete tardó 318 s y un presupuesto de 5 s se agotó ANTES de que el juez
// llegara a llamar al motor (windows-latest, PR #265).
const (
	// esperaArranque es una espera de VIVEZA: cuánto se le da al falso para recibir la llamada.
	// No es la aserción — es el preámbulo que la habilita.
	esperaArranque = 60 * time.Second
	// esperaMax es LA ASERCIÓN: cuánto tolera la sonda concurrente antes de declarar bloqueo.
	esperaMax = 30 * time.Second
)

// motorBloqueante se cuelga en Ask hasta que la prueba cierra `soltar`.
type motorBloqueante struct {
	entro    chan struct{}
	soltar   chan struct{}
	llamadas atomic.Int64
}

func nuevoMotorBloqueante() *motorBloqueante {
	return &motorBloqueante{entro: make(chan struct{}, 1), soltar: make(chan struct{})}
}

func (m *motorBloqueante) Name() string { return "fake-bloqueante" }

func (m *motorBloqueante) Ask(ctx context.Context, _, _ string) (string, error) {
	m.llamadas.Add(1)
	select {
	case m.entro <- struct{}{}:
	default:
	}
	select {
	case <-m.soltar:
		return `["a"]`, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

// embedderBloqueante se cuelga en Embed hasta que la prueba lo suelta. Mismo defecto que el motor:
// es I/O externa en el mismo camino, con 30 s de techo.
//
// El interruptor `activo` existe porque guardar una observación TAMBIÉN embebe: sin él, el
// embedder bloqueante frenaría la propia siembra y la prueba moriría antes de medir nada.
type embedderBloqueante struct {
	entro  chan struct{}
	soltar chan struct{}
	activo atomic.Bool
}

func nuevoEmbedderBloqueante() *embedderBloqueante {
	return &embedderBloqueante{entro: make(chan struct{}, 1), soltar: make(chan struct{})}
}

func (e *embedderBloqueante) Name() string    { return "fake-bloqueante" }
func (e *embedderBloqueante) Dimensions() int { return 3 }
func (e *embedderBloqueante) Embed(ctx context.Context, _ string) ([]float32, error) {
	if !e.activo.Load() {
		return []float32{1, 0, 0}, nil
	}
	select {
	case e.entro <- struct{}{}:
	default:
	}
	select {
	case <-e.soltar:
		return []float32{1, 0, 0}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// llamarSinT despacha una tool sin depender de *testing.T: se usa desde goroutines, donde t.Fatal
// no está permitido.
func llamarSinT(s *McpServer, name string, args map[string]interface{}) *RpcError {
	argBytes, _ := json.Marshal(args)
	params, _ := json.Marshal(CallToolRequest{Name: name, Arguments: argBytes})
	_, rpcErr := s.handleToolsCall(context.Background(), params)
	return rpcErr
}

// esperarQueElFalsoReciba es el preámbulo de G3/G4/G5, y distingue TRES desenlaces en vez de uno.
//
// POR QUÉ EXISTE. La primera versión sólo esperaba `entro` contra un timeout, así que cuando el
// falso no era llamado —por el motivo que fuera— la prueba quemaba el presupuesto entero y culpaba
// al reloj. En CI (PR #265) eso dio un mensaje FALSO: decía «revisá que la flag esté encendida»
// tras 60 s, cuando el dato útil era que la tool bajo prueba ya había TERMINADO sin tocar el falso.
//
// Con `terminó` en el select, ese caso se detecta al instante y se nombra por lo que es —una
// precondición rota, no un bloqueo— y el presupuesto de tiempo sólo cubre lo que debe cubrir: una
// máquina lenta.
func esperarQueElFalsoReciba(t *testing.T, entro <-chan struct{}, termino <-chan struct{}, quien string, errTool func() *RpcError) {
	t.Helper()
	select {
	case <-entro:
	case <-termino:
		t.Fatalf("PRECONDICIÓN ROTA: la tool terminó sin llamar a %s (error devuelto: %+v). "+
			"No es un problema de concurrencia — revisá que el recall devuelva ≥2 items y que la flag esté encendida",
			quien, errTool())
	case <-time.After(esperaArranque):
		t.Fatalf("%s no recibió la llamada en %v y la tool sigue en vuelo: la máquina está muy lenta o hay un bloqueo antes de llegar",
			quien, esperaArranque)
	}
}

// sembrar deja N observaciones que compartan un término, para que el recall devuelva ≥2 items (el
// juez no se activa con menos).
func sembrar(t *testing.T, s *McpServer, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		_, rpcErr := call(t, s, "musubi_save_observation", map[string]interface{}{
			"topic_key": "candado/prueba",
			"content":   "el candado del despacho no puede cruzar una llamada de red, caso " + string(rune('a'+i)),
		})
		if rpcErr != nil {
			t.Fatalf("no pude sembrar la observación %d: %+v", i, rpcErr)
		}
	}
}

// servidorConMotor devuelve TAMBIÉN el engine concreto: s.engine es la interfaz StorageBackend y no
// expone CountObservations, que es lo que necesita exigeSembradas para verificar la precondición.
func servidorConMotor(t *testing.T, motor *motorBloqueante, cfg config.CognitionConfig, emb embedding.Provider) (*McpServer, *memory.DbEngine) {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	// LA DETECCIÓN DE CONFLICTOS VA APAGADA, y no es higiene: era la causa de un fallo intermitente
	// REAL de G4 en CI. Medido: 4 fallos en 400 corridas locales antes de esto, 0 después.
	//
	// La siembra de estos tests son observaciones casi idénticas —difieren en un carácter— bajo el
	// mismo topic_key: justo lo que el detector llama casi-duplicado. El auto-supersede dispara si la
	// segunda es ESTRICTAMENTE más nueva, y esa comparación se hace sobre `created_at`, que es el
	// CURRENT_TIMESTAMP de SQLite y tiene RESOLUCIÓN DE UN SEGUNDO. Si los tres saves caen dentro del
	// mismo segundo no pasa nada; si el segundo tickea entre el segundo y el tercero, la tercera
	// oculta a las otras dos, el recall devuelve UN item, el juez no se activa —necesita ≥2— y la
	// tool termina sin llamar al motor. Ése era exactamente el síntoma: «la tool terminó sin llamar
	// a el juez», con error nil, una vez cada cien.
	//
	// Lo que estos tests prueban es que NINGÚN CANDADO DEL DESPACHO cruza una llamada de red. El
	// detector no tiene nada que ver con eso, así que apagarlo saca una variable que sólo aportaba
	// ruido. Es la misma decisión, por el mismo motivo, que ya había tomado TestMaintainTool.
	opts := []Option{WithCognitionConfig(cfg), WithConflicts(config.ConflictConfig{})}
	if motor != nil {
		opts = append(opts, WithCognition(motor))
	}
	return NewMcpServer(engine, t.TempDir(), emb, opts...), engine
}

// exigeSembradas verifica que la siembra sobrevivió entera Y VISIBLE.
//
// Contaba filas con CountObservations, y ahí estaba el segundo defecto: el auto-supersede NO BORRA
// la fila, le prende una marca que la esconde del recall. O sea que la precondición daba verde con
// 3 filas mientras el recall veía 1 — medía bien, pero otra cosa. El test se caía después, dos pasos
// más adelante, con un mensaje que mandaba a revisar el candado.
//
// Ahora se verifica lo que el juez realmente necesita: cuántos items DEVUELVE el recall. Y cuando
// falla, el mensaje dice las dos cifras, para que el próximo rojo se diagnostique en una corrida y
// no en una tarde.
func exigeSembradas(t *testing.T, engine *memory.DbEngine, n int) {
	t.Helper()
	total, err := engine.CountObservations()
	if err != nil {
		t.Fatalf("CountObservations: %v", err)
	}
	res, err := engine.Recall(context.Background(), "candado despacho red", memory.RecallOptions{NoBump: true})
	if err != nil {
		t.Fatalf("Recall de precondición: %v", err)
	}
	if len(res.Items) < n {
		t.Fatalf("PRECONDICIÓN ROTA: sembré %d observaciones, hay %d filas en la base y el recall "+
			"devuelve %d items visibles. El juez necesita ≥2 para activarse. Filas == %d con menos "+
			"items visibles significa que algo las OCULTÓ (auto-supersede de casi-duplicados) en vez "+
			"de borrarlas", n, total, len(res.Items), total)
	}
}

// exigeQueOtraToolResponda corre una tool en paralelo y falla si no termina dentro de esperaMax.
// Es la aserción central de G3/G4/G5.
func exigeQueOtraToolResponda(t *testing.T, s *McpServer, motivo string) {
	t.Helper()
	listo := make(chan *RpcError, 1)
	arranque := time.Now()
	go func() {
		listo <- llamarSinT(s, "musubi_save_observation", map[string]interface{}{
			"topic_key": "candado/concurrente",
			"content":   "esta escritura tiene que entrar mientras la otra tool espera a la red",
		})
	}()
	select {
	case rpcErr := <-listo:
		if rpcErr != nil {
			t.Fatalf("la tool concurrente falló: %+v", rpcErr)
		}
		// El número, no la intención: con el candado cruzando la red esto esperaría hasta que el
		// motor conteste (120 s de techo en producción). Visible con `go test -v`.
		t.Logf("la tool concurrente respondió en %v con el motor todavía colgado", time.Since(arranque).Round(time.Millisecond))
	case <-time.After(esperaMax):
		t.Fatalf("%s: la tool concurrente quedó bloqueada — el candado del despacho está cruzando una llamada de red", motivo)
	}
}

// exigeQueUnEscritorSinEmbedderResponda es la sonda de G5. Tiene dos requisitos que la sonda de
// G3/G4 no necesita:
//
//   - NO puede embeber, porque guardar una observación también pasa por el embedder y la sonda
//     quedaría atrapada en el mismo cuelgue que se está midiendo.
//   - TIENE que ser escritora (candado exclusivo). Una sonda de sólo lectura no serviría: si el
//     embed volviera a quedar adentro de la sección crítica —que toma RLock— dos lectores conviven
//     y la prueba pasaría igual. El escritor es el único que se bloquea con CUALQUIER candado
//     tomado, sea compartido o exclusivo.
//
// musubi_save_fact cumple las dos: guarda una tripleta y no toca el embedder.
func exigeQueUnEscritorSinEmbedderResponda(t *testing.T, s *McpServer, motivo string) {
	t.Helper()
	listo := make(chan *RpcError, 1)
	arranque := time.Now()
	go func() {
		listo <- llamarSinT(s, "musubi_save_fact", map[string]interface{}{
			"subject":   "el candado",
			"predicate": "no cruza",
			"object":    "una llamada de red",
		})
	}()
	select {
	case rpcErr := <-listo:
		if rpcErr != nil {
			t.Fatalf("la escritura concurrente falló: %+v", rpcErr)
		}
		t.Logf("la escritura concurrente respondió en %v con el embedder todavía colgado", time.Since(arranque).Round(time.Millisecond))
	case <-time.After(esperaMax):
		t.Fatalf("%s: la escritura concurrente quedó bloqueada — hay un candado tomado durante una llamada de red", motivo)
	}
}

// ---------------------------------------------------------------------------
// H1 · Los dos ejes se separan
// ---------------------------------------------------------------------------

// G1 — La clase por default reproduce EXACTAMENTE la de hoy: toda tool que no declara clase
// conserva el candado que tenía antes del refactor. Sin esto, el cambio puede mover el candado de
// una tool cualquiera y nadie se entera hasta que se pierde una escritura.
func TestG1ClasePorDefaultEsLaDeHoy(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	declaradas := map[string]bool{"musubi_recall": true, "musubi_ask": true}
	for _, e := range s.tools {
		clase, hayClase := s.toolLock[e.Name]
		if declaradas[e.Name] {
			if !hayClase || clase != lockSelf {
				t.Errorf("%s: esperaba lockSelf declarado, obtuve clase=%v presente=%v", e.Name, clase, hayClase)
			}
			continue
		}
		if hayClase {
			t.Errorf("%s: no debería declarar clase de candado (el cero es el comportamiento histórico), obtuve %v", e.Name, clase)
		}
	}
	if len(s.toolLock) != len(declaradas) {
		t.Errorf("toolLock tiene %d entradas y sólo %d tools declaran clase: alguna se coló", len(s.toolLock), len(declaradas))
	}
}

// G2 — Las escrituras siguen serializadas. Es el invariante que el candado exclusivo existe para
// sostener: no alcanza con "no lo toqué", hay que probar que sobrevive.
func TestG2EscriturasConcurrentesNoSePisan(t *testing.T) {
	// El engine se arma acá y no con newTestServer porque la aserción necesita el *DbEngine
	// concreto: s.engine es la interfaz StorageBackend y no expone CountObservations.
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine error: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	const n = 20

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			llamarSinT(s, "musubi_save_observation", map[string]interface{}{
				"topic_key": "candado/serializado",
				"content":   "escritura concurrente número " + string(rune('a'+i)) + " que debe persistir entera",
			})
		}(i)
	}
	wg.Wait()

	total, err := engine.CountObservations()
	if err != nil {
		t.Fatalf("CountObservations: %v", err)
	}
	if total != n {
		t.Fatalf("esperaba %d observaciones tras %d escrituras concurrentes, hay %d: se perdió alguna", n, n, total)
	}
}

// ---------------------------------------------------------------------------
// H2 · Ningún candado del despacho cruza una llamada al motor
// ---------------------------------------------------------------------------

// G3 — Un musubi_ask lento NO bloquea a otra tool. Es el invariante central del spec.
func TestG3AskLentoNoBloqueaOtraTool(t *testing.T) {
	motor := nuevoMotorBloqueante()
	s, _ := servidorConMotor(t, motor, config.CognitionConfig{Provider: "fake"}, embedding.NoopProvider{})
	sembrar(t, s, 2)

	// El error de la tool se GUARDA en vez de descartarse: si el ask falla antes de llegar al
	// motor, ése es el dato que explica el fallo, y la primera versión lo tiraba a la basura.
	var errAsk atomic.Pointer[RpcError]
	hecho := make(chan struct{})
	go func() {
		errAsk.Store(llamarSinT(s, "musubi_ask", map[string]interface{}{"question": "candado despacho red"}))
		close(hecho)
	}()

	esperarQueElFalsoReciba(t, motor.entro, hecho, "el motor", errAsk.Load)

	exigeQueOtraToolResponda(t, s, "musubi_ask colgado en el motor")

	close(motor.soltar)
	<-hecho
}

// G4 — Un musubi_recall con el juez ENCENDIDO tampoco bloquea. Es una prueba APARTE de G3 y no un
// duplicado: el juez entra por otro camino (rerankIfEnabled dentro de toolRecall) y se puede
// arreglar uno dejando el otro roto.
func TestG4RecallConJuezLentoNoBloqueaOtraTool(t *testing.T) {
	motor := nuevoMotorBloqueante()
	si := true
	s, engine := servidorConMotor(t, motor, config.CognitionConfig{
		Provider:       "fake",
		ReadTimeRerank: &si,
	}, embedding.NoopProvider{})
	// El juez sólo se activa con ≥2 items, así que la siembra es una PRECONDICIÓN y se verifica en
	// vez de suponerse: si algo la colapsara (dedup, consolidación), la prueba tiene que decir eso
	// y no «el juez nunca llamó al motor», que fue el mensaje falso que dio en CI.
	sembrar(t, s, 3)
	exigeSembradas(t, engine, 3)

	var errRecall atomic.Pointer[RpcError]
	hecho := make(chan struct{})
	go func() {
		errRecall.Store(llamarSinT(s, "musubi_recall", map[string]interface{}{"query": "candado despacho red"}))
		close(hecho)
	}()

	esperarQueElFalsoReciba(t, motor.entro, hecho, "el juez", errRecall.Load)

	exigeQueOtraToolResponda(t, s, "musubi_recall colgado en el juez")

	close(motor.soltar)
	<-hecho
}

// G5 — El embedder tampoco corre bajo candado. Mismo camino y mismo defecto, con 30 s de techo:
// arreglar sólo el LLM dejaría un segundo cuello.
func TestG5EmbedderLentoNoBloqueaOtraTool(t *testing.T) {
	emb := nuevoEmbedderBloqueante()
	s, _ := servidorConMotor(t, nil, config.CognitionConfig{}, emb)
	sembrar(t, s, 2)
	emb.activo.Store(true) // recién ahora: sembrar también embebe

	var errRecall atomic.Pointer[RpcError]
	hecho := make(chan struct{})
	go func() {
		errRecall.Store(llamarSinT(s, "musubi_recall", map[string]interface{}{"query": "candado despacho red"}))
		close(hecho)
	}()

	esperarQueElFalsoReciba(t, emb.entro, hecho, "el embedder", errRecall.Load)

	exigeQueUnEscritorSinEmbedderResponda(t, s, "musubi_recall colgado en el embedder")

	close(emb.soltar)
	<-hecho
}

// G6 — El candado se suelta aunque el handler entre en pánico. Sin esto, el refactor cambia un
// cuello de botella (que se destraba solo en 120 s) por un DEADLOCK (que no se destraba nunca).
func TestG6ElCandadoSeSueltaAunqueHayaPanico(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})

	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatal("esperaba que el pánico se propagara fuera de withReadLock")
			}
		}()
		s.withReadLock(func() { panic("boom") })
	}()

	// La señal se manda DESDE ADENTRO de la sección crítica: así el verde prueba que el candado se
	// pudo tomar de verdad, no sólo que la goroutine llegó al final.
	libre := make(chan struct{}, 1)
	go func() {
		s.dispatchMu.Lock()
		libre <- struct{}{}
		s.dispatchMu.Unlock()
	}()
	select {
	case <-libre:
	case <-time.After(esperaMax):
		t.Fatal("el RLock quedó tomado tras el pánico: se cambió un cuello de botella por un deadlock")
	}
}

// ---------------------------------------------------------------------------
// H3 · La autorización no se mueve
// ---------------------------------------------------------------------------

// G7 — Un reader sigue sin poder llamar al motor. Es el control que impide que el arreglo de
// performance se convierta en una apertura de acceso: marcar recall como readOnly para destrabar el
// candado habría hecho exactamente eso.
func TestG7ReaderSigueSinPoderLlamarAlMotor(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	reader := &Principal{Name: "cabina", Role: RoleReader}

	for _, tool := range []string{"musubi_ask", "musubi_recall"} {
		params, _ := json.Marshal(CallToolRequest{Name: tool, Arguments: json.RawMessage(`{}`)})
		_, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), reader), params)
		if rpcErr == nil || rpcErr.Code != codeUnauthorized {
			t.Errorf("%s: un reader debería seguir sin autorización, obtuve %+v", tool, rpcErr)
		}
	}
}

// G8 — El mapa de autorización completo queda igual que ANTES del refactor.
//
// La lista va FIJA y escrita a mano a propósito. Derivarla del registry —comparar
// canCall(toolReadOnly[n]) contra e.readOnly— es una tautología: canCall devuelve exactamente
// readOnly para un reader, así que la prueba pasaría igual marcando cualquier tool como readOnly.
// Lo detectó el sabotaje «se marca musubi_recall como readOnly», que quedaba en verde.
//
// Estas son las 22 tools que un reader podía llamar antes de este spec. Si el conjunto cambia, la
// prueba se rompe y hay que decidir a conciencia si el cambio se quería — que es el punto.
func TestG8MapaDeAutorizacionIntacto(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	reader := &Principal{Name: "cabina", Role: RoleReader}

	esperadas := map[string]bool{
		"musubi_brain_graph": true, "musubi_code_context": true, "musubi_code_graph": true,
		"musubi_code_graph_viz": true, "musubi_cognition_stats": true, "musubi_conflicts": true,
		"musubi_detect_changes": true, "musubi_detect_stack": true, "musubi_discover_skills": true,
		"musubi_entity_context": true, "musubi_impact": true, "musubi_insights": true,
		"musubi_list_skills": true, "musubi_map": true, "musubi_recall_facts": true,
		"musubi_search_keyword": true, "musubi_search_semantic": true, "musubi_search_skills": true,
		"musubi_skill_usage": true, "musubi_sync_pull": true, "musubi_sync_status": true,
		"musubi_tool_usage": true, "musubi_whoami": true,
		// F3 · madurez medida: la cabina (read=all, write=none) es su consumidor principal, así que
		// un reader TIENE que poder llamarla. Si deja de poder, el tablero del cuerpo y el CRM se
		// quedan sin el puntaje y nadie se entera hasta que alguien abra la pantalla.
		"musubi_readiness": true,
	}

	for _, e := range s.tools {
		puede := reader.canCall(s.toolReadOnly[e.Name])
		if puede != esperadas[e.Name] {
			t.Errorf("%s: un reader %s llamarla, y antes del spec %s — la autorización se movió",
				e.Name,
				map[bool]string{true: "SÍ puede", false: "NO puede"}[puede],
				map[bool]string{true: "SÍ podía", false: "NO podía"}[esperadas[e.Name]])
		}
	}

	var llamables int
	for _, e := range s.tools {
		if reader.canCall(s.toolReadOnly[e.Name]) {
			llamables++
		}
	}
	if llamables != len(esperadas) {
		t.Errorf("un reader puede llamar %d tools y antes podía %d", llamables, len(esperadas))
	}
}

// G9 — La autorización NO se deriva de la clase de candado. El atajo obvio al implementar es
// volver a atar los dos ejes; esta prueba lo impide.
func TestG9LaClaseDeCandadoNoOtorgaAcceso(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	reader := &Principal{Name: "cabina", Role: RoleReader}

	for name := range s.toolLock {
		if reader.canCall(s.toolReadOnly[name]) && !s.toolReadOnly[name] {
			t.Errorf("%s está en lockSelf y eso le abrió la puerta a un reader: los ejes se volvieron a atar", name)
		}
	}
}

// ---------------------------------------------------------------------------
// H4 · El recall no pierde nada por ganar concurrencia
// ---------------------------------------------------------------------------

// G11 — Con el juez apagado —la config del central HOY— el recall no toca el motor. Si este spec
// cambiara algo ahí, lo cambiaría en producción sin que nadie lo haya pedido.
func TestG11ConElJuezApagadoElMotorNoSeToca(t *testing.T) {
	motor := nuevoMotorBloqueante()
	close(motor.soltar) // que no cuelgue: si lo llaman, queremos ver la llamada, no un timeout
	s, _ := servidorConMotor(t, motor, config.CognitionConfig{Provider: "fake"}, embedding.NoopProvider{})
	sembrar(t, s, 3)

	if _, rpcErr := call(t, s, "musubi_recall", map[string]interface{}{"query": "candado despacho red"}); rpcErr != nil {
		t.Fatalf("el recall falló: %+v", rpcErr)
	}
	if n := motor.llamadas.Load(); n != 0 {
		t.Fatalf("con read_time_rerank ausente el recall debe ser 100%% model-free, pero llamó al motor %d veces", n)
	}
}
