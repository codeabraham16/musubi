package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/arbol"
)

// ─────────────────────────────────────────────────────────────────────────────────────────────
// A129 · TODO CLIENTE QUE LE HABLA AL CEREBRO VERIFICA CONTRA EL NOMBRE DECLARADO
// ─────────────────────────────────────────────────────────────────────────────────────────────

// laMarcaDeQueNoEsElCerebro es lo que tiene que decir un `http.Client` armado a mano para no ser
// acusado. Va PEGADA a la línea, no en una lista central: una lista de excepciones envejece lejos
// del código que describe, y nadie la revisa. Acá el motivo vive donde se lo va a leer.
const laMarcaDeQueNoEsElCerebro = "no habla con el cerebro:"

// TestTodoClienteDelCerebroPasaPorElConstructor — la guarda del hermano, y pregunta UNA cosa.
//
// EL DEFECTO, MEDIDO EL 2026-09-20. `MUSUBI_BRAIN_TLS_NAME` existe porque el certificado del
// tailnet no tiene SAN de IP y las Windows discan la IP (con NordVPN el MagicDNS no resuelve).
// De los SIETE clientes de este paquete que le hablan al cerebro central, el nombre lo llevaba
// UNO: el del latido. Los otros seis se armaban su `http.Client` a mano —`musubi cerebro`,
// `musubi shell`, el canal de shell del agente, el relay del panel y sus dos handlers— y todos
// habrían fallado el día que el cerebro pase a HTTPS por IP, con un error de certificado que no
// habla de la causa. Es el defecto dominante de este repo: la guarda puesta en N−1 de N caminos.
//
// LA PREGUNTA ES SINTÁCTICA Y ES UNA SOLA: ¿hay algún `http.Client{…}` armado a mano en este
// paquete? No hay que decidir a quién le habla —esa pregunta no converge, igual que la de A127—:
// quien no le habla al cerebro lo DECLARA al lado, con un motivo, y esa declaración es cara de
// escribir por accidente.
//
// Y LA SEGUNDA MITAD, que es la que evita el rodeo: `MUSUBI_BRAIN_TLS_NAME` se lee en UN SOLO
// lugar (`nombreTLSDelCerebro`). Un segundo lector es un segundo lugar donde olvidarse.
//
// Sabotaje verificado que la pone roja: devolverle a `musubi shell` su cliente a mano.
// arnes: archivo="cmd/musubi/shell.go"
// arnes: de="\tcli := clienteHaciaElCerebro(nombreTLSDelCerebro(), 40*time.Second, nil)"
// arnes: a="\tcli := &http.Client{Timeout: 40 * time.Second}"
func TestTodoClienteDelCerebroPasaPorElConstructor(t *testing.T) {
	raiz := filepath.Join("..", "..")
	fset := token.NewFileSet()

	// El árbol lo dice git y no el directorio (A128).
	gos, err := arbol.ConSufijo(raiz, ".go")
	if err != nil {
		t.Fatal(err)
	}

	var culpables, marcados, lectoresDelNombre []string
	archivos := 0
	for _, rel := range gos {
		// EL ALCANCE ES EL REPO ENTERO, y hasta el 2026-09-20 era `cmd/musubi/`. Esa acotación
		// dejaba exactamente el defecto que esta guarda vino a cerrar, en otros dos paquetes:
		// el cliente del sync saliente (internal/mcp/syncclient.go), que es el que usan TODOS
		// los daemons, y los dos del self-check de provisioning, que además tenían el esquema
		// `http://` escrito a máquina. La guarda en N−1 de N caminos, con el N−1 creado por su
		// propio alcance.
		if strings.HasSuffix(rel, "_test.go") {
			continue
		}
		ruta := filepath.Join(raiz, filepath.FromSlash(rel))
		f, e := parser.ParseFile(fset, ruta, nil, parser.ParseComments)
		if e != nil {
			t.Errorf("%s no parsea (%v): un archivo invisible acá es un cliente sin verificar que "+
				"nadie va a reclamar", rel, e)
			continue
		}
		archivos++
		crudo, _ := filepath.Abs(ruta)
		_ = crudo

		// ¿Quién lee la variable del nombre TLS? Tiene que ser uno solo.
		for _, fn := range funcionesDe(f) {
			if leeElNombreTLS(fn) {
				lectoresDelNombre = append(lectoresDelNombre, rel+":"+fn.Name.Name)
			}
		}

		// Los `http.Client{…}` armados a mano, y si traen su motivo al lado.
		for _, lit := range clientesArmadosAMano(f) {
			if elConstructorCompartido(rel, f, lit) {
				continue
			}
			linea := fset.Position(lit.Pos()).Line
			if motivoCerca(f, fset, linea) {
				marcados = append(marcados, rel)
				continue
			}
			culpables = append(culpables, rel+":"+itoa(linea))
		}
	}

	// ── CONTROLES QUE NO PUEDEN ENMUDECER ──────────────────────────────────────────────────
	// Una guarda que espera CERO escribe igual «medí y no hay» que «no pude medir».
	if archivos < 300 {
		t.Fatalf("se parsearon %d archivos .go y el repo tiene más de 350: el barrido dejó de "+
			"mirar y esta guarda estaría verde sin haber comprobado nada", archivos)
	}
	if len(marcados) < 11 {
		t.Fatalf("sólo %d cliente(s) quedaron marcados como ajenos al cerebro, y hay CATORCE en el "+
			"repo: Anthropic, la bajada de URLs de `musubi ingest`, OpenAI ×2, Ollama, el mirror de "+
			"modelos, el raspado de métricas, dos de ingesta, GitHub (selfupdate), el colector OTLP "+
			"y los tres de skillsource.\n"+
			"  O el reconocedor de literales se rompió, o el de la marca. En los dos casos el cero "+
			"de culpables de abajo no significa «no hay»: significa «no estoy mirando».", len(marcados))
	}
	// EL CENSO NO LLEVA EL NÚMERO QUE EL SABOTAJE MUEVE, y es a propósito. Cuando llevaba también
	// los «sin declarar», esta línea CAMBIABA con el defecto puesto, así que el arnés la elegía como
	// «el motivo del rojo» —siendo un `t.Logf` y no una aserción— y cada corrida del paquete salía
	// con el aviso «el motivo es un t.Log, compara por lo que la prueba IMPRIME». El aviso era
	// cierto y no se podía contestar, que es como se entrena a ignorar los avisos. Los culpables se
	// cuentan solos: uno por `t.Errorf`, abajo.
	t.Logf("%d archivos .go del repo; %d cliente(s) declarados ajenos al cerebro", archivos, len(marcados))

	for _, x := range culpables {
		t.Errorf("UN `http.Client` ARMADO A MANO EN %s.\n"+
			"  Si le habla al CEREBRO CENTRAL, le falta el nombre contra el que verificar el\n"+
			"  certificado: el del tailnet NO tiene SAN de IP, y las Windows discan la IP porque con\n"+
			"  NordVPN el MagicDNS no resuelve. Sin declararlo, el handshake verifica contra la IP y\n"+
			"  falla — y el error habla del certificado, no de la causa.\n"+
			"  Arreglo: `cerebro.Cliente(cerebro.NombreTLS(), <espera>, <transport o nil>)`, de\n"+
			"  `musubi/internal/cerebro`.\n"+
			"  El Transport lo seguís armando vos si necesitás una forma propia; el nombre se declara\n"+
			"  DESPUÉS, que es lo que impide olvidarlo.\n"+
			"  Si NO le habla al cerebro, decilo al lado del literal con una línea que empiece por\n"+
			"  «%s» y el motivo. Vive pegada al código a propósito: una lista central de excepciones\n"+
			"  envejece lejos de lo que describe.", x, laMarcaDeQueNoEsElCerebro)
	}

	// ── EL SEGUNDO LECTOR ──────────────────────────────────────────────────────────────────
	const elUnicoLector = elArchivoDelConstructor + ":NombreTLS"
	if len(lectoresDelNombre) != 1 || lectoresDelNombre[0] != elUnicoLector {
		t.Errorf("`%s` se lee en %v, y tiene que leerse SÓLO en `%s`.\n"+
			"  Un segundo lector es un segundo lugar donde olvidarse, y es exactamente cómo este\n"+
			"  repo terminó con seis clientes sin el nombre declarado en cmd/musubi y cuatro más "+
			"afuera.", envNombreTLS, lectoresDelNombre, elUnicoLector)
	}
}

// elConstructorCompartido exceptúa al ÚNICO literal que tiene derecho a existir: el que arma
// `internal/cerebro.Cliente`. Se exceptúa por FUNCIÓN y dentro de su archivo, nunca el archivo
// entero: exceptuar el archivo dejaría ciega a la guarda justo donde vive lo que custodia, que en
// este repo ya pasó cuatro veces. Que el constructor no relaje el TLS lo custodia otra prueba
// (TestElClienteDelLatidoDeclaraElNombreYNoApagaLaVerificacion), con su propio sabotaje.
func elConstructorCompartido(rel string, f *ast.File, lit ast.Node) bool {
	if rel != elArchivoDelConstructor {
		return false
	}
	for _, fn := range funcionesDe(f) {
		// FUNCIÓN SUELTA Y NO MÉTODO: `dentroDe` casa por nombre, y un método `func (x T) Cliente()`
		// en este mismo archivo se llama igual. La excepción es para EL constructor, no para
		// cualquier cosa que se llame como él.
		if fn.Name.Name == "Cliente" && fn.Recv == nil && fn.Pos() <= lit.Pos() && lit.End() <= fn.End() {
			return true
		}
	}
	return false
}

// elArchivoDelConstructor está aparte para que la excepción tenga UN dueño y se lea de un vistazo.
const elArchivoDelConstructor = "internal/cerebro/cliente.go"

func funcionesDe(f *ast.File) []*ast.FuncDecl {
	var out []*ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
			out = append(out, fd)
		}
	}
	return out
}

// leeElNombreTLS busca una lectura de `MUSUBI_BRAIN_TLS_NAME` en el cuerpo, en las TRES formas en
// que se puede escribir. Miraba sólo la primera, y ésa es la única que un paquete de AFUERA no
// puede usar — o sea que estaba ciega exactamente donde puede aparecer el segundo lector:
//
//	os.Getenv(EnvNombreTLS)          identificador, dentro del paquete que la declara
//	os.Getenv(cerebro.EnvNombreTLS)  selector, que es lo que escribe cualquier otro paquete
//	os.Getenv("MUSUBI_BRAIN_TLS_NAME")  el literal a mano, que es la forma más fácil de olvidar
func leeElNombreTLS(fn *ast.FuncDecl) bool {
	visto := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok || len(c.Args) != 1 {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Getenv" {
			return true
		}
		switch a := c.Args[0].(type) {
		case *ast.Ident:
			if a.Name == "envNombreTLS" || a.Name == "EnvNombreTLS" {
				visto = true
			}
		case *ast.SelectorExpr:
			if a.Sel.Name == "envNombreTLS" || a.Sel.Name == "EnvNombreTLS" {
				visto = true
			}
		case *ast.BasicLit:
			if a.Kind == token.STRING && strings.Contains(a.Value, "MUSUBI_BRAIN_TLS_NAME") {
				visto = true
			}
		}
		return true
	})
	return visto
}

// clientesArmadosAMano devuelve TODA forma de conseguir un cliente HTTP que no sea el
// constructor compartido. Se llamaba `clientesLiterales` y sólo miraba `http.Client{…}`, que es
// una de cuatro: preguntar por una FORMA deja afuera las demás, y el repo ya tenía tres usos de
// `http.DefaultClient` —que no es un literal— invisibles para la guarda.
//
//	http.Client{…}     literal compuesto
//	new(http.Client)   constructor del stdlib
//	var c http.Client  declaración con tipo
//	http.DefaultClient el cliente global, que NO declara ServerName y cualquiera puede usar
func clientesArmadosAMano(f *ast.File) []ast.Node {
	var out []ast.Node
	esHTTPClient := func(e ast.Expr) bool {
		sel, ok := e.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Client" {
			return false
		}
		id, ok := sel.X.(*ast.Ident)
		return ok && id.Name == "http"
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CompositeLit:
			if esHTTPClient(v.Type) {
				out = append(out, v)
			}
		case *ast.CallExpr:
			if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "new" && len(v.Args) == 1 && esHTTPClient(v.Args[0]) {
				out = append(out, v)
			}
		case *ast.ValueSpec:
			if v.Type != nil && esHTTPClient(v.Type) {
				out = append(out, v)
			}
		case *ast.SelectorExpr:
			if v.Sel.Name == "DefaultClient" {
				if id, ok := v.X.(*ast.Ident); ok && id.Name == "http" {
					out = append(out, v)
				}
			}
		}
		return true
	})
	return out
}

func dentroDe(f *ast.File, n ast.Node, nombre string) bool {
	for _, fn := range funcionesDe(f) {
		if fn.Name.Name == nombre && fn.Pos() <= n.Pos() && n.End() <= fn.End() {
			return true
		}
	}
	return false
}

// motivoCerca mira las líneas de comentario INMEDIATAMENTE anteriores al literal. No se busca en
// todo el archivo a propósito: un comentario en otra función no declara nada sobre éste, y esa
// confusión —el texto que está donde no decide— ya costó siete guardas verdes en este repo.
func motivoCerca(f *ast.File, fset *token.FileSet, linea int) bool {
	for _, g := range f.Comments {
		fin := fset.Position(g.End()).Line
		if fin != linea-1 {
			continue
		}
		texto := g.Text()
		i := strings.Index(texto, laMarcaDeQueNoEsElCerebro)
		if i < 0 {
			continue
		}
		motivo := strings.TrimSpace(texto[i+len(laMarcaDeQueNoEsElCerebro):])
		if len(strings.Fields(motivo)) >= 6 {
			return true
		}
	}
	return false
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
