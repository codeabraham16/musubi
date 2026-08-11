package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// doctor_deep_visible_test.go — «una capacidad que no se anuncia no existe».
//
// EL CASO REAL QUE LO ORIGINA (2026-08-10): el modo rápido de musubi_doctor se implementó, se
// mergeó y se DESPLEGÓ en el cerebro central. El servidor lo aceptaba. Y sin embargo el sondeo del
// cuerpo seguía pagando los ~730 ms, porque `deep` no figuraba en el inputSchema de tools/list:
// ningún cliente MCP podía descubrirlo, y el que lee el catálogo concluye —con razón— que el modo
// rápido no existe.
//
// El default retrocompatible (deep ausente = full) es correcto y hay que conservarlo, pero es
// exactamente lo que vuelve el defecto invisible: nadie falla, nadie se entera, y la mejora no
// llega. Por eso hacen falta las DOS pruebas de abajo: que se anuncie, y que haga algo.

// checksDe extrae los códigos de check de una respuesta de musubi_doctor.
func checksDe(t *testing.T, out interface{}) []string {
	t.Helper()
	texto := out.(CallToolResponse).Content[0].Text
	var rep struct {
		Checks []struct {
			Code string `json:"code"`
		} `json:"checks"`
	}
	if err := json.Unmarshal([]byte(texto), &rep); err != nil {
		t.Fatalf("respuesta de doctor no parseable: %v\n%s", err, texto)
	}
	out2 := make([]string, 0, len(rep.Checks))
	for _, c := range rep.Checks {
		out2 = append(out2, c.Code)
	}
	if len(out2) == 0 {
		t.Fatalf("doctor no devolvió ningún check; la prueba no estaría midiendo nada:\n%s", texto)
	}
	return out2
}

func contiene(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// ★ D1 — EL MODO RÁPIDO SE ANUNCIA EN tools/list.
//
// Sin esto la capacidad es inalcanzable para cualquier cliente que se guíe por el catálogo, que es
// lo que hacen todos.
func TestElModoRapidoDelDoctorSeAnuncia(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())

	var doctor *Tool
	for i := range s.tools {
		if s.tools[i].Tool.Name == "musubi_doctor" {
			doctor = &s.tools[i].Tool
			break
		}
	}
	if doctor == nil {
		t.Fatal("musubi_doctor no está registrada")
	}

	prop, ok := doctor.InputSchema.Properties["deep"]
	if !ok {
		t.Fatal("musubi_doctor acepta 'deep' en el handler pero NO lo declara en su inputSchema: el modo rápido queda invisible para todo cliente MCP")
	}
	if prop.Type != "boolean" {
		t.Errorf("'deep' se anuncia como %q y el handler lo decodifica como *bool", prop.Type)
	}
	if !strings.Contains(strings.ToLower(prop.Description), "rápido") {
		t.Errorf("la descripción de 'deep' no dice para qué sirve; un flag sin porqué no se usa: %q", prop.Description)
	}
}

// ★ D2 — Y HACE ALGO: deep=false SALTEA LAS PASADAS CARAS.
//
// Anunciarlo sin que cambie el trabajo sería peor que no anunciarlo: prometería un ahorro que no
// ocurre. Las tres pasadas que el modo rápido evita son las que costaban los ~675 ms.
func TestDeepFalseSalteaLasPasadasCaras(t *testing.T) {
	s := newTestServerWithPath(t, t.TempDir())
	const (
		integridad = "db_integrity"
		fts        = "fts_consistency"
		gists      = "stale_gists"
	)

	full, rerr := call(t, s, "musubi_doctor", map[string]interface{}{})
	if rerr != nil {
		t.Fatalf("doctor sin args: %+v", rerr)
	}
	deFull := checksDe(t, full)
	for _, c := range []string{integridad, fts} {
		if !contiene(deFull, c) {
			t.Fatalf("el diagnóstico completo no trae %q, así que esta prueba no puede demostrar que el rápido lo saltee: %v", c, deFull)
		}
	}

	rapido, rerr := call(t, s, "musubi_doctor", map[string]interface{}{"deep": false})
	if rerr != nil {
		t.Fatalf("doctor con deep=false: %+v", rerr)
	}
	deRapido := checksDe(t, rapido)
	for _, c := range []string{integridad, fts, gists} {
		if contiene(deRapido, c) {
			t.Errorf("deep=false igual corrió %q: el modo rápido no está ahorrando nada", c)
		}
	}

	// Y NO ES UN DIAGNÓSTICO VACÍO: si saltearlo todo fuera la implementación, el ahorro sería
	// total y la señal, nula. Tiene que seguir informando de los checks baratos.
	if len(deRapido) == 0 {
		t.Error("deep=false no devolvió ningún check: un pulso que no mide nada no es un pulso")
	}

	// D2b — RETROCOMPAT: deep ausente ≡ deep=true. Los clientes viejos no pueden perder cobertura
	// en silencio, que es la forma en que un flag nuevo rompe a quien no lo conoce.
	explicito, rerr := call(t, s, "musubi_doctor", map[string]interface{}{"deep": true})
	if rerr != nil {
		t.Fatalf("doctor con deep=true: %+v", rerr)
	}
	if got, want := len(checksDe(t, explicito)), len(deFull); got != want {
		t.Errorf("deep ausente (%d checks) y deep=true (%d) tienen que ser el mismo diagnóstico", want, got)
	}
}
