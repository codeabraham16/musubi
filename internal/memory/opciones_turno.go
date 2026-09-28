package memory

import (
	"strings"

	"musubi/internal/config"
	"musubi/internal/logx"
)

// AlcanceDelTurno es el pedazo de las opciones del turno que decide DE QUÉ PROYECTOS trae memoria
// el hook. Vive aparte de config.MemoryConfig porque no sale sólo del yaml: lo arma
// AlcanceDelTurnoSegun con loop.recall_otros_proyectos, loop.recall_otros_max y el proyecto del
// workspace, que es algo que sabe quien corre el hook (resolveProjectID), no la config.
//
// SU VALOR CERO ES EL RECALL FEDERADO DE SIEMPRE, bit a bit: ProjectScope vacío y Federate en false
// no filtran nada (scope.go: «Federate o ProjectID vacío ⇒ sin filtro») y TopeOtrosProyectos en 0 no
// reparte nada. Es lo que devuelve la traducción en «mezclado» o sin proyecto propio, y lo que corren
// los callers que no conocen el alcance: la tool musubi_recall, musubi_ask y el central no lo tocan.
//
// Los tres modos, en estos campos:
//   - «aparte»:   ProjectScope = propio y TopeOtrosProyectos = el tope (> 0).
//   - «aislado»:  ProjectScope = propio y TopeOtrosProyectos = 0.
//   - «mezclado»: el valor cero.
//
// EL VOCABULARIO DEL CORRECTOR DE TIPEO sale de lo que este alcance deja ENTRAR al ranking, porque
// el alcance del corrector es el filtro DURO que el caller le aplica a Recall, no el reparto:
//   - ProjectScope vacío, o Federate en true: vocabulario FEDERADO, el de todo el acervo.
//   - ProjectScope puesto, Federate en false y TopeOtrosProyectos > 0 («aparte»): también FEDERADO.
//     El tope reparte el presupuesto al empaquetar y no le cierra la puerta a lo ajeno: un prompt
//     sobre otro proyecto tiene que poder corregirse con las palabras de ese proyecto, porque si nada
//     propio viene al caso es lo ajeno lo que llena el bloque.
//   - ProjectScope puesto, Federate en false y TopeOtrosProyectos == 0 («aislado»): el vocabulario
//     del proyecto propio y de lo sin atribuir, que es exactamente lo que ese recall deja ver.
//
// La regla no tiene función ni campo propio a propósito: la aplica quien llama al corrector, con
// estos tres campos a la vista, y una función sin ese lector sería una guarda desconectada.
type AlcanceDelTurno struct {
	ProjectScope string
	Federate     bool
	// TopeOtrosProyectos es cuántas notas de otro proyecto entran como mucho cuando algo propio
	// viene al caso. Ver RecallOptions.TopeOtrosProyectos.
	TopeOtrosProyectos int
}

// AlcanceDelTurnoSegun traduce loop.recall_otros_proyectos (modo) y loop.recall_otros_max (tope) al
// alcance del recall para el proyecto propio. Es el ÚNICO lugar donde el modo se vuelve alcance: lo
// llaman el hook del turno (buildTurnRecall), el de arranque (detectOutput, con ScopeDelPriming) y el
// banco (recalleval.ConfigTurnoConAlcance), así que los tres corren lo mismo.
//
//   - propio vacío ⇒ el valor cero, sea cual sea el modo: sin proyecto propio no hay nada ajeno
//     (MismoProyecto), y acotar a "" no significa nada.
//   - modo vacío ⇒ el default. Desconocido ⇒ el default, con un aviso: un error de tipeo en el yaml
//     no puede dejar al hook sin memoria ni devolverle en silencio la mezcla que se quiso sacar.
//     Mayúsculas y espacios no cuentan, como en brevity_mode.
//   - tope 0 ⇒ el default. Negativo ⇒ sin notas ajenas, o sea «aislado»: la misma regla que el resto
//     de los numéricos de loop, donde el negativo apaga.
func AlcanceDelTurnoSegun(modo string, tope int, propio string) AlcanceDelTurno {
	if propio == "" {
		return AlcanceDelTurno{}
	}
	d := config.Default().Loop
	m := strings.ToLower(strings.TrimSpace(modo))
	switch m {
	case config.OtrosProyectosAparte, config.OtrosProyectosAislado, config.OtrosProyectosMezclado:
	case "":
		m = d.RecallOtrosProyectos
	default:
		logx.Warn("loop.recall_otros_proyectos no es un modo conocido: uso el default",
			"valor", modo, "default", d.RecallOtrosProyectos, "modos", "aparte | aislado | mezclado")
		m = d.RecallOtrosProyectos
	}
	switch m {
	case config.OtrosProyectosMezclado:
		return AlcanceDelTurno{}
	case config.OtrosProyectosAislado:
		return AlcanceDelTurno{ProjectScope: propio}
	}
	if tope == 0 {
		tope = d.RecallOtrosMax
	}
	if tope < 0 {
		return AlcanceDelTurno{ProjectScope: propio}
	}
	return AlcanceDelTurno{ProjectScope: propio, TopeOtrosProyectos: tope}
}

// ScopeDelPriming es el alcance del priming de arranque (PrimeContextCtx): en «aparte» y en
// «aislado», lo propio y lo sin atribuir; en «mezclado», o sin proyecto propio, todo el acervo.
//
// En «aparte» el arranque NO lleva el cupo de ajenas. El priming no tiene consulta, así que nada
// ajeno puede «venir al caso»: lo que trae es contexto general DEL PROYECTO. Hoy es una guarda más que
// una mejora (medido en davantis-1: 0 notas ajenas de 462 en el priming de 33 sesiones que sólo
// arrancaron); lo que fija es que siga así el día que la saliencia de lo ajeno crezca. La excepción,
// cuando lo propio no trae NADA, es PrimingDeRespaldo.
func (a AlcanceDelTurno) ScopeDelPriming() ProjectScope {
	return ProjectScope{ProjectID: a.ProjectScope, Federate: a.Federate}
}

// PrimingDeRespaldo dice si el arranque, cuando el priming acotado vuelve VACÍO, lo repite con todo
// el acervo (y el hook marca cada nota con su proyecto). Es la excepción del turno llevada al
// arranque —«salvo que no haya nada propio que venga al caso»— y vale sólo en «aparte»: en «aislado»
// lo ajeno no entra nunca, y en «mezclado» el priming ya es de todo el acervo.
//
// El caso es un repo cuya memoria entera lleva el sello de otro proyecto: una base copiada, una
// carpeta renombrada, un repo sin project_id que sólo tiene lo que bajó. Sin esto arrancaba sin
// memoria, cuando antes del modo le llegaba todo.
func (a AlcanceDelTurno) PrimingDeRespaldo() bool {
	return a.ProjectScope != "" && !a.Federate && a.TopeOtrosProyectos > 0
}

// OpcionesDeRecallDelTurno arma las RecallOptions del recall por turno (el hook UserPromptSubmit),
// la superficie donde ocurre casi todo el recall del sistema. Es la ÚNICA fuente: la usan
// buildTurnRecall (cmd/musubi/turn.go) y el banco (recalleval.ConfigTurno).
//
// POR QUÉ HACE FALTA UNA FUNCIÓN Y NO UN LITERAL EN CADA LADO. El hook armaba sus opciones a mano
// (RankedFTS=true) y el banco armaba las suyas desde config.Default() SIN RankedFTS y con el pool
// subido al corpus entero. O sea que el banco medía un ranker que el hook no corre: el mismo
// defecto que configs.go ya había matado una vez para VectorFloor y MMRLambda. Con dos literales,
// el día que uno cambia el otro no se entera.
//
// TokenBudget NO se fija acá: el hook lo toma de loop.recall_budget y el banco lo sube para medir
// orden y no empaquetado. Cada caller lo pone.
//
// CandidatePool y GistMaxTokens van EXPLÍCITOS y con los valores que el hook corrió siempre (dejaba
// los dos en el cero de Go y Recall los normalizaba a defaultCandidatePool y defaultGistMaxTokens).
// No salen de memCfg a propósito: memory.candidate_pool y memory.gist_max_tokens gobiernan la tool
// musubi_recall y nunca gobernaron el hook, y cambiar eso sería cambiar el ranking, que no es lo que
// hace esta función. Lo que sí cambia es que el número queda escrito, y el banco lo lee de acá.
func OpcionesDeRecallDelTurno(memCfg config.MemoryConfig, alcance AlcanceDelTurno) RecallOptions {
	return RecallOptions{
		NoBump:        true, // el hook sólo lee: un turno no refuerza la memoria que inyecta
		RankedFTS:     true, // filtrar stopwords: es la superficie más caliente, evita ruido
		CandidatePool: defaultCandidatePool,
		GistMaxTokens: defaultGistMaxTokens,
		// Los toggles semánticos model-free, los mismos que ya usa la tool musubi_recall.
		Stemming:        memCfg.RecallStemming,
		Cooccurrence:    memCfg.RecallCooccurrence,
		GraphCentrality: memCfg.RecallGraphCentrality,
		// Los dos que deciden calidad. En el cero de Go, MMRLambda 0 apaga MMR y VectorFloor 0 deja
		// entrar al RRF cualquier vecino vectorial: ver TestBuildTurnRecallPasaLaConfigDeProduccion.
		VectorFloor: memCfg.VectorFloor,
		MMRLambda:   memCfg.MMRLambda,
		// El alcance, en su valor cero, es el recall federado de siempre. Ver AlcanceDelTurno.
		ProjectScope:       alcance.ProjectScope,
		Federate:           alcance.Federate,
		TopeOtrosProyectos: alcance.TopeOtrosProyectos,
	}
}
