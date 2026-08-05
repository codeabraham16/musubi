package config

import (
	"fmt"
	"strings"
)

// DIAL DE POTENCIA de la cognición (F5).
//
// Un solo parámetro en vez de coordinar a mano `read_time_rerank`, `read_time_rerank_top_k` y
// `cache.ttl_seconds`. No es maquinaria nueva: es un PRESET sobre perillas que ya existen. `turbo`
// no inventa potencia — prende el juez del recall y le sube el top-K.

// Niveles del dial.
const (
	// EffortEco: el LLM sólo cuando se lo invoca explícitamente. Caché largo: mejor una respuesta
	// de ayer que una llamada.
	EffortEco = "eco"
	// EffortBalanced: el default del dial. Deja el juez del recall APAGADO a propósito — es el
	// seam de mayor riesgo (latencia en el camino caliente y rate-limit), y un default que
	// enciende el camino caliente no es "balanceado", es turbo con otro nombre.
	EffortBalanced = "balanced"
	// EffortTurbo: prende el juez del recall y acorta el caché. Quien pide turbo quiere respuestas
	// frescas y está dispuesto a pagarlas.
	EffortTurbo = "turbo"
)

// TTL del caché por nivel. Más potencia ⇒ TTL más CORTO: parece al revés y no lo es.
const (
	effortTTLEco      = 86400 // 1 día
	effortTTLBalanced = 3600  // 1 hora
	effortTTLTurbo    = 900   // 15 minutos
	effortTurboTopK   = 12
)

// NormalizeEffort valida el nivel. Vacío ⇒ vacío (sin preset). Tolera espacios y mayúsculas, pero
// un valor desconocido es ERROR y no un default silencioso: la misma regla que un `gateway.mode`
// mal escrito, que apaga el pilar entero en vez de adivinar.
func NormalizeEffort(effort string) (string, error) {
	e := strings.ToLower(strings.TrimSpace(effort))
	switch e {
	case "":
		return "", nil
	case EffortEco, EffortBalanced, EffortTurbo:
		return e, nil
	default:
		return "", fmt.Errorf("cognition.effort desconocido: %q (usá %q, %q o %q)",
			effort, EffortEco, EffortBalanced, EffortTurbo)
	}
}

// ReadTimeRerankOn resuelve el puntero: ausente ⇒ false (el juez nace apagado, invariante de F1).
func (c CognitionConfig) ReadTimeRerankOn() bool {
	return c.ReadTimeRerank != nil && *c.ReadTimeRerank
}

// ApplyEffort devuelve la config con el preset del dial aplicado.
//
// D0 — LO EXPLÍCITO LE GANA AL PRESET. El preset sólo llena lo que quedó SIN DECLARAR. Si el dial
// pisara lo escrito a mano, una config existente cambiaría de comportamiento en silencio al
// agregarle `effort`; un preset que pisa lo explícito no es un default, es una sorpresa.
//
// D1 — Sin `effort`, devuelve la config TAL CUAL. `balanced` es el default DEL DIAL, no de Musubi:
// confundirlos cambiaría el comportamiento de toda instalación existente.
//
// D3 — Es puro: no toca red, ni disco, ni reloj.
func (c CognitionConfig) ApplyEffort() (CognitionConfig, error) {
	effort, err := NormalizeEffort(c.Effort)
	if err != nil {
		return c, err
	}
	if effort == "" {
		return c, nil // D1: sin dial no se aplica nada
	}
	c.Effort = effort

	// Cada campo se toca SÓLO si sigue sin declarar. El chequeo es "está ausente", no "vale el
	// cero": por eso ReadTimeRerank es *bool y no bool.
	if c.ReadTimeRerank == nil {
		on := effort == EffortTurbo
		c.ReadTimeRerank = &on
	}
	if c.ReadTimeRerankTopK == 0 && effort == EffortTurbo {
		c.ReadTimeRerankTopK = effortTurboTopK
	}
	if c.Cache.TTLSeconds == 0 {
		switch effort {
		case EffortEco:
			c.Cache.TTLSeconds = effortTTLEco
		case EffortBalanced:
			c.Cache.TTLSeconds = effortTTLBalanced
		case EffortTurbo:
			c.Cache.TTLSeconds = effortTTLTurbo
		}
	}
	return c, nil
}
