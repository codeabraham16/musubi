# Proposal — recall-fusion-quality

## Intención

Corregir dos defectos **medidos** en la fusión del recall híbrido que degradan la calidad semántica y la robustez, ambos en `internal/memory/recall.go`, 100% model-free:

- **Q1 — Piso de coseno en el pool vectorial.** `augmentWithVectorPool` (recall.go:307) hace `vecRank[r.ID] = i` y **descarta la similitud coseno** que `SearchObservations` sí calcula. Sin umbral, inyecta hasta `limit` (50) vecinos con peso RRF pleno: un coseno **0.42 pesa igual que 0.95**. Medido en vivo (2026-07-11): el recall semántico traía ruido temático en vez de clavar la respuesta. Fix: descartar los resultados vectoriales bajo un piso `tau` **antes** de armar `vecRank` y traer los `missing`.

- **Q2 — Degradación elegante ante FTS malformado.** `Recall` (recall.go:127-130) retorna en error de `recallCandidates` **antes** de `augmentWithVectorPool`: un FTS corrupto tumba TODO el recall, aunque haya `QueryVector` semántico servible. Fix: ante un error de corrupción del FTS, logear + telemetrizar y **seguir** con pool léxico vacío, dejando que el pool vectorial y/o el fallback por recencia llenen.

## Por qué ahora

Sale de la investigación (deep-research 96 ag.) + auditoría verificada (13 ag.) del techo semántico. Q1 es el ítem de **máximo ROI** del roadmap: arregla directo la calidad semántica mediocre que medimos al encender los embeddings. Q2 es su complemento de robustez: sin él, el recall híbrido queda a merced de la mitad léxica.

## Alcance

- **Q1:** un piso `tau` (config nueva, default conservador) aplicado a los resultados de `SearchObservations` dentro de `augmentWithVectorPool`. Solo afecta el recall HÍBRIDO (con `QueryVector`); el recall 100% léxico queda idéntico.
- **Q2:** manejo del error de `recallCandidates` acotado a la clase de corrupción (SQLITE_CORRUPT / FTS malformado): degradar en vez de abortar, con una señal de telemetría/log. Los demás errores siguen propagándose.
- Tests: unitarios del piso (bajo/alto coseno) y de la degradación (FTS que falla + QueryVector que sirve). Golden regenerado si el ranking híbrido cambia (esperado y deseado).

## Fuera de alcance (explícito)

- **Q3** (importance como término RRF propio) — cambia la fórmula GLOBAL de scoring, re-baselinea todos los golden; va a un cambio aparte.
- Calibración empírica fina de `tau` sobre el modelo instalado (Q5) — acá se elige un default razonable + config; la calibración medida es un cambio siguiente.
- Cualquier auto-supresión/auto-NOOP: el coseno acá solo FILTRA la ENTRADA al pool (candidatos de baja señal), nunca descarta memoria ni cambia lo que se persiste. Respeta la regla de oro del track (model-free amplía/filtra pools, no juzga).
- Endurecer el `doctor` repair (P0) — track/cambio aparte.

## Estrategia de rollback

- Ambos cambios son aditivos y config-gated. `tau = 0` reproduce el comportamiento histórico (sin piso) bit-a-bit → un solo valor de config revierte Q1.
- Q2 solo cambia el camino de ERROR de FTS (hoy: abortar); revertir = volver a propagar el error. No toca el happy path.
- Sin migración de esquema, sin cambio de datos. Revertir el PR restaura el comportamiento previo por completo.

## Riesgos

- El piso `tau` cambia el ranking del recall híbrido → golden a regenerar; un `tau` mal elegido podría filtrar de más (mitigado: default conservador + config + tests de borde).
- Q2 podría enmascarar un error real de FTS si se generaliza demasiado (mitigado: acotar a la clase de corrupción + telemetría para que no pase inadvertido).
