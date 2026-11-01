// recall.go es el delta 2 de spec-graph (F3-A): recall SEMÁNTICO sobre el cuerpo de las specs, el
// complemento del grep LITERAL de query.go ("nada de semántica, eso es F3"). Contenido dentro de
// specgraph y alimentado del FS: NO toca la DB ni importa el motor de memoria —la misma regla
// FS-nativa por la que anchors.go redefine StaleChanged/StaleMissing en vez de importar
// internal/memory, y por la que el coseno se redefine acá en vez de importar memory.CosineSimilarity.
//
// Model-free con degradación OBLIGATORIA (R5): con embebedor rankea por similitud (coseno); SIN
// embebedor, o si el embed del query falla, o si el portero de privacidad veta un cuerpo (R7), cae
// a un ranking por keyword determinista y NUNCA falla por eso. Los dos modos puntúan en [0,1] para
// un interleave coherente (D8), y el orden es SIEMPRE (score desc, id asc, path asc) ⇒ reproducible
// y testeable (R6).

package specgraph

import (
	"context"
	"errors"
	"hash/fnv"
	"math"
	"slices"
	"strings"
	"sync"
	"unicode"
)

// DefaultRecallLimit es el tope de hits por defecto (limit<=0). Más chico que DefaultGrepLimit (200)
// a propósito: un recall rankeado sirve los MÁS relevantes, así que 20 es una página útil y no un
// volcado. El handler recorta además por arriba (tope duro).
const DefaultRecallLimit = 20

// snippetMaxChars recorta el snippet para no desbordar la respuesta (D4). Se mide en RUNAS, no en
// bytes, para no cortar a mitad de un carácter multibyte: los cuerpos en español llevan acentos.
const snippetMaxChars = 200

// EmbedFunc es el seam MÍNIMO que specgraph necesita del motor de embeddings: una función que
// embebe un texto, definida acá para NO depender de internal/embedding (regla FS-nativa, D7).
//
// Es una FUNCIÓN y no una interfaz A PROPÓSITO, y es una decisión de candado, no de estilo: la
// guarda candado_no_cruza_la_red reconoce el borde de red `Embed` por NOMBRE SIMPLE y SÓLO dentro
// de internal/mcp (no recorre specgraph). Si Recall llamara a un `Embedder.Embed` propio, ese
// `.Embed` viviría en specgraph, el borde quedaría INVISIBLE para la guarda y el `lockSelf` del
// handler figuraría MUERTO (declarado sin llegar al borde) ⇒ la guarda en rojo. Con una función, el
// handler pasa la clausura `func(ctx, text) { return s.embedder.Embed(ctx, text) }`: el `.Embed`
// queda LÉXICAMENTE en toolSpecRecall (ast.Inspect desciende a los func-literal), la guarda ve que
// el handler llega al embebedor y exige `lockSelf` —que acá es la verdad: el embed es una llamada de
// red que NO debe correr bajo el candado del despacho—. El Provider del server ya viene blindado, así
// que el portero de privacidad (R7) viaja GRATIS sin que specgraph conozca la política.
//
// nil ⇒ sin embebedor: Recall rankea TODO por keyword (R5) sin intentar una sola llamada de red.
type EmbedFunc func(ctx context.Context, text string) ([]float32, error)

// SpecHit es una spec rankeada. Score ∈ [0,1] sin importar el modo (coseno o keyword). Snippet es
// una línea del CUERPO (sin frontmatter), como preview.
type SpecHit struct {
	ID      string
	Type    string
	Title   string
	Path    string
	Score   float64
	Snippet string
}

// RecallResult espeja la forma de GrepResult: paginado estable (Truncated + NextOffset).
type RecallResult struct {
	Hits       []SpecHit
	Truncated  bool
	NextOffset int
}

// RecallOptions son los parámetros de Recall.
type RecallOptions struct {
	Query  string
	Type   string // "" = sin filtro
	Limit  int    // <=0 ⇒ DefaultRecallLimit
	Offset int
}

// cachedVec es un vector memoizado junto a la huella del Content del que salió: si el cuerpo cambia,
// el hash cambia, la entrada deja de coincidir y se recomputa (R9, capa cache).
type cachedVec struct {
	hash uint64
	vec  []float32
}

// Recaller guarda el cache de vectores por proceso, con su PROPIO mutex (patrón D3 del Index): el
// cache es un read-modify-write, y el embed corre FUERA de ese mutex (nunca se sostiene un candado
// durante una llamada de red). Vive en el server, construido una vez; el corpus llega FRESCO en
// cada Recall porque ContentEntries() revalida (mtime,size) en cada llamada (R9, capa FS).
type Recaller struct {
	mu      sync.Mutex
	cache   map[string]cachedVec
	embName string // embebedor fijo por proceso: entra en la clave del cache como defensa (D9)
}

// NewRecaller crea un Recaller con el cache vacío. embName es el nombre del embebedor del server
// (embedder.Name()): entra en la clave del cache para que un vector nunca se sirva bajo otro
// embebedor. En la práctica el embebedor es fijo por proceso, pero meterlo en la clave hace el
// invariante explícito y verificable en vez de un comentario.
func NewRecaller(embName string) *Recaller {
	return &Recaller{cache: make(map[string]cachedVec), embName: embName}
}

// Recall rankea el corpus por relevancia al query y pagina. embed es el embebedor del caller, que
// pasa nil cuando embedding.Enabled(provider) es false:
//   - embed == nil        ⇒ keyword para TODO el corpus (R5), sin tocar el embebedor.
//   - embed != nil, query falla  ⇒ degrada TODA la llamada a keyword (R5).
//   - embed != nil, un spec falla ⇒ ESE spec cae a keyword (R7); el resto por coseno.
//
// El ÚNICO error que devuelve es un offset negativo (culpa del llamador, como GrepSpecs): la falta
// o el fallo del embebedor NUNCA es error (invariante central model-free, R5).
func (r *Recaller) Recall(ctx context.Context, entries []SpecContentEntry, opts RecallOptions, embed EmbedFunc) (RecallResult, error) {
	if opts.Offset < 0 {
		return RecallResult{}, errors.New("offset inválido: debe ser >= 0")
	}

	// Filtro por type (R10) ANTES de rankear y de embeber: no se gasta una llamada de red en un
	// spec que el filtro ya descartó.
	corpus := entries
	if opts.Type != "" {
		corpus = make([]SpecContentEntry, 0, len(entries))
		for _, e := range entries {
			if t, _ := Scalar(e.Frontmatter, FieldType); t == opts.Type {
				corpus = append(corpus, e)
			}
		}
	}

	qset := toSet(tokenize(opts.Query))

	// Modo semántico SÓLO si hay embebedor Y el query se pudo embeber en un vector USABLE. Un query
	// que falla o devuelve vacío (gotcha banda-muerta: el embebedor trunca en silencio) degrada toda
	// la llamada a keyword (R5) en vez de rankear contra un vector basura.
	var qvec []float32
	semantic := false
	if embed != nil {
		if v, err := embed(ctx, opts.Query); err == nil && len(v) > 0 {
			qvec = v
			semantic = true
		}
	}

	hits := make([]SpecHit, 0, len(corpus))
	for _, e := range corpus {
		// id/type están garantizados: ContentEntries() sólo devuelve specs reales (IsSpec ⇒ R8); el
		// diario SDD (frontmatter artifact+schema_version, sin id+type) nunca llega acá. title puede
		// faltar (IsSpec no lo exige).
		id, _ := Scalar(e.Frontmatter, FieldID)
		typ, _ := Scalar(e.Frontmatter, FieldType)
		title, _ := Scalar(e.Frontmatter, FieldTitle)

		var score float64
		if semantic {
			if sv, ok := r.vectorFor(ctx, embed, e.Path, e.Content); ok && len(sv) == len(qvec) {
				score = cosine(qvec, sv)
			} else {
				// R7 / banda-muerta: este spec no se pudo embeber en un vector usable ⇒ keyword.
				score = keywordScore(qset, e.Content)
			}
		} else {
			score = keywordScore(qset, e.Content)
		}

		hits = append(hits, SpecHit{
			ID:      id,
			Type:    typ,
			Title:   title,
			Path:    e.Path,
			Score:   score,
			Snippet: bestSnippet(qset, e.Content),
		})
	}

	// Orden determinista (R6): score desc, luego id asc, luego path asc. Path es único por entrada
	// (el walk no repite rutas), así que (id,path) es un orden TOTAL ⇒ el resultado es reproducible
	// entre corridas sin depender de la estabilidad del sort.
	slices.SortFunc(hits, func(a, b SpecHit) int {
		if a.Score != b.Score {
			if a.Score > b.Score {
				return -1
			}
			return 1
		}
		if c := strings.Compare(a.ID, b.ID); c != 0 {
			return c
		}
		return strings.Compare(a.Path, b.Path)
	})

	// Paginado estable (R10), espejando GrepResult: Truncated=true + NextOffset al próximo índice.
	limit := opts.Limit
	if limit <= 0 {
		limit = DefaultRecallLimit
	}
	total := len(hits)
	if opts.Offset >= total {
		return RecallResult{Hits: make([]SpecHit, 0)}, nil
	}
	end := opts.Offset + limit
	truncated := end < total
	if end > total {
		end = total
	}
	next := 0
	if truncated {
		next = end
	}
	return RecallResult{Hits: hits[opts.Offset:end], Truncated: truncated, NextOffset: next}, nil
}

// vectorFor devuelve el vector del Content, del cache si la huella coincide (R9) o recomputándolo.
// EL EMBED CORRE FUERA DEL MUTEX (D6): lock→mirar→unlock→embed→lock→guardar→unlock. Dos llamadas
// concurrentes pueden re-embeber el mismo spec (desperdicio, nunca incorrecto: mismo vector). ok=false
// ⇒ el embed falló o devolvió vacío; el caller cae a keyword (R7) y NO se cachea el vacío (podría ser
// un embebedor intermitente, y reintentar en la próxima llamada es lo correcto).
func (r *Recaller) vectorFor(ctx context.Context, embed EmbedFunc, path, content string) ([]float32, bool) {
	h := hashContent(content)
	key := r.embName + "\x00" + path

	r.mu.Lock()
	cached, ok := r.cache[key]
	r.mu.Unlock()
	if ok && cached.hash == h {
		return cached.vec, true
	}

	vec, err := embed(ctx, content)
	if err != nil || len(vec) == 0 {
		return nil, false
	}

	r.mu.Lock()
	r.cache[key] = cachedVec{hash: h, vec: vec}
	r.mu.Unlock()
	return vec, true
}

// --- Helpers puros (testeables por separado) ---

// tokenize baja a minúscula y parte por todo lo que no sea letra o número (unicode-aware: los
// cuerpos en español tienen acentos). Devuelve los tokens con repetición; toSet los deduplica.
func tokenize(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
}

func toSet(tokens []string) map[string]struct{} {
	set := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		set[t] = struct{}{}
	}
	return set
}

// keywordScore es la señal PROPIA del fallback (D2): la fracción de términos DISTINTOS del query
// presentes en el cuerpo, ∈ [0,1]. NO reusa el FTS5 de observaciones (indexa memory.db, no los
// bodies de specs). Un query sin términos (sólo puntuación) puntúa 0 sin dividir por cero.
func keywordScore(qset map[string]struct{}, content string) float64 {
	if len(qset) == 0 {
		return 0
	}
	cset := toSet(tokenize(content))
	matched := 0
	for term := range qset {
		if _, ok := cset[term]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(qset))
}

// cosine redefine la aritmética de memory.CosineSimilarity LOCAL (D7: specgraph no importa el
// motor) y clampa a [0,1]: un coseno natural es [-1,1], pero un valor negativo (vectores opuestos)
// significa "no relacionado", que para rankear es lo mismo que 0; clampar deja los dos modos en la
// misma escala que keywordScore (D8). Longitudes distintas o vector cero ⇒ 0.
func cosine(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	c := dot / (math.Sqrt(na) * math.Sqrt(nb))
	if c < 0 {
		return 0
	}
	if c > 1 {
		return 1 // guarda contra el redondeo de float
	}
	return c
}

// bestSnippet elige el preview (D4): la línea del CUERPO con más términos DISTINTOS del query
// (desempate: menor nº de línea). Si NINGUNA línea contiene un término literal —match puro-semántico
// (el caso estrella) o query sin términos— cae a la primera línea de prosa no vacía. Independiente
// del modo de ranking y 100% determinista; recorta a snippetMaxChars runas.
func bestSnippet(qset map[string]struct{}, content string) string {
	lines := bodyLines(content)
	bestIdx, bestCount := -1, 0
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		n := 0
		lset := toSet(tokenize(ln))
		for term := range qset {
			if _, ok := lset[term]; ok {
				n++
			}
		}
		if n > bestCount {
			bestCount, bestIdx = n, i
		}
	}
	if bestIdx == -1 {
		for _, ln := range lines {
			if t := strings.TrimSpace(ln); t != "" {
				return clip(t)
			}
		}
		return ""
	}
	return clip(strings.TrimSpace(lines[bestIdx]))
}

// bodyLines parte el CUERPO (sin el bloque de frontmatter) en líneas, reusando splitFrontmatter —el
// mismo parseo que el resto de specgraph—. El vector se calcula sobre el Content CRUDO (D3, incluye
// el frontmatter), pero el snippet debe ser prosa, no YAML.
func bodyLines(content string) []string {
	_, _, body := splitFrontmatter(content)
	raw := strings.Split(body, "\n")
	out := make([]string, 0, len(raw))
	for i, ln := range raw {
		if i == 0 {
			ln = strings.TrimPrefix(ln, bom)
		}
		out = append(out, strings.TrimSuffix(ln, "\r"))
	}
	return out
}

// clip recorta a snippetMaxChars RUNAS (no bytes) para no partir un carácter multibyte.
func clip(s string) string {
	r := []rune(s)
	if len(r) <= snippetMaxChars {
		return s
	}
	return string(r[:snippetMaxChars])
}

// hashContent es la huella del Content para invalidar el cache (R9). FNV-1a 64: barato y sin
// pretensión criptográfica — sólo distingue "este cuerpo cambió" de "es el mismo".
func hashContent(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}
