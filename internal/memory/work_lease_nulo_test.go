package memory

import "testing"

// UNA UNIDAD RECLAMADA SIN LEASE NO TIENE POR DÓNDE VOLVER.
//
// El reclamo considera huérfana a una unidad `claimed` cuyo lease VENCIÓ, y esa condición exige
// `lease_expires_at IS NOT NULL`. El dead-letter de agotadas la exige igual. O sea que una unidad
// `claimed` con el lease en NULL no la reclama nadie, no la cierra nadie, y `ReopenWorkUnit` sólo
// toca las `failed`: el único camino que queda es borrar el lote entero.
//
// ESE ESTADO EXISTE Y NO ES HIPOTÉTICO: la migración que agregó los leases hizo backfill de las
// unidades ya reclamadas copiando `claimed_by` a `owner_id` y dejando `lease_expires_at` en NULL a
// propósito, para no expropiar trabajo en curso durante el upgrade. La intención era buena y el
// efecto es permanente: si aquel dueño no volvió, la unidad quedó trabada para siempre.
//
// Y NADIE LO VE: ni el doctor, ni una métrica, ni una alerta miran `work_units`. Una unidad muerta
// se ve igual que una que está trabajando, que es el modo de falla de siempre en este repo.
//
// Sabotaje: devolverle al reclamo y al dead-letter el `lease_expires_at IS NOT NULL`, que es el
// estado anterior — la unidad sin lease vuelve a ser inmortal.
//
// arnes: archivo="internal/memory/work.go"
// arnes: de="(status=? OR (status=? AND (lease_expires_at IS NULL OR lease_expires_at < datetime('now'))))"
// arnes: a="(status=? OR (status=? AND lease_expires_at IS NOT NULL AND lease_expires_at < datetime('now')))"
// arnes: arreglo_de="(status=? OR (status=? AND (lease_expires_at IS NULL OR lease_expires_at < datetime('now'))))"
// arnes: arreglo_a="(status=? OR (status=? AND (lease_expires_at < datetime('now') OR lease_expires_at IS NULL)))"
func TestUnaUnidadReclamadaSinLeaseNoQuedaTrabadaParaSiempre(t *testing.T) {
	// CONTROL — una unidad con lease VIGENTE no se expropia. Sin esto, la guarda de abajo la
	// satisface hacer que cualquiera pueda robarle el trabajo a un dueño vivo.
	t.Run("control: un lease vigente no se expropia", func(t *testing.T) {
		e := newTestEngine(t)
		if _, err := e.CreateWorkBatch("b-ctrl", twoUnits()); err != nil {
			t.Fatal(err)
		}
		if _, ok, err := e.ClaimWorkUnit("b-ctrl", "agente-1", 300, 5); err != nil || !ok {
			t.Fatalf("el primer reclamo falló: ok=%v err=%v", ok, err)
		}
		// La segunda unidad sigue abierta, así que un segundo reclamo la toma a ella y NO le roba
		// la primera al dueño vivo.
		u2, ok, err := e.ClaimWorkUnit("b-ctrl", "agente-2", 300, 5)
		if err != nil || !ok {
			t.Fatalf("el segundo reclamo falló: ok=%v err=%v", ok, err)
		}
		if u2.OwnerID != "agente-2" || u2.Title != "unidad B" {
			t.Errorf("el segundo agente no tomó la unidad libre sino %q de %q: se expropió un lease vigente", u2.Title, u2.OwnerID)
		}
	})

	t.Run("reclamada con el lease en NULL", func(t *testing.T) {
		e := newTestEngine(t)
		if _, err := e.CreateWorkBatch("b1", twoUnits()); err != nil {
			t.Fatal(err)
		}
		u, ok, err := e.ClaimWorkUnit("b1", "agente-que-murio", 300, 5)
		if err != nil || !ok {
			t.Fatalf("no se pudo reclamar: ok=%v err=%v", ok, err)
		}
		// EL ESTADO DE LA MIGRACIÓN, montado a mano: reclamada y sin lease.
		if _, err := e.db.Exec(`UPDATE work_units SET lease_expires_at=NULL WHERE id=?`, u.ID); err != nil {
			t.Fatalf("no se pudo montar el estado: %v", err)
		}

		// 1 — ¿la puede retomar otro agente? Se pide del MISMO lote y se mira que vuelva ESA
		// unidad: la otra del lote sigue abierta, así que un `ok` a secas no probaría nada.
		var rescatada bool
		for i := 0; i < 2; i++ {
			v, ok, err := e.ClaimWorkUnit("b1", "agente-nuevo", 300, 5)
			if err != nil {
				t.Fatalf("reclamo: %v", err)
			}
			if !ok {
				break
			}
			if v.ID == u.ID {
				rescatada = true
			}
		}
		if !rescatada {
			t.Errorf("la unidad reclamada SIN lease no la puede retomar nadie: el reclamo exige "+
				"`lease_expires_at IS NOT NULL`, así que una unidad que quedó sin lease —el backfill "+
				"de la migración de leases— es inmortal. Ni el dead-letter la alcanza, ni ReopenWorkUnit "+
				"la toca (sólo reabre `failed`): el único camino es borrar el lote entero. id=%s", u.ID)
		}
	})
}
