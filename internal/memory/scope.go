package memory

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"musubi/internal/redact"
)

// scope.go es la fundación del CEREBRO HÍBRIDO local+central (F1): el modelo de SCOPE de
// una observación (local/shared) y el predicado CENTRALIZADO de visibilidad. F1 es aditivo
// y backward-compat: NO sincroniza nada ni filtra por scope todavía (eso llega en F2/F3/F4);
// acá sólo se dejan el modelo y el seam listos.

// Scopes canónicos de una observación en la memoria híbrida.
//
//	local  -> privada del proyecto (default; comportamiento histórico)
//	shared -> promovida, candidata a sincronizarse con el cerebro central (F2+)
const (
	ScopeLocal  = "local"
	ScopeShared = "shared"
)

// visibleObsPredicate es el predicado CANÓNICO de "observación visible en el recall":
// no archivada y no reemplazada (superseded). Centralizarlo en una sola const deja el
// SEAM para F3 (cuando el filtrado por scope se sume acá) sin cambiar hoy el comportamiento:
// las queries de lectura lo concatenan en vez de repetir el literal inline. El SQL resultante
// es semánticamente idéntico al anterior (mismo predicado; a lo sumo cambia el prefijo de
// alias, que es redundante cuando `observations` es la única tabla con esas columnas).
const visibleObsPredicate = "archived = 0 AND superseded_by IS NULL"

// normalizeScope acota un scope al conjunto válido. Vacío o desconocido ⇒ 'local' (el
// default privado), de modo que la columna NOT NULL siempre reciba un valor sano y un
// scope ausente conserve el comportamiento previo.
func normalizeScope(scope string) string {
	if strings.ToLower(strings.TrimSpace(scope)) == ScopeShared {
		return ScopeShared
	}
	return ScopeLocal
}

// ValidScopeParam valida el parámetro `scope` que llega por el handler MCP: se aceptan
// "" (⇒ local por default), "local" y "shared"; cualquier otra cosa es un error de param.
func ValidScopeParam(scope string) bool {
	switch scope {
	case "", ScopeLocal, ScopeShared:
		return true
	default:
		return false
	}
}

// ErrObservationNotFound lo devuelve PromoteObservation cuando el id no existe.
var ErrObservationNotFound = errors.New("observación no encontrada")

// PromoteObservation marca una observación como 'shared' (candidata al cerebro central) y la
// encola en el OUTBOX en la MISMA transacción (F2): el cambio de scope y la intención de
// sincronizar son atómicos (nada de "shared sin encolar"; un rollback no deja outbox huérfano).
// Idempotente: promover una ya-shared es un no-op exitoso y el enqueue no duplica (ON CONFLICT
// por obs_id). Si el id no existe (0 filas afectadas) devuelve ErrObservationNotFound.
func (e *DbEngine) PromoteObservation(id string) error {
	tx, err := e.db.Begin()
	if err != nil {
		return fmt.Errorf("error al iniciar transacción de promoción: %w", err)
	}
	defer tx.Rollback()

	// Redacción de secretos al cruzar a 'shared' (C2): un secreto que vivía en una obs local no
	// debe viajar al cerebro compartido. Se lee el contenido, se redacta, y se reescribe junto con
	// gist/hash/tokens derivados del texto limpio. Idempotente: redactar algo ya redactado es no-op
	// (el hash no cambia ⇒ el outbox no re-encola).
	var content string
	if err := tx.QueryRow(`SELECT content FROM observations WHERE id=?`, id).Scan(&content); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: %s", ErrObservationNotFound, id)
		}
		return fmt.Errorf("error al leer observación a promover: %w", err)
	}
	clean, _ := redact.Redact(content)
	if _, err := tx.Exec(
		`UPDATE observations SET scope=?, content=?, gist=?, content_hash=?, tokens=? WHERE id=?`,
		ScopeShared, clean, Gist(clean, defaultGistMaxTokens), ContentHash(clean), EstimateTokens(clean), id,
	); err != nil {
		return fmt.Errorf("error al promover observación: %w", err)
	}
	// Encolar en el outbox dentro de la misma tx (ahora la obs ya es 'shared', así que el
	// INSERT..SELECT sí produce fila). Idempotente por obs_id.
	if err := enqueueOutboxTx(tx, id); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al commitear promoción: %w", err)
	}
	return nil
}
