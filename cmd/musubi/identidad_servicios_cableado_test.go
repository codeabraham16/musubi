package main

// EL CABLE DEL MODO ROOT, y no su comportamiento: que lo resuelto al arrancar sea lo que usan las
// fuentes. Las pruebas de identidad_servicios_test.go y servicios_usuario_test.go fijan la identidad
// A MANO (identidadParaEnumerar = id, o se la pasan a enumerarUnitsDeUsuario), así que con la
// asignación de runAgent borrada, o con la llamada de servicios_linux.go pasando el valor cero,
// seguían todas en verde — y un agente root volvía a correr `podman ps` como root y a podar los 18
// contenedores del dueño. Es la guarda definida y desconectada: la pieza probada, el cable no.

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"
	"testing"
	"time"
)

// EL AGENTE LE EXPORTA SU RUNTIME A LOS HIJOS AL ARRANCAR, y la identidad propia lo nombra.
//
// Hasta el 2026-09-25 esta prueba custodiaba también el ORDEN (exportar antes de resolver), porque la
// identidad tomaba su Runtime del entorno y resolver primero la dejaba vacía. Ya no: sin la variable,
// resolverIdentidadDeServicios nombra el mismo /run/user/<uid> que se exporta, así que ese sabotaje
// quedaría en verde y se sacó. Lo que se custodia acá es que el hijo reciba la variable.
//
// Sabotaje que la hace fallar: no exportar el runtime propio.
// arnes: archivo="cmd/musubi/identidad_servicios.go"
// arnes: de="\t\t_ = setenv(\"XDG_RUNTIME_DIR\", d)\n"
// arnes: a="\t\t_ = d\n"
func TestElAgenteExportaSuRuntimeALosHijos(t *testing.T) {
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

// EL RUNTIME QUE APARECE DESPUÉS DEL ARRANQUE ENTRA AL INVENTARIO, sin reiniciar el agente.
//
// Un agente con su propio uid —musubi-server antes de pasar a root, la vuelta atrás, cualquier Linux
// que lo corra como su usuario— puede llegar antes que user@<uid>.service: el drop-in de contenedores
// lo admite por escrito. La versión anterior tomaba el runtime del entorno EN ESE INSTANTE, vacío, y
// así quedaba: la fuente --user salía «ausente» en cada latido, el cerebro podaba las usuario:* y no
// volvían hasta un reinicio. core01-ensayo-local FAILED dejaba de alertar, que era el objetivo.
//
// El «latido» de acá es lo que hace enumerarServiciosDelSistema (Linux): heredarRuntimePropio y
// después enumerarUnitsDeUsuario. Que ese archivo lo haga así lo custodia la prueba de abajo.
//
// Sabotaje que la hace fallar: no nombrar el runtime propio cuando el entorno no lo trae.
// arnes: archivo="cmd/musubi/identidad_servicios.go"
// arnes: de="\tif propia.Runtime == \"\" && uid > 0 {"
// arnes: a="\tif false && propia.Runtime == \"\" && uid > 0 {"
func TestElRuntimeQueApareceTardeEntraAlInventario(t *testing.T) {
	existe := false // /run/user/1000: todavía no lo creó el manager del usuario
	esDe := func(dir string, uid uint32) bool { return existe && dir == "/run/user/1000" && uid == 1000 }
	// Lo que estadoDelBusDeUsuario leería del disco, con el runtime que la identidad le pase.
	bus := func(rt string) estadoDelBus {
		if rt != "/run/user/1000" || !existe {
			return runtimeAusente
		}
		return busListo
	}
	env := map[string]string{"HOME": "/home/musubi", "USER": "musubi"}
	setenv := func(k, v string) error { env[k] = v; return nil }

	original := ejecutarComoParaEnumerar
	t.Cleanup(func() { ejecutarComoParaEnumerar = original })
	var heredado []string // el XDG_RUNTIME_DIR que el hijo habría heredado, llamada por llamada
	ejecutarComoParaEnumerar = func(identidadDeServicios, string, ...string) ([]byte, error) {
		heredado = append(heredado, env["XDG_RUNTIME_DIR"])
		if env["XDG_RUNTIME_DIR"] == "" {
			return nil, errors.New("Failed to connect to user scope bus")
		}
		return []byte(salidaDeUsuarioMedida), nil
	}

	id, err := prepararIdentidadDeServicios(1000, entornoDe(env), setenv, cuentasDePrueba, esDe)
	if err != nil {
		t.Fatalf("el agente musubi no resolvió su identidad: %v", err)
	}
	if v, puesto := env["XDG_RUNTIME_DIR"]; puesto {
		t.Errorf("al arrancar se exportó XDG_RUNTIME_DIR=%q y el directorio no existía: podman rootless falla", v)
	}
	latido := func() (int, error) {
		heredarRuntimePropio(1000, entornoDe(env), setenv, esDe)
		rs, err := enumerarUnitsDeUsuario(id, bus, time.Now())
		return len(rs), err
	}

	if n, err := latido(); n != 0 || err != nil {
		t.Errorf("primer latido, sin runtime: %d units y err=%v; tenía que ser una fuente que no está", n, err)
	}
	existe = true
	n, err := latido()
	if err != nil || n != 4 {
		t.Errorf("con el manager del usuario ya arriba, la fuente --user devolvió %d units y err=%v; tenían "+
			"que entrar las 4 del dueño. La identidad quedó con el runtime del arranque (%q)", n, err, id.Runtime)
	}
	if len(heredado) != 1 || heredado[0] != "/run/user/1000" {
		t.Errorf("el hijo de systemctl --user heredó XDG_RUNTIME_DIR=%q: sin él no llega al bus", heredado)
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
//
// LOS ARGUMENTOS TAMBIÉN SON EL CABLE. Un setenv que no escribe, o un esDe que siempre dice que no,
// dejan al agente sin XDG_RUNTIME_DIR y a la fuente --user ausente en silencio, con todas las pruebas
// de comportamiento en verde: ésas le pasan sus propios dobles, nunca los de runAgent.
//
// Sabotaje que la hace fallar: resolver con un uid fijo en vez del del proceso.
// arnes: archivo="cmd/musubi/agent.go"
// arnes: de="os.Getuid(),"
// arnes: a="1000,"
// Sabotaje que la hace fallar: resolver con un getenv que no lee el entorno (se pierde MUSUBI_AGENTE_USUARIO).
// arnes: archivo="cmd/musubi/agent.go"
// arnes: de="os.Getenv,"
// arnes: a="func(string) string { return \"\" },"
// Sabotaje que la hace fallar: exportar el runtime con un setenv que no escribe.
// arnes: archivo="cmd/musubi/agent.go"
// arnes: de="os.Setenv,"
// arnes: a="func(string, string) error { return nil },"
// Sabotaje que la hace fallar: buscar al usuario en un passwd que devuelve una cuenta vacía.
// arnes: archivo="cmd/musubi/agent.go"
// arnes: de="buscarCuenta,"
// arnes: a="func(string) (cuentaDelSistema, error) { return cuentaDelSistema{}, nil },"
// Sabotaje que la hace fallar: un esDe que siempre dice que el runtime no es del agente.
// arnes: archivo="cmd/musubi/agent.go"
// arnes: de="esDeUid)"
// arnes: a="func(string, uint32) bool { return false })"
//
// Y EL ENUMERADOR DE LINUX LE REINTENTA EL RUNTIME AL HIJO ANTES DE CADA CONSULTA --user
// (TestElRuntimeQueApareceTardeEntraAlInventario dice por qué).
//
// Sabotaje que la hace fallar: reintentarlo DESPUÉS de consultar (diferido al final del enumerador).
// arnes: archivo="cmd/musubi/servicios_linux.go"
// arnes: de="\theredarRuntimePropio("
// arnes: a="\tdefer heredarRuntimePropio("
// Sabotaje que la hace fallar: reintentarlo con un setenv que no escribe.
// arnes: archivo="cmd/musubi/servicios_linux.go"
// arnes: de="os.Setenv,"
// arnes: a="func(string, string) error { return nil },"
// Sabotaje que la hace fallar: reintentarlo con un esDe que siempre dice que no.
// arnes: archivo="cmd/musubi/servicios_linux.go"
// arnes: de="esDeUid)"
// arnes: a="func(string, uint32) bool { return false })"
func TestElAgenteEnumeraConLaIdentidadQueResolvio(t *testing.T) {
	fset := token.NewFileSet()

	run := funcionDelArchivo(t, fset, "agent.go", "runAgent")
	var (
		resuelta       string        // el nombre al que runAgent le asigna prepararIdentidadDeServicios(...)
		posResuelta    token.Pos     // dónde
		llamada        *ast.CallExpr // y con qué argumentos
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
					llamada = x.Rhs[0].(*ast.CallExpr)
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

	quiero := []string{"os.Getuid()", "os.Getenv", "os.Setenv", "buscarCuenta", "esDeUid"}
	if got := argumentosDe(llamada); !reflect.DeepEqual(got, quiero) {
		t.Errorf("%s: runAgent resuelve la identidad con %v y tienen que ser %v. Con otro setenv o "+
			"esDe el agente se queda sin XDG_RUNTIME_DIR, y la fuente --user ausente, sin que nada avise",
			fset.Position(llamada.Pos()), got, quiero)
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
	var consulta token.Pos
	ast.Inspect(enumerador.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok || !esLlamadaA(c, "enumerarUnitsDeUsuario") || len(c.Args) == 0 {
			return true
		}
		if id, ok := c.Args[0].(*ast.Ident); ok && id.Name == "identidadParaEnumerar" {
			conIdentidad++
			consulta = c.Pos()
		}
		return true
	})
	if conIdentidad != 1 {
		t.Errorf("enumerarServiciosDelSistema (Linux) llama %d vez/veces a enumerarUnitsDeUsuario con "+
			"identidadParaEnumerar; tiene que ser UNA. Con otra identidad, un agente root pregunta por el "+
			"manager de root y las usuario:* quedan fuera del inventario", conIdentidad)
	}

	// El reintento del runtime: una sentencia DIRECTA del cuerpo (ni diferida ni bajo un if), antes de
	// la consulta, y con las funciones de verdad del proceso.
	var reintentos []*ast.CallExpr
	for _, st := range enumerador.Body.List {
		if es, ok := st.(*ast.ExprStmt); ok && esLlamadaA(es.X, "heredarRuntimePropio") {
			reintentos = append(reintentos, es.X.(*ast.CallExpr))
		}
	}
	if len(reintentos) != 1 {
		t.Fatalf("enumerarServiciosDelSistema (Linux) llama %d vez/veces a heredarRuntimePropio como sentencia "+
			"propia; tiene que ser UNA. Sin ella, un agente que arrancó antes que user@<uid> nunca le da el "+
			"runtime al hijo de systemctl --user", len(reintentos))
	}
	r := reintentos[0]
	if consulta == token.NoPos || r.Pos() > consulta {
		t.Errorf("%s: el runtime no se le reintenta al hijo ANTES de consultar las units --user", fset.Position(r.Pos()))
	}
	if got, quiero := argumentosDe(r), []string{"os.Getuid()", "os.Getenv", "os.Setenv", "esDeUid"}; !reflect.DeepEqual(got, quiero) {
		t.Errorf("%s: heredarRuntimePropio recibe %v y tienen que ser %v", fset.Position(r.Pos()), got, quiero)
	}
}

// argumentosDe devuelve los argumentos de una llamada como texto de Go («os.Getuid()», «esDeUid»).
func argumentosDe(c *ast.CallExpr) []string {
	if c == nil {
		return nil
	}
	out := make([]string, len(c.Args))
	for i, a := range c.Args {
		out[i] = types.ExprString(a)
	}
	return out
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
