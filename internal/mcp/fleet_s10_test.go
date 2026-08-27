package mcp

// Pruebas de S10: la allowlist, el umbral por tier, la poda y las políticas de auto-heal.
// Cada prueba dice cuál es el sabotaje que la hace fallar. Las que no lo dicen no valen nada.

import (
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// ── Ayudas ──────────────────────────────────────────────────────────────────────────────────

// conAllowlist devuelve un principal con exec sobre todo y la allowlist que se le pase.
func conAllowlist(proyecto string, allow map[string][]string) *Principal {
	p := conExec(proyecto)
	p.ExecAllow = allow
	return p
}

// muestraSana es una muestra creíble con la memoria al `memPct` por ciento.
func muestraSana(memPct float64, cuando time.Time) fleet.Muestra {
	total := uint64(16 << 30)
	return fleet.Muestra{
		Tomada: cuando, NumCPU: 8,
		MemTotal: total, MemUsada: uint64(float64(total) * memPct / 100),
		DiscoTotal: 1000 << 30, DiscoUsado: 400 << 30, DiscoDisponible: 550 << 30,
	}
}

// latir estampa una señal de vida con una muestra, en el momento que se le diga.
func latir(t *testing.T, s *McpServer, deviceID string, m fleet.Muestra, cuando time.Time) {
	t.Helper()
	texto, err := m.Serializar()
	if err != nil {
		t.Fatalf("serializar la muestra: %v", err)
	}
	if _, err := s.engine.LatirDevice(deviceID, cuando, texto); err != nil {
		t.Fatalf("latir: %v", err)
	}
}

// ── La allowlist ────────────────────────────────────────────────────────────────────────────

// I8 — LA ALLOWLIST RESTRINGE, NO OTORGA.
//
// Es la confusión que convertiría el cuarto lado en un agujero: si nombrar una máquina en
// `fleet_exec_allow` alcanzara para poder ejecutar en ella, la sección sería una vía paralela de
// concesión que no pasa por `fleet:` — y quien pueda editar su propia allowlist se otorgaría exec.
//
// Sabotaje que la hace fallar: evaluar argvPermitido ANTES de PuedeSobreDevice, o hacer que un
// argvPermitido positivo saltee la compuerta.
func TestLaAllowlistNoLeDaExecAQuienNoLoTenia(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "pc-gio")

	// Sin ninguna concesión `exec` en su sección fleet:, pero con la máquina nombrada en la
	// allowlist y el comando permitido. Todo lo que la allowlist podría "conceder".
	sinConcesion := &Principal{
		Name: "curioso", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
		Fleet:     map[fleet.Cap][]string{fleet.CapMetrics: {"*"}},
		ExecAllow: map[string][]string{"pc-gio": {"uptime"}},
	}
	if _, e := callAsPrincipal(t, s, sinConcesion, "musubi_fleet_exec", map[string]any{
		"device": "pc-gio", "argv": []string{"uptime"}, "no_wait": true,
	}); e == nil {
		t.Fatal("FUGA: figurar en una allowlist alcanzó para ejecutar. La allowlist RECORTA algo ya concedido; no concede nada")
	}
}

// I9 — LA SECCIÓN ES EL OPT-IN, Y UNA VEZ PRESENTE ES EXHAUSTIVA.
//
// El caso que cierra: alguien acota `nas` a dos comandos y después se da de alta `pc-gio`. Sin
// esta regla, la máquina nueva queda con exec IRRESTRICTO para quien creía tener una allowlist —
// o sea, el permiso más amplio se obtiene por olvido, que es exactamente al revés de lo que una
// allowlist significa.
//
// Sabotaje que la hace fallar: devolver true en el paso 4 de argvPermitido (máquina sin entrada
// y sin "*").
func TestUnaMaquinaQueLaAllowlistNoNombraNoPermiteNada(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "nas")
	enrolarConExec(t, s, "casa", "pc-gio") // la máquina nueva, que nadie nombró

	p := conAllowlist("casa", map[string][]string{"nas": {"systemctl", "journalctl"}})

	// En la máquina nombrada, el comando permitido pasa. (Control positivo: sin esto, la
	// aserción negativa de abajo pasaría también con un argvPermitido que niega todo.)
	if _, e := callAsPrincipal(t, s, p, "musubi_fleet_exec", map[string]any{
		"device": "nas", "argv": []string{"systemctl", "status", "x"}, "no_wait": true,
	}); e != nil {
		t.Fatalf("el comando permitido en la máquina nombrada debería pasar: %+v", e)
	}
	// En la máquina NO nombrada, nada pasa — ni siquiera lo que se permitió en la otra.
	for _, argv := range [][]string{{"systemctl", "status", "x"}, {"uptime"}} {
		if _, e := callAsPrincipal(t, s, p, "musubi_fleet_exec", map[string]any{
			"device": "pc-gio", "argv": argv, "no_wait": true,
		}); e == nil {
			t.Errorf("FUGA: %v corrió en una máquina que la allowlist no nombra. Con la sección presente, lo no listado no se permite", argv)
		}
	}
}

// La entrada VACÍA apaga exec sobre una máquina puntual sin sacarla de la concesión — y no puede
// confundirse con «no hay entrada».
//
// Sabotaje que la hace fallar: descartar en el parser las claves con lista vacía (con "*"
// presente, la máquina caería en el comodín y quedaría permitida).
func TestUnaEntradaVaciaApagaLaMaquinaAunqueHayaComodin(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "nas")
	enrolarConExec(t, s, "casa", "produccion")

	p := conAllowlist("casa", map[string][]string{
		"*":          {"uptime"}, // el techo general
		"produccion": {},         // ...y esta máquina, apagada a mano
	})
	if _, e := callAsPrincipal(t, s, p, "musubi_fleet_exec", map[string]any{
		"device": "nas", "argv": []string{"uptime"}, "no_wait": true,
	}); e != nil {
		t.Fatalf("el comodín debería permitir `uptime` en la máquina sin entrada propia: %+v", e)
	}
	if _, e := callAsPrincipal(t, s, p, "musubi_fleet_exec", map[string]any{
		"device": "produccion", "argv": []string{"uptime"}, "no_wait": true,
	}); e == nil {
		t.Fatal("FUGA: una entrada vacía cayó en el comodín. `produccion: []` es una decisión explícita de cero comandos, no una ausencia")
	}
}

// Sin sección, `exec` significa lo mismo que antes de que la allowlist existiera. Es lo que hace
// que estrenar la función no le rompa la configuración a nadie de un día para el otro.
func TestSinAllowlistExecSigueSiendoLoQueEra(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "pc-gio")
	if _, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_exec", map[string]any{
		"device": "pc-gio", "argv": []string{"cualquier", "cosa"}, "no_wait": true,
	}); e != nil {
		t.Fatalf("sin sección `fleet_exec_allow` no debería haber restricción: %+v", e)
	}
}

// El rechazo se CUENTA, y con razón propia: chocar contra la propia allowlist no es lo mismo
// que intentar tocar una máquina ajena, y un panel que los sume enseñaría a ignorar los dos.
func TestElRechazoPorAllowlistSeMideAparteDelDeAuthz(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarConExec(t, s, "casa", "nas")
	antes := s.metrics.execAllowDenied.Load()

	if _, e := callAsPrincipal(t, s, conAllowlist("casa", map[string][]string{"nas": {"uptime"}}),
		"musubi_fleet_exec", map[string]any{"device": "nas", "argv": []string{"rm", "-rf", "/"}, "no_wait": true}); e == nil {
		t.Fatal("`rm` no estaba permitido")
	}
	if got := s.metrics.execAllowDenied.Load(); got != antes+1 {
		t.Errorf("execAllowDenied = %d, esperaba %d: un rechazo que no se cuenta no se ve en /metrics ni dispara una alerta", got, antes+1)
	}
}

// ── El umbral por tier (I2) ─────────────────────────────────────────────────────────────────

// EL BUG QUE ESTE SLICE VINO A CORREGIR, Y QUE UN SONDEO AUTOMÁTICO SOLO NO ARREGLABA.
//
// Un Tier B no late: la única señal de vida posible es que el cerebro lo haya ido a buscar. Con
// el umbral fijo de 90 s y un sondeo cada 5 min, ese dispositivo figura CAÍDO el 97 % del tiempo
// — y `MaquinaCaida` dispara para siempre, que es la manera más eficiente de conseguir que
// alguien silencie la alerta y, con ella, todas las demás.
//
// Sabotaje que la hace fallar: devolver umbralEnLineaDefault para todos los tiers.
func TestUnTierBSondeadoNoFiguraCaidoEntreDosSondeos(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.sondaIntervalo = 5 * time.Minute
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "router", "tier": "B", "caps": []string{"metrics"}, "project": "infra",
		"address": "gio@router.local", "os": "linux"}); e != nil {
		t.Fatal(e)
	}
	d, _, _ := s.engine.DevicePorNombre("infra", "router")

	// Sondeado hace 2 minutos: dentro de su intervalo, todavía no tocaba volver a visitarlo.
	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(30, ahora.Add(-2*time.Minute)), ahora.Add(-2*time.Minute))

	res, e := callAsPrincipal(t, s, conMetrics("infra"), "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	fila := jsonOf(t, res)["devices"].([]any)[0].(map[string]any)
	if fila["online"] != true {
		t.Fatalf("un Tier B sondeado hace 2 min con sondeo cada 5 figura CAÍDO (umbral=%vs): su frescura máxima posible es el intervalo de sondeo, no 90 s", fila["umbral_segundos"])
	}
	// Y el umbral viaja, para que dos filas con el mismo silencio y distinto `online` no parezcan
	// un bug.
	if fila["umbral_segundos"] != float64(900) {
		t.Errorf("umbral_segundos = %v; esperaba 900 (3 × 5 min)", fila["umbral_segundos"])
	}
}

// La otra mitad, que es la que evita que el arreglo se convierta en otro problema: el umbral del
// Tier A NO se afloja. Un agente que late cada 30 s y lleva 2 min callado está caído, sondeo o no.
//
// Sabotaje que la hace fallar: derivar el umbral del intervalo de sondeo también para Tier A.
func TestElUmbralDelTierANoSeAflojaPorElSondeo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.sondaIntervalo = 5 * time.Minute
	enrolarConExec(t, s, "casa", "pc-gio")
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")

	ahora := time.Now()
	latir(t, s, d.ID, muestraSana(30, ahora.Add(-2*time.Minute)), ahora.Add(-2*time.Minute))

	res, e := callAsPrincipal(t, s, conExec("casa"), "musubi_fleet_list", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	fila := jsonOf(t, res)["devices"].([]any)[0].(map[string]any)
	if fila["online"] != false {
		t.Fatalf("un Tier A callado 2 min (4 latidos perdidos) figura en línea: el umbral del agente se aflojó al del sondeo")
	}
	if fila["umbral_segundos"] != float64(90) {
		t.Errorf("umbral_segundos del Tier A = %v; esperaba 90 (3 × 30 s de latido)", fila["umbral_segundos"])
	}
}

// ── La poda (A11 · I6) ──────────────────────────────────────────────────────────────────────

// I6 — LA PODA VACÍA LA SALIDA Y NO BORRA LA FILA.
//
// Son dos retenciones distintas porque son dos riesgos distintos: perder la auditoría es un
// problema de gobierno, guardar salidas para siempre es uno de privacidad.
//
// SE PRUEBA EN DOS MITADES A PROPÓSITO. La poda filtra por `creado`, y un comando creado dentro
// del test tiene la edad del test: para verlo podar de punta a punta habría que envejecer la fila
// por SQL (el engine no lo expone) o inyectarle un reloj al scheduler, que ningún otro scheduler
// del repo tiene. Así que una prueba fija el EFECTO adelantando el reloj de la poda, y la otra
// fija que ALGUIEN LA LLAMA — que es el cabo que S10 vino a atar (A11: la función existía desde
// S5, estaba probada, y no la llamaba nadie).
//
// Sabotaje que la hace fallar: quitar el UPDATE, o hacer que borre la fila.
func TestLaPodaVaciaLaSalidaYConservaLaBitacora(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.retencionSalidasDias = 30
	enrolarConExec(t, s, "casa", "pc-gio")
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")

	cmd, err := s.engine.EncolarComando(fleet.Comando{
		DeviceID: d.ID, ProjectID: "casa", Principal: "op",
		Argv: []string{"cat", "/etc/algo"}, Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.engine.GuardarResultado(d.ID, cmd.ID, ptrInt(0), "CONTRASEÑA=hunter2", "", "", time.Now()); err != nil {
		t.Fatal(err)
	}

	// 60 días después: la salida ya cumplió su retención de 30.
	if n := s.podarSalidasSiToca(time.Now().AddDate(0, 0, 60)); n != 1 {
		t.Fatalf("la poda tocó %d filas, esperaba 1", n)
	}

	filas, err := s.engine.BitacoraDeComandos("casa", "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(filas) != 1 {
		t.Fatalf("la poda BORRÓ la fila: quedan %d. Qué se ejecutó, quién y cuándo es permanente", len(filas))
	}
	if filas[0].Stdout != "" {
		t.Errorf("la salida sigue ahí (%q)", filas[0].Stdout)
	}
	if len(filas[0].Argv) == 0 || filas[0].Principal != "op" || filas[0].ExitCode == nil {
		t.Error("la poda se llevó puesta la auditoría: el argv, el principal y el exit code tienen que sobrevivir")
	}
}

// A11 — EL CABO SUELTO ERA QUE NADIE LA LLAMABA.
//
// Sabotaje que la hace fallar: quitar la llamada a podarSalidasSiToca de barrerFlotaUnaVez.
func TestElBarridoDeFlotaLlamaALaPoda(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.retencionSalidasDias = 30

	if !s.ultimaPoda.IsZero() {
		t.Fatal("ultimaPoda debería nacer en cero")
	}
	s.barrerFlotaUnaVez(t.Context())
	if s.ultimaPoda.IsZero() {
		t.Fatal("el barrido terminó sin podar: hoy las salidas de comandos NO caducarían solas, que es exactamente el cabo A11")
	}
}

// I6 (la otra mitad) — la poda NO corre en cada tick. Es un UPDATE sobre la tabla de comandos y
// el tick es de minutos; una vez por hora alcanza de sobra para una retención en días.
//
// Sabotaje que la hace fallar: quitar la guarda de podaCadaTanto.
func TestLaPodaNoCorreEnCadaTick(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.retencionSalidasDias = 30
	ahora := time.Now()

	s.podarSalidasSiToca(ahora)
	primera := s.ultimaPoda
	s.podarSalidasSiToca(ahora.Add(5 * time.Minute)) // el siguiente tick
	if !s.ultimaPoda.Equal(primera) {
		t.Error("podó dos veces en cinco minutos: la poda es un UPDATE sobre toda la tabla, no una operación de cada tick")
	}
	s.podarSalidasSiToca(ahora.Add(2 * time.Hour))
	if s.ultimaPoda.Equal(primera) {
		t.Error("no volvió a podar en dos horas: la guarda de cadencia se convirtió en un apagado")
	}
}

// Retención desactivada explícitamente (negativa en el YAML ⇒ 0 acá): las salidas no caducan
// nunca, y eso tiene que ser una decisión que se pueda tomar, no un accidente.
func TestConRetencionDesactivadaLaPodaNoTocaNada(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.retencionSalidasDias = 0
	if n := s.podarSalidasSiToca(time.Now().AddDate(0, 0, 365)); n != 0 {
		t.Errorf("podó %d con la retención desactivada", n)
	}
	if !s.ultimaPoda.IsZero() {
		t.Error("con la retención desactivada no debería ni marcar la fecha")
	}
}

func ptrInt(v int) *int { return &v }

// ── El sondeo automático (A19) ──────────────────────────────────────────────────────────────

// A19 — EL CABO ERA QUE NADIE SONDEABA SOLO.
//
// `musubi_fleet_probe` existía desde S7b y se podía colgar de un cron, pero el cerebro no lo
// llamaba: un Tier B figuraba caído aunque estuviera perfecto, porque la única prueba de vida
// posible es que vayamos a buscarlo.
//
// Sabotaje que la hace fallar: quitar sondearProyecto del barrido.
func TestElBarridoSaleASondearALosQueNoTienenAgente(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.sondaIntervalo = 5 * time.Minute
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "nas", "tier": "B", "caps": []string{"metrics"}, "project": "infra",
		"address": "gio@nas.local", "os": "linux"}); e != nil {
		t.Fatal(e)
	}
	restaurar := fleet.SSHFalsoParaTest(t, "cat <<'EOF'\n"+lecturaProcFalsa()+"\nEOF")
	defer restaurar()

	antes, _, _ := s.engine.DevicePorNombre("infra", "nas")
	if !antes.LastSeen.IsZero() {
		t.Fatal("recién dado de alta no debería tener señal de vida")
	}

	s.barrerFlotaUnaVez(t.Context())

	d, _, _ := s.engine.DevicePorNombre("infra", "nas")
	if d.LastSeen.IsZero() {
		t.Fatal("el barrido no sondeó a nadie: un Tier B que nadie sondea figura caído aunque esté bien (A19)")
	}
	if d.UltimaMuestra == nil {
		t.Fatal("sondeó pero no guardó la muestra")
	}
}

// I3 — el sondeo automático no mide lo que no le corresponde: nunca un Tier A (tiene agente y no
// hay por dónde entrarle) y nunca uno al que no se le concedió `metrics`.
//
// Sabotaje que la hace fallar: quitar el filtro de tier, o el de Permite(metrics).
func TestElBarridoNoSondeaNiAlTierANiAQuienNoTieneMetrics(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.sondaIntervalo = 5 * time.Minute
	// El Tier A se da de alta CON DIRECCIÓN a propósito: sin ella, quitar el filtro de tier
	// tampoco lo sondearía (el SSH fallaría por falta de destino) y la prueba pasaría por la
	// razón equivocada. Con dirección, lo único que lo salva de ser sondeado es el filtro.
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "pc-gio", "tier": "A", "caps": []string{"metrics", "exec"}, "project": "infra",
		"address": "gio@pc-gio.local", "os": "linux"}); e != nil {
		t.Fatal(e)
	}
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "switch", "tier": "B", "caps": []string{"exec"}, "project": "infra",
		"address": "gio@switch.local", "os": "linux"}); e != nil { // Tier B SIN metrics
		t.Fatal(e)
	}
	// Un SSH falso que, si se lo llamara, respondería bien. Que igual no haya señal de vida
	// prueba que no se lo llamó — y no que el transporte falló.
	restaurar := fleet.SSHFalsoParaTest(t, "cat <<'EOF'\n"+lecturaProcFalsa()+"\nEOF")
	defer restaurar()

	s.barrerFlotaUnaVez(t.Context())

	for _, nombre := range []string{"pc-gio", "switch"} {
		d, _, _ := s.engine.DevicePorNombre("infra", nombre)
		if !d.LastSeen.IsZero() {
			t.Errorf("%s fue sondeado y no debía: un Tier A reporta solo, y sin `metrics` concedido la máquina no se mide", nombre)
		}
	}
}

// I5 — un barrido que sigue corriendo no arranca otro. Con 40 máquinas por SSH, dos barridos
// solapados son 80 conexiones; y si un tick tarda más que el intervalo, solaparse es la regla.
//
// Sabotaje que la hace fallar: quitar el CompareAndSwap de flotaBusy.
func TestDosBarridosNoSeSolapan(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	s.retencionSalidasDias = 30
	// Se simula un barrido en vuelo tomando la bandera a mano, que es lo que hace el primero.
	if !s.flotaBusy.CompareAndSwap(false, true) {
		t.Fatal("la bandera debería nacer libre")
	}
	s.barrerFlotaUnaVez(t.Context())
	if !s.ultimaPoda.IsZero() {
		t.Fatal("el segundo barrido hizo trabajo con el primero todavía en vuelo")
	}
	s.flotaBusy.Store(false)
	s.barrerFlotaUnaVez(t.Context())
	if s.ultimaPoda.IsZero() {
		t.Fatal("liberada la bandera, el barrido tendría que correr: la guarda se convirtió en un apagado")
	}
}
