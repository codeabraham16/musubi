package memory

import (
	"fmt"
	"strconv"
	"strings"
)

// metaBajadaLease es la fila de `meta` que dice qué proceso está bajando memoria del central.
const metaBajadaLease = "sync:inbound_lease"

// ReclamarBajada intenta quedarse con la bajada de esta base durante leaseSeconds. Devuelve true si
// el candado es de quien llama —estaba libre, vencido, o ya era suyo— y false si lo tiene otro
// proceso vivo. Un valor que no tiene la forma dueño|vence cuenta como libre: una escritura cortada
// (un apagón deja archivos en ceros en esta PC) no puede trabar la bajada para siempre.
//
// EL DEFECTO QUE ESTO ARREGLA (medido 2026-09-23). La SUBIDA ya estaba protegida contra varios
// daemons sobre la misma base —ClaimOutboxBatch toma un lease de 60 s— pero la BAJADA no tenía
// nada: drainInboundOnce leía el cursor con GetMeta y lo escribía con SetMeta, sin reclamar nada.
// Con dos terminales abiertas en el mismo proyecto, las dos bajaban las mismas páginas en cada tick.
// Medido en el central, en 24 h y contra las 2.880 consultas que haría UN proceso cada 30 s: la
// laptop (davantis-2) tiró 4.997, el equivalente a 1,7 procesos; Altura, 4.374, a 1,5.
//
// Es UNA sentencia atómica, y el vencimiento sale del reloj de SQLite (`datetime('now')`), igual que
// el lease del outbox: todos los procesos que comparten esta base comparten también ese reloj, así
// que la comparación no depende de que dos relojes de Go coincidan. El valor es `dueño|vence`, con
// `vence` en el formato de datetime(), que se ordena bien como texto.
//
// Quien lo tiene lo RENUEVA en cada tick en vez de soltarlo: soltarlo al terminar dejaría que el
// otro proceso bajara en SU tick siguiente, y los dos volverían a alternarse bajando lo mismo. Si el
// dueño muere, el candado vence solo y otro lo toma.
//
// El valor se lee DESDE LA DERECHA: el vencimiento es un datetime de ancho fijo (19 caracteres) y el
// dueño es todo lo anterior al último '|'. Partir por el PRIMER '|' —la primera versión— confundía
// dueño y vencimiento en cuanto el dueño traía un '|', y el candado quedaba trabado para siempre.
func (e *DbEngine) ReclamarBajada(dueno string, leaseSeconds int) (bool, error) {
	if err := validarDuenoBajada(dueno); err != nil {
		return false, err
	}
	if leaseSeconds <= 0 {
		leaseSeconds = 120
	}
	res, err := e.db.Exec(`
		INSERT INTO meta (key, value, updated_at)
		VALUES (?, ? || '|' || datetime('now', '+' || ? || ' seconds'), datetime('now'))
		ON CONFLICT(key) DO UPDATE
		SET value = excluded.value, updated_at = excluded.updated_at
		WHERE meta.value = ''
		   OR instr(meta.value, '|') = 0
		   OR length(meta.value) < 21
		   OR substr(meta.value, 1, length(meta.value) - 20) = ?
		   OR substr(meta.value, -19) <= datetime('now')`,
		metaBajadaLease, dueno, strconv.Itoa(leaseSeconds), dueno)
	if err != nil {
		return false, fmt.Errorf("error al reclamar la bajada: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("error al leer el resultado del reclamo de la bajada: %w", err)
	}
	return n == 1, nil
}

// SoltarBajada libera el candado, sólo si es de quien llama.
//
// EL DEFECTO QUE ESTO CIERRA, y que encontró una revisión adversarial antes del merge: el dueño
// renovaba el candado al EMPEZAR cada tick, antes de ir a la red. Si su bajada fallaba siempre
// —el token vencido de una terminal que arrancó antes de rotarlo, un central_url viejo—, igual lo
// renovaba, y la terminal sana no bajaba NUNCA MÁS. Sin candado, la sana bajaba sola; con el
// candado, una sola terminal rota dejaba a toda la base sin bajada. Reproducido: con un dueño que
// recibe 401 y otra terminal sana, cinco ticks cada una, la sana hizo cero pedidos.
//
// Soltar en vez de dejar vencer: vencer tarda el lease entero (cuatro ticks), y en ese rato nadie
// baja. Soltado, la terminal sana lo toma en su tick siguiente.
func (e *DbEngine) SoltarBajada(dueno string) error {
	if err := validarDuenoBajada(dueno); err != nil {
		return err
	}
	_, err := e.db.Exec(`
		UPDATE meta SET value = '', updated_at = datetime('now')
		WHERE key = ? AND length(value) >= 21 AND substr(value, 1, length(value) - 20) = ?`,
		metaBajadaLease, dueno)
	if err != nil {
		return fmt.Errorf("error al soltar la bajada: %w", err)
	}
	return nil
}

// AvanzarCursorBajada guarda el cursor de la bajada SÓLO si es mayor que el guardado.
//
// El candado deja un solo proceso bajando por vez, pero no alcanza solo: un tick que tarda más que el
// lease —veinte páginas sobre una red lenta— puede seguir escribiendo después de que otro proceso
// tomó el candado vencido, y con SetMeta a secas el más lento pisaba al más rápido y el cursor
// RETROCEDÍA. Retroceder no pierde datos —la ingesta es idempotente— pero hace re-bajar lo ya bajado,
// que es justo el tráfico que este arreglo viene a sacar. Monótono en la misma sentencia, sin ventana.
func (e *DbEngine) AvanzarCursorBajada(key string, v int64) error {
	_, err := e.db.Exec(`
		INSERT INTO meta (key, value, updated_at) VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE
		SET value = excluded.value, updated_at = excluded.updated_at
		WHERE CAST(meta.value AS INTEGER) < CAST(excluded.value AS INTEGER)`,
		key, strconv.FormatInt(v, 10))
	if err != nil {
		return fmt.Errorf("error al avanzar el cursor de la bajada: %w", err)
	}
	return nil
}

// validarDuenoBajada rechaza el dueño vacío y el que contiene '|'. El valor guardado es
// «dueño|vence» y se parte por el PRIMER '|': con un dueño «a|b» se leería el dueño «a» —que no puede
// renovar ni soltar— y el vencimiento «b|…», que como texto nunca queda atrás de una fecha. El
// candado quedaba trabado para todos, para siempre. Hoy el dueño es un uuid y no llega a pasar, pero
// la función es exportada.
func validarDuenoBajada(dueno string) error {
	if dueno == "" {
		return fmt.Errorf("candado de la bajada: dueño vacío")
	}
	if strings.ContainsRune(dueno, '|') {
		return fmt.Errorf("candado de la bajada: el dueño %q contiene '|', el separador del valor guardado", dueno)
	}
	return nil
}
