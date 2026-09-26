package mcp

import (
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// EL ORIGEN DE UNA FILA DE SESIÓN EN LA CRONOLOGÍA ES EL QUE DECIDE SU PUERTA DE fleet, Y NINGUNO MÁS.
//
// La regla vive en el dominio: una sesión no es una fila de device_commands y no lleva origen
// (TestElOrigenDelHechoLoDecideSuPuerta, internal/fleet, la mide en HechoDeSesionPantalla y
// HechoDeSesionShell). En la superficie, TestLasDosSuperficiesDicenElMismoOrigenDeCadaClaseDeFila
// cruza el origen de cada clase de fila de COMANDO entre la cronología y la bitácora; las de sesión no
// tienen bitácora con la que cruzarse y quedaban sin nadie. La segunda revisión de A131 (tema T9) lo
// señaló: un `origen` que filaDeHecho llenara por tipo de sesión —la C4-m2 del dominio, repetida en
// la superficie— no lo habría visto ninguna prueba.
//
// Acá se siembra cada tipo de sesión —salen del enum: los que se pueden mostrar y que una fila de
// device_commands no puede declarar—, abierta y cerrada, por las mismas puertas del motor que usan
// las tools; se relee la sesión guardada, se le pregunta a SU puerta de fleet qué origen le toca, y se
// exige que la fila de musubi_fleet_cronologia diga exactamente eso: `origen` y `automatico` en null
// si la puerta dice desconocido, y el origen con su bit si algún día dijera otro. La prueba no clava
// «las sesiones no llevan origen»: eso es del dominio, y si cambia, cambia en un lugar.
//
// EXPOSICIÓN: ninguna hoy; filaDeHecho no mira el tipo para decidir el origen. Lo que cuida esto es
// que la superficie no le agregue a una sesión un dueño que el dominio no le dio.
//
// Sabotaje: que filaDeHecho le ponga origen `persona` a la sesión de pantalla (C4-m2, en la
// superficie) → la cronología atribuye la sesión a una persona que el dominio no nombró.
// arnes: archivo="internal/mcp/methods_cronologia.go"
// arnes: de="\t\tfila[\"automatico\"] = h.Origen.EsAutomatico()\n\t}\n"
// arnes: a="\t\tfila[\"automatico\"] = h.Origen.EsAutomatico()\n\t}\n\tif h.Tipo == fleet.HechoPantalla {\n\t\tfila[\"origen\"] = string(fleet.OrigenPersona)\n\t\tfila[\"automatico\"] = false\n\t}\n"
func TestElOrigenDeUnaFilaDeSesionEsElQueDecideSuPuerta(t *testing.T) {
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
	comienzo := time.Now().UTC().Truncate(time.Second).Add(-10 * time.Minute)

	// Cómo se abre, se cierra y se relee una sesión de cada tipo, y qué hecho arma su puerta de
	// fleet con lo que quedó guardado.
	type sembrador struct {
		abrir  func() string
		cerrar func(id string)
		puerta func(id string) fleet.Hecho
	}
	sembradores := map[fleet.TipoDeHecho]sembrador{
		fleet.HechoPantalla: {
			abrir: func() string {
				ses, err := s.engine.AbrirSesionPantalla(fleet.SesionPantalla{
					DeviceID: d.ID, ProjectID: "infra", Principal: "gio", Creada: comienzo,
				})
				if err != nil {
					t.Fatal(err)
				}
				return ses.ID
			},
			cerrar: func(id string) {
				if err := s.engine.MarcarSesion(d.ID, id, fleet.SesionVencida, "", comienzo.Add(time.Minute)); err != nil {
					t.Fatal(err)
				}
			},
			puerta: func(id string) fleet.Hecho {
				sesiones, err := s.engine.SesionesDePantalla("infra", d.ID, 100, time.Now().UTC())
				if err != nil {
					t.Fatal(err)
				}
				for _, ses := range sesiones {
					if ses.ID == id {
						return fleet.HechoDeSesionPantalla(ses, "pc-gio")
					}
				}
				t.Fatalf("la sesión de pantalla %s no se pudo releer", id)
				return fleet.Hecho{}
			},
		},
		fleet.HechoShell: {
			abrir: func() string {
				ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
					DeviceID: d.ID, ProjectID: "infra", Principal: "gio", Creada: comienzo,
				})
				if err != nil {
					t.Fatal(err)
				}
				return ses.ID
			},
			cerrar: func(id string) {
				if err := s.engine.CerrarSesionShell(id, fleet.ShellCerrada, "", comienzo.Add(time.Minute)); err != nil {
					t.Fatal(err)
				}
			},
			puerta: func(id string) fleet.Hecho {
				ses, ok, err := s.engine.SesionShellPorID(id)
				if err != nil || !ok {
					t.Fatalf("la sesión de shell %s no se pudo releer: ok=%v err=%v", id, ok, err)
				}
				return fleet.HechoDeSesionShell(ses, "pc-gio")
			},
		},
	}

	// Los tipos de sesión salen del enum: los que se muestran y que una fila de device_commands no
	// puede declarar (TipoDeComando no los acepta como clasificación).
	var tipos []fleet.TipoDeHecho
	for _, tipo := range fleet.TiposDeHecho {
		if _, mostrable := fleet.CapDeHecho(tipo); !mostrable {
			continue
		}
		if fleet.TipoDeComando(fleet.Comando{Argv: []string{"uptime"}, Clasificacion: tipo}) == tipo {
			continue
		}
		if _, ok := sembradores[tipo]; !ok {
			t.Fatalf("el tipo %q es una sesión que se puede mostrar y esta prueba no sabe sembrarla: sumale cómo "+
				"se abre, se cierra y se relee, o su origen en la superficie queda sin medir", tipo)
		}
		tipos = append(tipos, tipo)
	}
	// EL PISO: cuando se escribió esto había dos tipos de sesión.
	if len(tipos) < 2 {
		t.Fatalf("el enum da %d tipos de sesión mostrables (%q) y cuando se escribió esta prueba eran 2: el "+
			"recorrido mide menos de lo que dice", len(tipos), tipos)
	}

	type sembrada struct {
		tipo    fleet.TipoDeHecho
		cerrada bool
	}
	porID := map[string]sembrada{}
	for _, tipo := range tipos {
		for _, cerrada := range []bool{false, true} {
			id := sembradores[tipo].abrir()
			if cerrada {
				sembradores[tipo].cerrar(id)
			}
			porID[id] = sembrada{tipo, cerrada}
		}
	}

	todo := conCaps("infra", map[fleet.Cap][]string{
		fleet.CapExec: {"*"}, fleet.CapScreen: {"*"}, fleet.CapShell: {"*"},
	})
	res, e := callAsPrincipal(t, s, todo, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio", "limite": 300})
	if e != nil {
		t.Fatalf("cronologia: %+v", e)
	}
	vistas := 0
	for _, f := range jsonOf(t, res)["hechos"].([]any) {
		fila := f.(map[string]any)
		ref, _ := fila["referencia"].(string)
		sem, ok := porID[ref]
		if !ok {
			continue
		}
		vistas++
		h := sembradores[sem.tipo].puerta(ref)
		if h.Tipo != sem.tipo {
			t.Fatalf("la puerta de %q armó un hecho de tipo %q: la prueba no está preguntándole a la puerta correcta", sem.tipo, h.Tipo)
		}
		quiero := [2]any{nil, nil}
		if h.Origen != fleet.OrigenDesconocido {
			quiero = [2]any{string(h.Origen), h.Origen.EsAutomatico()}
		}
		if got := [2]any{fila["origen"], fila["automatico"]}; got != quiero {
			t.Errorf("una sesión %s (cerrada=%v) sale (origen, automatico) = %v en la cronología y su puerta de "+
				"fleet dice %v: la superficie le pone a una sesión un origen que el dominio no le dio",
				sem.tipo, sem.cerrada, got, quiero)
		}
	}
	// EL PISO: cada sesión sembrada llegó a la respuesta.
	if vistas != len(porID) {
		t.Fatalf("llegaron %d de las %d sesiones sembradas: la prueba no midió el origen de las que faltan (%s)",
			vistas, len(porID), textOf(t, res))
	}
}
