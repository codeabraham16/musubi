package mcp

import (
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// LA BITÁCORA DE COMANDOS MIDE LA EJECUCIÓN CON LA REGLA DE LA CRONOLOGÍA: la duración se sabe sólo
// con las dos puntas, y en orden.
//
// musubi_fleet_log —y el resultado de musubi_fleet_exec, que comparte conResultado— restaba
// `Terminado − Entregado` a mano. Tenía una de las dos reglas (sin entrega no hay duración) y le
// faltaba la otra: un resultado anterior a la entrega salía con una duración NEGATIVA. Es la misma
// forma que A131·T9 arregló en musubi_fleet_shell_log, en el hermano que quedó sin mirar; lo encontró
// la revisión de T9. Ahora pregunta por fleet.Comando.DuracionDeEjecucion, que no escribe la regla:
// se la pide a Hecho.Duracion por su misma puerta, con el comienzo en la entrega.
//
// Se siembra por las puertas del motor —EncolarComando, TomarComandos y GuardarResultado, las que
// usan el exec, el agente y el latido— con cada final que la tabla puede guardar respecto de la
// ENTREGA: ninguno, en el mismo segundo (400 ms después, que la tabla trunca), 90 s después y antes.
// Y el que nunca se entregó: TomarComandos lo vence y le estampa `terminado` sin `entregado`.
//
// EXPOSICIÓN: no medida en producción desde acá; ninguna sonda de esta revisión toca una máquina. El
// estado se alcanza por las puertas reales —un resultado reportado con una hora anterior a la de su
// entrega— y es lo que esta prueba siembra. El vencido sin entrega sí está medido (la auditoría contó
// 11.010 en 30 días), y a ése ya lo cubría la regla de la entrega: acá es el control de que la regla
// nueva no la perdió.
//
// Sabotaje: que la bitácora vuelva a restar por su cuenta → el resultado anterior a la entrega sale
// con una duración negativa.
// arnes: archivo="internal/mcp/methods_exec.go"
// arnes: de="\t\tif dur, hay := c.DuracionDeEjecucion(); hay {\n\t\t\tfila[\"duracion_ms\"] = dur.Milliseconds()\n\t\t}"
// arnes: a="\t\tif !c.Entregado.IsZero() {\n\t\t\tfila[\"duracion_ms\"] = c.Terminado.Sub(c.Entregado).Milliseconds()\n\t\t}"
func TestLaBitacoraDeComandosMideLaEjecucionConLaReglaDeLaCronologia(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]interface{}{
		"name": "pc-gio", "tier": "A", "project": "infra", "caps": []string{"metrics", "exec"}, "os": "linux",
	}); e != nil {
		t.Fatalf("enroll: %+v", e)
	}
	d, existe, err := s.engine.DevicePorNombre("infra", "pc-gio")
	if err != nil || !existe {
		t.Fatalf("no quedó la máquina: %v %v", existe, err)
	}
	// La entrega, en un segundo entero: la tabla guarda segundos, y el final «en el mismo segundo»
	// tiene que salir de ese truncado y no de un fixture.
	entrega := time.Now().UTC().Truncate(time.Second).Add(-10 * time.Minute)
	encolar := func(creado time.Time) string {
		t.Helper()
		c, err := s.engine.EncolarComando(fleet.Comando{
			DeviceID: d.ID, ProjectID: "infra", Principal: "gio", Creado: creado,
			Argv: []string{"echo", "x"}, Timeout: 30 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		return c.ID
	}

	type sembrado struct {
		nombre string
		sabe   bool
		ms     float64
	}
	porID := map[string]sembrado{}
	// El que nunca se entregó: creado antes de su vida máxima, la toma lo vence en vez de entregarlo.
	porID[encolar(entrega.Add(-fleet.ComandoVidaMax-5*time.Minute))] = sembrado{nombre: "vencido sin entregar"}
	// Los que se entregan todos juntos, en `entrega`.
	porCierre := map[string]cierreDeHecho{}
	for _, c := range cierresDeHecho {
		porCierre[encolar(entrega.Add(-time.Minute))] = c
	}
	entregados, err := s.engine.TomarComandos(d.ID, entrega, fleet.ColaMaxPorDevice)
	if err != nil {
		t.Fatal(err)
	}
	if len(entregados) != len(cierresDeHecho) {
		t.Fatalf("la toma entregó %d comandos y se sembraron %d para entregar: la prueba no mide lo que dice", len(entregados), len(cierresDeHecho))
	}
	for id, c := range porCierre {
		if c.cierra {
			cero := 0
			if err := s.engine.GuardarResultado(d.ID, id, &cero, "", "", "", entrega.Add(c.tras)); err != nil {
				t.Fatal(err)
			}
		}
		sabe := c.cierra && c.tras >= 0
		porID[id] = sembrado{nombre: c.nombre, sabe: sabe, ms: float64(c.tras.Truncate(time.Second).Milliseconds())}
	}

	res, e := callAsPrincipal(t, s, conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}}),
		"musubi_fleet_log", map[string]any{"limite": 50})
	if e != nil {
		t.Fatalf("fleet_log: %+v", e)
	}
	vistos := 0
	for _, f := range jsonOf(t, res)["comandos"].([]any) {
		fila := f.(map[string]any)
		id, _ := fila["command_id"].(string)
		sem, ok := porID[id]
		if !ok {
			continue
		}
		vistos++
		duracion, trae := fila["duracion_ms"]
		switch {
		case !sem.sabe && trae:
			t.Errorf("un comando %s (medido desde su entrega) viaja con duracion_ms=%v y terminado=%v: no se sabe "+
				"cuánto corrió, y un número ahí se lee «corrió eso» —negativo, o contado desde una entrega que no "+
				"existió—", sem.nombre, duracion, fila["terminado"])
		case sem.sabe && !trae:
			t.Errorf("un comando %s (medido desde su entrega) viaja sin duracion_ms y corrió %v ms: la fila dice que "+
				"terminó y no cuánto", sem.nombre, sem.ms)
		case sem.sabe && duracion != sem.ms:
			t.Errorf("un comando %s (medido desde su entrega) viaja con duracion_ms=%v y corrió %v ms", sem.nombre, duracion, sem.ms)
		}
	}
	// EL PISO: todo lo sembrado llegó a la bitácora, los que se saben y los que no.
	if vistos != len(porID) {
		t.Fatalf("llegaron %d de los %d comandos sembrados: la prueba no midió la duración de los que faltan (%s)",
			vistos, len(porID), textOf(t, res))
	}
}
