package memory

import "testing"

// TestAutoHealRepairsLowRisk verifica que AutoHeal repara los checks de bajo riesgo
// (acá: orphan_relations) y persiste el reporte final en MetaLastHealth (T5.4).
func TestAutoHealRepairsLowRisk(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer e.Close()

	// Sembrar una relación huérfana (apunta a observaciones inexistentes).
	if _, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "fantasma", TargetID: "otro", Relation: RelPending, Status: RelStatusPending,
	}); err != nil {
		t.Fatalf("UpsertObsRelation: %v", err)
	}

	// Pre-condición: el check debe estar en no-ok.
	if c, _ := e.RunCheck("orphan_relations"); c.Status == "ok" {
		t.Fatal("pre-condición: orphan_relations debería estar en no-ok antes de curar")
	}

	final, err := e.AutoHeal()
	if err != nil {
		t.Fatalf("AutoHeal: %v", err)
	}

	for _, c := range final.Checks {
		if c.Code == "orphan_relations" && c.Status != "ok" {
			t.Errorf("orphan_relations debió auto-repararse, quedó %q: %s", c.Status, c.Message)
		}
	}

	if v, ok, _ := e.GetMeta(MetaLastHealth); !ok || v == "" {
		t.Error("AutoHeal debió persistir MetaLastHealth con el reporte final")
	}
}
