package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/memory"
)

// conflicts_triage_test.go cubre los filtros de musubi_conflicts.
//
// El motivo de que existan está medido: en el cerebro central la cola llegó a 358 relaciones —77 KB
// por respuesta— y el panel del cuerpo la pedía ENTERA cada 4 segundos para leer un solo entero. La
// tool no aceptaba ni un parámetro, así que no había forma de pedir menos ni de mirar primero lo que
// importa.
//
// Lo que estos tests defienden no es «los filtros existen» sino las dos formas en que un filtro mal
// hecho miente: que `count` se achique al paginar, y que el recorte pase en silencio.

// pendientesEn siembra n relaciones pendientes con confianzas crecientes y devuelve el server.
func pendientesEn(t *testing.T, n int) *McpServer {
	t.Helper()
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { engine.Close() })
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})
	for i := 0; i < n; i++ {
		src, tgt := "s"+itoa(i), "t"+itoa(i)
		for _, id := range []string{src, tgt} {
			if err := engine.SaveObservation(id, "tema/"+id, "contenido de "+id, nil); err != nil {
				t.Fatalf("SaveObservation: %v", err)
			}
		}
		// Confianzas 0.50, 0.55, 0.60... para poder filtrar y ordenar de forma predecible.
		conf := 0.50 + float64(i)*0.05
		if _, err := engine.UpsertObsRelation(memory.ObsRelation{
			SourceID: src, TargetID: tgt, Confidence: conf, Status: memory.RelStatusPending}); err != nil {
			t.Fatalf("UpsertObsRelation: %v", err)
		}
	}
	return s
}

type respConflictos struct {
	Count     int                  `json:"count"`
	Relations []memory.ObsRelation `json:"relations"`
	Truncated bool                 `json:"truncated"`
	CountOnly bool                 `json:"count_only"`
}

func pedirConflictos(t *testing.T, s *McpServer, args map[string]interface{}) respConflictos {
	t.Helper()
	res, e := call(t, s, "musubi_conflicts", args)
	if e != nil {
		t.Fatalf("musubi_conflicts %v: %+v", args, e)
	}
	var r respConflictos
	if err := json.Unmarshal([]byte(textOf(t, res)), &r); err != nil {
		t.Fatalf("respuesta no parseable: %v", err)
	}
	return r
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// T1: sin argumentos, IDÉNTICO a como era antes. Es el contrato de compatibilidad: hay clientes
// desplegados (el cuerpo) que llaman sin parámetros y no pueden cambiar de comportamiento.
func TestConflictsSinArgumentosDevuelveTodo(t *testing.T) {
	s := pendientesEn(t, 6)
	r := pedirConflictos(t, s, map[string]interface{}{})
	if r.Count != 6 || len(r.Relations) != 6 {
		t.Errorf("sin argumentos debe venir la cola completa: count=%d relaciones=%d", r.Count, len(r.Relations))
	}
	if r.Truncated {
		t.Error("sin limit no hay nada truncado")
	}
}

// T2: LA TRAMPA. Con `limit`, `count` tiene que seguir siendo el TOTAL.
//
// Si el conteo viniera truncado, un panel que muestra «N conflictos» y además pagina mostraría
// siempre el tamaño de la página. El indicador se achicaría solo al paginar, y nadie lo notaría
// porque el número seguiría siendo plausible.
func TestConflictsCountEsElTotalAunqueSeTrunque(t *testing.T) {
	s := pendientesEn(t, 10)
	r := pedirConflictos(t, s, map[string]interface{}{"limit": 3})
	if len(r.Relations) != 3 {
		t.Errorf("limit=3 debe devolver 3 relaciones, devolvió %d", len(r.Relations))
	}
	if r.Count != 10 {
		t.Errorf("count debe ser el TOTAL (10), no el tamaño de la página: %d", r.Count)
	}
	if !r.Truncated {
		t.Error("si se recortó, hay que decirlo con truncated=true: un tope silencioso se lee como «no hay más»")
	}
	// Y sin recorte, la bandera NO aparece.
	completa := pedirConflictos(t, s, map[string]interface{}{"limit": 50})
	if completa.Truncated {
		t.Error("con limit mayor que el total no hay truncamiento")
	}
}

// T3: count_only no trae relaciones, y el número sigue siendo correcto. Es el caso del panel.
func TestConflictsCountOnlyNoTraeLaLista(t *testing.T) {
	s := pendientesEn(t, 7)
	r := pedirConflictos(t, s, map[string]interface{}{"count_only": true})
	if r.Count != 7 {
		t.Errorf("count_only debe seguir contando bien: %d", r.Count)
	}
	if len(r.Relations) != 0 {
		t.Errorf("count_only no debe traer relaciones, trajo %d", len(r.Relations))
	}
	if !r.CountOnly {
		t.Error("la respuesta debe declararse count_only: un array vacío sin bandera se lee como «no hay conflictos»")
	}
}

// T4: min_confidence filtra, y el conteo refleja el FILTRO (no el total sin filtrar). Son dos
// números distintos y confundirlos haría que el panel mostrara más de lo que puede listar.
func TestConflictsFiltraPorConfianza(t *testing.T) {
	s := pendientesEn(t, 6) // confianzas 0.50 .. 0.75
	r := pedirConflictos(t, s, map[string]interface{}{"min_confidence": 0.65})
	if len(r.Relations) != 3 {
		t.Fatalf("con umbral 0.65 quedan 0.65/0.70/0.75 = 3, quedaron %d", len(r.Relations))
	}
	if r.Count != 3 {
		t.Errorf("con filtro, count cuenta lo FILTRADO: %d", r.Count)
	}
	for _, rel := range r.Relations {
		if rel.Confidence < 0.65 {
			t.Errorf("se coló una de confianza %v", rel.Confidence)
		}
	}
}

// T5: el orden por confianza ordena de verdad. Un parámetro de orden que no ordena es peor que no
// tenerlo: el que lo pide cree estar mirando lo más fuerte primero.
func TestConflictsOrdenPorConfianza(t *testing.T) {
	s := pendientesEn(t, 5)
	r := pedirConflictos(t, s, map[string]interface{}{"order": "confidence", "limit": 3})
	if len(r.Relations) != 3 {
		t.Fatalf("esperaba 3, hubo %d", len(r.Relations))
	}
	for i := 1; i < len(r.Relations); i++ {
		if r.Relations[i].Confidence > r.Relations[i-1].Confidence {
			t.Errorf("orden por confianza roto: %v después de %v",
				r.Relations[i].Confidence, r.Relations[i-1].Confidence)
		}
	}
	// La más alta sembrada es 0.70 (0.50 + 4*0.05); con order=confidence tiene que venir primera.
	if r.Relations[0].Confidence < 0.69 {
		t.Errorf("la primera debería ser la de mayor confianza, vino %v", r.Relations[0].Confidence)
	}
}

// T6: los argumentos inválidos se rechazan en vez de interpretarse. Un `order` con typo que cayera
// silenciosamente en el default dejaría al que lo pidió creyendo que ordenó.
func TestConflictsRechazaArgumentosInvalidos(t *testing.T) {
	s := pendientesEn(t, 2)
	casos := []map[string]interface{}{
		{"limit": -1},
		{"min_confidence": 1.5},
		{"min_confidence": -0.2},
		{"order": "confianza"},
	}
	for _, c := range casos {
		if _, e := call(t, s, "musubi_conflicts", c); e == nil {
			t.Errorf("%v debería rechazarse", c)
		} else if e.Code != codeInvalidParams {
			t.Errorf("%v: code=%d, esperaba invalid params", c, e.Code)
		}
	}
}

// T7: los filtros NO rompen el aislamiento por proyecto. Es la guarda que un parámetro nuevo puede
// tirar sin querer: si el WHERE del scope se pierde al armar la consulta filtrada, un tenant vería
// la cola de otro.
func TestConflictsFiltradoSigueAcotadoAlProyecto(t *testing.T) {
	engine, err := memory.NewDbEngine(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer engine.Close()
	engine.SetProjectID("")
	s := NewMcpServer(engine, t.TempDir(), embedding.NoopProvider{})

	if err := engine.SaveObservationTypedFrom("web", "", "w1", "web/a", "cosa de web", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	if err := engine.SaveObservationTypedFrom("web", "", "w2", "web/b", "otra cosa de web", 1.0, "semantic", "local", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.UpsertObsRelation(memory.ObsRelation{
		SourceID: "w1", TargetID: "w2", Confidence: 0.9, Status: memory.RelStatusPending}); err != nil {
		t.Fatal(err)
	}

	pedirComo := func(p *Principal, args map[string]interface{}) respConflictos {
		t.Helper()
		raw, _ := json.Marshal(args)
		params, _ := json.Marshal(CallToolRequest{Name: "musubi_conflicts", Arguments: raw})
		out, rpcErr := s.handleToolsCall(withPrincipal(context.Background(), p), params)
		if rpcErr != nil {
			t.Fatalf("conflicts: %+v", rpcErr)
		}
		var r respConflictos
		json.Unmarshal([]byte(out.(CallToolResponse).Content[0].Text), &r)
		return r
	}

	vecino := &Principal{Name: "alice", Role: RoleWriter, ProjectID: "crm"}
	federado := &Principal{Name: "root", Role: RoleAdmin}

	for _, args := range []map[string]interface{}{
		{}, {"count_only": true}, {"limit": 10}, {"min_confidence": 0.5}, {"order": "confidence"},
	} {
		if r := pedirComo(vecino, args); r.Count != 0 || len(r.Relations) != 0 {
			t.Errorf("FUGA con args %v: crm ve %d conflictos de web", args, r.Count)
		}
	}
	// Control positivo: el federado SÍ los ve, con y sin filtros. Sin esto, una tool rota que
	// devolviera cero para todos pasaría el test de aislamiento.
	if r := pedirComo(federado, map[string]interface{}{}); r.Count != 1 {
		t.Errorf("el admin federado debería ver el conflicto de web: %+v", r)
	}
	if r := pedirComo(federado, map[string]interface{}{"count_only": true}); r.Count != 1 {
		t.Errorf("count_only también debe contar para el federado: %+v", r)
	}
}
