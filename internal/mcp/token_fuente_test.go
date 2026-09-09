package mcp

import (
	"encoding/json"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// filaDeFleetList devuelve la fila de una máquina tal como la ve quien llama a la tool.
//
// SE LEE EL JSON DE LA RESPUESTA y no el Device de la base: lo que esta prueba custodia es que el
// dato guardado LLEGUE a quien pregunta. Un campo persistido y no expuesto deja la pregunta igual de
// incontestable que antes, que es exactamente el defecto de A99 y A102.
func filaDeFleetList(t *testing.T, s *McpServer, nombre string) map[string]any {
	t.Helper()
	res, e := call(t, s, "musubi_fleet_list", map[string]any{"project": "casa"})
	if e != nil {
		t.Fatalf("musubi_fleet_list: %+v", e)
	}
	var d struct {
		Devices []map[string]any `json:"devices"`
	}
	if err := json.Unmarshal([]byte(textOf(t, res)), &d); err != nil {
		t.Fatalf("la respuesta de fleet_list no es JSON: %v", err)
	}
	for _, f := range d.Devices {
		if f["name"] == nombre {
			return f
		}
	}
	t.Fatalf("fleet_list no trajo la máquina %q", nombre)
	return nil
}

// ═════════════════════════════════════════════════════════════════════════════════════════════
// A102 · EL AGENTE DICE DE DÓNDE SALIÓ SU TOKEN, Y EL CEREBRO POR FIN LO GUARDA
//
// UNA MÁQUINA QUE RECIBIÓ SU TOKEN POR VARIABLE NO PUEDE COMPLETAR UNA ROTACIÓN —un proceso no
// reescribe su propio entorno— y desde afuera late EXACTAMENTE igual que una que sí puede. La
// rotación vence siempre y el síntoma no señala la causa.
//
// Medido el 2026-09-05 en `davantis-1`: su lanzador tenía la forma vieja
// (`set /p MUSUBI_DEVICE_TOKEN=<archivo`), el arreglo estaba en el repo desde antes, y para
// descubrirlo hubo que LEER UN .cmd EN LA MÁQUINA. El agente lo sabía todo el tiempo: su
// `credencial.ruta` está vacía justamente cuando el token vino por variable.
//
// La pregunta «¿qué máquinas de la flota no pueden rotar?» ahora se le hace al cerebro.
func TestElCerebroGuardaYExponeDeDondeSalioElTokenDelAgente(t *testing.T) {
	casos := []struct {
		nombre  string
		fuente  string
		rotable bool
		seSabe  bool
	}{
		{"por archivo: puede rotar", fleet.CredencialDeArchivo, true, true},
		{"por variable: NO puede rotar", fleet.CredencialDeVariable, false, true},
		{"el agente no lo dijo: no se sabe", "", false, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			tok := enrolarConExec(t, s, "casa", "pc-gio")
			ts := servidorHTTP(t, s)

			cuerpo := `{"version":"0.1.0"}`
			if c.fuente != "" {
				cuerpo = `{"version":"0.1.0","token_fuente":"` + c.fuente + `"}`
			}
			if code, b := postCon(t, ts.URL+fleetHeartbeatPath, tok, cuerpo); code != 200 {
				t.Fatalf("latido: %d %s", code, b)
			}

			d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
			if d.TokenFuente != c.fuente {
				t.Errorf("la fila guardó %q y el agente dijo %q", d.TokenFuente, c.fuente)
			}
			rot, sabe := fleet.CredencialRotable(d.TokenFuente)
			if sabe != c.seSabe || rot != c.rotable {
				t.Errorf("CredencialRotable(%q) = (%v,%v), esperaba (%v,%v)", d.TokenFuente, rot, sabe, c.rotable, c.seSabe)
			}

			// Y QUE SE PUEDA LEER DESDE AFUERA, que es todo el punto: el dato guardado y no
			// expuesto deja la pregunta igual de incontestable que antes.
			fila := filaDeFleetList(t, s, "pc-gio")
			if c.seSabe {
				if fila["token_fuente"] != c.fuente {
					t.Errorf("fleet_list no expone la fuente: %v", fila["token_fuente"])
				}
				if v, ok := fila["token_rotable"].(bool); !ok || v != c.rotable {
					t.Errorf("fleet_list · token_rotable = %v, esperaba %v", fila["token_rotable"], c.rotable)
				}
			} else {
				// NO SE INVENTA UN CAMPO. «No lo dijo» se ve como ausente, no como false: un false
				// acusaría a la máquina de no poder rotar, que es un hecho que nadie midió.
				if _, hay := fila["token_fuente"]; hay {
					t.Errorf("con el agente callado, fleet_list inventó token_fuente = %v", fila["token_fuente"])
				}
				if _, hay := fila["token_rotable"]; hay {
					t.Errorf("con el agente callado, fleet_list inventó token_rotable = %v", fila["token_rotable"])
				}
			}
		})
	}
}

// LA SERIE ESTÁ AUSENTE CUANDO EL AGENTE NO LO DIJO, Y ÉSA ES LA PRUEBA QUE IMPORTA.
//
// `0` en esta serie significa «reportó que su token vino por variable y NO puede rotar» — una
// acusación concreta sobre una máquina. Un agente VIEJO no manda el campo, y publicar 0 por su
// silencio marcaría como defectuosa a media flota sin que nadie lo haya medido. Es la regla que este
// plano repite en todas sus series: AUSENTE NO ES CERO.
//
// Sabotaje verificado que la pone en rojo: devolver `(0, true)` en vez de `(0, false)` cuando
// `CredencialRotable` dice que no se sabe.
func TestLaSerieDeRotabilidadNoExisteSiElAgenteNoLoDijo(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConExec(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)

	// (1) Agente viejo: late sin el campo.
	if code, b := postCon(t, ts.URL+fleetHeartbeatPath, tok, `{"version":"0.1.0"}`); code != 200 {
		t.Fatalf("latido: %d %s", code, b)
	}
	texto := exportar(t, s, nil)
	if strings.Contains(texto, "musubi_fleet_device_token_rotable{") {
		for _, l := range strings.Split(texto, "\n") {
			if strings.HasPrefix(l, "musubi_fleet_device_token_rotable{") {
				t.Errorf("con el agente callado la serie EXISTE, y su 0 acusa a la máquina de algo que "+
					"nadie midió:\n    %s", l)
			}
		}
	}

	// (2) El MISMO agente, ahora reportando: la serie aparece. Sin este control positivo, la
	// aserción de arriba pasaría igual contra un exportador que nunca publica la serie.
	if code, b := postCon(t, ts.URL+fleetHeartbeatPath, tok,
		`{"version":"0.1.0","token_fuente":"`+fleet.CredencialDeVariable+`"}`); code != 200 {
		t.Fatalf("latido: %d %s", code, b)
	}
	texto = exportar(t, s, nil)
	var linea string
	for _, l := range strings.Split(texto, "\n") {
		if strings.HasPrefix(l, "musubi_fleet_device_token_rotable{") {
			linea = l
		}
	}
	if linea == "" {
		t.Fatal("con el agente reportando `variable` la serie NO aparece: el exportador no la publica " +
			"nunca, y entonces la aserción de arriba pasaba por el motivo equivocado")
	}
	if !strings.HasSuffix(strings.TrimSpace(linea), " 0") {
		t.Errorf("reportó `variable` y la serie no vale 0: %s", linea)
	}
}

// UNA FUENTE QUE EL DOMINIO NO CONOCE NO SE GUARDA.
//
// El campo se va a usar para decidir si una máquina puede completar una rotación. Una fila con un
// texto que nadie puede interpretar se lee después como si significara algo, y el vacío YA tiene un
// significado definido —«no lo dijo»— que se trata correctamente. Guardar basura es peor que no
// guardar: rompe la única distinción que el campo mantiene.
func TestUnaFuenteDeCredencialDesconocidaNoSeGuarda(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConExec(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)

	// El latido NO falla: un agente que manda basura sigue siendo una máquina que hay que ver
	// latir. Lo que no pasa es que la basura entre a la fila.
	if code, b := postCon(t, ts.URL+fleetHeartbeatPath, tok,
		`{"version":"0.1.0","token_fuente":"inventado"}`); code != 200 {
		t.Fatalf("un latido con una fuente rara tiene que seguir aceptándose: %d %s", code, b)
	}
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	if d.TokenFuente != "" {
		t.Errorf("se guardó %q, que el dominio no entiende: una fila con un texto ilegible se lee "+
			"después como si significara algo", d.TokenFuente)
	}
	if err := s.engine.FijarFuenteDeCredencial(d.ID, "inventado"); err == nil {
		t.Error("el setter aceptó una fuente desconocida sin error")
	}
}
