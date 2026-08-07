package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"musubi/internal/provision"
)

// arsenal_step_test.go sella G8 de la spec «arsenal-arranque»: sin --skills, `provision` NO
// toca el arsenal NI sale a la red.
//
// El riesgo concreto que cubre: que a alguien le parezca útil que el paso informativo muestre
// «el arsenal tiene N skills». Eso convierte a `provision` —cuyo trabajo es unir una máquina—
// en algo que depende de que el arsenal esté sano, y lo hace fallar cuando más se lo necesita.

// proyectoConCentral deja un proyecto con el bloque sync: apuntando a un central de mentira que
// cuenta cuántas veces lo llamaron.
func proyectoConCentral(t *testing.T) (dir string, hits *int32) {
	t.Helper()
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"jsonrpc":"2.0","id":"x","result":{"content":[{"type":"text","text":"[]"}]}}`)
	}))
	t.Cleanup(srv.Close)

	dir = t.TempDir()
	t.Setenv("MUSUBI_TEST_TOKEN", "secreto-abc")
	musubiDir := filepath.Join(dir, ".musubi")
	if err := os.MkdirAll(musubiDir, 0o755); err != nil {
		t.Fatalf("no se pudo crear .musubi: %v", err)
	}
	cfg := fmt.Sprintf("sync:\n  enabled: true\n  central_url: %s\n  auth_token_env: MUSUBI_TEST_TOKEN\n  allow_insecure_token: true\n", srv.URL)
	if err := os.WriteFile(filepath.Join(musubiDir, "config.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("no se pudo escribir el config: %v", err)
	}
	return dir, &n
}

// TestG8SinElFlagNoSeTocaElArsenal
func TestG8SinElFlagNoSeTocaElArsenal(t *testing.T) {
	dir, hits := proyectoConCentral(t)

	paso := arsenalStep(provision.Options{ProjectDir: dir, Skills: false})

	if paso.Status != provision.StatusTodo {
		t.Errorf("sin --skills el paso debe quedar en 'todo', obtuve %+v", paso)
	}
	// El detalle tiene que decir CÓMO pedirlo: si no, el arsenal es invisible para quien
	// acaba de unir su máquina, que es justo a quien esta fase apunta.
	if !strings.Contains(paso.Detail, "--skills") {
		t.Errorf("el paso debe decir cómo instalar el arsenal, obtuve %q", paso.Detail)
	}
	if n := atomic.LoadInt32(hits); n != 0 {
		t.Errorf("sin --skills no debe haber NINGUNA llamada al central, hubo %d", n)
	}
	if _, err := os.Stat(filepath.Join(dir, ".musubi", "skills")); err == nil {
		t.Error("sin --skills no se debe crear el directorio de skills")
	}

	// Control: CON el flag sí se llama al central. Sin esto, el test pasaría con un
	// arsenalStep que nunca hace nada.
	if paso := arsenalStep(provision.Options{ProjectDir: dir, Skills: true}); paso.Status == provision.StatusTodo &&
		strings.Contains(paso.Detail, "--skills") {
		t.Errorf("con --skills el paso no debía ser la guía informativa: %+v", paso)
	}
	if n := atomic.LoadInt32(hits); n == 0 {
		t.Error("con --skills debía llamarse al central")
	}
}

// TestArsenalStepSinCentralNoRompeProvision — unir la máquina tiene que seguir andando aunque
// el arsenal no esté configurado: se reporta como pendiente, no como error duro.
func TestArsenalStepSinCentralNoRompeProvision(t *testing.T) {
	dir := t.TempDir() // sin .musubi/config.yaml: no hay central

	paso := arsenalStep(provision.Options{ProjectDir: dir, Skills: true})
	if paso.Status == provision.StatusError {
		t.Errorf("sin central el paso debe quedar pendiente, no en error: %+v", paso)
	}
	if paso.Detail == "" {
		t.Error("el paso debe explicar por qué no se pudo")
	}
}
