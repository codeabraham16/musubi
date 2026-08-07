# Tareas — El arsenal federado (Fase A · track «Conocimiento unificado»)

Estado: **completo**. Build, vet, `golangci-lint` (0 issues) y las 17 suites en verde.

- [x] **T1 — Medir antes de proponer.** De los tres pilares, sólo uno está roto: memoria y grafo de
      código YA se federan; las skills no. Verificado en el código: cero menciones a skills en el
      outbox y el inbound, y ninguna tool que instale del central. La consecuencia se ve en el dato
      real: el arsenal del central tiene **1** skill —la única creada por el único camino que
      llegaba, la Forja— y esta PC tiene **11** que nunca subieron.

- [x] **T2 — Decidir DÓNDE se construye, no sólo qué.** En el cerebro, por la regla de reparto del
      track: *si el cuerpo puede hacer algo que la terminal no puede, ese algo está en el lugar
      equivocado*. Construirlo acá hace que la terminal lo gane el mismo día y que la mudanza al
      cuerpo no tenga que migrar nada.

- [x] **T3 — Reusar el transporte, no inventar uno.** `SyncClient` ya habla MCP-sobre-HTTP con el
      central (`Push`, `PushGraph`, `Pull`). Se sumaron `PushSkill` y `FetchSkill` sobre el mismo
      cliente, el mismo bearer y la misma config, con `callCentral` centralizando el sobre JSON-RPC
      y la clasificación transitorio/permanente para que los dos no la desincronicen.

- [x] **T4 — `skillPayload` con tags JSON.** `skills.Skill` tiene sólo tags YAML. Sin el DTO, el
      receptor recibiría `{"Name":…}` y guardaría una skill con todos los campos VACÍOS **sin
      fallar**. Es la misma trampa que ya mordió en `musubi_list_skills`; acá se selló antes de que
      mordiera.

- [x] **T5 — Las dos tools**, `musubi_promote_skill` y `musubi_install_skill` (52 → 54).

- [x] **T6 — 9 invariantes con test**, todos con control donde hacía falta (F5 verifica que con
      `overwrite:true` SÍ reemplaza; F6 que la skill existente SÍ se promueve), para que ninguno
      pase por «nunca hace nada».

- [x] **T7 — Sabotaje: 9 mutaciones, cada una en rojo.**

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | Enviar `skills.Skill` directo (claves PascalCase) | F1 | rojo |
      | Fingir éxito sin central configurado | F2 | rojo |
      | Escribir sin `writeSkillFile` (abrir una 2ª puerta) | F3 | rojo |
      | No marcar la procedencia | F4 | rojo |
      | Ignorar el chequeo de `overwrite` | F5 | rojo |
      | Promover un esqueleto por un nombre inexistente | F6 | rojo |
      | Marcarlas `readOnly` | F7 | rojo |
      | Aceptar el 1er resultado en vez del nombre exacto | match exacto | rojo |
      | Tragarse el rechazo del central | propagación | rojo |

- [x] **T8 — Extremo a extremo contra el binario**, con un cerebro central REAL (`musubi serve` en
      loopback) y **dos proyectos distintos**: el arsenal arranca vacío → B no puede instalar lo que
      no existe → A promueve → la skill aparece en el arsenal con sus triggers y reglas completas →
      **B la instala**, marcada como adoptada y conservando sus triggers → re-instalar sin
      `overwrite` se rechaza. 11 de 11 chequeos.

## El sabotaje que reveló un test vacuo

La primera versión de F3 —el invariante de SEGURIDAD— **pasó en verde bajo sabotaje**. Yo había
saboteado quitando `validateSkillStructural`, y el test siguió verde porque `writeSkillFile`
**también** guarda la ruta: la defensa en profundidad tapaba la mutación.

El test estaba bien; **el sabotaje estaba mal elegido**. F3 no dice «valido el nombre», dice «esta
tool no abre una segunda puerta». El sabotaje correcto es escribir el archivo sin pasar por
`writeSkillFile` — y con ése, rojo.

Vale como método: cuando un sabotaje no pone en rojo, la pregunta no es sólo «¿el test sirve?» sino
también «¿estoy saboteando el invariante que el test declara?».

## Decisión: explícita, nunca automática

Subir todo solo sería tentador y está mal. Medido sobre las 11 skills locales: **7 tienen trigger
`*`**, o sea que disparan en cualquier archivo — la resolución por triggers protege a las que
declaran extensiones, no a esas. Y hay skills locales por naturaleza: `project-profile` describe
*este* proyecto, `starter` es la plantilla de `musubi setup`.

La curaduría es del dueño; la herramienta sólo la hace fácil.

## Fuera de alcance, dicho de frente

- **Un proyecto nuevo todavía necesita `sync.central_url` configurado** para usar estas tools. Es el
  problema de la Fase B (arranque de proyecto), no de ésta — y es real: hoy alguien que clona un
  repo nuevo no tiene el arsenal a mano hasta configurar el sync.
- **El arsenal es un espacio de nombres plano y compartido.** Dos proyectos que promuevan `go-rules`
  compiten; hoy se resuelve con `overwrite` explícito.
- **No se re-traen actualizaciones.** La marca de procedencia deja la puerta abierta para
  «volver a bajar la que cambió», pero esa comparación no está construida.
- **El cuerpo no se tocó.** La Forja ya llama a lo que necesita; recablear «Adoptar» a
  `musubi_install_skill` es trabajo de su propia terminal.
