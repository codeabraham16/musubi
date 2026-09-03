package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// A75 — EL TECHO DEL DUEÑO DE LA MÁQUINA VALE EN LOS TRES CAMINOS, NO SÓLO EN LA PANTALLA.
//
// El eje de consentimiento se resuelve en Device.ConsentimientoEfectivo y, hasta esta prueba, lo
// consultaba UN SOLO camino: el de pantalla. En una máquina marcada `prohibido`, mirar la pantalla
// se rechazaba y ABRIR UNA SHELL no — siendo que una shell interactiva se saltea cualquier
// allowlist de comandos, o sea que puede estrictamente más que una pantalla. La asimetría no
// estaba escrita en ningún lado.
//
// `prohibido` es el único grado cuyo significado no depende de una decisión pendiente: quiere
// decir «acá no entra nadie de forma interactiva». `avisa` y `pide` sobre exec son otra cosa y
// siguen abiertos a propósito (ver A75 en specs/control-de-flota/ABIERTO.md).
//
// Sabotaje que la hace fallar: sacar el `if consent.Bloquea()` de toolFleetExec, o el
// `case consent.Bloquea()` de toolFleetShell. Cada uno rompe su propio subtest.
func TestElConsentimientoProhibidoTambienCierraExecYShell(t *testing.T) {
	casos := []struct {
		nombre string
		correr func(*McpServer, context.Context) *RpcError
	}{
		{"exec", func(s *McpServer, ctx context.Context) *RpcError {
			_, e := s.toolFleetExec(ctx, json.RawMessage(`{"project":"casa","device":"pc-gio","argv":["echo","hola"]}`))
			return e
		}},
		{"shell", func(s *McpServer, ctx context.Context) *RpcError {
			_, e := s.toolFleetShell(ctx, json.RawMessage(`{"project":"casa","device":"pc-gio"}`))
			return e
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			// La máquina se enrola CON `shell`: el helper compartido sólo concede metrics+exec, y
			// sin la capacidad el rechazo llegaría por falta de permiso — o sea, la prueba pasaría
			// por el motivo equivocado. El tramo de control de abajo lo detecta, y esto lo evita.
			if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
				"name": "pc-gio", "tier": "A", "caps": []string{"metrics", "exec", "shell"},
				"project": "casa", "os": "linux", "arch": "amd64",
			}); e != nil {
				t.Fatalf("no se pudo enrolar la máquina de prueba: %+v", e)
			}
			ctx := context.Background()

			d, hay, err := s.engine.DevicePorNombre("casa", "pc-gio")
			if err != nil || !hay {
				t.Fatalf("no se pudo leer la máquina de prueba: %v", err)
			}

			// CONTROL: con el consentimiento por default la tool NO se rechaza por consentimiento.
			// Sin este tramo, un rechazo por cualquier otro motivo —una capacidad que falta, un
			// device que no late— haría pasar la prueba sin que la guarda exista.
			if e := c.correr(s, ctx); e != nil && strings.Contains(e.Message, "PROHIBIDO por configuración de consentimiento") {
				t.Fatalf("con el consentimiento por default ya se rechaza por consentimiento: %s", e.Message)
			}

			if _, err := s.engine.FijarConsentimiento(d.ID, fleet.ConsentimientoProhibido); err != nil {
				t.Fatalf("no se pudo marcar la máquina como prohibido: %v", err)
			}

			e := c.correr(s, ctx)
			if e == nil {
				t.Fatalf("la máquina está marcada `prohibido` y %s se ejecutó igual: el candado del dueño lo mira sólo el camino de pantalla", c.nombre)
			}
			if !strings.Contains(e.Message, "PROHIBIDO por configuración de consentimiento") {
				t.Errorf("%s se rechazó por otro motivo y no por el consentimiento: %s", c.nombre, e.Message)
			}
			// El error tiene que mandar a mirar el OTRO lado: la capacidad puede estar concedida.
			// Confundirlo con un problema de permisos manda a alguien a revisar principals.yaml
			// durante media hora buscando algo que ya está.
			if !strings.Contains(e.Message, "no es un problema de permisos") {
				t.Errorf("%s: el mensaje no distingue el candado del dueño de una falta de permiso: %s", c.nombre, e.Message)
			}
		})
	}
}

// A75 CERRADO — `avisa` AVISA EN EL EXEC, PERO UNA VEZ POR VENTANA.
//
// Era la mitad que quedaba abierta, y las tres opciones estaban escritas en la fila: no avisar
// nunca, avisar siempre, o avisar con estrangulador. Se eligió el estrangulador porque el modo de
// falla de las otras dos es peor: avisar en cada comando convierte el aviso en ruido —veinte
// ventanitas seguidas es cómo se le enseña a alguien a poner `libre` en todas sus máquinas— y no
// avisar nunca deja al eje mintiendo, en el camino que puede MÁS que la pantalla.
//
// Sabotaje que la hace fallar: sacar el `case consent.AvisaAlUsuario()` de
// aplicarConsentimientoDeExec (deja de avisar), o sacar el chequeo de la ventana en
// encolarAvisoDeExecConVentana (avisa en cada comando).
func TestElAvisaDelExecAvisaUnaVezPorVentanaYNoUnaPorComando(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "pc-gio", "tier": "A", "caps": []string{"metrics", "exec"},
		"project": "casa", "os": "linux", "arch": "amd64",
	}); e != nil {
		t.Fatalf("no se pudo enrolar: %+v", e)
	}
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	if _, err := s.engine.FijarConsentimiento(d.ID, fleet.ConsentimientoAvisa); err != nil {
		t.Fatal(err)
	}
	// El agente tiene que declarar que SABE avisar; si no, el camino correcto es el del log.
	if err := s.engine.FijarCapacidadDePreguntar(d.ID, true); err != nil {
		t.Fatal(err)
	}

	acumulados := 0
	avisos := func() int {
		t.Helper()
		n := acumulados
		// TomarComandos entrega y MARCA entregado, así que se llama una sola vez por medición y
		// se acumula: llamarla dos veces devolvería vacío la segunda y la prueba contaría 0 sin
		// que nada esté mal.
		cmds, err := s.engine.TomarComandos(d.ID, time.Now(), 100)
		if err != nil {
			t.Fatalf("no se pudo leer la cola: %v", err)
		}
		for _, c := range cmds {
			if len(c.Argv) > 0 && c.Argv[0] == comandoAviso {
				n++
			}
		}
		acumulados = n
		return n
	}

	ctx := context.Background()
	correr := func() {
		t.Helper()
		if _, e := s.toolFleetExec(ctx, json.RawMessage(`{"project":"casa","device":"pc-gio","argv":["echo","hola"]}`)); e != nil {
			t.Fatalf("el exec se rechazó y `avisa` no bloquea: %s", e.Message)
		}
	}

	correr()
	if n := avisos(); n != 1 {
		t.Fatalf("el primer exec dejó %d avisos, esperaba 1: en una máquina marcada `avisa`, quien está sentado adelante no se entera de que le ejecutaron algo", n)
	}
	// Una tanda de trabajo: cinco comandos más, ningún aviso más.
	for i := 0; i < 5; i++ {
		correr()
	}
	if n := avisos(); n != 1 {
		t.Errorf("seis exec dejaron %d avisos: una ventanita por comando es exactamente cómo se le enseña a alguien a poner `libre` en todas sus máquinas", n)
	}

	// Y pasada la ventana, vuelve a avisar: el estrangulador no puede volverse un silencio
	// permanente. Se envejece la marca en vez de dormir la prueba.
	s.avisosDados.Store("aviso_exec\x00"+d.ID, time.Now().Add(-ventanaDeAvisoDeExec-time.Minute))
	correr()
	if n := avisos(); n != 2 {
		t.Errorf("pasada la ventana quedaron %d avisos, esperaba 2: el estrangulador se volvió un silencio permanente", n)
	}
}
