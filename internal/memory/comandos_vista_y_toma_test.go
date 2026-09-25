package memory

import (
	"fmt"
	"testing"
	"time"

	"musubi/internal/fleet"
)

// LA VISTA VENCE UN PENDIENTE EN EL MISMO SEGUNDO EN QUE LA TOMA LO VENCE, Y ÉSE ES EL SEGUNDO DE
// SU VIDA MÁXIMA.
//
// `expirado` lo DECIDE la toma (tomarComandosEnTx, por TomarComandos y por el latido): lo que vence
// no se entrega. Lo MUESTRA EstadoActual, que las superficies llaman al leer. Si las dos no usan el
// mismo reloj, una pantalla dice `expirado` sobre algo que el agente todavía se lleva y ejecuta, o
// `pendiente` sobre algo que ya no va a correr nunca.
//
// LA GUARDA VIEJA (TestUnComandoPendienteYViejoSeMuestraExpirado, internal/fleet) CLAVABA LA EDAD en
// 10 h y 1 min: cualquier umbral entre los dos la dejaba verde. La auditoría A131 (C3-m6) le puso a
// la vista el reloj de los ENTREGADOS (EsperaMaxDeEntregado, entonces de 102 min; desde T10, de tres
// horas y media) y fleet, memory y mcp quedaron verdes. Acá la edad recorre el borde de a un
// segundo, a los dos lados, y a una hora con fracción y otra sin: la tabla guarda segundos enteros,
// así que el borde de la decisión es un segundo entero.
//
// Y ESO DESTAPÓ UN DEFECTO VIVO DEL ÁRBOL SANO: la vista restaba con nanosegundos y la toma compara
// un texto truncado al segundo. A las hh:mm:ss.7, un pendiente creado justo `ComandoVidaMax` antes
// se dibujaba `expirado` mientras la toma, a esa misma hora, lo entregaba. Ahora las dos leen el
// límite de fleet.LimiteDeVida.
//
// EXPOSICIÓN medida por la auditoría: 11.010 comandos vencieron en los últimos 30 días, todos después
// de quedar pendientes más de 15 minutos; con la mutación cada uno se habría mostrado `pendiente`
// hasta 87 minutos de más (957.746 filas·minuto). Hoy no hay pendientes. El caso vuelve cada vez que
// un agente se cae y alguien le encola algo.
//
// Sabotaje: que la vista vuelva a restar con nanosegundos, con su propio reloj → en el segundo del
// borde dibuja `expirado` sobre lo que la toma entrega.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="\treturn c.Estado == EstadoPendiente && c.Creado.Before(LimiteDeVida(ahora))"
// arnes: a="\treturn c.Estado == EstadoPendiente && ahora.Sub(c.Creado) > ComandoVidaMax"
// arnes: colision_ok="TestUnPendienteVenceAlSegundoDeSuVidaMax"
// Sabotaje: que el límite de vida use el reloj de los entregados → la vista y la toma coinciden entre
// sí y las dos dejan vivo 87 minutos de más lo que tenía que vencer.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="\treturn ahora.Add(-ComandoVidaMax).Truncate(time.Second)"
// arnes: a="\treturn ahora.Add(-EsperaMaxDeEntregado).Truncate(time.Second)"
func TestLaVistaYLaTomaVencenUnPendienteEnElMismoSegundo(t *testing.T) {
	e := newTestEngine(t)
	// Una hora atrás, en un segundo entero: la toma no mira el reloj de la máquina, mira `ahora`.
	base := time.Now().UTC().Truncate(time.Second).Add(-time.Hour)

	tomas := []struct {
		nombre string
		tomar  func(id string, ahora time.Time) ([]fleet.Comando, error)
	}{
		{"TomarComandos", func(id string, ahora time.Time) ([]fleet.Comando, error) {
			return e.TomarComandos(id, ahora, fleet.ColaMaxPorDevice)
		}},
		{"LatirYTomarComandos", func(id string, ahora time.Time) ([]fleet.Comando, error) {
			_, cs, err := e.LatirYTomarComandos(id, ahora, "", fleet.ColaMaxPorDevice)
			return cs, err
		}},
	}
	// La edad, relativa al borde en segundos enteros: los dos lados, a uno y dos segundos, y a treinta
	// —adentro de lo que el reloj de los entregados todavía dejaría vivo—.
	desplazamientos := []time.Duration{-30 * time.Second, -2 * time.Second, -time.Second, 0, time.Second, 2 * time.Second, 30 * time.Second}

	for _, toma := range tomas {
		for _, fraccion := range []time.Duration{0, 700 * time.Millisecond} {
			ahora := base.Add(fraccion)
			caso := fmt.Sprintf("%s a las %s", toma.nombre, ahora.Format("15:04:05.000"))
			d, _ := altaDePrueba(t, e, "casa", fmt.Sprintf("pc-%s-%d", toma.nombre, fraccion.Milliseconds()))
			// El borde: el `creado` más viejo que la toma todavía entrega, en segundos enteros.
			borde := ahora.Truncate(time.Second).Add(-fleet.ComandoVidaMax)

			desplazamientoDe := map[string]time.Duration{}
			for _, dz := range desplazamientos {
				c, err := e.EncolarComando(fleet.Comando{
					DeviceID: d.ID, ProjectID: "casa", Principal: "gio", Creado: borde.Add(dz),
					Argv: []string{"echo", "x"}, Timeout: 30 * time.Second,
				})
				if err != nil {
					t.Fatal(err)
				}
				desplazamientoDe[c.ID] = dz
			}

			// LA VISTA, antes de tomar: lo que cualquier superficie mostraría a esta hora.
			vista := map[string]fleet.EstadoComando{}
			for id := range desplazamientoDe {
				c, ok, err := e.ComandoPorID(id)
				if err != nil || !ok {
					t.Fatalf("ComandoPorID(%s): ok=%v err=%v", id, ok, err)
				}
				vista[id] = c.EstadoActual(ahora)
			}

			if _, err := toma.tomar(d.ID, ahora); err != nil {
				t.Fatalf("%s: %v", caso, err)
			}

			// LA DECISIÓN: lo que la toma hizo con cada uno.
			vencidos, entregados := 0, 0
			for id, dz := range desplazamientoDe {
				c, ok, err := e.ComandoPorID(id)
				if err != nil || !ok {
					t.Fatalf("ComandoPorID(%s): ok=%v err=%v", id, ok, err)
				}
				venceSegunLaToma := c.Estado == fleet.EstadoExpirado
				if venceSegunLaToma {
					vencidos++
				} else {
					entregados++
				}
				if (vista[id] == fleet.EstadoExpirado) != venceSegunLaToma {
					t.Errorf("%s, creado %s respecto del borde: la vista mostraba %q y la toma lo dejó %q — "+
						"una superficie dice una cosa y el agente hace otra", caso, dz, vista[id], c.Estado)
				}
				// Y el borde es el de la vida máxima: vence lo creado ANTES, y nada más.
				if quiero := dz < 0; venceSegunLaToma != quiero {
					t.Errorf("%s, creado %s respecto del borde: la toma lo dejó %q y con una vida máxima de %s "+
						"tenía que quedar %s", caso, dz, c.Estado, fleet.ComandoVidaMax, map[bool]string{true: "expirado", false: "entregado"}[quiero])
				}
			}
			// EL PISO: los dos lados del borde se midieron.
			if vencidos == 0 || entregados == 0 {
				t.Fatalf("%s: %d vencidos y %d entregados; sin los dos lados, la comparación no midió el borde", caso, vencidos, entregados)
			}
		}
	}
}
