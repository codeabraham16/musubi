package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// ═════════════════════════════════════════════════════════════════════════════════════════════
// A91 · EL AUTO-HEAL ES EL CUARTO CAMINO DEL EJE, Y ERA EL ÚNICO QUE NO LO CONSULTABA
//
// `ConsentimientoEfectivo()` tenía OCHO llamadores y ninguno en `politicas.go` ni en
// `scheduler_flota.go`, y del lado de la máquina no hay red: el agente corre lo que hay en la
// cola. Medido el 2026-09-05 con el barrido real: `libre`, `avisa`, `pide` y `prohibido` daban los
// cuatro «1 comando encolado», y en la fila de `avisa` el único comando era el `journalctl` — no
// había ningún `musubi:avisar`.
//
// El eje se describe en todas las superficies como «el candado del dueño de la máquina», que «no
// se abre NUNCA, aunque la capacidad esté perfectamente concedida». Cerraba los tres caminos con
// una persona detrás y ninguno cuando la orden la dispara un temporizador — que es justo el que
// nadie mira ejecutarse.
//
// Decisión de gio (2026-09-05): EL EJE GOBIERNA AL AUTO-HEAL.
//
// LA TABLA ES POR GRADOS PORQUE EL DEFECTO DE A83/A85 FUE GENERALIZAR SOBRE EL OTRO EJE. Una
// guarda que recorría los tres caminos con `avisa` fijo daba tranquilidad y nunca probaba `pide`,
// donde estaba el agujero: `AvisaAlUsuario()` es true para `pide` también. Acá el camino es uno y
// los grados son los cuatro.
func TestElAutoHealPasaPorElEjeDeConsentimiento(t *testing.T) {
	casos := []struct {
		grado  fleet.Consentimiento
		actua  bool
		avisa  bool
		porque string
	}{
		{fleet.ConsentimientoLibre, true, false,
			"`libre` es «ni se entera», y está elegido: avisar acá sería ruido que enseña a cerrar sin leer"},
		{fleet.ConsentimientoAvisa, true, true,
			"`avisa` promete notificación sin veto. Sin el aviso, `avisa` es indistinguible de `libre` " +
				"y quien está sentado adelante no se entera de lo que la automatización le corre"},
		{fleet.ConsentimientoPide, false, false,
			"mismo criterio que A86 en `exec`: `pide` promete que su usuario ACEPTE, y un barrido por " +
				"temporizador no tiene dónde esperar esa respuesta. Y NO avisa: un aviso que no se " +
				"puede contestar es el defecto de A85"},
		{fleet.ConsentimientoProhibido, false, false,
			"el candado del dueño. Si un temporizador lo abre, el eje entero es decoración"},
	}

	for _, c := range casos {
		t.Run(string(c.grado), func(t *testing.T) {
			s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
			// `puede_preguntar = true` es lo que hace que esto mida el EJE y no su degradación: con
			// false, `ConsentimientoEfectivo` endurece `pide` a `prohibido` y la fila de `pide`
			// probaría otra cosa — pasaría en verde por el motivo equivocado.
			if err := s.engine.FijarCapacidadDePreguntar(d.ID, true); err != nil {
				t.Fatalf("FijarCapacidadDePreguntar: %v", err)
			}
			if _, err := s.engine.FijarConsentimiento(d.ID, c.grado); err != nil {
				t.Fatalf("FijarConsentimiento: %v", err)
			}
			d, _, _ = s.engine.DevicePorNombre("casa", "pc-gio")
			if got := d.ConsentimientoEfectivo(); got != c.grado {
				t.Fatalf("el grado efectivo quedó en %q y se pidió %q: esta fila no está midiendo lo que dice", got, c.grado)
			}

			ahora := time.Now()
			latir(t, s, d.ID, muestraSana(95, ahora), ahora) // 95 % de RAM: la condición se cumple

			acciones := s.aplicarPoliticas("casa", ahora)

			// LO QUE SE MIDE ES LA COLA DE LA MÁQUINA, no el contador de acciones: el contador podría
			// quedar en cero mientras el comando ya está encolado, y al revés. Lo que le pasa a la
			// máquina es lo único que importa.
			var deLaPolitica, avisos int
			for _, cmd := range comandosEncolados(t, s) {
				switch {
				case len(cmd.Argv) > 0 && cmd.Argv[0] == comandoAviso:
					avisos++
				case cmd.Origen == fleet.OrigenPolitica:
					deLaPolitica++
				}
			}
			if (deLaPolitica > 0) != c.actua {
				t.Errorf("la política encoló %d comando(s) y se esperaba actuar=%v (acciones=%d)\n  %s",
					deLaPolitica, c.actua, acciones, c.porque)
			}
			if (avisos > 0) != c.avisa {
				t.Errorf("se encolaron %d aviso(s) y se esperaba avisar=%v\n  %s", avisos, c.avisa, c.porque)
			}
		})
	}
}

// EL CONTROL POSITIVO, SEPARADO Y EXPLÍCITO.
//
// Sin esto, todas las filas de «no actúa» de arriba pasarían igual contra un motor de políticas
// roto que no hace nada — y la prueba entera diría que el eje funciona cuando lo que pasa es que
// el auto-heal no anda. Es el mismo par que TestUnaPoliticaNoPuedeMasQueSuPrincipal ya usa.
func TestSinGradoDeclaradoElAutoHealSigueActuando(t *testing.T) {
	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(95, ahora), ahora)
	if n := s.aplicarPoliticas("casa", ahora); n != 1 {
		t.Fatalf("sin grado declarado la política tiene que actuar y actuó %d veces: la tercera compuerta "+
			"está frenando de más, y con eso las filas de «no actúa» de la matriz pasarían por el motivo equivocado", n)
	}
}

// ═════════════════════════════════════════════════════════════════════════════════════════════
// LA GUARDA QUE IMPIDE EL QUINTO CAMINO, Y ES LA QUE DE VERDAD CIERRA A91
//
// Esto ya pasó DOS veces con la misma forma. A83: había dos copias del bloque de aviso, se agregó
// la shell como tercer camino, nadie se acordó de copiarlo, y `avisa` quedó escrito y sin efecto en
// el único camino que se saltea cualquier allowlist. A91: se agregó `politicas.go` como cuarto
// camino y no consultó el eje, mientras su propio comentario decía «las mismas compuertas que para
// una persona».
//
// Las dos veces el camino nuevo llegó EN UN ARCHIVO NUEVO. Así que la guarda mira exactamente eso:
// todo archivo del paquete que le hace HACER algo a una máquina —o sea que llama a
// `EncolarComando`— tiene que conocer el eje. Un archivo nuevo que encole sin nombrarlo falla acá,
// en el repo, en vez de descubrirse cuando una máquina en `prohibido` ejecuta algo.
//
// POR QUÉ POR ARCHIVO Y NO POR FUNCIÓN: la consulta al eje pasa en el LLAMADOR, no en la función
// que encola —`encolarAvisoDeAcceso` no consulta nada, y está bien—, así que exigirlo por función
// daría falsos positivos y se desactivaría por ruidoso. El archivo es la granularidad donde la
// falla real ocurrió las dos veces.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// SE MIRA EL AST Y NO EL TEXTO, PORQUE LA PRIMERA VERSIÓN NO SERVÍA
//
// Buscaba `ConsentimientoEfectivo\(` con un regex sobre el archivo. Saqué el switch entero de
// `actuarSiCorresponde` —el estado exacto anterior a A91— y la prueba quedó EN VERDE: el archivo
// seguía nombrando el eje en un COMENTARIO, y un comentario satisface un regex igual que una
// llamada. La prueba afirmaba en su propio doc que ese sabotaje la ponía en rojo. No lo hacía.
//
// Es la tercera vez en el día que un chequeo mío mira TEXTO donde tenía que mirar ESTRUCTURA, y
// las tres veces el síntoma fue el mismo: verde sobre el defecto que decía cubrir. Queda escrito
// porque el patrón de mis propios errores es más útil que el arreglo.
//
// Ahora se parsea con go/ast y se cuentan LLAMADAS. Un comentario no produce un CallExpr.
//
// Sabotaje verificado que la pone en rojo: sacar el switch de `ConsentimientoEfectivo` de
// `actuarSiCorresponde` dejando sus comentarios intactos.
func TestTodoArchivoQueLeHaceHacerAlgoAUnaMaquinaConoceElEjeDeConsentimiento(t *testing.T) {
	// Las excepciones se enumeran CON SU MOTIVO. Un allowlist sin razones se llena solo.
	exentos := map[string]string{}

	// llama dice si el archivo INVOCA ese método en algún lado. Se busca por el nombre del
	// selector, que es lo que sobrevive a un renombre del receptor (`s.engine` → otra cosa).
	llama := func(t *testing.T, ruta, metodo string) bool {
		t.Helper()
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, ruta, nil, 0)
		if err != nil {
			t.Fatalf("no se pudo parsear %s: %v", ruta, err)
		}
		visto := false
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == metodo {
				visto = true
			}
			return !visto
		})
		return visto
	}

	entradas, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("no se pudo leer el paquete: %v", err)
	}
	var encolan, sinEje []string
	for _, e := range entradas {
		n := e.Name()
		if e.IsDir() || !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		if !llama(t, n, "EncolarComando") {
			continue
		}
		encolan = append(encolan, n)
		if !llama(t, n, "ConsentimientoEfectivo") {
			if _, ok := exentos[n]; !ok {
				sinEje = append(sinEje, n)
			}
		}
	}
	sort.Strings(encolan)

	// CONTROL DE QUE MIRÓ ALGO: si `EncolarComando` cambiara de nombre, la lista quedaría vacía y
	// todo lo de abajo pasaría en verde sin haber revisado un solo archivo. Hoy son cuatro
	// —methods_exec.go, methods_pantalla.go, methods_shell.go y politicas.go—, que son exactamente
	// los cuatro caminos del eje.
	if len(encolan) < 4 {
		t.Fatalf("se encontraron %d archivos que encolan comandos (%v) y son al menos cuatro: "+
			"cambió el nombre del encolador y esta guarda dejó de mirar", len(encolan), encolan)
	}

	for _, n := range sinEje {
		t.Errorf("%s le hace HACER algo a una máquina (llama a EncolarComando) y NUNCA llama a "+
			"ConsentimientoEfectivo.\n"+
			"  Si es un camino nuevo del plano de actuar, tiene que pasar por el eje: `prohibido` es el\n"+
			"  candado del dueño de la máquina y se describe en cinco superficies como «no se abre NUNCA».\n"+
			"  Esto ya pasó dos veces —A83 con la shell, A91 con el auto-heal— y las dos veces el camino\n"+
			"  nuevo llegó en un archivo nuevo, igual que éste.\n"+
			"  Si de verdad NO corresponde, sumalo a `exentos` CON SU MOTIVO escrito.", n)
	}

	// Y al revés: una exención sobre un archivo que ya no encola nada queda diciendo que cuida algo.
	for n := range exentos {
		presente := false
		for _, e := range encolan {
			if e == n {
				presente = true
				break
			}
		}
		if !presente {
			t.Errorf("`exentos` exime a %s y ese archivo ya no encola comandos: sacá la entrada", n)
		}
	}
}
