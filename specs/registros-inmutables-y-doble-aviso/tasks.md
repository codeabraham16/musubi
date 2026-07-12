# Tasks — registros-inmutables-y-doble-aviso

## Implementación

- [ ] **T1** — `band.go`: calcular `lex` y usar `relevantPair(lex, &cos, opts)` para **excluir** todo lo que la cola ya va a proponer (F1). El rango `[BandFloor, CosineFloor)` se conserva.
- [ ] **T2** — `conflicts.go`: `historicalRecord(topicKey)` (`isCommit || isSDD`) y `sameKind(a, b)`.
- [ ] **T3** — `conflicts.go`: **G3** en `complementaryPair` — `historicalRecord(target) && !sameKind(source, target) ⇒ true`. **Asimétrica: mira sólo el target.**
- [ ] **T4** — Documentar en el comentario de `complementaryPair` que ahora responde dos formas de la **misma** pregunta (*«¿es siquiera comparable?»*): «son complementarios» **y** «el veredicto no significaría nada».

## Tests

- [ ] **T5** — S.a: léxico alto (entra a la cola) + coseno 0.82 ⇒ **NO** aparece en la banda. **Es el test del defecto 1.**
- [ ] **T6** — S.b: léxico bajo + coseno 0.82 ⇒ **SÍ** aparece en la banda (F1 no puede vaciarla de su razón de ser).
- [ ] **T7** — S.c/S.d: nota → `git-commit` y nota → `sdd/*` ⇒ **0 relaciones**.
- [ ] **T8** — **S.e: `git-commit` → nota ⇒ SÍ hay relación.** *El test que prueba que la regla es asimétrica y no un martillo.*
- [ ] **T9** — S.f: commit → commit ⇒ **sí** (misma clase).
- [ ] **T10** — S.g: `sdd/a/design` → `sdd/b/design` ⇒ **sí** (misma clase, cambios distintos).
- [ ] **T11** — S.h: nada queda `superseded` ni `archived`.

## Cierre

- [ ] **T12** — `go test ./...` verde + `golangci-lint` limpio.
- [ ] **T13** — **Verificación adversarial**: desactivar G3 y F1 y confirmar que T5/T7 **fallan**.
- [ ] **T14** — CHANGELOG (`Fixed` ⇒ patch).
- [ ] **T15** — PR.
- [ ] **T16** — (fuera del código) resolver las 8 pendientes ya creadas como `related`.

## Orden

T1 (F1) y T2→T3→T4 (G3) son independientes. T5-T11 después. T12→T16.

**T8 es el test que más importa.** El modo de fallo peligroso acá es el **martillo**: una guarda que, por ancha, apague el caso valioso — *el commit que demuestra que una nota quedó vieja*. T6, T9 y T10 cumplen la misma función: pinean lo que **NO** se debe bloquear.
