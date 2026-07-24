package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"musubi/internal/codeintel"
	"musubi/internal/memory"
)

// methods_codegraph.go dispara el poblado del GRAFO DE CÓDIGO (Track 20 · F1). En F1 NO hay
// tool pública ni hook que responda consultas (eso es F2): el grafo se puebla como EFECTO del
// guardado de un gist de código `.go`, derivándolo del AST y persistiéndolo scopeado por la
// credencial. La derivación es model-free (internal/codeintel, sin fs/db); acá se aportan el
// filesystem, los fingerprints por archivo y la atribución por proyecto.

// moduleImportPath lee la línea `module X` del go.mod del proyecto (model-free). Sirve para
// clasificar imports in-module vs. externos. "" si no lo encuentra (se pierde el flag external,
// no rompe nada).
func (s *McpServer) moduleImportPath() string {
	data, err := os.ReadFile(filepath.Join(s.projectPath, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

// refreshCodeGraphForPackage deriva el grafo del paquete (directorio relativo a la raíz, con
// separadores "/") y lo persiste, atribuido al proyecto de la credencial. Es best-effort: si el
// directorio no existe o nada deriva, devuelve nil/err sin efectos, y el llamador (save_code) no
// debe fallar por esto. Deriva del contenido ACTUAL de cada archivo (derivar-no-desfasar) y
// estampa el fingerprint de ese mismo snapshot en cada fila.
func (s *McpServer) refreshCodeGraphForPackage(ctx context.Context, dir string) error {
	absDir := dir
	if !filepath.IsAbs(absDir) {
		absDir = filepath.Join(s.projectPath, dir)
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return err
	}

	files := map[string]string{}
	fps := map[string]string{}
	var fileKeys []string
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".go") {
			continue
		}
		rel := filepath.Join(dir, ent.Name())
		key := memory.NormalizeCodePath(s.projectPath, filepath.Join(s.projectPath, rel))
		content, rerr := s.readProjectFile(rel)
		if rerr != nil {
			continue
		}
		files[key] = content
		if fp, ferr := memory.FileFingerprint(s.projectPath, rel); ferr == nil {
			fps[key] = fp
		}
		fileKeys = append(fileKeys, key)
	}
	if len(files) == 0 {
		return nil
	}

	g := codeintel.DerivePackage(dir, files, s.moduleImportPath())

	nodes := make([]memory.GraphNode, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		nodes = append(nodes, memory.GraphNode{
			Key: n.Key, Kind: n.Kind, Name: n.Name, Path: n.Path,
			StartLine: n.StartLine, EndLine: n.EndLine, External: n.External,
			SrcFingerprint: fps[n.Path],
		})
	}
	edges := make([]memory.GraphEdge, 0, len(g.Edges))
	for _, ed := range g.Edges {
		edges = append(edges, memory.GraphEdge{
			FromKey: ed.FromKey, ToKey: ed.ToKey, Kind: ed.Kind,
			Confidence: ed.Confidence, Provenance: ed.Provenance,
			SrcPath: ed.SrcPath, SrcFingerprint: fps[ed.SrcPath],
		})
	}

	// Atribución por credencial (como save_code): sin proyecto atribuible no persistimos, para
	// no dejar filas sin atribuir que verían todos los tenants.
	origin, ok := writeOriginFor(principalFrom(ctx), "")
	if !ok {
		return nil
	}
	return s.engine.UpsertPackageGraphFrom(origin, fileKeys, nodes, edges)
}

// packageDirOf devuelve el directorio del paquete de una clave de path normalizada (con "/").
// Sin "/" ⇒ "." (archivo en la raíz).
func packageDirOf(key string) string {
	if i := strings.LastIndex(key, "/"); i >= 0 {
		return key[:i]
	}
	return "."
}
