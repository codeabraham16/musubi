package codeintel

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Runner abstrae la obtención del diff de git, para que el detector sea testeable con un
// diff fijo (FakeRunner) sin depender de un repo real.
type Runner interface {
	// Diff devuelve la salida de `git diff` en formato unified. Si staged es true compara
	// el índice (--staged); si ref no está vacío, compara contra ese ref.
	Diff(ref string, staged bool) (string, error)
}

// GitRunner ejecuta git en un directorio de trabajo, de forma model-free y determinista:
// sin paginador, sin color, sin dependencia de locale.
type GitRunner struct {
	Dir     string
	Timeout time.Duration
}

// NewGitRunner crea un runner sobre el directorio dado con un timeout por defecto.
func NewGitRunner(dir string) GitRunner {
	return GitRunner{Dir: dir, Timeout: 15 * time.Second}
}

func (g GitRunner) Diff(ref string, staged bool) (string, error) {
	timeout := g.Timeout
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	args := []string{"--no-pager", "diff", "--no-color", "--find-renames"}
	if staged {
		args = append(args, "--staged")
	}
	if strings.TrimSpace(ref) != "" {
		args = append(args, ref)
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.Dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff falló: %w", err)
	}
	return string(out), nil
}

// FakeRunner devuelve un diff prefijado; para tests.
type FakeRunner struct {
	Out string
	Err error
}

func (f FakeRunner) Diff(string, bool) (string, error) { return f.Out, f.Err }
