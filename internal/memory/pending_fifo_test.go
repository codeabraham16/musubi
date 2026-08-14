package memory

// Tests del orden FIFO de la cola de relaciones pendientes (PendingQuery.MasViejasPrimero).
//
// Existen por una falla de sistema, no de código. La cola del cerebro central llegó a 405
// pendientes con la más vieja del 2026-07-29 sin que nadie la tocara, mientras un adjudicador
// corría por timer creyendo que iba al día. La causa NO era el adjudicador: los dos órdenes que
// existían —recencia y confianza— son ESTABLES, así que pedir «las primeras 30» devuelve siempre
// las mismas 30 y el fondo de la cola es inalcanzable POR CONSTRUCCIÓN.
//
// F1 reproduce esa inalcanzabilidad y la levanta. F3 protege el aviso, que es la otra mitad: un
// orden nuevo que el consumidor no sabe que existe no arregla nada.

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"musubi/internal/config"

	_ "modernc.org/sqlite"
)

// colaEnvejecida siembra n relaciones pendientes y les fija un updated_at distinto y creciente, de
// la más vieja a la más nueva. Hace falta envejecerlas a mano: UpsertObsRelation estampa
// CURRENT_TIMESTAMP, que tiene resolución de UN SEGUNDO, así que todo lo sembrado en un test empata
// y el orden no se podría observar. Devuelve el engine y los ids en orden de VIEJO a NUEVO.
func colaEnvejecida(t *testing.T, n int) (*DbEngine, []string) {
	t.Helper()
	dir := t.TempDir()
	e, err := NewDbEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close() })

	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		src, tgt := "src"+string(rune('a'+i)), "tgt"+string(rune('a'+i))
		for _, id := range []string{src, tgt} {
			if err := e.SaveObservation(id, "tema/"+id, "contenido de "+id, nil); err != nil {
				t.Fatalf("SaveObservation: %v", err)
			}
		}
		id, err := e.UpsertObsRelation(ObsRelation{
			SourceID: src, TargetID: tgt, Confidence: 0.50 + float64(i)*0.05, Status: RelStatusPending,
		})
		if err != nil {
			t.Fatalf("UpsertObsRelation: %v", err)
		}
		ids = append(ids, id)
	}

	// Segunda conexión al MISMO archivo para envejecer las filas. WAL lo permite.
	db, err := sql.Open("sqlite", filepath.Join(dir, config.DirName, config.DBFile))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// 2026-07-29 es la fecha real de la más vieja que quedó varada en el central. Un día por fila.
	for i, id := range ids {
		fecha := "2026-07-" + itoaDosDigitos(29+i) + " 10:00:00"
		if _, err := db.Exec(`UPDATE observation_relations SET updated_at=? WHERE id=?`, fecha, id); err != nil {
			t.Fatalf("envejecer la relación %s: %v", id, err)
		}
	}
	return e, ids
}

func itoaDosDigitos(n int) string {
	if n < 10 {
		return "0" + string(rune('0'+n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

func idsDePagina(p PendingPage) []string {
	out := make([]string, 0, len(p.Relations))
	for _, r := range p.Relations {
		out = append(out, r.ID)
	}
	return out
}

// F1 — EL TEST QUE IMPORTA. Con el orden por defecto y un tope, la misma página vuelve siempre y hay
// relaciones que NUNCA aparecen; con MasViejasPrimero, esas mismas son las primeras.
//
// No se afirma que «recencia esté mal»: se afirma que es ESTABLE, que es justo lo que la vuelve
// inservible para drenar. Las dos mitades van juntas porque por separado no dicen nada.
func TestLaColaViejaEraInalcanzableYAhoraNo(t *testing.T) {
	e, ids := colaEnvejecida(t, 6)
	ctx := context.Background()
	const tope = 3

	p1, err := e.PendingObsRelationsQueryCtx(ctx, PendingQuery{Limit: tope})
	if err != nil {
		t.Fatal(err)
	}
	p2, err := e.PendingObsRelationsQueryCtx(ctx, PendingQuery{Limit: tope})
	if err != nil {
		t.Fatal(err)
	}
	if p1.Count != len(ids) {
		t.Fatalf("count = %d, esperaba %d (el tope no debe afectar al conteo)", p1.Count, len(ids))
	}
	a, b := idsDePagina(p1), idsDePagina(p2)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("el orden por defecto no es estable en la posición %d; el test no probaría la inalcanzabilidad", i)
		}
	}

	// Las tres más VIEJAS son justo las que ese consumidor no ve nunca.
	vistas := map[string]bool{}
	for _, id := range a {
		vistas[id] = true
	}
	for _, id := range ids[:tope] {
		if vistas[id] {
			t.Fatalf("la relación vieja %s apareció en la página por recencia; la siembra no quedó ordenada", id)
		}
	}

	// FIFO: la primera página son exactamente las tres más viejas, en orden.
	fifo, err := e.PendingObsRelationsQueryCtx(ctx, PendingQuery{Limit: tope, MasViejasPrimero: true})
	if err != nil {
		t.Fatal(err)
	}
	got := idsDePagina(fifo)
	for i, id := range ids[:tope] {
		if got[i] != id {
			t.Errorf("FIFO posición %d: obtuve %s, esperaba %s (la más vieja primero)", i, got[i], id)
		}
	}
}

// F2 — FIFO es el inverso exacto de recencia. Guarda contra que quede implementado como un alias
// silencioso del default: la query aceptaría el campo, nadie se quejaría, y la cola seguiría igual
// de atascada. Es el modo de falla más probable de este cambio.
func TestFifoEsElInversoDeRecencia(t *testing.T) {
	e, _ := colaEnvejecida(t, 5)
	ctx := context.Background()
	rec, err := e.PendingObsRelationsQueryCtx(ctx, PendingQuery{})
	if err != nil {
		t.Fatal(err)
	}
	old, err := e.PendingObsRelationsQueryCtx(ctx, PendingQuery{MasViejasPrimero: true})
	if err != nil {
		t.Fatal(err)
	}
	a, b := idsDePagina(rec), idsDePagina(old)
	if len(a) != len(b) || len(a) == 0 {
		t.Fatalf("los dos órdenes deberían traer las mismas relaciones: %d contra %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[len(b)-1-i] {
			t.Fatalf("FIFO no es el inverso de recencia en la posición %d", i)
		}
	}
}

// F3 — el aviso de cola. Con la lista truncada en un orden estable hay que decir CUÁN VIEJA es la
// más vieja; con FIFO no, porque ya viene en la página y repetirlo sería ruido.
func TestElAvisoDeColaSoloApareceCuandoSirve(t *testing.T) {
	e, _ := colaEnvejecida(t, 5)
	ctx := context.Background()

	trunc, err := e.PendingObsRelationsQueryCtx(ctx, PendingQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if !trunc.Truncated {
		t.Fatal("con limit=2 sobre 5 esperaba Truncated")
	}
	if trunc.MasViejaPendiente != "2026-07-29 10:00:00" {
		t.Errorf("MasViejaPendiente = %q, esperaba la fecha de la más vieja sembrada", trunc.MasViejaPendiente)
	}

	fifo, err := e.PendingObsRelationsQueryCtx(ctx, PendingQuery{Limit: 2, MasViejasPrimero: true})
	if err != nil {
		t.Fatal(err)
	}
	if fifo.MasViejaPendiente != "" {
		t.Errorf("con FIFO el aviso sobra y vino %q", fifo.MasViejaPendiente)
	}

	// Sin tope no hay cola oculta: tampoco corresponde el aviso.
	entera, err := e.PendingObsRelationsQueryCtx(ctx, PendingQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if entera.Truncated || entera.MasViejaPendiente != "" {
		t.Errorf("sin límite no hay nada oculto: Truncated=%v MasViejaPendiente=%q", entera.Truncated, entera.MasViejaPendiente)
	}
}

// F4 — el aviso respeta los MISMOS filtros que la lista. Si `MIN(updated_at)` se calculara sobre la
// tabla entera, un consumidor que filtra por min_lex vería la fecha de una relación que su filtro
// excluye y saldría a buscar una cola que, para él, no existe.
func TestElAvisoRespetaLosFiltros(t *testing.T) {
	e, ids := colaEnvejecida(t, 4)
	ctx := context.Background()

	// Confianzas sembradas: 0.50, 0.55, 0.60, 0.65 de más vieja a más nueva. Con el umbral en 0.58
	// quedan las dos ÚLTIMAS, así que la más vieja que matchea es la tercera, no la primera.
	p, err := e.PendingObsRelationsQueryCtx(ctx, PendingQuery{MinConfidence: 0.58, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if p.Count != 2 {
		t.Fatalf("con min_confidence=0.58 esperaba 2 relaciones, obtuve %d", p.Count)
	}
	if p.MasViejaPendiente != "2026-07-31 10:00:00" {
		t.Errorf("MasViejaPendiente = %q; el aviso se calculó fuera del filtro (la más vieja de la tabla es la de %s)", p.MasViejaPendiente, ids[0])
	}
}
