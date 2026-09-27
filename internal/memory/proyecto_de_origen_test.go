package memory

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"
)

// CADA NOTA DICE DE QUÉ PROYECTO VIENE.
//
// El recall por defecto es FEDERADO: el hook del turno y el stdio local traen memoria de todos los
// proyectos del acervo, a propósito (ver scope.go). Lo que faltaba era decirlo: el candidato traía
// su project_id desde la consulta y packByBudget lo tiraba al armar el item, así que una nota de
// Altura llegaba al agente de Musubi sin una sola señal de que describía otro repo. Medido el
// 2026-09-27 contra la base local: «el fichaje del kiosko no anda» trae 11 notas de altura y
// ninguna lo decía.

// sembrarTresProyectos siembra la MISMA charla —el fichaje del kiosko— en tres notas que difieren
// en el proyecto de origen: una de altura, una de musubi y una sin atribuir. Así, lo que distinga a
// una nota de otra en la salida sólo puede venir del proyecto.
func sembrarTresProyectos(t *testing.T) *DbEngine {
	t.Helper()
	e := newTestEngine(t)
	for _, n := range []struct{ proyecto, id, topic, texto string }{
		{"altura", "n-altura", "feature/fichaje-f18", "El fichaje del kiosko F18 se cae cuando la Pi pierde el WiFi del galpón."},
		{"musubi", "n-musubi", "hooks/turno", "El fichaje del kiosko aparece en el hook del turno porque el recall es federado."},
		{"", "n-suelta", "notas/sueltas", "Una nota vieja sobre el fichaje del kiosko, anterior a la atribución por proyecto."},
	} {
		e.SetProjectID(n.proyecto)
		if err := e.SaveObservation(n.id, n.topic, n.texto, nil); err != nil {
			t.Fatalf("sembrar %s: %v", n.id, err)
		}
	}
	e.SetProjectID("")
	return e
}

// elProyectoDeCadaUna es lo que las pruebas esperan: la nota sin atribuir, vacía.
var elProyectoDeCadaUna = map[string]string{"n-altura": "altura", "n-musubi": "musubi", "n-suelta": ""}

// TestRecallItemLlevaElProyectoDeOrigen: el recall por consulta y el priming de arranque devuelven
// en cada item el proyecto del que vino la nota, y en el JSON la clave va con su valor y SE OMITE en
// la nota sin atribuir (omitempty: el camino de siempre no paga un token por el campo nuevo).
//
// Sabotaje: packByBudget vuelve a tirar el proyecto del candidato.
// arnes: archivo="internal/memory/recall.go"
// arnes: de="\t\t\tProjectID:   c.projectID,\n"
// arnes: a="\t\t\tProjectID:   \"\",\n"
func TestRecallItemLlevaElProyectoDeOrigen(t *testing.T) {
	e := sembrarTresProyectos(t)

	res, err := e.Recall(context.Background(), "fichaje kiosko", RecallOptions{TokenBudget: 2000, NoBump: true})
	if err != nil {
		t.Fatalf("Recall: %v", err)
	}
	prime, err := e.PrimeContext(2000)
	if err != nil {
		t.Fatalf("PrimeContext: %v", err)
	}

	for superficie, items := range map[string][]RecallItem{"Recall": res.Items, "PrimeContext": prime.Items} {
		vistos := map[string]string{}
		for _, it := range items {
			vistos[it.ID] = it.ProjectID
		}
		for id, quiere := range elProyectoDeCadaUna {
			got, ok := vistos[id]
			if !ok {
				t.Errorf("%s no trajo %s: sin la nota no se mide nada (trajo %v)", superficie, id, vistos)
				continue
			}
			if got != quiere {
				t.Errorf("%s: %s dice project_id=%q y la nota es de %q", superficie, id, got, quiere)
			}
		}
	}

	for _, it := range res.Items {
		crudo, err := json.Marshal(it)
		if err != nil {
			t.Fatal(err)
		}
		switch it.ID {
		case "n-altura":
			if !strings.Contains(string(crudo), `"project_id":"altura"`) {
				t.Errorf("el JSON de la nota de altura no dice su proyecto: %s", crudo)
			}
		case "n-suelta":
			if strings.Contains(string(crudo), `"project_id"`) {
				t.Errorf("la nota sin atribuir paga la clave vacía: %s", crudo)
			}
		}
	}
}

// TestExpandDiceElProyecto: musubi_memory_expand hidrata por id, y cada observación dice de qué
// proyecto es, con la misma omisión en el JSON. Y LA MURALLA NO SE MOVIÓ: con la credencial de
// musubi, la nota de altura sigue sin hidratarse; decir el proyecto no abre nada.
//
// Sabotaje: la hidratación deja de leer la columna.
// arnes: archivo="internal/memory/expand.go"
// arnes: de="COALESCE(project_id,"
// arnes: a="COALESCE(NULL,"
func TestExpandDiceElProyecto(t *testing.T) {
	e := sembrarTresProyectos(t)
	ids := []string{"n-altura", "n-musubi", "n-suelta"}

	obs, _, err := e.GetObservationsBudget(ids, 0)
	if err != nil {
		t.Fatalf("GetObservationsBudget: %v", err)
	}
	if len(obs) != len(ids) {
		t.Fatalf("federado, la hidratación tenía que traer las %d: trajo %d", len(ids), len(obs))
	}
	for _, o := range obs {
		if quiere := elProyectoDeCadaUna[o.ID]; o.ProjectID != quiere {
			t.Errorf("expand: %s dice project_id=%q y la nota es de %q", o.ID, o.ProjectID, quiere)
		}
		crudo, err := json.Marshal(o)
		if err != nil {
			t.Fatal(err)
		}
		tieneClave := strings.Contains(string(crudo), `"project_id"`)
		if tieneClave != (o.ProjectID != "") {
			t.Errorf("expand: el JSON de %s tiene la clave=%v con project_id=%q: %s", o.ID, tieneClave, o.ProjectID, crudo)
		}
	}

	conCredencial := WithProjectScope(context.Background(), ProjectScope{ProjectID: "musubi"})
	obs, _, err = e.GetObservationsBudgetCtx(conCredencial, ids, 0)
	if err != nil {
		t.Fatalf("GetObservationsBudgetCtx: %v", err)
	}
	var vistas []string
	for _, o := range obs {
		vistas = append(vistas, o.ID)
	}
	sort.Strings(vistas)
	if strings.Join(vistas, ",") != "n-musubi,n-suelta" {
		t.Errorf("con la credencial de musubi la hidratación trajo %v: la muralla cambió", vistas)
	}
}

// TestMismoProyectoEsElCriterioDeLaMuralla: memory.MismoProyecto contesta, fila por fila, lo mismo
// que las dos murallas que ya existen —scopeClause en SQL y filterCandidatesByProject en el recall—
// para los casos que las separarían si alguien «normalizara» de un solo lado: la misma palabra con
// otra capitalización, con un espacio adelante, la columna vacía y la nula, y el proyecto propio
// vacío. Si una nota saliera sin marca en la viñeta y a la vez la muralla la tratara como ajena,
// habría dos criterios de «mismo proyecto», que es lo que esta función existe para impedir.
//
// Sabotaje: el criterio de la viñeta se normaliza y la muralla no.
// arnes: archivo="internal/memory/scope.go"
// arnes: de="\treturn deLaNota == \"\" || deLaNota == propio\n"
// arnes: a="\treturn deLaNota == \"\" || strings.EqualFold(strings.TrimSpace(deLaNota), propio)\n"
//
// Sabotaje: sin proyecto propio, las notas atribuidas pasan a ser ajenas.
// arnes: archivo="internal/memory/scope.go"
// arnes: de="\tif propio == \"\" {\n\t\treturn true"
// arnes: a="\tif false {\n\t\treturn true"
func TestMismoProyectoEsElCriterioDeLaMuralla(t *testing.T) {
	e := newTestEngine(t)
	filas := map[string]*string{
		"p-musubi":  columnaCon("musubi"),
		"p-Musubi":  columnaCon("Musubi"),
		"p-espacio": columnaCon(" musubi"),
		"p-altura":  columnaCon("altura"),
		"p-vacia":   columnaCon(""),
		"p-nula":    nil,
	}
	for id, proyecto := range filas {
		if err := e.SaveObservation(id, "t/"+id, "una nota de "+id, nil); err != nil {
			t.Fatalf("sembrar %s: %v", id, err)
		}
		var valor interface{}
		if proyecto != nil {
			valor = *proyecto
		}
		if _, err := e.db.Exec(`UPDATE observations SET project_id = ? WHERE id = ?`, valor, id); err != nil {
			t.Fatalf("fijar project_id de %s: %v", id, err)
		}
	}

	for _, propio := range []string{"musubi", "Musubi", "altura", ""} {
		clausula, args := ProjectScope{ProjectID: propio}.scopeClause("")
		rows, err := e.db.Query(`SELECT id, COALESCE(project_id,'') FROM observations WHERE 1 = 1`+clausula, args...)
		if err != nil {
			t.Fatalf("consulta con la muralla de %q: %v", propio, err)
		}
		veSQL := map[string]bool{}
		for rows.Next() {
			var id, p string
			if err := rows.Scan(&id, &p); err != nil {
				t.Fatal(err)
			}
			veSQL[id] = true
		}
		rows.Close()

		var cands []candidate
		for id, proyecto := range filas {
			c := candidate{id: id}
			if proyecto != nil {
				c.projectID = *proyecto
			}
			cands = append(cands, c)
		}
		veRecall := map[string]bool{}
		for _, c := range filterCandidatesByProject(cands, propio) {
			veRecall[c.id] = true
		}

		for id, proyecto := range filas {
			deLaNota := ""
			if proyecto != nil {
				deLaNota = *proyecto
			}
			mismo := MismoProyecto(propio, deLaNota)
			if mismo != veSQL[id] {
				t.Errorf("propio=%q, nota %s (project_id=%q): MismoProyecto=%v y scopeClause la deja ver=%v", propio, id, deLaNota, mismo, veSQL[id])
			}
			if mismo != veRecall[id] {
				t.Errorf("propio=%q, nota %s (project_id=%q): MismoProyecto=%v y filterCandidatesByProject la deja=%v", propio, id, deLaNota, mismo, veRecall[id])
			}
		}
	}
}

// columnaCon es el valor de una columna TEXT que no es nula.
func columnaCon(s string) *string { return &s }
