package memory

import (
	"strings"
	"testing"
)

// columnas devuelve los nombres de columna de una tabla.
func columnas(t *testing.T, e *DbEngine, tabla string) []string {
	t.Helper()
	rows, err := e.db.Query(`SELECT name FROM pragma_table_info(?)`, tabla)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}

// D7/R6 — LAS ANCLAS NO VIAJAN. Un fingerprint calculado en esta máquina no significa nada
// contra el checkout de otra: si cruzaran al cerebro central, el nodo remoto marcaría como
// rancia toda memoria cuyo archivo no exista en SU disco — es decir, casi toda.
//
// El outbox no guarda payload: sólo obs_id, y el payload se RECONSTRUYE desde la fila de
// observations al drenar. Entonces el invariante estructural es que ninguna de esas dos
// tablas tenga datos de ancla — así el payload no puede llevarlos por ninguna vía. Hoy sale
// gratis (las anclas viven en su propia tabla), pero es una propiedad EMERGENTE que nada
// defendía. Este test la vuelve explícita.
func TestAnclasNoViajanEnElSync(t *testing.T) {
	engine, _ := engineConArchivos(t, map[string]string{"src/a.go": "package a\n"})

	if err := engine.SaveObservationTypedWithOrigins("", "", "O1", "t/k", "nota compartida", 1.0, "", ScopeShared,
		[]string{"src/a.go"}, nil); err != nil {
		t.Fatalf("save shared con ancla: %v", err)
	}
	if contarAnclas(t, engine, "O1") != 1 {
		t.Fatal("pre-condición: la observación debía quedar anclada")
	}

	// Ni el outbox ni observations pueden exponer datos de ancla: de ahí sale el payload.
	for _, tabla := range []string{"outbox", "observations"} {
		for _, col := range columnas(t, engine, tabla) {
			lc := strings.ToLower(col)
			if strings.Contains(lc, "fingerprint") || strings.Contains(lc, "origin_path") {
				t.Errorf("la tabla %q expone %q: el payload de sync se arma de acá y las anclas son LOCALES a esta máquina", tabla, col)
			}
		}
	}

	// Y las anclas siguen viviendo sólo en su tabla, que el sync no toca.
	if cols := columnas(t, engine, "observation_origins"); len(cols) == 0 {
		t.Fatal("observation_origins debe existir")
	}
}
