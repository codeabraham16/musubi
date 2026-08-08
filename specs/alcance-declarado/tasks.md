# Tareas — El porqué del `*` se vuelve alcance

Estado: **completo**. Build, `go vet ./...`, la suite entera y `golangci-lint` (0 issues) en verde.
8 invariantes (A1–A8), 8 sabotajes, los 8 en rojo.

## Lo que se midió antes, y las dos veces que me corrigió

- [x] **T0 — El límite no era el vocabulario: era la PREGUNTA.** `musubi_resolve_skills` rechazaba
      una lista de archivos vacía. Una skill que no habla de archivos no tenía cómo ser encontrada.

- [x] **T0b — ★ Primera corrección: el «cuándo» YA estaba escrito.** Yo había concluido que para las
      6 skills con `*` esa información «no estaba en ninguna parte del artefacto». Falso: está en
      `always_because`. Verificarlo cambió el plan entero — de «hacer que las skills digan cuándo
      aplican» a «volver matcheable lo que ya dicen».

- [x] **T0c — El vocabulario se transcribió, no se inventó.** Los 5 valores salen de los
      `always_because` que ya existían; cada uno tiene un autor real detrás:

      | valor | de quién sale |
      |---|---|
      | `phase:planning` | `plan-ahead` — «planificar es una FASE del trabajo» |
      | `phase:implementing` | `sdd-flow` — «el flujo completo de un cambio» |
      | `phase:reviewing` | `adversarial-review` — «al cerrar un cambio de riesgo» |
      | `task:audit` | `audit-structure-flow` — «el pedido de auditoría» |
      | `task:orchestration` | `orchestrate-multiagent` — «grande y paralelizable» |

- [x] **T0d — La fuente canónica NO son los YAML.** `.musubi/skills/` está **gitignored** y los
      archivos los **genera el binario** (`cmd/musubi/cognitive.go`, con `managed_checksum`).
      Editarlos a mano no sólo era inútil: les habría roto el checksum y Musubi habría dejado de
      actualizarlos, tomándolos por editados a mano.

## Lo construido

- [x] **T1 — `Skill.AppliesTo`** + vocabulario cerrado (`VocabularioDeAlcance`, `AlcanceValido`).
- [x] **T2 — `ResolveRequest{ModifiedFiles, Phase, Task}`** reemplaza al `[]string`: la pregunta
      ahora admite que el llamador **declare qué está haciendo**.
- [x] **T3 — El matcheo es un OR**: archivo **o** alcance. Aditivo por diseño (ver T5).
- [x] **T4 — La tool acepta `phase`/`task`** y sólo falla si no se declara **nada**.
- [x] **T5 — Las 5 skills migradas conservan su `*`.** Si migrar les quitara el match por archivo,
      dejarían de activarse mientras ningún llamador declare su fase: un arreglo con cara de
      regresión. Primero existe el canal, después se aprieta.
- [x] **T6 — El validador deja de mentir**: `containsAny` respeta límites de palabra.
- [x] **T7 — `applies_to` fuera del vocabulario es ERROR**, no warning: un typo produce una skill
      que nunca se activa, indistinguible de una que no aplica.
- [x] **T8 — Golden de `tools/list` regenerado** (cambió el esquema de la tool).
- [x] **T9 — 8 invariantes y 8 sabotajes**, los 8 en rojo.

      | Sabotaje | Inv. | Resultado |
      |---|---|---|
      | El resolvedor ignora el alcance | A1 | rojo |
      | Una petición vacía se acepta | A2 | rojo |
      | Los globs dejan de matchear | A3 | rojo |
      | Tener `applies_to` anula los globs | A4 | rojo |
      | El match de fase pasa a ser por prefijo | A5 | rojo |
      | El vocabulario acepta cualquier cosa | A6 | rojo |
      | Sin `applies_to` matchea por declarar | A7 | rojo |
      | Vuelve el match por subcadena | A8 | rojo |

## El defecto real que encontró el sabotaje (y no era del código)

**A7 quedó VERDE en la primera pasada, y el problema era mi mutación.** Cambié
`if len(skill.AppliesTo) == 0 {` por `if false {` esperando romper el invariante, y no rompió nada:
la guarda es **redundante** — con el slice vacío el bucle de abajo no itera y devuelve `false` igual.

La mutación era **vacua**: no creaba el defecto que A7 vigila. El defecto real es que una skill que
NO declara alcance matchee CUALQUIER declaración, y ésa (`return false` → `return true`) sí la puso
en rojo.

Vale como método: **un sabotaje verde tiene dos explicaciones —el test no protege, o la mutación no
rompe— y hay que distinguirlas antes de tocar el test.** Si hubiera "arreglado" A7 sin mirar, habría
endurecido una prueba que ya estaba bien.

## Fuera de alcance, dicho de frente

- **Progressive disclosure**, problema 3 de la investigación. Con una interacción ya detectada: el
  nivel 1 (`name`+`description`) **no** contiene el «cuándo» de estas 6 skills, que vive en
  `always_because`.
- **Medir si una skill sirvió**, problema 4.
- **Un eje para el juicio** («aplica cuando el código huele a X»): necesita un modelo. Sería opt-in,
  como la cognición.
- **`project-profile` no se migró**: es de proyecto entero y ningún valor del vocabulario le queda.
  Forzarlo sería inventar el eje que la propuesta dice no inventar.
- **Los YAML locales se actualizan solos** la próxima vez que corra un binario nuevo: son managed y
  su checksum está intacto.
