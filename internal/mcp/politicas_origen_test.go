package mcp

// A131 · T5 — TODO LO QUE ESCRIBE EL BARRIDO DE UNA POLÍTICA SE LEE COMO AUTOMÁTICO.
//
// La acción de una política va a la MISMA bitácora que la de una persona (I16) y se DISTINGUE al
// leer por su origen (A59). Las guardas de antes miraban esa distinción con UNA política de host y
// sobre una máquina que no sabe avisar; las de este archivo recorren las condiciones enteras y los
// planos enteros, leídos del código.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/fleet"
)

// condicionesDeclaradas devuelve las constantes de tipo Condicion, leídas del AST de
// internal/fleet/politica.go. Es el conjunto CERRADO que recorre
// TestLaAccionYElAvisoDeUnaPoliticaSeLeenComoAutomaticosEnCadaCondicion: una condición nueva sin su
// fila la pone roja.
func condicionesDeclaradas(t *testing.T) map[fleet.Condicion]string {
	t.Helper()
	const fuente = "../fleet/politica.go"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, fuente, nil, 0)
	if err != nil {
		t.Fatalf("no se pudo parsear %s: %v", fuente, err)
	}
	out := map[fleet.Condicion]string{}
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			tipo, ok := vs.Type.(*ast.Ident)
			if !ok || tipo.Name != "Condicion" {
				continue
			}
			for i, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("%s: una constante de Condicion no es un literal de cadena y esta guarda no la puede leer", fset.Position(v.Pos()))
				}
				valor, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("%s: %v", fset.Position(v.Pos()), err)
				}
				out[fleet.Condicion(valor)] = vs.Names[i].Name
			}
		}
	}
	// PISO: cero condiciones no es «no hay condiciones», es que el tipo se renombró o se mudó de
	// archivo y la tabla dejó de medirse. Hoy son ocho: seis de host y dos de servicio.
	if len(out) < 6 {
		t.Fatalf("encontré %d constantes de Condicion en %s y son al menos seis: el tipo cambió de nombre o de lugar y esta guarda quedó mirando al aire", len(out), fuente)
	}
	return out
}

// argvDeFila lee el argv de una fila de musubi_fleet_log.
func argvDeFila(fila map[string]any) []string {
	crudo, _ := fila["argv"].([]any)
	argv := make([]string, 0, len(crudo))
	for _, a := range crudo {
		v, _ := a.(string)
		argv = append(argv, v)
	}
	return argv
}

// bitacoraComoLaLeeUnaPersona devuelve las filas de musubi_fleet_log de la casa, leídas con una
// credencial de persona con exec sobre todo el proyecto: la superficie donde el origen se MUESTRA,
// no el valor guardado en la base.
func bitacoraComoLaLeeUnaPersona(t *testing.T, s *McpServer) []map[string]any {
	t.Helper()
	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_log", map[string]any{})
	if e != nil {
		t.Fatalf("fleet_log: %+v", e)
	}
	crudas, _ := jsonOf(t, res)["comandos"].([]any)
	filas := make([]map[string]any, 0, len(crudas))
	for _, x := range crudas {
		fila, _ := x.(map[string]any)
		filas = append(filas, fila)
	}
	return filas
}

// A131 · T5 — LA ACCIÓN Y EL AVISO DE UNA POLÍTICA SE LEEN COMO AUTOMÁTICOS, SEA CUAL SEA LA CONDICIÓN.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ EJES CLAVABA LA GUARDA DE ANTES
//
// Las guardas que verifican que correrAccionDePolitica estampa `Origen: politica` —
// TestLaAccionDeUnaPoliticaQuedaEnLaMismaBitacoraQueLasPersonas y, desde T1, la del argv entero,
// que filtra por origen; TestElOrigenAutomaticoSeDistingue… siembra la fila a mano— lo hacen con
// UNA política, `mem_pct`, de host. Así, un origen que dependiera de la CLASE de política (P2-m3:
// `persona` si es de servicio) las dejaba en verde: la primera política de servicio que alguien
// declare —el caso para el que existe A44— habría escrito sus reinicios como pedidos de una persona.
//
// Y clavaba otro eje: su máquina no sabe preguntar (`puede_preguntar=false`), así que el barrido
// no encola el `musubi:avisar` y la fila del aviso nunca se miraba. Ese aviso salía con
// `Origen: persona` (P2-live1, un defecto VIVO): `encolarAvisoDeAcceso` lo tenía escrito a fuego
// para los cuatro planos. En la bitácora y en la cronología —HechoDeComando copia el origen—, una
// acción que disparó una regla se leía `origen: persona, automatico: false`.
//
// Exposición medida (auditoría A131): 0 filas. La única política configurada, `vaciar-journal`, es
// de host y su única máquina (`musubi-server`) tiene `puede_preguntar=0`: la base tiene 0 avisos
// con el texto de la política, y las 7 filas `politica` de la historia son de `prueba-de-fuego`,
// también de host. Los dos defectos eran latentes: el primero se volvía vivo con la primera
// política de servicio, el segundo con la primera política sobre una máquina que sabe avisar
// (davantis-1 está en `avisa` con `puede_preguntar=1`, a un `devices:` de distancia).
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE
//
// Una fila por CADA condición declarada en internal/fleet/politica.go, leídas del AST: una
// condición nueva sin su fila pone esto rojo. En cada fila la máquina está en `avisa` y SABE
// preguntar, así que el barrido escribe DOS filas —el comando y el aviso—; la bitácora está vacía
// antes, y se lee con la misma tool que usa una persona. TODA fila tiene que decir
// `origen: politica` y `automatico: true`. El piso exige exactamente un comando (con el argv de la
// política) y un aviso: sin las dos filas, «toda fila es automática» pasaría sobre una bitácora
// que no tiene la que importa.
//
// Sabotaje: que el origen del comando dependa de la clase de política (P2-m3).
//
// La guarda de I16 sabotea esta MISMA línea —le saca el origen entero—, así que los dos `de` se
// pisan a propósito: aquélla mide que el campo exista, ésta que no dependa de la condición.
// arnes: archivo="internal/mcp/politicas.go"
// arnes: de="\t\t\tOrigen: fleet.OrigenPolitica,\n"
// arnes: a="\t\t\tOrigen: map[bool]fleet.OrigenComando{true: fleet.OrigenPersona, false: fleet.OrigenPolitica}[pol.EsDeServicio()],\n"
// arnes: colision_ok="TestLaAccionDeUnaPoliticaQuedaEnLaMismaBitacoraQueLasPersonas"
//
// Sabotaje: que el aviso de la política vuelva a declararse como de una persona (P2-live1).
// arnes: archivo="internal/mcp/methods_pantalla.go"
// arnes: de="\"politica\", fleet.OrigenPolitica}"
// arnes: a="\"politica\", fleet.OrigenPersona}"
func TestLaAccionYElAvisoDeUnaPoliticaSeLeenComoAutomaticosEnCadaCondicion(t *testing.T) {
	// Cómo se cumple cada condición vive en disparosPorCondicion (politicas_alcance_test.go) y no acá:
	// la tabla del alcance de A131·T3 recorre las mismas condiciones y las tiene que cumplir igual.
	filas := disparosPorCondicion()

	// LA TABLA RECORRE EL CONJUNTO ENTERO, derivado de su fuente, en los dos sentidos.
	declaradas := condicionesDeclaradas(t)
	var sinFila []string
	for c, nombre := range declaradas {
		if _, ok := filas[c]; !ok {
			sinFila = append(sinFila, nombre+" ("+strconv.Quote(string(c))+")")
		}
	}
	sort.Strings(sinFila)
	if len(sinFila) > 0 {
		t.Errorf("la tabla no tiene fila para %s: una condición sin fila es justo la que puede escribir sus "+
			"acciones como pedidas por una persona sin que nada se ponga rojo", strings.Join(sinFila, ", "))
	}
	var orden []fleet.Condicion
	for c := range filas {
		if _, ok := declaradas[c]; !ok {
			t.Errorf("la tabla tiene una fila para %q e internal/fleet/politica.go no la declara: la fila mide una condición que no existe", c)
		}
		orden = append(orden, c)
	}
	sort.Slice(orden, func(i, j int) bool { return orden[i] < orden[j] })

	// Un curador que puede correr los dos comandos de la tabla en toda la casa: lo que se mide es el
	// ORIGEN, y ninguna fila puede quedarse sin disparo por la allowlist.
	curador := Principal{
		Name: "curador", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet:     map[fleet.Cap][]string{fleet.CapExec: {"*"}, fleet.CapMetrics: {"*"}},
		ExecAllow: map[string][]string{"*": {"journalctl", "systemctl"}},
	}
	for _, cond := range orden {
		f := filas[cond]
		t.Run(string(cond), func(t *testing.T) {
			pol := config.PolicyConfig{
				Name: "origen-" + string(cond), Principal: curador.Name, When: string(cond), Threshold: f.supera,
				Devices: []string{"*"}, Service: f.servicio, Run: f.hacer, CooldownMinutes: 60,
			}
			s, d := prepararPolitica(t, pol, registroDePrueba(curador))
			// `avisa` y la máquina SABE preguntar: el barrido encola también el aviso. Es el eje que la
			// guarda de antes dejaba en `false`, y con él la fila del aviso fuera de la vista.
			fijarEjeYCapacidad(t, s, d.ID, true)
			ahora := time.Now().UTC()
			f.preparar(t, s, d, ahora)
			if previas := comandosEncolados(t, s); len(previas) != 0 {
				t.Fatalf("la bitácora tiene %d fila(s) antes del barrido: no se podría decir cuáles escribió la política", len(previas))
			}
			if n := s.aplicarPoliticas("casa", ahora); n != 1 {
				t.Fatalf("control: la política de %s actuó %d vez/veces y tenía que actuar 1. Sin el disparo, "+
					"«toda fila es automática» pasaría sobre una bitácora vacía", cond, n)
			}

			var comandos, avisos int
			for _, fila := range bitacoraComoLaLeeUnaPersona(t, s) {
				argv := argvDeFila(fila)
				switch {
				case len(argv) > 0 && argv[0] == comandoAviso:
					avisos++
				case reflect.DeepEqual(argv, f.hacer):
					comandos++
				}
				if fila["origen"] != string(fleet.OrigenPolitica) || fila["automatico"] != true {
					t.Errorf("%s: la fila %q la escribió el barrido de una política y la bitácora de las personas "+
						"la muestra con origen=%v automatico=%v. Una acción que disparó una regla se lee como pedida "+
						"por alguien, y cuarenta de éstas seguidas en una cronología cuentan una historia que no pasó",
						cond, argv, fila["origen"], fila["automatico"])
				}
			}
			if comandos != 1 || avisos != 1 {
				t.Errorf("control: el barrido dejó %d comando(s) de la política y %d aviso(s) en la bitácora, y "+
					"tenían que ser 1 y 1 (la máquina está en `avisa` y sabe preguntar). Sin las dos filas, esta "+
					"fila no mira lo que dice mirar", comandos, avisos)
			}
		})
	}
}

// planosConAviso devuelve el plano (`operacion`) de cada avisoDeAcceso escrito en el código del
// paquete, leído del AST, con dónde está escrito. Es el conjunto CERRADO de planos que le prometen
// un aviso al usuario de una máquina: TestNingunPlanoLePrometeUnAvisoAQuienNoSabeMostrarlo tiene
// que recorrerlo entero.
func planosConAviso(t *testing.T) map[string]string {
	t.Helper()
	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("no se pudo leer el paquete: %v", err)
	}
	out := map[string]string{}
	fset := token.NewFileSet()
	for _, e := range entradas {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, n, nil, 0)
		if err != nil {
			t.Fatalf("no se pudo parsear %s: %v", n, err)
		}
		ast.Inspect(f, func(nodo ast.Node) bool {
			cl, ok := nodo.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if id, ok := cl.Type.(*ast.Ident); !ok || id.Name != "avisoDeAcceso" {
				return true
			}
			// El plano es el campo `operacion`: por nombre si el literal va con claves, y si no por
			// posición, la tercera. Si el struct se reordena, lo que haya en esa posición deja de ser
			// un literal de cadena y cae en el rojo de abajo, no en un plano inventado.
			var plano ast.Expr
			for i, el := range cl.Elts {
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					if k, ok := kv.Key.(*ast.Ident); ok && k.Name == "operacion" {
						plano = kv.Value
					}
				} else if i == 2 {
					plano = el
				}
			}
			lit, ok := plano.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				t.Errorf("%s: no pude leer el plano (`operacion`) de este avisoDeAcceso. Mientras no se pueda leer "+
					"del código, nadie exige que la tabla de planos lo recorra", fset.Position(cl.Pos()))
				return true
			}
			v, err := strconv.Unquote(lit.Value)
			if err != nil {
				t.Errorf("%s: %v", fset.Position(lit.Pos()), err)
				return true
			}
			out[v] = fset.Position(cl.Pos()).String()
			return true
		})
	}
	// PISO: hoy son cuatro (pantalla, shell, exec y política). Menos de tres es que el tipo se
	// renombró y esta lectura quedó mirando al aire.
	if len(out) < 3 {
		t.Fatalf("encontré %d avisoDeAcceso en el paquete y son al menos tres: el tipo cambió de nombre y la tabla de planos dejó de medirse", len(out))
	}
	return out
}

// exigirQueElAvisoDigaElOrigenDeLaAccion lee la bitácora como una persona y exige que el aviso
// tenga el MISMO origen que la acción que anuncia: las demás filas que dejó el mismo pedido.
func exigirQueElAvisoDigaElOrigenDeLaAccion(t *testing.T, s *McpServer, plano string) {
	t.Helper()
	var aviso map[string]any
	var accion []map[string]any
	for _, fila := range bitacoraComoLaLeeUnaPersona(t, s) {
		argv := argvDeFila(fila)
		if len(argv) > 0 && argv[0] == comandoAviso {
			if aviso != nil {
				t.Fatalf("plano %q: hay más de un aviso en la bitácora y la comparación no sabría con cuál quedarse", plano)
			}
			aviso = fila
			continue
		}
		accion = append(accion, fila)
	}
	// PISO: sin el aviso, o sin la acción que anuncia, no hay nada que comparar, y un «coinciden»
	// sobre una lista vacía no dice nada.
	if aviso == nil || len(accion) == 0 {
		t.Fatalf("plano %q: la bitácora tiene %d fila(s) de acción y aviso=%v; hacen falta las dos para comparar sus orígenes",
			plano, len(accion), aviso != nil)
	}
	for _, f := range accion {
		if f["origen"] != aviso["origen"] || f["automatico"] != aviso["automatico"] {
			t.Errorf("plano %q: el aviso dice origen=%v automatico=%v y la acción que anuncia (%q) dice origen=%v "+
				"automatico=%v. El aviso es parte del mismo acto: si lo disparó una regla y se lee como pedido por "+
				"una persona —o al revés—, la bitácora y la cronología cuentan dos historias del mismo hecho",
				plano, aviso["origen"], aviso["automatico"], argvDeFila(f), f["origen"], f["automatico"])
		}
	}
}
