package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// Las pruebas de este archivo salen de la revisión adversarial de `uso-agente --contexto`
// (2026-09-26): cada una custodia una regla que el instrumento de antes/después no puede perder sin
// que CI se entere, porque un «después» medido con otra regla no es comparable con el «antes».

// Sabotaje que la hace fallar: volver a publicar un número de resume.
// arnes: archivo="cmd/musubi/uso_agente_contexto.go"
// arnes: de="\t\tReanudaciones: Reanudaciones{Resume: motivoResumeSinMedir, Fork: motivoForkSinMedir}}\n"
// arnes: a="\t\tReanudaciones: Reanudaciones{Resume: \"0\", Fork: motivoForkSinMedir}}\n"
func TestMedirContextoResumeSaleSinMedir(t *testing.T) {
	// Las dos formas medidas en los reales. Un tramo reescrito pegado a una compactación, sin hueco
	// de tiempo: así son los 67 tramos de la historia de Musubi y Altura, y no hubo un solo /resume.
	// Y una sesión retomada tras el corte del 09-26, que no dejó tramo. El informe no puede decir
	// «resume 1» por la primera ni «resume 0» por la segunda: no lo mide, y lo dice.
	raiz := t.TempDir()
	dia := "2026-09-20"
	p1 := fxPrompt("p1", fxTS(dia, 1), "armá el deploy del central con la receta")
	h1 := fxHook("h1", fxTS(dia, 2), "UserPromptSubmit", "[Musubi — memoria relevante] Contexto.\n- a [id:1]")
	escribirTranscript(t, raiz, "-home-x/s1.jsonl",
		p1, h1,
		p1, h1, // el tramo reescrito, sin hueco
		map[string]any{"type": "system", "subtype": "stop_hook_summary", "uuid": "s1", "timestamp": fxTS(dia, 3)},
		fxBordeDeCompactacion("b1", fxTS(dia, 4)),
	)
	escribirTranscript(t, raiz, "-home-x/s2.jsonl",
		fxPrompt("q1", "2026-09-26T20:50:00.000Z", "armá el deploy del central con la receta"),
		fxPrompt("q2", "2026-09-26T22:12:00.000Z", "seguimos con el banco después del corte"),
	)

	var js, tabla, errOut bytes.Buffer
	if code := usoAgente([]string{"--dir", raiz, "--contexto", "--json"}, &js, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	var inf struct {
		Principal struct {
			Reanudaciones map[string]json.RawMessage `json:"reanudaciones"`
		} `json:"principal"`
	}
	if err := json.Unmarshal(js.Bytes(), &inf); err != nil {
		t.Fatalf("el --json no es JSON: %v", err)
	}
	for _, clave := range []string{"resume", "fork"} {
		var motivo string
		if json.Unmarshal(inf.Principal.Reanudaciones[clave], &motivo) != nil || !strings.HasPrefix(motivo, "sin medir") {
			t.Errorf("reanudaciones.%s = %s: quería «sin medir» con el motivo, no un número que el transcript "+
				"no sostiene", clave, inf.Principal.Reanudaciones[clave])
		}
	}
	if code := usoAgente([]string{"--dir", raiz, "--contexto"}, &tabla, &errOut); code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, errOut.String())
	}
	for _, l := range strings.Split(tabla.String(), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "resume") && !strings.Contains(l, "sin medir") {
			t.Errorf("la tabla dice %q: quería «sin medir»", l)
		}
	}
}
