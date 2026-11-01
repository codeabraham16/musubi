// validate.go porta core/validate.ts (enlaces colgantes, ids duplicados, ciclos
// de parent) y agrega el delta model-free de Musubi UnknownTypes (type fuera del
// vocabulario, R3/R12). El OTRO delta de R12 —archivos NOMBRADOS SPEC.md que no
// llegan a ser spec— no vive acá: el core sólo ve entradas que YA son specs, así
// que eso lo aporta el índice (RejectedSpecs) y lo compone el handler (D9).

package specgraph

import "strings"

type DanglingLink struct {
	From     string
	FromPath string
	Kind     string
	Target   string
}

type DuplicateID struct {
	ID    string
	Paths []string
}

type ParentCycle struct {
	IDs []string
}

// ValidationReport son los 3 chequeos portados de thinkrail. Los deltas de
// Musubi (UnknownTypes, RejectedSpecs) se reportan por separado en el handler.
type ValidationReport struct {
	DanglingLinks []DanglingLink
	DuplicateIDs  []DuplicateID
	ParentCycles  []ParentCycle
}

// UnknownType es el delta R3/R12: un nodo cuyo `type` no está en SpecTypes.
type UnknownType struct {
	ID   string
	Path string
	Type string
}

// nulSep separa ids al armar la clave de dedup de un ciclo. Se construye por
// bytes para no meter un escape \u ni un NUL literal en el fuente (lección del
// BOM en parse.go). Un id no puede contener NUL, así que no hay colisión.
var nulSep = string([]byte{0})

// findParentCycles recorre la cadena de parent de cada nodo buscando ciclos.
// `seen` es global entre nodos de arranque: una cadena ya visitada no se
// reexplora. Un ciclo se deduplica por su conjunto de ids ordenado, así que el
// mismo anillo alcanzado desde distintos arranques se reporta una sola vez.
func findParentCycles(graph *SpecGraph) []ParentCycle {
	cycles := make([]ParentCycle, 0)
	cycleKeys := make(map[string]bool)
	seen := make(map[string]bool)

	for _, startID := range graph.NodeOrder {
		if seen[startID] {
			continue
		}
		path := make([]string, 0)
		onPath := make(map[string]int)
		current := startID
		for {
			node, exists := graph.Nodes[current]
			if !exists { // parent apunta a un id inexistente: lo ve danglingLinks
				break
			}
			if idx, on := onPath[current]; on {
				ring := append([]string(nil), path[idx:]...)
				key := ringKey(ring)
				if !cycleKeys[key] {
					cycleKeys[key] = true
					cycles = append(cycles, ParentCycle{IDs: ring})
				}
				break
			}
			if seen[current] {
				break
			}
			onPath[current] = len(path)
			path = append(path, current)
			parent, hasParent := Scalar(node.Frontmatter, FieldParent)
			if !hasParent {
				break
			}
			current = parent
		}
		for _, id := range path {
			seen[id] = true
		}
	}
	return cycles
}

func ringKey(ring []string) string {
	sorted := append([]string(nil), ring...)
	sortStrings(sorted)
	return strings.Join(sorted, nulSep)
}

// ValidateGraph corre los 3 chequeos estructurales de thinkrail. Recorre
// NodeOrder (no el map) para que la salida sea determinista, y toma los ids
// duplicados en el mismo orden de primer avistaje que thinkrail.
func ValidateGraph(graph *SpecGraph) ValidationReport {
	dangling := make([]DanglingLink, 0)
	for _, id := range graph.NodeOrder {
		node := graph.Nodes[id]
		for _, kind := range LinkKinds {
			for _, target := range linkTargets(node.Frontmatter, kind) {
				if _, ok := graph.Nodes[target]; !ok {
					dangling = append(dangling, DanglingLink{
						From: node.ID, FromPath: node.Path, Kind: kind, Target: target,
					})
				}
			}
		}
	}

	// NodeOrder y el orden de primer avistaje de duplicateIds coinciden (ambos
	// se pueblan en la misma primera aparición del id), así que esto reproduce
	// el orden del Map que itera thinkrail.
	duplicates := make([]DuplicateID, 0, len(graph.DuplicateIds))
	for _, id := range graph.NodeOrder {
		if paths, ok := graph.DuplicateIds[id]; ok {
			duplicates = append(duplicates, DuplicateID{ID: id, Paths: paths})
		}
	}

	return ValidationReport{
		DanglingLinks: dangling,
		DuplicateIDs:  duplicates,
		ParentCycles:  findParentCycles(graph),
	}
}

// IsValid es true si los 3 chequeos portados no encontraron nada. OJO: no
// contempla los deltas de Musubi (UnknownTypes/RejectedSpecs); el handler los
// suma por separado para decidir el veredicto final.
func IsValid(report ValidationReport) bool {
	return len(report.DanglingLinks) == 0 &&
		len(report.DuplicateIDs) == 0 &&
		len(report.ParentCycles) == 0
}

// UnknownTypes es el delta R3/R12: nodos cuyo `type` no pertenece a SpecTypes.
// Un type desconocido NO impide que el archivo sea spec (R1 manda), pero se
// reporta como problema.
func UnknownTypes(graph *SpecGraph) []UnknownType {
	out := make([]UnknownType, 0)
	for _, id := range graph.NodeOrder {
		node := graph.Nodes[id]
		if !contains(SpecTypes, node.Type) {
			out = append(out, UnknownType{ID: node.ID, Path: node.Path, Type: node.Type})
		}
	}
	return out
}
