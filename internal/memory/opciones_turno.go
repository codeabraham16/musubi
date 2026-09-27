package memory

import "musubi/internal/config"

// AlcanceDelTurno es el pedazo de las opciones del turno que decide DE QUÉ PROYECTOS trae memoria
// el hook. Vive aparte de config.MemoryConfig porque no sale del yaml: lo resuelve quien corre el
// hook (la carpeta en la que está, la credencial).
//
// SU VALOR CERO ES EL COMPORTAMIENTO DE HOY, y es un contrato con los PR que vienen: ProjectScope
// vacío y Federate en false dan el recall federado histórico (scope.go: «Federate o ProjectID vacío
// ⇒ sin filtro»), bit a bit. Nadie los llena y el hook sigue viendo todo el acervo, que es lo que un
// workspace local quiere ver. El frente «proyecto» de la ola 2 decidió NO llenarlos ni agregar un
// tope de ajenos: marca en la viñeta de qué proyecto es cada nota (marcaDeProyecto, cmd/musubi) y el
// recall sigue federado.
type AlcanceDelTurno struct {
	ProjectScope string
	Federate     bool
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
		// El alcance, en su valor cero, es el recall federado de hoy. Ver AlcanceDelTurno.
		ProjectScope: alcance.ProjectScope,
		Federate:     alcance.Federate,
	}
}
