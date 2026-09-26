package mcp

import (
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// cierreDeHecho es cómo termina un hecho sembrado: si termina, y cuánto después de empezar.
type cierreDeHecho struct {
	nombre string
	cierra bool
	tras   time.Duration
}

// cierresDeHecho son los cuatro finales que la tabla puede guardar: ninguno, en el mismo segundo
// —400 ms después, que la tabla trunca al segundo del comienzo—, después, y antes (dato corrupto).
var cierresDeHecho = []cierreDeHecho{
	{"en curso", false, 0},
	{"con fin en el mismo segundo", true, 400 * time.Millisecond},
	{"con fin 90 s después", true, 90 * time.Second},
	{"con el fin anterior al comienzo", true, -time.Minute},
}

// CADA FILA DE LA CRONOLOGÍA DICE SU DURACIÓN SI Y SÓLO SI SE SABE, en la superficie donde se lee,
// para cada tipo de hecho y cada final posible.
//
// La guarda de la duración (TestLaDuracionDiceSiSeSabe) prueba el MÉTODO; el «cero mentiroso» de C16
// se dibuja en filaDeHecho, y ninguna prueba del árbol miraba `duracion_seg`. La auditoría A131
// (C3-m3) hizo que filaDeHecho ignorara el booleano —`if d, _ := h.Duracion(); true`— y fleet y mcp
// quedaron verdes: todo comando pendiente y toda sesión abierta viajaba con `duracion_seg: 0`,
// «duró nada» sobre algo que sigue corriendo. Acá se siembra un hecho de cada tipo mostrable
// —salen del enum— con cada final, por las mismas puertas del motor que usan las tools, y se lee la
// fila que llega. El final en el mismo segundo sale del formato de la tabla (se cierra 400 ms después
// y el RFC3339 lo trunca), no de un fixture escrito a mano: es el caso de C3-m2 en su consumidor.
//
// EXPOSICIÓN medida por la auditoría en el cerebro: 8 hechos en curso que la mutación habría mandado
// con `duracion_seg: 0` (4 comandos `entregado` de davantis-1 y 4 sesiones de pantalla `activa` de
// gio), y 30 comandos terminados en el mismo segundo en que se crearon. La tool se llamó 14 veces
// entre el 2026-08-30 y el 2026-09-21.
//
// Sabotaje: que filaDeHecho ignore el booleano de Duracion (C3-m3) → todo hecho en curso viaja con
// `duracion_seg: 0`.
// arnes: archivo="internal/mcp/methods_cronologia.go"
// arnes: de="\tif d, hay := h.Duracion(); hay {\n\t\tfila[\"duracion_seg\"] = int64(d.Seconds())"
// arnes: a="\tif d, _ := h.Duracion(); true {\n\t\tfila[\"duracion_seg\"] = int64(d.Seconds())"
func TestCadaFilaDeLaCronologiaDiceSuDuracionSiYSoloSiSeSabe(t *testing.T) {
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

	// Cómo abrir y cerrar un hecho de cada tipo, por las puertas del motor que usan las tools.
	type sembrador struct {
		abrir  func(creado time.Time) string
		cerrar func(id string, ahora time.Time)
	}
	comando := func(argv []string, clase fleet.TipoDeHecho) sembrador {
		return sembrador{
			abrir: func(creado time.Time) string {
				c, err := s.engine.EncolarComando(fleet.Comando{
					DeviceID: d.ID, ProjectID: "infra", Principal: "gio", Creado: creado,
					Argv: argv, Timeout: 30 * time.Second, Clasificacion: clase,
				})
				if err != nil {
					t.Fatal(err)
				}
				return c.ID
			},
			cerrar: func(id string, ahora time.Time) {
				cero := 0
				if err := s.engine.GuardarResultado(d.ID, id, &cero, "", "", "", ahora); err != nil {
					t.Fatal(err)
				}
			},
		}
	}
	sembradores := map[fleet.TipoDeHecho]sembrador{
		fleet.HechoComando:       comando([]string{"echo", "x"}, ""),
		fleet.HechoCanalExec:     comando([]string{fleet.OpAvisar, "Musubi: gio está ejecutando comandos en esta máquina."}, fleet.HechoCanalExec),
		fleet.HechoCanalPantalla: comando([]string{fleet.OpPantalla, "ses-1", "clave", "30m0s"}, ""),
		fleet.HechoCanalShell:    comando([]string{fleet.OpShell, "ses-2", "24", "80"}, ""),
		fleet.HechoPantalla: {
			abrir: func(creado time.Time) string {
				ses, err := s.engine.AbrirSesionPantalla(fleet.SesionPantalla{
					DeviceID: d.ID, ProjectID: "infra", Principal: "gio", Creada: creado,
				})
				if err != nil {
					t.Fatal(err)
				}
				return ses.ID
			},
			cerrar: func(id string, ahora time.Time) {
				if err := s.engine.MarcarSesion(d.ID, id, fleet.SesionVencida, "", ahora); err != nil {
					t.Fatal(err)
				}
			},
		},
		fleet.HechoShell: {
			abrir: func(creado time.Time) string {
				ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
					DeviceID: d.ID, ProjectID: "infra", Principal: "gio", Creada: creado,
				})
				if err != nil {
					t.Fatal(err)
				}
				return ses.ID
			},
			cerrar: func(id string, ahora time.Time) {
				if err := s.engine.CerrarSesionShell(id, fleet.ShellCerrada, "", ahora); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	// Los tipos salen del enum: uno nuevo que se pueda mostrar pide su sembrador acá.
	var tipos []fleet.TipoDeHecho
	for _, tipo := range fleet.TiposDeHecho {
		if _, mostrable := fleet.CapDeHecho(tipo); !mostrable {
			continue
		}
		if _, ok := sembradores[tipo]; !ok {
			t.Fatalf("el tipo %q se puede mostrar y esta prueba no sabe sembrarlo: sumale cómo se abre y se "+
				"cierra, o su `duracion_seg` queda sin medir", tipo)
		}
		tipos = append(tipos, tipo)
	}
	if len(tipos) < 6 {
		t.Fatalf("fleet.TiposDeHecho trae %d tipos mostrables y cuando se escribió esta prueba eran 6: "+
			"el recorrido mide menos de lo que dice", len(tipos))
	}

	type sembrado struct {
		tipo   fleet.TipoDeHecho
		cierre cierreDeHecho
	}
	porReferencia := map[string]sembrado{}
	comienzo := time.Now().UTC().Truncate(time.Second).Add(-10 * time.Minute)
	for _, tipo := range tipos {
		for _, c := range cierresDeHecho {
			id := sembradores[tipo].abrir(comienzo)
			if c.cierra {
				sembradores[tipo].cerrar(id, comienzo.Add(c.tras))
			}
			porReferencia[id] = sembrado{tipo, c}
		}
	}

	todo := conCaps("infra", map[fleet.Cap][]string{
		fleet.CapExec: {"*"}, fleet.CapScreen: {"*"}, fleet.CapShell: {"*"},
	})
	res, e := callAsPrincipal(t, s, todo, "musubi_fleet_cronologia", map[string]any{"device": "pc-gio", "limite": 300})
	if e != nil {
		t.Fatalf("cronologia: %+v", e)
	}
	vistos := 0
	for _, f := range jsonOf(t, res)["hechos"].([]any) {
		fila := f.(map[string]any)
		ref, _ := fila["referencia"].(string)
		sem, ok := porReferencia[ref]
		if !ok {
			continue
		}
		vistos++
		que := string(sem.tipo) + " " + sem.cierre.nombre
		duracion, trae := fila["duracion_seg"]
		if !trae {
			t.Errorf("un hecho %s no trae `duracion_seg`: la clave tiene que viajar siempre, en null cuando no se sabe", que)
			continue
		}
		cuando, errC := time.Parse(time.RFC3339, fila["cuando"].(string))
		if errC != nil {
			t.Fatalf("un hecho %s trae un `cuando` que no es RFC3339: %v", que, fila["cuando"])
		}
		termino, _ := fila["termino"].(string)
		switch {
		case !sem.cierre.cierra:
			if fila["termino"] != nil || duracion != nil {
				t.Errorf("un hecho %s viaja con termino=%v y duracion_seg=%v: sigue en curso, y un número ahí "+
					"se dibuja «duró eso» —el cero mentiroso de C16—", que, fila["termino"], duracion)
			}
		case sem.cierre.tras < 0:
			if termino == "" || duracion != nil {
				t.Errorf("un hecho %s viaja con termino=%v y duracion_seg=%v: el fin está, y una duración "+
					"negativa no es una duración — tiene que viajar null", que, fila["termino"], duracion)
			}
		default:
			fin, errT := time.Parse(time.RFC3339, termino)
			if errT != nil {
				t.Errorf("un hecho %s terminó y viaja con termino=%v", que, fila["termino"])
				continue
			}
			quiero := sem.cierre.tras.Truncate(time.Second)
			if fin.Sub(cuando) != quiero {
				t.Errorf("un hecho %s viaja de %s a %s: la tabla tenía que dejar %s entre las dos puntas", que, cuando, fin, quiero)
			}
			if duracion != quiero.Seconds() {
				t.Errorf("un hecho %s viaja con termino=%s y duracion_seg=%v, y duró %v s: la fila dice que terminó "+
					"y no dice cuánto duró, o dice otra cosa", que, termino, duracion, quiero.Seconds())
			}
		}
	}
	// EL PISO: cada tipo con cada final llegó a la respuesta.
	if vistos != len(porReferencia) {
		t.Fatalf("llegaron %d de los %d hechos sembrados: la prueba no midió la duración de los que faltan (%s)",
			vistos, len(porReferencia), textOf(t, res))
	}
}

// LA BITÁCORA DE SHELL MIDE LA DURACIÓN CON LA MISMA REGLA QUE LA CRONOLOGÍA.
//
// musubi_fleet_shell_log calculaba su `duracion_seg` por su cuenta —`Cerrada.Sub(Creada)`—, sin
// Duracion y sin su descarte del fin anterior al comienzo: una sesión cerrada antes de abrirse salía
// con una duración NEGATIVA, y una con la apertura ilegible, con la duración saturada. La auditoría
// A131 la señaló junto a C3-m1 como el hermano del mismo cálculo. Ahora pregunta por la misma puerta
// (fleet.HechoDeSesionShell) y con la misma regla (Hecho.Duracion); los cuatro finales de acá son los
// mismos que recorre la prueba de la cronología.
//
// EXPOSICIÓN medida por la auditoría: shell_sessions tiene una sola fila, legible y cerrada después de
// abierta. No hay ninguna expuesta hoy.
//
// Sabotaje: que la bitácora vuelva a restar por su cuenta → la sesión cerrada antes de abrirse sale
// con una duración negativa.
// arnes: archivo="internal/mcp/methods_shell.go"
// arnes: de="\t\t\tif dur, hay := fleet.HechoDeSesionShell(ses, nombre).Duracion(); hay {\n\t\t\t\tfila[\"duracion_seg\"] = int(dur.Seconds())\n\t\t\t}"
// arnes: a="\t\t\tfila[\"duracion_seg\"] = int(ses.Cerrada.Sub(ses.Creada).Seconds())"
func TestLaBitacoraDeShellMideLaDuracionConLaReglaDeLaCronologia(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	d := enrolarConShell(t, s, "casa", "nas")
	comienzo := time.Now().UTC().Truncate(time.Second).Add(-10 * time.Minute)

	cierrePorID := map[string]cierreDeHecho{}
	for _, c := range cierresDeHecho {
		ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{
			DeviceID: d.ID, ProjectID: "casa", Principal: "gio", Creada: comienzo,
		})
		if err != nil {
			t.Fatal(err)
		}
		if c.cierra {
			if err := s.engine.CerrarSesionShell(ses.ID, fleet.ShellCerrada, "", comienzo.Add(c.tras)); err != nil {
				t.Fatal(err)
			}
		}
		cierrePorID[ses.ID] = c
	}

	res, e := callAsPrincipal(t, s, conShell("casa"), "musubi_fleet_shell_log", map[string]any{"limite": 50})
	if e != nil {
		t.Fatalf("shell_log: %+v", e)
	}
	vistas := 0
	for _, f := range jsonOf(t, res)["sesiones"].([]any) {
		fila := f.(map[string]any)
		id, _ := fila["session_id"].(string)
		c, ok := cierrePorID[id]
		if !ok {
			continue
		}
		vistas++
		duracion, trae := fila["duracion_seg"]
		sabe := c.cierra && c.tras >= 0
		switch {
		case !sabe && trae:
			t.Errorf("una sesión %s viaja con duracion_seg=%v: no se sabe cuánto duró, y la cronología dice "+
				"null para la misma sesión", c.nombre, duracion)
		case sabe && !trae:
			t.Errorf("una sesión %s viaja sin duracion_seg y duró %s", c.nombre, c.tras.Truncate(time.Second))
		case sabe && duracion != c.tras.Truncate(time.Second).Seconds():
			t.Errorf("una sesión %s viaja con duracion_seg=%v y duró %v s", c.nombre, duracion, c.tras.Truncate(time.Second).Seconds())
		}
	}
	// EL PISO: las cuatro sesiones llegaron a la bitácora.
	if vistas != len(cierresDeHecho) {
		t.Fatalf("llegaron %d de las %d sesiones sembradas: la prueba no midió las que faltan (%s)",
			vistas, len(cierresDeHecho), textOf(t, res))
	}
}
