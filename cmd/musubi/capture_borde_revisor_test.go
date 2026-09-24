package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/guiones"
	"musubi/internal/memory"
)

// capture_borde_test.go: regresiones de la 2da ronda de revisión (lente: corrección y casos borde)
// sobre la tanda congelada. Usa repoDePrueba y corridasHastaAlDia de capture_tanda_test.go, y
// newEngine / recordingStore de capture_test.go.

const fechaFija = "2026-09-01T10:00:00+00:00"

// commitCon escribe un archivo y commitea con asunto y cuerpo explícitos; devuelve el SHA.
func commitCon(t *testing.T, dir string, git func(string, ...string) string, archivo, asunto, cuerpo string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, archivo), []byte(asunto+cuerpo+dir), 0o644); err != nil {
		t.Fatal(err)
	}
	git(fechaFija, "add", ".")
	args := []string{"commit", "-q", "-m", asunto}
	if cuerpo != "" {
		args = append(args, "-m", cuerpo)
	}
	git(fechaFija, args...)
	return git(fechaFija, "rev-parse", "HEAD")
}

// H1a — un fallo DETERMINISTA en la PRIMERA corrida congela la tanda para siempre (regresión: en main
// la corrida siguiente volvía a pedir `-1 HEAD` y se destrababa sola en cuanto el HEAD avanzaba).
// Dos repos del central con el mismo «Initial commit» + README.md dan el MISMO commitObsID; el segundo,
// con otro --project, choca con ErrCrossTenant y :objetivo queda fijo en ese commit.
func TestCaptureFalloDeterministaEnPrimeraCorridaCongelaLaTanda(t *testing.T) {
	engine := newEngine(t)
	dirA, gitA := repoDePrueba(t)
	gitA(fechaFija, "init", "-q", "-b", "main")
	commitCon(t, dirA, gitA, "README.md", "Initial commit", "")
	dirB, gitB := repoDePrueba(t)
	gitB(fechaFija, "init", "-q", "-b", "main")
	commitCon(t, dirB, gitB, "README.md", "Initial commit", "")
	keyA, keyB := repoCursorKey(dirA), repoCursorKey(dirB)

	engine.SetProjectID("proyecto-a")
	if n, err := captureCommitsKeyed(engine, realGit{dir: dirA}, nil, nil, memory.ScopeShared, keyA, 0); err != nil || n != 1 {
		t.Fatalf("repo A: n=%d err=%v", n, err)
	}
	engine.SetProjectID("proyecto-b")
	_, _ = captureCommitsKeyed(engine, realGit{dir: dirB}, nil, nil, memory.ScopeShared, keyB, 0)

	commitCon(t, dirB, gitB, "b1.go", "feat: el primer cambio real del repo B", "")
	head := commitCon(t, dirB, gitB, "b2.go", "fix: el segundo cambio real del repo B", "")
	var ultimoErr error
	for i := 0; i < 5; i++ {
		_, ultimoErr = captureCommitsKeyed(engine, realGit{dir: dirB}, nil, nil, memory.ScopeShared, keyB, 0)
	}
	if base, _, _ := engine.GetMeta(keyB); base != head {
		obj, _, _ := engine.GetMeta(keyB + sufijoTandaObjetivo)
		t.Fatalf("5 corridas y la base de B no llegó al HEAD %.7s: base=%q objetivo=%.7s err=%v", head, base, obj, ultimoErr)
	}
}

// H1b — el mismo veneno a MITAD de tanda, modo hook, un solo proyecto: un commit vacío (sin
// «Archivos:») cuyo cuerpo termina en `</content>` lo rechaza siempre SobreDeLlamadaComido. Cada
// corrida muere en el mismo commit y los posteriores no se capturan nunca (esto también pasa en main).
func TestCaptureCommitVenenoAMitadDeTandaTrabaLaCaptura(t *testing.T) {
	engine := newEngine(t)
	dir, git := repoDePrueba(t)
	git(fechaFija, "init", "-q", "-b", "main")
	base := commitCon(t, dir, git, "base.go", "feat: la base ya capturada", "")
	commitCon(t, dir, git, "c1.go", "feat: el primero despues de la base", "")
	git(fechaFija, "commit", "-q", "--allow-empty", "-m", "fix(memoria): rechazar el sobre comido",
		"-m", "Asi llegaba el texto:\n<content>la nota</content>")
	commitCon(t, dir, git, "c3.go", "fix: el que viene despues del veneno", "")
	head := commitCon(t, dir, git, "c4.go", "feat: el ultimo de la tanda", "")
	if err := engine.SetMeta(metaCaptureLastCommit, base); err != nil {
		t.Fatal(err)
	}
	var ultimoErr error
	for i := 0; i < 10; i++ {
		_, ultimoErr = captureCommitsKeyed(engine, realGit{dir: dir}, nil, nil, memory.ScopeLocal, metaCaptureLastCommit, maxCommitsPorCorrida)
	}
	if got, _, _ := engine.GetMeta(metaCaptureLastCommit); got != head {
		t.Fatalf("10 corridas y la base no llegó al HEAD: base=%.7s err=%v", got, ultimoErr)
	}
}

// H2 — la BASE desaparece (reset + reflog expire + gc): el rango falla, realGit.CommitsEntre cae a
// `-1 objetivo` y la tanda cierra con base=objetivo; d1..d3 quedan declarados capturados sin mirarse.
func TestCaptureBaseBorradaPorGCSaltaLoIntermedio(t *testing.T) {
	dir, git := repoDePrueba(t)
	git(fechaFija, "init", "-q", "-b", "main")
	c0 := commitCon(t, dir, git, "c0.go", "feat: el commit que sobrevive", "")
	c1 := commitCon(t, dir, git, "c1.go", "feat: el commit que se reescribe", "")
	git(fechaFija, "reset", "-q", "--hard", c0)
	for _, n := range []string{"d1", "d2", "d3", "d4"} {
		commitCon(t, dir, git, n+".go", "feat: el commit "+n+" posterior al reset", "")
	}
	git(fechaFija, "reflog", "expire", "--expire=now", "--all")
	git(fechaFija, "gc", "-q", "--prune=now")
	if err := guiones.Herramienta(t, "git", "-C", dir, "cat-file", "-e", c1+"^{commit}").Run(); err == nil {
		t.Skip("el gc no borró la base; el escenario no se armó")
	}
	store := &recordingStore{meta: map[string]string{metaCaptureLastCommit: c1}}
	corridasHastaAlDia(t, store, dir, maxCommitsPorCorrida, 10)
	// Lo que importa es que d1..d4 estén TODOS: la ventana también repasa c0, que sigue en la historia,
	// y repasar es UPSERT (los ids son determinísticos), así que contar exactamente 4 era pedir de más.
	for _, n := range []string{"d1", "d2", "d3", "d4"} {
		esta := false
		for _, contenido := range store.byID {
			if strings.Contains(contenido, "el commit "+n+" posterior al reset") {
				esta = true
			}
		}
		if !esta {
			t.Errorf("la base borrada saltó lo intermedio: %s no se capturó (capturados: %d)", n, len(store.byID))
		}
	}
}

// H3 — un cuerpo con \x1e<SHA>\x1f…\x1f fabrica en parseCommits un registro con un SHA REAL repetido.
// Si ese registro cae justo al final de una corrida, :hecho = SHA repetido; la corrida siguiente lo
// encuentra en su PRIMERA aparición y rehace el mismo tramo, que vuelve a terminar en el falso: ciclo.
func TestCaptureSHARepetidoPorSeparadoresCicla(t *testing.T) {
	dir, git := repoDePrueba(t)
	git(fechaFija, "init", "-q", "-b", "main")
	base := commitCon(t, dir, git, "base.go", "feat: la base ya capturada", "")
	for i := 0; i < 24; i++ { // índices 0..23
		commitCon(t, dir, git, fmt.Sprintf("r%02d.go", i), fmt.Sprintf("feat: relleno previo numero %02d", i), "")
	}
	x := commitCon(t, dir, git, "x.go", "feat: el commit X que se va a repetir", "") // índice 24
	for i := 0; i < 23; i++ {                                                        // índices 25..47
		commitCon(t, dir, git, fmt.Sprintf("s%02d.go", i), fmt.Sprintf("feat: relleno posterior numero %02d", i), "")
	}
	// índice 48 (el commit real) y 49 (el registro falso con SHA = X)
	commitCon(t, dir, git, "m.go", "feat: cuerpo con separadores", "antes\x1e"+x+"\x1fasunto falso bastante largo\x1fresto")
	head := commitCon(t, dir, git, "z.go", "feat: el ultimo commit del repo", "")
	store := &recordingStore{meta: map[string]string{metaCaptureLastCommit: base}}
	for i := 1; i <= 20; i++ {
		if _, err := captureCommitsKeyed(store, realGit{dir: dir}, nil, nil, memory.ScopeLocal, metaCaptureLastCommit, maxCommitsPorCorrida); err != nil {
			t.Fatal(err)
		}
		if store.meta[metaCaptureLastCommit] == head {
			return
		}
	}
	t.Fatalf("20 corridas sin llegar al HEAD; :hecho=%.7s y X=%.7s (el SHA repetido): el tramo 25..49 se repite para siempre",
		store.meta[metaCaptureLastCommit+sufijoTandaHecho], x)
}

// Con git de verdad: un objetivo que ya no existe (acá, un SHA que nunca existió) abandona la tanda,
// y es el ÚNICO error del rango que lo hace; realGit lo distingue preguntando si el objetivo existe.
//
// Sabotaje que la pone roja: no marcar el error como «objetivo perdido».
// arnes: archivo="cmd/musubi/capture.go"
// arnes: de="return nil, fmt.Errorf(\"%w (%s): %v\", errObjetivoPerdido, hasta, err)"
// arnes: a="return nil, fmt.Errorf(\"(%s): %v\", hasta, err)"
func TestCaptureObjetivoInexistenteEnRepoRealAbandonaLaTanda(t *testing.T) {
	dir, git := repoDePrueba(t)
	git(fechaFija, "init", "-q", "-b", "main")
	base := commitCon(t, dir, git, "a.go", "feat: la base", "")
	perdido := "0123456789abcdef0123456789abcdef01234567"
	store := &recordingStore{meta: map[string]string{
		metaCaptureLastCommit:                       base,
		metaCaptureLastCommit + sufijoTandaObjetivo: perdido,
		metaCaptureLastCommit + sufijoTandaHecho:    "",
	}}
	_, err := captureCommitsKeyed(store, realGit{dir: dir}, nil, nil, memory.ScopeLocal, metaCaptureLastCommit, maxCommitsPorCorrida)
	if !errors.Is(err, errObjetivoPerdido) {
		t.Fatalf("un objetivo inexistente tenía que reportarse como errObjetivoPerdido, dio %v", err)
	}
	if store.meta[metaCaptureLastCommit+sufijoTandaObjetivo] != "" {
		t.Fatalf("la tanda con el objetivo perdido no se abandonó: %v", store.meta)
	}
}
