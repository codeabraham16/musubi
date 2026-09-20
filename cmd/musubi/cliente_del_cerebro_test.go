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
		if !strings.HasPrefix(rel, "cmd/musubi/") || strings.HasSuffix(rel, "_test.go") {
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
				lectoresDelNombre = append(lectoresDelNombre, fn.Name.Name)
			}
		}

		// Los `http.Client{…}` armados a mano, y si traen su motivo al lado.
		for _, lit := range clientesLiterales(f) {
			if dentroDe(f, lit, "clienteHaciaElCerebro") {
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
	if archivos < 20 {
		t.Fatalf("se parsearon %d archivos de cmd/musubi y el paquete tiene decenas: el barrido "+
			"dejó de mirar y esta guarda estaría verde sin haber comprobado nada", archivos)
	}
	if len(marcados) == 0 {
		t.Fatal("NINGÚN `http.Client` quedó marcado como ajeno al cerebro, y hay al menos dos " +
			"(la API de Anthropic en calibrate.go y la bajada de URLs en fetch.go).\n" +
			"  O el reconocedor de literales se rompió, o el de la marca. En los dos casos el cero " +
			"de culpables de abajo no significa «no hay»: significa «no estoy mirando».")
	}
	// EL CENSO NO LLEVA EL NÚMERO QUE EL SABOTAJE MUEVE, y es a propósito. Cuando llevaba también
	// los «sin declarar», esta línea CAMBIABA con el defecto puesto, así que el arnés la elegía como
	// «el motivo del rojo» —siendo un `t.Logf` y no una aserción— y cada corrida del paquete salía
	// con el aviso «el motivo es un t.Log, compara por lo que la prueba IMPRIME». El aviso era
	// cierto y no se podía contestar, que es como se entrena a ignorar los avisos. Los culpables se
	// cuentan solos: uno por `t.Errorf`, abajo.
	t.Logf("%d archivos de cmd/musubi; %d cliente(s) declarados ajenos al cerebro", archivos, len(marcados))

	for _, x := range culpables {
		t.Errorf("UN `http.Client` ARMADO A MANO EN %s.\n"+
			"  Si le habla al CEREBRO CENTRAL, le falta el nombre contra el que verificar el\n"+
			"  certificado: el del tailnet NO tiene SAN de IP, y las Windows discan la IP porque con\n"+
			"  NordVPN el MagicDNS no resuelve. Sin declararlo, el handshake verifica contra la IP y\n"+
			"  falla — y el error habla del certificado, no de la causa.\n"+
			"  Arreglo: `clienteHaciaElCerebro(nombreTLSDelCerebro(), <espera>, <transport o nil>)`.\n"+
			"  El Transport lo seguís armando vos si necesitás una forma propia; el nombre se declara\n"+
			"  DESPUÉS, que es lo que impide olvidarlo.\n"+
			"  Si NO le habla al cerebro, decilo al lado del literal con una línea que empiece por\n"+
			"  «%s» y el motivo. Vive pegada al código a propósito: una lista central de excepciones\n"+
			"  envejece lejos de lo que describe.", x, laMarcaDeQueNoEsElCerebro)
	}

	// ── EL SEGUNDO LECTOR ──────────────────────────────────────────────────────────────────
	if len(lectoresDelNombre) != 1 || lectoresDelNombre[0] != "nombreTLSDelCerebro" {
		t.Errorf("`%s` se lee en %v, y tiene que leerse SÓLO en `nombreTLSDelCerebro`.\n"+
			"  Un segundo lector es un segundo lugar donde olvidarse, y es exactamente cómo este\n"+
			"  paquete terminó con seis clientes sin el nombre declarado.", envNombreTLS, lectoresDelNombre)
	}
}

func funcionesDe(f *ast.File) []*ast.FuncDecl {
	var out []*ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
			out = append(out, fd)
		}
	}
	return out
}

// leeElNombreTLS busca `os.Getenv(envNombreTLS)` en el cuerpo, por el IDENTIFICADOR de la
// constante y no por su valor: quien escriba el literal `"MUSUBI_BRAIN_TLS_NAME"` a mano ya está
// haciendo otra cosa mal y lo caza la guarda de literales duplicados.
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
		if id, ok := c.Args[0].(*ast.Ident); ok && id.Name == "envNombreTLS" {
			visto = true
		}
		return true
	})
	return visto
}

// clientesLiterales devuelve los `http.Client{…}` construidos como literal.
func clientesLiterales(f *ast.File) []*ast.CompositeLit {
	var out []*ast.CompositeLit
	ast.Inspect(f, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Client" {
			return true
		}
		if id, ok := sel.X.(*ast.Ident); ok && id.Name == "http" {
			out = append(out, lit)
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
