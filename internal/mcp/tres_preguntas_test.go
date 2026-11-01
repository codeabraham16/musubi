package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// tres_preguntas_test.go custodia la SEPARACIÓN de lo que antes era un solo booleano
// (`s.forceRedact`) y decidía tres cosas sin relación: redactar el ingest, pisar un scope
// explícito, y si el árbol de código está en este disco. Los tests de cada una de las tres
// siguen viviendo en sus archivos; acá están los que sólo se pueden escribir una vez separadas.

// TestArbolAusenteEnMideSiHayCodigoEnDisco fija el contrato del medidor.
//
// EL NOMBRE ANTES DECÍA «SoloMarcaLaFirmaDelCentral» Y ERA EL TEST EL QUE ESTABA MAL, no el
// nombre: certificaba como contrato que un directorio con exactamente `.musubi` es el central,
// cuando esa firma LA PLANTA EL PROPIO ARRANQUE del daemon en cualquier carpeta nueva
// (`ensureWorkspace` antes de `ListenAndServeHTTP`). O sea que el caso con el que el test abría
// —y que afirmaba `ausente: true`— es la carpeta recién iniciada de cualquier máquina, y el
// `porQue` de su vecino decía lo contrario a dos líneas de distancia. Lo encontró el panel
// adversarial de este PR. El criterio de hoy es «hay archivos indexables», que no se deja voltear
// por un archivo ajeno ni por lo que el arranque fabrica.
//
// El caso que más importa sigue siendo el repo de desarrollo —si el medidor lo diera por ausente
// apagaría la frescura en todas las máquinas, y en silencio—, pero ahora se arma con una fuente
// de verdad: antes tenía un `go.mod` y ningún `.go`, o sea un módulo vacío, y el test pasaba
// midiendo algo que no es «la forma de toda máquina de desarrollo».
//
// Sabotaje: que el medidor nunca llegue a ver el código que encontró. Caen los TRES casos que
// dependen de detectarlo (repo, código suelto, código en subdirectorio) y siguen verdes los que
// no tienen código que detectar, que es la señal de que pegó donde el invariante vive.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="hayCodigo = true"
// arnes: a="hayCodigo = false"
func TestArbolAusenteEnMideSiHayCodigoEnDisco(t *testing.T) {
	escribir := func(t *testing.T, dir, rel, cuerpo string) {
		t.Helper()
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("armar %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte(cuerpo), 0o644); err != nil {
			t.Fatalf("armar %s: %v", rel, err)
		}
	}
	casos := []struct {
		nombre  string
		armar   func(t *testing.T) string
		ausente bool
		porQue  string
	}{
		{
			nombre: "exactamente .musubi y nada mas",
			armar: func(t *testing.T) string {
				dir := t.TempDir()
				if err := os.MkdirAll(filepath.Join(dir, ".musubi"), 0o755); err != nil {
					t.Fatalf("armar .musubi: %v", err)
				}
				return dir
			},
			ausente: true,
			porQue:  "es la forma del cerebro central: su grafo es federado y no hay una sola fuente acá",
		},
		{
			nombre: "repo de desarrollo",
			armar: func(t *testing.T) string {
				dir := t.TempDir()
				if err := os.MkdirAll(filepath.Join(dir, ".musubi"), 0o755); err != nil {
					t.Fatalf("armar .musubi: %v", err)
				}
				escribir(t, dir, "go.mod", "module x\n")
				escribir(t, dir, "main.go", "package main\n")
				return dir
			},
			ausente: false,
			porQue:  "es la forma de toda máquina de desarrollo; darla por ausente apaga la frescura en silencio",
		},
		{
			nombre: "codigo en un subdirectorio",
			armar: func(t *testing.T) string {
				dir := t.TempDir()
				escribir(t, dir, "internal/mcp/server.go", "package mcp\n")
				return dir
			},
			ausente: false,
			porQue:  "el árbol real tiene el código en paquetes, no en la raíz: el medidor RECORRE, no lista",
		},
		{
			nombre: "archivos, pero ninguno indexable",
			armar: func(t *testing.T) string {
				dir := t.TempDir()
				escribir(t, dir, "README.md", "# x\n")
				escribir(t, dir, "datos.csv", "a,b\n")
				return dir
			},
			ausente: true,
			porQue:  "es el borde que este criterio acepta: sin una fuente que el indexador entienda, todo nodo del grafo vino de otra parte",
		},
		{
			nombre: "codigo solo en un directorio que el indexador saltea",
			armar: func(t *testing.T) string {
				dir := t.TempDir()
				escribir(t, dir, "vendor/x/a.go", "package x\n")
				return dir
			},
			ausente: true,
			porQue:  "si el índice no entra ahí, ese código no produce nodos: el medidor tiene que saltear lo mismo que walkSourceTree",
		},
		{
			nombre:  "directorio vacio",
			armar:   func(t *testing.T) string { return t.TempDir() },
			ausente: true,
			porQue:  "sin código no hay árbol que describir; que el código llegue después NO lo cubre el medidor sino remedirArbol, y eso lo custodia TestElArbolQueLlegaDespuesDelArranqueSeMide",
		},
		{
			nombre: "solo codigo, sin .musubi",
			armar: func(t *testing.T) string {
				dir := t.TempDir()
				escribir(t, dir, "main.go", "package main\n")
				return dir
			},
			ausente: false,
			porQue:  "una carpeta suelta con código tiene el árbol en disco aunque no sea un repo ni tenga .musubi",
		},
		{
			nombre:  "ruta que no existe",
			armar:   func(t *testing.T) string { return filepath.Join(t.TempDir(), "no-existe") },
			ausente: false,
			porQue:  "ante un error de lectura el medidor falla hacia PRESENTE, que es la dirección ruidosa",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			dir := c.armar(t)
			if got := arbolAusenteEn(dir); got != c.ausente {
				t.Fatalf("arbolAusenteEn(%q) = %v, se esperaba %v — %s", dir, got, c.ausente, c.porQue)
			}
		})
	}
}

// ─── LOS TRES TESTS DE LA SEPARACIÓN ──────────────────────────────────────────────────────────
//
// El de arriba mide el medidor. Estos tres miden que las preguntas estén DE VERDAD separadas, y
// ninguno de los tres se podía escribir mientras las tres salían del mismo booleano.

// arranqueConRedaccion corre el arranque REAL —el mismo `ListenAndServeHTTP` que ejecuta
// musubi-brain— en loopback, puerto efímero y con `force_redact: true`: la configuración de una
// máquina de una sola persona que decidió redactar igual.
//
// No reusa `arranqueReal` (politicas_arranque_test.go) porque ése no pasa `ForceRedact`, y acá la
// CLAVE DE CONFIG es el sujeto. Escribir `s.pozoCompartido = true` a mano probaría que una
// asignación asigna, no que una configuración alcanzable produce esta combinación.
//
// Devolver nil significa que sirvió hasta que venció el contexto, o sea que pasó por las dos
// asignaciones de http.go. Un error ahí es una configuración que el arranque rechaza, y entonces
// el test no tiene nada que medir: corta.
func arranqueConRedaccion(t *testing.T, s *McpServer) {
	t.Helper()
	arranqueConPolitica(t, s, true)
}

// arranqueConPolitica es el mismo arranque con `force_redact` en el valor que se le pase, y
// existe porque H1 necesita el eje: la regresión que custodia NO TIENE NADA QUE VER CON LA
// REDACCIÓN, y un custodio que sólo corriera con `true` dejaría sin medir la máquina común.
// Lo señaló el panel adversarial de este PR, y con razón: los cinco tests de la separación
// estaban todos construidos alrededor de `force_redact: true`, que es la forma que hizo
// invisible el defecto durante toda la fase de diseño.
func arranqueConPolitica(t *testing.T, s *McpServer, redactar bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	if err := s.ListenAndServeHTTP(ctx, config.ServiceConfig{
		Addr: "127.0.0.1:0", ForceRedact: redactar, RequestTimeoutSeconds: 10,
	}); err != nil {
		t.Fatalf("el arranque rechazó loopback + force_redact=%v, que es una configuración "+
			"soportada: %v", redactar, err)
	}
}

// proyectoEnDisco deja un árbol de código de verdad, que es la forma de cualquier máquina de
// desarrollo. Sin `.musubi`: la base de los tests vive aparte (ver newTestServerWithPath), y el
// caso «repo CON .musubi dentro» ya lo cubre el test del medidor.
func proyectoEnDisco(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/proj\n")
	writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\nfunc Alpha() { beta() }\n\nfunc beta() {}\n")
	return dir
}

// ★ S1 — LAS TRES PREGUNTAS PUEDEN DIVERGIR, Y LA CONFIGURACIÓN QUE LAS SEPARA ES ALCANZABLE.
//
// Este es el test que antes no se podía escribir. Con un solo `s.forceRedact`, pedirle al cerebro
// que redacte el ingest le apagaba la medición de frescura del grafo en el mismo movimiento: no
// había forma de expresar «redactá todo, pero el código SÍ está en este disco». Y esa combinación
// no es de laboratorio — es la máquina de una persona que puso `force_redact: true` a conciencia.
//
// Lo que hace que el test valga es que NO escribe los campos. Arranca el server de verdad, con la
// clave de config de verdad, sobre un árbol de verdad. Un test que hiciera
// `s.pozoCompartido = true; s.arbolAusente = false` mediría que dos variables distintas pueden
// valer distinto, que es cierto por construcción y no dice nada del producto.
//
// Sabotaje: volver a atar el árbol a la política en la asignación, que es exactamente el defecto
// que este cambio corrige. La divergencia desaparece y el test cae.
//
// COMPARTE LÍNEA CON S2 A PROPÓSITO, y el `colision_ok` lo declara: la asignación del arranque es
// UNA, y son dos propiedades distintas de ella. Acá el `a` la ata a la POLÍTICA (lo que este
// cambio vino a romper); en S2 la deja CONSTANTE (que el hecho no salga del disco). Un sabotaje
// no sustituye al otro, Y NO ES UNA SOLA GUARDA CONTADA DOS VECES — medido, no razonado:
// `Store(false)` deja a ESTE test en VERDE y hace caer sólo a S2, así que S2 ve un defecto que
// éste no ve. Al revés NO es simétrico: `Store(s.pozoCompartido)` tumba a los tres tests que
// quieren el árbol presente (S1, S2 y S3), porque volver a atar el hecho a la política es una
// regresión más ancha que dejarlo constante. De las dos, la de S2 es la fina.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="s.arbolAusente.Store(arbolAusenteEn(s.projectPath))"
// arnes: a="s.arbolAusente.Store(s.pozoCompartido)"
// arnes: colision_ok="TestElArbolSaleDelDiscoNoDeLaPolitica"
func TestLasTresPreguntasPuedenDivergir(t *testing.T) {
	s := newTestServerWithPath(t, proyectoEnDisco(t))
	arranqueConRedaccion(t, s)

	if !s.redactaTodoIngest() {
		t.Error("force_redact: true en la config y redactaTodoIngest() = false: la clave dejó de encender la redacción")
	}
	if !s.pisaElScopeExplicito() {
		t.Error("force_redact: true en la config y pisaElScopeExplicito() = false: el ruteo dejó de seguir a la política")
	}
	if s.arbolFueraDeAlcance() {
		t.Error("hay go.mod y pkg/a.go en el disco del proyecto y arbolFueraDeAlcance() = true: la " +
			"redacción volvió a apagar la medición del grafo, que es justo el defecto que esto arregla")
	}
}

// ★ S2 — EL ÁRBOL SALE DEL DISCO, NO DE LA POLÍTICA.
//
// Dos servers con el MISMO bind y la MISMA clave de config, que difieren sólo en qué hay en el
// disco, tienen que contestar distinto. Es la contracara de S1: ahí la política no arrastra al
// hecho, acá el hecho se mueve solo. Juntos clavan los dos ejes, porque un predicado CONSTANTE
// dejaría a uno de los dos en verde.
//
// Sabotaje: que el hecho deje de salir del disco y quede CONSTANTE en la asignación del
// arranque. Los dos servers pasan a contestar lo mismo y el test cae.
//
// ⚠️ ESTA DIRECTIVA APUNTABA AL `return false` DEL CRITERIO VIEJO, y cuando el criterio cambió
// —de «el directorio contiene exactamente .musubi» a «no hay archivos indexables»— ese texto dejó
// de existir en el archivo. Un `de` que no está no se aplica, así que el sabotaje daba VERDE, y
// ese verde decía «no lo apliqué» mientras se leía como «la guarda cubre». Lo cazó
// `TestLaDeudaDeSabotajesNoCreceYElCorpusNoSePodre`, que para eso existe.
//
// Y EL REEMPLAZO NO ES EL INTERIOR DEL MEDIDOR, aunque fuera lo más corto de escribir:
// `hayCodigo = false` ataca el CRITERIO —que es lo que mide `TestArbolAusenteEnMideSiHayCodigoEnDisco`,
// con su tabla de ocho formas de directorio— y además tumba a S1 y a S3, que quieren el árbol
// presente, así que su rojo no diría nada de lo que ESTE test declara. Lo que este test declara
// es que el hecho se DERIVA del disco de cada server, y eso vive en el cableado del arranque.
//
// Y QUE ESTA GUARDA NO SOBRA está MEDIDO: con `Store(false)` cae SÓLO este test —S1, S3 y la
// tabla del criterio quedan VERDES—, así que sin él esa regresión entra sin poner nada en rojo.
// arnes: archivo="internal/mcp/http.go"
// arnes: de="s.arbolAusente.Store(arbolAusenteEn(s.projectPath))"
// arnes: a="s.arbolAusente.Store(false)"
// arnes: colision_ok="TestLasTresPreguntasPuedenDivergir"
func TestElArbolSaleDelDiscoNoDeLaPolitica(t *testing.T) {
	// La firma del central: el `.musubi` con la base federada y nada más.
	soloBase := t.TempDir()
	if err := os.MkdirAll(filepath.Join(soloBase, ".musubi"), 0o755); err != nil {
		t.Fatalf("armar .musubi: %v", err)
	}
	central := newTestServerWithPath(t, soloBase)
	local := newTestServerWithPath(t, proyectoEnDisco(t))

	arranqueConRedaccion(t, central)
	arranqueConRedaccion(t, local)

	// Que la política sea la MISMA en los dos es parte de la medición: si difirieran también en
	// ella, la diferencia del árbol podría venir de ahí y el test no probaría nada. Se compara en
	// vez de exigirla encendida, y la diferencia importa: así este test cae por el DISCO y nunca
	// por la redacción, que es otra pregunta y tiene sus propios custodios.
	if central.redactaTodoIngest() != local.redactaTodoIngest() {
		t.Fatalf("los dos arrancaron con la misma config y la política salió distinta (central=%v local=%v): "+
			"desde acá, una diferencia en el árbol ya no se puede atribuir al disco",
			central.redactaTodoIngest(), local.redactaTodoIngest())
	}
	if !central.arbolFueraDeAlcance() {
		t.Error("un directorio sin un solo archivo indexable (acá, sólo .musubi) y arbolFueraDeAlcance() = false")
	}
	if local.arbolFueraDeAlcance() {
		t.Error("un proyecto con código en disco y arbolFueraDeAlcance() = true")
	}
}

// ★ S3 — UN LOCAL QUE REDACTA SIGUE MIDIENDO LA FRESCURA.
//
// El caso que el booleano único prohibía, visto desde el consumidor: alguien pone
// `force_redact: true` en su máquina y la frescura del grafo tiene que seguir funcionando. Antes
// no — `graphFreshness` devolvía (0,0,0) y `musubi_map` informaba un grafo impecable:
// `stale: 0, ghosts: 0, missing: 0`, que son las mismas tres cifras que devuelve un grafo que dejó
// de mirar. Esa es la razón por la que el medidor falla hacia PRESENTE: el indicador que uno
// miraría para darse cuenta es justo el que el defecto pone en cero.
//
// Sabotaje: devolverle a `graphFreshness` la política en lugar del hecho, que es lo que miraba
// cuando las dos preguntas eran un campo. Vuelven los tres ceros y el test cae.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="if s.arbolFueraDeAlcance() {\n\t\treturn 0, 0, 0"
// arnes: a="if s.redactaTodoIngest() {\n\t\treturn 0, 0, 0"
func TestUnLocalQueRedactaSigueMidiendoLaFrescura(t *testing.T) {
	dir := proyectoEnDisco(t)
	s := newTestServerWithPath(t, dir)
	mustCall(t, s, "musubi_codegraph_index", map[string]interface{}{})

	// El archivo cambia DESPUÉS de indexar: desde acá hay algo que medir.
	writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\nfunc Alpha() { beta() }\n\nfunc beta() {}\n\nfunc Gamma() {}\n")

	arranqueConRedaccion(t, s)
	// La premisa —«un local que redacta»— se lee en el CAMPO y no en `redactaTodoIngest()`, que es
	// el predicado de otra pregunta: mirarlo ahí hacía que sabotear la redacción tirara este test,
	// y un test que cae por una pregunta ajena no está midiendo la suya. Que el predicado siga al
	// campo lo custodia S1.
	if !s.pozoCompartido {
		t.Fatal("el arranque con force_redact: true no encendió la política: sin eso este test no mide lo que dice medir")
	}

	m := decodeCG(t, mustCall(t, s, "musubi_map", map[string]interface{}{}))
	if stale, _ := m["stale"].(float64); stale < 1 {
		t.Errorf("musubi_map informa stale=%v sobre un archivo cambiado en disco con la redacción "+
			"encendida: la política volvió a apagar la frescura (respuesta completa: %v)", m["stale"], m)
	}
	// Y el mismo hecho por el otro consumidor del predicado, que se pregunta nodo por nodo.
	nodo := memory.GraphNode{Path: "pkg/a.go", SrcFingerprint: "huella-de-antes-del-cambio"}
	if !s.cgStale(nodo) {
		t.Error("cgStale() = false sobre un nodo cuya huella no es la del archivo en disco: con la " +
			"redacción encendida volvió a cortar antes de comparar")
	}
	// 🔴 Y LA OTRA MITAD DEL PAR, QUE FALTABA: un nodo cuya huella SÍ es la del disco tiene que dar
	// false. Sin este control la aserción de arriba era de un solo lado y por lo tanto VACUA, porque
	// `cgStale` FALLA HACIA TRUE —un archivo ausente o ilegible cuenta como stale—: cablearla a
	// `return true` la dejaba verde. Lo encontró el panel adversarial de este PR, midiendo el par en
	// la misma línea: `return false` la pone en rojo, `return true` deja los cinco tests en verde.
	// Con las dos mitades, cualquiera de los dos cables constantes cae.
	huella, err := memory.FileFingerprint(dir, "pkg/a.go")
	if err != nil {
		t.Fatalf("no se pudo derivar la huella del archivo en disco, que es el control de esta medición: %v", err)
	}
	if s.cgStale(memory.GraphNode{Path: "pkg/a.go", SrcFingerprint: huella}) {
		t.Error("cgStale() = true sobre un nodo cuya huella ES la del archivo en disco: el predicado " +
			"dejó de comparar y empezó a contestar siempre lo mismo")
	}
}

// ★ S4 — EL SCOPE FORZADO TIENE SU PROPIO CUSTODIO, Y NO LO TENÍA.
//
// Este test no estaba en el plan: lo pidió la matriz de sabotaje de T11. Sabotear
// `pisaElScopeExplicito()` a `false` dejaba `internal/mcp` ENTERO en verde salvo S1 —medido el
// 2026-10-31: **1 solo `--- FAIL:` en todo el paquete**—, y S1 mira el predicado, no su efecto.
//
// El motivo es DEFENSA EN PROFUNDIDAD, y es la trampa que este repo ya tiene nombrada: el scope
// forzado y la redacción forzada cierran el MISMO hueco por dos caminos distintos. Con las dos
// encendidas, apagar una deja los tests de la otra en verde —el contenido se redacta igual, por el
// otro camino— y ninguno distingue cuál de las dos actuó. Un verde que no distingue no está
// midiendo lo que su nombre dice.
//
// Lo que sí distingue es el SCOPE PERSISTIDO, que la redacción no toca: un cliente que declara
// `scope: "local"` contra un pozo compartido tiene que terminar guardado en `shared`. Se lee por
// SQL crudo con `filaObs` —el estado en la fila, no lo que el engine dice que hizo— y se mide
// contra su control, porque sin pozo compartido ese mismo `local` explícito es un escape hatch que
// el contrato de `internal/config/config.go` promete respetar.
//
// Acá los campos SÍ se escriben a mano, y es la diferencia con S1: S1 custodia que una
// configuración alcanzable produzca este estado; S4 custodia qué hace el estado una vez puesto.
//
// Sabotaje: que el predicado del scope deje de pisar. El `local` explícito sobrevive al ruteo y la
// fila queda en `local` con el pozo compartido.
// arnes: archivo="internal/mcp/server.go"
// arnes: de="func (s *McpServer) pisaElScopeExplicito() bool { return s.pozoCompartido }"
// arnes: a="func (s *McpServer) pisaElScopeExplicito() bool { return false }"
func TestElScopeForzadoSeVeEnLaFilaGuardada(t *testing.T) {
	guardarLocalYLeerScope := func(t *testing.T, pozoCompartido bool) string {
		t.Helper()
		dir := memtest.DirSembrado(t)
		engine, err := memory.NewDbEngine(dir)
		if err != nil {
			t.Fatalf("NewDbEngine: %v", err)
		}
		t.Cleanup(func() { engine.Close() })
		s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
		s.pozoCompartido = pozoCompartido

		const id = "s4-declara-local"
		if _, e := call(t, s, "musubi_save_observation", map[string]interface{}{
			"id": id, "topic_key": "cfg/x", "content": "el cliente declaró local a propósito",
			"scope": "local",
		}); e != nil {
			t.Fatalf("save: %+v", e)
		}
		scope, _, _ := filaObs(t, dir, id)
		return scope
	}

	if got := guardarLocalYLeerScope(t, true); got != "shared" {
		t.Errorf("con el pozo compartido, un scope=local explícito quedó guardado como %q y tenía que "+
			"terminar en \"shared\": un secreto crudo entra al pozo de todos por la puerta del scope", got)
	}
	// Control: sin pozo compartido, el escape hatch se respeta. Sin esta mitad, un predicado que
	// devolviera `true` SIEMPRE pasaría el test de arriba y rompería el contrato del scope
	// explícito sin que nadie se enterara.
	if got := guardarLocalYLeerScope(t, false); got != "local" {
		t.Errorf("sin pozo compartido, un scope=local explícito quedó guardado como %q: el escape "+
			"hatch que promete internal/config/config.go dejó de valer", got)
	}
}

// ★ H1 — EL ÁRBOL QUE LLEGA DESPUÉS DEL ARRANQUE SE MIDE, Y ANTES NO.
//
// El hecho «¿está el árbol de fuentes en ESTE disco?» se medía UNA sola vez, en el arranque, y
// quedaba congelado para toda la vida del daemon. Un daemon que arranca en una carpeta todavía sin
// código —el `musubi init` antes del `git clone`, el agente que abre un directorio nuevo— se
// quedaba con «ausente» para siempre: el código llegaba, se indexaba, y la frescura seguía
// APAGADA EN SILENCIO, porque los tres consumidores del predicado (`cgStale`, `graphFreshness` y
// `pistaDelMiss`) preguntaban por un hecho de hace horas. Lo encontró el panel adversarial de este
// PR midiendo la grilla entera. El defecto es ANTERIOR al cambio de criterio, pero el criterio
// nuevo lo hace más fácil de pisar: «carpeta sin fuentes indexables» es una carpeta de trabajo
// normal, y antes era casi sólo la firma del central.
//
// EL ARREGLO NO ES UN RELOJ, ES EL ÍNDICE. `remedirArbol` corre cuando el grafo vuelve a leer el
// disco, que es exactamente el momento en que el hecho pudo cambiar. Por eso este test es
// DETERMINISTA y no espera a nada: un TTL habría necesitado un sleep, y el sleep habría dejado el
// test flaky en la máquina lenta.
//
// Y NO LLAMA `remedirArbol` DIRECTO, que sería tautológico: pasa por la tool real, en sus DOS
// modos, porque son dos sitios de re-medición distintos —`indexAllPackages` y `planearIncremental`—
// y el incremental es el que usa el scheduler del daemon largo. Borrar UNO de los dos deja rojo
// exactamente un subtest, que es cómo se sabe cuál falta.
//
// Y CRUZA ESOS DOS MODOS CON LA POLÍTICA, que es el eje que a este test le faltaba: el defecto
// no tiene NADA que ver con la redacción, y los cinco tests de la separación estaban todos
// construidos alrededor de `force_redact: true` —la forma que lo hizo invisible durante el
// diseño entero—. El caso que de verdad importa es el de abajo a la izquierda: `sin_redaccion`,
// o sea la máquina de cualquiera. Lo pidió el panel adversarial de este PR por nombre.
//
// Y EL DIRECTORIO ARRANCA CON `.musubi` Y NADA MÁS a propósito: es la forma que la tabla del
// criterio le DELEGA a este test en su `porQue` —«que el código llegue después no lo cubre el
// medidor sino remedirArbol»— y hasta ahora esa referencia cruzada apuntaba a un caso que acá
// no se armaba. Es, además, el workspace que deja `musubi init` antes del `git clone`.
//
// Sabotaje: que la re-medición no escriba. Caen los dos modos; el resto del archivo sigue verde.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="s.arbolAusente.Store(!hayIndexables)"
// arnes: a="_ = hayIndexables"
func TestElArbolQueLlegaDespuesDelArranqueSeMide(t *testing.T) {
	for _, c := range []struct {
		nombre   string
		redactar bool
		modo     string
	}{
		{"sin_redaccion full", false, ""},
		{"sin_redaccion incremental", false, "incremental"},
		{"con_redaccion full", true, ""},
		{"con_redaccion incremental", true, "incremental"},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			// Un workspace recién iniciado: `.musubi` y NADA más. La mide el arranque REAL.
			dir := t.TempDir()
			if err := os.MkdirAll(filepath.Join(dir, ".musubi"), 0o755); err != nil {
				t.Fatalf("no se pudo armar el workspace recién iniciado: %v", err)
			}
			s := newTestServerWithPath(t, dir)
			arranqueConPolitica(t, s, c.redactar)

			// PRECONDICIÓN, y no es ceremonia: si el arranque ya hubiera medido «presente», el
			// assert de abajo pasaría también con la re-medición borrada, y el test sería vacuo.
			if !s.arbolFueraDeAlcance() {
				t.Fatal("el arranque midió «árbol presente» sobre una carpeta sin una sola fuente: la " +
					"precondición de este test no se cumple y lo de abajo no mediría nada")
			}

			// El código llega. Esto es justo lo que el arranque no podía ver.
			writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/tarde\n")
			writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\nfunc Alpha() { beta() }\n\nfunc beta() {}\n")

			args := map[string]interface{}{}
			if c.modo != "" {
				args["mode"] = c.modo
			}
			mustCall(t, s, "musubi_codegraph_index", args)

			if s.arbolFueraDeAlcance() {
				t.Fatal("después de indexar un árbol que SÍ está en disco el predicado sigue diciendo " +
					"que no hay árbol: el hecho quedó congelado en el arranque")
			}

			// Y LA CONSECUENCIA, que es lo que de verdad se perdía. Sin esta mitad el test sólo
			// diría que un booleano cambió de valor; con ella dice que la frescura volvió a medirse.
			writeFile(t, filepath.Join(dir, "pkg", "a.go"), "package pkg\n\nfunc Alpha() { beta() }\n\nfunc beta() { _ = 1 }\n")
			m := decodeCG(t, mustCall(t, s, "musubi_map", map[string]interface{}{}))
			if stale, _ := m["stale"].(float64); stale < 1 {
				t.Errorf("stale = %v sobre un archivo editado después del índice: la medición de "+
					"frescura quedó apagada aunque el árbol está en disco (mapa: %v)", stale, m)
			}
		})
	}
}

// ★ C3 — LA PISTA DEL MISS NO SONDEA FUERA DEL PROYECTO.
//
// `pistaDelMiss` le preguntaba al disco con una ruta que manda el CLIENTE: `archivo` es el prefijo
// del `symbol` que llega por `musubi_code_graph`, `musubi_impact` y `musubi_code_context`, y sale
// de un `symbol[:i]` sin pasar por ninguna validación. Y la respuesta DISTINGUE: «existe en disco
// pero no está en el grafo» contra «no existe en el árbol indexado». O sea que un `symbol` como
// «../../../etc/passwd#func:x» convertía el hint en un ORÁCULO DE EXISTENCIA sobre todo el
// filesystem del proceso. Lo encontró el panel adversarial de este PR; el agujero es preexistente
// y el idioma para taparlo ya estaba en el repo (`methods.go:2131` y `:2288`).
//
// ESTE TEST AFIRMA LA PROPIEDAD, NO EL MECANISMO: un archivo que EXISTE afuera tiene que recibir el
// MISMO hint que una ruta que no existe en ninguna parte. No nombra `dentroDelProyecto`, así que
// sobrevive a que alguien reimplemente la guarda de otra forma —`EvalSymlinks`, una allowlist,
// cortar antes en el handler— y sigue cayendo si la reimplementación filtra.
//
// Y EL ORÁCULO ES CERRADO: el hint arranca con «<ruta>» y lo que sigue es la rama. Se comparan las
// COLAS, así que el test no se rompe si alguien reescribe el texto del hint, y sí se rompe si las
// dos rutas caen en ramas distintas.
//
// Sabotaje: desarmar la guarda dejando el sondeo. El caso de afuera pasa a la rama «existe» y el
// test cae; los demás tests del miss siguen verdes porque sus rutas son todas de adentro.
// arnes: archivo="internal/mcp/methods_codegraph.go"
// arnes: de="if s.dentroDelProyecto(archivo) {"
// arnes: a="if true {"
func TestPistaDelMissNoSondeaFueraDelProyecto(t *testing.T) {
	s, dir := proyectoIndexado(t)

	colaDelHint := func(t *testing.T, ruta string) string {
		t.Helper()
		cg := decodeCG(t, mustCall(t, s, "musubi_code_graph",
			map[string]interface{}{"symbol": ruta + "#func:Zeta"}))
		if cg["found"] != false {
			t.Fatalf("%q no debería encontrarse en el grafo: %v", ruta, cg)
		}
		hint, _ := cg["hint"].(string)
		if hint == "" {
			t.Fatalf("un miss sin hint no deja nada que comparar: %v", cg)
		}
		cola := strings.TrimPrefix(hint, "«"+ruta+"»")
		if cola == hint {
			t.Fatalf("el hint de %q no arranca con la ruta, así que la cola no es comparable: %q", ruta, hint)
		}
		return cola
	}

	// CONTRASTE: un archivo de ADENTRO que existe y no está en el grafo SÍ recibe la rama «existe».
	// Sin esta mitad, una guarda que cortara TODO —o un hint que contestara siempre lo mismo— pasaría
	// la comparación de abajo sin que el producto sirviera para nada.
	writeFile(t, filepath.Join(dir, "otro", "z.go"), "package otro\n\nfunc Zeta() {}\n")
	if adentro := colaDelHint(t, "otro/z.go"); !strings.Contains(adentro, "codegraph_index") {
		t.Fatalf("un archivo de adentro que existe en disco tiene que mandar a indexar; obtuve %q", adentro)
	}

	// Y AHORA LO MISMO, AFUERA. `dir` es un t.TempDir(), así que su padre está fuera del proyecto y
	// lo limpia el mismo `testing`. El archivo EXISTE: ése es el dato que no se puede filtrar.
	writeFile(t, filepath.Join(filepath.Dir(dir), "zz-fuera-del-proyecto.go"), "package fuera\n\nfunc Zeta() {}\n")

	existeAfuera := colaDelHint(t, "../zz-fuera-del-proyecto.go")
	noExisteEnNingunaParte := colaDelHint(t, "../zz-no-existe-en-ninguna-parte.go")

	if existeAfuera != noExisteEnNingunaParte {
		t.Errorf("el hint distingue si un archivo de AFUERA existe, y eso es un oráculo de existencia "+
			"sobre el filesystem del proceso:\n  existe afuera: %q\n  no existe:     %q",
			existeAfuera, noExisteEnNingunaParte)
	}
}
