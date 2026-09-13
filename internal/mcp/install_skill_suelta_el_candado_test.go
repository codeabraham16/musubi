package mcp

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"musubi/internal/config"
)

// TestInstalarUnaSkillNoCongelaElServidor mide el EFECTO de `lockSelf` en musubi_install_skill: que
// el candado del despacho no cruce el viaje al arsenal del central.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA SEGUNDA DE LAS SIETE, Y POR QUÉ NO ALCANZABA CON COPIAR A promote
//
// `musubi_promote_skill` se convirtió declarando la clase y nada más: lee del disco y empuja por
// HTTP, así que no tiene ninguna sección crítica que acotar. Ésta SÍ TOCA LA BASE — `writeSkillFile`
// estampa el fingerprint del stack con `SetMeta`—, así que declarar `lockSelf` a secas la habría
// dejado escribiendo SIN NINGÚN candado: un arreglo que cambia un defecto de concurrencia por otro
// peor. El handler acota su propio tramo con `withWriteLock`.
//
// Y ADENTRO DE ESE TRAMO VIAJA TAMBIÉN EL `os.Stat`. Hasta ahora el chequeo de «ya existe» y la
// escritura estaban separados y eso no era un defecto: el despacho sostenía el candado EXCLUSIVO
// sobre el handler entero, así que nadie se colaba en el medio. Sacar la red del candado destapa esa
// carrera —es la condición donde se comprueba y no donde se escribe—, así que las dos operaciones
// tienen que quedar en la misma sección crítica. El arreglo del cuelgue no puede pagar con una
// carrera nueva.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LO QUE SOSTENÍA ERA EL EXCLUSIVO
//
// `musubi_install_skill` no declara `readOnly`, y cuando la clase es la de por default el despacho
// deriva el candado de ese campo: RLock si está, Lock si no (methods.go). O sea que durante todo el
// `FetchSkill` —un POST al central, hasta el timeout de sync— el servidor entero quedaba serializado,
// no sólo sus escritores.
//
// Medido sobre el registro el 2026-09-13, de las SIETE tools del trinquete son SEIS las que no
// declaran `readOnly`; la única que sí es `musubi_list_skills`. La prueba de promote llegó a afirmar
// lo contrario y quedó mergeada: ver la corrección escrita en su propio comentario.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ HACE FALTA ESTO SI YA ESTÁ LA GUARDA ESTRUCTURAL
//
// candado_no_cruza_la_red_test.go mira la FORMA: que la tool declare `lockSelf` y que ninguna
// primitiva de salida quede alcanzable bajo el candado del despacho. Una tool puede declarar la
// clase y meter igual la llamada de red ADENTRO de su propio `withWriteLock` — el defecto con la
// marca puesta. Esto mide el EFECTO: se cuelga el central de verdad y se exige que otro escritor
// entre igual.
//
// EL CENTRAL COLGADO TIENE QUE DEVOLVER UN ARSENAL DE VERDAD, y ésa es la única diferencia mecánica
// con promote. `FetchSkill` no hace su propio viaje: llama a `ListArsenal`, que parsea la respuesta
// como un ARRAY JSON de skillPayload y exige después un match de nombre EXACTO. Con el `"ok"` que le
// alcanza a `PushSkill`, la aserción final caería por «respuesta del arsenal ilegible» — roja, sí,
// pero por el motivo equivocado, que es el tropiezo que la prueba de promote ya documenta.
//
// SE ESPERA A ESTAR ADENTRO DEL POST antes de sondear: sin eso la sonda podría correr antes de que
// install alcance la red, y la prueba pasaría con el defecto puesto.
//
// LA SONDA ES ESCRITORA a propósito. Una de sólo lectura conviviría con un RLock y pasaría igual; el
// escritor es el único que se bloquea con CUALQUIER candado tomado. Y `musubi_save_fact` no escribe
// skills, así que no contiende con el `withWriteLock` de este handler: lo que mide es el candado del
// despacho, que es lo que se está sacando del camino.
//
// Sabotaje que la hace fallar: sacarle `lock: lockSelf` a `musubi_install_skill` en el registro, con
// lo que vuelve a `lockFromReadOnly` y —al no ser readOnly— al candado exclusivo sobre todo el
// handler, red incluida.
//
// EL ANCLA ARRANCA EN EL COMENTARIO Y NO EN EL `lock:`, porque `lock: lockSelf,` NO ES ÚNICO en el
// registro —hay varias entradas con esa misma línea— y el arnés exige que el literal lo sea. Acá
// decía «aparece nueve veces» y estaba mal: cuando se escribió eran ocho, y ya son más. Un conteo a
// mano de algo que crece con cada conversión se pudre solo, y lo que decide el ancla no es el número
// sino que haya más de una. La línea de arriba es la que lo ata a ESTA tool,
// y se eligió DESPUÉS de imprimir los bytes con los tabs a la vista: la directiva de promote nació
// rota justamente por anclar en dos campos que gofmt dejó de alinear al meterles un comentario en el
// medio.
// arnes: archivo="internal/mcp/registry.go"
// arnes: de="\t\t\t// existencia junto con la escritura. Lo mide TestInstalarUnaSkillNoCongelaElServidor.\n\t\t\tlock: lockSelf,"
// arnes: a="\t\t\t// existencia junto con la escritura. Lo mide TestInstalarUnaSkillNoCongelaElServidor."
// arnes: prueba="TestInstalarUnaSkillNoCongelaElServidor"
func TestInstalarUnaSkillNoCongelaElServidor(t *testing.T) {
	// El arsenal se serializa con el mismo payload que ya usan las pruebas de la federación, que es
	// lo que garantiza que atraviese validateSkillStructural Y el gate de calidad. Inventar una
	// skill acá sería fabricar el fixture y arriesgar un rojo por la puerta equivocada.
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
	// que esto mida el CANDADO y no el timeout. Con `newTestSyncClient` —que lo fija en 2 s— el POST
	// moriría antes que `esperaMax`, el candado se soltaría solo, y el bloqueo nunca se declararía.
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

	// NO se escribe la skill en el proyecto: si ya existiera, el handler la rechazaría por F5 sin
	// llegar nunca a la red, y esta prueba mediría el vacío.

	instalado := make(chan *RpcError, 1)
	go func() {
		// llamarSinT y no call: esto corre en otra goroutine, y un `t.Fatal` desde ahí es un error
		// propio — la prueba seguiría corriendo con el resultado a medias.
		instalado <- llamarSinT(s, "musubi_install_skill", map[string]interface{}{
			"name": "go-table-driven-tests",
		})
	}()

	select {
	case <-central.entro:
	case <-time.After(esperaArranque):
		t.Fatal("musubi_install_skill no llegó al central: esta prueba no pudo empezar a medir, " +
			"así que su verde no significaría nada")
	}

	exigeQueUnEscritorSinEmbedderResponda(t, s,
		"musubi_install_skill está colgada esperando el arsenal del central")

	// SOLTAR ES LO ÚNICO QUE TERMINA EL VIAJE, ahora que el cliente espera 120 s. Una prueba que
	// depende de un timeout para destrabarse mide el timeout, no el candado.
	central.liberar()
	select {
	case rpcErr := <-instalado:
		if rpcErr != nil {
			t.Fatalf("al soltar el central, el install falló: %+v", rpcErr)
		}
	case <-time.After(esperaMax):
		t.Fatal("soltado el central, el install igual no volvió: el candado quedó tomado por otra cosa")
	}

	// CONTROL: LA SKILL TIENE QUE HABER QUEDADO ESCRITA.
	//
	// Sin esto, la prueba se conformaría con que el escritor entre — y eso lo cumple igual una
	// versión donde el handler falla ANTES de escribir, o donde `withWriteLock` no llega a correr.
	// Estaría midiendo que nadie se bloquea en un camino que no es el que digo medir. Exigir el
	// archivo en disco es lo que ata la medición al camino completo: red afuera, escritura adentro.
	ruta := filepath.Join(root, config.DirName, config.SkillsDir, "go-table-driven-tests.yaml")
	if _, err := os.Stat(ruta); err != nil {
		t.Fatalf("el install respondió sin error pero la skill no quedó escrita en %s: %v — "+
			"la sonda entró en un camino que no llegó a la sección crítica", ruta, err)
	}
}
