package memory

import (
	"database/sql"
	"fmt"
)

// migrations.go implementa el versionado de esquema de Musubi sobre el PRAGMA
// user_version de SQLite (un entero en el header de la base). Antes el esquema se
// creaba ad-hoc con CREATE ... IF NOT EXISTS + ADD COLUMN hardcodeados: no había
// forma de aplicar un cambio NO aditivo (rename, cambio de tipo, tabla nueva con
// backfill) de manera ordenada y resumible. Ahora cada cambio de esquema es una
// `migration` numerada que se aplica una sola vez, en su propia transacción.
//
// Invariante: las migraciones son estructurales (DDL). Los backfills de datos que
// dependen de lógica de runtime (gist/tokens según la versión del estimador) NO van
// acá: siguen como pasos idempotentes post-migración en NewDbEngine, porque deben
// re-evaluarse cuando cambia el estimador, no una sola vez.

// execQuerier abstrae *sql.DB y *sql.Tx para que una misma rutina de esquema corra
// tanto dentro de una transacción (migración) como directamente sobre la conexión
// (doctor, idempotencia). Ambos tipos satisfacen esta interfaz sin adaptadores.
type execQuerier interface {
	Exec(query string, args ...any) (sql.Result, error)
	Query(query string, args ...any) (*sql.Rows, error)
}

// migration es un paso de esquema versionado. `version` debe ser estrictamente
// creciente y único; `up` aplica el cambio sobre la transacción de esa migración.
type migration struct {
	version int
	name    string
	up      func(execQuerier) error
}

// schemaMigrations devuelve las migraciones conocidas por este binario, en orden
// ascendente de versión. Para evolucionar el esquema: agregar una nueva entrada con
// la siguiente versión (nunca editar ni reordenar las ya publicadas).
func schemaMigrations() []migration {
	return []migration{
		{
			version: 1,
			name:    "baseline",
			// Baseline = el esquema histórico completo (tablas/índices/triggers) +
			// las columnas de eficiencia de memoria. Todo es IF NOT EXISTS / ADD COLUMN
			// guardado, así que correrla sobre una base preexistente (v0.14, user_version=0)
			// es un no-op estructural: solo avanza user_version a 1.
			up: func(x execQuerier) error {
				if err := initSchemaOn(x); err != nil {
					return err
				}
				return addObservationColumns(x)
			},
		},
		{
			version: 2,
			name:    "idx_obs_archived",
			// Índice por `archived`: acelera la purga de retención (WHERE archived=1)
			// y el scan del olvido (WHERE archived=0). Primera migración post-baseline:
			// alcanza también a bases ya migradas a v1 (que no re-ejecutan la baseline).
			up: func(x execQuerier) error {
				_, err := x.Exec(`CREATE INDEX IF NOT EXISTS idx_obs_archived ON observations(archived)`)
				return err
			},
		},
		{
			version: 3,
			name:    "archived_at",
			// Columna archived_at: marca CUÁNDO se archivó una observación, para que la
			// purga de retención cuente la ventana DESDE el archivado (período de gracia
			// real) y no desde el último acceso. Backfill de las ya archivadas con su
			// último uso, para no cambiar su elegibilidad de purga retroactivamente.
			up: func(x execQuerier) error {
				if _, err := x.Exec(`ALTER TABLE observations ADD COLUMN archived_at DATETIME`); err != nil {
					return err
				}
				_, err := x.Exec(`UPDATE observations SET archived_at = COALESCE(last_accessed, created_at) WHERE archived = 1 AND archived_at IS NULL`)
				return err
			},
		},
		{
			version: 4,
			name:    "work_lease_ttl",
			// Lease/TTL para claims huérfanos en la pizarra: sin esto, una unidad que un
			// agente reclama y luego abandona (crash/timeout) queda 'claimed' para siempre
			// y ningún otro agente puede retomarla (bug de liveness). Columnas aditivas:
			//   owner_id         -> dueño canónico del lease (alias nuevo de claimed_by)
			//   lease_expires_at -> vencimiento del lease; NULL = sin lease (unidad vieja)
			//   heartbeat_at     -> última renovación
			//   attempts         -> reclamos acumulados (para dead-letter)
			//   fencing_token    -> token monótono anti-zombie
			// El índice (status, lease_expires_at) soporta el subselect del reclamo lazy.
			up: func(x execQuerier) error {
				for _, ddl := range []string{
					`ALTER TABLE work_units ADD COLUMN owner_id TEXT`,
					`ALTER TABLE work_units ADD COLUMN lease_expires_at DATETIME`,
					`ALTER TABLE work_units ADD COLUMN heartbeat_at DATETIME`,
					`ALTER TABLE work_units ADD COLUMN attempts INTEGER NOT NULL DEFAULT 0`,
					`ALTER TABLE work_units ADD COLUMN fencing_token INTEGER NOT NULL DEFAULT 0`,
					`CREATE INDEX IF NOT EXISTS idx_work_lease ON work_units(status, lease_expires_at)`,
				} {
					if _, err := x.Exec(ddl); err != nil {
						return err
					}
				}
				// Backfill: las unidades ya reclamadas bajo el esquema viejo tienen
				// claimed_by pero owner_id NULL. Copiar claimed_by -> owner_id para que su
				// dueño pueda seguir completándolas tras el upgrade (owner_id es la columna
				// canónica de propiedad). lease_expires_at queda NULL a propósito: se tratan
				// como no-huérfanas (no se expropia trabajo en curso durante la migración).
				_, err := x.Exec(`UPDATE work_units SET owner_id=claimed_by WHERE owner_id IS NULL AND claimed_by IS NOT NULL`)
				return err
			},
		},
		{
			version: 5,
			name:    "relations_bitemporal",
			// Modelo bi-temporal del grafo de hechos: sin esto, save_fact solo ACUMULA
			// tripletas y nunca retira ninguna, así que (Ana,trabaja_en,Acme) y
			// (Ana,trabaja_en,Globex) conviven como si ambas fueran verdad. Columnas:
			//   valid_from / valid_to    -> tiempo del EVENTO (desde/hasta cuándo es verdad)
			//   invalidated_at           -> tiempo de TRANSACCIÓN (cuándo dejó de ser vigente)
			//   superseded_by            -> id de la relación que la reemplazó
			// "Verdad actual" = invalidated_at IS NULL. Backfill: los hechos previos quedan
			// vigentes con valid_from = created_at. El índice acelera la búsqueda de hechos
			// vivos por (sujeto, predicado).
			up: func(x execQuerier) error {
				for _, ddl := range []string{
					`ALTER TABLE relations ADD COLUMN valid_from DATETIME`,
					`ALTER TABLE relations ADD COLUMN valid_to DATETIME`,
					`ALTER TABLE relations ADD COLUMN invalidated_at DATETIME`,
					`ALTER TABLE relations ADD COLUMN superseded_by INTEGER`,
					`CREATE INDEX IF NOT EXISTS idx_rel_live ON relations(from_id, predicate, invalidated_at)`,
				} {
					if _, err := x.Exec(ddl); err != nil {
						return err
					}
				}
				_, err := x.Exec(`UPDATE relations SET valid_from = created_at WHERE valid_from IS NULL`)
				return err
			},
		},
		{
			version: 6,
			name:    "run_events_journal",
			// Journal append-only del motor de workflows: hasta ahora workflow_runs solo
			// guardaba un snapshot mutable, sin idempotencia (un complete repetido
			// sobrescribía) ni historia (no se podía auditar/exportar/replay). run_events
			// registra cada transición como un evento inmutable. UNIQUE(run_id, seq) da
			// orden total; UNIQUE(run_id, idempotency_key) da idempotencia (en SQLite,
			// múltiples idempotency_key NULL coexisten). Aditivo: no toca workflow_runs.
			up: func(x execQuerier) error {
				if _, err := x.Exec(`
					CREATE TABLE IF NOT EXISTS run_events (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						run_id TEXT NOT NULL,
						seq INTEGER NOT NULL,
						step_id TEXT,
						event_type TEXT NOT NULL,
						payload TEXT,
						idempotency_key TEXT,
						created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
						UNIQUE(run_id, seq),
						UNIQUE(run_id, idempotency_key)
					);`); err != nil {
					return err
				}
				_, err := x.Exec(`CREATE INDEX IF NOT EXISTS idx_run_events_run ON run_events(run_id, seq)`)
				return err
			},
		},
		{
			version: 7,
			name:    "observations_mem_type",
			// Tipo de memoria (semantic/episodic/procedural, estilo LangMem): sin esto todas
			// las observaciones se olvidan con la misma curva. mem_type es un enum model-free
			// que el agente declara al guardar y que modula la saliencia del olvido (episódico
			// se enfría antes; procedural persiste). Aditiva: NULL = sin tipo = peso 1.0, así
			// que las observaciones previas decaen EXACTAMENTE como antes (backward-compat).
			up: func(x execQuerier) error {
				_, err := x.Exec(`ALTER TABLE observations ADD COLUMN mem_type TEXT`)
				return err
			},
		},
	}
}

// latestSchemaVersion es la versión a la que apunta este binario (la mayor migración).
func latestSchemaVersion() int {
	ms := schemaMigrations()
	if len(ms) == 0 {
		return 0
	}
	return ms[len(ms)-1].version
}

// runMigrations aplica al esquema activo las migraciones que falten, según el
// PRAGMA user_version de la base.
func runMigrations(db *sql.DB) error {
	return applyMigrations(db, schemaMigrations())
}

// applyMigrations es el runner: lee user_version y aplica, en orden, cada migración
// con versión mayor a la actual. Cada migración corre en SU PROPIA transacción y
// fija user_version dentro de esa misma tx, de modo que aplicar la migración y
// avanzar la versión es atómico: si `up` falla, se hace rollback y la versión no
// avanza (la próxima apertura reintenta). Separar el runner de schemaMigrations()
// permite testearlo con migraciones sintéticas.
func applyMigrations(db *sql.DB, migs []migration) error {
	var current int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("error al leer user_version: %w", err)
	}
	for _, m := range migs {
		if m.version <= current {
			continue
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("error al iniciar tx de migración %d (%s): %w", m.version, m.name, err)
		}
		if err := m.up(tx); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migración %d (%s) falló: %w", m.version, m.name, err)
		}
		// user_version no admite parámetros enlazados; m.version es un int controlado
		// por nosotros (no hay inyección posible).
		if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, m.version)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("error al fijar user_version=%d: %w", m.version, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("error al commitear migración %d (%s): %w", m.version, m.name, err)
		}
		current = m.version
	}
	return nil
}

// schemaVersion devuelve el PRAGMA user_version de la base (la última migración
// aplicada). Útil para diagnóstico y tests.
func (e *DbEngine) schemaVersion() (int, error) {
	var v int
	if err := e.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return 0, fmt.Errorf("error al leer user_version: %w", err)
	}
	return v, nil
}
