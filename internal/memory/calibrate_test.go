package memory

import (
	"strings"
	"testing"
)

func TestConfigureDivisorsAffectsEstimate(t *testing.T) {
	defer ResetDivisors()
	base := EstimateTokens("abcdefgh") // prosa, 8 chars / 4 = 2
	if base != 2 {
		t.Fatalf("baseline esperado 2, obtuve %d", base)
	}
	ConfigureDivisors(8, 0, 0) // prosa a 8 chars/token -> 1
	if got := EstimateTokens("abcdefgh"); got != 1 {
		t.Errorf("tras calibrar prosa a 8, esperaba 1 token, obtuve %d", got)
	}
	ResetDivisors()
	if got := EstimateTokens("abcdefgh"); got != 2 {
		t.Errorf("tras reset esperaba 2, obtuve %d", got)
	}
}

func TestFitDivisorForSamples(t *testing.T) {
	// 8 chars no-CJK, 2 tokens reales -> divisor 4.0
	samples := []TokenSample{
		{Text: "abcdefgh", Kind: kindProse, Actual: 2},
		{Text: "abcdefgh", Kind: kindProse, Actual: 2},
	}
	if d := fitDivisorForSamples(samples); d < 3.9 || d > 4.1 {
		t.Errorf("esperaba divisor ~4.0, obtuve %v", d)
	}
}

func TestBuildCalibrationReport(t *testing.T) {
	counts := []TextCount{
		{Text: "una oración de prosa normal y corriente", Actual: 9},
		{Text: `{"a":1,"b":2}`, Actual: 9},
	}
	rep := BuildCalibrationReport(counts)
	if len(rep.PerKind) != 2 {
		t.Fatalf("esperaba 2 tipos en el reporte, obtuve %d", len(rep.PerKind))
	}
	for _, k := range rep.PerKind {
		if k.Samples == 0 || k.ActualTokens == 0 || k.SuggestedDivisor <= 0 {
			t.Errorf("entrada incompleta del reporte: %+v", k)
		}
	}
}

func TestSaveLoadDivisorsRoundtrip(t *testing.T) {
	e := newTestEngine(t)
	if err := e.SaveDivisors(3.0, 2.0, 1.5); err != nil {
		t.Fatalf("SaveDivisors error: %v", err)
	}
	p, c, j, ok, err := e.LoadDivisors()
	if err != nil || !ok {
		t.Fatalf("LoadDivisors ok=%v err=%v", ok, err)
	}
	if p != 3.0 || c != 2.0 || j != 1.5 {
		t.Errorf("divisores mal persistidos: %v %v %v", p, c, j)
	}
}

func TestRecomputeTokensAppliesNewDivisors(t *testing.T) {
	defer ResetDivisors()
	e := newTestEngine(t)
	content := "abcdefghabcdefgh" // 16 chars prosa -> /4 = 4 tokens
	if err := e.SaveObservation("c1", "t", content, nil); err != nil {
		t.Fatal(err)
	}
	var before int
	e.db.QueryRow(`SELECT tokens FROM observations WHERE id=?`, "c1").Scan(&before)
	if before != 4 {
		t.Fatalf("tokens inicial esperado 4, obtuve %d", before)
	}

	// Calibrar prosa a 8 chars/token y recomputar -> 16/8 = 2.
	if err := e.SaveDivisors(8, 0, 0); err != nil {
		t.Fatal(err)
	}
	ConfigureDivisors(8, 0, 0)
	if err := e.RecomputeTokens(); err != nil {
		t.Fatalf("RecomputeTokens error: %v", err)
	}
	var after int
	e.db.QueryRow(`SELECT tokens FROM observations WHERE id=?`, "c1").Scan(&after)
	if after != 2 {
		t.Errorf("tras recomputar con divisor 8 esperaba 2 tokens, obtuve %d", after)
	}
}

func TestEngineOpenAppliesCalibratedDivisors(t *testing.T) {
	defer ResetDivisors()
	root := t.TempDir()
	e, err := NewDbEngine(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.SaveDivisors(5.0, 3.0, 2.0); err != nil {
		t.Fatal(err)
	}
	e.Close()

	ResetDivisors() // simular proceso nuevo con divisores por defecto
	e2, err := NewDbEngine(root)
	if err != nil {
		t.Fatal(err)
	}
	defer e2.Close()
	p, c, j := CurrentDivisors()
	if p != 5.0 || c != 3.0 || j != 2.0 {
		t.Errorf("al abrir, los divisores calibrados deben aplicarse: %v %v %v", p, c, j)
	}
}

// TestSampleContentsNoSacaLoQueNoDebeSalir fija el contrato de EGRESO de SampleContents: lo que
// devuelve cruza la red hacia count_tokens de Anthropic, así que el filtro tiene que ser el
// predicado canónico de visibilidad MÁS la exclusión del scope local.
//
// Antes del fix el SELECT decía `archived = 0` escrito a mano, y dejaba pasar cuarentenadas,
// superseded y locales. Medido sobre la base real: 3 de las primeras 20 filas no debían salir.
func TestSampleContentsNoSacaLoQueNoDebeSalir(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer e.Close()

	// Los ids fijan el orden (SampleContents ordena por id) para que el test sea determinista.
	const bueno = "el unico contenido que tiene permitido cruzar la red hacia un tercero"
	casos := []struct {
		id, content string
		sale        bool
		prep        string // SQL que lo vuelve no-exportable
	}{
		{"a-visible-shared", bueno, true, ""},
		{"b-local", "secreto de esta maquina que no sale nunca de aca", false,
			`UPDATE observations SET scope = 'local' WHERE id = ?`},
		{"c-cuarentena", "texto no confiable escrito por un LLM sin corroborar", false,
			`UPDATE observations SET quarantined = 1 WHERE id = ?`},
		{"d-superseded", "version vieja que ya fue reemplazada por otra", false,
			`UPDATE observations SET superseded_by = 'a-visible-shared' WHERE id = ?`},
		{"e-archivada", "archivada, el unico caso que el filtro viejo si tapaba", false,
			`UPDATE observations SET archived = 1 WHERE id = ?`},
	}
	for _, c := range casos {
		if err := e.SaveObservationTyped(c.id, "calibracion/egreso", c.content, 1.0, "", "shared", nil); err != nil {
			t.Fatalf("SaveObservationTyped(%s): %v", c.id, err)
		}
		if c.prep != "" {
			if _, err := e.db.Exec(c.prep, c.id); err != nil {
				t.Fatalf("preparar %s: %v", c.id, err)
			}
		}
	}

	got, err := SampleContentsDe(e, 50)
	if err != nil {
		t.Fatalf("SampleContents: %v", err)
	}

	for _, c := range casos {
		presente := false
		for _, g := range got {
			if g == c.content {
				presente = true
				break
			}
		}
		if presente != c.sale {
			if c.sale {
				t.Errorf("%s: tenía que salir y no salió", c.id)
			} else {
				t.Errorf("%s: NO tenía que cruzar la red y salió (contenido %q)", c.id, c.content)
			}
		}
	}
	if len(got) != 1 {
		t.Errorf("esperaba exactamente 1 muestra exportable, obtuve %d: %q", len(got), got)
	}
}

// SampleContentsDe existe para que el test llame al método real sin depender de su receptor
// concreto, y para que quede obvio que el sujeto bajo prueba es SampleContents y no otra cosa.
func SampleContentsDe(e *DbEngine, limit int) ([]string, error) { return e.SampleContents(limit) }

// TestSampleContentsTapaSecretoDeFilaHistorica prueba la SEGUNDA línea del punto de egreso: el
// portero.
//
// Por qué la fila se siembra con UPDATE y no con SaveObservation: la guarda C2 tapa el contenido
// AL GUARDAR cuando el scope es 'shared', así que por la vía pública no se puede crear una fila
// shared con un secreto crudo. Ese es justamente el punto — el caso que el portero defiende NO es
// alcanzable escribiendo hoy, es una fila 'shared' escrita ANTES de que C2 existiera.
//
// Esto se descubrió corriendo el sabotaje: la primera versión de esta prueba guardaba con
// SaveObservation y quedaba VERDE con el portero sacado, porque el secreto nunca llegaba crudo a
// la base. Una guarda verde sobre su propio sabotaje es una garantía falsa.
func TestSampleContentsTapaSecretoDeFilaHistorica(t *testing.T) {
	e, err := NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatalf("NewDbEngine: %v", err)
	}
	defer e.Close()

	// LA MISMA CLAVE SINTÉTICA QUE USAN LOS OTROS SIETE HERMANOS DE ESTE REPO
	// (internal/memory/redact_guard_test.go, los cuatro de internal/mcp, y los dos gateways).
	// No es cosmético: la primera versión de esta prueba se inventó su propia clave —la de
	// ejemplo de la documentación de AWS con el último carácter cambiado— y la protección de
	// push de GitHub la marcó como «Amazon AWS Access Key ID», rechazando la rama entera. La
	// clave de la casa ya pasa esa aduana y ya está probada contra la regla `aws-access-key`.
	const secreto = "AKIA1234567890ABCDEF"
	const cola = " y despues correr el provisioning del cluster de produccion"

	if err := e.SaveObservationTyped("h-historica", "deploy/credenciales",
		"placeholder que se reemplaza abajo por el texto crudo de una fila vieja", 1.0, "", "shared", nil); err != nil {
		t.Fatalf("SaveObservationTyped: %v", err)
	}
	// La fila pre-C2: texto crudo, scope shared, visible.
	if _, err := e.db.Exec(`UPDATE observations SET content = ? WHERE id = 'h-historica'`,
		"credencial "+secreto+cola); err != nil {
		t.Fatalf("sembrar la fila histórica: %v", err)
	}

	got, err := e.SampleContents(50)
	if err != nil {
		t.Fatalf("SampleContents: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("esperaba 1 muestra, obtuve %d: %q", len(got), got)
	}
	if strings.Contains(got[0], secreto) {
		t.Errorf("el secreto de una fila histórica cruzaría la red en claro: %q", got[0])
	}
	// Tapar no es descartar: si el portero se comiera la muestra, el chequeo de arriba pasaría
	// por el motivo equivocado.
	if !strings.Contains(got[0], "provisioning del cluster") {
		t.Errorf("la muestra se perdió en vez de quedar tapada: %q", got[0])
	}
}
