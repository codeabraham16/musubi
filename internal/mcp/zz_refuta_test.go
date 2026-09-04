package mcp

import (
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

func TestZZCancelarCruzandoTenant(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "pc-gio", "tier": "A", "caps": []string{"metrics", "exec"},
		"project": "casa", "os": "linux", "arch": "amd64",
	}); e != nil {
		t.Fatalf("enroll casa: %+v", e)
	}
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "srv-prod", "tier": "A", "caps": []string{"metrics", "exec"},
		"project": "empresa", "os": "linux", "arch": "amd64",
	}); e != nil {
		t.Fatalf("enroll empresa: %+v", e)
	}
	dCasa, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	dEmp, _, _ := s.engine.DevicePorNombre("empresa", "srv-prod")
	t.Logf("casa=%s empresa=%s", dCasa.ID, dEmp.ID)

	ahora := time.Now().UTC()
	m, err := s.engine.AbrirMantenimiento(fleet.Mantenimiento{
		DeviceID: dEmp.ID, ProjectID: dEmp.ProjectID, Principal: "admin-empresa",
		Desde: ahora.Add(-time.Minute), Hasta: ahora.Add(time.Hour), Motivo: "migracion",
	})
	if err != nil {
		t.Fatalf("abrir: %v", err)
	}

	gio := &Principal{Name: "gio", ProjectID: "casa", Role: RoleContributor, Read: ReadOwn, Write: WriteOwn,
		Fleet: map[fleet.Cap][]string{fleet.CapMetrics: {"pc-gio"}}}

	// Control negativo: nombrar la maquina ajena directo.
	_, e := callAsPrincipal(t, s, gio, "musubi_fleet_maintenance", map[string]any{
		"project": "empresa", "device": "srv-prod", "minutos": 30,
	})
	t.Logf("control negativo: %+v", e)
	if e == nil {
		t.Fatalf("el control negativo paso: la compuerta no compuertea nada")
	}

	// El ataque: compuerta contra pc-gio, escribe la fila de srv-prod.
	res, e2 := callAsPrincipal(t, s, gio, "musubi_fleet_maintenance", map[string]any{
		"project": "casa", "device": "pc-gio", "cancelar": m.ID,
	})
	if e2 != nil {
		t.Logf("REFUTADO: el ataque fue rechazado: %+v", e2)
		return
	}
	t.Logf("ATAQUE ACEPTADO, respuesta: %s", textOf(t, res))

	set, _ := s.engine.DevicesEnMantenimiento(ahora)
	if set[dEmp.ID] {
		t.Logf("REFUTADO: la ventana de srv-prod sigue activa")
		return
	}
	t.Errorf("CONFIRMADO: se cancelo la ventana de srv-prod (proyecto empresa) desde un principal de casa")
}
