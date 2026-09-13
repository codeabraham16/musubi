package mcp

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"musubi/internal/config"
)

// TestListarElArsenalNoCongelaElServidor mide el EFECTO de `lockSelf` en musubi_list_skills: que el
// candado del despacho no cruce el pedido del catálogo al cerebro central.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA TERCERA DE LAS SIETE, Y LA ÚNICA QUE ERA EL CASO COMPARTIDO
//
// De las siete tools que sostenían el candado del despacho durante una llamada de red, ésta es la
// ÚNICA que declara `readOnly` — y `readOnly` es de donde el despacho deriva el candado cuando la
// clase es la de por default: RLock si está, Lock si no (methods.go). O sea que promote e install
// frenaban a todos desde el primer instante con el exclusivo, y ésta tomaba el COMPARTIDO.
//
// NO LA SALVA, Y ÉSE ES EL PUNTO DE ESTA PRUEBA. Un RWMutex de Go no deja pasar lectores nuevos
// cuando ya hay un escritor esperando: basta un save_fact encolado para que el servidor se frene
// igual, sólo que un instante después. Medir «compartido» como si fuera inocuo es el error que esta
// prueba existe para no dejar repetir.
//
// POR ESO LA SONDA ES ESCRITORA, y acá no es una precaución sino la aserción entera: una sonda de
// sólo lectura convive tranquila con un RLock y pasaría VERDE con el defecto puesto. El escritor es
// el único que se bloquea con CUALQUIER candado tomado.
//
// `readOnly` NO SE TOCA en la conversión: gobierna la AUTORIZACIÓN —un principal reader puede llamar
// esta tool— y eso no cambia. `lock` pisa el default sólo para la concurrencia.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA SECCIÓN CRÍTICA ES DE LECTURA, Y SE CONSERVA A PROPÓSITO
//
// `LoadSkills` lee el DISCO, no la base — `os.ReadDir` y `os.ReadFile` sobre .musubi/skills/. Igual
// va adentro de un `withReadLock`: hasta esta conversión el RLock del despacho la excluía de los
// escritores, y `writeSkillFile` —quien escribe esos mismos .yaml, desde install_skill— toma el
// exclusivo. Sacar el candado del despacho sin conservar esa exclusión cambiaría un cuello de
// botella por una lectura sucia.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LO QUE ESTA PRUEBA NO PRUEBA
//
// Que la lista devuelta sea correcta ya lo cubren las pruebas de la federación; acá sólo se exige
// que la llamada NO devuelva error, y eso alcanza como control de que se ejercitó el camino de
// verdad: con source=central, un arsenal que no llega, no parsea o no matchea sale como error. Lo
// que NO cubre es el contenido de la lista — y no se agrega acá para no duplicar una aserción que
// ya vive en su lugar.
//
// Sabotaje que la hace fallar: sacarle `lock: lockSelf` a `musubi_list_skills` en el registro, con
// lo que vuelve a `lockFromReadOnly` y —al declarar `readOnly`— al RLock sobre todo el handler, red
// incluida.
//
// EL ANCLA ARRANCA EN EL COMENTARIO Y NO EN EL `lock:`, porque `lock: lockSelf,` NO ES ÚNICO en el
// registro —hay varias entradas con esa misma línea— y el arnés exige que el literal lo sea. Cuántas
// son exactamente no se escribe acá a propósito: un conteo a mano se pudre en cuanto alguien convierta
// la próxima tool, y lo que decide el ancla no es el número sino que haya más de una.
// Se eligió DESPUÉS de correr gofmt e imprimir los
// bytes con los tabs a la vista: la directiva de promote nació rota por anclar en dos campos que
// gofmt dejó de alinear al meterles un comentario en el medio.
// arnes: archivo="internal/mcp/registry.go"
// arnes: de="\t\t\t// withReadLock. Lo mide TestListarElArsenalNoCongelaElServidor.\n\t\t\tlock: lockSelf,"
// arnes: a="\t\t\t// withReadLock. Lo mide TestListarElArsenalNoCongelaElServidor."
// arnes: prueba="TestListarElArsenalNoCongelaElServidor"
func TestListarElArsenalNoCongelaElServidor(t *testing.T) {
	// El arsenal se serializa con el mismo payload que ya usan las pruebas de la federación: es lo
	// que garantiza que `ListArsenal` lo parsee. Inventarlo acá sería fabricar el fixture.
	arsenal, err := json.Marshal([]skillPayload{skillDelArsenal("go-table-driven-tests")})
	if err != nil {
		t.Fatalf("no se pudo serializar el arsenal falso: %v", err)
	}

	central := &centralQueCuelga{
		entro:  make(chan struct{}),
		soltar: make(chan struct{}),
		texto:  string(arsenal),
	}
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	ts := httptest.NewServer(central)
	t.Cleanup(ts.Close)
	// EL ORDEN IMPORTA Y ES AL REVÉS: los cleanups corren en orden INVERSO al que se registran, así
	// que éste —registrado después— se ejecuta ANTES de `ts.Close`. Sin él, una prueba que muere en
	// un `t.Fatalf` deja el handler esperando para siempre, `httptest.Server.Close()` espera a los
	// handlers en vuelo, y el binario se cuelga hasta el timeout de `go test` — que no imprime
	// `--- FAIL:`, así que el arnés no ve un rojo sino una corrida sin veredicto.
	t.Cleanup(central.liberar)

	// EL TIMEOUT DEL CLIENTE TIENE QUE SER MAYOR QUE LA PACIENCIA DE LA SONDA: es lo único que hace
	// que esto mida el CANDADO y no el timeout. Con `newTestSyncClient` —que lo fija en 2 s— el
	// pedido moriría antes que `esperaMax`, el candado se soltaría solo, y el bloqueo nunca se
	// declararía; la prueba podría igual ponerse roja, pero por otra aserción y sin cubrir esto.
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

	listado := make(chan *RpcError, 1)
	go func() {
		// llamarSinT y no call: esto corre en otra goroutine, y un `t.Fatal` desde ahí es un error
		// propio — la prueba seguiría corriendo con el resultado a medias.
		//
		// source=central y no "all": "all" también funcionaría, pero "central" deja la prueba atada a
		// la única razón por la que esta tool toca la red.
		listado <- llamarSinT(s, "musubi_list_skills", map[string]interface{}{"source": "central"})
	}()

	select {
	case <-central.entro:
	case <-time.After(esperaArranque):
		t.Fatal("musubi_list_skills no llegó al central: esta prueba no pudo empezar a medir, " +
			"así que su verde no significaría nada")
	}

	exigeQueUnEscritorSinEmbedderResponda(t, s,
		"musubi_list_skills está colgada esperando el catálogo del central")

	// SOLTAR ES LO ÚNICO QUE TERMINA EL VIAJE, ahora que el cliente espera 120 s. Una prueba que
	// depende de un timeout para destrabarse mide el timeout, no el candado.
	central.liberar()
	select {
	case rpcErr := <-listado:
		if rpcErr != nil {
			t.Fatalf("al soltar el central, el listado falló: %+v — con source=central eso significa "+
				"que el arsenal no llegó, no parseó o no matcheó, así que la sonda de arriba midió "+
				"un camino que no es el que esta prueba dice medir", rpcErr)
		}
	case <-time.After(esperaMax):
		t.Fatal("soltado el central, el listado igual no volvió: el candado quedó tomado por otra cosa")
	}
}
