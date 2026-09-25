package fleet

import (
	"testing"
	"time"
)

// UN PENDIENTE VENCE AL SEGUNDO DE SU VIDA MÁXIMA: ni antes, ni con otro reloj, ni con más
// resolución que la tabla.
//
// La guarda vieja (TestUnComandoPendienteYViejoSeMuestraExpirado) CLAVABA LA EDAD en 10 h y 1 min,
// así que cualquier umbral entre los dos la dejaba verde. La auditoría A131 (C3-m6) le puso a la vista
// el reloj de los ENTREGADOS —EsperaMaxDeEntregado, entonces de 102 min; desde T10, con la shell en la
// cuenta, de tres horas y media—, la confusión contra la que advierte el propio comentario de
// Vencido, y fleet, memory y mcp quedaron verdes: un pendiente de entre 15 y 102 minutos se mostraba
// `pendiente` aunque la toma ya no lo entregaría. Acá la edad recorre el borde de a un segundo,
// derivado de ComandoVidaMax, y la hora recorre el segundo entero y uno con fracción.
//
// LA FRACCIÓN ES EL OTRO EJE, Y ESTABA VIVO. La toma decide contra un texto RFC3339, o sea al segundo;
// la vista restaba con nanosegundos. A las hh:mm:ss.7, un pendiente creado justo ComandoVidaMax antes
// se mostraba `expirado` y la toma, a esa hora, todavía lo entregaba. Las dos leen ahora el mismo
// límite (LimiteDeVida), y la prueba que las cruza contra la base es
// TestLaVistaYLaTomaVencenUnPendienteEnElMismoSegundo (internal/memory).
//
// EXPOSICIÓN medida por la auditoría: 11.010 comandos vencieron en los últimos 30 días, todos
// pendientes más de 15 minutos; con el reloj equivocado cada uno se habría visto `pendiente` hasta 87
// minutos de más. El último de una persona, el 2026-09-20, se habría visto así 47 minutos.
//
// Sabotaje: que la vista use el reloj de los entregados (C3-m6, portado a LimiteDeVida) → un pendiente
// de 15 minutos y medio se muestra vivo.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="\treturn c.Estado == EstadoPendiente && c.Creado.Before(LimiteDeVida(ahora))"
// arnes: a="\treturn c.Estado == EstadoPendiente && c.Creado.Before(ahora.Add(-EsperaMaxDeEntregado).Truncate(time.Second))"
// arnes: colision_ok="TestLaVistaYLaTomaVencenUnPendienteEnElMismoSegundo"
func TestUnPendienteVenceAlSegundoDeSuVidaMax(t *testing.T) {
	entera := time.Date(2026, 8, 30, 5, 0, 0, 0, time.UTC)
	for _, ahora := range []time.Time{entera, entera.Add(700 * time.Millisecond)} {
		// El borde en segundos enteros: el `creado` más viejo que todavía se entrega a esta hora.
		borde := ahora.Truncate(time.Second).Add(-ComandoVidaMax)
		casos := []struct {
			nombre string
			creado time.Time
			quiero EstadoComando
		}{
			{"treinta segundos más viejo que su vida", borde.Add(-30 * time.Second), EstadoExpirado},
			{"un segundo más viejo que su vida", borde.Add(-time.Second), EstadoExpirado},
			{"exactamente su vida", borde, EstadoPendiente},
			{"un segundo más joven que su vida", borde.Add(time.Second), EstadoPendiente},
		}
		for _, c := range casos {
			got := Comando{Estado: EstadoPendiente, Creado: c.creado}.EstadoActual(ahora)
			if got != c.quiero {
				laToma := "todavía lo entrega"
				if c.quiero == EstadoExpirado {
					laToma = "ya no lo entrega"
				}
				t.Errorf("a las %s, un pendiente %s (vida máxima %s) se muestra %q y tiene que mostrarse %q, "+
					"porque a esa hora la toma %s: la vista y la decisión dicen dos cosas sobre el mismo comando",
					ahora.Format("15:04:05.000"), c.nombre, ComandoVidaMax, got, c.quiero, laToma)
			}
		}
	}
}
