package main

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"musubi/internal/config"
	"musubi/internal/memory"
)

// EL DELTA ES POR SESIÓN.
//
// El estado de «memoria ya inyectada» vivía en UN solo par de claves de meta
// (loop_delta_session + loop_delta_injected) para toda la base: cada sesión que escribía le
// reiniciaba el estado a las demás, y el arranque de cualquier ventana lo vaciaba para todas.
// Medido el 2026-09-13 sobre 16 días de transcripts de esta PC: el 88% de las líneas de memoria
// que el hook por turno inyectó ya se habían inyectado antes en la misma sesión. En el 29% de
// esas repeticiones otra sesión del mismo proyecto había inyectado en el medio; otro 43% son
// compactaciones de la propia sesión, que re-inyectan a propósito porque el contexto es nuevo.

// Las dos direcciones del aislamiento: otra sesión en el medio no hace que s1 repita lo que ya
// vio, y lo que vio s1 no se le esconde a s2, que nunca lo tuvo en su contexto.
//
// Sabotaje que la pone roja: volver a una sola clave para todas las sesiones.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="return metaDeltaInjected + sepDeltaKey + sessionID"
// arnes: a="return metaDeltaInjected"
func TestElDeltaDeUnaSesionSobreviveAOtraSesion(t *testing.T) {
	store := &fakeTurnStore{meta: map[string]string{}}
	unoYDos := memory.RecallResult{Count: 2, Items: []memory.RecallItem{
		{ID: "x1", TopicKey: "t", Gist: "memoria uno", ContentHash: "h1"},
		{ID: "x2", TopicKey: "t", Gist: "memoria dos", ContentHash: "h2"},
	}}
	deOtraVentana := memory.RecallResult{Count: 1, Items: []memory.RecallItem{
		{ID: "y1", TopicKey: "t", Gist: "de la otra ventana", ContentHash: "h9"},
	}}
	turno := func(sesion string, r memory.RecallResult) string {
		store.recall = r
		in := `{"prompt":"qué sabemos","session_id":"` + sesion + `"}`
		return turnOutput(store, deltaLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in))
	}

	if out := turno("s1", unoYDos); !strings.Contains(out, "x1") || !strings.Contains(out, "x2") {
		t.Fatalf("el primer turno de s1 tiene que inyectar x1 y x2, inyectó: %q", out)
	}
	if out := turno("s2", deOtraVentana); !strings.Contains(out, "y1") {
		t.Fatalf("la otra sesión tiene que inyectar lo suyo, inyectó: %q", out)
	}
	if out := turno("s1", unoYDos); out != "" {
		t.Errorf("s1 ya había visto x1 y x2: que otra sesión inyecte en el medio no puede hacer que se repitan, y se inyectó:\n%s", out)
	}
	if out := turno("s2", unoYDos); !strings.Contains(out, "x1") || !strings.Contains(out, "x2") {
		t.Errorf("s2 nunca tuvo x1 ni x2 en su contexto: el delta de s1 no se los puede esconder, y s2 inyectó: %q", out)
	}
}

// El arranque o la compactación de OTRA sesión no le borra el delta a la primera; el de la propia
// sesión sí lo limpia, porque después de compactar el contexto ya no tiene lo inyectado.
//
// Sabotaje que la pone roja: que el arranque de la sesión no limpie su propio delta.
// arnes: archivo="cmd/musubi/detect.go"
// arnes: de="clearDeltaState(store, sessionID)"
// arnes: a="_ = sessionID"
func TestElArranqueDeOtraSesionNoBorraElDelta(t *testing.T) {
	turnos := &fakeTurnStore{meta: map[string]string{}, recall: memory.RecallResult{
		Count: 1,
		Items: []memory.RecallItem{{ID: "x1", TopicKey: "t", Gist: "memoria", ContentHash: "h1"}},
	}}
	in := `{"prompt":"q","session_id":"s1"}`
	turno := func() string {
		return turnOutput(turnos, deltaLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(in))
	}
	arranque := newFakeStore()
	arranque.meta = turnos.meta // la misma base

	if out := turno(); !strings.Contains(out, "x1") {
		t.Fatalf("el primer turno de s1 tiene que inyectar x1, inyectó: %q", out)
	}
	if _, err := buildHookOutput(t.TempDir(), arranque, defaultStartup(), "s2"); err != nil {
		t.Fatal(err)
	}
	if out := turno(); out != "" {
		t.Errorf("el arranque de s2 le borró el delta a s1 y x1 se volvió a inyectar:\n%s", out)
	}
	if _, err := buildHookOutput(t.TempDir(), arranque, defaultStartup(), "s1"); err != nil {
		t.Fatal(err)
	}
	if out := turno(); !strings.Contains(out, "x1") {
		t.Errorf("después del arranque de s1 la memoria tiene que volver a inyectarse, y el turno inyectó: %q", out)
	}
}

// Las claves por sesión no crecen sin límite: se conservan las maxDeltaSessions más recientes y
// las demás se vacían.
//
// Sabotaje que la pone roja: no podar nunca.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="if len(sesiones) > maxDeltaSessions {"
// arnes: a="if false && len(sesiones) > maxDeltaSessions {"
//
// Sabotaje que la pone roja: podar las más nuevas en vez de las más viejas.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="return ps[i].t < ps[j].t"
// arnes: a="return ps[i].t > ps[j].t"
func TestLasSesionesViejasDelDeltaSePodan(t *testing.T) {
	store := &fakeTurnStore{meta: map[string]string{}}
	total := maxDeltaSessions + 3
	for i := 0; i < total; i++ {
		id := "s" + strconv.Itoa(i)
		_ = store.SetMeta(deltaKey(id), `{"x":"h"}`)
		registrarSesionDelta(store, id, int64(1000+i))
	}

	var indice map[string]int64
	if err := json.Unmarshal([]byte(store.meta[metaDeltaSessions]), &indice); err != nil {
		t.Fatalf("el índice de sesiones del delta no es JSON: %v", err)
	}
	if len(indice) != maxDeltaSessions {
		t.Errorf("el índice tiene que quedar en %d sesiones, quedó en %d", maxDeltaSessions, len(indice))
	}
	for i := 0; i < 3; i++ {
		if v := store.meta[deltaKey("s"+strconv.Itoa(i))]; v != "" {
			t.Errorf("la sesión más vieja s%d tenía que quedar vaciada, y conserva %q", i, v)
		}
	}
	for i := 3; i < total; i++ {
		if v := store.meta[deltaKey("s"+strconv.Itoa(i))]; v == "" {
			t.Errorf("la sesión reciente s%d no tenía que podarse", i)
		}
	}
}
