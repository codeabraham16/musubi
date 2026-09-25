package main

// Pruebas de la fuente `systemctl --user` (punto 38a). Fixture armado con lo medido en
// musubi-server el 2026-09-24: los puentes y el gateway escritos a mano, `core01-ensayo-local`
// FAILED, el quadlet de Vaultwarden con UnitFileState=generated, y las units de /usr/lib que el
// manager de usuario carga igual.

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"musubi/internal/fleet"
)

func bloqueSystemd(campos map[string]string) string {
	claves := make([]string, 0, len(campos))
	for k := range campos {
		claves = append(claves, k)
	}
	sort.Strings(claves)
	var b strings.Builder
	for _, k := range claves {
		b.WriteString(k + "=" + campos[k] + "\n")
	}
	return b.String()
}

func unitDeUsuario(id, activo, sub, archivo, fragmento, fuente string) string {
	return bloqueSystemd(map[string]string{
		"Id": id, "ActiveState": activo, "SubState": sub, "UnitFileState": archivo,
		"FragmentPath": fragmento, "SourcePath": fuente, "MainPID": "0", "NRestarts": "0",
		"Result": map[bool]string{true: "exit-code", false: "success"}[activo == "failed"],
	})
}

const propias = "/home/musubi/.config/systemd/user/"

// salidaDeUsuarioMedida es un `systemctl --user show '*.service'` de musubi-server, recortado.
var salidaDeUsuarioMedida = strings.Join([]string{
	unitDeUsuario("altura-wa-bridge.service", "active", "running", "enabled", propias+"altura-wa-bridge.service", ""),
	unitDeUsuario("forgejo.service", "active", "running", "enabled", propias+"forgejo.service", ""),
	unitDeUsuario("core01-ensayo-local.service", "failed", "failed", "static", propias+"core01-ensayo-local.service", ""),
	// Un oneshot de timer que terminó bien: static e inactivo. NO entra (entra el día que falle).
	unitDeUsuario("b1-extract.service", "inactive", "dead", "static", propias+"b1-extract.service", ""),
	// Escrita a mano y deshabilitada: ruido, igual que del lado del sistema.
	unitDeUsuario("vieja.service", "inactive", "dead", "disabled", propias+"vieja.service", ""),
	// El quadlet: FragmentPath en el generador, SourcePath en el home, UnitFileState=generated.
	unitDeUsuario("vaultwarden.service", "active", "running", "generated",
		"/run/user/1000/systemd/generator/vaultwarden.service", "/home/musubi/.config/containers/systemd/vaultwarden.container"),
	// Lo que trae el sistema al manager de usuario: afuera, aunque esté habilitado.
	unitDeUsuario("dbus-broker.service", "active", "running", "enabled", "/usr/lib/systemd/user/dbus-broker.service", ""),
	unitDeUsuario("podman-restart.service", "inactive", "dead", "enabled", "/usr/lib/systemd/user/podman-restart.service", ""),
	unitDeUsuario("systemd-tmpfiles-setup.service", "active", "exited", "enabled", "/usr/lib/systemd/user/systemd-tmpfiles-setup.service", ""),
	// Un home que EMPIEZA igual y es de otro: el prefijo tiene que ser el directorio entero.
	unitDeUsuario("ajena.service", "active", "running", "enabled", "/home/musubix/.config/systemd/user/ajena.service", ""),
}, "\n")

func nombresDe(rs []fleet.ReporteServicio) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Nombre)
	}
	sort.Strings(out)
	return out
}

// EL PREFIJO ES LO QUE EVITA QUE UNA UNIT DE USUARIO SE COMA A UNA DE SISTEMA (o al revés).
// serviciosParaElLatido deduplica por nombre sin distinguir mayúsculas; `podman-restart` existe de
// verdad en los dos managers de musubi-server. Sin el prefijo, una de las dos se descarta en
// silencio y la poda del cerebro la da de baja.
//
// Sabotaje que la hace fallar: no ponerle el prefijo al nombre de la unit de usuario.
// arnes: archivo="cmd/musubi/servicios_parsers.go"
// arnes: de="\t\t\tnombre = prefijoUnitDeUsuario + nombre\n"
// arnes: a="\t\t\tnombre = \"\" + nombre\n"
func TestLasUnitsDeUsuarioEntranConPrefijoYNoPisanALasDelSistema(t *testing.T) {
	ahora := time.Now()
	sistema := parsearSystemctlShow(bloqueSystemd(map[string]string{
		"Id": "forgejo.service", "ActiveState": "active", "SubState": "running", "UnitFileState": "enabled",
		"FragmentPath": "/etc/systemd/system/forgejo.service",
	}), ahora)
	usuario := parsearSystemctlShowDeUsuario(salidaDeUsuarioMedida, ahora, "/home/musubi")

	lista, afuera := serviciosParaElLatido(append(sistema, usuario...))
	if afuera != 0 {
		t.Fatalf("quedaron %d afuera en un inventario de %d", afuera, len(lista))
	}
	hay := map[string]fleet.ReporteServicio{}
	for _, r := range lista {
		hay[r.Nombre] = r
	}
	if _, ok := hay["forgejo"]; !ok {
		t.Errorf("la unit de SISTEMA forgejo desapareció: %v", nombresDe(lista))
	}
	u, ok := hay[prefijoUnitDeUsuario+"forgejo"]
	if !ok {
		t.Fatalf("la unit de USUARIO forgejo no llegó al latido con su prefijo: %v", nombresDe(lista))
	}
	if u.Clase != "systemd" {
		t.Errorf("la clase de una unit de usuario es %q; tiene que ser systemd, que ya está en el enum", u.Clase)
	}
}

// ENTRA SÓLO LO QUE ESCRIBIÓ EL DUEÑO, Y LOS QUADLETS SON LO QUE ESCRIBIÓ.
//
// Sabotaje que la hace fallar: no filtrar por dónde vive la unit (entran las de /usr/lib).
// arnes: archivo="cmd/musubi/servicios_parsers.go"
// arnes: de="\t\tif deUsuario && !escritaPorElUsuario(p, home) {"
// arnes: a="\t\tif false && deUsuario && !escritaPorElUsuario(p, home) {"
// Sabotaje que la hace fallar: filtrar sólo por FragmentPath, que deja afuera al quadlet.
// arnes: archivo="cmd/musubi/servicios_parsers.go"
// arnes: de="strings.HasPrefix(p[\"SourcePath\"], home+\"/.config/containers/systemd/\")"
// arnes: a="false && strings.HasPrefix(p[\"SourcePath\"], home+\"/.config/containers/systemd/\")"
// Sabotaje que la hace fallar: no contar `generated` como habilitada del lado del usuario.
// arnes: archivo="cmd/musubi/servicios_parsers.go"
// arnes: de="(deUsuario && p[\"UnitFileState\"] == \"generated\")"
// arnes: a="(false && deUsuario && p[\"UnitFileState\"] == \"generated\")"
// arnes: colision_ok="TestSoloEntranLasUnitsQueEscribioElUsuario"
// Sabotaje que la hace fallar: contar `generated` como habilitada también del lado del SISTEMA.
// Es la misma línea que el de arriba mirada desde el otro lado: aquél pierde el quadlet del
// usuario, éste mete `rc-local` en el inventario del sistema. Dos motivos, los dos corridos.
// arnes: archivo="cmd/musubi/servicios_parsers.go"
// arnes: de="(deUsuario && p[\"UnitFileState\"] == \"generated\")"
// arnes: a="(p[\"UnitFileState\"] == \"generated\")"
// arnes: colision_ok="TestSoloEntranLasUnitsQueEscribioElUsuario"
func TestSoloEntranLasUnitsQueEscribioElUsuario(t *testing.T) {
	ahora := time.Now()
	for _, home := range []string{"/home/musubi", "/home/musubi/"} {
		rs := parsearSystemctlShowDeUsuario(salidaDeUsuarioMedida, ahora, home)
		quiero := []string{
			prefijoUnitDeUsuario + "altura-wa-bridge",
			prefijoUnitDeUsuario + "core01-ensayo-local",
			prefijoUnitDeUsuario + "forgejo",
			prefijoUnitDeUsuario + "vaultwarden",
		}
		if got := nombresDe(rs); strings.Join(got, ",") != strings.Join(quiero, ",") {
			t.Errorf("home %q:\n  entraron %v\n  quería   %v", home, got, quiero)
		}
		for _, r := range rs {
			if r.Nombre == prefijoUnitDeUsuario+"core01-ensayo-local" && r.Salud.Estado != fleet.EstadoFallado {
				t.Errorf("core01-ensayo-local está FAILED y se reporta %q: es exactamente lo que nadie veía", r.Salud.Estado)
			}
		}
	}

	// Sin home no se sabe qué es del dueño: no entra nada, en vez de entrar todo.
	if rs := parsearSystemctlShowDeUsuario(salidaDeUsuarioMedida, ahora, ""); len(rs) != 0 {
		t.Errorf("sin home entraron %v", nombresDe(rs))
	}

	// Y del lado del SISTEMA nada cambia: `generated` sigue sin contar (montajes, scripts SysV), y
	// FragmentPath no filtra nada.
	sistema := parsearSystemctlShow(strings.Join([]string{
		bloqueSystemd(map[string]string{"Id": "rc-local.service", "ActiveState": "active", "SubState": "exited",
			"UnitFileState": "generated", "FragmentPath": "/run/systemd/generator/rc-local.service"}),
		bloqueSystemd(map[string]string{"Id": "sshd.service", "ActiveState": "active", "SubState": "running",
			"UnitFileState": "enabled", "FragmentPath": "/usr/lib/systemd/system/sshd.service"}),
	}, "\n"), ahora)
	if got := nombresDe(sistema); strings.Join(got, ",") != "sshd" {
		t.Errorf("del lado del sistema entraron %v; tenía que entrar sólo sshd", got)
	}
}

// «RUNTIME SIN BUS» NO ES «FUENTE AUSENTE». Sin runtime, la fuente no está y el inventario sigue
// completo. Con runtime y sin socket —el agente llegó antes que user@1000— hay que abortar: si se
// tomara como ausente, el cerebro podaría las usuario:* y volverían en el latido siguiente.
//
// Sabotaje que la hace fallar: que una máquina sin manager de usuario le pregunte igual.
// arnes: archivo="cmd/musubi/servicios_usuario.go"
// arnes: de="\tcase runtimeAusente:\n\t\treturn nil, nil\n"
// arnes: a="\tcase runtimeAusente:\n"
// Sabotaje que la hace fallar: tomar el runtime sin bus como fuente ausente.
// arnes: archivo="cmd/musubi/servicios_usuario.go"
// arnes: de="\tcase busAusente:\n\t\treturn nil, fmt.Errorf("
// arnes: a="\tcase busAusente:\n\t\t_ = fmt.Errorf("
// Sabotaje que la hace fallar: tragarse el error de systemctl --user (inventario parcial).
// arnes: archivo="cmd/musubi/servicios_usuario.go"
// arnes: de="\t\treturn nil, fmt.Errorf(\"units --user de %s: %w\", id.Nombre, err)\n"
// arnes: a="\t\treturn nil, nil\n"
func TestSinBusDeUsuarioElInventarioNoSeAborta(t *testing.T) {
	id := identidadDeServicios{Nombre: "musubi", Uid: 1000, Gid: 1000, Home: "/home/musubi",
		Runtime: "/run/user/1000", Bajar: true}
	original := ejecutarComoParaEnumerar
	t.Cleanup(func() { ejecutarComoParaEnumerar = original })
	llamadas := 0
	falla := false
	ejecutarComoParaEnumerar = func(identidadDeServicios, string, ...string) ([]byte, error) {
		llamadas++
		if falla {
			return nil, errors.New("Failed to connect to user scope bus")
		}
		return []byte(salidaDeUsuarioMedida), nil
	}
	fijo := func(e estadoDelBus) func(string) estadoDelBus { return func(string) estadoDelBus { return e } }

	t.Run("sin runtime: la fuente no está", func(t *testing.T) {
		llamadas, falla = 0, true
		rs, err := enumerarUnitsDeUsuario(id, fijo(runtimeAusente), time.Now())
		if err != nil || rs != nil {
			t.Errorf("sin runtime devolvió %v, %v; tenía que ser nil, nil: una máquina sin manager de "+
				"usuario abortaría el inventario entero", nombresDe(rs), err)
		}
		if llamadas != 0 {
			t.Errorf("sin runtime se le preguntó igual a systemctl --user (%d vez/veces)", llamadas)
		}
	})
	t.Run("runtime sin bus: se aborta", func(t *testing.T) {
		llamadas, falla = 0, false
		rs, err := enumerarUnitsDeUsuario(id, fijo(busAusente), time.Now())
		if err == nil {
			t.Errorf("runtime sin bus devolvió %v sin error: tomarlo como ausente hace que el cerebro "+
				"pode las usuario:* en el arranque", nombresDe(rs))
		}
		if llamadas != 0 {
			t.Errorf("sin bus se le preguntó igual a systemctl --user (%d vez/veces)", llamadas)
		}
	})
	t.Run("bus listo y systemctl falla: completo o nada", func(t *testing.T) {
		llamadas, falla = 0, true
		if _, err := enumerarUnitsDeUsuario(id, fijo(busListo), time.Now()); err == nil {
			t.Error("systemctl --user falló y no hubo error: el inventario viajaría sin las units del " +
				"dueño y la poda las daría de baja")
		}
	})
	t.Run("bus listo: anda", func(t *testing.T) {
		llamadas, falla = 0, false
		rs, err := enumerarUnitsDeUsuario(id, fijo(busListo), time.Now())
		if err != nil || len(rs) != 4 {
			t.Errorf("con el bus listo devolvió %v, %v", nombresDe(rs), err)
		}
	})
}

// EL BUS SE MIRA EN EL DISCO, Y LOS TRES ESTADOS SALEN DE DOS PREGUNTAS.
//
// Sabotaje que la hace fallar: que un runtime que existe sin socket se lea como runtime ausente.
// arnes: archivo="cmd/musubi/servicios_usuario.go"
// arnes: de="\treturn busAusente\n}"
// arnes: a="\treturn runtimeAusente\n}"
func TestElEstadoDelBusSeLeeDelRuntime(t *testing.T) {
	dir := t.TempDir()
	if got := estadoDelBusDeUsuario(""); got != runtimeAusente {
		t.Errorf("runtime vacío → %v", got)
	}
	if got := estadoDelBusDeUsuario(filepath.Join(dir, "no-existe")); got != runtimeAusente {
		t.Errorf("runtime inexistente → %v", got)
	}
	if got := estadoDelBusDeUsuario(dir); got != busAusente {
		t.Errorf("runtime sin socket → %v; es el arranque antes que user@<uid>, y tiene que abortar", got)
	}
	if err := os.MkdirAll(filepath.Join(dir, "systemd"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "systemd", "private"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := estadoDelBusDeUsuario(dir); got != busListo {
		t.Errorf("runtime con systemd/private → %v", got)
	}
}

// LAS UNITS DEL DUEÑO SE PIDEN CON SU IDENTIDAD Y SE FILTRAN CON SU HOME. Es la reconciliación
// con el modo root: como root, el HOME del proceso es /root, y filtrar con él dejaba la fuente
// vacía EN SILENCIO — ninguna unit de /root/.config/systemd/user existe.
//
// Sabotaje que la hace fallar: filtrar con el HOME del proceso en vez del de la identidad.
// arnes: archivo="cmd/musubi/servicios_usuario.go"
// arnes: de="parsearSystemctlShowDeUsuario(salida, ahora, id.Home)"
// arnes: a="parsearSystemctlShowDeUsuario(salida, ahora, os.Getenv(\"HOME\"))"
// Sabotaje que la hace fallar: preguntarle a systemctl --user sin la identidad del dueño.
// arnes: archivo="cmd/musubi/servicios_usuario.go"
// arnes: de="enumerarFuenteComo(id, \"systemctl\", args...)"
// arnes: a="enumerarFuenteComo(identidadDeServicios{}, \"systemctl\", args...)"
func TestLasUnitsDeUsuarioSeEnumeranConLaIdentidadDelDueno(t *testing.T) {
	t.Setenv("HOME", "/root")
	id := identidadDeServicios{Nombre: "musubi", Uid: 1000, Gid: 1000, Home: "/home/musubi",
		Runtime: "/run/user/1000", Bajar: true}
	original := ejecutarComoParaEnumerar
	t.Cleanup(func() { ejecutarComoParaEnumerar = original })
	var quien identidadDeServicios
	var argv []string
	ejecutarComoParaEnumerar = func(q identidadDeServicios, nombre string, args ...string) ([]byte, error) {
		quien, argv = q, append([]string{nombre}, args...)
		return []byte(salidaDeUsuarioMedida), nil
	}

	rs, err := enumerarUnitsDeUsuario(id, func(string) estadoDelBus { return busListo }, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if len(argv) < 2 || argv[0] != "systemctl" || argv[1] != "--user" {
		t.Fatalf("no se le preguntó al manager de usuario: %v", argv)
	}
	if !quien.Bajar || quien.Uid != 1000 {
		t.Errorf("systemctl --user corrió como %+v: como root mira el manager de root, no el del dueño", quien)
	}
	if !strings.Contains(strings.Join(nombresDe(rs), ","), prefijoUnitDeUsuario+"altura-wa-bridge") {
		t.Errorf("con HOME=/root en el proceso, el filtro dejó afuera las units del dueño: %v", nombresDe(rs))
	}
	var pedidas string
	for _, a := range argv {
		if strings.HasPrefix(a, "--property=") {
			pedidas = a
		}
	}
	for _, p := range []string{"FragmentPath", "SourcePath", "UnitFileState"} {
		if !strings.Contains(pedidas, p) {
			t.Errorf("no se le pide %s a systemctl --user, y el filtro lo necesita: %q", p, pedidas)
		}
	}
}
