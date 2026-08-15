package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"musubi/internal/memory"
)

// precompact.go implementa 'musubi precompact --hook-mode': el hook PreCompact de Claude Code, que
// dispara JUSTO ANTES de que la conversación se compacte.
//
// POR QUÉ ACÁ Y NO EN SessionStart. Musubi ya reacciona a la compactación, pero DESPUÉS: el
// SessionStart con source=compact trata el contexto como fresco y vuelve a primear (ver detect.go).
// Para entonces el resumen ya se escribió y lo que se perdió, se perdió. La compactación es el
// momento de MAYOR pérdida de memoria de toda la sesión —el modelo decide solo qué tirar, y lo
// primero que se diluye son las decisiones y su porqué— así que el aviso tiene que llegar antes.
//
// ⚠️ EL BLOQUE MANDA A `musubi_propose_observation`, NUNCA a `musubi_save_observation`, y eso NO es
// un detalle de estilo: es la condición que hace aceptable a este hook. Lo que el agente escribe acá
// es una síntesis SUYA, no algo que la persona dijo. `save_observation` sella procedencia `human`, o
// sea que guardaría una invención del modelo como si fuera testimonio, y contamina el libro mayor
// justo donde más caro sale. `propose` la deja en CUARENTENA con sello llm:<modelo>, invisible al
// recall hasta que alguien la corrobore. La cuarentena existe exactamente para este caso.
//
// El bloque se contabiliza en el ledger como una superficie más. Es raro (una compactación cada
// tanto) y chico, pero el principio del repo es que ninguna superficie inyectada quede sin medir.

// surfacePrecompact es la superficie del ledger a la que se imputa este bloque.
const surfacePrecompact = "precompact_capture"

// precompactInput es el subconjunto del JSON de stdin de PreCompact que usamos. Claude Code manda
// también `trigger` (auto|manual) y, si es manual, las instrucciones del usuario; no los usamos: el
// aviso es el mismo en los dos casos, porque lo que se pierde es lo mismo.
type precompactInput struct {
	SessionID string `json:"session_id"`
}

// readPrecompactInput tolera entrada vacía o inválida (devuelve campos vacíos), igual que el resto
// de los hooks: un stdin raro no puede tumbar la sesión del usuario.
func readPrecompactInput(stdin io.Reader) precompactInput {
	var in precompactInput
	_ = json.NewDecoder(stdin).Decode(&in)
	in.SessionID = strings.TrimSpace(in.SessionID)
	return in
}

// buildPrecompactNudge es el texto que se inyecta antes de compactar. Estático y acotado: el
// criterio de qué merece guardarse lo pone el agente, que es el que estuvo en la conversación.
//
// El último párrafo no es relleno: sin él, este hook se convierte en una fábrica de síntesis vacías
// en cuarentena, y cada una es un ítem que después alguien tiene que arbitrar a mano. La cola de
// conflictos es un recurso escaso.
func buildPrecompactNudge() string {
	return `[Musubi — se va a compactar] Este tramo está por resumirse, y el resumen lo escribe el modelo: las decisiones concretas y su porqué son lo primero que se diluye. Es el momento de bajar lo durable, antes de perderlo.

GUARDALO CON musubi_propose_observation, NO con musubi_save_observation. Lo que escribas acá es una síntesis TUYA, no algo que la persona dijo: propose la deja en cuarentena con procedencia llm, invisible al recall hasta que alguien la corrobore. Guardarla con save_observation la sellaría como testimonio humano y sería mentir sobre de dónde salió.

Sólo esto, y sólo si pasó de verdad:
- Decisiones tomadas en este tramo, con su porqué (y lo que se descartó, que es lo que nadie vuelve a escribir).
- Gotchas o hallazgos no obvios que costaron encontrar.
- Estado del trabajo: qué quedó a medias y cuál es el próximo paso.

Si en este tramo no se decidió ni se aprendió nada, NO guardes nada. Una síntesis vacía en cuarentena es ruido que después hay que arbitrar a mano.`
}

// precompactOutput arma el envelope del hook PreCompact, contabilizando el bloque en el ledger.
func precompactOutput(store ledgerStore, stdin io.Reader) string {
	in := readPrecompactInput(stdin)
	return assembleAccounted(store, "PreCompact", in.SessionID, []accountedBlock{
		{surface: surfacePrecompact, text: buildPrecompactNudge()},
	})
}

// runPrecompact ejecuta el hook. Degrada con gracia: si la memoria no abre, igual inyecta el aviso
// (las tools son MCP y siguen disponibles), sólo que sin contabilizarlo.
func runPrecompact() {
	root := workspaceDir()
	var store ledgerStore
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "musubi precompact: memoria no disponible, se inyecta sin contabilizar: %v\n", err)
	} else {
		defer engine.Close()
		store = engine
	}
	if out := precompactOutput(store, os.Stdin); out != "" {
		fmt.Println(out)
	}
}
