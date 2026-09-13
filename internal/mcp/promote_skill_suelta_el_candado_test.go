package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"musubi/internal/config"
)

// centralQueCuelga es un central falso que NO contesta hasta que se lo suelta.
//
// No se reusa `centralFalso` (skillfed_test.go): ése responde al toque, que es exactamente lo
// contrario de lo que hay que medir acá. Avisa por `entro` cuando el POST llegó, y libera cuando se
// cierra `soltar`.
type centralQueCuelga struct {
	entro  chan struct{}
	soltar chan struct{}
	// texto es el contenido que devuelve UNA VEZ SOLTADO. Vacío significa "ok", que es todo lo que
	// `PushSkill` necesita para dar por buena la promoción.
	//
	// install_skill NO se conforma con eso: su `FetchSkill` pasa por `ListArsenal`, que parsea este
	// mismo texto como un ARRAY JSON de skillPayload. Con "ok" ahí, la aserción final de esa prueba
	// caería por «respuesta del arsenal ilegible» — roja, sí, pero por el motivo equivocado, que es
	// el tropiezo que esta prueba ya documenta más abajo.
	texto  string
	unaVez sync.Once
	unSolo sync.Once
}

// liberar destraba el handler, y TIENE que poder llamarse dos veces.
//
// Se llama al final del camino feliz y también desde un `t.Cleanup`, porque el camino que importa
// es el otro: cuando la prueba muere en un `t.Fatalf`, el `close` de más abajo NO se ejecuta, la
// goroutine del handler queda esperando para siempre, y `httptest.Server.Close()` —registrado en su
// propio cleanup— ESPERA a los handlers en vuelo. Con el sabotaje puesto eso colgaba el binario
// entero hasta el timeout de `go test`, que no imprime `--- FAIL:`: el arnés no veía un rojo, veía
// una corrida sin veredicto, y encima tardaba veinte minutos en decirlo.
func (c *centralQueCuelga) liberar() { c.unSolo.Do(func() { close(c.soltar) }) }

func (c *centralQueCuelga) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.unaVez.Do(func() { close(c.entro) })
	<-c.soltar
	// Se lee DESPUÉS del <-soltar, pero `texto` se fija antes de arrancar el servidor y no se toca
	// más: no hay carrera que proteger.
	texto := c.texto
	if texto == "" {
		texto = "ok"
	}
	w.Header().Set("Content-Type", "application/json")
	// Se serializa con el encoder y no con una cadena a mano: el arsenal que manda install lleva
	// comillas adentro, y concatenarlo produciría un JSON roto que se leería como «el central
	// contestó mal» en vez de como el error de la prueba.
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": "x",
		"result": map[string]any{"content": []map[string]string{{"type": "text", "text": texto}}}})
}

// TestPromoverUnaSkillNoCongelaElServidor mide el EFECTO de `lockSelf` en musubi_promote_skill: que
// el candado del despacho no cruce el POST al central.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ ESTA TOOL Y POR QUÉ PRIMERO
//
// Medido el 2026-09-13, siete tools sostenían el candado del despacho mientras esperaban la red.
// `musubi_promote_skill` es la más simple de convertir: NO toca la base —lee las skills del disco
// con el resolver y empuja por HTTP— así que su conversión es sólo declarar `lockSelf`, sin ninguna
// sección crítica que acotar con withReadLock/withWriteLock. Por eso estrena el molde que las otras
// seis van a copiar.
//
// Y sostenía el candado EXCLUSIVO, porque no declara `readOnly`: cuando la clase es la de por
// default, el despacho deriva el candado de ese campo —RLock si está, Lock si no— en methods.go.
//
// CORRECCIÓN DEL 2026-09-13, Y QUEDA ESCRITA COMO CORRECCIÓN PORQUE LA FRASE ANTERIOR SE MERGEÓ.
// Acá decía que promote era «la ÚNICA que no declara readOnly» y que «las otras seis toman el
// compartido». Es exactamente al revés, y lo dice el censo del registro: de las siete del trinquete
// SEIS no declaran `readOnly` —promote, install_skill, codegraph_index, fleet_exec, fleet_probe y
// fleet_shell— y la única que sí es `musubi_list_skills`. O sea que seis de siete tenían el servidor
// entero serializado mientras esperaban la red, no una sola.
//
// El error no fue de medición sino de no medir: la frase se heredó de un análisis anterior y se
// repitió. Un número derivado no se cita, se vuelve a contar.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ HACE FALTA ESTA PRUEBA SI YA ESTÁ LA ESTRUCTURAL
//
// La guarda del candado (candado_no_cruza_la_red_test.go) mira la FORMA: que la tool declare
// `lockSelf`. Una tool puede declararlo y meter igual la llamada de red ADENTRO de un
// `withWriteLock`, que es el defecto con la marca puesta. Esto mide el EFECTO: se cuelga el central
// de verdad y se exige que otro escritor entre igual.
//
// SE ESPERA A ESTAR ADENTRO DEL POST antes de sondear. Sin eso la sonda podría correr antes de que
// promote alcance la red, y la prueba pasaría con el defecto puesto — un verde que mide el momento
// equivocado.
//
// LA SONDA ES ESCRITORA a propósito: una de sólo lectura conviviría con un RLock y pasaría igual.
// El escritor es el único que se bloquea con CUALQUIER candado tomado.
//
// Sabotaje que la hace fallar: sacarle `lock: lockSelf` a `musubi_promote_skill` en el registro.
//
// EL ANCLA ARRANCA EN EL COMENTARIO Y NO EN EL `lock:`, por la misma razón que la de `musubi_recall`:
// `lock: lockSelf,` aparece varias veces en el registro y el arnés exige que el literal sea ÚNICO.
// La línea de arriba es la que lo ata a ESTA tool.
//
// La primera versión de esta directiva ancló en `handler: …\n\t\t\tlock:    lockSelf,` —las dos
// líneas contiguas y alineadas— y quedó rota en el acto, porque el comentario que escribí las separó
// y `gofmt` dejó un solo espacio al no ser campos contiguos. El literal no existía en el archivo.
// No lo vi al escribirlo: lo dijo `-validar` DESPUÉS de trackear el archivo, porque el censo deriva
// de `git ls-files` y hasta entonces esta directiva no se estaba contando.
// arnes: archivo="internal/mcp/registry.go"
// arnes: de="\t\t\t// hasta el timeout de sync. Lo mide TestPromoverUnaSkillNoCongelaElServidor.\n\t\t\tlock: lockSelf,"
// arnes: a="\t\t\t// hasta el timeout de sync. Lo mide TestPromoverUnaSkillNoCongelaElServidor."
// arnes: prueba="TestPromoverUnaSkillNoCongelaElServidor"
func TestPromoverUnaSkillNoCongelaElServidor(t *testing.T) {
	central := &centralQueCuelga{entro: make(chan struct{}), soltar: make(chan struct{})}
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	ts := httptest.NewServer(central)
	t.Cleanup(ts.Close)
	// EL ORDEN IMPORTA Y ES AL REVÉS: los cleanups corren en orden INVERSO al que se registran, así
	// que éste —registrado después— se ejecuta ANTES de `ts.Close`. Es lo que garantiza que el
	// handler esté suelto cuando el servidor intente cerrar, incluso si la prueba murió en un
	// `t.Fatalf` sin llegar al `liberar()` del final.
	t.Cleanup(central.liberar)

	// EL TIMEOUT DEL CLIENTE TIENE QUE SER MAYOR QUE LA PACIENCIA DE LA SONDA, y no es un detalle:
	// es lo único que hace que esta prueba mida el CANDADO y no el timeout.
	//
	// La primera versión usaba `newTestSyncClient`, que lo fija en 2 s. Con el sabotaje puesto, el
	// POST moría a los 2 s, promote soltaba el candado, y el escritor entraba — a los 2,015 s,
	// medido. La sonda, que espera hasta 30 s, NUNCA declaraba el bloqueo: la prueba igual se ponía
	// roja, pero por la aserción de abajo («al soltar el central, el promote falló»), o sea por otro
	// motivo. Un rojo por la aserción equivocada se lee igual que el correcto y no cubre lo mismo:
	// el día que un timeout dejara de devolver error, esta prueba quedaba VERDE con el defecto puesto.
	//
	// Con el candado bien puesto nada de esto se espera: el escritor entra en ~1 ms y el central se
	// suelta enseguida, así que el timeout largo no alarga la corrida sana.
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
	escribirSkill(t, root, "revisar-go.yaml", skillLocal)

	promovido := make(chan *RpcError, 1)
	go func() {
		// llamarSinT y no call: esto corre en otra goroutine, y `t.Fatal` desde ahí es un error
		// propio — la prueba seguiría corriendo con el resultado a medias.
		promovido <- llamarSinT(s, "musubi_promote_skill", map[string]interface{}{"name": "revisar-go"})
	}()

	select {
	case <-central.entro:
	case <-time.After(esperaArranque):
		t.Fatal("musubi_promote_skill no llegó al central: esta prueba no pudo empezar a medir, " +
			"así que su verde no significaría nada")
	}

	exigeQueUnEscritorSinEmbedderResponda(t, s,
		"musubi_promote_skill está colgada en el POST al central")

	// SOLTAR ES LO ÚNICO QUE TERMINA EL POST, ahora que el cliente espera 120 s. Antes el timeout
	// corto lo terminaba solo, y eso era justamente el defecto de esta prueba: una prueba que
	// depende de un timeout para destrabarse mide el timeout, no el candado.
	//
	// Va por `liberar()` y no por un `close` directo porque el cleanup hace lo mismo: cerrar dos
	// veces el canal entra en pánico, y el `sync.Once` es lo que vuelve idempotente al camino feliz.
	central.liberar()
	select {
	case rpcErr := <-promovido:
		if rpcErr != nil {
			t.Fatalf("al soltar el central, el promote falló: %+v", rpcErr)
		}
	case <-time.After(esperaMax):
		t.Fatal("soltado el central, el promote igual no volvió: el candado quedó tomado por otra cosa")
	}
}
