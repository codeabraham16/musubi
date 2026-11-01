// frontmatter_edit.go porta la mitad de edición lossless de core/parse.ts:
// FrontmatterEdit + updateFrontmatterText. Edita SÓLO el frontmatter y deja el
// cuerpo en prosa byte a byte idéntico (R5/R6). Conserva comentarios, orden de
// claves, BOM y line-ending (CRLF/LF) del original. La prosa se edita con los
// write/edit normales, nunca acá.

package specgraph

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// FrontmatterEdit es una edición quirúrgica del frontmatter. Los campos se
// aplican en orden: Set (escalares), Remove (borrar), luego AddList/RemoveList
// por cada campo lista, y por último SetList (reemplazo wholesale de un campo
// lista).
type FrontmatterEdit struct {
	Set        map[string]string
	Remove     []string
	AddList    map[string][]string
	RemoveList map[string][]string
	// SetList reemplaza un campo lista por EXACTAMENTE estos items (una lista
	// vacía borra la clave). A diferencia de AddList —que hace union-dedup por
	// string entero— acá el caller ya resolvió el conjunto final; es la vía que
	// usa el manejo anchor-aware de anchors (dedup por ref, D7), donde un
	// union-dedup por string dejaría `ruta#S` y `ruta#S=hash` como dos entradas.
	SetList map[string][]string
}

var (
	errNoFrontmatter          = errors.New("file has no frontmatter to update")
	errInvalidYAML            = errors.New("file frontmatter is not valid YAML")
	errCannotRename           = errors.New("cannot rename a spec's id via set")
	errLeavesInvalid          = errors.New("update would leave the file without a valid id and type")
	errAnchorAliasUnsupported = errors.New("update would break a YAML anchor or alias; edit this frontmatter by hand")
)

// UpdateFrontmatterText aplica una edición y devuelve el contenido reescrito, o
// un error si no hay frontmatter, el YAML es inválido, o la edición violaría una
// de las cuatro guardas (renombrar id, editar una lista con set, borrar id/type,
// o dejar el archivo sin id+type válidos).
func UpdateFrontmatterText(fileText string, edit FrontmatterEdit) (string, error) {
	fmText, hasFM, body := splitFrontmatter(fileText)
	if !hasFM {
		return "", errNoFrontmatter
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(fmText), &doc); err != nil {
		// El error del parser no agrega nada accionable al caller; devolvemos el
		// contrato estable de thinkrail. (No hay cadena que envolver con %w.)
		return "", errInvalidYAML
	}
	// Tiene que ser un documento cuyo contenido raíz sea un mapa. Frontmatter
	// vacío (`---\n---`) deja el nodo en cero: también es inválido para editar.
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return "", errInvalidYAML
	}
	mapping := doc.Content[0]

	// 1) Set escalares. Se ordenan las claves para que, ante dos claves
	//    ofensivas, el error reportado sea determinista (un map de Go no tiene
	//    orden de inserción como el objeto JS de thinkrail).
	setKeys := sortedKeys(edit.Set)
	for _, key := range setKeys {
		if key == FieldID {
			return "", errCannotRename
		}
		if contains(ListFields, key) {
			return "", fmt.Errorf("use addList/removeList to edit the list field %q", key)
		}
		fmSetScalar(mapping, key, edit.Set[key])
	}

	// 2) Remove. No se puede borrar la identidad (id/type).
	for _, key := range edit.Remove {
		if contains(IdentityFields, key) {
			return "", fmt.Errorf("cannot remove protected field %q", key)
		}
		fmDelete(mapping, key)
	}

	// 3) Campos lista: union-dedup para agregar, filtro para quitar. Se recorre
	//    ListFields (orden fijo) para que el resultado sea determinista.
	for _, field := range ListFields {
		if add := edit.AddList[field]; len(add) > 0 {
			fmSetList(mapping, field, unionDedup(fmDocList(mapping, field), add))
		}
		if remove := edit.RemoveList[field]; len(remove) > 0 {
			next := filterOut(fmDocList(mapping, field), remove)
			if len(next) > 0 {
				fmSetList(mapping, field, next)
			} else {
				fmDelete(mapping, field)
			}
		}
	}

	// 3b) SetList: reemplazo wholesale de un campo lista. El caller ya resolvió el
	//     conjunto final (p. ej. anchors deduplicadas por ref con el digest nuevo
	//     ganando, D7), así que acá sólo se escribe. Corre DESPUÉS de AddList/
	//     RemoveList y los pisa, y se recorre ListFields (orden fijo) para
	//     determinismo. Una lista vacía borra la clave, igual que un RemoveList
	//     que vacía el campo.
	for _, field := range ListFields {
		if items, ok := edit.SetList[field]; ok {
			if len(items) > 0 {
				fmSetList(mapping, field, items)
			} else {
				fmDelete(mapping, field)
			}
		}
	}

	// 4) Guarda final: la edición no puede dejar el archivo sin id+type válidos.
	for _, field := range IdentityFields {
		if _, ok := fmDocScalar(mapping, field); !ok {
			return "", errLeavesInvalid
		}
	}

	// Se codifica el DOCUMENTO entero (no sólo el mapa) para conservar un
	// comentario que haya quedado colgado del documento. Nota: a diferencia de
	// thinkrail, NO se fuerza flow sobre TODA lista, sólo sobre las que esta
	// edición tocó (fmSetList ya las pone en flow). Así una lista en bloque
	// escrita a mano con comentarios por item, en una clave NO modificada,
	// conserva sus comentarios (R5). Las specs creadas por la tool ya son flow,
	// así que para ese caso el resultado es idéntico al de thinkrail.
	rendered, err := marshalDoc(&doc)
	if err != nil {
		return "", fmt.Errorf("re-serializar frontmatter: %w", err)
	}
	// Guarda lossless (R5): fmSetScalar re-infiere el escalar y por eso limpia
	// Anchor/Alias del nodo que toca. Si ese nodo era un &anchor referenciado por
	// un *alias en otra clave, el alias queda colgado y el YAML re-emitido ya no
	// parsea (medido: "unknown anchor 'b' referenced"). Antes que escribir un
	// archivo roto, se re-parsea lo emitido y se rechaza la edición.
	var check yaml.Node
	if yaml.Unmarshal([]byte(rendered), &check) != nil {
		return "", errAnchorAliasUnsupported
	}
	block := fence + "\n" + rendered + fence + "\n"
	if lineEnding(fileText) == crlf {
		block = strings.ReplaceAll(block, "\n", crlf)
	}

	prefix := ""
	if strings.HasPrefix(fileText, bom) {
		prefix = bom
	}
	return prefix + block + body, nil
}

// lineEnding devuelve el terminador de línea del archivo según su PRIMER salto:
// CRLF si el \n viene precedido de \r, LF en otro caso. Igual que thinkrail,
// un \n en el índice 0 (first>0 es falso) cuenta como LF.
func lineEnding(text string) string {
	i := strings.IndexByte(text, '\n')
	if i > 0 && text[i-1] == '\r' {
		return crlf
	}
	return "\n"
}

// marshalDoc codifica un DocumentNode con indentación de 2 espacios,
// preservando comentarios/orden/estilos capturados al parsear.
func marshalDoc(doc *yaml.Node) (string, error) {
	var sb strings.Builder
	enc := yaml.NewEncoder(&sb)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		_ = enc.Close()
		return "", err
	}
	if err := enc.Close(); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// --- helpers sobre el MappingNode (Content es [k0,v0,k1,v1,...]) ---

func fmFindKey(mapping *yaml.Node, key string) int {
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return i
		}
	}
	return -1
}

func fmGet(mapping *yaml.Node, key string) *yaml.Node {
	if i := fmFindKey(mapping, key); i >= 0 {
		return mapping.Content[i+1]
	}
	return nil
}

// fmSetScalar fija/sobrescribe una clave escalar. Si la clave existe, MUTA el
// nodo valor en vez de reemplazarlo: así conserva cualquier comentario colgado
// de ese valor. Re-infiere tag y estilo para el valor nuevo (el emisor cita si
// hace falta). Si no existe, agrega el par al final.
func fmSetScalar(mapping *yaml.Node, key, value string) {
	if i := fmFindKey(mapping, key); i >= 0 {
		v := mapping.Content[i+1]
		v.Kind = yaml.ScalarNode
		v.Value = value
		v.Tag = ""
		v.Style = 0
		v.Content = nil
		v.Anchor = ""
		v.Alias = nil
		return
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Value: value},
	)
}

// fmSetList reemplaza el valor de un campo lista por un seq en estilo flow.
// Copia los comentarios del valor anterior para no perderlos.
func fmSetList(mapping *yaml.Node, key string, items []string) {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, it := range items {
		seq.Content = append(seq.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: it})
	}
	if i := fmFindKey(mapping, key); i >= 0 {
		old := mapping.Content[i+1]
		seq.HeadComment, seq.LineComment, seq.FootComment = old.HeadComment, old.LineComment, old.FootComment
		mapping.Content[i+1] = seq
		return
	}
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Value: key},
		seq,
	)
}

func fmDelete(mapping *yaml.Node, key string) {
	if i := fmFindKey(mapping, key); i >= 0 {
		mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
	}
}

// fmDocScalar espeja docScalar de thinkrail: valor escalar, no-null, no vacío.
func fmDocScalar(mapping *yaml.Node, key string) (string, bool) {
	v := fmGet(mapping, key)
	if v != nil && v.Kind == yaml.ScalarNode && v.Value != "" && v.Tag != "!!null" {
		return v.Value, true
	}
	return "", false
}

// fmDocList espeja docList de thinkrail: items escalares de un seq (filtrando
// null), o [valor] si es un escalar no vacío.
func fmDocList(mapping *yaml.Node, key string) []string {
	v := fmGet(mapping, key)
	if v == nil {
		return nil
	}
	if v.Kind == yaml.SequenceNode {
		out := make([]string, 0, len(v.Content))
		for _, item := range v.Content {
			if item.Kind == yaml.ScalarNode && item.Tag != "!!null" {
				out = append(out, item.Value)
			}
		}
		return out
	}
	if v.Kind == yaml.ScalarNode && v.Value != "" && v.Tag != "!!null" {
		return []string{v.Value}
	}
	return nil
}

// --- utilidades puras ---

// unionDedup = [...new Set([...existing, ...add])]: existentes primero (sin
// duplicados), luego los nuevos que no estaban.
func unionDedup(existing, add []string) []string {
	seen := make(map[string]bool, len(existing)+len(add))
	out := make([]string, 0, len(existing)+len(add))
	for _, v := range existing {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	for _, v := range add {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func filterOut(items, remove []string) []string {
	rm := make(map[string]bool, len(remove))
	for _, r := range remove {
		rm[r] = true
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if !rm[it] {
			out = append(out, it)
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sortStrings(out)
	return out
}
