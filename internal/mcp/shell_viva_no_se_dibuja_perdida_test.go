package mcp

import (
	"testing"
	"time"

	"musubi/internal/embedding"
	"musubi/internal/fleet"
)

// UNA SHELL VIVA Y LO QUE VIAJA DETRÁS DE ELLA NO SE DIBUJAN PERDIDOS, EN NINGUNA SUPERFICIE.
//
// El defecto VIVO que encontró la auditoría A131 (C4-LD1): el agente atiende `musubi:shell`
// BLOQUEANDO la sesión entera y la reporta recién al cerrarla, así que la fila del canal queda
// `entregado` lo que dure la sesión —hasta fleet.ShellVidaMax— y todo lo que viajó en la misma tanda
// espera detrás. La cota de `perdido` se había derivado sin la shell (102 minutos): pasado eso, la
// bitácora y la cronología dibujaban `perdido` la fila de una sesión que la misma cronología mostraba
// `activa` dos renglones más abajo, y el comando de atrás —vivo, esperando su turno— también. Es el
// error caro: manda a relanzar lo que va a correr igual.
//
// Se siembra por las puertas del motor lo que deja el camino real —la sesión, su comando de canal y
// un exec entregados en la misma tanda— con la sesión abierta hace casi su vida máxima y con tráfico
// de hace treinta segundos, o sea VIVA. Las guardas del dominio (internal/fleet/cota_de_perdido_test.go)
// miden la cuenta; ésta mide que lo que se ve no se contradiga.
//
// EXPOSICIÓN medida por la auditoría: una sola fila `musubi:shell` en la historia de producción, de
// un minuto, y ninguna sesión abierta hoy. Hace falta una shell de Tier A de más de 102 minutos, con
// tráfico, para verlo.
//
// Sabotaje: la cuenta de la cota sin la shell, la de la base → la fila del canal y el comando de atrás
// se dibujan perdidos al lado de la sesión activa.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="ShellVidaMax +\n"
// arnes: a="ComandoTimeoutMax + EsperaDeCierreDelAgente +\n"
// arnes: colision_ok="TestLaCotaDePerdidoEsLaPeorTandaRealNiMasNiMenos"
func TestUnaShellVivaYLoQueViajaDetrasNoSeDibujanPerdidos(t *testing.T) {
	s := newTestServer(t, embedding.NoopProvider{})
	if _, e := call(t, s, "musubi_fleet_enroll", map[string]interface{}{
		"name": "pc-gio", "tier": "A", "project": "infra",
		"caps": []string{"metrics", "exec", "shell"}, "os": "linux",
	}); e != nil {
		t.Fatalf("enroll: %+v", e)
	}
	d, existe, err := s.engine.DevicePorNombre("infra", "pc-gio")
	if err != nil || !existe {
		t.Fatalf("no quedó la máquina: %v %v", existe, err)
	}

	ahora := time.Now().UTC().Truncate(time.Second)
	// Abierta hace casi su vida máxima: le quedan cinco minutos.
	abierta := ahora.Add(-fleet.ShellVidaMax + 5*time.Minute)
	ses, err := s.engine.AbrirSesionShell(fleet.SesionShell{DeviceID: d.ID, ProjectID: "infra", Principal: "gio", Creada: abierta})
	if err != nil {
		t.Fatal(err)
	}
	// Alguien la está usando: un `top`, un `tail -f`.
	if err := s.engine.TocarSesionShell(ses.ID, ahora.Add(-30*time.Second)); err != nil {
		t.Fatal(err)
	}
	// Su comando de canal, tal como lo encola methods_shell.go, y un exec que viajó en la misma tanda.
	for _, c := range []fleet.Comando{
		{Argv: []string{fleet.OpShell, ses.ID, "24", "80"}, Timeout: fleet.ComandoTimeoutDefault, Creado: abierta},
		{Argv: []string{"systemctl", "restart", "MARCADETRAS"}, Timeout: 30 * time.Second, Creado: abierta.Add(5 * time.Second)},
	} {
		c.DeviceID, c.ProjectID, c.Principal, c.Origen = d.ID, "infra", "gio", fleet.OrigenPersona
		if _, err := s.engine.EncolarComando(c); err != nil {
			t.Fatal(err)
		}
	}
	tanda, err := s.engine.TomarComandos(d.ID, abierta.Add(20*time.Second), fleet.ComandosPorEntregaMax)
	if err != nil || len(tanda) != 2 {
		t.Fatalf("la tanda tenía que llevar la shell y el comando de atrás: %d (%v)", len(tanda), err)
	}
	// EL PISO: la sesión está VIVA y la tanda lleva entregada más de lo que la cota vieja concedía. Sin
	// esto la prueba mediría otra cosa: una sesión muerta, o una espera que ninguna cota discute.
	leida, ok, err := s.engine.SesionShellPorID(ses.ID)
	if err != nil || !ok || !leida.Viva(time.Now()) {
		t.Fatalf("la sesión sembrada no está viva (ok=%v err=%v estado=%q): la prueba no mide una shell en uso",
			ok, err, leida.Estado)
	}
	if lleva := ahora.Sub(abierta.Add(20 * time.Second)); lleva <= fleet.ComandosPorEntregaMax*fleet.ComandoTimeoutMax+fleet.MargenDeReporte {
		t.Fatalf("la tanda lleva %s entregada, y la cota sin la shell todavía la dejaba viva: la prueba no "+
			"cruza la franja del defecto", lleva)
	}

	p := conCaps("infra", map[fleet.Cap][]string{fleet.CapExec: {"*"}, fleet.CapShell: {"*"}})
	marcas := map[string]string{fleet.OpShell: "la fila del canal de la shell", "MARCADETRAS": "el comando que viajó detrás"}
	revisar := func(tool string, args map[string]any, filas string) {
		t.Helper()
		res, e := callAsPrincipal(t, s, p, tool, args)
		if e != nil {
			t.Fatalf("%s: %+v", tool, e)
		}
		lista, _ := jsonOf(t, res)[filas].([]any)
		vistas := 0
		for _, f := range lista {
			fila, _ := f.(map[string]any)
			argv, _ := fila["argv"].([]any)
			for _, a := range argv {
				quien, es := marcas[a.(string)]
				if !es {
					continue
				}
				vistas++
				if fila["estado"] != string(fleet.EstadoEntregado) {
					t.Errorf("%s muestra %q para %s, con la sesión viva y en uso: la shell sigue abierta y lo de "+
						"atrás espera su turno. Dibujarlo perdido manda a relanzar lo que va a correr igual "+
						"(EsperaMaxDeEntregado=%s, sesión abierta hace %s)",
						tool, fila["estado"], quien, fleet.EsperaMaxDeEntregado, ahora.Sub(abierta))
				}
			}
			if fila["tipo"] == string(fleet.HechoShell) && fila["estado"] != string(fleet.ShellActiva) &&
				fila["estado"] != string(fleet.ShellAbriendo) {
				t.Errorf("%s muestra la sesión %q: la prueba sembró una sesión viva", tool, fila["estado"])
			}
		}
		if vistas != len(marcas) {
			t.Errorf("%s devolvió %d de las %d filas de la tanda: la prueba no midió esa superficie", tool, vistas, len(marcas))
		}
	}
	revisar("musubi_fleet_log", map[string]any{"device": "pc-gio", "limite": 50}, "comandos")
	revisar("musubi_fleet_cronologia", map[string]any{"device": "pc-gio", "horas": 6, "limite": 100}, "hechos")
}
