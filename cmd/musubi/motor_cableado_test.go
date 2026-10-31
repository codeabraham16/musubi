package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// ─────────────────────────────────────────────────────────────────────────────────────────────
// EL MOTOR DE TURNOS SE CONSTRUYE Y SE ENCIENDE EN `serve`, Y EN NINGÚN OTRO LADO.
//
// `internal/motor` y `internal/mcp` tienen sus propias baterías: el traductor, los dos motores,
// la selección por configuración y el endpoint están medidos ahí, con 60 sabotajes. Lo que NINGUNO
// de esos mide es si alguien lo llama. Antes de este cableado, `motor.Desde` y `motor.DescribirMotor`
// tenían CERO callers de producción: el paquete entero existía, compilaba, pasaba sus tests, y el
// `POST /api/stream` contestaba 405 para siempre porque `s.motor` nunca se poblaba.
//
// Ese modo de fallo tiene nombre en este repo —`patron-se-construye-y-no-se-enciende`, 25 de 58
// tools sin un solo llamador— y es el peor de todos para un test: el paquete está verde, la
// cobertura es alta, y la feature no existe. Nada se pone rojo, porque no hay nada roto; lo que
// falta es una línea que nadie escribió.
//
// SE LEE EL AST Y NO EL TEXTO, y la diferencia se midió el mismo día que se escribió esto. La
// guarda de `WithVersion` ancla su sabotaje en un literal con la option de AL LADO —porque
// `mcp.WithVersion(version))` solo aparece en dos construcciones y no identifica una—, y al
// insertar `WithMotor` entre esas dos el sabotaje dejó de aplicarse: su verde pasó a significar
// «no lo apliqué». Preguntarle al árbol sintáctico por la función y por los argumentos de la
// llamada no se pudre cuando el vecino cambia.
//
// ⚠️ Y TIENE UN PRECIO QUE HAY QUE SABER ANTES DE LEER UN VERDE: `arnes -overlay` NO LES PUEDE DAR
// VEREDICTO A ESTOS CINCO. Ese modo reemplaza archivos para el BUILD de Go, y estas guardas no
// corren `main.go`: lo PARSEAN del disco, así que ven el archivo sano y pasan con el sabotaje
// puesto. Medido el 2026-10-29: los cinco salieron «VERDE BAJO OVERLAY ← candidato a rojo falso»,
// y en la misma corrida la posición 182 —`identidad_servicios_unix_test.go:44`, del repo y ajena a
// esto— salió igual, o sea que es la CLASE y no estas guardas. Es el segundo caso que el propio
// comando separa a mano (deploy/cmd/arnes/main.go:885): «ahí el rojo es legítimo y el overlay es
// el ciego». Su veredicto se saca aplicando al disco, y es ROJO 5/5 —cada sabotaje tumbando su
// prueba, con una prueba TESTIGO del mismo paquete verde en los cinco, que es lo que separa un
// rojo de la guarda de un rojo del paquete.
// ─────────────────────────────────────────────────────────────────────────────────────────────

// mainParseado devuelve el AST de `main.go`. La ruta es relativa porque `go test` corre con el
// directorio del paquete como cwd, que es el mismo donde vive el archivo.
func mainParseado(t *testing.T) *ast.File {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("no se pudo parsear cmd/musubi/main.go: %v", err)
	}
	return f
}

// funcionDeMain busca una función de nivel superior por nombre.
//
// Recorre con `funcionesDe` (cliente_del_cerebro_test.go), que ya filtra las declaraciones sin
// cuerpo, en vez de volver a caminar `f.Decls` acá. No se usa `funcionDelArchivo`
// (identidad_servicios_cableado_test.go) porque ése PARSEA el archivo él mismo, y estas cuatro
// guardas necesitan además el `*ast.File` entero —para contar las construcciones de McpServer de
// TODO el archivo, no las de una función— así que con él `main.go` se parsearía dos veces.
//
// No encontrarla es FATAL y no un skip: una guarda que no encuentra lo que mira pasa siempre, y
// un verde así se lee igual que «el cableado está bien».
func funcionDeMain(t *testing.T, f *ast.File, nombre string) *ast.FuncDecl {
	t.Helper()
	for _, fd := range funcionesDe(f) {
		if fd.Recv == nil && fd.Name.Name == nombre {
			return fd
		}
	}
	t.Fatalf("no encontré `func %s` en cmd/musubi/main.go: la guarda quedó mirando un archivo "+
		"que cambió de forma, y una guarda que no encuentra nada pasa siempre", nombre)
	return nil
}

// llamadaA devuelve "paquete.Nombre" cuando el nodo es una llamada a un selector de paquete, y ""
// en cualquier otro caso. Es el único lugar donde se decide qué cuenta como «llamar a algo», para
// que las cuatro guardas de abajo hablen del mismo conjunto.
func llamadaA(n ast.Node) string {
	c, ok := n.(*ast.CallExpr)
	if !ok {
		return ""
	}
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return pkg.Name + "." + sel.Sel.Name
}

func cuentaLlamadas(n ast.Node, nombre string) int {
	k := 0
	ast.Inspect(n, func(x ast.Node) bool {
		if llamadaA(x) == nombre {
			k++
		}
		return true
	})
	return k
}

func llama(n ast.Node, nombre string) bool { return cuentaLlamadas(n, nombre) > 0 }

// TestSoloServeCableaElMotorDeTurnos: el motor llega al servidor por `WithMotor`, en `runServe` y
// en ninguna otra construcción.
//
// Las dos mitades son el invariante, no una sola. Que ESTÉ en `serve` es lo que hace que la ruta
// se pueda encender; que NO esté en las otras es la decisión de diseño que `runDaemon` necesita:
// un `provider: api` sin credencial es fatal al construir, así que un daemon que construyera el
// motor le voltearía el arranque a cada sesión del agente por un servicio que esa sesión no tiene
// por dónde servir — un stdio no tiene ruta HTTP.
//
// ⚠️ EL BORRADO DE LA OPTION NO ES SABOTEABLE, Y ESO NO ES UN HUECO: ES EL COMPILADOR CUSTODIANDO
// LA MITAD DEL INVARIANTE. `motorDeTurnos` tiene UN solo consumidor, así que sacar
// `mcp.WithMotor(motorDeTurnos)` deja la variable huérfana y el paquete no compila —medido el
// 2026-10-29: «declared and not used: motorDeTurnos»—. Un sabotaje así pone en rojo a TODAS las
// pruebas del paquete, así que su rojo no dice nada sobre esta guarda: es el patrón VACUO. Lo que
// queda del invariante —que el argumento sea el motor CONSTRUIDO— no lo ve el compilador, porque
// `WithMotor` acepta el nil en silencio, y es lo que ataca W1.
//
// Sabotaje W1: cablear la option con un motor que nunca se construyó (nil tipado; la ruta queda en
// 405 con la línea puesta, y el `err` se sigue chequeando, así que W2 no lo tapa).
// arnes: archivo="cmd/musubi/main.go"
// arnes: de="\tmotorDeTurnos, err := motor.Desde(cfg.Motor)\n"
// arnes: a="\tvar motorDeTurnos motor.Motor\n\t_, err = motor.Desde(cfg.Motor)\n"
// arnes: colision_ok="TestServeNoArrancaSiElMotorEstaMalConfigurado"
//
// Sabotaje W3: cablear el motor TAMBIÉN en el daemon (le voltea el arranque a cada sesión del agente).
// arnes: archivo="cmd/musubi/main.go"
// arnes: de="mcp.WithSpoolLocal(filepath.Join(root, \".musubi\", \"live\")),"
// arnes: a="mcp.WithMotor(nil), mcp.WithSpoolLocal(filepath.Join(root, \".musubi\", \"live\")),"
func TestSoloServeCableaElMotorDeTurnos(t *testing.T) {
	f := mainParseado(t)

	type construccion struct {
		donde    string
		conMotor bool
		arg      string // el identificador que recibe `WithMotor`, para ver que no sea un nil pelado
	}
	var vistas []construccion
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		ast.Inspect(fd.Body, func(x ast.Node) bool {
			if llamadaA(x) != "mcp.NewMcpServer" {
				return true
			}
			c := construccion{donde: fd.Name.Name}
			for _, a := range x.(*ast.CallExpr).Args {
				if llamadaA(a) != "mcp.WithMotor" {
					continue
				}
				c.conMotor = true
				if args := a.(*ast.CallExpr).Args; len(args) == 1 {
					if id, ok := args[0].(*ast.Ident); ok {
						c.arg = id.Name
					}
				}
			}
			vistas = append(vistas, c)
			return true
		})
	}

	// EL CONTROL, antes de cualquier veredicto: este archivo tiene que seguir teniendo varias
	// construcciones y entre ellas las dos que esta guarda contrasta. Sin esto, un `main.go` que
	// cambiara de forma dejaría la lista vacía y las tres comprobaciones de abajo pasarían solas.
	donde := map[string]bool{}
	for _, c := range vistas {
		donde[c.donde] = true
	}
	if len(vistas) < 2 || !donde["runServe"] || !donde["runDaemon"] {
		t.Fatalf("esperaba varias construcciones de McpServer y entre ellas las de runServe y "+
			"runDaemon; encontré %d en %v. La guarda quedó mirando un archivo que cambió de forma",
			len(vistas), donde)
	}

	var conMotor []construccion
	for _, c := range vistas {
		if c.conMotor {
			conMotor = append(conMotor, c)
		}
	}
	if len(conMotor) != 1 {
		t.Fatalf("`mcp.WithMotor` aparece en %d construcciones y tiene que aparecer en UNA (la de "+
			"runServe): %v.\nSin ninguna, `POST /api/stream` contesta 405 para siempre porque "+
			"`s.motor` nunca se puebla — el paquete entero queda verde y la ruta no existe. En el "+
			"daemon, un `provider: api` sin credencial le voltea el arranque a cada sesión del "+
			"agente por un servicio que un stdio no tiene por dónde servir", len(conMotor), vistas)
	}
	if conMotor[0].donde != "runServe" {
		t.Errorf("el motor se cablea en `%s` y tiene que cablearse en `runServe`: es la única que "+
			"sirve el mux HTTP donde vive la ruta", conMotor[0].donde)
	}

	// Y EL ARGUMENTO TIENE QUE SER EL MOTOR CONSTRUIDO, no un nil ni un valor de otra procedencia.
	// `WithMotor` ignora el nil a propósito, así que `WithMotor(nil)` cablea la option, deja la
	// lista con su línea y NO enciende nada: pasaría las dos comprobaciones de arriba.
	serve := funcionDeMain(t, f, "runServe")
	esperado := ""
	for _, st := range serve.Body.List {
		as, ok := st.(*ast.AssignStmt)
		if !ok || !llama(as, "motor.Desde") || len(as.Lhs) == 0 {
			continue
		}
		if id, ok := as.Lhs[0].(*ast.Ident); ok {
			esperado = id.Name
		}
	}
	if esperado == "" {
		t.Fatal("`runServe` no asigna el resultado de `motor.Desde` a ninguna variable: sin eso no " +
			"hay motor que cablear y la option sólo puede estar recibiendo un nil")
	}
	if conMotor[0].arg != esperado {
		t.Errorf("`mcp.WithMotor` recibe %q y tiene que recibir %q, lo que devolvió `motor.Desde`: "+
			"la option acepta nil en silencio, así que cablearla con otra cosa deja la ruta en 405 "+
			"con la línea puesta", conMotor[0].arg, esperado)
	}
}

// TestServeNoArrancaSiElMotorEstaMalConfigurado: el error de `motor.Desde` es FATAL, no un aviso.
//
// Un proveedor mal escrito o una credencial ausente son errores de CONFIGURACIÓN, y el único que
// los puede arreglar es quien editó el yaml. Si `serve` arranca igual, el que ve el síntoma es el
// cliente remoto y lo que ve es un 405 — en la ruta que acaba de configurar y que su cliente tiene
// hardcodeada. Un síntoma que no nombra la causa cuesta una sesión de diagnóstico; no arrancar
// cuesta diez segundos.
//
// Sabotaje W2: bajar el fatal a un aviso (arranca «bien» y contesta 405 en la ruta recién configurada).
// arnes: archivo="cmd/musubi/main.go"
// arnes: de="\tmotorDeTurnos, err := motor.Desde(cfg.Motor)\n\tif err != nil {\n\t\tfmt.Fprintf(os.Stderr, \"musubi serve: %v\\n\", err)\n\t\tos.Exit(1)\n\t}"
// arnes: a="\tmotorDeTurnos, err := motor.Desde(cfg.Motor)\n\tif err != nil {\n\t\tfmt.Fprintf(os.Stderr, \"musubi serve: aviso: %v\\n\", err)\n\t}"
func TestServeNoArrancaSiElMotorEstaMalConfigurado(t *testing.T) {
	f := mainParseado(t)
	serve := funcionDeMain(t, f, "runServe")

	i := -1
	for k, st := range serve.Body.List {
		if llama(st, "motor.Desde") {
			i = k
			break
		}
	}
	if i < 0 {
		t.Fatal("`runServe` no llama a `motor.Desde`: el motor no se construye en ningún lado y " +
			"esta guarda no tiene qué medir")
	}
	if i+1 >= len(serve.Body.List) {
		t.Fatal("`motor.Desde` es la última sentencia de `runServe`: su error no se chequea")
	}

	// SE EXIGE EN LA SENTENCIA SIGUIENTE, y no «en algún lugar de la función», porque un `if err
	// != nil` más abajo mira el error de OTRA cosa: entre medio hay veinte llamadas que reusan la
	// misma variable, y un chequeo que llega tarde no es el chequeo de esto.
	guarda, ok := serve.Body.List[i+1].(*ast.IfStmt)
	if !ok || !esErrNoNil(guarda.Cond) {
		t.Fatalf("la sentencia siguiente a `motor.Desde` es un %T y tiene que ser `if err != nil`: "+
			"el error del constructor no se está mirando", serve.Body.List[i+1])
	}
	if !llama(guarda.Body, "os.Exit") {
		t.Error("el error de `motor.Desde` se reporta pero no corta el arranque: `serve` sigue de " +
			"largo y contesta 405 en `POST /api/stream`, que es el síntoma que no nombra la causa. " +
			"Es el mismo criterio que la validación de las políticas de flota: fail-closed al " +
			"levantar el servicio, donde lo ve quien editó el yaml")
	}
}

// esErrNoNil reconoce la condición `err != nil` y nada más.
func esErrNoNil(e ast.Expr) bool {
	b, ok := e.(*ast.BinaryExpr)
	if !ok || b.Op != token.NEQ {
		return false
	}
	x, okX := b.X.(*ast.Ident)
	y, okY := b.Y.(*ast.Ident)
	return okX && okY && x.Name == "err" && y.Name == "nil"
}

// TestElDaemonNoConstruyeElMotorDeTurnos: `runDaemon` no toca `motor.Desde`.
//
// Es la otra cara de W1 y mide algo que `TestSoloServeCableaElMotorDeTurnos` no puede ver: aquél
// mira los ARGUMENTOS de `NewMcpServer`, así que un `motor.Desde` construido en el daemon y no
// cableado —que es igual de fatal, porque el error voltea el arranque antes de llegar al
// servidor— le pasaría por debajo.
//
// Sabotaje W5: construir el motor en el daemon sin cablearlo (el `provider: api` sin credencial
// voltea cada sesión del agente, y W1 no lo ve porque la option no está).
//
// EL ANCLA NO ES LA LISTA DE OPTIONS DEL DAEMON, y la primera versión de esto se equivocó ahí. El
// `de` decía `"mcp.WithInstruccionesParaElAgente(), skillsDelPluginParaElMapa(), mcp.WithVersion(version))"`
// —las tres options finales— y arrastraba a dos vecinos sólo para ser único: `daemon_agente_test.go`
// sabotea `"mcp.WithInstruccionesParaElAgente(), "` y `agente_plugin_test.go`
// `"skillsDelPluginParaElMapa(), mcp.WithVersion(version))"`, los DOS contenidos en aquél. El censo
// lo denunció como colisión triple el 2026-10-29, y era cierta: al aplicarse rompía el `de` de los
// otros dos. Un `de` que arrastra vecinos para ser único se pudre cuando el vecino cambia.
//
// Ancla en `maintCtx, stopMaint := …`, que es único en `main.go`, exclusivo de `runDaemon` y de
// nadie más, y está después de que `cfg` exista. El statement entra DELANTE, y dónde entre da lo
// mismo: el invariante es que `runDaemon` no llame a `motor.Desde` en NINGÚN lado.
// arnes: archivo="cmd/musubi/main.go"
// arnes: de="\tmaintCtx, stopMaint := context.WithCancel(context.Background())"
// arnes: a="\tif _, err := motor.Desde(cfg.Motor); err != nil {\n\t\tfmt.Fprintf(os.Stderr, \"musubi daemon: %v\\n\", err)\n\t\tos.Exit(1)\n\t}\n\tmaintCtx, stopMaint := context.WithCancel(context.Background())"
func TestElDaemonNoConstruyeElMotorDeTurnos(t *testing.T) {
	f := mainParseado(t)
	daemon := funcionDeMain(t, f, "runDaemon")

	// Control: que esto esté mirando la función que cree. `runDaemon` construye su McpServer, así
	// que si eso no aparece, lo que cambió es la forma del archivo y no el cableado.
	if !llama(daemon.Body, "mcp.NewMcpServer") {
		t.Fatal("`runDaemon` no construye un McpServer: la guarda quedó mirando otra función")
	}

	for _, prohibida := range []string{"motor.Desde", "mcp.WithMotor"} {
		if llama(daemon.Body, prohibida) {
			t.Errorf("`runDaemon` llama a `%s`, y no tiene que hacerlo. El motor no es una "+
				"capacidad del proceso sino de una RUTA: un daemon stdio no tiene por dónde "+
				"servir `POST /api/stream`, y construirlo ahí sólo sirve para que un `provider: "+
				"api` sin credencial —que es fatal al construir— le voltee el arranque a cada "+
				"sesión del agente, rompiendo el servicio que esa sesión sí pidió", prohibida)
		}
	}
}

// TestServeDiceQueMotorQuedoAunqueEsteApagado: la línea de `DescribirMotor` sale SIEMPRE.
//
// Existe porque el único síntoma de un motor mal configurado es un 405 silencioso en una ruta que
// el cliente tiene hardcodeada, y el caso en que más sorprende es justo el apagado — que es el
// estado de toda instalación de hoy. Una línea en el arranque es lo más barato que convierte eso
// en algo averiguable, y ponerla detrás de un `if` la saca exactamente del caso que la necesita.
//
// El CONTENIDO lo miden `TestLaDescripcionDelMotorApagadoExplicaEl405` y
// `TestLaDescripcionDelMotorNuncaImprimeElSecreto`, en `internal/motor`. Acá se mide que se
// llame, y que se llame sin condición.
//
// Sabotaje W4: imprimirla sólo con el motor prendido (desaparece justo en el caso que sorprende).
// arnes: archivo="cmd/musubi/main.go"
// arnes: de="\tfmt.Fprintf(os.Stderr, \"musubi: %s\\n\", motor.DescribirMotor(cfg.Motor))"
// arnes: a="\tif cfg.Motor.Provider != \"\" {\n\t\tfmt.Fprintf(os.Stderr, \"musubi: %s\\n\", motor.DescribirMotor(cfg.Motor))\n\t}"
func TestServeDiceQueMotorQuedoAunqueEsteApagado(t *testing.T) {
	f := mainParseado(t)
	serve := funcionDeMain(t, f, "runServe")

	if n := cuentaLlamadas(serve.Body, "motor.DescribirMotor"); n != 1 {
		t.Fatalf("`runServe` llama a `motor.DescribirMotor` %d veces y tiene que llamarla UNA: "+
			"con cero, el único síntoma de un motor mal configurado sigue siendo un 405 mudo; con "+
			"dos, una de las dos puede estar detrás de una condición", n)
	}

	// SIN CONDICIÓN: la llamada tiene que colgar de una sentencia de PRIMER NIVEL del cuerpo. Que
	// exista no alcanza — metida en un `if cfg.Motor.Provider != ""` la línea desaparece
	// exactamente en el caso en que el 405 sorprende.
	for _, st := range serve.Body.List {
		if !llama(st, "motor.DescribirMotor") {
			continue
		}
		if _, ok := st.(*ast.ExprStmt); !ok {
			t.Errorf("la línea que describe el motor cuelga de un %T y tiene que ser una sentencia "+
				"suelta: detrás de una condición desaparece justo en el caso que la necesita —el "+
				"motor apagado, que es el estado de toda instalación de hoy—, y el síntoma vuelve "+
				"a ser un 405 que no nombra su causa", st)
		}
		return
	}
	t.Error("la llamada a `motor.DescribirMotor` no cuelga de ninguna sentencia de primer nivel de " +
		"`runServe`: está dentro de una condición, un bucle o una clausura")
}
