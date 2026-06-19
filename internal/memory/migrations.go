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
