package testbudget

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func politicaFija(techo time.Duration) func() (Politica, error) {
	return func() (Politica, error) {
		return Politica{Timeout: techo, MargenMinimo: 2.0, UmbralLineasTest: 10000}, nil
	}
}

// EL DEFAULT DE GO NO ALCANZA, Y LA CORRIDA SE ENTERA EN EL PRIMER SEGUNDO.
//
// Lo que decide es el string vacío / no vacío que hace `os.Exit(1)` en el TestMain de cada
// paquete caro (quiénes son los enumera TestLosPaquetesCarosLlamanAlGuard, no una lista escrita
// acá). La aserción mira ESO, no el texto del mensaje.
//
// El caso «10m» es el del cabo: es el default EXACTO de Go, el que usa cualquiera que corra
// `go test -race ./...` a secas.
func TestElDefaultDeGoBajoRaceEsInsuficiente(t *testing.T) {
	const techo = 30 * time.Minute
	casos := []struct {
		nombre       string
		bajoDetector bool
		timeout      time.Duration
		timeoutLeido bool
		quieroMotivo bool
	}{
		{"default de Go (10m) bajo -race: insuficiente", true, 10 * time.Minute, true, true},
		{"un pelo por debajo del techo: insuficiente", true, techo - time.Second, true, true},
		{"justo el techo: alcanza", true, techo, true, false},
		{"por encima del techo: alcanza", true, 45 * time.Minute, true, false},
		{"-timeout 0 (sin límite): alcanza", true, 0, true, false},
		// SIN -race el mismo 10m está bien: el paquete sin instrumentar entra holgado, y exigir
		// el techo grande volvería rojo el job `test-cross` y el paso de treesitter, que corren
		// a propósito sin detector.
		{"default de Go SIN -race: alcanza", false, 10 * time.Minute, true, false},
		{"sin -race ni siquiera mira el timeout", false, time.Second, true, false},
		// No poder LEER el flag no es «alcanza»: es no saber, y no saber es rojo.
		{"no se pudo leer el flag: motivo", true, 0, false, true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			motivo := decidirTimeout(c.bajoDetector, c.timeout, c.timeoutLeido, "no pude leer el flag", politicaFija(techo))
			if (motivo != "") != c.quieroMotivo {
				t.Fatalf("decidirTimeout(bajoDetector=%v, timeout=%v, leido=%v) dio %q; quería motivo=%v",
					c.bajoDetector, c.timeout, c.timeoutLeido, motivo, c.quieroMotivo)
			}
		})
	}
}

// El mensaje tiene que nombrar el techo que hay que pasar: si dijera sólo «insuficiente», el
// que lo lee vuelve a adivinar y no arreglamos nada.
func TestElMotivoNombraElTechoQueFalta(t *testing.T) {
	motivo := decidirTimeout(true, 10*time.Minute, true, "", politicaFija(30*time.Minute))
	if motivo == "" {
		t.Fatal("10m bajo -race contra un techo de 30m tiene que dar motivo")
	}
	if !strings.Contains(motivo, "30m") {
		t.Errorf("el motivo no nombra el techo a usar (30m): %q", motivo)
	}
	if !strings.Contains(motivo, "-timeout") {
		t.Errorf("el motivo no nombra el flag: %q", motivo)
	}
}

// Si la política no se puede leer, el guard NO se calla. Un guard que da verde cuando dejó de
// poder mirar es peor que no tenerlo: nadie se entera de que se apagó.
func TestPoliticaIlegibleEsMotivo(t *testing.T) {
	roto := func() (Politica, error) { return Politica{}, errors.New("borré el archivo") }
	if motivo := decidirTimeout(true, 45*time.Minute, true, "", roto); motivo == "" {
		t.Fatal("con la política ilegible el guard dio VERDE: tendría que ser un motivo")
	}
}

// TimeoutDeEstaCorrida tiene que poder LEER de verdad el flag en una corrida normal de `go test`.
// Sin esto, el guard entero podría estar contestando siempre «no pude leer» y los TestMain se
// caerían por la razón equivocada bajo -race.
func TestTimeoutDeEstaCorridaSeLee(t *testing.T) {
	d, ok := TimeoutDeEstaCorrida()
	if !ok {
		t.Fatal("no se pudo leer -test.timeout en una corrida normal de `go test`")
	}
	if d < 0 {
		t.Fatalf("-test.timeout = %v, no puede ser negativo", d)
	}
}
