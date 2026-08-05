package memory

import (
	"context"
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
// `quarantined = 0` se agregó en F4 y es EL punto donde se cumple la Muralla 2: nueve
// consultas concatenan esta const (recall vectorial y léxico, prime, context, conflicts,
// consolidate, opstats, backfill de embeddings y la cola del sync saliente), así que una
// observación en cuarentena queda fuera de TODOS los caminos de recuperación con un solo
// cambio — y de paso no puede viajar al cerebro central.
//
// Escribir el filtro acá y no en cada query es deliberado: la única forma de que una
// consulta nueva se saltee la cuarentena es NO usar el predicado canónico, y eso es una
// desviación visible en el diff. Repetir el literal inline dejaría la muralla a merced de
// que el próximo que escriba una query se acuerde.
//
// OJO: `promote` (local → shared) NO pasa por acá. Se bloquea aparte, en
// PromoteObservationCtx, y tiene test propio.
const visibleObsPredicate = "archived = 0 AND superseded_by IS NULL AND quarantined = 0"

// ProjectScope acota una lectura a un proyecto para el AISLAMIENTO multi-tenant (Track 17).
// Federate (o ProjectID vacío) ⇒ SIN filtro: comportamiento federado histórico (stdio local,
// bearer legacy, admin). Deriva de la CREDENCIAL en el MCP (recallScopeFor), nunca del cliente.
type ProjectScope struct {
	ProjectID string
	Federate  bool
}

type projectScopeKey struct{}

// WithProjectScope adjunta un scope de proyecto al contexto. El MCP lo inyecta tras derivarlo
// del principal; las queries de lectura del engine lo aplican vía projectScopeFrom. Viaja por
// el ctx (no por la firma) para no cambiar el contrato StorageBackend ni sus ~30 callers.
func WithProjectScope(ctx context.Context, sc ProjectScope) context.Context {
	return context.WithValue(ctx, projectScopeKey{}, sc)
}

// projectScopeFrom recupera el scope del contexto. AUSENTE ⇒ ProjectScope{} = federado (sin
// filtro): así un caller que no inyecta scope (tests, recall pool, stdio) conserva el
// comportamiento previo bit-a-bit.
func projectScopeFrom(ctx context.Context) ProjectScope {
	if sc, ok := ctx.Value(projectScopeKey{}).(ProjectScope); ok {
		return sc
	}
	return ProjectScope{}
}

// scopeClause devuelve el fragmento SQL (con su AND inicial) + args que acotan una query al
// proyecto pedido CONSERVANDO las filas sin atribuir (project_id NULL o ''), idéntico criterio
// que filterCandidatesByProject del recall. Federate o ProjectID vacío ⇒ ("", nil): no filtra.
// alias es el prefijo de la tabla observations (p.ej. "o"); vacío para columnas sin calificar.
func (sc ProjectScope) scopeClause(alias string) (string, []interface{}) {
	if sc.Federate || sc.ProjectID == "" {
		return "", nil
	}
	col := "project_id"
	if alias != "" {
		col = alias + ".project_id"
	}
	return fmt.Sprintf(" AND (%s = ? OR %s IS NULL OR %s = '')", col, col, col), []interface{}{sc.ProjectID}
}

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
	// Bump de sync_seq (auditoría #4): al pasar a 'shared' la obs se vuelve pulleable; si conservara su
	// sync_seq viejo, un cliente cuyo cursor ya lo pasó nunca la bajaría. Fresco ⇒ se re-entrega.
	if _, err := tx.Exec(
		`UPDATE observations SET sync_seq = (SELECT IFNULL(MAX(sync_seq),0) FROM observations) + 1 WHERE id = ?`, id,
	); err != nil {
		return fmt.Errorf("error al asignar sync_seq en promoción: %w", err)
	}
	// Encolar en el outbox dentro de la misma tx (ahora la obs ya es 'shared', así que el
	// INSERT..SELECT sí produce fila). Idempotente por obs_id. Apagado en el central (terminal).
	if e.outboxEnabled {
		if err := enqueueOutboxTx(tx, id); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("error al commitear promoción: %w", err)
	}
	return nil
}

// obsInScope verifica que la observación id pertenezca al proyecto del ctx. Federado o sin scope
// (admin / stdio local) ⇒ siempre ok. Si la fila NO existe, devuelve ok=true a propósito: dejá que el
// mutador de arriba devuelva "no encontrada" en vez de convertir esto en un ORÁCULO de tenant.
// AISLAMIENTO (auditoría 2026-07-26 #11).
func (e *DbEngine) obsInScope(ctx context.Context, id string) (bool, error) {
	sc := projectScopeFrom(ctx)
	if sc.Federate || sc.ProjectID == "" {
		return true, nil
	}
	var proj string
	err := e.db.QueryRowContext(ctx, `SELECT COALESCE(project_id,'') FROM observations WHERE id=?`, id).Scan(&proj)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return proj == sc.ProjectID, nil
}

// PromoteObservationCtx es PromoteObservation acotada al proyecto de la credencial: un principal
// acotado NO puede forzar a 'shared' (y al outbox central) la observación de OTRO tenant con sólo
// conocer su id. Auditoría 2026-07-26 #11.
func (e *DbEngine) PromoteObservationCtx(ctx context.Context, id string) error {
	ok, err := e.obsInScope(ctx, id)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: no se puede promover %s desde otro proyecto", ErrCrossTenant, id)
	}
	// Muralla 2 (F4), segunda vía. La primera —el outbox y toda consulta de recall— ya queda
	// tapada por `quarantined = 0` dentro de visibleObsPredicate, pero la promoción a 'shared'
	// NO pasa por ese predicado: es un UPDATE por id. Sin esta guarda, texto de un LLM sin
	// corroborar viajaría al cerebro central y aparecería como memoria de equipo en las
	// máquinas de otras personas — el daño que la muralla existe para evitar, multiplicado.
	quarantined, err := e.IsQuarantined(id)
	if err != nil {
		return err
	}
	if quarantined {
		return fmt.Errorf("%w: %s no puede promoverse a 'shared' sin corroborar primero", ErrQuarantined, id)
	}
	return e.PromoteObservation(id)
}
