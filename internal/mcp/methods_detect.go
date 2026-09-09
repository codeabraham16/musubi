package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"musubi/internal/codeintel"
	"musubi/internal/memory"
)

// methods_detect.go implementa musubi_detect_changes: la inteligencia de cambios de
// código, model-free. Corre `git diff`, y para cada archivo tocado RE-DERIVA sus símbolos
// del contenido ACTUAL (nunca de datos guardados), así el diff (coordenadas del estado
// nuevo) y los símbolos viven en el mismo sistema de coordenadas y jamás se desalinean.
// Cruza además con la memoria de código (gists stale por fingerprint) y con la memoria de
// decisiones (observaciones que referencian el archivo), para responder no solo "qué
// cambió" sino "qué gist/decisión quedó potencialmente obsoleto". Es de solo-lectura.

// fileChange es el reporte por archivo de detect_changes.
type fileChange struct {
	Path           string   `json:"path"`
	ChangeType     string   `json:"change_type"`
	ChangedSymbols []string `json:"changed_symbols"`
	GistStale      bool     `json:"gist_stale"`
	RelatedMemory  []string `json:"related_memory"`
}

// detectReport es la salida compacta de detect_changes.
type detectReport struct {
	Files    []fileChange        `json:"files"`
	Summary  string              `json:"summary"`
	Revision codeintel.Veredicto `json:"revision"`
}

// Cotas del sondeo al grafo. detect_changes corre seguido y el BFS de impacto es lo mas caro
// que hace: sin tope, un cambio que toca cien simbolos dispara cien recorridos.
const (
	topeSimbolosSondeados = 25
	profundidadRadio      = 3
	topeNodosRadio        = 200
)

// runnerFor devuelve el Runner inyectado (tests) o uno real sobre projectPath.
func (s *McpServer) runnerFor() codeintel.Runner {
	if s.gitRunner != nil {
		return s.gitRunner
	}
	return codeintel.NewGitRunner(s.projectPath)
}

func (s *McpServer) toolDetectChanges(ctx context.Context, raw json.RawMessage) (interface{}, *RpcError) {
	var args struct {
		Ref    string `json:"ref"`
		Staged bool   `json:"staged"`
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, rpcErrorf(codeInvalidParams, "Invalid arguments: %v", err)
	}

	out, err := s.runnerFor().Diff(args.Ref, args.Staged)
	if err != nil {
		return nil, rpcErrorf(codeInternalError, "no se pudo obtener el diff de git: %v", err)
	}
	diffs := codeintel.ParseUnifiedDiff(out)

	// Aislamiento por proyecto (Track 18): detect_changes es una superficie de LECTURA
	// (readOnly) que cruza el diff con la memoria compartida (código + observaciones). Deriva
	// el scope de la credencial UNA vez y pásalo a las dos consultas de memoria (gistStale /
	// relatedMemory); sin esto un reader vería gists/topic_keys de otros proyectos (bleed de
	// metadata, misma clase que el HIGH de aislamiento de Track 17). Ausencia de scope ⇒ federado.
	scoped := s.scopedCtx(ctx)

	report := detectReport{Files: make([]fileChange, 0, len(diffs))}
	changedFiles, changedSymbols := 0, 0
	// Las claves de NODO de los simbolos tocados, que es distinto de su Ref: el grafo indexa
	// por `path#kind:nombre`. Se juntan mientras se recorre para no releer los archivos.
	var clavesGrafo []string
	for _, fd := range diffs {
		if fd.Binary {
			continue
		}
		fc := fileChange{
			Path:           fd.Path,
			ChangeType:     fd.ChangeType,
			ChangedSymbols: []string{},
			RelatedMemory:  []string{},
		}
		key := memory.NormalizeCodePath(s.projectPath, fd.Path)

		// Símbolos + staleness: solo tienen sentido si el archivo existe (no borrado).
		//
		// SE ARMAN DOS LISTAS PORQUE SON DOS TRABAJOS. Lo que se REPORTA va calificado
		// (`DbEngine.Close`): es la IDENTIDAD del símbolo, y es la clave que el que lee esto va a
		// copiar para anclar una observación. Lo que se BUSCA va pelado (`Close`): son términos
		// para FTS, y el punto del calificador sólo parte el token y ensucia la consulta.
		var paraBuscar []string
		if fd.ChangeType != codeintel.ChangeDeleted {
			if content, rerr := s.readProjectFile(fd.Path); rerr == nil {
				syms := codeintel.ExtractSymbols(fd.Path, content)
				for _, sym := range codeintel.SymbolsInRanges(syms, fd.NewRanges) {
					fc.ChangedSymbols = append(fc.ChangedSymbols, sym.Ref())
					paraBuscar = append(paraBuscar, sym.Name)
					clavesGrafo = append(clavesGrafo, codeintel.SymbolKey(fd.Path, sym.Kind, sym.Ref()))
				}
				fc.GistStale = s.gistStale(scoped, key, fd.Path)
			}
		}

		fc.RelatedMemory = s.relatedMemory(scoped, key, paraBuscar)
		changedFiles++
		changedSymbols += len(fc.ChangedSymbols)
		report.Files = append(report.Files, fc)
	}

	report.Revision = s.profundidadDe(scoped, diffs, changedSymbols, clavesGrafo)
	report.Summary = fmt.Sprintf("%d archivo(s) cambiados, %d símbolo(s) afectados. Revisión sugerida: %s (%d/13 puntos) — %d juez/jueces, %d ronda(s), quórum %d.",
		changedFiles, changedSymbols, report.Revision.Nivel, report.Revision.Puntos,
		report.Revision.Panel.Jueces, report.Revision.Panel.Rondas, report.Revision.Panel.Quorum)
	return jsonResult(report)
}

// readProjectFile lee el contenido actual de un path relativo a la raíz del proyecto.
func (s *McpServer) readProjectFile(path string) (string, error) {
	full := path
	if !filepath.IsAbs(full) {
		full = filepath.Join(s.projectPath, path)
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// gistStale indica si hay un gist guardado para el archivo cuyo fingerprint ya no coincide
// con el contenido actual (el gist quedó desactualizado). Usa la variante ctx-aware para
// acotar la lectura al proyecto de la credencial (Track 18): sin scope, GetCodeMemory hacía
// `WHERE path=? LIMIT 1` y podía comparar el fingerprint del archivo LOCAL contra el gist de
// OTRO proyecto (tras la migración v13 varias filas comparten path) ⇒ staleness falso + fuga.
func (s *McpServer) gistStale(ctx context.Context, key, path string) bool {
	cm, ok, err := s.engine.GetCodeMemoryCtx(ctx, key)
	if err != nil || !ok || cm.Fingerprint == "" {
		return false
	}
	current, ferr := memory.FileFingerprint(s.projectPath, path)
	return ferr == nil && current != "" && current != cm.Fingerprint
}

// relatedMemory busca observaciones que referencian el archivo (por path) y sus símbolos
// cambiados, devolviendo sus topic_keys deduplicados. Es keyword (FTS), barato y preciso;
// no usa embeddings para no depender de un proveedor ni traer ruido semántico.
func (s *McpServer) relatedMemory(ctx context.Context, key string, symbols []string) []string {
	terms := []string{key, filepath.Base(key)}
	terms = append(terms, symbols...)
	seen := map[string]bool{}
	var out []string
	for _, term := range terms {
		if strings.TrimSpace(term) == "" {
			continue
		}
		obs, err := s.engine.SearchObservationsFTS(ctx, term, 5)
		if err != nil {
			continue
		}
		for _, o := range obs {
			ref := o.TopicKey
			if ref == "" {
				ref = o.ID
			}
			if ref == "" || seen[ref] {
				continue
			}
			seen[ref] = true
			out = append(out, ref)
		}
	}
	sort.Strings(out)
	return out
}

// profundidadDe completa las señales del diff con las dos que salen del grafo y devuelve el
// veredicto de cuánta revisión pide el cambio.
//
// 🔴 EL CERO DEL RADIO ES AMBIGUO, Y ACÁ SE DESAMBIGUA. Preguntarle al grafo por un símbolo que
// no tiene nodo devuelve cero callers — exactamente lo mismo que preguntarle por uno al que no
// llama nadie. Ese cero es el valor de fallo disfrazado de valor tranquilizador: si se le cree,
// un cambio en código sin indexar puntúa como el más inocuo posible.
//
// Por eso antes de creerle a un cero se comprueba que el nodo EXISTA. Si no existe —o si el
// archivo ni siquiera es indexable, o si el grafo no contesta— el radio queda declarado CIEGO y
// el nivel no puede bajar a mínima. El motivo viaja en el veredicto: subir el nivel en silencio
// dejaría al que lee sin poder distinguir «el cambio es grande» de «el índice está flojo».
func (s *McpServer) profundidadDe(ctx context.Context, diffs []codeintel.FileDiff, simbolos int, claves []string) codeintel.Veredicto {
	sen := codeintel.SenalesDelDiff(diffs, simbolos)

	cobertura, err := s.engine.GraphFileFingerprintsCtx(ctx)
	if err != nil {
		sen.RadioCiego = true
		sen.MotivoCiego = "el grafo no contestó por su cobertura"
		return codeintel.Profundidad(sen)
	}

	// 1) Cobertura por ARCHIVO: qué tocamos que el grafo nunca vio.
	noIndexables, archivosSinNodo := 0, 0
	for _, fd := range diffs {
		if fd.Binary {
			continue
		}
		// Un archivo que NO PUEDE tener radio -un README, un YAML- no deja el radio ciego:
		// no hay nada que el grafo pudiera haber sabido de el. Lo que si cuenta es codigo
		// que este build no indexa, porque ahi el cero significa "no pude medir".
		if !codeintel.PuedeTenerRadio(fd.Path) {
			continue
		}
		if !codeintel.IndexableForGraph(fd.Path) {
			noIndexables++
			continue
		}
		if _, visto := cobertura[fd.Path]; !visto {
			archivosSinNodo++
		}
	}

	// 2) Radio por SÍMBOLO, sólo sobre los que sí tienen nodo.
	radio := map[string]bool{}
	simbolosSinNodo, sondeados, truncado := 0, 0, 0
	for _, k := range claves {
		if sondeados >= topeSimbolosSondeados {
			truncado = len(claves) - sondeados
			break
		}
		if _, hay, gerr := s.engine.GetGraphNodeCtx(ctx, k); gerr != nil || !hay {
			simbolosSinNodo++
			continue
		}
		sondeados++
		callers, cerr := s.engine.GraphImpactCtx(ctx, k, profundidadRadio, topeNodosRadio)
		if cerr != nil {
			simbolosSinNodo++
			continue
		}
		for _, c := range callers {
			radio[c] = true
		}
	}

	sen.CallersEnRadio = len(radio)
	sen.PaquetesEnRadio = len(paquetesDe(radio))

	// 3) El piso de honestidad, con su motivo concreto.
	var faltas []string
	if noIndexables > 0 {
		faltas = append(faltas, fmt.Sprintf("%d archivo(s) de codigo que el grafo no indexa", noIndexables))
	}
	if archivosSinNodo > 0 {
		faltas = append(faltas, fmt.Sprintf("%d archivo(s) sin un solo nodo en el grafo", archivosSinNodo))
	}
	if simbolosSinNodo > 0 {
		faltas = append(faltas, fmt.Sprintf("%d símbolo(s) sin nodo", simbolosSinNodo))
	}
	if len(faltas) > 0 {
		sen.RadioCiego = true
		sen.MotivoCiego = strings.Join(faltas, ", ")
	}

	v := codeintel.Profundidad(sen)
	// El tope NO se calla. Un recorte silencioso se lee como «se miró todo» justo cuando es lo
	// contrario, y el que decide el panel merece saber que el radio está sub-contado.
	if truncado > 0 {
		v.Motivos = append(v.Motivos, fmt.Sprintf("el radio está SUB-CONTADO: se sondearon %d símbolos y quedaron %d afuera por el tope", sondeados, truncado))
	}
	return v
}

// paquetesDe agrupa las claves de nodo del radio por paquete. Una clave es `path#kind:nombre`,
// así que el paquete es el directorio de la parte anterior al `#`.
func paquetesDe(radio map[string]bool) map[string]bool {
	pkgs := map[string]bool{}
	for k := range radio {
		ruta := k
		if i := strings.Index(ruta, "#"); i >= 0 {
			ruta = ruta[:i]
		}
		if i := strings.LastIndex(ruta, "/"); i >= 0 {
			pkgs[ruta[:i]] = true
		} else {
			pkgs[ruta] = true
		}
	}
	return pkgs
}
