package mcp

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// LA LÍNEA «BAJADA» DE musubi_sync_status DICE CUÁNDO BAJÓ POR ÚLTIMA VEZ, Y NO LO INVENTA.
//
// La edad y la próxima salen de la meta que anota el dueño del candado (memory.MetaUltimaBajada);
// el volumen, de sync_viajes. Estas pruebas fijan las dos maneras en que esa línea puede mentir: no
// decir la edad que hay, y decir una que no hay —«nunca» en el central, que no baja; «nunca» al lado
// de páginas bajadas hoy por un binario que no la anota; una meta cortada que parte la línea en dos—.

// centralConFilas contesta musubi_sync_pull con tantas filas como diga `filas` en cada pedido.
func centralConFilas(filas *atomic.Int64) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(filas.Load())
		var items []string
		for i := 1; i <= n; i++ {
			items = append(items, `{"rowid":`+strconv.Itoa(i)+`,"id":"e`+strconv.Itoa(i)+`","topic_key":"t/a","content":"del central `+strconv.Itoa(i)+`","importance":1,"mem_type":"semantic","author":"ana","project_id":"acme"}`)
		}
		payload := `{"items":[` + strings.Join(items, ",") + `],"next_cursor":` + strconv.Itoa(n) + `}`
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"pull","result":{"content":[{"type":"text","text":` + strconv.Quote(payload) + `}]}}`))
	}))
}

// serverConLaBase es serverQueAnotaViajes con una conexión cruda a la MISMA base, para armar lo que
// el engine no deja armar: una fila que la base rechaza, o viajes de un día cualquiera. WAL admite
// el segundo escritor.
func serverConLaBase(t *testing.T, url string, teamMode bool) (*McpServer, *engineQueAnotaViajes, *sql.DB) {
	t.Helper()
	dir := memtest.DirSembrado(t)
	real, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { real.Close() })
	db, err := sql.Open("sqlite", filepath.Join(dir, config.DirName, config.DBFile)+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("abrir la base cruda: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	eng := &engineQueAnotaViajes{StorageBackend: real}
	s := NewMcpServer(eng, t.TempDir(), embedding.NoopProvider{}, WithMemory(config.MemoryConfig{TeamMode: teamMode}))
	s.SetSyncClient(newTestSyncClient(t, url), config.SyncConfig{BatchSize: 50, LeaseSeconds: 60, BackoffBaseSeconds: 5, BackoffMaxSeconds: 300})
	return s, eng, db
}

// textoDeSyncStatus llama a musubi_sync_status sin credencial, como un daemon local.
func textoDeSyncStatus(t *testing.T, s *McpServer) string {
	t.Helper()
	res, e := call(t, s, "musubi_sync_status", map[string]interface{}{})
	if e != nil {
		t.Fatalf("sync_status: %+v", e)
	}
	return res.(CallToolResponse).Content[0].Text
}

// lineaDeBajada devuelve la línea «bajada:» del texto de musubi_sync_status y la que le sigue.
func lineaDeBajada(t *testing.T, txt string) (linea, siguiente string) {
	t.Helper()
	i := strings.Index(txt, "\nbajada: ")
	if i < 0 {
		t.Fatalf("musubi_sync_status no trae la línea «bajada:»:\n%s", txt)
	}
	partes := strings.SplitN(txt[i+1:], "\n", 3)
	linea = partes[0]
	if len(partes) > 1 {
		siguiente = partes[1]
	}
	return linea, siguiente
}

// proximaEnSegundos saca el N de «próxima en ~N s». -1 si la línea no lo trae en segundos.
func proximaEnSegundos(linea string) int {
	const marca = "próxima en ~"
	i := strings.Index(linea, marca)
	if i < 0 || !strings.HasSuffix(linea, " s") {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimSuffix(linea[i+len(marca):], " s"))
	if err != nil {
		return -1
	}
	return n
}

// edadEnSegundos saca el N de « · última hace N s». -1 si la línea no lo trae en segundos.
func edadEnSegundos(linea string) int {
	const marca = " · última hace "
	i := strings.Index(linea, marca)
	if i < 0 {
		return -1
	}
	numero, resto, ok := strings.Cut(linea[i+len(marca):], " ")
	if !ok || !strings.HasPrefix(resto, "s ") {
		return -1
	}
	n, err := strconv.Atoi(numero)
	if err != nil {
		return -1
	}
	return n
}

// TestSyncStatusDiceLaEdadDeLaBajada: la línea «bajada» dice cuándo bajó por última vez, si trajo
// algo y cuándo se espera la próxima —un tick, 30 s sin config—. Antes del primer Pull, «nunca»;
// después de una página sin filas, «vacía»; después de una con dos, «2 filas». Y la edad es la del
// Pull que acaba de volver: «hace 0 s», con hasta 2 s para el commit del tick y el redondeo a
// segundos. Lo lee de la meta que anota el dueño del candado, no de la memoria del proceso que
// contesta.
//
// Sabotaje: que la meta anote un instante viejo y no el del tick que volvió.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="u := memory.UltimaBajada{Unix: ahora.Unix(), Filas:"
// arnes: a="u := memory.UltimaBajada{Unix: ahora.Unix() - 7200, Filas:"
//
// Sabotaje: que el defer de drainInboundOnce no anote la bajada.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="\ts.anotarUltimaBajada("
// arnes: a="\t_ = ("
//
// Sabotaje: que la transacción de la bajada sume el viaje y no escriba la meta.
// arnes: archivo="internal/memory/sync_viajes.go"
// arnes: de="MetaUltimaBajada, u.Valor()); err != nil {"
// arnes: a="MetaUltimaBajada+\"-otra\", u.Valor()); err != nil {"
//
// Sabotaje: que la meta anote cero filas aunque el tick haya bajado dos.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="Filas: v.Filas, ProximaUnix:"
// arnes: a="Filas: 0, ProximaUnix:"
//
// Sabotaje: que la próxima sea el mismo instante de la última y no un tick después.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="ProximaUnix: ahora.Add(s.tickBajada()).Unix()"
// arnes: a="ProximaUnix: ahora.Unix()"
func TestSyncStatusDiceLaEdadDeLaBajada(t *testing.T) {
	var filas atomic.Int64
	central := centralConFilas(&filas)
	defer central.Close()
	s, _ := serverQueAnotaViajes(t, central.URL, true)

	if l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s)); !strings.HasSuffix(l, " · última: nunca") {
		t.Errorf("antes del primer Pull la línea tenía que terminar en «· última: nunca»:\n%s", l)
	}

	s.drainInboundOnce(context.Background()) // una página sin filas
	l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
	if !strings.Contains(l, " · última hace ") || !strings.Contains(l, " (vacía), próxima en ~") {
		t.Errorf("después de una bajada vacía la línea tenía que decir «última hace … (vacía), próxima en ~…»:\n%s", l)
	}
	// La próxima es la última más un tick; entre el Pull y esta lectura pasan fracciones de segundo,
	// y la cota de abajo deja lugar a una máquina cargada.
	if n := proximaEnSegundos(l); n < 20 || n > 30 {
		t.Errorf("la próxima tenía que caer a ~30 s (un tick sin config), vino %d:\n%s", n, l)
	}
	if n := edadEnSegundos(l); n < 0 || n > 2 {
		t.Errorf("recién volvió el Pull: la edad tenía que ser «hace 0 s» (hasta 2 s), vino %d:\n%s", n, l)
	}
	t.Logf("después de una bajada vacía: %s", l)

	filas.Store(2)
	s.drainInboundOnce(context.Background()) // una página con dos filas
	l, _ = lineaDeBajada(t, textoDeSyncStatus(t, s))
	if !strings.Contains(l, " · última hace ") || !strings.Contains(l, " (2 filas), próxima en ~") {
		t.Errorf("después de bajar dos filas la línea tenía que decir «última hace … (2 filas), próxima en ~…»:\n%s", l)
	}
	if n := edadEnSegundos(l); n < 0 || n > 2 {
		t.Errorf("recién volvió el Pull con dos filas: la edad tenía que ser «hace 0 s» (hasta 2 s), vino %d:\n%s", n, l)
	}
	t.Logf("después de una bajada con dos filas: %s", l)
}

// TestUnaPaginaQueNoEntraNoEsVacia: una página que trae filas y no puede ingerir ninguna —una fila
// que esta base rechaza; drainInboundOnce no avanza el cursor y la misma fila vuelve primera en cada
// tick, así que no es un tick: es cada tick— no se anota «(vacía)». sync_viajes la cuenta como
// página con filas (páginas sin vacías), y la edad dice lo mismo: «(vacía)» al lado de «0 vacías»
// se contradice, y con la edad fresca se lee «no hay nada nuevo» con la bajada atascada.
//
// Sabotaje: que la meta no anote las páginas con filas del tick.
// arnes: archivo="internal/mcp/scheduler.go"
// arnes: de="ConFilas: v.Posts - v.Vacias"
// arnes: a="ConFilas: 0"
//
// Sabotaje: que la línea no mire las páginas con filas y vuelva a decir «vacía».
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="\tcase u.ConFilas == 1:"
// arnes: a="\tcase false && u.ConFilas == 1:"
func TestUnaPaginaQueNoEntraNoEsVacia(t *testing.T) {
	var filas atomic.Int64
	filas.Store(1)
	central := centralConFilas(&filas)
	defer central.Close()
	s, _, db := serverConLaBase(t, central.URL, true)
	// e1 no entra nunca: la base la rechaza como rechazaría un dato que viola una restricción.
	if _, err := db.Exec(`CREATE TRIGGER veneno BEFORE INSERT ON observations WHEN NEW.id = 'e1'
		BEGIN SELECT RAISE(ABORT, 'fila veneno a propósito'); END`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		s.drainInboundOnce(context.Background())
	}
	l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
	// El control: el volumen cuenta tres páginas con filas y ninguna fila. Sin esto la prueba podría
	// pasar sobre un escenario que no es el que dice.
	if !strings.Contains(l, "bajada: hoy 0 filas en 3 páginas (0 vacías, ") {
		t.Fatalf("el escenario no es el de la prueba: tenían que ser tres páginas con filas y ninguna ingerida:\n%s", l)
	}
	if strings.Contains(l, "(vacía)") || !strings.Contains(l, " · última hace ") || !strings.Contains(l, " (1 página con filas y 0 ingeridas), ") {
		t.Errorf("tres ticks con una fila que no entra: la edad tenía que decir «(1 página con filas y 0 ingeridas)» y no «(vacía)»:\n%s", l)
	}
	t.Logf("una fila que no entra: %s", l)
}

// TestLaBajadaEnElCentralNoDiceNunca: musubi_sync_status también lo sirve el central, que no baja
// nunca. «Última: nunca» sería cierto del proceso que contesta y se leería «tu máquina nunca bajó»:
// la línea dice que ESE proceso no baja y dónde se mira. Lo mismo en un proyecto sin team_mode, cuyo
// scheduler de bajada no arranca.
//
// Sabotaje: que un proceso sin cliente de sync diga «nunca» como uno que baja.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="\t\tcase noBaja != \"\":"
// arnes: a="\t\tcase false && noBaja != \"\":"
//
// Sabotaje: que un proyecto sin team_mode cuente como uno que baja.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="\tcase !s.memory.TeamMode:"
// arnes: a="\tcase false && !s.memory.TeamMode:"
func TestLaBajadaEnElCentralNoDiceNunca(t *testing.T) {
	t.Run("el central, sin cliente de sync", func(t *testing.T) {
		engine := memtest.NuevoEngine(t, t.TempDir())
		engine.SetProjectID("")
		central := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
		l, _ := lineaDeBajada(t, textoDeSyncStatusComo(t, central, &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"}))
		if !strings.Contains(l, " · este proceso no baja (no tiene cliente de sync): ") || strings.Contains(l, "nunca") {
			t.Errorf("el central tenía que decir que no baja, sin «nunca»:\n%s", l)
		}
		t.Logf("servido por el central: %s", l)
	})

	t.Run("proyecto sin team_mode", func(t *testing.T) {
		s, _ := serverQueAnotaViajes(t, "http://127.0.0.1:1", false)
		l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
		if !strings.Contains(l, " · este proceso no baja (el proyecto no está en team_mode)") || strings.Contains(l, "nunca") {
			t.Errorf("un proyecto sin team_mode tenía que decir que no baja, sin «nunca»:\n%s", l)
		}
	})
}

// TestLaEdadDeLaBajadaNoContradiceAlVolumen: la edad sale de la meta y el volumen de sync_viajes, y
// un binario anterior a esta versión escribe el segundo sin la primera. La línea no dice «nunca» al
// lado de páginas bajadas, ni «última hace 3 d» con bajadas más nuevas: dice que bajó un binario que
// no anota la edad. Se compara contra el ÚLTIMO día con bajadas, sin ventana, y el mismo día contra
// las páginas que la meta anotó: las cubetas de hoy y de la semana dejaban afuera los días del medio.
//
// Sabotaje: que sin meta diga «nunca» aunque haya viajes de bajada.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="\t\tcase r.UltimoDiaConBajada != \"\":"
// arnes: a="\t\tcase false && r.UltimoDiaConBajada != \"\":"
//
// Sabotaje: que no compare el día de la meta con el último día con bajadas.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="return r.UltimoDiaConBajada > dia"
// arnes: a="return false"
//
// Sabotaje: que el mismo día no compare las páginas con las que la meta anotó.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="despues := r.PaginasDeEseDia - u.PaginasDelDia"
// arnes: a="despues := int64(0)"
//
// Sabotaje: que el mismo día con las mismas páginas —el caso sano— se lea como un binario anterior.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="return despues > 0"
// arnes: a="return despues >= 0"
func TestLaEdadDeLaBajadaNoContradiceAlVolumen(t *testing.T) {
	const dia = int64(24 * 3600)
	ahora := time.Now().Unix()

	t.Run("viajes de hoy y ninguna meta", func(t *testing.T) {
		s, eng := serverQueAnotaViajes(t, "http://127.0.0.1:1", true)
		// Lo que deja un binario anterior: el viaje sin la edad.
		if err := eng.StorageBackend.RegistrarViaje(memory.ViajeBajada, memory.Viaje{Posts: 3, Vacias: 3, BytesVacias: 400}); err != nil {
			t.Fatal(err)
		}
		l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
		if !strings.Contains(l, " · última: sin anotar (bajó un binario anterior") || strings.Contains(l, "nunca") {
			t.Errorf("con páginas bajadas hoy y sin meta, la línea no podía decir «nunca»:\n%s", l)
		}
		t.Logf("viajes sin meta: %s", l)
	})

	t.Run("viajes de hace diez días, fuera de la semana, y ninguna meta", func(t *testing.T) {
		s, eng, db := serverConLaBase(t, "http://127.0.0.1:1", true)
		hace10 := memory.UltimaBajada{Unix: ahora - 10*dia, ProximaUnix: ahora - 10*dia + 30}
		if err := eng.StorageBackend.RegistrarBajada(memory.Viaje{Posts: 2, Vacias: 2}, hace10); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`DELETE FROM meta WHERE key = ?`, memory.MetaUltimaBajada); err != nil {
			t.Fatal(err)
		}
		l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
		if !strings.Contains(l, " · última: sin anotar (bajó un binario anterior") || strings.Contains(l, "nunca") {
			t.Errorf("con páginas bajadas hace diez días y sin meta, la línea no podía decir «nunca»:\n%s", l)
		}
	})

	t.Run("meta de hace tres días y viajes de hoy", func(t *testing.T) {
		s, eng := serverQueAnotaViajes(t, "http://127.0.0.1:1", true)
		vieja := memory.UltimaBajada{Unix: ahora - 3*dia, ProximaUnix: ahora - 3*dia + 30}
		if err := eng.SetMeta(memory.MetaUltimaBajada, vieja.Valor()); err != nil {
			t.Fatal(err)
		}
		if err := eng.StorageBackend.RegistrarViaje(memory.ViajeBajada, memory.Viaje{Posts: 5, Vacias: 5}); err != nil {
			t.Fatal(err)
		}
		l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
		if !strings.Contains(l, " · última anotada hace 3 d (vacía), pero después bajó un binario anterior") || strings.Contains(l, "próxima") {
			t.Errorf("con bajadas de hoy y la meta de hace tres días, la línea tenía que decir que bajó un binario que no la anota:\n%s", l)
		}
	})

	// anotadaYDespues deja la meta de un tick de hace metaHace días y, además, las páginas de otro
	// de hace viajesHace días que NO la anotó: registra los dos y devuelve la meta a la del primero,
	// que es lo que queda cuando el segundo es de un binario anterior. El segundo trae MENOS páginas
	// que las que anotó el primero, para que un día posterior no se detecte por las páginas.
	anotadaYDespues := func(t *testing.T, eng *engineQueAnotaViajes, metaHace, viajesHace int64) {
		t.Helper()
		anotada := memory.UltimaBajada{Unix: ahora - metaHace*dia, ProximaUnix: ahora - metaHace*dia + 30}
		if err := eng.StorageBackend.RegistrarBajada(memory.Viaje{Posts: 5, Vacias: 5}, anotada); err != nil {
			t.Fatal(err)
		}
		meta, _, err := eng.GetMeta(memory.MetaUltimaBajada)
		if err != nil {
			t.Fatal(err)
		}
		sinAnotar := memory.UltimaBajada{Unix: ahora - viajesHace*dia, ProximaUnix: ahora - viajesHace*dia + 30}
		if err := eng.StorageBackend.RegistrarBajada(memory.Viaje{Posts: 2, Vacias: 2}, sinAnotar); err != nil {
			t.Fatal(err)
		}
		if err := eng.SetMeta(memory.MetaUltimaBajada, meta); err != nil {
			t.Fatal(err)
		}
	}
	for _, c := range []struct {
		nombre               string
		metaHace, viajesHace int64
	}{
		{"meta de hace tres días y viajes de ayer, ninguno hoy", 3, 1},
		{"meta de hace nueve días y viajes de la semana", 9, 3},
		{"meta en el borde de la semana y viajes del día siguiente", 6, 5},
		{"meta y viajes fuera de la semana", 10, 8},
		{"el mismo día de la meta, más páginas que las que anotó", 2, 2},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			s, eng := serverQueAnotaViajes(t, "http://127.0.0.1:1", true)
			anotadaYDespues(t, eng, c.metaHace, c.viajesHace)
			l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
			quiero := " · última anotada hace " + strconv.FormatInt(c.metaHace, 10) + " d (vacía), pero después bajó un binario anterior"
			if !strings.Contains(l, quiero) {
				t.Errorf("la meta es de hace %d d y hay páginas de hace %d d que no anotó: la línea tenía que decir %q:\n%s", c.metaHace, c.viajesHace, quiero, l)
			}
		})
	}

	t.Run("el mismo día con las páginas que anotó: el caso sano", func(t *testing.T) {
		s, eng := serverQueAnotaViajes(t, "http://127.0.0.1:1", true)
		u := memory.UltimaBajada{Unix: ahora - 2*dia, ProximaUnix: ahora - 2*dia + 30}
		for i := 0; i < 2; i++ { // dos ticks del mismo día, los dos de un binario que anota
			if err := eng.StorageBackend.RegistrarBajada(memory.Viaje{Posts: 1, Vacias: 1}, u); err != nil {
				t.Fatal(err)
			}
		}
		l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
		if !strings.Contains(l, " · última hace 2 d (vacía), ") || strings.Contains(l, "binario anterior") {
			t.Errorf("dos ticks del mismo día que anotaron la edad no son un binario anterior:\n%s", l)
		}
	})
}

// TestLaUltimaBajadaIlegibleNoRompeLaLinea: una meta sin la forma unix|filas|proxima —un apagón deja
// escrituras cortadas— o con valores imposibles se dice ilegible, y va entre comillas y escapada: un
// salto de línea adentro partiría la línea «bajada» en dos.
//
// Sabotaje: mostrar el valor crudo, sin escapar.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="strconv.Quote(recortarConMarca(raw, 40))"
// arnes: a="recortarConMarca(raw, 40)"
//
// Sabotaje: aceptar una próxima anterior a la última.
// arnes: archivo="internal/memory/bajada_lease.go"
// arnes: de="u.Unix <= 0 || u.Filas < 0 || u.ProximaUnix < u.Unix"
// arnes: a="u.Unix <= 0 || u.Filas < 0"
func TestLaUltimaBajadaIlegibleNoRompeLaLinea(t *testing.T) {
	for _, c := range []struct{ nombre, valor string }{
		{"cortada con un salto de línea", "1758900000|dos\nfilas"},
		{"la próxima antes que la última", "1758900000|2|1758800000|1|1"},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			s, eng := serverQueAnotaViajes(t, "http://127.0.0.1:1", true)
			if err := eng.SetMeta(memory.MetaUltimaBajada, c.valor); err != nil {
				t.Fatal(err)
			}
			l, sigue := lineaDeBajada(t, textoDeSyncStatus(t, s))
			if !strings.Contains(l, " · última: ilegible (\"") {
				t.Errorf("la meta %q tenía que leerse ilegible, entre comillas:\n%s", c.valor, l)
			}
			if !strings.HasPrefix(sigue, "no viajan: ") {
				t.Errorf("la línea «bajada» se partió: la que le sigue es %q y tenía que ser «no viajan:»", sigue)
			}
			t.Logf("meta %q: %s", c.valor, l)
		})
	}
}

// TestLaProximaVencidaSeDiceComoTal: si la próxima prevista ya pasó hace más de un tick, nadie anotó
// una bajada desde entonces —nadie baja, sus Pull fallan, o uno tarda mucho más que el anterior— y
// la línea lo dice, en vez de prometer «próxima en ~-570 s».
//
// Sabotaje: tratar una próxima vencida como futura.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="\tcase falta >= 0:"
// arnes: a="\tcase true:"
func TestLaProximaVencidaSeDiceComoTal(t *testing.T) {
	s, eng := serverQueAnotaViajes(t, "http://127.0.0.1:1", true)
	ahora := time.Now().Unix()
	u := memory.UltimaBajada{Unix: ahora - 600, Filas: 1, ProximaUnix: ahora - 570}
	if err := eng.StorageBackend.RegistrarBajada(memory.Viaje{Filas: 1, Posts: 1}, u); err != nil {
		t.Fatal(err)
	}
	l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
	if !strings.HasSuffix(l, " · última hace 10 min (1 fila), la próxima se esperaba hace 9 min") {
		t.Errorf("con la próxima vencida hace 9 min la línea tenía que decirlo:\n%s", l)
	}
	t.Logf("próxima vencida: %s", l)
}

// TestLaProximaDentroDeUnTickNoEsUnaAlarma: la próxima sale del FIN del Pull anterior y la meta
// nueva se escribe al FIN del siguiente, así que vence un rato con el dueño sano cada vez que un
// Pull tarda más que el anterior o el candado cambia de dueño, que tickea en otra fase. Dentro de un
// tick la línea no dice «se esperaba», que se lee como alarma: dice que la próxima es ahora. Pasado
// el tick, sí lo dice. El margen es el tick de la bajada y no una constante: con 60 s, 45 s de
// atraso todavía no alarman.
//
// Sabotaje: que no haya margen y una próxima vencida hace unos segundos ya sea una alarma.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="\tcase -falta <= int64(tick/time.Second):"
// arnes: a="\tcase false && -falta <= int64(tick/time.Second):"
func TestLaProximaDentroDeUnTickNoEsUnaAlarma(t *testing.T) {
	ahora := time.Now().Unix()
	for _, c := range []struct {
		nombre      string
		vencidaHace int64
		alarma      bool
	}{
		{"vencida hace 10 s, dentro del tick", 10, false},
		{"vencida hace 45 s, dentro de un tick de 60 s", 45, false},
		{"vencida hace 90 s, pasado el tick", 90, true},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			s, eng := serverQueAnotaViajes(t, "http://127.0.0.1:1", true)
			s.intervaloBajada.Store(int64(60 * time.Second))
			u := memory.UltimaBajada{Unix: ahora - c.vencidaHace - 60, ProximaUnix: ahora - c.vencidaHace}
			if err := eng.SetMeta(memory.MetaUltimaBajada, u.Valor()); err != nil {
				t.Fatal(err)
			}
			l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
			if alarma := strings.Contains(l, "la próxima se esperaba"); alarma != c.alarma || (!c.alarma && !strings.HasSuffix(l, ", próxima: ahora")) {
				t.Errorf("con la próxima vencida hace %d s y un tick de 60 s, ¿alarma? tenía que ser %v:\n%s", c.vencidaHace, c.alarma, l)
			}
		})
	}
}
