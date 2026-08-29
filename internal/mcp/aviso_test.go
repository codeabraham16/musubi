package mcp

// Pruebas de la mitad del AGENTE del eje de consentimiento (A57), del lado del cerebro: recibir
// la capacidad medida y encolar el aviso que `avisa` promete.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

func cuerpoConCapacidad(puede *bool, motivo string) string {
	c := map[string]any{"version": "v0-prueba"}
	if puede != nil {
		c["puede_preguntar"] = *puede
	}
	if motivo != "" {
		c["motivo_no_preguntar"] = motivo
	}
	b, _ := json.Marshal(c)
	return string(b)
}

func boolPtr(b bool) *bool { return &b }

// UN AGENTE VIEJO NO OPINA, Y ESO NO ES «NO PUEDO».
//
// Es la distinción que hace al campo un PUNTERO. Con un bool pelado, un agente que no manda el
// campo sería indistinguible de uno que midió y dijo que no — y como `puede_preguntar` endurece
// un `pide` a `prohibido`, esa confusión cerraría el acceso por pantalla a máquinas que quizás sí
// pueden, sin que nada lo dijera. La primera flota con agentes mezclados se rompería callada.
//
// Sabotaje que la hace fallar: cambiar PuedePreguntar de *bool a bool en cuerpoLatido.
func TestUnAgenteViejoQueNoOpinaNoPisaLaCapacidadMedida(t *testing.T) {
	s, ts, tokenDevice, _ := servidorConFlota(t)
	id := idDeDevice(t, s, "pc-gio")

	// Un agente NUEVO mide y dice que sí.
	if code, body := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice,
		cuerpoConCapacidad(boolPtr(true), "")); code != http.StatusOK {
		t.Fatalf("el latido devolvió %d: %s", code, body)
	}
	d, _, _ := s.engine.DevicePorID(id)
	if !d.PuedePreguntar {
		t.Fatal("la capacidad afirmativa no se guardó")
	}

	// Ahora late un agente VIEJO: sin el campo. NO tiene que pisar nada.
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice,
		cuerpoConCapacidad(nil, "")); code != http.StatusOK {
		t.Fatal("el latido del agente viejo falló")
	}
	d, _, _ = s.engine.DevicePorID(id)
	if !d.PuedePreguntar {
		t.Error("un agente que NO manda el campo borró la capacidad medida: «no opinó» se " +
			"confundió con «no puedo», y un `pide` en esta máquina se endurecería a `prohibido` " +
			"sin que nadie lo haya medido")
	}

	// Y un `false` EXPLÍCITO sí escribe: una máquina que perdió su escritorio tiene que dejar de
	// declarar que puede.
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice,
		cuerpoConCapacidad(boolPtr(false), "el agente corre como servicio")); code != http.StatusOK {
		t.Fatal("el latido con false falló")
	}
	d, _, _ = s.engine.DevicePorID(id)
	if d.PuedePreguntar {
		t.Error("un `false` explícito no bajó la capacidad: una máquina que perdió su escritorio " +
			"seguiría prometiendo que puede preguntar")
	}
}

// EL AVISO SE ENCOLA CUANDO EL AGENTE SABE DARLO, y es lo que `avisa` prometía y no entregaba.
//
// Sabotaje que la hace fallar: sacar el `case consent.AvisaAlUsuario()` que encola.
// Sabotaje que la hace fallar: encolar el aviso DESPUÉS de crear la sesión (llega tarde).
func TestConUnAgenteQueSabeAvisarElAvisoSeEncola(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tokenDevice := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	id := idDeDevice(t, s, "pc-gio")

	// El agente declara que sabe avisar, y late con muestra para figurar en línea.
	cuerpo := map[string]any{"version": "v1", "puede_preguntar": true,
		"rustdesk_id": "123456789", "muestra": muestraSana(30, time.Now())}
	b, _ := json.Marshal(cuerpo)
	if code, body := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, string(b)); code != http.StatusOK {
		t.Fatalf("el latido devolvió %d: %s", code, body)
	}
	// `avisa` es el grado por defecto, así que no hace falta fijarlo.
	d, _, _ := s.engine.DevicePorID(id)
	if !d.PuedePreguntar {
		t.Fatal("la capacidad no se guardó: el resto de la prueba no probaría nada")
	}

	antes := comandosEncolados(t, s)
	principal := conPantalla("casa")
	if _, e := callAsPrincipal(t, s, principal, "musubi_fleet_screen",
		map[string]any{"device": "pc-gio"}); e != nil {
		t.Fatalf("fleet_screen: %+v", e)
	}

	nuevos := comandosEncolados(t, s)
	var aviso *fleet.Comando
	for i := range nuevos {
		if len(nuevos[i].Argv) > 0 && nuevos[i].Argv[0] == comandoAviso {
			aviso = &nuevos[i]
		}
	}
	if aviso == nil {
		t.Fatalf("no se encoló ningún aviso (antes %d comandos, ahora %d): `avisa` sigue "+
			"prometiendo una notificación que no viaja", len(antes), len(nuevos))
	}
	// EL TEXTO NOMBRA A QUIEN ENTRA. «Alguien está viendo tu pantalla» no le sirve a nadie: sin
	// el nombre no hay a quién preguntarle, y el aviso se vuelve ruido que se cierra sin leer.
	if len(aviso.Argv) < 2 {
		t.Fatalf("el aviso viajó sin texto: %#v", aviso.Argv)
	}
	if !strings.Contains(aviso.Argv[1], "mirador") {
		t.Errorf("el texto del aviso no nombra a quien entra: %q", aviso.Argv[1])
	}
	if !strings.Contains(aviso.Argv[1], "pantalla") {
		t.Errorf("el texto no dice QUÉ está pasando: %q", aviso.Argv[1])
	}
}

// SIN AGENTE QUE SEPA AVISAR, LA PANTALLA SE ABRE IGUAL Y NO SE ENCOLA NADA.
//
// `avisa` NO bloquea —ése es el grado siguiente— y encolar un aviso que nadie va a poder mostrar
// dejaría un comando pendiente para siempre en la cola de esa máquina, que además es ruido en la
// bitácora. Se deja la constancia en el log y listo.
//
// Sabotaje que la hace fallar: encolar el aviso sin mirar PuedePreguntar.
func TestSinCapacidadDeAvisarNoSeEncolaUnAvisoQueNadieVaAMostrar(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tokenDevice := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)

	cuerpo := map[string]any{"version": "v1", "puede_preguntar": false,
		"motivo_no_preguntar": "el agente corre como servicio de sistema",
		"rustdesk_id":         "987654321", "muestra": muestraSana(30, time.Now())}
	b, _ := json.Marshal(cuerpo)
	if code, _ := postCon(t, ts.URL+fleetHeartbeatPath, tokenDevice, string(b)); code != http.StatusOK {
		t.Fatal("el latido falló")
	}

	if _, e := callAsPrincipal(t, s, conPantalla("casa"), "musubi_fleet_screen",
		map[string]any{"device": "pc-gio"}); e != nil {
		t.Fatalf("la pantalla NO se abrió, y `avisa` no bloquea: %+v", e)
	}
	for _, c := range comandosEncolados(t, s) {
		if len(c.Argv) > 0 && c.Argv[0] == comandoAviso {
			t.Error("se encoló un aviso que esta máquina no puede mostrar: queda pendiente para " +
				"siempre en su cola y ensucia la bitácora")
		}
	}
}
