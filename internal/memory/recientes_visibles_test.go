package memory

import (
	"context"
	"testing"
)

// EL PANEL NO PUEDE PUBLICAR LO QUE EL RECALL NO DEVUELVE.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EL DEFECTO, MEDIDO CON CAMINOS PÚBLICOS (2026-09-11)
//
// `RecentObservations` filtraba con un `WHERE archived = 0` escrito a mano en vez del predicado
// canónico. Le faltaban las otras dos mitades —`superseded_by IS NULL` y `quarantined = 0`— y sus
// dos consumidores son superficies que SALEN: `dashboard.go` lo sirve por HTTP y `export.go` lo
// mete en el snapshot exportable.
//
// LA MITAD QUE IMPORTA ES LA CUARENTENA. Una observación propuesta por un LLM entra con procedencia
// `llm:<modelo>` y su contrato dice que «NO aparece en ningún recall»: la cuarentena existe
// exactamente para que lo que propuso un modelo no se lea como memoria hasta que una persona lo
// corrobore. El panel la dibujaba igual, y ahí ya no se distingue de una nota humana.
//
// Y EL MISMO JSON SE CONTRADECÍA SOLO: sin el arreglo, esta prueba mide `lista=3` en `out.Recent`
// contra `visible=1` en `out.Insights`, los dos campos del MISMO `/api/pulse`. El consumidor no
// tiene cómo saber cuál de los dos números creer.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// NO ERA LATENTE: MEDIDO CONTRA LA BASE REAL, EN SÓLO LECTURA (2026-09-11)
//
// Corriendo la query vieja contra `.musubi/memory.db` (1.971 observaciones sin archivar), el
// top-20 que sirven `dashboard.go:260` y `export.go:125` traía CUATRO filas que no debían estar:
// tres en cuarentena y una superada. Las tres de cuarentena eran las TRES PRIMERAS, creadas ese
// mismo día.
//
// Y ACÁ ESTÁ EL AGRAVANTE, QUE NO SE VE LEYENDO EL CÓDIGO: la query ordena por `created_at DESC`,
// y la cuarentena se llena de contenido NUEVO —cada `propose_observation` entra ahí—. O sea que lo
// cuarentenado no aparece disperso en el corpus: SE CONCENTRA ARRIBA, que es justo lo que el panel
// muestra. Medido: 8 filas invisibles sobre 1.971 es el 0,4% del corpus, y 4 sobre 20 es el 20%
// del panel. Casi cincuenta veces más denso en la única ventana que alguien mira.
//
// Cuanto más se usa `propose_observation`, más alto sale — y el aparato empuja a usarlo, porque el
// hook por sesión pide bajar lo durable cada varios turnos. El defecto se agrava solo con el uso
// normal del sistema.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ ESTA PRUEBA NO USA UN `UPDATE` CRUDO
//
// Sembrar el estado con SQL directo deja la puerta abierta al descarte más común —«ese estado no
// se puede dar por los caminos reales»— y este repo ya se comió una vez un fixture saneado aguas
// arriba. Acá la cuarentena la crea `ProposeObservation`, que es lo que corre detrás de
// `musubi_propose_observation`, y la supersesión la crea un veredicto `supersedes` de verdad sobre
// una relación de verdad. Los dos estados llegan como llegan en producción.
//
// ES EL QUINTO PREDICADO DE VISIBILIDAD ESCRITO A MANO de este repo, y los cuatro anteriores
// —SampleContents, FixtureDesdeDB, buildObsGraph y TopicExists— aparecieron en una sola rama. La
// causa raíz está dicha y no se resuelve con esta prueba: mientras la forma CALIFICADA que cada
// sitio necesita no exista, escribir el predicado a mano es la única salida y el defecto se repite.
//
// Sabotaje: devolverle a `RecentObservations` su `WHERE archived = 0`. Sale ROJA nombrando cuál de
// las dos clases se escapó.
func TestElPanelNoPublicaLoQueElRecallNoDevuelve(t *testing.T) {
	e := newTestEngine(t)
	ctx := context.Background()

	if err := e.SaveObservation("viva", "cuerpo/viva",
		"El panel de Musubi corre en loopback y sirve el latido cada cinco segundos.", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservation("vieja", "cuerpo/vieja",
		"El panel de Musubi sondeaba el snapshot completo de seiscientos kilobytes.", nil); err != nil {
		t.Fatal(err)
	}

	// CUARENTENA POR EL CAMINO PÚBLICO: esto es lo que corre detrás de musubi_propose_observation.
	idQ, err := e.ProposeObservation("", "", "cuerpo/propuesta",
		"El panel de Musubi filtra las memorias en cuarentena antes de dibujarlas.",
		"modelo-de-prueba", 0.4, "", nil)
	if err != nil {
		t.Fatalf("ProposeObservation: %v", err)
	}

	// SUPERSESIÓN POR EL CAMINO PÚBLICO: una relación de verdad y su veredicto.
	relID, err := e.UpsertObsRelation(ObsRelation{
		SourceID: "viva", TargetID: "vieja", Relation: RelPending, Status: RelStatusPending})
	if err != nil {
		t.Fatalf("UpsertObsRelation: %v", err)
	}
	if err := e.ResolveObsRelation(relID, RelSupersedes, "prueba", "la vieja quedó obsoleta"); err != nil {
		t.Fatalf("ResolveObsRelation: %v", err)
	}

	// CONTROL DE QUE EL ESCENARIO QUEDÓ ARMADO. Sin esto, un `ProposeObservation` que dejara de
	// cuarentenar —o un `supersedes` que dejara de ocultar— haría pasar esta prueba en verde por
	// no tener nada que esconder, que es el peor verde posible: el que mide otra cosa.
	res, err := e.Recall(ctx, "panel de Musubi", RecallOptions{TokenBudget: 4000, CandidatePool: 20})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("el escenario no quedó armado: el recall devolvió %d items y tenía que devolver 1 "+
			"(la viva). Si devolvió 3, ni la cuarentena ni la supersesión están ocultando, y esta "+
			"prueba no puede medir la brecha que existe para medir", len(res.Items))
	}

	recientes, err := e.RecentObservations(20)
	if err != nil {
		t.Fatalf("RecentObservations: %v", err)
	}

	var vioCuarentena, vioSuperada bool
	for _, c := range recientes {
		switch c.TopicKey {
		case "cuerpo/propuesta":
			vioCuarentena = true
		case "cuerpo/vieja":
			vioSuperada = true
		}
	}
	if vioCuarentena {
		t.Errorf("el panel publica la observación EN CUARENTENA (%s). Es contenido de un LLM sin "+
			"corroborar, y su contrato dice que no aparece en NINGÚN recall: dibujarla en el panel "+
			"—que además sale por HTTP y en el snapshot de export— la vuelve indistinguible de una "+
			"nota humana", idQ)
	}
	if vioSuperada {
		t.Error("el panel publica una observación SUPERADA: alguien juzgó que quedó obsoleta y el " +
			"panel la muestra igual, al lado de la que la reemplazó")
	}

	// LA CONTRADICCIÓN DENTRO DEL MISMO JSON, que es lo que lo vuelve indefendible: `out.Recent` y
	// `out.Insights` viajan juntos en /api/pulse.
	ins, err := e.InsightsCtx(ctx)
	if err != nil {
		t.Fatalf("InsightsCtx: %v", err)
	}
	if len(recientes) != ins.Observations.Visible {
		t.Errorf("el mismo /api/pulse lleva una lista de %d y un contador que dice visible=%d. "+
			"El consumidor no tiene cómo saber a cuál de los dos creerle",
			len(recientes), ins.Observations.Visible)
	}
}
