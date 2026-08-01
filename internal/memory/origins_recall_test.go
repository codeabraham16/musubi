package memory

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// recallItem devuelve el item de una observación en un recall, o falla.
func recallItem(t *testing.T, e *DbEngine, query, id string) RecallItem {
	t.Helper()
	res, err := e.Recall(context.Background(), query, RecallOptions{TokenBudget: 4000})
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	for _, it := range res.Items {
		if it.ID == id {
			return it
		}
	}
	t.Fatalf("la observación %q no vino en el recall (items: %d)", id, len(res.Items))
	return RecallItem{}
}

// R7/R8: si el archivo anclado cambia, la observación viene MARCADA y la marca lo nombra.
func TestRecallMarcaCuandoElArchivoCambia(t *testing.T) {
	engine, root := engineConArchivos(t, map[string]string{"src/a.go": "package a // v1\n"})
	if err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k",
		"el parser de tokens vive en src/a.go y usa el algoritmo viejo", 1.0, "", "local",
		[]string{"src/a.go"}, nil); err != nil {
		t.Fatal(err)
	}

	// Antes de tocar nada: sin marca.
	if it := recallItem(t, engine, "parser de tokens", "O1"); len(it.Stale) != 0 {
		t.Fatalf("sin cambios no debe haber marca, obtuve %v", it.Stale)
	}

	// Alguien toca el archivo.
	if err := os.WriteFile(filepath.Join(root, "src", "a.go"), []byte("package a // v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	it := recallItem(t, engine, "parser de tokens", "O1")
	if len(it.Stale) != 1 {
		t.Fatalf("el archivo cambió: esperaba 1 ancla rancia, obtuve %v", it.Stale)
	}
	if it.Stale[0].Path != "src/a.go" || it.Stale[0].Reason != StaleChanged {
		t.Errorf("la marca debe decir qué archivo y por qué, obtuve %+v", it.Stale[0])
	}
	if !strings.Contains(it.Gist, "src/a.go") || !strings.Contains(it.Gist, "rancia") {
		t.Errorf("la advertencia debe viajar EN EL GIST (es lo que el modelo lee), obtuve %q", it.Gist)
	}
}

// R9: un archivo borrado se distingue de uno que cambió.
func TestRecallDistingueArchivoBorrado(t *testing.T) {
	engine, root := engineConArchivos(t, map[string]string{"src/viejo.go": "package v\n"})
	if err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k",
		"nota sobre el modulo viejo del parser", 1.0, "", "local", []string{"src/viejo.go"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "src", "viejo.go")); err != nil {
		t.Fatal(err)
	}

	it := recallItem(t, engine, "modulo viejo del parser", "O1")
	if len(it.Stale) != 1 || it.Stale[0].Reason != StaleMissing {
		t.Fatalf("un archivo borrado debe marcarse como %q, obtuve %+v", StaleMissing, it.Stale)
	}
	if !strings.Contains(it.Gist, "ya no existe") {
		t.Errorf("la advertencia debe distinguir 'ya no existe' de 'cambió', obtuve %q", it.Gist)
	}
}

// R5: una observación SIN anclas atraviesa el recall sin campo ni marca.
func TestRecallSinAnclasNoMarcaNada(t *testing.T) {
	engine, root := engineConArchivos(t, map[string]string{"src/a.go": "package a\n"})
	if err := engine.SaveObservationTyped("O1", "t/k", "nota sin anclas sobre el parser", 1.0, "", "local", nil); err != nil {
		t.Fatal(err)
	}
	// Cambiar el archivo no puede afectarla: no está anclada a nada.
	os.WriteFile(filepath.Join(root, "src", "a.go"), []byte("package a // otro\n"), 0o644)

	it := recallItem(t, engine, "nota sin anclas sobre el parser", "O1")
	if it.Stale != nil {
		t.Errorf("sin anclas el campo Stale debe quedar nil (omitempty), obtuve %v", it.Stale)
	}
	if strings.Contains(it.Gist, "rancia") {
		t.Errorf("sin anclas el gist no debe llevar advertencia, obtuve %q", it.Gist)
	}
}

// R11 — EL GUARDIÁN: una observación marcada sigue apareciendo, en la MISMA posición.
// Si esto se rompe, la marca dejó de ser advertencia y se convirtió en censura.
func TestRecallMarcadaPeroServidaEnLaMismaPosicion(t *testing.T) {
	engine, root := engineConArchivos(t, map[string]string{
		"src/a.go": "package a\n",
		"src/b.go": "package b\n",
	})
	// Tres observaciones del mismo tema; sólo la del medio queda anclada.
	engine.SaveObservationTyped("O1", "t/k", "el indice invertido guarda postings por termino", 3.0, "", "local", nil)
	engine.SaveObservationTypedWithOrigins("", "", "O2", "t/k",
		"el indice invertido se arma en src/a.go al cargar", 2.0, "", "local", []string{"src/a.go"}, nil)
	engine.SaveObservationTyped("O3", "t/k", "el indice invertido se compacta cada noche", 1.0, "", "local", nil)

	orden := func() []string {
		res, err := engine.Recall(context.Background(), "indice invertido", RecallOptions{TokenBudget: 4000})
		if err != nil {
			t.Fatal(err)
		}
		out := []string{}
		for _, it := range res.Items {
			out = append(out, it.ID)
		}
		return out
	}
	antes := orden()

	// Ahora se vuelve rancia.
	os.WriteFile(filepath.Join(root, "src", "a.go"), []byte("package a // cambiado\n"), 0o644)
	despues := orden()

	if len(antes) != len(despues) {
		t.Fatalf("marcar no puede cambiar cuántos items vuelven: antes %v, después %v", antes, despues)
	}
	for i := range antes {
		if antes[i] != despues[i] {
			t.Fatalf("marcar no puede reordenar ni ocultar: antes %v, después %v", antes, despues)
		}
	}
	// Y efectivamente quedó marcada (si no, el test pasaría por no hacer nada).
	if it := recallItem(t, engine, "indice invertido", "O2"); len(it.Stale) == 0 {
		t.Error("pre-condición: O2 debía quedar marcada")
	}
}

// T11 — EL CASO REAL del 2026-08-01: la nota decía "PENDIENTE" de algo ya resuelto y
// nadie lo notó. Con el ancla, el recall lo dice solo.
func TestRecallDetectaElPendienteYaResuelto(t *testing.T) {
	engine, root := engineConArchivos(t, map[string]string{
		"deploy/tareas.md": "gio-dashboard: arreglado\ngio-bot-gateway: PENDIENTE\ngio-bot-bridge: PENDIENTE\n",
	})
	if err := engine.SaveObservationTypedWithOrigins("", "", "OBS-GIO", "bot-gio/estado",
		"Fix durable aplicado al dashboard de gio. PENDIENTE: gateway y bridge siguen con el defecto.",
		1.0, "", "local", []string{"deploy/tareas.md"}, nil); err != nil {
		t.Fatal(err)
	}

	// Se resuelve el pendiente en el mundo real, pero nadie actualiza la nota.
	if err := os.WriteFile(filepath.Join(root, "deploy", "tareas.md"),
		[]byte("gio-dashboard: arreglado\ngio-bot-gateway: arreglado\ngio-bot-bridge: arreglado\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	it := recallItem(t, engine, "estado del gateway y el bridge de gio", "OBS-GIO")
	if len(it.Stale) == 0 {
		t.Fatal("la nota quedó vencida y el recall debe decirlo sin que ningún agente lo note")
	}
	if !strings.Contains(it.Gist, "deploy/tareas.md") {
		t.Errorf("la advertencia debe nombrar el archivo que se movió, obtuve %q", it.Gist)
	}
}
