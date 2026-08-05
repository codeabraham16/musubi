package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"musubi/internal/cognition"
)

// toolCognitionStats expone los contadores de la cognición (F5).
//
// POR QUÉ UNA TOOL MCP Y NO `musubi doctor`. Parecía obvio ponerlo en el doctor, que ya inspecciona
// la config de cognición. No funciona: los contadores viven EN MEMORIA del proceso que atiende, y
// el CLI es OTRO PROCESO. `musubi doctor` construiría su propio provider con el caché vacío y
// reportaría ceros para siempre — un número convincente y falso, que es peor que no tener número.
//
// D5 — ACÁ NUNCA VIAJA UN SECRETO: se devuelven conteos y TIPOS ('aws-access-key'), jamás valores,
// prompts ni respuestas.
//
// D8 — Es READ-ONLY: leer no resetea nada. Un contador que se vacía al leerlo hace que dos lectores
// se roben los datos entre sí.
func (s *McpServer) toolCognitionStats(_ json.RawMessage) (interface{}, *RpcError) {
	if !cognition.Enabled(s.cognition) {
		// No es un error: es una respuesta legítima. El pilar es opt-in y estar apagado es un
		// estado normal, no una falla.
		return jsonResult(map[string]interface{}{
			"enabled": false,
			"summary": "La cognición está apagada (cognition.provider vacío o 'none'): no hay nada que medir.",
		})
	}

	st := cognition.Stats(s.cognition)

	// Tasa de acierto del caché. Sin llamadas todavía se informa null y NO 0: cero por ciento y
	// "no hay datos" son cosas distintas, y confundirlas hace tomar decisiones sobre humo.
	var hitRate interface{}
	if total := st.CacheHits + st.CacheMisses; total > 0 {
		hitRate = float64(st.CacheHits) / float64(total)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Motor: %s\n", s.cognition.Name())
	fmt.Fprintf(&b, "Caché: %d hits, %d misses", st.CacheHits, st.CacheMisses)
	if hr, ok := hitRate.(float64); ok {
		fmt.Fprintf(&b, " (%.0f%% de acierto)", hr*100)
	} else {
		b.WriteString(" (sin llamadas todavía)")
	}
	fmt.Fprintf(&b, ", %d entradas guardadas\n", st.CacheSize)

	fmt.Fprintf(&b, "Portero: %d llamadas, %d taparon algo, %d bloqueadas por política",
		st.GatewayCalls, st.GatewayScrubbed, st.GatewayBlocked)
	if st.GatewayFailed > 0 {
		// Debería ser 0 siempre. Si no lo es hay un bug en la redacción, y conviene que grite.
		fmt.Fprintf(&b, ", %d FALLAS DEL PORTERO (revisar: no deberían existir)", st.GatewayFailed)
	}
	b.WriteString("\n")
	if len(st.GatewayTypes) > 0 {
		tipos := make([]string, 0, len(st.GatewayTypes))
		for t := range st.GatewayTypes {
			tipos = append(tipos, t)
		}
		sort.Strings(tipos) // orden estable: si no, dos lecturas seguidas parecen distintas
		partes := make([]string, 0, len(tipos))
		for _, t := range tipos {
			partes = append(partes, fmt.Sprintf("%s×%d", t, st.GatewayTypes[t]))
		}
		fmt.Fprintf(&b, "Tipos de secreto tapados: %s\n", strings.Join(partes, ", "))
	}

	if st.RouterEscalations > 0 || st.RouterExhausted > 0 || len(st.OpenCircuits) > 0 {
		fmt.Fprintf(&b, "Router: %d escaladas, %d veces se agotó la flota", st.RouterEscalations, st.RouterExhausted)
		if len(st.OpenCircuits) > 0 {
			sort.Strings(st.OpenCircuits)
			fmt.Fprintf(&b, "; circuitos ABIERTOS ahora: %s", strings.Join(st.OpenCircuits, ", "))
		}
		b.WriteString("\n")
	}

	body, _ := json.Marshal(map[string]interface{}{
		"enabled":        true,
		"model":          s.cognition.Name(),
		"cache_hit_rate": hitRate,
		"stats":          st,
	})
	return textResult(b.String() + string(body)), nil
}
