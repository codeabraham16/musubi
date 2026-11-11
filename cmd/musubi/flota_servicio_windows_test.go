//go:build windows

package main

import (
	"testing"

	"golang.org/x/sys/windows/svc"
)

// describirEstadoSvc es la parte PURA del subsistema de servicio: traduce el estado del SCM a la
// palabra que `musubi flota estado` le muestra al operador. Esta tabla fija cada estado contra su
// palabra, así que un swap de dos ramas del switch —el error más plausible— la pone en rojo.
//
// El resto del archivo (instalar/quitar/estado) toca el SCM real y NO se puede probar sin elevación
// ni un servicio instalado; queda cubierto por la revisión y la prueba manual, y así se declara en
// vez de fingir cobertura.
func TestDescribirEstadoSvc(t *testing.T) {
	casos := []struct {
		estado   svc.State
		esperado string
	}{
		{svc.Stopped, "detenido"},
		{svc.StartPending, "arrancando"},
		{svc.StopPending, "deteniéndose"},
		{svc.Running, "en marcha"},
		{svc.PausePending, "pausándose"},
		{svc.Paused, "pausado"},
		{svc.ContinuePending, "reanudándose"},
	}
	for _, c := range casos {
		if got := describirEstadoSvc(c.estado); got != c.esperado {
			t.Errorf("describirEstadoSvc(%d) = %q; esperaba %q", c.estado, got, c.esperado)
		}
	}

	// Un estado fuera del enum conocido no entra en pánico ni devuelve vacío: cae al default con el
	// número, que es lo que un operador necesita para googlear un estado raro.
	const desconocido = svc.State(99)
	if got := describirEstadoSvc(desconocido); got == "" {
		t.Errorf("un estado desconocido devolvió cadena vacía; debería describir el número")
	}
}

// rutaDentroDe es la guarda anti-EoP de `instalar`: decide si el binario que se va a registrar como
// servicio LocalSystem cuelga de una carpeta protegida. Si se aflojara, un .exe en una ruta
// escribible por un usuario sin privilegios se registraría callado como SYSTEM. La trampa que tiene
// que resistir es el prefijo TEXTUAL: `C:\Program FilesMalo` NO está dentro de `C:\Program Files`
// aunque el string empiece igual. Por eso usa filepath.Rel+IsLocal y no strings.HasPrefix.
func TestRutaDentroDe(t *testing.T) {
	const base = `C:\Program Files`
	casos := []struct {
		nombre string
		ruta   string
		dentro bool
	}{
		{"anidado normal", `C:\Program Files\Musubi\musubi.exe`, true},
		{"la base misma", `C:\Program Files`, true},
		{"prefijo textual tramposo", `C:\Program FilesMalo\musubi.exe`, false},
		{"fuera, en el perfil del usuario", `C:\Users\admin\Downloads\musubi.exe`, false},
		{"fuera, otro volumen", `D:\Proyectos\Musubi\musubi.exe`, false},
	}
	for _, c := range casos {
		if got := rutaDentroDe(c.ruta, base); got != c.dentro {
			t.Errorf("%s: rutaDentroDe(%q, %q) = %v; esperaba %v", c.nombre, c.ruta, base, got, c.dentro)
		}
	}
}
