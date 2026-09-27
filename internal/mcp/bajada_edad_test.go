package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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

// TestSyncStatusDiceLaEdadDeLaBajada: la línea «bajada» dice cuándo bajó por última vez, si trajo
// algo y cuándo se espera la próxima —un tick, 30 s sin config—. Antes del primer Pull, «nunca»;
// después de una página sin filas, «vacía»; después de una con dos, «2 filas». Lo lee de la meta
// que anota el dueño del candado, no de la memoria del proceso que contesta.
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
	t.Logf("después de una bajada vacía: %s", l)

	filas.Store(2)
	s.drainInboundOnce(context.Background()) // una página con dos filas
	l, _ = lineaDeBajada(t, textoDeSyncStatus(t, s))
	if !strings.Contains(l, " · última hace ") || !strings.Contains(l, " (2 filas), próxima en ~") {
		t.Errorf("después de bajar dos filas la línea tenía que decir «última hace … (2 filas), próxima en ~…»:\n%s", l)
	}
	t.Logf("después de una bajada con dos filas: %s", l)
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
// lado de las páginas de hoy, ni «última hace 3 d» con bajadas más nuevas: dice que bajó un binario
// que no anota la edad.
//
// Sabotaje: que sin meta diga «nunca» aunque haya viajes de bajada.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="\t\tcase r.Bajada7d.Posts > 0:"
// arnes: a="\t\tcase false && r.Bajada7d.Posts > 0:"
//
// Sabotaje: que no compare el día de la meta con el de los viajes de hoy.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="if r.BajadaHoy.Posts > 0 && dia < hoy.Format(time.DateOnly) {"
// arnes: a="if false && r.BajadaHoy.Posts > 0 && dia < hoy.Format(time.DateOnly) {"
//
// Sabotaje: que no compare el día de la meta con el de los viajes de la semana.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="return r.Bajada7d.Posts > 0 && dia < hoy.AddDate(0, 0, -6).Format(time.DateOnly)"
// arnes: a="return false && r.Bajada7d.Posts > 0 && dia < hoy.AddDate(0, 0, -6).Format(time.DateOnly)"
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

	t.Run("meta de hace nueve días y viajes de la semana", func(t *testing.T) {
		s, eng := serverQueAnotaViajes(t, "http://127.0.0.1:1", true)
		// RegistrarBajada deja el viaje en el día de su instante: hace tres días.
		hace3 := memory.UltimaBajada{Unix: ahora - 3*dia, ProximaUnix: ahora - 3*dia + 30}
		if err := eng.StorageBackend.RegistrarBajada(memory.Viaje{Posts: 2, Vacias: 2}, hace3); err != nil {
			t.Fatal(err)
		}
		vieja := memory.UltimaBajada{Unix: ahora - 9*dia, ProximaUnix: ahora - 9*dia + 30}
		if err := eng.SetMeta(memory.MetaUltimaBajada, vieja.Valor()); err != nil {
			t.Fatal(err)
		}
		l, _ := lineaDeBajada(t, textoDeSyncStatus(t, s))
		if !strings.Contains(l, " · última anotada hace 9 d (vacía), pero después bajó un binario anterior") {
			t.Errorf("con bajadas de la semana y la meta de hace nueve días, la línea tenía que decir que bajó un binario que no la anota:\n%s", l)
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
		{"la próxima antes que la última", "1758900000|2|1758800000"},
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

// TestLaProximaVencidaSeDiceComoTal: si la próxima prevista ya pasó, nadie anotó una bajada desde
// entonces —no hay un proceso bajando, o sus Pull fallan— y la línea lo dice, en vez de prometer
// «próxima en ~-570 s».
//
// Sabotaje: tratar una próxima vencida como futura.
// arnes: archivo="internal/mcp/sync_viajes.go"
// arnes: de="\tif falta < 0 {"
// arnes: a="\tif false && falta < 0 {"
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
