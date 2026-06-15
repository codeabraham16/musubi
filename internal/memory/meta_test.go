package memory

import (
	"testing"
	"time"
)

func TestMetaGetSet(t *testing.T) {
	e := newTestEngine(t)

	if _, ok, err := e.GetMeta("k"); err != nil || ok {
		t.Fatalf("clave ausente debería dar ok=false, obtuve ok=%v err=%v", ok, err)
	}

	if err := e.SetMeta("k", "v1"); err != nil {
		t.Fatalf("SetMeta error: %v", err)
	}
	v, ok, err := e.GetMeta("k")
	if err != nil || !ok || v != "v1" {
		t.Fatalf("esperaba v1, obtuve v=%q ok=%v err=%v", v, ok, err)
	}

	// Sobreescribe.
	if err := e.SetMeta("k", "v2"); err != nil {
		t.Fatalf("SetMeta error: %v", err)
	}
	if v, _, _ := e.GetMeta("k"); v != "v2" {
		t.Errorf("esperaba v2 tras sobreescribir, obtuve %q", v)
	}
}

func TestMaintenanceDue(t *testing.T) {
	e := newTestEngine(t)

	// Sin registro previo: corresponde correr.
	due, err := e.MaintenanceDue(24)
	if err != nil || !due {
		t.Fatalf("sin marca previa debería estar due, obtuve due=%v err=%v", due, err)
	}

	// Tras marcar recién: no corresponde con intervalo de 24h.
	if err := e.MarkMaintenanceNow(); err != nil {
		t.Fatalf("MarkMaintenanceNow error: %v", err)
	}
	if due, _ := e.MaintenanceDue(24); due {
		t.Error("recién marcado no debería estar due con intervalo 24h")
	}

	// Con una marca vieja: vuelve a corresponder.
	if err := e.SetMeta(metaLastMaintenance, time.Now().UTC().Add(-48*time.Hour).Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	if due, _ := e.MaintenanceDue(24); !due {
		t.Error("con marca de 48h atrás debería estar due")
	}
}
