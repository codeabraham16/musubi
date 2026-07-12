# Proposal — registros-inmutables-y-doble-aviso

## Intención

**El PRIMER uso real de la banda ciega (v0.87.0) encontró dos defectos en lo que se acababa de shippear.** Un solo `musubi_save_observation` (una nota destilando el aprendizaje de la sesión) produjo:

```
8 pendientes de UN save:
  0.88  sdd/banda-ciega-vecinos-al-guardar/proposal
  0.86  sdd/banda-ciega-vecinos-al-guardar/archive
  0.85  git-commit                                   <-- ADEMÁS salió en la banda
  0.76  sdd/conflictos-artefactos-mismo-cambio/archive
  0.74  sdd/banda-ciega-vecinos-al-guardar/design
  0.72  git-commit
  0.71  git-commit
  0.61  sdd/conflictos-artefactos-mismo-cambio/proposal
```

## Defecto 1 — DOBLE AVISO (la banda muestra algo que YA está en la cola)

El diseño de la banda decía explícitamente: *«si alcanza `CosineFloor`, el par YA es una relación `pending` y el agente lo ve por el camino de siempre. Avisar dos veces por lo mismo entrena a ignorar el aviso.»*

Pero la condición implementada fue `cos >= CosineFloor` — y **eso es una proxy equivocada**. A `pending` se entra por **DOS puertas** (`relevantPair`: léxico **O** coseno). Ese commit entró por la **léxica**, con coseno **0.849** — justo por debajo del piso. La banda, mirando **sólo el coseno**, lo mostró igual.

**Lo que se quería decir es más simple y más fuerte: la banda muestra lo que la cola NO muestra. Es su COMPLEMENTO, no un rango de coseno.**

## Defecto 2 — Se le puede pedir un veredicto IMPOSIBLE

Las 8 pendientes son la nota-resumen contra **los artefactos del trabajo que resume**. Y la pregunta que las mata es:

> **¿Qué veredicto podría emitirse?** El único disponible sería *«esta nota reemplaza al commit»* o *«reemplaza al spec»*. **Eso no significa nada.** Un commit es lo que **PASÓ**; un contrato SDD es lo que se **ACORDÓ**. No se pueden **des-hacer**. Ninguna nota los puede reemplazar.

Son relaciones **cuyo veredicto no puede ser otra cosa que `related`**. Pedir un juicio que ya está decidido de antemano es, por definición, **ruido**.

### La asimetría es lo que hace la regla correcta (y lo que salva el diseño anterior)

**Al revés SÍ importa.** Un commit que dice *«feat: migrar de X a Y»* **sí** puede volver obsoleta una nota que decía *«usamos X»*: el commit es **evidencia** de que la nota quedó vieja. Ese caso hay que **conservarlo**.

> **La regla: un REGISTRO HISTÓRICO (`git-commit`, contrato `sdd/*`) nunca puede ser el DESTINO de una relación propuesta por algo de OTRA clase.** Superseder uno sería **ocultar historia**.

Como es **asimétrica**, no rompe nada de lo ya shippeado:

| par (source → target) | hoy | con la regla |
|---|---|---|
| nota → commit | pending (ruido) | **bloqueado** |
| nota → sdd | pending (ruido) | **bloqueado** |
| **commit → nota** | pending | **se conserva** (el commit es evidencia de que la nota envejeció) |
| **commit → commit** | pending | **se conserva** (el gemelo del squash, la redundancia real) |
| **sdd → sdd** (cambios distintos) | pending | **se conserva** (misma clase) |
| commit ↔ sdd | ya bloqueado (#203) | igual |

## Alcance

- **F1** — La banda excluye **todo** par que la cola ya vaya a proponer (usa `relevantPair`, la **misma** función que decide si entra a la cola). La banda pasa a ser, literalmente, el **complemento** de la cola.
- **F2** — Guarda nueva: si el **target** es un registro histórico (`git-commit` o `sdd/*`) y el **source** NO es de su misma clase ⇒ **no se propone relación**. Se suma a las guardas estructurales de #203, en el **mismo choke point**.

## Fuera de alcance (explícito)

- NO se toca el piso de coseno, ni el AND-gate, ni el gate de novedad, ni `band_floor`.
- NO se ocultan ni archivan observaciones (las guardas **evitan crear**, no borran).
- NO se hace limpieza retroactiva en el código (las 8 pendientes ya creadas se resuelven a mano).

## Estrategia de rollback

Revertir el PR restaura el comportamiento actual. Sin migración, sin cambio de esquema, sin datos tocados.

## Riesgos

- **F2 podría tapar una supersesión legítima de un commit por otra cosa.** Mitigación: no existe tal caso — un commit **ya ocurrió**; nada lo puede des-hacer. Si un commit posterior lo revierte, **ése es un commit** (misma clase ⇒ **no** se bloquea).
- **F1 podría dejar la banda vacía si el piso léxico es generoso.** Es el comportamiento **correcto**: si el par ya está en la cola, mostrarlo otra vez no agrega información. La banda existe para lo que la cola **NO** ve.
- **Es una corrección de algo shippeado hace horas.** Vale decirlo en voz alta en vez de disimularlo: el dogfooding encontró en el primer uso lo que el diseño no vio.
