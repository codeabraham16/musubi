package memory

import (
	"errors"
	"fmt"
	"strings"
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

// PromoteObservation marca una observación como 'shared' (candidata al cerebro central).
// Idempotente: promover una ya-shared es un no-op exitoso. Si el id no existe (0 filas
// afectadas) devuelve ErrObservationNotFound. F1 no sincroniza: sólo cambia el scope.
func (e *DbEngine) PromoteObservation(id string) error {
	res, err := e.db.Exec(`UPDATE observations SET scope=? WHERE id=?`, ScopeShared, id)
	if err != nil {
		return fmt.Errorf("error al promover observación: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("error al leer filas afectadas al promover: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("%w: %s", ErrObservationNotFound, id)
	}
	return nil
}
