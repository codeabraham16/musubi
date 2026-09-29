package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
	"musubi/internal/transcripts"
)

// Después de compactar, el hook de arranque devuelve la memoria que el resumen perdió
// (detect_compactar.go). Las pruebas fijan las tres cosas que hace la rama «compact» —vaciar el
// delta de ESA sesión, olvidar sus marcas y volver a buscar sobre el último pedido sin mostrarlo—,
// que NO corre el arranque entero, que «clear» sí va por el arranque, que setup y el plugin instalan
// los tres matchers, que el plugin cede según el matcher y que el arranque avisa una sola vez cuando
// un repo quedó sin el de la compactación.

// pedidoDelTLS es un pedido sustantivo que el corpus de motorConCorpusDeRuido contesta con la nota
// «tls». Ninguna nota lo contiene textual: si aparece en el bloque, lo puso el hook.
const pedidoDelTLS = "revisá cómo verifica el cerebro el TLS contra el nombre del certificado"

// notaDelTLS es la nota que ese pedido trae.
const notaDelTLS = "el cerebro va por TLS: se disca la IP y se verifica contra el nombre del certificado"

// aislarDelPluginInstalado saca de la prueba el plugin que haya en la PC que la corre: la cobertura
// de la compactación mira CLAUDE_PLUGIN_ROOT y, sin él, la carpeta de configuración de Claude Code.
func aislarDelPluginInstalado(t *testing.T) {
	t.Helper()
	t.Setenv("CLAUDE_PLUGIN_ROOT", "")
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
}

// proyectoConPedido arma un proyecto Go con memoria real, la nota del TLS y el pedido del TLS
// guardado en la sesión, como lo deja el hook del turno.
func proyectoConPedido(t *testing.T, sesion string) string {
	t.Helper()
	dir := crearGoProject(t)
	memtest.Sembrar(t, dir)
	eng, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	if err := eng.SaveObservation("tls", "prueba/tls", notaDelTLS, nil); err != nil {
		t.Fatal(err)
	}
	recordarPedido(eng, sesion, pedidoDelTLS, time.Now())
	return dir
}

// metaDe lee una clave de meta de la base del proyecto, abriéndola y cerrándola.
func metaDe(t *testing.T, dir, key string) (string, bool) {
	t.Helper()
	eng, err := memory.NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()
	v, ok, _ := eng.GetMeta(key)
	return v, ok && v != ""
}

// TestCompactarLimpiaElDeltaAunqueNoHayaPedidos: sin pedidos guardados la compactación calla, pero
// el delta de ESA sesión queda vacío igual —el resumen se llevó lo inyectado, y el próximo turno
// tiene que traerlo—, y el de otra sesión sigue entero.
//
// Sabotaje: la compactación no vacía el delta.
// arnes: archivo="cmd/musubi/detect_compactar.go"
// arnes: de="\tclearDeltaState(store, sessionID)\n"
// arnes: a="\t_ = sessionID\n"
func TestCompactarLimpiaElDeltaAunqueNoHayaPedidos(t *testing.T) {
	store := newFakeTurnStore()
	saveDeltaState(store, "S", map[string]string{"a": "h1", "b": "h2"}, true)
	saveDeltaState(store, "T", map[string]string{"c": "h3"}, true)

	if got := buildHookOutputDeCompactacion(store, deltaLoop(), config.Default().Memory, "S", "", nil); got != "" {
		t.Errorf("sin pedidos guardados la compactación tiene que callar; salió %q", got)
	}
	if d := loadDeltaState(store, "S"); len(d) != 0 {
		t.Errorf("la compactación dejó el delta de la sesión: %v", d)
	}
	if d := loadDeltaState(store, "T"); len(d) != 1 {
		t.Errorf("la compactación de S tocó el delta de T: %v", d)
	}
}

// TestCompactarTraeMemoriaYNoRepiteLosPedidos: con el motor real, el turno trae la nota del TLS y
// el mismo pedido otra vez ya no (CONTROL: el delta la esconde, que es lo que la compactación
// tiene que deshacer). Compactar la devuelve bajo SessionStart, sin el texto del pedido, la cobra en
// el ledger como compact_recall y deja el delta sembrado: el turno siguiente no la repite.
//
// Sabotaje: el bloque vuelve a mostrar el pedido.
// arnes: archivo="cmd/musubi/detect_compactar.go"
// arnes: de="\t\t{surface: surfaceCompactRecall, text: bloque},\n"
// arnes: a="\t\t{surface: surfaceCompactRecall, text: \"Pediste: \" + ultimo + \"\\n\" + bloque},\n"
func TestCompactarTraeMemoriaYNoRepiteLosPedidos(t *testing.T) {
	eng := motorConCorpusDeRuido(t)
	if got := turnoReal(t, eng, "S", pedidoDelTLS); !strings.Contains(got, "[id:tls]") {
		t.Fatalf("control: el turno tenía que traer la nota del TLS; salió %q", got)
	}
	if got := turnoReal(t, eng, "S", pedidoDelTLS); strings.Contains(got, "[id:tls]") {
		t.Fatalf("control: el delta tenía que esconder la nota ya inyectada; salió %q", got)
	}

	loopCfg := config.LoopConfig{PerTurnRecall: true, RecallBudget: 400, DeltaInjection: true}
	out := buildHookOutputDeCompactacion(eng, loopCfg, config.Default().Memory, "S", "", nil)
	evento, ctx := hookAdditionalContext(t, out)
	if evento != "SessionStart" {
		t.Fatalf("la memoria tras compactar tiene que salir bajo SessionStart; salió %q en %q", evento, out)
	}
	if !strings.Contains(ctx, "[id:tls]") {
		t.Fatalf("la compactación no devolvió la nota que el delta escondía: %q", ctx)
	}
	if strings.Contains(ctx, pedidoDelTLS) {
		t.Errorf("el bloque repite el pedido, que el resumen ya trae: %q", ctx)
	}
	if _, ok := loadDeltaState(eng, "S")["tls"]; !ok {
		t.Errorf("la compactación no sembró el delta con lo que devolvió: %v", loadDeltaState(eng, "S"))
	}
	led, err := eng.LedgerStatusDe("S")
	if err != nil {
		t.Fatal(err)
	}
	if led.Surfaces[surfaceCompactRecall] <= 0 {
		t.Errorf("el bloque no se cobró como %s: %v", surfaceCompactRecall, led.Surfaces)
	}
	if got := turnoReal(t, eng, "S", pedidoDelTLS); strings.Contains(got, "[id:tls]") {
		t.Errorf("el turno después de compactar repitió lo que la compactación ya devolvió: %q", got)
	}
}

// TestCompactarRespetaElRecallPorTurnoApagado: con `loop.per_turn_recall` apagado el turno no trae
// memoria, y la compactación tampoco: vacía el delta y calla. CONTROL: con el recall prendido, la
// misma compactación devuelve la nota.
//
// Sabotaje: la compactación ignora per_turn_recall.
// arnes: archivo="cmd/musubi/detect_compactar.go"
// arnes: de="\tif !loopCfg.PerTurnRecall {\n"
// arnes: a="\tif false {\n"
func TestCompactarRespetaElRecallPorTurnoApagado(t *testing.T) {
	eng := motorConCorpusDeRuido(t)
	if got := turnoReal(t, eng, "S", pedidoDelTLS); !strings.Contains(got, "[id:tls]") {
		t.Fatalf("control: el turno tenía que traer la nota del TLS; salió %q", got)
	}

	apagado := config.LoopConfig{PerTurnRecall: false, RecallBudget: 400, DeltaInjection: true}
	if out := buildHookOutputDeCompactacion(eng, apagado, config.Default().Memory, "S", "", nil); out != "" {
		t.Errorf("con per_turn_recall apagado la compactación trajo memoria: %q", out)
	}
	if d := loadDeltaState(eng, "S"); len(d) != 0 {
		t.Errorf("con el recall apagado la compactación igual tiene que vaciar el delta: %v", d)
	}

	prendido := apagado
	prendido.PerTurnRecall = true
	if out := buildHookOutputDeCompactacion(eng, prendido, config.Default().Memory, "S", "", nil); !strings.Contains(out, "[id:tls]") {
		t.Errorf("control: con el recall prendido la compactación tenía que devolver la nota; salió %q", out)
	}
}

// TestCompactarOlvidaSoloLasMarcasDeEsaSesion: las marcas «ya avisado en esta sesión» de la fase, el
// lote y los conflictos se van con la conversación de S; las de T se quedan, con su orden.
//
// Sabotaje: la compactación no olvida las marcas.
// arnes: archivo="cmd/musubi/detect_compactar.go"
// arnes: de="\t\tolvidarMarcaDeSesion(store, key, sessionID)\n"
// arnes: a="\t\t_ = key\n"
//
// Sabotaje: olvidar la marca de S borra las de todas las sesiones.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tdelete(m.Valor, sessionID)\n"
// arnes: a="\tclear(m.Valor)\n"
func TestCompactarOlvidaSoloLasMarcasDeEsaSesion(t *testing.T) {
	store := newFakeTurnStore()
	for _, key := range marcasQueOlvidaLaCompactacion {
		marcarUnaVezPorSesion(store, key, "S", "v-S")
		marcarUnaVezPorSesion(store, key, "T", "v-T")
	}

	buildHookOutputDeCompactacion(store, deltaLoop(), config.Default().Memory, "S", "", nil)

	for _, key := range marcasQueOlvidaLaCompactacion {
		m := leerMarcasPorSesion(store, key)
		if _, ok := m.Valor["S"]; ok || slices.Contains(m.Orden, "S") {
			t.Errorf("%s: la compactación de S no olvidó su marca: %+v", key, m)
		}
		if m.Valor["T"] != "v-T" || !slices.Contains(m.Orden, "T") {
			t.Errorf("%s: la compactación de S tocó la marca de T: %+v", key, m)
		}
	}
}

// TestCompactarNoMuestraLaCorreccionDeTipeo: decisión 2 del dueño. La consulta tras compactar sale
// corregida, pero sin la línea «busqué … por …», que es del turno, para quien acaba de tipear. El
// CONTROL es el mismo recall del turno con el mismo pedido: ahí la línea sí sale.
//
// Sabotaje: la compactación vuelve a pedir el aviso de corrección.
// arnes: archivo="cmd/musubi/detect_compactar.go"
// arnes: de="\t\tsinAvisoDeCorreccion: true,\n"
// arnes: a="\t\tsinAvisoDeCorreccion: false,\n"
//
// Sabotaje: el recall del turno ignora el pedido de callar la corrección.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tif p.sinAvisoDeCorreccion {\n"
// arnes: a="\tif false && p.sinAvisoDeCorreccion {\n"
func TestCompactarNoMuestraLaCorreccionDeTipeo(t *testing.T) {
	memCfg := config.Default().Memory
	control := &fakeTurnStore{recall: itemDelPi(), corregir: corrigeDosTipeos, meta: map[string]string{}}
	if got := buildTurnRecall(control, parametrosDelTurno{sesion: "T", prompt: promptConTipeos, presupuesto: 250, memCfg: memCfg}); !strings.Contains(got, transcripts.PrefijoDeCorreccion) {
		t.Fatalf("control: el turno tenía que avisar la corrección; salió %q", got)
	}

	store := &fakeTurnStore{recall: itemDelPi(), corregir: corrigeDosTipeos, meta: map[string]string{}}
	recordarPedido(store, "S", promptConTipeos, time.Now())
	out := buildHookOutputDeCompactacion(store, deltaLoop(), memCfg, "S", "", nil)
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "[id:x1]") {
		t.Fatalf("la compactación no devolvió memoria: %q", out)
	}
	if strings.Contains(ctx, transcripts.PrefijoDeCorreccion) {
		t.Errorf("el bloque tras compactar trae la línea de corrección: %q", ctx)
	}
	if store.lastQuery != "la informacion del comando" {
		t.Errorf("la consulta tras compactar no salió corregida: %q", store.lastQuery)
	}
}

// TestCompactarNoCorreElArranqueEntero: con source «compact» el hook devuelve la memoria del
// último pedido y NADA del arranque: ni la generación de skills (y no escribe la huella del stack),
// ni el bloque cognitivo, ni la captura proactiva, ni el priming, ni el de salud. El CONTROL es el
// arranque sobre el mismo proyecto, que sí ofrece las skills y escribe la huella.
//
// Sabotaje: la compactación va por el arranque.
// arnes: archivo="cmd/musubi/detect.go"
// arnes: de="\tif in.esCompactacion() {\n"
// arnes: a="\tif false && in.esCompactacion() {\n"
// arnes: colision_ok="TestCompactarNoRefrescaLosManuales"
func TestCompactarNoCorreElArranqueEntero(t *testing.T) {
	aislarDelPluginInstalado(t)
	dir := proyectoConPedido(t, "S")
	t.Setenv("CLAUDE_PROJECT_DIR", dir)

	out, err := detectOutputSegunFuente(dir, true, entradaDeArranque{SessionID: "S", Source: fuenteCompact})
	if err != nil {
		t.Fatal(err)
	}
	_, ctx := hookAdditionalContext(t, out)
	if !strings.Contains(ctx, "[id:tls]") {
		t.Fatalf("la compactación no devolvió la memoria del último pedido: %q", out)
	}
	for _, delArranque := range []string{"musubi_save_skill", "autoconocimiento", "captura proactiva", "[Musubi — memoria]", "[Musubi — salud]"} {
		if strings.Contains(ctx, delArranque) {
			t.Errorf("la compactación corrió el arranque: trae %q", delArranque)
		}
	}
	if _, ok := metaDe(t, dir, memory.MetaStackFingerprint); ok {
		t.Error("la compactación escribió la huella del stack: corrió la generación de skills")
	}

	// CONTROL: el arranque del mismo proyecto sí ofrece las skills y deja la huella.
	out, err = detectOutputSegunFuente(dir, true, entradaDeArranque{SessionID: "S2", Source: fuenteStartup})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "musubi_save_skill") {
		t.Errorf("control: el arranque tenía que ofrecer las skills; salió %q", out)
	}
	if _, ok := metaDe(t, dir, memory.MetaStackFingerprint); !ok {
		t.Error("control: el arranque tenía que dejar la huella del stack")
	}
}

// TestCompactarNoRefrescaLosManuales: la compactación tampoco pone al día los manuales, que es del
// arranque (ver skills_frescas.go). TestCompactarNoCorreElArranqueEntero no lo ve: su proyecto no
// tiene .musubi/skills/, y sin esa carpeta el refresco no hace nada. Acá la tiene, y el CONTROL es el
// arranque del mismo proyecto, que sí deja la huella de los manuales.
//
// Sabotaje: el hook refresca los manuales antes de despachar la compactación.
// arnes: archivo="cmd/musubi/detect.go"
// arnes: de="\tif in.esCompactacion() {\n"
// arnes: a="\tif eng, err := memory.NewDbEngine(root); err == nil {\n\t\tst, _ := detector.DetectStack(root)\n\t\t_, _ = refrescarSkillsSiHaceFalta(root, eng, st)\n\t\t_ = eng.Close()\n\t}\n\tif in.esCompactacion() {\n"
//
// Sabotaje: la rama de la compactación refresca los manuales.
// arnes: archivo="cmd/musubi/detect_compactar.go"
// arnes: de="\tdefer engine.Close()\n\t// El proyecto propio y el embebedor salen"
// arnes: a="\tdefer engine.Close()\n\t_, _ = refrescarSkillsSiHaceFalta(root, engine, nil)\n\t// El proyecto propio y el embebedor salen"
func TestCompactarNoRefrescaLosManuales(t *testing.T) {
	aislarDelPluginInstalado(t)
	dir := proyectoConPedido(t, "S")
	if err := os.MkdirAll(filepath.Join(dir, config.DirName, config.SkillsDir), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_PROJECT_DIR", dir)

	out, err := detectOutputSegunFuente(dir, true, entradaDeArranque{SessionID: "S", Source: fuenteCompact})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "[id:tls]") {
		t.Fatalf("la compactación no devolvió la memoria: %q", out)
	}
	if v, ok := metaDe(t, dir, metaHuellaSkills); ok {
		t.Errorf("la compactación refrescó los manuales: dejó la huella %q", v)
	}
	if n := len(skillMDs(t, dir)); n != 0 {
		t.Errorf("la compactación exportó %d manuales a .claude/skills", n)
	}

	// CONTROL: el arranque del mismo proyecto sí los refresca.
	if _, err := detectOutputSegunFuente(dir, true, entradaDeArranque{SessionID: "S2", Source: fuenteStartup}); err != nil {
		t.Fatal(err)
	}
	if _, ok := metaDe(t, dir, metaHuellaSkills); !ok {
		t.Error("control: el arranque tenía que refrescar los manuales y dejar la huella")
	}
}

// TestClearVaPorElArranque: /clear abre una sesión nueva, así que va por el arranque entero —acá, la
// oferta de skills y la huella—, no por la rama de la compactación.
//
// Sabotaje: «clear» se trata como una compactación.
// arnes: archivo="cmd/musubi/detect.go"
// arnes: de="\treturn e.Source == fuenteCompact\n"
// arnes: a="\treturn e.Source == fuenteCompact || e.Source == fuenteClear\n"
func TestClearVaPorElArranque(t *testing.T) {
	aislarDelPluginInstalado(t)
	dir := proyectoConPedido(t, "S")
	t.Setenv("CLAUDE_PROJECT_DIR", dir)

	out, err := detectOutputSegunFuente(dir, true, entradaDeArranque{SessionID: "S", Source: fuenteClear})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "musubi_save_skill") {
		t.Errorf("«clear» no fue por el arranque: %q", out)
	}
	if _, ok := metaDe(t, dir, memory.MetaStackFingerprint); !ok {
		t.Error("«clear» no dejó la huella del stack: no corrió el arranque")
	}
	if got := leerEntradaDeArranque(strings.NewReader(`{"session_id":" S ","source":" clear "}`)); got.SessionID != "S" || got.Source != fuenteClear || got.esCompactacion() {
		t.Errorf("la entrada del hook se leyó mal: %+v", got)
	}
}

// TestSetupInstalaCompactYClear: setup y el plugin atan el hook de arranque a startup, compact y
// clear, con el mismo comando, y a ningún otro source (resume y fork son otro PR). Y lo que escribe
// setup es lo que la cobertura reconoce: el escritor y el lector no se separan.
//
// Sabotaje: setup deja de instalar «compact».
// arnes: archivo="cmd/musubi/setup.go"
// arnes: de="var fuentesDelArranque = []string{fuenteStartup, fuenteCompact, fuenteClear}\n"
// arnes: a="var fuentesDelArranque = []string{fuenteStartup, fuenteClear}\n"
//
// Sabotaje: el plugin deja de instalar «compact».
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\t{\"SessionStart\", \"compact\", \"detect --hook-mode\"},\n"
// arnes: a=""
func TestSetupInstalaCompactYClear(t *testing.T) {
	root := t.TempDir()
	if err := writeClaudeHook(root, "/opt/musubi/musubi"); err != nil {
		t.Fatal(err)
	}
	deSetup, err := os.ReadFile(filepath.Join(root, config.ClaudeDir, config.ClaudeSettingsFile))
	if err != nil {
		t.Fatal(err)
	}
	delPlugin, err := ganchosDelPlugin("/opt/musubi/musubi")
	if err != nil {
		t.Fatal(err)
	}
	for nombre, crudo := range map[string][]byte{"setup": deSetup, "plugin": delPlugin} {
		ganchos := ganchosDeUnSettings(t, crudo)
		for _, fuente := range []string{"startup", "compact", "clear"} {
			if !slices.Contains(ganchos, "SessionStart|"+fuente+"|detect --hook-mode") {
				t.Errorf("%s no ata el arranque a «%s»: %v", nombre, fuente, ganchos)
			}
		}
		for _, g := range ganchos {
			if strings.HasPrefix(g, "SessionStart|resume|") || strings.HasPrefix(g, "SessionStart|fork|") {
				t.Errorf("%s instaló un matcher que no es de este PR: %s", nombre, g)
			}
		}
	}
	if !elProyectoYaTieneElArranque(root, fuenteCompact) {
		t.Error("lo que escribe setup no cuenta como cobertura de la compactación")
	}
}

// TestElPluginNoCedeCompactSiElProyectoSoloTieneStartup: un repo cableado con el setup viejo
// (SessionStart sólo con «startup») y el plugin nuevo. El plugin cede el arranque, que el repo
// corre, pero NO la compactación, que el repo no corre: si cediera por encontrar «detect
// --hook-mode» en el settings, nadie devolvería la memoria. Se mide sobre el proceso, como lo lanza
// el plugin.
//
// Sabotaje: el plugin cede con sólo encontrar el comando en el settings.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\treturn corriendoComoPlugin() && elProyectoYaTieneElArranque(carpetaDeLaSesion(), fuente)\n"
// arnes: a="\treturn elPluginCedeElGancho(\"detect\")\n"
func TestElPluginNoCedeCompactSiElProyectoSoloTieneStartup(t *testing.T) {
	home := t.TempDir()
	repo := proyectoConMemoria(t, home)
	memtest.Sembrar(t, repo)
	eng, err := memory.NewDbEngine(repo)
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.SaveObservation("tls", "prueba/tls", notaDelTLS, nil); err != nil {
		t.Fatal(err)
	}
	recordarPedido(eng, "S", pedidoDelTLS, time.Now())
	eng.Close()
	if err := os.MkdirAll(filepath.Join(repo, config.ClaudeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	soloStartup := `{"hooks":{"SessionStart":[{"matcher":"startup","hooks":[{"type":"command","command":"musubi detect --hook-mode","timeout":10}]}]}}`
	if err := os.WriteFile(filepath.Join(repo, config.ClaudeDir, config.ClaudeSettingsFile), []byte(soloStartup), 0o644); err != nil {
		t.Fatal(err)
	}
	comoPlugin := []string{"CLAUDE_PLUGIN_ROOT=" + filepath.Join(home, ".claude", "skills", "musubi"), "CLAUDE_CONFIG_DIR=" + filepath.Join(home, ".claude")}

	_, ctx := hookAdditionalContext(t, correrMusubiCon(t, repo, home, `{"session_id":"S","source":"compact"}`, comoPlugin, "detect", "--hook-mode"))
	if !strings.Contains(ctx, "[id:tls]") {
		t.Errorf("el plugin cedió la compactación a un repo que no la corre: salió %q", ctx)
	}
	if out := correrMusubiCon(t, repo, home, `{"session_id":"S3","source":"startup"}`, comoPlugin, "detect", "--hook-mode"); out != "" {
		t.Errorf("control: el plugin tenía que ceder el arranque que el repo ya corre; salió %q", out)
	}
}

// TestAgenteEstadoDiceSiLaCompactacionEstaCubierta: `musubi agente estado` dice si el repo devuelve
// la memoria después de compactar. Con el setup viejo (sólo «startup») y sin plugin sale SIN CUBRIR;
// el mismo repo con el settings que escribe setup sale cubierto. Se mide sobre el proceso: la línea
// la imprime runAgente.
//
// Sabotaje: estado no dice nada de la compactación.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\t\tfmt.Println(estadoDeLaCompactacion(carpetaDeLaSesion(), dir))\n"
// arnes: a="\t\t_ = estadoDeLaCompactacion\n"
func TestAgenteEstadoDiceSiLaCompactacionEstaCubierta(t *testing.T) {
	home := t.TempDir()
	repo := t.TempDir()
	plugin := filepath.Join(home, ".claude", "skills", "musubi")
	sinPlugin := []string{"CLAUDE_CONFIG_DIR=" + filepath.Join(home, ".claude")}
	if err := os.MkdirAll(filepath.Join(repo, config.ClaudeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	soloStartup := `{"hooks":{"SessionStart":[{"matcher":"startup","hooks":[{"type":"command","command":"musubi detect --hook-mode","timeout":10}]}]}}`
	if err := os.WriteFile(filepath.Join(repo, config.ClaudeDir, config.ClaudeSettingsFile), []byte(soloStartup), 0o644); err != nil {
		t.Fatal(err)
	}

	if out := correrMusubiCon(t, repo, home, "", sinPlugin, "agente", "estado", "--dir", plugin); !strings.Contains(out, "Compactación: SIN CUBRIR") {
		t.Errorf("estado no avisa que el repo quedó sin el hook de la compactación:\n%s", out)
	}
	if err := writeClaudeHook(repo, "/opt/musubi/musubi"); err != nil {
		t.Fatal(err)
	}
	if out := correrMusubiCon(t, repo, home, "", sinPlugin, "agente", "estado", "--dir", plugin); !strings.Contains(out, "Compactación: cubierta en") {
		t.Errorf("estado no reconoce el hook de la compactación que escribió setup:\n%s", out)
	}
}

// TestElAvisoSinCompactSaleUnaSolaVez: un repo sin el hook de la compactación lo oye en el arranque
// UNA vez —la marca vive en la base del proyecto—; uno cubierto no lo oye ni gasta la marca.
//
// Sabotaje: el aviso ignora la marca y sale en cada arranque.
// arnes: archivo="cmd/musubi/detect_compactar.go"
// arnes: de="\tif v, ok, _ := store.GetMeta(metaAvisoSinCompact); ok && v != \"\" {\n"
// arnes: a="\tif v, ok, _ := store.GetMeta(metaAvisoSinCompact); ok && v != \"\" && false {\n"
//
// Sabotaje: el arranque no pide el aviso.
// arnes: archivo="cmd/musubi/detect.go"
// arnes: de="\taviso := buildAvisoSinCompact(store, root, carpetaDeLaSesion())\n"
// arnes: a="\taviso := \"\"\n"
//
// Sabotaje: el plugin instalado no cuenta como cobertura.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\treturn elPluginEnCubre(dirDelPluginInstalado(), fuenteCompact)\n"
// arnes: a="\treturn false\n"
func TestElAvisoSinCompactSaleUnaSolaVez(t *testing.T) {
	aislarDelPluginInstalado(t)
	dir := proyectoConPedido(t, "S")
	t.Setenv("CLAUDE_PROJECT_DIR", dir)

	primero, err := detectOutputSegunFuente(dir, true, entradaDeArranque{SessionID: "A", Source: fuenteStartup})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(primero, "SessionStart «compact»") {
		t.Fatalf("el primer arranque de un repo sin el hook de compactación no avisó: %q", primero)
	}
	segundo, err := detectOutputSegunFuente(dir, true, entradaDeArranque{SessionID: "B", Source: fuenteStartup})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(segundo, "SessionStart «compact»") {
		t.Errorf("el aviso salió dos veces en el mismo proyecto: %q", segundo)
	}

	// Un repo cubierto por su settings: sin aviso y sin marca.
	cubierto := proyectoConPedido(t, "S")
	t.Setenv("CLAUDE_PROJECT_DIR", cubierto)
	if err := writeClaudeHook(cubierto, "/opt/musubi/musubi"); err != nil {
		t.Fatal(err)
	}
	out, err := detectOutputSegunFuente(cubierto, true, entradaDeArranque{SessionID: "A", Source: fuenteStartup})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "SessionStart «compact»") {
		t.Errorf("avisó en un repo que tiene el hook de compactación: %q", out)
	}
	if _, ok := metaDe(t, cubierto, metaAvisoSinCompact); ok {
		t.Error("un repo cubierto gastó la marca del aviso")
	}

	// Un repo sin nada propio, pero con el plugin nuevo instalado: el plugin lo cubre.
	plugin := t.TempDir()
	ganchos, err := ganchosDelPlugin("/opt/musubi/musubi")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(plugin, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, "hooks", "hooks.json"), ganchos, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(plugin, marcaDelPlugin), []byte("version: prueba\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CLAUDE_PLUGIN_ROOT", plugin)
	conPlugin := proyectoConPedido(t, "S")
	t.Setenv("CLAUDE_PROJECT_DIR", conPlugin)
	out, err = detectOutputSegunFuente(conPlugin, true, entradaDeArranque{SessionID: "A", Source: fuenteStartup})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "SessionStart «compact»") {
		t.Errorf("avisó en un repo que el plugin instalado cubre: %q", out)
	}
}

// TestLaCoberturaSigueLaReglaDelMatcher: el matcher se lee con la regla que Claude Code aplica a
// SessionStart —vacío o «*» es todo; una lista con «|» o «,», con o sin espacios alrededor, es
// igualdad con cada valor; lo demás es una expresión regular—, y el settings.local cuenta igual que el
// settings: Claude Code junta los dos. La regla de la lista está leída del binario de Claude Code
// 2.1.284: para SessionStart acepta ^[a-zA-Z0-9_|, -]+$, parte por [|,] y le saca los espacios a cada
// valor.
//
// Sabotaje: la lista vuelve a ser sólo letras, dígitos, «_» y «|», la regla de los otros eventos.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="var matcherLista = regexp.MustCompile(`^[a-zA-Z0-9_|, -]+$`)\n"
// arnes: a="var matcherLista = regexp.MustCompile(`^[a-zA-Z0-9_|]+$`)\n"
// arnes: colision_ok="TestElPluginCedeConListaConComa"
//
// Sabotaje: la lista se parte sólo por «|».
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\t\tfor _, m := range separadorDeLista.Split(matcher, -1) {\n"
// arnes: a="\t\tfor _, m := range strings.Split(matcher, \"|\") {\n"
//
// Sabotaje: los valores de la lista no pierden los espacios de los bordes.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\t\t\tif strings.TrimSpace(m) == valor {\n"
// arnes: a="\t\t\tif m == valor {\n"
//
// Sabotaje: el matcher que es una expresión regular no cubre nada.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="\treturn re.MatchString(valor)\n"
// arnes: a="\t_ = re\n\treturn false\n"
//
// Sabotaje: el settings.local no se mira.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="func elProyectoYaTieneElArranque(dir, fuente string) bool {\n\tfor _, f := range []string{config.ClaudeSettingsFile, \"settings.local.json\"} {\n"
// arnes: a="func elProyectoYaTieneElArranque(dir, fuente string) bool {\n\tfor _, f := range []string{config.ClaudeSettingsFile} {\n"
func TestLaCoberturaSigueLaReglaDelMatcher(t *testing.T) {
	casos := []struct {
		matcher, fuente string
		cubre           bool
	}{
		{"", "compact", true},
		{"*", "compact", true},
		{"startup", "compact", false},
		{"startup", "startup", true},
		{"startup|compact", "compact", true},
		{"compactar", "compact", false},
		{"comp", "compact", false},
		{"comp.*", "compact", true},
		{"(", "compact", false},
		// Con «-» sigue siendo una lista, y una lista no es una expresión regular: «comp» no cubre
		// «compact».
		{"comp|start-up", "compact", false},
		{"compact ", "compact", true},
		{"startup,compact", "compact", true},
		{"startup, compact", "compact", true},
		{"startup | compact", "compact", true},
		{"startup, clear", "startup", true},
		{"startup, clear", "compact", false},
		// El espacio no separa: «startup compact» es un solo valor.
		{"startup compact", "compact", false},
	}
	for _, c := range casos {
		if got := matcherCubre(c.matcher, c.fuente); got != c.cubre {
			t.Errorf("matcherCubre(%q, %q) = %v, quería %v", c.matcher, c.fuente, got, c.cubre)
		}
	}

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, config.ClaudeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	soloStartup := `{"hooks":{"SessionStart":[{"matcher":"startup","hooks":[{"type":"command","command":"musubi detect --hook-mode"}]}]}}`
	conCompact := `{"hooks":{"SessionStart":[{"matcher":"compact","hooks":[{"type":"command","command":"musubi detect --hook-mode"}]}]}}`
	if err := os.WriteFile(filepath.Join(repo, config.ClaudeDir, config.ClaudeSettingsFile), []byte(soloStartup), 0o644); err != nil {
		t.Fatal(err)
	}
	if elProyectoYaTieneElArranque(repo, fuenteCompact) {
		t.Fatal("un settings sólo con «startup» no cubre la compactación")
	}
	if !elProyectoYaTieneElArranque(repo, "") {
		t.Error("un evento sin source va por el arranque: lo cubre «startup»")
	}
	if err := os.WriteFile(filepath.Join(repo, config.ClaudeDir, "settings.local.json"), []byte(conCompact), 0o644); err != nil {
		t.Fatal(err)
	}
	if !elProyectoYaTieneElArranque(repo, fuenteCompact) {
		t.Error("el settings.local con «compact» no cuenta: Claude Code lo junta con el settings")
	}
}

// settingsConArranque arma un proyecto cuyo .claude/settings.json corre el hook de arranque de
// Musubi con ese matcher.
func settingsConArranque(t *testing.T, matcher string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, config.ClaudeDir), 0o755); err != nil {
		t.Fatal(err)
	}
	s := `{"hooks":{"SessionStart":[{"matcher":"` + matcher + `","hooks":[{"type":"command","command":"musubi detect --hook-mode","timeout":10}]}]}}`
	if err := os.WriteFile(filepath.Join(dir, config.ClaudeDir, config.ClaudeSettingsFile), []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestElPluginCedeConListaConComa: un proyecto con el matcher escrito a mano como lista con coma
// hace que Claude Code corra SU arranque para esas fuentes, así que el plugin cede, o hay dos bloques.
// Con «startup, clear» es además la conducta de antes de este cambio: el plugin cedía el arranque si
// el proyecto nombraba `detect --hook-mode`, sin mirar el matcher.
//
// Sabotaje: la lista vuelve a ser sólo letras, dígitos, «_» y «|», la regla de los otros eventos.
// arnes: archivo="cmd/musubi/agente_plugin.go"
// arnes: de="var matcherLista = regexp.MustCompile(`^[a-zA-Z0-9_|, -]+$`)\n"
// arnes: a="var matcherLista = regexp.MustCompile(`^[a-zA-Z0-9_|]+$`)\n"
// arnes: colision_ok="TestLaCoberturaSigueLaReglaDelMatcher"
func TestElPluginCedeConListaConComa(t *testing.T) {
	t.Setenv("CLAUDE_PLUGIN_ROOT", t.TempDir())
	t.Setenv("CLAUDE_PROJECT_DIR", settingsConArranque(t, "startup, compact"))
	if !elPluginCedeElArranque(fuenteCompact) {
		t.Error("«startup, compact»: el plugin no cede la compactación y Claude Code corre también la del proyecto: dos bloques tras compactar")
	}
	if !elPluginCedeElArranque(fuenteStartup) {
		t.Error("«startup, compact»: el plugin no cede el arranque: dos primings")
	}
	t.Setenv("CLAUDE_PROJECT_DIR", settingsConArranque(t, "startup, clear"))
	if !elPluginCedeElArranque(fuenteStartup) {
		t.Error("«startup, clear»: el plugin no cede el arranque: dos primings")
	}
	if elPluginCedeElArranque(fuenteCompact) {
		t.Error("«startup, clear»: el plugin cede la compactación, que el proyecto no corre: nadie devuelve la memoria")
	}
}
