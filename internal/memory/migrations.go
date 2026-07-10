package memory

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrSchemaTooNew se devuelve cuando la base fue migrada por un binario MÁS NUEVO
// (su user_version supera la última migración que este binario conoce). Es una guarda
// de compatibilidad hacia adelante fail-closed: preferible negarse a abrir que operar a
// ciegas sobre columnas/tablas desconocidas y arriesgar corrupción lógica en una flota
// mixta (laptop/PC/central con binarios de distinta versión).
var ErrSchemaTooNew = errors.New("el esquema de la base es más nuevo que este binario")

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
		{
			version: 8,
			name:    "work_bids",
			// Contract-Net bidding en la pizarra multi-agente: sin esto las unidades se
			// asignan solo por claim de orden de llegada (first-come). work_bids registra las
			// OFERTAS de los agentes por unidad; el orquestador adjudica (award) a la mejor.
			// UNIQUE(unit_id, agent): una oferta vigente por agente (re-bid la actualiza). FK
			// ON DELETE CASCADE: limpiar el batch borra sus ofertas. Aditiva.
			up: func(x execQuerier) error {
				if _, err := x.Exec(`
					CREATE TABLE IF NOT EXISTS work_bids (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						unit_id TEXT NOT NULL REFERENCES work_units(id) ON DELETE CASCADE,
						agent TEXT NOT NULL,
						bid REAL NOT NULL,
						note TEXT,
						created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
						UNIQUE(unit_id, agent)
					);`); err != nil {
					return err
				}
				_, err := x.Exec(`CREATE INDEX IF NOT EXISTS idx_work_bids_unit ON work_bids(unit_id)`)
				return err
			},
		},
		{
			version: 9,
			name:    "debate",
			// Debate topology (multi-agent debate / Society of Minds) como subsistema
			// model-free: sin esto el patrón solo existe como prosa en la skill
			// adversarial-review (sin persistencia del voto ni reproducibilidad). Tres tablas:
			//   debates          -> la sesión (topic, rondas, quórum, estado, ganador)
			//   debate_postures  -> N posturas atribuidas POR RONDA (crítica cruzada persistida);
			//                       UNIQUE(debate_id,round,agent) = una postura por agente y ronda
			//   debate_votes     -> voto por agente; UNIQUE(debate_id,agent) = un voto vigente
			// El tally (mayoría/quórum) es SQL COUNT determinista: Musubi cuenta, no razona. FK
			// ON DELETE CASCADE: borrar el debate limpia posturas y votos. Aditiva.
			up: func(x execQuerier) error {
				for _, ddl := range []string{
					`CREATE TABLE IF NOT EXISTS debates (
						id TEXT PRIMARY KEY,
						topic TEXT NOT NULL,
						rounds INTEGER NOT NULL,
						current_round INTEGER NOT NULL DEFAULT 1,
						quorum INTEGER NOT NULL DEFAULT 0,
						status TEXT NOT NULL DEFAULT 'open',
						winner TEXT,
						created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
						closed_at DATETIME
					);`,
					`CREATE TABLE IF NOT EXISTS debate_postures (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						debate_id TEXT NOT NULL REFERENCES debates(id) ON DELETE CASCADE,
						round INTEGER NOT NULL,
						agent TEXT NOT NULL,
						stance TEXT NOT NULL,
						created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
						UNIQUE(debate_id, round, agent)
					);`,
					`CREATE TABLE IF NOT EXISTS debate_votes (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						debate_id TEXT NOT NULL REFERENCES debates(id) ON DELETE CASCADE,
						agent TEXT NOT NULL,
						choice TEXT NOT NULL,
						created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
						UNIQUE(debate_id, agent)
					);`,
					`CREATE INDEX IF NOT EXISTS idx_debate_postures ON debate_postures(debate_id, round)`,
					`CREATE INDEX IF NOT EXISTS idx_debate_votes ON debate_votes(debate_id)`,
				} {
					if _, err := x.Exec(ddl); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			version: 10,
			name:    "observations_scope_project",
			// Fundación del CEREBRO HÍBRIDO local+central: sin esto una observación no sabe
			// si es privada del proyecto o compartible a la memoria central, ni de qué
			// proyecto proviene. Dos columnas aditivas:
			//   scope      -> 'local' (privada, default) | 'shared' (promovible al cerebro)
			//   project_id -> proyecto de origen (para atribución/filtrado en F2/F3/F4)
			// El índice acelera el filtrado por proyecto. Es ADITIVA y BACKWARD-COMPAT: scope
			// default 'local' + project_id NULL en las filas previas = comportamiento idéntico
			// al de antes (F1 no sincroniza ni filtra por scope todavía; eso llega en F2/F3/F4).
			up: func(x execQuerier) error {
				if _, err := x.Exec(`ALTER TABLE observations ADD COLUMN scope TEXT NOT NULL DEFAULT 'local'`); err != nil {
					return err
				}
				if _, err := x.Exec(`ALTER TABLE observations ADD COLUMN project_id TEXT`); err != nil {
					return err
				}
				_, err := x.Exec(`CREATE INDEX IF NOT EXISTS idx_obs_project ON observations(project_id)`)
				return err
			},
		},
		{
			version: 11,
			name:    "outbox",
			// Cerebro híbrido F2: OUTBOX DURABLE para el sync SALIENTE offline-first. Sin esto una
			// observación promovida a 'shared' no tiene forma de sincronizarse al cerebro central
			// que sobreviva a un crash o a un corte de red. El outbox es el patrón transaccional
			// canónico: encolar la INTENCIÓN de sincronizar en la MISMA tx que promueve/guarda a
			// 'shared', drenarla después con reintentos. NO copia el contenido —guarda sólo obs_id
			// + metadatos de entrega—; el payload se reconstruye con un JOIN a observations al
			// drenar (siempre entrega el contenido fresco, habilita re-sync). El estado
			// next_attempt_at cubre backoff (pending futuro), lease (claimed futuro) y
			// auto-recuperación (un claimed con lease vencido se re-reclama solo). enqueued_hash
			// guarda el content_hash al encolar para re-sincronizar sólo cuando el contenido
			// cambió. El índice (status, next_attempt_at) soporta el claim atómico. Aditiva: NO
			// toca observations.
			up: func(x execQuerier) error {
				if _, err := x.Exec(`
					CREATE TABLE IF NOT EXISTS outbox (
						id              INTEGER PRIMARY KEY AUTOINCREMENT,
						obs_id          TEXT NOT NULL,
						status          TEXT NOT NULL DEFAULT 'pending',
						enqueued_hash   TEXT,
						attempts        INTEGER NOT NULL DEFAULT 0,
						next_attempt_at DATETIME NOT NULL DEFAULT (datetime('now')),
						last_error      TEXT,
						created_at      DATETIME NOT NULL DEFAULT (datetime('now')),
						updated_at      DATETIME NOT NULL DEFAULT (datetime('now')),
						UNIQUE(obs_id)
					);`); err != nil {
					return err
				}
				_, err := x.Exec(`CREATE INDEX IF NOT EXISTS idx_outbox_claim ON outbox(status, next_attempt_at)`)
				return err
			},
		},
		{
			version: 12,
			name:    "embeddings_model_id",
			// Contrato de vector + PROCEDENCIA (Track 16 / Producible F2.2). Sin esto un vector
			// no sabía QUÉ modelo lo produjo, así que al cambiar de embedder los vectores viejos
			// (otra procedencia) se comparaban por coseno con los nuevos y CORROMPÍAN el recall
			// EN SILENCIO: misma dimensión pero semántica de otro espacio ⇒ similitudes basura que
			// se colaban al top. La única guarda previa era por dimensión (coseno falla si difieren
			// las dims), que no cubre "misma dim, distinto modelo". model_id estampa la procedencia
			// del vector; la REGLA DE HOMOGENEIDAD (comparar sólo vectores de igual procedencia)
			// vive en la búsqueda exacta. Aditiva y backward-compat: '' = procedencia desconocida
			// (vectores legacy y los de engines sin embedder nombrado); un engine con '' sólo
			// compara contra '', así que el comportamiento histórico no cambia.
			up: func(x execQuerier) error {
				_, err := x.Exec(`ALTER TABLE embeddings ADD COLUMN model_id TEXT NOT NULL DEFAULT ''`)
				return err
			},
		},
		{
			version: 13,
			name:    "code_memory_project_id",
			// Aislamiento multi-tenant de la memoria de código (Track 17). No es SOLO aislamiento:
			// con PRIMARY KEY(path), dos proyectos con el mismo path (p.ej. internal/auth.go)
			// colisionaban en el ON CONFLICT(path) y se PISABAN el gist entre sí — corrupción
			// cross-tenant. Se agrega project_id y la unicidad pasa a (path, project_id). SQLite no
			// soporta ALTER de PRIMARY KEY ⇒ rebuild de tabla. project_id es NOT NULL DEFAULT ''
			// (sentinel, NO nullable: SQLite trata cada NULL como distinto en UNIQUE, así que un
			// project_id nullable rompería la dedup del upsert). Las filas legacy quedan con ''.
			up: func(x execQuerier) error {
				stmts := []string{
					`CREATE TABLE code_memory_new (
						path TEXT NOT NULL,
						gist TEXT NOT NULL,
						symbols TEXT,
						fingerprint TEXT,
						tokens INTEGER NOT NULL DEFAULT 0,
						updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
						project_id TEXT NOT NULL DEFAULT '',
						UNIQUE(path, project_id)
					)`,
					`INSERT INTO code_memory_new (path, gist, symbols, fingerprint, tokens, updated_at, project_id)
						SELECT path, gist, symbols, fingerprint, tokens, updated_at, '' FROM code_memory`,
					`DROP TABLE code_memory`,
					`ALTER TABLE code_memory_new RENAME TO code_memory`,
				}
				for _, s := range stmts {
					if _, err := x.Exec(s); err != nil {
						return err
					}
				}
				return nil
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
	// Guarda de compatibilidad hacia adelante: si la base ya está en un esquema mayor
	// que el que este binario conoce, negarse (fail-closed) en vez de operar a ciegas.
	// Sin esto, un binario viejo abría una DB migrada por uno nuevo y el bucle de abajo
	// era un no-op silencioso, corriendo sobre columnas/tablas que no entiende.
	latest := 0
	for _, m := range migs {
		if m.version > latest {
			latest = m.version
		}
	}
	if current > latest {
		return fmt.Errorf("%w: la base está en el esquema v%d pero este binario solo llega a v%d; actualizá musubi", ErrSchemaTooNew, current, latest)
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
