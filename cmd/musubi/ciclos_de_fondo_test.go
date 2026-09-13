package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// ciclosDeFondo_test.go custodia el CABLEADO de los ciclos de fondo del daemon: que el grafo espere
// al mantenimiento de arranque y que nada de lo que se lanza bloquee a runDaemon.

// cicloFalso retiene el mantenimiento de arranque hasta que se cierre `soltar`, y los schedulers
// hasta que se cancele el contexto, como los de verdad.
type cicloFalso struct {
	soltar       chan struct{}
	grafoLanzado chan (<-chan struct{})
}

func nuevoCicloFalso() *cicloFalso {
	return &cicloFalso{soltar: make(chan struct{}), grafoLanzado: make(chan (<-chan struct{}), 1)}
}

func (f *cicloFalso) RunScheduledMaintenance() (bool, memory.MaintenanceReport, error) {
	<-f.soltar
	return false, memory.MaintenanceReport{}, nil
}

func (f *cicloFalso) RunMaintenanceScheduler(ctx context.Context, _ time.Duration) { <-ctx.Done() }

func (f *cicloFalso) RunCodeGraphScheduler(ctx context.Context, _ time.Duration, despuesDe <-chan struct{}) {
	f.grafoLanzado <- despuesDe
	<-ctx.Done()
}

var ambosCiclos = config.MaintenanceConfig{AutoIntervalHours: 6, GraphIndexHours: 1}

// lanzarConPlazo llama a lanzarMantenimientoYGrafo desde OTRA goroutine y le da 5 s para volver.
//
// Toda prueba de este archivo la llama por acá y no directo, por lo que pasa con S5: un scheduler
// sin `go` no vuelve nunca, y si la llamada está en la goroutine de la prueba el t.Fatal no llega
// jamás. La prueba se queda esperando un ctx que sólo cancela su propio Cleanup, y el que falla es
// el PAQUETE entero, por el -timeout de go test. Medido con UNA sola prueba llamando directo: la
// otra falló a los 5 s y el paquete murió igual con «panic: test timed out after 5m0s», 300 s.
// Con el plazo la prueba falla, su Cleanup cancela el ctx y el scheduler falso vuelve solo.
func lanzarConPlazo(t *testing.T, ctx context.Context, srv ciclosDeFondo, m config.MaintenanceConfig) {
	t.Helper()
	volvio := make(chan struct{})
	go func() {
		defer close(volvio)
		lanzarMantenimientoYGrafo(ctx, srv, m, io.Discard)
	}()
	select {
	case <-volvio:
	case <-time.After(5 * time.Second):
		t.Fatal("lanzarMantenimientoYGrafo no volvió en 5 s: algún ciclo corre en la goroutine de runDaemon y el loop stdio no arranca")
	}
}

// S5 — LANZAR LOS CICLOS NO BLOQUEA AL DAEMON.
//
// runDaemon llama esto y DESPUÉS atiende el loop stdio. Un scheduler lanzado sin `go` no vuelve
// nunca, y el daemon arranca mudo: el cliente MCP espera un initialize que nadie contesta.
//
// Sabotaje que la pone roja: lanzar RunCodeGraphScheduler sin `go` (S5). Cada prueba de este archivo
// que la llama falla a los 5 s, y el PAQUETE no cuelga: ninguna la llama sin plazo (ver lanzarConPlazo).
// arnes: archivo="cmd/musubi/ciclos_de_fondo.go"
// arnes: de="go srv.RunCodeGraphScheduler("
// arnes: a="srv.RunCodeGraphScheduler("
func TestLanzarLosCiclosDeFondoNoBloqueaAlDaemon(t *testing.T) {
	f := nuevoCicloFalso()
	close(f.soltar)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel) // desbloquea al que se haya quedado esperando, así la prueba no deja nada colgado

	lanzarConPlazo(t, ctx, f, ambosCiclos)
	select {
	case <-f.grafoLanzado:
	case <-time.After(5 * time.Second):
		t.Fatal("con graph_index_hours > 0 el scheduler del grafo no se lanzó")
	}
}

// S2 — EL GRAFO ESPERA A QUE TERMINE EL MANTENIMIENTO DE ARRANQUE.
//
// Los dos escriben, y el mantenimiento puede traer un VACUUM. El canal que recibe el scheduler del
// grafo tiene que seguir abierto mientras el mantenimiento corre, y cerrarse cuando termina.
//
// Sabotaje que la pone roja: cerrar el canal al ENTRAR a la goroutine del mantenimiento y no al salir (S2).
// arnes: archivo="cmd/musubi/ciclos_de_fondo.go"
// arnes: de="defer close(mantenimientoDeArranque)"
// arnes: a="close(mantenimientoDeArranque)"
func TestElGrafoEsperaAlMantenimientoDeArranque(t *testing.T) {
	f := nuevoCicloFalso()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	t.Cleanup(func() {
		select {
		case <-f.soltar:
		default:
			close(f.soltar)
		}
	})

	lanzarConPlazo(t, ctx, f, ambosCiclos)
	var despuesDe <-chan struct{}
	select {
	case despuesDe = <-f.grafoLanzado:
	case <-time.After(5 * time.Second):
		t.Fatal("el scheduler del grafo no se lanzó")
	}
	select {
	case <-despuesDe:
		t.Fatal("el canal del grafo se cerró con el mantenimiento de arranque todavía corriendo: los dos compiten por la base")
	case <-time.After(300 * time.Millisecond):
	}
	close(f.soltar)
	select {
	case <-despuesDe:
	case <-time.After(5 * time.Second):
		t.Fatal("el mantenimiento de arranque terminó y el canal del grafo no se cerró en 5 s: el grafo no arranca nunca")
	}

	// Con el mantenimiento apagado no hay nada que esperar.
	g := nuevoCicloFalso()
	lanzarConPlazo(t, ctx, g, config.MaintenanceConfig{GraphIndexHours: 1})
	select {
	case d := <-g.grafoLanzado:
		select {
		case <-d:
		case <-time.After(5 * time.Second):
			t.Fatal("con el mantenimiento apagado el canal del grafo tenía que llegar cerrado")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("con el mantenimiento apagado el scheduler del grafo no se lanzó")
	}
}

// S7 — RUNDAEMON LLAMA A LOS CICLOS DE UNA FORMA QUE CORRE, Y ANTES DE ATENDER EL LOOP STDIO.
//
// Las pruebas de arriba custodian QUÉ hace lanzarMantenimientoYGrafo; ésta, que runDaemon la llame
// de verdad. La guarda de internal/mcp (TestM3CadaServidorArrancaElCicloSegunPuedaEscribir) busca
// TEXTO, y el nombre sigue en el cuerpo en las formas que la neutralizan: adentro de un `go func`
// que espera a maintCtx.Done() —que se cancela recién al salir de runDaemon—, en un `defer`, o en
// un `if false`. Con cualquiera de las tres el daemon atiende sin mantenimiento y con el grafo
// rancio, y la suite entera pasaba: medido con la primera, ok cmd/musubi y ok internal/mcp.
//
// POR QUÉ AST Y NO UNA PRUEBA DE HUMO DEL DAEMON. runDaemon arma el workspace desde el cwd, lee
// os.Stdin, sale con os.Exit en sus caminos de error y escucha señales; y lo que habría que
// observar —un mantenimiento que corrió, un grafo indexado— depende de relojes y de una base real.
// Sería una prueba lenta que mide un proxy. Las neutralizaciones, en cambio, son de FORMA, y la
// forma sana es una sola: una sentencia de expresión suelta en el cuerpo de runDaemon, que nada
// condiciona, difiere ni manda a otra goroutine, ANTES de la sentencia que llama a server.Start()
// —que bloquea hasta el EOF de stdin—. Lo que la función hace ya lo cubren las pruebas de arriba.
//
// LO QUE NO VE, para que nadie lo dé por cubierto: la llamada suelta con argumentos que la vacían
// (un maintCtx ya cancelado, una config en cero) y un `if` previo que siempre retorna. El `return`
// pelado antes de la llamada no hace falta mirarlo acá: lo denuncia `go vet` (unreachable code).
//
// Sabotaje que la pone roja: diferir la llamada, dentro de un `go func`, a que se cancele maintCtx (SAB-7).
// arnes: archivo="cmd/musubi/main.go"
// arnes: de="\tlanzarMantenimientoYGrafo(maintCtx, server, cfg.Maintenance, os.Stderr)\n"
// arnes: a="\tgo func() { <-maintCtx.Done(); lanzarMantenimientoYGrafo(maintCtx, server, cfg.Maintenance, os.Stderr) }()\n"
// arnes: colision_ok="TestRunDaemonLanzaLosCiclosAntesDelLoopStdio"
//
// Sabotaje que la pone roja: llamarla con `defer`, que corre recién al salir de runDaemon.
// arnes: archivo="cmd/musubi/main.go"
// arnes: de="\tlanzarMantenimientoYGrafo(maintCtx, server, cfg.Maintenance, os.Stderr)\n"
// arnes: a="\tdefer lanzarMantenimientoYGrafo(maintCtx, server, cfg.Maintenance, os.Stderr)\n"
// arnes: colision_ok="TestRunDaemonLanzaLosCiclosAntesDelLoopStdio"
//
// Sabotaje que la pone roja: dejarla dentro de un bloque que nunca corre.
// arnes: archivo="cmd/musubi/main.go"
// arnes: de="\tlanzarMantenimientoYGrafo(maintCtx, server, cfg.Maintenance, os.Stderr)\n"
// arnes: a="\tif false {\n\t\tlanzarMantenimientoYGrafo(maintCtx, server, cfg.Maintenance, os.Stderr)\n\t}\n"
//
// Sabotaje que la pone roja: atender el loop stdio antes de lanzar los ciclos.
// arnes: archivo="cmd/musubi/main.go"
// arnes: de="\tdefer stopMaint()\n"
// arnes: a="\tdefer stopMaint()\n\tserver.Start()\n"
func TestRunDaemonLanzaLosCiclosAntesDelLoopStdio(t *testing.T) {
	fset := token.NewFileSet()
	archivo, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatalf("parsear main.go: %v", err)
	}
	var cuerpo *ast.BlockStmt
	for _, d := range archivo.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == "runDaemon" {
			cuerpo = fn.Body
		}
	}
	if cuerpo == nil {
		t.Fatal("no está runDaemon en main.go: ¿se renombró o se mudó? Esta guarda la busca por nombre")
	}

	// El Obj descarta una variable local que tape el nombre: el parser resuelve lo declarado en este
	// archivo, y una función de paquete declarada en otro queda sin resolver (nil).
	llamaALanzar := func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return false
		}
		id, ok := c.Fun.(*ast.Ident)
		return ok && id.Name == "lanzarMantenimientoYGrafo" && (id.Obj == nil || id.Obj.Kind == ast.Fun)
	}
	arrancaStdio := func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return false
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		x, ok := sel.X.(*ast.Ident)
		return ok && x.Name == "server" && sel.Sel.Name == "Start"
	}
	contiene := func(st ast.Stmt, pred func(ast.Node) bool) bool {
		hay := false
		ast.Inspect(st, func(n ast.Node) bool {
			if n != nil && pred(n) {
				hay = true
			}
			return !hay
		})
		return hay
	}

	directa, stdio := -1, -1
	var envueltas []string
	for i, st := range cuerpo.List {
		if es, ok := st.(*ast.ExprStmt); ok && llamaALanzar(es.X) {
			if directa < 0 {
				directa = i
			}
		} else if contiene(st, llamaALanzar) {
			envueltas = append(envueltas, fmt.Sprintf("main.go:%d, adentro de un %T", fset.Position(st.Pos()).Line, st))
		}
		if stdio < 0 && contiene(st, arrancaStdio) {
			stdio = i
		}
	}

	if directa < 0 {
		donde := "no aparece en el cuerpo"
		if len(envueltas) > 0 {
			donde = "aparece en " + strings.Join(envueltas, "; ")
		}
		t.Fatalf("runDaemon no llama a lanzarMantenimientoYGrafo como sentencia suelta de su cuerpo (%s). "+
			"Adentro de un go func, un defer o un if el nombre sigue ahí y la guarda de texto pasa, pero la "+
			"llamada puede correr al salir o no correr nunca: el daemon sirve sin mantenimiento de la memoria "+
			"y con el grafo rancio, y RESPONDE IGUAL que uno sano", donde)
	}
	if stdio < 0 {
		t.Fatal("no encontré en runDaemon la sentencia que llama a server.Start(): sin ella esta guarda ya no sabe qué es «antes del loop stdio»")
	}
	if directa > stdio {
		t.Errorf("runDaemon lanza los ciclos (main.go:%d) DESPUÉS de la sentencia que arranca el loop stdio (main.go:%d): "+
			"server.Start() bloquea hasta el EOF de stdin, así que los ciclos arrancarían con el cliente ya ido",
			fset.Position(cuerpo.List[directa].Pos()).Line, fset.Position(cuerpo.List[stdio].Pos()).Line)
	}
}
