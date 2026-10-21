package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/repovivo"
)

// TestOpcionesDeConfigCablea fija el CABLEADO config → Opciones de la zona en vivo: cada campo que el
// daemon le pasa a repovivo.Arrancar sale de donde debe, SIN hardcode ni transformación. El fixture usa
// valores CENTINELA que no coinciden con ningún literal que un hardcode plausible clavaría ("auto"/"on"/
// "off" para Modo, "central"/"origin" para Remoto): así un mapeo hardcodeado se detecta CUALQUIERA sea el
// literal que fije —no sólo cuando difiere del fixture—. Sin eso, un Modo:"auto" hardcodeado sobrevivía
// verde porque el fixture también decía "auto" (el hueco exacto que encontró la revisión adversarial).
// opcionesDeConfig es una función pura que copia tal cual, así que un valor imposible es legítimo acá:
// lo que se prueba es que NO hay hardcode ni transformación. Es la mitad determinista del cableado;
// TestRunRepoVivoEnciendeSegunConfigConSyncApagado cubre el comportamiento end-to-end.
func TestOpcionesDeConfigCablea(t *testing.T) {
	cfg := config.RepoVivoConfig{
		Enabled:           "centinela-modo",
		Remote:            "centinela-remoto",
		IntervaloSegundos: 7,
		SecretosExtra:     []string{"secretos/", "*.zzsecreto"},
	}
	got := opcionesDeConfig("/ruta/proyecto", true, cfg)
	esperado := repovivo.Opciones{
		Dir:           "/ruta/proyecto",
		Remoto:        "centinela-remoto",
		Modo:          "centinela-modo",
		TeamMode:      true,
		Intervalo:     7 * time.Second,
		SecretosExtra: []string{"secretos/", "*.zzsecreto"},
	}
	if !reflect.DeepEqual(got, esperado) {
		t.Errorf("opcionesDeConfig cableó mal:\n got=%+v\n esp=%+v", got, esperado)
	}
	// team_mode false se propaga tal cual: ni siempre-true ni siempre-false.
	if o := opcionesDeConfig("/x", false, cfg); o.TeamMode {
		t.Error("TeamMode=false debía propagarse como false (no hardcodeado true)")
	}
}

// TestRunRepoVivoEnciendeSegunConfigConSyncApagado cubre el COMPORTAMIENTO del cableado de punta a punta,
// con el sync de memoria APAGADO en los cuatro casos (syncCfg.Enabled=false): esa es la condición
// adversarial —la zona en vivo NO debe depender del sync—. La matriz auto/off/on × team_mode mata las
// mutaciones que el DeepEqual de TestOpcionesDeConfigCablea no puede ver porque viven en el CALL-SITE (lo
// que RunRepoVivo le pasa a opcionesDeConfig) o en RunRepoVivo mismo:
//   - off+team NO enciende  → mata un Modo hardcodeado a "auto"/"on" (encendería) y fija el kill-switch.
//   - auto sin team NO enciende; auto+team(remoto empresarial) SÍ → mata un TeamMode hardcodeado.
//   - auto+team usa un remoto ("empresa") distinto del de los otros casos → mata un Remoto hardcodeado.
//   - cada caso que publica trackea un secreto-extra (cred.zzsecreto, patrón NO-default) y comprueba que
//     NO esté en el árbol de la foto → mata un SecretosExtra que se pierda en el call-site.
//   - un sync-gate (if !s.syncCfg.Enabled {return}) en RunRepoVivo pondría rojos los dos casos positivos.
//
// NO cubre los call-sites de startRepoVivo en cmd/musubi (viven en package main; arrancar runServe o
// runDaemon para medirlo sería desproporcionado). Esa invariante la sostienen el gate único de
// startRepoVivo y el comentario de main.go que explica por qué la zona en vivo va aparte del sync.
// Remoto bare local (no github, para que la guarda dura deje pasar).
func TestRunRepoVivoEnciendeSegunConfigConSyncApagado(t *testing.T) {
	casos := []struct {
		nombre     string
		enabled    string
		remoto     string
		teamMode   bool
		quiereFoto bool
	}{
		{"auto_con_team_publica", "auto", "empresa", true, true},
		{"auto_sin_team_no_enciende", "auto", "central", false, false},
		{"off_con_team_no_enciende", "off", "central", true, false},
		{"on_sin_team_publica", "on", "central", false, true},
	}
	for _, c := range casos {
		c := c
		t.Run(c.nombre, func(t *testing.T) {
			dir, bare := t.TempDir(), t.TempDir()
			runGit := func(d string, args ...string) {
				t.Helper()
				cmd := exec.Command("git", args...)
				if d != "" {
					cmd.Dir = d
				}
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("git %v: %v\n%s", args, err, out)
				}
			}
			runGit("", "init", "-q", dir)
			runGit(dir, "config", "user.email", "t@t")
			runGit(dir, "config", "user.name", "t")
			runGit(dir, "config", "commit.gpgsign", "false")
			if err := os.WriteFile(filepath.Join(dir, "app.txt"), []byte("v1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			// Un secreto-extra TRACKEADO con patrón NO-default: de la foto sólo lo saca el SecretosExtra
			// de la config. Si el call-site lo pierde (nil), el blob aterriza en el árbol de la foto.
			if err := os.WriteFile(filepath.Join(dir, "cred.zzsecreto"), []byte("TOKEN\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			runGit(dir, "add", "app.txt", "cred.zzsecreto")
			runGit(dir, "commit", "-qm", "inicial")
			runGit("", "init", "--bare", "-q", bare)
			runGit(dir, "remote", "add", c.remoto, bare)

			// sync APAGADO a propósito en los cuatro casos: la zona en vivo no debe depender de él.
			s := &McpServer{
				projectPath: dir,
				memory:      config.MemoryConfig{TeamMode: c.teamMode},
				syncCfg:     config.SyncConfig{Enabled: false},
			}
			cfg := config.RepoVivoConfig{
				Enabled:           c.enabled,
				Remote:            c.remoto,
				IntervaloSegundos: 1,
				SecretosExtra:     []string{"*.zzsecreto"},
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan struct{})
			go func() { s.RunRepoVivo(ctx, cfg); close(done) }() // BLOQUEA en el bucle: su goroutine, como en el daemon.

			refs := func() string {
				out, _ := exec.Command("git", "--git-dir="+bare, "for-each-ref", "--format=%(refname)", "refs/vivo/").Output()
				return strings.TrimSpace(string(out))
			}

			if !c.quiereFoto {
				// Brazo negativo: "no debe encender" ⇒ RunRepoVivo VUELVE enseguida y no hay ref.
				select {
				case <-done:
				case <-time.After(3 * time.Second):
					t.Fatalf("%s: debía quedar apagado (RunRepoVivo vuelve) y sigue corriendo", c.nombre)
				}
				if r := refs(); r != "" {
					t.Fatalf("%s: publicó %q pese a que no debía encender", c.nombre, r)
				}
				return
			}

			// Brazo positivo: la foto automática crea una ref bajo refs/vivo/. shaEnBare devuelve "" hasta
			// que exista; si RunRepoVivo vuelve antes, es que no encendió o no entró al bucle.
			limite := time.After(10 * time.Second)
			tic := time.NewTicker(50 * time.Millisecond)
			defer tic.Stop()
			for refs() == "" {
				select {
				case <-limite:
					t.Fatalf("%s: con el sync apagado la foto no llegó a refs/vivo/ en 10s: ¿RunRepoVivo gatea en el sync?", c.nombre)
				case <-done:
					t.Fatalf("%s: RunRepoVivo volvió sin publicar (no encendió o no entró al bucle)", c.nombre)
				case <-tic.C:
				}
			}

			// Cerramos el bucle y esperamos a que la goroutine salga ANTES de inspeccionar/limpiar: así no
			// hay una escritura tardía de la foto sobre el dir que t.TempDir está por borrar.
			ref := refs()
			cancel()
			<-done

			// El secreto-extra NO debe estar en el árbol de la foto; app.txt SÍ. Fija que el SecretosExtra
			// de la config llegó hasta la exclusión (mata un call-site que lo anule a nil).
			arbol, err := exec.Command("git", "--git-dir="+bare, "ls-tree", "-r", "--name-only", ref).Output()
			if err != nil {
				t.Fatalf("%s: no pude leer el árbol de %s: %v", c.nombre, ref, err)
			}
			listado := string(arbol)
			if !strings.Contains(listado, "app.txt") {
				t.Fatalf("%s: la foto no incluyó app.txt; árbol=%q", c.nombre, listado)
			}
			if strings.Contains(listado, "cred.zzsecreto") {
				t.Fatalf("%s: el secreto-extra cred.zzsecreto entró a la foto (SecretosExtra no llegó al call-site); árbol=%q", c.nombre, listado)
			}
		})
	}
}
