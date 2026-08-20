package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// methods_design.go implementa musubi_design: el MOTOR DE DISEÑO de Musubi como CAPACIDAD del cerebro,
// invocable desde CUALQUIER terminal con token —con o sin proyecto abierto— no sólo desde la pantalla
// Lienzo del cuerpo. Es model-free (Track "Conocimiento Unificado": el cerebro es dueño de QUÉ es el
// conocimiento; el caller sintetiza el CÓMO): NO llama a ningún LLM ni compone el diseño. Ensambla un
// BRIEF —rol de diseñador senior + principios + la marca de Musubi + los patrones recallados del acervo
// de diseño— y se lo devuelve al agente que la invocó, que es quien compone el diseño final.
//
// El acervo vive en un TENANT propio del central (`musubi-design`, sembrado con literatura de sistemas de
// diseño + la identidad de la marca). El handler lo lee con un scope FIJO a ese tenant, sin importar el
// proyecto del caller: el conocimiento de diseño es un acervo COMPARTIDO, cualquier principal autenticado
// lo recibe. Por eso la tool es readOnly (las búsquedas no bumpean access) y la puede llamar hasta la
// cabina (write=none) o una sesión stdio local (sin principal).

// designCorpusScope es el tenant del cerebro central donde vive el acervo de diseño. Fijo a propósito:
// el brief se arma SIEMPRE contra este scope, no contra el proyecto del caller.
const designCorpusScope = "musubi-design"

// designCorpusLimit es cuántos patrones del acervo trae el brief por default (un techo sano: bastan unos
// pocos patrones concretos, no una investigación sin fin).
const designCorpusLimit = 6

// designBrief es lo que musubi_design le entrega al caller: todo el conocimiento de diseño ensamblado
// para que EL agente componga. El cerebro no dibuja; prepara el terreno.
type designBrief struct {
	Ask          string      `json:"ask"`                // el pedido, tal como llegó
	Target       string      `json:"target"`             // painter | web | html | any
	Role         string      `json:"role"`               // el rol de diseñador senior (universal)
	Principles   string      `json:"principles"`         // los principios que se aplican siempre
	Brand        string      `json:"brand"`              // la identidad de Musubi (no-negociables)
	Emit         string      `json:"emit"`               // cómo entregar según el target
	Corpus       []searchHit `json:"corpus"`             // patrones recallados del acervo (gists por id)
	CorpusScope  string      `json:"corpus_scope"`       // de qué tenant salió el acervo
	CorpusNote   string      `json:"corpus_note"`        // cómo profundizar un patrón
	Instructions string      `json:"instructions"`       // qué hace el caller ahora
	Degraded     bool        `json:"degraded,omitempty"` // true si el acervo no devolvió nada (queda el núcleo estático)
}

func (s *McpServer) toolDesign(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Prompt string `json:"prompt"`
		Target string `json:"target"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Prompt) == "" {
		return nil, rpcErrorf(codeInvalidParams, "prompt es obligatorio: describí qué querés diseñar")
	}
	target := normalizeDesignTarget(args.Target)
	limit := args.Limit
	if limit <= 0 {
		limit = designCorpusLimit
	}
	limit = clampLimit(limit)

	// Scope FIJO al acervo de diseño, sin importar el proyecto del caller: el conocimiento de diseño
	// es compartido. Federate=false ⇒ acota a `musubi-design` (más las filas sin atribuir), idéntico
	// criterio que el recall por proyecto.
	corpusCtx := memory.WithProjectScope(ctx, memory.ProjectScope{ProjectID: designCorpusScope, Federate: false})

	// Recall del acervo, best-effort: si algo falla, el brief conserva el NÚCLEO estático (rol +
	// principios + marca), que ya vale por sí solo. Un fallo del acervo NO tumba la tool.
	hits, degraded := s.recallDesignCorpus(ctx, corpusCtx, args.Prompt, limit)

	brief := designBrief{
		Ask:          strings.TrimSpace(args.Prompt),
		Target:       target,
		Role:         designRole,
		Principles:   designPrinciples,
		Brand:        designBrand,
		Emit:         designEmitFor(target),
		Corpus:       hits,
		CorpusScope:  designCorpusScope,
		CorpusNote:   "Cada item es un gist (titular). Para traer el patrón completo, expandí su id con musubi_memory_expand — 1 o 2, no más.",
		Instructions: designInstructions,
		Degraded:     degraded,
	}
	return jsonResult(brief)
}

// recallDesignCorpus trae los patrones más relevantes del acervo para el pedido. Prioriza la búsqueda
// semántica (embedder) y cae a la léxica (FTS) si no hay embedder o si la semántica no devolvió nada.
// Cualquier error es best-effort: devuelve lo que tenga (o vacío) y marca degraded, nunca falla la tool.
func (s *McpServer) recallDesignCorpus(ctx, corpusCtx context.Context, query string, limit int) (hits []searchHit, degraded bool) {
	if embedding.Enabled(s.embedder) {
		embCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		vec, err := s.embedder.Embed(embCtx, query)
		cancel()
		if err == nil {
			if results, serr := s.engine.SearchObservations(corpusCtx, vec, limit); serr == nil && len(results) > 0 {
				sources := make([]searchSource, len(results))
				for i, r := range results {
					sources[i] = searchSource{id: r.ID, topicKey: r.TopicKey, content: r.Content, sim: r.Similarity}
				}
				return toSearchHits(sources, s.memory.GistMaxTokens, searchGistBudget), false
			}
		}
	}
	// Fallback léxico (FTS5): siempre disponible, sin embedder. Best-effort.
	if results, ferr := s.engine.SearchObservationsFTS(corpusCtx, query, limit); ferr == nil && len(results) > 0 {
		sources := make([]searchSource, len(results))
		for i, r := range results {
			sources[i] = searchSource{id: r.ID, topicKey: r.TopicKey, content: r.Content}
		}
		return toSearchHits(sources, s.memory.GistMaxTokens, searchGistBudget), false
	}
	return nil, true
}

// normalizeDesignTarget acota el target a los cuatro emisores conocidos. Vacío/desconocido ⇒ "any"
// (el caller elige el mejor formato para el pedido).
func normalizeDesignTarget(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "painter", "cuerpo", "lienzo", "spec":
		return "painter"
	case "web", "react", "tailwind", "css":
		return "web"
	case "html", "mock", "mockup":
		return "html"
	default:
		return "any"
	}
}

// designEmitFor devuelve la guía de ENTREGA según el target. El núcleo (rol/principios/marca) es
// universal; sólo cambia en qué formato materializa el caller el diseño.
func designEmitFor(target string) string {
	switch target {
	case "painter":
		return designEmitPainter
	case "web":
		return designEmitWeb
	case "html":
		return designEmitHTML
	default:
		return designEmitAny
	}
}

// designToolEntry registra musubi_design. Va en local Y en el central: en el central lee el acervo
// `musubi-design` compartido; en stdio local (sin ese tenant) degrada al núcleo estático del brief.
// readOnly=true: las búsquedas del acervo no mutan nada, así la puede llamar cualquier principal
// —incluida la cabina write=none y una sesión sin proyecto— que es justo el uso pedido: diseñar
// "estando donde sea".
func (s *McpServer) designToolEntry() toolEntry {
	return toolEntry{
		Tool: Tool{
			Name:        "musubi_design",
			Description: "El MOTOR DE DISEÑO de Musubi, invocable desde CUALQUIER proyecto (o sin proyecto). Dado un pedido en lenguaje libre ('una pantalla de login oscura para finanzas', 'un dashboard de ventas'), devuelve un BRIEF de diseño anclado en el acervo compartido: el rol de diseñador senior, los principios que se aplican siempre, la identidad de marca de Musubi y los PATRONES relevantes recallados del acervo de diseño del central (sistemas de diseño, tokens, layout, tipografía, color, accesibilidad). Es model-free: el cerebro NO dibuja ni llama a un LLM — ensambla el conocimiento y VOS (el agente que la llamó) componés el diseño con ese brief. Pasá 'target' para orientar la ENTREGA: painter (spec de bloques del cuerpo) | web (React/Tailwind + tokens, para CRM/Altura) | html (mock autocontenido) | any (default, elegís el formato). Tras el brief, expandí 1-2 ids del corpus con musubi_memory_expand si querés el patrón completo, y entregá el diseño.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"prompt": {Type: "string", Description: "Descripción en lenguaje libre de lo que querés diseñar (tipo de pantalla, tono, contexto)."},
					"target": {Type: "string", Description: "Formato de ENTREGA que orienta el brief: 'painter' (spec de bloques del cuerpo/Lienzo) | 'web' (React + Tailwind + tokens, para CRM/Altura) | 'html' (mock HTML autocontenido) | 'any' (default: el caller elige el mejor)."},
					"limit":  {Type: "number", Description: "Cuántos patrones del acervo traer (default 6, máximo 100)."},
				},
				Required: []string{"prompt"},
			},
		},
		handler:  s.toolDesign,
		readOnly: true,
	}
}

// ─────────────────────────────────────────────────────────────────────────────────────────────────────
// EL NÚCLEO ESTÁTICO DEL BRIEF. Es el conocimiento de diseño que viaja SIEMPRE, aún sin acervo (stdio
// local, o un acervo vacío). El acervo lo AMPLÍA con patrones concretos; no lo reemplaza. Adaptado del
// design_prompt.txt del cuerpo (cmd/musubi-body/design_prompt.txt), generalizado para cualquier target.
// ─────────────────────────────────────────────────────────────────────────────────────────────────────

const designRole = `Sos el MOTOR DE DISEÑO de Musubi: un diseñador de producto senior, de clase mundial, experto en todo tipo de diseño de interfaz — apps móviles, web, dashboards de datos, landing/marketing, formularios, onboarding, e-commerce, paneles de administración, chat/mensajería, ajustes, estados vacíos, data-viz. Pensás en jerarquía, ritmo, contraste, alineación y aire. No decorás: componés. Recibís una descripción en lenguaje libre y componés UN diseño completo y excelente del tipo que se pide. Si el pedido es vago, elegís la interpretación más útil y la hacés impecable.`

const designPrinciples = `PRINCIPIOS QUE APLICÁS SIEMPRE:
1. JERARQUÍA: una sola cosa manda por pantalla. Título grande, lo demás cede.
2. GRILLA 4pt: posiciones y tamaños en múltiplos de 4 (8/12/16/24/32/40). Ritmo vertical consistente.
3. ESCALA TIPOGRÁFICA: no inventes tamaños; usá una escala (11/12/13/15/18/24/30).
4. ESPACIO EN BLANCO: agrupá lo relacionado, separá lo distinto. El aire ordena; no llenes.
5. ALINEACIÓN: un eje izquierdo fuerte. Todo cuelga de pocos ejes.
6. CONTRASTE: jerarquía por tamaño/color/peso, no por líneas y cajas por todos lados.
7. AGRUPACIÓN: eyebrow → título → subtítulo → contenido → acción. Ese ritmo lee profesional.
8. UN CTA claro por pantalla.`

const designBrand = `LA IDENTIDAD DE MUSUBI (no se pega encima: sale de lo que hace el producto). Musubi es 結び, el nudo que ata todo → la interfaz HACE eso, no lo dibuja. LA REGLA QUE SOSTIENE TODO: ningún elemento de identidad entra si no hace un trabajo — el nudo aparece porque hay vínculo; el sello porque hay ruta o estado; la voz porque hay algo que explicar; el ornamento SÓLO donde no hay datos. Un vacío se explica (qué lo va a llenar); un error NUNCA se disfraza de dato. PALETA: fondo oscuro, texto casi-blanco (principal) y secundario/tenue, UN acento dominante (violeta, la marca); el color se GANA, no se reparte — degradados e imágenes para héroes y momentos, no para todo. PROHIBIDO Y NO SE NEGOCIA: gradiente en textos, serifas, glows de color, vidrio con blur, color como adorno, adorno que tape un dato.`

const designInstructions = `CON ESTE BRIEF: (1) si querés más patrón concreto, expandí 1 o 2 ids del corpus con musubi_memory_expand — no más, el objetivo es traer estructura, no investigar sin fin; (2) componé UN diseño completo y excelente, anclado en los principios + la marca + lo que traiga el corpus (el corpus informa la ESTRUCTURA; vos componés — nunca copies texto de marca ajena ni inventes datos); (3) entregá en el formato del target (ver 'emit'). NO narres el proceso ni lo que recallaste: entregá el diseño.`

const designEmitAny = `TARGET = any (no fijado): elegí el formato que mejor sirva al pedido —un spec de pantalla, un mock HTML autocontenido, componentes React con tokens, o una descripción estructurada de pantalla— y declaralo en una línea arriba de tu entrega. Ante la duda, un mock HTML autocontenido es el más portable.`

const designEmitWeb = `TARGET = web (React + Tailwind + tokens, para CRM/Altura). Emití componentes que consuman TOKENS semánticos (no valores mágicos ni Tailwind de fábrica sin tocar): definí --bg, --surface, --ink, --muted, --accent, --ok, --warn, --danger y derivá todo de ahí. Nombrá los tokens por su ROL, no por su color. Respetá la marca (sobria, fondo oscuro, un acento). Stack objetivo: React 19 + Tailwind. No serifas, no glow de color, no glass/blur.`

const designEmitHTML = `TARGET = html (mock autocontenido para render headless). Un solo archivo HTML, SIN ninguna URL de red (CSP estricta): fuentes por @font-face local o system-stack, CSS inline, imágenes como data: o dibujadas con CSS. Ideal para capturar con Chrome headless. Dejá respirar los bordes; una sola pantalla, completa y con intención.`

const designEmitPainter = `TARGET = painter (el motor nativo del cuerpo/Lienzo dibuja un SPEC JSON de bloques). Devolvés SÓLO el JSON del spec: { "blocks": [ BLOQUE, ... ] } — los bloques se dibujan EN ORDEN (el último queda encima). Frame (artboard) 340×520 (pantalla de teléfono), margen 28 a cada lado, fondo oscuro. Cada BLOQUE: {"kind","x","y","w","h","label","px","tint","radius","primary","shadow","fill","children"}.
kind ∈ card | panel | button | text | eyebrow | divider | dot | chip | row | col.
  card=contenedor elevado (héroes/tarjetas) · panel=superficie plana con borde (campos/filas) · button=acción (primary:true relleno) · text=texto (px marca jerarquía: título 22–30, subtítulo 14–18, cuerpo 13–15, meta 11–12) · eyebrow=rótulo mayúsculas con barrita (px 11) · divider=línea (h:1) · dot=indicador 12×12 · chip=etiqueta translúcida · row/col=auto-acomodan sus children con gap.
tint (rol de color) ∈ INK (principal) | MUTED (secundario) | FAINT (tenue) | CORD (acento primario, violeta) | BRAIN (acento secundario) | BODY (positivo/verde) | WARN (ámbar).
fill (cuerpo de una caja) ∈ "CORD" (sólido nombrado) | "solid:BODY" | "grad:CORD,BRAIN,vertical|horizontal|diagonal" | "grad:BRAIN,CORD,radial" | "image:foto" | "image:textura".
REGLAS DE SALIDA: SÓLO el JSON (sin ` + "```" + `json, sin comentarios, sin prosa antes ni después), JSON válido (comillas dobles, sin comas colgantes), todo dentro del frame 340×520, un CTA por pantalla.`
