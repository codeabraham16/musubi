# Tareas — Dominios ajenos no se juzgan (F3 · track «Potencia medida»)

Estado: **completo**. Build, vet, `golangci-lint` (0 issues) y las 17 suites del repo en verde.

- [x] **T1 — Medir ANTES de escribir una línea.** Consulta directa sobre las 494 relaciones de la
      memoria de dogfood: 45 de señal (9,8 %), 31 de las 36 pendientes cruzando dominios, y —el
      número que decidió el diseño— 44 de las 45 señales son del mismo dominio.

- [x] **T2 — Medir el COSTO de la guarda antes de proponerla.** La pregunta no era «¿cuánto ruido
      saca?» sino «¿cuánta señal cuesta?». Respuesta: una sola relación cross-dominio fue señal en
      toda la historia (`git-commit` × `bugs`), y con la excepción de registros históricos el costo
      es **cero**.

- [x] **T3 — `dominioDe` + `dominiosAjenos`** en `conflicts.go`, junto a las guardas que ya existían.

- [x] **T4 — Enganche en el loop de `DetectRelations`**, después del aislamiento por tenant y de
      `complementaryPair`. Es el único lugar donde nacen las relaciones, así que la guarda es
      imposible de saltear.

- [x] **T5 — 7 tests de invariantes** en `dominios_test.go`, incluido el end-to-end contra el
      detector real **con control**: una nota del mismo dominio y el mismo texto SÍ propone
      relación, así que el test no puede pasar por «el detector no detectó nada».

- [x] **T6 — Sabotaje: 6 mutaciones, cada una puso en rojo el test de su invariante.**

      | Sabotaje | Invariante | Resultado |
      |---|---|---|
      | Desconectar la guarda del loop | D1 | rojo |
      | Sacar la excepción de registros históricos | D2 | rojo |
      | Filtrar también dentro del mismo dominio | D3 | rojo |
      | Que la guarda marque `superseded_by` al filtrar | D4 | rojo |
      | Mirar sólo el source (romper la simetría) | D5 | rojo |
      | `dominioDe` acepta barra inicial | frontera | rojo |

## La mitad del diseño la encontró un test, no yo

La primera versión eximía **sólo** a `git-commit`, porque eso era lo único que el dato exigía. Con
esa versión la guarda evitaba **163** relaciones (33 %) con cero señal perdida — un número mejor que
el final.

Y rompía `TestSoloLasCreenciasSeReemplazan/contrato -> nota`, uno de los tests del PR #203 que mi
propia spec declaraba intocables. Su mensaje: *«el destino es una CREENCIA y SÍ se puede reemplazar:
la guarda se pasó de ancha, es un martillo»*.

**La lección vale más que el número:** el dato dijo que ese caso nunca ocurrió; el invariante dice
que debe seguir siendo posible. No son lo mismo, y cuando chocan gana el invariante — un caso que no
apareció en 494 relaciones puede aparecer mañana, y el precio de permitirlo son 35 relaciones más de
cola. La red del #203 hizo exactamente aquello para lo que se construyó.

Después de corregir, mi propio test D2 falló: afirmaba que la excepción era exclusiva de
`git-commit`. El test viejo tenía razón y el mío estaba mal.

## Resultado final, medido con la función que corre en producción

| | |
|---|---|
| Relaciones totales | 494 |
| Que la guarda evitaría crear | **128 (26 %)** |
| Señal perdida | **0** de 45 |
| Pendientes de hoy | **36 → 6** |

## Fuera de alcance, dicho de frente

- **No arregla el ruido del MISMO dominio.** Quedan 6 pendientes. Bajarlas exigiría mirar contenido,
  que es otro problema y otra fase.
- **La guarda depende de la disciplina de los `topic_key`.** Si todo se guardara bajo `notas/`, el
  dominio deja de discriminar y la guarda no filtra nada. No rompe: degrada a lo de hoy.
- **El modo de falla honesto:** si dos notas del MISMO tema se guardan bajo dominios distintos
  (`cognicion/x` y `roadmap/x`), su contradicción real deja de detectarse. La medición no encontró
  ni un caso en 494 relaciones, pero es el riesgo real de esta decisión.
- **No purga la cola existente.** Las 36 pendientes de hoy siguen ahí; la guarda actúa sobre las
  relaciones NUEVAS. Purgar retroactivamente sería otra decisión, y con más riesgo.
