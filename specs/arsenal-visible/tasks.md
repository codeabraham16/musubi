# Tareas — El arsenal se ve (F5.1 · track «Potencia medida»)

Estado: **completo del lado del código**. Build, vet, `golangci-lint` (0 issues) y las 17 suites del
cerebro en verde; build, vet y las 26 suites del cuerpo en verde. Falta el despliegue al central,
que es decisión del usuario (abajo).

- [x] **T1 — Medir la superficie del cuerpo antes de tocar nada.** El plan del track decía «~4 de
      ~10 familias», de una auditoría previa al re-plataformado. Medido de nuevo: **25 tools con
      cliente real** en `internal/brain/`, **7 que el cuerpo sólo sabe dibujar** cuando el agente
      las llama (viven únicamente en `chatstate/format.go`), **19 sin presencia** y **1 referencia
      a una tool inexistente**.

      El primer conteo del grep daba 33 y habría sido una cifra inflada: no distingue una tool
      cableada de una string en un formateador. La distinción cambió qué se construyó.

- [x] **T2 — El hallazgo: `musubi_list_skills` no existe en el cerebro.** El cuerpo la llama desde
      `internal/brain/skills.go` con su DTO y su parseo completos. Nunca existió del otro lado.

- [x] **T3 — El error se tragaba.** `cmd/musubi-body/bridge.go`: `if sk, err := cc.ListSkills(...);
      err == nil` sin rama else ⇒ el panel Arsenal quedaba **vacío en silencio**, y un arsenal vacío
      se lee como «no hay skills guardadas». La peor clase de falla: la que se lee como un dato.

- [x] **T4 — La tool, sobre la capacidad que ya existía.** `toolListSkills` envuelve
      `skills.Resolver.LoadSkills()` en vez de releer el disco: dos lecturas con criterios propios
      se desincronizan el día que cambie qué es una skill válida.

- [x] **T5 — El DTO con tags JSON, y por qué no es un detalle.** `skills.Skill` tiene **sólo tags
      YAML**. Serializarla directo produce `{"Name":…}`; el cuerpo, que parsea en minúscula, no
      habría fallado: habría mostrado **N filas en blanco**. El bug «arreglado» habría quedado peor
      que el original.

- [x] **T6 — 7 invariantes con test propio** en `internal/mcp/list_skills_test.go` y 3 más en
      `cmd/musubi-body/bridge_arsenal_test.go`, con control en los dos lados (A3 verifica primero
      que sin filtro vengan las dos skills; A7 verifica que el camino feliz NO registre error).

- [x] **T7 — Sabotaje: 9 mutaciones, cada una puso en rojo el test de su invariante.**

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | Serializar `skills.Skill` directo, sin el DTO | A1 | rojo |
      | `var out` en vez de `make` (nil ⇒ `null`) | A2 | rojo |
      | Filtrar sólo por nombre, ignorando la descripción | A3 | rojo |
      | Aplicar `limit` también cuando es ≤ 0 | A4 | rojo |
      | Registrar la tool como no-readOnly | A5 | rojo |
      | Sacarla de `noScopedRead` (dejarla sin clasificar) | guard Track 19 | rojo |
      | Que un YAML roto tumbe la carga entera | A6 | rojo |
      | Volver a tragarse el error del arsenal (el bug original) | A7 | rojo |
      | Reportar error SIEMPRE, aun sin fallo | control de A7 | rojo |

- [x] **T8 — Verificación de extremo a extremo contra el binario**, no contra el paquete: 52 tools
      publicadas en `tools/list`, 11 skills devueltas con las claves en minúscula y **valores
      reales** (`name` = `adversarial-review`, `rules` = 1739 chars), `query="sdd"` → 1, `limit=3`
      → 3.

## Cinco guardas del repo me frenaron, y las cinco tenían razón

Agregar una tool rompió, en orden: `TestToolReadOnlyClassification` (el conjunto exacto de tools de
solo-lectura), `TestHTTPToolsList`, `TestServeToolsListCountsAllTools`, `TestDispatchConcurrentSafe`
(el catálogo completo bajo concurrencia) y `TestReadmeToolCountMatchesRegistry` — que además obligó
a **listar la tool en el README**, no sólo a subir el número.

Ninguna se relajó. `TestEveryReadOnlyToolClassified` obligó a **decidir** si la tool necesita scope
de proyecto en vez de dejarla pasar; la respuesta quedó escrita al lado de la clasificación. Es la
misma red que en F3 obligó a corregir una guarda que se había pasado de ancha.

El golden de `tools/list` se regeneró y su diff se revisó: **17 líneas, todas de la tool nueva**.

## Una medición que casi reporto mal

La primera comparación de lint del cuerpo dio «60 antes, 62 ahora» — parecía que el cambio agregaba
dos issues. Era falso: `git stash` sin `-u` dejó el test nuevo sin trackear, el paquete no compiló y
el linter lo salteó entero, así que la línea base estaba rota.

Con `-u`: **62 antes y 62 después**, y cero en los archivos tocados. Además, dos corridas del
**mismo código** difieren en 4 entradas: el set que reporta `golangci-lint` no es determinista, así
que «la lista de issues nuevos está vacía» no es una afirmación verificable acá — lo verificable es
el total y que mis archivos no aparecen.

## Fuera de alcance, dicho de frente

- **El loop completo cuerpo → central NO está probado**, y no puede estarlo desde acá: el central
  corre el binario anterior, que no tiene la tool. Probarlo exige desplegar, que es una acción
  privilegiada y la decide el usuario.
- **Quedan 25 tools sin cara en el cuerpo**, entre ellas `musubi_save_observation` —que el cuerpo
  sólo sabe *dibujar*, no invocar—. Es el hueco de fondo de F5 y necesita diseño de UI, no una tool
  más.
- **El filtro es por subcadena, no semántico.** Deliberado: la búsqueda inteligente es el trabajo de
  `musubi_search_skills`.
- **Se arregló también la tragada de `Doctor()` del central**, dos líneas más arriba en el mismo
  bloque. El diseño la había subestimado: al fallar no deja un campo vacío, deja en pantalla la
  memoria **local** presentada como si fuera la del central. Arreglar una tragada y dejar su gemela
  al lado no era defendible.
