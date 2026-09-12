package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TODA TOOL QUE PUEDA LLEGAR A UNA LLAMADA DE RED TIENE QUE DECLARAR `lockSelf`, Y AL REVÉS.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA REGLA YA ESTABA ESCRITA. LA GUARDA DEFENDÍA SU VIOLACIÓN.
//
// `server.go` la dice sin ambigüedad: el handler que hace I/O externa (motor LLM, embedder) se
// declara `lockSelf` y acota su propia sección crítica. Sostener el candado del despacho durante
// una llamada de red serializa el servidor entero.
//
// La guarda que existía —`TestG1ClasePorDefaultEsLaDeHoy`— hacía lo contrario de custodiarla:
// ENUMERABA cuatro nombres y fallaba si CUALQUIER OTRA tool declaraba `lockSelf`.
//
//	declaradas := map[string]bool{"musubi_recall", "musubi_ask", "musubi_distill", "musubi_sharpen"}
//	...
//	if hayClase { t.Errorf("%s: no debería declarar clase de candado", e.Name) }
//
// O sea que arreglar el bug PONÍA LA SUITE EN ROJO. Una guarda que convierte el arreglo en una
// regresión aparente es peor que no tener guarda: le enseña al próximo a aflojarla. Y su hermana
// lo confesaba por escrito — la sonda de G5 fue elegida para NO pasar por el embebedor, «porque
// guardar una observación también pasa por el embedder y la sonda quedaría atrapada en el mismo
// cuelgue que se está midiendo». La prueba esquivaba a propósito el camino defectuoso.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ SE DERIVA DEL AST Y NO DE UNA LISTA
//
// Una lista de nombres es una copia de la regla, y por lo tanto la próxima en envejecer: es el
// defecto que esta guarda viene a cerrar, cometido de nuevo. Acá el conjunto se DEDUCE:
//
//  1. se parsea `internal/mcp` con modo 0 —sin comentarios, así ningún comentario puede satisfacer
//     ni disparar esta guarda—;
//  2. se marcan las funciones que llaman al BORDE DE RED (los únicos métodos de red de las dos
//     interfaces: `Embed`/`EmbedBatch` del embebedor y `Ask` del motor; `Name()`, `Dimensions()`,
//     `Enabled()` y `Stats()` son locales y no cuentan);
//  3. se propaga hacia atrás por el grafo de llamadas del paquete hasta los handlers;
//  4. se lee el registro —`Name:`, `handler:` y `lock:` del mismo literal— y se cruza.
//
// Agregar una tool nueva que embeba, o mover una llamada de red a una función que hoy no la tiene,
// pone esto rojo sin que nadie tenga que acordarse de nada.
//
// SE EXIGE EN LOS DOS SENTIDOS, y el segundo no es simetría decorativa:
//
//	llega al borde ⇒ declara lockSelf ......... el defecto: el candado cruza la red
//	declara lockSelf ⇒ llega al borde ......... el otro: o la declaración quedó muerta cuando se
//	                                            sacó la llamada, o el ANÁLISIS SE QUEDÓ CIEGO y
//	                                            está devolviendo un conjunto vacío
//
// Sin la segunda dirección, esta guarda podría pasar en verde con el grafo roto: un `Inspect` que
// no matchea nada da cero violaciones. El verde de un barrido vacío se ve idéntico al de un
// árbol sano, y eso ya nos costó una sesión entera.
//
// EL ANÁLISIS SOBRE-APROXIMA A PROPÓSITO. El grafo liga por nombre simple de función, así que
// `s.foo()` y `foo()` son el mismo nodo y dos funciones homónimas se colapsan. Eso produce falsos
// POSITIVOS, nunca falsos negativos — para una guarda es la dirección segura: acusa de más y
// alguien mira, en vez de callar de menos y nadie mira.
//
// Sabotaje que la hace fallar: sacarle `lock: lockSelf` a `musubi_recall` (dirección 1), o
// sacársela a una tool y borrarle además la llamada al embebedor (dirección 2). Los dos corridos.
func TestNingunCandadoDelDespachoCruzaUnaLlamadaDeRed(t *testing.T) {
	// ── El BORDE DE RED. Son los únicos métodos de las dos interfaces que salen a la red.
	// Escrito a mano a propósito: es un HECHO DEL MUNDO —cuáles métodos hacen I/O—, no un
	// derivado. Derivarlo de la lista de tools que ya declaran `lockSelf` volvería espejo a la
	// guarda, que es justo cómo la anterior terminó defendiendo el bug.
	bordeDeRed := map[string]string{
		"Embed":      "embedding.Provider.Embed",
		"EmbedBatch": "embedding.BatchProvider.EmbedBatch / embedding.EmbedBatch",
		"Ask":        "cognition.Provider.Ask",
	}

	fset := token.NewFileSet()
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("no pude leer el paquete: %v", err)
	}

	llama := map[string]map[string]bool{} // función -> funciones que llama (por nombre simple)
	enElBorde := map[string]string{}      // función -> qué borde toca, y dónde
	bordeVisto := map[string]int{}        // método del borde -> cuántas llamadas se le vieron
	type entradaTool struct {
		handlers []string // la cadena entera: el envoltorio Y el handler envuelto
		lockSelf bool
		pos      string
	}
	tools := map[string]entradaTool{}
	var archivos, sinHandlerLegible int

	for _, e := range entradas {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
		if err != nil {
			t.Errorf("%s: no pude parsear: %v", e.Name(), err)
			continue
		}
		archivos++

		// ── 1) GRAFO DE LLAMADAS Y BORDE. Se recorre función por función para poder atribuir
		// cada llamada a quien la hace.
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			yo := fn.Name.Name
			if llama[yo] == nil {
				llama[yo] = map[string]bool{}
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				var nombre string
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					nombre = fun.Name
				case *ast.SelectorExpr:
					if fun.Sel == nil {
						return true
					}
					nombre = fun.Sel.Name
				default:
					return true
				}
				llama[yo][nombre] = true
				if _, esBorde := bordeDeRed[nombre]; esBorde {
					bordeVisto[nombre]++
					if _, ya := enElBorde[yo]; !ya {
						enElBorde[yo] = fset.Position(call.Pos()).String()
					}
				}
				return true
			})
		}

		// ── 2) EL REGISTRO. `Name:`, `handler:` y `lock:` viven en el MISMO literal, así que se
		// leen juntos y no hay que aparear por cercanía de líneas.
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			var nombre, pos string
			var handlers []string
			var teniaCampoHandler bool
			var lockSelf bool
			// El nombre puede estar un nivel más abajo (`Tool: Tool{Name: ...}`), así que se
			// busca en todo el literal; `handler` y `lock` son campos directos de toolEntry.
			ast.Inspect(lit, func(m ast.Node) bool {
				kv, ok := m.(*ast.KeyValueExpr)
				if !ok {
					return true
				}
				k, ok := kv.Key.(*ast.Ident)
				if !ok {
					return true
				}
				switch k.Name {
				case "Name":
					if bl, ok := kv.Value.(*ast.BasicLit); ok && bl.Kind == token.STRING {
						if v, err := strconv.Unquote(bl.Value); err == nil && strings.HasPrefix(v, "musubi_") {
							nombre = v
							pos = fset.Position(bl.Pos()).String()
						}
					}
				case "handler":
					// EL HANDLER PUEDE VENIR ENVUELTO, y ahí se cayó la primera versión de esta
					// guarda: `handler: s.countingSaveCtx(s.toolSaveObservation)` es un CallExpr,
					// no un selector. Al no reconocerlo, la tool quedaba FUERA del análisis sin
					// una palabra — un cero que significaba «no sé» adentro de la guarda escrita
					// para cazar esa clase. `musubi_save_observation`, el caso con 8 s medidos,
					// era una de las siete que se caían.
					//
					// Se recogen TODOS los nombres del árbol del valor: el envoltorio y lo
					// envuelto. Cualquiera de los dos que alcance el borde compromete el candado,
					// así que la unión es la respuesta correcta y además sobre-aproxima.
					teniaCampoHandler = true
					vistos := map[string]bool{}
					ast.Inspect(kv.Value, func(h ast.Node) bool {
						switch v := h.(type) {
						case *ast.SelectorExpr:
							// Se toma el SELECTOR (`toolX` de `s.toolX`) y no se desciende al
							// receptor: sin esto, el `s` entraba como nodo del grafo.
							if v.Sel != nil && !vistos[v.Sel.Name] {
								vistos[v.Sel.Name] = true
								handlers = append(handlers, v.Sel.Name)
							}
							return false
						case *ast.Ident:
							if !vistos[v.Name] {
								vistos[v.Name] = true
								handlers = append(handlers, v.Name)
							}
						}
						return true
					})
				case "lock":
					if id, ok := kv.Value.(*ast.Ident); ok && id.Name == "lockSelf" {
						lockSelf = true
					}
				}
				return true
			})
			switch {
			case nombre != "" && len(handlers) > 0:
				tools[nombre] = entradaTool{handlers: handlers, lockSelf: lockSelf, pos: pos}
			case nombre != "" && teniaCampoHandler:
				// DENUNCIAR, NO SALTEAR. Una tool con `handler:` en una forma que no sé leer es
				// cobertura que se pierde en silencio, y el silencio se ve igual que el verde.
				sinHandlerLegible++
				t.Errorf("%s: la tool `%s` declara `handler:` en una forma que esta guarda no sabe "+
					"leer, así que quedó FUERA del análisis. Arreglá el extractor: saltearla "+
					"convierte esta guarda en un verde que no cubre esa tool", pos, nombre)
			}
			return true
		})
	}

	// ════════════════════════════════════════════════════════════════════════════════════════
	// CONTROLES DE «MIRÉ ALGO». Los tres son fatales: un cero en cualquiera de ellos significa
	// «no pude medir», no «está todo bien», y las dos cosas dan el mismo verde.
	// ════════════════════════════════════════════════════════════════════════════════════════
	if archivos == 0 {
		t.Fatal("no parseé NI UN archivo de internal/mcp. El barrido no llegó: esta guarda no midió nada")
	}
	if len(tools) == 0 {
		t.Fatal("no encontré NI UNA tool con `Name:` y `handler:` en el mismo literal. O el registro " +
			"cambió de forma, o esta guarda quedó apuntando al aire — en los dos casos dejó de cubrir")
	}
	// Se exige que se vea AL MENOS UNO de los bordes, no todos: `EmbedBatch` legítimamente no se
	// llama desde `internal/mcp` —vive en el backfill de `internal/memory`— y exigirlo produciría
	// un rojo permanente que enseña a ignorar esta guarda. Pero cero bordes SÍ es ceguera.
	var bordesVivos []string
	for metodo := range bordeDeRed {
		if bordeVisto[metodo] > 0 {
			bordesVivos = append(bordesVivos, metodo)
		}
	}
	sort.Strings(bordesVivos)
	if len(bordesVivos) == 0 {
		nombres := make([]string, 0, len(bordeDeRed))
		for m, d := range bordeDeRed {
			nombres = append(nombres, m+" ("+d+")")
		}
		sort.Strings(nombres)
		t.Fatalf("no vi NI UNA llamada a ninguno de los bordes de red en todo internal/mcp: %s. "+
			"O los métodos se renombraron, o las llamadas se movieron a otro paquete: en los dos "+
			"casos esta guarda quedó CIEGA y su verde no significa nada", strings.Join(nombres, ", "))
	}

	// ── 3) PROPAGACIÓN HACIA ATRÁS. Punto fijo: una función alcanza el borde si lo toca ella, o
	// si llama a alguna que lo alcance.
	alcanza := map[string]bool{}
	for fn := range enElBorde {
		alcanza[fn] = true
	}
	for cambio := true; cambio; {
		cambio = false
		for quien, llamadas := range llama {
			if alcanza[quien] {
				continue
			}
			for a := range llamadas {
				if alcanza[a] {
					alcanza[quien] = true
					cambio = true
					break
				}
			}
		}
	}

	// ── 4) EL CRUCE, en los dos sentidos.
	var faltanLock, lockMuerto []string
	for nombre, e := range tools {
		var llega bool
		var porQuien string
		for _, h := range e.handlers {
			if alcanza[h] {
				llega, porQuien = true, h
				break
			}
		}
		switch {
		case llega && !e.lockSelf:
			faltanLock = append(faltanLock, nombre+" (por "+porQuien+", "+e.pos+")")
		case !llega && e.lockSelf:
			lockMuerto = append(lockMuerto, nombre+" ("+strings.Join(e.handlers, "/")+", "+e.pos+")")
		}
	}
	sort.Strings(faltanLock)
	sort.Strings(lockMuerto)

	if len(faltanLock) > 0 {
		t.Errorf("EL CANDADO DEL DESPACHO CRUZA UNA LLAMADA DE RED en %d tools. Sus handlers pueden "+
			"llegar al embebedor o al motor, y el despachador les toma el candado ANTES de entrar: "+
			"mientras esa llamada de red tarda, el servidor entero no atiende a nadie.\n"+
			"La regla está escrita en server.go y estas tools no la cumplen:\n  %s\n"+
			"Arreglo: `lock: lockSelf` en su entrada del registro, y acotar la sección crítica con "+
			"withReadLock/withWriteLock adentro del handler.",
			len(faltanLock), strings.Join(faltanLock, "\n  "))
	}
	if len(lockMuerto) > 0 {
		t.Errorf("%d tools declaran `lockSelf` y su handler NO llega a ninguna llamada de red:\n  %s\n"+
			"Una de dos, y hay que decidir cuál: o la llamada se sacó y la declaración quedó muerta "+
			"(entonces borrala, porque `lockSelf` deja al handler sin el candado que el despachador "+
			"le daba), o el ANÁLISIS DE ESTA GUARDA se quedó ciego y su verde de arriba no vale.",
			len(lockMuerto), strings.Join(lockMuerto, "\n  "))
	}

	t.Logf("%d archivos · %d tools legibles (%d con handler ilegible) · bordes vivos: %s · "+
		"%d funciones tocan el borde, %d lo alcanzan transitivamente",
		archivos, len(tools), sinHandlerLegible, strings.Join(bordesVivos, ", "),
		len(enElBorde), len(alcanza))
}

// ════════════════════════════════════════════════════════════════════════════════════════════
// LA MITAD DE COMPORTAMIENTO: la guarda de arriba mira la FORMA, ésta mide el EFECTO.
//
// Una guarda estructural sola no alcanza. Puede estar verde con `lock: lockSelf` declarado y el
// embed adentro del `withWriteLock`, que es exactamente el defecto con la marca puesta. Así que
// acá se cuelga el embebedor de verdad, se deja a `musubi_save_observation` atrapada dentro de la
// llamada de red, y se exige que OTRO escritor entre igual.
//
// POR QUÉ ESTA MEDICIÓN NO EXISTÍA. La sonda de G5 —`exigeQueUnEscritorSinEmbedderResponda`— fue
// elegida para NO pasar por el embebedor, y su comentario lo dice: «guardar una observación
// también pasa por el embedder y la sonda quedaría atrapada en el mismo cuelgue que se está
// midiendo». El arnés esquivaba el caso defectuoso a propósito, y el `embedderBloqueante` que hacía
// falta para medirlo estaba escrito, con su interruptor `activo` y su comentario explicando para
// qué servía, y NADIE lo usaba. Construido y nunca prendido, en el propio arnés de prueba.
//
// LA SONDA TIENE QUE SER ESCRITORA, y no es un detalle: una de sólo lectura pasaría igual con el
// defecto puesto si el tramo tomara RLock, porque dos lectores conviven. El escritor es el único
// que se bloquea con CUALQUIER candado tomado, compartido o exclusivo.
//
// Sabotaje que la hace fallar: sacarle `lock: lockSelf` a `musubi_save_observation` en el registro.
// Corrido: la sonda concurrente no vuelve y la prueba declara el bloqueo.
func TestGuardarUnaObservacionNoCongelaElServidor(t *testing.T) {
	emb := nuevoEmbedderBloqueante()
	s := newTestServer(t, emb)

	// El interruptor se prende DESPUÉS de construir el servidor: si el embebedor colgara desde el
	// arranque, frenaría cualquier siembra y la prueba moriría antes de medir.
	emb.activo.Store(true)

	guardado := make(chan *RpcError, 1)
	go func() {
		guardado <- llamarSinT(s, "musubi_save_observation", map[string]interface{}{
			"topic_key": "candado/medicion",
			"content":   "esta observación se queda colgada en el embebedor a propósito, para medir si el servidor sigue atendiendo",
		})
	}()

	// SE ESPERA A ESTAR ADENTRO DEL Embed. Sin esto la sonda podría correr antes de que la tool
	// llegue a la llamada de red, y la prueba pasaría con el defecto puesto — un verde que mide
	// el momento equivocado.
	select {
	case <-emb.entro:
	case <-time.After(esperaArranque):
		t.Fatal("musubi_save_observation no llegó al embebedor: esta prueba no pudo empezar a medir, " +
			"así que su verde no significaría nada")
	}

	exigeQueUnEscritorSinEmbedderResponda(t, s,
		"musubi_save_observation está colgada dentro del embebedor")

	close(emb.soltar)
	select {
	case rpcErr := <-guardado:
		if rpcErr != nil {
			t.Fatalf("al soltar el embebedor, el guardado falló: %+v", rpcErr)
		}
	case <-time.After(esperaMax):
		t.Fatal("soltado el embebedor, el guardado igual no volvió: el candado quedó tomado por otra cosa")
	}
}
