package mcp

import (
	"slices"
	"strings"
	"testing"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// formasDisfrazadas arma un argv con `cabeza` adelante en las formas que LimpiarArgv deshace —con
// blancos alrededor y con una parte vacía o en blanco antes—, que son las formas en que lo que
// llega a la tool difiere de lo que se guarda y se ejecuta. Es el mismo eje que recorre
// formasDeLaCabeza en internal/fleet, acotado a lo que cabe en la cola de una máquina.
func formasDisfrazadas(cabeza string, cola ...string) [][]string {
	var out [][]string
	for _, pre := range [][]string{nil, {""}, {" "}} {
		for _, c := range []string{cabeza, " " + cabeza, "\t" + cabeza + "\n"} {
			out = append(out, append(append(append([]string{}, pre...), c), cola...))
		}
	}
	return out
}

// CON `exec` NO SE ENCOLA UNA OPERACIÓN INTERNA, VENGA COMO VENGA.
//
// DEFECTO VIVO en el árbol sano (5017a45), medido con este servidor de prueba: una credencial con
// SÓLO `exec` mandó `["", "musubi:pantalla", <sesión>, <contraseña>, "30m"]` y la tool contestó
// «encolado». La guarda miraba `args.Argv[0]` crudo —vacío, o sea «no es interna»—, EncolarComando
// limpiaba el argv a un `musubi:pantalla` perfecto, y el agente, que despacha sobre el argv limpio,
// habría puesto esa contraseña en RustDesk: una pantalla abierta sin `screen` y sin la fila de
// `screen_sessions` que registra quién miró (G7). Con `[" ", "\tmusubi:pantalla", …]` igual, y ahí
// además la fila quedaba guardada con el tab adelante, que el tapado de A74 compara exacto: medido,
// después de la entrega la contraseña seguía en claro en la tabla.
//
// Es la puerta lateral que S6 cerró —fabricar los mensajes de la pantalla teniendo sólo el permiso
// de ejecutar— abierta otra vez por la FORMA. Quien tiene `exec` sin allowlist ya puede correr
// cualquier cosa en la máquina; lo que se saltea es la compuerta y la bitácora de `screen`, o sea
// que `exec` y `screen` sean dos permisos. La guarda vieja
// (TestConExecNoSePuedeFabricarUnaSesionDePantalla) clavaba la forma en `["musubi:pantalla", …]`:
// sin partes adelante, que es justo donde argv[0] crudo y la cabeza limpia son el mismo elemento.
//
// EXPOSICIÓN: no se puede medir sin tocar el cerebro, y este tema no toca máquinas. Hace falta una
// credencial con `exec` SIN allowlist (con allowlist, PermiteArgv compara la cabeza limpia contra
// la lista y `musubi:pantalla` no está en ninguna). La fila que deja es indistinguible de una
// legítima: las 4 `musubi:pantalla` que midió la auditoría habría que cruzarlas contra
// `screen_sessions` para descartarlo.
//
// Sabotaje: que la guarda del exec vuelva a mirar sólo la primera parte cruda → pasan las formas
// con una parte vacía adelante.
// arnes: archivo="internal/mcp/methods_exec.go"
// arnes: de="\tif fleet.EsOperacionInterna(args.Argv) {"
// arnes: a="\tif len(args.Argv) > 0 && fleet.EsOperacionInterna(args.Argv[:1]) {"
// arnes: colision_ok="TestConExecNoSePuedeFabricarUnaSesionDePantalla"
func TestConExecNoSeEncolaUnaOperacionInternaDisfrazada(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	tok := enrolarConPantalla(t, s, "casa", "pc-gio")
	ts := servidorHTTP(t, s)
	postCon(t, ts.URL+fleetHeartbeatPath, tok, "")

	soloExec := &Principal{
		Name: "op", Role: RoleAdmin, Read: ReadAll, Write: WriteAny, ProjectID: "casa",
		Fleet: map[fleet.Cap][]string{fleet.CapExec: {"*"}},
	}
	exec := func(argv []string) (map[string]any, *RpcError) {
		t.Helper()
		res, e := callAsPrincipal(t, s, soloExec, "musubi_fleet_exec", map[string]any{
			"device": "pc-gio", "argv": argv, "no_wait": true,
		})
		if e != nil {
			return nil, e
		}
		return jsonOf(t, res), nil
	}

	const secreto = "ContraseñaElegida42"
	internas := []string{fleet.OpPantalla, fleet.OpShell, fleet.OpAvisar, fleet.OpPreguntar, "musubi:todavia-no-existe"}
	medidas := 0
	for _, op := range internas {
		for _, argv := range formasDisfrazadas(op, "ses-x", secreto, "30m") {
			medidas++
			out, e := exec(argv)
			if e == nil {
				// Lo que el agente despacha es lo guardado, limpiado otra vez de su lado.
				t.Errorf("FUGA DE CAPACIDAD: con sólo `exec` se encoló %q (%v): el agente lo despacha como "+
					"%q, una operación interna del canal", argv, out["argv"], fleet.LimpiarArgv(fleet.LimpiarArgv(argv))[0])
				continue
			}
			// EL MOTIVO, y no sólo el rechazo: la cola de una máquina tiene techo, y un rechazo por
			// cola llena también es `unauthorized`. Sin esto, llenar la cola con los primeros
			// disfrazados haría pasar a los demás por rechazados.
			if e.Code != codeUnauthorized || !strings.Contains(e.Message, "operaciones internas") {
				t.Errorf("%q se rechazó por otro motivo (%d): %s", argv, e.Code, e.Message)
			}
		}
	}
	if medidas < len(internas)*len(formasDisfrazadas("x")) {
		t.Fatalf("se midieron %d formas: la prueba no recorrió el eje", medidas)
	}
	cs, err := s.engine.BitacoraDeComandos("casa", "", 200)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cs {
		if fleet.EsOperacionInterna(c.Argv) {
			t.Errorf("quedó encolada una operación interna pedida por exec: %q", c.Argv)
		}
		if strings.Contains(strings.Join(c.Argv, " "), secreto) {
			t.Errorf("la contraseña quedó guardada en la cola: %q", c.Argv)
		}
	}

	// CONTROL: las mismas formas con un comando del host SE ENCOLAN —sin esto, una guarda que
	// rechazara todo pasaría la mitad de arriba— y lo que se guarda es lo que el agente va a
	// ejecutar: limpiarlo otra vez, como hace el agente, no lo cambia.
	for _, argv := range formasDisfrazadas("uptime", "-p") {
		out, e := exec(argv)
		if e != nil {
			t.Errorf("un comando del host con la forma %q se rechazó: %s", argv, e.Message)
			continue
		}
		id, _ := out["command_id"].(string)
		c, ok, err := s.engine.ComandoPorID(id)
		if err != nil || !ok {
			t.Fatalf("no se pudo releer el comando %q (ok=%v): %v", id, ok, err)
		}
		if ejecuta := fleet.LimpiarArgv(c.Argv); !slices.Equal(ejecuta, c.Argv) {
			t.Errorf("con %q se guardó %q y el agente ejecuta %q: lo registrado no es lo corrido", argv, c.Argv, ejecuta)
		}
	}
}
