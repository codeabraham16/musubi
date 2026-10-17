package deadcode

import (
	"strings"
	"testing"
)

// TestNormalizeProyectaIdentidadSinLineaNiColumna (R3): la identidad es `ruta<TAB>símbolo`, sin
// línea ni columna y con `\`→`/`. Es el detector del churn: la MISMA función en otra línea del
// mismo archivo tiene que dar la MISMA identidad, o el baseline se ensuciaría con cada movida.
func TestNormalizeProyectaIdentidadSinLineaNiColumna(t *testing.T) {
	enLinea10 := `internal\mcp\banco_diseno.go:10:6: unreachable func: Jaccard`
	enLinea99 := `internal\mcp\banco_diseno.go:99:6: unreachable func: Jaccard`

	a, err := Normalize(enLinea10)
	if err != nil {
		t.Fatalf("Normalize(linea10): %v", err)
	}
	b, err := Normalize(enLinea99)
	if err != nil {
		t.Fatalf("Normalize(linea99): %v", err)
	}
	quiero := "internal/mcp/banco_diseno.go\tJaccard"
	if len(a) != 1 || a[0] != quiero {
		t.Errorf("Normalize = %q; quería [%q]", a, quiero)
	}
	if len(b) != 1 || b[0] != a[0] {
		t.Errorf("la misma función en otra línea dio otra identidad: %q vs %q (R3 roto)", b, a)
	}
}

// TestNormalizeFallaCerrado (R2): una sola línea que no matchea el patrón aborta la normalización
// con error y sin resultado. Una comparación sobre una salida a medias es peor que ninguna.
func TestNormalizeFallaCerrado(t *testing.T) {
	crudo := strings.Join([]string{
		`internal/cerebro/alcance.go:63:6: unreachable func: Sondear`,
		`go: downloading something unexpected`, // basura: el analizador escupió algo raro
		`internal/mcp/banco_diseno.go:222:6: unreachable func: Jaccard`,
	}, "\n")

	ids, err := Normalize(crudo)
	if err == nil {
		t.Fatalf("Normalize no falló ante una línea basura; devolvió %q (R2: tiene que abortar)", ids)
	}
	if ids != nil {
		t.Errorf("Normalize devolvió %q además del error; tiene que ser nil (no comparar a medias)", ids)
	}
	if !strings.Contains(err.Error(), "línea 2") {
		t.Errorf("el error no señala la línea ofensora: %v", err)
	}
}

// TestNormalizeOrdenaYDeduplica (R3): la salida sale ordenada por bytes y sin repetidos, venga como
// venga la entrada. El orden LC_ALL=C pone las mayúsculas antes que las minúsculas.
func TestNormalizeOrdenaYDeduplica(t *testing.T) {
	crudo := strings.Join([]string{
		`x/b.go:2:1: unreachable func: zeta`,
		`x/a.go:1:1: unreachable func: Beta`,
		`x/b.go:2:1: unreachable func: zeta`, // duplicado exacto (otra corrida lo repite)
		`x/a.go:9:9: unreachable func: Alfa`,
	}, "\n")

	ids, err := Normalize(crudo)
	if err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	quiero := []string{
		"x/a.go\tAlfa",
		"x/a.go\tBeta",
		"x/b.go\tzeta",
	}
	if len(ids) != len(quiero) {
		t.Fatalf("Normalize = %q; quería %q", ids, quiero)
	}
	for i := range quiero {
		if ids[i] != quiero[i] {
			t.Errorf("orden/dedup en [%d]: %q; quería %q", i, ids[i], quiero[i])
		}
	}
}

// TestCompareDetectaAltas (R1): una función inalcanzable NUEVA, que no está en el baseline, sale
// como alta — es lo que hace fallar el gate.
func TestCompareDetectaAltas(t *testing.T) {
	base := []string{"a\tA", "b\tB"}
	actual := []string{"a\tA", "b\tB", "c\tC"}

	altas, bajas := Compare(base, actual)
	if len(altas) != 1 || altas[0] != "c\tC" {
		t.Errorf("altas = %q; quería [c\\tC]", altas)
	}
	if len(bajas) != 0 {
		t.Errorf("bajas = %q; quería vacío", bajas)
	}
}

// TestCompareDetectaBajas (R1): una del baseline que ya no aparece sale como baja (se conectó o se
// borró) — no falla el gate, sólo avisa. Es la dirección OPUESTA a las altas: invertirlas es el
// sabotaje clásico, y este test lo ataja.
func TestCompareDetectaBajas(t *testing.T) {
	base := []string{"a\tA", "b\tB", "c\tC"}
	actual := []string{"a\tA", "b\tB"}

	altas, bajas := Compare(base, actual)
	if len(altas) != 0 {
		t.Errorf("altas = %q; quería vacío (una baja NO es una alta)", altas)
	}
	if len(bajas) != 1 || bajas[0] != "c\tC" {
		t.Errorf("bajas = %q; quería [c\\tC]", bajas)
	}
}

// TestCompareIguales (R1): sin diferencias, nada que reportar.
func TestCompareIguales(t *testing.T) {
	base := []string{"a\tA", "b\tB"}
	altas, bajas := Compare(base, append([]string(nil), base...))
	if len(altas) != 0 || len(bajas) != 0 {
		t.Errorf("iguales dieron altas=%q bajas=%q; quería ambos vacíos", altas, bajas)
	}
}

// TestSanityBaseline (R7): el baseline tiene que venir ordenado-único y con forma; si no, el helper
// se niega a confiar en él.
func TestSanityBaseline(t *testing.T) {
	casos := []struct {
		nombre   string
		lineas   []string
		quieroOK bool
	}{
		{"ordenado y único", []string{"a\tA", "a\tB", "b\tA"}, true},
		{"vacío", nil, true},
		{"desordenado", []string{"b\tA", "a\tA"}, false},
		{"duplicado", []string{"a\tA", "a\tA"}, false},
		{"sin tab", []string{"a A"}, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			err := SanityBaseline(c.lineas)
			if c.quieroOK && err != nil {
				t.Errorf("SanityBaseline(%q) = %v; quería nil", c.lineas, err)
			}
			if !c.quieroOK && err == nil {
				t.Errorf("SanityBaseline(%q) = nil; quería error", c.lineas)
			}
		})
	}
}

// TestLineasYSerializarIdaYVuelta: Serializar produce lo que Lineas vuelve a leer, y el baseline
// resultante pasa su propia validación. Es la garantía de que --update siempre escribe algo sano.
func TestLineasYSerializarIdaYVuelta(t *testing.T) {
	ids := []string{"x/a.go\tAlfa", "x/b.go\tBeta"}
	texto := Serializar(ids)
	vuelta := Lineas(texto)
	if len(vuelta) != len(ids) {
		t.Fatalf("ida y vuelta: %q → %q", ids, vuelta)
	}
	for i := range ids {
		if vuelta[i] != ids[i] {
			t.Errorf("ida y vuelta [%d]: %q; quería %q", i, vuelta[i], ids[i])
		}
	}
	if err := SanityBaseline(vuelta); err != nil {
		t.Errorf("lo que escribe Serializar no pasa SanityBaseline: %v", err)
	}
	if Serializar(nil) != "" {
		t.Errorf("Serializar(nil) = %q; quería cadena vacía", Serializar(nil))
	}
}
