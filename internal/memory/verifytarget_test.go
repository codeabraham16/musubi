package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlobMatchSegmentosYDobleAsterisco(t *testing.T) {
	casos := []struct {
		pat, name string
		want      bool
	}{
		{"internal/memory/*.go", "internal/memory/workflow.go", true},
		{"internal/memory/*.go", "internal/memory/sub/workflow.go", false}, // * no cruza '/'
		{"internal/**/*.go", "internal/memory/sub/workflow.go", true},
		{"internal/**", "internal/a/b/c.go", true},
		{"**/*.md", "docs/guia.md", true},
		{"**/*.md", "guia.md", true}, // ** matchea cero segmentos
		{"cmd/*.go", "internal/memory/workflow.go", false},
		{"internal/memory/w?rkflow.go", "internal/memory/workflow.go", true},
	}
	for _, c := range casos {
		if got := globMatch(c.pat, c.name); got != c.want {
			t.Errorf("globMatch(%q, %q) = %v, quería %v", c.pat, c.name, got, c.want)
		}
	}
}

// escribirArbol arma un proyecto de prueba y devuelve su raíz.
func escribirArbol(t *testing.T, archivos map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, contenido := range archivos {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// El digest depende del CONTENIDO: mismo contenido → mismo digest; un byte distinto → otro.
func TestVerifyTargetDigestSensibleAlContenido(t *testing.T) {
	root := escribirArbol(t, map[string]string{
		"src/a.go":  "package a\n",
		"src/b.go":  "package b\n",
		"README.md": "no entra\n",
	})
	d1, files, err := verifyTargetDigest(root, []string{"src/*.go"})
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if len(files) != 2 || files[0] != "src/a.go" || files[1] != "src/b.go" {
		t.Fatalf("los archivos deben venir ordenados y sin el README, obtuve %v", files)
	}

	// Recalcular sin tocar nada: estable.
	d2, _, _ := verifyTargetDigest(root, []string{"src/*.go"})
	if d1 != d2 {
		t.Error("el digest debe ser estable si nada cambió")
	}

	// Cambiar un byte de un archivo que SÍ entra: cambia.
	os.WriteFile(filepath.Join(root, "src", "b.go"), []byte("package b // tocado\n"), 0o644)
	d3, _, _ := verifyTargetDigest(root, []string{"src/*.go"})
	if d1 == d3 {
		t.Error("el digest debe cambiar si cambia un archivo del target")
	}

	// Cambiar un archivo que NO entra: no cambia.
	os.WriteFile(filepath.Join(root, "README.md"), []byte("otra cosa\n"), 0o644)
	d4, _, _ := verifyTargetDigest(root, []string{"src/*.go"})
	if d3 != d4 {
		t.Error("un archivo fuera del target no debe mover el digest")
	}
}

// Un target que no matchea nada es error: un gate sobre el vacío pasaría siempre.
func TestVerifyTargetSinMatchesEsError(t *testing.T) {
	root := escribirArbol(t, map[string]string{"src/a.go": "package a\n"})
	if _, _, err := verifyTargetDigest(root, []string{"noexiste/*.go"}); err == nil {
		t.Error("un verify_target que no matchea archivos debe fallar, no devolver digest vacío")
	}
}

// .musubi se ignora: si entrara, el journal movería el digest y nunca coincidiría consigo mismo.
func TestVerifyTargetIgnoraDirectoriosDeRuido(t *testing.T) {
	root := escribirArbol(t, map[string]string{
		"src/a.go":              "package a\n",
		".musubi/interno.go":    "package x\n",
		"node_modules/dep/i.go": "package d\n",
	})
	_, files, err := verifyTargetDigest(root, []string{"**/*.go"})
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if len(files) != 1 || files[0] != "src/a.go" {
		t.Errorf(".musubi y node_modules deben quedar fuera, obtuve %v", files)
	}
}

// EL TEST QUE DEFINE LA PRUEBA ESTRUCTURAL: el agente dice "pass", pero los archivos que
// declaró verificar cambiaron desde que se congeló el candidato. Musubi lo deriva del disco
// —sin preguntarle a nadie— y el pass NO se aplica: el gate se reabre.
func TestVerifyTargetDerivaDerivaYRechazaElPass(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	escribir := func(contenido string) {
		if err := os.WriteFile(filepath.Join(src, "a.go"), []byte(contenido), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	escribir("package a // v1\n")

	engine, err := NewDbEngine(root) // la base vive en <root>/.musubi → projectRoot() == root
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()

	def := WorkflowDef{ID: "vt", Steps: []WorkflowStep{
		{ID: "impl", Verify: "revisá el código", VerifyTarget: []string{"src/*.go"}, MaxIterations: 3},
		{ID: "deploy", Needs: []string{"impl"}},
	}}
	if _, err := engine.StartWorkflowRun("R", def); err != nil {
		t.Fatal(err)
	}
	engine.WorkflowReady("R")
	if _, err := engine.CompleteWorkflowStep("R", "impl", "listo", StepDone, ""); err != nil {
		t.Fatal(err)
	}

	// Alguien toca el archivo DESPUÉS de congelar el candidato.
	escribir("package a // v2, cambiado por atras\n")

	run, reflections, err := engine.VerifyWorkflowStep("R", "impl", true, "revisé y está bien", "")
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if run.StepStatus["impl"] == StepDone {
		t.Fatal("el pass no puede valer: los archivos verificados cambiaron desde que se congelaron")
	}
	if run.StepStatus["impl"] != StepPending {
		t.Errorf("con presupuesto, la deriva debe reabrir el step, está %q", run.StepStatus["impl"])
	}
	if len(reflections) == 0 {
		t.Fatal("la deriva debe dejar una reflexión que la explique")
	}
	if !strings.Contains(reflections[len(reflections)-1], "cambió en disco") {
		t.Errorf("la reflexión debe decir qué pasó, obtuve %q", reflections[len(reflections)-1])
	}
	ready, _ := engine.WorkflowReady("R")
	for _, s := range ready {
		if s == "deploy" {
			t.Error("'deploy' no puede destrabarse con un candidato a la deriva")
		}
	}

	// Sin tocar nada más, el candidato vigente sí pasa.
	engine.WorkflowReady("R")
	if _, err := engine.CompleteWorkflowStep("R", "impl", "listo v2", StepDone, ""); err != nil {
		t.Fatal(err)
	}
	run, _, err = engine.VerifyWorkflowStep("R", "impl", true, "", "")
	if err != nil {
		t.Fatalf("verify sobre el candidato vigente: %v", err)
	}
	if run.StepStatus["impl"] != StepDone {
		t.Errorf("con el disco quieto el pass debe valer, está %q", run.StepStatus["impl"])
	}
}

// verify_target sin verify no valida.
func TestVerifyTargetExigeVerify(t *testing.T) {
	def := WorkflowDef{ID: "w", Steps: []WorkflowStep{
		{ID: "a", VerifyTarget: []string{"src/*.go"}},
	}}
	if errs := def.Validate(); len(errs) == 0 {
		t.Error("verify_target sin verify debe ser inválido")
	}
}
