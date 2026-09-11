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

// ErrEsquemaLegible acompaña a ErrSchemaTooNew cuando la base, aun siendo más nueva, declara un
// piso de lectura que este binario alcanza: no se puede migrar ni escribir, pero SÍ leer.
//
// Va como error aparte y ENVUELTO JUNTO a ErrSchemaTooNew —Go admite varios %w— para que todo el
// código que ya preguntaba `errors.Is(err, ErrSchemaTooNew)` siga viendo lo mismo. Si fuera un
// error suelto, cada caller existente pasaría a tratar la base legible como un fallo desconocido,
// que es peor que el estado anterior: hoy al menos sabe que el esquema es el problema.
var ErrEsquemaLegible = errors.New("la base se puede leer, no escribir")

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
//
// readCompatible declara si un binario que sólo conoce hasta la migración ANTERIOR puede seguir
// LEYENDO esta base correctamente. Es lo que alimenta el piso de lectura (ver pisoDeLectura).
//
// LA DEFINICIÓN, Y ES ESTRECHA A PROPÓSITO: una migración es readCompatible cuando un binario
// anterior, abriendo la base en sólo lectura, devuelve para TODA consulta que sabía hacer
// exactamente las mismas filas que devolvería el binario que la aplicó. No alcanza con que la
// migración no reescriba datos: si a partir de ella pueden APARECER filas que el lector viejo no
// sabe filtrar, no es readCompatible aunque su DDL sea un puro ADD COLUMN.
//
//	El caso que fija el criterio es la v22. Su propio comentario la llama «aditiva», y lo es en
//	el sentido de que ADD COLUMN NOT NULL DEFAULT no hace una pasada de escritura. Pero desde
//	la v22 existen filas con quarantined=1, y el predicado de visibilidad pasó a ser
//	`archived = 0 AND superseded_by IS NULL AND quarantined = 0`. Un binario v21 no tiene ese
//	tercer filtro y devolvería contenido de LLM en cuarentena como si fuera memoria verificada.
//	Dos sentidos distintos de «aditiva», y el que importa acá es el del lector.
//
// EL CERO DE GO ES `false`, O SEA «NO COMPATIBLE», Y ESO ES DELIBERADO. Es el mismo fail-safe que
// `toolEntry.readOnly`: una migración nueva que nadie clasificó sube el piso y hace que los
// binarios viejos se nieguen, que es el lado seguro del error. Marcarla `true` es una afirmación
// que hay que ganarse mirando si alguna consulta EXISTENTE cambia de resultado.
//
// EL ALCANCE ES GENERAL, NO «SÓLO LA MEMORIA». Se evaluó acotar el piso a las tablas de la memoria
// —observations, relations, embeddings, code_*— porque da un piso mucho más bajo (22 en vez de 43)
// y dejaría leer a más binarios. Se descartó: ese piso sólo sería cierto si el modo sólo-lectura
// jamás sirviera una lectura de flota, y ese acoplamiento no está escrito en ningún lado. Un
// número que depende de una promesa tácita es la clase de dato que después alguien cita como
// medición. Si algún día hace falta, el camino es un SEGUNDO piso declarado con su propio alcance,
// no reinterpretar éste.
type migration struct {
	version        int
	name           string
	readCompatible bool
	up             func(execQuerier) error
}

// schemaMigrations devuelve las migraciones conocidas por este binario, en orden
// ascendente de versión. Para evolucionar el esquema: agregar una nueva entrada con
// la siguiente versión (nunca editar ni reordenar las ya publicadas).
func schemaMigrations() []migration {
	return []migration{
		{
			version:        1,
			name:           "baseline",
			readCompatible: true,
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
			version:        2,
			name:           "idx_obs_archived",
			readCompatible: true,
			// Índice por `archived`: acelera la purga de retención (WHERE archived=1)
			// y el scan del olvido (WHERE archived=0). Primera migración post-baseline:
			// alcanza también a bases ya migradas a v1 (que no re-ejecutan la baseline).
			up: func(x execQuerier) error {
				_, err := x.Exec(`CREATE INDEX IF NOT EXISTS idx_obs_archived ON observations(archived)`)
				return err
			},
		},
		{
			version:        3,
			name:           "archived_at",
			readCompatible: true,
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
			version:        4,
			name:           "work_lease_ttl",
			readCompatible: true,
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
			version:        5,
			name:           "relations_bitemporal",
			readCompatible: true,
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
			version:        6,
			name:           "run_events_journal",
			readCompatible: true,
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
			version:        7,
			name:           "observations_mem_type",
			readCompatible: true,
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
			version:        8,
			name:           "work_bids",
			readCompatible: true,
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
			version:        9,
			name:           "debate",
			readCompatible: true,
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
			version:        10,
			name:           "observations_scope_project",
			readCompatible: true,
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
			version:        11,
			name:           "outbox",
			readCompatible: true,
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
			version:        12,
			name:           "embeddings_model_id",
			readCompatible: true,
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
			// NO readCompatible: code_memory se reconstruye y pasa a estar partida por project_id: un lector sin ese
			// filtro devuelve la memoria de codigo de OTROS proyectos.
			readCompatible: false,
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
			// NO readCompatible: relations se reconstruye con project_id y ademas barre las huerfanas: un lector viejo
			// devuelve relaciones de otros proyectos, y cuenta distinto.
			readCompatible: false,
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
			version:        15,
			name:           "telemetry_decisions_project_id",
			readCompatible: true,
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
			version:        16,
			name:           "observations_author",
			readCompatible: true,
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
			// NO readCompatible: el indice FTS se tira y se rehace como external-content, y se borran los triggers que
			// mantenia el binario anterior.
			readCompatible: false,
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
			version:        18,
			name:           "code_graph",
			readCompatible: true,
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
			version:        19,
			name:           "sync_seq",
			readCompatible: true,
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
			version:        20,
			name:           "relations_source",
			readCompatible: true,
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
			version:        21,
			name:           "observation_origins",
			readCompatible: true,
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
			// NO readCompatible: desde aca existen filas quarantined=1 y el predicado de visibilidad suma
			// `quarantined = 0`: un lector v21 devuelve contenido de LLM en cuarentena como si
			// fuera memoria verificada. Ver la nota del campo en el struct `migration`.
			readCompatible: false,
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
			version:        23,
			name:           "tool_invocations_ledger",
			readCompatible: true,
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
			version:        24,
			name:           "skill_usage_counters",
			readCompatible: true,
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
			version:        25,
			name:           "work_claim_log",
			readCompatible: true,
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
			version:        26,
			name:           "work_autonomy",
			readCompatible: true,
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
			version:        27,
			name:           "relation_signals_split",
			readCompatible: true,
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
			version:        28,
			name:           "shadow_verdicts",
			readCompatible: true,
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
			version:        29,
			name:           "devices_registro_de_flota",
			readCompatible: true,
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
			version:        30,
			name:           "devices_ultima_muestra",
			readCompatible: true,
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
			version:        31,
			name:           "device_commands_bitacora",
			readCompatible: true,
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
			version:        32,
			name:           "screen_sessions_bitacora",
			readCompatible: true,
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
			version:        33,
			name:           "fleet_policy_state",
			readCompatible: true,
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
			version:        34,
			name:           "shell_sessions_bitacora",
			readCompatible: true,
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
			version:        35,
			name:           "rustdesk_id_procedencia",
			readCompatible: true,
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
			version:        36,
			name:           "services_inventario_por_maquina",
			readCompatible: true,
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
			version:        37,
			name:           "services_declared_no_los_poda_el_latido",
			readCompatible: true,
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
			// una y en toda la flota a la vez, TODO lo que alguien declaró a mano. Cuando esto se
			// escribió no explotaba sólo porque el agente no enumeraba (A42 estaba abierto); A42 se
			// cerró y hoy el agente SÍ enumera, así que lo único que separa a este esquema de esa
			// pérdida es el `declared` que esta migración introduce.
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
			version:        38,
			name:           "consentimiento_por_maquina",
			readCompatible: true,
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
			// NO readCompatible: fleet_policy_state se reconstruye con `alcance` en la clave: una lectura vieja por
			// (policy, device_id) ya no identifica una sola fila.
			readCompatible: false,
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
			version:        40,
			name:           "consentimiento_en_la_sesion_de_pantalla",
			readCompatible: true,
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
		{
			version:        41,
			name:           "origen_del_comando",
			readCompatible: true,
			// QUIÉN LO ORIGINÓ: una persona o una regla (A59).
			//
			// ────────────────────────────────────────────────────────────────────────────────
			// SE GUARDA PORQUE ES UN HECHO DEL PASADO, NO UN ESTADO DERIVABLE
			//
			// Hoy la diferencia se lee del NOMBRE del principal —`auto-heal` contra `gio`—, que
			// es una convención sostenida por `config.yaml`. Derivarla al leer sería lo barato y
			// sería falso: las políticas se agregan y se sacan, así que un comando de hace tres
			// meses, disparado por un principal que hoy ya no es una política, se etiquetaría
			// como manual. «Esto lo originó una regla» es un hecho de CUANDO PASÓ.
			//
			// El resto del dominio deriva lo que sigue siendo cierto ahora (que una sesión venció,
			// que un comando expiró) y guarda lo que ocurrió. Ésta es de las segundas.
			//
			// EL DEFAULT ES '' Y SIGNIFICA «NO SE SABE», NO «PERSONA». Es la regla del cero
			// mentiroso llevada al origen: las filas anteriores a esta migración no dicen quién
			// las originó, y rellenarlas con `persona` haría que cada disparo automático viejo
			// figure como una acción humana — en la cronología de una máquina, eso es atribuirle
			// a alguien algo que no hizo. Un backfill por nombre de principal tampoco sirve:
			// reproduciría la misma convención que esta columna viene a reemplazar.
			up: func(x execQuerier) error {
				return agregarColumnaSiFalta(x, "device_commands", "origen",
					"origen TEXT NOT NULL DEFAULT ''")
			},
		},
		{
			version:        42,
			name:           "ventanas_de_mantenimiento",
			readCompatible: true,
			// LA VENTANA DE MANTENIMIENTO ES UN HECHO DEL DOMINIO, NO UN SILENCE DE ALERTMANAGER.
			//
			// ────────────────────────────────────────────────────────────────────────────────
			// POR QUÉ NO ALCANZA CON SILENCIAR LA ALERTA
			//
			// Un `amtool silence` calla el aviso y NO frena nada más. Pero las políticas
			// (scheduler_flota.go) no leen alertas: leen la muestra y actúan solas. Así que un
			// reinicio planificado de postgres dispara `servicio_caido`, la política lo levanta
			// EN MITAD DEL MANTENIMIENTO, y el silence sólo garantiza que nadie se entere.
			//
			// Es la peor combinación posible: la automatización sigue actuando y el canal que lo
			// contaría está apagado. Por eso la ventana vive acá, donde el scheduler la puede
			// leer, y no en la configuración de la herramienta que sólo entrega mensajes.
			//
			// APPEND-ONLY, como device_commands y shell_sessions: la cronología de una máquina se
			// construye SOLO sobre tablas que no se editan, y «hubo un mantenimiento de tal hora
			// a tal hora» es exactamente la clase de hecho que explica por qué esa máquina estuvo
			// callada. Cancelar una ventana es escribir otra fila, no borrar la primera.
			up: func(x execQuerier) error {
				if _, err := x.Exec(`
					CREATE TABLE IF NOT EXISTS device_maintenance (
						id          TEXT PRIMARY KEY,
						device_id   TEXT NOT NULL,
						project_id  TEXT NOT NULL,
						principal   TEXT NOT NULL,
						desde       TEXT NOT NULL,
						hasta       TEXT NOT NULL,
						motivo      TEXT NOT NULL DEFAULT '',
						cancelada   INTEGER NOT NULL DEFAULT 0,
						creado      TEXT NOT NULL
					)`); err != nil {
					return fmt.Errorf("error al crear device_maintenance: %w", err)
				}
				// El índice cubre la única consulta caliente: «¿esta máquina está en ventana
				// AHORA?», que el scheduler hace en cada barrido y el exportador en cada scrape.
				if _, err := x.Exec(`
					CREATE INDEX IF NOT EXISTS idx_maintenance_device_hasta
					ON device_maintenance(device_id, hasta)`); err != nil {
					return fmt.Errorf("error al indexar device_maintenance: %w", err)
				}
				return nil
			},
		},
		{
			version: 43,
			name:    "rotacion_del_token_de_dispositivo",
			// NO readCompatible: durante la rotacion el dispositivo se autentica por token_sha256_nuevo
			// (internal/memory/rotacion.go:135): un lector viejo, que solo mira token_sha256,
			// NO lo encuentra y contesta que no existe.
			readCompatible: false,
			// ROTAR UN TOKEN SIN REINSTALAR EL AGENTE (Ola 2 del plan empresa).
			//
			// Hasta acá el token de un device se escribía UNA vez, al enrolar, y se vaciaba al
			// revocar. No había punto medio: rotar era revocar + enrolar + ir a la máquina a
			// pegar el token nuevo. Con cuatro máquinas se hace; con cuarenta no, y un auditor
			// pide rotación demostrable (SOC2 CC6.1, ISO A.5.17).
			//
			// DOS HASHES A LA VEZ, Y ES LO ÚNICO QUE HACE POSIBLE LA ROTACIÓN EN CALIENTE. El
			// agente se entera del token nuevo en la RESPUESTA de un latido, o sea después de
			// haber usado el viejo. Si el viejo dejara de valer en el instante de emitir el
			// nuevo, el agente quedaría afuera entre que recibe y guarda — y si ese guardado
			// falla, para siempre. Con los dos válidos, la ventana existe y está acotada.
			//
			// `rotacion_vence` la acota: pasado el plazo, la rotación se ABANDONA (se descarta el
			// nuevo y sigue el viejo), no se completa a la fuerza. Ver el comentario de
			// AbandonarRotacionesVencidas para por qué ése es el lado seguro.
			up: func(x execQuerier) error {
				if err := agregarColumnaSiFalta(x, "devices", "token_sha256_nuevo",
					"token_sha256_nuevo TEXT NOT NULL DEFAULT ''"); err != nil {
					return err
				}
				return agregarColumnaSiFalta(x, "devices", "rotacion_vence",
					"rotacion_vence TEXT NOT NULL DEFAULT ''")
			},
		},
		{
			version:        44,
			name:           "aprobacion_de_cuatro_ojos",
			readCompatible: true,
			// LA SEGUNDA PERSONA (Ola 2 del plan empresa).
			//
			// Una shell interactiva se saltea cualquier allowlist de comandos, y hasta acá una
			// sola persona con `shell` sobre producción podía abrirla sin que nadie se enterara
			// hasta después. La bitácora lo cuenta DESPUÉS; esto exige que alguien lo sepa ANTES.
			//
			// SON DOS COSAS Y VAN JUNTAS PORQUE UNA SIN LA OTRA NO SIRVE: la marca en la máquina
			// (`requiere_aprobacion`) y la tabla de los pedidos. Con la marca sola no hay dónde
			// aprobar; con la tabla sola no hay nada que la exija.
			//
			// DEFAULT 0, Y ES LA ÚNICA ELECCIÓN DEFENDIBLE. Encender cuatro ojos en toda la flota
			// de golpe dejaría a cada máquina esperando un segundo par de ojos que nadie sabe que
			// tiene que dar, y la salida que la gente encuentra es apagar el control entero. Se
			// enciende máquina por máquina, que es como se sabe cuáles importan.
			//
			// LA TABLA ES APPEND-ONLY, como device_commands, shell_sessions y device_maintenance:
			// «esta sesión la aprobó fulano» es el hecho que este control existe para dejar
			// escrito. Usar una aprobación la marca `usada`, no la borra — borrarla dejaría la
			// sesión en la bitácora sin quién la avaló, que es la mitad que importa.
			up: func(x execQuerier) error {
				if err := agregarColumnaSiFalta(x, "devices", "requiere_aprobacion",
					"requiere_aprobacion INTEGER NOT NULL DEFAULT 0"); err != nil {
					return err
				}
				if _, err := x.Exec(`
					CREATE TABLE IF NOT EXISTS fleet_approvals (
						id           TEXT PRIMARY KEY,
						device_id    TEXT NOT NULL,
						project_id   TEXT NOT NULL,
						solicitante  TEXT NOT NULL,
						capacidad    TEXT NOT NULL,
						motivo       TEXT NOT NULL DEFAULT '',
						estado       TEXT NOT NULL,
						aprobador    TEXT NOT NULL DEFAULT '',
						nota         TEXT NOT NULL DEFAULT '',
						creada       TEXT NOT NULL,
						vence        TEXT NOT NULL,
						resuelta     TEXT NOT NULL DEFAULT '',
						usada        TEXT NOT NULL DEFAULT ''
					)`); err != nil {
					return fmt.Errorf("error al crear fleet_approvals: %w", err)
				}
				// El índice cubre la consulta caliente, que es la del CAMINO DE ACCESO: «¿este
				// principal tiene un permiso vigente para esta máquina y esta capacidad?», una
				// vez por cada intento de abrir una shell o una pantalla.
				if _, err := x.Exec(`
					CREATE INDEX IF NOT EXISTS idx_approvals_busqueda
					ON fleet_approvals(device_id, solicitante, capacidad, estado)`); err != nil {
					return fmt.Errorf("error al indexar fleet_approvals: %w", err)
				}
				return nil
			},
		},
		{
			version:        45,
			name:           "consentimiento_en_la_shell",
			readCompatible: true,
			// `pide` SOBRE UNA SHELL NO PREGUNTABA NADA Y SE ABRÍA IGUAL.
			//
			// `AvisaAlUsuario()` es true para `pide` también —es `nivel >= avisa`—, así que el
			// switch del camino de shell, que sólo tenía las dos ramas de `avisa`, mandaba una
			// notificación y abría el prompt en el acto. La persona sentada enfrente recibía un
			// aviso QUE NO PODÍA CONTESTAR mientras el operador ya estaba adentro, y el grado
			// promete lo contrario: «tiene que aceptar. Sin respuesta, no hay sesión».
			//
			// La columna es la que hace posible el flujo de dos llamadas que pantalla ya tiene:
			// la primera deja la sesión en `esperando_permiso` SIN abrir ningún canal ni tocar
			// SSH, y la respuesta del usuario la mueve a `abriendo` o a `sin_permiso`.
			//
			// DEFAULT '' Y NO 'concedida': vacío significa «no hizo falta preguntar», que es lo
			// que pasa en `libre` y `avisa` y en todas las filas anteriores a esta migración.
			// Rellenarlas con «concedida» le atribuiría a alguien un permiso que nadie dio.
			up: func(x execQuerier) error {
				return agregarColumnaSiFalta(x, "shell_sessions", "consentimiento",
					"consentimiento TEXT NOT NULL DEFAULT ''")
			},
		},
		{
			version:        46,
			name:           "plano_del_comando",
			readCompatible: true,
			// EL AVISO DE UN EXEC SE LEÍA COMO PANTALLA, Y LO VEÍA CUALQUIERA CON `screen:view`.
			//
			// ────────────────────────────────────────────────────────────────────────────────
			// Los tres caminos —pantalla, shell y exec— encolan `musubi:avisar` con EXACTAMENTE
			// el mismo argv. Eso es a propósito desde A83: un solo encolador, para que sumar un
			// camino no sea acordarse de copiar un bloque. Lo que no era a propósito es que
			// `TipoDeArgv` clasificara los tres como plomería de PANTALLA, así que la cronología
			// le mostraba a un principal con sólo `screen:view` la línea «Musubi: fulano está
			// ejecutando comandos en esta máquina» y «...está abriendo una terminal...».
			//
			// El texto lo dice todo: quién, cuándo y en qué plano. Dos planos que esa credencial
			// no puede ver, entrando por la puerta de atrás de una fila de plomería.
			//
			// NO SE PODÍA ARREGLAR MIRANDO EL ARGV, porque los tres son idénticos, ni parseando
			// el texto, que es una cadena de presentación y se reescribe. El plano lo declara
			// quien encola y viaja acá.
			//
			// DEFAULT '' Y NO 'canal_pantalla'. Vacío significa «no se sabe», y para estas filas
			// eso es la verdad: las anteriores a esta migración pudieron salir de cualquiera de
			// los tres caminos. No hay backfill honesto —el argv no distingue— y no hay valor
			// seguro que no sea esconderlas: ponerlas en pantalla filtra el exec y la shell,
			// ponerlas en exec filtra la pantalla. TipoDeComando las deja en HechoSinClasificar,
			// que es el fail-closed que la cronología ya tenía escrito para lo que no conoce.
			//
			// El costo se dice entero: los avisos VIEJOS dejan de aparecer en la cronología. No
			// se pierde nada que no esté contado —cada aviso acompaña a un hecho que ya está ahí
			// con su compuerta correcta (la sesión, o el propio exec)— y la fila sigue en la
			// bitácora de comandos, que es donde se audita si el aviso se entregó.
			up: func(x execQuerier) error {
				return agregarColumnaSiFalta(x, "device_commands", "plano",
					"plano TEXT NOT NULL DEFAULT ''")
			},
		},
		{
			version:        47,
			name:           "panel_con_modelo_y_evidencia",
			readCompatible: true,
			// DOS JUECES DEL MISMO MODELO NO SON DOS OPINIONES, Y UNA OPINION NO PESA LO MISMO
			// QUE UNA COMPROBACION.
			//
			// ────────────────────────────────────────────────────────────────────────────────
			// Son dos problemas distintos que caen en las mismas dos tablas, y por eso van en
			// una sola migración.
			//
			// UNO. La skill le da a cada escéptico un LENTE distinto y nada le da un MODELO
			// distinto. Peor: aunque alguien los lanzara con modelos distintos, el sistema no
			// guardaba nada de eso —`debate_postures` tenía {round, agent, stance, created_at}
			// y ni una columna de modelo—. La frase «este cambio lo revisaron tres modelos
			// distintos» era inverificable a posteriori: no había dónde leerla.
			//
			// DOS. El tally cuenta filas con GROUP BY choice, así que un lente que corrió los
			// tests pesa exactamente igual que uno que leyó el diff y opinó, y que uno que no
			// pudo comprobar nada. La skill ya nombraba el riesgo en prosa —«un panel que opina
			// sin haber corrido nada es teatro de verificación»— sin ningún mecanismo que lo
			// hiciera cumplir.
			//
			// POR QUE LOS CAMPOS PUEDEN NACER OBLIGATORIOS. Las tres tablas del debate están en
			// CERO filas (medido en las seis bases locales el 2026-09-06). Sin datos vivos no
			// hace falta plan de migración, ni default piadoso, ni período de gracia: la
			// validación puede exigirlos desde el primer post. Con datos, no se podría.
			//
			// EL DEFAULT ES CADENA VACIA Y NO UN VALOR PIADOSO. Vacío significa «no se declaró»,
			// y para la compuerta eso vale lo mismo que «no verifiqué nada» — que es el
			// fail-closed correcto. Un default como 'deterministica' habría desarmado la
			// compuerta con cada fila vieja, o sea justo al revés.
			//
			// gated_choice VA EN `debates` Y NO EN LOS VOTOS porque es una propiedad del DEBATE:
			// se declara al abrirlo, antes de saber quién va a votar qué. Declararla después
			// sería mover el arco con la pelota en el aire.
			up: func(x execQuerier) error {
				for _, c := range []struct{ tabla, col, ddl string }{
					{"debate_postures", "model", "model TEXT NOT NULL DEFAULT ''"},
					{"debate_postures", "evidence", "evidence TEXT NOT NULL DEFAULT ''"},
					{"debate_votes", "model", "model TEXT NOT NULL DEFAULT ''"},
					{"debate_votes", "evidence", "evidence TEXT NOT NULL DEFAULT ''"},
					{"debates", "gated_choice", "gated_choice TEXT NOT NULL DEFAULT ''"},
				} {
					if err := agregarColumnaSiFalta(x, c.tabla, c.col, c.ddl); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			version:        48,
			name:           "schema_read_floor",
			readCompatible: true,
			// EL PISO DE LECTURA, GRABADO EN LA BASE PARA QUE UN BINARIO VIEJO PUEDA LEERLO.
			//
			// La guarda de compatibilidad hacia adelante es un booleano: o el binario llega al
			// esquema de la base, o se niega. Eso trata igual dos situaciones muy distintas —una
			// base que cambió de forma y una que sólo sumó columnas— y obliga a un cutover duro
			// de toda la malla por cada migración, aunque el 87% de ellas (41 de 47, medido) no
			// cambie una sola lectura.
			//
			// El piso lo DERIVA el binario que migra (`pisoDeLectura`: la migración no-compatible
			// más alta) y lo deja acá, porque el binario viejo no puede derivarlo: no conoce las
			// migraciones futuras. Es el único dato de esta pieza que tiene que viajar por la
			// base y no por el código.
			//
			// EN UNA TABLA Y NO EN `PRAGMA application_id`. El PRAGMA está libre y sería más
			// barato, pero su valor por defecto es 0 y 0 también sería un piso válido: «ausente»
			// y «cualquier binario puede leer» serían el mismo número, o sea el valor de fallo
			// sería el tranquilizador. Con una tabla, ausente es ausente, y el lector viejo
			// puede negarse por falta de evidencia en vez de por un cero que se lee como permiso.
			//
			// CHECK (id = 1) para que sea una fila y no una bitácora: si hubiera varias, «el
			// piso» pasaría a depender de cuál se lee.
			up: func(x execQuerier) error {
				_, err := x.Exec(`CREATE TABLE IF NOT EXISTS schema_floor (
					id            INTEGER PRIMARY KEY CHECK (id = 1),
					read_floor    INTEGER NOT NULL,
					set_by_schema INTEGER NOT NULL,
					set_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`)
				return err
			},
		},
		{
			version:        49,
			name:           "capver_por_maquina",
			readCompatible: true,
			// EL CAPVER QUE DECLARA CADA MÁQUINA, GUARDADO DONDE YA VIVE SU AUTORREPORTE.
			//
			// `devices.agent_ver` dice qué BUILD corre; esto dice qué CONTRATO habla, que no es lo
			// mismo: dos builds distintos pueden compartir capver, y ése es el punto de tener la
			// banda. Sin guardarlo, la pregunta «¿qué máquinas no pueden hablar mi protocolo?»
			// sólo se puede contestar esperando el próximo latido de cada una.
			//
			// ES readCompatible: ningún camino de lectura filtra por esta columna, así que un
			// binario anterior devuelve exactamente las mismas filas de `devices` que hoy. El
			// default 0 significa «no declara», igual que en el cuerpo del latido.
			up: func(x execQuerier) error {
				return agregarColumnaSiFalta(x, "devices", "capver", "capver INTEGER NOT NULL DEFAULT 0")
			},
		},
		{
			// ERAN LA 47 Y LA 48 EN LA RAMA DE FLOTA, Y SE RENUMERARON AL MERGEAR.
			//
			// Las dos ramas estrenaron 47 y 48 EN PARALELO con contenido distinto: acá vivían
			// `lo_que_el_agente_dijo_y_se_tiraba` y `cuantos_servicios_no_entraron`, y en main
			// `panel_con_modelo_y_evidencia`, `schema_read_floor` y `capver_por_maquina`.
			//
			// DEJARLAS DUPLICADAS NO HABRÍA FALLADO, Y ÉSE ERA EL PELIGRO: el aplicador saltea con
			// `if m.version <= current { continue }`, así que sobre cualquier base que ya hubiera
			// pasado por las 47/48 de main estas dos quedaban MUERTAS en silencio — sin columna,
			// sin error, y con `user_version` diciendo que todo se aplicó.
			version:        50,
			name:           "lo_que_el_agente_dijo_y_se_tiraba",
			readCompatible: true,
			// DOS HECHOS QUE EL AGENTE REPORTA, QUE EL CEREBRO USABA UNA VEZ Y NO GUARDABA.
			//
			// `MotivoNoPreguntar` viajaba en el latido, alimentaba UNA línea de `logx.Info` «una vez
			// por máquina» y se tiraba; `token_fuente` igual. Así que a «¿por qué esta máquina no
			// puede preguntar?» (A99) y «¿por qué su token no puede rotar?» (A102) el sistema no
			// sabía responder aunque el agente ya se lo había dicho. Las dos veces el costo se pagó
			// leyendo código y leyendo un `.cmd` EN la máquina.
			//
			// LAS DOS ARRANCAN VACÍAS Y EL VACÍO SIGNIFICA «NO LO DIJO», no «no puede»: leer el
			// silencio de un agente viejo como una incapacidad sería acusar a la flota de un
			// defecto que nadie midió.
			//
			// ES readCompatible: son dos ADD COLUMN sobre `devices` con default, y ninguna consulta
			// existente filtra por ellas — un binario anterior devuelve exactamente las mismas
			// filas. No es el caso de la v22, donde la columna nueva entró en el predicado de
			// visibilidad y un lector viejo habría servido filas que no sabía descartar.
			up: func(x execQuerier) error {
				if err := agregarColumnaSiFalta(x, "devices", "motivo_no_preguntar",
					"motivo_no_preguntar TEXT NOT NULL DEFAULT ''"); err != nil {
					return err
				}
				return agregarColumnaSiFalta(x, "devices", "token_fuente",
					"token_fuente TEXT NOT NULL DEFAULT ''")
			},
		},
		{
			version:        51,
			name:           "cuantos_servicios_no_entraron",
			readCompatible: true,
			// EL RECORTE DEL LATIDO, QUE HASTA HOY NO SALÍA DE LA MÁQUINA (A116).
			//
			// El latido lleva un techo (`fleet.ServiciosPorLatido` = 64) y el agente cortaba la
			// lista escribiendo el número en su propio log, en la máquina, una vez por arranque:
			// donde nadie mira. Del lado del cerebro, 64 truncados y 64 completos eran el MISMO
			// mensaje — y como la poda por ausencia da de baja lo que no vino, eso no era una
			// ceguera sino una AFIRMACIÓN FALSA: 26 servicios `docker` y 87 de Windows anotados
			// como revocados en `davantis-1`, entre ellos los 11 contenedores de `altura-erp`, que
			// estaban corriendo.
			//
			// Arranca en 0 y el 0 significa «no recortó». Acá la ambigüedad con «no lo dijo» SÍ es
			// aceptable —al revés que en la v50— porque un agente viejo que trunca deja el 0, que
			// es exactamente el comportamiento de hoy: no se pierde nada que ahora exista.
			//
			// ES readCompatible: mismo criterio que la v50.
			up: func(x execQuerier) error {
				return agregarColumnaSiFalta(x, "devices", "servicios_omitidos",
					"servicios_omitidos INTEGER NOT NULL DEFAULT 0")
			},
		},
		{
			// LA REPARACIÓN DE LA BIFURCACIÓN, Y NO ES TEÓRICA: SIN ESTO EL MERGE DEJA BASES QUE
			// NO SE PUEDEN ABRIR.
			//
			// Las dos ramas estrenaron 47 y 48 en paralelo. Renumerar las de flota a 50/51 arregla
			// las bases que venían por main, pero NO las que ya pasaron por las 47/48 de flota:
			// para ésas, `if m.version <= current { continue }` saltea las 47 y 48 de main, que son
			// las que crean `schema_floor` y las columnas del panel. Y el aplicador igual deja
			// `user_version` en la última, así que la base queda marcada COMO SI TODO SE HUBIERA
			// APLICADO.
			//
			// MEDIDO EL 2026-09-09 sobre una copia real de una base de esta rama en esquema 48:
			//
			//	Error al abrir la base de datos: error al grabar el piso de lectura:
			//	SQL logic error: no such table: schema_floor
			//	user_version = 51
			//
			// O sea: no es una degradación silenciosa, es una base que el binario no abre — y con
			// el número diciendo que está al día. El diagnóstico costaría lo mismo que costó A111.
			//
			// SE PUEDE CORRER SIEMPRE porque los dos cuerpos de main son idempotentes por
			// construcción (`agregarColumnaSiFalta` y `CREATE TABLE IF NOT EXISTS`): sobre una base
			// que vino por main esto es un no-op, y sobre una que vino por flota la completa. Se
			// REPITE el DDL en vez de llamar a las migraciones de arriba a propósito: una migración
			// que invoca a otra ata dos versiones que después nadie puede mover por separado.
			version:        52,
			name:           "reparar_la_bifurcacion_de_47_y_48",
			readCompatible: true,
			up: func(x execQuerier) error {
				// De la 47 de main: el panel deja de ser un eco.
				for _, c := range []struct{ tabla, col, ddl string }{
					{"debate_postures", "model", "model TEXT NOT NULL DEFAULT ''"},
					{"debate_postures", "evidence", "evidence TEXT NOT NULL DEFAULT ''"},
					{"debate_votes", "model", "model TEXT NOT NULL DEFAULT ''"},
					{"debate_votes", "evidence", "evidence TEXT NOT NULL DEFAULT ''"},
					{"debates", "gated_choice", "gated_choice TEXT NOT NULL DEFAULT ''"},
				} {
					if err := agregarColumnaSiFalta(x, c.tabla, c.col, c.ddl); err != nil {
						return err
					}
				}
				// De la 48 de main: el piso de lectura grabado en la base.
				_, err := x.Exec(`CREATE TABLE IF NOT EXISTS schema_floor (
					id            INTEGER PRIMARY KEY CHECK (id = 1),
					read_floor    INTEGER NOT NULL,
					set_by_schema INTEGER NOT NULL,
					set_at        DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
				)`)
				return err
			},
		},
		{
			version:        53,
			name:           "por_que_esta_maquina_no_puede_enumerar",
			readCompatible: true,
			// EL FALLO DEL ENUMERADOR, QUE HASTA HOY SÓLO EXISTÍA EN EL LOG DE LA MÁQUINA.
			//
			// Cuando `enumerarServicios` falla, el agente NO manda el inventario —a propósito: media
			// lista haría que el cerebro pode lo que no vino— y avisa una vez por hora en su propio
			// log. Del lado del cerebro ese silencio es IDÉNTICO al de un inventario que no cambió,
			// así que una máquina con el enumerador roto se ve exactamente igual que una sana.
			//
			// MEDIDO EL 2026-09-09 EN `davantis-1`: 64 alertas `ServicioSinNoticias` —una por
			// servicio conocido—, 61 horas sin reportar, con el agente vivo (latido de hace 4,8 s) y
			// mandando CPU y uptime sin problema. Sesenta y cuatro alertas para UNA causa, y ninguna
			// la nombra. Eso no es una alarma: es ruido que enseña a ignorar el canal, la misma
			// lección que dejaron los trece `MaquinaCaida` de A79.
			//
			// GUARDA EL MOTIVO Y NO UN BOOLEANO por el mismo criterio que `motivo_no_preguntar`: las
			// causas se arreglan distinto —falta un binario, WMI no contesta, el usuario no tiene
			// permiso— y un `true` obligaría a entrar a la máquina para saber cuál es, que es
			// justamente el paso manual que esto elimina.
			//
			// Arranca VACÍA y el vacío significa «no hay falla que reportar»: un agente viejo que no
			// manda el campo y uno nuevo que enumeró bien llegan los dos como "", y para el
			// consumidor eso quiere decir lo mismo. La ambigüedad es aceptable acá —al revés que en
			// `motivo_no_preguntar`— porque no hay ninguna acción que dependa de distinguirlas.
			//
			// ES readCompatible: ADD COLUMN sobre `devices` con default, y ninguna consulta
			// existente cambia de resultado.
			up: func(x execQuerier) error {
				return agregarColumnaSiFalta(x, "devices", "servicios_error",
					"servicios_error TEXT NOT NULL DEFAULT ''")
			},
		},
		{
			version:        54,
			name:           "quien_esta_latiendo_sobre_esta_fila",
			readCompatible: true,
			// DOS AGENTES LATIENDO SOBRE UNA FILA SON INDISTINGUIBLES DE UNO.
			//
			// El latido no manda NADA que identifique al proceso emisor: sólo la credencial, que es
			// de la MÁQUINA y no del proceso. Así que dos agentes corriendo a la vez —un servicio
			// más una corrida a mano, una instalación duplicada, un zombi del binario renombrado—
			// escriben los dos sobre la misma fila y el cerebro ve un único agente sano latiendo el
			// doble de seguido.
			//
			// NO ES HIPOTÉTICO: es lo que hizo que A92 se cerrara con el problema todavía puesto. El
			// diagnóstico se hizo midiendo la CADENCIA (un diente de sierra de 37,8 s contra 25,8 s
			// del vecino), que es una inferencia estadística sobre un efecto de segundo orden — y
			// eso sólo se puede hacer mirando a mano, sabiendo que hay que mirar.
			//
			// `emisor` es un identificador OPACO que el agente genera UNA VEZ por proceso. No es el
			// PID: un PID se repite entre reinicios y entre máquinas, y lo que hay que distinguir no
			// es «qué número de proceso» sino «¿el que late ahora es el mismo de recién?».
			//
			// `emisor_desde` es CUÁNDO empezó a latir el emisor actual, y es lo que convierte esto
			// en una serie útil: con un solo agente esa marca envejece; con dos alternándose vuelve
			// a cero en cada latido y se queda cerca de cero para siempre. Guardar un contador de
			// cambios habría dado lo mismo sin poder decir «desde cuándo», que es lo que un operador
			// necesita para saber si ya lo arregló.
			//
			// Las dos arrancan VACÍAS, y el vacío significa «este agente no lo declara» —un binario
			// anterior a la pieza—: no «cambió recién». La serie se OMITE en ese caso, que es la
			// regla de este plano.
			//
			// ES readCompatible: dos ADD COLUMN sobre `devices` con default, y ninguna consulta
			// existente cambia de resultado.
			up: func(x execQuerier) error {
				if err := agregarColumnaSiFalta(x, "devices", "emisor",
					"emisor TEXT NOT NULL DEFAULT ''"); err != nil {
					return err
				}
				return agregarColumnaSiFalta(x, "devices", "emisor_desde",
					"emisor_desde TEXT NOT NULL DEFAULT ''")
			},
		},
		{
			version:        55,
			name:           "cuando_se_revoco_esta_maquina",
			readCompatible: true,
			// REVOCAR UNA MÁQUINA RESUELVE SUS ALERTAS EN SILENCIO.
			//
			// `revoked` es una BANDERA y no un DELETE, así que la fila queda — pero la máquina sale
			// del export, sus series se vuelven obsoletas, y TODAS sus alertas se resuelven solas.
			// Del otro lado del canal, «se arregló» y «la sacamos del inventario» llegan como el
			// MISMO mensaje: un `[RESOLVED]` sin una palabra que los distinga.
			//
			// No es cosmético. Quien lee ese resolved concluye que el problema se atendió, y la
			// máquina con el disco lleno que se dio de baja sin arreglar queda cerrada en la cabeza
			// de todos. Es el mismo defecto que este track ya arregló dos veces con otros nombres:
			// dos causas distintas produciendo la misma señal.
			//
			// GUARDA EL CUÁNDO Y NO UN BOOLEANO: `revoked` ya dice que pasó. Lo que faltaba es
			// poder emitir una serie ACOTADA EN EL TIEMPO —«esta máquina se dio de baja recién»—
			// para que el aviso acompañe a las resoluciones y después desaparezca solo. Con un
			// booleano, la serie viviría para siempre y el aviso se volvería permanente, que es
			// otra forma de no decir nada.
			//
			// Arranca VACÍA en las máquinas ya revocadas, y el vacío significa «se revocó antes de
			// que esto existiera»: la serie no se emite para ellas, que es lo correcto — su baja ya
			// pasó y nadie está esperando un aviso.
			//
			// ES readCompatible: ADD COLUMN sobre `devices` con default.
			up: func(x execQuerier) error {
				return agregarColumnaSiFalta(x, "devices", "revoked_at",
					"revoked_at TEXT NOT NULL DEFAULT ''")
			},
		},
		{
			version:        56,
			name:           "indices_observation_relations",
			readCompatible: true,
			// LA TABLA DE SINAPSIS NO TENÍA NI UN ÍNDICE PROPIO.
			//
			// `observation_relations` traía sólo los dos autoindex que SQLite crea solo: el de la
			// PRIMARY KEY y el del UNIQUE. Los dos arrancan por `source_id`, así que sirven para
			// «¿qué sale de esta observación?» y para nada más. Toda lectura por el OTRO extremo
			// —`target_id`— y toda lectura por `status` son scan completo de la tabla.
			//
			// Y son lecturas del camino caliente, no de un reporte: la cola de conflictos busca
			// por status ('pending'), y la corroboración y el supersede entran por el target.
			//
			// Hoy son 252 filas y no se nota. Esa es exactamente la razón de ponerlo ahora: el
			// costo de crear los índices con la tabla chica es de milisegundos, y la tasa de
			// llegada medida de la cola de conflictos es de ~10 por día. El scan se vuelve visible
			// cuando ya molesta.
			//
			// Es readCompatible: un índice no cambia el RESULTADO de ninguna consulta, sólo el
			// plan. Un lector viejo sobre esta base devuelve exactamente lo mismo que antes.
			up: func(x execQuerier) error {
				for _, ddl := range []string{
					`CREATE INDEX IF NOT EXISTS idx_obs_rel_target ON observation_relations(target_id)`,
					`CREATE INDEX IF NOT EXISTS idx_obs_rel_status ON observation_relations(status)`,
				} {
					if _, err := x.Exec(ddl); err != nil {
						return err
					}
				}
				return nil
			},
		},
		{
			version:        57,
			name:           "expansion_como_relevancia_elegida",
			readCompatible: true,
			// LA ÚNICA SEÑAL EXÓGENA QUE TIENE EL RECALL SE ESTABA GUARDANDO EN LA COLUMNA DE LA
			// ENDÓGENA.
			//
			// `bumpAccess` lo llaman DOS caminos que no significan lo mismo:
			//   - recall.go: «te serví estos gists». Lo escribe el ranker sobre su propia salida.
			//     Es el lazo endógeno que la invariante N4 existe para amortiguar.
			//   - expand.go: «de todos los gists que me diste, ÉSTE lo quiero entero». Eso lo
			//     decidió el agente DESPUÉS de leer los titulares; el ranker no lo puede fabricar.
			//     Es relevancia ELEGIDA, no derivada.
			//
			// Las dos sumaban en `access_count`, así que la segunda quedaba indistinguible de la
			// primera y heredaba su descuento. Y sin poder separarlas, el banco de recall no tiene
			// una etiqueta de relevancia que no derive de la similitud: hoy las saca del
			// `topic_key`, o sea que los documentos relevantes de una consulta SON los que se
			// parecen entre sí. Por eso el costo de relevancia que el banco le cobra a MMR es una
			// COTA SUPERIOR, inflada por construcción (ver internal/recalleval/mmr_real_test.go).
			//
			// ESTO NO CAMBIA EL RANKING, y es deliberado. `access_count` sigue recibiendo
			// exactamente los mismos incrementos que antes, de los mismos dos caminos: las columnas
			// nuevas SE SUMAN, no reemplazan. Separar la señal y usarla para rankear son dos
			// decisiones distintas, y la segunda necesita una medición que todavía no existe —
			// justamente la que ésta desbloquea. Hacer las dos juntas sería cambiar el ranker
			// basándose en lo que uno espera que la medición diga.
			//
			// ARRANCA EN CERO Y NO SE PUEDE RELLENAR. Hay 53 expansiones en el ledger de uso, pero
			// `tool_invocations` no guarda argumentos (invariante L1, y está bien que así sea), y
			// dentro de `access_count` las dos señales ya están sumadas sin forma de restarlas. La
			// historia previa está perdida: esto empieza a medir desde acá, no desde antes.
			//
			// Es readCompatible: dos ADD COLUMN con default sobre `observations`. Ningún lector
			// viejo consulta estas columnas, así que ninguna respuesta existente cambia.
			up: func(x execQuerier) error {
				if err := agregarColumnaSiFalta(x, "observations", "expand_count",
					"expand_count INTEGER NOT NULL DEFAULT 0"); err != nil {
					return err
				}
				return agregarColumnaSiFalta(x, "observations", "last_expanded", "last_expanded DATETIME")
			},
		},
	}
}

// pisoDeLectura es la migración más baja que un binario tiene que conocer para poder LEER una
// base migrada por `migs` sin devolver datos distintos.
//
// Se deriva: es la versión más alta entre las migraciones NO readCompatible. Por debajo de ella
// hay al menos un cambio que le mueve el resultado a una consulta existente; de ella para arriba,
// todo lo que se agregó es invisible para el lector viejo y no le cambia ninguna respuesta.
//
// Cero significa «ninguna migración rompe lectura», no «no se sabe»: el caso «no se sabe» no
// existe acá porque la lista siempre está completa en el binario que la calcula. El «no se sabe»
// aparece del otro lado —al LEER el piso de una base que no lo tiene grabado— y ahí se distingue
// con un booleano, no con un cero.
func pisoDeLectura(migs []migration) int {
	piso := 0
	for _, m := range migs {
		if !m.readCompatible && m.version > piso {
			piso = m.version
		}
	}
	return piso
}

// PisoDeLectura es el piso que declara ESTE binario, para diagnóstico y para el despliegue.
func PisoDeLectura() int { return pisoDeLectura(schemaMigrations()) }

// registrarPisoDeLectura deja en la base el piso derivado por el binario que acaba de migrar.
//
// Se llama DESPUÉS de aplicar las migraciones y no dentro del runner: `applyMigrations` corre
// también con migraciones sintéticas en los tests, sobre bases donde `schema_floor` no existe, y
// un runner que escribiera ahí dejaría de ser el runner puro que esos tests ejercitan.
//
// El fallo NO es fatal para el que escribe —la base quedó migrada y usable— pero sí se propaga:
// un piso que no se grabó deja a los binarios viejos sin evidencia, y prefiero enterarme al
// escribir que descubrirlo cuando otro se niegue a abrir.
func registrarPisoDeLectura(db *sql.DB, migs []migration) error {
	piso := pisoDeLectura(migs)
	alcanzado := 0
	for _, m := range migs {
		if m.version > alcanzado {
			alcanzado = m.version
		}
	}
	_, err := db.Exec(`INSERT INTO schema_floor (id, read_floor, set_by_schema, set_at)
		VALUES (1, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			read_floor    = excluded.read_floor,
			set_by_schema = excluded.set_by_schema,
			set_at        = excluded.set_at`, piso, alcanzado)
	if err != nil {
		return fmt.Errorf("error al grabar el piso de lectura: %w", err)
	}
	return nil
}

// leerPisoDeLectura devuelve el piso grabado en la base. El segundo valor distingue «no hay piso
// grabado» de «el piso es 0», que son cosas distintas y se leerían igual con un solo int: la
// primera es una base migrada por un binario anterior a esta pieza —sin evidencia, hay que
// negarse— y la segunda sería un permiso amplio.
//
// No devuelve error a propósito: cualquier fallo —tabla ausente, fila ausente, base ilegible— es
// la MISMA respuesta operativa («no hay evidencia»), y darle tres formas al mismo no-sé invita a
// que algún caller trate una de ellas como un sí.
func leerPisoDeLectura(db *sql.DB) (int, bool) {
	var piso int
	if err := db.QueryRow(`SELECT read_floor FROM schema_floor WHERE id = 1`).Scan(&piso); err != nil {
		return 0, false
	}
	return piso, true
}

// EsquemaEsperado es la versión de esquema a la que apunta ESTE binario.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// EXISTE PARA QUE EL GUION DE DESPLIEGUE NO TENGA QUE ADIVINARLA
//
// `deploy/redesplegar-cerebro.sh` verificaba la migración con un número TIPEADO A MANO
// (`[[ "$ESQUEMA" -ge 37 ]]`). Entre la migración 37 y la 44 esa comprobación siguió pasando y
// dejó de verificar nada: 44 ≥ 37 es cierto, y también lo sería si la migración se hubiera
// quedado a mitad de camino en la 40. Una comprobación que no puede ponerse roja se ve idéntica
// a una que funciona — que es el defecto que este repo persigue en todos lados menos, hasta hoy,
// en la herramienta que lo despliega.
//
// Derivada del binario, la comprobación se actualiza sola cada vez que se agrega una migración,
// y nadie tiene que acordarse.
func EsquemaEsperado() int { return latestSchemaVersion() }

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
	migs := schemaMigrations()
	if err := applyMigrations(db, migs); err != nil {
		return err
	}
	// El piso se re-graba en CADA arranque y no sólo cuando hubo migraciones que aplicar: si no,
	// una base ya migrada por un binario anterior a esta pieza se quedaría para siempre sin piso
	// —nadie volvería a tocarla— y los binarios viejos no tendrían de dónde sacar la evidencia.
	return registrarPisoDeLectura(db, migs)
}

// applyMigrations es el runner: lee user_version y aplica, en orden, cada migración
// con versión mayor a la actual. Cada migración corre en SU PROPIA transacción y
// fija user_version dentro de esa misma tx, de modo que aplicar la migración y
// avanzar la versión es atómico: si `up` falla, se hace rollback y la versión no
// avanza (la próxima apertura reintenta). Separar el runner de schemaMigrations()
// permite testearlo con migraciones sintéticas.
func applyMigrations(db *sql.DB, migs []migration) error {
	// LA LISTA SE REVISA ANTES DE TOCAR LA BASE, Y ESTO NO ES PARANOIA: es el modo de falla que
	// dos ramas abiertas producen SOLAS.
	//
	// El bucle de más abajo hace `if m.version <= current { continue }`. Con dos migraciones que
	// declaran el MISMO número —lo normal cuando dos ramas agregan una cada una al final del
	// archivo y git las auto-mergea sin marcar conflicto— aplica la primera, `current` queda en
	// ese número, y la SEGUNDA CAE EN EL `continue`. Sin error, sin warning, y `user_version`
	// termina afirmando que se aplicó todo. No deja la base en rojo: la deja INCOMPLETA EN VERDE,
	// que es peor, porque el arranque siguiente no reintenta nada.
	//
	// POR QUÉ ACÁ Y NO SÓLO EN UNA PRUEBA. Una guarda de test protege al REPO: impide que el
	// archivo malo se mergee. Esto protege a un BINARIO YA CONSTRUIDO desde un merge que nadie
	// revisó —el release de ayer, el binario que alguien copió a mano, la rama de un tercero—, que
	// es el único caso en que este defecto llega a una base de producción. Las dos hacen falta y
	// ninguna reemplaza a la otra.
	//
	// SE EXIGE ESTRICTAMENTE CRECIENTE y no sólo «sin repetidos», porque el desorden tiene el mismo
	// efecto: una migración con número menor que la anterior también entra al `continue` y se
	// saltea. Un solo control cubre las dos formas.
	for i := 1; i < len(migs); i++ {
		if migs[i].version > migs[i-1].version {
			continue
		}
		if migs[i].version == migs[i-1].version {
			return fmt.Errorf("migración %d declarada DOS VECES (%q y %q): el runner aplicaría sólo la primera y saltearía la segunda EN SILENCIO, dejando la base incompleta con user_version diciendo que se aplicó todo. Casi siempre es un merge de dos ramas que agregaron una migración cada una; renumerá la que tenga menos datos atrás",
				migs[i].version, migs[i-1].name, migs[i].name)
		}
		return fmt.Errorf("las migraciones no están en orden creciente: %d (%q) viene después de %d (%q). El runner saltea toda migración con versión menor o igual a la ya aplicada, así que la de atrás no correría nunca y nadie se enteraría",
			migs[i].version, migs[i].name, migs[i-1].version, migs[i-1].name)
	}

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
		// DOS SITUACIONES DISTINTAS QUE ANTES SE CONTESTABAN IGUAL. Si el piso grabado en la base
		// está a la altura de este binario, la base es MÁS NUEVA pero SE PUEDE LEER: lo único que
		// cambió desde acá para arriba es invisible para este lector. Se sigue devolviendo un
		// error —abrir para escribir sería un error— pero uno que el caller puede distinguir para
		// ofrecer sólo lectura en vez de negarse del todo.
		//
		// Sin piso grabado no hay evidencia, y la falta de evidencia se responde con el NO de
		// siempre: una base migrada por un binario anterior a esta pieza no dice nada sobre qué
		// puede leer quien la abre.
		if piso, hay := leerPisoDeLectura(db); hay && latest >= piso {
			// El texto describe una CAPACIDAD, no una promesa: dice que los datos son legibles
			// para este binario, no que alguien los vaya a servir. Hoy ningún caller abre en
			// sólo lectura todavía, y un mensaje que dijera «se abre en sólo lectura» sería
			// exactamente la clase de afirmación que después se cita como si fuera el
			// comportamiento.
			return fmt.Errorf("%w: %w: la base está en el esquema v%d y este binario llega a v%d, pero el piso de lectura de la base es v%d y este binario lo alcanza: sus datos son LEGIBLES para él, aunque no puede migrarla ni escribirla",
				ErrSchemaTooNew, ErrEsquemaLegible, current, latest, piso)
		}
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
