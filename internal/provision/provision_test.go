package provision

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/config"
)

// ── fakes (compartidos por los tests del paquete) ────────────────────────────

type fakeProber struct{ public, tailnet bool }

func (f *fakeProber) PublicReachable() bool        { return f.public }
func (f *fakeProber) TailnetReachable(string) bool { return f.tailnet }

type fakeVerifier struct {
	reach, auth           bool
	reachCalls, authCalls int
}

func (f *fakeVerifier) Reach(string) (bool, string) { f.reachCalls++; return f.reach, "reach-detail" }
func (f *fakeVerifier) Auth(string, string) (bool, string) {
	f.authCalls++
	return f.auth, "auth-detail"
}

type fakeNetwork struct {
	present, joined bool
	ensureCalls     int
	lastEnsureDry   bool
	joinCalls       int
}

func (f *fakeNetwork) TailscaleState() (bool, bool) { return f.present, f.joined }
func (f *fakeNetwork) EnsureTailnetAllowed(dry bool) StepResult {
	f.ensureCalls++
	f.lastEnsureDry = dry
	return StepResult{Name: "firewall", Status: StatusOK, Detail: "regla presente"}
}
func (f *fakeNetwork) JoinTailscale(string) StepResult {
	f.joinCalls++
	return StepResult{Name: "tailscale", Status: StatusDone, Detail: "unido"}
}

func stepByName(rep Report, name string) (StepResult, bool) {
	for _, s := range rep.Steps {
		if s.Name == name {
			return s, true
		}
	}
	return StepResult{}, false
}

// ── escenarios ───────────────────────────────────────────────────────────────

func TestRunCleanWiresAndSelfChecks(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROV_TOKEN", "secreto")
	v := &fakeVerifier{reach: true, auth: true}
	nc := &fakeNetwork{present: true, joined: true}
	opts := Options{Brain: "1.2.3.4:7717", ProjectDir: dir, TokenEnv: "PROV_TOKEN"}
	deps := Deps{Prober: &fakeProber{public: true, tailnet: true}, Verifier: v, NetworkConfigurator: nc, ExePath: "musubi"}

	rep, err := Run(opts, deps)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Mode != ModeClean {
		t.Fatalf("modo = %v; quería Clean", rep.Mode)
	}
	if !rep.Connected {
		t.Fatal("esperaba Connected=true")
	}
	if v.reachCalls != 1 || v.authCalls != 1 {
		t.Fatalf("self-check no corrió: reach=%d auth=%d", v.reachCalls, v.authCalls)
	}
	// El .mcp.json debe tener AMBAS entradas, preservando estructura.
	data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf("no se escribió .mcp.json: %v", err)
	}
	s := string(data)
	if !strings.Contains(s, `"musubi"`) || !strings.Contains(s, `"musubi-cerebro"`) {
		t.Fatalf(".mcp.json sin ambas entradas:\n%s", s)
	}
	if strings.Contains(s, "secreto") {
		t.Fatal("FUGA: el token no debe aparecer en el .mcp.json")
	}
}

func TestRunTunneledStopsBeforeMutating(t *testing.T) {
	dir := t.TempDir()
	nc := &fakeNetwork{present: true, joined: true}
	opts := Options{Brain: "1.2.3.4:7717", ProjectDir: dir, TokenEnv: "PROV_TOKEN"}
	// público OK, tailnet NO = Tunneled.
	deps := Deps{Prober: &fakeProber{public: true, tailnet: false}, Verifier: &fakeVerifier{}, NetworkConfigurator: nc, ExePath: "musubi"}

	rep, err := Run(opts, deps)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if rep.Mode != ModeTunneled {
		t.Fatalf("modo = %v; quería Tunneled", rep.Mode)
	}
	if rep.Connected {
		t.Fatal("Tunneled no debería declararse conectado")
	}
	if nc.ensureCalls != 0 {
		t.Fatal("no debería tocar el firewall antes de resolver el bloqueo")
	}
	if _, err := os.Stat(filepath.Join(dir, ".mcp.json")); !os.IsNotExist(err) {
		t.Fatal("no debería escribir el .mcp.json en modo Tunneled")
	}
	if s, ok := stepByName(rep, "red"); !ok || s.Status != StatusTodo {
		t.Fatalf("esperaba un paso 'red' TODO con guía; got %+v", s)
	}
}

func TestRunDryRunDoesNotMutate(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROV_TOKEN", "secreto")
	v := &fakeVerifier{reach: true, auth: true}
	nc := &fakeNetwork{present: true, joined: true}
	opts := Options{Brain: "1.2.3.4:7717", ProjectDir: dir, TokenEnv: "PROV_TOKEN", DryRun: true}
	deps := Deps{Prober: &fakeProber{public: true, tailnet: true}, Verifier: v, NetworkConfigurator: nc, ExePath: "musubi"}

	rep, err := Run(opts, deps)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".mcp.json")); !os.IsNotExist(err) {
		t.Fatal("dry-run no debería escribir el .mcp.json")
	}
	if !nc.lastEnsureDry {
		t.Fatal("EnsureTailnetAllowed debería recibir dryRun=true")
	}
	if v.reachCalls != 0 || v.authCalls != 0 {
		t.Fatal("dry-run no debería correr el self-check")
	}
	if _, err := os.Stat(filepath.Join(dir, ".musubi", "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("dry-run no debería escribir el config.yaml")
	}
	if rep.Connected {
		t.Fatal("dry-run no conecta")
	}
}

func TestRunWritesSyncConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROV_TOKEN", "secreto")
	opts := Options{Brain: "1.2.3.4:7717", ProjectDir: dir, TokenEnv: "PROV_TOKEN"}
	deps := Deps{Prober: &fakeProber{public: true, tailnet: true}, Verifier: &fakeVerifier{reach: true, auth: true}, NetworkConfigurator: &fakeNetwork{present: true, joined: true}, ExePath: "musubi"}

	rep, err := Run(opts, deps)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".musubi", "config.yaml"))
	if err != nil {
		t.Fatalf("no se escribió .musubi/config.yaml: %v", err)
	}
	s := string(data)
	for _, want := range []string{"sync:", "enabled: true", "central_url: http://1.2.3.4:7717", "auth_token_env: PROV_TOKEN", "allow_insecure_token: true"} {
		if !strings.Contains(s, want) {
			t.Fatalf("config.yaml sin %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "secreto") {
		t.Fatal("FUGA: el token no debe aparecer en el config.yaml")
	}
	if st, ok := stepByName(rep, "sync-config"); !ok || st.Status != StatusDone {
		t.Fatalf("esperaba sync-config=done; got %+v", st)
	}

	// Idempotencia: segunda corrida NO duplica el bloque y reporta ok.
	rep2, err := Run(opts, deps)
	if err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}
	data2, _ := os.ReadFile(filepath.Join(dir, ".musubi", "config.yaml"))
	if n := strings.Count(string(data2), "\nsync:"); n != 1 {
		t.Fatalf("el bloque sync: se duplicó (%d veces)", n)
	}
	if st, ok := stepByName(rep2, "sync-config"); !ok || st.Status != StatusOK {
		t.Fatalf("re-ejecución debería reportar sync-config=ok; got %+v", st)
	}
}

func TestSyncConfigPreservesExistingConfig(t *testing.T) {
	dir := t.TempDir()
	musubiDir := filepath.Join(dir, ".musubi")
	if err := os.MkdirAll(musubiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	prior := "embedding:\n  model: nomic-embed-text\n"
	if err := os.WriteFile(filepath.Join(musubiDir, "config.yaml"), []byte(prior), 0o644); err != nil {
		t.Fatal(err)
	}
	res := ensureSyncConfig(dir, "1.2.3.4:7717", "MUSUBI_TOKEN", false)
	if res.Status != StatusDone {
		t.Fatalf("esperaba done; got %+v", res)
	}
	data, _ := os.ReadFile(filepath.Join(musubiDir, "config.yaml"))
	s := string(data)
	if !strings.Contains(s, "model: nomic-embed-text") {
		t.Fatal("no preservó la config previa")
	}
	if !strings.Contains(s, "sync:") {
		t.Fatal("no agregó el bloque sync:")
	}
}

// REGRESIÓN (auditoría 2026-07-26, #2): el caso REAL. ensureWorkspace deja un config.yaml completo
// (config.Default().Marshal()) que YA trae un bloque `sync:` con enabled:false. El match textual de
// `^sync:` lo daba por "ya configurado" y NO habilitaba nada ⇒ el outbox nunca drenaba, con ✓ verde.
func TestSyncConfigEnablesWhenPresentButDisabled(t *testing.T) {
	dir := t.TempDir()
	musubiDir := filepath.Join(dir, ".musubi")
	if err := os.MkdirAll(musubiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	def, err := config.Default().Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(musubiDir, "config.yaml"), def, 0o644); err != nil {
		t.Fatal(err)
	}

	res := ensureSyncConfig(dir, "100.79.126.62:7717", "MUSUBI_TOKEN", false)
	if res.Status == StatusOK {
		t.Fatalf("no debe reportar 'ya configurado' con sync enabled:false; status=%v detail=%q", res.Status, res.Detail)
	}

	// El config resultante debe parsear (sin clave `sync:` duplicada) y quedar habilitado.
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("el config resultante no parsea (¿clave sync duplicada?): %v", err)
	}
	if !cfg.Sync.Enabled {
		t.Errorf("provision debe dejar sync.enabled=true, quedó false")
	}
	if cfg.Sync.CentralURL == "" {
		t.Errorf("provision debe fijar sync.central_url, quedó vacío")
	}
}

func TestRunIdempotent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROV_TOKEN", "secreto")
	mk := func() (Report, error) {
		return Run(
			Options{Brain: "1.2.3.4:7717", ProjectDir: dir, TokenEnv: "PROV_TOKEN"},
			Deps{Prober: &fakeProber{public: true, tailnet: true}, Verifier: &fakeVerifier{reach: true, auth: true}, NetworkConfigurator: &fakeNetwork{present: true, joined: true}, ExePath: "musubi"},
		)
	}
	if _, err := mk(); err != nil {
		t.Fatalf("primera corrida: %v", err)
	}
	rep2, err := mk()
	if err != nil {
		t.Fatalf("segunda corrida: %v", err)
	}
	s, ok := stepByName(rep2, "mcp.json")
	if !ok || s.Status != StatusOK {
		t.Fatalf("la re-ejecución debería reportar mcp.json sin cambios (ok); got %+v", s)
	}
}

func TestRunEmptyTokenSkipsAuth(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROV_TOKEN", "") // explícitamente vacío
	v := &fakeVerifier{reach: true, auth: true}
	opts := Options{Brain: "1.2.3.4:7717", ProjectDir: dir, TokenEnv: "PROV_TOKEN"}
	deps := Deps{Prober: &fakeProber{public: true, tailnet: true}, Verifier: v, NetworkConfigurator: &fakeNetwork{present: true, joined: true}, ExePath: "musubi"}

	rep, err := Run(opts, deps)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if v.reachCalls != 1 {
		t.Fatal("reach debería correr aún sin token")
	}
	if v.authCalls != 0 {
		t.Fatal("auth NO debería correr sin token")
	}
	if s, ok := stepByName(rep, "auth"); !ok || s.Status != StatusSkipped {
		t.Fatalf("esperaba auth=skipped; got %+v", s)
	}
	if rep.Connected {
		t.Fatal("sin auth verificada no hay conexión confirmada")
	}
}

// ─────────────────────────────────────────────────────────────────────────────────────────────
// A129 · LOS DOS ARTEFACTOS DE UNA MISMA CORRIDA DICEN LA MISMA DIRECCIÓN
// ─────────────────────────────────────────────────────────────────────────────────────────────

// TestUnBrainConEsquemaLlegaIgualALosDosArtefactos — la guarda del cuarto sitio.
//
// EL DEFECTO, MEDIDO EL 2026-09-20. `provision` escribe la dirección del cerebro en DOS archivos:
// el `.mcp.json` (lo lee el host MCP) y el `.musubi/config.yaml` (lo lee el daemon). Tres de los
// cuatro sitios que la arman pasaron por el normalizador y el cuarto —`ensureSyncConfig`— quedó
// con `http://%s` y el brain crudo. Con `--brain https://host:10000` la MISMA corrida dejaba el
// .mcp.json bien y el config.yaml con `central_url: http://https://host:10000`.
//
// Y ESO NO FALLA RUIDOSO, QUE ES LO PEOR. `url.Parse` acepta esa cadena, `NewSyncClient` la acepta
// porque empieza con `http://`, y el error llega recién en el primer drain como fallo de RED — o
// sea transitorio: la fila vuelve a `pending` con backoff, sin dead-letter, mientras el paso se
// reporta `done`. Un sync que no sube nada y no se queja.
//
// POR QUÉ LA SUITE NO LO VEÍA: los seis tests del paquete usan `Brain: "1.2.3.4:7717"`, el ÚNICO
// input donde `"http://" + brain` y `direccionDelCerebro(brain)` dan el mismo byte. Este test
// entra por el otro lado.
//
// Sabotaje que lo hace fallar: devolverle a `ensureSyncConfig` su `"  central_url: http://%s\n"`.
// arnes: archivo="internal/provision/syncconfig.go"
// arnes: de="\t\t\"  central_url: %s\\n\"+"
// arnes: a="\t\t\"  central_url: http://%s\\n\"+"
func TestUnBrainConEsquemaLlegaIgualALosDosArtefactos(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PROV_TOKEN", "secreto")
	const destino = "https://musubi-server.tail89e295.ts.net:10000"
	opts := Options{Brain: destino, ProjectDir: dir, TokenEnv: "PROV_TOKEN"}
	deps := Deps{Prober: &fakeProber{public: true, tailnet: true}, Verifier: &fakeVerifier{reach: true, auth: true}, NetworkConfigurator: &fakeNetwork{present: true, joined: true}, ExePath: "musubi"}

	if _, err := Run(opts, deps); err != nil {
		t.Fatalf("Run: %v", err)
	}

	cfg, err := os.ReadFile(filepath.Join(dir, ".musubi", "config.yaml"))
	if err != nil {
		t.Fatalf("no se escribió .musubi/config.yaml: %v", err)
	}
	mcp, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf("no se escribió .mcp.json: %v", err)
	}

	// EL CONTROL QUE CAZA LA FORMA ROTA, y es distinto de «contiene la buena»: `http://https://…`
	// CONTIENE la cadena buena como sufijo, así que un Contains a secas pasaría con el defecto puesto.
	if strings.Contains(string(cfg), "http://https://") || strings.Contains(string(mcp), "http://https://") {
		t.Fatalf("un esquema quedó pegado a otro — el brain se usó crudo en algún sitio:\nconfig.yaml:\n%s\n.mcp.json:\n%s", cfg, mcp)
	}
	if !strings.Contains(string(cfg), "central_url: "+destino) {
		t.Fatalf("el config.yaml no lleva la dirección tal como se pidió (%s):\n%s", destino, cfg)
	}
	// LA DIRECCIÓN VA SIN EL SUFIJO `/mcp`, y eso NO es un descuido: la entrada del cerebro se
	// cablea por STDIO (`musubi cerebro --url <base>`) y es ese comando quien arma el endpoint
	// (`base + "/mcp"`). Pedirle el sufijo al archivo ataría esta prueba a la forma remota.
	if !strings.Contains(string(mcp), destino) {
		t.Fatalf("el .mcp.json no lleva la dirección tal como se pidió (%s):\n%s", destino, mcp)
	}

	// Y LA GUARDA QUE HABRÍA CAZADO EL DEFECTO DEL 2026-09-21, que es de lo que se trata todo esto.
	//
	// El cerebro se cableaba como `{"type":"http","url":…,"headers":{"Authorization":"Bearer
	// ${VAR}"}}`, apoyado en que el cliente MCP expandiera la variable y MANDARA el header. El
	// cliente de Claude Code NO manda los `headers` del .mcp.json (bug anthropics/claude-code
	// #48514), así que la credencial nunca llegaba: el servidor quedaba en «Failed» con un error
	// que no nombra la causa, y NADA en el árbol lo decía — `musubi cerebro` existía justamente
	// para eso y este sitio seguía generando la forma que ese comando vino a reemplazar.
	//
	// Se mira la FORMA y no el texto del header: pedir que no aparezca «Authorization» dejaría
	// pasar un `type: "http"` sin credencial, que falla igual y por lo mismo.
	if strings.Contains(string(mcp), `"type": "http"`) || strings.Contains(string(mcp), `"type":"http"`) {
		t.Errorf("el .mcp.json volvió a cablear el cerebro como servidor remoto `type: http`.\n"+
			"  El cliente MCP de Claude Code no manda los `headers` que declara el archivo, así que "+
			"el bearer nunca llega y la entrada queda en «Failed».\n"+
			"  Va por stdio: `musubi cerebro --url <base> --token-env <VAR>`, que además resuelve la "+
			"credencial con `config.SecretoDeEnv` y por lo tanto honra `<VAR>_FILE`.\n%s", mcp)
	}
	if !strings.Contains(string(mcp), `"cerebro"`) || !strings.Contains(string(mcp), `"--token-env"`) {
		t.Errorf("el .mcp.json no cablea el cerebro por stdio con `musubi cerebro --token-env`:\n%s", mcp)
	}

	// Con una base https, el opt-in a texto plano NO se escribe: dejarlo puesto es autorizar de
	// antemano una vuelta atrás que después nadie nota.
	if strings.Contains(string(cfg), "allow_insecure_token") {
		t.Errorf("con central_url https se escribió allow_insecure_token, y eso deja permitido volver a texto plano:\n%s", cfg)
	}

	// Y el control de siempre: el secreto no toca el disco.
	if strings.Contains(string(cfg), "secreto") || strings.Contains(string(mcp), "secreto") {
		t.Fatal("FUGA: el token no debe aparecer en ningún archivo generado")
	}
}

// TestUnaDireccionInservibleNoSeEscribeEnDisco — lo que no sirve no se persiste.
//
// Antes, un esquema que no fuera http(s) se devolvía tal cual y los llamadores lo ESCRIBÍAN: el
// error aparecía mucho después, en otra máquina y sin la causa a la vista.
//
// Sabotaje: hacer que `direccionDelCerebro` devuelva ok=true para cualquier esquema.
// arnes: archivo="internal/provision/probe.go"
// arnes: de="\t\tif esquema != \"http\" && esquema != \"https\" {"
// arnes: a="\t\tif false {"
func TestUnaDireccionInservibleNoSeEscribeEnDisco(t *testing.T) {
	for _, brain := range []string{"ftp://100.79.126.62:10000", "://roto", ""} {
		dir := t.TempDir()
		t.Setenv("PROV_TOKEN", "secreto")
		opts := Options{Brain: brain, ProjectDir: dir, TokenEnv: "PROV_TOKEN"}
		deps := Deps{Prober: &fakeProber{public: true, tailnet: true}, Verifier: &fakeVerifier{reach: true, auth: true}, NetworkConfigurator: &fakeNetwork{present: true, joined: true}, ExePath: "musubi"}

		rep, err := Run(opts, deps)
		if err != nil {
			continue // rechazado antes de escribir: es la respuesta correcta
		}
		if data, rerr := os.ReadFile(filepath.Join(dir, ".musubi", "config.yaml")); rerr == nil {
			if strings.Contains(string(data), "central_url: "+brain) || strings.Contains(string(data), "http://"+brain) {
				t.Errorf("con brain %q se escribió una central_url inservible en disco:\n%s", brain, data)
			}
		}
		if st, ok := stepByName(rep, "sync-config"); ok && st.Status == StatusDone {
			t.Errorf("con brain %q el paso sync-config se reportó DONE; un valor que no sirve no puede salir bien", brain)
		}
	}
}
