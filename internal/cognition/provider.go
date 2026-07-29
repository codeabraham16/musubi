// Package cognition es el 3er pilar de Musubi (junto a memoria y orquestación): el JUICIO
// potenciado por un LLM. Su contrato identitario, que lo hace compatible con la garantía
// model-free del libro mayor, es simple y duro:
//
//	el motor de cognición PROPONE; NUNCA escribe directo al libro mayor durable.
//
// Toda salida del LLM entra como PROPUESTA con procedencia (ver relations.source) y sólo se
// vuelve autoritativa tras corroboración determinista o confirmación del caller. Por eso el
// pilar es OPT-IN y removible: sin motor configurado (NoopProvider), Musubi se comporta
// bit-idéntico a hoy — el LLM es un acelerador, jamás el camino crítico.
//
// F0 entrega SÓLO el andamiaje (esta interfaz + el null-object + el enchufe en el server).
// Los métodos reales (extraer tripletas, juzgar pertinencia, reconciliar) llegan en F1+,
// extendiendo esta interfaz de forma aditiva.
package cognition

import (
	"context"
	"errors"
)

// ErrCognitionDisabled se devuelve cuando no hay un motor de cognición configurado. Falla de
// forma explícita en vez de degradar en silencio.
var ErrCognitionDisabled = errors.New("cognición deshabilitada: configurá cognition.provider en .musubi/config.yaml (opt-in; el default es model-free)")

// Provider es el motor intercambiable del pilar Cognición. F0 declaró sólo Name() (identidad para
// la procedencia); F3.5b agrega Ask() — la generación de texto a-demanda que consumen las tools de
// cognición síncronas (musubi_ask/synthesize). Los métodos se agregan de forma ADITIVA; el
// null-object los implementa devolviendo ErrCognitionDisabled ⇒ sin motor, el pilar sigue apagado.
type Provider interface {
	// Name identifica el motor y su modelo para estampar la procedencia de lo que proponga
	// (ej. "llm-extract:claude-...", o "none" cuando está deshabilitado).
	Name() string
	// Ask envía un prompt (system + user) al motor y devuelve la respuesta de texto. Es la
	// primitiva generativa del pilar; las tools la usan A-DEMANDA (el agente elige invocarla y
	// esperar), NO en el camino caliente de cada recall. Devuelve ErrCognitionDisabled si el
	// pilar está apagado, para que el caller degrade a la respuesta model-free sin romperse.
	Ask(ctx context.Context, system, user string) (string, error)
}

// NoopProvider es el motor por defecto: no hay cognición. Hace que el pilar nazca apagado.
type NoopProvider struct{}

// Name devuelve "none": ninguna arista se atribuye a un LLM mientras el pilar esté apagado.
func (NoopProvider) Name() string { return "none" }

// Ask falla explícito: sin motor configurado no hay generación. El caller distingue este error
// (pilar apagado) de un fallo de red y responde en consecuencia (degradar a model-free).
func (NoopProvider) Ask(context.Context, string, string) (string, error) {
	return "", ErrCognitionDisabled
}

// Enabled indica si p es un motor de cognición real (no el null-object).
func Enabled(p Provider) bool {
	_, isNoop := p.(NoopProvider)
	return !isNoop
}
