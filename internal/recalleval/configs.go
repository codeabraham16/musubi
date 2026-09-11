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
