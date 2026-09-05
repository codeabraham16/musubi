package memory

import (
	"errors"
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// altaDePrueba da de alta un Tier A con todas las capacidades y devuelve la fila + su credencial.
func altaDePrueba(t *testing.T, e *DbEngine, projectID, name string) (fleet.Device, string) {
	t.Helper()
	token, err := fleet.NuevoToken()
	if err != nil {
		t.Fatalf("NuevoToken: %v", err)
	}
	d, err := e.AltaDevice(fleet.Device{
		Name:      name,
		ProjectID: projectID,
		Tier:      fleet.TierAgente,
		Caps:      []fleet.Cap{fleet.CapMetrics, fleet.CapExec, fleet.CapScreen},
		OS:        "linux",
		Arch:      "amd64",
	}, token)
	if err != nil {
		t.Fatalf("AltaDevice(%q): %v", name, err)
	}
	return d, token
}

// A1 — el id lo asigna el CEREBRO. Un id declarado por el cliente se ignora.
// Sabotaje: quitar `d.ID = uuid.NewString()` de AltaDevice → el cliente elige su identidad y
// puede afirmar ser otra máquina.
func TestAltaIgnoraElIDQueDeclaraElCliente(t *testing.T) {
	e := newTestEngine(t)
	token, _ := fleet.NuevoToken()
	d, err := e.AltaDevice(fleet.Device{
		ID:        "soy-el-servidor-de-produccion",
		Name:      "pc-cualquiera",
		ProjectID: "casa",
		Tier:      fleet.TierAgente,
		Caps:      []fleet.Cap{fleet.CapMetrics},
	}, token)
	if err != nil {
		t.Fatalf("AltaDevice: %v", err)
	}
	if d.ID == "soy-el-servidor-de-produccion" {
		t.Fatal("el registro aceptó el id que declaró el cliente: un device puede afirmar ser otro")
	}
	if d.ID == "" {
		t.Fatal("el registro no asignó ningún id")
	}
}

// A1 — la identidad se DERIVA del token, y el id que sale es el que asignó el cerebro.
func TestIdentidadSeDerivaDelToken(t *testing.T) {
	e := newTestEngine(t)
	alta, token := altaDePrueba(t, e, "casa", "pc-gio")

	got, ok, err := e.DevicePorToken(token)
	if err != nil || !ok {
		t.Fatalf("DevicePorToken: ok=%v err=%v", ok, err)
	}
	if got.ID != alta.ID || got.Name != "pc-gio" || got.ProjectID != "casa" {
		t.Fatalf("resolvió a otro device: %+v", got)
	}
	if got.Tier != fleet.TierAgente || len(got.Caps) != 3 {
		t.Fatalf("no sobrevivieron tier/caps a la ida y vuelta: %+v", got)
	}
	// Una credencial que no existe no resuelve a nada.
	otro, _ := fleet.NuevoToken()
	if _, ok, _ := e.DevicePorToken(otro); ok {
		t.Fatal("un token inexistente resolvió a un device")
	}
}

// A1 (bypass) — un token VACÍO no autentica NI SIQUIERA contra una fila que hasheó el vacío.
//
// El primer intento de esta prueba NO sabía fallar, y vale dejar por qué. Daba de alta un Tier B
// sin credencial (token_sha256 = cadena vacía) y pedía autenticar con "". Pasaba con la guarda y
// SIN la guarda, porque HashToken("") es e3b0c442... y nunca iguala a la cadena vacía: el SELECT
// no encontraba nada de todos modos. Medido, no supuesto.
//
// La guarda protege el caso en que AltaDevice hashee SIEMPRE —el "simplificar" que está a 80
// líneas y que nada en el compilador impide—. Entonces todo device sin credencial comparte el
// hash del vacío y una petición sin credencial autentica como uno de ellos. Esta versión INSERTA
// esa fila a mano para simular exactamente ese error.
//
// Sabotaje que la hace fallar: quitar la guarda de token vacío de DevicePorToken.
func TestTokenVacioNoAutenticaNiConUnaFilaQueHasheoElVacio(t *testing.T) {
	e := newTestEngine(t)

	// La fila que produciría un AltaDevice "simplificado": credencial = hash de la cadena vacía.
	if _, err := e.db.Exec(
		`INSERT INTO devices (id, name, project_id, tier, caps, token_sha256, enrolled_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"id-tier-b", "switch-sala", "infra", string(fleet.TierProtocolo), "metrics",
		fleet.HashToken(""), time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		t.Fatal(err)
	}

	for _, vacio := range []string{"", "   "} {
		got, ok, err := e.DevicePorToken(vacio)
		if err != nil {
			t.Fatalf("DevicePorToken(%q): %v", vacio, err)
		}
		if ok {
			t.Fatalf("una peticion SIN credencial autentico como %q: bypass de autenticacion", got.Name)
		}
	}

	// Y el caso normal sigue andando: un Tier B guardado como corresponde tampoco autentica vacío.
	if _, err := e.AltaDevice(fleet.Device{
		Name: "nas", ProjectID: "infra", Tier: fleet.TierProtocolo, Caps: []fleet.Cap{fleet.CapMetrics},
	}, ""); err != nil {
		t.Fatalf("alta de Tier B sin token: %v", err)
	}
	if _, ok, _ := e.DevicePorToken(""); ok {
		t.Fatal("un token vacio autentico contra un Tier B guardado correctamente")
	}
}

// A2 — el token crudo no queda en NINGUNA columna de la fila.
// Sabotaje: guardar `token` en vez de `fleet.HashToken(token)` en AltaDevice.
func TestElTokenCrudoNoSeGuardaEnNingunaColumna(t *testing.T) {
	e := newTestEngine(t)
	_, token := altaDePrueba(t, e, "casa", "pc-gio")

	rows, err := e.db.Query(`SELECT * FROM devices`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	if !rows.Next() {
		t.Fatal("no hay filas")
	}
	celdas := make([]any, len(cols))
	punteros := make([]any, len(cols))
	for i := range celdas {
		punteros[i] = &celdas[i]
	}
	if err := rows.Scan(punteros...); err != nil {
		t.Fatal(err)
	}
	hashVisto := false
	for i, c := range celdas {
		s := ""
		switch v := c.(type) {
		case string:
			s = v
		case []byte:
			s = string(v)
		default:
			continue
		}
		if strings.Contains(s, token) {
			t.Errorf("la columna %q contiene el token CRUDO", cols[i])
		}
		if s == fleet.HashToken(token) {
			hashVisto = true
		}
	}
	if !hashVisto {
		t.Error("no se encontró el SHA-256 del token: la fila no guardó ninguna credencial")
	}
}

// A3 — un token identifica a UN device, y lo impone la BASE.
// Sabotaje: quitar el índice único parcial de la migración 29.
func TestDosDevicesNoPuedenCompartirCredencial(t *testing.T) {
	e := newTestEngine(t)
	_, token := altaDePrueba(t, e, "casa", "pc-uno")

	_, err := e.AltaDevice(fleet.Device{
		Name: "pc-dos", ProjectID: "casa", Tier: fleet.TierAgente, Caps: []fleet.Cap{fleet.CapMetrics},
	}, token) // ¡la MISMA credencial!
	if err == nil {
		t.Fatal("dos devices compartieron credencial: la auditoría no podría distinguirlos")
	}
}

// A3 — varios Tier B SIN credencial conviven (el índice único es PARCIAL).
// Sabotaje: hacer el índice único total → el segundo Tier B no se puede dar de alta.
func TestVariosTierBSinCredencialConviven(t *testing.T) {
	e := newTestEngine(t)
	for _, n := range []string{"router", "nas", "ups"} {
		if _, err := e.AltaDevice(fleet.Device{
			Name: n, ProjectID: "infra", Tier: fleet.TierProtocolo, Caps: []fleet.Cap{fleet.CapMetrics},
		}, ""); err != nil {
			t.Fatalf("alta de %q sin credencial: %v", n, err)
		}
	}
	got, err := e.ListarDevices("infra", false)
	if err != nil || len(got) != 3 {
		t.Fatalf("esperaba 3 devices sin credencial, obtuve %d (err=%v)", len(got), err)
	}
}

// A6 — el alta fail-closed del dominio llega hasta la base: sin project_id no se inserta.
func TestAltaSinProyectoNoLlegaALaBase(t *testing.T) {
	e := newTestEngine(t)
	if _, err := e.AltaDevice(fleet.Device{Name: "huerfano", Tier: fleet.TierAgente}, "tok"); !errors.Is(err, fleet.ErrSinProyecto) {
		t.Fatalf("esperaba ErrSinProyecto, obtuve %v", err)
	}
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM devices`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("se insertaron %d filas pese al alta rechazada", n)
	}
}

// A4 — la matriz del tier también corta en la capa de persistencia.
func TestAltaConCapFueraDeTierNoLlegaALaBase(t *testing.T) {
	e := newTestEngine(t)
	_, err := e.AltaDevice(fleet.Device{
		Name: "switch", ProjectID: "infra", Tier: fleet.TierProtocolo, Caps: []fleet.Cap{fleet.CapScreen},
	}, "")
	if !errors.Is(err, fleet.ErrCapFueraDeTier) {
		t.Fatalf("esperaba ErrCapFueraDeTier, obtuve %v", err)
	}
}

// A7 — el listado aísla por proyecto.
// Sabotaje: quitar el `WHERE project_id = ?` de ListarDevices.
func TestListarAislaPorProyecto(t *testing.T) {
	e := newTestEngine(t)
	altaDePrueba(t, e, "casa", "pc-gio")
	altaDePrueba(t, e, "casa", "laptop")
	altaDePrueba(t, e, "cliente-acme", "server-acme")

	casa, err := e.ListarDevices("casa", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(casa) != 2 {
		t.Fatalf("esperaba 2 devices en casa, obtuve %d", len(casa))
	}
	for _, d := range casa {
		if d.ProjectID != "casa" {
			t.Errorf("se filtró un device de %q listando casa: %q", d.ProjectID, d.Name)
		}
	}
	// Orden estable por nombre: una lista que baila entre llamadas hace ilegible un panel.
	if casa[0].Name != "laptop" || casa[1].Name != "pc-gio" {
		t.Errorf("el listado no vino ordenado por nombre: %v, %v", casa[0].Name, casa[1].Name)
	}
}

// A7 — listar sin proyecto no devuelve las filas HUÉRFANAS.
//
// Igual que la prueba de arriba, la primera versión no sabía fallar: pedía listar con "" y
// esperaba []. Pasaba con y sin la guarda, porque el WHERE filtra por project_id = cadena vacía
// y ninguna fila legítima la tiene (el alta exige proyecto). La guarda no es lo que impide
// devolver «todos» — eso ya lo impide el WHERE.
//
// Lo que la guarda evita es que un llamador que se OLVIDÓ el proyecto reciba justo las filas sin
// dueño, si alguna vez existieran (un backfill, una reparación a mano, una migración que afloje
// el NOT NULL). Esta versión inserta una a mano para que la guarda tenga algo que tapar.
//
// Sabotaje que la hace fallar: quitar la guarda de projectID vacío de ListarDevices.
func TestListarSinProyectoNoDevuelveLasFilasHuerfanas(t *testing.T) {
	e := newTestEngine(t)
	altaDePrueba(t, e, "casa", "pc-gio")

	// La fila huérfana que el alta nunca dejaría entrar, puesta a mano.
	if _, err := e.db.Exec(
		`INSERT INTO devices (id, name, project_id, tier, caps, enrolled_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		"id-huerfano", "sin-duenio", "", string(fleet.TierAgente), "metrics,exec",
		time.Now().UTC().Format(time.RFC3339),
	); err != nil {
		t.Fatal(err)
	}

	for _, vacio := range []string{"", "  "} {
		got, err := e.ListarDevices(vacio, true)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 0 {
			t.Fatalf("listar con projectID %q devolvio %d filas huerfanas (%v): un llamador que se olvido el proyecto no deberia verlas", vacio, len(got), nombres(got))
		}
	}

	// Y el listado normal sigue viendo lo suyo, sin arrastrar la huérfana.
	casa, err := e.ListarDevices("casa", true)
	if err != nil || len(casa) != 1 || casa[0].Name != "pc-gio" {
		t.Fatalf("esperaba solo pc-gio en casa, obtuve %v (err=%v)", nombres(casa), err)
	}
}

// A8 — «en línea» no se guarda: no hay columna `online`, y el estado sale del latido.
// Sabotaje: agregar una columna `online` a la migración 29.
func TestNoExisteColumnaOnline(t *testing.T) {
	e := newTestEngine(t)
	rows, err := e.db.Query(`PRAGMA table_info(devices)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid         int
			name, ctype string
			notnull, pk int
			dflt        any
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatal(err)
		}
		if strings.EqualFold(name, "online") || strings.EqualFold(name, "en_linea") {
			t.Fatalf("la tabla tiene una columna %q: el estado de conexión tiene que DERIVARSE de last_seen", name)
		}
	}
}

// A8 — el latido se persiste y el estado se deriva de él.
func TestLatidoSePersisteYElEstadoSeDeriva(t *testing.T) {
	e := newTestEngine(t)
	alta, token := altaDePrueba(t, e, "casa", "pc-gio")

	// Recién dado de alta: nunca latió, así que no está en línea con ningún umbral.
	if alta.LastSeen.IsZero() != true {
		t.Fatal("un device recién dado de alta no debería tener last_seen")
	}
	if alta.EnLinea(time.Now(), time.Hour) {
		t.Fatal("un device que nunca latió figuró en línea")
	}

	ahora := time.Now().UTC().Truncate(time.Second)
	ok, err := e.LatirDevice(alta.ID, ahora, "")
	if err != nil || !ok {
		t.Fatalf("LatirDevice: ok=%v err=%v", ok, err)
	}

	got, _, err := e.DevicePorToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if !got.LastSeen.UTC().Equal(ahora) {
		t.Fatalf("last_seen = %v, esperaba %v", got.LastSeen.UTC(), ahora)
	}
	if !got.EnLinea(ahora.Add(30*time.Second), time.Minute) {
		t.Error("con un latido de hace 30 s y umbral de 1 min, debería estar en línea")
	}
	if got.EnLinea(ahora.Add(10*time.Minute), time.Minute) {
		t.Error("con un latido de hace 10 min y umbral de 1 min, debería estar caído")
	}
}

// A10 — un latido que no encuentra a quién actualizar NO es un error.
// Sabotaje: devolver error cuando RowsAffected es 0 → cada máquina dada de baja produce una
// cascada de ruido hasta que alguien la apague.
func TestLatidoDeUnDeviceQueYaNoEstaNoEsError(t *testing.T) {
	e := newTestEngine(t)
	alta, _ := altaDePrueba(t, e, "casa", "pc-gio")

	if _, err := e.RevocarDevice("casa", "pc-gio"); err != nil {
		t.Fatal(err)
	}
	ok, err := e.LatirDevice(alta.ID, time.Now(), "")
	if err != nil {
		t.Fatalf("el latido de un device revocado devolvió error: %v", err)
	}
	if ok {
		t.Error("el latido de un device revocado dijo que actualizó")
	}
	// Un id que nunca existió: mismo trato.
	if ok, err := e.LatirDevice("id-inventado", time.Now(), ""); err != nil || ok {
		t.Errorf("latido de un id inexistente: ok=%v err=%v", ok, err)
	}
}

// A9 — revocar corta el acceso en el acto y la fila QUEDA para la auditoría.
// Sabotaje: cambiar el UPDATE por un DELETE → se pierde a quién pertenecía la telemetría.
func TestRevocarCortaElAccesoYConservaLaHistoria(t *testing.T) {
	e := newTestEngine(t)
	alta, token := altaDePrueba(t, e, "casa", "pc-gio")

	ok, err := e.RevocarDevice("casa", "pc-gio")
	if err != nil || !ok {
		t.Fatalf("RevocarDevice: ok=%v err=%v", ok, err)
	}
	// Deja de autenticar.
	if _, ok, _ := e.DevicePorToken(token); ok {
		t.Fatal("un device revocado siguió autenticando")
	}
	// La fila sigue, con su identidad y sus fechas.
	got, existe, err := e.DevicePorNombre("casa", "pc-gio")
	if err != nil || !existe {
		t.Fatalf("la fila desapareció al revocar: existe=%v err=%v", existe, err)
	}
	if got.ID != alta.ID || !got.Revoked || got.EnrolledAt.IsZero() {
		t.Fatalf("la fila revocada perdió información de auditoría: %+v", got)
	}
	// Y no permite nada, aunque conserve sus capacidades declaradas.
	if got.Permite(fleet.CapExec) {
		t.Error("un device revocado permitió exec")
	}
	// Revocar dos veces es idempotente y no miente.
	if ok, err := e.RevocarDevice("casa", "pc-gio"); err != nil || ok {
		t.Errorf("segunda revocación: ok=%v err=%v (esperaba false, nil)", ok, err)
	}
}

// A9 — el listado esconde a los revocados salvo que se los pida.
func TestListadoEscondeRevocadosSalvoQueSePidan(t *testing.T) {
	e := newTestEngine(t)
	altaDePrueba(t, e, "casa", "pc-gio")
	altaDePrueba(t, e, "casa", "vieja")
	if _, err := e.RevocarDevice("casa", "vieja"); err != nil {
		t.Fatal(err)
	}

	vivos, err := e.ListarDevices("casa", false)
	if err != nil || len(vivos) != 1 || vivos[0].Name != "pc-gio" {
		t.Fatalf("esperaba sólo pc-gio, obtuve %d: %v (err=%v)", len(vivos), nombres(vivos), err)
	}
	todos, err := e.ListarDevices("casa", true)
	if err != nil || len(todos) != 2 {
		t.Fatalf("esperaba 2 incluyendo revocados, obtuve %d (err=%v)", len(todos), err)
	}
}

// Dos «pc-gio» en el mismo proyecto harían ambiguo cualquier comando dirigido por nombre.
func TestNombreDuplicadoEnElMismoProyectoFalla(t *testing.T) {
	e := newTestEngine(t)
	altaDePrueba(t, e, "casa", "pc-gio")

	tok, _ := fleet.NuevoToken()
	_, err := e.AltaDevice(fleet.Device{
		Name: "pc-gio", ProjectID: "casa", Tier: fleet.TierAgente, Caps: []fleet.Cap{fleet.CapMetrics},
	}, tok)
	if !errors.Is(err, ErrDeviceDuplicado) {
		t.Fatalf("esperaba ErrDeviceDuplicado, obtuve %v", err)
	}
	// Pero el MISMO nombre en OTRO proyecto es legítimo: son flotas distintas.
	if _, err := e.AltaDevice(fleet.Device{
		Name: "pc-gio", ProjectID: "cliente-acme", Tier: fleet.TierAgente, Caps: []fleet.Cap{fleet.CapMetrics},
	}, tok+"-otro"); err != nil {
		t.Fatalf("el mismo nombre en otro proyecto debería poder: %v", err)
	}
}

// La base vacía no es un caso raro: es el primer arranque.
func TestFlotaVaciaNoEsError(t *testing.T) {
	e := newTestEngine(t)
	got, err := e.ListarDevices("casa", true)
	if err != nil || len(got) != 0 {
		t.Fatalf("flota vacía: esperaba [] sin error, obtuve %v / %v", got, err)
	}
	if _, ok, err := e.DevicePorNombre("casa", "no-existe"); ok || err != nil {
		t.Fatalf("DevicePorNombre inexistente: ok=%v err=%v", ok, err)
	}
}

func nombres(ds []fleet.Device) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Name
	}
	return out
}

// S4/D10 — la última muestra vive en la MISMA fila y se escribe en el MISMO UPDATE que el
// latido: la telemetría no agrega ni una escritura. Y un latido sin muestra no borra la anterior.
//
// Sabotaje que lo hace fallar: escribir la columna siempre (sin el CASE), o guardarla en una
// tabla aparte con su propio INSERT.
func TestLaUltimaMuestraViveEnLaFilaYNoSeBorraSola(t *testing.T) {
	e := newTestEngine(t)
	alta, token := altaDePrueba(t, e, "casa", "pc-gio")

	// Recién dada de alta: nunca reportó.
	if alta.UltimaMuestra != nil {
		t.Fatal("una máquina recién enrolada no debería tener muestra")
	}

	cpu := 42.5
	m := fleet.Muestra{Tomada: time.Now().UTC().Truncate(time.Second), CPUPct: &cpu, NumCPU: 8, MemTotal: 100, MemUsada: 40}
	txt, err := m.Serializar()
	if err != nil {
		t.Fatal(err)
	}
	if ok, err := e.LatirDevice(alta.ID, time.Now(), txt); err != nil || !ok {
		t.Fatalf("latido con muestra: ok=%v err=%v", ok, err)
	}

	got, _, err := e.DevicePorToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if got.UltimaMuestra == nil {
		t.Fatal("la muestra no se guardó")
	}
	if got.UltimaMuestra.CPUPct == nil || *got.UltimaMuestra.CPUPct != cpu || got.UltimaMuestra.NumCPU != 8 {
		t.Fatalf("la muestra no sobrevivió la ida y vuelta: %+v", got.UltimaMuestra)
	}

	// Un latido SIN muestra conserva la anterior.
	if ok, err := e.LatirDevice(alta.ID, time.Now(), ""); err != nil || !ok {
		t.Fatalf("latido sin muestra: ok=%v err=%v", ok, err)
	}
	got2, _, _ := e.DevicePorToken(token)
	if got2.UltimaMuestra == nil {
		t.Fatal("un latido sin muestra borró la anterior")
	}
	if got2.UltimaMuestra.NumCPU != 8 {
		t.Errorf("la muestra conservada cambió: %+v", got2.UltimaMuestra)
	}
}

// Una muestra guardada ILEGIBLE se trata como ausente, no rompe el listado. Misma regla que las
// capacidades: una fila rara no puede dejarte sin inventario.
func TestUnaMuestraIlegibleNoRompeElListado(t *testing.T) {
	e := newTestEngine(t)
	alta, _ := altaDePrueba(t, e, "casa", "pc-gio")
	if _, err := e.db.Exec(`UPDATE devices SET last_sample = ? WHERE id = ?`, "{esto no es json", alta.ID); err != nil {
		t.Fatal(err)
	}
	ds, err := e.ListarDevices("casa", false)
	if err != nil {
		t.Fatalf("una muestra ilegible rompió el listado: %v", err)
	}
	if len(ds) != 1 {
		t.Fatalf("esperaba 1 device, obtuve %d", len(ds))
	}
	if ds[0].UltimaMuestra != nil {
		t.Error("una muestra ilegible se devolvió como si fuera válida")
	}
}
