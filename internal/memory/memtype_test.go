package memory

import "testing"

// memTypeOf lee el mem_type persistido de una observación (” si NULL).
func memTypeOf(t *testing.T, e *DbEngine, id string) string {
	t.Helper()
	var mt string
	if err := e.db.QueryRow(`SELECT COALESCE(mem_type,'') FROM observations WHERE id=?`, id).Scan(&mt); err != nil {
		t.Fatalf("leer mem_type de %s: %v", id, err)
	}
	return mt
}

// R2: normalización case-insensitive + trim; vacío/desconocido → "".
func TestNormalizeMemType(t *testing.T) {
	cases := map[string]string{
		"semantic":     MemSemantic,
		"Episodic":     MemEpisodic,
		"  PROCEDURAL": MemProcedural,
		"SeMaNtIc":     MemSemantic,
		"":             "",
		"foo":          "",
		"proc":         "", // no hacemos matching parcial
		"episodica":    "", // el enum es en inglés
	}
	for in, want := range cases {
		if got := normalizeMemType(in); got != want {
			t.Errorf("normalizeMemType(%q) = %q, esperaba %q", in, got, want)
		}
	}
}

// R5 (escenario g): pesos de saliencia por tipo.
func TestMemTypeSalienceWeight(t *testing.T) {
	cases := map[string]float64{
		MemEpisodic:   0.6,
		MemSemantic:   1.0,
		MemProcedural: 1.5,
		"":            1.0, // sin tipo = neutro
		"foo":         1.0, // desconocido = neutro
	}
	for mt, want := range cases {
		if got := memTypeSalienceWeight(mt); got != want {
			t.Errorf("memTypeSalienceWeight(%q) = %v, esperaba %v", mt, got, want)
		}
	}
}

// Escenarios a/b/c: persistencia del mem_type normalizado al guardar.
func TestSaveObservationTypedPersistsMemType(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservationTyped("a", "t", "un evento de hoy", 1.0, "Episodic", ScopeLocal, nil); err != nil {
		t.Fatal(err)
	}
	if got := memTypeOf(t, e, "a"); got != MemEpisodic {
		t.Errorf("(a) mem_type='Episodic' debe persistir 'episodic', obtuve %q", got)
	}
	// Sin tipo.
	if err := e.SaveObservationTyped("b", "t", "algo sin tipo", 1.0, "", ScopeLocal, nil); err != nil {
		t.Fatal(err)
	}
	if got := memTypeOf(t, e, "b"); got != "" {
		t.Errorf("(b) mem_type='' debe persistir '' (sin tipo), obtuve %q", got)
	}
	// Basura → sin tipo.
	if err := e.SaveObservationTyped("c", "t", "tipo invalido", 1.0, "foo", ScopeLocal, nil); err != nil {
		t.Fatal(err)
	}
	if got := memTypeOf(t, e, "c"); got != "" {
		t.Errorf("(c) mem_type basura debe persistir '', obtuve %q", got)
	}
}

// Escenario f: UPSERT por id actualiza el mem_type.
func TestSaveObservationTypedUpsertUpdatesMemType(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservationTyped("x", "t", "primero episodic", 1.0, MemEpisodic, ScopeLocal, nil); err != nil {
		t.Fatal(err)
	}
	if got := memTypeOf(t, e, "x"); got != MemEpisodic {
		t.Fatalf("mem_type inicial debe ser episodic, obtuve %q", got)
	}
	// Re-guardar el mismo id como procedural.
	if err := e.SaveObservationTyped("x", "t", "ahora procedural", 1.0, MemProcedural, ScopeLocal, nil); err != nil {
		t.Fatal(err)
	}
	if got := memTypeOf(t, e, "x"); got != MemProcedural {
		t.Errorf("UPSERT por id debe actualizar mem_type a procedural, obtuve %q", got)
	}
}

// Un guardado SIN tipo (vía histórica, memType='') NO debe borrar el mem_type ya fijado por
// un guardado tipado (preservación de clasificación en UPSERT).
func TestUntypedUpsertPreservesMemType(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservationTyped("k", "t", "clasificada procedural", 1.0, MemProcedural, ScopeLocal, nil); err != nil {
		t.Fatal(err)
	}
	// Update por la vía histórica (untyped): cambia contenido/importancia, NO el tipo.
	if err := e.SaveObservationWithImportance("k", "t", "contenido actualizado", 2.0, nil); err != nil {
		t.Fatal(err)
	}
	if got := memTypeOf(t, e, "k"); got != MemProcedural {
		t.Errorf("un update untyped debe preservar mem_type=procedural, obtuve %q", got)
	}
}

// Escenario e: decay diferencial. Tres observaciones idénticas salvo el tipo, a la misma
// edad; con MinSalience calibrado, sólo la episódica cae bajo el umbral.
func TestDecayDifferentialByMemType(t *testing.T) {
	e := newTestEngine(t)
	// A 30 días (half-life 30 => recency=0.5), con importancia 1 y 0 accesos (freq=1):
	//   episodic  = 0.5 * 0.6 = 0.30  -> < 0.4  => archivada
	//   semantic  = 0.5 * 1.0 = 0.50  -> >= 0.4 => se conserva
	//   procedural= 0.5 * 1.5 = 0.75  -> >= 0.4 => se conserva
	for _, tc := range []struct{ id, mt string }{
		{"epi", MemEpisodic}, {"sem", MemSemantic}, {"pro", MemProcedural},
	} {
		if err := e.SaveObservationTyped(tc.id, "t", "contenido "+tc.id, 1.0, tc.mt, ScopeLocal, nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := e.db.Exec(`UPDATE observations SET created_at = datetime('now','-30 days')`); err != nil {
		t.Fatal(err)
	}

	res, err := e.Decay(DecayOptions{HalfLifeDays: 30, MinSalience: 0.4, MinAgeDays: 14})
	if err != nil {
		t.Fatalf("Decay: %v", err)
	}
	if res.Archived != 1 {
		t.Fatalf("esperaba archivar sólo 1 (la episódica), archivé %d", res.Archived)
	}
	archived := func(id string) int {
		var a int
		if err := e.db.QueryRow(`SELECT archived FROM observations WHERE id=?`, id).Scan(&a); err != nil {
			t.Fatal(err)
		}
		return a
	}
	if archived("epi") != 1 {
		t.Error("la memoria episódica debe archivarse antes (peso 0.6)")
	}
	if archived("sem") != 0 || archived("pro") != 0 {
		t.Error("semantic (1.0) y procedural (1.5) NO deben archivarse a esta saliencia")
	}
}

// Escenario d (equivalencia): una observación SIN tipo decae exactamente como antes de B2.
// El peso neutro (1.0) hace que salience() con mem_type=” sea idéntica a la fórmula previa;
// aquí lo verificamos comparando una sin-tipo contra una 'semantic' (ambas peso 1.0): mismo
// destino de archivado a la misma edad/saliencia.
func TestDecayUntypedEqualsSemantic(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveObservation("untyped", "t", "sin tipo declarado", nil); err != nil {
		t.Fatal(err)
	}
	if err := e.SaveObservationTyped("semantic", "t", "tipo semantic", 1.0, MemSemantic, ScopeLocal, nil); err != nil {
		t.Fatal(err)
	}
	// A 400 días ambas caen muy por debajo de cualquier umbral razonable.
	if _, err := e.db.Exec(`UPDATE observations SET created_at = datetime('now','-400 days')`); err != nil {
		t.Fatal(err)
	}
	res, err := e.Decay(DecayOptions{HalfLifeDays: 30, MinSalience: 0.2, MinAgeDays: 14})
	if err != nil {
		t.Fatal(err)
	}
	if res.Archived != 2 {
		t.Errorf("sin-tipo y semantic (ambas peso 1.0) deben decaer igual: esperaba 2 archivadas, obtuve %d", res.Archived)
	}
}
