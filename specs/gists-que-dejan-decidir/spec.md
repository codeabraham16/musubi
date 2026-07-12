# Spec — gists-que-dejan-decidir

Vocabulario RFC 2119. Alcance: `internal/memory/digest.go`, `internal/memory/doctor.go`.

## El invariante

- **R0 — El gist es DERIVADO. Regenerarlo NO puede perder información.** Se recalcula desde `content`, que no se toca. Es **idempotente**: correr la reparación dos veces da lo mismo que correrla una.

## F1 — El gist USA su techo

- **R1** — `Gist(content, maxTokens)` DEBE seguir agregando **oraciones completas** mientras la siguiente **entre** en `maxTokens`.
- **R2** — Si la **primera** oración ya alcanza o excede `maxTokens`, el comportamiento DEBE ser **idéntico al actual** (se devuelve, truncada si hace falta). El fix **sólo** agrega texto donde antes se **abandonaba** presupuesto.
- **R3** — NUNCA se DEBE exceder `maxTokens`: se corta **por tokens**, no por cantidad de oraciones.
- **R4** — Una oración que **no entra completa** NO DEBE agregarse a medias: se prefiere un gist más corto a uno cortado al medio (un gist truncado a mitad de frase **también** es un gist que no deja decidir).
  - Excepción: la **primera** oración sí se trunca si no entra — es preferible a devolver un gist **vacío** (R2, comportamiento de hoy).

## F2 — La reparación del doctor

- **R5** — El `doctor` DEBE ofrecer una reparación que **regenere** los gists con el extractor actual.
- **R6** — La regeneración DEBE ser **explícita** (`--fix`), NO un efecto colateral silencioso del arranque del binario.
- **R7** — La regeneración NO DEBE tocar `content`, ni `content_hash`, ni los embeddings, ni las relaciones.

## Lo que NO cambia

- **R8** — `gist_max_tokens` (el techo), el presupuesto del recall, el ranking, MMR y el empaquetado: **intactos**.
- **R9** — NO se cambia cómo se **redactan** los contratos SDD (sería tratar el síntoma, no la causa).

## Escenarios

**S.a (el gist mudo se llena)** — *Given* un contenido cuya 1ª oración son ~8 tokens y hay más texto detrás, *When* se calcula el gist con techo 24, *Then* el gist incluye **más de una oración** y usa **más de 15** tokens.

**S.b (nunca se pasa del techo)** — *Given* cualquier contenido, *Then* `EstimateTokens(gist) <= maxTokens`.

**S.c (no se corta al medio)** — *Given* que la 2ª oración **no entra** completa, *Then* **no se agrega** (el gist queda con la 1ª sola), en vez de cortarse a mitad de frase.

**S.d (comportamiento idéntico cuando ya llenaba)** — *Given* un contenido cuya 1ª oración **ya alcanza** el techo, *Then* el gist es **exactamente** el de hoy.

**S.e (una sola oración disponible)** — *Given* un contenido de **una sola** oración corta, *Then* el gist es esa oración (no se inventa nada).

**S.f (la reparación es idempotente)** — *Given* la reparación ya corrida, *When* se corre otra vez, *Then* **ningún** gist cambia.

**S.g (la reparación no toca nada más)** — *Given* la reparación corrida, *Then* `content`, `content_hash`, embeddings y relaciones quedan **intactos**.

## La medición (obligatoria, no opcional)

- **R10** — DEBE medirse, sobre la memoria **real**, **cuántos items menos** entran en un recall típico con los gists engordados. El canje se **acepta**, pero se **informa con el número**, no con una intuición.

## No-objetivos (verificables)

- NO se pierde información (R0/S.g).
- NO se excede el techo (S.b) ni se corta a mitad de frase (S.c).
- NO cambia el gist de las memorias que ya llenaban su techo (S.d).
