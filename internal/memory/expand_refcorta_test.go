package memory

import "testing"

// La ref corta de 8 hex que emite el hook (ver IDCorto) tiene que resolver a la observación entera por
// musubi_memory_expand, o el agente no puede hidratar. Este es el extremo RESOLVEDOR del contrato; el
// extremo EMISOR lo fija TestElHookAcortaElUuidYDejaElResto en cmd/musubi.
//
// Sabotaje: si el prefijo se toma del tramo equivocado del id (substr(id,9,8)), la ref corta no
// matchea nada y la primera aserción cae.
// arnes: archivo="internal/memory/expand.go"
// arnes: de="substr(id,1,8) IN ("
// arnes: a="substr(id,9,8) IN ("
func TestExpandResuelveRefCorta(t *testing.T) {
	e := newTestEngine(t)
	const uuid = "2c580abd-1b3f-422c-8e5e-347eb568a584"
	if err := e.SaveObservation(uuid, "topic/x", "contenido completo de la obs", nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	// La ref corta (8 hex) resuelve a la observación entera.
	res, err := e.GetObservations([]string{"2c580abd"})
	if err != nil {
		t.Fatalf("GetObservations(ref corta): %v", err)
	}
	if len(res) != 1 || res[0].ID != uuid {
		t.Fatalf("la ref corta no resolvió al id completo: %+v", res)
	}
	if res[0].Content != "contenido completo de la obs" {
		t.Errorf("contenido equivocado: %q", res[0].Content)
	}
	// El id COMPLETO sigue resolviendo por el camino exacto (retrocompat).
	res2, err := e.GetObservations([]string{uuid})
	if err != nil || len(res2) != 1 || res2[0].ID != uuid {
		t.Fatalf("el id completo dejó de resolver: %+v err=%v", res2, err)
	}
	// Una ref corta que no matchea nada no trae nada (ni error).
	res3, err := e.GetObservations([]string{"ffffffff"})
	if err != nil {
		t.Fatalf("ref corta inexistente dio error: %v", err)
	}
	if len(res3) != 0 {
		t.Errorf("ref corta inexistente trajo %d obs", len(res3))
	}
}

func TestExpandRefCortaAmbiguaDevuelveSuperconjunto(t *testing.T) {
	e := newTestEngine(t)
	// Dos observaciones cuyo id comparte los primeros 8 hex y difiere después.
	a := "2c580abd-0000-0000-0000-000000000001"
	b := "2c580abd-0000-0000-0000-000000000002"
	if err := e.SaveObservation(a, "t", "obs A", nil); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if err := e.SaveObservation(b, "t", "obs B", nil); err != nil {
		t.Fatalf("save b: %v", err)
	}
	res, err := e.GetObservations([]string{"2c580abd"})
	if err != nil {
		t.Fatalf("GetObservations: %v", err)
	}
	// Superconjunto: devuelve LAS DOS. El agente recibe la que buscaba más una extra, nunca menos
	// (cero riesgo de perder info; la colisión de un prefijo de 32 bits es rara y acotada).
	if len(res) != 2 {
		t.Fatalf("la ref corta ambigua debía traer 2 (superconjunto), trajo %d: %+v", len(res), res)
	}
	got := map[string]bool{res[0].ID: true, res[1].ID: true}
	if !got[a] || !got[b] {
		t.Errorf("faltó una de las dos: %+v", res)
	}
}

func TestExpandDedupRefCortaMasCompleto(t *testing.T) {
	e := newTestEngine(t)
	const uuid = "2c580abd-1b3f-422c-8e5e-347eb568a584"
	if err := e.SaveObservation(uuid, "t", "única", nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Pedir la ref corta Y el id completo de la MISMA obs: se emite una sola vez (dedup por id).
	res, err := e.GetObservations([]string{"2c580abd", uuid})
	if err != nil {
		t.Fatalf("GetObservations: %v", err)
	}
	if len(res) != 1 {
		t.Errorf("esperaba 1 (dedup), obtuve %d: %+v", len(res), res)
	}
}

// Una ref corta ambigua BAJO PRESUPUESTO: el grupo de colisión entra entero, el budget no lo parte.
// Sin esto, con lugar para una sola, el empaquetado se quedaba con la de menor rowid (el SELECT no
// ordena) y perdía EN SILENCIO la que el agente vio en el hook. Es el extremo del principio «cero
// riesgo de perder info»: ante la colisión rara se prefiere exceder el presupuesto a perder la obs.
//
// Sabotaje: si el grupo ambiguo pasa por el chequeo normal de presupuesto (forzar=false), el segundo
// candidato no entra y este test cae.
// arnes: archivo="internal/memory/expand.go"
// arnes: de="agregar(o, true, true)"
// arnes: a="agregar(o, true, false)"
func TestExpandRefCortaAmbiguaBajoPresupuestoNoPierdeNada(t *testing.T) {
	e := newTestEngine(t)
	// Dos observaciones con el mismo prefijo de 8 hex y el MISMO cuerpo (así cada una cuesta lo mismo).
	a := "2c580abd-0000-0000-0000-000000000001"
	b := "2c580abd-0000-0000-0000-000000000002"
	const cuerpo = "contenido de una observación con varias palabras para estimar varios tokens"
	if err := e.SaveObservation(a, "t", cuerpo, nil); err != nil {
		t.Fatalf("save a: %v", err)
	}
	if err := e.SaveObservation(b, "t", cuerpo, nil); err != nil {
		t.Fatalf("save b: %v", err)
	}
	// Presupuesto que alcanza para UNA sola de las dos: sin el grupo indivisible, la segunda se pierde.
	budget := EstimateTokens(cuerpo)
	res, _, err := e.GetObservationsBudget([]string{"2c580abd"}, budget)
	if err != nil {
		t.Fatalf("GetObservationsBudget: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("la ref ambigua bajo presupuesto debía traer las 2 (grupo indivisible), trajo %d: %+v", len(res), res)
	}
	got := map[string]bool{res[0].ID: true, res[1].ID: true}
	if !got[a] || !got[b] {
		t.Errorf("faltó una de las dos: %+v", res)
	}
}

// FASE 1 (sólo resolvedor): sin ninguna ref corta, el empaquetado es BIT-IDÉNTICO a como venía
// —incluido el caso marginal de pedir el MISMO id dos veces, que devuelve la obs dos veces—. Así
// desplegar sólo el resolvedor es un no-op para todo caller existente (ninguno manda refs cortas
// todavía). El dedup es exclusivo del camino de prefijos, donde una ref corta y el id completo de la
// misma fila se pisan.
//
// Sabotaje: si el camino exacto deduplica (agregar con dedup=true), el id repetido vuelve una sola vez
// y este test cae.
// arnes: archivo="internal/memory/expand.go"
// arnes: de="agregar(o, false, false)"
// arnes: a="agregar(o, true, false)"
func TestExpandCaminoExactoNoDeduplica(t *testing.T) {
	e := newTestEngine(t)
	const uuid = "11111111-2222-3333-4444-555555555555"
	if err := e.SaveObservation(uuid, "t", "una obs", nil); err != nil {
		t.Fatalf("save: %v", err)
	}
	res, err := e.GetObservations([]string{uuid, uuid})
	if err != nil {
		t.Fatalf("GetObservations: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("el camino exacto debía preservar el id repetido (2), trajo %d: %+v", len(res), res)
	}
}
