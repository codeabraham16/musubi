package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"musubi/internal/codeintel"
	"musubi/internal/memory"
)

// precheck.go implementa 'musubi precheck --hook-mode': el hook PreToolUse, atado a DOS momentos
// que preguntan cosas distintas.
//
// ANTES DE LEER un archivo, Musubi mira su memoria de código: si ya tiene un gist FRESCO lo
// inyecta (para no re-leer el archivo entero), si está desactualizado avisa, y si no hay gist y el
// archivo es grande recuerda guardarlo. Suma además la ESTRUCTURA sacada del grafo, para navegarlo
// sin abrirlo (encendido por default desde el 2026-09-05; se apaga con MUSUBI_CODEGRAPH_HOOK=0).
//
// LAS TRES SUPERFICIES DICEN SI ESTÁN AL DÍA. Cada una deriva de un archivo que pudo cambiar
// después, y la diferencia entre «esto es el archivo» y «esto es lo que el archivo era» es la que
// decide si el agente vuelve a mirar. Ver medirFrescura.
//
// ANTES DE ESCRIBIRLO contesta la otra pregunta —"¿qué se rompe si toco esto?"— con el RADIO DE
// IMPACTO: qué símbolos del archivo tienen callers, cuántos de ellos son de producción y cuántos
// arrastra el cierre transitivo.
//
// Hace AUTOMÁTICO el uso de la memoria de código y del grafo, sin que el agente tenga que
// acordarse. 100% model-free.

// umbralArchivoGrande es el tamaño (bytes) a partir del cual, si no hay gist,
// vale la pena recordar guardarlo. Por debajo, no molesta.
const umbralArchivoGrande = 1500

// codeStore es lo que el hook necesita del motor: leer la memoria de código, los errores
// conocidos (telemetría) y el GRAFO DE CÓDIGO (Track 20 · F2-B) del archivo que se va a leer,
// y contabilizar en el ledger lo que inyecta (estas superficies también gastan contexto y antes
// no se medían).
type codeStore interface {
	GetCodeMemory(path string) (memory.CodeMemory, bool, error)
	GetUnresolvedTelemetryLogsForFiles(files []string) ([]memory.TelemetryLog, error)
	LedgerAdd(sessionID, surface string, tokens int) (memory.TokenLedger, error)
	// Lecturas del grafo de código (F2-B): estructura del archivo sin leerlo.
	ListGraphNodesForFileCtx(ctx context.Context, path string) ([]memory.GraphNode, error)
	GraphOutEdgesCtx(ctx context.Context, fromKey string) ([]memory.GraphEdge, error)
	GraphInEdgesCtx(ctx context.Context, toKey string) ([]memory.GraphEdge, error)
	// Cierre transitivo de callers: el radio de impacto de un símbolo que se va a editar.
	GraphImpactCtx(ctx context.Context, key string, maxDepth, maxNodes int) ([]string, error)
}

// maxPrecheckTelemetry acota cuántos errores conocidos se surfacean por lectura, para no
// inundar el contexto del hook.
const maxPrecheckTelemetry = 3

// precheckInput es el subconjunto del JSON de stdin de PreToolUse que usamos.
type precheckInput struct {
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath string `json:"file_path"`
	} `json:"tool_input"`
	SessionID string `json:"session_id"`
}

// precheckOutput arma el additionalContext del hook PreToolUse para una lectura.
// Devuelve "" (silencioso) si no aplica.
func precheckOutput(store codeStore, root string, stdin io.Reader) string {
	if store == nil {
		return ""
	}
	data, err := io.ReadAll(stdin)
	if err != nil {
		return ""
	}
	var in precheckInput
	if err := json.Unmarshal(data, &in); err != nil {
		return ""
	}
	if in.ToolInput.FilePath == "" {
		return ""
	}

	path := in.ToolInput.FilePath
	key := memory.NormalizeCodePath(root, path)

	// ANTES DE ESCRIBIR es otro momento y merece otra superficie. Leer y editar disparan preguntas
	// distintas: al leer, "¿de qué va este archivo?"; al editar, "¿qué se rompe si lo toco?". El
	// grafo sabe responder la segunda desde que existe Track 20, y no la respondía nunca: medido
	// contra el ledger del cerebro central (400 días), musubi_impact tenía CERO invocaciones con
	// 3.771 nodos indexados en este repo, 1.135 en altura-erp y 988 en musubi-body. La causa no era
	// que faltara la herramienta sino que el único empujón vivía al final del mensaje de lectura
	// —"profundizá con musubi_impact"—, o sea en el turno equivocado: para cuando el agente decide
	// cambiar una firma, ese texto quedó veinte mensajes atrás.
	if esEdicion(in.ToolName) {
		m := impactMessage(store, root, key)
		if m == "" {
			return ""
		}
		_, _ = store.LedgerAdd(in.SessionID, "precheck_impacto", memory.EstimateTokens(m))
		return preEnvelope(m)
	}
	if in.ToolName != "Read" {
		return ""
	}

	// Dos superficies que se combinan: la memoria de código (gist) y los errores conocidos
	// del archivo (telemetría, T6.3). Cualquiera puede estar vacía. Cada una se contabiliza
	// en el ledger por su huella real (model-free) para que el gasto del PreToolUse sea
	// medible junto con el resto de las superficies de Musubi.
	parts := make([]string, 0, 2)
	if m := codeMemoryMessage(store, root, path, key); m != "" {
		_, _ = store.LedgerAdd(in.SessionID, "precheck_code", memory.EstimateTokens(m))
		parts = append(parts, m)
	}
	if m := telemetryMessage(store, key, path); m != "" {
		_, _ = store.LedgerAdd(in.SessionID, "precheck_telemetry", memory.EstimateTokens(m))
		parts = append(parts, m)
	}
	// Grafo de código (F2-B): la palanca de tokens — inyecta la ESTRUCTURA del archivo (imports +
	// símbolos con sus callers/callees) para navegarlo sin leerlo. VIENE ENCENDIDO desde el
	// 2026-09-05, con MUSUBI_CODEGRAPH_HOOK=0 como apagador (ver codegraphHookEnabled, que explica
	// la medición detrás del cambio). Este comentario decía «OPT-IN … default OFF» al lado del
	// gate que ya devolvía true, que es cómo se lee un default nuevo como si fuera el viejo.
	// Dispara sólo si el archivo está indexado, así que es inerte en un repo sin grafo.
	if codegraphHookEnabled() {
		if m := codeGraphMessage(store, root, key); m != "" {
			_, _ = store.LedgerAdd(in.SessionID, "precheck_codegraph", memory.EstimateTokens(m))
			parts = append(parts, m)
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return preEnvelope(strings.Join(parts, "\n\n"))
}

// codegraphHookEnabled indica si se inyecta la estructura del grafo al LEER un archivo.
//
// NACIÓ OPT-IN Y AHORA VIENE ENCENDIDO, con el apagador a mano. El opt-in se eligió por prudencia de
// tokens (la estructura de un archivo real mide ~1.745 chars), pero medido el 2026-09-03 esa
// prudencia salía carísima: contra el ledger del cerebro central en 14 días, el grafo se ALIMENTÓ
// 161 veces y se LEYÓ 14 — y `musubi_impact`, `musubi_code_graph_viz` y `musubi_entity_context`
// tenían CERO invocaciones. Un default apagado no es neutral: es la decisión de que nadie lo use.
//
// El costo se paga contra la alternativa, no contra cero: para «quién llama a formasPara», el grafo
// devolvió 450 chars con callers Y callees, y `grep` 1.367 sin poder distinguir una llamada de una
// mención en un comentario. Inyectar es TRES VECES más barato que el camino que reemplaza — y es
// inerte si el archivo no está indexado, así que en un repo sin grafo no cuesta nada.
//
// El escape queda explícito (MUSUBI_CODEGRAPH_HOOK=0) porque un default nuevo sin apagador es un
// default impuesto: quien mida que en su repo no le conviene tiene que poder salirse sin parchear.
func codegraphHookEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MUSUBI_CODEGRAPH_HOOK"))) {
	case "0", "false", "no", "off":
		return false
	}
	return true
}

// Cotas para que el contexto del grafo sea compacto (la palanca de tokens, no un volcado).
const (
	maxGraphSymbols = 10
	maxGraphRefs    = 5
)

// ─────────────────────────────────────────────────────────────────────────────────────────────
// LA FRESCURA DEL GRAFO SE DICE, NO SE SUPONE — Y ESTE ARCHIVO YA LO HACÍA BIEN EN UN LADO
//
// Las tres superficies del hook inyectan algo derivado de un archivo, así que las tres pueden
// estar viejas. `codeMemoryMessage` lo trata desde el primer día: compara la huella y si no
// coincide AVISA en vez de callar. Las dos del grafo no lo hacían, y decían lo contrario de lo
// que sabían: «está indexado: navegá su estructura SIN leerlo» y «tocarlo no arrastra a nadie
// conocido», las dos afirmadas sin mirar el disco.
//
// Mientras estaba OPT-IN el hueco no costaba nada. El 2026-09-05 el default pasó a ENCENDIDO
// (ver codegraphHookEnabled) y se empezó a pagar en CADA lectura. Medido ese día en este repo:
// 798 archivos con huella, 791 al día y 7 viejos — cuatro de los siete cambiados por el mismo
// despliegue que encendió el hook. En concreto, un Read de internal/fleet/procparse.go inyectaba
// una lista de diez símbolos SIN `ElegirTemperatura`, que es justo la función que ese despliegue
// agregó y que reemplazó a `ParsearTempMiligrados` como punto de entrada. La ventana llega a las
// 6 h que dura el reindexado (Maintenance.GraphIndexHours).
//
// Y la tool equivalente SÍ lo marca (`cgView` → `Stale: s.cgStale(n)`), o sea que la señal
// existía y este camino la tiraba: N superficies que deberían decir lo mismo, N-1 lo decían.
//
// NO SE SUPRIME EL MENSAJE CUANDO ESTÁ VIEJO. Los callers y callees no se pueden sacar del
// archivo que se está leyendo —viven en otros archivos— así que siguen valiendo. Lo que se
// retira es la INVITACIÓN a no leer, que es la parte que se vuelve una trampa.
type frescuraDelGrafo int

const (
	grafoAlDia    frescuraDelGrafo = iota // se comparó la huella y coincide
	grafoCambiado                         // se comparó la huella y NO coincide: el archivo cambió
	grafoSinSaber                         // no se pudo comparar: sin archivo, o sin huella guardada
)

// medirFrescura compara la huella del archivo EN DISCO con la que guardaron sus nodos.
//
// Devuelve grafoSinSaber y no grafoCambiado cuando el archivo no se puede leer, porque son cosas
// distintas y se arreglan distinto: «cambió» se arregla reindexando, «no está» quiere decir que
// el grafo tiene un fantasma o que este binario está mirando otro árbol. Confundirlas mandaría a
// reindexar a alguien cuyo problema es otro.
func medirFrescura(root, key string, nodes []memory.GraphNode) frescuraDelGrafo {
	actual, err := memory.FileFingerprint(root, key)
	if err != nil || actual == "" {
		return grafoSinSaber
	}
	// SIN HUELLA GUARDADA NO HAY «AL DÍA», Y ESTE ERA EL MISMO DEFECTO OTRA VEZ. La primera
	// versión saltaba los nodos sin `SrcFingerprint` —para no poner en rojo un grafo indexado
	// antes de que se guardaran huellas— y devolvía grafoAlDia, con lo cual el mensaje afirmaba
	// «su huella COINCIDE con el archivo en disco» sin haber comparado nada. Lo destapó una
	// prueba existente, que arma sus nodos sin huella y pasaba en verde leyendo esa afirmación.
	// Un dato ausente que se lee como confirmación es exactamente lo que este hook vino a dejar
	// de hacer.
	comparadas := 0
	for _, n := range nodes {
		if n.SrcFingerprint == "" {
			continue
		}
		comparadas++
		if n.SrcFingerprint != actual {
			return grafoCambiado
		}
	}
	if comparadas == 0 {
		return grafoSinSaber
	}
	return grafoAlDia
}

// codeGraphMessage arma el contexto de ESTRUCTURA de un archivo desde el grafo de código: sus
// imports y sus funciones/métodos con a quién llaman y quién los llama. "" si el archivo no está
// indexado (inerte hasta correr musubi_codegraph_index). Model-free: solo recorre el grafo.
func codeGraphMessage(store codeStore, root, key string) string {
	ctx := context.Background()
	nodes, err := store.ListGraphNodesForFileCtx(ctx, key)
	if err != nil || len(nodes) == 0 {
		return ""
	}
	var b strings.Builder
	switch medirFrescura(root, key, nodes) {
	case grafoAlDia:
		fmt.Fprintf(&b, "[Musubi — grafo de código] «%s» está indexado y su huella COINCIDE con el archivo en disco: navegá su estructura SIN leerlo.", key)
	case grafoCambiado:
		fmt.Fprintf(&b, "[Musubi — grafo de código] «%s» está indexado pero el archivo CAMBIÓ desde entonces: "+
			"esta lista puede faltar símbolos nuevos y nombrar borrados, así que NO reemplaza la lectura que estás "+
			"haciendo. Los callers/callees sí siguen valiendo (no salen de este archivo). Reindexá con musubi_codegraph_index.", key)
	case grafoSinSaber:
		fmt.Fprintf(&b, "[Musubi — grafo de código] «%s» está en el grafo pero NO pude verificar su frescura "+
			"—el archivo no se pudo leer, o se indexó sin guardar huella—: puede estar al día o de hace horas. "+
			"Tomá esto como una pista, no como el archivo.", key)
	}

	if fe, _ := store.GraphOutEdgesCtx(ctx, key); len(fe) > 0 {
		var imps []string
		for _, e := range fe {
			if e.Kind == codeintel.EdgeImports {
				imps = append(imps, strings.TrimPrefix(e.ToKey, "pkg:"))
			}
		}
		if len(imps) > 0 {
			b.WriteString("\nimporta: " + joinCapped(imps, 8))
		}
	}

	shown := 0
	for _, n := range nodes {
		if n.Kind != codeintel.KindFunc && n.Kind != codeintel.KindMethod {
			continue
		}
		if shown >= maxGraphSymbols {
			b.WriteString("\n(+más símbolos)")
			break
		}
		callees := graphRefNames(store, ctx, n.Key, true)
		callers := graphRefNames(store, ctx, n.Key, false)
		fmt.Fprintf(&b, "\n- %s → llama a: %s | ← lo llaman: %s",
			n.Name, noneIfEmpty(joinCapped(callees, maxGraphRefs)), noneIfEmpty(joinCapped(callers, maxGraphRefs)))
		shown++
	}
	b.WriteString("\nProfundizá con musubi_code_graph / musubi_impact / musubi_code_context.")
	return b.String()
}

// Cotas del radio de impacto. Son más chicas que las de musubi_impact a propósito: acá el trabajo
// se paga en el camino crítico de CADA edición, y lo que se busca no es el cierre completo sino
// saber si hace falta pedirlo.
const (
	maxSimbolosImpacto = 3  // sólo los símbolos más conectados; el resto se resume en una línea
	profundidadImpacto = 4  // saltos de BFS hacia atrás por aristas CALLS
	topeNodosImpacto   = 60 // techo duro del recorrido
)

// esEdicion dice si la tool que está por correr ESCRIBE el archivo. Son los momentos en que
// "¿quién depende de esto?" deja de ser curiosidad y pasa a ser la pregunta que evita el bug.
func esEdicion(tool string) bool {
	switch tool {
	case "Edit", "Write", "MultiEdit", "NotebookEdit":
		return true
	}
	return false
}

// impactMessage arma el RADIO DE IMPACTO de un archivo que se va a editar: qué símbolos suyos
// tienen quien los llame, cuántos son de forma directa y cuántos arrastrando el cierre transitivo.
// "" si el archivo no está en el grafo — inerte hasta que se indexe, igual que codeGraphMessage.
//
// El caso "ningún símbolo tiene callers" NO devuelve vacío: que un archivo esté aislado es
// justamente lo que uno quiere saber antes de cambiarlo, y cuesta una línea decirlo. Callar ahí
// sería confundir "no hay riesgo" con "no sé", que es la distinción que el resto de esta memoria
// se toma el trabajo de mantener.
func impactMessage(store codeStore, root, key string) string {
	ctx := context.Background()
	nodes, err := store.ListGraphNodesForFileCtx(ctx, key)
	if err != nil || len(nodes) == 0 {
		return ""
	}
	// ACÁ LA FRESCURA PESA MÁS QUE EN LA LECTURA, porque el mensaje no describe: TRANQUILIZA.
	// «Ningún símbolo tiene callers: tocarlo no arrastra a nadie conocido» es una afirmación de
	// seguridad, y sobre un grafo viejo es falsa en la dirección que duele — el caller nuevo es
	// exactamente el que el grafo todavía no vio. Y esto corre antes de CADA edición, o sea en el
	// momento en que alguien va a decidir si mira quién depende de lo que está por cambiar.
	frescura := medirFrescura(root, key, nodes)
	reserva := ""
	switch frescura {
	case grafoCambiado:
		reserva = " · OJO: el archivo cambió desde que se indexó, así que esto puede estar incompleto — un caller nuevo no aparece acá"
	case grafoSinSaber:
		reserva = " · OJO: no pude verificar la frescura del grafo (sin archivo en disco o sin huella guardada), así que no sé si está al día"
	}

	type simboloConectado struct {
		nombre     string
		clave      string
		directos   []string // nombres, los de producción primero
		produccion int      // cuántos de esos callers NO son tests
	}
	var conectados []simboloConectado
	funciones := 0
	for _, n := range nodes {
		if n.Kind != codeintel.KindFunc && n.Kind != codeintel.KindMethod {
			continue
		}
		funciones++
		claves := graphRefKeys(store, ctx, n.Key)
		if len(claves) == 0 {
			continue
		}
		nombres, prod := ordenarPorProduccion(claves)
		conectados = append(conectados, simboloConectado{n.Name, n.Key, nombres, prod})
	}
	if funciones == 0 {
		return ""
	}
	if len(conectados) == 0 {
		// La versión al día afirma; las otras dos dicen lo que saben y lo que no. La diferencia
		// entre «no arrastra a nadie» y «el grafo no conoce a nadie, y está viejo» es toda.
		if frescura == grafoAlDia {
			return fmt.Sprintf("[Musubi — radio de impacto] Ningún símbolo de «%s» tiene callers en el grafo, "+
				"y su huella coincide con el disco: tocarlo no arrastra a nadie conocido.", key)
		}
		return fmt.Sprintf("[Musubi — radio de impacto] Ningún símbolo de «%s» tiene callers EN EL GRAFO%s. "+
			"No lo leas como «no arrastra a nadie»: leelo como «el grafo no sabe».", key, reserva)
	}

	// Ordena por callers DE PRODUCCIÓN, no por callers a secas. Medido sobre este mismo repo: los
	// tests dominan las listas (scoreCandidates tiene 9 callers y 8 son Test*), así que rankear por
	// el total pone arriba lo más testeado en vez de lo más usado, que es justo al revés de lo que
	// hace falta saber antes de cambiar una firma. Un test que se rompe lo dice el compilador; un
	// caller de producción que se rompe lo decís vos. Estable para que dos ediciones seguidas del
	// mismo archivo no reordenen el mensaje sin motivo.
	sort.SliceStable(conectados, func(a, b int) bool {
		if conectados[a].produccion != conectados[b].produccion {
			return conectados[a].produccion > conectados[b].produccion
		}
		return len(conectados[a].directos) > len(conectados[b].directos)
	})

	var b strings.Builder
	fmt.Fprintf(&b, "[Musubi — radio de impacto] Vas a editar «%s». Según el grafo%s, esto cuelga de sus símbolos:", key, reserva)
	mostrados := conectados
	if len(mostrados) > maxSimbolosImpacto {
		mostrados = mostrados[:maxSimbolosImpacto]
	}
	for _, s := range mostrados {
		// El cierre transitivo se calcula SÓLO para los que se muestran: es un BFS por símbolo y
		// esto corre antes de cada edición, no en una consulta que alguien pidió.
		total := len(s.directos)
		if cierre, err := store.GraphImpactCtx(ctx, s.clave, profundidadImpacto, topeNodosImpacto); err == nil && len(cierre) > total {
			total = len(cierre)
		}
		fmt.Fprintf(&b, "\n- %s ← %d directo(s), %d fuera de tests · %d en total: %s",
			s.nombre, len(s.directos), s.produccion, total, joinCapped(s.directos, maxGraphRefs))
	}
	if resto := len(conectados) - len(mostrados); resto > 0 {
		fmt.Fprintf(&b, "\n(+%d símbolo(s) más con callers)", resto)
	}
	b.WriteString("\nSi vas a cambiar una FIRMA, pedí el cierre completo con musubi_impact (symbol='" +
		mostrados[0].clave + "').")
	return b.String()
}

// graphRefKeys devuelve las CLAVES (no los nombres) de quienes llaman a key. A diferencia de
// graphRefNames, conserva la ruta del archivo, que es lo único que permite después distinguir un
// caller de producción de uno de test.
func graphRefKeys(store codeStore, ctx context.Context, key string) []string {
	edges, _ := store.GraphInEdgesCtx(ctx, key)
	var claves []string
	for _, e := range edges {
		if e.Kind != codeintel.EdgeCalls {
			continue
		}
		claves = append(claves, e.FromKey)
	}
	return claves
}

// esRutaDeTest reconoce los archivos de test por la convención de cada ecosistema: `_test.go`,
// `foo_test.py`, `foo.test.ts`, `foo.spec.js`. Es una heurística por NOMBRE y puede errarle a un
// proyecto con convención propia; el costo de errarle es sólo el orden de una lista, así que no
// vale la pena algo más caro.
func esRutaDeTest(clave string) bool {
	ruta := clave
	if i := strings.Index(ruta, "#"); i >= 0 {
		ruta = ruta[:i]
	}
	ruta = strings.ToLower(ruta)
	for _, marca := range []string{"_test.", ".test.", ".spec.", "_spec."} {
		if strings.Contains(ruta, marca) {
			return true
		}
	}
	return strings.Contains(ruta, "/tests/") || strings.HasPrefix(ruta, "tests/")
}

// ordenarPorProduccion convierte claves de callers en NOMBRES únicos, con los de producción
// adelante, y devuelve cuántos de esos nombres no vienen de un archivo de test.
func ordenarPorProduccion(claves []string) (nombres []string, produccion int) {
	var prod, tests []string
	visto := make(map[string]bool, len(claves))
	for _, c := range claves {
		n := symNameFromKey(c)
		if visto[n] {
			continue
		}
		visto[n] = true
		if esRutaDeTest(c) {
			tests = append(tests, n)
		} else {
			prod = append(prod, n)
		}
	}
	return append(prod, tests...), len(prod)
}

// graphRefNames devuelve los NOMBRES de los símbolos conectados por CALLS a key (out=callees,
// in=callers). Extrae el nombre del node_key para no volcar claves largas.
func graphRefNames(store codeStore, ctx context.Context, key string, out bool) []string {
	var edges []memory.GraphEdge
	if out {
		edges, _ = store.GraphOutEdgesCtx(ctx, key)
	} else {
		edges, _ = store.GraphInEdgesCtx(ctx, key)
	}
	var names []string
	for _, e := range edges {
		if e.Kind != codeintel.EdgeCalls {
			continue
		}
		ref := e.ToKey
		if !out {
			ref = e.FromKey
		}
		names = append(names, symNameFromKey(ref))
	}
	return names
}

// symNameFromKey extrae el nombre de un node_key "path#kind:name".
func symNameFromKey(key string) string {
	if i := strings.Index(key, "#"); i >= 0 {
		rest := key[i+1:]
		if j := strings.Index(rest, ":"); j >= 0 {
			return rest[j+1:]
		}
		return rest
	}
	return key
}

// joinCapped une los items con coma, mostrando a lo sumo n y "(+K)" si hay más.
func joinCapped(items []string, n int) string {
	if len(items) <= n {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:n], ", ") + fmt.Sprintf(" (+%d)", len(items)-n)
}

func noneIfEmpty(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// codeMemoryMessage arma el aviso de memoria de código para el archivo (gist fresco,
// desactualizado, o nudge de guardar si es grande y no hay). "" si no aplica.
func codeMemoryMessage(store codeStore, root, path, key string) string {
	cm, ok, err := store.GetCodeMemory(key)
	if err != nil {
		return ""
	}
	if !ok {
		if fileIsLarge(root, path) {
			return fmt.Sprintf("[Musubi — código] No hay gist de «%s». Tras leerlo, guardá uno con musubi_save_code (path, gist, symbols) para no re-leerlo entero en futuros turnos/sesiones.", key)
		}
		return ""
	}
	current, _ := memory.FileFingerprint(root, path)
	if current != "" && current == cm.Fingerprint {
		msg := fmt.Sprintf("[Musubi — código] Ya tenés un gist FRESCO de «%s»: %s", key, cm.Gist)
		if cm.Symbols != "" {
			msg += " | símbolos: " + cm.Symbols
		}
		msg += ". Si solo necesitás una parte, leé el rango puntual en vez del archivo entero (evitás re-pagar la lectura)."
		return msg
	}
	return fmt.Sprintf("[Musubi — código] Tenés un gist de «%s» pero el archivo CAMBIÓ desde entonces. Leé lo necesario y actualizá el gist con musubi_save_code.", key)
}

// telemetryMessage arma el aviso de errores conocidos NO resueltos del archivo (T6.3):
// Musubi recuerda proactivamente "este archivo ya te dio este error, este fue el fix"
// ANTES de que lo edites. "" si no hay. Acota a maxPrecheckTelemetry para no inundar.
func telemetryMessage(store codeStore, key, path string) string {
	logs, err := store.GetUnresolvedTelemetryLogsForFiles([]string{key, path})
	if err != nil || len(logs) == 0 {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[Musubi — errores conocidos] Este archivo tiene %d error(es) sin resolver registrado(s):", len(logs))
	shown := logs
	if len(shown) > maxPrecheckTelemetry {
		shown = shown[:maxPrecheckTelemetry]
	}
	for _, l := range shown {
		fmt.Fprintf(&b, "\n- [id %d] %s", l.ID, l.ErrorMessage)
		if strings.TrimSpace(l.SuggestedPatch) != "" {
			fmt.Fprintf(&b, " → fix sugerido: %s", l.SuggestedPatch)
		}
	}
	if len(logs) > maxPrecheckTelemetry {
		fmt.Fprintf(&b, "\n(+%d más)", len(logs)-maxPrecheckTelemetry)
	}
	b.WriteString("\nSi lo resolviste, marcalo con musubi_resolve_telemetry {id}.")
	return b.String()
}

// fileIsLarge indica si el archivo supera el umbral (best-effort; false si no se
// puede stat-ear, para no molestar).
func fileIsLarge(root, path string) bool {
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(root, path)
	}
	fi, err := os.Stat(full)
	if err != nil {
		return false
	}
	return fi.Size() >= umbralArchivoGrande
}

// preEnvelope serializa el envelope de PreToolUse con additionalContext y
// permissionDecision=allow (no bloquea: solo aporta contexto).
func preEnvelope(ctx string) string {
	envelope := map[string]interface{}{
		"hookSpecificOutput": map[string]interface{}{
			"hookEventName":      "PreToolUse",
			"permissionDecision": "allow",
			"additionalContext":  ctx,
		},
	}
	datos, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi precheck: error al serializar: %v\n", err)
		return ""
	}
	return string(datos)
}

// runPrecheck implementa 'musubi precheck [--hook-mode]'. Sin --hook-mode es no-op.
// En hook-mode lee stdin, abre la memoria (best-effort) y escribe el envelope en
// stdout. Errores no fatales van a stderr y sale 0 para no romper la lectura.
func runPrecheck() {
	hookMode := false
	for _, arg := range os.Args[2:] {
		if arg == "--hook-mode" {
			hookMode = true
		}
	}
	if !hookMode {
		return
	}

	root := workspaceDir()
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi precheck: memoria no disponible: %v\n", err)
		os.Exit(0)
	}
	defer engine.Close()

	if out := precheckOutput(engine, root, os.Stdin); out != "" {
		fmt.Println(out)
	}
}
