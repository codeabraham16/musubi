package codeintel

import "go/ast"

// tipos_declarados.go cierra el hueco que dejó F8-A/F8-C: una llamada a un MÉTODO sobre un valor de
// un tipo de OTRO paquete no generaba arista, así que `musubi_impact` sobre cualquier método
// exportado subestimaba el blast radius —perdía a todos los llamadores de afuera de su paquete—, en
// silencio. Medido el 2026-08-17: `DbEngine.AutoEmbedBackfill` decía `callers: []` y lo llama
// `cmd/musubi/main.go#func:autoBackfill`.
//
// ── POR QUÉ SE PUEDE SIN INFERENCIA DE TIPOS ────────────────────────────────
//
// El comentario original de crosspkg.go dice que resolver métodos «exigiría inferencia de tipos
// (fuera de alcance)», y para el caso general es CIERTO: en `x := hacerAlgo()` saber qué es `x`
// obliga a seguir la cadena de retornos. Pero hay un subconjunto grande donde el tipo está
// ESCRITO en el código, a la vista, y leerlo es aritmética de AST:
//
//	func autoBackfill(engine *memory.DbEngine, ...)   ← el tipo está en la firma
//	var e memory.DbEngine                             ← el tipo está en la declaración
//
// Este archivo cubre exactamente ese subconjunto. Lo que no está declarado se OMITE, igual que
// siempre: la política del grafo es que una arista inventada es peor que una ausente.
//
// LA ALTERNATIVA QUE SE DESCARTÓ: indexar los métodos por su nombre PELADO y resolver cuando hay
// uno solo en el módulo. Es más barato y cubre más, pero inventa aristas — un `client.Do(...)`
// sobre un tipo de una librería de terceros se resolvería hacia un `Do` propio que casualmente
// fuera único. Falso positivo silencioso, y manda a revisar código que no tiene nada que ver.

// tipoDeVar es el tipo declarado de una variable local, ya resuelto a (import path, nombre del tipo).
type tipoDeVar struct {
	importPath string
	typeName   string
}

// tiposDeclarados mapea nombre de variable → tipo, para las variables de una función cuyo tipo es
// de otro paquete DEL MÓDULO y está declarado explícitamente. Cubre tres lugares:
//
//	parámetros · resultados con nombre · `var` con tipo explícito en el cuerpo
//
// NO cubre `:=` a propósito: ahí el tipo sale del retorno de otra función y eso ya es inferencia.
// El receptor tampoco entra: lo maneja el pase intra-paquete, que es anterior y más preciso.
func tiposDeclarados(d *ast.FuncDecl, inModImports map[string]string) map[string]tipoDeVar {
	out := map[string]tipoDeVar{}
	if d == nil || len(inModImports) == 0 {
		return out
	}

	anotar := func(campos *ast.FieldList) {
		if campos == nil {
			return
		}
		for _, f := range campos.List {
			ip, tn, ok := tipoCalificado(f.Type, inModImports)
			if !ok {
				continue
			}
			for _, n := range f.Names {
				// `_` no se puede usar como receptor de nada, y un nombre vacío tampoco.
				if n == nil || n.Name == "" || n.Name == "_" {
					continue
				}
				out[n.Name] = tipoDeVar{importPath: ip, typeName: tn}
			}
		}
	}

	if d.Type != nil {
		anotar(d.Type.Params)
		anotar(d.Type.Results) // sólo aporta si están nombrados; si no, Names viene vacío
	}

	// `var x pkg.T` — SÓLO en el nivel superior del cuerpo, no en bloques anidados.
	//
	// ⚠️ LA RESTRICCIÓN ES LA QUE HACE CORRECTO ESTO, no una simplificación. Recorriendo todo el
	// cuerpo, dos bloques hermanos pueden declarar el MISMO nombre con tipos DISTINTOS y el último
	// leído gana: las llamadas del primer bloque quedarían atribuidas al tipo del segundo. Eso es
	// una arista EQUIVOCADA, no una ausente, y la política del grafo prohíbe justamente eso.
	//
	// En el nivel superior el problema no existe: los parámetros y las declaraciones del cuerpo
	// comparten bloque, así que Go rechaza `func f(x T) { var x U }` con "x redeclared". No puede
	// haber dos tipos para un mismo nombre.
	if d.Body != nil {
		for _, stmt := range d.Body.List {
			ds, ok := stmt.(*ast.DeclStmt)
			if !ok {
				continue
			}
			gd, ok := ds.Decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || vs.Type == nil {
					continue // sin tipo explícito es `var x = f()`: inferencia, fuera de alcance
				}
				ip, tn, ok := tipoCalificado(vs.Type, inModImports)
				if !ok {
					continue
				}
				for _, name := range vs.Names {
					if name == nil || name.Name == "" || name.Name == "_" {
						continue
					}
					out[name.Name] = tipoDeVar{importPath: ip, typeName: tn}
				}
			}
		}
	}

	return out
}

// tipoCalificado lee una expresión de tipo y devuelve (import path, nombre del tipo) si es
// `pkg.T` o `*pkg.T` con `pkg` importado DEL MÓDULO.
//
// Deliberadamente estrecho: nada de slices, maps, canales ni genéricos. En todos esos casos el
// valor sobre el que se llama al método no es la variable sino un elemento suyo, y adjudicarle el
// tipo del contenedor daría una arista equivocada.
func tipoCalificado(e ast.Expr, inModImports map[string]string) (importPath, typeName string, ok bool) {
	if star, esPuntero := e.(*ast.StarExpr); esPuntero {
		e = star.X
	}
	sel, esSelector := e.(*ast.SelectorExpr)
	if !esSelector {
		return "", "", false
	}
	base, esIdent := sel.X.(*ast.Ident)
	if !esIdent {
		return "", "", false
	}
	ip, esDelModulo := inModImports[base.Name]
	if !esDelModulo {
		return "", "", false // stdlib o terceros: su grafo no es nuestro
	}
	return ip, sel.Sel.Name, true
}
