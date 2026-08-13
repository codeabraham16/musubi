package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"musubi/internal/memory"
)

// frescura_cruza_maquinas_test.go — «no se sabe» no es «rancio».
//
// EL PORQUÉ (medido el 2026-08-12, estrenando musubi_recall_code contra el cerebro central). Tras
// federar los gists, el central pasó de 0 a 23 y la tool empezó a devolver contenido de verdad. Pero
// `fresh` venía SIEMPRE false, y no por rancio: la frescura se derivaba del disco DEL SERVIDOR, y el
// central no tiene el repo de altura. El contrato que la tool publica —«false ⇒ conviene re-leerlo»—
// mandaba al agente a re-leer el archivo siempre, que es exactamente lo único que la memoria de
// código existe para evitar. El gist federado quedaba informativo para un humano y sin valor para un
// agente que le hiciera caso.
//
// El arreglo no es adivinar mejor: es dejar de aplastar dos hechos distintos en un solo booleano.

// gistGuardado mete un gist a mano con la huella que se le pida, sin pasar por la tool de guardado
// (que derivaría la huella del disco local y no dejaría montar el caso federado).
func gistGuardado(t *testing.T, s *McpServer, path, huella string) {
	t.Helper()
	if err := s.engine.SaveCodeMemoryFrom("", memory.CodeMemory{
		Path:        memory.NormalizeCodePath(s.projectPath, path),
		Gist:        "gist de prueba",
		Symbols:     "Foo L1",
		Fingerprint: huella,
		Tokens:      3,
	}); err != nil {
		t.Fatalf("SaveCodeMemoryFrom: %v", err)
	}
}

// recordar llama a la tool y devuelve la respuesta ya parseada como la parsearía un cliente.
func recordar(t *testing.T, s *McpServer, args map[string]interface{}) struct {
	Found     bool   `json:"found"`
	Fresh     bool   `json:"fresh"`
	Freshness string `json:"freshness"`
	Gist      string `json:"gist"`
} {
	t.Helper()
	res, rerr := call(t, s, "musubi_recall_code", args)
	if rerr != nil {
		t.Fatalf("musubi_recall_code falló: %+v", rerr)
	}
	var out struct {
		Found     bool   `json:"found"`
		Fresh     bool   `json:"fresh"`
		Freshness string `json:"freshness"`
		Gist      string `json:"gist"`
	}
	txt := textOf(t, res)
	if err := json.Unmarshal([]byte(txt), &out); err != nil {
		t.Fatalf("un cliente no pudo parsear la respuesta: %v\n%s", err, txt)
	}
	return out
}

// ★ F1 — LOS TRES ESTADOS, JUNTOS.
//
// La tabla completa en una sola prueba a propósito: el valor de un tri-estado está en leer las tres
// filas de corrido. Separadas es como se cuela la que quedó al revés.
func TestLosTresEstadosDeLaFrescura(t *testing.T) {
	root := t.TempDir()
	s := newTestServerWithPath(t, root)

	visible := filepath.Join(root, "visible.go")
	if err := os.WriteFile(visible, []byte("package p\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	huellaReal, err := memory.FileFingerprint(root, "visible.go")
	if err != nil {
		t.Fatalf("FileFingerprint: %v", err)
	}

	casos := []struct {
		nombre   string
		path     string
		guardada string
		esperado string
		esperaOK bool
		porque   string
	}{
		{"el archivo no cambió", "visible.go", huellaReal, frescuraFresca, true,
			"mismo contenido que cuando se gisteó: el gist sirve tal cual"},
		{"el archivo cambió", "visible.go", "0000000000000000", frescuraRancia, false,
			"el servidor lo ve y no coincide: es un hecho medido, decir 'rancio' es correcto"},
		{"el servidor no ve el archivo", "de/otra/maquina.jsx", "aaaaaaaaaaaaaaaa", frescuraDesconocida, false,
			"EL CASO DEL CENTRAL: nadie miró el archivo, así que nadie puede afirmar que esté rancio"},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			gistGuardado(t, s, c.path, c.guardada)
			got := recordar(t, s, map[string]interface{}{"path": c.path})

			if !got.Found {
				t.Fatalf("el gist tiene que encontrarse igual: la frescura no decide si existe")
			}
			if got.Freshness != c.esperado {
				t.Errorf("freshness = %q, esperaba %q — %s", got.Freshness, c.esperado, c.porque)
			}
			if got.Fresh != c.esperaOK {
				t.Errorf("fresh = %v, esperaba %v", got.Fresh, c.esperaOK)
			}
		})
	}
}

// ★ F2 — LA REGRESIÓN MEDIDA: el gist federado NO se reporta rancio.
//
// Es el caso de altura contra el central, aislado. Si alguien vuelve a derivar la frescura del disco
// del servidor y nada más, esta prueba se pone roja: el estado cae a "stale" y el agente vuelve a
// re-leer un archivo que quizá no cambió nunca.
func TestElGistFederadoNoSeReportaRancioSinoDesconocido(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir()) // raíz vacía: el servidor NO tiene el repo del otro proyecto
	gistGuardado(t, s, "src/pages/CotizadorPrincipal.jsx", "863e9bf380f9bab0")

	got := recordar(t, s, map[string]interface{}{"path": "src/pages/CotizadorPrincipal.jsx"})

	if got.Freshness == frescuraRancia {
		t.Fatalf("el servidor afirmó 'stale' sobre un archivo que NO PUEDE VER: eso es inventar un hecho")
	}
	if got.Freshness != frescuraDesconocida {
		t.Errorf("freshness = %q, esperaba %q", got.Freshness, frescuraDesconocida)
	}
	if got.Gist == "" {
		t.Error("no poder verificar la frescura no es motivo para ocultar el gist")
	}
}

// ★ F3 — LA HUELLA DEL LLAMADOR LE GANA AL DISCO DEL SERVIDOR.
//
// Con el archivo VISIBLE para el servidor y una huella distinta mandada por el llamador, la respuesta
// tiene que seguir a quien miró el archivo de verdad. Si el código ignorara el parámetro, contestaría
// "fresh" leyendo su propio disco y esta prueba lo cazaría.
func TestLaHuellaDelLlamadorLeGanaAlDiscoDelServidor(t *testing.T) {
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	if err := os.WriteFile(filepath.Join(root, "compartido.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	huellaDelServidor, err := memory.FileFingerprint(root, "compartido.go")
	if err != nil {
		t.Fatalf("FileFingerprint: %v", err)
	}
	gistGuardado(t, s, "compartido.go", huellaDelServidor)

	// El llamador tiene OTRA versión del archivo en su máquina.
	distinta := recordar(t, s, map[string]interface{}{
		"path": "compartido.go", "fingerprint": "ffffffffffffffff",
	})
	if distinta.Freshness != frescuraRancia {
		t.Errorf("freshness = %q, esperaba %q: mandó su huella y no coincide, el disco del servidor no manda",
			distinta.Freshness, frescuraRancia)
	}
	if distinta.Fresh {
		t.Error("fresh = true: se ignoró la huella del llamador y se contestó con el disco propio")
	}

	// Y con la huella que sí coincide, el central puede afirmar frescura sin ver el archivo.
	igual := recordar(t, s, map[string]interface{}{
		"path": "compartido.go", "fingerprint": huellaDelServidor,
	})
	if igual.Freshness != frescuraFresca || !igual.Fresh {
		t.Errorf("con la huella coincidente esperaba fresh: freshness=%q fresh=%v", igual.Freshness, igual.Fresh)
	}
}

// ★ F4 — UN GIST SIN HUELLA ES DESCONOCIDO, NO RANCIO.
//
// FileFingerprint es best-effort al guardar: un gist puede haber quedado sin huella. Sin nada contra
// qué comparar, "stale" sería una afirmación que nadie midió — y encima haría re-leer para siempre un
// archivo que no se puede verificar jamás.
func TestGistSinHuellaEsDesconocidoNoRancio(t *testing.T) {
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	if err := os.WriteFile(filepath.Join(root, "sinhuella.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	gistGuardado(t, s, "sinhuella.go", "")

	got := recordar(t, s, map[string]interface{}{"path": "sinhuella.go"})
	if got.Freshness != frescuraDesconocida {
		t.Errorf("freshness = %q, esperaba %q", got.Freshness, frescuraDesconocida)
	}
	if got.Fresh {
		t.Error("fresh = true sin huella guardada: se afirmó identidad contra nada")
	}
}

// ★ F5 — COMPATIBILIDAD: `fresh` conserva su semántica exacta.
//
// El booleano viejo sigue siendo true SÓLO con identidad verificada. Un cliente que nunca oyó hablar
// de `freshness` tiene que comportarse igual que antes de este cambio, incluido el caso federado
// (donde antes daba false y tiene que seguir dando false).
func TestFreshMantieneSuSemanticaParaClientesViejos(t *testing.T) {
	root := t.TempDir()
	s := newTestServerWithPath(t, root)
	if err := os.WriteFile(filepath.Join(root, "local.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	huella, _ := memory.FileFingerprint(root, "local.go")
	gistGuardado(t, s, "local.go", huella)
	gistGuardado(t, s, "remoto/ajeno.jsx", "aaaaaaaaaaaaaaaa")

	if got := recordar(t, s, map[string]interface{}{"path": "local.go"}); !got.Fresh {
		t.Error("fresh = false sobre un archivo idéntico y visible: se rompió el camino del daemon local")
	}
	if got := recordar(t, s, map[string]interface{}{"path": "remoto/ajeno.jsx"}); got.Fresh {
		t.Error("fresh = true sobre un archivo que el servidor no puede ver: se aflojó la garantía del booleano")
	}
}
