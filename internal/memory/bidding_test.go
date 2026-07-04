package memory

import "testing"

// oneUnitBatch crea un batch con una sola unidad y devuelve (batchID, unitID).
func oneUnitBatch(t *testing.T, e *DbEngine) (string, string) {
	t.Helper()
	b, err := e.CreateWorkBatch("", []WorkUnitSpec{{Title: "u", Spec: "hacer algo"}})
	if err != nil {
		t.Fatalf("CreateWorkBatch: %v", err)
	}
	return b.BatchID, b.Units[0].ID
}

func unitStatus(t *testing.T, e *DbEngine, id string) (status, owner string) {
	t.Helper()
	if err := e.db.QueryRow(`SELECT status, COALESCE(owner_id,'') FROM work_units WHERE id=?`, id).Scan(&status, &owner); err != nil {
		t.Fatalf("leer unidad %s: %v", id, err)
	}
	return
}

// Escenario (a): la mejor oferta gana y la unidad queda claimed por ella con fencing_token.
func TestAwardBestBidWins(t *testing.T) {
	e := newTestEngine(t)
	_, unit := oneUnitBatch(t, e)
	for _, b := range []struct {
		agent string
		bid   float64
	}{{"ana", 0.4}, {"beto", 0.9}, {"caro", 0.7}} {
		if err := e.BidWorkUnit(unit, b.agent, b.bid, ""); err != nil {
			t.Fatalf("bid %s: %v", b.agent, err)
		}
	}
	u, winner, ok, err := e.AwardWorkUnit(unit, 300)
	if err != nil || !ok {
		t.Fatalf("award: ok=%v err=%v", ok, err)
	}
	if winner.Agent != "beto" || winner.Bid != 0.9 {
		t.Errorf("debe ganar beto (0.9), obtuve %+v", winner)
	}
	if u.Status != WorkClaimed || u.OwnerID != "beto" {
		t.Errorf("la unidad debe quedar claimed por beto, obtuve status=%q owner=%q", u.Status, u.OwnerID)
	}
	if u.FencingToken <= 0 {
		t.Errorf("el award debe entregar un fencing_token > 0, obtuve %d", u.FencingToken)
	}
}

// Escenario (b): re-bid — un agente sube su oferta y gana.
func TestReBidUpdatesAndWins(t *testing.T) {
	e := newTestEngine(t)
	_, unit := oneUnitBatch(t, e)
	e.BidWorkUnit(unit, "ana", 0.5, "")
	e.BidWorkUnit(unit, "beto", 0.6, "")
	// Ana re-oferta más alto.
	if err := e.BidWorkUnit(unit, "ana", 0.9, "mejoro mi oferta"); err != nil {
		t.Fatal(err)
	}
	bids, _ := e.WorkUnitBids(unit)
	if len(bids) != 2 {
		t.Fatalf("el re-bid no debe duplicar: esperaba 2 ofertas, obtuve %d", len(bids))
	}
	_, winner, ok, _ := e.AwardWorkUnit(unit, 300)
	if !ok || winner.Agent != "ana" || winner.Bid != 0.9 {
		t.Errorf("tras el re-bid debe ganar ana (0.9), obtuve %+v (ok=%v)", winner, ok)
	}
}

// Escenario (c): award sin ofertas → ok=false, unidad intacta.
func TestAwardNoBids(t *testing.T) {
	e := newTestEngine(t)
	_, unit := oneUnitBatch(t, e)
	_, _, ok, err := e.AwardWorkUnit(unit, 300)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("award sin ofertas debe dar ok=false")
	}
	if st, _ := unitStatus(t, e, unit); st != WorkOpen {
		t.Errorf("la unidad debe seguir open, está %q", st)
	}
}

// Escenario (d): no se puede ofertar sobre una unidad ya claimed.
func TestBidOnClaimedFails(t *testing.T) {
	e := newTestEngine(t)
	_, unit := oneUnitBatch(t, e)
	if _, _, err := e.ClaimWorkUnit("", "ana", 300, 5); err != nil {
		t.Fatal(err)
	}
	if err := e.BidWorkUnit(unit, "beto", 0.9, ""); err == nil {
		t.Error("ofertar sobre una unidad ya claimed debe fallar")
	}
}

// Escenario (e): doble award — el segundo no re-asigna.
func TestDoubleAwardIsNoop(t *testing.T) {
	e := newTestEngine(t)
	_, unit := oneUnitBatch(t, e)
	e.BidWorkUnit(unit, "ana", 0.5, "")
	if _, _, ok, _ := e.AwardWorkUnit(unit, 300); !ok {
		t.Fatal("el primer award debe adjudicar")
	}
	_, _, ok, err := e.AwardWorkUnit(unit, 300)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("un segundo award sobre una unidad ya adjudicada debe dar ok=false")
	}
}

// Escenario (f): empate de bid → gana el que ofertó primero (created_at ASC).
func TestAwardTieBreaksByEarliest(t *testing.T) {
	e := newTestEngine(t)
	_, unit := oneUnitBatch(t, e)
	// Mismo bid; forzamos created_at distintos para un desempate determinista.
	e.BidWorkUnit(unit, "ana", 0.8, "")
	e.BidWorkUnit(unit, "beto", 0.8, "")
	if _, err := e.db.Exec(`UPDATE work_bids SET created_at='2020-01-01 00:00:00' WHERE agent='ana' AND unit_id=?`, unit); err != nil {
		t.Fatal(err)
	}
	if _, err := e.db.Exec(`UPDATE work_bids SET created_at='2020-01-02 00:00:00' WHERE agent='beto' AND unit_id=?`, unit); err != nil {
		t.Fatal(err)
	}
	_, winner, ok, _ := e.AwardWorkUnit(unit, 300)
	if !ok || winner.Agent != "ana" {
		t.Errorf("en empate debe ganar el más antiguo (ana), obtuve %+v", winner)
	}
}

// Escenario (g): el ganador puede completar con owner + fencing_token (reusa el lease).
func TestAwardWinnerCanComplete(t *testing.T) {
	e := newTestEngine(t)
	_, unit := oneUnitBatch(t, e)
	e.BidWorkUnit(unit, "ana", 0.9, "")
	u, _, ok, _ := e.AwardWorkUnit(unit, 300)
	if !ok {
		t.Fatal("award debe adjudicar")
	}
	if err := e.CompleteWorkUnit(unit, "listo", WorkDone, u.OwnerID, u.FencingToken); err != nil {
		t.Errorf("el ganador debe poder completar con owner+fencing: %v", err)
	}
	if st, _ := unitStatus(t, e, unit); st != WorkDone {
		t.Errorf("la unidad debe quedar done, está %q", st)
	}
}

// Escenario (h): limpiar el batch cascada-borra las ofertas (FK ON DELETE CASCADE).
func TestClearBatchCascadesBids(t *testing.T) {
	e := newTestEngine(t)
	batch, unit := oneUnitBatch(t, e)
	e.BidWorkUnit(unit, "ana", 0.5, "")
	e.BidWorkUnit(unit, "beto", 0.7, "")
	if err := e.ClearWorkBatch(batch); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM work_bids WHERE unit_id=?`, unit).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("limpiar el batch debe borrar las ofertas (cascade), quedaron %d", n)
	}
}
