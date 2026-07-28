package memory

import (
	"testing"
)

// TestConsolidateNegationPolarityGuard prueba el guard de polaridad (auditoría, "medir"→"arreglar"):
// dos observaciones que empatan en trigramas por encima del umbral pero difieren en la cuenta de
// negadores explícitos ("usa X" vs "no usa X", sim=0.889 > 0.85) NO se fusionan — porque colapsarlas
// ocultaría del recall la polaridad opuesta. En cambio, dos casi-duplicados de la MISMA polaridad
// ("usa X en producción" vs "usa X en la producción", sim=0.852) SÍ se fusionan: el guard no
// introduce falsos negativos. Cubre solo la negación EXPLÍCITA; antónimos/negación implícita son el
// techo semántico model-free (Track 15).
func TestConsolidateNegationPolarityGuard(t *testing.T) {
	e := newTestEngine(t)

	// Mismo tenant: pos y pos2 son casi-dup (misma polaridad); neg es la polaridad opuesta.
	seedProj(t, e, "alpha", "pos", "arch/db", "usa Postgres en producción")
	seedProj(t, e, "alpha", "pos2", "arch/db", "usa Postgres en la producción")
	seedProj(t, e, "alpha", "neg", "arch/db", "no usa Postgres en producción")

	if _, err := e.Consolidate(0); err != nil { // 0 → umbral por defecto 0.85
		t.Fatalf("consolidate: %v", err)
	}

	archived := func(id string) int {
		t.Helper()
		var a int
		if err := e.db.QueryRow(`SELECT archived FROM observations WHERE id=?`, id).Scan(&a); err != nil {
			t.Fatalf("leer archived de %s: %v", id, err)
		}
		return a
	}

	// La negación NO debe fusionarse con ninguna afirmación: sigue viva y visible en recall.
	if archived("neg") != 0 {
		t.Errorf("la observación negada se archivó (fusionada por falso positivo de polaridad): archived=1")
	}
	// Las dos afirmaciones de misma polaridad SÍ se fusionan (exactamente una queda archivada):
	// el guard no bloquea duplicados legítimos.
	if archived("pos")+archived("pos2") != 1 {
		t.Errorf("los casi-dup de misma polaridad debían fusionarse (exactamente 1 archivado), pos=%d pos2=%d",
			archived("pos"), archived("pos2"))
	}
}
