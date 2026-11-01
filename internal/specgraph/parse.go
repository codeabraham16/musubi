// Package specgraph es el port model-free del core de thinkrail
// (packages/spec-graph/core, Apache-2.0): detecta "esto es una spec",
// parsea/edita su frontmatter sin pérdida, y deriva el grafo e índice del
// filesystem. Sin pi, sin cgo, sin LLM en el camino.
//
// parse.go porta core/parse.ts: el vocabulario del dialecto, el split del
// frontmatter y los accesos tipados. La edición lossless vive en
// frontmatter_edit.go (la segunda mitad de parse.ts).
package specgraph

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter es el frontmatter YAML ya normalizado a escalares/listas de
// strings, igual que el `Record<string, string | string[]>` de thinkrail.
// Un valor es `string` o `[]string`; cualquier objeto anidado se descarta al
// normalizar (ver toFrontmatter). Un mapa nil representa el `null` de
// thinkrail: "no hay frontmatter de spec acá".
type Frontmatter map[string]any

// Nombres de campo del dialecto. Verbatim en inglés porque son el contrato de
// datos que comparten las specs en disco, independiente del idioma del código.
const (
	FieldID         = "id"
	FieldType       = "type"
	FieldStatus     = "status"
	FieldTitle      = "title"
	FieldParent     = "parent"
	FieldDependsOn  = "depends-on"
	FieldReferences = "references"
	FieldImplements = "implements"
	FieldCovers     = "covers"
	FieldTags       = "tags"
)

// Vocabulario cerrado. Se declaran como slices nuevos (no con append sobre
// otro) para que nadie comparta backing array y una mutación accidental no
// contamine otra lista.
var (
	// RequiredFields: lo que una spec "completa" debería tener. OJO: la
	// identidad (R1) es sólo id+type; title es "requerido" para estar bien
	// formada pero no decide si algo ES una spec.
	RequiredFields = []string{FieldID, FieldType, FieldTitle}

	// IdentityFields: id+type. Su presencia (ambos escalares no vacíos) es la
	// definición EXACTA de "esto es una spec" (IsSpec). Es lo que distingue un
	// SPEC.md de un proposal.md/spec.md del diario SDD, que no los tiene.
	IdentityFields = []string{FieldID, FieldType}

	// SpecTypes: los 5 tipos, en orden jerárquico de mayor a menor alcance.
	SpecTypes = []string{
		"goal-and-requirements",
		"architecture-design",
		"module-design",
		"submodule-design",
		"task-spec",
	}

	// SpecStatuses: estados de ciclo de vida de una spec.
	SpecStatuses = []string{"draft", "active", "stale", "done", "deprecated"}

	// SingleLinkFields: campos de enlace con a lo sumo un target (árbol).
	SingleLinkFields = []string{FieldParent}

	// ListLinkFields: campos de enlace con múltiples targets (DAG).
	ListLinkFields = []string{FieldDependsOn, FieldReferences, FieldImplements}

	// ListFields: todos los campos lista (enlaces + covers + tags). covers y
	// tags son listas pero NO aristas del grafo.
	ListFields = []string{FieldDependsOn, FieldReferences, FieldImplements, FieldCovers, FieldTags}

	// FieldOrder: orden canónico al serializar frontmatter nuevo (create).
	FieldOrder = []string{
		FieldID, FieldType, FieldStatus, FieldTitle,
		FieldParent,
		FieldDependsOn, FieldReferences, FieldImplements, FieldCovers, FieldTags,
	}

	// LinkKinds: las 4 clases de arista (parent + los 3 de lista). Definido en
	// graph.ts como LINK_KINDS; vive acá por ser vocabulario compartido.
	LinkKinds = []string{FieldParent, FieldDependsOn, FieldReferences, FieldImplements}
)

const (
	fence = "---"
	crlf  = "\r\n"
)

// bom es el marcador BOM UTF-8 (U+FEFF). Se arma por bytes a propósito: escribir
// el carácter literal (o un escape \u que el pipeline resuelva) mete un BOM en
// medio del fuente y el compilador de Go lo rechaza.
var bom = string([]byte{0xEF, 0xBB, 0xBF})

// ParsedFile es el resultado de partir un archivo en frontmatter + cuerpo.
// Frontmatter nil ⇔ el `frontmatter: null` de thinkrail (no hay fence válido,
// el YAML falló, o la raíz no era un objeto). Un mapa vacío no-nil ⇔ hubo
// frontmatter pero quedó vacío: no es spec, pero no es "null".
type ParsedFile struct {
	Frontmatter Frontmatter
	Body        string
}

// toFrontmatter normaliza el valor cargado por YAML al dialecto escalar/lista.
// Espeja la función homónima de thinkrail:
//   - null/undefined (YAML vacío) ⇒ objeto vacío, no nil.
//   - no es objeto (array o escalar en la raíz) ⇒ nil.
//   - por clave: null se saltea; array ⇒ lista de strings; escalar ⇒ string;
//     objeto anidado ⇒ se descarta.
//
// Divergencia deliberada: thinkrail hace String(v) sobre CADA item de un array,
// así que un objeto anidado dentro de una lista queda como "[object Object]".
// Acá ese item se descarta (scalarString devuelve false). Es input fuera del
// esquema —ninguna spec válida mete objetos en depends-on/tags—, y el camino
// lossless (frontmatter_edit.go) ni siquiera pasa por acá, así que soltar la
// basura es más limpio que propagarla y no afecta a ninguna spec real.
func toFrontmatter(loaded any) Frontmatter {
	if loaded == nil {
		return Frontmatter{}
	}
	obj, ok := loaded.(map[string]any)
	if !ok {
		return nil
	}
	fm := Frontmatter{}
	for key, value := range obj {
		if value == nil {
			continue
		}
		if arr, isArr := value.([]any); isArr {
			out := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := scalarString(item); ok {
					out = append(out, s)
				}
			}
			fm[key] = out
			continue
		}
		if s, ok := scalarString(value); ok {
			fm[key] = s
		}
	}
	return fm
}

// scalarString convierte un valor YAML escalar a string, como String(value) en
// JS. Devuelve false para objetos/arrays anidados (no son escalares).
func scalarString(v any) (string, bool) {
	switch x := v.(type) {
	case nil:
		return "", false
	case map[string]any, []any:
		return "", false
	case string:
		return x, true
	case bool:
		return strconv.FormatBool(x), true
	default:
		// números (int/int64/float64), time.Time, etc.: su representación
		// textual natural, equivalente a String(value).
		return fmt.Sprint(x), true
	}
}

// splitFrontmatter parte el contenido en (texto de frontmatter, cuerpo).
// hasFM=false ⇔ no hay un bloque `---\n...\n---` al inicio: entonces body es el
// contenido ORIGINAL completo (no el normalizado), igual que thinkrail.
//
// El fmText se devuelve con los `\r` finales quitados por línea (siempre LF),
// porque va al parser YAML. El body se arma con Join("\n") sobre las líneas
// crudas, así que conserva byte a byte los `\r` del archivo CRLF: eso es lo que
// hace posible el round-trip sin pérdida del cuerpo.
func splitFrontmatter(content string) (fmText string, hasFM bool, body string) {
	normalized := strings.TrimPrefix(content, bom)
	lines := strings.Split(normalized, "\n")
	if strings.TrimSpace(lines[0]) != fence {
		return "", false, content
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == fence {
			end = i
			break
		}
	}
	if end == -1 {
		return "", false, content
	}
	body = strings.Join(lines[end+1:], "\n")
	fmLines := make([]string, 0, end-1)
	for _, line := range lines[1:end] {
		fmLines = append(fmLines, strings.TrimSuffix(line, "\r"))
	}
	return strings.Join(fmLines, "\n"), true, body
}

// ParseFile parte un archivo markdown en frontmatter normalizado + cuerpo.
// Si no hay fence, el frontmatter es nil. Si el YAML está malformado, el
// frontmatter es nil y el cuerpo vuelve a ser el archivo ENTERO (como thinkrail:
// un frontmatter roto no se interpreta como "cuerpo que empieza tras el fence").
func ParseFile(content string) ParsedFile {
	fmText, hasFM, body := splitFrontmatter(content)
	if !hasFM {
		return ParsedFile{Frontmatter: nil, Body: body}
	}
	var loaded any
	if err := yaml.Unmarshal([]byte(fmText), &loaded); err != nil {
		return ParsedFile{Frontmatter: nil, Body: content}
	}
	return ParsedFile{Frontmatter: toFrontmatter(loaded), Body: body}
}

// Scalar devuelve el valor de una clave sólo si es un string no vacío.
// Un mapa nil, una clave ausente, una lista o "" ⇒ ("", false).
func Scalar(fm Frontmatter, key string) (string, bool) {
	if v, ok := fm[key].(string); ok && v != "" {
		return v, true
	}
	return "", false
}

// List devuelve el valor de una clave como lista de strings: la lista tal cual
// si ya lo es, [valor] si es un escalar no vacío, o nil si no hay nada. Los
// callers sólo iteran el resultado, así que nil equivale a lista vacía.
func List(fm Frontmatter, key string) []string {
	switch v := fm[key].(type) {
	case []string:
		return v
	case string:
		if v != "" {
			return []string{v}
		}
	}
	return nil
}

// IsSpec es la definición de "esto es una spec": frontmatter no-nil con id y
// type presentes como escalares no vacíos. Es lo único que separa una spec del
// diario SDD (R1).
func IsSpec(fm Frontmatter) bool {
	if fm == nil {
		return false
	}
	for _, field := range IdentityFields {
		if _, ok := Scalar(fm, field); !ok {
			return false
		}
	}
	return true
}

// SerializeFrontmatter produce el bloque `---\n...\n---\n` para una spec NUEVA
// (camino create). Descarta strings vacíos y listas vacías, y emite las listas
// en estilo flow (`[a, b]`). No necesita ser byte-idéntico a nada: lo lossless
// (R5) es la EDICIÓN (frontmatter_edit.go), no la creación.
//
// Orden de claves: un map de Go no tiene orden de inserción como el objeto JS
// que thinkrail serializa, así que se emite FieldOrder primero (las presentes)
// y luego las claves desconocidas en orden estable. El único caller (create)
// arma siempre en FieldOrder, de modo que para ese camino el resultado es
// idéntico al de thinkrail.
func SerializeFrontmatter(fm Frontmatter) string {
	type pair struct {
		key string
		val any
	}
	clean := func(v any) (any, bool) {
		switch x := v.(type) {
		case []string:
			if len(x) > 0 {
				return x, true
			}
		case string:
			if x != "" {
				return x, true
			}
		default:
			if x != nil {
				return x, true
			}
		}
		return nil, false
	}

	pairs := make([]pair, 0, len(fm))
	seen := make(map[string]bool, len(fm))
	for _, key := range FieldOrder {
		if v, ok := clean(fm[key]); ok {
			pairs = append(pairs, pair{key, v})
			seen[key] = true
		}
	}
	// Claves fuera del vocabulario (forward-compat): orden estable alfabético.
	extra := make([]string, 0)
	for key := range fm {
		if !seen[key] {
			extra = append(extra, key)
		}
	}
	sortStrings(extra)
	for _, key := range extra {
		if v, ok := clean(fm[key]); ok {
			pairs = append(pairs, pair{key, v})
		}
	}

	if len(pairs) == 0 {
		return fence + "\n" + fence + "\n"
	}

	mapping := &yaml.Node{Kind: yaml.MappingNode}
	for _, p := range pairs {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: p.key}
		var valNode *yaml.Node
		if items, ok := p.val.([]string); ok {
			seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
			for _, item := range items {
				seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: item})
			}
			valNode = seq
		} else {
			valNode = &yaml.Node{Kind: yaml.ScalarNode, Value: toScalarString(p.val)}
		}
		mapping.Content = append(mapping.Content, keyNode, valNode)
	}
	return fence + "\n" + marshalNode(mapping) + fence + "\n"
}

// toScalarString saca el texto de un valor escalar para serializar. En el
// camino create los valores son siempre strings; el fallback cubre el resto.
func toScalarString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// marshalNode codifica un nodo YAML con indentación de 2 espacios. Envuelve en
// un DocumentNode para que el encoder lo emita como documento.
func marshalNode(node *yaml.Node) string {
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{node}}
	var sb strings.Builder
	enc := yaml.NewEncoder(&sb)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		// Un nodo que construimos nosotros no debería fallar al codificar; si
		// pasa, devolvemos lo que haya en vez de romper el create.
		_ = enc.Close()
		return sb.String()
	}
	_ = enc.Close()
	return sb.String()
}

// sortStrings ordena in-place una lista corta de claves desconocidas. Insertion
// sort para no arrastrar el import de sort a este archivo de vocabulario.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
