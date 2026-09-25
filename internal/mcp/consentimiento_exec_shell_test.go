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
// Sabotaje que la hace fallar: sacar el `case consent.Bloquea()` de toolFleetExec, o el
// `if consent.Bloquea()` de toolFleetShell. Cada uno rompe su propio subtest.
//
// (Las dos formas estaban CRUZADAS en esta línea hasta el 2026-09-21. Medido: el `case` vive en
// `methods_exec.go:62`, adentro de un switch, y el `if` en `methods_shell.go:76`. El corte valía
// igual de los dos lados —por eso nadie lo notó— pero mandaba a buscar la forma equivocada en
// cada archivo.)
// arnes: archivo="internal/mcp/methods_exec.go"
// arnes: de="\tcase consent.Bloquea():"
// arnes: a="\tcase consent.Bloquea() && false:"
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
// arnes: archivo="internal/mcp/methods_exec.go"
// arnes: de="\tcase consent.AvisaAlUsuario():\n\t\ts.encolarAvisoDeExecConVentana(d, p)\n"
// arnes: a=""
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

// A83 — TODO CAMINO QUE HONRA `avisa` LE AVISA A QUIEN ESTÁ EN LA MÁQUINA. LOS TRES.
//
// La shell no lo hacía. `exec` y `pantalla` encolan su aviso desde A57; la shell tenía SÓLO la
// rama del agente que no sabe notificar —la que deja una línea en el log— y ninguna para la
// máquina que SÍ sabe: el `if` no tenía `else`. En la máquina donde el aviso podía entregarse,
// abrir una terminal no le decía una palabra a quien estaba sentado ahí.
//
// Que fuera justo la shell es lo peor: este archivo argumenta que una shell interactiva se lleva
// puestos los otros dos permisos —«quien obtiene un prompt corre lo que quiera, las veces que
// quiera, sin que nadie vuelva a mirar un argv»— y methods_shell.go dice de los tres ejes que éste
// «es el que MÁS le corresponde a la shell». El eje estaba escrito, la razón estaba escrita, y la
// mitad que avisa no estaba.
//
// LA CAUSA ERA MECÁNICA, y por eso esta prueba recorre los TRES y no sólo la shell: el bloque que
// encola estaba COPIADO en pantalla y en exec, así que sumar un camino exigía acordarse de
// copiarlo por tercera vez. Arreglar sólo la shell —escribiendo esa tercera copia— dejaba la causa
// intacta para el cuarto camino. Ahora hay un solo encolarAvisoDeAcceso, y esta tabla es lo que
// obliga a que el que agregue un camino aparezca acá.
//
// NINGUNA PRUEBA LO ATRAPABA porque ninguna combinaba las dos condiciones: `avisa` con una máquina
// que declara saber notificar. Las de shell probaban `prohibido` (que bloquea) y las de aviso
// probaban pantalla y exec.
//
// Sabotaje que la hace fallar: sacar el `if` que encola el aviso en cualquiera de los tres.
// arnes: archivo="internal/mcp/methods_shell.go"
// arnes: de="\tif d.ConsentimientoEfectivo().AvisaAlUsuario() {\n\t\ts.encolarAvisoDeAcceso(d, p, avisoShell)\n\t}\n"
// arnes: a=""
func TestTodoCaminoQueHonraAvisaLeAvisaAlUsuario(t *testing.T) {
	for _, c := range []struct {
		nombre, espera string
		correr         func(*McpServer, context.Context) *RpcError
	}{
		{"shell", "terminal", func(s *McpServer, ctx context.Context) *RpcError {
			_, e := s.toolFleetShell(ctx, json.RawMessage(`{"project":"casa","device":"pc-gio"}`))
			return e
		}},
		// `no_wait` en exec porque la máquina está latiendo y sin él la tool espera el resultado
		// de un comando que ningún agente va a reportar: 45 s de prueba para medir algo que se
		// decide al encolar. El aviso no depende de esperar.
		{"exec", "ejecutando comandos", func(s *McpServer, ctx context.Context) *RpcError {
			_, e := s.toolFleetExec(ctx, json.RawMessage(`{"project":"casa","device":"pc-gio","argv":["echo","hola"],"no_wait":true}`))
			return e
		}},
		{"pantalla", "sesión de pantalla", func(s *McpServer, ctx context.Context) *RpcError {
			_, e := s.toolFleetScreen(ctx, json.RawMessage(`{"project":"casa","device":"pc-gio"}`))
			return e
		}},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
				"name": "pc-gio", "tier": "A", "caps": []string{"metrics", "exec", "shell", "screen"},
				"project": "casa", "os": "linux", "arch": "amd64",
			}); e != nil {
				t.Fatalf("no se pudo enrolar: %+v", e)
			}
			d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
			if _, err := s.engine.FijarConsentimiento(d.ID, fleet.ConsentimientoAvisa); err != nil {
				t.Fatal(err)
			}
			// El agente DECLARA que sabe avisar. Sin esto el camino correcto es el del log, que en
			// la shell era la ÚNICA rama que existía — y la prueba pasaría sobre el defecto.
			if err := s.engine.FijarCapacidadDePreguntar(d.ID, true); err != nil {
				t.Fatal(err)
			}

			// LA MÁQUINA TIENE QUE ESTAR LATIENDO para el camino de pantalla: sin latido, esa
			// tool sale antes con «no hay a quién entregarle la contraseña de sesión» y la prueba
			// mediría un rechazo, no el aviso. Es la clase de detalle que hace pasar una prueba
			// por el motivo equivocado, así que se le da un latido a las tres por igual.
			if _, err := s.engine.LatirDevice(d.ID, time.Now(), ""); err != nil {
				t.Fatal(err)
			}

			p := conShell("casa")
			p.Fleet[fleet.CapScreen] = []string{"*"}
			ctx := withPrincipal(context.Background(), p)
			// El resultado de la operación no importa acá —una pantalla puede fallar por no tener
			// motor— pero el aviso se encola igual, porque se decide ANTES y `avisa` no bloquea.
			_ = c.correr(s, ctx)

			cmds, err := s.engine.TomarComandos(d.ID, time.Now(), 100)
			if err != nil {
				t.Fatal(err)
			}
			var aviso *fleet.Comando
			for i := range cmds {
				if len(cmds[i].Argv) > 0 && cmds[i].Argv[0] == comandoAviso {
					aviso = &cmds[i]
				}
			}
			if aviso == nil {
				t.Fatalf("%s en `avisa` no encoló ningún aviso al usuario: %d comando(s) en la cola "+
					"y ninguno es uno", c.nombre, len(cmds))
			}
			// EL TEXTO NOMBRA A QUIEN ENTRA: un aviso sin nombre no se puede accionar —no hay a
			// quién preguntarle— y se vuelve ruido que se aprende a cerrar sin leer.
			if len(aviso.Argv) < 2 || !strings.Contains(aviso.Argv[1], "op") {
				t.Errorf("%s: el aviso no nombra a quien entra: %v", c.nombre, aviso.Argv)
			}
			// Y DICE QUÉ ESTÁ PASANDO, distinto en cada camino: sin eso, quien lo recibe no puede
			// distinguir una terminal de una sesión de pantalla, que son cosas distintas para él.
			if len(aviso.Argv) < 2 || !strings.Contains(aviso.Argv[1], c.espera) {
				t.Errorf("%s: el aviso no dice qué está pasando (esperaba %q): %v",
					c.nombre, c.espera, aviso.Argv)
			}
		})
	}
}

// Y LA MÁQUINA QUE NO SABE AVISAR SIGUE YENDO POR EL LOG, no por la cola.
//
// Es el control negativo de la prueba de arriba: sin él, encolar el aviso SIEMPRE —también donde
// el agente no lo sabe entregar— la dejaría en verde, y estaríamos prometiendo una notificación
// que nadie va a mostrar. Que es literalmente lo que el eje viene a evitar.
//
// Sabotaje que la hace fallar: que el embudo (encolarAvisoDeAcceso) deje pasar a la shell como si
// su máquina supiera avisar.
//
// EL CAMINO QUE ESTA PRUEBA MIDE ES EL DE LA SHELL, y conviene que lo diga la directiva y no haya
// que deducirlo: abre con `toolFleetShell`, así que el `case` que le importa es el de
// `methods_shell.go`. Mecanizarla primero contra `methods_exec.go` —el nombre del archivo de
// pruebas dice «exec_shell» y el `case` es idéntico en los dos— la dejó en VERDE sobre su propio
// sabotaje. Son tres hermanas con la misma forma, una por camino.
//
// Y AL BUSCAR A LAS OTRAS DOS APARECIÓ QUE EXEC NO TIENE HERMANA. Pantalla la tiene en
// `aviso_test.go` (`TestSinCapacidadDeAvisarNoSeEncolaUnAvisoQueNadieVaAMostrar`) y shell la tiene
// acá. Exec no tiene ninguna que lo diga: `TestTodoCaminoQueHonraAvisaLeAvisaAlUsuario` recorre los
// tres, pero prueba la dirección CONTRARIA —que un `avisa` sí avisa—, que es otra cosa.
//
// No es que nadie lo note: es que lo notan por aritmética ajena. Medido el 2026-09-21 poniéndole
// `&& false` al `case` de `methods_exec.go` y corriendo el paquete entero — la suite se pone roja,
// pero en tres pruebas que no hablan de esto y por CONTAR comandos:
//
//	fleet_exec_test.go:323     se entregaron 6 comandos distintos, esperaba 5
//	fleet_exec_test.go:454     esperaba 1 comando encolado, hay 2
//	fleet_pantalla_test.go:194 quedaron 1 comandos encolados pese al rechazo
//
// Ninguna dice «se encoló un aviso para una máquina que no declara saber notificar». El aviso de
// más les corre un conteo, y eso alcanza HOY: el día que cualquiera de las tres cambie su fixture,
// la detección se evapora sin que nada lo diga. Un invariante detectado de rebote no está
// guardado; falta la hermana que lo nombre.
//
// Y EL SABOTAJE SE MUDÓ AL EMBUDO, sin dejar de ser de la shell (2026-09-24). La directiva ponía
// `&& false` en el `case` de `methods_shell.go`, y la primera corrida nocturna del arnés (PR #652)
// la dejó en VERDE en Linux y acá: desde #621 la misma precondición vivía TAMBIÉN en el embudo,
// que la atajaba igual. No era la guarda: era un sabotaje que ya no rompía nada. Se sacaron las
// copias de los llamadores y el sabotaje exceptúa SÓLO al plano de la shell en el embudo —la
// forma exacta del defecto que #621 cerró para la política—, así que la que se pone roja es ésta y
// no sus hermanas de exec y pantalla.
// arnes: archivo="internal/mcp/methods_pantalla.go"
// arnes: de="func (s *McpServer) encolarAvisoDeAcceso(d fleet.Device, p *Principal, a avisoDeAcceso) bool {"
// arnes: a="func (s *McpServer) encolarAvisoDeAcceso(d fleet.Device, p *Principal, a avisoDeAcceso) bool {\n\tif a.operacion == \"shell\" {\n\t\td.PuedePreguntar = true\n\t}"
func TestUnaMaquinaQueNoSabeAvisarNoRecibeUnAvisoQueNadieVaAMostrar(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]any{
		"name": "pc-gio", "tier": "A", "caps": []string{"metrics", "exec", "shell"},
		"project": "casa", "os": "linux", "arch": "amd64",
	}); e != nil {
		t.Fatalf("no se pudo enrolar: %+v", e)
	}
	d, _, _ := s.engine.DevicePorNombre("casa", "pc-gio")
	if _, err := s.engine.FijarConsentimiento(d.ID, fleet.ConsentimientoAvisa); err != nil {
		t.Fatal(err)
	}
	// PuedePreguntar queda en false: es el default de una máquina cuyo agente no lo declaró.

	ctx := withPrincipal(context.Background(), conShell("casa"))
	if _, e := s.toolFleetShell(ctx, json.RawMessage(`{"project":"casa","device":"pc-gio"}`)); e != nil {
		t.Fatalf("no se abrió la shell: %+v", e)
	}
	cmds, err := s.engine.TomarComandos(d.ID, time.Now(), 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cmds {
		if len(c.Argv) > 0 && c.Argv[0] == comandoAviso {
			t.Errorf("se encoló un aviso para una máquina que no declara saber notificar: %v — "+
				"prometer una notificación que no se puede entregar es lo que este eje evita", c.Argv)
		}
	}
}

// ═════════════════════════════════════════════════════════════════════════════════════════════
// Y POR EXEC TAMPOCO. ESTA ES LA HERMANA QUE FALTABA.
//
// El invariante es uno solo —a una máquina cuyo agente no declara saber notificar no se le encola
// un aviso que nadie va a mostrar— y el código lo implementa TRES VECES, con el mismo `case`
// idéntico carácter por carácter: `methods_pantalla.go`, `methods_exec.go` y `methods_shell.go`.
// Pantalla lo custodiaba `aviso_test.go` y shell la prueba de acá arriba. Exec no tenía a nadie.
//
// NO ES QUE NADIE LO NOTARA: ES QUE LO NOTABAN POR ARITMÉTICA AJENA, y esa diferencia es todo el
// punto. Medido el 2026-09-21 poniéndole `&& false` al `case` de `methods_exec.go` y corriendo
// `go test ./internal/mcp` entero: la suite SE PONE ROJA, pero en tres pruebas que no hablan de
// esto y que fallan CONTANDO comandos —
//
//	fleet_exec_test.go:323      se entregaron 6 comandos distintos, esperaba 5
//	fleet_exec_test.go:454      esperaba 1 comando encolado, hay 2
//	fleet_pantalla_test.go:194  quedaron 1 comandos encolados pese al rechazo
//
// El aviso de más les corre un conteo. Ninguna dice «se encoló un aviso para una máquina que no
// declara saber notificar», así que el día que cualquiera de las tres cambie su fixture la
// detección se evapora y nada lo dice. Un invariante detectado de rebote no está guardado: la
// prueba que lo NOMBRA es la que sobrevive a que le muevan el decorado alrededor.
//
// (Y lo que lo destapó vale para el próximo: la prueba de shell de acá arriba se mecanizó primero
// contra `methods_exec.go` —el nombre del archivo dice «exec_shell» y el `case` es el mismo— y
// quedó en VERDE sobre su propio sabotaje, porque abre con `toolFleetShell`. Al ir a corregir el
// archivo apareció que exec no tenía guarda propia.)
//
// `TestTodoCaminoQueHonraAvisaLeAvisaAlUsuario` recorre los tres caminos y NO cubre esto: prueba la
// dirección contraria —que un `avisa` sí avisa—, que es la otra mitad del eje.
//
// Sabotaje que la hace fallar: que el embudo (encolarAvisoDeAcceso) deje pasar al plano de exec
// como si su máquina supiera avisar. Antes era sacar el `&& !d.PuedePreguntar` del `case` de
// aplicarConsentimientoDeExec, y la corrida nocturna del PR #652 lo dejó en VERDE: esa copia de la
// precondición la tapaba el embudo, que desde #621 la tenía también. Las copias se sacaron y la
// precondición vive en un solo lugar; el sabotaje la exceptúa sólo para exec.
// arnes: archivo="internal/mcp/methods_pantalla.go"
// arnes: de="func (s *McpServer) encolarAvisoDeAcceso(d fleet.Device, p *Principal, a avisoDeAcceso) bool {"
// arnes: a="func (s *McpServer) encolarAvisoDeAcceso(d fleet.Device, p *Principal, a avisoDeAcceso) bool {\n\tif a.operacion == \"exec\" {\n\t\td.PuedePreguntar = true\n\t}"
func TestUnaMaquinaQueNoSabeAvisarTampocoRecibeUnAvisoPorExec(t *testing.T) {
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
	// NO se llama a FijarCapacidadDePreguntar: `puede_preguntar` queda en false, que es el default
	// de una máquina cuyo agente nunca declaró saber notificar. Es justamente el caso que se fija.

	ctx := context.Background()
	if _, e := s.toolFleetExec(ctx, json.RawMessage(`{"project":"casa","device":"pc-gio","argv":["echo","hola"]}`)); e != nil {
		t.Fatalf("el exec se rechazó, y `avisa` NO bloquea —ése es el grado siguiente—: %s", e.Message)
	}

	cmds, err := s.engine.TomarComandos(d.ID, time.Now(), 100)
	if err != nil {
		t.Fatal(err)
	}
	// CONTROL DE «MIRÓ ALGO»: si el exec no llegó a encolarse, no hay nada que contar y el verde
	// sería sobre la nada. Tiene que haber exactamente el comando del exec y ninguno más.
	if len(cmds) == 0 {
		t.Fatal("no se encoló ni el propio exec: la prueba no llegó a ejercitar el camino del aviso")
	}
	for _, c := range cmds {
		if len(c.Argv) > 0 && c.Argv[0] == comandoAviso {
			t.Errorf("se encoló un aviso POR EXEC para una máquina que no declara saber notificar: %v — "+
				"prometer una notificación que no se puede entregar es lo que este eje evita, y la "+
				"constancia va al log una vez por máquina", c.Argv)
		}
	}
}
