package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// escribir deja un archivo con ese contenido en el tempdir del test y devuelve su ruta.
func escribir(t *testing.T, dir, nombre, contenido string) string {
	t.Helper()
	ruta := filepath.Join(dir, nombre)
	if err := os.WriteFile(ruta, []byte(contenido), 0o644); err != nil {
		t.Fatalf("escribir %s: %v", nombre, err)
	}
	return ruta
}

// TestRunVerdeCuandoCoincide: salida == baseline → exit 0, sin ruido de altas.
func TestRunVerdeCuandoCoincide(t *testing.T) {
	dir := t.TempDir()
	salida := escribir(t, dir, "raw.txt", "internal/x/a.go:1:1: unreachable func: A\n")
	base := escribir(t, dir, "base.txt", "internal/x/a.go\tA\n")

	var out, errw bytes.Buffer
	if code := run([]string{"-salida", salida, "-baseline", base}, &out, &errw); code != 0 {
		t.Errorf("code = %d; quería 0\nout: %s\nerr: %s", code, out.String(), errw.String())
	}
}

// TestRunRojoAnteAlta (R1): una inalcanzable nueva que no está en el baseline → exit 1, y el
// símbolo aparece nombrado en la salida.
func TestRunRojoAnteAlta(t *testing.T) {
	dir := t.TempDir()
	salida := escribir(t, dir, "raw.txt", strings.Join([]string{
		"internal/x/a.go:1:1: unreachable func: A",
		"internal/x/b.go:2:2: unreachable func: Nueva",
	}, "\n")+"\n")
	base := escribir(t, dir, "base.txt", "internal/x/a.go\tA\n")

	var out, errw bytes.Buffer
	code := run([]string{"-salida", salida, "-baseline", base}, &out, &errw)
	if code != 1 {
		t.Errorf("code = %d; quería 1", code)
	}
	if !strings.Contains(errw.String(), "Nueva") {
		t.Errorf("la salida no nombra la función nueva:\n%s", errw.String())
	}
}

// TestRunAvisaBajaPeroVerde: una del baseline que ya no aparece → exit 0 con aviso (no es un rojo).
func TestRunAvisaBajaPeroVerde(t *testing.T) {
	dir := t.TempDir()
	salida := escribir(t, dir, "raw.txt", "internal/x/a.go:1:1: unreachable func: A\n")
	base := escribir(t, dir, "base.txt", "internal/x/a.go\tA\ninternal/x/c.go\tVieja\n")

	var out, errw bytes.Buffer
	code := run([]string{"-salida", salida, "-baseline", base}, &out, &errw)
	if code != 0 {
		t.Errorf("code = %d; quería 0 (una baja no falla el gate)\nerr: %s", code, errw.String())
	}
	if !strings.Contains(out.String(), "Vieja") {
		t.Errorf("no avisó de la baja:\n%s", out.String())
	}
}

// TestRunBaselineAusente (R5): sin baseline, el helper se niega y pide --update; no lo autocrea.
func TestRunBaselineAusente(t *testing.T) {
	dir := t.TempDir()
	salida := escribir(t, dir, "raw.txt", "internal/x/a.go:1:1: unreachable func: A\n")
	base := filepath.Join(dir, "no-existe.txt")

	var out, errw bytes.Buffer
	code := run([]string{"-salida", salida, "-baseline", base}, &out, &errw)
	if code != 2 {
		t.Errorf("code = %d; quería 2", code)
	}
	if !strings.Contains(errw.String(), "--update") {
		t.Errorf("no sugirió --update:\n%s", errw.String())
	}
	if _, err := os.Stat(base); !os.IsNotExist(err) {
		t.Errorf("el helper AUTOCREÓ el baseline en una corrida de comparación (R5 roto)")
	}
}

// TestRunUpdateRegeneraOrdenado (R6): --update escribe el baseline desde la salida, ordenado-único,
// y ese baseline pasa la comparación inmediata contra la misma salida.
func TestRunUpdateRegeneraOrdenado(t *testing.T) {
	dir := t.TempDir()
	salida := escribir(t, dir, "raw.txt", strings.Join([]string{
		"internal/x/b.go:2:2: unreachable func: zeta",
		"internal/x/a.go:1:1: unreachable func: Alfa",
	}, "\n")+"\n")
	base := filepath.Join(dir, "base.txt")

	var out, errw bytes.Buffer
	if code := run([]string{"-salida", salida, "-baseline", base, "--update"}, &out, &errw); code != 0 {
		t.Fatalf("update code = %d; quería 0\nerr: %s", code, errw.String())
	}
	got, err := os.ReadFile(base)
	if err != nil {
		t.Fatalf("leer baseline regenerado: %v", err)
	}
	quiero := "internal/x/a.go\tAlfa\ninternal/x/b.go\tzeta\n"
	if string(got) != quiero {
		t.Errorf("baseline regenerado = %q; quería %q", got, quiero)
	}

	var out2, errw2 bytes.Buffer
	if code := run([]string{"-salida", salida, "-baseline", base}, &out2, &errw2); code != 0 {
		t.Errorf("comparar contra el baseline recién regenerado dio %d; quería 0", code)
	}
}

// TestRunSalidaNoParseable (R2): si la salida cruda tiene una línea basura, el helper sale 2 (no
// pudo chequear), no 0 ni 1.
func TestRunSalidaNoParseable(t *testing.T) {
	dir := t.TempDir()
	salida := escribir(t, dir, "raw.txt", "esto no es una línea de deadcode\n")
	base := escribir(t, dir, "base.txt", "internal/x/a.go\tA\n")

	var out, errw bytes.Buffer
	if code := run([]string{"-salida", salida, "-baseline", base}, &out, &errw); code != 2 {
		t.Errorf("code = %d; quería 2 ante salida no parseable", code)
	}
}

// TestRunFaltaSalida: sin -salida, error de uso (exit 2).
func TestRunFaltaSalida(t *testing.T) {
	var out, errw bytes.Buffer
	if code := run(nil, &out, &errw); code != 2 {
		t.Errorf("code = %d; quería 2 cuando falta -salida", code)
	}
}
