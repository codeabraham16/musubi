package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"musubi/internal/memory"
)

// capture.go es la RED DE SEGURIDAD determinista de la captura automática (Fase C3): un hook
// `Stop` que, al cerrar el turno, captura los COMMITS nuevos del repo como memoria LOCAL, sin
// depender del agente ni de un LLM. El mensaje de commit ES el "por qué" destilado por el humano
// (la señal estructurada de mayor valor y menor ruido, según la investigación SOTA).

// metaCaptureLastCommit guarda el último HEAD capturado. Es GLOBAL al repo (no por sesión): el
// HEAD no depende de la sesión, así que scopearlo re-capturaría en cada sesión nueva.
const metaCaptureLastCommit = "capture:last_commit"

// commit es un commit ya parseado, listo para volverse memoria.
type commit struct {
	SHA     string
	Subject string
	Body    string
	Files   []string
}

// gitLog abstrae la lectura del historial, para testear el core con un git falso.
type gitLog interface {
	Head() (string, error) // SHA del HEAD; error si no es un repo git
	CommitsSince(last string) ([]commit, error)
}

// captureStore es lo mínimo que el core necesita del motor. *memory.DbEngine lo satisface.
type captureStore interface {
	GetMeta(key string) (string, bool, error)
	SetMeta(key, value string) error
	SaveObservationDedupedTyped(topicKey, content string, importance float64, memType, scope string, embedding []float32) (string, bool, error)
}

// realGit ejecuta git en un directorio, model-free y determinista (sin pager/color/locale).
type realGit struct{ dir string }

func (g realGit) run(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	full := append([]string{"-C", g.dir, "--no-pager"}, args...)
	out, err := exec.CommandContext(ctx, "git", full...).Output()
	return string(out), err
}

func (g realGit) Head() (string, error) {
	out, err := g.run("rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func (g realGit) CommitsSince(last string) ([]commit, error) {
	// Separadores de control para un parseo robusto: %x1e entre commits, %x1f entre campos.
	args := []string{"log", "--no-color", "--no-merges", "--reverse", "--name-only",
		"--format=%x1e%H%x1f%s%x1f%b%x1f"}
	if strings.TrimSpace(last) != "" {
		args = append(args, last+"..HEAD")
	} else {
		args = append(args, "-1", "HEAD")
	}
	out, err := g.run(args...)
	if err != nil {
		// El rango last..HEAD puede fallar si `last` ya no existe (rebase/force-push): caer a
		// capturar sólo el HEAD actual en vez de romper.
		if strings.TrimSpace(last) != "" {
			return g.CommitsSince("")
		}
		return nil, err
	}
	return parseCommits(out), nil
}

// parseCommits parsea la salida de `git log` con separadores %x1e/%x1f + --name-only.
func parseCommits(out string) []commit {
	var res []commit
	for _, rec := range strings.Split(out, "\x1e") {
		if strings.TrimSpace(rec) == "" {
			continue
		}
		parts := strings.SplitN(strings.TrimLeft(rec, "\n"), "\x1f", 4)
		if len(parts) < 3 {
			continue
		}
		c := commit{
			SHA:     strings.TrimSpace(parts[0]),
			Subject: strings.TrimSpace(parts[1]),
			Body:    strings.TrimSpace(parts[2]),
		}
		if len(parts) == 4 {
			for _, f := range strings.Split(parts[3], "\n") {
				if f = strings.TrimSpace(f); f != "" {
					c.Files = append(c.Files, f)
				}
			}
		}
		res = append(res, c)
	}
	return res
}

// classifyCommit deduce (model-free, por keyword del subject) el tipo/importancia de un commit, o
// si es trivial y hay que omitirlo. fix/bug/security → alto; feat/refactor/perf → medio;
// chore/docs/style/test/build/ci y merge/wip/subjects cortos → skip.
func classifyCommit(subject string) (memType string, importance float64, skip bool) {
	s := strings.ToLower(strings.TrimSpace(subject))
	if len(s) < 10 || strings.HasPrefix(s, "merge") || strings.HasPrefix(s, "wip") {
		return "", 0, true
	}
	typ := s
	if i := strings.IndexAny(s, ":("); i > 0 {
		typ = strings.TrimSpace(s[:i])
	}
	switch {
	case hasWord(s, "fix", "bug", "hotfix", "security", "cve", "vuln", "revert"):
		return "episodic", 0.7, false
	case hasWord(s, "feat", "refactor", "perf"):
		return "episodic", 0.5, false
	case typ == "chore" || typ == "docs" || typ == "doc" || typ == "style" || typ == "test" || typ == "build" || typ == "ci":
		return "", 0, true
	default:
		return "episodic", 0.4, false
	}
}

// hasWord matchea w como palabra/prefijo de tipo (fix, fix:, fix(scope), "... fix ..."), evitando
// falsos positivos por substring (prefix, suffix).
func hasWord(s string, words ...string) bool {
	for _, w := range words {
		if s == w ||
			strings.HasPrefix(s, w+":") || strings.HasPrefix(s, w+"(") || strings.HasPrefix(s, w+" ") ||
			strings.Contains(s, " "+w+" ") || strings.Contains(s, " "+w+":") {
			return true
		}
	}
	return false
}

// captureCommits es el core testeable: captura los commits nuevos desde el último HEAD guardado
// como observaciones LOCALES (nunca shared: C3 no debe filtrar un secreto de un diff; compartir
// pasa por promote, que C2 redacta). Devuelve cuántas guardó. No-op silencioso si no es repo git
// o no hay commits nuevos.
func captureCommits(store captureStore, git gitLog) (int, error) {
	head, err := git.Head()
	if err != nil || head == "" {
		return 0, nil
	}
	last, _, _ := store.GetMeta(metaCaptureLastCommit)
	if strings.TrimSpace(last) == head {
		return 0, nil
	}
	commits, err := git.CommitsSince(last)
	if err != nil {
		return 0, err
	}
	saved := 0
	for _, c := range commits {
		memType, importance, skip := classifyCommit(c.Subject)
		if skip {
			continue
		}
		content := c.Subject
		if c.Body != "" {
			content += "\n\n" + c.Body
		}
		if len(c.Files) > 0 {
			content += "\n\nArchivos: " + strings.Join(c.Files, ", ")
		}
		if _, _, err := store.SaveObservationDedupedTyped("git-commit", content, importance, memType, memory.ScopeLocal, nil); err != nil {
			return saved, err
		}
		saved++
	}
	_ = store.SetMeta(metaCaptureLastCommit, head)
	return saved, nil
}

// runCapture implementa `musubi capture [--hook-mode]`: en el hook Stop es silencioso; en modo
// normal imprime un resumen. Best-effort: cualquier fallo (sin repo, sin memoria) no rompe el turno.
func runCapture(args []string) {
	hookMode := false
	for _, a := range args {
		if a == "--hook-mode" {
			hookMode = true
		}
	}
	if hookMode {
		_, _ = io.Copy(io.Discard, os.Stdin) // drenar el payload del Stop (no lo necesitamos)
	}

	root := workspaceDir()
	if err := ensureWorkspace(root); err != nil {
		if !hookMode {
			fmt.Fprintf(os.Stderr, "capture: workspace no disponible: %v\n", err)
		}
		return
	}
	engine, err := memory.NewDbEngine(root)
	if err != nil {
		if !hookMode {
			fmt.Fprintf(os.Stderr, "capture: memoria no disponible: %v\n", err)
		}
		return
	}
	defer engine.Close()

	n, err := captureCommits(engine, realGit{dir: root})
	if err != nil {
		if !hookMode {
			fmt.Fprintf(os.Stderr, "capture: %v\n", err)
		}
		return
	}
	if !hookMode {
		fmt.Printf("Capturados %d commit(s) nuevos en memoria local.\n", n)
	}
}
