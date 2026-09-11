package testbudget

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LOS HERMANOS SE ENUMERAN, NO SE LISTAN.
//
// Sacarle la llamada a ExigirTimeoutSuficiente al TestMain de internal/mcp —o al de
// internal/memory— dejaba TODO en verde: la única cosa que decía qué paquetes tenían que
// llamarlo era una frase en el mensaje de un commit. Acá el conjunto sale del repo: los
// paquetes que pasan UMBRAL_GUARDA_LINEAS_TEST tienen que tener el guard, y el que crezca
// mañana queda nombrado solo.
func TestLosPaquetesCarosLlamanAlGuard(t *testing.T) {
	raiz, err := RaizDelRepo(".")
	if err != nil {
		t.Fatal(err)
	}
	pol, err := CargarPoliticaDelRepo(".")
	if err != nil {
		t.Fatal(err)
	}
	todos, err := PaquetesConTests(raiz)
	if err != nil {
		t.Fatal(err)
	}
	var caros []PaqueteDeTest
	for _, p := range todos {
		if p.LineasDeTest >= pol.UmbralLineasTest {
			caros = append(caros, p)
		}
	}
	// CERO CAROS ES «NO PUDE MIRAR». Con el conjunto vacío este test —y las tres guardas del
	// ci.yml que dependen de él— pasarían sin haber exigido nada.
	if len(caros) == 0 {
		t.Fatalf("ningún paquete pasa UMBRAL_GUARDA_LINEAS_TEST=%d, con %d paquetes con tests en "+
			"el repo: el umbral quedó arriba de todo y la enumeración no exige nada",
			pol.UmbralLineasTest, len(todos))
	}
	for _, p := range caros {
		t.Logf("caro: %-24s %6d líneas de test", p.Dir, p.LineasDeTest)
		if !p.GuardEnTestMain {
			t.Errorf("%s tiene %d líneas de código de test (umbral %d) y su TestMain NO llama a "+
				"testbudget.ExigirTimeoutSuficiente. Sin eso, `go test -race ./...` a secas vuelve "+
				"a morir con «panic: test timed out after 10m0s» y el stack de un test inocente.\n"+
				"    Agregá en %s/main_test.go:\n"+
				"        func TestMain(m *testing.M) {\n"+
				"            flag.Parse()\n"+
				"            if motivo := testbudget.ExigirTimeoutSuficiente(\".\"); motivo != \"\" {\n"+
				"                fmt.Fprintln(os.Stderr, \"presupuesto insuficiente —\", motivo)\n"+
				"                os.Exit(1)\n"+
				"            }\n"+
				"            os.Exit(m.Run())\n"+
				"        }",
				p.Dir, p.LineasDeTest, pol.UmbralLineasTest, p.Dir)
		}
		if !p.VerificaEnVivo {
			t.Errorf("%s no tiene ningún test que llame a testbudget.ErrorSiElGuardNoCorrio: el "+
				"guard de su TestMain podría estar mudo (sin flag.Parse() lee 0, o sea «sin "+
				"límite») y nada lo notaría", p.Dir)
		}
	}
}

// El proxy tiene que estar MIDIENDO algo: si el recorrido no encuentra archivos, es un error y
// no una lista vacía.
func TestEnumerarSinArchivosEsError(t *testing.T) {
	if _, err := PaquetesConTests(t.TempDir()); err == nil {
		t.Fatal("enumerar un árbol sin tests devolvió una lista vacía y ningún error")
	}
}

// EL ANCLA ES LA LLAMADA, NO EL TEXTO.
//
// Este repo perdió siete guardas por preguntar por un texto: la palabra estaba en un comentario,
// en un mensaje de error, en un prefijo o en el vecino. Acá se prueba explícitamente que ninguna
// de esas formas satisface la enumeración.
func TestLaEnumeracionMiraLaLlamadaYNoElTexto(t *testing.T) {
	casos := []struct {
		nombre     string
		fuente     string
		enTestMain bool
		enVivo     bool
	}{
		{
			nombre: "la llamada de verdad",
			fuente: `package p
import ("testing"; "os"; "musubi/internal/testbudget")
func TestMain(m *testing.M) {
	if motivo := testbudget.ExigirTimeoutSuficiente("."); motivo != "" { os.Exit(1) }
	os.Exit(m.Run())
}`,
			enTestMain: true,
		},
		{
			nombre: "sólo en un comentario",
			fuente: `package p
import ("testing"; "os"; "musubi/internal/testbudget")
// TestMain llama a testbudget.ExigirTimeoutSuficiente(".") como manda la política.
func TestMain(m *testing.M) { _ = testbudget.NombreArchivoPolitica; os.Exit(m.Run()) }`,
		},
		{
			nombre: "sólo en un string",
			fuente: `package p
import ("testing"; "os"; "musubi/internal/testbudget")
func TestMain(m *testing.M) {
	_ = "testbudget.ExigirTimeoutSuficiente"
	_ = testbudget.NombreArchivoPolitica
	os.Exit(m.Run())
}`,
		},
		{
			nombre: "la llamada, pero fuera del TestMain",
			fuente: `package p
import ("testing"; "os"; "musubi/internal/testbudget")
func ayuda() string { return testbudget.ExigirTimeoutSuficiente(".") }
func TestMain(m *testing.M) { os.Exit(m.Run()) }`,
		},
		{
			nombre: "la verificación en vivo",
			fuente: `package p
import ("testing"; "os"; "musubi/internal/testbudget")
func TestGuard(t *testing.T) {
	if err := testbudget.ErrorSiElGuardNoCorrio(os.Args); err != nil { t.Fatal(err) }
}`,
			enVivo: true,
		},
		{
			nombre: "un paquete con el mismo nombre de método pero otro import",
			fuente: `package p
import ("testing"; "os"; "otro/testbudget")
func TestMain(m *testing.M) {
	_ = testbudget.ExigirTimeoutSuficiente(".")
	os.Exit(m.Run())
}`,
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			dir := t.TempDir()
			ruta := filepath.Join(dir, "algo_test.go")
			if err := os.WriteFile(ruta, []byte(c.fuente), 0o644); err != nil {
				t.Fatal(err)
			}
			enTestMain, enVivo, err := analizarGuarda([]string{ruta})
			if err != nil {
				t.Fatal(err)
			}
			if enTestMain != c.enTestMain {
				t.Errorf("guard en TestMain = %v, quiero %v", enTestMain, c.enTestMain)
			}
			if enVivo != c.enVivo {
				t.Errorf("verificación en vivo = %v, quiero %v", enVivo, c.enVivo)
			}
		})
	}
}

// Un archivo de test que no parsea es un error, no un «no encontré la llamada».
func TestFuenteRotaEsError(t *testing.T) {
	dir := t.TempDir()
	ruta := filepath.Join(dir, "roto_test.go")
	if err := os.WriteFile(ruta, []byte("package p\nfunc ("), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := analizarGuarda([]string{ruta}); err == nil {
		t.Fatal("una fuente que no parsea pasó como «sin guard» en vez de romper")
	}
}

// El módulo se lee de go.mod: es de donde salen las rutas con las que se comparan los paquetes.
func TestModuloDelRepo(t *testing.T) {
	raiz, err := RaizDelRepo(".")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ModuloDelRepo(raiz)
	if err != nil {
		t.Fatal(err)
	}
	if m == "" || strings.ContainsAny(m, " \t") {
		t.Fatalf("módulo = %q", m)
	}
	if !strings.HasPrefix(ImportDelGuard, m+"/") {
		t.Fatalf("ImportDelGuard %q no cuelga del módulo %q", ImportDelGuard, m)
	}
}

// UN GUARD CONDICIONADO A LA PLATAFORMA NO ES UN GUARD.
//
// Este repo ya perdió una suite entera por un `*_windows_test.go` que `go test` no compila fuera
// de Windows y que igual decía «ok». Mover el TestMain a un archivo con sufijo de SO o con
// `//go:build` apagaría el guard en Linux —donde corre CI— y la enumeración no lo notaría, así
// que sólo cuenta lo que está en un archivo sin condiciones.
func TestSoloCuentaLoQueCompilaSiempre(t *testing.T) {
	casos := []struct {
		ruta      string
		contenido string
		universal bool
	}{
		{"main_test.go", "package p\n", true},
		{"algo_windows_test.go", "package p\n", false},
		{"algo_linux_test.go", "package p\n", false},
		{"algo_arm64_test.go", "package p\n", false},
		{"main_test.go", "//go:build linux\n\npackage p\n", false},
		{"main_test.go", "//go:build !windows\n\npackage p\n", false},
		{"main_test.go", "// +build linux\n\npackage p\n", false},
		// Un `//go:build` DESPUÉS del package ya no es una restricción de build.
		{"main_test.go", "package p\n\n// go:build no es esto\n", true},
		// Y un nombre que sólo TERMINA parecido no es un sufijo de plataforma.
		{"despliegue_powershell_test.go", "package p\n", true},
	}
	for _, c := range casos {
		if got := archivoUniversal(filepath.Join("x", c.ruta), c.contenido); got != c.universal {
			t.Errorf("archivoUniversal(%q, %q) = %v, quiero %v", c.ruta, c.contenido, got, c.universal)
		}
	}
}
