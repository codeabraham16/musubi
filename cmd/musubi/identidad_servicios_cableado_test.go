package main

// EL CABLE DEL MODO ROOT, y no su comportamiento: que lo resuelto al arrancar sea lo que usan las
// fuentes. Las pruebas de identidad_servicios_test.go y servicios_usuario_test.go fijan la identidad
// A MANO (identidadParaEnumerar = id, o se la pasan a enumerarUnitsDeUsuario), así que con la
// asignación de runAgent borrada, o con la llamada de servicios_linux.go pasando el valor cero,
// seguían todas en verde — y un agente root volvía a correr `podman ps` como root y a podar los 18
// contenedores del dueño. Es la guarda definida y desconectada: la pieza probada, el cable no.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// EL RUNTIME PROPIO SE EXPORTA ANTES DE RESOLVER, porque la identidad propia lo toma del entorno.
//
// Sabotaje que la hace fallar: resolver primero y exportar después.
// arnes: archivo="cmd/musubi/identidad_servicios.go"
// arnes: de="\tif d, ok := runtimeParaHeredar(getenv, uid, func(d string) bool { return esDe(d, uint32(uid)) }); ok {\n\t\t_ = setenv(\"XDG_RUNTIME_DIR\", d)\n\t}\n\treturn resolverIdentidadDeServicios(uid, getenv, buscar)\n"
// arnes: a="\tid, err := resolverIdentidadDeServicios(uid, getenv, buscar)\n\tif d, ok := runtimeParaHeredar(getenv, uid, func(d string) bool { return esDe(d, uint32(uid)) }); ok {\n\t\t_ = setenv(\"XDG_RUNTIME_DIR\", d)\n\t}\n\treturn id, err\n"
func TestElAgenteExportaSuRuntimeAntesDeResolverLaIdentidad(t *testing.T) {
	esDe := func(dir string, uid uint32) bool { return dir == "/run/user/1000" && uid == 1000 }

	// El despliegue de hoy: unidad de sistema con User=musubi, sin XDG_RUNTIME_DIR en el entorno.
	env := map[string]string{"HOME": "/home/musubi", "USER": "musubi"}
	setenv := func(k, v string) error { env[k] = v; return nil }
	id, err := prepararIdentidadDeServicios(1000, entornoDe(env), setenv, cuentasDePrueba, esDe)
	if err != nil {
		t.Fatalf("el agente musubi no resolvió su identidad: %v", err)
	}
	if env["XDG_RUNTIME_DIR"] != "/run/user/1000" {
		t.Errorf("no se exportó XDG_RUNTIME_DIR (%q): los hijos no llegan al bus del dueño", env["XDG_RUNTIME_DIR"])
	}
	if id.Runtime != "/run/user/1000" || id.Bajar {
		t.Errorf("identidad %+v: la propia tiene que llevar el runtime recién exportado. Con Runtime vacío "+
			"la fuente --user se lee como ausente y ninguna usuario:* entra al inventario", id)
	}

	// Como root no se exporta nada: el mundo del dueño lo resuelve la identidad, no el entorno.
	envRoot := map[string]string{"HOME": "/root", envUsuarioDeServicios: "musubi"}
	id, err = prepararIdentidadDeServicios(0, entornoDe(envRoot), func(k, v string) error { envRoot[k] = v; return nil },
		cuentasDePrueba, esDe)
	if err != nil {
		t.Fatalf("root con %s=musubi no resolvió: %v", envUsuarioDeServicios, err)
	}
	if _, puesto := envRoot["XDG_RUNTIME_DIR"]; puesto {
		t.Errorf("un agente root exportó XDG_RUNTIME_DIR=%q: el exec y la shell de las personas lo heredarían", envRoot["XDG_RUNTIME_DIR"])
	}
	if !id.Bajar || id.Runtime != "/run/user/1000" || id.Home != "/home/musubi" {
		t.Errorf("root con %s=musubi resolvió %+v; tenía que bajar al dueño", envUsuarioDeServicios, id)
	}
}

// LO QUE runAgent RESUELVE ES LO QUE ENUMERAN LAS FUENTES, y se asigna antes del primer latido.
//
// Se lee el código con go/ast y no con un proceso: runAgent llama a os.Exit y abre la red, y lo que
// se custodia es el cable (quién asigna, a partir de qué, antes de qué), no lo que hace cada pieza.
//
// Sabotaje que la hace fallar: que runAgent resuelva la identidad y no la guarde.
// arnes: archivo="cmd/musubi/agent.go"
// arnes: de="\tidentidadParaEnumerar = id\n"
// arnes: a="\t_ = id\n"
// Sabotaje que la hace fallar: que el enumerador de Linux le pase a la fuente --user el valor cero.
// arnes: archivo="cmd/musubi/servicios_linux.go"
// arnes: de="enumerarUnitsDeUsuario(identidadParaEnumerar, estadoDelBusDeUsuario, ahora)"
// arnes: a="enumerarUnitsDeUsuario(identidadDeServicios{}, estadoDelBusDeUsuario, ahora)"
func TestElAgenteEnumeraConLaIdentidadQueResolvio(t *testing.T) {
	fset := token.NewFileSet()

	run := funcionDelArchivo(t, fset, "agent.go", "runAgent")
	var (
		resuelta       string    // el nombre al que runAgent le asigna prepararIdentidadDeServicios(...)
		posResuelta    token.Pos // dónde
		asignaciones   []*ast.AssignStmt
		primerLatido   token.Pos
		llamadasLatido int
	)
	ast.Inspect(run.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Rhs) == 1 && esLlamadaA(x.Rhs[0], "prepararIdentidadDeServicios") && len(x.Lhs) > 0 {
				if id, ok := x.Lhs[0].(*ast.Ident); ok {
					resuelta, posResuelta = id.Name, x.Pos()
				}
			}
			for _, l := range x.Lhs {
				if id, ok := l.(*ast.Ident); ok && id.Name == "identidadParaEnumerar" {
					asignaciones = append(asignaciones, x)
				}
			}
		case *ast.CallExpr:
			if esLlamadaA(x, "latir") {
				llamadasLatido++
				if primerLatido == token.NoPos || x.Pos() < primerLatido {
					primerLatido = x.Pos()
				}
			}
		}
		return true
	})

	// CONTROL DE QUE MIRÓ ALGO: sin estas dos, la guarda pasaría en verde sobre un runAgent que
	// cambió de forma sin haber mirado nada.
	if resuelta == "" {
		t.Fatal("runAgent no asigna el resultado de prepararIdentidadDeServicios: o dejó de resolver la " +
			"identidad al arrancar, o la guarda perdió de vista cómo lo hace")
	}
	if llamadasLatido == 0 {
		t.Fatal("runAgent no llama a latir: la guarda no puede decir si la identidad va antes del primer latido")
	}

	if len(asignaciones) != 1 {
		t.Fatalf("runAgent asigna identidadParaEnumerar %d vez/veces; tiene que ser UNA. Sin ninguna, un agente "+
			"root enumera podman como root y el cerebro da de baja los contenedores del dueño", len(asignaciones))
	}
	a := asignaciones[0]
	if v, ok := a.Rhs[0].(*ast.Ident); !ok || v.Name != resuelta {
		t.Errorf("%s: identidadParaEnumerar no recibe %q, que es lo que resolvió prepararIdentidadDeServicios",
			fset.Position(a.Pos()), resuelta)
	}
	if a.Pos() < posResuelta {
		t.Errorf("%s: identidadParaEnumerar se asigna antes de resolver la identidad", fset.Position(a.Pos()))
	}
	if a.Pos() > primerLatido {
		t.Errorf("%s: identidadParaEnumerar se asigna DESPUÉS del primer latido (%s): ese inventario sale con "+
			"la identidad en cero", fset.Position(a.Pos()), fset.Position(primerLatido))
	}

	// La fuente --user de Linux recibe ESA identidad. servicios_linux.go se lee como texto de Go aunque
	// esta prueba corra en Windows: es justamente donde el cable no se compila y nadie lo miraría.
	enumerador := funcionDelArchivo(t, fset, "servicios_linux.go", "enumerarServiciosDelSistema")
	conIdentidad := 0
	ast.Inspect(enumerador.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok || !esLlamadaA(c, "enumerarUnitsDeUsuario") || len(c.Args) == 0 {
			return true
		}
		if id, ok := c.Args[0].(*ast.Ident); ok && id.Name == "identidadParaEnumerar" {
			conIdentidad++
		}
		return true
	})
	if conIdentidad != 1 {
		t.Errorf("enumerarServiciosDelSistema (Linux) llama %d vez/veces a enumerarUnitsDeUsuario con "+
			"identidadParaEnumerar; tiene que ser UNA. Con otra identidad, un agente root pregunta por el "+
			"manager de root y las usuario:* quedan fuera del inventario", conIdentidad)
	}
}

// funcionDelArchivo parsea un archivo del paquete y devuelve la función nombrada, o corta la prueba.
func funcionDelArchivo(t *testing.T, fset *token.FileSet, archivo, nombre string) *ast.FuncDecl {
	t.Helper()
	f, err := parser.ParseFile(fset, archivo, nil, 0)
	if err != nil {
		t.Fatalf("%s no parsea: %v", archivo, err)
	}
	for _, fd := range funcionesDe(f) {
		if fd.Recv == nil && fd.Name.Name == nombre {
			return fd
		}
	}
	t.Fatalf("%s ya no define %s: la guarda perdió de vista el cable que custodia", archivo, nombre)
	return nil
}

// esLlamadaA dice si la expresión es una llamada a la función del paquete con ese nombre.
func esLlamadaA(e ast.Expr, nombre string) bool {
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := c.Fun.(*ast.Ident)
	return ok && id.Name == nombre
}
