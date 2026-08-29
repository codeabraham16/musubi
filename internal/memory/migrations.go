package memory

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
		{
			version: 14,
			name:    "relations_project_id",
			// Aislamiento multi-tenant del GRAFO DE HECHOS (Track 17). Como en code_memory (v13),
			// no es sólo fuga de lectura: con UNIQUE(from_id,predicate,to_id) el MISMO triple no
			// podía coexistir entre proyectos, y —peor— la invalidación por cardinalidad de un
			// predicado funcional cruzaba proyectos (un save en A cerraba la ventana de un hecho
			// vivo de B). Se agrega project_id y la unicidad pasa a (from_id,predicate,to_id,
			// project_id); la invalidación por cardinalidad se acota al proyecto de origen.
			//
			// relations tiene FKs a entities (ON DELETE CASCADE) ⇒ rebuild de tabla. Se PRESERVA el
			// id explícito porque superseded_by es una auto-referencia (relations.id → relations.id):
			// copiar con ids nuevos rompería esas referencias. Nada apunta a relations (superseded_by
			// es INTEGER plano, no FK declarada), así que el DROP+RENAME no arrastra referencias
			// ajenas. project_id es NOT NULL DEFAULT '' (sentinel, NO nullable: SQLite trata cada NULL
			// como distinto en UNIQUE y rompería la dedup del upsert). Las filas legacy quedan con ''
			// (espacio federado histórico, visible a cualquier proyecto). El índice idx_rel_live se
			// recrea porque se va con el DROP de la tabla vieja.
			up: func(x execQuerier) error {
				stmts := []string{
					// GUARD anti-brick (auditoría v0.98.0): el rebuild corre con foreign_keys=ON, así que
					// una relación LEGACY huérfana (from_id/to_id apuntando a una entidad inexistente —
					// posible en datos creados antes de que el CASCADE se aplicara) haría fallar el
					// INSERT..SELECT por la FK y ABORTARÍA la migración (rollback ⇒ NewDbEngine devuelve
					// error ⇒ la base "que funcionaba" NO abre con el binario nuevo). Barrer las huérfanas
					// ANTES del rebuild: son aristas a un nodo que ya no existe (no aportan al grafo).
					`DELETE FROM relations WHERE from_id NOT IN (SELECT id FROM entities) OR to_id NOT IN (SELECT id FROM entities)`,
					`CREATE TABLE relations_new (
						id INTEGER PRIMARY KEY AUTOINCREMENT,
						from_id INTEGER NOT NULL,
						predicate TEXT NOT NULL,
						to_id INTEGER NOT NULL,
						created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
						valid_from DATETIME,
						valid_to DATETIME,
						invalidated_at DATETIME,
						superseded_by INTEGER,
						project_id TEXT NOT NULL DEFAULT '',
						UNIQUE(from_id, predicate, to_id, project_id),
						FOREIGN KEY(from_id) REFERENCES entities(id) ON DELETE CASCADE,
						FOREIGN KEY(to_id) REFERENCES entities(id) ON DELETE CASCADE
					)`,
					`INSERT INTO relations_new
						(id, from_id, predicate, to_id, created_at, valid_from, valid_to, invalidated_at, superseded_by, project_id)
						SELECT id, from_id, predicate, to_id, created_at, valid_from, valid_to, invalidated_at, superseded_by, ''
						FROM relations`,
					`DROP TABLE relations`,
					`ALTER TABLE relations_new RENAME TO relations`,
					`CREATE INDEX IF NOT EXISTS idx_rel_live ON relations(from_id, predicate, invalidated_at)`,
				}
				for _, s := range stmts {
					if _, err := x.Exec(s); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			version: 15,
			name:    "telemetry_decisions_project_id",
			// Aislamiento multi-tenant del subsistema de TELEMETRÍA y DECISIONES (Track 18). La
			// auditoría de re-medición marcó que telemetry_logs y skill_decisions eran las dos
			// tablas de lectura SIN project_id: resolve_telemetry leía/persistía logs crudos de
			// cualquier proyecto y los hotspots/decisiones de insights sumaban entre tenants
			// (misma clase que el bleed cross-project de Track 17, pero en superficies no
			// enumeradas por la auditoría de cierre). A diferencia de code_memory (v13) y
			// relations (v14), acá NO hay PK/UNIQUE que cambiar ⇒ ADD COLUMN aditivo (como v10/v12),
			// sin rebuild. project_id es NOT NULL DEFAULT '' (sentinel, no nullable): las filas
			// legacy quedan en el espacio federado '' (visible a cualquier proyecto, histórico
			// bit-a-bit). Los índices aceleran el filtrado por proyecto.
			up: func(x execQuerier) error {
				for _, ddl := range []string{
					`ALTER TABLE telemetry_logs ADD COLUMN project_id TEXT NOT NULL DEFAULT ''`,
					`ALTER TABLE skill_decisions ADD COLUMN project_id TEXT NOT NULL DEFAULT ''`,
					`CREATE INDEX IF NOT EXISTS idx_telemetry_project ON telemetry_logs(project_id)`,
					`CREATE INDEX IF NOT EXISTS idx_skill_dec_project ON skill_decisions(project_id)`,
				} {
					if _, err := x.Exec(ddl); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			version: 16,
			name:    "observations_author",
			// Atribución por PERSONA (C5.1 del track captura-automatica de equipo). La memoria
			// compartida ya se atribuye al PROYECTO (project_id, v10) pero no a la persona que la
			// aportó: en un cerebro de equipo no se distingue lo que aprendió Ana de lo de Juan. author
			// se DERIVA de la credencial (principal.Name) en el write path y se SELLA en el central
			// (nunca del cliente). Como v15, NO hay PK/UNIQUE que cambiar ⇒ ADD COLUMN aditivo sin
			// rebuild. NOT NULL DEFAULT '' (sentinel): las filas legacy y las capturas sin principal
			// (stdio local) quedan sin atribución (vacío), comportamiento bit-a-bit al previo.
			up: func(x execQuerier) error {
				_, err := x.Exec(`ALTER TABLE observations ADD COLUMN author TEXT NOT NULL DEFAULT ''`)
				return err
			},
		},
		{
			version: 17,
			name:    "fts_external_content",
			// La FTS pasa de REGULAR (guardaba su propia copia del contenido) a EXTERNAL-CONTENT
			// (lee el contenido de `observations` por rowid). Elimina la duplicación del texto en
			// disco. Cambia el DDL, los 3 triggers (patrón external-content: el 'delete' toma los
			// valores viejos de old.*) y el join de las queries (por rowid, no por id).
			//
			// Dos casos, ambos terminan en 'rebuild':
			//   - FTS REGULAR (base pre-v17 con datos): se convierte — dropear triggers viejos +
			//     FTS regular, recrear external-content + triggers.
			//   - Ya EXTERNAL-CONTENT (base fresca: la baseline usa el DDL nuevo): no se toca el
			//     esquema.
			// El 'rebuild' corre SIEMPRE, y es CRÍTICO que así sea: en una base pre-FTS (muy vieja),
			// la baseline crea la FTS external-content VACÍA pero `observations` ya tiene filas que
			// no quedan indexadas; un UPDATE posterior dispararía el 'delete' external-content sobre
			// una entrada inexistente y CORROMPERÍA el índice ("database disk image is malformed").
			// 'rebuild' puebla esas filas desde la tabla base. En una base fresca (observations
			// vacío) es instantáneo.
			up: func(x execQuerier) error {
				var ddl string
				rows, err := x.Query(`SELECT COALESCE(sql,'') FROM sqlite_master WHERE type='table' AND name='observations_fts'`)
				if err != nil {
					return fmt.Errorf("fts_external_content: no se pudo leer el DDL de la FTS: %w", err)
				}
				if rows.Next() {
					if err := rows.Scan(&ddl); err != nil {
						rows.Close()
						return fmt.Errorf("fts_external_content: %w", err)
					}
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					return err
				}
				rows.Close()

				if !strings.Contains(ddl, "content=") {
					// Convertir la FTS regular a external-content.
					for _, stmt := range []string{
						`DROP TRIGGER IF EXISTS observations_ai`,
						`DROP TRIGGER IF EXISTS observations_ad`,
						`DROP TRIGGER IF EXISTS observations_au`,
						`DROP TABLE IF EXISTS observations_fts`,
						ftsTableDDL,
						ftsTriggerAI,
						ftsTriggerAD,
						ftsTriggerAU,
					} {
						if _, err := x.Exec(stmt); err != nil {
							return fmt.Errorf("fts_external_content: %w", err)
						}
					}
				}
				if _, err := x.Exec(`INSERT INTO observations_fts(observations_fts) VALUES('rebuild')`); err != nil {
					return fmt.Errorf("fts_external_content: rebuild: %w", err)
				}
				return nil
			},
		},
		{
			version: 18,
			name:    "code_graph",
			// GRAFO DE CÓDIGO derivado del AST (Track 20 · F1). Dos tablas nuevas —nodos y
			// aristas— scopeadas por project_id, con el mismo patrón de tenancy que code_memory
			// (v13) y relations (v14): project_id NOT NULL DEFAULT '' sentinel (SQLite trata cada
			// NULL como distinto en UNIQUE y rompería la dedup del upsert), legacy en '' = espacio
			// federado. Es ADITIVA: no toca ninguna tabla existente (patrón de run_events/outbox:
			// tabla nueva sólo en su migración, no en la baseline). El grafo NACE derivado del AST
			// y se persiste para poder FEDERARSE (el central no tiene el fuente) y servir consultas
			// baratas; cada fila lleva el src_fingerprint del archivo del que se derivó, de modo que
			// una desincronía se reporte STALE (comparando contra el fingerprint actual en la capa
			// MCP) en vez de mentir. La arista es PROPIEDAD de su src_path: el refresco borra por
			// src_path y reinserta, así el grafo nunca queda con aristas stale. Índices: (project_id,
			// path) para lectura scopeada y borrado por archivo de nodos; (project_id, from_key) y
			// (project_id, to_key) para el recorrido; (project_id, src_path) para el borrado de aristas.
			up: func(x execQuerier) error {
				for _, ddl := range []string{
					`CREATE TABLE IF NOT EXISTS code_graph_nodes (
						project_id      TEXT NOT NULL DEFAULT '',
						node_key        TEXT NOT NULL,
						kind            TEXT NOT NULL,
						name            TEXT NOT NULL,
						path            TEXT NOT NULL DEFAULT '',
						start_line      INTEGER NOT NULL DEFAULT 0,
						end_line        INTEGER NOT NULL DEFAULT 0,
						external        INTEGER NOT NULL DEFAULT 0,
						src_fingerprint TEXT NOT NULL DEFAULT '',
						updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
						UNIQUE(project_id, node_key)
					)`,
					`CREATE TABLE IF NOT EXISTS code_graph_edges (
						project_id      TEXT NOT NULL DEFAULT '',
						from_key        TEXT NOT NULL,
						to_key          TEXT NOT NULL,
						kind            TEXT NOT NULL,
						confidence      REAL NOT NULL DEFAULT 1.0,
						provenance      TEXT NOT NULL DEFAULT 'EXTRACTED',
						src_path        TEXT NOT NULL DEFAULT '',
						src_fingerprint TEXT NOT NULL DEFAULT '',
						updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
						UNIQUE(project_id, from_key, to_key, kind)
					)`,
					`CREATE INDEX IF NOT EXISTS idx_cg_nodes_scope ON code_graph_nodes(project_id, path)`,
					`CREATE INDEX IF NOT EXISTS idx_cg_edges_from ON code_graph_edges(project_id, from_key)`,
					`CREATE INDEX IF NOT EXISTS idx_cg_edges_to ON code_graph_edges(project_id, to_key)`,
					`CREATE INDEX IF NOT EXISTS idx_cg_edges_src ON code_graph_edges(project_id, src_path)`,
				} {
					if _, err := x.Exec(ddl); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			version: 19,
			name:    "sync_seq",
			// SECUENCIA DE SYNC MONÓTONA (auditoría 2026-07-26 #4). El pull entrante paginaba por
			// `rowid`, que NO cambia en un UPDATE (el UPSERT reescribe in-place) ⇒ las EDICIONES de una
			// obs shared ya sincronizada nunca se re-bajaban (mirror stale). Peor: `rowid` puede CAMBIAR
			// en un VACUUM (gotcha FTS external-content), corrompiendo el cursor. sync_seq es una columna
			// ESTABLE que se bumpea en cada insert/update de una obs (ver saveObservation) y sobrevive al
			// VACUUM, así que el cursor entrante pasa a ser por sync_seq: monótono y captura ediciones.
			// Backfill: sync_seq = rowid preserva el orden histórico para no re-bajar todo de golpe.
			up: func(x execQuerier) error {
				for _, ddl := range []string{
					`ALTER TABLE observations ADD COLUMN sync_seq INTEGER NOT NULL DEFAULT 0`,
					`UPDATE observations SET sync_seq = rowid`,
					`CREATE INDEX IF NOT EXISTS idx_obs_sync_seq ON observations(sync_seq)`,
				} {
					if _, err := x.Exec(ddl); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			version: 20,
			name:    "relations_source",
			// PROCEDENCIA DE ARISTAS (pilar Cognición · F0). El grafo de hechos DEBE poder
			// distinguir QUIÉN afirmó cada arista para auditarla, excluirla del baseline
			// model-free y revertirla: 'agent' (un caller humano/agente vía musubi_save_fact),
			// 'llm-extract:<model_id>' (extracción del pilar Cognición, F1+) o 'heuristic'. Sin
			// esta columna una arista derivada por un LLM sería indistinguible de una afirmada
			// por una persona y "no alucina" quedaría sin evidencia. ADITIVA (patrón v15/v16):
			// ADD COLUMN NOT NULL DEFAULT 'agent' ⇒ las filas legacy quedan atribuidas al agente
			// (bit-idéntico al previo). El índice sirve al filtrado por procedencia que el
			// read-time usará para EXCLUIR aristas no corroboradas (F1+).
			up: func(x execQuerier) error {
				for _, ddl := range []string{
					`ALTER TABLE relations ADD COLUMN source TEXT NOT NULL DEFAULT 'agent'`,
					`CREATE INDEX IF NOT EXISTS idx_rel_source ON relations(source)`,
				} {
					if _, err := x.Exec(ddl); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			version: 21,
			name:    "observation_origins",
			// ANCLAS AL ESTADO DEL PROYECTO. Una observación puede declarar de qué archivos
			// habla; se guarda el fingerprint de cada uno y el recall lo re-deriva del disco
			// para MARCAR la nota si cambió. Cierra el hueco que el detector de conflictos no
			// puede ver: compara observaciones ENTRE SÍ, así que nunca detecta una nota válida
			// con una línea vencida adentro.
			//
			// La tabla también está en initSchemaOn (baseline), pero eso SÓLO alcanza a bases
			// nuevas: una base ya migrada no re-ejecuta la baseline, y sin esta migración la
			// feature quedaría muerta en toda instalación existente — el check del doctor
			// fallando con "no such table". Es el mismo patrón que documenta la v2.
			up: func(x execQuerier) error {
				for _, ddl := range []string{
					`CREATE TABLE IF NOT EXISTS observation_origins (
						observation_id TEXT NOT NULL REFERENCES observations(id) ON DELETE CASCADE,
						path           TEXT NOT NULL,
						fingerprint    TEXT NOT NULL,
						captured_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
						PRIMARY KEY (observation_id, path)
					)`,
					`CREATE INDEX IF NOT EXISTS idx_obs_origins_path ON observation_origins(path)`,
				} {
					if _, err := x.Exec(ddl); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			version: 22,
			name:    "observation_provenance_quarantine",
			// CUARENTENA DE ESCRITURA Y PROCEDENCIA (Murallas 2+3 · F4). Hasta acá una
			// observación no decía de dónde salió su contenido. `author` existe pero es
			// otra cosa: la atribución por credencial del Track C5 (QUÉ persona o máquina
			// escribió), y un agente-LLM y una persona escriben con la misma credencial.
			//
			// Sin esto, la respuesta sintetizada por `musubi_ask` se podía guardar con
			// `musubi_save_observation` y quedaba en el libro mayor indistinguible de una
			// nota verificada a mano. Es el mismo agujero que la v20 cerró para el grafo
			// de hechos con `relations.source`, del lado del libro mayor.
			//
			// ADITIVA (patrón v15/v16/v20): ADD COLUMN NOT NULL DEFAULT rellena las filas
			// existentes sin una pasada de escritura, así que la base vieja queda
			// bit-idéntica en comportamiento (Q5) y sin filas sin sello (Q1).
			//
			// Las filas legacy quedan en 'human'. Es la mejor descripción disponible y no
			// una verdad verificada: no sabemos qué generó cada una, pero todas entraron
			// por un camino donde una persona o su agente eligió el contenido, y no había
			// motor de cognición escribiendo. Marcarlas 'unknown' agregaría ruido a cada
			// recall para no informar nada.
			//
			// SIN ÍNDICE sobre quarantined a propósito: casi todas las filas valen 0, así
			// que el planificador no usaría un índice de cardinalidad 2 y sólo costaría
			// escrituras. Viaja pegado a `archived = 0`, que tiene el mismo perfil y
			// tampoco lo tiene.
			//
			// DELEGA en addObservationColumns en vez de repetir la DDL, y no es cosmético:
			// las tres columnas están TAMBIÉN en la lista de esa función, que es la que arma
			// el esquema de una base nueva. Con la DDL escrita a mano acá, una base nueva la
			// recibía por la baseline y después esta migración la volvía a agregar
			// ⇒ "duplicate column name" y el engine no abría. ALTER TABLE ADD COLUMN no es
			// idempotente; addObservationColumns sí (consulta las columnas existentes antes
			// de tocar nada). Una sola fuente de verdad para la lista, y sirve para los dos
			// caminos: base nueva y base ya migrada.
			up: func(x execQuerier) error {
				return addObservationColumns(x)
			},
		},
		{
			version: 23,
			name:    "tool_invocations_ledger",
			// LEDGER DE USO (F0 del track «Potencia medida»). Hasta acá Musubi no podía
			// responder cuáles de sus tools se usan: el histograma por-tool de
			// observability.go vive en memoria y se resetea en cada reinicio, /metrics pide
			// bearer y el modo daemon (stdio, el 99% del uso) ni siquiera levanta HTTP.
			// La tabla `telemetry_logs` no ayuda: guarda errores de compilación.
			//
			// LO QUE NO TIENE ESTA TABLA ES LA MITAD DEL DISEÑO. No hay columna de
			// argumentos, ni de resultado, ni de mensaje de error: la fuga es imposible
			// porque no hay dónde escribirla. `save_observation` recibe exactamente el
			// contenido que el portero de privacidad existe para proteger, así que un
			// registro de invocaciones con los argumentos adentro sería una segunda copia
			// de toda la memoria sensible, sin ninguna de sus murallas.
			//
			// `outcome` es taxonomía CERRADA y no texto libre, por la misma razón que la
			// procedencia de la v22: un mensaje de error puede arrastrar adentro el
			// contenido que lo causó.
			//
			// Los dos índices son a propósito: las únicas consultas son "agrupá por tool" y
			// "ventana reciente / purgá lo viejo". Sin el de fecha, la purga hace scan
			// completo justo sobre la tabla que más crece.
			up: func(x execQuerier) error {
				stmts := []string{
					`CREATE TABLE IF NOT EXISTS tool_invocations (
						id          INTEGER PRIMARY KEY AUTOINCREMENT,
						tool        TEXT     NOT NULL,
						outcome     TEXT     NOT NULL,
						duration_us INTEGER  NOT NULL,
						project_id  TEXT     NOT NULL DEFAULT '',
						principal   TEXT     NOT NULL DEFAULT '',
						created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
					);`,
					`CREATE INDEX IF NOT EXISTS idx_tool_invocations_tool ON tool_invocations(tool);`,
					`CREATE INDEX IF NOT EXISTS idx_tool_invocations_ts ON tool_invocations(created_at);`,
				}
				for _, s := range stmts {
					if _, err := x.Exec(s); err != nil {
						return fmt.Errorf("v23 ledger de uso: %w", err)
					}
				}
				return nil
			},
		},
		{
			version: 24,
			name:    "skill_usage_counters",
			// EL ARSENAL SE MIDE (§7 del track «Forja global»). Hasta acá nadie podía decir qué
			// skill vale la pena: `skill_decisions` guarda «acepté o rechacé INSTALARLA», y
			// `tool_invocations` no guarda argumentos —a propósito, es una garantía de
			// privacidad— así que ni siquiera indirectamente se sabía qué skill se activó.
			//
			// SON CONTADORES Y NO UN LOG DE EVENTOS. Una resolución activa ~10 skills: un evento
			// por activación escribiría diez filas por llamada y traería el problema de retención
			// del ledger de tools. Y no hace falta: las preguntas de mantenimiento son «cuántas
			// veces» y «cuándo fue la última», no series de tiempo. Así la tabla queda acotada al
			// tamaño del arsenal, no crece con el uso, y no necesita purga.
			//
			// `evidence` Y `kind` SON TAXONOMÍAS CERRADAS, como `outcome` en la v23. Guardar la
			// evidencia por separado es lo que habilita la única lectura que no se puede adivinar
			// de otra forma: «esta skill matcheó SIEMPRE por comodín y sin embargo le piden el
			// cuerpo» — o sea, aplica de verdad y no tiene cómo decir cuándo.
			//
			// NO HAY COLUMNA DE UTILIDAD NI DE PUNTAJE. Lo que se puede medir sin un modelo es
			// activación y pedido; llamarle utilidad a eso sería opinión con un número al lado.
			up: func(x execQuerier) error {
				stmts := []string{
					`CREATE TABLE IF NOT EXISTS skill_usage (
						skill      TEXT     NOT NULL,
						project_id TEXT     NOT NULL DEFAULT '',
						evidence   TEXT     NOT NULL DEFAULT '',
						kind       TEXT     NOT NULL,
						n          INTEGER  NOT NULL DEFAULT 0,
						first_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
						last_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
						PRIMARY KEY (skill, project_id, evidence, kind)
					);`,
					`CREATE INDEX IF NOT EXISTS idx_skill_usage_project ON skill_usage(project_id);`,
				}
				for _, s := range stmts {
					if _, err := x.Exec(s); err != nil {
						return fmt.Errorf("v24 contadores de skills: %w", err)
					}
				}
				return nil
			},
		},
		{
			version: 25,
			name:    "work_claim_log",
			// LA ESCALADA TIENE QUE CONTAR QUÉ PASÓ. Cuando una unidad agota sus reintentos, el
			// dead-letter escribía un string FIJO: «lease agotado: superó el máximo de reintentos».
			// El humano que lo lee no sabe lo único que importa para decidir: si cinco agentes
			// distintos murieron al azar (infraestructura) o si el mismo agente murió cinco veces
			// a los treinta segundos (un cuelgue reproducible). Los dos casos producían el MISMO
			// mensaje, y el segundo es un bug esperando a que alguien lo mire.
			//
			// `claim_log` es append-only y de una línea por reclamo: `agente<TAB>instante`. Texto
			// plano y no JSON a propósito — se escribe con una concatenación en el mismo UPDATE
			// atómico del claim, sin leer-modificar-escribir, que es donde dos reclamos
			// concurrentes se pisarían. Se llena en ClaimWorkUnit y sólo se lee al dead-letterear.
			//
			// LO QUE NO GUARDA ES DELIBERADO, igual que en la v23: no hay campo de error ni de
			// resultado. El motivo de una falla es texto libre que viene del trabajo mismo, y esta
			// columna se lee en un mensaje de escalada que va a parar a un reporte; un motivo
			// libre acá sería una vía para que contenido sensible salga por un camino que no pasa
			// por el portero de privacidad. Con agente + marca de tiempo alcanza para distinguir
			// azar de patrón, que es la pregunta que la escalada tiene que responder.
			//
			// Aditiva y con default: una base vieja queda con '' y el dead-letter cae al mensaje
			// de siempre. Ninguna unidad en vuelo se rompe por migrar.
			up: func(x execQuerier) error {
				return agregarColumnaSiFalta(x, "work_units", "claim_log", `claim_log TEXT NOT NULL DEFAULT ''`)
			},
		},
		{
			version: 26,
			name:    "work_autonomy",
			// CUÁNTA AUTONOMÍA TIENE ESTA TAREA ERA UNA PREGUNTA QUE NADIE PODÍA HACER. El cerebro
			// sabía QUIÉN opera (el rol del token: reader/writer/admin) y eso es una propiedad de la
			// CREDENCIAL, no del trabajo. Un mismo agente, con el mismo token, puede tener encargado
			// «andá y mirá, no toques nada» en una unidad y «arreglalo solo» en la siguiente; la
			// pizarra no tenía dónde anotar esa diferencia, así que la única manera de sostenerla era
			// que el humano se acordara. Un encargo que sólo vive en la cabeza del que lo dio es
			// exactamente la deuda de intención: se paga cuando el agente hace de más y nadie puede
			// señalar la regla que rompió, porque no había regla.
			//
			// `autonomy` la escribe el que POSTEA la unidad y ya no cambia: L1 sólo reporta, L2
			// arregla pero necesita que otro apruebe, L3 cierra solo. El default es 'L3' porque es
			// exactamente lo que la pizarra hacía hasta hoy — una base vieja y un cliente que no sabe
			// del campo siguen comportándose igual.
			//
			// La terna `approved_*` es la contracara: la firma del revisor de L2. `approved_token`
			// guarda el fencing_token VIGENTE al aprobar, y ahí está el invariante que hace que la
			// firma valga algo — una aprobación aprueba EL INTENTO que se revisó, no la unidad para
			// siempre. Si al dueño le vence el lease y otro agente retoma la unidad, el
			// fencing_token avanza y la firma vieja deja de coincidir: el trabajo nuevo, que nadie
			// miró, no se cuela por la puerta que abrió el trabajo viejo.
			up: func(x execQuerier) error {
				cols := [][2]string{
					{"autonomy", `autonomy TEXT NOT NULL DEFAULT 'L3'`},
					{"approved_by", `approved_by TEXT NOT NULL DEFAULT ''`},
					{"approved_at", `approved_at DATETIME`},
					{"approved_token", `approved_token INTEGER NOT NULL DEFAULT 0`},
				}
				for _, c := range cols {
					if err := agregarColumnaSiFalta(x, "work_units", c[0], c[1]); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			version: 27,
			name:    "relation_signals_split",
			// `confidence` SIGNIFICABA DOS COSAS DISTINTAS SEGÚN LA FILA, y nadie podía notarlo desde
			// afuera. En una relación PENDIENTE era `max(léxico, coseno)`; en una auto-resuelta era el
			// léxico a secas. Un mismo 0,86 podía ser «comparten muchos trigramas» o «el coseno entre
			// dos documentos cualesquiera», que no es lo mismo ni parecido: la línea de base del coseno
			// documento-contra-documento medida en este repo da p50 0,60 y llega a 0,88 para pares SIN
			// ninguna relación. O sea que la mitad alta de la escala es ruido con forma de señal.
			//
			// Costó caro y de la peor manera: el 2026-08-11 se triaron los conflictos del cerebro
			// central por «confianza ≥ 0,85» creyendo que eso ordenaba por gravedad, cuando ordenaba
			// por parecido. Las dos señales se guardan ahora POR SEPARADO para que quien filtre sepa
			// por cuál está filtrando.
			//
			// NULL ES UN VALOR CON SIGNIFICADO ACÁ, y por eso las columnas son nullable en vez de
			// tener default 0: una fila anterior a esta migración no tiene las señales desglosadas y
			// no se pueden reconstruir sin volver a scorear los pares. `0` sería una mentira —un coseno
			// de 0 quiere decir «ortogonales», que es un dato— así que las viejas quedan en NULL, que
			// quiere decir «no se sabe». `confidence` no se toca: sigue siendo lo que siempre fue.
			up: func(x execQuerier) error {
				cols := [][2]string{
					{"lex_score", `lex_score REAL`},
					{"cosine_score", `cosine_score REAL`},
				}
				for _, c := range cols {
					if err := agregarColumnaSiFalta(x, "observation_relations", c[0], c[1]); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			version: 28,
			name:    "shadow_verdicts",
			// LA MESA DONDE EL MOTOR HABLA Y NADIE LE HACE CASO.
			//
			// El detector de conflictos decide model-free y esa decisión es la que vale. Esta tabla
			// guarda, al lado, lo que el motor de cognición habría dicho del MISMO par — y esa
			// segunda lectura SE DESCARTA. No es redundancia: es la única forma de saber si el
			// umbral model-free acierta, sin arriesgar que el LLM escriba en el libro mayor.
			//
			// POR QUÉ HACÍA FALTA. Los pisos de coseno se calibraron contra la DISTRIBUCIÓN de
			// pares al azar (77k medidos, p99 = 0,803): eso dice dónde está el ruido, no si el
			// veredicto acierta. Para lo segundo hacen falta pares ETIQUETADOS, y de ésos había 8.
			//
			// LA SEPARACIÓN ES ESTRUCTURAL, NO UNA PROMESA. Ninguna consulta del camino de decisión
			// lee esta tabla; no tiene FK que la ate a observation_relations en la dirección que
			// importa, y el worker que la escribe no tiene forma de tocar una relación. Si alguna
			// vez alguien quiere ascender un veredicto de acá, va a tener que escribir código nuevo
			// y visible, que es exactamente el punto.
			//
			// relation_id NO es una FK: la relación puede ser re-juzgada, fusionada o borrada, y la
			// evidencia de qué dijo cada lado ESE día no debería desaparecer con ella. Se guardan
			// también source_id/target_id y las señales del momento, para que la fila se explique
			// sola aunque el par ya no exista.
			up: func(x execQuerier) error {
				_, err := x.Exec(`
					CREATE TABLE IF NOT EXISTS shadow_verdicts (
						id             TEXT PRIMARY KEY,
						relation_id    TEXT NOT NULL,
						source_id      TEXT NOT NULL,
						target_id      TEXT NOT NULL,
						heur_relation  TEXT NOT NULL,
						heur_status    TEXT NOT NULL,
						lex_score      REAL,
						cosine_score   REAL,
						judge_relation TEXT NOT NULL,
						judge_raw      TEXT,
						judge_model    TEXT NOT NULL,
						agree          INTEGER NOT NULL,
						created_at     TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
					)`)
				if err != nil {
					return err
				}
				// El índice es por (heur_relation, agree): la consulta que motiva la tabla es
				// «de los supersedes auto-resueltos, ¿en cuántos el juez discrepó?».
				_, err = x.Exec(`CREATE INDEX IF NOT EXISTS idx_shadow_heur ON shadow_verdicts(heur_relation, agree)`)
				return err
			},
		},
		{
			version: 29,
			name:    "devices_registro_de_flota",
			// EL REGISTRO DE LA FLOTA (track «Control de flota», slice S1).
			//
			// Hasta acá una máquina existía en Musubi sólo como ORIGEN de sync de memoria: un
			// project_id y un nombre en un log. No había a-qué-máquina atribuir una métrica, un
			// comando ni una sesión de pantalla. Esta tabla es esa entidad.
			//
			// LO QUE NO TIENE, Y ES EL DISEÑO:
			//
			//   - NO hay columna `online`. El estado de conexión se DERIVA de last_seen con un
			//     umbral que elige quien pregunta (fleet.Device.EnLinea). Un booleano guardado se
			//     queda en `true` para siempre cuando la máquina muere de golpe — que es
			//     exactamente cuando querés saber que se cayó. Es la misma lección que la poda de
			//     procesos muertos del riel local.
			//   - NO se guarda el token crudo, sólo su SHA-256, igual que principals.yaml. Un
			//     volcado de esta tabla no entrega credenciales usables.
			//   - `revoked` es una BANDERA, no un DELETE. Borrar la fila perdería a quién
			//     pertenecían la telemetría y las sesiones ya ocurridas, que es justo lo que hace
			//     falta después de un incidente.
			//
			// project_id es NOT NULL y el alta lo exige no vacío (fleet.ValidarAlta). No es
			// ceremonia: ya está medido en este mismo cerebro que una fila sin atribuir se ve
			// desde TODOS los proyectos — pasó con 2 observaciones de test contaminando 3
			// proyectos. Un dispositivo sin dueño sería la misma fuga, con exec adosado.
			up: func(x execQuerier) error {
				_, err := x.Exec(`
					CREATE TABLE IF NOT EXISTS devices (
						id            TEXT PRIMARY KEY,
						name          TEXT NOT NULL,
						project_id    TEXT NOT NULL,
						tier          TEXT NOT NULL,
						caps          TEXT NOT NULL DEFAULT '',
						os            TEXT NOT NULL DEFAULT '',
						arch          TEXT NOT NULL DEFAULT '',
						address       TEXT NOT NULL DEFAULT '',
						agent_version TEXT NOT NULL DEFAULT '',
						tags          TEXT NOT NULL DEFAULT '',
						token_sha256  TEXT NOT NULL DEFAULT '',
						enrolled_at   TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
						last_seen     TEXT,
						revoked       INTEGER NOT NULL DEFAULT 0
					)`)
				if err != nil {
					return err
				}
				// El listado de la flota es siempre por proyecto y casi siempre sin los
				// revocados: ése es el índice que sirve a la consulta real.
				if _, err = x.Exec(`CREATE INDEX IF NOT EXISTS idx_devices_project ON devices(project_id, revoked)`); err != nil {
					return err
				}
				// UN TOKEN IDENTIFICA A UN DISPOSITIVO. Es único y lo impone la BASE, no el
				// código: dos máquinas compartiendo credencial hacen que la auditoría no pueda
				// distinguirlas, y una auditoría que no distingue no es auditoría. Parcial
				// porque un device de Tier B puede no tener credencial propia (se lo alcanza por
				// SSH/SNMP con las llaves del cerebro) y varios '' colisionarían.
				if _, err = x.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_token ON devices(token_sha256) WHERE token_sha256 <> ''`); err != nil {
					return err
				}
				// El nombre es la clave HUMANA dentro de un proyecto (la que se escribe en el
				// CLI y en una alerta). Dos «pc-gio» en el mismo proyecto harían ambiguo
				// cualquier comando dirigido por nombre.
				_, err = x.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_nombre ON devices(project_id, name)`)
				return err
			},
		},
		{
			version: 30,
			name:    "devices_ultima_muestra",
			// LA ÚLTIMA MUESTRA DE CADA MÁQUINA (track «Control de flota», S4).
			//
			// UNA COLUMNA, NO UNA TABLA DE SERIES. Es la decisión de diseño del slice y conviene
			// que quede acá: Musubi guarda el PRESENTE de la flota, no su historia. Una tabla de
			// muestras con 40 máquinas latiendo cada 30 s son 115.000 filas por día que nadie
			// consulta salvo para graficar — y graficar series es exactamente para lo que existe
			// Prometheus, que este repo ya despliega en deploy/prometheus/.
			//
			// Es la MISMA separación que el proyecto ya eligió una vez: el ledger de uso es la
			// HISTORIA (sobrevive al reinicio, se consulta con SQL) y el feed en vivo es el
			// PRESENTE. Son dos cosas distintas y conviene que no se mezclen. Acá igual.
			//
			// Se escribe en el MISMO UPDATE que ya estampa last_seen, así que la telemetría no
			// agrega ni una escritura: el latido ya tocaba la fila.
			up: func(x execQuerier) error {
				return agregarColumnaSiFalta(x, "devices", "last_sample", "last_sample TEXT NOT NULL DEFAULT ''")
			},
		},
		{
			version: 31,
			name:    "device_commands_bitacora",
			// LA BITÁCORA DE EJECUCIÓN REMOTA (track «Control de flota», S5).
			//
			// Es la tabla más sensible que tiene este esquema: guarda quién pidió correr qué, en
			// qué máquina ajena, y cómo salió. Tres decisiones de diseño viven acá:
			//
			// 1. LA FILA SE CREA AL ENCOLAR, NO AL TERMINAR. Si el cerebro se cae, si el agente
			//    nunca responde, si la máquina se apaga a mitad: el PEDIDO queda registrado
			//    igual. Una auditoría que sólo guarda lo que terminó bien no sirve para lo único
			//    que se le pide.
			//
			// 2. `principal` ES UNA COLUMNA, NO UNA FK. Tiene que sobrevivir a que esa persona
			//    sea dada de baja del registro de identidades: la pregunta «¿quién corrió esto?»
			//    se hace justamente después de que alguien se fue.
			//
			// 3. LA SALIDA VIVE EN LA MISMA FILA PERO NO TIENE LA MISMA VIDA. stdout puede traer
			//    secretos —una clave en un log, datos de un cliente— y se poda; el resto de la
			//    fila (quién, qué, cuándo, exit code) se conserva. Son dos retenciones sobre una
			//    tabla, y la poda las separa vaciando las columnas de salida sin borrar la fila.
			up: func(x execQuerier) error {
				_, err := x.Exec(`
					CREATE TABLE IF NOT EXISTS device_commands (
						id          TEXT PRIMARY KEY,
						device_id   TEXT NOT NULL,
						project_id  TEXT NOT NULL,
						principal   TEXT NOT NULL DEFAULT '',
						argv        TEXT NOT NULL,
						timeout_seg INTEGER NOT NULL,
						estado      TEXT NOT NULL,
						creado      TEXT NOT NULL,
						entregado   TEXT,
						terminado   TEXT,
						exit_code   INTEGER,
						stdout      TEXT NOT NULL DEFAULT '',
						stderr      TEXT NOT NULL DEFAULT '',
						error       TEXT NOT NULL DEFAULT ''
					)`)
				if err != nil {
					return err
				}
				// La consulta caliente: «dame lo pendiente de ESTA máquina, lo más viejo
				// primero». Corre en cada latido de cada máquina de la flota.
				if _, err = x.Exec(`CREATE INDEX IF NOT EXISTS idx_cmd_cola ON device_commands(device_id, estado, creado)`); err != nil {
					return err
				}
				// La consulta de la bitácora: por proyecto, lo más reciente primero.
				_, err = x.Exec(`CREATE INDEX IF NOT EXISTS idx_cmd_bitacora ON device_commands(project_id, creado DESC)`)
				return err
			},
		},
		{
			version: 32,
			name:    "screen_sessions_bitacora",
			// LA BITÁCORA DE SESIONES DE PANTALLA (S6).
			//
			// LO QUE ESTA TABLA NO TIENE ES SU RAZÓN DE SER: **no hay columna para la
			// contraseña**, ni en claro ni hasheada. La contraseña de una sesión se acuña, viaja
			// dos veces (a la máquina y a quien la pidió) y se descarta.
			//
			// Guardarla en claro convertiría esta tabla en un llavero de acceso a toda la flota:
			// un volcado y se tiene la pantalla de cada máquina. Hashearla no serviría de nada —
			// quien verifica la contraseña es RustDesk, no Musubi—, así que sería el costo sin el
			// beneficio.
			//
			// Lo que se guarda es que HUBO acceso: quién, a qué máquina, cuándo y hasta cuándo.
			// Eso es lo que se mira después de un incidente, y no sirve para entrar.
			//
			// `vence` se guarda pero el estado NO se recalcula por un barrido: se DERIVA al leer
			// (SesionPantalla.Vencida). Una columna de estado que alguien tiene que ir a
			// actualizar miente en cuanto nadie la actualiza.
			up: func(x execQuerier) error {
				_, err := x.Exec(`
					CREATE TABLE IF NOT EXISTS screen_sessions (
						id         TEXT PRIMARY KEY,
						device_id  TEXT NOT NULL,
						project_id TEXT NOT NULL,
						principal  TEXT NOT NULL DEFAULT '',
						estado     TEXT NOT NULL,
						creada     TEXT NOT NULL,
						vence      TEXT NOT NULL,
						cerrada    TEXT,
						error      TEXT NOT NULL DEFAULT ''
					)`)
				if err != nil {
					return err
				}
				if _, err = x.Exec(`CREATE INDEX IF NOT EXISTS idx_sesion_bitacora ON screen_sessions(project_id, creada DESC)`); err != nil {
					return err
				}
				// El ID de RustDesk de cada máquina: lo reporta el agente, y sin él quien mira no
				// sabe a qué conectarse. Es un identificador público del cliente, no un secreto.
				return agregarColumnaSiFalta(x, "devices", "rustdesk_id", "rustdesk_id TEXT NOT NULL DEFAULT ''")
			},
		},
		{
			version: 33,
			name:    "fleet_policy_state",
			// EL COOLDOWN DE LAS POLÍTICAS, QUE HASTA ACÁ VIVÍA SÓLO EN MEMORIA (S10b · A24).
			//
			// El cooldown es lo único que separa «una política que corrige algo» de «una tormenta
			// de comandos idénticos»: la métrica no baja hasta que el comando termine, así que sin
			// espera la política dispara en cada tick. Con el estado en memoria, un reinicio del
			// cerebro lo rearmaba entero — y el reinicio no es un evento raro justo cuando algo va
			// mal: es lo primero que alguien hace.
			//
			// El caso concreto que cierra: la política vacía un journal, el operador reinicia el
			// cerebro treinta segundos después para tocar otra cosa, y la política vuelve a
			// vaciarlo porque la muestra vieja todavía cruza el umbral. Dos acciones donde tenía
			// que haber una.
			//
			// LA CLAVE PRIMARIA ES (política, máquina) Y NO UN ID: el cooldown es por par, y una
			// tabla que permitiera dos filas para el mismo par tendría que decidir cuál gana. Con
			// la clave compuesta, el UPSERT es la operación entera.
			//
			// No se guarda el RESULTADO del comando —eso es la bitácora, que ya existe y es la
			// misma para lo automático y lo manual—. Acá sólo vive «cuándo se decidió actuar».
			up: func(x execQuerier) error {
				_, err := x.Exec(`
					CREATE TABLE IF NOT EXISTS fleet_policy_state (
						policy     TEXT NOT NULL,
						device_id  TEXT NOT NULL,
						last_fired TEXT NOT NULL,
						PRIMARY KEY (policy, device_id)
					)`)
				return err
			},
		},
		{
			version: 34,
			name:    "shell_sessions_bitacora",
			// LA BITÁCORA DE SESIONES DE SHELL INTERACTIVA (S5b).
			//
			// Es el registro más sensible del esquema, y por eso conviene decir qué NO tiene:
			// **no hay columna para el contenido de la sesión**. Ni lo tecleado ni lo impreso.
			// Eso es GRABACIÓN, y grabar lo que alguien escribe en una terminal es una decisión
			// legal antes que técnica — la misma que quedó sin dueño para las sesiones de
			// pantalla (A14). Lo que se guarda es que HUBO acceso: quién, dónde, cuándo, y por
			// cuánto tiempo.
			//
			// `ultimo_trafico` NO es cosmético: alimenta el techo de INACTIVIDAD, que es distinto
			// del techo de vida. Sin él, una terminal abierta en una pestaña que nadie mira es un
			// prompt vivo hasta que venza la vida máxima.
			//
			// Ni `vence` ni el vencimiento por inactividad se recalculan con un barrido: se
			// DERIVAN al leer (SesionShell.Vencida). Una columna de estado que alguien tiene que
			// ir a actualizar miente en cuanto nadie la actualiza.
			up: func(x execQuerier) error {
				_, err := x.Exec(`
					CREATE TABLE IF NOT EXISTS shell_sessions (
						id             TEXT PRIMARY KEY,
						device_id      TEXT NOT NULL,
						project_id     TEXT NOT NULL,
						principal      TEXT NOT NULL DEFAULT '',
						estado         TEXT NOT NULL,
						creada         TEXT NOT NULL,
						vence          TEXT NOT NULL,
						ultimo_trafico TEXT NOT NULL DEFAULT '',
						cerrada        TEXT,
						error          TEXT NOT NULL DEFAULT ''
					)`)
				if err != nil {
					return err
				}
				_, err = x.Exec(`CREATE INDEX IF NOT EXISTS idx_shell_bitacora ON shell_sessions(project_id, creada DESC)`)
				return err
			},
		},
		{
			version: 35,
			name:    "rustdesk_id_procedencia",
			// DE DÓNDE VIENE EL `rustdesk_id` DE UNA MÁQUINA (S6b · A13).
			//
			// Ese id lo REPORTA la propia máquina en su latido, así que es entrada no confiable:
			// una máquina comprometida puede declarar el id de otra y mandar a un operador a la
			// pantalla equivocada. No le da acceso a nada —la contraseña de sesión se aplicó en la
			// máquina que mintió— pero desorienta a alguien en el peor momento.
			//
			// Estas dos columnas guardan el CAMBIO, que es lo que no se puede derivar leyendo la
			// fila: cuándo se movió y cuál era el valor anterior. Un id que cambia solo tiene dos
			// explicaciones —se reinstaló la máquina, o alguien está mintiendo— y las dos ameritan
			// que quede escrito.
			//
			// La COLISIÓN (dos máquinas con el mismo id) NO se guarda: se DERIVA con una consulta,
			// como el «en línea». Una columna de colisión habría que ir a actualizarla en cada
			// alta y en cada latido de cualquier máquina, y el día que alguien olvide una ruta la
			// columna miente justo cuando importa.
			up: func(x execQuerier) error {
				if err := agregarColumnaSiFalta(x, "devices", "rustdesk_id_previo", "rustdesk_id_previo TEXT NOT NULL DEFAULT ''"); err != nil {
					return err
				}
				if err := agregarColumnaSiFalta(x, "devices", "rustdesk_id_cambiado", "rustdesk_id_cambiado TEXT NOT NULL DEFAULT ''"); err != nil {
					return err
				}
				// El índice es por el id REPORTADO: la consulta de colisión pregunta «¿quién más
				// dice ser esto?», y sin índice recorre la tabla entera en cada apertura de
				// pantalla y en cada listado.
				_, err := x.Exec(`CREATE INDEX IF NOT EXISTS idx_devices_rustdesk ON devices(rustdesk_id) WHERE rustdesk_id <> ''`)
				return err
			},
		},
		{
			version: 36,
			name:    "services_inventario_por_maquina",
			// QUÉ CORRE ADENTRO DE CADA MÁQUINA DE LA FLOTA (S12).
			//
			// ────────────────────────────────────────────────────────────────────────────────
			// CUÁL DE LAS DOS «FLOTAS» ES ÉSTA, porque en este mismo servidor hay dos (B17).
			//
			// La sección «Flota» del CRM inventaría BOTS, PUENTES Y SERVICIOS PUBLICADOS A MANO,
			// leídos de un archivo. Esta tabla es la OTRA: las máquinas de `devices` —que se miden
			// solas y latan— y las unidades que corren ADENTRO de ellas (una unit de systemd, un
			// servicio de Windows, un contenedor). Comparten el nombre y no comparten nada más.
			// Sin esta línea, alguien va a mirar una creyendo que es la otra.
			// ────────────────────────────────────────────────────────────────────────────────
			//
			// LO QUE ESTA TABLA NO TIENE, y las tres ausencias son el diseño:
			//
			//   - NINGUNA COLUMNA DE ESTADO (`healthy`, `up`, `activo`). El estado se DERIVA al
			//     leer, de `last_health` y de la EDAD de `last_report`. Es la misma lección que
			//     `devices` no tiene columna `online`: un booleano guardado se queda en `true`
			//     para siempre cuando la cosa muere de golpe, que es justo cuando querés saber
			//     que se cayó. Hay una prueba de FORMA que recorre el PRAGMA y lo custodia.
			//   - NINGUNA SERIE TEMPORAL. Se guarda el PRESENTE, igual que `devices.last_sample`.
			//     La historia la guarda Prometheus (decisión B5): 40 máquinas × 40 servicios cada
			//     30 s son millones de filas que nadie consulta salvo para graficar.
			//   - NINGUNA FOREIGN KEY a `devices`. No hay ni una en todo el repo y no hay
			//     `PRAGMA foreign_keys=ON` en el arranque, así que la primera sólo para esta
			//     tabla sería una inconsistencia peor que el hueco. La integridad se sostiene en
			//     el ALTA —se resuelve el device y de ÉL se copia el project_id— y en un escaneo
			//     tolerante, nunca en el esquema.
			//
			// El `project_id` va DENORMALIZADO en la fila y no por JOIN a `devices`, igual que en
			// `device_commands` y `screen_sessions`: el aislamiento por tenant no puede depender
			// de que la fila de la máquina siga existiendo con el mismo proyecto.
			up: func(x execQuerier) error {
				if _, err := x.Exec(`CREATE TABLE IF NOT EXISTS services (
						id            TEXT PRIMARY KEY,
						name          TEXT NOT NULL,
						project_id    TEXT NOT NULL,
						device_id     TEXT NOT NULL,
						kind          TEXT NOT NULL DEFAULT '',
						registered_at TEXT NOT NULL DEFAULT '',
						last_report   TEXT,
						last_health   TEXT NOT NULL DEFAULT '',
						revoked       INTEGER NOT NULL DEFAULT 0
					)`); err != nil {
					return err
				}
				if _, err := x.Exec(`CREATE INDEX IF NOT EXISTS idx_services_project ON services(project_id, revoked)`); err != nil {
					return err
				}
				if _, err := x.Exec(`CREATE INDEX IF NOT EXISTS idx_services_device ON services(device_id, revoked)`); err != nil {
					return err
				}
				// EL ÚNICO ES (project_id, device_id, name) Y NO (project_id, name).
				//
				// El nombre de un servicio sólo es único DENTRO de su máquina: dos hosts pueden
				// correr cada uno su `postgres` y son dos servicios distintos. Con el índice por
				// proyecto y nombre, el segundo host no podría registrar el suyo — y el síntoma
				// sería «el alta falla en la máquina nueva», que nadie asocia con un índice.
				//
				// Y la unicidad la decide el ÍNDICE, no un SELECT previo: entre un SELECT y un
				// INSERT hay una carrera y la base no la tiene.
				_, err := x.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_services_nombre ON services(project_id, device_id, name)`)
				return err
			},
		},
		{
			version: 37,
			name:    "services_declared_no_los_poda_el_latido",
			// QUIÉN PUSO LA FILA, PORQUE ESO DECIDE QUIÉN PUEDE SACARLA.
			//
			// ────────────────────────────────────────────────────────────────────────────────
			// EL AGUJERO QUE CIERRA, Y POR QUÉ TODAVÍA NO SE VEÍA
			//
			// La poda por ausencia (PodarServiciosAusentes, disparada por cada latido) da de baja
			// lo que la máquina dejó de reportar. Hasta acá, la tabla no distinguía un servicio
			// REPORTADO por el agente de uno DECLARADO a mano con musubi_fleet_service_declare —y
			// lo declarado a mano es, por definición, lo que ninguna máquina va a reportar nunca:
			// el bot de un Tier B, un puente, un contenedor en un host que no se enumera solo.
			//
			// O sea que el primer latido que traiga un enumerador de systemd se lleva puesto, de
			// una y en toda la flota a la vez, TODO lo que alguien declaró a mano. Hoy no explota
			// sólo porque el agente todavía no enumera (A42 abierto): es una mina, no un bug
			// latente, y el día que se despache ese slice explota en todas las máquinas juntas.
			//
			// EL BACKFILL NO ES `DEFAULT 1` NI `DEFAULT 0` A CIEGAS. Las filas que ya existen se
			// marcan declaradas si NUNCA reportaron (`last_report IS NULL`), que es la firma
			// exacta e inconfundible de AltaServicio: es el único camino que inserta con
			// last_report en NULL, y el agente siempre escribe la fecha del latido. Una fila con
			// last_report vino de un reporte —o alguien la declaró y la máquina la reporta, que es
			// justo el caso en que la poda dice algo cierto— y queda podable.
			//
			// Y NO es una columna de estado de las que este esquema se prohíbe (hay una prueba de
			// forma que las persigue): no describe cómo está el servicio ni se puede quedar vieja
			// mientras el mundo cambia. Describe su PROCEDENCIA, que es un hecho del pasado y no
			// se mueve más.
			// ────────────────────────────────────────────────────────────────────────────────
			up: func(x execQuerier) error {
				if err := agregarColumnaSiFalta(x, "services", "declared", "declared INTEGER NOT NULL DEFAULT 0"); err != nil {
					return err
				}
				// Idempotente: correrlo dos veces marca las mismas filas.
				_, err := x.Exec(`UPDATE services SET declared = 1 WHERE last_report IS NULL`)
				return err
			},
		},
		{
			version: 38,
			name:    "consentimiento_por_maquina",
			// QUÉ SE LE DEBE A LA PERSONA QUE ESTÁ EN LA MÁQUINA, Y SI HAY ALGUIEN A QUIEN
			// PREGUNTARLE.
			//
			// ────────────────────────────────────────────────────────────────────────────────
			// SON DOS COLUMNAS Y NO UNA, PORQUE SON DOS HECHOS DE DUEÑOS DISTINTOS
			//
			// `consentimiento` es una POLÍTICA: la escribe quien administra la máquina y dice qué
			// se le debe a quien la usa —nada, un aviso, un permiso, o nunca—. No cambia sola.
			//
			// `puede_preguntar` es una CAPACIDAD MEDIDA: la reporta el agente y dice si en esa
			// máquina hay dónde dibujar un diálogo y quién lo conteste. Un servidor headless
			// contesta que no; un escritorio con sesión abierta, que sí. Cambia con el mundo.
			//
			// Juntarlas en una sola columna obligaría a que la política mienta sobre el hardware o
			// a que el hardware pise la política. Separadas, el dominio las cruza:
			// `pide` sobre una máquina que no puede preguntar se degrada a PROHIBIDO —no a
			// libre—, porque quien escribió `pide` pidió que nadie entre sin permiso, y si el
			// permiso no se puede pedir, no se entra.
			//
			// EL DEFAULT DE `consentimiento` ES EL VACÍO Y NO UN GRADO. El dominio resuelve el
			// vacío al default (`avisa`), y escribirlo acá sería tener el mismo default en dos
			// lugares que se pueden desincronizar: cambiarlo en el código dejaría las filas
			// viejas con el anterior, en silencio.
			//
			// `puede_preguntar` ARRANCA EN 0 PARA TODOS, y eso es correcto aunque sea incómodo:
			// ningún agente desplegado sabe preguntar todavía. Arrancar en 1 sería afirmar una
			// capacidad que nadie midió, y `pide` se comportaría como si hubiera alguien del otro
			// lado cuando no lo hay. Se llena cuando el agente lo reporte, no antes.
			// ────────────────────────────────────────────────────────────────────────────────
			up: func(x execQuerier) error {
				if err := agregarColumnaSiFalta(x, "devices", "consentimiento",
					"consentimiento TEXT NOT NULL DEFAULT ''"); err != nil {
					return err
				}
				return agregarColumnaSiFalta(x, "devices", "puede_preguntar",
					"puede_preguntar INTEGER NOT NULL DEFAULT 0")
			},
		},
		{
			version: 39,
			name:    "cooldown_de_politica_por_alcance",
			// EL ENFRIAMIENTO DEJA DE SER POR MÁQUINA Y PASA A SER POR LO QUE LA POLÍTICA TOCA.
			//
			// ────────────────────────────────────────────────────────────────────────────────
			// EL BLOQUEO QUE A44 TENÍA ANOTADO, Y POR QUÉ ERA REAL
			//
			// La clave era (policy, device_id). Para una política de HOST alcanza: hay un solo
			// disco por máquina, una sola memoria. Para una de SERVICIO no, y el daño es peor
			// que no tener la política:
			//
			// Dos políticas sobre `nginx` y sobre `postgres` de la misma máquina caerían en la
			// misma fila. Reiniciar uno DEJARÍA MUDO al otro durante todo el enfriamiento, y el
			// segundo servicio se quedaría caído sin que nada actúe — justo por haber actuado
			// sobre el primero. Y el panel mostraría las dos políticas instaladas y activas.
			//
			// ────────────────────────────────────────────────────────────────────────────────
			// SE RECREA LA TABLA PORQUE SQLITE NO SABE CAMBIAR UNA PRIMARY KEY
			//
			// Agregar la columna con `ALTER TABLE` no alcanza: la clave seguiría siendo
			// (policy, device_id) y la base rechazaría la segunda fila del par. Así que se crea
			// la tabla nueva, se copia con `alcance = ''` —que es lo que corresponde a todo lo
			// que hay: son cooldowns de políticas de host— y se reemplaza.
			//
			// LA COPIA VA PRIMERO Y EL DROP DESPUÉS, en la misma transacción de la migración: si
			// algo falla en el medio, no queda ni media tabla. Y el nombre nuevo se renombra al
			// viejo para que ninguna consulta de arriba tenga que enterarse.
			//
			// ────────────────────────────────────────────────────────────────────────────────
			// `alcance` Y NO `servicio`, y el nombre importa
			//
			// Hoy lo único que llena esa columna es un nombre de servicio. Pero lo que la
			// columna representa es «QUÉ, dentro de la máquina, toca esta política» — y la
			// próxima cosa que se vigile adentro de un host (un contenedor por id, un punto de
			// montaje, una interfaz) va a querer el mismo espaciado sin que haya que migrar de
			// nuevo. Un nombre que describe la posición y no el ejemplo actual.
			up: func(x execQuerier) error {
				if _, err := x.Exec(`CREATE TABLE IF NOT EXISTS fleet_policy_state_v2 (
						policy     TEXT NOT NULL,
						device_id  TEXT NOT NULL,
						alcance    TEXT NOT NULL DEFAULT '',
						last_fired TEXT NOT NULL,
						PRIMARY KEY (policy, device_id, alcance)
					)`); err != nil {
					return err
				}
				// Idempotente: `INSERT OR IGNORE` deja correr la migración dos veces sin duplicar.
				if _, err := x.Exec(`INSERT OR IGNORE INTO fleet_policy_state_v2 (policy, device_id, alcance, last_fired)
					SELECT policy, device_id, '', last_fired FROM fleet_policy_state`); err != nil {
					return err
				}
				if _, err := x.Exec(`DROP TABLE fleet_policy_state`); err != nil {
					return err
				}
				_, err := x.Exec(`ALTER TABLE fleet_policy_state_v2 RENAME TO fleet_policy_state`)
				return err
			},
		},
		{
			version: 40,
			name:    "consentimiento_en_la_sesion_de_pantalla",
			// CÓMO CONTESTÓ EL USUARIO CUANDO HUBO QUE PREGUNTARLE (A57).
			//
			// ────────────────────────────────────────────────────────────────────────────────
			// COLUMNA PROPIA Y NO UN TEXTO ADENTRO DE `error`
			//
			// «Me dijeron que no» NO ES UN ERROR: es el sistema funcionando como se pidió. Y las
			// tres formas de no conceder se arreglan distinto:
			//
			//   negada        → una decisión de una persona, que hay que respetar
			//   sin_respuesta → nadie estaba; si pasa siempre, esa máquina no debería estar en `pide`
			//   no_se_pudo    → no había con qué preguntar; le falta software o le sobra aislamiento
			//
			// Metidas las tres en un texto libre, la diferencia sobrevive exactamente hasta que
			// alguien mejora la redacción del mensaje. Con columna, cualquier consulta las separa.
			//
			// VACÍA ES UN VALOR LEGÍTIMO Y ES EL DE CASI TODAS LAS FILAS: significa «no hizo
			// falta preguntar», que es lo que pasa con `libre` y con `avisa`. Por eso el DEFAULT
			// es '' y no algo como 'desconocido' — inventar un tercer significado para las filas
			// viejas obligaría a interpretarlo en cada lectura.
			up: func(x execQuerier) error {
				return agregarColumnaSiFalta(x, "screen_sessions", "consentimiento",
					"consentimiento TEXT NOT NULL DEFAULT ''")
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

// agregarColumnaSiFalta hace idempotente un `ALTER TABLE ... ADD COLUMN`.
//
// SQLite no acepta `ADD COLUMN IF NOT EXISTS`, y una migración que corre dos veces sobre la misma
// base NO es hipotético: pasa cuando un test rebobina user_version para ejercitar el camino de
// actualización, y pasa cuando una base nueva recibe la columna por la baseline y la migración se
// la vuelve a agregar. Es la trampa que ya documentaron la v21 y la v22 — ésta sólo le pone una
// función al patrón para no volver a resolverla a mano cada vez.
//
// `tabla` y `columna` se interpolan porque PRAGMA y DDL no admiten parámetros. Son literales del
// código, nunca entrada de usuario; si algún día lo fueran, esto sería una inyección.
func agregarColumnaSiFalta(x execQuerier, tabla, columna, ddl string) error {
	rows, err := x.Query(`PRAGMA table_info(` + tabla + `)`)
	if err != nil {
		return fmt.Errorf("error al leer columnas de %s: %w", tabla, err)
	}
	existe := false
	for rows.Next() {
		var (
			cid         int
			name, ctype string
			notnull, pk int
			dflt        interface{}
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			rows.Close()
			return fmt.Errorf("error al escanear PRAGMA table_info(%s): %w", tabla, err)
		}
		if name == columna {
			existe = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("error al recorrer PRAGMA table_info(%s): %w", tabla, err)
	}
	rows.Close()

	if existe {
		return nil
	}
	if _, err := x.Exec(`ALTER TABLE ` + tabla + ` ADD COLUMN ` + ddl); err != nil {
		return fmt.Errorf("error al agregar %s.%s: %w", tabla, columna, err)
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
