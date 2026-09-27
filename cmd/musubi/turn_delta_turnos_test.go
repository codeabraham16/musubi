package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// EL DESALOJO SACA PRIMERO A LAS SESIONES SIN TURNOS (ver registrarSesionDelta).
//
// El índice del delta acota a maxDeltaSessions las sesiones que conservan su delta y sus pedidos, y
// desalojaba por LRU a secas. Desde la compuerta del turno, un «sigue» o un aviso del sistema no
// refrescan la marca, así que la de una sesión interactiva que espera un workflow se queda vieja, y
// las 40 hijas que arranca el workflow la sacaban del índice sembrando su delta.

// TestDesalojoPrefiereSesionesSinTurnos: una sesión interactiva cuyos últimos turnos son sólo «sigue»
// y un <task-notification> no sale del índice mientras 40 hijas siembran su delta, y conserva su
// delta y sus pedidos. Va con el motor real y los hooks reales: el turno por turnOutputWith y el
// arranque de cada hija por buildHookOutput. La interactiva se llama «a-…» a propósito: con la misma
// marca en segundos, el LRU desempata por id y la elegiría antes que a cualquier «z-hija-…».
//
// Sabotaje: volver al orden por marca sin mirar el turno.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t\tif idx.conTurno[a] != idx.conTurno[b] {\n"
// arnes: a="\t\tif false && idx.conTurno[a] != idx.conTurno[b] {\n"
//
// Sabotaje: la siembra del priming cuenta como turno.
// arnes: archivo="cmd/musubi/detect.go"
// arnes: de="\tsaveDeltaState(store, sessionID, seed, false)\n"
// arnes: a="\tsaveDeltaState(store, sessionID, seed, true)\n"
func TestDesalojoPrefiereSesionesSinTurnos(t *testing.T) {
	eng := motorConCorpusDeRuido(t)
	const interactiva = "a-interactiva"
	arranque := config.StartupConfig{PrimeMemory: true, RecallBudget: 300}

	if got := turnoReal(t, eng, interactiva, "revisá el TLS del cerebro"); !strings.Contains(got, "[id:tls]") {
		t.Fatalf("CONTROL: el pedido sustantivo tenía que traer la nota del TLS y anotar el delta; trajo:\n%s", got)
	}
	turnoReal(t, eng, interactiva, "sigue")
	turnoReal(t, eng, interactiva, avisoDeMonitor)
	delta, _, _ := eng.GetMeta(deltaKey(interactiva))
	pedidos := leerPedidos(eng, interactiva)
	if delta == "" || len(pedidos) != 1 {
		t.Fatalf("CONTROL: la interactiva tenía que tener su delta y un pedido; delta %q, pedidos %+v", delta, pedidos)
	}

	const hijas = 40
	for i := 0; i < hijas; i++ {
		if _, err := buildHookOutput(t.TempDir(), eng, arranque, fmt.Sprintf("z-hija-%02d", i)); err != nil {
			t.Fatal(err)
		}
	}
	idx := indiceDe(t, eng)
	if ultima, _, _ := eng.GetMeta(deltaKey(fmt.Sprintf("z-hija-%02d", hijas-1))); ultima == "" || len(idx.marca) != maxDeltaSessions {
		t.Fatalf("CONTROL: las hijas tenían que sembrar su delta y llenar el índice (%d sesiones); la última sembró %q y el índice tiene %d", maxDeltaSessions, ultima, len(idx.marca))
	}

	if _, esta := idx.marca[interactiva]; !esta || !idx.conTurno[interactiva] {
		t.Fatalf("la interactiva tuvo un turno y salió del índice (está: %v, con turno: %v) mientras sembraban las hijas", esta, idx.conTurno[interactiva])
	}
	if got, _, _ := eng.GetMeta(deltaKey(interactiva)); got != delta {
		t.Errorf("la interactiva perdió su delta: era %q y quedó %q", delta, got)
	}
	if got := leerPedidos(eng, interactiva); !slices.Equal(got, pedidos) {
		t.Errorf("la interactiva perdió sus pedidos: eran %+v y quedaron %+v", pedidos, got)
	}
}

// TestLaSiembraNoLeBajaElTurno: «turno» es pegajoso. El arranque de una compactación de la misma
// sesión vuelve a sembrar su delta sin turno, y la sesión sigue contando como una que tuvo uno.
//
// Sabotaje: la siembra baja el turno.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif turno && !idx.conTurno[sessionID] {\n\t\tidx.conTurno[sessionID] = true\n"
// arnes: a="\tif idx.conTurno[sessionID] != turno {\n\t\tidx.conTurno[sessionID] = turno\n"
func TestLaSiembraNoLeBajaElTurno(t *testing.T) {
	store := newFakeTurnStore()
	anotarEn(store, "a-interactiva", time.Unix(1000, 0), true)  // un turno con recall
	anotarEn(store, "a-interactiva", time.Unix(1001, 0), false) // su compactación: el priming siembra
	for i := 0; i < 40; i++ {
		saveDeltaStateEn(store, fmt.Sprintf("z-hija-%02d", i), time.Unix(int64(2000+i), 0))
	}
	idx := indiceDe(t, store)
	if _, esta := idx.marca["a-interactiva"]; !esta || !idx.conTurno["a-interactiva"] {
		t.Errorf("la siembra de su compactación le bajó el turno a la interactiva y la desalojaron las hijas (está: %v, con turno: %v)", esta, idx.conTurno["a-interactiva"])
	}
}

// TestUnIndiceDeUnBinarioViejoConvive: los binarios viejos comparten la base hasta que se
// actualizan, y el índice va y viene entre los dos. En las dos direcciones:
//
//   - Lo que escribió un binario viejo ({id: unix}, sin la clave de turnos) se lee acá sin turnos, sin
//     perder ninguna sesión; y la sesión que tiene un pedido sustantivo de acá en más recupera el
//     turno aunque ya esté en el índice, así que la siembra de las hijas no la saca.
//   - Lo que escribe esta rama lo decodifica un binario viejo en su map[string]int64 sin error y sin
//     ceros, y su LRU sigue andando: saca a la más vieja y le vacía los pedidos. (A secas: no sabe de
//     turnos, y hasta que se actualice puede sacar a una interactiva.) La marca de turno que le
//     queda colgada a la que sacó se barre la próxima vez que esta rama escribe la clave.
//
// Sabotaje: una sesión que anotó un binario viejo no recupera el turno con su pedido.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\treturn esta && idx.conTurno[sessionID]\n"
// arnes: a="\treturn esta\n"
// arnes: colision_ok="TestElPedidoRepetidoVuelveAlIndice"
//
// Sabotaje: el turno va adentro del índice, donde lo lee un binario viejo.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="metaDeltaConTurno = \"loop_delta_con_turno\""
// arnes: a="metaDeltaConTurno = metaDeltaSessions"
//
// Sabotaje: la marca de turno de una sesión que ya no está en el índice no se barre.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t\t\tdelete(idx.conTurno, id) // la desalojó un binario viejo, que no conoce esta clave\n"
// arnes: a="\t\t\t_ = id // la desalojó un binario viejo, que no conoce esta clave\n"
func TestUnIndiceDeUnBinarioViejoConvive(t *testing.T) {
	store := newFakeTurnStore()

	// El binario viejo: la interactiva guarda un pedido y 30 hijas siembran su delta.
	recordarPedidoDeMain(store, "a-interactiva", "revisá el TLS del cerebro", 1000)
	for i := 0; i < 30; i++ {
		id := fmt.Sprintf("m-hija-%02d", i)
		_ = store.SetMeta("loop_delta_injected:"+id, `{"x":"h"}`)
		registrarSesionDeltaDeMain(store, id, int64(1001+i))
	}
	if _, hay := store.meta[metaDeltaConTurno]; hay {
		t.Fatal("CONTROL: el binario viejo no escribe la clave de turnos")
	}
	idx := indiceDe(t, store)
	if len(idx.marca) != 31 || len(idx.conTurno) != 0 {
		t.Fatalf("el índice del binario viejo tenía que leerse entero y sin turnos: %d sesiones (eran 31), %d con turno", len(idx.marca), len(idx.conTurno))
	}

	// Esta rama: un pedido sustantivo de la interactiva, cuyo recall no trajo nada, y 40 hijas.
	recordarPedido(store, "a-interactiva", "medí la latencia del hook", time.Unix(1100, 0))
	for i := 0; i < 40; i++ {
		saveDeltaStateEn(store, fmt.Sprintf("z-hija-%02d", i), time.Unix(int64(2000+i), 0))
	}
	idx = indiceDe(t, store)
	if _, esta := idx.marca["a-interactiva"]; !esta || len(leerPedidos(store, "a-interactiva")) != 2 {
		t.Fatalf("la interactiva tuvo un pedido sustantivo y la sacaron las hijas (está: %v, pedidos: %+v)", esta, leerPedidos(store, "a-interactiva"))
	}
	for i := 0; i < 30; i++ {
		if v := store.meta[deltaKey(fmt.Sprintf("m-hija-%02d", i))]; v != "" {
			t.Errorf("m-hija-%02d, de las más viejas sin turno, tenía que salir primero y conserva su delta %q", i, v)
		}
	}
	sinHuerfanasEnElFake(t, store)

	// El binario viejo lee lo que escribió esta rama.
	var viejo map[string]int64
	if err := json.Unmarshal([]byte(store.meta["loop_delta_sessions"]), &viejo); err != nil {
		t.Fatalf("un binario viejo no puede leer el índice que escribe esta rama: %v", err)
	}
	for id, marca := range viejo {
		if marca == 0 {
			t.Errorf("un binario viejo lee la marca de %q como 0: la tomaría por la sesión más vieja de todas", id)
		}
	}
	registrarSesionDeltaDeMain(store, "m-hija-nueva", 3000)
	var despues map[string]int64
	if err := json.Unmarshal([]byte(store.meta["loop_delta_sessions"]), &despues); err != nil || len(despues) != maxDeltaSessions {
		t.Fatalf("el binario viejo tenía que seguir podando a %d sesiones: %d, error %v", maxDeltaSessions, len(despues), err)
	}
	if _, esta := despues["a-interactiva"]; esta || len(leerPedidos(store, "a-interactiva")) != 0 {
		t.Errorf("el LRU del binario viejo tenía que sacar a la más vieja, la interactiva, y vaciarle los pedidos")
	}

	// El binario viejo no conoce la clave de turnos: la marca de la interactiva quedó colgada, y se
	// barre la próxima vez que esta rama escribe la clave.
	if !indiceDe(t, store).conTurno["a-interactiva"] {
		t.Fatal("CONTROL: la marca de turno de la interactiva tenía que seguir en la clave después del binario viejo")
	}
	recordarPedido(store, "b-interactiva", "revisá la bajada del sync", time.Unix(3001, 0))
	if indiceDe(t, store).conTurno["a-interactiva"] {
		t.Error("la marca de turno de una sesión que desalojó un binario viejo no se barrió al reescribir la clave: la clave crece sin tope")
	}
}

// TestUnTurnoSustantivoNoEscribeDeMas: lo que un turno sustantivo escribe en la meta del delta. El
// primero de la sesión escribe una vez la clave de turnos, y los demás no la tocan; el índice se
// escribe una sola vez por turno, porque el recall ya anotó la sesión con turno y recordarPedido no
// la vuelve a anotar. Dos transacciones por turno: la del delta y la del pedido. El pedido repetido
// no escribe ni abre la suya.
//
// Sabotaje: el recall del turno guarda el delta sin turno.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t\tsaveDeltaState(store, sessionID, seen, true)\n"
// arnes: a="\t\tsaveDeltaState(store, sessionID, seen, false)\n"
//
// Sabotaje: la clave de turnos se escribe en cada anotación.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif !cambioElTurno {\n"
// arnes: a="\tif false && !cambioElTurno {\n"
//
// Sabotaje: el pedido repetido abre la transacción aunque no tenga nada que escribir.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif ps := leerPedidos(store, sessionID); len(ps) > 0 && ps[len(ps)-1].Texto == texto && sesionConTurno(store, sessionID) {\n"
// arnes: a="\tif ps := leerPedidos(store, sessionID); false && len(ps) > 0 && ps[len(ps)-1].Texto == texto && sesionConTurno(store, sessionID) {\n"
func TestUnTurnoSustantivoNoEscribeDeMas(t *testing.T) {
	store := &storeQueCuenta{fakeTurnStore: &fakeTurnStore{meta: map[string]string{}, recall: memory.RecallResult{
		Count: 1, Items: []memory.RecallItem{{ID: "x1", TopicKey: "t", Gist: "memoria", ContentHash: "h1"}},
	}}, escrituras: map[string]int{}}
	turno := func(prompt string) map[string]int {
		store.escrituras, store.transacciones = map[string]int{}, 0
		in, _ := json.Marshal(map[string]string{"session_id": "A", "prompt": prompt})
		turnOutput(store, deltaLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(string(in)))
		return map[string]int{
			"delta":   store.escrituras[deltaKey("A")],
			"indice":  store.escrituras[metaDeltaSessions],
			"turnos":  store.escrituras[metaDeltaConTurno],
			"pedidos": store.escrituras[pedidosKey("A")],
		}
	}

	primero := turno("armá el banco de tipeo")
	if want := map[string]int{"delta": 1, "indice": 1, "turnos": 1, "pedidos": 1}; !maps.Equal(primero, want) || store.transacciones != 2 {
		t.Errorf("el primer turno sustantivo escribió %v en %d transacciones; tenía que ser %v en 2", primero, store.transacciones, want)
	}
	segundo := turno("medí la latencia del hook")
	if want := map[string]int{"delta": 1, "indice": 1, "turnos": 0, "pedidos": 1}; !maps.Equal(segundo, want) || store.transacciones != 2 {
		t.Errorf("el segundo turno sustantivo escribió %v en %d transacciones; tenía que ser %v en 2", segundo, store.transacciones, want)
	}
	repetido := turno("medí la latencia del hook")
	if want := map[string]int{"delta": 1, "indice": 1, "turnos": 0, "pedidos": 0}; !maps.Equal(repetido, want) || store.transacciones != 1 {
		t.Errorf("el pedido repetido escribió %v en %d transacciones; tenía que ser %v en 1, la del delta", repetido, store.transacciones, want)
	}
}

// DOS ESCRITORES DEL ÍNDICE A LA VEZ, CON EL MOTOR REAL.
//
// Dos hooks de sesiones distintas, cada uno con su engine sobre la misma base, como dos procesos.
// Uno está por escribir el índice que ya leyó, y en ese instante el otro intenta hacer lo suyo. Sin
// transacción, el otro escribe en el medio y el primero lo pisa con lo que había leído: la sesión
// del otro queda fuera del índice, y lo suyo sin poda. Con la transacción, el primero tiene el
// candado de escritura y el otro espera a que termine.
//
// NO PASA POR TIMING. En el instante del medio, una sonda sin espera (busy_timeout 0) intenta tomar
// el candado. Si no puede, el primero lo tiene: el otro corre DESPUÉS de que termine, que es lo que
// haría su busy_timeout. Si puede, nadie lo tiene: el otro corre AHORA, en el medio, que es la
// carrera. Una sola goroutine, sin sleeps.

// TestDosEscritoresDelIndiceNoSePisan: en cada caso, la sesión del otro termina en el índice con lo
// suyo, y ningún delta ni pedido queda fuera del índice.
//
// Sabotaje: saveDeltaState sin transacción.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t_ = store.MetaEnTransaccion(func(tx memory.MetaTx) error {\n\t\tif err := tx.SetMeta(deltaKey(sessionID), string(data)); err != nil {\n\t\t\treturn err\n\t\t}\n\t\treturn registrarSesionDelta(tx, sessionID, time.Now().Unix(), desdeTurno)\n\t})\n"
// arnes: a="\t_ = func(tx memory.MetaTx) error {\n\t\tif err := tx.SetMeta(deltaKey(sessionID), string(data)); err != nil {\n\t\t\treturn err\n\t\t}\n\t\treturn registrarSesionDelta(tx, sessionID, time.Now().Unix(), desdeTurno)\n\t}(store)\n"
//
// Sabotaje: recordarPedido sin transacción.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\t_ = store.MetaEnTransaccion(func(tx memory.MetaTx) error {\n\t\treturn guardarPedido(tx, sessionID, texto, ahora)\n\t})\n"
// arnes: a="\t_ = guardarPedido(store, sessionID, texto, ahora)\n"
//
// Sabotaje: MetaEnTransaccion no abre la transacción.
// arnes: archivo="internal/memory/meta_tx.go"
// arnes: de="\ttx, err := e.db.Begin()\n"
// arnes: a="\tif true {\n\t\treturn fn(e)\n\t}\n\ttx, err := e.db.Begin()\n"
func TestDosEscritoresDelIndiceNoSePisan(t *testing.T) {
	// La sesión que se anota y la hora de las que ya estaban, para que el desalojo sea determinista.
	sembrar := func(t *testing.T, eng *memory.DbEngine, ids ...string) {
		t.Helper()
		for i, id := range ids {
			if err := eng.MetaEnTransaccion(func(tx memory.MetaTx) error {
				if err := tx.SetMeta(deltaKey(id), `{"x":"h"}`); err != nil {
					return err
				}
				return registrarSesionDelta(tx, id, int64(1000+i), false)
			}); err != nil {
				t.Fatal(err)
			}
		}
	}
	hijas := func(n int) []string {
		ids := make([]string, n)
		for i := range ids {
			ids[i] = fmt.Sprintf("hija-%02d", i)
		}
		return ids
	}

	t.Run("el recall de A no pisa la anotación de B", func(t *testing.T) {
		b := nuevaBaseCompartida(t)
		// B, una siembra, es la más vieja de un índice lleno: la siembra de A la desaloja, y B vuelve
		// a anotarse con turno cuando guarda su pedido.
		sembrar(t, b.uno, append([]string{"B"}, hijas(maxDeltaSessions-1)...)...)
		a := b.escritorConPausa(t, b.uno, func() {
			recordarPedido(b.otro, "B", "revisá el TLS del cerebro", time.Now())
		})
		saveDeltaState(a, "A", map[string]string{"x1": "h1"}, false)
		a.terminar()

		idx := indiceDe(t, b.uno)
		if _, esta := idx.marca["B"]; !esta || !idx.conTurno["B"] || len(leerPedidos(b.uno, "B")) != 1 {
			t.Errorf("B guardó su pedido mientras A escribía el índice y quedó fuera (está: %v, con turno: %v, pedidos: %+v; el otro esperó al candado: %v)",
				esta, idx.conTurno["B"], leerPedidos(b.uno, "B"), a.espero)
		}
		b.sinHuerfanas(t)
	})

	t.Run("el pedido de B no pisa la siembra de C", func(t *testing.T) {
		bb := nuevaBaseCompartida(t)
		// B, una siembra, guarda su primer pedido: se anota con turno, y mientras escribe el índice
		// que leyó, C siembra su delta.
		sembrar(t, bb.uno, append([]string{"B"}, hijas(maxDeltaSessions-2)...)...)
		pedido := bb.escritorConPausa(t, bb.uno, func() {
			saveDeltaState(bb.otro, "C", map[string]string{"x1": "h1"}, false)
		})
		recordarPedido(pedido, "B", "revisá el TLS del cerebro", time.Now())
		pedido.terminar()

		idx := indiceDe(t, bb.uno)
		if _, esta := idx.marca["C"]; !esta {
			t.Errorf("C sembró su delta mientras B escribía el índice y quedó fuera, con su delta sin poda (el otro esperó al candado: %v)", pedido.espero)
		}
		if _, esta := idx.marca["B"]; !esta || !idx.conTurno["B"] {
			t.Errorf("B tenía que quedar en el índice con turno")
		}
		bb.sinHuerfanas(t)
	})
}

// baseCompartida son dos engines sobre la misma base —dos hooks, dos procesos— y una sonda sin
// espera para ver si alguien tiene el candado de escritura.
type baseCompartida struct {
	uno, otro *memory.DbEngine
	sonda     *sql.DB
}

func nuevaBaseCompartida(t *testing.T) *baseCompartida {
	t.Helper()
	dir := memtest.DirSembrado(t)
	b := &baseCompartida{}
	for _, eng := range []**memory.DbEngine{&b.uno, &b.otro} {
		e, err := memory.NewDbEngine(dir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { e.Close() })
		*eng = e
	}
	sonda, err := sql.Open("sqlite", filepath.Join(dir, config.DirName, config.DBFile)+"?_pragma=busy_timeout(0)")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { sonda.Close() })
	b.sonda = sonda
	return b
}

// candadoTomado dice si alguien tiene el candado de escritura: la sonda intenta tomarlo sin esperar
// y, si puede, lo suelta. Cualquier otro error corta la prueba: no es la respuesta que se busca.
func (b *baseCompartida) candadoTomado(t *testing.T) bool {
	t.Helper()
	ctx := context.Background()
	conn, err := b.sonda.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		if s := strings.ToLower(err.Error()); !strings.Contains(s, "database is locked") && !strings.Contains(s, "sqlite_busy") {
			t.Fatalf("la sonda falló por otra cosa que el candado: %v", err)
		}
		return true
	}
	if _, err := conn.ExecContext(ctx, `ROLLBACK`); err != nil {
		t.Fatal(err)
	}
	return false
}

// sinHuerfanas: todo delta o pedido que no esté vacío es de una sesión que está en el índice. Si no,
// no lo poda nadie.
func (b *baseCompartida) sinHuerfanas(t *testing.T) {
	t.Helper()
	idx := indiceDe(t, b.uno)
	filas, err := b.sonda.Query(`SELECT key FROM meta WHERE (key LIKE 'loop_pedidos:%' OR key LIKE 'loop_delta_injected:%') AND value != ''`)
	if err != nil {
		t.Fatal(err)
	}
	defer filas.Close()
	for filas.Next() {
		var k string
		if err := filas.Scan(&k); err != nil {
			t.Fatal(err)
		}
		if id := k[strings.Index(k, sepDeltaKey)+1:]; idx.marca[id] == 0 {
			t.Errorf("%q no está vacío y su sesión no está en el índice: no lo poda nadie", k)
		}
	}
	if err := filas.Err(); err != nil {
		t.Fatal(err)
	}
}

// escritorConPausa es un hook que, justo antes de escribir el índice del delta —ya leído—, deja
// correr la operación del otro: en el medio si nadie tiene el candado, o después si lo tiene él.
type escritorConPausa struct {
	*memory.DbEngine
	b         *baseCompartida
	t         *testing.T
	otro      func()
	pendiente func()
	espero    bool // el otro tuvo que esperar al candado
}

func (b *baseCompartida) escritorConPausa(t *testing.T, eng *memory.DbEngine, otro func()) *escritorConPausa {
	return &escritorConPausa{DbEngine: eng, b: b, t: t, otro: otro}
}

func (s *escritorConPausa) antesDeEscribir(key string) {
	if key != metaDeltaSessions || s.otro == nil {
		return
	}
	otro := s.otro
	s.otro = nil
	if s.b.candadoTomado(s.t) {
		s.espero = true
		s.pendiente = otro
		return
	}
	otro()
}

// terminar corre lo que quedó esperando al candado, y comprueba que el instante del medio llegó.
func (s *escritorConPausa) terminar() {
	s.t.Helper()
	if s.otro != nil {
		s.t.Fatal("CONTROL: nunca se llegó a escribir el índice; la prueba no midió nada")
	}
	if s.pendiente != nil {
		s.pendiente()
	}
}

func (s *escritorConPausa) SetMeta(key, value string) error {
	s.antesDeEscribir(key)
	return s.DbEngine.SetMeta(key, value)
}

func (s *escritorConPausa) MetaEnTransaccion(fn func(memory.MetaTx) error) error {
	return s.DbEngine.MetaEnTransaccion(func(tx memory.MetaTx) error {
		return fn(txConPausa{MetaTx: tx, s: s})
	})
}

// txConPausa es la transacción del escritorConPausa: la misma pausa antes de escribir el índice.
type txConPausa struct {
	memory.MetaTx
	s *escritorConPausa
}

func (x txConPausa) SetMeta(key, value string) error {
	x.s.antesDeEscribir(key)
	return x.MetaTx.SetMeta(key, value)
}

// sinHuerfanasEnElFake es sinHuerfanas sobre un fake.
func sinHuerfanasEnElFake(t *testing.T, store *fakeTurnStore) {
	t.Helper()
	idx := indiceDe(t, store)
	for k, v := range store.meta {
		if v == "" || !(strings.HasPrefix(k, metaPedidos+sepDeltaKey) || strings.HasPrefix(k, metaDeltaInjected+sepDeltaKey)) {
			continue
		}
		if id := k[strings.Index(k, sepDeltaKey)+1:]; idx.marca[id] == 0 {
			t.Errorf("%q no está vacío y su sesión no está en el índice: no lo poda nadie", k)
		}
	}
}

// LO QUE CORRE UN BINARIO VIEJO. Copia congelada de registrarSesionDelta y del pedaso de
// recordarPedido que toca el índice, como estaban en main en 76c6bce9 (cmd/musubi/turn.go:682 y
// :788), con las claves escritas a mano: la prueba es de convivencia con ESE código, y tiene que
// seguir siéndolo aunque el de hoy cambie.
func registrarSesionDeltaDeMain(store metaStore, sessionID string, ahora int64) {
	sesiones := map[string]int64{}
	if raw, ok, _ := store.GetMeta("loop_delta_sessions"); ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &sesiones)
	}
	sesiones[sessionID] = ahora
	if len(sesiones) > 32 {
		type par struct {
			id string
			t  int64
		}
		ps := make([]par, 0, len(sesiones))
		for id, t := range sesiones {
			ps = append(ps, par{id, t})
		}
		sort.Slice(ps, func(i, j int) bool {
			if ps[i].t != ps[j].t {
				return ps[i].t < ps[j].t
			}
			return ps[i].id < ps[j].id
		})
		for _, p := range ps[:len(ps)-32] {
			_ = store.SetMeta("loop_delta_injected:"+p.id, "")
			if raw, ok, _ := store.GetMeta("loop_pedidos:" + p.id); ok && raw != "" {
				_ = store.SetMeta("loop_pedidos:"+p.id, "")
			}
			delete(sesiones, p.id)
		}
	}
	if data, err := json.Marshal(sesiones); err == nil {
		_ = store.SetMeta("loop_delta_sessions", string(data))
	}
}

// recordarPedidoDeMain es recordarPedido de main sin el redactor ni el truncado: anota la sesión si
// falta en el índice y agrega el pedido.
func recordarPedidoDeMain(store metaStore, sessionID, texto string, ahora int64) {
	var ps []pedidoDeSesion
	if raw, ok, _ := store.GetMeta("loop_pedidos:" + sessionID); ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &ps)
	}
	var idx map[string]json.RawMessage
	if raw, ok, _ := store.GetMeta("loop_delta_sessions"); ok && raw != "" {
		_ = json.Unmarshal([]byte(raw), &idx)
	}
	if _, esta := idx[sessionID]; !esta {
		registrarSesionDeltaDeMain(store, sessionID, ahora)
	}
	ps = append(ps, pedidoDeSesion{T: ahora, Texto: texto})
	if data, err := json.Marshal(ps); err == nil {
		_ = store.SetMeta("loop_pedidos:"+sessionID, string(data))
	}
}
