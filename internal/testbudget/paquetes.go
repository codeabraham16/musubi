package testbudget

// QUÉ PAQUETES TIENEN QUE LLAMAR AL GUARD, ENUMERADO Y NO LISTADO.
//
// El fix de la ronda 1 puso el guard en internal/mcp y en internal/memory, y escribió «el
// hermano» en el mensaje del commit. Nada lo sostenía: sacarle la llamada a cualquiera de los
// dos —o a los dos— dejaba TODO en verde. Una lista de paquetes escrita a mano en un comentario
// es exactamente el número tipeado en prosa que este cabo vino a matar, una capa más arriba.
//
// Así que el conjunto se DERIVA del repo en cada corrida: se recorren todos los paquetes con
// tests, se mide su tamaño de código de test, y los que pasan el umbral de la política tienen
// que tener el guard. El día que un paquete nuevo crezca, esta guarda lo NOMBRA sola; el día que
// alguien le saque la llamada a uno de los que ya lo tienen, se pone roja.
//
// POR QUÉ LÍNEAS DE TEST Y NO SEGUNDOS. Los segundos son lo que decide de verdad, pero sólo se
// conocen DESPUÉS de correr la suite (y el guard de CI, internal/testbudget/cmd/presupuesto, los
// juzga ahí con la medición real). Un test unitario no puede correr la suite entera para saber a
// quién exigirle el guard. El tamaño del código de test es lo más cercano que se puede derivar
// estáticamente, y en este repo la correlación está medida (todo bajo -race, 2026-09-10, 12
// núcleos con carga):
//
//	internal/mcp      45.103 líneas   1059,0 s   23,5 s por cada 1.000 líneas
//	internal/memory   25.689 líneas    679,0 s   26,4 s
//	cmd/musubi        13.092 líneas    118,2 s    9,0 s
//	internal/fleet     6.174 líneas      5,4 s    0,9 s
//	internal/cognition 1.887 líneas      1,3 s    0,7 s
//
// El orden por líneas es el mismo orden por segundos, y el salto está entre cmd/musubi y fleet.
// El umbral vive en la política (UMBRAL_GUARDA_LINEAS_TEST) con el porqué escrito ahí.
//
// Es un PROXY y se dice que lo es: puede sobrar (un paquete con mucho test barato) y el costo de
// que sobre son seis líneas de TestMain. Lo que no puede es faltar en silencio, que es lo que
// pasaba antes.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ImportDelGuard es el paquete que tienen que importar los paquetes caros.
const ImportDelGuard = "musubi/internal/testbudget"

// PaqueteDeTest es un directorio con tests, con lo medido y lo encontrado.
type PaqueteDeTest struct {
	Dir             string // relativo a la raíz del repo, p. ej. "internal/mcp"
	LineasDeTest    int
	Archivos        []string
	GuardEnTestMain bool // su TestMain llama a testbudget.ExigirTimeoutSuficiente
	VerificaEnVivo  bool // algún test llama a testbudget.ErrorSiElGuardNoCorrio
}

// ModuloDelRepo devuelve la ruta de módulo declarada en go.mod.
func ModuloDelRepo(raiz string) (string, error) {
	b, err := os.ReadFile(filepath.Join(raiz, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("no se pudo leer go.mod: %w", err)
	}
	for _, l := range strings.Split(string(b), "\n") {
		l = strings.TrimSpace(l)
		if resto, ok := strings.CutPrefix(l, "module "); ok {
			return strings.TrimSpace(resto), nil
		}
	}
	return "", fmt.Errorf("go.mod no declara `module`")
}

// PaquetesConTests recorre el repo y devuelve todos los paquetes que tienen archivos _test.go.
func PaquetesConTests(raiz string) ([]PaqueteDeTest, error) {
	porDir := map[string]*PaqueteDeTest{}
	err := filepath.WalkDir(raiz, func(ruta string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		nombre := d.Name()
		if d.IsDir() {
			if ruta != raiz && (strings.HasPrefix(nombre, ".") || nombre == "vendor" ||
				nombre == "testdata" || nombre == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(nombre, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(raiz, filepath.Dir(ruta))
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		p := porDir[rel]
		if p == nil {
			p = &PaqueteDeTest{Dir: rel}
			porDir[rel] = p
		}
		b, err := os.ReadFile(ruta)
		if err != nil {
			return err
		}
		p.LineasDeTest += strings.Count(string(b), "\n") + 1
		p.Archivos = append(p.Archivos, ruta)
		return nil
	})
	if err != nil {
		return nil, err
	}
	// CERO PAQUETES ES «NO PUDE MIRAR», NO «ESTÁ TODO BIEN».
	if len(porDir) == 0 {
		return nil, fmt.Errorf("no se encontró NI UN archivo _test.go bajo %s: la enumeración de "+
			"paquetes caros no miró nada, así que su verde no significaría nada", raiz)
	}
	out := make([]PaqueteDeTest, 0, len(porDir))
	for _, p := range porDir {
		sort.Strings(p.Archivos)
		enTestMain, enVivo, err := analizarGuarda(p.Archivos)
		if err != nil {
			return nil, err
		}
		p.GuardEnTestMain, p.VerificaEnVivo = enTestMain, enVivo
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].LineasDeTest > out[j].LineasDeTest })
	return out, nil
}

// PaquetesCaros son los que pasan el umbral: los que tienen que llamar al guard.
func PaquetesCaros(raiz string, umbral int) ([]PaqueteDeTest, error) {
	todos, err := PaquetesConTests(raiz)
	if err != nil {
		return nil, err
	}
	var caros []PaqueteDeTest
	for _, p := range todos {
		if p.LineasDeTest >= umbral {
			caros = append(caros, p)
		}
	}
	return caros, nil
}

// analizarGuarda mira el AST, no el texto.
//
// Un grep por «ExigirTimeoutSuficiente» lo satisface un comentario, un nombre de test, o una
// línea comentada — que es la forma exacta en que este repo perdió siete guardas. Acá lo que
// cuenta es una LLAMADA: un CallExpr sobre el selector del paquete importado, y para el TestMain
// además tiene que estar adentro de la función TestMain.
func analizarGuarda(archivos []string) (enTestMain, enVivo bool, err error) {
	fset := token.NewFileSet()
	for _, ruta := range archivos {
		b, err := os.ReadFile(ruta)
		if err != nil {
			return false, false, err
		}
		f, err := parser.ParseFile(fset, ruta, b, parser.ParseComments)
		if err != nil {
			return false, false, fmt.Errorf("no se pudo parsear %s: %w", ruta, err)
		}
		// UN GUARD QUE NO COMPILA EN TODAS LAS PLATAFORMAS NO CUENTA. Este repo ya perdió una
		// suite entera por un `_windows_test.go` que `go test` no compilaba fuera de Windows y
		// que igual decía «ok»; mover el TestMain a un archivo con `//go:build` o con sufijo de
		// SO apagaría el guard en Linux —donde corre CI— sin que esta enumeración lo notara.
		// Por eso sólo cuenta lo que está en un archivo sin condiciones.
		if !archivoUniversal(ruta, string(b)) {
			continue
		}
		alias := aliasDelGuard(f)
		if alias == "" {
			continue
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				llamada, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := llamada.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				id, ok := sel.X.(*ast.Ident)
				if !ok || id.Name != alias {
					return true
				}
				switch sel.Sel.Name {
				case "ExigirTimeoutSuficiente":
					if fd.Name.Name == "TestMain" && fd.Recv == nil {
						enTestMain = true
					}
				case "ErrorSiElGuardNoCorrio":
					enVivo = true
				}
				return true
			})
		}
	}
	return enTestMain, enVivo, nil
}

func aliasDelGuard(f *ast.File) string {
	for _, imp := range f.Imports {
		ruta := strings.Trim(imp.Path.Value, `"`)
		if ruta != ImportDelGuard {
			continue
		}
		if imp.Name != nil {
			if imp.Name.Name == "_" || imp.Name.Name == "." {
				return ""
			}
			return imp.Name.Name
		}
		return "testbudget"
	}
	return ""
}

// sistemasYArquitecturas son los sufijos de nombre de archivo que Go interpreta como una
// condición de plataforma (`algo_windows_test.go`, `algo_arm64_test.go`).
var sistemasYArquitecturas = map[string]bool{
	"aix": true, "android": true, "darwin": true, "dragonfly": true, "freebsd": true,
	"hurd": true, "illumos": true, "ios": true, "js": true, "linux": true, "nacl": true,
	"netbsd": true, "openbsd": true, "plan9": true, "solaris": true, "wasip1": true,
	"windows": true, "zos": true,
	"386": true, "amd64": true, "arm": true, "arm64": true, "loong64": true, "mips": true,
	"mips64": true, "mips64le": true, "mipsle": true, "ppc64": true, "ppc64le": true,
	"riscv64": true, "s390x": true, "wasm": true,
}

// archivoUniversal dice si el archivo se compila SIEMPRE: sin `//go:build`, sin `// +build` y
// sin sufijo de plataforma en el nombre. Es a propósito conservador —un guard que sólo corre en
// algunas plataformas no es el guard que se está exigiendo—.
func archivoUniversal(ruta, contenido string) bool {
	base := strings.TrimSuffix(filepath.Base(ruta), ".go")
	base = strings.TrimSuffix(base, "_test")
	partes := strings.Split(base, "_")
	for _, p := range partes[max(len(partes)-2, 0):] {
		if sistemasYArquitecturas[p] {
			return false
		}
	}
	for _, linea := range strings.Split(contenido, "\n") {
		l := strings.TrimSpace(linea)
		if strings.HasPrefix(l, "//go:build") || strings.HasPrefix(l, "// +build") {
			return false
		}
		if strings.HasPrefix(l, "package ") {
			break
		}
	}
	return true
}
