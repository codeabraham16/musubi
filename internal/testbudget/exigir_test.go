package testbudget

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// politicaFija arma una política COHERENTE con los dos techos: el de CI y el piso local. La
// medición se deriva del piso local para que la política de la prueba cumpla la misma banda que
// CargarPolitica exige en el archivo de verdad — si no, la prueba estaría ejercitando una
// política que el repo nunca podría tener.
func politicaFija(techo, local time.Duration) func() (Politica, error) {
	return func() (Politica, error) {
		return Politica{
			Timeout:          techo,
			TimeoutLocal:     local,
			MedicionLocal:    local / 2, // margen local exactamente 2,00×
			MargenMinimo:     2.0,
			UmbralLineasTest: 10000,
		}, nil
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
	const local = 45 * time.Minute
	casos := []struct {
		nombre       string
		bajoDetector bool
		timeout      time.Duration
		timeoutLeido bool
		quieroMotivo bool
	}{
		{"default de Go (10m) bajo -race: insuficiente", true, 10 * time.Minute, true, true},
		{"un pelo por debajo del techo: insuficiente", true, techo - time.Second, true, true},
		{"justo el techo de CI: alcanza", true, techo, true, false},
		{"entre el techo de CI y el piso local: alcanza (eso sí, con aviso)", true, 40 * time.Minute, true, false},
		{"justo el piso local: alcanza", true, local, true, false},
		{"por encima del piso local: alcanza", true, 60 * time.Minute, true, false},
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
			motivo, _ := decidirTimeout(c.bajoDetector, c.timeout, c.timeoutLeido, "no pude leer el flag", politicaFija(techo, local))
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
	motivo, _ := decidirTimeout(true, 10*time.Minute, true, "", politicaFija(30*time.Minute, 45*time.Minute))
	if motivo == "" {
		t.Fatal("10m bajo -race contra un techo de 30m tiene que dar motivo")
	}
	if !strings.Contains(motivo, "30m") {
		t.Errorf("el motivo no nombra el techo a usar (30m): %q", motivo)
	}
	// Y también el piso local: quien corre en su máquina necesita ESE número, no el de CI, que
	// es el que le puede dejar la corrida a 1,13× de morir.
	if !strings.Contains(motivo, "45m") {
		t.Errorf("el motivo no nombra el piso local (45m): %q", motivo)
	}
	if !strings.Contains(motivo, "-timeout") {
		t.Errorf("el motivo no nombra el flag: %q", motivo)
	}
}

// EL TRAMO DEL MEDIO: AVISA, NO ROMPE — Y AVISA DE VERDAD.
//
// Este es el agujero que abrió el 2026-09-11 y que ninguna prueba miraba: con un solo número,
// bajar RACE_TIMEOUT de 40m a 20m para que el guard del margen de CI dejara de estar rojo le
// pasó a recomendar 20m a quien corre en una máquina donde el paquete más lento mide 1059 s —o
// sea 1,13× de margen—. El guard decía «alcanza» sobre una corrida que iba a morir con el mismo
// panic ilegible que el guard vino a cerrar.
//
// Los casos van pegados a los DOS bordes del tramo, así que un juez que contestara siempre lo
// mismo falla en uno.
func TestElTramoEntreElTechoDeCIYElPisoLocalAvisaSinRomper(t *testing.T) {
	const techo = 20 * time.Minute
	const local = 40 * time.Minute
	casos := []struct {
		nombre      string
		timeout     time.Duration
		quieroFat   bool
		quieroAviso bool
	}{
		{"un pelo por debajo del techo de CI: fatal, sin aviso", techo - time.Second, true, false},
		{"justo el techo de CI (el número con el que corre CI): aviso, no fatal", techo, false, true},
		{"un pelo por debajo del piso local: todavía avisa", local - time.Second, false, true},
		{"justo el piso local: silencio", local, false, false},
		{"por encima del piso local: silencio", 2 * local, false, false},
		{"-timeout 0 (sin límite): silencio, no aviso", 0, false, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			fatal, aviso := decidirTimeout(true, c.timeout, true, "", politicaFija(techo, local))
			if (fatal != "") != c.quieroFat {
				t.Errorf("con -timeout %v el fatal fue %q; quería fatal=%v", c.timeout, fatal, c.quieroFat)
			}
			if (aviso != "") != c.quieroAviso {
				t.Errorf("con -timeout %v el aviso fue %q; quería aviso=%v", c.timeout, aviso, c.quieroAviso)
			}
			if fatal != "" && aviso != "" {
				t.Errorf("con -timeout %v salieron los dos a la vez: fatal=%q aviso=%q", c.timeout, fatal, aviso)
			}
			if !c.quieroAviso {
				return
			}
			// UN AVISO SIN NÚMEROS ES UNA MOLESTIA, NO UNA AYUDA. Tiene que nombrar el piso que
			// hace falta y los segundos de la medición que lo presupuesta.
			for _, quiero := range []string{"40m", "1200.0s"} {
				if !strings.Contains(aviso, quiero) {
					t.Errorf("el aviso no nombra %q:\n%s", quiero, aviso)
				}
			}
		})
	}
}

// Y EL AVISO SE IMPRIME DE VERDAD. Un aviso que se calcula y se devuelve pero que nadie escribe
// es la mitad exacta del defecto: la guarda que decide y no actúa. Esto lo afirma sobre el
// writer real que usa ExigirTimeoutSuficiente.
func TestElAvisoSeImprime(t *testing.T) {
	raiz := repoSintetico(t, "RACE_TIMEOUT=20m\nTIMEOUT_LOCAL=40m\nMEDICION_LOCAL_SEGUNDOS=1059.0\n"+
		"MARGEN_MINIMO=2.0\nUMBRAL_GUARDA_LINEAS_TEST=10000\n")

	var b strings.Builder
	anteriorW := salidaAvisos
	salidaAvisos = &b
	t.Cleanup(func() { salidaAvisos = anteriorW; guardarObservacionNil() })

	// exigir() es el cable entero: carga la política del archivo, decide, guarda la observación
	// e imprime. No se llama a decidirTimeout acá a propósito.
	if fatal := exigir(true, raiz, 20*time.Minute, true, ""); fatal != "" {
		t.Fatalf("20m es el techo de CI: no puede ser fatal, dio %q", fatal)
	}
	if b.Len() == 0 {
		t.Fatal("con -timeout 20m y un piso local de 40m el aviso NO se imprimió: se calculó y " +
			"se tiró, que es la guarda que decide y no actúa")
	}
	for _, quiero := range []string{"40m", "1059.0s"} {
		if !strings.Contains(b.String(), quiero) {
			t.Errorf("lo impreso no nombra %q:\n%s", quiero, b.String())
		}
	}
	// Y queda en la observación, para que se pueda afirmar sobre ello desde otro lado.
	obs, corrio := UltimaObservacion()
	if !corrio || obs.Aviso == "" {
		t.Errorf("la observación no guardó el aviso: corrio=%v obs=%+v", corrio, obs)
	}

	// EL OTRO LADO DEL MISMO CABLE: con el piso local satisfecho no se imprime nada. Sin esto,
	// un exigir() que imprimiera SIEMPRE pasaría lo de arriba.
	b.Reset()
	if fatal := exigir(true, raiz, 40*time.Minute, true, ""); fatal != "" {
		t.Fatalf("40m es el piso local: no puede ser fatal, dio %q", fatal)
	}
	if b.Len() != 0 {
		t.Errorf("con -timeout 40m no tendría que haber aviso, se imprimió:\n%s", b.String())
	}
}

// repoSintetico arma un directorio que RaizDelRepo reconoce (tiene go.mod) con el archivo de
// política que se le pase, y devuelve su ruta.
func repoSintetico(t *testing.T, politica string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module sintetico\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, NombreArchivoPolitica), []byte(politica), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Si la política no se puede leer, el guard NO se calla. Un guard que da verde cuando dejó de
// poder mirar es peor que no tenerlo: nadie se entera de que se apagó.
func TestPoliticaIlegibleEsMotivo(t *testing.T) {
	roto := func() (Politica, error) { return Politica{}, errors.New("borré el archivo") }
	if motivo, _ := decidirTimeout(true, 45*time.Minute, true, "", roto); motivo == "" {
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
