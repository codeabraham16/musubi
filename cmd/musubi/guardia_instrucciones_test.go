package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// guardiaDia es el «hoy» de las pruebas de la guardia de instrucciones.
var guardiaDia = time.Date(2026, 10, 14, 9, 0, 0, 0, time.Local)

// guardiaEntorno arma un proyecto, una carpeta de configuración de Claude Code y un HOME vacíos,
// aislados de los de la PC que corre la prueba.
func guardiaEntorno(t *testing.T) (root, dirConfig, home string) {
	t.Helper()
	t.Setenv("CLAUDE_CODE_PROJECT_DIR_NAME", "")
	return t.TempDir(), t.TempDir(), t.TempDir()
}

func guardiaEscribir(t *testing.T, ruta, contenido string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(ruta), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
}

// guardiaRenglones arma n renglones ASCII (un byte por carácter), cada uno con su salto.
func guardiaRenglones(n int, salto string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "regla %d%s", i, salto)
	}
	return b.String()
}

func guardiaCorrer(store startupStore, root, dirConfig, home string, cuando time.Time) string {
	return sumarGuardiaDeInstruccionesCon("", store, "S", root, dirConfig, home, cuando)
}

// TestGuardiaAvisaConMasDe200Lineas: 201 líneas avisan y 200 no, ni gastan la marca del día.
//
// Sabotaje: el tope de líneas sube a 300.
// arnes: archivo="cmd/musubi/guardia_instrucciones.go"
// arnes: de="const topeLineasDeInstrucciones = 200\n"
// arnes: a="const topeLineasDeInstrucciones = 300\n"
func TestGuardiaAvisaConMasDe200Lineas(t *testing.T) {
	root, dirConfig, home := guardiaEntorno(t)
	guardiaEscribir(t, filepath.Join(root, "CLAUDE.md"), guardiaRenglones(200, "\n"))
	store := newFakeStore()
	if out := guardiaCorrer(store, root, dirConfig, home, guardiaDia); out != "" {
		t.Errorf("200 líneas justas no pasan el tope y avisó: %s", out)
	}
	if _, ok := store.meta[metaGuardiaDeInstrucciones]; ok {
		t.Error("sin aviso, la guardia gastó la marca del día")
	}
	guardiaEscribir(t, filepath.Join(root, "CLAUDE.md"), guardiaRenglones(201, "\n"))
	if out := guardiaCorrer(store, root, dirConfig, home, guardiaDia); !strings.Contains(out, "CLAUDE.md (201 líneas") {
		t.Errorf("201 líneas pasan el tope y no avisó: %q", out)
	}
}

// TestGuardiaAvisaConMasDe25KB: 25.001 caracteres en una sola línea avisan y 25.000 no. El salto del
// final no cuenta (Claude Code recorta antes de medir).
func TestGuardiaAvisaConMasDe25KB(t *testing.T) {
	root, dirConfig, home := guardiaEntorno(t)
	guardiaEscribir(t, filepath.Join(root, "CLAUDE.md"), strings.Repeat("a", 25000)+"\n")
	if out := guardiaCorrer(newFakeStore(), root, dirConfig, home, guardiaDia); out != "" {
		t.Errorf("25 KB justos no pasan el tope y avisó: %s", out)
	}
	guardiaEscribir(t, filepath.Join(root, "CLAUDE.md"), strings.Repeat("a", 25001)+"\n")
	if out := guardiaCorrer(newFakeStore(), root, dirConfig, home, guardiaDia); !strings.Contains(out, "CLAUDE.md (1 línea, 25,0 KB)") {
		t.Errorf("25 KB + 1 byte pasan el tope y no avisó: %q", out)
	}
}

// TestGuardiaAvisaUnaVezPorDia: el mismo día calla, y al día siguiente vuelve.
func TestGuardiaAvisaUnaVezPorDia(t *testing.T) {
	root, dirConfig, home := guardiaEntorno(t)
	guardiaEscribir(t, filepath.Join(root, "CLAUDE.md"), guardiaRenglones(262, "\n"))
	store := newFakeStore()
	if out := guardiaCorrer(store, root, dirConfig, home, guardiaDia); out == "" {
		t.Fatal("CONTROL: el primer arranque del día tenía que avisar")
	}
	if out := guardiaCorrer(store, root, dirConfig, home, guardiaDia.Add(5*time.Hour)); out != "" {
		t.Errorf("avisó dos veces el mismo día: %s", out)
	}
	if out := guardiaCorrer(store, root, dirConfig, home, guardiaDia.AddDate(0, 0, 1)); out == "" {
		t.Error("al día siguiente el archivo sigue pasando el tope y no volvió a avisar")
	}
}

// TestGuardiaNoCorreEnCompact: con el motor real, la compactación no avisa ni gasta la marca del día,
// y el arranque siguiente sí avisa (CONTROL: sin él, una guardia muerta pasaría en verde).
func TestGuardiaNoCorreEnCompact(t *testing.T) {
	aislarDelPluginInstalado(t)
	t.Setenv("CLAUDE_CODE_PROJECT_DIR_NAME", "")
	dir := proyectoConPedido(t, "S")
	t.Setenv("CLAUDE_PROJECT_DIR", dir)
	guardiaEscribir(t, filepath.Join(dir, "CLAUDE.md"), guardiaRenglones(201, "\n"))

	compactado, err := detectOutputSegunFuente(dir, true, entradaDeArranque{SessionID: "S", Source: fuenteCompact})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(compactado, "guardia de instrucciones") || strings.Contains(compactado, "systemMessage") {
		t.Errorf("la compactación corrió la guardia: %s", compactado)
	}
	if _, ok := metaDe(t, dir, metaGuardiaDeInstrucciones); ok {
		t.Error("la compactación gastó la marca del día")
	}

	arranque, err := detectOutputSegunFuente(dir, true, entradaDeArranque{SessionID: "S", Source: fuenteStartup})
	if err != nil {
		t.Fatal(err)
	}
	var env struct {
		System string `json:"systemMessage"`
		H      struct {
			Ctx string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(arranque), &env); err != nil {
		t.Fatalf("el arranque no salió como json: %v\n%s", err, arranque)
	}
	if !strings.Contains(env.System, "CLAUDE.md (201 líneas") || !strings.Contains(env.H.Ctx, "[Musubi — guardia de instrucciones]") {
		t.Errorf("CONTROL: el arranque tenía que avisar al dueño y al agente: %s", arranque)
	}
}

// TestGuardiaEncuentraLaMemoriaConLaUnidadEnMayusculaYMinuscula: VS Code pasa la letra de unidad en
// minúscula y la CLI en mayúscula; la carpeta se encuentra igual con cualquiera de las dos.
func TestGuardiaEncuentraLaMemoriaConLaUnidadEnMayusculaYMinuscula(t *testing.T) {
	if got := carpetaDeProyecto(`C:\Proyectos\Proyecto menir\altura-erp`); got != "C--Proyectos-Proyecto-menir-altura-erp" {
		t.Errorf("carpetaDeProyecto = %q", got)
	}
	for _, c := range []struct{ enDisco, root string }{
		{"c--Proyectos-Demo", `C:\Proyectos\Demo`},
		{"C--Proyectos-Demo", `c:\Proyectos\Demo`},
	} {
		dirProyectos := filepath.Join(t.TempDir(), "projects")
		guardiaEscribir(t, filepath.Join(dirProyectos, c.enDisco, "memory", "MEMORY.md"), "x")
		guardiaEscribir(t, filepath.Join(dirProyectos, "c--Proyectos-Otro", "memory", "MEMORY.md"), "x")
		got := carpetasDeMemoriaPorNombre(dirProyectos, []string{carpetaDeProyecto(c.root)})
		if len(got) != 1 || filepath.Base(got[0]) != c.enDisco {
			t.Errorf("root %s: buscaba %s y encontró %v", c.root, c.enDisco, got)
		}
	}

	// Y de punta a punta: la carpeta en disco con la letra dada vuelta, y el aviso del corte.
	root, dirConfig, home := guardiaEntorno(t)
	carpeta := carpetaDeProyecto(root)
	if l := carpeta[:1]; strings.ToUpper(l) != l {
		carpeta = strings.ToUpper(l) + carpeta[1:]
	} else {
		carpeta = strings.ToLower(l) + carpeta[1:]
	}
	guardiaEscribir(t, filepath.Join(dirConfig, "projects", carpeta, "memory", "MEMORY.md"), guardiaRenglones(201, "\n"))
	out := guardiaCorrer(newFakeStore(), root, dirConfig, home, guardiaDia)
	if !strings.Contains(out, "MEMORY.md (201 líneas") || !strings.Contains(out, "lo que pasa de ahí no") {
		t.Errorf("no avisó del corte del MEMORY.md en %s: %q", carpeta, out)
	}
}

// TestGuardiaEnUnWorktreeMiraLaMemoriaDelPrincipal: la doc dice que los worktrees y las subcarpetas
// comparten la memoria del repo; la raíz que nombra la carpeta es la del checkout principal.
func TestGuardiaEnUnWorktreeMiraLaMemoriaDelPrincipal(t *testing.T) {
	base := t.TempDir()
	principal := filepath.Join(base, "repo")
	wt := filepath.Join(base, "repo-wt")
	gitdir := filepath.Join(principal, ".git", "worktrees", "repo-wt")
	guardiaEscribir(t, filepath.Join(gitdir, "commondir"), "../..\n")
	guardiaEscribir(t, filepath.Join(wt, ".git"), "gitdir: "+filepath.ToSlash(gitdir)+"\n")
	if got := raizDelRepoPrincipal(wt); got != principal {
		t.Errorf("worktree: raíz %q, quería %q", got, principal)
	}
	if got := raizDelRepoPrincipal(filepath.Join(principal, "cmd", "x")); got != principal {
		t.Errorf("subcarpeta: raíz %q, quería %q", got, principal)
	}
	if got := raizDelRepoPrincipal(base); got != base {
		t.Errorf("fuera de git: raíz %q, quería la misma carpeta", got)
	}
}

// TestGuardiaSumaLoQueImportaElClaudeMd: un núcleo chico que importa uno grande pasa el tope (la doc:
// los @imports no bajan el costo). Lo que está entre comillas invertidas o en un bloque cercado no se
// importa, y un `~/` es el HOME.
func TestGuardiaSumaLoQueImportaElClaudeMd(t *testing.T) {
	root, dirConfig, home := guardiaEntorno(t)
	guardiaEscribir(t, filepath.Join(root, "CLAUDE.md"),
		"# Núcleo\n@docs/area.md\n`@docs/no-se-importa.md`\n```\n@docs/tampoco.md\n```\n")
	guardiaEscribir(t, filepath.Join(root, "docs", "area.md"), guardiaRenglones(160, "\n")+"@~/personal.md\n")
	guardiaEscribir(t, filepath.Join(home, "personal.md"), guardiaRenglones(40, "\n"))
	guardiaEscribir(t, filepath.Join(root, "docs", "no-se-importa.md"), guardiaRenglones(500, "\n"))
	guardiaEscribir(t, filepath.Join(root, "docs", "tampoco.md"), guardiaRenglones(500, "\n"))
	for _, m := range medirInstrucciones(root, dirConfig, home) {
		if m.etiqueta != "CLAUDE.md" {
			continue
		}
		if m.lineas != 6+161+40 || m.importados != 2 {
			t.Errorf("CLAUDE.md con imports: %d líneas y %d importados, quería 207 y 2", m.lineas, m.importados)
		}
		if out := guardiaCorrer(newFakeStore(), root, dirConfig, home, guardiaDia); !strings.Contains(out, "CLAUDE.md con sus @imports (207 líneas") {
			t.Errorf("no avisó del núcleo con imports: %q", out)
		}
		return
	}
	t.Fatal("no midió el CLAUDE.md")
}

// TestGuardiaCuentaIgualConCRLF: un CRLF da las mismas líneas que un LF; el `\r` suma al tamaño, como
// en Claude Code.
func TestGuardiaCuentaIgualConCRLF(t *testing.T) {
	for _, n := range []int{200, 201} {
		lf, _ := medirTexto(guardiaRenglones(n, "\n"))
		crlf, tam := medirTexto(guardiaRenglones(n, "\r\n"))
		if lf != n || crlf != n {
			t.Errorf("%d renglones: LF cuenta %d y CRLF %d", n, lf, crlf)
		}
		if conLF := len(strings.TrimSpace(guardiaRenglones(n, "\n"))); tam != conLF+n-1 {
			t.Errorf("%d renglones CRLF: tamaño %d, quería %d", n, tam, conLF+n-1)
		}
	}
}

// TestGuardiaSinCarpetaDeConfig: sin `.claude` en el HOME (o sin HOME) no se cae y mide lo del proyecto.
func TestGuardiaSinCarpetaDeConfig(t *testing.T) {
	root, _, _ := guardiaEntorno(t)
	guardiaEscribir(t, filepath.Join(root, "CLAUDE.md"), guardiaRenglones(201, "\n"))
	for _, dirConfig := range []string{filepath.Join(t.TempDir(), "no-existe"), ""} {
		if out := guardiaCorrer(newFakeStore(), root, dirConfig, "", guardiaDia); !strings.Contains(out, "CLAUDE.md (201 líneas") {
			t.Errorf("dirConfig %q: no avisó del CLAUDE.md del proyecto: %q", dirConfig, out)
		}
	}
}

// TestGuardiaLasDosLineasDelEnvelope: el aviso se suma al contexto que el arranque ya traía, va al
// dueño como systemMessage, son una línea cada uno, y se cobra y se marca una sola vez.
func TestGuardiaLasDosLineasDelEnvelope(t *testing.T) {
	root, dirConfig, home := guardiaEntorno(t)
	guardiaEscribir(t, filepath.Join(root, "CLAUDE.md"), guardiaRenglones(262, "\n"))
	type envelope struct {
		System string `json:"systemMessage"`
		H      struct {
			Event string `json:"hookEventName"`
			Ctx   string `json:"additionalContext"`
		} `json:"hookSpecificOutput"`
	}
	for _, previo := range []string{"", assembleHookContext("SessionStart", "[Musubi — memoria] lo que ya iba")} {
		store := newFakeStore()
		out := sumarGuardiaDeInstruccionesCon(previo, store, "S", root, dirConfig, home, guardiaDia)
		var env envelope
		if err := json.Unmarshal([]byte(out), &env); err != nil {
			t.Fatalf("no es json: %v\n%s", err, out)
		}
		partes := strings.Split(env.H.Ctx, "\n\n")
		agente := partes[len(partes)-1]
		if env.H.Event != "SessionStart" || !strings.HasPrefix(agente, "[Musubi — guardia de instrucciones] CLAUDE.md (262 líneas") {
			t.Errorf("la línea del agente no llegó: %q", env.H.Ctx)
		}
		if previo != "" && (len(partes) != 2 || !strings.Contains(partes[0], "lo que ya iba")) {
			t.Errorf("se perdió el contexto que el arranque ya traía: %q", env.H.Ctx)
		}
		if !strings.Contains(env.System, "CLAUDE.md (262 líneas") || !strings.Contains(env.System, ".claude/rules/") || !strings.Contains(env.System, "paths:") {
			t.Errorf("la línea del dueño no nombra el archivo o no sugiere las reglas: %q", env.System)
		}
		if strings.Contains(env.System, "\n") || strings.Contains(agente, "\n") {
			t.Errorf("cada aviso tiene que ser una sola línea: %q / %q", env.System, agente)
		}
		if store.ledger[surfaceGuardiaDeInstrucciones] <= 0 || store.meta[metaGuardiaDeInstrucciones] != "2026-10-14" {
			t.Errorf("no se cobró o no se marcó: ledger %v, meta %v", store.ledger, store.meta)
		}
	}
	if out := sumarGuardiaDeInstruccionesCon("", nil, "S", root, dirConfig, home, guardiaDia); out != "" {
		t.Errorf("sin memoria no hay marca del día: tiene que callar, salió %q", out)
	}
}
