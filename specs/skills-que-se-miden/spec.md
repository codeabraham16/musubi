# Spec — El arsenal se mide

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

```sql
CREATE TABLE IF NOT EXISTS skill_usage (
  skill      TEXT     NOT NULL,
  project_id TEXT     NOT NULL DEFAULT '',
  evidence   TEXT     NOT NULL,   -- alcance | glob | comodin | '' (para los pedidos)
  kind       TEXT     NOT NULL,   -- resolved | body_sent | body_requested
  n          INTEGER  NOT NULL DEFAULT 0,
  first_at   DATETIME NOT NULL,
  last_at    DATETIME NOT NULL,
  PRIMARY KEY (skill, project_id, evidence, kind)
);
```

```go
// memory
type SkillEvent struct { Skill, ProjectID, Evidence, Kind string }
func (e *DbEngine) RecordSkillEvents(ctx context.Context, batch []SkillEvent) error
func (e *DbEngine) SkillUsageCtx(ctx context.Context, dias int) ([]SkillUsage, error)
```

Y una tool nueva: `musubi_skill_usage`.

---

## H1 · Se cuenta lo que pasa, y se distingue

### A1 — Resolver cuenta una activación por skill matcheada, con su evidencia

Una resolución que activa 10 skills produce 10 conteos, cada uno con su `evidence`. Sin la evidencia
el dato no sirve para la lectura que importa —«matcheó siempre por comodín»—, que es justamente la
que descubre un `*` que merece volverse `applies_to`.

### A2 — «Matcheó» y «viajó su cuerpo» son cosas distintas

Una skill que entró por comodín cuenta `resolved` pero **no** `body_sent`. Si se contaran juntos, la
lectura «ocupa contexto y nadie la abrió» sería imposible de escribir: es exactamente la diferencia
entre las dos.

### A3 — Pedir una skill por nombre cuenta como pedido de cuerpo

`musubi_list_skills` con `query` que nombra una skill local cuenta `body_requested` para ella. Es la
señal que #277 hizo observable: el llamador vio el nivel 1 y decidió gastar los tokens.

Un `query` vacío —mirar el arsenal entero— **no** cuenta para nadie: no es un pedido, es una lista.

---

## H2 · La medición no puede romper nada

### A4 — Un sink que falla no hace fallar la resolución

Misma garantía que el ledger de tools (L2). Una tool que empieza a fallar porque su telemetría falla
es peor que no tener telemetría.

### A5 — No se escribe a disco en el camino caliente

El conteo es un `append` bajo mutex y nada más. El handler corre con `dispatchMu` tomado; escribir a
disco ahí alargaría el lock de toda tool, y la goroutine de flush no puede tomar `dispatchMu` sin
deadlock (la trampa que documenta `maybeTriggerMaintenance`).

### A6 — El buffer tiene techo y lo descartado se dice

Igual que `ledgerBufferCap`: la telemetría jamás puede ser el motivo por el que el daemon se quede
sin memoria, y un ledger que pierde datos en silencio es peor que no tenerlo.

---

## H3 · El dato está acotado y es honesto

### A7 — Los contadores están acotados por proyecto

Track 19: la lectura se acota al `project_id` de la credencial. Un arsenal federado no puede filtrar
qué skills usa otro tenant.

### A8 — Una skill del arsenal que nunca matcheó aparece con 0, no ausente

«0 activaciones» es la lectura más accionable de las tres, y una fila ausente es indistinguible de
«no hay dato». Es el mismo criterio que A7 del spec de niveles: lo que falta se declara.

### ★ A9 — La herramienta NO llama «utilidad» a lo que mide

La respuesta habla de `resolved`, `body_sent` y `body_requested`. No hay campo `utilidad`, ni
`score`, ni ranking. Que el pedido de cuerpo se parezca a la utilidad no lo convierte en utilidad, y
ponerle ese nombre sería decidir con opinión pero con un número al lado para que parezca medición —
el error exacto que el §7 denuncia.

### A10 — Las candidatas se marcan, no se retiran

La respuesta puede decir `candidata: "retiro"` o `candidata: "alcance"`, con el patrón que la
justifica. Nada se borra ni se apaga solo: retirar es del dueño del arsenal, igual que
`musubi_promote_skill` es explícita a propósito.

---

## Alcance declarado

- **Contadores, no eventos.** La tabla queda acotada al tamaño del arsenal y no necesita purga.
- **`skill_decisions` no se toca.** Mide otra pregunta —instalarla o no— y las dos son legítimas.
- **El camino de `.claude/skills/` no se mide.** Ahí quien decide cargar el cuerpo es el consumidor y
  Musubi no ve esa decisión. Es un límite real; se declara y no se disimula.
- **Cero inferencia.** Contar y comparar contadores.
