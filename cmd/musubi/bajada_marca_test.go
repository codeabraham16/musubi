package main

import (
	"strconv"
	"strings"
	"testing"
	"time"

	"musubi/internal/config"
	"musubi/internal/memory"
	"musubi/internal/memory/memtest"
)

// LA MARCA DE ACTIVIDAD PARA LA BAJADA (bajada_marca.go): el hook del turno le avisa al dueño del
// candado de la bajada —otro proceso, casi siempre— que alguien está trabajando sobre esta base, y el
// dueño vuelve a preguntarle al central en su tick siguiente (internal/mcp/bajada_ritmo.go).

// TestElHookDelTurnoMarcaLaBajada: el turno escribe la marca con TODO prompt no vacío, también con los
// que no son consultas: un «sigue» y el aviso de una tarea de fondo son turnos en los que la persona
// está trabajando, y son justamente los que la compuerta deja afuera. El hook corre entero, con el
// motor real, y la marca se lee de la base. El control prueba que la compuerta los distingue: si la
// marca quedara detrás de ella, se vería.
//
// Sabotaje: la marca detrás de la compuerta, sólo con las consultas.
// arnes: archivo="cmd/musubi/turn.go"
// arnes: de="\tmarcarActividadParaLaBajada(store)\n\tesConsulta := esUnaConsulta(prompt)\n"
// arnes: a="\tesConsulta := esUnaConsulta(prompt)\n\tif esConsulta {\n\t\tmarcarActividadParaLaBajada(store)\n\t}\n"
func TestElHookDelTurnoMarcaLaBajada(t *testing.T) {
	for _, c := range []struct {
		nombre, prompt string
		consulta       bool
	}{
		{"un pedido sustantivo", "revisá el TLS del cerebro", true},
		{"un «sigue»", "sigue", false},
		{"el aviso de una tarea de fondo", avisoDeMonitor, false},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			if got := esUnaConsulta(c.prompt); got != c.consulta {
				t.Fatalf("control: esUnaConsulta(%q) = %v; la prueba supone %v", c.prompt, got, c.consulta)
			}
			eng, err := memory.NewDbEngine(memtest.DirSembrado(t))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { eng.Close() })
			if err := eng.SetMeta(memory.MetaDespertarBajada, "1"); err != nil { // una marca vieja
				t.Fatal(err)
			}
			antes := time.Now().Unix()
			turnoReal(t, eng, "S", c.prompt)
			raw, _, err := eng.GetMeta(memory.MetaDespertarBajada)
			if err != nil {
				t.Fatal(err)
			}
			if n, err := strconv.ParseInt(raw, 10, 64); err != nil || n < antes {
				t.Errorf("después de un turno con %s la marca de actividad tenía que ser de ahora (≥ %d), y es %q", c.nombre, antes, raw)
			}
		})
	}

	t.Run("un prompt vacío no marca", func(t *testing.T) {
		store := newFakeTurnStore()
		turnOutput(store, deltaLoop(), pipeOff(), maOff(), config.MemoryConfig{}, strings.NewReader(`{"session_id":"S","prompt":""}`))
		if v, ok := store.meta[memory.MetaDespertarBajada]; ok {
			t.Errorf("sin prompt no hay turno, y la marca se escribió: %q", v)
		}
	})
}

// TestLaMarcaDeLaBajadaNoSeReescribeEnRafaga: con una marca de menos de 15 s el turno no escribe:
// los avisos y los «sigue» llegan de a varios por minuto sobre una base que comparten varios daemons,
// y para el dueño —que mira una vez por tick de 30 s— una marca de hace 5 s dice lo mismo que una de
// ahora. Desde los 15 s se reescribe; una marca en el futuro o que no se entiende, también: no puede
// trabar la señal.
//
// Sabotaje: sin el umbral, cada turno escribe.
// arnes: archivo="cmd/musubi/bajada_marca.go"
// arnes: de="edad < int64(umbralMarcaDeBajada/time.Second)"
// arnes: a="edad < 0*int64(umbralMarcaDeBajada/time.Second)"
//
// Sabotaje: una marca en el futuro se toma por reciente y la señal queda trabada hasta que llegue.
// arnes: archivo="cmd/musubi/bajada_marca.go"
// arnes: de="edad >= 0 && "
// arnes: a="true && "
func TestLaMarcaDeLaBajadaNoSeReescribeEnRafaga(t *testing.T) {
	ahora := time.Unix(1_800_000_000, 0)
	hace := func(d time.Duration) string { return strconv.FormatInt(ahora.Add(-d).Unix(), 10) }
	for _, c := range []struct {
		nombre, marca string // "" = sin marca
		escribe       bool
	}{
		{"sin marca", "", true},
		{"una de hace 5 s", hace(5 * time.Second), false},
		{"una de hace 14 s", hace(14 * time.Second), false},
		{"una de hace 15 s", hace(15 * time.Second), true},
		{"una de hace una hora", hace(time.Hour), true},
		{"una en el futuro", hace(-time.Hour), true},
		{"una ilegible", "mañana", true},
	} {
		t.Run(c.nombre, func(t *testing.T) {
			store := &storeQueCuenta{fakeTurnStore: newFakeTurnStore(), escrituras: map[string]int{}}
			if c.marca != "" {
				store.meta[memory.MetaDespertarBajada] = c.marca
			}
			marcarActividadParaLaBajadaEn(store, ahora)
			if escribio := store.escrituras[memory.MetaDespertarBajada] == 1; escribio != c.escribe {
				t.Errorf("con %s: ¿escribió la marca? %v; tenía que ser %v", c.nombre, escribio, c.escribe)
			}
			if want := strconv.FormatInt(ahora.Unix(), 10); c.escribe && store.meta[memory.MetaDespertarBajada] != want {
				t.Errorf("con %s la marca tenía que quedar en %s, quedó %q", c.nombre, want, store.meta[memory.MetaDespertarBajada])
			}
		})
	}
}
