package mcp

// A131 · T5 — LO QUE ESTÁ MAL ESCRITO NO ARRANCA (I12), MEDIDO EN EL ARRANQUE DE VERDAD.
//
// Las guardas de antes llamaban a vincularRegistroDeFlota y a ConfigurarFlota con UN caso de cada
// eje. Las de este archivo pasan por el consumidor —ListenAndServeHTTP, el mismo que arranca
// musubi-brain— y recorren cada eje entero.

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// capsDeclaradas devuelve las constantes de tipo Cap, leídas del AST de internal/fleet/device.go:
// el conjunto CERRADO de capacidades de flota que una concesión puede nombrar.
func capsDeclaradas(t *testing.T) []fleet.Cap {
	t.Helper()
	const fuente = "../fleet/device.go"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, fuente, nil, 0)
	if err != nil {
		t.Fatalf("no se pudo parsear %s: %v", fuente, err)
	}
	var out []fleet.Cap
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
			if tipo, ok := vs.Type.(*ast.Ident); !ok || tipo.Name != "Cap" {
				continue
			}
			for _, v := range vs.Values {
				lit, ok := v.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("%s: una constante de Cap no es un literal de cadena y esta guarda no la puede leer", fset.Position(v.Pos()))
				}
				valor, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("%s: %v", fset.Position(v.Pos()), err)
				}
				out = append(out, fleet.Cap(valor))
			}
		}
	}
	// PISO: hoy son cinco (metrics, exec, screen, screen:view, shell). Menos de tres es que el tipo
	// se renombró o se mudó y el eje dejó de recorrerse.
	if len(out) < 3 {
		t.Fatalf("encontré %d constantes de Cap en %s y son al menos tres: el tipo cambió de nombre o de lugar", len(out), fuente)
	}
	return out
}

// registroEnDisco escribe los principals en un principals.yaml de verdad, con el mismo tipo que
// lee loadPrincipals: cada fila pasa por el parser de producción y no por un registro armado en
// memoria, que es justamente lo que el arranque real no usa.
func registroEnDisco(t *testing.T, ps []Principal) string {
	t.Helper()
	var doc principalsFileYAML
	for i, p := range ps {
		e := principalEntry{
			Name: p.Name, TokenSHA256: hashToken(fmt.Sprintf("tok-%d-%s", i, p.Name)),
			ProjectID: p.ProjectID, Role: p.Role, Read: p.Read, Write: p.Write, ExecAllow: p.ExecAllow,
		}
		if len(p.Fleet) > 0 {
			e.Fleet = map[string][]string{}
			for c, sel := range p.Fleet {
				e.Fleet[string(c)] = sel
			}
		}
		doc.Principals = append(doc.Principals, e)
	}
	crudo, err := yaml.Marshal(doc)
	if err != nil {
		t.Fatalf("yaml: %v", err)
	}
	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	if err := os.WriteFile(ruta, crudo, 0o600); err != nil {
		t.Fatal(err)
	}
	return ruta
}

// arranqueReal corre ListenAndServeHTTP —loopback, puerto efímero, en proceso— y devuelve lo que
// devolvió. Una configuración que el arranque rechaza vuelve con el error ANTES de servir; una que
// acepta sirve hasta que vence el contexto y vuelve nil.
func arranqueReal(t *testing.T, s *McpServer, principals string) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	return s.ListenAndServeHTTP(ctx, config.ServiceConfig{
		Addr: "127.0.0.1:0", PrincipalsFile: principals, RequestTimeoutSeconds: 10,
	})
}

// A131 · T5 — EL ARRANQUE REAL NO SIRVE CON UNA POLÍTICA QUE NO PODRÍA ACTUAR.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ EJES CLAVABA LA GUARDA DE ANTES
//
// TestUnaPoliticaSinPrincipalUsableNoDejaArrancar llama a vincularRegistroDeFlota DIRECTO, con UNA
// política y dos casos: el principal no existe, o existe con sólo `metrics`. Cuatro mutaciones
// pasaban en verde por los ejes que clavaba:
//
//   - EL CONSUMIDOR (P2-m9): ListenAndServeHTTP loguea el error de vincularRegistroDeFlota y sigue.
//     La guarda nunca pasaba por ahí, así que el cerebro SERVÍA con la política muerta.
//   - LA AUSENCIA DE REGISTRO (P2-m8): `lookup == nil ⇒ return nil`. Sin principals.yaml el cerebro
//     arrancaba con políticas y, después, actuarSiCorresponde corta en `frenoSinRegistro` sin
//     métrica ni aviso: una alarma muerta en silencio.
//   - LA POSICIÓN (P2-m7): validar sólo la primera política. La guarda tenía una sola.
//   - QUÉ CUENTA COMO CONCESIÓN (P2-m6): tomar `fleet_exec_allow` como si otorgara `exec`. La allowlist
//     RECORTA (I8), y el principal de la guarda no tenía allowlist.
//
// Exposición medida (auditoría A131): 0. Producción tiene una política (`vaciar-journal`, principal
// `auto-heal`) y principals.yaml existe con 16 principals, así que el arranque la valida y pasa;
// `rechazada` y `sin_principal` en 0 en 30 días. Latente: muerde en el primer reinicio después de
// mover principals.yaml o de nombrar mal un principal en la segunda política.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// QUÉ MIDE
//
// Cada fila arranca el servidor ENTERO con un principals.yaml escrito en disco (o sin él) y exige
// una de dos: que no arranque, con un error que nombre a quién hay que arreglar, o —los controles—
// que arranque. Los ejes, recorridos enteros:
//
//   - la CONCESIÓN: una fila por cada capacidad de flota declarada en internal/fleet/device.go (leídas
//     del AST), con un principal admin, `read: all` y una allowlist que permite el comando: arranca
//     sólo la fila de `exec`. Admin y allowlist van en TODAS a propósito: son las dos cosas que se
//     pueden confundir con una concesión, y la única diferencia entre la fila que arranca y las que
//     no es la concesión. Cada fila se contrasta con PuedeSobreDevice, la compuerta que decide en
//     el barrido: arrancar tiene que coincidir con «podría ejecutar en alguna máquina»;
//   - la POSICIÓN: tres políticas con la mala en cada lugar, y el error nombra a la mala;
//   - el REGISTRO AUSENTE: sin principals.yaml y sin token, que es el `resolver` nil;
//   - y el HERMANO del mismo consumidor: el empuje OTLP con un principal que no está.
//
// Sabotaje: que el arranque loguee el error de las políticas y sirva igual (P2-m9).
// arnes: archivo="internal/mcp/http.go"
// arnes: de="\tif err := s.vincularRegistroDeFlota(resolver); err != nil {\n\t\treturn err\n\t}\n"
// arnes: a="\tif err := s.vincularRegistroDeFlota(resolver); err != nil {\n\t\tlogx.Warn(\"flota: política mal configurada; se arranca igual\", \"error\", err)\n\t}\n"
//
// Sabotaje: que sin registro de principals las políticas pasen la validación (P2-m8).
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="\tif lookup == nil {\n\t\treturn fmt.Errorf(\"política %q: hay políticas configuradas pero no hay registro de principals (principals.yaml). Una política actúa con la autoridad de alguien: sin registro no hay a quién nombrar\", pol.Nombre)\n\t}\n"
// arnes: a="\tif lookup == nil {\n\t\treturn nil\n\t}\n"
//
// Sabotaje: validar sólo la primera política (P2-m7).
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="\tfor _, pol := range s.politicas {\n\t\tif err := s.validarPrincipalDePolitica(pol, lookup); err != nil {\n"
// arnes: a="\tfor _, pol := range s.politicas[:min(1, len(s.politicas))] {\n\t\tif err := s.validarPrincipalDePolitica(pol, lookup); err != nil {\n"
//
// Sabotaje: tomar la allowlist como si fuera una concesión de `exec` (P2-m6).
//
// Y la otra dirección: limpiar los selectores antes de contarlos es un cambio correcto —una
// concesión de puros blancos tampoco alcanza a ninguna máquina— y esta guarda no lo castiga.
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="\tif len(pr.Fleet[fleet.CapExec]) == 0 {\n"
// arnes: a="\tif len(pr.Fleet[fleet.CapExec]) == 0 && len(pr.ExecAllow) == 0 {\n"
// arnes: arreglo_de="\tif len(pr.Fleet[fleet.CapExec]) == 0 {\n"
// arnes: arreglo_a="\tif len(fleet.LimpiarSelectores(pr.Fleet[fleet.CapExec])) == 0 {\n"
func TestElArranqueRealNoSirveConUnaPoliticaQueNoPodriaActuar(t *testing.T) {
	type fila struct {
		caso       string
		politicas  []config.PolicyConfig
		principals []Principal // nil = no hay principals.yaml
		empuje     config.OTLPPushConfig
		nombra     string // lo que el error tiene que nombrar; "" = tiene que ARRANCAR
	}
	pol := func(nombre, principal string) config.PolicyConfig {
		p := politicaDeMemoria()
		p.Name, p.Principal = nombre, principal
		return p
	}
	unaSola := []config.PolicyConfig{pol("vaciar-journal", "auto-heal")}
	vigente := []Principal{autoHeal()}

	filas := []fila{
		{caso: "no hay principals.yaml (resolver nil)", politicas: unaSola, principals: nil,
			nombra: `política "vaciar-journal"`},
	}

	// ── LA CONCESIÓN, recorrida entera ──
	// La máquina más permisiva que puede existir: tier A, todas las capacidades, en la casa del
	// principal. Si el principal no podría ejecutar ni ahí, no puede en ninguna.
	caps := capsDeclaradas(t)
	sonda := fleet.Device{Name: "pc-gio", ProjectID: "casa", Tier: fleet.TierAgente, Caps: caps}
	arrancan := 0
	for _, c := range caps {
		pr := Principal{
			Name: "auto-heal", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "casa",
			Fleet:     map[fleet.Cap][]string{c: {"*"}},
			ExecAllow: map[string][]string{"*": {"journalctl"}},
		}
		nombra := `política "vaciar-journal"`
		if c == fleet.CapExec {
			nombra = ""
			arrancan++
		}
		// LA FILA CONTRA LA FUENTE: arrancar tiene que coincidir con lo que la compuerta del barrido
		// dejaría hacer. Una fila que dijera «arranca» donde el principal no podría ejecutar en
		// ninguna máquina estaría escribiendo como correcta una política garantizadamente muerta.
		if puede := PuedeSobreDevice(&pr, sonda, fleet.CapExec); puede != (nombra == "") {
			t.Fatalf("LA FILA ESTÁ MAL ESCRITA: con sólo %q la fila dice arranca=%v y PuedeSobreDevice dice %v", c, nombra == "", puede)
		}
		filas = append(filas, fila{
			caso:      fmt.Sprintf("el principal sólo tiene `%s` (admin, read=all, allowlist que permite el comando)", c),
			politicas: unaSola, principals: []Principal{pr}, nombra: nombra,
		})
	}
	// PISO: sin la fila de `exec` no hay control, y sin las demás no hay eje.
	if arrancan != 1 || len(caps) < 2 {
		t.Fatalf("el eje de la concesión tiene %d fila(s) que arrancan de %d: hace falta exactamente una (la de exec) y al menos una que no", arrancan, len(caps))
	}

	// ── LA POSICIÓN: la política mala en cada lugar de tres ──
	nombres := []string{"uno", "dos", "tres"}
	tres := func(mala int) []config.PolicyConfig {
		var out []config.PolicyConfig
		for i, n := range nombres {
			principal := "auto-heal"
			if i == mala {
				principal = "fantasma"
			}
			out = append(out, pol(n, principal))
		}
		return out
	}
	filas = append(filas, fila{caso: "tres políticas sanas", politicas: tres(-1), principals: vigente, nombra: ""})
	for i, n := range nombres {
		filas = append(filas, fila{
			caso:      fmt.Sprintf("la política %d de 3 nombra a un principal que no está", i+1),
			politicas: tres(i), principals: vigente, nombra: fmt.Sprintf("política %q", n),
		})
	}

	// ── EL HERMANO: el empuje OTLP, validado por el mismo consumidor ──
	// El destino es un servidor de prueba en memoria: con el arranque sano nadie le habla, y con uno
	// roto que sirviera igual, el empuje no tendría a quién mandarle nada que no sea esto.
	receptor := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	t.Cleanup(receptor.Close)
	filas = append(filas, fila{
		caso:       "el empuje OTLP nombra a un principal que no está",
		principals: vigente,
		empuje:     config.OTLPPushConfig{Endpoint: receptor.URL + "/api/v1/otlp/v1/metrics", Principal: "prometheus-fantasma", IntervalSeconds: 30},
		nombra:     `"prometheus-fantasma"`,
	})

	for _, f := range filas {
		t.Run(f.caso, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			if err := s.ConfigurarFlota(config.FleetConfig{Policies: f.politicas, OTLP: f.empuje}); err != nil {
				t.Fatalf("LA FILA ESTÁ MAL ESCRITA: la sintaxis es válida y ConfigurarFlota la rechazó: %v", err)
			}
			ruta := filepath.Join(t.TempDir(), "no-existe.yaml")
			if f.principals != nil {
				ruta = registroEnDisco(t, f.principals)
			}
			err := arranqueReal(t, s, ruta)
			if f.nombra == "" {
				if err != nil {
					t.Fatalf("control: una configuración con la que la política SÍ podría actuar no arrancó: %v. "+
						"Sin este control, cada fila de «no arranca» pasaría contra un arranque que falla por "+
						"cualquier otra cosa", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("EL SERVIDOR SIRVIÓ (%s). La política está garantizadamente muerta —evaluaría, daría "+
					"positivo y no podría hacer nada—, y de las dos formas de enterarse, «no arranca» es mucho más "+
					"barata que «el disco se llenó igual y nadie sabe por qué»", f.caso)
			}
			if !strings.Contains(err.Error(), f.nombra) {
				t.Errorf("el arranque falló, pero el error no nombra %s: %v. O falló por otra cosa —y esta fila no "+
					"mide la validación—, o no le dice al operador qué tiene que arreglar", f.nombra, err)
			}
		})
	}
}

// A131 · T5 — DOS POLÍTICAS HOMÓNIMAS NO ARRANCAN, ESTÉN DONDE ESTÉN.
//
// TestDosPoliticasConElMismoNombreNoArrancan configura [a, a]: las homónimas CONTIGUAS. Comparar
// cada política sólo con la anterior (P2-m11) la dejaba en verde, y [a, x, a] arrancaba: dos
// políticas que comparten cooldown y contador de métricas por nombre, una tapando a la otra sin
// que nada falle.
//
// Exposición medida (auditoría A131): 0. Producción tiene una sola política; el riesgo es la
// primera vez que alguien agregue una tercera con el nombre de una que no es su vecina.
//
// Acá no se suma un caso: se recorre el eje entero. Todas las secuencias de 1 a 4 políticas sobre
// tres nombres (120 configuraciones), y la propiedad es de CARDINALIDAD, no de pares: arranca si y
// sólo si todos los nombres son distintos, y cuando no arranca, el error nombra a una repetida. El
// control positivo está adentro del mismo recorrido: las 15 secuencias sin repetidos tienen que
// arrancar, así que una validación que rechazara todo también se pone roja.
//
// Sabotaje: comparar cada política sólo con la anterior (P2-m11).
//
// Y la otra dirección: preguntar por la presencia de la clave en vez de por su valor es la misma
// cardinalidad escrita de otra forma, y esta guarda no la castiga.
// arnes: archivo="internal/mcp/scheduler_flota.go"
// arnes: de="\t\tif vistos[pol.Nombre] {\n"
// arnes: a="\t\tif len(politicas) > 0 && politicas[len(politicas)-1].Nombre == pol.Nombre {\n"
// arnes: arreglo_de="\t\tif vistos[pol.Nombre] {\n"
// arnes: arreglo_a="\t\tif _, repetida := vistos[pol.Nombre]; repetida {\n"
func TestDosPoliticasHomonimasNoArrancanEnNingunaPosicion(t *testing.T) {
	nombres := []string{"a", "b", "c"}
	var secuencias [][]string
	var armar func(prefijo []string)
	armar = func(prefijo []string) {
		if len(prefijo) > 0 {
			secuencias = append(secuencias, append([]string(nil), prefijo...))
		}
		if len(prefijo) == 4 {
			return
		}
		for _, n := range nombres {
			armar(append(prefijo, n))
		}
	}
	armar(nil)
	// PISO: 3 + 9 + 27 + 81. Menos es que el recorrido se cortó y el eje quedó a medias.
	if len(secuencias) != 120 {
		t.Fatalf("el recorrido armó %d secuencias y son 120: la propiedad no se mide entera", len(secuencias))
	}

	s := newTestServer(t, embedding.NoopProvider{})
	var arrancaron, rechazadas int
	for _, seq := range secuencias {
		veces := map[string]int{}
		pols := make([]config.PolicyConfig, len(seq))
		for i, n := range seq {
			p := politicaDeMemoria()
			p.Name = n
			// Cada política es DISTINTA en lo que hace: lo único que se repite es el nombre.
			p.Run = []string{"journalctl", fmt.Sprintf("--vacuum-size=%dM", 100*(i+1))}
			pols[i] = p
			veces[n]++
		}
		var repetidas []string
		for n, k := range veces {
			if k > 1 {
				repetidas = append(repetidas, n)
			}
		}
		sort.Strings(repetidas)
		err := s.ConfigurarFlota(config.FleetConfig{Policies: pols})
		switch {
		case len(repetidas) == 0 && err != nil:
			t.Errorf("%v: los nombres son todos distintos y el arranque la rechazó: %v", seq, err)
		case len(repetidas) == 0:
			arrancaron++
		case err == nil:
			t.Errorf("%v: %v está más de una vez y arrancó. El cooldown y las métricas se llevan POR NOMBRE, así "+
				"que las homónimas se pisan entre sí: una tapa a la otra sin que nada falle", seq, repetidas)
		default:
			rechazadas++
			nombrada := false
			for _, n := range repetidas {
				if strings.Contains(err.Error(), strconv.Quote(n)) {
					nombrada = true
				}
			}
			if !nombrada {
				t.Errorf("%v: el arranque se negó, pero el error no nombra a ninguna repetida (%v): %v", seq, repetidas, err)
			}
		}
	}
	// El recorrido tiene los dos lados: sin repetidos hay 3 + 3·2 + 3·2·1 = 15 (con cuatro políticas y
	// tres nombres, alguno se repite siempre), y las otras 105 repiten alguno.
	if arrancaron != 15 || rechazadas != 105 {
		t.Errorf("arrancaron %d y se rechazaron %d de 120; tenían que ser 15 y 105", arrancaron, rechazadas)
	}
}
