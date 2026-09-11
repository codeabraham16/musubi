package mcp

// UNA CREDENCIAL VENCIDA NO EJECUTA NADA — TAMPOCO POR EL LADO AUTOMÁTICO.
//
// El vencimiento nació mirándose en UN solo lugar: resolve(), el que autentica un bearer. El otro
// lookup del registro —porNombre(), el que usan las POLÍTICAS de flota y el empuje OTLP para
// actuar en nombre de alguien SIN presentar token— no lo miraba.
//
// Medido en main (f9be15a) con el reloj en 2099 y `expires: 2026-06-01`: resolve negaba y
// porNombre devolvía la credencial VIVA. O sea: al contratista vencido se le cerraba el bearer y
// las políticas le seguían corriendo comandos en su nombre, para siempre.
//
// Es exactamente la forma del defecto dominante de este repo: una guarda presente en N-1 de N
// caminos. La guarda ahora vive en el LOOKUP (porNombre), no en cada llamador, para que el
// próximo llamador que alguien agregue la herede sin acordarse de nada.

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// relojDeVencimiento fija el reloj del registro y lo restaura al terminar.
func relojDeVencimiento(t *testing.T, ahora time.Time) {
	t.Helper()
	anterior := ahoraParaVencimiento
	t.Cleanup(func() { ahoraParaVencimiento = anterior })
	ahoraParaVencimiento = func() time.Time { return ahora }
}

// LA PRUEBA DEL AGUJERO: una credencial vencida no puede actuar por el camino de las políticas.
//
// No mira el lookup —eso no probaría que el exec quedó cerrado—: mira las DOS superficies que
// deciden, y las mira por separado porque se pueden romper una sin la otra:
//
//   - politicaPuedeActuar, el indicador que le contesta a un operador «¿si la condición se
//     cumpliera ahora, pasaría algo?»;
//   - aplicarPoliticas, el camino real, hasta el comando encolado en la bitácora.
//
// El control positivo (la MISMA credencial, el MISMO servidor, con el reloj un rato antes del
// vencimiento) es lo que impide que esto pase con un motor de políticas que no hace nada.
//
// Sabotaje que la hace fallar: sacar el `p.Vencida(...)` de porNombre en principals.go.
func TestUnaCredencialVencidaNoActuaPorUnaPolitica(t *testing.T) {
	vence := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	contratista := autoHeal()
	contratista.Expires = vence

	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(contratista))
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(95, ahora), ahora) // 95 % de RAM: la condición se cumple

	// Se cuentan sólo las filas con Origen=politica, y ésa es la aserción que importa: un disparo
	// deja DOS filas —el aviso al dueño de la máquina, que sale como `persona`, y el comando de
	// verdad—, así que contar la bitácora entera mediría el aviso y no la EJECUCIÓN.
	base := comandosDePolitica(t, s)

	// ── VIGENTE: el control positivo. Todo lo demás de esta prueba no dice nada sin él.
	relojDeVencimiento(t, vence.Add(-time.Hour))
	if !s.politicaPuedeActuar(politicaDeMemoria2(), d) {
		t.Fatal("con la credencial VIGENTE el indicador ya dice que no puede actuar: la prueba no ejercita el vencimiento")
	}
	if n := s.aplicarPoliticas("casa", ahora); n != 1 {
		t.Fatalf("con la credencial VIGENTE la política tendría que actuar; actuó %d veces", n)
	}
	conVigente := comandosDePolitica(t, s)
	if conVigente != base+1 {
		t.Fatalf("con la credencial VIGENTE tendría que encolarse 1 comando de política; se encolaron %d", conVigente-base)
	}

	// ── VENCIDA POR UN SEGUNDO. La credencial es la misma, el registro es el mismo, la máquina
	// es la misma y la concesión sigue puesta: lo ÚNICO que cambió es el reloj.
	relojDeVencimiento(t, vence.Add(time.Second))
	if s.politicaPuedeActuar(politicaDeMemoria2(), d) {
		t.Error("el indicador dice que una credencial VENCIDA puede actuar: le enseña a un operador " +
			"a confiar en un permiso que ya no existe")
	}

	// LA CONDICIÓN TIENE QUE SEGUIR DÁNDOSE. Sin volver a latir, la política no actuaría por la
	// guarda de muestra rancia (I13) y esto pasaría con y sin el arreglo.
	despues := ahora.Add(70 * time.Minute) // 70 > cooldown de 60: tampoco la frena el cooldown
	latir(t, s, d.ID, muestraSana(95, despues), despues)
	d2, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	if v, dispara := politicaDeMemoria2().Dispara(d2.UltimaMuestra); !dispara {
		t.Fatalf("la condición tiene que seguir cumpliéndose (mem=%v) o la prueba no dice nada sobre el vencimiento", v)
	}

	if n := s.aplicarPoliticas("casa", despues); n != 0 {
		t.Errorf("la política EJECUTÓ %d vez/veces en nombre de una credencial VENCIDA: al contratista "+
			"se le cerró el bearer y le siguen corriendo comandos en su nombre", n)
	}
	if n := comandosDePolitica(t, s); n != conVigente {
		t.Errorf("se encolaron %d comando(s) de política en nombre de una credencial VENCIDA: "+
			"la bitácora pasó de %d a %d", n-conVigente, conVigente, n)
	}

	// ── EL BORDE, EXACTAMENTE EN EL INSTANTE. Va para el lado seguro, igual que en resolve().
	relojDeVencimiento(t, vence)
	if s.politicaPuedeActuar(politicaDeMemoria2(), d) {
		t.Error("justo en el instante del vencimiento la política todavía puede actuar: el borde tiene que ir para el lado seguro")
	}
}

// EL HERMANO DEL MISMO LOOKUP: el empuje OTLP.
//
// `principalDelEmpuje` sale del mismo porNombre y exporta la telemetría de la flota CON LA
// AUTORIDAD DE ALGUIEN. Un empujador fantasma no queda inerte como una política: sigue mandando
// datos, que es peor.
//
// Sabotaje que la hace fallar: sacar el `p.Vencida(...)` de porNombre en principals.go.
func TestElEmpujeOTLPNoExportaConUnaCredencialVencida(t *testing.T) {
	vence := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	pr := autoHeal()
	pr.Name = "exportador"
	pr.Expires = vence
	pr.Fleet = map[fleet.Cap][]string{fleet.CapMetrics: {"*"}}

	s := &McpServer{buscarPrincipal: registroDePrueba(pr)}
	s.empujeCfg.Principal = "exportador"

	relojDeVencimiento(t, vence.Add(-time.Hour))
	if _, ok := s.principalDelEmpuje(); !ok {
		t.Fatal("con la credencial VIGENTE el empuje ya no resuelve: la prueba no ejercita el vencimiento")
	}
	relojDeVencimiento(t, vence.Add(time.Second))
	if _, ok := s.principalDelEmpuje(); ok {
		t.Error("el empuje OTLP sigue exportando la telemetría de la flota con una credencial VENCIDA")
	}
}

// LA DECISIÓN QUE NO HAY QUE DESHACER: una fecha que pasó anoche NO puede impedir ARRANCAR.
//
// Es el candado de la otra mitad del arreglo. La forma tentadora de cerrar el agujero es poner la
// guarda en porNombre y dejar que TODOS los llamadores la hereden, incluidos los dos validadores
// de arranque. Con eso, el `expires:` de un contratista convierte una política inerte —que ya se
// avisa y se cuenta en cada tick— en que el cerebro entero no levanta; y el error diría «el
// principal no existe en principals.yaml» de alguien que está ahí escrito.
//
// Por eso los validadores usan porNombreAunqueVencida, con ese nombre y no con el otro.
//
// Sabotaje que la hace fallar: cambiar porNombreAunqueVencida por porNombre en
// validarPrincipalDePolitica / validarPrincipalDeEmpuje (scheduler_flota.go).
func TestUnaCredencialVencidaNoImpideArrancar(t *testing.T) {
	vence := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	relojDeVencimiento(t, vence.Add(24*time.Hour))

	pr := autoHeal()
	pr.Expires = vence
	pr.Fleet = map[fleet.Cap][]string{fleet.CapExec: {"*"}, fleet.CapMetrics: {"*"}}
	reg := registroDePrueba(pr)

	s := &McpServer{}
	pol := fleet.Politica{
		Nombre: "vaciar-journal", Principal: "auto-heal", Cuando: fleet.CondMemPct, Supera: 90,
		Sobre: []string{"*"}, Hacer: []string{"journalctl", "--vacuum-size=200M"},
	}
	if err := s.validarPrincipalDePolitica(pol, reg); err != nil {
		t.Errorf("una credencial vencida impide arrancar: %v\n"+
			"la política queda inerte (y se avisa), pero el cerebro tiene que levantar", err)
	}

	s.empujeCfg.Principal = "auto-heal"
	s.empujeCfg.Endpoint = "https://otlp.ejemplo/v1/metrics"
	if !s.empujeCfg.Activo() {
		t.Fatal("el empuje de prueba no quedó activo: la validación se saltea y la prueba no dice nada")
	}
	if err := s.validarPrincipalDeEmpuje(reg); err != nil {
		t.Errorf("una credencial vencida impide arrancar por el lado del empuje: %v", err)
	}
}

// UN OPERADOR TIENE QUE PODER VER QUE ESA CREDENCIAL ESTÁ MUERTA.
//
// Medido en main: `ListPrincipalsInfo` sobre un contratista con `expires: 2020-01-01` devolvía
// `{Name:contratista ProjectID:casa Role:reader Read:own Write:none}` — ni una palabra del
// vencimiento. El listado es la superficie donde se pregunta QUIÉN tiene acceso, y era la que no
// lo decía: una credencial muerta se veía idéntica a una viva.
//
// La cuarta fila es la que impide que esto se escriba con un bool: un `expires:` ILEGIBLE tiene
// que llamarse ilegible y no «false» — «no pude medir» disfrazado de «medí y está bien» es el
// modo de falla exacto que el vencimiento vino a eliminar.
//
// Sabotaje que la hace fallar: no poblar Expires/Vencimiento en ListPrincipalsInfo.
func TestElListadoDePrincipalsDiceQueLaCredencialVencio(t *testing.T) {
	relojDeVencimiento(t, time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC))

	ruta := filepath.Join(t.TempDir(), "principals.yaml")
	if err := os.WriteFile(ruta, []byte(`principals:
  - name: contratista
    token_sha256: `+hashToken("a")+`
    project_id: casa
    role: reader
    expires: "2020-01-01T00:00:00Z"
  - name: de-siempre
    token_sha256: `+hashToken("b")+`
    project_id: casa
    role: reader
  - name: renovado
    token_sha256: `+hashToken("c")+`
    project_id: casa
    role: reader
    expires: "2030-01-01T00:00:00Z"
  - name: con-typo
    token_sha256: `+hashToken("d")+`
    project_id: casa
    role: reader
    expires: "el jueves que viene"
`), 0o600); err != nil {
		t.Fatal(err)
	}

	infos, err := ListPrincipalsInfo(ruta)
	if err != nil {
		t.Fatalf("ListPrincipalsInfo: %v", err)
	}
	estado := map[string]PrincipalInfo{}
	for _, p := range infos {
		estado[p.Name] = p
	}
	if len(estado) != 4 {
		t.Fatalf("se esperaban 4 principals; llegaron %d", len(estado))
	}

	for _, caso := range []struct{ nombre, quiere, fecha string }{
		{"contratista", VencimientoVencida, "2020-01-01T00:00:00Z"},
		{"de-siempre", VencimientoNoVence, ""},
		{"renovado", VencimientoVigente, "2030-01-01T00:00:00Z"},
		{"con-typo", VencimientoIlegible, "el jueves que viene"},
	} {
		got := estado[caso.nombre]
		if got.Vencimiento != caso.quiere {
			t.Errorf("%s: el listado dice vencimiento=%q y tendría que decir %q",
				caso.nombre, got.Vencimiento, caso.quiere)
		}
		if got.Expires != caso.fecha {
			t.Errorf("%s: el listado dice expires=%q y tendría que decir %q",
				caso.nombre, got.Expires, caso.fecha)
		}
	}
}

// comandosDePolitica cuenta las filas de la bitácora que disparó el motor de políticas —lo que
// EJECUTA—, y no las que abrió una persona ni el aviso que acompaña a cada disparo.
func comandosDePolitica(t *testing.T, s *McpServer) int {
	t.Helper()
	n := 0
	for _, c := range comandosEncolados(t, s) {
		if c.Origen == fleet.OrigenPolitica {
			n++
		}
	}
	return n
}

// EL CAMINO QUE CORRE DE VERDAD: el envoltorio con recarga en caliente.
//
// En producción `s.buscarPrincipal` NO es un *PrincipalRegistry pelado: http.go arma un
// *reloadableRegistry —el que recarga principals.yaml por mtime— y mete ESE en el campo. Todo lo
// de arriba ejercita el registro directo, así que la guarda del vencimiento podía desaparecer del
// único camino que corre sin que nada se pusiera rojo.
//
// MEDIDO antes de escribir esta prueba, sobre b976d9e: con `reloadableRegistry.porNombre`
// delegando en `porNombreAunqueVencida` —un identificador de diferencia, y el agujero entero de
// vuelta por el camino de producción— `go test ./internal/mcp/` daba `ok`. Es la forma del defecto
// dominante de este repo: la guarda puesta en N-1 de N caminos, y el que faltaba era el que corre.
//
// Sabotaje que la hace fallar: en principals_reload.go, que rr.porNombre delegue en
// reg.porNombreAunqueVencida.
func TestElEnvoltorioDeProduccionTampocoDejaActuarAUnaVencida(t *testing.T) {
	vence := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	contratista := autoHeal()
	contratista.Expires = vence

	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(contratista))

	// LA LÍNEA QUE IMPORTA: el campo pasa a llevar lo MISMO que le pone http.go en producción.
	// Si esta prueba se rompe porque cambió cómo se arma el registro allá, el arreglo es hacerla
	// seguir a producción — no volver al registro pelado, que es lo que dejó pasar el agujero.
	s.buscarPrincipal = newReloadableRegistry(
		filepath.Join(t.TempDir(), "principals.yaml"), "",
		registroDePrueba(contratista), time.Now(),
	)

	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(95, ahora), ahora)
	base := comandosDePolitica(t, s)

	// ── VIGENTE: el control positivo. Sin él, todo lo de abajo pasaría con un motor muerto.
	relojDeVencimiento(t, vence.Add(-time.Hour))
	if !s.politicaPuedeActuar(politicaDeMemoria2(), d) {
		t.Fatal("con la credencial VIGENTE el envoltorio ya dice que no puede actuar: la prueba no ejercita el vencimiento")
	}
	if n := s.aplicarPoliticas("casa", ahora); n != 1 {
		t.Fatalf("con la credencial VIGENTE la política tendría que actuar por el envoltorio; actuó %d veces", n)
	}
	conVigente := comandosDePolitica(t, s)
	if conVigente != base+1 {
		t.Fatalf("con la credencial VIGENTE tendría que encolarse 1 comando; se encolaron %d", conVigente-base)
	}

	// ── VENCIDA. Mismo envoltorio, mismo snapshot, misma máquina: lo único que cambia es el reloj.
	relojDeVencimiento(t, vence.Add(time.Second))
	if s.politicaPuedeActuar(politicaDeMemoria2(), d) {
		t.Error("POR EL ENVOLTORIO DE PRODUCCIÓN el indicador dice que una credencial VENCIDA puede actuar")
	}

	// La condición tiene que seguir dándose, o esto pasaría por la guarda de muestra rancia (I13).
	despues := ahora.Add(70 * time.Minute)
	latir(t, s, d.ID, muestraSana(95, despues), despues)
	d2, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	if v, dispara := politicaDeMemoria2().Dispara(d2.UltimaMuestra); !dispara {
		t.Fatalf("la condición tiene que seguir cumpliéndose (mem=%v) o la prueba no dice nada", v)
	}

	if n := s.aplicarPoliticas("casa", despues); n != 0 {
		t.Errorf("POR EL ENVOLTORIO DE PRODUCCIÓN la política EJECUTÓ %d vez/veces con una credencial VENCIDA", n)
	}
	if n := comandosDePolitica(t, s); n != conVigente {
		t.Errorf("POR EL ENVOLTORIO DE PRODUCCIÓN se encolaron %d comando(s) con una credencial VENCIDA", n-conVigente)
	}
}
