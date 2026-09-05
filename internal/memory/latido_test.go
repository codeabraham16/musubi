package memory

// Pruebas de LatirYTomarComandos: la operación que junta en UNA transacción la señal de vida y el
// paso por la cola (ver latido.go para la aritmética que la justifica).
//
// Lo que se prueba acá NO es el ahorro —eso lo cuenta internal/mcp/latido_una_tx_test.go, que es
// donde se puede contar— sino que juntarlas no aflojó ninguna de las dos semánticas que ya
// valían por separado.

import (
	"testing"
	"time"

	"musubi/internal/fleet"
)

// UNA MÁQUINA REVOCADA NO SE LLEVA SU COLA, y este orden es la mitad delicada de la unificación:
// el UPDATE de la señal de vida es el que descubre que la fila ya no está activa, y hasta que no
// devuelve `false` no se sabe. Si la cola se tocara antes —o si el `false` commiteara la
// transacción en vez de abandonarla— una máquina dada de baja hace treinta segundos se llevaría
// el `systemctl` que quedó encolado, y el kill-switch dejaría de ser un kill-switch.
//
// Sabotaje que la hace fallar: en internal/memory/latido.go, mover el `tomarComandosEnTx` ARRIBA
// del `if !vivo` (o borrar ese `if`) → el comando vuelve entregado y la fila queda marcada.
func TestElLatidoDeUnaMaquinaRevocadaNoLeEntregaLaCola(t *testing.T) {
	e := newTestEngine(t)
	alta, _ := altaDePrueba(t, e, "casa", "pc-gio")

	c, err := e.EncolarComando(fleet.Comando{
		DeviceID: alta.ID, ProjectID: "casa", Principal: "gio",
		Argv: []string{"uptime"}, Timeout: 10 * time.Second, Creado: time.Now(),
	})
	if err != nil {
		t.Fatalf("EncolarComando: %v", err)
	}
	if _, err := e.RevocarDevice("casa", "pc-gio"); err != nil {
		t.Fatal(err)
	}

	vivo, entregados, err := e.LatirYTomarComandos(alta.ID, time.Now(), "", 10)
	if err != nil {
		t.Fatalf("el latido de una máquina revocada devolvió error: %v (A10: no lo es)", err)
	}
	if vivo {
		t.Error("el latido de una máquina revocada dijo que actualizó")
	}
	if len(entregados) != 0 {
		t.Fatalf("una máquina REVOCADA se llevó %d comando(s): el kill-switch no cortó nada",
			len(entregados))
	}
	// Y la fila del comando tampoco puede haber quedado marcada: si se marcó `entregado` sin que
	// nadie lo reciba, ese comando no se le entrega NUNCA más a la máquina que lo pidió.
	guardado, ok, err := e.ComandoPorID(c.ID)
	if err != nil || !ok {
		t.Fatalf("ComandoPorID: ok=%v err=%v", ok, err)
	}
	if guardado.Estado != fleet.EstadoPendiente {
		t.Errorf("el comando quedó en %q y esperaba %q: se marcó entregado sin entregarse a nadie",
			guardado.Estado, fleet.EstadoPendiente)
	}
}

// VACÍO NO BORRA LA MUESTRA ANTERIOR, por el camino nuevo también. La regla ya valía en
// LatirDevice y es la que sostiene que «estar viva» y «saber medirse» son cosas distintas: un
// agente en un OS sin colector late igual y no tiene por qué hacer desaparecer la última medición
// buena. Se prueba de nuevo acá porque ÉSTE es el camino que corre en producción desde que la
// puerta del latido usa la operación unificada.
//
// Sabotaje que la hace fallar: en internal/memory/devices.go, sacarle el CASE a latirDeviceCon y
// escribir `last_sample = ?` pelado.
func TestElLatidoUnificadoConMuestraVaciaNoBorraLaAnterior(t *testing.T) {
	e := newTestEngine(t)
	alta, token := altaDePrueba(t, e, "casa", "pc-gio")

	const muestra = `{"tomada":"2026-09-03T12:00:00Z","num_cpu":8,"mem_total":16000,"mem_usada":8000}`
	if vivo, _, err := e.LatirYTomarComandos(alta.ID, time.Now(), muestra, 10); err != nil || !vivo {
		t.Fatalf("primer latido: vivo=%v err=%v", vivo, err)
	}
	// El colector se rompió: la máquina late sin muestra.
	if vivo, _, err := e.LatirYTomarComandos(alta.ID, time.Now(), "", 10); err != nil || !vivo {
		t.Fatalf("segundo latido: vivo=%v err=%v", vivo, err)
	}

	got, ok, err := e.DevicePorToken(token)
	if err != nil || !ok {
		t.Fatalf("DevicePorToken: ok=%v err=%v", ok, err)
	}
	if got.UltimaMuestra == nil {
		t.Fatal("un latido sin muestra borró la última medición buena: la máquina queda viva y " +
			"muda, y el panel no puede distinguirla de una que nunca midió")
	}
	if got.UltimaMuestra.NumCPU != 8 {
		t.Errorf("la muestra conservada dice num_cpu=%d, esperaba 8", got.UltimaMuestra.NumCPU)
	}
}

// LO VENCIDO SIGUE VENCIENDO ADENTRO DEL LATIDO. El UPDATE de vencimiento era lo primero que
// hacía TomarComandos, y al mudarlo adentro de la transacción del latido es exactamente el tipo
// de línea que se cae en un refactor sin que nada se queje: el síntoma es un agente que estuvo
// caído una semana y al volver corre lo que se le pidió el lunes.
//
// Sabotaje que la hace fallar: quitar el UPDATE de vencimiento de tomarComandosEnTx
// (internal/memory/comandos.go).
func TestElLatidoUnificadoNoEntregaLoQueYaVencio(t *testing.T) {
	e := newTestEngine(t)
	alta, _ := altaDePrueba(t, e, "casa", "pc-gio")

	viejo := time.Now().Add(-2 * fleet.ComandoVidaMax)
	c, err := e.EncolarComando(fleet.Comando{
		DeviceID: alta.ID, ProjectID: "casa", Principal: "gio",
		Argv: []string{"reboot"}, Timeout: 10 * time.Second, Creado: viejo,
	})
	if err != nil {
		t.Fatalf("EncolarComando: %v", err)
	}

	vivo, entregados, err := e.LatirYTomarComandos(alta.ID, time.Now(), "", 10)
	if err != nil || !vivo {
		t.Fatalf("latido: vivo=%v err=%v", vivo, err)
	}
	if len(entregados) != 0 {
		t.Fatalf("el latido entregó %d comando(s) VENCIDO(s): un agente que estuvo caído una "+
			"semana correría al volver lo que se le pidió el lunes", len(entregados))
	}
	guardado, ok, err := e.ComandoPorID(c.ID)
	if err != nil || !ok {
		t.Fatalf("ComandoPorID: ok=%v err=%v", ok, err)
	}
	if guardado.Estado != fleet.EstadoExpirado {
		t.Errorf("el comando viejo quedó en %q y esperaba %q: sin el vencimiento adentro de la "+
			"transacción del latido, la cola nunca se limpia", guardado.Estado, fleet.EstadoExpirado)
	}
}
