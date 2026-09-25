package main

// Pruebas del modo root (decisión del dueño, 2026-09-24): el agente corre como root y el mundo del
// dueño —podman rootless y las units --user— se enumera con SU identidad. Todas con seams: ninguna
// lanza un proceso, así que corren igual en Windows, en macOS y en la CI de Linux.

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

// cuentasDePrueba es el passwd de musubi-server, reducido a lo que importa.
func cuentasDePrueba(nombre string) (cuentaDelSistema, error) {
	switch nombre {
	case "musubi":
		return cuentaDelSistema{Nombre: "musubi", Uid: 1000, Gid: 1000, Grupos: []uint32{1000, 10}, Home: "/home/musubi"}, nil
	case "otro":
		return cuentaDelSistema{Nombre: "otro", Uid: 1001, Gid: 1001, Grupos: []uint32{1001}, Home: "/home/otro"}, nil
	case "root":
		return cuentaDelSistema{Nombre: "root", Uid: 0, Gid: 0, Grupos: []uint32{0}, Home: "/root"}, nil
	}
	return cuentaDelSistema{}, errors.New("user: unknown user " + nombre)
}

func entornoDe(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

// LA TABLA DE LA DECISIÓN. Los casos de error son los que importan: en los tres, arrancar igual
// habría hecho que un agente enumere el mundo equivocado, y el cerebro —que poda por ausencia— daba
// de baja todo lo del dueño en el primer latido.
//
// Sabotaje que la hace fallar: tragarse el error del lookup de un usuario que no existe (sin él,
// «noexiste» cae al chequeo de uid 0 y el mensaje ya no dice que el usuario no existe).
// arnes: archivo="cmd/musubi/identidad_servicios.go"
// arnes: de="\tc, err := buscar(pedido)\n\tif err != nil {\n"
// arnes: a="\tc, err := buscar(pedido)\n\tif false {\n"
// Sabotaje que la hace fallar: aceptar que la variable nombre a root.
// arnes: archivo="cmd/musubi/identidad_servicios.go"
// arnes: de="\tif c.Uid == 0 {"
// arnes: a="\tif false && c.Uid == 0 {"
// Sabotaje que la hace fallar: que un agente sin privilegios acepte que la variable nombre a otro.
// arnes: archivo="cmd/musubi/identidad_servicios.go"
// arnes: de="if err != nil || int64(c.Uid) != int64(uid) {"
// arnes: a="if false && (err != nil || int64(c.Uid) != int64(uid)) {"
func TestResolverIdentidadDeServicios(t *testing.T) {
	deMusubi := map[string]string{"HOME": "/home/musubi", "USER": "musubi", "XDG_RUNTIME_DIR": "/run/user/1000"}
	deRoot := map[string]string{"HOME": "/root", "USER": "root"}
	con := func(m map[string]string, usuario string) map[string]string {
		out := map[string]string{envUsuarioDeServicios: usuario}
		for k, v := range m {
			out[k] = v
		}
		return out
	}

	musubiBaja := identidadDeServicios{Nombre: "musubi", Uid: 1000, Gid: 1000, Grupos: []uint32{1000, 10},
		Home: "/home/musubi", Runtime: "/run/user/1000", Bajar: true}
	musubiPropia := identidadDeServicios{Nombre: "musubi", Uid: 1000,
		Home: "/home/musubi", Runtime: "/run/user/1000"}
	rootPropia := identidadDeServicios{Nombre: "root", Home: "/root"}

	casos := []struct {
		nombre     string
		uid        int
		env        map[string]string
		quiero     identidadDeServicios
		errDiceQue string // "" = sin error
	}{
		{"el despliegue de siempre: musubi sin variable", 1000, deMusubi, musubiPropia, ""},
		{"musubi nombrándose a sí mismo", 1000, con(deMusubi, "musubi"), musubiPropia, ""},
		{"musubi nombrando a otro", 1000, con(deMusubi, "otro"), identidadDeServicios{}, "sin ser root"},
		{"root sin variable: su propio mundo", 0, deRoot, rootPropia, ""},
		{"root baja a musubi", 0, con(deRoot, "musubi"), musubiBaja, ""},
		{"root con un usuario que no existe", 0, con(deRoot, "noexiste"), identidadDeServicios{}, "no existe"},
		{"root nombrando a root", 0, con(deRoot, "root"), identidadDeServicios{}, "es root"},
		{"un sistema sin uid con la variable", -1, con(nil, "musubi"), identidadDeServicios{}, "no hay uid"},
		{"un sistema sin uid sin la variable", -1, map[string]string{"USER": "gio"}, identidadDeServicios{Nombre: "gio"}, ""},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got, err := resolverIdentidadDeServicios(c.uid, entornoDe(c.env), cuentasDePrueba)
			if c.errDiceQue != "" {
				if err == nil {
					t.Fatalf("se esperaba un error que dijera %q y resolvió %+v: el agente arrancaría "+
						"enumerando el mundo equivocado", c.errDiceQue, got)
				}
				if !strings.Contains(err.Error(), c.errDiceQue) {
					t.Errorf("el error no dice %q, que es lo que la persona tiene que arreglar: %v", c.errDiceQue, err)
				}
				if !strings.Contains(err.Error(), envUsuarioDeServicios) {
					t.Errorf("el error no nombra la variable: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("error inesperado: %v", err)
			}
			if !reflect.DeepEqual(got, c.quiero) {
				t.Errorf("identidad:\n  obtuve  %+v\n  quería  %+v", got, c.quiero)
			}
		})
	}
}

// EL HIJO QUE BAJA NO LLEVA NADA DE ROOT. Podman rootless decide su store con HOME y
// XDG_CONFIG_HOME, y sus locks con XDG_RUNTIME_DIR: con uid 1000 y HOME=/root no puede ni leer su
// configuración. Y las credenciales del agente no viajan a un proceso de otro usuario.
//
// Sabotaje que la hace fallar: no ponerle al hijo el HOME del dueño.
// arnes: archivo="cmd/musubi/identidad_servicios.go"
// arnes: de="\tout = append(out, \"HOME=\"+id.Home)\n"
// arnes: a=""
// Sabotaje que la hace fallar: poner XDG_RUNTIME_DIR aunque el directorio no exista o sea ajeno.
// arnes: archivo="cmd/musubi/identidad_servicios.go"
// arnes: de=" || !esDe(id.Runtime, id.Uid)"
// arnes: a=""
// Sabotaje que la hace fallar: no sacar del entorno base lo que es de root.
// arnes: archivo="cmd/musubi/identidad_servicios.go"
// arnes: de="\t\tif fuera(clave) {"
// arnes: a="\t\tif false && fuera(clave) {"
func TestElHijoQueBajaLlevaElEntornoDelDuenoYNoElDeRoot(t *testing.T) {
	base := []string{
		"HOME=/root", "USER=root", "LOGNAME=root", "SHELL=/bin/bash",
		"XDG_CONFIG_HOME=/root/.config", "XDG_RUNTIME_DIR=/run/user/0",
		"DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/0/bus",
		"MUSUBI_DEVICE_TOKEN_FILE=/etc/musubi-agente/token", "MUSUBI_DEVICE_TOKEN=secreto",
		"PATH=/usr/local/bin:/usr/bin", "LANG=es_AR.UTF-8",
	}
	id := identidadDeServicios{Nombre: "musubi", Uid: 1000, Gid: 1000, Home: "/home/musubi",
		Runtime: "/run/user/1000", Bajar: true}

	var preguntado struct {
		dir string
		uid uint32
	}
	esDe := func(dir string, uid uint32) bool {
		preguntado.dir, preguntado.uid = dir, uid
		return true
	}
	env := entornoPara(id, base, esDe)
	cuenta := func(prefijo string) (n int, valor string) {
		for _, kv := range env {
			if strings.HasPrefix(kv, prefijo) {
				n++
				valor = strings.TrimPrefix(kv, prefijo)
			}
		}
		return n, valor
	}
	for _, c := range []struct{ clave, quiero string }{
		{"HOME=", "/home/musubi"},
		{"USER=", "musubi"},
		{"LOGNAME=", "musubi"},
		{"XDG_RUNTIME_DIR=", "/run/user/1000"},
		{"PATH=", "/usr/local/bin:/usr/bin"},
		{"LANG=", "es_AR.UTF-8"},
	} {
		if n, v := cuenta(c.clave); n != 1 || v != c.quiero {
			t.Errorf("%s aparece %d vez/veces con %q; tiene que estar UNA vez con %q:\n  %v",
				strings.TrimSuffix(c.clave, "="), n, v, c.quiero, env)
		}
	}
	for _, prohibida := range []string{"XDG_CONFIG_HOME=", "DBUS_SESSION_BUS_ADDRESS=",
		"MUSUBI_DEVICE_TOKEN_FILE=", "MUSUBI_DEVICE_TOKEN="} {
		if n, _ := cuenta(prohibida); n != 0 {
			t.Errorf("el hijo que baja hereda %s de root:\n  %v", strings.TrimSuffix(prohibida, "="), env)
		}
	}

	// Un runtime que no existe —o no es suyo— NO se pone: podman rootless falla con una ruta que
	// no existe, y eso abortaría el inventario entero. VA ANTES que la pregunta de abajo, que es
	// un proxy: sin el chequeo también quedaría sin preguntar, pero lo que importa es el entorno.
	sinRuntime := entornoPara(id, base, func(string, uint32) bool { return false })
	for _, kv := range sinRuntime {
		if strings.HasPrefix(kv, "XDG_RUNTIME_DIR=") {
			t.Errorf("se puso %s aunque el directorio no es del dueño", kv)
		}
	}
	if preguntado.dir != "/run/user/1000" || preguntado.uid != 1000 {
		t.Errorf("se preguntó si %q es de %d; había que preguntar por el runtime del dueño y su uid",
			preguntado.dir, preguntado.uid)
	}
}

// EL QUE NO BAJA PASA POR LA PUERTA DE SIEMPRE. Todas las pruebas de parseo apuntan
// ejecutarParaEnumerar a un doble; si el camino sin bajar la saltara, en cada máquina de la flota
// menos una el enumerador correría el comando de verdad por un costado que ninguna prueba mira.
//
// Sabotaje que la hace fallar: que el camino sin bajar no delegue en ejecutarParaEnumerar.
// arnes: archivo="cmd/musubi/servicios_exec.go"
// arnes: de="\tif !id.Bajar {\n\t\treturn ejecutarParaEnumerar(nombre, args...)\n\t}"
// arnes: a="\tif false && !id.Bajar {\n\t\treturn ejecutarParaEnumerar(nombre, args...)\n\t}"
func TestSinBajarElEnumeradorPasaPorLaPuertaDeSiempre(t *testing.T) {
	original := ejecutarParaEnumerar
	t.Cleanup(func() { ejecutarParaEnumerar = original })
	var pedido string
	ejecutarParaEnumerar = func(nombre string, args ...string) ([]byte, error) {
		pedido = nombre + " " + strings.Join(args, " ")
		return []byte("ok"), nil
	}
	b, err := ejecutarComoParaEnumerar(identidadDeServicios{}, "musubi-herramienta-que-no-existe", "ps")
	if err != nil || string(b) != "ok" {
		t.Fatalf("sin bajar no se usó la puerta de siempre: salida %q, error %v", b, err)
	}
	if pedido != "musubi-herramienta-que-no-existe ps" {
		t.Errorf("la puerta de siempre recibió %q", pedido)
	}
}

// COMO ROOT, PODMAN SE PREGUNTA COMO EL DUEÑO Y DOCKER COMO ROOT. Podman rootless guarda su store
// en el home del dueño: preguntado como root lee el store rootful, vacío, y el cerebro da de baja
// los 18 contenedores de musubi-server. Docker es un demonio de root, y ahí root es quien pregunta.
//
// Sabotaje que la hace fallar: consultar podman sin la identidad del dueño.
// arnes: archivo="cmd/musubi/servicios_contenedores.go"
// arnes: de="\t\tif cli == \"podman\" {"
// arnes: a="\t\tif false {"
func TestComoRootLosContenedoresSeEnumeranConLaIdentidadDelDueno(t *testing.T) {
	id := identidadDeServicios{Nombre: "musubi", Uid: 1000, Gid: 1000, Home: "/home/musubi",
		Runtime: "/run/user/1000", Bajar: true}
	originalId, originalExec := identidadParaEnumerar, ejecutarComoParaEnumerar
	t.Cleanup(func() { identidadParaEnumerar, ejecutarComoParaEnumerar = originalId, originalExec })
	identidadParaEnumerar = id

	vistas := map[string]identidadDeServicios{}
	ejecutarComoParaEnumerar = func(quien identidadDeServicios, nombre string, args ...string) ([]byte, error) {
		vistas[nombre] = quien
		if nombre == "podman" {
			return []byte("supabase-db\trunning\tUp 3 days\t0\n"), nil
		}
		return []byte("nginx\trunning\tUp 1 day\t0\n"), nil
	}

	rs, err := enumerarContenedores(time.Now())
	if err != nil {
		t.Fatalf("enumerarContenedores: %v", err)
	}
	if len(rs) != 2 {
		t.Fatalf("se esperaban los dos contenedores, hubo %d: %+v", len(rs), rs)
	}
	if got := vistas["podman"]; !got.Bajar || got.Uid != 1000 || got.Home != "/home/musubi" {
		t.Errorf("podman se preguntó como %+v: como root lee el store rootful, vacío, y el cerebro "+
			"poda los contenedores del dueño", got)
	}
	if got := vistas["docker"]; got.Bajar {
		t.Errorf("docker se preguntó bajando a %s: su demonio es de root y su socket también", got.Nombre)
	}
}

// LA PRECONDICIÓN DEL DESPLIEGUE ES UN GREP: `musubi agent --help | grep -c MUSUBI_AGENTE_USUARIO`
// tiene que dar 1 antes de poner el drop-in de root. Un binario viejo da 0 y enumeraría como root.
//
// Sabotaje que la hace fallar: sacar la variable de la ayuda.
// arnes: archivo="cmd/musubi/agent.go"
// arnes: de="cBold(envUsuarioDeServicios))"
// arnes: a="cBold(\"\"))"
func TestLaAyudaNombraLaVariableDelUsuarioUnaSolaVez(t *testing.T) {
	salida := capturarSalida(t, ayudaAgent)
	n := 0
	for _, l := range strings.Split(salida, "\n") {
		if strings.Contains(l, envUsuarioDeServicios) {
			n++
		}
	}
	if n != 1 {
		t.Errorf("la ayuda nombra %s en %d línea(s) y el despliegue verifica con `grep -c` que sea 1:\n%s",
			envUsuarioDeServicios, n, salida)
	}
}
