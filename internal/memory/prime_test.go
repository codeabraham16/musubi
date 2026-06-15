package memory

import (
	"testing"
)

func TestPrimeContextRespetaBudget(t *testing.T) {
	e := newTestEngine(t)
	for i := 0; i < 10; i++ {
		id := "p" + string(rune('a'+i))
		if err := e.SaveObservation(id, "topic/x", "Observación de prueba número con bastante contenido para gastar varios tokens "+id, nil); err != nil {
			t.Fatal(err)
		}
	}
	res, err := e.PrimeContext(40)
	if err != nil {
		t.Fatalf("PrimeContext error: %v", err)
	}
	if res.UsedTokens > 40 {
		t.Errorf("used_tokens=%d excede el budget 40", res.UsedTokens)
	}
	if res.Count == 0 {
		t.Error("se esperaba al menos un item dentro del budget")
	}
	for _, it := range res.Items {
		if it.Gist == "" {
			t.Error("cada item de priming debe traer un gist")
		}
	}
}

func TestPrimeContextVacio(t *testing.T) {
	e := newTestEngine(t)
	res, err := e.PrimeContext(100)
	if err != nil {
		t.Fatalf("PrimeContext error en DB vacía: %v", err)
	}
	if res.Count != 0 || len(res.Items) != 0 {
		t.Errorf("DB vacía debe dar 0 items, obtuve %d", res.Count)
	}
}

func TestPrimeContextPriorizaSalience(t *testing.T) {
	e := newTestEngine(t)
	// Observación poco importante.
	if err := e.SaveObservationWithImportance("low", "topic/low", "dato menor irrelevante", 0.5, nil); err != nil {
		t.Fatal(err)
	}
	// Observación muy importante: debe aparecer primero / entrar con budget chico.
	if err := e.SaveObservationWithImportance("high", "topic/high", "decisión de arquitectura crítica del proyecto", 5.0, nil); err != nil {
		t.Fatal(err)
	}
	res, err := e.PrimeContext(200)
	if err != nil {
		t.Fatalf("PrimeContext error: %v", err)
	}
	if res.Count == 0 {
		t.Fatal("se esperaban items")
	}
	if res.Items[0].ID != "high" {
		t.Errorf("la observación más importante debe rankear primero, obtuve %q", res.Items[0].ID)
	}
}

func TestPrimeContextExcluyeArchivadas(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("vis", "topic/vis", "observación visible", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("arc", "topic/arc", "observación archivada", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE observations SET archived = 1 WHERE id = 'arc'`); err != nil {
		t.Fatal(err)
	}
	res, err := e.PrimeContext(500)
	if err != nil {
		t.Fatalf("PrimeContext error: %v", err)
	}
	for _, it := range res.Items {
		if it.ID == "arc" {
			t.Error("PrimeContext no debe incluir observaciones archivadas")
		}
	}
}
