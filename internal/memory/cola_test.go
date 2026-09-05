package memory

// Pruebas del TECHO DE LA COLA de un dispositivo (A62).
//
// Lo pidió una medición, no una idea: una máquina caída hacía dos días tenía 11.007 comandos
// pendientes, encolados por un lazo que corrió treinta y cuatro horas contra un agente que no iba
// a levantar ninguno.

import (
	"errors"
	"testing"
	"time"

	"musubi/internal/fleet"
)

func encolarN(t *testing.T, e *DbEngine, deviceID string, n int, creado time.Time) error {
	t.Helper()
	var err error
	for i := 0; i < n; i++ {
		_, err = e.EncolarComando(fleet.Comando{
			DeviceID: deviceID, ProjectID: "casa", Principal: "gio", Creado: creado,
			Argv: []string{"echo", "x"}, Timeout: 30 * time.Second,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// LA COLA TIENE TECHO. Sin esto, un lazo contra una máquina caída llena la tabla sin límite — y
// contra una máquina VIVA, todo lo acumulado se ejecuta.
//
// Sabotaje: quitar la guarda de EncolarComando → falla acá.
func TestLaColaDeUnaMaquinaTieneTecho(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "pc-gio")
	ahora := time.Now().UTC()

	if err := encolarN(t, e, d.ID, fleet.ColaMaxPorDevice, ahora); err != nil {
		t.Fatalf("los primeros %d tienen que entrar: %v", fleet.ColaMaxPorDevice, err)
	}
	err := encolarN(t, e, d.ID, 1, ahora)
	if !errors.Is(err, fleet.ErrColaLlena) {
		t.Fatalf("el que pasa el techo tiene que rebotar con ErrColaLlena, obtuve: %v", err)
	}
	// EL MENSAJE TIENE QUE SERVIR PARA ALGO: quien lo lee necesita saber que el problema es la
	// MÁQUINA y no su comando, o va a reintentar cambiando el argv.
	if msg := err.Error(); !contiene(msg, "agente") || !contiene(msg, "esperando") {
		t.Errorf("el mensaje no explica qué pasa: %q", msg)
	}
}

// EL TECHO CUENTA SÓLO LO QUE TODAVÍA PODRÍA EJECUTARSE — y de paso es un FRENO DE RITMO: son
// cincuenta dentro de cualquier ventana de quince minutos. El lazo que llenó a `gio` iba a cinco
// por minuto, o sea setenta y cinco por ventana: habría rebotado a los cincuenta, incluso contra
// una máquina viva. Si contara todo lo pendiente, una
// máquina que estuvo caída un día quedaría bloqueada PARA SIEMPRE: sus filas muertas ocupando el
// cupo, y destrabarla exigiría borrar bitácora — que es justo lo que este repo no hace.
//
// Sabotaje: sacar el `AND creado >= ?` del conteo → falla acá, y `gio` (11.007 pendientes viejos)
// no podría volver a recibir un comando nunca.
func TestLoVencidoNoOcupaLugarEnLaCola(t *testing.T) {
	e := newTestEngine(t)
	d, _ := altaDePrueba(t, e, "casa", "pc-gio")
	ahora := time.Now().UTC()

	// El pasado de una máquina caída: el doble del techo, REPARTIDO EN EL TIEMPO como pasa de
	// verdad. Encolarlos todos con la misma marca sería otra cosa —cada uno vería a los otros
	// como frescos y el techo saltaría con razón—, y esa versión de la prueba mentía sobre lo
	// que estaba simulando.
	for i := 0; i < fleet.ColaMaxPorDevice*2; i++ {
		cuando := ahora.Add(-48*time.Hour + time.Duration(i)*time.Minute)
		if err := encolarN(t, e, d.ID, 1, cuando); err != nil {
			t.Fatalf("lo viejo tiene que poder entrar (era su momento, y venía espaciado): %v", err)
		}
	}
	// Y ahora uno nuevo: la máquina volvió, o alguien quiere mandarle algo.
	if err := encolarN(t, e, d.ID, 1, ahora); err != nil {
		t.Fatalf("una máquina con la cola llena de comandos VENCIDOS tiene que poder recibir uno nuevo: %v", err)
	}
}

// El techo es POR MÁQUINA. Una máquina saturada no puede dejar muda a la de al lado.
//
// Sabotaje: sacar el `device_id = ?` del conteo → falla acá.
func TestElTechoDeLaColaEsPorMaquina(t *testing.T) {
	e := newTestEngine(t)
	llena, _ := altaDePrueba(t, e, "casa", "pc-llena")
	otra, _ := altaDePrueba(t, e, "casa", "pc-otra")
	ahora := time.Now().UTC()

	if err := encolarN(t, e, llena.ID, fleet.ColaMaxPorDevice, ahora); err != nil {
		t.Fatal(err)
	}
	if err := encolarN(t, e, otra.ID, 1, ahora); err != nil {
		t.Fatalf("la máquina de al lado quedó muda por la cola de otra: %v", err)
	}
}

func contiene(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
