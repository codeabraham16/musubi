package memory

import "testing"

// Los tests de la banda necesitan CONTROLAR el coseno, así que inyectan vectores a mano en vez de
// depender del embedder real: lo que se prueba es el RANGO, no la calidad del embedding.

// bandOpts: banda [0.80, 0.85), dedup a partir de 0.85.
func bandOpts() ConflictOptions {
	return ConflictOptions{CosineFloor: 0.85, BandFloor: 0.80}
}

// newBandEngine: un engine CON procedencia de vectores. Sin model_id no hay vector legible, y sin
// vector no hay banda (es una noción puramente vectorial).
func newBandEngine(t *testing.T) *DbEngine {
	t.Helper()
	e := newTestEngine(t)
	e.SetVectorModelID("static:tabla@banda")
	return e
}

// saveWithVec guarda una observación con un vector EXACTO (2 dimensiones alcanzan para fijar
// cualquier coseno: el ángulo lo es todo).
func saveWithVec(t *testing.T, e *DbEngine, id, topic, content string, vec []float32) {
	t.Helper()
	if err := e.SaveObservation(id, topic, content, vec); err != nil {
		t.Fatal(err)
	}
}

// vecAt devuelve un vector unitario que forma un coseno EXACTO con (1,0).
func vecAt(cos float64) []float32 {
	sin := 0.0
	if s := 1 - cos*cos; s > 0 {
		sin = sqrt(s)
	}
	return []float32{float32(cos), float32(sin)}
}

func sqrt(x float64) float64 {
	// Newton, para no importar math sólo por esto en un helper de test.
	z := x
	for i := 0; i < 40; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z
}

func countRelations(t *testing.T, e *DbEngine) int {
	t.Helper()
	var n int
	if err := e.db.QueryRow(`SELECT COUNT(*) FROM observation_relations`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestBandaMuestraPeroNoEncola es EL test de esta rebanada. Todo lo demás es comodidad; lo único
// que no se puede romper es que MOSTRAR NO SEA ENCOLAR. Por eso se verifica contra la BASE, no
// contra el valor de retorno.
func TestBandaMuestraPeroNoEncola(t *testing.T) { // S.a
	e := newBandEngine(t)
	saveWithVec(t, e, "vieja", "server/vpn", "NordVPN y Tailscale no pueden coexistir de forma confiable.", vecAt(1.0))
	antes := countRelations(t, e)

	saveWithVec(t, e, "nueva", "server/vpn-solucion", "Sí coexisten: hay una regla de firewall que lo resuelve.", vecAt(0.82))

	near, omitted, err := e.BandNeighbors("nueva", bandOpts())
	if err != nil {
		t.Fatalf("BandNeighbors error: %v", err)
	}
	if len(near) != 1 || near[0].ID != "vieja" {
		t.Fatalf("el vecino de la banda debía mostrarse, obtuve %+v", near)
	}
	if omitted != 0 {
		t.Errorf("no debería haber vecinos recortados, omitted=%d", omitted)
	}
	if got := countRelations(t, e); got != antes {
		t.Fatalf("MOSTRAR NO ES ENCOLAR: la banda persistió %d relación(es) (antes %d, ahora %d)",
			got-antes, antes, got)
	}
}

// TestBandaNoAvisaPorLoQueYaEstaEnLaCola es el test del defecto que encontró el PRIMER uso real:
// un par que entra a la cola por la vía LÉXICA, con coseno JUSTO por debajo del piso, caía igual en
// la banda ⇒ el agente recibía el mismo par avisado DOS VECES.
//
// La banda es el COMPLEMENTO de la cola: muestra lo que la cola NO muestra.
func TestBandaNoAvisaPorLoQueYaEstaEnLaCola(t *testing.T) { // S.a
	e := newBandEngine(t)
	// Textos MUY parecidos ⇒ léxico alto ⇒ el par entra a la cola por esa puerta...
	saveWithVec(t, e, "a", "notas/uno", "El detector de conflictos usa dos señales para emitir su veredicto.", vecAt(1.0))
	// ...y el coseno (0.82) lo pondría en la banda. No debe salir en las dos.
	saveWithVec(t, e, "b", "notas/dos", "El detector de conflictos usa dos señales para emitir un veredicto.", vecAt(0.82))

	rels, err := e.DetectRelations("b", bandOpts())
	if err != nil {
		t.Fatalf("DetectRelations error: %v", err)
	}
	if len(rels) == 0 {
		t.Fatal("precondición del test: el par DEBE entrar a la cola por la vía léxica")
	}

	near, _, err := e.BandNeighbors("b", bandOpts())
	if err != nil {
		t.Fatalf("BandNeighbors error: %v", err)
	}
	if len(near) != 0 {
		t.Fatalf("DOBLE AVISO: el par ya está en la cola (%d relación/es) y la banda lo muestra"+
			" igual. La banda es el COMPLEMENTO de la cola. Obtuve %+v", len(rels), near)
	}
}

func TestBandaNoAvisaDosVecesPorLoMismo(t *testing.T) { // S.b
	e := newBandEngine(t)
	saveWithVec(t, e, "a", "arch/db", "Usamos PostgreSQL como base principal.", vecAt(1.0))
	saveWithVec(t, e, "b", "arch/db2", "Usamos PostgreSQL como base principal del sistema.", vecAt(0.92))

	near, _, err := e.BandNeighbors("b", bandOpts())
	if err != nil {
		t.Fatalf("BandNeighbors error: %v", err)
	}
	// coseno 0.92 >= CosineFloor ⇒ ya es `pending` y el agente lo ve por el camino de siempre.
	if len(near) != 0 {
		t.Fatalf("por encima del piso del dedup NO debe avisarse además como vecino de banda: %+v", near)
	}
}

func TestBandaIgnoraLoQueEstaDebajoDelPiso(t *testing.T) { // S.c
	e := newBandEngine(t)
	saveWithVec(t, e, "a", "tema/uno", "Los gatos duermen mucho durante el día.", vecAt(1.0))
	saveWithVec(t, e, "b", "tema/dos", "El compilador de Go es notablemente rápido.", vecAt(0.60))

	near, _, err := e.BandNeighbors("b", bandOpts())
	if err != nil {
		t.Fatalf("BandNeighbors error: %v", err)
	}
	if len(near) != 0 {
		t.Fatalf("por debajo del piso de la banda no hay vecinos: %+v", near)
	}
}

func TestBandaApagada(t *testing.T) { // S.d y S.e
	e := newBandEngine(t)
	saveWithVec(t, e, "a", "server/vpn", "NordVPN y Tailscale no pueden coexistir.", vecAt(1.0))
	saveWithVec(t, e, "b", "server/vpn2", "Sí coexisten con una regla de firewall.", vecAt(0.82))

	t.Run("band_floor=0 apaga la banda (rollback por config)", func(t *testing.T) {
		near, _, err := e.BandNeighbors("b", ConflictOptions{CosineFloor: 0.85, BandFloor: 0})
		if err != nil {
			t.Fatal(err)
		}
		if len(near) != 0 {
			t.Fatalf("con BandFloor=0 la banda debe estar APAGADA: %+v", near)
		}
	})

	t.Run("sin coseno no hay banda", func(t *testing.T) {
		// CosineFloor=0 apaga el coseno ⇒ la banda, que es puramente vectorial, no puede existir.
		near, _, err := e.BandNeighbors("b", ConflictOptions{CosineFloor: 0, BandFloor: 0.80})
		if err != nil {
			t.Fatal(err)
		}
		if len(near) != 0 {
			t.Fatalf("sin coseno no hay banda: %+v", near)
		}
	})
}

func TestBandaRecortaYLoDICE(t *testing.T) { // S.g — el techo es honesto
	e := newBandEngine(t)
	saveWithVec(t, e, "src", "notas/src", "El detector de conflictos usa dos señales para decidir.", vecAt(1.0))
	// 5 vecinos en la banda, con un techo de 3. Los textos son LÉXICAMENTE AJENOS entre sí y al
	// source: si compartieran palabras entrarían a la COLA, y la banda —que es su complemento— los
	// excluiría con razón. La banda existe justo para lo que se dice con OTRAS palabras.
	textos := []string{
		"Mañana probablemente llueva sobre toda la región litoral.",
		"Las tortugas marinas migran miles de kilómetros cada temporada.",
		"El violonchelo barroco tenía cinco cuerdas y afinación distinta.",
		"La levadura fermenta mejor con una hidratación del setenta por ciento.",
		"Los volcanes islandeses expulsan basalto de baja viscosidad.",
	}
	for i, cos := range []float64{0.81, 0.83, 0.804, 0.845, 0.82} {
		saveWithVec(t, e, string(rune('a'+i)), "notas/otra", textos[i], vecAt(cos))
	}

	near, omitted, err := e.BandNeighbors("src", bandOpts())
	if err != nil {
		t.Fatalf("BandNeighbors error: %v", err)
	}
	if len(near) != maxBandNeighbors {
		t.Fatalf("se esperaban %d vecinos (el techo), obtuve %d", maxBandNeighbors, len(near))
	}
	if omitted != 2 {
		t.Errorf("el recorte debe INFORMARSE: esperaba omitted=2, obtuve %d", omitted)
	}
	// Los de MAYOR coseno primero: se muestra lo más parecido, no lo primero que salió.
	for i := 1; i < len(near); i++ {
		if near[i-1].Cosine < near[i].Cosine {
			t.Errorf("los vecinos deben venir ordenados por coseno descendente: %+v", near)
		}
	}
	if near[0].Cosine < 0.84 {
		t.Errorf("el primero debe ser el de mayor coseno (0.845), obtuve %.3f", near[0].Cosine)
	}
}

func TestBandaRespetaLasGuardasEstructurales(t *testing.T) { // S.h
	e := newBandEngine(t)
	saveWithVec(t, e, "spec", "sdd/mi-cambio/spec", "El detector debe proponer una relación pendiente.", vecAt(1.0))
	saveWithVec(t, e, "design", "sdd/mi-cambio/design", "El detector propone una relación pendiente al guardar.", vecAt(0.82))

	near, _, err := e.BandNeighbors("design", bandOpts())
	if err != nil {
		t.Fatalf("BandNeighbors error: %v", err)
	}
	if len(near) != 0 {
		t.Fatalf("sería absurdo sacar el ruido de la cola por una puerta y mostrárselo al agente por la otra: %+v", near)
	}
}
