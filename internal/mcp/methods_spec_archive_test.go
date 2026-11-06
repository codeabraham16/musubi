package mcp

// Tests a NIVEL TOOL de musubi_spec_archive (spec-graph F4b): se despachan por
// handleToolsCall —el MISMO camino que un cliente MCP, con la clasificación write y
// el candado exclusivo del dispatch— y se afirma sobre la salida SERIALIZADA Y SOBRE
// EL ARCHIVO EN DISCO. El core del merge (MergeSpecInto) y la proyección
// (projectSDDProcessDelta) ya tienen su batería en internal/specgraph/archive_test.go
// y internal/mcp/spec_archive_test.go; lo que se fija ACÁ son los invariantes cuyo
// sitio de declaración es el HANDLER, no el core:
//   - el handler PROYECTA el delta de proceso ANTES de fusionar (si no, las claves de
//     proceso y el status del delta contaminan la canónica — H1/H7 en el borde);
//   - una promoción sin status explícito nace "active" (un módulo recién archivado
//     tiene una canónica viva);
//   - surface-not-auto en el borde: con conflictos NO escribe el FS y surfacea.
// Cada ancla «Sabotaje» lleva su directiva `// arnes:` colgada debajo, con una
// sustitución de texto que compila, para que el censo del arnés la cuente como
// mecanizada. Los tres sabotajes viven en methods_spec_archive.go (el código nuevo
// de la tool) y atacan el invariante que su test declara.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeSpecFile escribe un archivo de spec en el projectPath del test, creando los
// directorios intermedios. Falla el test ante cualquier error de FS: un fixture que
// no se puede plantar no es un caso a tolerar.
func writeSpecFile(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatalf("no se pudo crear el dir de %s: %v", rel, err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatalf("no se pudo escribir el fixture %s: %v", rel, err)
	}
}

// readSpecFile lee un archivo del projectPath del test.
func readSpecFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", rel, err)
	}
	return string(b)
}

const procDeltaMerge = `---
artifact: spec
schema_version: "1.0"
change: add-widget
status: draft
---

# Widget

## Interface
NuevaFuncion() hace lo nuevo.
`

const canonActivo = `---
id: widget
type: module-design
title: Widget
status: active
---

# Widget

## Responsibility
El widget hace lo de siempre.
`

// TestSpecArchiveMergeMantieneLaCanonicaLimpia fija el invariante de borde H1+H7: el
// handler PROYECTA el delta de proceso antes de fusionar, así que las claves de
// proceso (artifact/schema_version/change) NO contaminan la canónica y el status de
// proceso (draft) NO degrada el status canónico (active). El cuerpo es aditivo: la
// sección nueva del delta se agrega y la prosa canónica se conserva. En el camino
// limpio ESCRIBE la canónica y la respuesta OMITE `merged` (acota el payload).
//
// Sabotaje que lo pone en ROJO: en toolSpecArchive, fusionar el delta CRUDO en vez
// del proyectado (MergeSpecInto(canonical, delta) en lugar de (..., projected)). En
// MERGE, el delta crudo mergea sin error pero mete schema_version/artifact/change en
// el frontmatter (H1) y pisa status:active con draft (H7): ambas aserciones caen.
// arnes: archivo="internal/mcp/methods_spec_archive.go"
// arnes: de="specgraph.MergeSpecInto(canonical, projected)"
// arnes: a="specgraph.MergeSpecInto(canonical, delta)"
func TestSpecArchiveMergeMantieneLaCanonicaLimpia(t *testing.T) {
	dir := t.TempDir()
	s := newTestServerWithPath(t, dir)
	writeSpecFile(t, dir, "internal/widget/SPEC.md", canonActivo)
	writeSpecFile(t, dir, "specs/add-widget/spec.md", procDeltaMerge)

	res, e := call(t, s, "musubi_spec_archive", map[string]interface{}{
		"change": "add-widget",
		"path":   "internal/widget/SPEC.md",
	})
	if e != nil {
		t.Fatalf("archive en merge no debía fallar: %+v", e)
	}
	text := specCallText(t, res)
	if !strings.Contains(text, `"written":true`) || !strings.Contains(text, `"mode":"merge"`) {
		t.Errorf("merge limpio debía escribir en modo merge; salida: %s", text)
	}
	if strings.Contains(text, `"merged"`) {
		t.Errorf("en el camino limpio la respuesta NO debe traer `merged` (presupuesto de cuerpo); salida: %s", text)
	}

	written := readSpecFile(t, dir, "internal/widget/SPEC.md")
	// H7: el status canónico no se degrada.
	if !strings.Contains(written, "status: active") {
		t.Errorf("H7: el status canónico debía quedar active; canónica:\n%s", written)
	}
	if strings.Contains(written, "status: draft") {
		t.Errorf("H7: el status:draft del delta de proceso degradó la canónica; canónica:\n%s", written)
	}
	// H1: ninguna clave de proceso contamina la canónica.
	if strings.Contains(written, "schema_version") || strings.Contains(written, "artifact:") {
		t.Errorf("H1: una clave de proceso contaminó la canónica; canónica:\n%s", written)
	}
	// Aditivo: la sección nueva se agrega, la prosa canónica se conserva.
	if !strings.Contains(written, "## Interface") || !strings.Contains(written, "NuevaFuncion()") {
		t.Errorf("la sección nueva del delta debía agregarse; canónica:\n%s", written)
	}
	if !strings.Contains(written, "El widget hace lo de siempre.") {
		t.Errorf("la prosa canónica debía conservarse; canónica:\n%s", written)
	}
}

const procDeltaPromo = `---
artifact: spec
schema_version: "1.0"
change: new-mod
status: draft
---

# Módulo Nuevo

## Responsibility
Hace algo flamante.
`

// TestSpecArchivePromoteNaceActive fija el invariante del borde en PROMOCIÓN: cuando
// la canónica no existe y el caller NO pasa status, la canónica nace "active" (un
// módulo recién archivado tiene una canónica viva). De paso cubre el camino feliz de
// promoción: id/type inyectados, cero claves de proceso, cuerpo íntegro, written=true.
//
// Sabotaje que lo pone en ROJO: cambiar el default de promoción de "active" a "draft"
// en toolSpecArchive. La canónica nacería draft y la aserción de status:active cae.
// arnes: archivo="internal/mcp/methods_spec_archive.go"
// arnes: de="promoteStatus = \"active\""
// arnes: a="promoteStatus = \"draft\""
func TestSpecArchivePromoteNaceActive(t *testing.T) {
	dir := t.TempDir()
	s := newTestServerWithPath(t, dir)
	writeSpecFile(t, dir, "specs/new-mod/spec.md", procDeltaPromo)

	res, e := call(t, s, "musubi_spec_archive", map[string]interface{}{
		"change": "new-mod",
		"path":   "internal/newmod/SPEC.md",
		"id":     "newmod",
		"type":   "module-design",
		// SIN status: debe defaultear a active.
	})
	if e != nil {
		t.Fatalf("promoción no debía fallar: %+v", e)
	}
	text := specCallText(t, res)
	if !strings.Contains(text, `"written":true`) || !strings.Contains(text, `"mode":"promote"`) {
		t.Errorf("promoción debía escribir en modo promote; salida: %s", text)
	}

	written := readSpecFile(t, dir, "internal/newmod/SPEC.md")
	if !strings.Contains(written, "status: active") {
		t.Errorf("una canónica promovida sin status explícito debía nacer active; canónica:\n%s", written)
	}
	if !strings.Contains(written, "id: newmod") || !strings.Contains(written, "type: module-design") {
		t.Errorf("la promoción debía inyectar id+type del módulo; canónica:\n%s", written)
	}
	if strings.Contains(written, "schema_version") || strings.Contains(written, "artifact:") {
		t.Errorf("H1: la canónica promovida no debe traer claves de proceso; canónica:\n%s", written)
	}
	if !strings.Contains(written, "Hace algo flamante.") {
		t.Errorf("el cuerpo del delta debía quedar íntegro; canónica:\n%s", written)
	}
}

const canonConflicto = `---
id: widget
type: module-design
title: Widget
status: active
---

# Widget

## Responsibility
AAA: la responsabilidad original.
`

const procDeltaConflicto = `---
artifact: spec
schema_version: "1.0"
change: edit-widget
status: draft
---

# Widget

## Responsibility
BBB: una redacción distinta que NO debe pisar la canónica.

## Notes
Sección nueva que NO debe escribirse mientras haya un conflicto sin resolver.
`

// TestSpecArchiveConflictoSurfaceaSinEscribir fija el invariante surface-not-auto en
// el borde (R15/R21): cuando una sección del MISMO heading tiene cuerpo distinto, el
// handler NO escribe el FS —devuelve written:false, surfacea el conflicto y el
// `merged` para inspección— y la canónica en disco queda intacta (ni la prosa en
// conflicto ni la sección nueva del delta llegan al archivo).
//
// Sabotaje que lo pone en ROJO: aflojar la compuerta de escritura para que escriba
// aunque haya conflictos (len(conflicts) == 0 -> >= 0). Entonces el merged se escribe,
// la sección nueva "## Notes" aparece en disco y written queda true: ambas aserciones
// caen. (La prosa en conflicto sigue siendo la canónica aun si escribe —el merge no la
// pisa—, por eso el testigo observable es la sección NUEVA, que sí se agregaría.)
// arnes: archivo="internal/mcp/methods_spec_archive.go"
// arnes: de="if len(conflicts) == 0 {"
// arnes: a="if len(conflicts) >= 0 {"
func TestSpecArchiveConflictoSurfaceaSinEscribir(t *testing.T) {
	dir := t.TempDir()
	s := newTestServerWithPath(t, dir)
	writeSpecFile(t, dir, "internal/widget/SPEC.md", canonConflicto)
	writeSpecFile(t, dir, "specs/edit-widget/spec.md", procDeltaConflicto)

	res, e := call(t, s, "musubi_spec_archive", map[string]interface{}{
		"change": "edit-widget",
		"path":   "internal/widget/SPEC.md",
	})
	if e != nil {
		t.Fatalf("un conflicto no es un error de la tool (se surfacea): %+v", e)
	}
	text := specCallText(t, res)
	if !strings.Contains(text, `"written":false`) {
		t.Errorf("con conflicto la tool NO debe escribir (written:false); salida: %s", text)
	}
	if !strings.Contains(text, `"heading"`) || !strings.Contains(text, "Responsibility") {
		t.Errorf("el conflicto de la sección debía surfacearse en conflicts; salida: %s", text)
	}
	if !strings.Contains(text, `"merged"`) {
		t.Errorf("sin escribir, la tool debía devolver `merged` para inspección; salida: %s", text)
	}

	onDisk := readSpecFile(t, dir, "internal/widget/SPEC.md")
	if !strings.Contains(onDisk, "AAA: la responsabilidad original.") {
		t.Errorf("la prosa canónica debía quedar intacta; canónica:\n%s", onDisk)
	}
	if strings.Contains(onDisk, "BBB: una redacción distinta") {
		t.Errorf("surface-not-auto: la prosa en conflicto del delta NO debe pisar la canónica; canónica:\n%s", onDisk)
	}
	// Testigo del no-write: la sección NUEVA del delta no debe haberse agregado.
	if strings.Contains(onDisk, "## Notes") || strings.Contains(onDisk, "Sección nueva que NO debe escribirse") {
		t.Errorf("con conflicto sin resolver NO debe escribirse nada en el FS; canónica:\n%s", onDisk)
	}
}
