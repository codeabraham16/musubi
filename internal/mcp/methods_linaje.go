package mcp

import (
	"context"

	"musubi/internal/logx"
	"musubi/internal/memory"
)

// methods_linaje.go cuelga la VUELTA del acervo (memory/linaje.go) de las dos superficies que sirven
// sus observaciones: musubi_memory_expand, que es la que se usa —1.493 llamadas en el central al
// 2026-09-24—, y el corpus de musubi_design, que es donde se sirve la ficha.
//
// QUEDA LATENTE, Y HAY QUE DECIRLO: medido el mismo día, expand_count sobre las fichas y los blobs de
// musubi-design es CERO. Nadie expande hoy un id del acervo, así que colgar el linaje de expand no lo
// enciende solo. Por eso el corpus de musubi_design también trae `fuentes`: es donde el agente ve la
// ficha y puede decidir ir a buscar el artículo entero. El criterio para darlo por encendido, después
// del despliegue: expand_count > 0 sobre topic_key LIKE 'ingested/%' en musubi-design.

// expandItem es una observación expandida con su linaje. Los dos van embebidos, así que el JSON queda
// plano —id, topic_key, content, created_at y, si hay, salio_de / destilado_en— y un item sin aristas
// sale byte a byte igual que antes. La respuesta sigue siendo un array: el cuerpo (musubi-body) la
// decodifica sin modo estricto y los campos nuevos le pasan de largo.
type expandItem struct {
	memory.Observation
	memory.Linaje
}

// conLinaje agrega el linaje a lo que hidrató musubi_memory_expand. ctx tiene que ser el MISMO
// contexto acotado con el que se hidrató: el linaje hereda así la frontera de la credencial, y no
// puede nombrar un id que la expansión no dejaría leer.
//
// ES BEST-EFFORT. Si el linaje falla, se loguea y la expansión sale como salía: el contenido ya está
// calculado, y quedarse sin él por culpa de un dato accesorio sería el peor intercambio posible.
func (s *McpServer) conLinaje(ctx context.Context, obs []memory.Observation) interface{} {
	if len(obs) == 0 {
		return obs
	}
	ids := make([]string, len(obs))
	for i, o := range obs {
		ids[i] = o.ID
	}
	lin, err := s.engine.LinajeCtx(ctx, ids)
	if err != nil {
		logx.Warn("linaje del acervo: no se pudo leer (la expansión sale igual, sin él)", "error", err)
		return obs
	}
	if len(lin) == 0 {
		return obs
	}
	out := make([]expandItem, len(obs))
	for i, o := range obs {
		out[i] = expandItem{Observation: o, Linaje: lin[o.ID]}
	}
	return out
}

// adjuntarFuentes llena `fuentes` en cada patrón del corpus con los ids de los artículos de los que
// salió esa ficha. Mismo trato que conLinaje: si falla, el brief sale sin fuentes y nada más.
func (s *McpServer) adjuntarFuentes(ctx context.Context, patrones []patronItem) {
	if len(patrones) == 0 {
		return
	}
	ids := make([]string, len(patrones))
	for i, p := range patrones {
		ids[i] = p.ID
	}
	lin, err := s.engine.LinajeCtx(ctx, ids)
	if err != nil {
		logx.Warn("linaje del acervo: no se pudieron leer las fuentes del corpus (el brief sale sin ellas)", "error", err)
		return
	}
	for i := range patrones {
		for _, ref := range lin[patrones[i].ID].SalioDe {
			patrones[i].Fuentes = append(patrones[i].Fuentes, ref.ID)
		}
	}
}
