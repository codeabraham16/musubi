package mcp

// La guarda del eje de consentimiento, en las DOS dimensiones que tiene.

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// TestElEjeDeConsentimientoEsUnaMatrizDeCaminosPorGrados.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ UNA MATRIZ Y NO UNA LISTA DE CAMINOS
//
// Ya hubo una guarda que recorría los TRES caminos —exec, shell, pantalla— y daba tranquilidad
// por haber generalizado. Fijaba `avisa` en las tres filas y nunca probaba `pide`: generalizaba
// sobre los CAMINOS y no sobre los GRADOS, y el agujero estaba en el otro eje.
//
// El defecto que se le escapó: `AvisaAlUsuario()` es true para `pide` también —es
// `nivel >= avisa`— así que el switch de la shell, que sólo tenía las dos ramas de `avisa`,
// mandaba una notificación y abría el prompt en el acto. La persona recibía un aviso QUE NO PODÍA
// CONTESTAR mientras el operador ya estaba adentro, y el grado promete «tiene que aceptar. Sin
// respuesta, no hay sesión».
//
// LA LECCIÓN, QUE VALE MÁS QUE EL ARREGLO: cuando el comportamiento es una matriz, una tabla que
// cubre UNA de sus dos dimensiones se siente igual de completa que una que cubre las dos. Antes
// de dar una generalización por hecha hay que preguntarse sobre QUÉ eje se generalizó.
//
// Sabotaje: sacar el reparto de `pide` de toolFleetShell (vuelve el defecto original), o hacer
// que `Bloquea()` deje de cubrir `prohibido` en cualquiera de los tres.
func TestElEjeDeConsentimientoEsUnaMatrizDeCaminosPorGrados(t *testing.T) {
	// Qué se espera en cada celda. `abre` es si el acceso procede; `pregunta` es si se le encoló
	// a la máquina un `musubi:preguntar` —no un aviso—, que es lo único que cumple `pide`.
	type celda struct {
		abre     bool
		pregunta bool
		avisa    bool
	}
	casos := []struct {
		camino string
		grado  fleet.Consentimiento
		esp    celda
		nota   string
	}{
		// libre: nadie se entera, y eso está elegido.
		{"exec", fleet.ConsentimientoLibre, celda{abre: true}, ""},
		{"shell", fleet.ConsentimientoLibre, celda{abre: true}, ""},
		{"pantalla", fleet.ConsentimientoLibre, celda{abre: true}, ""},

		// avisa: se abre y se notifica. No puede negarse.
		{"exec", fleet.ConsentimientoAvisa, celda{abre: true, avisa: true}, ""},
		{"shell", fleet.ConsentimientoAvisa, celda{abre: true, avisa: true}, ""},
		{"pantalla", fleet.ConsentimientoAvisa, celda{abre: true, avisa: true}, ""},

		// pide: NO se abre hasta que conteste. Ésta es la fila que faltaba.
		{"shell", fleet.ConsentimientoPide, celda{pregunta: true},
			"el grado promete «tiene que aceptar. Sin respuesta, no hay sesión»"},
		{"pantalla", fleet.ConsentimientoPide, celda{pregunta: true}, ""},
		// EXEC EN `pide` NO ABRE. A86, decidido por gio el 2026-09-05.
		//
		// Esta fila decía lo contrario, y con razón: hasta ayer `pide` en exec se comportaba como
		// `avisa` —notificaba y ejecutaba— y la celda lo dejaba MEDIDO en vez de supuesto,
		// justamente porque las dos salidas eran de política y no de código. Ya se eligió:
		// endurecer.
		//
		// Lo que se pagó por elegir así, escrito acá para que el próximo no lo descubra
		// desplegando: se ROMPE el auto-heal en cualquier máquina en `pide`. La salida no está en
		// el código sino en la máquina — su dueño baja el grado a `avisa`, que es una decisión
		// explícita y queda registrada.
		//
		// ESTA FRASE ERA FALSA CUANDO SE ESCRIBIÓ, Y HOY ES VERDAD. Medido el 2026-09-05: el
		// auto-heal NO pasaba por el eje, así que una máquina en `pide` seguía recibiendo su exec
		// igual —el costo declarado no se pagaba, y con eso se caía también el argumento «bloquear
		// de más SE NOTA» con el que se eligió endurecer—. A91 (decisión de gio, el mismo día)
		// puso la tercera compuerta en `actuarSiCorresponde`, y la fila de `pide` de
		// TestElAutoHealPasaPorElEjeDeConsentimiento es la que ahora lo MIDE en vez de suponerlo.
		// Se deja escrito porque un texto que pasó de falso a verdadero sin que nadie lo tocara es
		// el caso donde más fácil se vuelve a mentir.
		//
		// Se eligió endurecer y no preguntar-por-comando porque endurecer NO INVENTA
		// COMPORTAMIENTO: es la misma regla que el dominio ya aplica cuando no hay a quién
		// preguntarle. Y porque los dos errores no cuestan igual — bloquear de más se nota, y
		// ejecutar sin preguntar no se nota nunca.
		//
		// LA ASIMETRÍA CON SHELL ES DELIBERADA Y ES LA LÍNEA QUE ESTA FILA CUSTODIA: `shell` en
		// `pide` PREGUNTA y espera (A85) porque abre una sesión, que tiene dónde esperar la
		// respuesta; `exec` es una orden suelta y no. Si algún día exec aprende a esperar, esta
		// celda cambia a `pregunta: true` y el cambio se ve acá.
		{"exec", fleet.ConsentimientoPide, celda{},
			"A86: exec en `pide` se endurece a prohibido — no tiene dónde esperar un sí"},

		// prohibido: el candado del dueño, y puede más que la capacidad.
		{"exec", fleet.ConsentimientoProhibido, celda{}, ""},
		{"shell", fleet.ConsentimientoProhibido, celda{}, ""},
		{"pantalla", fleet.ConsentimientoProhibido, celda{}, ""},
	}

	for _, c := range casos {
		t.Run(c.camino+"/"+string(c.grado), func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			d, p := maquinaConGrado(t, s, c.grado)

			var res interface{}
			var e *RpcError
			switch c.camino {
			case "exec":
				res, e = callAsPrincipal(t, s, p, "musubi_fleet_exec", map[string]any{
					"device": d.Name, "argv": []string{"echo", "hola"}, "no_wait": true})
			case "shell":
				res, e = callAsPrincipal(t, s, p, "musubi_fleet_shell", map[string]any{"device": d.Name})
			case "pantalla":
				res, e = callAsPrincipal(t, s, p, "musubi_fleet_screen", map[string]any{"device": d.Name})
			}

			// ¿Se abrió? Un rechazo por consentimiento es un error; una espera es una respuesta
			// SIN el artefacto que da acceso.
			abrio := e == nil && tieneAcceso(t, c.camino, res)
			if abrio != c.esp.abre {
				t.Fatalf("abre = %v, esperaba %v (err=%v) · %s", abrio, c.esp.abre, e, c.nota)
			}

			preguntas, avisos := preguntasYAvisosEncolados(t, s, d.ID)
			if (preguntas > 0) != c.esp.pregunta {
				t.Errorf("preguntó = %v, esperaba %v · un `pide` que no pregunta no es `pide`: la persona recibe algo que no puede contestar mientras el otro ya entró. %s",
					preguntas > 0, c.esp.pregunta, c.nota)
			}
			if (avisos > 0) != c.esp.avisa {
				t.Errorf("avisó = %v, esperaba %v · %s", avisos > 0, c.esp.avisa, c.nota)
			}
		})
	}
}

// maquinaConGrado enrola una Tier A que SABE preguntar y le fija el grado.
//
// `puede_preguntar = true` es lo que hace que la matriz mida el eje y no su degradación: con
// false, `pide` se endurece a `prohibido` y las filas de `pide` probarían otra cosa.
func maquinaConGrado(t *testing.T, s *McpServer, g fleet.Consentimiento) (fleet.Device, *Principal) {
	d, p, _, _ := maquinaConGradoYToken(t, s, g)
	return d, p
}

// maquinaConGradoYToken devuelve además el token del dispositivo y el servidor HTTP, que hacen
// falta para que el «agente» conteste la pregunta.
func maquinaConGradoYToken(t *testing.T, s *McpServer, g fleet.Consentimiento) (fleet.Device, *Principal, *httptest.Server, string) {
	t.Helper()
	res, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "pc-gio", "tier": "A", "caps": []string{"metrics", "exec", "shell", "screen"},
		"project": "casa", "os": "linux",
	})
	if e != nil {
		t.Fatalf("enroll: %+v", e)
	}
	tok, _ := jsonOf(t, res)["token"].(string)
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "")

	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	if _, err := s.engine.FijarConsentimiento(d.ID, g); err != nil {
		t.Fatal(err)
	}
	if err := s.engine.FijarCapacidadDePreguntar(d.ID, true); err != nil {
		t.Fatal(err)
	}
	d, _, _ = s.engine.DevicePorNombre("casa", "pc-gio")

	p := conShell("casa")
	p.Fleet[fleet.CapScreen] = []string{"*"}
	return d, p, ts, tok
}

// tieneAcceso mira si la respuesta trae LO QUE DA ACCESO, que es distinto en cada camino.
func tieneAcceso(t *testing.T, camino string, res interface{}) bool {
	t.Helper()
	if res == nil {
		return false
	}
	m := jsonOf(t, res)
	switch camino {
	case "exec":
		_, hay := m["command_id"]
		return hay
	case "shell":
		id, _ := m["session_id"].(string)
		return id != "" && m["estado"] != string(fleet.ShellEsperandoPermiso)
	default:
		_, hay := m["password"]
		return hay
	}
}

func preguntasYAvisosEncolados(t *testing.T, s *McpServer, deviceID string) (preguntas, avisos int) {
	t.Helper()
	cmds, err := s.engine.TomarComandos(deviceID, time.Now(), 100)
	if err != nil {
		t.Fatalf("no se pudo leer la cola: %v", err)
	}
	for _, c := range cmds {
		if len(c.Argv) == 0 {
			continue
		}
		switch c.Argv[0] {
		case comandoPreguntar:
			preguntas++
		case comandoAviso:
			avisos++
		}
	}
	return preguntas, avisos
}

// Y EL TEXTO DE LA PREGUNTA DICE QUÉ SE ESTÁ PIDIENDO. «Alguien quiere entrar» no se puede
// contestar: la persona necesita saber si le van a mirar la pantalla o abrirle una terminal, que
// se responden distinto.
func TestLaPreguntaDiceQueSeEstaPidiendo(t *testing.T) {
	for _, c := range []struct{ camino, tool, espera string }{
		{"shell", "musubi_fleet_shell", "TERMINAL"},
		{"pantalla", "musubi_fleet_screen", "pantalla"},
	} {
		t.Run(c.camino, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			d, p := maquinaConGrado(t, s, fleet.ConsentimientoPide)
			if _, e := callAsPrincipal(t, s, p, c.tool, map[string]any{"device": d.Name}); e != nil {
				t.Fatalf("%s: %+v", c.tool, e)
			}
			cmds, _ := s.engine.TomarComandos(d.ID, time.Now(), 100)
			var texto string
			for _, cm := range cmds {
				if len(cm.Argv) > 2 && cm.Argv[0] == comandoPreguntar {
					texto = cm.Argv[2]
				}
			}
			if texto == "" {
				t.Fatal("no se encoló ninguna pregunta")
			}
			if !strings.Contains(texto, c.espera) {
				t.Errorf("la pregunta no dice qué se pide (%q): la persona no puede decidir sobre algo que no sabe qué es", texto)
			}
		})
	}
}

// EL CIRCUITO COMPLETO DE LA SHELL: se pregunta, el usuario contesta, y el prompt aparece o no.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// SIN ESTO, LA MATRIZ DE ARRIBA PRUEBA SÓLO LA MITAD
//
// La matriz mide que se PREGUNTE. Que la respuesta llegue y mande es otra cosa, y lo descubrió un
// sabotaje: cambiar `ShellSinPermiso` por `ShellAbriendo` en ResponderConsentimientoDeShell —o
// sea, que un «no» abriera la shell— dejaba la matriz entera en VERDE. Una guarda que cubre el
// pedido y no la respuesta se siente completa y deja pasar el caso que importa.
//
// Sabotaje: que un «negada» deje la sesión en `abriendo`; o que ResponderConsentimientoDeShell no
// se llame nunca desde registrarRespuestaDePermiso.
func TestElCircuitoCompletoDeUnPideEnLaShell(t *testing.T) {
	for _, c := range []struct {
		nombre    string
		respuesta fleet.RespuestaAviso
		abre      bool
		enError   string
	}{
		{"dijeron que sí", fleet.RespuestaConcedida, true, ""},
		{"dijeron que NO", fleet.RespuestaNegada, false, "dijo que NO"},
		{"nadie contestó", fleet.RespuestaSinRespuesta, false, "El silencio NO es permiso"},
		{"no tuvieron con qué preguntar", fleet.RespuestaNoSePudo, false, "no tuvo con qué preguntar"},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			d, p, ts, tokDev := maquinaConGradoYToken(t, s, fleet.ConsentimientoPide)

			res, e := callAsPrincipal(t, s, p, "musubi_fleet_shell", map[string]any{"device": d.Name})
			if e != nil {
				t.Fatalf("primer pedido: %+v", e)
			}
			sesID, _ := jsonOf(t, res)["session_id"].(string)
			if sesID == "" {
				t.Fatal("el primer pedido no registró la sesión que espera permiso")
			}

			cmdID := idDelComando(t, s, comandoPreguntar)
			responder(t, ts, tokDev, cmdID, prefijoRespuestaPermiso+string(c.respuesta))

			res2, e2 := callAsPrincipal(t, s, p, "musubi_fleet_shell", map[string]any{"device": d.Name})
			if c.abre {
				if e2 != nil {
					t.Fatalf("con el permiso concedido la shell no abrió: %+v", e2)
				}
				out := jsonOf(t, res2)
				if out["session_id"] != sesID {
					t.Errorf("se abrió una sesión NUEVA (%v) en vez de reusar la que tenía el permiso (%v): la concedida queda colgada y la bitácora muestra dos filas para un solo permiso",
						out["session_id"], sesID)
				}
				return
			}
			if e2 == nil {
				t.Fatalf("la respuesta fue %q y la shell se abrió igual: el eje no se honra en el camino que más puede", c.respuesta)
			}
			if !strings.Contains(e2.Message, c.enError) {
				t.Errorf("el motivo del rechazo no distingue este «no» de los otros dos: %q", e2.Message)
			}
		})
	}
}
