package memory

import (
	"fmt"
	"strconv"
	"strings"
)

// metaBajadaLease es la fila de `meta` que dice qué proceso está bajando memoria del central.
const metaBajadaLease = "sync:inbound_lease"

// MetaUltimaBajada es la fila de `meta` con la última bajada que SALIÓ A LA RED, en la forma
// formaDeLaUltimaBajada (ver UltimaBajada). La escribe sólo el dueño del candado, al final de un
// tick en que algún Pull volvió bien y en la misma transacción que suma ese viaje a sync_viajes
// (RegistrarBajada), y la lee musubi_sync_status.
//
// VA EN LA BASE Y NO EN LA MEMORIA DEL PROCESO porque sobre una misma base corren varios daemons
// —seis en davantis-1— y baja uno solo, el que tiene el candado. El agente le pregunta al SUYO, que
// casi nunca es ése: lo que dijera la memoria del que contesta sería el estado de uno que no baja.
//
// LA FORMA CRECE SÓLO AGREGANDO CAMPOS AL FINAL. LeerUltimaBajada exige los que este binario conoce
// e ignora los que vengan después, así que un binario lee la meta que escribió otro más nuevo —el
// ritmo de la bajada va a sumar los suyos— sin declararla ilegible. Sacar o reordenar un campo
// rompería eso: el que lee no tiene cómo saber que el tercero dejó de ser la próxima.
const MetaUltimaBajada = "sync:inbound_ultima"

// formaDeLaUltimaBajada son los campos que este binario escribe en MetaUltimaBajada, en orden.
const formaDeLaUltimaBajada = "unix|filas|proxima_unix|con_filas|paginas_del_dia"

// UltimaBajada es lo que guarda MetaUltimaBajada: cuándo volvió el último Pull (Unix, en segundos),
// cuántas filas se ingirieron en ese tick (Filas: las mismas que ese tick sumó a sync_viajes),
// cuándo espera el dueño volver a salir a la red (ProximaUnix), cuántas páginas del tick trajeron
// filas (ConFilas: las páginas del viaje menos las vacías, la misma cuenta que hace sync_viajes) y
// cuántas páginas de bajada tenía sync_viajes en el día de Unix al anotar, las de este tick
// incluidas (PaginasDelDia).
//
// UNA BAJADA VACÍA ES ConFilas EN CERO, NO Filas EN CERO. Una página que trae filas y no puede
// ingerir ninguna —una fila «veneno» que esta base rechaza, o un SQLITE_BUSY— deja Filas en 0 y el
// cursor quieto, y la misma fila vuelve primera en el tick siguiente. Con Filas sola eso se leía
// «(vacía)» al lado de un sync_viajes que cuenta esa página como página con filas, y tapaba justo
// la bajada atascada.
//
// PaginasDelDia la completa RegistrarBajada adentro de la transacción —lo que traiga del llamador
// se pisa—, y es lo que deja ver un binario que baja SIN anotar la edad el MISMO día de la meta: si
// ese día sync_viajes tiene más páginas que las anotadas, las sumó alguien después y no anotó.
type UltimaBajada struct {
	Unix, Filas, ProximaUnix, ConFilas, PaginasDelDia int64
}

// Valor es la forma en que se guarda (formaDeLaUltimaBajada).
func (u UltimaBajada) Valor() string {
	campos := []int64{u.Unix, u.Filas, u.ProximaUnix, u.ConFilas, u.PaginasDelDia}
	partes := make([]string, len(campos))
	for i, n := range campos {
		partes[i] = strconv.FormatInt(n, 10)
	}
	return strings.Join(partes, "|")
}

// LeerUltimaBajada es la inversa de Valor: lee los campos que este binario conoce y TOLERA los que
// vengan después (ver MetaUltimaBajada). Un valor sin esa forma es un ERROR y no un cero: un cero
// se leería «bajó el 1 de enero de 1970», y un apagón en esta PC deja escrituras cortadas.
func LeerUltimaBajada(v string) (UltimaBajada, error) {
	partes := strings.Split(strings.TrimSpace(v), "|")
	var n [5]int64
	if len(partes) < len(n) {
		return UltimaBajada{}, fmt.Errorf("la última bajada %q no tiene la forma %s", v, formaDeLaUltimaBajada)
	}
	for i := range n {
		x, err := strconv.ParseInt(partes[i], 10, 64)
		if err != nil {
			return UltimaBajada{}, fmt.Errorf("la última bajada %q no tiene la forma %s: %w", v, formaDeLaUltimaBajada, err)
		}
		n[i] = x
	}
	u := UltimaBajada{Unix: n[0], Filas: n[1], ProximaUnix: n[2], ConFilas: n[3], PaginasDelDia: n[4]}
	imposible := ""
	switch {
	case u.Unix <= 0:
		imposible = "un instante no positivo"
	case u.ProximaUnix < u.Unix:
		imposible = "la próxima antes que la última"
	case u.Filas < 0 || u.ConFilas < 0 || u.PaginasDelDia < 0:
		imposible = "un conteo negativo"
	}
	if imposible != "" {
		return UltimaBajada{}, fmt.Errorf("la última bajada %q es imposible: %s", v, imposible)
	}
	return u, nil
}

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

// ReiniciarBajadaPorAlcance pone el cursor de la bajada en CERO y registra el alcance nuevo, las dos
// cosas en UNA transacción. Es la ÚNICA vía del repo que hace RETROCEDER el cursor, y su nombre dice
// la única razón por la que se permite.
//
// POR QUÉ EXISTE. El filtro de proyecto del central no oculta filas: las SALTA. Entra al mismo WHERE
// que `sync_seq > ?` y el LIMIT se aplica después, así que la página se corre hacia arriba por encima
// de lo ajeno; y el `next_cursor` sale de las filas DEVUELTAS, o sea ya filtradas. Mientras la
// credencial es la misma eso no molesta —lo saltado no le corresponde—, pero cuando se ENSANCHA, el
// filtro desaparece y el cursor ya está arriba: esa historia no volvía JAMÁS, porque
// AvanzarCursorBajada es monótona a propósito y nada más escribía esta clave.
//
// Medido el 2026-09-24 en davantis-1: 64 filas que el central servía eran inalcanzables, las 64 por
// debajo del cursor y ninguna por encima, con el corte exacto en el borde de proyecto (`altura` 61 de
// 61 bajo sync_seq 855, 0 de 643 por encima). En la laptop eran 980 de 3.071.
//
// LAS DOS ESCRITURAS VAN JUNTAS O NINGUNA. Si el cursor quedara en 0 sin registrar el alcance, el
// próximo tick volvería a detectar un cambio de alcance y reiniciaría otra vez: un bucle que re-baja
// el corpus entero en cada tick. Si se registrara el alcance sin bajar el cursor, la reparación se
// perdería en silencio y el hueco quedaría igual pero ya sin forma de detectarlo.
//
// EL COSTO, que es real y se asume: reiniciar re-baja todo el corpus una vez. La ingesta es
// idempotente, así que no duplica ni pisa con contenido distinto, y con el sello 'espejo' tampoco
// vuelve a subir. Lo que sí hace es bumpear el `sync_seq` LOCAL de cada fila re-ingerida: en un nodo
// que a su vez sirve pulls (relay), sus clientes las van a ver como recién editadas.
func (e *DbEngine) ReiniciarBajadaPorAlcance(claveCursor, claveAlcance, alcance string) error {
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al abrir la transacción del reinicio de la bajada: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	const upsert = `INSERT INTO meta (key, value, updated_at) VALUES (?, ?, datetime('now'))
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`
	if _, err := tx.Exec(upsert, claveCursor, "0"); err != nil {
		return fmt.Errorf("error al poner en cero el cursor de la bajada: %w", err)
	}
	if _, err := tx.Exec(upsert, claveAlcance, alcance); err != nil {
		return fmt.Errorf("error al registrar el alcance de la bajada: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al commitear el reinicio de la bajada: %w", err)
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
