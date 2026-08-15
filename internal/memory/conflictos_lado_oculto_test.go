package memory

import "testing"

// ocultar marca una observación como fuera del recall por el motivo pedido. Los tres motivos son
// ejes distintos (archivado, reemplazado, en cuarentena) y visibleObsPredicate los junta: si el
// arreglo cubriera sólo uno, los otros dos seguirían filtrando.
func ocultar(t *testing.T, e *DbEngine, id, motivo string) {
	t.Helper()
	var q string
	switch motivo {
	case "archivada":
		q = `UPDATE observations SET archived=1 WHERE id=?`
	case "supersedida":
		q = `UPDATE observations SET superseded_by='otra' WHERE id=?`
	case "en cuarentena":
		q = `UPDATE observations SET quarantined=1 WHERE id=?`
	default:
		t.Fatalf("motivo desconocido: %s", motivo)
	}
	if _, err := e.db.Exec(q, id); err != nil {
		t.Fatalf("ocultar %s (%s): %v", id, motivo, err)
	}
}

// LA FUGA QUE ESTE TEST CIERRA. loadObsRow filtraba sólo `archived=0`, así que una observación
// SUPERSEDIDA o EN CUARENTENA seguía sirviendo de source y generaba pares nuevos para que alguien
// los arbitrara. Medido en el cerebro central: 12 pendientes con el source ya supersedido, la más
// nueva del mismo día en que se encontró — o sea la fuga estaba viva, no era residuo.
func TestUnaObservacionOcultaNoGeneraConflictosNuevos(t *testing.T) {
	for _, motivo := range []string{"archivada", "supersedida", "en cuarentena"} {
		t.Run(motivo, func(t *testing.T) {
			e := newTestEngine(t)
			if _, ok, err := e.loadObsRow("fuente"); err != nil || ok {
				t.Fatalf("estado inicial sucio: ok=%v err=%v", ok, err)
			}
			if err := e.SaveObservation("fuente", "tema/x", "el deploy del cerebro central usa restorecon", nil); err != nil {
				t.Fatal(err)
			}
			if _, ok, err := e.loadObsRow("fuente"); err != nil || !ok {
				t.Fatalf("una observación visible tiene que cargarse: ok=%v err=%v", ok, err)
			}

			ocultar(t, e, "fuente", motivo)

			_, ok, err := e.loadObsRow("fuente")
			if err != nil {
				t.Fatalf("loadObsRow: %v", err)
			}
			if ok {
				t.Errorf("una observación %s sigue sirviendo de source: va a generar pares que nadie puede aprovechar", motivo)
			}
		})
	}
}

// La contracara: una observación normal se sigue cargando igual. Apretar el filtro de más habría
// apagado la detección entera en silencio, que es peor que la fuga.
func TestLaObservacionVisibleSigueCargando(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("sana", "tema/y", "contenido cualquiera pero real", nil); err != nil {
		t.Fatal(err)
	}
	r, ok, err := e.loadObsRow("sana")
	if err != nil || !ok {
		t.Fatalf("la observación visible dejó de cargarse: ok=%v err=%v", ok, err)
	}
	if r.topicKey != "tema/y" {
		t.Errorf("se cargó mal: %+v", r)
	}
}

// LA OTRA MITAD DEL ARREGLO. La guarda impide que nazcan así, pero una pendiente puede ENVEJECER
// hasta acá: se detectó con los dos lados visibles y después alguien supersedió uno. El adjudicador
// la paga igual —lee las dos observaciones y gasta una llamada al motor— para producir un veredicto
// sobre memoria que el recall ya no muestra.
func TestLaPodaLevantaLasPendientesConUnLadoOculto(t *testing.T) {
	for _, caso := range []struct{ lado, motivo string }{
		{"source", "supersedida"},
		{"target", "supersedida"},
		{"target", "en cuarentena"},
	} {
		t.Run(caso.lado+" "+caso.motivo, func(t *testing.T) {
			e := newTestEngine(t)
			if err := e.SaveObservation("a", "tema/uno", "una nota sobre el deploy", nil); err != nil {
				t.Fatal(err)
			}
			if err := e.SaveObservation("b", "tema/uno", "otra nota sobre el mismo deploy", nil); err != nil {
				t.Fatal(err)
			}
			if _, err := e.UpsertObsRelation(ObsRelation{
				SourceID: "a", TargetID: "b", Relation: RelPending, Status: RelStatusPending,
			}); err != nil {
				t.Fatal(err)
			}

			// Con los dos lados visibles la relación es trabajo legítimo: NO se poda.
			if n, err := countStaleConflicts(e); err != nil || n != 0 {
				t.Fatalf("una pendiente con los dos lados visibles no es ruido: n=%d err=%v", n, err)
			}

			id := "a"
			if caso.lado == "target" {
				id = "b"
			}
			ocultar(t, e, id, caso.motivo)

			n, err := countStaleConflicts(e)
			if err != nil {
				t.Fatal(err)
			}
			if n != 1 {
				t.Errorf("con el %s %s la pendiente sigue en la cola (n=%d): el adjudicador la va a pagar para nada", caso.lado, caso.motivo, n)
			}
		})
	}
}
