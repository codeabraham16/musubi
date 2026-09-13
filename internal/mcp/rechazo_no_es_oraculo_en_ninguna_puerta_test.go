package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"

	"musubi/internal/fleet"
)

// puertaDeCredencial es una ruta HTTP que autentica con el limitador de credenciales.
type puertaDeCredencial struct {
	ruta, metodo, handler string
}

// puertasConLimitador DERIVA del fuente las rutas que reciben el limitador de credenciales.
//
// No se listan a mano, y la razón está medida: la guarda que existía (TestElRechazoNoDiceCualExistio)
// probaba UNA puerta, el latido, y el propio código dice en su comentario que hay «tres puertas
// —latido, resultado y salud—». Al ir a mirar eran CINCO: las dos del agente de shell se sumaron
// después, con el mismo limitador y el mismo motivo de rechazo, y nadie les escribió la prueba. Una
// lista a mano habría repetido exactamente ese olvido en la sexta.
//
// Lo que se reconoce es la FORMA del registro —`mux.HandleFunc(<const>, s.<handler>(limiter, …))`—
// y no un nombre de ruta; el valor de la ruta sale de su `const` y el método del
// `r.Method != http.MethodX` del handler.
func puertasConLimitador(t *testing.T) []puertaDeCredencial {
	t.Helper()
	fset := token.NewFileSet()
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var archivos []*ast.File
	for _, e := range entradas {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, 0)
		if err != nil {
			t.Fatalf("no pude parsear %s: %v — no medí nada", n, err)
		}
		archivos = append(archivos, f)
	}

	// Las constantes de string del paquete: de ahí sale el valor de cada ruta.
	constantes := map[string]string{}
	// Los handlers por nombre: de ahí sale el método que exigen.
	handlers := map[string]*ast.FuncDecl{}
	for _, f := range archivos {
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.GenDecl:
				if x.Tok != token.CONST {
					continue
				}
				for _, sp := range x.Specs {
					vs, ok := sp.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, nombre := range vs.Names {
						if i < len(vs.Values) {
							if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.STRING {
								if v, err := strconv.Unquote(lit.Value); err == nil {
									constantes[nombre.Name] = v
								}
							}
						}
					}
				}
			case *ast.FuncDecl:
				if x.Recv != nil && x.Body != nil {
					handlers[x.Name.Name] = x
				}
			}
		}
	}

	var puertas []puertaDeCredencial
	for _, f := range archivos {
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) != 2 {
				return true
			}
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "HandleFunc" {
				return true
			}
			rutaID, ok := c.Args[0].(*ast.Ident)
			if !ok {
				return true
			}
			fabrica, ok := c.Args[1].(*ast.CallExpr)
			if !ok || len(fabrica.Args) == 0 {
				return true
			}
			if lim, ok := fabrica.Args[0].(*ast.Ident); !ok || lim.Name != "limiter" {
				return true
			}
			hsel, ok := fabrica.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ruta, hay := constantes[rutaID.Name]
			if !hay {
				t.Errorf("%s: la ruta %s no es una constante de string del paquete; no puedo derivar su valor",
					fset.Position(c.Pos()), rutaID.Name)
				return true
			}
			metodo := ""
			if h, hay := handlers[hsel.Sel.Name]; hay {
				ast.Inspect(h.Body, func(m ast.Node) bool {
					b, ok := m.(*ast.BinaryExpr)
					if !ok || b.Op != token.NEQ || metodo != "" {
						return true
					}
					if s, ok := b.Y.(*ast.SelectorExpr); ok && strings.HasPrefix(s.Sel.Name, "Method") {
						metodo = strings.ToUpper(strings.TrimPrefix(s.Sel.Name, "Method"))
					}
					return true
				})
			}
			if metodo == "" {
				t.Errorf("%s: no encontré qué método exige %s; la sonda iría con el equivocado y "+
					"mediría el 405, no el 401", fset.Position(c.Pos()), hsel.Sel.Name)
				return true
			}
			puertas = append(puertas, puertaDeCredencial{ruta: ruta, metodo: metodo, handler: hsel.Sel.Name})
			return true
		})
	}
	return puertas
}

// TestNingunaPuertaDeCredencialEsUnOraculo exige que TODA ruta autenticada con el limitador de
// credenciales conteste lo mismo ante un token desconocido, uno bien formado que nunca existió, uno
// revocado y basura.
//
// ────────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EXISTE, SI YA HABÍA UNA
//
// TestElRechazoNoDiceCualExistio (B3) fija esto mismo sobre el LATIDO, y sólo sobre el latido. El
// comentario del código habla de «tres puertas»; derivadas del registro son CINCO:
//
//	/fleet/heartbeat · /fleet/result · /fleet/service-health · /fleet/shell/agent/in · /fleet/shell/agent/out
//
// Las cuatro de la derecha comparten limitador y motivo de rechazo con el latido, y ninguna tenía
// prueba de oráculo — las del shell se probaban sólo con tokens válidos. Es la forma dominante de
// este árbol: la lección aprendida de un lado y no del hermano.
//
// POR QUÉ UN SERVIDOR POR PUERTA: el limitador es local a cada HTTPHandler y corta a los 5 fallos
// por minuto por IP. Cuatro sondas por puerta quedan abajo; cinco puertas contra el MISMO servidor
// lo pasarían, el cuerpo cambiaría a «too many failed auth attempts», y la prueba se pondría roja
// por el candado y no por un oráculo.
//
// POR QUÉ UNA SONDA BIEN FORMADA: el oráculo de formato —decir distinto a lo que «no parece un
// token»— sólo se ve si hay algo con la forma real que no existe. Se le pide a `fleet.NuevoToken`,
// así que acompaña cualquier cambio de formato.
//
// Sabotaje que la hace fallar: en `deviceDeRequest`, contestar un motivo distinto cuando el token no
// tiene el largo del formato real — un oráculo de formato en las dos puertas del shell, que B3 no mira.
// arnes: archivo="internal/mcp/shell_agente_http.go"
// arnes: de="\t\tw.Header().Set(\"WWW-Authenticate\", \"Bearer\")\n\t\thttp.Error(w, motivoRechazo, http.StatusUnauthorized)\n"
// arnes: a="\t\tw.Header().Set(\"WWW-Authenticate\", \"Bearer\")\n\t\trespuesta := motivoRechazo\n\t\tif muestra, _ := fleet.NuevoToken(); len(bearer) != len(muestra) {\n\t\t\trespuesta = \"formato de credencial inválido\"\n\t\t}\n\t\thttp.Error(w, respuesta, http.StatusUnauthorized)\n"
// arnes: prueba="TestNingunaPuertaDeCredencialEsUnOraculo"
func TestNingunaPuertaDeCredencialEsUnOraculo(t *testing.T) {
	puertas := puertasConLimitador(t)

	// CONTROL DE QUE SE MIRÓ ALGO: un verde de cero puertas se lee igual que un verde de todas. Y
	// la del latido tiene que estar: si el barrido no la ve, el reconocedor está roto.
	if len(puertas) == 0 {
		t.Fatal("no derivé NI UNA puerta con el limitador de credenciales: el barrido está roto y esta guarda no mide nada")
	}
	vioElLatido := false
	for _, p := range puertas {
		if p.ruta == fleetHeartbeatPath {
			vioElLatido = true
		}
	}
	if !vioElLatido {
		t.Fatalf("derivé %d puertas y ninguna es el latido (%s): el reconocedor dejó de ver el registro", len(puertas), fleetHeartbeatPath)
	}

	for _, p := range puertas {
		p := p
		t.Run(p.metodo+" "+p.ruta, func(t *testing.T) {
			s, ts, tokenDevice, _ := servidorConFlota(t)
			pedir := func(auth string) (int, string) {
				t.Helper()
				req, _ := http.NewRequest(p.metodo, ts.URL+p.ruta, strings.NewReader(""))
				req.Header.Set("Authorization", "Bearer "+auth)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("%s %s: %v", p.metodo, p.ruta, err)
				}
				defer resp.Body.Close()
				b, _ := io.ReadAll(resp.Body)
				return resp.StatusCode, string(b)
			}

			bienFormado, err := fleet.NuevoToken()
			if err != nil {
				t.Fatalf("NuevoToken falló: %v — no medí nada", err)
			}
			if _, e := call(t, s, "musubi_fleet_revoke", map[string]any{"name": "pc-gio", "project": "casa"}); e != nil {
				t.Fatalf("fleet_revoke: %+v — no medí nada", e)
			}

			type sonda struct{ caso, token string }
			sondas := []sonda{
				{"desconocido", "token-que-jamas-existio"},
				{"bien formado y nunca emitido", bienFormado},
				{"revocado", tokenDevice},
				{"basura", "@@@no-es-un-token@@@"},
			}
			var primero string
			for i, sd := range sondas {
				code, cuerpo := pedir(sd.token)
				// CONTROL DE QUE LA SONDA LLEGÓ A LA CREDENCIAL: un 405 diría que el método derivado
				// está mal, un 429 que el limitador se metió. En los dos casos esto no mide el oráculo.
				if code != http.StatusUnauthorized {
					t.Fatalf("la sonda %q contestó %d y no 401: no llegó a la comprobación de la credencial "+
						"(método equivocado, candado, o la puerta ya no autentica primero) — no medí nada. Cuerpo: %s",
						sd.caso, code, cuerpo)
				}
				if i == 0 {
					primero = cuerpo
					continue
				}
				if cuerpo != primero {
					t.Errorf("%s %s (%s) distingue casos y funciona como ORÁCULO:\n  %s → %q\n  %s → %q\n"+
						"  Un 401 tiene que decir lo mismo sin importar si el token existió, se revocó,\n"+
						"  o no tiene forma de token: si no, sirve para enumerar credenciales.",
						p.metodo, p.ruta, p.handler, sondas[0].caso, primero, sd.caso, cuerpo)
				}
			}
		})
	}
	t.Logf("%d puerta/s con el limitador de credenciales revisadas", len(puertas))
}
