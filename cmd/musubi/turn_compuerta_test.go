package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// La compuerta del hook del turno (esUnaConsulta): un pedido de continuación o un aviso del sistema
// no busca memoria ni se guarda como pedido de la sesión. Las pruebas van con el motor REAL: con el
// fake, el recall devuelve lo que le digan fuera cual fuera la consulta, y una compuerta que no
// filtra nada se vería igual que una que filtra.

const marcaDeMemoria = "[Musubi — memoria relevante]"

// avisoDeMonitor es un <task-notification> con la forma de los que llegan al hook: una tarea de
// fondo terminó. Sus palabras («monitor», «event», «notification») están en el corpus de la prueba a
// propósito, como pasaba en la base real.
const avisoDeMonitor = "<task-notification>\n<task-id>b7x2</task-id>\n<status>completed</status>\n" +
	"<summary>Monitor event: notification del workflow</summary>\n</task-notification>"

// motorConCorpusDeRuido arma una base con notas que dicen «sigue», notas que comparten las palabras
// del aviso de un Monitor, y la nota del TLS del cerebro, que es la que un pedido de verdad trae.
func motorConCorpusDeRuido(t *testing.T) *memory.DbEngine {
	t.Helper()
	eng, err := memory.NewDbEngine(memtest.DirSembrado(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { eng.Close() })
	notas := map[string]string{
		"sigue-1":   "el deploy del central sigue pendiente hasta la ventana V3",
		"sigue-2":   "la bajada del sync sigue sin ritmo: un pull cada 5 minutos",
		"sigue-3":   "el kiosko sigue en blanco si la raspberry pierde el WiFi",
		"aviso-1":   "architecture/notifications: el sistema de notification del CRM con su badge",
		"aviso-2":   "el Monitor event del workflow completed avisa por task-notification",
		"tls":       "el cerebro va por TLS: se disca la IP y se verifica contra el nombre del certificado",
		"version-2": "la migración v2 del índice de vectores reescribe la tabla entera",
	}
	for id, c := range notas {
		if err := eng.SaveObservation(id, "prueba/"+id, c, nil); err != nil {
			t.Fatal(err)
		}
	}
	return eng
}

// turnoReal corre el hook entero con el motor real y devuelve el additionalContext.
func turnoReal(t *testing.T, eng *memory.DbEngine, sesion, prompt string) string {
	t.Helper()
	in, err := json.Marshal(map[string]string{"session_id": sesion, "prompt": prompt})
	if err != nil {
		t.Fatal(err)
	}
	loopCfg := config.LoopConfig{PerTurnRecall: true, RecallBudget: 400, DeltaInjection: true}
	return turnOutputWith(eng, loopCfg, pipeOff(), maOff(), config.Default().Memory, nil, strings.NewReader(string(in)), nil)
}

// recallSinCompuerta es lo que el recall del turno traería con ese prompt si nadie lo filtrara: el
// CONTROL de las pruebas, para que un «no trajo memoria» signifique «la compuerta la calló» y no
// «no había nada que traer».
func recallSinCompuerta(eng *memory.DbEngine, prompt string) string {
	return buildTurnRecall(eng, parametrosDelTurno{sesion: "control-" + prompt, prompt: prompt, presupuesto: 400, memCfg: config.Default().Memory})
}

// TestContinuacionYSistemaNoTraenMemoria: «sigue» y el aviso de un Monitor no traen memoria, y
// «sigue con el TLS del cerebro» sí, con la nota del TLS. El control prueba que sin la compuerta los
// dos primeros SÍ traían: el corpus tiene notas que dicen «sigue» y que comparten las palabras del
// aviso, como la base real.
//
// Sabotaje: la compuerta deja pasar los avisos del sistema.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif transcripts.EsDeSistema(prompt) {\n"
// arnes: a="\tif prompt == \"\" && transcripts.EsDeSistema(prompt) {\n"
//
// Sabotaje: la compuerta deja pasar los pedidos de continuación.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\treturn !transcripts.EsPedidoDeContinuacion(prompt)\n"
// arnes: a="\treturn !transcripts.EsPedidoDeContinuacion(prompt) || prompt != \"\"\n"
//
// Sabotaje: el recall del turno no mira la compuerta.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif loopCfg.PerTurnRecall && esConsulta {\n"
// arnes: a="\tif loopCfg.PerTurnRecall && (esConsulta || true) {\n"
func TestContinuacionYSistemaNoTraenMemoria(t *testing.T) {
	eng := motorConCorpusDeRuido(t)

	for _, ruido := range []string{"sigue", avisoDeMonitor} {
		if recallSinCompuerta(eng, ruido) == "" {
			t.Fatalf("CONTROL: sin la compuerta, %.30q tendría que traer memoria del corpus; si no trae, la prueba no mide nada", ruido)
		}
		if got := turnoReal(t, eng, "s-ruido", ruido); strings.Contains(got, marcaDeMemoria) {
			t.Errorf("%.30q no es una consulta y trajo memoria:\n%s", ruido, got)
		}
	}

	got := turnoReal(t, eng, "s-pedido", "sigue con el TLS del cerebro")
	if !strings.Contains(got, marcaDeMemoria) || !strings.Contains(got, "[id:tls]") {
		t.Errorf("«sigue con el TLS del cerebro» dice qué, y tenía que traer la nota del TLS:\n%s", got)
	}
}

// TestUnPromptSinTerminosNoBuscaMemoriaAlAzar: qué hace HOY el recall del turno con una consulta sin
// términos, y por qué la compuerta la trata como continuación. Sin un solo tramo de letras o dígitos
// («?») el recall no llega al FTS y trae las notas más recientes; con puras palabras vacías («de la»)
// el fallback de rankedTerms busca "de" OR "la". Las dos cosas son memoria al azar. Y al revés: «v2»
// es un término para el recall, y la compuerta no puede callarlo (con el tokenizador viejo de la
// lista, «v2» eran «v» y «2», dos runas sueltas, y se callaba).
//
// Sabotaje: la definición de término deja de descartar las palabras vacías.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\t\tif len([]rune(f)) <= 1 || ftsStopwords[strings.ToLower(f)] {\n"
// arnes: a="\t\tif len([]rune(f)) <= 1 || ftsStopwords[strings.ToLower(f)] && false {\n"
func TestUnPromptSinTerminosNoBuscaMemoriaAlAzar(t *testing.T) {
	eng := motorConCorpusDeRuido(t)

	for _, vacio := range []string{"?", "de la"} {
		if recallSinCompuerta(eng, vacio) == "" {
			t.Fatalf("CONTROL: sin la compuerta, %q trae memoria al azar; si ya no trae, cambió el recall y esta prueba hay que revisarla", vacio)
		}
		if got := turnoReal(t, eng, "s-vacio", vacio); strings.Contains(got, marcaDeMemoria) {
			t.Errorf("%q no tiene un solo término y trajo memoria al azar:\n%s", vacio, got)
		}
	}

	if got := turnoReal(t, eng, "s-v2", "v2"); !strings.Contains(got, "[id:version-2]") {
		t.Errorf("«v2» es un término para el recall y la compuerta lo calló:\n%s", got)
	}
}

// TestElPedidoSustantivoSeRecuerdaPorSesion: cada sesión guarda SUS pedidos sustantivos, los tres
// últimos, pasados por el redactor y DESPUÉS truncados; un «sigue» o un aviso del sistema no pisan el
// último, y el mismo pedido otra vez no vuelve a escribir. (El sabotaje de la compuerta que deja
// pasar los avisos también la pone roja: ver TestContinuacionYSistemaNoTraenMemoria.)
//
// Sabotaje: los pedidos no son por sesión.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\treturn metaPedidos + sepDeltaKey + sessionID\n"
// arnes: a="\treturn metaPedidos + sepDeltaKey\n"
//
// Sabotaje: el pedido se guarda sin pasar por el redactor.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\ttexto, _ := redact.Redact(prompt)\n"
// arnes: a="\ttexto, _ := prompt, []redact.Finding(nil)\n"
//
// Sabotaje: el pedido no se trunca.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif r := []rune(texto); len(r) > maxRunasDePedido {\n"
// arnes: a="\tif r := []rune(texto); len(r) > maxRunasDePedido*100 {\n"
//
// Sabotaje: el pedido se trunca antes de pasar por el redactor.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t\ttexto = string(r[:maxRunasDePedido])\n"
// arnes: a="\t\ttexto, _ = redact.Redact(string([]rune(prompt)[:maxRunasDePedido]))\n"
//
// Sabotaje: sin tope de pedidos.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif len(ps) > maxPedidos {\n"
// arnes: a="\tif len(ps) > maxPedidos*10 {\n"
//
// Sabotaje: el pedido repetido se vuelve a guardar.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif n := len(ps); n > 0 && ps[n-1].Texto == texto {\n"
// arnes: a="\tif n := len(ps); n > 0 && ps[n-1].Texto == texto && false {\n"
func TestElPedidoSustantivoSeRecuerdaPorSesion(t *testing.T) {
	store := &storeQueCuenta{fakeTurnStore: &fakeTurnStore{meta: map[string]string{}, recall: memory.RecallResult{
		Count: 1, Items: []memory.RecallItem{{ID: "x1", TopicKey: "t", Gist: "memoria", ContentHash: "h1"}},
	}}, escrituras: map[string]int{}}
	turno := func(sesion, prompt string) {
		in, _ := json.Marshal(map[string]string{"session_id": sesion, "prompt": prompt})
		turnOutput(store, deltaLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(string(in)))
	}
	ultimo := func(sesion string) string {
		ps := leerPedidos(store, sesion)
		if len(ps) == 0 {
			return ""
		}
		return ps[len(ps)-1].Texto
	}

	turno("A", "armá el banco de tipeo")
	turno("B", "revisá el TLS del cerebro")
	turno("A", "sigue")
	turno("A", avisoDeMonitor)
	if got := ultimo("A"); got != "armá el banco de tipeo" {
		t.Errorf("el último pedido de A tenía que seguir siendo el sustantivo (ni «sigue» ni el aviso lo pisan), es %q", got)
	}
	if got := ultimo("B"); got != "revisá el TLS del cerebro" {
		t.Errorf("B tiene que ver su propio pedido, no el de A: es %q", got)
	}

	// El mismo pedido otra vez no escribe: recordarPedido es una escritura más por turno sobre una
	// base con _txlock=immediate que comparten varios daemons, y la que no cambia nada se ahorra.
	turno("A", "armá el banco de tipeo")
	if n := store.escrituras[pedidosKey("A")]; n != 1 {
		t.Errorf("A pidió dos veces lo mismo y sus pedidos se escribieron %d veces, no una", n)
	}
	if ps := leerPedidos(store, "A"); len(ps) != 1 {
		t.Errorf("el pedido repetido se guardó dos veces: %+v", ps)
	}

	// El secreto no llega a la meta (queda «…»: ver TestLaMarcaDelRedactorNoEntraALaConsulta), y el
	// largo queda acotado.
	secreto := "ghp_" + strings.Repeat("a1B2c3D4e5", 4)
	turno("A", "hacé el push con el token "+secreto+" al mirror")
	if got := ultimo("A"); strings.Contains(got, secreto) || !strings.Contains(got, "…") {
		t.Errorf("el pedido se guardó sin redactar: %q", got)
	}
	largo := "migrá la tabla " + strings.Repeat("ñandú ", 100)
	turno("A", largo)
	if got := []rune(ultimo("A")); len(got) != maxRunasDePedido {
		t.Errorf("el pedido tenía que quedar truncado a %d runas, quedó con %d", maxRunasDePedido, len(got))
	}

	// Primero el redactor y después el truncado. Al revés, un secreto que cruza la runa
	// maxRunasDePedido le llega cortado al redactor, que ya no lo reconoce (la regla de ghp_ pide 20
	// caracteres, y el catch-all de entropía otros tantos), y el trozo queda en claro. Es sintético.
	// La cola va larga a propósito: con el secreto ya tapado el pedido tiene que seguir pasándose del
	// tope, o el truncado —y su sabotaje— no corre.
	cruza := "ghp_" + strings.Repeat("Zq9Xw2Lm7Kp4", 3)
	pedido := strings.Repeat("revisá el mirror ", 11) + "con " + cruza + " y seguí con el certificado del tailnet"
	if desde := len([]rune(pedido[:strings.Index(pedido, cruza)])); desde >= maxRunasDePedido || desde+len(cruza) <= maxRunasDePedido {
		t.Fatalf("CONTROL: el secreto tiene que cruzar la runa %d, y va de la %d a la %d", maxRunasDePedido, desde, desde+len(cruza))
	}
	if n := len([]rune(pedido)) - len(cruza) + len([]rune("…")); n <= maxRunasDePedido {
		t.Fatalf("CONTROL: con el secreto tapado el pedido tiene que seguir pasándose de %d runas, y queda en %d", maxRunasDePedido, n)
	}
	turno("A", pedido)
	if got := ultimo("A"); strings.Contains(got, "ghp_") {
		t.Errorf("quedó en claro un trozo del secreto que cruzaba el corte: %q", got)
	}

	turno("A", "medí la latencia del hook")
	ps := leerPedidos(store, "A")
	if len(ps) != maxPedidos {
		t.Fatalf("A guardó %d pedidos, el tope es %d: %+v", len(ps), maxPedidos, ps)
	}
	if ps[len(ps)-1].Texto != "medí la latencia del hook" || ps[len(ps)-1].T == 0 {
		t.Errorf("el último pedido de A tiene que ser el último que pidió, con su hora: %+v", ps[len(ps)-1])
	}
}

// storeQueCuenta cuenta las escrituras en la meta, clave por clave.
type storeQueCuenta struct {
	*fakeTurnStore
	escrituras map[string]int
}

func (s *storeQueCuenta) SetMeta(key, value string) error {
	s.escrituras[key]++
	return s.fakeTurnStore.SetMeta(key, value)
}

// TestElPedidoSinSesionNoSeGuarda: sin session_id no hay a qué compactación devolverle el pedido, y
// guardarlo lo dejaría en una clave que comparten todos los turnos sin sesión («loop_pedidos:»),
// anotada en el índice del delta como una sesión más.
//
// Sabotaje: recordarPedido guarda aunque no haya sesión.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif sessionID == \"\" {\n\t\treturn // sin sesión no hay a qué compactación devolvérselo\n"
// arnes: a="\tif sessionID == \"\" && false {\n\t\treturn // sin sesión no hay a qué compactación devolvérselo\n"
func TestElPedidoSinSesionNoSeGuarda(t *testing.T) {
	store := &fakeTurnStore{meta: map[string]string{}}
	recordarPedido(store, "", "revisá el TLS del cerebro", time.Unix(1_800_000_000, 0))
	if len(store.meta) != 0 {
		t.Errorf("sin sesión, recordarPedido escribió en la meta: %v", store.meta)
	}
}

// TestLaMarcaDelRedactorNoEntraALaConsulta: el pedido guardado es la consulta con la que
// feat/memoria-tras-compactar vuelve a buscar memoria, así que cada secreto queda como «…», que no
// tiene términos, y no como «[REDACTED:<tipo>]», que le sumaría «REDACTED» y las palabras del tipo.
// Los términos del pedido guardado tienen que ser los del prompt sin el secreto. Es sintético.
//
// Sabotaje: el pedido se guarda con la marca del redactor adentro.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\ttexto = marcaDelRedactor.ReplaceAllLiteralString(texto, \"…\")\n"
// arnes: a="\ttexto = marcaDelRedactor.ReplaceAllString(texto, \"$0\")\n"
func TestLaMarcaDelRedactorNoEntraALaConsulta(t *testing.T) {
	store := &fakeTurnStore{meta: map[string]string{}}
	secreto := "ghp_" + strings.Repeat("a1B2c3D4e5", 4)
	recordarPedido(store, "S", "rotá el token "+secreto+" del mirror", time.Unix(1_800_000_000, 0))
	ps := leerPedidos(store, "S")
	if len(ps) != 1 || strings.Contains(ps[0].Texto, secreto) {
		t.Fatalf("el pedido tenía que guardarse, y sin el secreto: %+v", ps)
	}
	got, want := memory.TerminosDeConsulta(ps[0].Texto), memory.TerminosDeConsulta("rotá el token del mirror")
	if !slices.Equal(got, want) {
		t.Errorf("el pedido guardado %q le da a la consulta los términos %q; tenían que ser los del prompt sin el secreto, %q", ps[0].Texto, got, want)
	}
}

// TestLosPedidosSePodanConElDelta: los pedidos de una sesión se vacían cuando la sesión sale del
// índice del delta, también la que nunca escribió su delta (el recall no trajo nada), y desalojar
// una sesión que nunca guardó un pedido no escribe nada.
//
// Sabotaje: el desalojo no vacía los pedidos.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t\t\tolvidarPedidos(store, p.id)\n"
// arnes: a="\t\t\t_ = olvidarPedidos\n"
//
// Sabotaje: recordarPedido no anota la sesión en el índice.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif !sesionEnElIndice(store, sessionID) {\n"
// arnes: a="\tif false && !sesionEnElIndice(store, sessionID) {\n"
//
// Sabotaje: el desalojo escribe un vacío aunque la sesión no tuviera pedidos.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif raw, ok, _ := store.GetMeta(pedidosKey(sessionID)); ok && raw != \"\" {\n"
// arnes: a="\tif raw, ok, _ := store.GetMeta(pedidosKey(sessionID)); ok || raw == \"\" {\n"
func TestLosPedidosSePodanConElDelta(t *testing.T) {
	store := &fakeTurnStore{meta: map[string]string{}} // el recall no trae nada: no hay delta
	ahora := time.Unix(1_800_000_000, 0)

	recordarPedido(store, "interactiva", "revisá el TLS del cerebro", ahora)
	if len(leerPedidos(store, "interactiva")) != 1 {
		t.Fatal("el pedido no se guardó")
	}
	// Llegan maxDeltaSessions+5 sesiones más nuevas, que sólo siembran su delta (hijas de workflow):
	// la interactiva sale primero, y detrás cinco hijas que nunca guardaron un pedido.
	for i := 0; i < maxDeltaSessions+5; i++ {
		saveDeltaStateEn(store, fmt.Sprintf("hija-%02d", i), ahora.Add(time.Duration(i+1)*time.Second))
	}
	if ps := leerPedidos(store, "interactiva"); len(ps) != 0 {
		t.Errorf("la sesión salió del índice y sus pedidos siguen ahí: %+v", ps)
	}
	for k := range store.meta {
		if strings.HasPrefix(k, metaPedidos+sepDeltaKey+"hija-") {
			t.Errorf("desalojar una sesión sin pedidos escribió %q: una fila por nada", k)
		}
	}
}

// saveDeltaStateEn es saveDeltaState con el reloj puesto, para ordenar el desalojo.
func saveDeltaStateEn(store metaStore, sesion string, en time.Time) {
	_ = store.SetMeta(deltaKey(sesion), `{"x":"h"}`)
	registrarSesionDelta(store, sesion, en.Unix())
}

// storeIntercalado corre UNA vez otra operación justo después de que alguien lee el índice del
// delta: es lo que pasa cuando dos hooks de sesiones distintas corren a la vez, porque el
// leer-modificar-escribir de registrarSesionDelta no tiene candado.
type storeIntercalado struct {
	*fakeTurnStore
	alLeerElIndice func()
}

func (s *storeIntercalado) GetMeta(key string) (string, bool, error) {
	v, ok, err := s.fakeTurnStore.GetMeta(key)
	if f := s.alLeerElIndice; key == metaDeltaSessions && f != nil {
		s.alLeerElIndice = nil
		f()
	}
	return v, ok, err
}

// TestElPedidoRepetidoVuelveAlIndice: una carrera entre dos hooks saca a una sesión del índice con
// su pedido adentro, y si la sesión repite el pedido, vuelve a anotarse, así que la poda la alcanza.
// Antes, el pedido repetido salía antes de mirar el índice y el pedido quedaba sin poda para siempre.
//
// Sabotaje: el pedido repetido sale antes de mirar el índice.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tps := leerPedidos(store, sessionID)\n"
// arnes: a="\tps := leerPedidos(store, sessionID)\n\tif len(ps) > 0 && ps[len(ps)-1].Texto == texto {\n\t\treturn\n\t}\n"
func TestElPedidoRepetidoVuelveAlIndice(t *testing.T) {
	store := &fakeTurnStore{meta: map[string]string{}}
	ahora := time.Unix(1_800_000_000, 0)
	const pedido = "revisá el TLS del cerebro"

	// Una hija lee el índice; antes de que lo reescriba, S guarda su pedido y se anota. La hija
	// escribe el índice que había leído, sin S.
	hija := &storeIntercalado{fakeTurnStore: store, alLeerElIndice: func() {
		recordarPedido(store, "S", pedido, ahora)
	}}
	registrarSesionDelta(hija, "hija", ahora.Add(time.Second).Unix())
	if sesionEnElIndice(store, "S") || len(leerPedidos(store, "S")) != 1 {
		t.Fatal("CONTROL: la carrera tenía que dejar a S fuera del índice con su pedido guardado; si no, la prueba no mide nada")
	}

	recordarPedido(store, "S", pedido, ahora.Add(2*time.Second)) // S repite el pedido
	if !sesionEnElIndice(store, "S") {
		t.Fatal("S repitió el pedido y no volvió al índice: su pedido no se podaría nunca")
	}
	for i := 0; i < maxDeltaSessions; i++ {
		saveDeltaStateEn(store, fmt.Sprintf("hija-%02d", i), ahora.Add(time.Duration(i+3)*time.Second))
	}
	if ps := leerPedidos(store, "S"); len(ps) != 0 {
		t.Errorf("S salió del índice y su pedido sigue ahí: %+v", ps)
	}
}
