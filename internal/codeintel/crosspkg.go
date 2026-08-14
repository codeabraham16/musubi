package codeintel

import (
	"path"
	"sort"
	"strings"
)

// crosspkg.go RESUELVE las llamadas cross-paquete que `DerivePackage` dejó pendientes (Track 20 ·
// F8-A). Derivar un paquete no alcanza para resolverlas: el destino vive en otro paquete, así que
// el archivo —y por lo tanto la node_key— del símbolo llamado está fuera de su alcance.
//
// La pieza central es la SEPARACIÓN entre el resolver y el ÍNDICE. Hay dos caminos que necesitan
// resolver y consiguen los símbolos de forma distinta: el índice COMPLETO los tiene en memoria
// porque acaba de derivarlos, y el refresco INCREMENTAL tiene que ir a buscarlos a la base. Si cada
// camino trajera su propia lógica de resolución habría dos, y el incremental —el menos mirado— se
// pudriría en silencio. Con la interfaz, los dos ejecutan EXACTAMENTE lo mismo.
//
// Todo acá es model-free y sin I/O: aritmética de strings sobre estructuras en memoria.

// SymbolIndex resuelve (import path, nombre de func) → node_key de la func top-level.
//
// ⚠️ CONTRATO: ok=false significa LAS DOS COSAS —no existe, o hay MÁS DE UNO—, y es a propósito.
// Colapsarlas acá impide que una implementación "resuelva" una ambigüedad eligiendo un candidato:
// la política del grafo es que lo que no se puede resolver se OMITE, nunca se inventa.
type SymbolIndex interface {
	LookupFunc(importPath, name string) (key string, ok bool)
}

// ModuleIndex es la implementación EN MEMORIA de SymbolIndex: se arma con los nodos ya derivados
// (el camino del índice completo). El camino incremental arma otra implementación desde la base.
type ModuleIndex struct {
	modulePath string
	// byDirName mapea dir+"\x00"+nombre → node_keys candidatas. Es un SLICE y no una key sola
	// justamente para poder DETECTAR la ambigüedad: un map[nombre]key se sobrescribe en silencio y
	// devolvería el último visto, que es exactamente la resolución inventada que la spec prohíbe.
	byDirName map[string][]string
}

// NewModuleIndex crea un índice vacío para un módulo dado (el path de su go.mod).
func NewModuleIndex(modulePath string) *ModuleIndex {
	return &ModuleIndex{modulePath: modulePath, byDirName: map[string][]string{}}
}

// Add incorpora nodos al índice. Sólo entran las funcs TOP-LEVEL: son las únicas invocables como
// `paquete.Func(...)` desde afuera. Los métodos se llaman por selector sobre un valor y resolverlos
// exigiría inferencia de tipos (fuera de alcance); los tipos y constantes no son call-sites.
func (m *ModuleIndex) Add(nodes []Node) {
	for _, n := range nodes {
		if n.Kind != KindFunc || n.Path == "" {
			continue
		}
		k := indexKey(dirOfPath(n.Path), n.Name)
		m.byDirName[k] = append(m.byDirName[k], n.Key)
	}
}

// LookupFunc implementa SymbolIndex. Devuelve ok sólo si hay EXACTAMENTE un candidato.
func (m *ModuleIndex) LookupFunc(importPath, name string) (string, bool) {
	dir, ok := DirForImportPath(importPath, m.modulePath)
	if !ok {
		return "", false
	}
	keys := m.byDirName[indexKey(dir, name)]
	if len(keys) != 1 {
		return "", false // 0 = no existe · >1 = ambiguo. Las dos se omiten.
	}
	return keys[0], true
}

// DirForImportPath traduce un import path IN-MODULE al directorio relativo donde vive su paquete.
// Es pura aritmética de strings sobre datos que ya tenemos: no lee el go.mod de nuevo ni toca el
// filesystem. Devuelve ok=false si el import no pertenece al módulo (stdlib o terceros), que es la
// guarda que impide que una llamada a `strings.HasPrefix` se lea como cross-paquete propio.
//
// El módulo raíz mapea a "." porque así es como el resto del indexador nombra el directorio de la
// raíz del repo (ver packageDirOf en la capa MCP).
func DirForImportPath(importPath, modulePath string) (string, bool) {
	if !inModule(importPath, modulePath) {
		return "", false
	}
	if importPath == modulePath {
		return ".", true
	}
	return strings.TrimPrefix(importPath, modulePath+"/"), true
}

// dirOfPath devuelve el directorio de una ruta de nodo, siempre con separadores "/" (las claves del
// grafo están normalizadas así). Un archivo en la raíz da ".".
func dirOfPath(p string) string {
	return path.Dir(strings.ReplaceAll(p, "\\", "/"))
}

func indexKey(dir, name string) string { return dir + "\x00" + name }

// ResolveCrossPackageCalls convierte pendientes en aristas CALLS consultando el índice. Lo que no
// resuelve se OMITE en silencio —igual que el pase 2 intra-paquete de DerivePackage—, porque una
// arista inventada es peor que una arista ausente: el grafo se usa para decidir qué se rompe si
// cambiás algo, y un falso positivo manda a revisar código que no tiene nada que ver.
//
// SrcPath sale del pendiente, o sea del archivo del CALLER. Eso NO es un detalle: el refresco borra
// las aristas por SrcPath y las re-inserta, así que ponerle el archivo del caller es lo que hace que
// re-indexar el paquete que cambió recalcule sus propias aristas cross y el invariante incremental
// se mantenga intacto.
func ResolveCrossPackageCalls(pending []PendingCall, idx SymbolIndex) []Edge {
	if len(pending) == 0 || idx == nil {
		return nil
	}
	var out []Edge
	seen := map[string]bool{}
	for _, pc := range pending {
		target, ok := idx.LookupFunc(pc.ImportPath, pc.Name)
		if !ok {
			continue
		}
		if target == pc.FromKey {
			continue // auto-llamada por el nombre del propio paquete: no aporta nada al grafo
		}
		id := pc.FromKey + "\x00" + target
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, Edge{
			FromKey:    pc.FromKey,
			ToKey:      target,
			Kind:       EdgeCalls,
			Confidence: 1.0,
			Provenance: ProvExtracted,
			SrcPath:    pc.SrcPath,
		})
	}
	return sortEdges(out)
}

// ImportPathsOf devuelve los import paths DISTINTOS de un conjunto de pendientes, ordenados. El
// camino incremental lo usa para saber qué paquetes tiene que ir a buscar a la base: así la consulta
// queda acotada por los imports in-module del paquete que se está refrescando, no por el repo entero.
func ImportPathsOf(pending []PendingCall) []string {
	seen := map[string]bool{}
	var out []string
	for _, pc := range pending {
		if seen[pc.ImportPath] {
			continue
		}
		seen[pc.ImportPath] = true
		out = append(out, pc.ImportPath)
	}
	sort.Strings(out)
	return out
}
