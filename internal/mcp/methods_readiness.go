package mcp

// methods_readiness.go expone el ESTADO DE MADUREZ MEDIDO (F3 · doctrina de loop-engineering).
//
// Las evaluaciones de madurez que se usan en la industria son cuestionarios: alguien marca
// casilleros sobre su propio equipo y sale un nivel. Miden la intención, no lo que pasó, y por eso
// el equipo que dice revisar todo y el que efectivamente revisa sacan el mismo puntaje. Musubi
// tiene lo que un cuestionario no tiene —un ledger de invocaciones, una cola de contradicciones, un
// grafo de código y el estado de su memoria— así que puede responder la misma pregunta sin
// preguntarle nada a nadie.
//
// SUS CONSUMIDORES ESTÁN DECLARADOS, que es lo que el repo exige antes de sumar una tool: (1) la
// cabina —el CRM y el tablero del cuerpo— que muestra el estado de cada proyecto del cerebro
// central y necesita un número comparable entre ellos; y (2) el agente que onboardea un proyecto
// nuevo, que hoy no tiene forma de contestar «¿esto ya sirve?» sin revisar cinco tools a mano.
// Es read-only, así que la cabina (write=none) puede llamarla.

import (
	"context"
	"encoding/json"
	"fmt"

	"musubi/internal/memory"
)

// readinessReader lo implementa el motor real. Interfaz y no *memory.DbEngine por el mismo motivo
// que el ledger: si el backend de turno no lo sabe calcular, la tool lo dice en vez de romper.
type readinessReader interface {
	Readiness(ctx context.Context, days int) (memory.ReadinessReport, error)
}

// readinessGrafoFrescuraMax es el tope de desactualización que la dimensión `grafo` tolera antes de
// declarar que el índice no está al día. Es una CONVENCIÓN, igual que los umbrales de readiness.go,
// pero está DERIVADA y no elegida a ojo: sobre los últimos 300 commits de este repo, un commit toca
// 4 archivos en la mediana (0,29 % de los 1.395 que tiene el grafo), 15 en el p90, 22 en el p95 y 65
// en el máximo medido (4,66 %). El 5 % cubre el peor commit real sin dar falso positivo, y es el
// mismo tope que readiness ya usa para errores y rechazos, así que no suma una convención nueva.
//
// Tiene que ser proporcional y no binario por una razón concreta: con tolerancia cero, crear un
// archivo y no indexarlo todavía tira abajo la dimensión, y un chequeo que está en rojo durante el
// trabajo normal se aprende a ignorar.
const readinessGrafoFrescuraMax = 0.05

// toolReadiness responde qué tan lista está esta instalación, según lo que hizo.
func (s *McpServer) toolReadiness(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Days int `json:"days"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
		}
	}

	lector, ok := s.engine.(readinessReader)
	if !ok {
		return textResult("El motor de memoria de esta instancia no sabe calcular el estado de madurez."), nil
	}
	// scopedCtx acota TODO al proyecto de la credencial (Track 17/18). Sin esto el puntaje de un
	// proyecto se calcularía con el uso, los conflictos y el grafo de los demás — y en un cerebro
	// central eso además filtra el patrón de trabajo ajeno, que es información de negocio.
	scoped := s.scopedCtx(ctx)
	rep, err := lector.Readiness(scoped, args.Days)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo calcular el estado de madurez: %v", err)
	}
	s.enriquecerGrafo(&rep, scoped)
	return jsonResult(rep)
}

// enriquecerGrafo le agrega a la dimensión `grafo` lo único que el motor de memoria NO PUEDE medir:
// si el índice todavía coincide con el disco.
//
// QUÉ ARREGLA: la dimensión se titula «El código está indexado» y lo verificaba con dos chequeos de
// VOLUMEN —nodos > 0 y aristas > 0—, así que puntuaba 1.0 con un solo nodo y una sola arista. Los
// tres indicadores de si el índice SIRVE (stale, fantasmas, sin indexar) los calculaba
// graphFreshness y los leía ÚNICAMENTE musubi_map, una tool que hay que llamar a propósito. Medido
// el 2026-09-05 en este mismo repo: 213 de 798 `.go` sin un solo nodo, `internal/fleet` entero entre
// ellos, y la dimensión daba 1.0 igual. Peor: el comentario de methods_codegraph.go:371 justifica su
// fail-safe «ante la duda, PRESENTE» diciendo que equivocarse hacia presente es RUIDOSO porque
// graphFreshness reporta fantasmas en masa — un argumento apoyado en una alarma que nadie escuchaba.
//
// POR QUÉ ACÁ Y NO EN dimensionGrafo: `dimensionGrafo` es método de *memory.DbEngine y sólo ve la
// base, así que sabe contar nodos pero no comparar un fingerprint contra el archivo que lo originó.
// `graphFreshness` es método de *McpServer justamente porque necesita el disco (projectPath, el
// árbol, FileFingerprint) y el scope de la credencial. La desconexión entre las dos es una FRONTERA
// DE CAPAS, no un olvido: conectarlas desde el engine obligaría a `memory` a depender de un concepto
// de `mcp`. toolReadiness ya está del lado correcto, y hasta acá era un paso-a-través.
func (s *McpServer) enriquecerGrafo(rep *memory.ReadinessReport, scoped context.Context) {
	var dim *memory.ReadinessDimension
	for i := range rep.Dimensions {
		if rep.Dimensions[i].Key == "grafo" {
			dim = &rep.Dimensions[i]
			break
		}
	}
	if dim == nil {
		return // un backend que no calcula la dimensión no puede ganar un chequeo
	}
	if dim.Evidence == nil {
		dim.Evidence = map[string]interface{}{}
	}

	// El grafo nunca se indexó: Observed=false, y ahí la dimensión ya vale 0 por la regla de oro de
	// readiness («una señal no observada puntúa cero»). AGREGARLE UN CHEQUEO LA ROMPERÍA: en un
	// proyecto sin código indexable los tres contadores dan cero, el chequeo pasaría, y una
	// dimensión SIN DATO terminaría puntuando 1.0 — exactamente el premio por no medir que ese
	// archivo existe para impedir. Y de paso se ahorra el costo: medir frescura es leer y hashear
	// el árbol entero para un veredicto que ya está cerrado.
	if !dim.Observed {
		return
	}

	// La frescura por fingerprint NO APLICA donde el árbol está fuera de alcance: en el cerebro
	// central el grafo es federado y sus archivos no están en este disco (todo saldría fantasma).
	// Se declara y se sale SIN agregar el chequeo. Agregarlo pasaría gratis, y un chequeo que pasa
	// porque no se midió nada sube el puntaje POR NO MIRAR; `Observed` tampoco se toca, porque la
	// dimensión sí fue observada —hay nodos y aristas— y lo que falta es un sub-aspecto.
	if s.arbolFueraDeAlcance() {
		dim.Evidence["frescura_no_medida"] = "el árbol de código no está en este disco " +
			"(grafo federado): comparar fingerprints marcaría todo como fantasma"
		return
	}

	stale, ghosts, missing, enGrafo, enDisco := s.graphFreshness(scoped)
	dim.Evidence["stale"] = stale
	dim.Evidence["ghosts"] = ghosts
	dim.Evidence["missing"] = missing
	dim.Evidence["archivos_en_grafo"] = enGrafo
	dim.Evidence["archivos_en_disco"] = enDisco

	// LA SEGUNDA PUERTA AL MISMO PREMIO, y la guarda de `Observed` no la cubre: que no haya NADA contra
	// qué medir. `GraphStatsCtx` cuenta TODOS los nodos, pero `GraphFileFingerprintsCtx` filtra
	// `WHERE path != ''`, así que un grafo cuyos nodos no declaran archivo da nodos > 0 —`Observed`
	// queda en true y la guarda de arriba deja pasar— con CERO archivos para comparar. No es
	// hipotético: este repo tiene hoy un nodo con `path = ''`, encontrado midiendo el costo de esta
	// misma función. Y el mismo estado llega por otra vía: si la consulta de fingerprints falla,
	// graphFreshness devuelve todo en cero, los dos denominadores incluidos. Con los dos en cero las
	// tres proporciones son 0 POR CONSTRUCCIÓN —`proporcion` lo hace a propósito, para no publicar un
	// NaN— y el chequeo pasaría sin haber mirado un archivo. Se declara y se sale, igual que la rama de
	// alcance; la evidencia cruda ya quedó arriba, así que el diagnóstico no se pierde.
	if enGrafo == 0 && enDisco == 0 {
		dim.Evidence["frescura_no_medida"] = "no hay un solo archivo contra el que medir: ni " +
			"archivos en el grafo (un nodo sin `path` no cuenta) ni indexables en disco"
		return
	}

	// Los dos denominadores son DISTINTOS: stale y fantasmas se miden sobre los archivos que están
	// en el grafo; los que faltan, sobre los indexables del disco. El chequeo mira el PEOR de los
	// tres, no el promedio: un eje podrido no se compensa con dos sanos.
	pStale := proporcion(stale, enGrafo)
	pGhosts := proporcion(ghosts, enGrafo)
	pMissing := proporcion(missing, enDisco)
	peor := pStale
	if pGhosts > peor {
		peor = pGhosts
	}
	if pMissing > peor {
		peor = pMissing
	}

	// UN chequeo compuesto y no tres, porque los tres ejes se curan con EL MISMO ACTO:
	// aplicarIncremental re-hashea los stale, indexa los que faltaban y poda los fantasmas con
	// PruneGraphFilesFrom. Tres chequeos sugerirían tres remedios donde hay uno solo, y el
	// diagnóstico no se pierde: los tres números van en el Why y crudos en la evidencia.
	dim.Checks = append(dim.Checks, memory.ReadinessCheck{
		Name: "el índice está al día",
		Pass: peor <= readinessGrafoFrescuraMax,
		Why: fmt.Sprintf("%d/%d desactualizados (%.1f %%) · %d/%d fantasmas (%.1f %%) · "+
			"%d/%d sin indexar (%.1f %%) · tope %.0f %% · se cura con musubi_codegraph_index",
			stale, enGrafo, pStale*100, ghosts, enGrafo, pGhosts*100,
			missing, enDisco, pMissing*100, readinessGrafoFrescuraMax*100),
	})

	// Sin esto el chequeo queda DE ADORNO: el engine ya calculó los puntajes sobre los dos chequeos
	// viejos, así que la dimensión seguiría en 1.0 con un chequeo nuevo en rojo a la vista. Recalcular
	// vive en `memory` para que la fórmula no se duplique (ver su comentario).
	rep.Recalcular()
}

// proporcion evita que un denominador en cero se lea como podredumbre. Si no hay archivos en el
// grafo no puede haber stale ni fantasmas, y si no hay indexables en disco no puede faltar ninguno:
// en los dos casos el numerador es 0 POR CONSTRUCCIÓN, así que la proporción honesta es 0 y no un
// NaN —que al compararse contra el umbral da false y pondría el chequeo en rojo sin evidencia.
func proporcion(n, total int) float64 {
	if total <= 0 {
		return 0
	}
	return float64(n) / float64(total)
}
