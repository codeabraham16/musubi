package memory

import (
	"os"
	"path/filepath"
	"testing"
)

// simbolo_calificado_test.go cubre que el ancla de una observación acepte la MISMA clave que
// entrega el grafo de código (`Tipo.Metodo`), y que al hacerlo no funda métodos homónimos.
//
// EL PROBLEMA, medido el 2026-08-17: el grafo indexa `#method:DbEngine.AutoEmbedBackfill`, la
// doc de la tool recomienda «PREFERÍ EL SÍMBOLO», y el ancla RECHAZABA esa clave — sólo comía el
// nombre pelado. Dos subsistemas sin acordar qué ES un símbolo. Y el nombre pelado funde
// homónimos: en este repo hay 8 archivos con métodos de igual nombre (18 métodos, 7 fuera de tests).

// dosTiposConCierre es el caso que separa las dos formas: dos tipos, el mismo nombre de método.
const dosTiposConCierre = `package p

type A struct{}

func (a *A) Close() error {
	return nil
}

type B struct{}

func (b *B) Close() error {
	return errAlgo
}

func Close() error { return nil }
`

func archivoGo(t *testing.T, contenido string) (root, rel string) {
	t.Helper()
	root = t.TempDir()
	rel = "pkg/p.go"
	if err := os.MkdirAll(filepath.Join(root, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(contenido), 0o644); err != nil {
		t.Fatal(err)
	}
	return root, rel
}

// S1 — EL CASO QUE MOTIVÓ EL CAMBIO: la clave del grafo resuelve como ancla.
func TestS1LaClaveDelGrafoResuelveComoAncla(t *testing.T) {
	root, rel := archivoGo(t, dosTiposConCierre)

	fpA, err := symbolFingerprint(root, rel, "A.Close")
	if err != nil {
		t.Fatalf("`A.Close` debería resolver como ancla: %v", err)
	}
	if fpA == "" {
		t.Fatal("huella vacía")
	}
}

// ⚠️ S2 — Y NO FUNDE HOMÓNIMOS. `A.Close` y `B.Close` tienen que dar huellas DISTINTAS, y las dos
// distintas de `Close` pelado (que las cubre a todas). Si `A.Close` == `B.Close`, el calificador
// no se está usando y el ancla sigue marcando por cambios en código que la nota no describe.
func TestS2LasFormasCalificadasNoSeFunden(t *testing.T) {
	root, rel := archivoGo(t, dosTiposConCierre)

	fpA, err := symbolFingerprint(root, rel, "A.Close")
	if err != nil {
		t.Fatal(err)
	}
	fpB, err := symbolFingerprint(root, rel, "B.Close")
	if err != nil {
		t.Fatal(err)
	}
	pelado, err := symbolFingerprint(root, rel, "Close")
	if err != nil {
		t.Fatal(err)
	}
	if fpA == fpB {
		t.Error("`A.Close` y `B.Close` dieron la MISMA huella: el receptor no se está usando")
	}
	if pelado == fpA || pelado == fpB {
		t.Error("la forma pelada debería cubrir a TODOS los homónimos, no coincidir con uno solo")
	}
}

// ⚠️ S3 — LA FORMA PELADA NO SE TOCA, y esto es compatibilidad hacia atrás con DATOS, no con
// código: las anclas ya guardadas están escritas así. Si angostáramos el matcheo, observaciones
// que hoy resuelven bien pasarían a `missing` de un día para otro — una marca de rancio masiva y
// falsa, el ruido exacto que este mecanismo existe para no producir.
func TestS3LaFormaPeladaSigueResolviendo(t *testing.T) {
	root, rel := archivoGo(t, dosTiposConCierre)

	for _, s := range []string{"Close", "A", "B"} {
		if _, err := symbolFingerprint(root, rel, s); err != nil {
			t.Errorf("la forma pelada %q dejó de resolver: %v", s, err)
		}
	}
}

// S4 — Un receptor que no existe NO resuelve, y falla como «símbolo no encontrado» (deriva
// legítima) y no como error de E/S. `Z.Close` no está aunque `Close` sí.
func TestS4UnReceptorInexistenteNoResuelve(t *testing.T) {
	root, rel := archivoGo(t, dosTiposConCierre)

	if _, err := symbolFingerprint(root, rel, "Z.Close"); err == nil {
		t.Error("`Z.Close` no existe: no debería resolver")
	}
}

// S5 — La huella cambia cuando cambia EL MÉTODO ANCLADO, y NO cuando cambia su homónimo. Es la
// propiedad que hace útil a la marca; sin esto lo anterior es contabilidad de strings.
func TestS5LaHuellaSiguePrecisamenteAlMetodoAnclado(t *testing.T) {
	root, rel := archivoGo(t, dosTiposConCierre)
	antesA, _ := symbolFingerprint(root, rel, "A.Close")
	antesB, _ := symbolFingerprint(root, rel, "B.Close")

	// Se toca SÓLO el cuerpo de B.Close.
	modificado := `package p

type A struct{}

func (a *A) Close() error {
	return nil
}

type B struct{}

func (b *B) Close() error {
	panic("cambio SOLO en B")
}

func Close() error { return nil }
`
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), []byte(modificado), 0o644); err != nil {
		t.Fatal(err)
	}

	despuesA, _ := symbolFingerprint(root, rel, "A.Close")
	despuesB, _ := symbolFingerprint(root, rel, "B.Close")

	if despuesB == antesB {
		t.Error("cambió B.Close y su huella no se movió")
	}
	if despuesA != antesA {
		t.Error("cambió B.Close y se marcó A.Close: eso es la marca que salta de más")
	}
}
