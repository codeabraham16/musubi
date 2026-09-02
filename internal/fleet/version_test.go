package fleet

import "testing"

// LAS DOS FAMILIAS DE VERSIÓN QUE HAY ENROLADAS TIENEN QUE PARSEAR.
//
// No es un caso teórico: el 2026-09-01 el cerebro corría `0.130.0-flota.38a0a9f` y los dos Windows
// `v0.106.0-28-gdf2ec21`. Si el parser sólo entiende una de las dos formas, la máquina vieja —que
// es justo la que hay que detectar— cae en el «no sé» y la serie no se emite: el agujero de A68
// quedaría exactamente igual, pero ahora con código que parece cubrirlo.
func TestElNucleoSaleDeLasDosFamiliasDeVersionQueExisten(t *testing.T) {
	casos := []struct {
		entrada string
		nucleo  string
		ok      bool
	}{
		{"0.130.0-flota.38a0a9f", "0.130.0", true}, // la que deriva construir.sh
		{"v0.106.0-28-gdf2ec21", "0.106.0", true},  // la vieja de git describe
		{"0.130.0", "0.130.0", true},               // pelada
		{"  0.130.0-flota.x  ", "0.130.0", true},   // con espacios alrededor
		{"0.130.0-sucio", "0.130.0", true},         // árbol sucio: el núcleo sigue siendo el mismo
		{"0.130.0+build5", "0.130.0", true},        // metadata semver
		{"dev", "", false},                         // build local sin ldflags
		{"", "", false},                            // Tier B
		{"0.130", "", false},                       // incompleta
		{"0.130.0.1", "", false},                   // cuatro componentes
		{"0.x.0", "", false},                       // no numérica
		{"v", "", false},                           // sólo el prefijo
	}
	for _, c := range casos {
		nucleo, ok := NucleoDeVersion(c.entrada)
		if ok != c.ok || nucleo != c.nucleo {
			t.Errorf("NucleoDeVersion(%q) = (%q, %v); esperaba (%q, %v)", c.entrada, nucleo, ok, c.nucleo, c.ok)
		}
	}
}

// EL CASO QUE HACE ÚTIL A LA MÉTRICA Y EL QUE LA HARÍA RUIDO, UNO AL LADO DEL OTRO.
//
// Sabotaje: comparar las cadenas completas en vez del núcleo → la primera fila (mismo release,
// commits distintos) pasa a difiere=true, y con ella toda la flota en cada redespliegue.
func TestDosCommitsDelMismoReleaseNoSonUnAgenteAtrasado(t *testing.T) {
	const cerebro = "0.130.0-flota.38a0a9f"

	casos := []struct {
		nombre     string
		agente     string
		difiere    bool
		comparable bool
	}{
		{"mismo release, otro commit", "0.130.0-flota.e140e0c", false, true},
		{"idéntica", cerebro, false, true},
		{"veinticuatro versiones atrás (el caso de A68)", "v0.106.0-28-gdf2ec21", true, true},
		{"adelantada: alguien desplegó un binario que el cerebro no tiene", "0.131.0-flota.aaaaaaa", true, true},
		{"Tier B: no corre nuestro binario", "", false, false},
		{"versión ilegible: no sabemos cuánto, sabemos que no es la nuestra", "vieja", true, true},
	}
	for _, c := range casos {
		difiere, comparable := VersionDelAgenteDifiere(c.agente, cerebro)
		if difiere != c.difiere || comparable != c.comparable {
			t.Errorf("%s: VersionDelAgenteDifiere(%q, %q) = (%v, %v); esperaba (%v, %v)",
				c.nombre, c.agente, cerebro, difiere, comparable, c.difiere, c.comparable)
		}
	}
}

// SI EL CEREBRO NO SABE SU PROPIA VERSIÓN, LA CULPA NO ES DE LA FLOTA.
//
// Un binario construido sin `-ldflags -X main.version` queda en `dev`. Con la comparación ingenua,
// `dev != 0.130.0` para TODAS las máquinas: la flota entera se dibuja atrasada por un problema de
// nuestro propio build. La serie se apaga, y el binario mal construido se descubre por otro lado
// —`musubi version` lo dice— en vez de por una alarma que acusa a las máquinas equivocadas.
//
// Sabotaje: devolver (true, true) cuando el núcleo del cerebro no parsea → falla acá.
func TestUnCerebroSinVersionNoMarcaAtrasadaALaFlotaEntera(t *testing.T) {
	for _, cerebro := range []string{"dev", "", "no-es-una-version"} {
		difiere, comparable := VersionDelAgenteDifiere("0.130.0-flota.e140e0c", cerebro)
		if comparable {
			t.Errorf("con el cerebro en %q la serie se emite igual (difiere=%v): un build propio sin "+
				"ldflags marcaría a toda la flota como atrasada", cerebro, difiere)
		}
	}
}
