package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// formasDisfrazadas arma un argv con `cabeza` adelante en las formas que LimpiarArgv deshace —con
// blancos alrededor y con una parte vacía o en blanco antes—, que son las formas en que lo que
// llega a la tool difiere de lo que se guarda y se ejecuta. Es el mismo eje que recorre
// formasDeLaCabeza en internal/fleet, acotado a lo que cabe en la cola de una máquina.
func formasDisfrazadas(cabeza string, cola ...string) [][]string {
	var out [][]string
	for _, pre := range [][]string{nil, {""}, {" "}} {
		for _, c := range []string{cabeza, " " + cabeza, "\t" + cabeza + "\n"} {
			out = append(out, append(append(append([]string{}, pre...), c), cola...))
		}
	}
	return out
}

// CON `exec` NO SE ENCOLA UNA OPERACIÓN INTERNA, VENGA COMO VENGA.
//
// DEFECTO VIVO en el árbol sano (5017a45), medido con este servidor de prueba: una credencial con
// SÓLO `exec` mandó `["", "musubi:pantalla", <sesión>, <contraseña>, "30m"]` y la tool contestó
// «encolado». La guarda miraba `args.Argv[0]` crudo —vacío, o sea «no es interna»—, EncolarComando
// limpiaba el argv a un `musubi:pantalla` perfecto, y el agente, que despacha sobre el argv limpio,
// habría puesto esa contraseña en RustDesk: una pantalla abierta sin `screen` y sin la fila de
// `screen_sessions` que registra quién miró (G7). Con `[" ", "\tmusubi:pantalla", …]` igual, y ahí
// además la fila quedaba guardada con el tab adelante, que el tapado de A74 compara exacto: medido,
// después de la entrega la contraseña seguía en claro en la tabla.
//
// Es la puerta lateral que S6 cerró —fabricar los mensajes de la pantalla teniendo sólo el permiso
// de ejecutar— abierta otra vez por la FORMA. Quien tiene `exec` sin allowlist ya puede correr
// cualquier cosa en la máquina; lo que se saltea es la compuerta y la bitácora de `screen`, o sea
// que `exec` y `screen` sean dos permisos. La guarda vieja
// (TestConExecNoSePuedeFabricarUnaSesionDePantalla) clavaba la forma en `["musubi:pantalla", …]`:
// sin partes adelante, que es justo donde argv[0] crudo y la cabeza limpia son el mismo elemento.
//
// EXPOSICIÓN: no se puede medir sin tocar el cerebro, y este tema no toca máquinas. Hace falta una
// credencial con `exec` SIN allowlist (con allowlist, PermiteArgv compara la cabeza limpia contra
// la lista y `musubi:pantalla` no está en ninguna). La fila que deja es indistinguible de una
// legítima: las 4 `musubi:pantalla` que midió la auditoría habría que cruzarlas contra
// `screen_sessions` para descartarlo.
//
// TRES COSAS QUE LA PRIMERA VERSIÓN DE ESTA PRUEBA CLAVABA, señaladas por la revisión de T7:
//
//   - EL LARGO. Todas las formas llevaban cola —cuatro o cinco partes—, así que un
//     `len(args.Argv) > 1 &&` delante de la guarda dejaba en mcp un solo rojo, el del censo, y
//     `["musubi:pantalla"]` pelado se encolaba (medido por la revisión). La guarda vieja usa
//     cuatro partes: tampoco lo ve. Ahora cada forma va con cola y sin cola, el piso exige los dos
//     extremos, y ese sabotaje es la segunda directiva de abajo.
//   - EL MENSAJE. Para no confundir el rechazo de la guarda con el de la cola llena —los dos son
//     `unauthorized`—, la prueba leía «operaciones internas» en el texto, y cambiarlo por «mensajes
//     internos del canal», que es correcto, la ponía roja (la falla 7 de sabotaje.sh). Ahora se
//     mide la COLA antes y después de cada llamada: la cola llena no puede tapar una fuga, porque
//     para llenarse algo tuvo que encolarse, y la llamada que lo encoló ya es roja acá. El `arreglo`
//     de la primera directiva es justamente ese cambio de texto, y la prueba tiene que seguir verde.
//   - LA LISTA DE OPERACIONES. Iban escritas a mano; ahora salen del bloque const de internal/fleet
//     (operacionesInternasDeFleet), así que una operación nueva entra sola.
//
// Sabotaje: que la guarda del exec vuelva a mirar sólo la primera parte cruda → pasan las formas
// con una parte vacía adelante.
// arnes: archivo="internal/mcp/methods_exec.go"
// arnes: de="\tif fleet.EsOperacionInterna(args.Argv) {"
// arnes: a="\tif len(args.Argv) > 0 && fleet.EsOperacionInterna(args.Argv[:1]) {"
// arnes: arreglo_de="son operaciones internas del canal, no comandos del host: no se pueden encolar con exec"
// arnes: arreglo_a="son mensajes internos del canal, no comandos del host: no se pueden encolar con exec"
// arnes: colision_ok="TestConExecNoSePuedeFabricarUnaSesionDePantalla TestConExecNoSeEncolaUnaOperacionInternaDisfrazada"
// Sabotaje: que la guarda del exec exija dos partes para reconocer una operación interna (el eje
// del largo, la forma de C1-m3 en este consumidor) → pasa la operación interna sin argumentos.
// arnes: archivo="internal/mcp/methods_exec.go"
// arnes: de="\tif fleet.EsOperacionInterna(args.Argv) {"
// arnes: a="\tif len(args.Argv) > 1 && fleet.EsOperacionInterna(args.Argv) {"
// arnes: colision_ok="TestConExecNoSePuedeFabricarUnaSesionDePantalla TestConExecNoSeEncolaUnaOperacionInternaDisfrazada"
func TestConExecNoSeEncolaUnaOperacionInternaDisfrazada(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "")

	soloExec := &Principal{
		Name: "op", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"*"}},
	}
	exec := func(argv []string) (map[string]any, *RpcError) {
		t.Helper()
		res, e := callAsPrincipal(t, s, soloExec, "musubi_fleet_exec", map[string]any{
			"device": "pc-gio", "argv": argv, "no_wait": true,
		})
		if e != nil {
			return nil, e
		}
		return jsonOf(t, res), nil
	}
	// enCola cuenta las filas de la máquina. El tope de lectura está muy por encima de lo que esta
	// prueba puede encolar (ColaMaxPorDevice por máquina), y si lo alcanzara la cuenta dejaría de
	// decir si creció: eso es un Fatal, no un número.
	const topeDeLectura = 10 * fleet.ColaMaxPorDevice
	enCola := func() int {
		t.Helper()
		cs, err := s.engine.BitacoraDeComandos("casa", "", topeDeLectura)
		if err != nil {
			t.Fatal(err)
		}
		if len(cs) >= topeDeLectura {
			t.Fatalf("la bitácora llenó el tope de lectura (%d filas): la cuenta ya no dice si la cola creció", len(cs))
		}
		return len(cs)
	}

	const secreto = "ContraseñaElegida42"
	internas := append(operacionesInternasDeFleet(t), "musubi:todavia-no-existe")
	// Cada forma con cola (cuatro partes limpias) y sin cola (una): los dos extremos del largo.
	formas := func(cabeza string, cola ...string) [][]string {
		return append(formasDisfrazadas(cabeza, cola...), formasDisfrazadas(cabeza)...)
	}
	porLargo := map[int]int{} // partes del argv que se guardaría → formas medidas
	for _, op := range internas {
		for _, argv := range formas(op, "ses-x", secreto, "30m") {
			porLargo[len(fleet.LimpiarArgv(argv))]++
			antes := enCola()
			out, e := exec(argv)
			despues := enCola()
			if e == nil {
				// Lo que el agente despacha es lo guardado, limpiado otra vez de su lado.
				t.Errorf("FUGA DE CAPACIDAD: con sólo `exec` se encoló %q (%v): el agente lo despacha como "+
					"%q, una operación interna del canal", argv, out["argv"], fleet.LimpiarArgv(fleet.LimpiarArgv(argv))[0])
				continue
			}
			// EL RECHAZO NO SE LEE EN EL TEXTO: se mide en la cola. Ver el comentario de la prueba.
			if despues != antes {
				t.Errorf("%q se rechazó (%d: %s) y aun así la cola pasó de %d a %d filas: algo se encoló "+
					"con un pedido que exec no puede hacer", argv, e.Code, e.Message, antes, despues)
			}
			if e.Code != codeUnauthorized {
				t.Errorf("%q se rechazó con el código %d y no por falta de permiso: si la guarda no lo hubiera "+
					"frenado, lo habría encolado o no según otra regla (%s)", argv, e.Code, e.Message)
			}
		}
	}
	// EL PISO: los dos extremos del largo, para cada operación. Sin el de una parte, esta prueba
	// vuelve a ser la que dejaba pasar `len(args.Argv) > 1`.
	if porLargo[1] < len(internas) || porLargo[4] < len(internas) {
		t.Fatalf("formas medidas por largo: %v, para %d operaciones: faltan las de una parte o las de "+
			"cuatro, y la prueba no recorrió el eje del largo", porLargo, len(internas))
	}
	cs, err := s.engine.BitacoraDeComandos("casa", "", topeDeLectura)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		if fleet.EsOperacionInterna(c.Argv) {
			t.Errorf("quedó encolada una operación interna pedida por exec: %q", c.Argv)
		}
		if strings.Contains(strings.Join(c.Argv, " "), secreto) {
			t.Errorf("la contraseña quedó guardada en la cola: %q", c.Argv)
		}
	}

	// CONTROL: las mismas formas con un comando del host SE ENCOLAN, con cola y sin ella —sin esto,
	// una guarda que rechazara todo, o todo lo de una parte, pasaría la mitad de arriba— y lo que
	// se guarda es lo que el agente va a ejecutar: limpiarlo otra vez, como hace el agente, no lo
	// cambia.
	for _, argv := range formas("uptime", "-p") {
		antes := enCola()
		out, e := exec(argv)
		if e != nil {
			t.Errorf("un comando del host con la forma %q se rechazó: %s", argv, e.Message)
			continue
		}
		if despues := enCola(); despues != antes+1 {
			t.Errorf("un comando del host con la forma %q contestó «encolado» y la cola pasó de %d a %d filas: "+
				"la medida de arriba no estaría midiendo la cola", argv, antes, despues)
		}
		id, _ := out["command_id"].(string)
		c, ok, err := s.engine.ComandoPorID(id)
		if err != nil || !ok {
			t.Fatalf("no se pudo releer el comando %q (ok=%v): %v", id, ok, err)
		}
		if ejecuta := fleet.LimpiarArgv(c.Argv); !slices.Equal(ejecuta, c.Argv) {
			t.Errorf("con %q se guardó %q y el agente ejecuta %q: lo registrado no es lo corrido", argv, c.Argv, ejecuta)
		}
	}
}

// operacionesInternasDeFleet devuelve, ordenadas, las operaciones internas que declara el bloque
// const de internal/fleet: cada constante cuyo valor es un literal con fleet.PrefijoOperacionInterna
// adelante y algo después.
//
// Se lee el FUENTE del dominio, igual que TestTodaOperacionInternaDelCodigoEstaClasificada, y no
// una lista: la lista es lo que se olvida el día que alguien agrega `OpActualizar`. Es la misma
// fuente que opsInternasDeclaradas en internal/fleet, que no se puede importar desde acá.
func operacionesInternasDeFleet(t *testing.T) []string {
	t.Helper()
	dir := filepath.Join("..", "fleet")
	entradas, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("no se pudo leer %s: %v", dir, err)
	}
	fset := token.NewFileSet()
	var out []string
	for _, e := range entradas {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				continue
			}
			for _, sp := range g.Specs {
				vs := sp.(*ast.ValueSpec)
				for i, nombre := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						// Una operación armada con una expresión no la ve este barrido, y callarlo
						// sería el verde por vacío de siempre.
						if expr := types.ExprString(vs.Values[i]); strings.Contains(expr, "PrefijoOperacionInterna") ||
							strings.Contains(expr, fleet.PrefijoOperacionInterna) {
							t.Fatalf("%s: %s arma una operación interna con una expresión; ampliá el barrido",
								fset.Position(vs.Pos()), nombre.Name)
						}
						continue
					}
					valor, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatal(err)
					}
					if strings.HasPrefix(valor, fleet.PrefijoOperacionInterna) && valor != fleet.PrefijoOperacionInterna {
						out = append(out, valor)
					}
				}
			}
		}
	}
	// EL PISO: si el barrido no encontrara nada, la prueba no mediría ninguna operación declarada.
	if len(out) < 4 {
		t.Fatalf("el barrido de %s encontró %d operaciones internas (%v); había 4 cuando se escribió "+
			"esta prueba: el barrido está roto", dir, len(out), out)
	}
	sort.Strings(out)
	return out
}
