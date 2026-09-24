package mcp

import (
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// TestLaAprobacionSeGastaTambienPorElCaminoDePide — EL GRADO QUE LA GUARDA DE UN-SOLO-USO CLAVA.
//
// `TestLaAprobacionSeGastaEnUnaSolaSesion` varía la REPETICIÓN —pide dos veces y exige que la
// segunda no abra— y clava el grado de consentimiento en el default. Con `pide`, el camino a la
// pantalla es OTRO: `toolFleetScreen` se reparte temprano hacia `pedirPermisoParaPantalla`, y es
// ÉSE el que llama a `entregarPantalla` cuando el usuario contesta que sí. Los dos caminos pasan
// por el mismo `gastarAprobacion`, pero sólo uno tiene quien lo mire.
//
// MEDIDO EL 2026-09-22, con control: envolviendo esa llamada en un `if consent != pide`, el
// PAQUETE `internal/mcp` ENTERO queda en verde y la aprobación sobrevive a la sesión que abrió —
// `estado="concedida"`, `usada=false`—, o sea que un permiso de un solo uso queda disponible para
// una segunda.
//
// SE PREGUNTA AL MOTOR Y NO SE PIDE UNA SEGUNDA SESIÓN, porque la reusabilidad se puede tapar
// aguas abajo por cualquier otra compuerta: lo que este eje promete es que el permiso quedó
// GASTADO, y eso es un hecho del almacén.
//
// Sabotaje que la hace fallar: saltear el gasto de la aprobación cuando el consentimiento es
// `pide` — o sea, en el único camino que esta guarda cubre y la vecina no.
// arnes: archivo="internal/mcp/methods_pantalla.go"
// arnes: de="\tif e := s.gastarAprobacion(d, p, fleet.CapScreen, time.Now().UTC()); e != nil {\n\t\treturn nil, e\n\t}"
// arnes: a="\tif consent := d.ConsentimientoEfectivo(); consent != fleet.ConsentimientoPide {\n\t\tif e := s.gastarAprobacion(d, p, fleet.CapScreen, time.Now().UTC()); e != nil {\n\t\t\treturn nil, e\n\t\t}\n\t}"
func TestLaAprobacionSeGastaTambienPorElCaminoDePide(t *testing.T) {
	s, ts, tokDev := maquinaQuePide(t)
	marcarCuatroOjos(t, s, "casa", "pc-gio")
	yo, otra := conPantalla("casa"), otroConPantalla("casa")

	// Los cuatro ojos van ANTES que el `pide`: el primer pedido abre la solicitud de aprobación.
	res, _ := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	solID, _ := jsonOf(t, res)["solicitud"].(string)
	if solID == "" {
		t.Fatalf("no se abrió la solicitud de cuatro ojos: %v", jsonOf(t, res))
	}
	if _, e := callAsPrincipal(t, s, otra, "musubi_fleet_approve", map[string]any{
		"solicitud": solID, "aprobar": true,
	}); e != nil {
		t.Fatalf("approve: %+v", e)
	}

	// Con la aprobación puesta, el pedido dispara la pregunta al usuario de la máquina.
	callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	cmdID := idDelComando(t, s, comandoPreguntar)
	responder(t, ts, tokDev, cmdID, prefijoRespuestaPermiso+string(fleet.RespuestaConcedida))

	// Dijo que sí: la sesión abre, y ACÁ tiene que quedar gastada la aprobación.
	res3, e := callAsPrincipal(t, s, yo, "musubi_fleet_screen", map[string]any{"device": "pc-gio"})
	if e != nil {
		t.Fatalf("CONTROL ROTO: el pedido con el permiso concedido devolvió un error: %+v", e)
	}
	m := jsonOf(t, res3)
	if m["password"] == nil || m["password"] == "" {
		t.Fatalf("CONTROL ROTO: con los cuatro ojos aprobados y el usuario diciendo que sí, la pantalla "+
			"no abrió, así que la comprobación de abajo no mediría nada: %v", m)
	}

	sol, hay, err := s.engine.AprobacionVigenteDe(idDeDevice(t, s, "pc-gio"), "mirador", fleet.CapScreen, time.Now().UTC())
	if err != nil {
		t.Fatalf("AprobacionVigenteDe: %v", err)
	}
	if hay && sol.Estado == fleet.AprobacionConcedida && sol.Usada.IsZero() {
		t.Error("la aprobación quedó SIN GASTAR después de abrir la sesión por el camino de `pide`: " +
			"el permiso es de un solo uso y sigue disponible para una segunda sesión. " +
			"Los dos caminos a `entregarPantalla` tienen que gastarla, no sólo el del default.")
	}
}

// TestSoloUnAdminApagaLosCuatroOjos — EL PRINCIPAL QUE LA GUARDA DEL INTERRUPTOR CLAVA.
//
// `TestApagarLosCuatroOjosExigeAdmin` prueba UN principal no-admin: `conPantalla`, que es
// `writer/own/own`. La compuerta pregunta por el ROL, pero `caps()` usa las capacidades
// DECLARADAS cuando existen, y `principals.yaml` acepta `write: any` con independencia del rol
// —lo valida sin mirarlo—. O sea que hay una forma de principal que el registro admite y que esa
// guarda nunca ejercita.
//
// MEDIDO EL 2026-09-22, con control: aflojando la compuerta a «admin O write=any» —el refactor
// plausible, porque `capsFromRole` le da `WriteAny` al admin y parece equivalente— el paquete
// entero queda en verde y un `writer` con escritura global apaga el control. Si quien entra puede
// apagar el control, no hay control.
//
// LA TABLA LLEVA EL ADMIN COMO CONTROL POSITIVO en la misma corrida: sin esa fila, una compuerta
// que rechazara a TODOS pasaría las tres de arriba y dejaría la tool inservible sin que nadie lo
// note.
//
// Sabotaje que la hace fallar: aflojar la compuerta del interruptor de «es admin» a «escribe en
// cualquier proyecto».
// arnes: archivo="internal/mcp/methods_aprobacion.go"
// arnes: de="\tif !p.isAdmin() {\n\t\treturn nil, rpcErrorf(codeUnauthorized,\n\t\t\t\"musubi_fleet_require_approval decide si esta máquina necesita una segunda persona"
// arnes: a="\tescribeEnTodo := false\n\tif p != nil {\n\t\tif _, w := p.caps(); w == WriteAny {\n\t\t\tescribeEnTodo = true\n\t\t}\n\t}\n\tif !p.isAdmin() && !escribeEnTodo {\n\t\treturn nil, rpcErrorf(codeUnauthorized,\n\t\t\t\"musubi_fleet_require_approval decide si esta máquina necesita una segunda persona"
// arnes: colision_ok="TestApagarLosCuatroOjosExigeAdmin"
func TestSoloUnAdminApagaLosCuatroOjos(t *testing.T) {
	formas := []struct {
		nombre string
		p      *Principal
		puede  bool
	}{
		{"writer/own — el único que la guarda vecina prueba", &Principal{
			Name: "mirador", Role: RoleWriter, Read: ReadOwn, Write: WriteOwn, ProjectID: "casa",
			Fleet: map[fleet.Cap][]string{fleet.CapScreen: {"*"}}}, false},
		{"writer con write=any — la forma que el registro admite y nadie probaba", &Principal{
			Name: "reparador", Role: RoleWriter, Read: ReadAll, Write: WriteAny,
			Fleet: map[fleet.Cap][]string{fleet.CapScreen: {"*"}}}, false},
		{"reader", &Principal{
			Name: "curiosa", Role: RoleReader, Read: ReadOwn, Write: WriteNone, ProjectID: "casa"}, false},
		{"admin — control positivo", &Principal{
			Name: "jefa", Role: RoleAdmin, ProjectID: "casa"}, true},
	}
	for _, f := range formas {
		t.Run(f.nombre, func(t *testing.T) {
			s := newTestServer(t, embedding.NoopProvider{})
			pantallaConCuatroOjos(t, s)
			_, e := callAsPrincipal(t, s, f.p, "musubi_fleet_require_approval", map[string]any{
				"device": "pc-gio", "project": "casa", "requerir": false,
			})
			if pudo := e == nil; pudo != f.puede {
				if pudo {
					t.Errorf("un principal %s/%s APAGÓ los cuatro ojos sin ser admin. "+
						"La compuerta tiene que preguntar por el ROL: si quien entra puede apagar el "+
						"control, no hay control.", f.p.Role, f.p.Write)
					return
				}
				t.Errorf("un principal %s/%s NO pudo apagar los cuatro ojos, y tenía que poder: la "+
					"compuerta se cerró de más y la tool quedó inservible", f.p.Role, f.p.Write)
			}
		})
	}
}
