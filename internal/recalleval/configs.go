package recalleval

import (
	"musubi/internal/config"
	"musubi/internal/memory"
)

// configs.go declara las configuraciones que el banco evalúa, DERIVADAS de config.Default().
//
// POR QUÉ ESTÁN ACÁ Y NO EN UN _test.go. Vivían como `var lexicalConfig`/`hybridConfig` en
// harness_test.go, con los valores escritos a mano. Eso tenía dos consecuencias, las dos medidas:
//
//  1. NO ERAN EXPORTABLES, así que nadie fuera del paquete podía correr el banco con la
//     configuración que el banco defiende, y ningún test podía compararlas contra config.Default().
//  2. DIVERGÍAN EN SILENCIO. hybridConfig dejaba VectorFloor y MMRLambda en el cero de Go mientras
//     los defaults de producción son 0.30 y 0.75. O sea que el gate de CI defendía un ranker que
//     nadie corre: sin piso de coseno y con MMR apagado. Nada avisaba, porque no había nada que
//     comparara los dos lados.
//
// La regla que dejan escrita: un banco que fija a mano los valores que su sistema ya tiene
// configurados no mide el sistema, mide una copia que se va a quedar vieja el día que alguien
// cambie el yaml.
//
// LO QUE SIGUE SIENDO EXPLÍCITO, A PROPÓSITO: las señales de pool (Stemming, Cooccurrence,
// GraphCentrality) se nombran una por una en vez de heredarse, porque son los EJES del
// experimento. Un brazo tiene que poder apagarlas sin que el default las vuelva a encender.

// OptsDeProduccion arma las RecallOptions con los valores que config.Default() declara para el
// recall. Es la única fuente: si mañana cambia el yaml, el banco cambia con él.
func OptsDeProduccion() memory.RecallOptions {
	m := config.Default().Memory
	return memory.RecallOptions{
		Stemming:        m.RecallStemming,
		Cooccurrence:    m.RecallCooccurrence,
		GraphCentrality: m.RecallGraphCentrality,
		VectorFloor:     m.VectorFloor,
		MMRLambda:       m.MMRLambda,
		CandidatePool:   m.CandidatePool,
		TokenBudget:     m.RecallTokenBudget,
	}
}

// ConfigLexica es el brazo model-free: las tres señales de pool encendidas y NINGUNA señal
// vectorial. Es la línea base contra la que se mide todo lo demás.
func ConfigLexica() Config {
	o := OptsDeProduccion()
	o.VectorFloor = 0 // sin pool vectorial no hay piso que aplicar
	o.MMRLambda = 0   // MMR se mide aparte: acá contaminaría el contraste léxico/vectorial
	return Config{Name: "lexical", Opts: o}
}

// ConfigHibrida enciende la señal vectorial SOBRE la línea base, conservando el piso de coseno de
// producción. Es el brazo que responde "¿cuánto suma la semántica?".
func ConfigHibrida() Config {
	o := OptsDeProduccion()
	o.MMRLambda = 0
	return Config{Name: "hybrid", Opts: o, UseVector: true}
}

// ConfigProduccion es TODO lo que producción declara encendido, MMR incluido. Existe porque es la
// única que responde "¿cómo se comporta el sistema tal como está configurado?", que es una pregunta
// distinta de "¿cuánto suma cada señal?".
func ConfigProduccion() Config {
	return Config{Name: "produccion", Opts: OptsDeProduccion(), UseVector: true}
}

// ConfigTurno es el ranker del HOOK por turno (UserPromptSubmit), donde ocurre casi todo el recall
// del sistema: las opciones salen de memory.OpcionesDeRecallDelTurno —la misma función que llama
// buildTurnRecall— y el banco corre con SU pool (PoolDelTurno), no con el corpus entero.
//
// POR QUÉ NO SE ARMA DESDE OptsDeProduccion. Esas son las opciones de la tool musubi_recall, y el
// hook corre otras: RankedFTS encendido (filtra stopwords) y el pool clavado en 50. Medir el hook
// con las de la tool es medir un ranker que el hook no corre, y los gates que decidan encender algo
// en el hook (el corrector, el vector del turno) se tienen que medir contra éste.
//
// EL MOTOR DEL HOOK, NO SÓLO SUS OPCIONES. El hook de hoy corre sin embebedor: la guarda de
// latencia (embedderCaroDeConstruir, cmd/musubi/embed.go) no le deja construirlo cuando la tabla
// está presente, así que no hay vector de consulta y el motor queda sin procedencia de vectores.
// Eso apaga, además del pool vectorial, a MMR: con MMRLambda 0,75 en las opciones, el hook no
// diversifica. SinEmbebedor pone al motor del banco en ese mismo estado (ver Config.SinEmbebedor).
// El día que el hook construya su embebedor (ola2/vector-en-el-turno), el brazo que lo mide es
// ConfigTurnoHibrido.
func ConfigTurno() Config {
	return ConfigTurnoCon(config.Default().Memory)
}

// ConfigTurnoCon es ConfigTurno para la config de un proyecto (su yaml) en vez de la de fábrica: el
// mismo motor sin embebedor y el mismo pool, con las opciones que la fuente única traduce de m.
func ConfigTurnoCon(m config.MemoryConfig) Config {
	return Config{
		Name:         "turno",
		Opts:         memory.OpcionesDeRecallDelTurno(m, memory.AlcanceDelTurno{}),
		PoolDelTurno: true,
		SinEmbebedor: true,
	}
}

// ConfigTurnoHibrido es el hook con su embebedor construido, que es lo que evalúa
// ola2/vector-en-el-turno: el prompt lleva vector y el motor lee con la procedencia del embebedor,
// así que, con las opciones de fábrica, TAMBIÉN corre MMR. Encender el vector en el turno enciende
// las dos cosas juntas; el banco que decide tiene que medirlas juntas (y por separado, bajando
// MMRLambda a 1 en un brazo aparte).
func ConfigTurnoHibrido() Config {
	c := ConfigTurno()
	c.Name, c.SinEmbebedor, c.UseVector = "turno-hibrido", false, true
	return c
}
