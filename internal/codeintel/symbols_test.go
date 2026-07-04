package codeintel

import "testing"

func symByName(syms []Symbol, name string) (Symbol, bool) {
	for _, s := range syms {
		if s.Name == name {
			return s, true
		}
	}
	return Symbol{}, false
}

func TestExtractGoSymbols(t *testing.T) {
	src := `package x

import "fmt"

const Answer = 42

type Config struct {
	Name string
}

func Load(p string) (Config, error) {
	return Config{}, nil
}

func (c Config) Validate() error {
	fmt.Println(c.Name)
	return nil
}
`
	syms := ExtractSymbols("config.go", src)
	if s, ok := symByName(syms, "Load"); !ok || s.Kind != KindFunc {
		t.Errorf("Load debería ser func, got %+v ok=%v", s, ok)
	}
	if s, ok := symByName(syms, "Validate"); !ok || s.Kind != KindMethod {
		t.Errorf("Validate debería ser method, got %+v ok=%v", s, ok)
	}
	if s, ok := symByName(syms, "Config"); !ok || s.Kind != KindType {
		t.Errorf("Config debería ser type, got %+v ok=%v", s, ok)
	}
	if s, ok := symByName(syms, "Answer"); !ok || s.Kind != KindConst {
		t.Errorf("Answer debería ser const, got %+v ok=%v", s, ok)
	}
	// Rango: Validate debe abarcar su cuerpo (más de una línea).
	if s, _ := symByName(syms, "Validate"); s.EndLine <= s.StartLine {
		t.Errorf("Validate debería ocupar varias líneas, got %+v", s)
	}
}

// El caso que hundía la versión ingenua: el símbolo se corrió de línea. Como derivamos
// del contenido ACTUAL, el rango refleja la posición nueva, no una guardada vieja.
func TestExtractGoSymbolsAfterLineShift(t *testing.T) {
	src := `package x

// un comentario nuevo
// otra línea agregada arriba
// y una más, empujando todo hacia abajo

func Parse() int {
	return 1
}
`
	syms := ExtractSymbols("p.go", src)
	s, ok := symByName(syms, "Parse")
	if !ok {
		t.Fatalf("no se encontró Parse")
	}
	if s.StartLine != 7 { // derivado del estado actual, no de un L viejo
		t.Errorf("Parse StartLine = %d, quería 7 (posición actual)", s.StartLine)
	}
}

func TestExtractGoTolerantOnBrokenFile(t *testing.T) {
	// Archivo a medio editar (no compila): no debe entrar en pánico; degrada a lo que pueda.
	src := `package x

func Good() {}

func Broken( {
`
	syms := ExtractSymbols("broken.go", src)
	if _, ok := symByName(syms, "Good"); !ok {
		t.Errorf("aún con un archivo roto debería recuperar Good, got %+v", syms)
	}
}

func TestExtractBraceSymbols(t *testing.T) {
	src := `import { z } from "z";

export function foo(a) {
  return a + 1;
}

export const bar = (x) => {
  return x * 2;
};

class Widget {
  render() {}
}
`
	syms := ExtractSymbols("app.tsx", src)
	for _, want := range []struct {
		name, kind string
	}{{"foo", KindFunc}, {"bar", KindFunc}, {"Widget", KindClass}} {
		if s, ok := symByName(syms, want.name); !ok || s.Kind != want.kind {
			t.Errorf("%s debería ser %s, got %+v ok=%v", want.name, want.kind, s, ok)
		}
	}
	// El bloque de foo debe cerrar por llaves (más de una línea).
	if s, _ := symByName(syms, "foo"); s.EndLine <= s.StartLine {
		t.Errorf("foo debería ocupar su bloque, got %+v", s)
	}
}

func TestExtractPySymbols(t *testing.T) {
	src := `import os

def top():
    x = 1
    return x

class Service:
    def method(self):
        return 2
`
	syms := ExtractSymbols("svc.py", src)
	if s, ok := symByName(syms, "top"); !ok || s.Kind != KindDef {
		t.Errorf("top debería ser def, got %+v ok=%v", s, ok)
	}
	if s, ok := symByName(syms, "Service"); !ok || s.Kind != KindClass {
		t.Errorf("Service debería ser class, got %+v ok=%v", s, ok)
	}
	// El cuerpo de top termina por des-indentación antes de class.
	if s, _ := symByName(syms, "top"); s.EndLine < 5 {
		t.Errorf("top debería abarcar su cuerpo hasta ~L5, got %+v", s)
	}
}

func TestExtractSymbolsUnsupportedIsEmpty(t *testing.T) {
	if syms := ExtractSymbols("styles.css", "body { color: red; }"); len(syms) != 0 {
		t.Errorf("extensión no soportada debería dar 0 símbolos, got %+v", syms)
	}
	if syms := ExtractSymbols("noext", "whatever"); len(syms) != 0 {
		t.Errorf("sin extensión debería degradar a vacío, got %+v", syms)
	}
}
