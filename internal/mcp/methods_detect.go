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
	Files   []fileChange `json:"files"`
	Summary string       `json:"summary"`
}

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

	report := detectReport{Files: make([]fileChange, 0, len(diffs))}
	changedFiles, changedSymbols := 0, 0
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
		if fd.ChangeType != codeintel.ChangeDeleted {
			if content, rerr := s.readProjectFile(fd.Path); rerr == nil {
				syms := codeintel.ExtractSymbols(fd.Path, content)
				for _, sym := range codeintel.SymbolsInRanges(syms, fd.NewRanges) {
					fc.ChangedSymbols = append(fc.ChangedSymbols, sym.Name)
				}
				fc.GistStale = s.gistStale(key, fd.Path)
			}
		}

		fc.RelatedMemory = s.relatedMemory(ctx, key, fc.ChangedSymbols)
		changedFiles++
		changedSymbols += len(fc.ChangedSymbols)
		report.Files = append(report.Files, fc)
	}

	report.Summary = fmt.Sprintf("%d archivo(s) cambiados, %d símbolo(s) afectados.", changedFiles, changedSymbols)
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
// con el contenido actual (el gist quedó desactualizado).
func (s *McpServer) gistStale(key, path string) bool {
	cm, ok, err := s.engine.GetCodeMemory(key)
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
