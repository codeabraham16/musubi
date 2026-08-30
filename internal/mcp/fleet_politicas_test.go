package mcp

// Pruebas del auto-heal (S10 · A10). Es lo más peligroso del track: ejecución remota SIN una
// persona detrás. Todo lo de acá abajo existe para fijar UNA frase — una política no tiene
// autoridad propia — y las guardas que la sostienen.

import (
	"strings"
	"testing"
	"time"

	"sync/atomic"

	"musubi/internal/config"
	"musubi/internal/embedding"
	"musubi/internal/fleet"
	"musubi/internal/memory"
)

// registroDePrueba arma un registro de principals en memoria (sin archivo).
func registroDePrueba(ps ...Principal) *PrincipalRegistry {
	return &PrincipalRegistry{principals: ps}
}

// autoHeal es el principal típico de una política: puede ejecutar en toda la casa, y sólo
// journalctl.
func autoHeal() Principal {
	return Principal{
		Name: "auto-heal", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet:     map[fleet.Cap][]string{fleet.CapExec: {"*"}, fleet.CapMetrics: {"*"}},
		ExecAllow: map[string][]string{"*": {"journalctl"}},
	}
}

// politicaDeMemoria es la política de referencia: si la RAM pasa del 90 %, correr journalctl.
func politicaDeMemoria() config.PolicyConfig {
	return config.PolicyConfig{
		Name: "vaciar-journal", Principal: "auto-heal", When: "mem_pct", Threshold: 90,
		Devices: []string{"*"}, Run: []string{"journalctl", "--vacuum-size=200M"},
		CooldownMinutes: 60,
	}
}

// prepararPolitica arma un server con una máquina, una política y un registro, y devuelve el
// device ya dado de alta.
func prepararPolitica(t *testing.T, pol config.PolicyConfig, reg *PrincipalRegistry) (*McpServer, fleet.Device) {
	t.Helper()
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "pc-gio")
	if err := s.ConfigurarFlota(config.FleetConfig{Policies: []config.PolicyConfig{pol}}); err != nil {
		t.Fatalf("ConfigurarFlota: %v", err)
	}
	s.buscarPrincipal = reg
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	return s, d
}

// comandosEncolados cuenta lo que hay en la bitácora del proyecto.
func comandosEncolados(t *testing.T, s *McpServer) []fleet.Comando {
	t.Helper()
	filas, err := s.engine.BitacoraDeComandos("casa", "", 50)
	if err != nil {
		t.Fatal(err)
	}
	return filas
}

// ── El invariante central ───────────────────────────────────────────────────────────────────

// I11 — UNA POLÍTICA NO TIENE AUTORIDAD PROPIA.
//
// Es lo único que hace defendible el auto-heal entero. La alternativa —un daemon que ejecuta
// «porque es el daemon»— habría sido más corta de escribir y sería el puente de privilegio que el
// track viene esquivando desde el proposal: bastaría con poder editar el archivo de configuración
// del cerebro para tener root en 40 máquinas, sin figurar en ninguna concesión y sin dejar un
// nombre en la auditoría.
//
// Acá el principal existe, la condición se cumple, la muestra está fresca — y NO tiene `exec`
// sobre esa máquina. No pasa nada.
//
// Sabotaje que la hace fallar: quitar el PuedeSobreDevice de evaluarPolitica.
func TestUnaPoliticaNoPuedeMasQueSuPrincipal(t *testing.T) {
	// Mismo principal, pero con la concesión de exec acotada a OTRA máquina.
	acotado := autoHeal()
	acotado.Fleet = map[fleet.Cap][]string{fleet.CapExec: {"otra-maquina"}, fleet.CapMetrics: {"*"}}

	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(acotado))
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(95, ahora), ahora) // 95 % de RAM: la condición se cumple

	if n := s.aplicarPoliticas("casa", ahora); n != 0 {
		t.Fatalf("la política actuó %d vez/veces sobre una máquina donde su principal NO tiene exec", n)
	}
	if filas := comandosEncolados(t, s); len(filas) != 0 {
		t.Fatalf("se encoló %d comando(s): la política se saltó la compuerta", len(filas))
	}
	// Control positivo: con la concesión puesta, la MISMA política sí actúa. Sin esto, la
	// aserción de arriba pasaría también con un motor de políticas que no hace nada.
	s2, d2 := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
	latir(t, s2, d2.ID, muestraSana(95, ahora), ahora)
	if n := s2.aplicarPoliticas("casa", ahora); n != 1 {
		t.Fatalf("con la concesión puesta la política tendría que actuar; actuó %d veces", n)
	}
}

// La misma frase, por el otro lado: la política tampoco puede más que la ALLOWLIST de su
// principal. Es lo que permite tener un `auto-heal` que puede ejecutar en todas las máquinas y
// aun así sólo puede correr journalctl.
//
// Sabotaje que la hace fallar: quitar el argvPermitido de evaluarPolitica.
func TestUnaPoliticaRespetaLaAllowlistDeSuPrincipal(t *testing.T) {
	pol := politicaDeMemoria()
	pol.Run = []string{"rm", "-rf", "/var/log"} // no está en la allowlist de auto-heal

	s, d := prepararPolitica(t, pol, registroDePrueba(autoHeal()))
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(95, ahora), ahora)

	if n := s.aplicarPoliticas("casa", ahora); n != 0 {
		t.Fatal("la política corrió un comando que la allowlist de su principal NO permite")
	}
	if filas := comandosEncolados(t, s); len(filas) != 0 {
		t.Fatal("se encoló igual: la allowlist no se está aplicando del lado automático")
	}
	if s.metrics == nil {
		t.Skip("sin métricas")
	}
}

// Y la consecuencia que hace que todo esto valga la pena en la práctica: REVOCAR AL PRINCIPAL
// APAGA LA POLÍTICA. No hay que acordarse de apagarla en un segundo lugar — y nadie se acuerda
// de un segundo lugar.
//
// Sabotaje que la hace fallar: resolver el principal UNA vez al arranque y guardarlo.
func TestRevocarAlPrincipalApagaLaPolitica(t *testing.T) {
	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(95, ahora), ahora)

	if n := s.aplicarPoliticas("casa", ahora); n != 1 {
		t.Fatalf("la política tendría que actuar antes de revocar; actuó %d veces", n)
	}
	// Se revoca: el principal desaparece del registro, como cuando se borra su línea del YAML.
	s.buscarPrincipal = registroDePrueba()

	// LA CONDICIÓN TIENE QUE SEGUIR DÁNDOSE, Y ÉSE ES EL PUNTO DELICADO DE ESTA PRUEBA.
	//
	// La primera versión avanzaba 24 h y dejaba la muestra vieja: la política no actuaba, sí,
	// pero por la guarda de rancidez (I13), no por la revocación. Pasaba con y sin el arreglo. Hay
	// que avanzar lo justo para salir del cooldown Y volver a latir con la RAM alta, para que lo
	// ÚNICO que pueda frenarla sea que su principal ya no existe.
	despues := ahora.Add(70 * time.Minute)
	latir(t, s, d.ID, muestraSana(95, despues), despues)
	d2, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	if v, dispara := politicaDeMemoria2().Dispara(d2.UltimaMuestra); !dispara {
		t.Fatalf("la condición tiene que seguir cumpliéndose (mem=%v) o la prueba no dice nada sobre la revocación", v)
	}
	if n := s.aplicarPoliticas("casa", despues); n != 0 {
		t.Fatal("la política siguió actuando con su principal revocado: se está resolviendo una sola vez, al arranque, en vez de en cada evaluación")
	}
}

// politicaDeMemoria2 es politicaDeMemoria en su forma de dominio, para poder evaluar la condición
// desde una prueba sin pasar por el motor.
func politicaDeMemoria2() fleet.Politica {
	c := politicaDeMemoria()
	return fleet.Politica{Nombre: c.Name, Principal: c.Principal, Cuando: fleet.Condicion(c.When),
		Supera: c.Threshold, Sobre: c.Devices, Hacer: c.Run}
}

// ── Las guardas de sensatez ─────────────────────────────────────────────────────────────────

// I13 — NO SE ACTÚA SOBRE UNA MUESTRA RANCIA.
//
// Es la guarda que más veces va a evitar un desastre y la más fácil de olvidar. Si la máquina
// dejó de reportar, el último dato puede ser de hace horas: el proceso que comía la RAM pudo
// haber muerto. Actuar con eso no es reaccionar tarde — es reaccionar a algo que ya no pasa.
//
// Y hay una segunda razón, más fea: la última muestra buena, siendo la última, SIEMPRE cruza el
// umbral. Una política sin esta guarda actuaría para siempre sobre una máquina muerta.
//
// Sabotaje que la hace fallar: quitar el chequeo de EnLinea / de la edad de la muestra.
func TestUnaPoliticaNoActuaSobreUnaMuestraRancia(t *testing.T) {
	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
	ahora := time.Now()
	// La máquina reportó 95 % de RAM... hace media hora, y desde entonces silencio.
	hace30 := ahora.Add(-30 * time.Minute)
	latir(t, s, d.ID, muestraSana(95, hace30), hace30)

	if n := s.aplicarPoliticas("casa", ahora); n != 0 {
		t.Fatal("actuó sobre una muestra de hace 30 min de una máquina que dejó de latir: la última muestra buena siempre cruza el umbral, así que sin esta guarda la política dispara para siempre")
	}
}

// El fallo silencioso que un `up` no detecta: EL AGENTE VIVE Y EL COLECTOR MURIÓ. La máquina
// late, así que figura en línea, pero su última muestra envejece sin parar.
//
// Sabotaje que la hace fallar: chequear sólo EnLinea y no la edad de la muestra.
func TestUnaMaquinaQueLateSinMedirNoDisparaPoliticas(t *testing.T) {
	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
	ahora := time.Now()

	// Primero, una muestra vieja con la RAM alta.
	hace30 := ahora.Add(-30 * time.Minute)
	latir(t, s, d.ID, muestraSana(95, hace30), hace30)
	// Ahora late SIN muestra: el UPDATE conserva la anterior (así es como se ve un colector
	// muerto desde el cerebro). La máquina queda "en línea" con un dato de hace media hora.
	if _, err := s.engine.LatirDevice(d.ID, ahora, ""); err != nil {
		t.Fatal(err)
	}
	d2, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	if !d2.EnLinea(ahora, s.umbralEnLinea(d2)) {
		t.Fatal("la máquina tendría que figurar EN LÍNEA, o la prueba no representa el caso")
	}
	if d2.UltimaMuestra == nil {
		t.Fatal("el latido sin muestra borró la anterior; la prueba no representa el caso")
	}

	if n := s.aplicarPoliticas("casa", ahora); n != 0 {
		t.Fatal("actuó con una máquina que late pero dejó de MEDIR: se está mirando el latido y no la edad del dato")
	}
}

// I14 — COOLDOWN POR (POLÍTICA × MÁQUINA).
//
// Sin él la política dispara en CADA tick hasta que la métrica baje, y la métrica no baja hasta
// que el comando termine. No es «más reactivo»: es una tormenta de comandos idénticos, y empieza
// justo cuando algo ya va mal.
//
// Sabotaje que la hace fallar: quitar la consulta a ultimoDisparo.
func TestElCooldownEvitaLaTormentaDeComandosIdenticos(t *testing.T) {
	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal())) // cooldown 60 min
	ahora := time.Now()

	// Doce ticks de cinco minutos con la condición cumplida todo el tiempo.
	actuadas := 0
	for i := 0; i < 12; i++ {
		t := ahora.Add(time.Duration(i) * 5 * time.Minute)
		latir2(s, d.ID, muestraSana(95, t), t)
		actuadas += s.aplicarPoliticas("casa", t)
	}
	if actuadas != 1 {
		t.Fatalf("la política actuó %d veces en una hora con cooldown de 60 min: esperaba 1", actuadas)
	}
	// Y pasado el cooldown, vuelve a poder actuar: un cooldown que no se suelta es un apagado.
	pasada := ahora.Add(70 * time.Minute)
	latir2(s, d.ID, muestraSana(95, pasada), pasada)
	if n := s.aplicarPoliticas("casa", pasada); n != 1 {
		t.Fatalf("pasado el cooldown actuó %d veces: esperaba 1", n)
	}
}

// latir2 es latir sin *testing.T, para usarlo dentro de un bucle que ya sombreó `t`.
func latir2(s *McpServer, deviceID string, m fleet.Muestra, cuando time.Time) {
	texto, _ := m.Serializar()
	_, _ = s.engine.LatirDevice(deviceID, cuando, texto)
}

// I16 — LA ACCIÓN AUTOMÁTICA VA A LA MISMA BITÁCORA QUE LAS PERSONAS.
//
// Un segundo registro de auditoría «para lo automático» es cómo se llega a auditar sólo la mitad
// de lo que pasa. Y la mitad automática es justo la que nadie miró ejecutarse.
//
// Sabotaje que la hace fallar: encolar el comando sin pasar por EncolarComando, o estampar el
// principal como "sistema" en vez del de la política.
func TestLaAccionDeUnaPoliticaQuedaEnLaMismaBitacoraQueLasPersonas(t *testing.T) {
	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(95, ahora), ahora)
	s.aplicarPoliticas("casa", ahora)

	// Se lee con la MISMA tool que usa una persona, con una credencial de persona.
	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_log", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	crudo := textOf(t, res)
	for _, quiero := range []string{`"principal":"auto-heal"`, "journalctl", `"device":"pc-gio"`} {
		if !strings.Contains(crudo, quiero) {
			t.Errorf("la bitácora de las personas no muestra %q de la acción automática:\n%s", quiero, crudo)
		}
	}
	// MISMA BITÁCORA, PERO DISTINGUIBLE (A59). Estar en la misma tabla con las mismas columnas
	// es lo correcto —un registro aparte «para lo automático» es cómo se termina auditando sólo
	// la mitad de lo que pasa—; no poder DISTINGUIRLAS al leer es otra cosa. Sin esto, cuarenta
	// reinicios de auto-heal en una cronología se leen como cuarenta pedidos de una persona.
	//
	// Sabotaje: quitarle `Origen: fleet.OrigenPolitica` a correrAccionDePolitica → falla acá, y
	// es el ÚNICO lugar donde ese cableado se verifica: sembrar el comando a mano con el origen
	// puesto probaría que el campo viaja, no que alguien lo setea.
	for _, quiero := range []string{`"origen":"politica"`, `"automatico":true`} {
		if !strings.Contains(crudo, quiero) {
			t.Errorf("la acción automática no se distingue de una manual: falta %q en\n%s", quiero, crudo)
		}
	}
}

// El alcance de la política es un selector de máquina, igual que el de las concesiones. Una
// política sobre `nas` no puede tocar `pc-gio` aunque su principal sí pueda.
func TestUnaPoliticaSoloTocaLasMaquinasQueNombra(t *testing.T) {
	pol := politicaDeMemoria()
	pol.Devices = []string{"nas"} // que no existe en este proyecto

	s, d := prepararPolitica(t, pol, registroDePrueba(autoHeal()))
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(95, ahora), ahora)

	if n := s.aplicarPoliticas("casa", ahora); n != 0 {
		t.Fatal("una política acotada a `nas` actuó sobre `pc-gio`")
	}
}

// ── La validación de arranque (I12) ─────────────────────────────────────────────────────────

// I12 — LO QUE ESTÁ MAL ESCRITO NO ARRANCA.
//
// De las dos formas de enterarse de que una política está mal, «el servidor no arranca» es mucho
// más barata que «el disco se llenó igual y nadie sabe por qué». Una alarma muerta que nadie sabe
// que está muerta es peor que no tener alarma.
//
// Sabotaje que la hace fallar: hacer que vincularRegistroDeFlota logee en vez de devolver error.
func TestUnaPoliticaSinPrincipalUsableNoDejaArrancar(t *testing.T) {
	casos := []struct {
		nombre string
		reg    *PrincipalRegistry
		porque string
	}{
		{
			"el principal no existe",
			registroDePrueba(),
			"nombra a alguien que no está en principals.yaml",
		},
		{
			"el principal existe pero no puede ejecutar en ninguna máquina",
			registroDePrueba(Principal{
				Name: "auto-heal", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
				Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}}, // metrics, no exec
			}),
			"está garantizadamente muerta: evaluaría, daría positivo y no podría hacer nada",
		},
	}
	for _, c := range casos {
		s := newTestServer(t, embedding.NoopProvider{})
		if err := s.ConfigurarFlota(config.FleetConfig{Policies: []config.PolicyConfig{politicaDeMemoria()}}); err != nil {
			t.Fatalf("%s: la sintaxis es válida, no debería fallar acá: %v", c.nombre, err)
		}
		if err := s.vincularRegistroDeFlota(c.reg); err == nil {
			t.Errorf("%s: el servidor arrancó igual. %s", c.nombre, c.porque)
		}
	}
}

// Dos políticas con el mismo nombre comparten cooldown y contador de métricas: una taparía a la
// otra sin que nada fallara. Es el error que sólo se descubre cuando ya importa.
func TestDosPoliticasConElMismoNombreNoArrancan(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	a, b := politicaDeMemoria(), politicaDeMemoria()
	b.Run = []string{"journalctl", "--rotate"}
	if err := s.ConfigurarFlota(config.FleetConfig{Policies: []config.PolicyConfig{a, b}}); err == nil {
		t.Fatal("dos políticas homónimas arrancaron: el cooldown y las métricas se llevan por nombre, así que se pisarían")
	}
}

// I15 — LAS POLÍTICAS NACEN APAGADAS. Sin sección, el motor no existe.
func TestSinPoliticasElBarridoNoActuaSobreNadie(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "pc-gio")
	if err := s.ConfigurarFlota(config.FleetConfig{}); err != nil {
		t.Fatal(err)
	}
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(99, ahora), ahora)

	if n := s.aplicarPoliticas("casa", ahora); n != 0 {
		t.Fatal("actuó sin políticas configuradas")
	}
	if len(comandosEncolados(t, s)) != 0 {
		t.Fatal("encoló algo sin políticas configuradas")
	}
}

// Un fallo de CONFIGURACIÓN de una política no es un evento: es un estado que dura hasta que
// alguien edita un archivo. Repetir el aviso en cada tick son 288 líneas idénticas por día — el
// ruido que entierra la línea que sí importa.
//
// Lo destapó el e2e: 17 avisos idénticos en un minuto. La MÉTRICA sí cuenta cada evaluación,
// porque de ella vive la alerta PoliticaSinPermiso: lo que se acota es el ruido, no la señal.
//
// Sabotaje que la hace fallar: quitar avisarUnaVez y logear directo (el contador de avisos
// crecería con cada evaluación).
func TestUnFalloDeConfiguracionDeUnaPoliticaSeAvisaUnaVezYSeCuentaSiempre(t *testing.T) {
	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba()) // registro VACÍO
	ahora := time.Now()

	avisos := 0
	for i := 0; i < 10; i++ {
		cuando := ahora.Add(time.Duration(i) * time.Minute)
		latir2(s, d.ID, muestraSana(95, cuando), cuando)
		s.avisosDados.Range(func(k, _ any) bool { return true })
		antes := contarAvisos(s)
		s.aplicarPoliticas("casa", cuando)
		if contarAvisos(s) > antes {
			avisos++
		}
	}
	if avisos != 1 {
		t.Errorf("se emitió el aviso %d veces en 10 evaluaciones; esperaba 1", avisos)
	}
	// La señal NO se acota: la alerta PoliticaSinPermiso vive del contador, no del log.
	if n := contarPoliticaOK(s, "vaciar-journal", "sin_principal"); n != 10 {
		t.Errorf("la métrica contó %d evaluaciones fallidas de 10: se acotó la señal y no el ruido", n)
	}
}

func contarAvisos(s *McpServer) int {
	n := 0
	s.avisosDados.Range(func(_, _ any) bool { n++; return true })
	return n
}

func contarPoliticaOK(s *McpServer, politica, resultado string) int64 {
	v, ok := s.metrics.politicaStats.Load(politica + "\x00" + resultado)
	if !ok {
		return 0
	}
	return v.(*atomic.Int64).Load()
}

// ── A24 · el cooldown sobrevive un reinicio ────────────────────────────────────────────────

// servidorSobre arma un servidor sobre un directorio DADO, para poder simular un reinicio:
// mismo disco, proceso nuevo. `newTestServer` usa un t.TempDir() propio y por eso no sirve acá.
func servidorSobre(t *testing.T, dir string, pol config.PolicyConfig, reg *PrincipalRegistry) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	t.Cleanup(func() { engine.Close() })
	s := NewMcpServer(engine, dir, embedding.NoopProvider{})
	if err := s.ConfigurarFlota(config.FleetConfig{Policies: []config.PolicyConfig{pol}}); err != nil {
		t.Fatalf("ConfigurarFlota: %v", err)
	}
	s.buscarPrincipal = reg
	return s
}

// A24 — REINICIAR EL CEREBRO NO PUEDE REARMAR LOS COOLDOWNS.
//
// El cooldown es lo único que separa «una política que corrige algo» de una tormenta de comandos
// idénticos. Con el estado sólo en memoria, duraba lo que durara el proceso — y el reinicio NO es
// un evento raro que ocurra en momentos tranquilos: es lo primero que alguien hace cuando algo va
// mal, que es exactamente cuando las políticas están disparando.
//
// El caso concreto: la política vacía un journal, el operador reinicia el cerebro treinta
// segundos después para tocar otra cosa, y la política vuelve a vaciarlo porque la muestra vieja
// todavía cruza el umbral. Dos acciones donde tenía que haber una.
//
// Sabotaje que la hace fallar: quitar la llamada a cargarCooldowns, o la persistencia del disparo.
func TestElCooldownSobreviveUnReinicioDelCerebro(t *testing.T) {
	dir := t.TempDir()
	reg := registroDePrueba(autoHeal())

	// --- primera vida del cerebro ---
	s1 := servidorSobre(t, dir, politicaDeMemoria(), reg) // cooldown de 60 min
	enrolarConExec(t, s1, "casa", "pc-gio")
	d, _, _ := s1.engine.DevicePorNombre("casa", "pc-gio")
	ahora := time.Now()
	latir(t, s1, d.ID, muestraSana(95, ahora), ahora)
	if n := s1.aplicarPoliticas("casa", ahora); n != 1 {
		t.Fatalf("la política tendría que actuar en la primera vida; actuó %d veces", n)
	}
	s1.engine.Close()

	// --- se reinicia treinta segundos después, con la condición todavía cumplida ---
	despues := ahora.Add(30 * time.Second)
	s2 := servidorSobre(t, dir, politicaDeMemoria(), reg)
	latir(t, s2, d.ID, muestraSana(95, despues), despues)
	if n := s2.aplicarPoliticas("casa", despues); n != 0 {
		t.Fatal("volvió a actuar tras el reinicio: el cooldown vivía sólo en memoria y el reinicio lo rearmó")
	}

	// Y pasado el cooldown de verdad, el cerebro reiniciado SÍ puede volver a actuar: un cooldown
	// que no se suelta es un apagado, y persistirlo no puede convertirlo en uno.
	pasada := ahora.Add(70 * time.Minute)
	latir(t, s2, d.ID, muestraSana(95, pasada), pasada)
	if n := s2.aplicarPoliticas("casa", pasada); n != 1 {
		t.Fatalf("pasado el cooldown, el cerebro reiniciado actuó %d veces: esperaba 1", n)
	}
}

// La limpieza tiene un default deliberado que conviene fijar: UNA LISTA VACÍA NO BORRA NADA.
//
// «No hay políticas configuradas» es también lo que se ve cuando alguien está editando el YAML y
// lo dejó a medias, o cuando el cerebro arrancó sin su sección. Borrar todo el historial de
// cooldowns por eso sería irreversible; conservarlo cuesta unas filas.
//
// Sabotaje que la hace fallar: quitar la guarda de len(vivas)==0 (el DELETE con un IN vacío borra
// la tabla entera).
func TestPodarElEstadoDePoliticasConListaVaciaNoBorraNada(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if err := s.engine.MarcarDisparoDePolitica("vaciar-journal", "dev-1", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if n, err := s.engine.PodarEstadoDePoliticas(nil); err != nil || n != 0 {
		t.Fatalf("con la lista vacía borró %d filas (err=%v): un YAML a medio editar no puede llevarse el historial", n, err)
	}
	cds, err := s.engine.CooldownsDePoliticas()
	if err != nil {
		t.Fatal(err)
	}
	// LA CLAVE INTERIOR ES COMPUESTA desde la v39: `device_id\x00alcance`. El alcance vacío es
	// una política de host, que es lo que esta fila representa. Se compone acá igual que en el
	// dominio para que no haya dos formas de armar la misma clave.
	if _, hay := cds["vaciar-journal"]["dev-1\x00"]; !hay {
		t.Fatalf("la fila desapareció: %+v", cds["vaciar-journal"])
	}
	// Con al menos una política viva, sí limpia lo que sobra.
	if n, err := s.engine.PodarEstadoDePoliticas([]string{"otra-politica"}); err != nil || n != 1 {
		t.Fatalf("con una política viva tendría que borrar la fila huérfana: borró %d (err=%v)", n, err)
	}
}

// ── A23 · el auto-heal se ve ────────────────────────────────────────────────────────────────

// A23 — UNA MÁQUINA CON AUTO-HEAL ENCIMA NO PUEDE VERSE IGUAL QUE UNA SIN ÉL.
//
// S10 dejó al cerebro ejecutando comandos en máquinas ajenas sin una persona detrás, y eso no
// aparecía en ningún lado salvo hurgando la bitácora DESPUÉS del hecho.
//
// Sabotaje que la hace fallar: no agregar `politicas`/`politicas_activas` al inventario.
func TestElInventarioDiceQueActuaSoloSobreCadaMaquina(t *testing.T) {
	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
	_ = d
	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	fila := jsonOf(t, res)["devices"].([]any)[0].(map[string]any)

	if fila["politicas_activas"] != float64(1) {
		t.Fatalf("politicas_activas = %v, esperaba 1", fila["politicas_activas"])
	}
	det, _ := fila["politicas"].([]any)
	if len(det) != 1 {
		t.Fatalf("no viajó el detalle de la política: %v", fila["politicas"])
	}
	p := det[0].(map[string]any)
	if p["nombre"] != "vaciar-journal" || p["principal"] != "auto-heal" {
		t.Errorf("la política no se identifica: %v", p)
	}
	if p["condicion"] != "mem_pct > 90.0" {
		t.Errorf("condicion = %v: quien mira tiene que poder saber CUÁNDO va a actuar", p["condicion"])
	}
	// Nunca actuó ⇒ null. «Todavía no» y «actuó hace mucho» son cosas distintas.
	if p["ultimo_disparo"] != nil {
		t.Errorf("ultimo_disparo = %v; una política que nunca actuó no puede traer una fecha", p["ultimo_disparo"])
	}
	if p["puede_actuar"] != true {
		t.Error("puede_actuar = false con todo bien configurado")
	}
}

// EL CAMPO QUE MÁS IMPORTA: una política INERTE se ve idéntica a una que funciona.
//
// Su principal perdió la concesión, o el comando se cayó de la allowlist: las dos figuran en la
// lista y ninguna hace nada visible hasta que la condición se cumple. Es una alarma apagada, y la
// única forma de que alguien lo note ANTES del incidente es decirlo en el inventario.
//
// Sabotaje que la hace fallar: devolver `puede_actuar: true` fijo, o calcularlo con una cadena de
// guardas distinta de la que usa evaluarPolitica.
func TestUnaPoliticaInerteSeDistingueDeUnaQueFunciona(t *testing.T) {
	// El principal existe y tiene exec, pero el comando de la política NO está en su allowlist.
	tullido := autoHeal()
	tullido.ExecAllow = map[string][]string{"*": {"uptime"}} // no journalctl

	s, d := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(tullido))

	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	fila := jsonOf(t, res)["devices"].([]any)[0].(map[string]any)
	p := fila["politicas"].([]any)[0].(map[string]any)
	if p["puede_actuar"] != false {
		t.Fatal("una política cuyo comando no pasa la allowlist de su principal figura como que PUEDE actuar: es una alarma apagada que nadie va a notar hasta el incidente")
	}

	// Y el indicador tiene que coincidir con la REALIDAD: si dice que no puede, no puede.
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(95, ahora), ahora)
	if n := s.aplicarPoliticas("casa", ahora); n != 0 {
		t.Error("el inventario dice `puede_actuar: false` y la política actuó igual: el indicador miente")
	}
}

// El DETALLE exige `exec` (misma regla que la bitácora: saber qué corre en un servidor es casi
// tan revelador como poder correrlo). El CONTEO no: que exista algo automático encima no es un
// secreto, y ocultarlo dejaría a quien sólo mira métricas viendo cambiar una máquina sin ninguna
// pista de por qué.
//
// Sabotaje que la hace fallar: mandar el detalle a todo el mundo, o esconder también el conteo.
func TestSinExecSeVeQueHayAlgoAutomaticoPeroNoQueHace(t *testing.T) {
	s, _ := prepararPolitica(t, politicaDeMemoria(), registroDePrueba(autoHeal()))
	soloMetrics := &Principal{
		Name: "mirón", Role: RoleReader, Read: ReadOwn, Write: WriteNone, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}},
	}
	res, e := callAsPrincipal(t, s, soloMetrics, "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	fila := jsonOf(t, res)["devices"].([]any)[0].(map[string]any)
	if fila["politicas_activas"] != float64(1) {
		t.Errorf("politicas_activas = %v: sin el conteo, alguien ve cambiar su máquina sin ninguna pista", fila["politicas_activas"])
	}
	if _, hay := fila["politicas"]; hay {
		t.Error("FUGA: viajó el detalle de la política a una credencial sin `exec`. Qué comando corre en un servidor se gatea igual que la bitácora")
	}
}

// Sin políticas configuradas, el campo NO aparece: un `politicas_activas: 0` en cada fila es
// ruido que entrena a ignorar la columna.
func TestSinPoliticasElInventarioNoMencionaNada(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "pc-gio")
	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	fila := jsonOf(t, res)["devices"].([]any)[0].(map[string]any)
	if _, hay := fila["politicas_activas"]; hay {
		t.Error("aparece politicas_activas sin políticas configuradas")
	}
}

// registroQuePermiteSystemctl es el registro de las pruebas de A44. Se arma uno propio en vez de
// reusar el de referencia para no torcer el COMANDO de la prueba: el de referencia sólo permite
// `journalctl`, y cambiar `systemctl restart nginx` por otra cosa haría que la prueba ejercite un
// camino distinto del que dice ejercitar.
func registroQuePermiteSystemctl() *PrincipalRegistry {
	return registroDePrueba(Principal{
		Name: "curador", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet:     map[fleet.Cap][]string{fleet.CapExec: {"*"}, fleet.CapMetrics: {"*"}},
		ExecAllow: map[string][]string{"*": {"systemctl"}},
	})
}

// UNA POLÍTICA DE SERVICIO ACTÚA SOBRE EL INVENTARIO, NO SOBRE LA MUESTRA.
//
// Es la bifurcación de A44, y su ausencia más importante es deliberada: NO se exige que la máquina
// esté en línea ni que su telemetría esté fresca. Esas guardas son de la MUESTRA, y un servicio no
// se mide con la muestra — se mide con el inventario, que tiene su propia frescura.
//
// Copiarlas volvería la política inútil justo donde más sirve: una máquina cuyo colector murió
// sigue mandando su inventario de servicios, y ahí es donde uno quiere que algo actúe.
//
// Sabotaje que la hace fallar: mandar las políticas de servicio por el camino de la muestra.
func TestUnaPoliticaDeServicioActuaSinTelemetriaDelHost(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, s, "casa", "pc-gio")
	ds, _ := s.engine.ListarDevices("casa", false)
	d := ds[0]
	ahora := time.Now().UTC()

	// La máquina NO tiene muestra: nunca reportó telemetría, o su colector murió.
	if d.UltimaMuestra != nil {
		t.Fatal("la máquina de prueba tiene muestra: no ejercita el caso")
	}
	// Pero SÍ reportó su inventario, y nginx está caído.
	if _, _, err := s.engine.ReportarServicios(d.ID, ahora, []fleet.ReporteServicio{
		{Nombre: "nginx", Clase: "systemd", Salud: fleet.SaludServicio{
			Tomada: ahora, Estado: fleet.EstadoFallado}}}); err != nil {
		t.Fatal(err)
	}

	pol := fleet.Politica{
		Nombre: "revivir-nginx", Principal: "curador", Cuando: fleet.CondServicioCaido,
		Sobre: []string{"*"}, Servicio: "nginx",
		Hacer: []string{"systemctl", "restart", "nginx"}, Cooldown: 10 * time.Minute,
	}
	if err := pol.Validar(); err != nil {
		t.Fatalf("la política de prueba no es válida: %v", err)
	}

	// SE MONTA EL PRINCIPAL DE VERDAD para que la decisión llegue hasta el final. Sin registro,
	// `actuarSiCorresponde` corta antes de cualquier punto observable, y la prueba fallaría por el
	// andamiaje y no por lo que custodia — que fue exactamente lo que pasó con sus dos primeras
	// versiones.
	s.buscarPrincipal = registroQuePermiteSystemctl()
	s.politicas = []fleet.Politica{pol}
	s.evaluarPolitica(pol, d, ahora)

	// El cooldown se marca ANTES de ejecutar, así que su presencia prueba que la decisión
	// atravesó las guardas de la muestra —que no le corresponden— y llegó al tramo de acción.
	if _, hay := s.ultimoDisparo.Load(pol.ClaveDeCooldown(d.ID)); !hay {
		t.Error("la política de servicio no llegó a decidir: la mataron las guardas de la MUESTRA, " +
			"que no le corresponden — un servicio no se mide con la telemetría del host, y esta " +
			"máquina no tiene ninguna")
	}

	// PERO EL INVENTARIO VIEJO SÍ LA FRENA, y ésa es la guarda que le corresponde. No se hereda
	// del host: se calcula con el umbral del INVENTARIO, que es otro. Sin esto, una máquina que
	// dejó de enumerar sus servicios —lo que el agente hace a propósito cuando una fuente falla—
	// tendría políticas actuando eternamente sobre el último estado conocido.
	otro := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, otro, "casa", "pc-gio")
	ds2, _ := otro.engine.ListarDevices("casa", false)
	d2 := ds2[0]
	if _, _, err := otro.engine.ReportarServicios(d2.ID, ahora, []fleet.ReporteServicio{
		{Nombre: "nginx", Clase: "systemd", Salud: fleet.SaludServicio{
			Tomada: ahora, Estado: fleet.EstadoFallado}}}); err != nil {
		t.Fatal(err)
	}
	otro.buscarPrincipal = registroQuePermiteSystemctl()
	otro.politicas = []fleet.Politica{pol}
	// Muy después: el inventario quedó viejo aunque el estado guardado siga diciendo «fallado».
	tarde := ahora.Add(3 * fleet.UmbralInventario)
	if otro.evaluarPolitica(pol, d2, tarde) {
		t.Error("actuó sobre un inventario viejo")
	}
	if _, hay := otro.ultimoDisparo.Load(pol.ClaveDeCooldown(d2.ID)); hay {
		t.Error("llegó a decidir con el inventario viejo: la frescura del INVENTARIO no se está " +
			"mirando, y el agente deja de mandarlo a propósito cuando una fuente falla")
	}
}

// UN SERVICIO QUE NO ESTÁ EN EL INVENTARIO NO SE TRATA COMO CAÍDO.
//
// Tienta al revés: no está, algo pasó, reinicialo. Pero la ausencia significa que la máquina NO LO
// ENUMERA — no que se cayó. Una política que reinicia lo que no existe es la que se lleva puesto
// un host donde alguien escribió mal el nombre del servicio, y lo hace en silencio y para siempre.
//
// Sabotaje que la hace fallar: devolver true cuando el servicio no aparece en el inventario.
func TestUnServicioAusenteDelInventarioNoDisparaLaPolitica(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, s, "casa", "pc-gio")
	ds, _ := s.engine.ListarDevices("casa", false)
	d := ds[0]
	ahora := time.Now().UTC()

	// La máquina reporta OTRO servicio: el inventario existe y el nombre buscado no está.
	if _, _, err := s.engine.ReportarServicios(d.ID, ahora, []fleet.ReporteServicio{
		{Nombre: "sshd", Clase: "systemd", Salud: fleet.SaludServicio{
			Tomada: ahora, Estado: fleet.EstadoCorriendo}}}); err != nil {
		t.Fatal(err)
	}

	pol := fleet.Politica{
		Nombre: "revivir-el-que-no-existe", Principal: "curador", Cuando: fleet.CondServicioCaido,
		Sobre: []string{"*"}, Servicio: "nombre-mal-escrito",
		Hacer: []string{"systemctl", "restart", "nginx"}, Cooldown: 10 * time.Minute,
	}
	s.buscarPrincipal = registroQuePermiteSystemctl()
	s.politicas = []fleet.Politica{pol}
	if s.evaluarPolitica(pol, d, ahora) {
		t.Error("actuó sobre un servicio que la máquina no enumera: la ausencia se tomó como caída")
	}
	if _, llego := s.ultimoDisparo.Load(pol.ClaveDeCooldown(d.ID)); llego {
		t.Error("llegó al tramo de acción sobre un servicio que la máquina no enumera")
	}
}
