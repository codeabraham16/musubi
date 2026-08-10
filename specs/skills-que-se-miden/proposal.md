# Propuesta — Medir el arsenal sin mentir sobre lo que se mide

Track **Forja global**, problema §7 de `specs/forja-global/investigacion.md`, el último que quedaba:

> Nadie mide si una skill sirvió. `skill_decisions` guarda «acepté o rechacé **instalarla**», no «me
> sirvió cuando se activó». El ledger de uso mide **tools**, no skills. Cualquier plan que prometa
> «potenciar las skills» sin resolver esto se va a evaluar con opinión.

Verificado hoy contra el esquema, y es exacto:

```sql
skill_decisions   (skill_id, name, decision, reason, created_at)   -- instalarla o no
tool_invocations  (tool, outcome, duration_us, project_id, ...)    -- SIN argumentos, a propósito
```

`tool_invocations` no guarda argumentos —el esquema no tiene dónde, y eso es una garantía de
privacidad que no se toca— así que ni siquiera indirectamente se puede saber qué skill se activó.

## ★ El instrumento lo creó #277, sin proponérselo

Antes de los niveles, cada resolución entregaba **todos** los cuerpos. No había ninguna decisión que
observar: la skill llegaba entera se usara o no.

Ahora sí la hay. El llamador recibe el nivel 1 con la cláusula `cuando` y **decide** si el cuerpo
vale sus tokens. Esa decisión es gratis de observar, no requiere que nadie declare nada, y es lo más
cerca de «sirvió» que se puede llegar sin un modelo.

## Lo que se puede medir, y lo que no

| pregunta | ¿model-free? | de dónde sale |
|---|---|---|
| ¿matcheó? | sí, completo | el resolvedor |
| ¿por qué evidencia matcheó? | sí, completo | `ComoMatcheo`, de #277 |
| ¿viajó su cuerpo? | sí, completo | la selección de #277 |
| ¿le pidieron el cuerpo? | sí, **parcial** | `musubi_list_skills` con `query` |
| **¿sirvió?** | **NO** | es juicio |

**La regla de honestidad de esta propuesta: la herramienta no puede llamar «utilidad» a lo que mide.**
Mide **activaciones** y **pedidos**. Que el pedido de cuerpo se parezca a la utilidad no lo convierte
en utilidad, y nombrarlo así sería el mismo error que el §7 denuncia — decidir con opinión, pero con
un número al lado para que parezca medición.

El pedido es parcial a propósito y hay que decirlo: `musubi_list_skills` también se usa para mirar el
arsenal, así que un pedido no siempre viene de una resolución. Se cuenta como lo que es.

## Para qué sirve el dato: tres lecturas accionables

| patrón | qué significa | qué hacer |
|---|---|---|
| matcheó N veces, cuerpo nunca enviado **ni pedido** | ocupa contexto en cada resolución y nadie la abrió | candidata a **retiro** |
| matcheó **siempre por comodín**, pero le piden el cuerpo | aplica de verdad y no puede decir cuándo | candidata a **`applies_to`** |
| 0 activaciones | ni siquiera matchea | está **muerta** |

La segunda es la más valiosa: es la única forma de descubrir un `*` que merece volverse alcance
declarado sin que alguien lo adivine.

## Contadores, no un log de eventos

Un evento por activación escribiría ~10 filas por resolución, necesitaría purga y traería el problema
de retención del ledger de tools. Y no hace falta: las preguntas de mantenimiento son **«cuántas
veces»** y **«cuándo fue la última»**, no series de tiempo.

Un contador por `(skill, proyecto, evidencia)` acota la tabla al tamaño del arsenal. No crece con el
uso.

## Se escribe con el buffer que ya existe

No se inventa una cañería nueva: se extiende `usageLedger`. Mismo ticker, mismo techo de buffer,
misma regla de no reencolar — y sobre todo la misma razón de fondo, escrita en su encabezado: el
handler corre **con `dispatchMu` tomado**, y escribir a disco ahí adentro alargaría el lock en el
camino caliente de toda tool. La goroutine de flush tampoco puede tomar `dispatchMu`: es la trampa de
deadlock que documenta `maybeTriggerMaintenance`.

Y hereda la propiedad que más importa: **el contador jamás puede hacer fallar una llamada.**

## Lo que esta propuesta NO hace

- **No juzga.** No hay puntaje de calidad, no hay ranking, no hay «esta skill es mejor».
- **No retira nada sola.** Marca candidatas; retirar es del dueño del arsenal, igual que
  `musubi_promote_skill` es explícita a propósito.
- **No toca `skill_decisions`.** Mide otra cosa y las dos preguntas son legítimas.
- **No mide el camino de `.claude/skills/`.** Ahí quien decide cargar el cuerpo es el consumidor, y
  Musubi no ve esa decisión. Es un límite real y se declara.
