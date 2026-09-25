package mcp

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// menciónDeTool reconoce un nombre de tool de Musubi dentro de un texto.
var menciónDeTool = regexp.MustCompile(`musubi_[a-z0-9_]+`)

// NINGÚN TEXTO NOMBRA UNA TOOL DORMIDA.
//
// Una tool dormida no está en tools/list, así que el agente no la puede llamar. Un texto que se la
// nombra —un hook, una skill que Musubi instala, la descripción o la respuesta de otra tool— lo
// manda a un callejón. Medido sobre los transcripts el 2026-09-25: los avisos que nombraban tools
// dormidas se siguieron CERO veces, y el modelo ni siquiera intentó cargarlas. Había diez lugares
// así: el auto-descubrimiento de skills empezaba por `musubi_detect_stack`, la skill
// adversarial-review se cerraba con `musubi_debate`, la fase VERIFY registraba fallos con
// `musubi_log_error`.
//
// ES ESTRUCTURAL A PROPÓSITO. Arreglar los diez y fijarlos uno por uno deja abierto el undécimo:
// esto recorre por AST TODO literal de texto del código de producción de cmd/ e internal/ y lo
// cruza con las dormidas que el REGISTRO declara. Lo único que se exceptúa es la propia entrada de
// cada dormida en el registro (su nombre y su descripción), que es donde tiene que estar. Dormir
// una tool que algún texto nombra pone esta prueba en rojo, y el arreglo es elegir: despertarla, o
// cambiar el texto.
//
// Los comentarios no cuentan: no le llegan al agente. Un nombre armado por concatenación
// ("musubi_" + x) tampoco se ve; es el límite declarado.
//
// Sabotaje que la hace fallar: volver a nombrar detect_stack en la skill analyze-project.
// arnes: archivo="cmd/musubi/cognitive.go"
// arnes: de="\"- Confirmá ecosistemas y frameworks en los manifests (go.mod, package.json, pyproject.toml, Cargo.toml…).\\n\" +"
// arnes: a="\"- Usá musubi_detect_stack para confirmar ecosistemas y frameworks.\\n\" +"
//
// Sabotaje que la hace fallar: volver a dormir musubi_debate, que la skill adversarial-review llama.
// arnes: archivo="internal/mcp/registry.go"
// arnes: de="\t\t\t// las tools llegan diferidas, así que despertarla cuesta el nombre, no los ~575 tokens.\n"
// arnes: a="\t\t\t// las tools llegan diferidas, así que despertarla cuesta el nombre, no los ~575 tokens.\n\t\t\tdormant: true,\n"
func TestNingunTextoNombraUnaToolDormida(t *testing.T) {
	dormidasDelRegistro := map[string]bool{}
	for _, e := range NewMcpServer(nil, "", nil).tools {
		if e.dormant {
			dormidasDelRegistro[e.Name] = true
		}
	}

	raiz := filepath.Join("..", "..")
	var hallazgos []string
	barridos := 0
	for _, rel := range archivosGo(t, raiz) {
		if strings.HasSuffix(rel, "_test.go") {
			continue
		}
		if !strings.HasPrefix(rel, "cmd/") && !strings.HasPrefix(rel, "internal/") {
			continue
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, filepath.Join(raiz, filepath.FromSlash(rel)), nil, 0)
		if err != nil {
			t.Fatalf("no pude parsear %s: %v", rel, err)
		}
		barridos++
		propias := entradasDeToolsDormidas(f, dormidasDelRegistro)
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			for _, r := range propias {
				if lit.Pos() >= r.Pos() && lit.End() <= r.End() {
					return true
				}
			}
			texto, err := strconv.Unquote(lit.Value)
			if err != nil {
				texto = lit.Value
			}
			for _, nombre := range menciónDeTool.FindAllString(texto, -1) {
				if dormidasDelRegistro[nombre] {
					hallazgos = append(hallazgos, fmt.Sprintf("%s:%d nombra %s", rel, fset.Position(lit.Pos()).Line, nombre))
				}
			}
			return true
		})
	}
	// CONTROL: sin archivos barridos, «no encontré nada» y «no miré» se escriben igual.
	if barridos < 50 {
		t.Fatalf("barrí sólo %d archivos de cmd/ e internal/: no estoy mirando el código de producción", barridos)
	}
	for _, h := range hallazgos {
		t.Errorf("%s, que está dormida: el agente no la puede llamar. Despertala o cambiá el texto.", h)
	}
	t.Logf("%d archivos barridos contra %d tools dormidas", barridos, len(dormidasDelRegistro))
}

// entradasDeToolsDormidas devuelve los literales `Tool{…}` cuyo Name es una tool dormida: la
// entrada propia de cada una en el registro, que es el único lugar donde su nombre tiene que estar.
func entradasDeToolsDormidas(f *ast.File, dormidas map[string]bool) []ast.Node {
	var out []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if id, ok := cl.Type.(*ast.Ident); !ok || id.Name != "Tool" {
			return true
		}
		for _, el := range cl.Elts {
			kv, ok := el.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			if k, ok := kv.Key.(*ast.Ident); !ok || k.Name != "Name" {
				continue
			}
			if v, ok := kv.Value.(*ast.BasicLit); ok {
				if nombre, err := strconv.Unquote(v.Value); err == nil && dormidas[nombre] {
					out = append(out, cl)
				}
			}
		}
		return true
	})
	return out
}
