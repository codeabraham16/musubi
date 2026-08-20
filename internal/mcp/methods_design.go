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

// brandTopicKey es la clave donde vive la MARCA ACTIVA de un proyecto (Musubi Renaissance · CAPA 3,
// marca-por-proyecto): una observación con los tokens + reglas de identidad de ESE proyecto.
const brandTopicKey = "diseno/marca"

// homeBrandProject es el proyecto dueño de la marca Musubi por default (el fallback cuando no hay
// principal, p.ej. stdio local). Sólo un caller de Musubi (o sin principal) hereda la marca Musubi;
// un proyecto ajeno SIN marca propia NO la hereda (ver brandFor → designBrandNeutral).
const homeBrandProject = "musubi"

// designBrief es lo que musubi_design le entrega al caller: todo el conocimiento de diseño ensamblado
// para que EL agente componga. El cerebro no dibuja; prepara el terreno.
type designBrief struct {
	Ask          string       `json:"ask"`                    // el pedido, tal como llegó
	Target       string       `json:"target"`                 // painter | web | html | any
	Role         string       `json:"role"`                   // el rol de diseñador senior (universal)
	Principles   string       `json:"principles"`             // los principios que se aplican siempre
	Brand        string       `json:"brand"`                  // la marca ACTIVA, resuelta por proyecto (CAPA 3)
	BrandScope   string       `json:"brand_scope"`            // de qué proyecto salió la marca
	BrandSource  string       `json:"brand_source"`           // project | default | none (ver brandFor)
	BrandTokens  *brandTokens `json:"brand_tokens,omitempty"` // tokens estructurados de la marca (F2), si los hay
	Emit         string       `json:"emit"`                   // cómo entregar según el target (relleno con los tokens)
	Corpus       []searchHit  `json:"corpus"`                 // patrones recallados del acervo (gists por id)
	CorpusScope  string       `json:"corpus_scope"`           // de qué tenant salió el acervo
	CorpusNote   string       `json:"corpus_note"`            // cómo profundizar un patrón
	Instructions string       `json:"instructions"`           // qué hace el caller ahora
	Degraded     bool         `json:"degraded,omitempty"`     // true si el acervo no devolvió nada (queda el núcleo estático)
}

func (s *McpServer) toolDesign(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Prompt string `json:"prompt"`
		Target string `json:"target"`
		Brand  string `json:"brand"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}
	if strings.TrimSpace(args.Prompt) == "" {
		return nil, rpcErrorf(codeInvalidParams, "prompt es obligatorio: describí qué querés diseñar")
	}
	target := normalizeDesignTarget(args.Target)

	// CAPA 3 — marca por proyecto: se resuelve por el PRINCIPAL autenticado (nunca por texto libre),
	// así "sólo la del target, nunca se cruza" sale del propio modelo. El acervo (materia prima +
	// método) es compartido; la marca es del proyecto.
	brandScope := brandScopeFor(principalFrom(ctx), args.Brand)
	brandText, brandSource, brandTok := s.brandFor(brandScope)
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
		Brand:        brandText,
		BrandScope:   brandScope,
		BrandSource:  brandSource,
		BrandTokens:  brandTok,
		Emit:         designEmitFor(target, brandTok),
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

// brandScopeFor decide DE QUÉ proyecto sale la marca activa, con la disciplina "nunca se cruza": el
// scope se DERIVA del principal autenticado (misma idea que writeOriginFor sella las escrituras), NO de
// texto libre que el cliente controle. El arg `brand` sólo se respeta para un principal read=all (la sala
// de mando), que puede diseñar a nombre de OTRO proyecto; un principal acotado lo ignora y usa el suyo.
// Sin principal (stdio local) ⇒ homeBrandProject (Musubi).
func brandScopeFor(p *Principal, argBrand string) string {
	if argBrand = strings.TrimSpace(argBrand); argBrand != "" && brandArgAllowed(p) {
		return argBrand
	}
	if p != nil && p.ProjectID != "" {
		return p.ProjectID
	}
	return homeBrandProject
}

// brandArgAllowed indica si el principal puede DECLARAR una marca ajena por el arg `brand`: sólo quien ve
// todos los proyectos (read=all, la sala de mando/cabina). Un writer acotado no puede pedir la marca de
// otro tenant. Sin principal (stdio) ⇒ confianza local.
func brandArgAllowed(p *Principal) bool {
	if p == nil {
		return true
	}
	read, _ := p.caps()
	return read == ReadAll
}

// brandFor resuelve la MARCA ACTIVA para un scope de proyecto (Musubi Renaissance · CAPA 3). Fetch KEYED
// y estricto de la obs 'diseno/marca' del tenant (nunca semántico, nunca hereda de otro): si existe, es
// la marca del proyecto (source "project"); si no y el scope es Musubi, la marca Musubi por default
// (source "default"); si no y es un proyecto ajeno, la marca neutra que NO cruza identidad (source
// "none"). Model-free. Un error de lectura degrada al default/neutral, nunca tumba el brief.
func (s *McpServer) brandFor(scope string) (identity, source string, tokens *brandTokens) {
	if content, found, err := s.engine.LatestObservationByTopicInProject(brandTopicKey, scope); err == nil && found && strings.TrimSpace(content) != "" {
		if bt := parseBrandTokens(content); bt != nil { // marca ESTRUCTURADA (doc JSON con tokens)
			id := strings.TrimSpace(bt.Identity)
			if id == "" {
				id = "Marca del proyecto (tokens estructurados: ver brand_tokens y emit)."
			}
			return id, "project", bt
		}
		return content, "project", nil // marca en PROSA: identidad sí, tokens no → emit genérico
	}
	if scope == homeBrandProject {
		return designBrand, "default", musubiBrandTokens
	}
	return designBrandNeutral, "none", nil
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

// designEmitFor devuelve la guía de ENTREGA según el target, RELLENA con los tokens de la marca cuando
// los hay (F2 · una fuente → N targets): a la guía base se le suma la paleta/tipografía/radios REALES de
// la marca resuelta, en el dialecto del target. Sin tokens (marca en prosa) queda sólo la guía genérica.
func designEmitFor(target string, tokens *brandTokens) string {
	base := designEmitBase(target)
	if r := tokens.render(target); r != "" {
		return base + "\n\nTOKENS DE LA MARCA (usá ESTOS valores; no inventes hex ni tamaños):\n" + r
	}
	return base
}

// designEmitBase es la guía de entrega UNIVERSAL por target (sin valores de marca): el vocabulario y el
// formato, que no cambian entre marcas.
func designEmitBase(target string) string {
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
			Description: "El MOTOR DE DISEÑO de Musubi (pilar 'Musubi Renaissance'), invocable desde CUALQUIER proyecto (o sin proyecto). Dado un pedido en lenguaje libre ('una pantalla de login oscura para finanzas', 'un dashboard de ventas'), devuelve un BRIEF de diseño en tres capas: la MATERIA PRIMA + el MÉTODO universal recallados del acervo de diseño compartido (rol de diseñador senior, principios anti-genérico, patrones de sistemas de diseño/tokens/layout/tipografía/color/accesibilidad) MÁS la MARCA del proyecto (resuelta por tu credencial: la de Musubi por default, la de otro proyecto si tenés acceso). Es model-free: el cerebro NO dibuja ni llama a un LLM — ensambla el conocimiento y VOS (el agente que la llamó) componés el diseño con ese brief. Pasá 'target' para orientar la ENTREGA: painter (spec de bloques del cuerpo) | web (React/Tailwind + tokens, para CRM/Altura) | html (mock autocontenido) | any (default). 'brand' opcional para diseñar a nombre de otro proyecto. Tras el brief, expandí 1-2 ids del corpus con musubi_memory_expand si querés el patrón completo, y entregá el diseño.",
			InputSchema: InputSchema{
				Type: "object",
				Properties: map[string]Property{
					"prompt": {Type: "string", Description: "Descripción en lenguaje libre de lo que querés diseñar (tipo de pantalla, tono, contexto)."},
					"target": {Type: "string", Description: "Formato de ENTREGA que orienta el brief: 'painter' (spec de bloques del cuerpo/Lienzo) | 'web' (React + Tailwind + tokens, para CRM/Altura) | 'html' (mock HTML autocontenido) | 'any' (default: el caller elige el mejor)."},
					"brand":  {Type: "string", Description: "Opcional: proyecto cuya MARCA aplicar (ej. 'crm', 'altura'). Por default la marca sale de TU proyecto (el del token); pasar 'brand' para diseñar a nombre de otro proyecto SÓLO lo respeta un principal read=all (la sala de mando). La identidad de un proyecto nunca se cruza a otro."},
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

// designBrand es la marca Musubi por DEFAULT (fallback cuando no hay un doc 'diseno/marca' en el tenant
// musubi). Los hex son los tokens REALES del cuerpo (body-rs/src/ui.rs), no el "violeta genérico" que
// tenía antes y que produjo un demo off-brand: la marca de Musubi es ÍNDIGO sobre AZUL-NOCHE, plana.
const designBrand = `LA IDENTIDAD DE MUSUBI (no se pega encima: sale de lo que hace el producto). Musubi es 結び, el nudo que ata todo → la interfaz HACE eso, no lo dibuja. LA REGLA QUE SOSTIENE TODO: ningún elemento de identidad entra si no hace un trabajo — el nudo aparece porque hay vínculo; el sello porque hay ruta o estado; la voz porque hay algo que explicar; el ornamento SÓLO donde no hay datos. Un vacío se explica (qué lo va a llenar); un error NUNCA se disfraza de dato. PALETA REAL (tokens del cuerpo, hex): fondo AZUL-NOCHE #0C1020 (NO negro puro), superficies #121734 / #182042, borde hairline #2A335C; texto INK #E9ECF7 (principal), MUTED #98A0C0, FAINT #5A6390; UN acento dominante — CORD = ÍNDIGO #6366F1 (la marca, hover #818CF8); segundo acento BRAIN cian #22D3EE (detalle); estado RESERVADO: BODY verde #34D399 (ok), WARN ámbar #FBBF24 (aviso), NO rosa-rojo #FB7185 (error). El color se GANA, no se reparte. ELEVACIÓN PLANA: la profundidad es por capas de fondo + hairline de 1px, NUNCA por sombra ni glow. Radio 8 (superficies), 4 (pills). PROHIBIDO Y NO SE NEGOCIA: gradiente en textos, serifas, glows de color, vidrio con blur, color como adorno, adorno que tape un dato.`

// designBrandNeutral es la marca para un proyecto AJENO que todavía no definió la suya. NO hereda la
// identidad de Musubi (eso sería cruzar la marca): pide el método universal + una paleta neutra, y cómo
// fijar la marca propia. Es el fallback con brand_source:"none".
const designBrandNeutral = `SIN MARCA DEFINIDA para este proyecto. Aplicá SÓLO el método universal (jerarquía, un CTA, "el color se gana", matar el look de IA) con una paleta sobria y sensata que el pedido sugiera. ⛔ NO uses la identidad de Musubi (el nudo 結び, el índigo #6366F1) — es de OTRO proyecto y no se cruza. Para fijar la marca de ESTE proyecto, guardá una observación con topic_key='diseno/marca' en su tenant: tokens (paleta por rol, tipografía, radios, elevación) + reglas de identidad + prohibiciones.`

const designInstructions = `CON ESTE BRIEF: (1) si querés más patrón concreto, expandí 1 o 2 ids del corpus con musubi_memory_expand — no más, el objetivo es traer estructura, no investigar sin fin; (2) componé UN diseño completo y excelente, anclado en los principios + la marca + lo que traiga el corpus (el corpus informa la ESTRUCTURA; vos componés — nunca copies texto de marca ajena ni inventes datos); (3) entregá en el formato del target (ver 'emit'). NO narres el proceso ni lo que recallaste: entregá el diseño.`

const designEmitAny = `TARGET = any (no fijado): elegí el formato que mejor sirva al pedido —un spec de pantalla, un mock HTML autocontenido, componentes React con tokens, o una descripción estructurada de pantalla— y declaralo en una línea arriba de tu entrega. Ante la duda, un mock HTML autocontenido es el más portable.`

const designEmitWeb = `TARGET = web (React + Tailwind + tokens, para CRM/Altura). Emití componentes que consuman TOKENS semánticos (no valores mágicos ni Tailwind de fábrica sin tocar): definí --bg, --surface, --ink, --muted, --accent, --ok, --warn, --danger y derivá todo de ahí. Nombrá los tokens por su ROL, no por su color. Respetá la marca (sobria, fondo oscuro, un acento). Stack objetivo: React 19 + Tailwind. No serifas, no glow de color, no glass/blur.`

const designEmitHTML = `TARGET = html (mock autocontenido para render headless). Un solo archivo HTML, SIN ninguna URL de red (CSP estricta): fuentes por @font-face local o system-stack, CSS inline, imágenes como data: o dibujadas con CSS. Ideal para capturar con Chrome headless. Dejá respirar los bordes; una sola pantalla, completa y con intención.`

const designEmitPainter = `TARGET = painter (el motor nativo del cuerpo/Lienzo dibuja un SPEC JSON de bloques). Devolvés SÓLO el JSON del spec: { "blocks": [ BLOQUE, ... ] } — los bloques se dibujan EN ORDEN (el último queda encima). Frame (artboard) 340×520 (pantalla de teléfono), margen 28 a cada lado, fondo oscuro. Cada BLOQUE: {"kind","x","y","w","h","label","px","tint","radius","primary","shadow","fill","children"}.
kind ∈ card | panel | button | text | eyebrow | divider | dot | chip | row | col.
  card=contenedor elevado (héroes/tarjetas) · panel=superficie plana con borde (campos/filas) · button=acción (primary:true relleno) · text=texto (px marca jerarquía: título 22–30, subtítulo 14–18, cuerpo 13–15, meta 11–12) · eyebrow=rótulo mayúsculas con barrita (px 11) · divider=línea (h:1) · dot=indicador 12×12 · chip=etiqueta translúcida · row/col=auto-acomodan sus children con gap.
tint (rol de color) ∈ INK (principal) | MUTED (secundario) | FAINT (tenue) | CORD (acento primario de la marca; en Musubi, índigo) | BRAIN (acento secundario) | BODY (positivo/verde) | WARN (ámbar). Los VALORES concretos de cada rol salen de la marca del brief, no son fijos.
fill (cuerpo de una caja) ∈ "CORD" (sólido nombrado) | "solid:BODY" | "grad:CORD,BRAIN,vertical|horizontal|diagonal" | "grad:BRAIN,CORD,radial" | "image:foto" | "image:textura".
REGLAS DE SALIDA: SÓLO el JSON (sin ` + "```" + `json, sin comentarios, sin prosa antes ni después), JSON válido (comillas dobles, sin comas colgantes), todo dentro del frame 340×520, un CTA por pantalla.`
