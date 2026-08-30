# Renaissance, rey del diseño — plan de ataque

Track de 9 fases para que `musubi_design` deje de ser un servidor de conocimiento y sea un motor.
Nace de la auditoría del 2026-08-29 (46 casos de ataque, 40 contra el central en producción y 6 en
banco local), que lo dejó en **3 de 7** en la placa de capacidad.

Cada fase es 1 PR con su SDD propio (`specs/renaissance-<fase>/`), su bump semántico y su entrada en
CHANGELOG. Este archivo es la **propuesta del track**, no la spec de una fase.

---

## 1 · Qué significa «rey», en números

No se declara ganado por sensación. El track cierra cuando el banco de la F0 mide esto:

| # | Dimensión | Hoy (medido 2026-08-29) | Objetivo |
|---|---|---|---|
| M1 | Estabilidad de paráfrasis (Jaccard entre las formas de pedir lo mismo) | **0,09** | **≥ 0,80** |
| M2 | Abstención correcta ante consultas fuera de dominio | **0 %** | **100 %** |
| M3 | Precisión@6 del corpus (toca el tema pedido) | **0,22** | **≥ 0,80** |
| M4 | Tamaño del brief | **5.850 tok** (11.131 con `limit=100`) | **≤ 2.000 tok, tope duro** |
| M5 | Fracción del brief que varía según el pedido | **6 %** | **≥ 60 %** |
| M6 | Payloads de inyección que llegan como instrucción | **todos** | **0** |
| M7 | Latencia p95 del camino model-free | 691 ms | **≤ 1.200 ms** (no empeorar) |
| M8 | Cobertura: ids distintos servidos contra el set dorado | **190 de 1.736** | **≥ 60 %** |

**Corrección (2026-08-29, tras correr la sonda):** M1 estaba anotado en 0,21 midiendo UN pedido.
Sobre los 16 pedidos reales del set dorado da **0,09**, con tres pedidos en **0,00** — dos formas de
pedir lo mismo sin un solo patrón en común. Y M3, que figuraba «sin medir», da **0,22**: apenas uno
de cada cinco items del corpus servido toca siquiera el tema del pedido.

M4 y M5 juntos son la tesis del track: **un brief más chico que dice más sobre tu pedido.**

---

## 2 · Las fases

### F0 · EL BANCO — medir antes de tocar
**Por qué va primero.** El motor se degradó el 2026-08-21 (el método saltó de 8 principios a 30
tarjetas, 24×) y nadie lo notó hasta que el usuario lo *sintió*, ocho días después. No existía
ninguna medida de calidad de recuperación. Sin marcador, cada fase siguiente sería una corazonada.

- Set dorado: 16 pedidos de diseño reales tomados de los proyectos vivos (Altura, CRM, cuerpo,
  panel), cada uno en 3 paráfrasis; +8 consultas fuera de dominio; +8 payloads de inyección.
- Métricas automáticas: M1, M2, M4, M5, M6, M7, M8. M3 arranca como aproximación por ejes
  (model-free, sin etiquetado humano); si deja de discriminar, se reemplaza por etiquetado.
- Entregable: `go test ./internal/mcp -run TestBancoDiseno` + un reporte imprimible.
- Invariantes: **I-BANCO1** el banco corre sin red y sin LLM (si no, no entra a CI). **I-BANCO2**
  una regresión de M1/M2/M4 pone el test en rojo, no en advertencia.

### F1 · PRECEDENCIA Y PRESUPUESTO
Cierra: la contradicción de Altura · lo perdido en el medio · la inundación del caller.

- **Orden nuevo del brief:** `ask → marca → corpus → método → emit`. La marca sube del 70 % de
  profundidad al principio; el método universal —que no depende del pedido— baja al final.
- **Regla de precedencia explícita** (*lex specialis*): la marca del proyecto derrota al método
  universal cuando chocan; el método sólo llena lo que la marca no cubre. Declarado en el propio
  brief, no en un comentario del código.
- **Presupuesto:** tope total (~2.000 tok) y por bloque. Todo recorte se **declara** en la salida
  (`truncated: {method: 12, corpus: 3}`) — nunca en silencio, que es el modo de falla de la casa.
- Sacar la marca Musubi de `designEmitWeb`: el emit dice formato y dialecto, nunca paleta ni
  prohibiciones estéticas.
- Invariantes: **I-PRE1** la marca aparece antes que el método. **I-PRE2** el brief nunca supera el
  tope, con ningún `limit`. **I-PRE3** todo recorte se declara con su total. **I-PRE4** el emit no
  nombra colores ni prohibiciones de ninguna marca.

### F2 · EL ACERVO ES DATO, NO ORDEN
Cierra: los dos críticos de inyección (A1 acervo, A2 marca) y el eco del pedido.

- El material recuperado viaja **delimitado y rotulado como material citado**, no como instrucción
  del sistema; con su procedencia visible (tenant, topic, autor).
- Saneamiento del contenido: sin marcado ejecutable, sin secuencias que simulen fin de instrucción.
- El **orden lo fija el sistema**: la `importance` de una tarjeta puede ordenar dentro de su grupo,
  nunca saltar por encima del núcleo del método.
- Invariantes: **I-INY1** ningún payload del banco llega al brief en posición de instrucción.
  **I-INY2** una tarjeta con importancia máxima no precede al núcleo.

### F3 · SABER CUÁNDO NO SE SABE
Cierra: «nunca dice no sé» · la caída silenciosa a búsqueda léxica.

- **Piso de similitud**, calibrado contra el banco (arranque ≈ 0,48; la banda de basura medida fue
  0,362–0,442 y la de pedidos reales 0,533–0,558).
- `degraded` honesto **con causa**: `sin_material` | `bajo_umbral` | `sin_embebedor` | `recortado`.
- La caída a FTS se declara (`retrieval: "fts" | "semantico"`).
- Timeout del embebedor de 30 s → ~5 s: con una persona esperando, 30 s ya es un fallo, no una
  espera. Además corta el DoS barato contra el embebedor compartido.
- Invariantes: **I-ABS1** toda consulta fuera de dominio devuelve `degraded` con causa y sin corpus
  de relleno. **I-ABS2** el modo de recuperación siempre se declara.

### F4 · SELECCIÓN — el método y el corpus se eligen
Cierra: el 68 % constante · el método que habla del pulgar en un ERP.

- `designMethod(prompt, target)` pasa a recibir el pedido: **núcleo obligatorio** (los universales
  que aplican siempre) **+ top-N relevante**. De ~4.182 a ~1.200 tokens.
- Etiquetar las tarjetas de método por superficie (escritorio / móvil / universal) y por eje
  (color, tipografía, layout, motion, a11y, microcopy) — model-free, derivado del `topic_key`.
- Corpus: **reservar slots para artículos crudos**. Hoy 1.438 micro-tarjetas saturan el pool y los
  268 artículos completos —toda la profundidad— no entran nunca. Más diversidad en el top-k, para
  que 6 resultados no sean 6 variaciones del mismo tema.
- Invariantes: **I-SEL1** dos pedidos opuestos reciben métodos distintos. **I-SEL2** el núcleo
  obligatorio está siempre, gane o no por relevancia. **I-SEL3** el top-6 cubre al menos 3 ejes.

### F5 · REPRODUCIBILIDAD
Cierra: Jaccard 0,21 · el ruido que desplaza a la señal.

- **Normalizar la consulta antes de embeber**: extraer el pedido de diseño del contexto de relleno.
  Hoy 256 bytes de texto extra ya cuestan dos tercios del corpus y 10 KB lo destruyen — el motor
  castiga la especificidad, que es exactamente al revés de lo que debería.
- Diversificación del top-k y consulta por segmentos cuando el pedido es largo.
- Invariantes: **I-REP1** M1 ≥ 0,80. **I-REP2** agregar 4 KB de contexto irrelevante no cambia más
  del 20 % del corpus servido.

---

## 3 · El foso — lo que ningún rival tiene

Las fases anteriores lo arreglan. Éstas lo vuelven difícil de copiar. La investigación del
2026-08-29 dejó claro que **el volumen no es el foso**: ya tenemos 1.736 entradas contra las 395 de
southleft, el comparable directo, y el resultado era peor. Figma sirve datos estructurados de *tu*
archivo, sin método ni multi-marca. Los sistemas agénticos de empresa (Encore) son de una sola casa.

### F6 · EL CICLO CERRADO — aprender del diseño aprobado
Nadie en el rubro lo hace: el límite que la industria se reconoce es *«MCP da acceso de lectura, no
comprensión»*.

- Cuando un diseño se **aprueba**, se destila qué funcionó y se refuerza el material que lo produjo.
- Las consultas que caen bajo el umbral (F3) se registran como **huecos** y dirigen la próxima
  ingesta. La cobertura deja de ser un reporte y pasa a ser el motor de crecimiento *dirigido*.
- Esto convierte a M8 en un ciclo, no en una foto.

### F7 · MÉTODO ARBITRADO POR ERA
El «tell» anti-genérico se mueve cada ~18 meses (violeta-gradiente 2024 → crema+serif 2026). Un
`rules.md` se pudre; un cerebro que arbitra, no.

- Cada tarjeta de método lleva su era. El método rancio se **retira** con `musubi_judge` en vez de
  acumularse. Es la única razón defendible por la que este motor envejece mejor que un prompt.

### F8 · MARCA PROFUNDA, Y UN LINTER QUE ATRAPA EL CHOQUE AL CARGARLA
- Tokens DTCG por proyecto: no sólo paleta — componentes, densidades, anti-patrones propios.
- Normalizar el scope de marca (hoy `'Altura'` ≠ `'altura'` y el proyecto pierde su marca en
  silencio); rechazar argumentos desconocidos en vez de ignorarlos.
- **Linter de marca al guardar:** si la marca de Altura pide `glass + sombra` y el método universal
  lo prohíbe, el sistema lo detecta **cuando se carga la marca** y obliga a declarar la excepción —
  en vez de servir orden y contraorden en cada brief. Convierte el defecto que originó todo este
  track en algo que no puede volver a entrar.

---

## 4 · Orden y entrega

```
F0 banco ──► F1 precedencia+presupuesto ──► F2 acervo=dato ──► F3 abstención
                                                                    │
                          F4 selección ◄─────────────────────────────┘
                                │
                          F5 reproducibilidad ──► F6 ciclo ──► F7 era ──► F8 marca profunda
```

- **F0 antes que todo.** Es el marcador; sin él las demás fases son opinión.
- **F1 y F2 pueden ir en el mismo PR**: tocan el mismo ensamblado del brief.
- F3 depende de F0 (el umbral se calibra con el banco, no se elige a ojo).
- F4 y F5 son las que más mueven M1, M3 y M5.
- F6–F8 son el foso y pueden esperar sin bloquear el uso diario.

**Camino caliente model-free en todas las fases.** Nada de LLM en el brief. El juicio abstracto
—etiquetar el set dorado, arbitrar el método— es offline y con persona o mantenimiento, nunca en
la llamada.

## 5 · Riesgos, con su medición barata

1. **El umbral de F3 deja fuera pedidos buenos.** → se calibra contra el set dorado de F0 y se
   reporta la tasa de falsos negativos; si sube, el umbral baja.
2. **La selección de F4 esconde método que hacía falta.** → I-SEL2 protege el núcleo; el banco mide
   si la calidad juzgada (M3) sube o baja.
3. **Normalizar la consulta (F5) puede tirar contexto útil.** → se compara M1 y M3 antes/después; si
   M3 cae, la normalización fue demasiado agresiva.
4. **El presupuesto de F1 recorta lo importante.** → I-PRE3 obliga a declarar el recorte, así que el
   caller ve qué le falta en vez de recibir un brief mutilado con cara de completo.

## 6 · Una decisión que es del usuario

**El molino de auto-destilado sigue moliendo** (`auto_distill_minutes: 10` en el config del central).
Produjo 1.211 tarjetas en un día y la utilización quedó igual de mala: se optimizó el numerador
fabricando denominador (Goodhart). Mientras corra, la línea base de F0 se mueve bajo la medición.

Recomendación: **pausarlo hasta cerrar F4**. Es un cambio de configuración en el server y no se toca
sin visto bueno explícito.
