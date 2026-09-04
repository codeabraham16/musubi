package memory

import (
	"fmt"
	"strings"
	"time"

	"musubi/internal/fleet"
)

// ════════════════════════════════════════════════════════════════════════════════════════════
// ROTACIÓN DEL TOKEN DE UN DISPOSITIVO
//
// El agente se entera del token nuevo en la RESPUESTA de un latido, o sea DESPUÉS de haber usado
// el viejo para autenticarse. Por eso los dos valen a la vez durante la ventana: si el viejo
// dejara de valer en el instante de emitir el nuevo, el agente quedaría afuera entre que lo
// recibe y lo guarda — y si ese guardado falla (disco lleno, permisos), para siempre.
//
// La rotación se COMPLETA cuando llega el primer latido con el token nuevo: eso es la prueba de
// que el agente lo guardó y lo puede usar. Ahí el viejo se borra, que es el punto de rotar.
// ════════════════════════════════════════════════════════════════════════════════════════════

// AbrirRotacion emite un token nuevo para un device y deja los dos válidos hasta `vence`.
//
// Devuelve el token EN CLARO una sola vez, igual que el alta: el cerebro guarda el hash y no
// tiene forma de volver a decirlo.
//
// OJO — QUIEN LLAMA TIENE QUE RECORDARLO EN MEMORIA (McpServer.recordarRotacion) o la rotación
// queda abierta y NO SE PUEDE ENTREGAR: el latido saca el token del mapa en memoria, no de acá.
// Es un acoplamiento real y se dice acá porque no hay forma de que el compilador lo ate: la
// alternativa era guardar el token en claro en la fila, que es justo lo que no se puede.
func (e *DbEngine) AbrirRotacion(deviceID string, vence time.Time) (string, error) {
	d, existe, err := e.DevicePorID(deviceID)
	if err != nil {
		return "", err
	}
	if !existe || d.Revoked {
		return "", fmt.Errorf("no hay una máquina activa con ese identificador")
	}
	// UN DEVICE SIN TOKEN NO TIENE NADA QUE ROTAR. Se pregunta por la COLUMNA y no por el tier:
	// «Tier B» es la razón habitual de no tener credencial, pero la verdad de si la tiene está en
	// la fila, y derivarla del tier sería creerle a un proxy en vez de al dato. Emitirle un token
	// a algo que no lo usa crearía una credencial que nadie va a usar nunca.
	var actual string
	if err := e.db.QueryRow(`SELECT token_sha256 FROM devices WHERE id = ?`, deviceID).Scan(&actual); err != nil {
		return "", fmt.Errorf("error al leer la credencial de %q: %w", deviceID, err)
	}
	if actual == "" {
		return "", fmt.Errorf("esa máquina no se autentica con un token de dispositivo (los Tier B los sondea el cerebro por su protocolo), así que no hay nada que rotar")
	}
	nuevo, err := fleet.NuevoToken()
	if err != nil {
		return "", err
	}
	if _, err := e.db.Exec(
		`UPDATE devices SET token_sha256_nuevo = ?, rotacion_vence = ? WHERE id = ? AND revoked = 0`,
		fleet.HashToken(nuevo), vence.UTC().Format(time.RFC3339), deviceID,
	); err != nil {
		return "", fmt.Errorf("error al abrir la rotación de %q: %w", deviceID, err)
	}
	return nuevo, nil
}

// CompletarRotacion se llama cuando un device se autenticó CON EL TOKEN NUEVO: promueve el nuevo
// a definitivo y borra el viejo, que es el punto entero de rotar.
//
// Idempotente: si ya se completó, no hay `token_sha256_nuevo` y el UPDATE no toca nada.
func (e *DbEngine) CompletarRotacion(deviceID string) error {
	if _, err := e.db.Exec(
		`UPDATE devices
		    SET token_sha256 = token_sha256_nuevo, token_sha256_nuevo = '', rotacion_vence = ''
		  WHERE id = ? AND token_sha256_nuevo <> ''`, deviceID); err != nil {
		return fmt.Errorf("error al completar la rotación de %q: %w", deviceID, err)
	}
	return nil
}

// AbandonarRotacionesVencidas descarta las rotaciones que nadie completó a tiempo.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// SE ABANDONA EL NUEVO Y SIGUE EL VIEJO, Y ES AL REVÉS DE LO QUE DECÍA EL PLAN
//
// El plan de la Ola 2 decía «vencido el plazo sin confirmar, se borra el previo igual
// (fail-closed)». Eso convierte una operación de higiene en un apagón: el agente que nunca
// levantó el token nuevo se queda sin ninguno válido, y la máquina más difícil de arreglar es
// justamente la que no está latiendo.
//
// El razonamiento correcto es cuál de los dos errores es peor, y depende de PARA QUÉ se rota:
//
//	· Rotar es HIGIENE — «esta credencial ya tiene un año». Abandonarla deja el token viejo
//	  vivo, o sea el estado que ya había: no empeora nada.
//	· Si el token SE FILTRÓ, lo que corresponde no es rotar sino REVOCAR, que ya existe, es
//	  instantáneo y no depende de que el agente coopere.
//
// O sea que la rotación nunca es la herramienta de la emergencia, y hacerla fail-closed le pone
// el costo de la emergencia a la operación de rutina. El plazo vencido se ANOTA (devuelve cuántas
// abandonó) para que no sea un silencio.
func (e *DbEngine) AbandonarRotacionesVencidas(ahora time.Time) (int64, error) {
	res, err := e.db.Exec(
		`UPDATE devices SET token_sha256_nuevo = '', rotacion_vence = ''
		  WHERE token_sha256_nuevo <> '' AND rotacion_vence <> '' AND rotacion_vence <= ?`,
		ahora.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("error al abandonar rotaciones vencidas: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error al leer el resultado del abandono: %w", err)
	}
	return n, nil
}

// DevicePorTokenConRotacion resuelve un device por CUALQUIERA de sus dos tokens válidos y dice si
// el que se usó fue el nuevo.
//
// El llamador necesita las dos cosas: la identidad para atender el latido, y `esNuevo` para saber
// si tiene que completar la rotación. Devolverlas juntas evita una segunda consulta y, sobre
// todo, evita que alguien deduzca `esNuevo` comparando hashes en otro lado — que es donde se
// separan las dos mitades de una condición.
func (e *DbEngine) DevicePorTokenConRotacion(token string) (d fleet.Device, esNuevo bool, existe bool, err error) {
	if strings.TrimSpace(token) == "" {
		return fleet.Device{}, false, false, nil
	}
	// Primero el camino de siempre: el token vigente.
	d, existe, err = e.DevicePorToken(token)
	if err != nil || existe {
		return d, false, existe, err
	}
	// Y sólo si no fue ése, el nuevo de una rotación abierta. La cadena vacía NO matchea: la
	// guarda es la misma que en DevicePorToken y por el mismo motivo — todas las filas sin
	// rotación abierta comparten el valor vacío, y sin esto una petición sin credencial se
	// autenticaría como cualquiera de ellas.
	h := fleet.HashToken(token)
	row := e.db.QueryRow(
		`SELECT `+columnasDevice+` FROM devices WHERE token_sha256_nuevo = ? AND token_sha256_nuevo <> '' AND revoked = 0`, h)
	d, existe, err = escanearUnDevice(row)
	return d, existe, existe, err
}
