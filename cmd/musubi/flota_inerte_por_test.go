package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// A131 · EL PANEL TIENE UN TEXTO PARA CADA COMPUERTA QUE PUEDE DEJAR INERTE A UNA POLÍTICA.
//
// El panel decía por qué una política estaba inerte con un texto FIJO que nombraba dos causas —la
// concesión y la allowlist— mientras la acción ya atravesaba tres (A91 le agregó el eje de
// consentimiento). Una máquina en `pide` o `prohibido` se dibujaba entonces con el motivo
// equivocado, que manda a buscar el arreglo en principals.yaml, donde no está.
//
// Desde A131 el cerebro publica la compuerta que frena en `inerte_por`, y flota.html la traduce con
// el mapa INERTE_POR. Productor y consumidor viven en dos paquetes y nada los ataba: esta guarda
// lee las constantes de `frenoDePolitica` del AST de internal/mcp/politicas.go —la fuente, no una
// lista copiada acá— y exige que el mapa tenga EXACTAMENTE esas claves. Un freno nuevo sin texto
// pone esto rojo; una clave que ya no existe, también (un texto para un estado imposible se lee
// como cobertura).
//
// Exposición medida: 0 hoy. La única política en producción no está inerte, así que el texto no se
// dibuja; se habría dibujado mal con el primer `pide` o `prohibido` sobre musubi-server.
//
// Sabotaje: sacarle al mapa del panel el texto de `consentimiento_pide`.
// arnes: archivo="cmd/musubi/assets/flota.html"
// arnes: de="  consentimiento_pide: 'la máquina exige que su usuario acepte, y un barrido automático no puede esperar la respuesta',\n"
// arnes: a=""
func TestElPanelTieneTextoParaCadaFrenoDePolitica(t *testing.T) {
	// ── EL PRODUCTOR: las constantes del cerebro ─────────────────────────────────────────────
	fuente := filepath.Join("..", "..", "internal", "mcp", "politicas.go")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, fuente, nil, 0)
	if err != nil {
		t.Fatalf("no se pudo parsear %s: %v", fuente, err)
	}
	frenos := map[string]bool{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			if tipo, ok := vs.Type.(*ast.Ident); !ok || tipo.Name != "frenoDePolitica" {
				continue
			}
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("%s: una constante de frenoDePolitica no es un literal de cadena y esta guarda no la puede leer", fset.Position(v.Pos()))
				}
				valor, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("%s: %v", fset.Position(v.Pos()), err)
				}
				// El vacío es «ninguna compuerta frena»: nunca viaja como `inerte_por`.
				if valor != "" {
					frenos[valor] = true
				}
			}
		}
	}
	// PISO: cero frenos no es «no hay nada que dibujar», es que el tipo se renombró o se mudó.
	if len(frenos) < 5 {
		t.Fatalf("encontré %d constantes de frenoDePolitica en %s y son al menos cinco: la guarda dejó de ver la fuente", len(frenos), fuente)
	}

	// ── EL CONSUMIDOR: las claves del mapa, en el asset EMBEBIDO que se sirve ──────────────────
	p := string(assetsFS(t, "assets/flota.html"))
	ini := strings.Index(p, "const INERTE_POR = {")
	if ini < 0 {
		t.Fatal("flota.html no tiene el mapa INERTE_POR: el panel volvió a un texto fijo, que es el defecto de A131")
	}
	cuerpo := p[ini:]
	fin := strings.Index(cuerpo, "\n};")
	if fin < 0 {
		t.Fatal("no encontré el cierre del mapa INERTE_POR en flota.html")
	}
	cuerpo = cuerpo[:fin]
	claves := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^\s*([a-z_]+):`).FindAllStringSubmatch(cuerpo, -1) {
		claves[m[1]] = true
	}
	if len(claves) == 0 {
		t.Fatal("el mapa INERTE_POR no tiene ninguna clave legible: la guarda no midió nada")
	}

	var sinTexto, sobran []string
	for fr := range frenos {
		if !claves[fr] {
			sinTexto = append(sinTexto, fr)
		}
	}
	for c := range claves {
		if !frenos[c] {
			sobran = append(sobran, c)
		}
	}
	sort.Strings(sinTexto)
	sort.Strings(sobran)
	if len(sinTexto) > 0 {
		t.Errorf("el panel no tiene texto para %s: una política frenada por esa compuerta se dibuja con el "+
			"código crudo en vez de decirle a quien mira dónde arreglarla. Agregala a INERTE_POR en flota.html",
			strings.Join(sinTexto, ", "))
	}
	if len(sobran) > 0 {
		t.Errorf("INERTE_POR tiene texto para %s, que no es ninguna constante de frenoDePolitica: explica un "+
			"estado que el cerebro ya no produce", strings.Join(sobran, ", "))
	}
	// Y el mapa se USA donde se dibuja lo inerte, no sólo existe.
	if !strings.Contains(p, "INERTE_POR[p.inerte_por]") {
		t.Error("automatico() no traduce `inerte_por` con INERTE_POR: el mapa existe y el panel no lo lee")
	}
}
