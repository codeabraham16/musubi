package testbudget

// EL LECTOR, PROBADO DE LOS DOS LADOS.
//
// TimeoutDeEstaCorrida tenía TRES ramas que devuelven «no pude leer» y ninguna estaba probada:
// el único test que la tocaba la llamaba en una corrida normal y comprobaba que leyera. O sea
// que el lector estaba probado del lado en que anda, que es justo el lado que no decide nada —
// si las tres ramas empezaran a devolver «leí, y da 0», el guard entero quedaría mudo y todo
// seguiría verde.

import (
	"flag"
	"strings"
	"testing"
	"time"
)

// valorSinGetter es un flag.Value que NO implementa flag.Getter.
type valorSinGetter struct{}

func (valorSinGetter) String() string   { return "" }
func (valorSinGetter) Set(string) error { return nil }

// getterDeOtroTipo implementa flag.Getter pero devuelve algo que no es una duración.
type getterDeOtroTipo struct{}

func (getterDeOtroTipo) String() string   { return "" }
func (getterDeOtroTipo) Set(string) error { return nil }
func (getterDeOtroTipo) Get() any         { return "no soy una duración" }

// getterDuracion es lo que hace de verdad el paquete testing.
type getterDuracion time.Duration

func (g getterDuracion) String() string    { return time.Duration(g).String() }
func (g *getterDuracion) Set(string) error { return nil }
func (g getterDuracion) Get() any          { return time.Duration(g) }

func buscarQueDevuelve(f *flag.Flag) func(string) *flag.Flag {
	return func(string) *flag.Flag { return f }
}

func flagCon(v flag.Value) *flag.Flag {
	return &flag.Flag{Name: nombreFlagTimeout, Value: v}
}

func TestLasTresRamasDeNoPuedoLeer(t *testing.T) {
	diez := getterDuracion(10 * time.Minute)
	cero := getterDuracion(0)

	casos := []struct {
		nombre   string
		buscar   func(string) *flag.Flag
		argv     []string
		leido    bool
		valor    time.Duration
		enMotivo string
	}{
		{
			nombre:   "el flag no existe",
			buscar:   buscarQueDevuelve(nil),
			argv:     []string{"binario"},
			enMotivo: "no registró",
		},
		{
			nombre:   "el flag no expone su valor",
			buscar:   buscarQueDevuelve(flagCon(valorSinGetter{})),
			argv:     []string{"binario"},
			enMotivo: "no expone su valor",
		},
		{
			nombre:   "el flag no es una duración",
			buscar:   buscarQueDevuelve(flagCon(getterDeOtroTipo{})),
			argv:     []string{"binario"},
			enMotivo: "no es una duración",
		},
		{
			// LA CUARTA RAMA, QUE ES LA QUE FALTABA. `go test` SIEMPRE le pasa -test.timeout al
			// binario; si el flag dice 0 y la línea de comando dice 10m, es que nadie parseó
			// todavía. Antes eso se leía como «-timeout 0 = sin límite» y el guard se callaba.
			nombre:   "el flag dice 0 y la línea de comando dice otra cosa: falta flag.Parse()",
			buscar:   buscarQueDevuelve(flagCon(&cero)),
			argv:     []string{"binario", "-test.timeout=10m0s"},
			enMotivo: "flag.Parse()",
		},
		{
			nombre: "flag y línea de comando coinciden",
			buscar: buscarQueDevuelve(flagCon(&diez)),
			argv:   []string{"binario", "-test.timeout=10m0s"},
			leido:  true,
			valor:  10 * time.Minute,
		},
		{
			nombre: "sin -test.timeout en la línea de comando, el flag manda",
			buscar: buscarQueDevuelve(flagCon(&diez)),
			argv:   []string{"binario"},
			leido:  true,
			valor:  10 * time.Minute,
		},
		{
			// -timeout 0 es legítimo: «sin límite». Con la línea de comando de acuerdo, se lee.
			nombre: "cero legítimo, sin nada que lo contradiga",
			buscar: buscarQueDevuelve(flagCon(&cero)),
			argv:   []string{"binario"},
			leido:  true,
			valor:  0,
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			d, leido, motivo := leerTimeout(c.buscar, c.argv)
			if leido != c.leido {
				t.Fatalf("leido = %v, quiero %v (motivo: %q)", leido, c.leido, motivo)
			}
			if leido && d != c.valor {
				t.Errorf("valor = %v, quiero %v", d, c.valor)
			}
			if !leido {
				if motivo == "" {
					t.Fatal("no pudo leer y no dijo por qué: ese silencio es el defecto")
				}
				if !strings.Contains(motivo, c.enMotivo) {
					t.Errorf("el motivo no menciona %q: %s", c.enMotivo, motivo)
				}
			}
		})
	}
}

func TestTimeoutEnArgv(t *testing.T) {
	casos := []struct {
		argv  []string
		valor time.Duration
		hay   bool
	}{
		{[]string{"b", "-test.timeout=40m"}, 40 * time.Minute, true},
		{[]string{"b", "--test.timeout=40m"}, 40 * time.Minute, true},
		{[]string{"b", "-test.timeout", "40m"}, 40 * time.Minute, true},
		{[]string{"b", "--test.timeout", "1h30m"}, 90 * time.Minute, true},
		{[]string{"b", "-test.timeout=0s"}, 0, true},
		{[]string{"b", "-test.v", "-test.run=X"}, 0, false},
		{[]string{"b", "-test.timeout"}, 0, false},          // sin valor
		{[]string{"b", "-test.timeout=cuarenta"}, 0, false}, // no parsea
		{[]string{"b", "-test.timeoutx=40m"}, 0, false},     // el vecino, no el flag
		{nil, 0, false},
	}
	for _, c := range casos {
		d, hay := timeoutEnArgv(c.argv)
		if hay != c.hay || d != c.valor {
			t.Errorf("timeoutEnArgv(%v) = (%v, %v), quiero (%v, %v)", c.argv, d, hay, c.valor, c.hay)
		}
	}
}

// ErrorSiElGuardNoCorrio es lo que convierte los dos sabotajes de una línea en rojo.
func TestErrorSiElGuardNoCorrio(t *testing.T) {
	restaurar := func() { guardarObservacionNil() }
	t.Cleanup(restaurar)

	casos := []struct {
		nombre  string
		obs     *Observacion
		argv    []string
		enError string
	}{
		{
			nombre:  "nadie lo llamó (le borraron la llamada al TestMain)",
			obs:     nil,
			argv:    []string{"b", "-test.timeout=40m"},
			enError: "NO corrió",
		},
		{
			nombre:  "corrió pero no pudo leer (le borraron el flag.Parse())",
			obs:     &Observacion{Leido: false, Motivo: "falta flag.Parse()"},
			argv:    []string{"b", "-test.timeout=40m"},
			enError: "quedó mudo",
		},
		{
			nombre:  "corrió y no pudo leer la política",
			obs:     &Observacion{Leido: true, Timeout: 40 * time.Minute, Politica: "no existe el archivo"},
			argv:    []string{"b", "-test.timeout=40m"},
			enError: "política",
		},
		{
			nombre:  "leyó un techo que no es el de esta corrida",
			obs:     &Observacion{Leido: true, Timeout: time.Minute},
			argv:    []string{"b", "-test.timeout=40m"},
			enError: "no es el que rige",
		},
		{
			nombre: "todo en orden",
			obs:    &Observacion{Leido: true, Timeout: 40 * time.Minute},
			argv:   []string{"b", "-test.timeout=40m"},
		},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if c.obs == nil {
				guardarObservacionNil()
			} else {
				guardarObservacion(*c.obs)
			}
			err := ErrorSiElGuardNoCorrio(c.argv)
			if c.enError == "" {
				if err != nil {
					t.Fatalf("quería nil, dio %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("quedó VERDE")
			}
			if !strings.Contains(err.Error(), c.enError) {
				t.Errorf("el error no menciona %q: %v", c.enError, err)
			}
		})
	}
}
