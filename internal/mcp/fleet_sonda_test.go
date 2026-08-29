package mcp

// Pruebas de la SONDA (S7b/S8): el cerebro saliendo a medir lo que no corre un agente.

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// lecturaProcFalsa es lo que devolvería un `cat /proc/...` remoto, con el separador del guion.
func lecturaProcFalsa() string {
	sep := "@@musubi@@"
	return strings.Join([]string{
		"cpu  931771 2364 439752 8613590 173159 0 7588 0 0 0",
		sep, "MemTotal: 7843996 kB\nMemAvailable: 4745104 kB\nSwapTotal: 0 kB",
		sep, "1.64 2.33 2.31 5/1181 94909",
		sep, "8623.50 86136.11",
		sep, "Filesystem 1B-blocks Used Available Use% Mounted\n/dev/x 502392610816 85868347392 391000000000 18% /",
		sep, "27800",
		sep, "12",
	}, "\n")
}

func conMetrics(proyecto string) *Principal {
	return &Principal{
		Name: "op", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: proyecto,
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"*"}},
	}
}

// La sonda mide un Tier B por SSH y GUARDA lo que trae, estampando la señal de vida.
//
// Sabotaje: no llamar a LatirDevice → el dispositivo queda medido pero figurando caído.
func TestLaSondaMideUnTierBYGuardaLaMuestra(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "nas", "tier": "B", "caps": []string{"metrics"}, "project": "infra",
		"address": "gio@nas.local", "os": "linux"}); e != nil {
		t.Fatal(e)
	}
	restaurar := fleet.SSHFalsoParaTest(t, "cat <<'EOF'\n"+lecturaProcFalsa()+"\nEOF")
	defer restaurar()

	res, e := callAsPrincipal(t, s, conMetrics("infra"), "musubi_fleet_probe", map[string]any{})
	if e != nil {
		t.Fatalf("probe: %+v", e)
	}
	out := jsonOf(t, res)
	if out["sondeados"] != float64(1) {
		t.Fatalf("sondeados = %v", out["sondeados"])
	}
	fila := out["resultados"].([]any)[0].(map[string]any)
	if fila["ok"] != true {
		t.Fatalf("el sondeo falló: %v", fila["error"])
	}
	if fila["transporte"] != "ssh" {
		t.Errorf("transporte = %v", fila["transporte"])
	}
	// La muestra quedó guardada Y la máquina figura viva.
	d, _, _ := s.engine.DevicePorNombre("infra", "nas")
	if d.UltimaMuestra == nil {
		t.Fatal("no se guardó la muestra")
	}
	if d.UltimaMuestra.NumCPU != 12 {
		t.Errorf("la muestra no es la que vino: %+v", d.UltimaMuestra)
	}
	if !d.EnLinea(time.Now(), umbralEnLineaDefault) {
		t.Error("se midió la máquina y no figura viva")
	}
	// El PRIMER sondeo no trae CPU: la derivada necesita una lectura previa.
	if fila["cpu_pct"] != nil {
		t.Errorf("el primer sondeo trajo cpu_pct=%v: no hay contra qué restar", fila["cpu_pct"])
	}
}

// EL TECHO DE iOS SE DICE ANTES DE INTENTAR NADA.
//
// Un error de adb mandaría a alguien a depurar el cable cuando el problema es que la plataforma
// no lo permite.
//
// Sabotaje que la hace fallar: quitar la guarda EsIOS de sondearUno.
func TestUnIPhoneNoSeIntentaSondearYSeDicePorQue(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "iphone-de-gio", "tier": "C", "caps": []string{"metrics"},
		"project": "casa", "os": "ios", "address": "no-importa"}); e != nil {
		t.Fatal(e)
	}
	res, e := callAsPrincipal(t, s, conMetrics("casa"), "musubi_fleet_probe", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	fila := jsonOf(t, res)["resultados"].([]any)[0].(map[string]any)
	if fila["ok"] != false {
		t.Fatal("se reportó éxito sondeando un iPhone")
	}
	if fila["transporte"] != "ninguno" {
		t.Errorf("se eligió un transporte (%v) para algo que no se puede medir", fila["transporte"])
	}
	err, _ := fila["error"].(string)
	if !strings.Contains(err, "MDM") {
		t.Errorf("el error no explica que hace falta un MDM: %q", err)
	}
	if strings.Contains(strings.ToLower(err), "adb") {
		t.Errorf("se intentó por adb: el mensaje habla del cable en vez de la plataforma: %q", err)
	}
}

// Los Tier A NO se sondean: reportan solos, y además nada les entra.
//
// Sabotaje: sondear también los Tier A → el cerebro intentaría abrir un ssh contra un portátil
// detrás de un NAT, por cada llamada.
func TestLosTierANoSeSondean(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	enrolarDePrueba(t, s, "casa", "pc-gio") // Tier A
	res, e := callAsPrincipal(t, s, conMetrics("casa"), "musubi_fleet_probe", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	out := jsonOf(t, res)
	if out["sondeados"] != float64(0) {
		t.Fatalf("se sondeó un Tier A: %v", out["sondeados"])
	}
	if out["con_agente_propio"] != float64(1) {
		t.Errorf("no se informa por qué se salteó: %v", out)
	}
	if nota, _ := out["nota"].(string); !strings.Contains(nota, "musubi agent") {
		t.Errorf("la nota no dice qué hacer en su lugar: %q", nota)
	}
}

// La compuerta manda: sondear es MEDIR, así que exige `metrics` por máquina.
//
// Sabotaje: quitar el PuedeSobreDevice de toolFleetProbe.
func TestSondearExigeLaCapacidadMetrics(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "nas", "tier": "B", "caps": []string{"metrics"}, "project": "infra",
		"address": "nas.local"}); e != nil {
		t.Fatal(e)
	}
	sinNada := &Principal{Name: "curioso", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "infra"}
	res, e := callAsPrincipal(t, s, sinNada, "musubi_fleet_probe", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	out := jsonOf(t, res)
	if out["sondeados"] != float64(0) {
		t.Fatalf("un admin sin concesiones sondeó %v máquinas", out["sondeados"])
	}
	if out["sin_permiso"] == nil {
		t.Error("no se informa cuántas quedaron fuera")
	}
}

// Un dispositivo inalcanzable NO queda marcado como vivo, y se dice qué pasó.
func TestUnDispositivoInalcanzableNoQuedaVivo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "router", "tier": "B", "caps": []string{"metrics"}, "project": "infra",
		"address": "router.local"}); e != nil {
		t.Fatal(e)
	}
	restaurar := fleet.SSHFalsoParaTest(t, "echo 'ssh: connect to host router.local port 22: Connection refused' >&2; exit 255")
	defer restaurar()

	res, e := callAsPrincipal(t, s, conMetrics("infra"), "musubi_fleet_probe", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	fila := jsonOf(t, res)["resultados"].([]any)[0].(map[string]any)
	if fila["ok"] != false {
		t.Fatal("un dispositivo inalcanzable reportó éxito")
	}
	d, _, _ := s.engine.DevicePorNombre("infra", "router")
	if !d.LastSeen.IsZero() {
		t.Fatal("un dispositivo INALCANZABLE quedó marcado como vivo")
	}
	if d.UltimaMuestra != nil {
		t.Error("se guardó una muestra de un dispositivo que no respondió")
	}
}

// Una salida que NO es de un Linux se rechaza en vez de guardarse como una muestra de ceros.
func TestUnRouterQueNoTieneProcNoProduceUnaMuestraDeCeros(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "router-viejo", "tier": "B", "caps": []string{"metrics"}, "project": "infra",
		"address": "router.local"}); e != nil {
		t.Fatal(e)
	}
	// Responde al ssh, pero es un firmware propietario sin /proc.
	restaurar := fleet.SSHFalsoParaTest(t, "echo 'Welcome to RouterOS'; exit 0")
	defer restaurar()

	res, e := callAsPrincipal(t, s, conMetrics("infra"), "musubi_fleet_probe", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	fila := jsonOf(t, res)["resultados"].([]any)[0].(map[string]any)
	if fila["ok"] != false {
		t.Fatal("un router sin /proc produjo una muestra: sería 0% de todo, y ese cero se cree")
	}
	if err, _ := fila["error"].(string); !strings.Contains(err, "/proc") {
		t.Errorf("el error no explica que no hay /proc: %q", err)
	}
	d, _, _ := s.engine.DevicePorNombre("infra", "router-viejo")
	if d.UltimaMuestra != nil {
		t.Error("se guardó una muestra de ceros")
	}
}

// ── A52 · La fila del sondeo, campo por campo ────────────────────────────────────────────────

// lecturaProcCompleta es la lectura de un Linux que SÍ expone todo: las OCHO secciones del guion,
// con `MemFree` en el meminfo y el listado de `/proc` al final.
//
// POR QUÉ HACE FALTA OTRA Y NO ALCANZABA `lecturaProcFalsa`: aquélla tiene SIETE secciones y un
// meminfo sin `MemFree`, así que produce una muestra con `MemLibre = nil` y `NumProcesos = 0` —
// exactamente los dos valores que las pruebas de esta sección tienen que distinguir de un campo
// borrado. Con esa lectura, borrar `fila["mem_libre"]` del código no cambia NADA observable: el
// campo valía null antes y después. Ése era el agujero de A52: no faltaba una aserción, faltaba
// un fixture que llegara a los campos.
func lecturaProcCompleta() string {
	sep := "@@musubi@@"
	// 42 entradas numéricas + las no-numéricas que `ls -1 /proc` mezcla y que ContarPids descarta.
	var pids []string
	for i := 1; i <= 42; i++ {
		pids = append(pids, strconv.Itoa(i*7))
	}
	pids = append(pids, "cpuinfo", "meminfo", "self", "sys")
	return strings.Join([]string{
		"cpu  931771 2364 439752 8613590 173159 0 7588 0 0 0",
		sep, "MemTotal: 7843996 kB\nMemAvailable: 4745104 kB\nMemFree: 1048576 kB\nSwapTotal: 0 kB",
		sep, "1.64 2.33 2.31 5/1181 94909",
		sep, "8623.50 86136.11",
		sep, "Filesystem 1B-blocks Used Available Use% Mounted\n/dev/x 502392610816 85868347392 391000000000 18% /",
		sep, "27800",
		sep, "12",
		sep, strings.Join(pids, "\n"),
	}, "\n")
}

// La fila que ve el operador tiene que traer lo que se midió. Los dos campos que A52 nombra
// —`mem_libre` y `num_procesos`— se agregaron en U1 y NINGUNA prueba los miraba en esta fila: el
// verificador borró la línea de uno y crudificó el otro, y la suite quedó en `ok`.
//
// Sabotaje que la hace fallar: borrar `fila["mem_libre"] = m.MemLibre` de sondearUno.
// Sabotaje que la hace fallar: cambiar `enteroONull(m.NumProcesos)` por `m.NumProcesos`.
func TestLaFilaDelSondeoTraeMemoriaLibreYProcesos(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "nas", "tier": "B", "caps": []string{"metrics"}, "project": "infra",
		"address": "gio@nas.local", "os": "linux"}); e != nil {
		t.Fatal(e)
	}
	defer fleet.SSHFalsoParaTest(t, "cat <<'EOF'\n"+lecturaProcCompleta()+"\nEOF")()

	res, e := callAsPrincipal(t, s, conMetrics("infra"), "musubi_fleet_probe", map[string]any{})
	if e != nil {
		t.Fatalf("probe: %+v", e)
	}
	fila := jsonOf(t, res)["resultados"].([]any)[0].(map[string]any)
	if fila["ok"] != true {
		t.Fatalf("el sondeo falló: %v", fila["error"])
	}

	// MemFree: 1048576 kB = 1 GiB. Se compara contra el número EXACTO y no contra «no es nil»:
	// `mem_libre` y `mem_usada` salen de campos distintos de meminfo y la confusión entre los dos
	// es de 3,5 GB en una máquina real, así que una aserción de presencia dejaría pasar el cruce.
	libre, hay := fila["mem_libre"].(float64)
	if !hay {
		t.Fatalf("la fila no trae mem_libre: %#v", fila["mem_libre"])
	}
	if libre != 1048576*1024 {
		t.Errorf("mem_libre = %.0f; se esperaba MemFree (1 GiB), no otro campo de meminfo", libre)
	}
	// Y NO es lo mismo que la usada, que sale de MemAvailable. Si alguien las cruza, este check
	// es el que lo dice.
	if usada := (7843996 - 4745104) * 1024.0; libre == usada {
		t.Error("mem_libre quedó igual a la memoria USADA: se cruzaron MemFree y MemAvailable")
	}

	procs, hay := fila["num_procesos"].(float64)
	if !hay {
		t.Fatalf("la fila no trae num_procesos: %#v", fila["num_procesos"])
	}
	if procs != 42 {
		t.Errorf("num_procesos = %.0f; `ls -1 /proc` traía 42 pids numéricos (y cuatro entradas "+
			"que no lo son y no se cuentan)", procs)
	}
	// El 4º campo de loadavg es 5/1181 — HILOS, no procesos. Si alguien lo usa de atajo, esto lo
	// dice; los dos números son plausibles y nada más se pondría rojo.
	if procs == 1181 {
		t.Error("num_procesos salió del 4º campo de loadavg: eso cuenta HILOS, no procesos")
	}
}

// EL CERO QUE SIGNIFICA «NO SÉ» NO PUEDE VIAJAR COMO 0. Es la regla central del track: un cero
// inventado es indistinguible de uno medido, y en la fila del sondeo se lee «esta máquina no
// tiene procesos», que es imposible.
//
// El caso es real y no hipotético: un Linux viejo cuya salida trae siete secciones —el guion
// apendea la de `/proc` al final justamente para no romperlo— y un macOS por SSH, que no la mide.
//
// Sabotaje que la hace fallar: cambiar `enteroONull(m.NumProcesos)` por `m.NumProcesos`.
func TestUnaMaquinaQueNoCuentaProcesosMandaNullYNoCero(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "nas-viejo", "tier": "B", "caps": []string{"metrics"}, "project": "infra",
		"address": "gio@viejo.local", "os": "linux"}); e != nil {
		t.Fatal(e)
	}
	// `lecturaProcFalsa` es exactamente la salida VIEJA de siete secciones: sin `ls -1 /proc`.
	defer fleet.SSHFalsoParaTest(t, "cat <<'EOF'\n"+lecturaProcFalsa()+"\nEOF")()

	res, e := callAsPrincipal(t, s, conMetrics("infra"), "musubi_fleet_probe", map[string]any{})
	if e != nil {
		t.Fatalf("probe: %+v", e)
	}
	fila := jsonOf(t, res)["resultados"].([]any)[0].(map[string]any)
	if fila["ok"] != true {
		t.Fatalf("el sondeo falló: %v", fila["error"])
	}
	// La clave TIENE que estar y valer null. Que falte no alcanza: un cliente que hace
	// `fila.num_procesos ?? 0` no distingue una clave ausente de un cero, y el punto es que se vea
	// el hueco.
	valor, presente := fila["num_procesos"]
	if !presente {
		t.Fatal("num_procesos desapareció de la fila: el hueco tiene que ser visible, no ausente")
	}
	if valor != nil {
		t.Errorf("num_procesos = %#v en una máquina que no los mide; un 0 se lee «no tiene "+
			"procesos», que es imposible, y es indistinguible de uno medido", valor)
	}
	// Y el mismo criterio para la memoria libre, que este fixture tampoco trae.
	if v, presente := fila["mem_libre"]; !presente {
		t.Error("mem_libre desapareció de la fila en vez de viajar como null")
	} else if v != nil {
		t.Errorf("mem_libre = %#v sin MemFree en el meminfo", v)
	}
}
