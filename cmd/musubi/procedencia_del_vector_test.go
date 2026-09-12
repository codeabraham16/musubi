package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestTodoServidorMcpEstampaLaProcedenciaDelVector exige que TODA función de este paquete que
// construya un `mcp.NewMcpServer` estampe también la procedencia del vector.
//
// EL DEFECTO QUE VIO NACER ESTA GUARDA. Había tres constructores de servidor —`runServe`,
// `runDaemon` y `servirSoloLectura`— y sólo dos estampaban. El de SÓLO LECTURA no, así que su
// engine quedaba con `vectorModelID == ""`, la regla de homogeneidad filtraba por `model_id = ”`
// y en una base poblada eso no matchea NADA: el pool vectorial salía **vacío, sin un solo error**.
// El servidor contestaba recall léxico creyendo que hacía semántico, y nada en la salida lo decía.
//
// POR QUÉ ES UNA GUARDA POR AST Y NO UN grep. El defecto no es un texto: es un CAMINO que se olvida.
// La forma de este repo —una regla escrita en N lugares envejece en N-1— no la ataja buscar una
// cadena, porque el que agrega el cuarto servidor tampoco escribe la cadena. Lo que sí converge es
// enumerar los constructores DEL CÓDIGO y exigirle a cada uno el cableado.
//
// NO se aserta CÓMO se estampa (llamar a `engine.SetVectorModelID` directo también cablearía, y la
// guarda lo acepta): se aserta que la función no se olvide. Hoy todos pasan por
// `cablearProcedenciaDelVector`, que es la derivación única.
func TestTodoServidorMcpEstampaLaProcedenciaDelVector(t *testing.T) {
	const (
		constructor = "NewMcpServer"
		cableado    = "cablearProcedenciaDelVector"
		directo     = "SetVectorModelID"
	)

	fset := token.NewFileSet()
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	type sitio struct{ archivo, fn string }
	var servidores, sinCablear []sitio

	for _, e := range entradas {
		nombre := e.Name()
		if e.IsDir() || !strings.HasSuffix(nombre, ".go") || strings.HasSuffix(nombre, "_test.go") {
			continue
		}
		archivo, err := parser.ParseFile(fset, nombre, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", nombre, err)
		}
		for _, decl := range archivo.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			var construye, cablea bool
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				llamada, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch f := llamada.Fun.(type) {
				case *ast.SelectorExpr: // mcp.NewMcpServer(...) o engine.SetVectorModelID(...)
					switch f.Sel.Name {
					case constructor:
						construye = true
					case directo:
						cablea = true
					}
				case *ast.Ident: // cablearProcedenciaDelVector(...)
					if f.Name == cableado {
						cablea = true
					}
				}
				return true
			})
			if !construye {
				continue
			}
			s := sitio{archivo: nombre, fn: fn.Name.Name}
			servidores = append(servidores, s)
			if !cablea {
				sinCablear = append(sinCablear, s)
			}
		}
	}

	// CONTROL POSITIVO. Si la enumeración se rompe (se renombra el constructor, se mueve a otro
	// paquete, cambia la forma de la llamada), esta guarda pasaría a verde vigilando un conjunto
	// VACÍO — verde por no mirar nada, que es el modo de falla clásico de una guarda así.
	if len(servidores) < 3 {
		t.Fatalf("esperaba al menos 3 constructores de %s en este paquete y encontré %d (%v): "+
			"si el constructor se renombró o se movió, esta guarda dejó de ver el código que dice vigilar",
			constructor, len(servidores), servidores)
	}

	for _, s := range sinCablear {
		t.Errorf("%s: %s() construye un %s pero NUNCA estampa la procedencia del vector. "+
			"Su engine va a quedar con vectorModelID vacío, la regla de homogeneidad va a filtrar "+
			"por model_id = '' y el pool vectorial va a salir VACÍO SIN ERROR. Llamá a %s(engine, embedder).",
			s.archivo, s.fn, constructor, cableado)
	}
	// El resumen cuenta lo MEDIDO, no lo esperado. Escrito como `len(servidores)` en los dos
	// huecos, esta línea decía «los 3 cableados» incluso con uno sin cablear y el test en ROJO:
	// un mensaje que contradice al error que acaba de imprimirse. Lo encontró el sabotaje.
	t.Logf("%d constructores de %s, %d cableados", len(servidores), constructor, len(servidores)-len(sinCablear))
}
