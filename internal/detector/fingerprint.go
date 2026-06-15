package detector

import (
	"sort"
	"strings"
)

// fingerprintSep separa los tokens dentro de una huella de stack.
const fingerprintSep = ";"

// StackTokens devuelve el conjunto ORDENADO y SIN DUPLICADOS de tokens que
// describen un stack. Cada ecosistema aporta su token (ej. "Go") y cada
// framework aporta un token calificado (ej. "Node.js:react"). El resultado es
// determinista e independiente del orden de entrada, lo que lo hace apto para
// comparar stacks entre sesiones (detección de drift) de forma model-free.
func StackTokens(results []StackResult) []string {
	set := make(map[string]struct{})
	for _, r := range results {
		if r.Ecosystem == "" {
			continue
		}
		set[r.Ecosystem] = struct{}{}
		for _, f := range r.Frameworks {
			if f == "" {
				continue
			}
			set[r.Ecosystem+":"+f] = struct{}{}
		}
	}
	tokens := make([]string, 0, len(set))
	for tk := range set {
		tokens = append(tokens, tk)
	}
	sort.Strings(tokens)
	return tokens
}

// StackFingerprint devuelve una huella estable del stack: los tokens ordenados
// unidos por fingerprintSep. Dos stacks con los mismos ecosistemas y frameworks
// (en cualquier orden) producen la misma huella. Un stack vacío da "".
func StackFingerprint(results []StackResult) string {
	return strings.Join(StackTokens(results), fingerprintSep)
}

// StackDelta devuelve los tokens del stack ACTUAL que no estaban presentes en la
// huella guardada (storedFingerprint). Es lo que permite re-generar skills solo
// para lo nuevo cuando el stack crece (ej. se agregó React a un proyecto Go).
// Si no hubo cambios, devuelve un slice vacío.
func StackDelta(storedFingerprint string, current []StackResult) []string {
	stored := make(map[string]struct{})
	if storedFingerprint != "" {
		for _, tk := range strings.Split(storedFingerprint, fingerprintSep) {
			stored[tk] = struct{}{}
		}
	}
	var delta []string
	for _, tk := range StackTokens(current) {
		if _, ok := stored[tk]; !ok {
			delta = append(delta, tk)
		}
	}
	return delta
}
