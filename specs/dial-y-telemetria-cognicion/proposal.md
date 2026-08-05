# Propuesta — Dial de potencia y telemetría (F5)

## Dos problemas, y el segundo es el que duele

**1. Las perillas están sueltas.** Para decidir "cuánto LLM quiero" hay que entender y coordinar
`read_time_rerank`, `read_time_rerank_top_k`, `cache.ttl_seconds` y la composición de la flota.
Cada una tiene su unidad y su riesgo; ninguna dice "más potencia" ni "menos gasto".

**2. Nadie sabe si algo de esto sirve.** El roadmap prometía **+20 puntos de precisión**. Hoy no hay
un solo número propio para confirmarlo o desmentirlo. Peor, tres fases dejaron preguntas sin
instrumento:

| Pregunta | Fase que la dejó abierta |
|---|---|
| ¿Cuántos secretos tapa el portero, y de qué tipo? | F1 — «sin medición no se sabe si actúa seguido o nunca» |
| ¿Cuántas veces escala el router, y qué motor se cae? | F2 |
| ¿Qué hit rate tiene el caché? | F3 — `Stats()` ya cuenta, nadie lo lee |

Sin eso, cada decisión sobre la cognición es opinión. **Medir es la fase.**

## La idea

**Un parámetro**: `cognition.effort: eco | balanced | turbo`.

No es maquinaria nueva: es un **preset sobre las perillas que ya existen**. `turbo` no inventa
potencia, prende el juez del recall y le sube el top-K. Que sea un preset y no un motor propio es la
diferencia entre un dial y una capa de indirección.

**Una superficie de lectura**: `musubi_cognition_stats`, read-only, que devuelve lo que las tres
fases anteriores ya cuentan o pueden contar sin costo.

## Por qué una tool MCP y no `musubi doctor`

Parecía obvio ponerlo en el doctor, que ya inspecciona la config de cognición. **No funciona**: los
contadores viven en memoria del proceso que atiende, y el CLI es **otro proceso**. `musubi doctor`
construiría su propio provider con el caché vacío y reportaría ceros para siempre — un número
convincente y falso, que es peor que no tener número.

## Qué NO es

- **No es un dial que "pide más inteligencia".** Mueve perillas concretas y documentadas. Si alguien
  quiere saber qué hace `turbo`, la respuesta es una tabla, no un adjetivo.
- **No es telemetría hacia afuera.** No hay red, no hay archivo, no hay exportador. Son contadores
  en memoria que se leen a pedido y mueren con el proceso.
- **No mide calidad.** Cuenta llamadas, hits y escaladas. Si el juez del recall *acierta* es otra
  medición, con corpus dorado, y ya tiene su propio gate (`recall-gate`).

## Costo y reversibilidad

Un enum de config con su resolución, contadores atómicos en tres lugares que ya existen, y una tool
de lectura. Sin dependencias nuevas, sin red, sin estado en disco. Sin `effort` declarado el
comportamiento es el de hoy, byte a byte.
