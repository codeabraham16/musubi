package main

// Pruebas de lo que el blindaje y el arranque tienen que saber del modo root: el token vive en
// /etc/musubi-agente y el mundo que se enumera es el del dueño, no el de /root.

import (
	"strings"
	"testing"
)

// EL VERIFICADOR MIRA DÓNDE ESTÁ EL TOKEN Y DE QUIÉN ES EL STORE, NO EL HOME DEL PROCESO.
//
// Con el agente como root, os.UserHomeDir() es /root: el verificador declaraba
// /root/.config/musubi-agente (que no toca nadie) y no miraba ni /etc/musubi-agente —donde la
// rotación reescribe el token— ni el store de podman del dueño, que es lo que rompió A42.
//
// Sabotaje que la hace fallar: sacar el directorio del token del HOME en vez de la variable.
// arnes: archivo="cmd/musubi/blindaje.go"
// arnes: de="\t\treturn path.Dir(filepath.ToSlash(r))"
// arnes: a="\t\treturn path.Join(filepath.ToSlash(home), \".config\", \"musubi-agente\")"
// Sabotaje que la hace fallar: declarar el runtime del proceso aunque el agente baje al dueño.
// arnes: archivo="cmd/musubi/blindaje.go"
// arnes: de="\tif id.Bajar {\n\t\treturn id.Home, id.Runtime\n\t}"
// arnes: a="\tif false && id.Bajar {\n\t\treturn id.Home, id.Runtime\n\t}"
func TestElBlindajeMiraElDirectorioDelTokenYNoElHomeDeRoot(t *testing.T) {
	t.Setenv("HOME", "/root")
	id := identidadDeServicios{Nombre: "musubi", Uid: 1000, Gid: 1000, Home: "/home/musubi",
		Runtime: "/run/user/1000", Bajar: true}
	// El runtime PROPIO de un agente root es "" (dirDeRuntime con uid 0): no hay store rootless.
	home, runtimeDir := rutasDelBlindaje(id, func() string { return "" })
	ns := necesidadesDelAgente(dirDelToken("/etc/musubi-agente/token", home), home, runtimeDir, conPodman)

	quiero := map[string]string{
		"/etc/musubi-agente":                   "latido",
		"/home/musubi/.local/share/containers": "inventario de contenedores",
		"/run/user/1000/containers":            "inventario de contenedores",
		"/run/user/1000/libpod":                "inventario de contenedores",
	}
	vistas := map[string]bool{}
	for _, n := range ns {
		if strings.HasPrefix(n.Ruta, "/root") {
			t.Errorf("declaró %q (%s): es el home del PROCESO root, y el agente no toca nada ahí", n.Ruta, n.Trabajo)
		}
		if trabajo, ok := quiero[n.Ruta]; ok && trabajo == n.Trabajo {
			vistas[n.Ruta] = true
		}
	}
	for ruta, trabajo := range quiero {
		if !vistas[ruta] {
			t.Errorf("no declaró %q para %q: el verificador sale en verde sin mirar lo que el agente root toca", ruta, trabajo)
		}
	}

	// Sin la variable —el token por MUSUBI_DEVICE_TOKEN, o el verificador corrido sin el entorno
	// de la unidad— cae al lugar de siempre, bajo el home de la IDENTIDAD.
	if d := dirDelToken("", "/home/musubi"); d != "/home/musubi/.config/musubi-agente" {
		t.Errorf("sin variable, el directorio del token es %q", d)
	}
	// Y sin bajar, el runtime es el que resuelve podman para el propio proceso.
	propia := identidadDeServicios{Nombre: "musubi", Uid: 1000, Home: "/home/musubi"}
	if _, rt := rutasDelBlindaje(propia, func() string { return "/run/user/1000" }); rt != "/run/user/1000" {
		t.Errorf("sin bajar, el runtime del blindaje es %q y tenía que ser el del proceso", rt)
	}
}

// EL AGENTE LE DA SU RUNTIME A LOS HIJOS SÓLO SI FALTA, SI NO ES ROOT Y SI EL DIRECTORIO ES SUYO.
// Sin XDG_RUNTIME_DIR, `systemctl --user` no encuentra el bus; con uno que no existe, podman
// rootless falla y aborta el inventario entero.
//
// Sabotaje que la hace fallar: exportarla sin mirar si el directorio existe y es del agente.
// arnes: archivo="cmd/musubi/blindaje.go"
// arnes: de="\tif !esMio(d) {"
// arnes: a="\tif false && !esMio(d) {"
// Sabotaje que la hace fallar: exportarla también con uid 0.
// arnes: archivo="cmd/musubi/blindaje.go"
// arnes: de="!= \"\" || uid <= 0 {"
// arnes: a="!= \"\" || uid < 0 {"
func TestElAgenteLeDaSuRuntimeALosHijosSoloSiExisteYEsSuyo(t *testing.T) {
	suyo := func(d string) bool { return true }
	ajeno := func(d string) bool { return false }
	sinVariable := entornoDe(map[string]string{})
	conVariable := entornoDe(map[string]string{"XDG_RUNTIME_DIR": "/run/user/1000"})

	casos := []struct {
		nombre string
		getenv func(string) string
		uid    int
		esMio  func(string) bool
		quiero string
	}{
		{"falta, uid 1000, es suyo → se exporta", sinVariable, 1000, suyo, "/run/user/1000"},
		{"ya está puesta → no se pisa", conVariable, 1000, suyo, ""},
		{"root → no (lo resuelve la identidad)", sinVariable, 0, suyo, ""},
		{"sin uid (Windows) → no", sinVariable, -1, suyo, ""},
		{"no existe o es ajeno → no", sinVariable, 1000, ajeno, ""},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			d, ok := runtimeParaHeredar(c.getenv, c.uid, c.esMio)
			if ok != (c.quiero != "") || d != c.quiero {
				t.Errorf("runtimeParaHeredar = (%q, %v); quería %q", d, ok, c.quiero)
			}
		})
	}
}
