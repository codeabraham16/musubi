package config

import (
	"strings"
	"testing"
)

// Invariantes del DIAL DE POTENCIA (F5). Ver specs/dial-y-telemetria-cognicion/spec.md.
// Cada test se verificó FALLANDO al sabotear la implementación (ver tasks.md).

func ptrBool(b bool) *bool { return &b }

// --- D1: sin `effort`, nada cambia ------------------------------------------------------------

func TestD1SinEffortLaConfigQuedaIgual(t *testing.T) {
	in := CognitionConfig{
		Provider:           "openai-compat",
		ReadTimeRerankTopK: 0,
		Cache:              CacheConfig{TTLSeconds: 0},
	}
	out, err := in.ApplyEffort()
	if err != nil {
		t.Fatalf("ApplyEffort: %v", err)
	}
	if out.ReadTimeRerank != nil {
		t.Errorf("D1: sin effort, read_time_rerank debe quedar SIN DECLARAR, quedó %v", *out.ReadTimeRerank)
	}
	if out.ReadTimeRerankTopK != 0 || out.Cache.TTLSeconds != 0 {
		t.Errorf("D1: sin effort no se toca nada; topK=%d ttl=%d", out.ReadTimeRerankTopK, out.Cache.TTLSeconds)
	}
	if out.Effort != "" {
		t.Errorf("D1: 'balanced' es el default DEL DIAL, no de Musubi; quedó %q", out.Effort)
	}
}

// --- D0: lo explícito le gana al preset --------------------------------------------------------

func TestD0LoExplicitoLeGanaAlPreset(t *testing.T) {
	// turbo querría prender el juez; el usuario lo apagó a mano y eso manda.
	in := CognitionConfig{Effort: EffortTurbo, ReadTimeRerank: ptrBool(false)}
	out, err := in.ApplyEffort()
	if err != nil {
		t.Fatalf("ApplyEffort: %v", err)
	}
	if out.ReadTimeRerank == nil || *out.ReadTimeRerank {
		t.Errorf("FUGA D0: turbo pisó un read_time_rerank:false explícito")
	}

	// Y al revés: eco querría apagarlo; el usuario lo prendió a mano.
	in2 := CognitionConfig{Effort: EffortEco, ReadTimeRerank: ptrBool(true)}
	out2, err := in2.ApplyEffort()
	if err != nil {
		t.Fatalf("ApplyEffort: %v", err)
	}
	if out2.ReadTimeRerank == nil || !*out2.ReadTimeRerank {
		t.Errorf("FUGA D0: eco pisó un read_time_rerank:true explícito")
	}

	// Mismo criterio para los numéricos.
	in3 := CognitionConfig{Effort: EffortTurbo, ReadTimeRerankTopK: 3, Cache: CacheConfig{TTLSeconds: 7}}
	out3, _ := in3.ApplyEffort()
	if out3.ReadTimeRerankTopK != 3 {
		t.Errorf("FUGA D0: turbo pisó top_k explícito, quedó %d", out3.ReadTimeRerankTopK)
	}
	if out3.Cache.TTLSeconds != 7 {
		t.Errorf("FUGA D0: turbo pisó ttl_seconds explícito, quedó %d", out3.Cache.TTLSeconds)
	}
}

// --- El preset SÍ llena lo que quedó sin declarar ---------------------------------------------

func TestElPresetLlenaLoQueFalta(t *testing.T) {
	casos := []struct {
		effort    string
		rerankOn  bool
		ttl       int
		topKTurbo bool
	}{
		{EffortEco, false, effortTTLEco, false},
		{EffortBalanced, false, effortTTLBalanced, false},
		{EffortTurbo, true, effortTTLTurbo, true},
	}
	for _, c := range casos {
		out, err := CognitionConfig{Effort: c.effort}.ApplyEffort()
		if err != nil {
			t.Fatalf("%s: %v", c.effort, err)
		}
		if out.ReadTimeRerank == nil || *out.ReadTimeRerank != c.rerankOn {
			t.Errorf("%s: read_time_rerank = %v, esperaba %v", c.effort, out.ReadTimeRerank, c.rerankOn)
		}
		if out.Cache.TTLSeconds != c.ttl {
			t.Errorf("%s: ttl = %d, esperaba %d", c.effort, out.Cache.TTLSeconds, c.ttl)
		}
		quiereTopK := 0
		if c.topKTurbo {
			quiereTopK = effortTurboTopK
		}
		if out.ReadTimeRerankTopK != quiereTopK {
			t.Errorf("%s: top_k = %d, esperaba %d", c.effort, out.ReadTimeRerankTopK, quiereTopK)
		}
	}
}

// `balanced` deja el juez APAGADO. Es el default y tiene que ser el que no sorprende: el juez es el
// seam de mayor riesgo (latencia en el camino caliente + rate-limit).
func TestBalancedNoPrendeElJuezDelRecall(t *testing.T) {
	out, _ := CognitionConfig{Effort: EffortBalanced}.ApplyEffort()
	if out.ReadTimeRerank == nil || *out.ReadTimeRerank {
		t.Errorf("balanced NO debe prender el juez del recall: un default que enciende el camino caliente es turbo con otro nombre")
	}
}

// Más potencia ⇒ TTL más CORTO. Parece al revés y es a propósito.
func TestMasPotenciaImplicaTTLMasCorto(t *testing.T) {
	eco, _ := CognitionConfig{Effort: EffortEco}.ApplyEffort()
	bal, _ := CognitionConfig{Effort: EffortBalanced}.ApplyEffort()
	tur, _ := CognitionConfig{Effort: EffortTurbo}.ApplyEffort()
	if !(eco.Cache.TTLSeconds > bal.Cache.TTLSeconds && bal.Cache.TTLSeconds > tur.Cache.TTLSeconds) {
		t.Errorf("el TTL debe DECRECER con la potencia: eco=%d bal=%d turbo=%d",
			eco.Cache.TTLSeconds, bal.Cache.TTLSeconds, tur.Cache.TTLSeconds)
	}
}

// --- D2: un effort desconocido es error --------------------------------------------------------

func TestD2EffortDesconocidoEsError(t *testing.T) {
	for _, malo := range []string{"turbol", "rapido", "ECO!", "max", "0"} {
		if _, err := NormalizeEffort(malo); err == nil {
			t.Errorf("D2: %q debía ser error y pasó", malo)
		}
		if _, err := (CognitionConfig{Effort: malo}).ApplyEffort(); err == nil {
			t.Errorf("D2: ApplyEffort aceptó %q", malo)
		} else if !strings.Contains(err.Error(), "cognition.effort") {
			t.Errorf("D2: el error debe nombrar el campo; obtuve %v", err)
		}
	}
}

// Adversarial: mayúsculas y espacios SÍ se toleran (es un enum, no un secreto).
func TestEffortToleraMayusculasYEspacios(t *testing.T) {
	for _, v := range []string{"  turbo", "TURBO", " Turbo ", "Eco", "BALANCED"} {
		got, err := NormalizeEffort(v)
		if err != nil {
			t.Errorf("%q debía normalizarse, obtuve %v", v, err)
			continue
		}
		if got != strings.ToLower(strings.TrimSpace(v)) {
			t.Errorf("%q normalizó a %q", v, got)
		}
	}
	if got, err := NormalizeEffort(""); err != nil || got != "" {
		t.Errorf("vacío debe quedar vacío (sin preset); obtuve %q / %v", got, err)
	}
}

// --- D3: el dial es puro -----------------------------------------------------------------------

func TestD3ElDialEsPuroYDeterminista(t *testing.T) {
	in := CognitionConfig{Effort: EffortTurbo, Provider: "openai-compat"}
	a, err1 := in.ApplyEffort()
	b, err2 := in.ApplyEffort()
	if err1 != nil || err2 != nil {
		t.Fatalf("errores: %v / %v", err1, err2)
	}
	if *a.ReadTimeRerank != *b.ReadTimeRerank || a.Cache.TTLSeconds != b.Cache.TTLSeconds || a.ReadTimeRerankTopK != b.ReadTimeRerankTopK {
		t.Errorf("D3: dos resoluciones de la MISMA config dieron distinto")
	}
	// Y no muta el receptor: ApplyEffort trabaja sobre una copia (receptor por valor).
	if in.ReadTimeRerank != nil {
		t.Errorf("D3: ApplyEffort mutó la config de entrada")
	}
}

// --- Resolución del puntero --------------------------------------------------------------------

func TestReadTimeRerankOnResuelveElPuntero(t *testing.T) {
	if (CognitionConfig{}).ReadTimeRerankOn() {
		t.Errorf("ausente ⇒ apagado (el juez nace apagado, invariante de F1)")
	}
	if (CognitionConfig{ReadTimeRerank: ptrBool(false)}).ReadTimeRerankOn() {
		t.Errorf("false ⇒ apagado")
	}
	if !(CognitionConfig{ReadTimeRerank: ptrBool(true)}).ReadTimeRerankOn() {
		t.Errorf("true ⇒ prendido")
	}
}
