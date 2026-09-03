package memory

// latido.go junta en UNA SOLA TRANSACCIÓN todo lo que un latido escribe sobre la máquina que
// late: la señal de vida con su muestra, el vencimiento de la cola y la entrega de lo que le
// toca.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ EXISTE ESTE ARCHIVO: LA ARITMÉTICA DE LA FLOTA
//
// Un latido hacía como PISO cuatro escrituras en transacciones separadas —autorreporte,
// capacidad de preguntar, `LatirDevice` y el vencimiento de la cola adentro de `TomarComandos`—
// y el agente manda versión y `puede_preguntar` en TODOS los latidos, así que las cuatro
// ocurrían siempre.
//
// A 2000 máquinas cada 30 s son ~67 latidos/s. Cuatro commits por latido son ~270 commits/s con
// fsync, porque el WAL de esta base no fija `synchronous` explícito y el default es FULL (ver
// database.go). Ningún disco de un servidor de oficina sostiene eso, y el modo en que se rompe
// es el peor posible: `busy_timeout` empieza a comerse los latidos, las máquinas figuran caídas
// de a tandas, y el panel dice que se cayó media empresa cuando lo único que pasó es que la base
// no da más.
//
// Las dos mitades del arreglo son independientes y las dos hacen falta:
//   - ACÁ: las escrituras que SIEMPRE ocurren se juntan en una transacción.
//   - EN EL TRANSPORTE (internal/mcp/fleet_http.go): las que casi nunca cambian nada
//     —autorreporte, capacidad de preguntar, id de RustDesk— se comparan contra la fila que ya
//     se leyó por token y NO se escriben si dicen lo mismo que ya está guardado.
//
// NO SE TOCÓ `synchronous`. Bajarlo a NORMAL habría hecho desaparecer el síntoma sin arreglar
// nada, y a cambio de perder los últimos latidos ante un corte — que es exactamente el momento
// en el que uno mira el panel.

import (
	"fmt"
	"strings"
	"time"

	"musubi/internal/fleet"
	"musubi/internal/logx"
)

// LatirYTomarComandos estampa la señal de vida y entrega la cola de esa máquina, TODO en una
// transacción. Devuelve si la fila sigue activa y los comandos entregados.
//
// `vivo == false` significa lo mismo que en LatirDevice (invariante A10): la fila ya no está
// activa. NO es un error —un agente revocado que todavía no se enteró es lo normal— y el
// llamador tiene que responderle que se dé por revocado. En ese caso NO SE COMMITEA NADA y no se
// mira la cola: entregarle comandos a una máquina que acaba de ser dada de baja sería justo lo
// contrario de revocarla.
//
// LA COLA NO PUEDE TUMBAR EL LATIDO, y esa asimetría es deliberada y viene del código anterior
// (donde eran dos llamadas y el transporte ignoraba el error de la segunda). Seguir viva es lo
// que el latido AFIRMA, y quedarse sin comandos un ciclo es recuperable a los 30 s; perder la
// señal de vida, en cambio, dibuja caída una máquina que está perfectamente bien y dispara las
// alertas de flota. Así que si la cola falla se rehace SÓLO el latido en autocommit: dos
// transacciones en el camino raro, una en el que corre 67 veces por segundo.
func (e *DbEngine) LatirYTomarComandos(id string, ahora time.Time, muestra string, tope int) (bool, []fleet.Comando, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return false, nil, nil
	}
	tx, err := e.db.Begin()
	if err != nil {
		return false, nil, fmt.Errorf("error al abrir la transacción del latido de %q: %w", id, err)
	}
	defer tx.Rollback()

	vivo, err := latirDeviceCon(tx, id, ahora, muestra)
	if err != nil {
		return false, nil, err
	}
	if !vivo {
		return false, nil, nil
	}

	// `tope <= 0` es «no me interesa la cola»: se commitea el latido solo. Es la misma guarda
	// que tiene TomarComandos, y acá importa además porque tomarComandosEnTx la da por hecha.
	if tope <= 0 {
		if err := tx.Commit(); err != nil {
			return false, nil, fmt.Errorf("error al confirmar el latido de %q: %w", id, err)
		}
		return true, nil, nil
	}

	pendientes, errCola := tomarComandosEnTx(tx, id, ahora, tope)
	if errCola == nil {
		if errCola = tx.Commit(); errCola == nil {
			return true, pendientes, nil
		}
	}
	// El camino raro. Se AVISA —el error de la cola se perdía en silencio antes de esto— y se
	// reintenta el latido pelado, que es la parte que no se puede perder.
	logx.Warn("flota: la cola de comandos falló adentro del latido; se conserva la señal de vida "+
		"y la entrega se reintenta en el próximo latido", "device", id, "error", errCola)
	_ = tx.Rollback()
	vivo, err = latirDeviceCon(e.db, id, ahora, muestra)
	return vivo, nil, err
}
