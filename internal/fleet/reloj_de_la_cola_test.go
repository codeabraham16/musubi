package fleet

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"musubi/internal/arbol"
	"musubi/internal/fleet/fleettest"
)

// EL RELOJ DE LA COLA SE CUENTA EN UN SOLO LUGAR, Y LOS TRES QUE LO USAN LLEGAN A ÉL.
//
// El doc de LimiteDeVida dice que es la única cuenta del reloj de la cola. Cuando se escribió (A131,
// tema T9) no lo era: el techo de EncolarComando conservaba su copia —`c.Creado.Add(-fleet.
// ComandoVidaMax)`— y lo encontró la revisión, no una prueba. Ninguna podía: la copia daba el MISMO
// texto que LimiteDeVida formateado (el RFC3339 corta la fracción igual que el Truncate), así que las
// tres guardas del techo (cola_test.go) y la de la vista contra la toma quedaban verdes con ella y sin
// ella. Y «hoy da lo mismo» es justo cómo la vista y la toma habían terminado discrepando en el
// segundo del borde: dos cuentas equivalentes, hasta que una cambia sola.
//
// POR ESO ESTO PREGUNTA POR EL FUENTE Y NO POR EL RESULTADO, sobre el repo entero (git ls-files, no
// el disco), y afirma DOS COSAS que juntas son el invariante:
//
//  1. LA CUENTA EXISTE EN UN SOLO LUGAR. ComandoVidaMax —la constante que el reloj resta— se lee SÓLO
//     adentro de LimiteDeVida. Cualquier otra lectura en código de producción es una cuenta del
//     reloj que no pasa por ahí, se escriba como se escriba. Se enumeran los consumidores de la
//     constante y no las formas de la cuenta.
//  2. LOS TRES LECTORES QUE SU DOC NOMBRA LLEGAN A ELLA —la vista (Vencido), la toma
//     (tomarComandosEnTx) y el techo (EncolarComando)—, directo o por un helper de su propio paquete.
//     Se sigue el grafo de llamadas del fuente (fleettest.Grafo).
//
// LA PRIMERA VERSIÓN EXIGÍA LA LLAMADA DIRECTA, Y ESO ERA FIJAR LA FORMA. La segunda revisión de T9
// la midió castigando un refactor correcto: extraer en memory `textoDelLimiteDeVida(t) =
// fleet.LimiteDeVida(t).UTC().Format(time.RFC3339)` y usarlo en los dos lectores daba «✗ LA GUARDA
// CASTIGA EL ARREGLO (falla 7)». El reloj seguía contándose en un solo lugar y los tres seguían
// leyéndolo; lo único que cambiaba era cuántos saltos había hasta la cuenta. Ese refactor es ahora el
// `arreglo` de la primera directiva de abajo, en la toma: el arnés reemplaza un solo tramo, y el helper
// tiene que nacer al lado del lector que lo usa. Con los dos lectores a la vez se midió a mano, en
// verde (el pulido de T9).
//
// EXPOSICIÓN: cero diferencia observable. La copia y LimiteDeVida daban el mismo límite en todo
// instante; lo que esto cuida es el próximo cambio de reloj, que se haría en un lado solo.
//
// LO QUE NO VE, dicho:
//   - Una cuenta que no nombre la constante —un `15 * time.Minute` escrito a mano— en una función que
//     ADEMÁS siga llegando a LimiteDeVida. Si reemplaza la llamada, el lector deja de llegar y el
//     punto 2 lo caza (la segunda directiva de abajo es exactamente eso); si la escribe la vista o la
//     toma, lo caza por comportamiento TestLaVistaYLaTomaVencenUnPendienteEnElMismoSegundo
//     (internal/memory).
//   - Un lector que llegue a LimiteDeVida a través de OTRO paquete (memory llamando a un helper de
//     fleet que la llame). El grafo sigue sólo los helpers del paquete del lector; hoy ninguno lo
//     necesita, y si alguno lo hace, esto lo acusa en rojo: no lo deja pasar callado.
//   - El grafo resuelve sin tipos: `x.M()` sobre un valor cualquiera se enlaza con todo método `M`
//     del paquete (ver fleettest.Grafo). Eso puede inventar un camino, nunca borrar uno.
//
// Sabotaje: devolverle al techo de la cola su copia del límite, que es lo que encontró la revisión →
// la cuenta vuelve a estar en dos lugares y ninguna prueba de comportamiento lo nota.
// arnes: archivo="internal/memory/comandos.go"
// arnes: de="\tvivos := fleet.LimiteDeVida(c.Creado).UTC().Format(time.RFC3339)\n"
// arnes: a="\tvivos := c.Creado.Add(-fleet.ComandoVidaMax).UTC().Format(time.RFC3339)\n"
// arnes: arreglo_de="// tomarComandosEnTx es el CUERPO de la entrega, sin abrir ni cerrar la transacción.\n//\n// Se partió así para que el LATIDO pueda meter el vencimiento y la entrega en la misma\n// transacción en la que estampa la señal de vida (ver latido.go): a 2000 máquinas cada 30 s, dos\n// transacciones separadas por latido son dos fsync donde alcanzaba uno. El paso de la\n// transacción por parámetro es lo que hace que la operación siga siendo indivisible desde las\n// DOS puertas — dos latidos concurrentes de la misma máquina no pueden llevarse el mismo comando,\n// que es la razón por la que esto era una transacción desde el principio.\n//\n// Asume `deviceID` ya recortado y `tope > 0`: las guardas son del llamador.\nfunc tomarComandosEnTx(tx *sql.Tx, deviceID string, ahora time.Time, tope int) ([]fleet.Comando, error) {\n\t// Primero vencer lo viejo. Se hace acá y no en un barrido de fondo porque el momento en que\n\t// importa es JUSTO antes de entregar: es la única ventana donde un comando podría colarse.\n\t// El límite es el que lee la vista (fleet.Comando.Vencido): lo que una superficie muestra\n\t// `expirado` es exactamente lo que acá no se entrega.\n\tlimite := fleet.LimiteDeVida(ahora).UTC().Format(time.RFC3339)\n"
// arnes: arreglo_a="// textoDelLimiteDeVida es fleet.LimiteDeVida en el formato de la columna `creado`.\nfunc textoDelLimiteDeVida(ahora time.Time) string {\n\treturn fleet.LimiteDeVida(ahora).UTC().Format(time.RFC3339)\n}\n\n// tomarComandosEnTx es el CUERPO de la entrega, sin abrir ni cerrar la transacción.\n//\n// Se partió así para que el LATIDO pueda meter el vencimiento y la entrega en la misma\n// transacción en la que estampa la señal de vida (ver latido.go): a 2000 máquinas cada 30 s, dos\n// transacciones separadas por latido son dos fsync donde alcanzaba uno. El paso de la\n// transacción por parámetro es lo que hace que la operación siga siendo indivisible desde las\n// DOS puertas — dos latidos concurrentes de la misma máquina no pueden llevarse el mismo comando,\n// que es la razón por la que esto era una transacción desde el principio.\n//\n// Asume `deviceID` ya recortado y `tope > 0`: las guardas son del llamador.\nfunc tomarComandosEnTx(tx *sql.Tx, deviceID string, ahora time.Time, tope int) ([]fleet.Comando, error) {\n\t// Primero vencer lo viejo. Se hace acá y no en un barrido de fondo porque el momento en que\n\t// importa es JUSTO antes de entregar: es la única ventana donde un comando podría colarse.\n\t// El límite es el que lee la vista (fleet.Comando.Vencido): lo que una superficie muestra\n\t// `expirado` es exactamente lo que acá no se entrega.\n\tlimite := textoDelLimiteDeVida(ahora)\n"
// arnes: colision_ok="TestLoVencidoNoOcupaLugarEnLaCola"
// Sabotaje: que la toma cuente su límite a mano, sin nombrar la constante → deja de llegar a
// LimiteDeVida y da el mismo número, así que tampoco la ve ninguna prueba de comportamiento.
// arnes: archivo="internal/memory/comandos.go"
// arnes: de="\tlimite := fleet.LimiteDeVida(ahora).UTC().Format(time.RFC3339)\n"
// arnes: a="\tlimite := ahora.Add(-15 * time.Minute).Truncate(time.Second).UTC().Format(time.RFC3339)\n"
func TestElRelojDeLaColaSeCuentaEnUnSoloLugar(t *testing.T) {
	const (
		constante = "ComandoVidaMax"
		unica     = "LimiteDeVida"
		suArchivo = "internal/fleet/comando.go"
		suPaquete = "internal/fleet"
		suImport  = "musubi/internal/fleet"
	)
	// Los lectores que nombra el doc de LimiteDeVida, con el paquete donde viven. Se les pide que
	// LLEGUEN a la cuenta única, no que la escriban en su cuerpo: mover una función de archivo o
	// partirla en un helper no cambia de dónde sale su reloj.
	lectores := []struct{ paquete, clave string }{
		{"internal/fleet", "Comando.Vencido"},
		{"internal/memory", "tomarComandosEnTx"},
		{"internal/memory", "DbEngine.EncolarComando"},
	}
	// Las lecturas de la constante que NO cuentan el reloj —un texto que sólo MUESTRA la vida
	// máxima, por ejemplo— van acá, por «archivo función», con su porqué. Hoy no hay ninguna: la
	// excepción se escribe a la vista, no se gana callando la guarda.
	leenSinContar := map[string]string{}

	raiz := filepath.Join("..", "..")

	// ── 1 · LA CUENTA EXISTE EN UN SOLO LUGAR ────────────────────────────────────────────────
	gos, err := arbol.ConSufijo(raiz, ".go")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var ajenas []string // lecturas de la constante fuera de la cuenta única
	enLaUnica := 0
	for _, rel := range gos {
		if strings.HasSuffix(rel, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(raiz, rel), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("no pude parsear %s: %v", rel, err)
		}
		// Los nombres que DECLARAN (el `ComandoVidaMax = …` del bloque const) no son lecturas.
		declaran := map[*ast.Ident]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			if vs, ok := n.(*ast.ValueSpec); ok {
				for _, id := range vs.Names {
					declaran[id] = true
				}
			}
			return true
		})
		for _, d := range f.Decls {
			donde, esLaUnica := "a nivel de paquete", false
			if fn, ok := d.(*ast.FuncDecl); ok {
				donde = fn.Name.Name
				esLaUnica = fn.Recv == nil && fn.Name.Name == unica && rel == suArchivo
			}
			ast.Inspect(d, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok || id.Name != constante || declaran[id] {
					return true
				}
				if esLaUnica {
					enLaUnica++
					return true
				}
				if _, muestra := leenSinContar[rel+" "+donde]; muestra {
					return true
				}
				ajenas = append(ajenas, rel+":"+strconv.Itoa(fset.Position(id.Pos()).Line)+" ("+donde+")")
				return true
			})
		}
	}

	// EL CONTROL POSITIVO: la lectura que tiene que estar. Sin ella, el barrido no está mirando el
	// fuente, y «ninguna copia» se leería igual que un repo sano.
	if enLaUnica == 0 {
		t.Fatalf("el barrido no encontró la lectura de %s adentro de %s (%s): no está mirando el fuente, "+
			"o la cuenta única se movió y esta prueba mide algo que ya no existe", constante, unica, suArchivo)
	}
	for _, sitio := range ajenas {
		t.Errorf("%s se lee en %s, fuera de %s: el reloj de la cola vuelve a contarse en dos lugares, y el "+
			"día que uno cambie la vista, la toma y el techo van a decir cosas distintas del mismo comando — "+
			"que llegue a %s (o, si esa lectura sólo muestra la vida máxima y no cuenta nada, que figure en "+
			"leenSinContar con su porqué)", constante, sitio, unica, unica)
	}

	// ── 2 · LOS TRES LECTORES LLEGAN A LA CUENTA ÚNICA ───────────────────────────────────────
	grafos := map[string]*fleettest.Grafo{}
	grafoDe := func(paquete string) *fleettest.Grafo {
		t.Helper()
		if g, ok := grafos[paquete]; ok {
			return g
		}
		g, err := fleettest.LeerGrafo(raiz, paquete)
		if err != nil {
			t.Fatal(err)
		}
		grafos[paquete] = g
		return g
	}
	// EL PISO del grafo: la cuenta única tiene que estar donde la busca el punto 1. Si no, «ningún
	// lector llega» se leería como tres hallazgos cuando es que se movió.
	if !grafoDe(suPaquete).Existe(unica) {
		t.Fatalf("el grafo de %s no tiene a %s: se movió o cambió de nombre, y esta prueba mide algo que ya "+
			"no existe", suPaquete, unica)
	}
	for _, l := range lectores {
		g := grafoDe(l.paquete)
		objetivo := suImport + "." + unica
		if l.paquete == suPaquete {
			objetivo = unica
		}
		switch {
		case !g.Existe(l.clave):
			t.Errorf("el doc de %s nombra a %s (%s) como lector y no está ahí: o se movió, o el doc cuenta "+
				"lectores que ya no existen", unica, l.clave, l.paquete)
		case g.Camino(l.clave, objetivo) == nil:
			t.Errorf("%s (%s) no llega a %s, ni directo ni por un helper de su paquete: o hace su propia "+
				"cuenta del reloj de la cola, o dejó de usarlo y el doc lo sigue nombrando", l.clave, g.Donde(l.clave), unica)
		}
	}
}
