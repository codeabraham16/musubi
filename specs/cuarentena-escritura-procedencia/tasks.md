# Tareas — Cuarentena de escritura y procedencia (F4)

Estado: **completo**. Build, vet, `golangci-lint` (0 issues) y las 17 suites del repo en verde.

- [x] **T1 — Esquema.** Migración **v22** más las tres columnas en `addObservationColumns`:
      `provenance TEXT NOT NULL DEFAULT 'human'`, `confidence REAL NOT NULL DEFAULT 1.0`,
      `quarantined INTEGER NOT NULL DEFAULT 0`.

      Hay que tocar **los dos lados** (la trampa que documenta la v21): la baseline no re-corre en
      una base ya migrada, y la migración no corre en una base nueva. La v22 **delega** en
      `addObservationColumns` en vez de repetir la DDL — ver T7, que es donde eso se pagó.

- [x] **T2 — El seam.** `visibleObsPredicate` pasa a
      `archived = 0 AND superseded_by IS NULL AND quarantined = 0`. Diez consultas en nueve
      archivos lo concatenan, así que Q0 se cumple en un solo lugar, y **Q6 sale gratis** porque
      la cola del sync saliente usa el mismo predicado.

- [x] **T3 — `braingraph` adoptado al predicado canónico.** No estaba en la lista: filtraba con un
      `archived = 0` propio. Como alimenta el dashboard, una observación en cuarentena se habría
      dibujado ahí como neurona. **Cambio de comportamiento visible y decidido**: las observaciones
      REEMPLAZADAS tampoco se dibujan más (inconsistencia anterior a esta fase, corregida de paso).

- [x] **T4 — Motor** (`internal/memory/observation_quarantine.go`): taxonomía cerrada,
      `obsStamp`, `ProposeObservation`, `CorroborateObservation` + variante `Ctx` con la guarda
      multi-tenant, `IsQuarantined`, `ObservationStamp`, y los cinco errores tipados.

      `saveObservation` recibe el sello y **no lo pisa en el UPSERT**. Eso no es una omisión: si lo
      pisara, un `save_observation` sobre el id de una observación en cuarentena la limpiaría en
      silencio (ver el sabotaje 2, que es exactamente ese escenario).

- [x] **T5 — Tools MCP**: `musubi_propose_observation` y `musubi_corroborate`. El schema **no
      expone** `provenance` ni `quarantined` — no es que se ignoren si los mandan, es que no
      existen. `confidence` viaja como puntero para distinguir "no la mandaron" de "mandaron 0".

- [x] **T6 — Procedencia en el recall (Q3)**: `RecallItem.Provenance`, poblado desde `candidate`
      por las cuatro consultas y filtrado por `stampProvenance`, que **calla lo que es `human`**.
      Si todas las memorias llegaran marcadas, el sello sería ruido y dejaría de leerse.

- [x] **T7 — Dos bugs que cazó la suite, no yo.**

      1. La v22 con la DDL escrita a mano rompía **toda base nueva**: la baseline ya agregaba las
         columnas y `ALTER TABLE ADD COLUMN` no es idempotente ⇒ `duplicate column name` y el engine
         no abría. Fixeado delegando en `addObservationColumns`, que sí consulta antes de tocar.
      2. `TestMigrationV11OutboxSchema` fija la versión de esquema esperada. Es una guarda
         deliberada y molesta a propósito: subirla obliga a darse cuenta de que el esquema cambió.

- [x] **T8 — 15 tests de invariantes** en `observation_quarantine_test.go`.

      Q0 se prueba **por cada camino de listado por separado** (léxico, priming, grafo neuronal),
      y cada uno lleva un **control**: si la observación de control no aparece, el test falla por
      "no está probando nada" en vez de pasar en falso.

      El archivo se llama `observation_quarantine_test.go` y no `quarantine_test.go` porque **ese
      nombre ya estaba tomado** por los tests de la cuarentena de HECHOS del grafo (pilar Cognición
      F1). Escribirlo ahí habría borrado cuatro tests reales.

- [x] **T9 — Sabotaje: 6 mutaciones, cada una puso en rojo el test de su invariante.**

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | Sacar `quarantined = 0` del predicado canónico | Q0 | rojo — los **3** caminos (`FUGA Q0`) |
      | El UPSERT pisa `provenance`/`quarantined` | Q2 | rojo (`FUGA Q2`, sale de cuarentena Y se lava el sello) |
      | `Corroborate` pone `provenance='human'` | Q3 y Q4 | rojo los dos |
      | Recortar `confidence` en vez de rechazarla | Q7 | rojo en los 3 valores fuera de rango |
      | Sacar la guarda de `PromoteObservationCtx` | Q6 | rojo |
      | Que `Propose` deduplique por content_hash | Q2 | rojo (devuelve el id de la autoritativa) |

- [x] **T10 — Golden y conteos.** Dos tools nuevas ⇒ 47 → 49: `toolslist.golden.json` regenerado
      con `-update`, y los cuatro tests/README que fijan el número.

## Fuera de alcance, dicho de frente

- **La hidratación por id explícito** (`memory_expand`, banda, detalle de conflictos) sigue leyendo
  observaciones en cuarentena. Q0 impide **descubrir**, no impide **leer lo que ya sabés que
  existe**. Está en la spec como Q0b y tiene test propio para que, si alguien lo cambia, sea una
  decisión y no un accidente.
- **No hay auditoría retroactiva.** Si alguien ya guardó una respuesta de `musubi_ask` como
  observación normal, esta fase no la encuentra ni la marca. Cierra la puerta de acá en adelante.
- **Un caller puede seguir usando `save_observation` para contenido de LLM.** Q2 protege la puerta
  de cuarentena; no obliga a usarla. Cerrarlo del todo exigiría detectar texto generado por IA, que
  es explícitamente lo que la propuesta dice que esto no es.
- **`musubi_ask` no propone automáticamente.** Devuelve la respuesta al caller y ahí termina.
  Cablear "guardá esto en cuarentena" dentro de `ask` es una decisión de producto aparte.
