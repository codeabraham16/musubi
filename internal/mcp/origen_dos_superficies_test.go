package mcp

import (
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// LA CRONOLOGÍA Y LA BITÁCORA DICEN EL MISMO ORIGEN DE CADA FILA DE device_commands, sea de la clase
// que sea.
//
// TestElOrigenAutomaticoSeDistingueYLoDesconocidoNoSeInventa cruza las dos superficies, pero sólo
// sobre execs comunes: ése era el eje clavado. La auditoría A131 (C4-m3) hizo que la cronología
// descartara el origen de toda fila que no fuera `comando` y mcp quedó verde, porque el vecino
// sembraba sólo exec. Acá se siembra cada clase de fila que la puerta de los comandos puede mostrar
// —salen del enum, con la clasificación que fleet.TipoDeComando respeta— con cada origen, y se exige
// que las dos tools cuenten lo mismo fila por fila. La regla del dominio la recorre
// TestElOrigenDelHechoLoDecideSuPuerta (internal/fleet); ésta cuida que la superficie no la tuerza.
//
// EXPOSICIÓN medida por la auditoría: 26 filas de canal con origen `persona` en los últimos 30 días
// (22 `musubi:avisar` del exec, la última del 2026-09-21; 4 `musubi:pantalla`). Con la mutación, las 26
// habrían salido `origen: null` en la cronología y `persona` en la bitácora.
//
// Sabotaje: que la cronología dibuje null el origen de lo que no es un exec común (C4-m3, en la
// superficie) → los avisos y las operaciones de canal pierden quién los originó en una sola de las
// dos tools.
// arnes: archivo="internal/mcp/methods_cronologia.go"
// arnes: de="\tif h.Origen == fleet.OrigenDesconocido {\n"
// arnes: a="\tif h.Origen == fleet.OrigenDesconocido || h.Tipo != fleet.HechoComando {\n"
func TestLasDosSuperficiesDicenElMismoOrigenDeCadaClaseDeFila(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]interface{}{
		"name": "pc-gio", "tier": "A", "project": "infra",
		"caps": []string{"metrics", "exec", "screen", "shell"}, "os": "linux",
	}); e != nil {
		t.Fatalf("enroll: %+v", e)
	}
	d, existe, err := s.engine.DevicePorNombre("infra", "pc-gio")
	if err != nil || !existe {
		t.Fatalf("no quedó la máquina: %v %v", existe, err)
	}

	// Un argv de verdad por clase de fila.
	argvDe := map[fleet.TipoDeHecho][]string{
		fleet.HechoComando:       {"systemctl", "restart", "nginx"},
		fleet.HechoCanalExec:     {fleet.OpAvisar, "Musubi: gio está ejecutando comandos en esta máquina."},
		fleet.HechoCanalPantalla: {fleet.OpPantalla, "ses-1", "clave", "30m0s"},
		fleet.HechoCanalShell:    {fleet.OpShell, "ses-2", "24", "80"},
	}
	// Las clases salen del enum: las que se pueden mostrar y que una fila de device_commands puede
	// declarar. Las de sesión quedan afuera solas, porque TipoDeComando no las acepta de una fila.
	clases := 0
	for _, tipo := range fleet.TiposDeHecho {
		if _, mostrable := fleet.CapDeHecho(tipo); !mostrable {
			continue
		}
		if fleet.TipoDeComando(fleet.Comando{Argv: []string{"uptime"}, Clasificacion: tipo}) != tipo {
			continue
		}
		if _, ok := argvDe[tipo]; !ok {
			t.Fatalf("una fila de device_commands puede ser %q y esta prueba no sabe sembrarla: sumale su argv, "+
				"o su origen queda sin cruzar entre las dos superficies", tipo)
		}
		clases++
	}
	if clases != len(argvDe) {
		t.Fatalf("el enum da %d clases de fila y esta prueba siembra %d: sobra alguna", clases, len(argvDe))
	}

	// Los orígenes también salen del enum, de la lista que fleet cierra contra su bloque const: una
	// copia a mano acá (la había hasta la revisión de T9) no se entera de un cuarto origen.
	if len(fleet.OrigenesDeComando) < 3 {
		t.Fatalf("fleet.OrigenesDeComando trae %d orígenes (%q) y cuando se escribió esta prueba eran 3: "+
			"las dos superficies quedan sin cruzar para los que faltan", len(fleet.OrigenesDeComando), fleet.OrigenesDeComando)
	}

	type sembrada struct {
		clase  fleet.TipoDeHecho
		origen fleet.OrigenComando
	}
	porID := map[string]sembrada{}
	for clase, argv := range argvDe {
		for _, o := range fleet.OrigenesDeComando {
			c, err := s.engine.EncolarComando(fleet.Comando{
				DeviceID: d.ID, ProjectID: "infra", Principal: "gio",
				Argv: argv, Timeout: 30 * time.Second, Clasificacion: clase, Origen: o,
			})
			if err != nil {
				t.Fatal(err)
			}
			porID[c.ID] = sembrada{clase, o}
		}
	}

	// origenDe lee el par (origen, automatico) de cada fila de una tool, por su id.
	origenDe := func(p *Principal, tool, filas, id string, args map[string]any) map[string][2]any {
		t.Helper()
		res, e := callAsPrincipal(t, s, p, tool, args)
		if e != nil {
			t.Fatalf("%s: %+v", tool, e)
		}
		out := map[string][2]any{}
		for _, f := range jsonOf(t, res)[filas].([]any) {
			fila := f.(map[string]any)
			ref, _ := fila[id].(string)
			out[ref] = [2]any{fila["origen"], fila["automatico"]}
		}
		return out
	}
	todo := conCaps("infra", map[fleet.Cap][]string{
		fleet.CapExec: {"*"}, fleet.CapScreen: {"*"}, fleet.CapShell: {"*"},
	})
	enCronologia := origenDe(todo, "musubi_fleet_cronologia", "hechos", "referencia",
		map[string]any{"device": "pc-gio", "limite": 300})
	enBitacora := origenDe(conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}}),
		"musubi_fleet_log", "comandos", "command_id", map[string]any{"limite": 200})

	for id, sem := range porID {
		crono, enC := enCronologia[id]
		bita, enB := enBitacora[id]
		if !enC || !enB {
			t.Errorf("la fila %s (%s, origen %q) no llegó a las dos superficies (cronología=%v, bitácora=%v): "+
				"la prueba no la cruzó", id, sem.clase, sem.origen, enC, enB)
			continue
		}
		quiero := [2]any{nil, nil}
		if sem.origen != fleet.OrigenDesconocido {
			quiero = [2]any{string(sem.origen), sem.origen.EsAutomatico()}
		}
		if crono != quiero || bita != quiero {
			t.Errorf("una fila %s con origen %q sale (origen, automatico) = %v en la cronología y %v en la "+
				"bitácora, y es %v: las dos superficies leen la misma fila y cuentan historias distintas",
				sem.clase, sem.origen, crono, bita, quiero)
		}
	}
}
