package memory

// toolledger.go es el LEDGER DE USO (F0 del track «Potencia medida»): la historia persistente de
// qué tools se invocaron, cuánto tardaron y cómo terminaron.
//
// Existe porque hasta acá Musubi no podía responder cuáles de sus tools se usan. El histograma
// por-tool de internal/mcp/observability.go vive en memoria y se resetea en cada reinicio,
// /metrics pide bearer, y el modo daemon (stdio, el 99% del uso) ni siquiera levanta HTTP donde
// exponerlo. Sin este dato, decidir dónde invertir potencia es opinar.
//
// LO QUE ESTE ARCHIVO NO HACE, y es deliberado: no guarda argumentos, ni resultados, ni mensajes
// de error. El esquema no tiene dónde ponerlos. `save_observation` recibe exactamente el contenido
// que el portero de privacidad existe para proteger, así que un ledger con los argumentos adentro
// sería una segunda copia de toda la memoria sensible sin ninguna de sus murallas.

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Outcomes del ledger. Taxonomía CERRADA y no texto libre: un mensaje de error puede arrastrar
// adentro el contenido que lo causó, que es justo lo que L1 prohíbe guardar.
const (
	OutcomeOK          = "ok"
	OutcomeError       = "error"        // el handler devolvió un RpcError
	OutcomeDeniedRole  = "denied_role"  // rechazada por rol antes de llegar al handler
	OutcomeDeniedQuota = "denied_quota" // rechazada por cuota antes de llegar al handler
	OutcomePanic       = "panic"        // el handler entró en pánico
)

// validOutcome mantiene la taxonomía cerrada. Un valor fuera del conjunto se normaliza a "error"
// en vez de escribirse: el ledger nunca es motivo para fallar nada (L2), pero tampoco acepta que
// le metan texto arbitrario en una columna que se lee sin escrutinio.
func validOutcome(o string) string {
	switch o {
	case OutcomeOK, OutcomeError, OutcomeDeniedRole, OutcomeDeniedQuota, OutcomePanic:
		return o
	default:
		return OutcomeError
	}
}

// ToolInvocation es una llamada registrada. Los campos son los que se pueden mirar sin riesgo:
// QUÉ se llamó y CÓMO salió, nunca CON QUÉ.
type ToolInvocation struct {
	Tool      string
	Outcome   string
	Duration  time.Duration
	ProjectID string
	Principal string
}

// RecordToolInvocations escribe un LOTE de invocaciones en una sola transacción.
//
// Es por lote y no de a una porque el caller la llama desde una goroutine de flush: agrupar N
// inserts en una transacción convierte N fsync en uno. El caller NO debe tener tomado el lock de
// dispatch — esta función va directo a la base, y la concurrencia con las escrituras de las tools
// la resuelve SQLite con busy_timeout(5000) + WAL del DSN.
func (e *DbEngine) RecordToolInvocations(ctx context.Context, batch []ToolInvocation) error {
	if len(batch) == 0 {
		return nil
	}
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ledger de uso: abrir transacción: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO tool_invocations (tool, outcome, duration_us, project_id, principal)
		 VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("ledger de uso: preparar insert: %w", err)
	}
	defer stmt.Close()

	for _, inv := range batch {
		if _, err := stmt.ExecContext(ctx, inv.Tool, validOutcome(inv.Outcome),
			inv.Duration.Microseconds(), inv.ProjectID, inv.Principal); err != nil {
			return fmt.Errorf("ledger de uso: insertar %q: %w", inv.Tool, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("ledger de uso: commit: %w", err)
	}
	return nil
}

// ToolUsageRow es el resumen por tool que se le devuelve a quien pregunta.
type ToolUsageRow struct {
	Tool       string  `json:"tool"`
	Calls      int     `json:"calls"`
	Errors     int     `json:"errors"`
	Denied     int     `json:"denied"`
	AvgMillis  float64 `json:"avg_ms"`
	P95Millis  float64 `json:"p95_ms"`
	MaxMillis  float64 `json:"max_ms"`
	LastUsedAt string  `json:"last_used_at"`
}

// ToolUsage devuelve el uso agregado por tool en los últimos `days` días, de más usada a menos.
//
// ACOTADA AL PROYECTO DEL PRINCIPAL (Track 17). La tabla tiene project_id, así que sin el scope
// un miembro del cerebro central vería el patrón de uso de OTRO proyecto — qué herramientas usa
// otro equipo y con qué frecuencia. El barrido de aislamiento del repo lo cazó antes de que
// llegara a main, que es exactamente para lo que existe.
//
// La p95 se calcula en SQL sobre los datos crudos y no con un histograma en la base: los
// duration_us están guardados uno por uno, así que el percentil sale exacto en vez de aproximado
// por buckets. Es más caro, pero esta consulta la corre una persona mirando un panel, no el
// camino caliente.
func (e *DbEngine) ToolUsage(ctx context.Context, days int) ([]ToolUsageRow, error) {
	if days <= 0 {
		days = 30
	}
	desde := fmt.Sprintf("-%d days", days)
	scopeSQL, scopeArgs := projectScopeFrom(ctx).scopeClause("")
	args := append([]interface{}{desde}, scopeArgs...)

	rows, err := e.db.QueryContext(ctx, `
		SELECT tool,
		       COUNT(*)                                                      AS calls,
		       SUM(CASE WHEN outcome = 'error' THEN 1 ELSE 0 END)            AS errors,
		       SUM(CASE WHEN outcome LIKE 'denied_%' THEN 1 ELSE 0 END)      AS denied,
		       AVG(duration_us)                                              AS avg_us,
		       MAX(duration_us)                                              AS max_us,
		       MAX(created_at)                                               AS last_used
		FROM tool_invocations
		WHERE created_at >= datetime('now', ?)`+scopeSQL+`
		GROUP BY tool
		ORDER BY calls DESC, tool ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("ledger de uso: consultar: %w", err)
	}
	defer rows.Close()

	out := []ToolUsageRow{}
	for rows.Next() {
		var r ToolUsageRow
		var avgUs, maxUs float64
		if err := rows.Scan(&r.Tool, &r.Calls, &r.Errors, &r.Denied, &avgUs, &maxUs, &r.LastUsedAt); err != nil {
			return nil, fmt.Errorf("ledger de uso: escanear fila: %w", err)
		}
		r.AvgMillis = redondearMillis(avgUs)
		r.MaxMillis = redondearMillis(maxUs)
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ledger de uso: iterar: %w", err)
	}

	// La p95 va en una segunda pasada por tool. Se podría hacer con window functions, pero
	// SQLite las soporta recién desde 3.25 y este camino es explícito y fácil de leer.
	for i := range out {
		p95, err := e.percentilTool(ctx, out[i].Tool, desde, 0.95, scopeSQL, scopeArgs)
		if err != nil {
			return nil, err
		}
		out[i].P95Millis = p95
	}
	return out, nil
}

// percentilTool devuelve el percentil q de la duración de una tool, en milisegundos.
// Usa el método del rango más cercano: ordena y saltea ceil(q*N)-1 filas.
//
// Recibe el scope YA RESUELTO del caller en vez de re-derivarlo del ctx. Es a propósito: si esta
// función lo calculara por su cuenta, un cambio en el scope entre la consulta de arriba y ésta
// daría un percentil de un conjunto distinto al que se contó — y el bug sería invisible.
func (e *DbEngine) percentilTool(ctx context.Context, tool, desde string, q float64, scopeSQL string, scopeArgs []interface{}) (float64, error) {
	argsBase := append([]interface{}{tool, desde}, scopeArgs...)

	var n int
	if err := e.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM tool_invocations WHERE tool = ? AND created_at >= datetime('now', ?)`+scopeSQL,
		argsBase...).Scan(&n); err != nil {
		return 0, fmt.Errorf("ledger de uso: contar para percentil: %w", err)
	}
	if n == 0 {
		return 0, nil
	}
	offset := int(float64(n)*q+0.9999) - 1 // ceil(q*n) - 1
	if offset < 0 {
		offset = 0
	}
	if offset > n-1 {
		offset = n - 1
	}
	var us float64
	if err := e.db.QueryRowContext(ctx,
		`SELECT duration_us FROM tool_invocations
		 WHERE tool = ? AND created_at >= datetime('now', ?)`+scopeSQL+`
		 ORDER BY duration_us ASC LIMIT 1 OFFSET ?`,
		append(argsBase, offset)...).Scan(&us); err != nil {
		return 0, fmt.Errorf("ledger de uso: percentil de %q: %w", tool, err)
	}
	return redondearMillis(us), nil
}

// PurgeToolInvocations borra las invocaciones más viejas que `retentionDays` y devuelve cuántas.
// Cuelga del mantenimiento que ya existe: una tabla de telemetría sin techo termina siendo el
// problema que vino a diagnosticar.
func (e *DbEngine) PurgeToolInvocations(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}
	res, err := e.db.ExecContext(ctx,
		`DELETE FROM tool_invocations WHERE created_at < datetime('now', ?)`,
		fmt.Sprintf("-%d days", retentionDays))
	if err != nil {
		return 0, fmt.Errorf("ledger de uso: purgar: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, nil // el driver no lo soporta; el borrado igual ocurrió
	}
	return n, nil
}

// redondearMillis pasa microsegundos a milisegundos con tres decimales, para que una tool de
// 300 µs no se muestre como 0 ms.
func redondearMillis(us float64) float64 {
	ms := us / 1000
	return float64(int64(ms*1000+0.5)) / 1000
}

// FormatToolUsage arma la tabla de texto que ve una persona. Vive acá y no en el handler MCP
// porque el cuerpo va a querer exactamente el mismo formato en su panel.
func FormatToolUsage(rows []ToolUsageRow, days int) string {
	if len(rows) == 0 {
		return fmt.Sprintf("Sin invocaciones registradas en los últimos %d días.", days)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Uso de herramientas · últimos %d días · %d tools con actividad\n\n", days, len(rows))
	fmt.Fprintf(&b, "%-28s %8s %8s %8s %10s %10s\n", "TOOL", "LLAMADAS", "ERRORES", "RECHAZ", "MEDIA ms", "P95 ms")
	total := 0
	for _, r := range rows {
		total += r.Calls
		fmt.Fprintf(&b, "%-28s %8d %8d %8d %10.1f %10.1f\n",
			r.Tool, r.Calls, r.Errors, r.Denied, r.AvgMillis, r.P95Millis)
	}
	fmt.Fprintf(&b, "\nTotal: %d invocaciones.\n", total)
	return b.String()
}
