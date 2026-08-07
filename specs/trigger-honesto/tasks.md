# Tareas — El trigger honesto

Estado: **completo**. Build, vet, gofmt, `golangci-lint` (0 issues) y la suite entera (17/17
paquetes) en verde.

- [x] **T1 — Medir antes de proponer, y corregirme.** Yo había planteado el `*` como un problema de
      disciplina. Las 12 skills locales dicen otra cosa: **ninguna de las 7 que usan `*` es
      pereza**. `orchestrate-multiagent` se activa por la *forma de la tarea*; `plan-ahead` y
      `sdd-flow`, por la *fase del trabajo*; `project-profile`, por el proyecto entero. El
      vocabulario de triggers sólo sabe de globs de archivo, así que todo lo que no es "por
      archivo" **no tiene otra forma de expresarse**. El `*` era el síntoma, no el defecto.

- [x] **T2 — El agujero real, que era otro.** `toolPromoteSkill` —la puerta al arsenal
      COMPARTIDO— **no corría ningún gate de calidad**. Las otras tres puertas sí
      (`save_skill`, `author_skill`, `install_skill`). Una skill escrita a mano, que nunca pasó
      por ninguna tool, subía tal cual al arsenal de todos.

- [x] **T3 — H1: el campo `always_because`.** Declara por qué una skill se activa siempre, con
      piso de 20 caracteres. No es longitud por longitud: un campo que se satisface con dos
      palabras no cambia ninguna conducta.

- [x] **T4 — H2: el detector deja de ser ingenuo.** `allWildcard` → `anyWildcard`. Un `*` vuelve
      decorativos a los demás triggers. El caso enmascarado (`["*", "*.go"]`) se nombra aparte
      —`triggers_wildcard_masks_specific`— porque es peor: *miente* sobre su alcance. Hoy no hay
      ninguna así en el repo; el día que escriban skills desde otra máquina, la habrá.

- [x] **T5 — H3: la puerta de promoción cobra.** Gate de calidad completo + el `*` sin declarar
      bloquea. La asimetría es deliberada: warning en el score, **error** al subir. En tu proyecto
      el alcance es tu problema; en el arsenal se lo comen todos.

- [x] **T6 — Un predicado, dos consumidores.** `WildcardUnjustified` es la única definición de
      «`*` sin declarar». Dos copias se desincronizarían y el síntoma sería *«la advertencia decía
      que estaba bien pero el central la rechazó»* — la clase de contradicción que nadie depura.

- [x] **T7 — El campo cruza los CUATRO sobres.** Subida (`skillPayload`), recepción
      (`argsGuardarSkill`, que corre **en el central**), catálogo (`skillListada`) y vuelta
      (`ListArsenal`). Cada uno lo podía tirar en silencio.

- [x] **T8 — 13 invariantes** (7 en `skills`, 6 en `mcp`), cada uno con su control: G9 verifica que
      lo declarado SÍ sube, G10 que lo acotado no paga por un problema que no tiene, y el caso
      honesto (`["*"]`) se prueba explícitamente contra el enmascarado.

- [x] **T9 — Sabotaje: 10 mutaciones, cada una en rojo.**

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | `anyWildcard` vuelve a exigir que TODOS sean `*` | G5 | rojo |
      | `WildcardUnjustified` nunca es true | G3·G4 | rojo |
      | El warning ignora la razón declarada | G3·G6 | rojo |
      | El piso de la justificación baja a 1 carácter | G4 | rojo |
      | La subida tira el campo (`payloadFromSkill`) | G2 | rojo |
      | La vuelta tira el campo (`ListArsenal`) | G2 | rojo |
      | El catálogo tira el campo (`entradaSkill`) | G2 | rojo |
      | **La recepción tira el campo** (`toolSaveSkill`) | G2 | rojo |
      | La promoción vuelve a no tener gate | G7·G8 | rojo |
      | El gate sólo mira calidad, no el wildcard | G8 | rojo |

- [x] **T10 — Las 8 cognitivas declaran su motivo, EN EL CÓDIGO.** Ver abajo: es el hallazgo que
      cambió dónde iba el arreglo.

## Tres cosas que el trabajo enseñó

**El arreglo casi va al lugar equivocado.** Iba a escribir `always_because` a mano en los 7 YAML.
Pero **6 de los 7 tienen `managed_checksum`**: son skills que Musubi escribe y refresca. Editarlas
a mano rompe el checksum, y a partir de ahí Musubi las trata como intervenidas y **deja de
refrescarlas para siempre**. El arreglo tenía que ir en `cognitiveSkills()`, que es quien las
escribe. Bonus: `.musubi/skills/` está gitignoreado, así que la edición a mano tampoco habría
viajado a ningún lado.

**Una razón fija habría sido una mentira condicional.** `analyze-project` y `deduce-conventions`
sólo caen a `*` cuando **no se reconoce ningún ecosistema**; con stack detectado reciben globs
concretos. Ponerles una justificación estática habría declarado «aplica siempre» en proyectos donde
la skill está perfectamente acotada. Se resolvió condicionando la razón al fallback.

**`starter.yaml` es un huérfano.** Tiene `*`, está siempre encendido, y **su texto no existe en
ningún lado del código**: lo escribió una versión vieja de `musubi setup` que ya no lo genera. No se
tocó —es del dueño decidir— pero queda dicho: es contexto que se paga en cada turno y nadie
mantiene.

## Fuera de alcance, dicho de frente

- **El vocabulario de alcance de verdad no se construyó.** `applies_to: task | language | repo |
  phase` es el arreglo de fondo de lo que este spec diagnostica; acá el problema queda *nombrado y
  contenido*, no resuelto de raíz. Pertenece al track de la **Forja global**.
- **Local no cambió.** `musubi_save_skill` sigue aceptando `*` sin justificar. Endurecerlo rompería
  skills vivas por un problema que sólo existe al compartir.
- **Lo ya promovido no se migra.** El arsenal tiene 2 skills y las dos están acotadas, así que no
  hay nada que migrar — pero si lo hubiera, tampoco se migraría solo.
- **Un central viejo tira el campo en silencio.** No se agregó negociación de versión. Queda por
  verificar contra el central real después del deploy.
- **Las cognitivas del disco todavía no tienen su razón.** El campo lo escribe el binario; los YAML
  de `.musubi/skills/` se refrescan recién cuando se actualice el binario global y corra
  `writeCognitiveSkills`. Hasta entonces siguen mostrando el warning.
