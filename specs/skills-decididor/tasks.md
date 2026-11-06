# Tasks — Decididor de skills

Checklist atómica por área, ordenada, sin solape; cada ítem cierra solo. Cada
requisito `R#` lleva su test en **ROJO bajo sabotaje DEL invariante que declara**
(musubi-rules #8, go-hygiene #5): el sabotaje ataca ese invariante, no una
validación cualquiera, o la defensa en profundidad lo tapa y el test queda vacuo.

**Puerta final:** `GOWORK=off go vet ./... && go build ./... && go test -race -count=1 ./...`
(GOWORK=off por el pin desfasado; `-race` NO corre en davantis-1 por diseño del CI; `-count=1` por el cacheado).

---

## A. Núcleo de cognición — `internal/cognition/elegir_skills.go` (NUEVO, calcado de `rerank.go`)

- [ ] **A1.** Tipos: `SkillN1{Name, Description, Cuando string}` (lo mínimo que el motor
  ve de una skill, igual que `Candidato`) y `Eleccion{Skill, Razon string}`
  (`Skill == ""` ⇒ ninguna skill exigida).
- [ ] **A2.** `PromptDecididor(query string, cat []SkillN1) (system, user string)` — la
  **ÚNICA** copia del prompt (producción + banco la comparten, como `PromptJuez`).
  `Description`/`Cuando` saneados con `memory.EnUnaLinea(x, 400)` para que ningún
  salto de línea fabrique candidatos o inyecte una instrucción con forma de lista.
  El system pide **SÓLO** un objeto JSON `{"skill":"<name|>","razon":"<1 frase>"}`,
  a lo sumo UNA skill del catálogo o ninguna (`skill` vacío).
- [ ] **A3.** `ElegirSkill(ctx, p Provider, query string, cat []SkillN1) (Eleccion, error)`:
  arma prompt → `p.Ask` → parseo JSON tolerante (del primer `{` al último `}`) →
  **VALIDA** que `Eleccion.Skill` ∈ `cat` (**R8**: una skill inventada se descarta ⇒
  `Eleccion{}` ninguna). `cat` vacío ⇒ error. **NO** cachea, **NO** fija timeout,
  **NO** gatea — todo eso es del llamador, exactamente como `Rerank`.
- [ ] **A4.** Test `elegir_skills_test.go` con motor **FAKE** (como `methods_ask_test.go`):
  up → elige una válida; devuelve skill fuera del catálogo → ninguna (**R8/E10**);
  basura / no-JSON → error sin panic (**R6**); tolera prosa alrededor del JSON.

## B. Dial default-off — `internal/config` (calcado de `ReadTimeRerankOn`)

- [ ] **B1.** `config.go`: `CognitionConfig += DecididorSkills *bool` con
  `yaml:"decididor_skills,omitempty"` (puntero: distingue "no lo escribieron" de
  "lo apagaron").
- [ ] **B2.** `effort.go`: `DecididorSkillsOn() bool { return c.DecididorSkills != nil && *c.DecididorSkills }`
  (nil ⇒ false, invariante **R2**). **NO** se toca en `ApplyEffort`: el decididor no
  cuelga del preset turbo.
- [ ] **B3.** Test: `{}` ⇒ false; `&false` ⇒ false; `&true` ⇒ true.

## C. Brazo A/B — `internal/cognition/brazo.go` (NUEVO)

- [ ] **C1.** `brazoAB(cohorte string, fecha time.Time) string` ∈ `{"on","off"}`,
  determinista: `hash(cohorte + "|" + fecha.UTC().Format("2006-01-02")) % 2`.
  **Cohorte = `project_id` (tenant), NO el principal crudo**: es la única identidad
  que existe igual en el central (la credencial) y en el local (el sync), así el
  brazo **coincide a los dos lados del sync sin protocolo** (crossover por día, D-cohorte).
- [ ] **C2.** Test **R9/E8**: mismo `(cohorte, fecha)` ⇒ mismo brazo; **existe** una
  fecha que lo voltea para la misma cohorte (barrido corto de días); determinismo
  estable entre corridas.

## D. Entrega estructurada — `internal/memory/recall.go` (**R7**, D4)

- [ ] **D1.** `RecallResult += ExigenciaSkill *ExigenciaSkill` con
  `json:"exigencia_skill,omitempty"`; `type ExigenciaSkill struct { Skill string json:"skill"; Razon string json:"razon" }`
  (tags JSON explícitos, musubi-rules #6: sin ellos el borde emite `{"Skill":...}`).
  **SIN cuerpo** de la skill (nivel-2): preserva el eje `loaded`.
- [ ] **D2.** Test de marshal: emite `skill`/`razon` en minúscula; ausente ⇒ la clave
  se omite (el camino model-free no paga ni un token).

## E. Medición A/B — `internal/memory` (**R10**, D5)

- [ ] **E1.** Migración **v59** (`readCompatible: true`): **REBUILD** de `skill_usage`
  para meter `ab_arm` en la PRIMARY KEY `(skill, project_id, evidence, kind, ab_arm)`
  — patrón de rebuild de v14/v15 (crear tabla nueva, copiar filas viejas con
  `ab_arm=''`, drop+rename, recrear índice). Es read-compatible: un lector v58 hace
  `SUM(n) GROUP BY skill,kind` y el total no cambia aunque las filas se partan por brazo.
- [ ] **E2.** `SkillEvent += ABArm string`; `RecordSkillEvents` lo escribe y el upsert
  usa la PK con `ab_arm`. `evidenciaValida`/`kindValido` sin cambio.
- [ ] **E3.** El registrador del eje `loaded` (hook Fase-0) estampa
  `ABArm = brazoAB(ev.ProjectID, time.Now())`. Así el numerador queda segmentado por
  brazo **con sólo info local** (sin cruzar el sync). `ab_arm=''`/NULL ⇒ fuera del A/B
  (fallback documentado).
- [ ] **E4.** Tabla `decididor_ab(project_id, arm, oportunidades, exigencias, last_at,
  PRIMARY KEY(project_id, arm))` en la misma migración v59. Es el **denominador**
  (oportunidades = el gate pasó) + cuántas veces el motor exigió, por brazo,
  lado-central.
- [ ] **E5.** Recorder async `RegistrarOportunidadDecididor(projectID, arm string, exigio bool)`
  que **encola** (como `SkillEvent`, fuera del lock de dispatch — **R12**) y upsertea en
  `decididor_ab`. Confirmar en implement el camino de flush exacto que ya usan los
  contadores de skills.
- [ ] **E6.** Lector `DecididorAB(ctx) ([]DecididorABRow, error)`: por brazo, junta
  `loaded` de `skill_usage` (numerador) con `oportunidades` de `decididor_ab`
  (denominador). `CalcularLift(rows) float64 = tasa(on) - tasa(off)`, agregación
  **model-free**, sin juez (**R10**).
- [ ] **E7.** Test **R10/E9**: fixture **SEMBRADO** de la cohorte (loaded+oportunidades
  en ambos brazos) — **NUNCA** leído del corpus vivo (gotcha techo auto-consumible);
  el lift se computa sobre lo sembrado; el test que lo custodia **siembra el techo, no
  lo lee**. Sabotaje: romper el `GROUP BY ab_arm` mezcla brazos ⇒ el lift sembrado
  deja de salir ⇒ rojo.

## F. Enganche en `toolRecall` — `internal/mcp/methods_cognition.go` + `methods.go` (D1, D2)

- [ ] **F1.** `exigirSkillSiCorresponde(ctx, query string, res memory.RecallResult, reflejas []skills.SkillResuelta, arsenal []skills.Skill) memory.RecallResult`,
  hermano de `rerankSiCorresponde`. Orden calcado del molde (methods_cognition.go:390):
  1. **GUARD** (R1/R2): `if !cognition.Enabled(s.cognition) || !s.cognitionCfg.DecididorSkillsOn() { return res }`.
  2. **A/B** (R9): `arm := brazoAB(cohorteDe(ctx), time.Now()); if arm == "off" { return res }`.
  3. **GATE conservador** (R3/D2a): `if hayMatchFuerte(reflejas) { return res }`
     (alguna `Matcheo` == `PorAlcance`/`PorGlob`) **+** trivialidad (D3): `query`
     sobre un piso de longitud **y** `len(arsenal) > 0`.
  4. **CACHÉ** fuera del freno (R11): `key := query + "\x00" + hashCat(arsenal)`;
     `decididorCacheGet(key)` → hit ⇒ setea exigencia sin tocar el freno ni el motor.
  5. **FRENO** dentro del miss (R5): `if !s.hayPresupuestoDeMotor(ctx) { s.metrics.motorDenied.Add(1); log; return res }` (degrada, no rechaza).
  6. **TIMEOUT** `context.WithTimeout(ctx, askTimeout)` + `cognition.ElegirSkill(...)`.
  7. **BEST-EFFORT** (R6): cualquier err ⇒ `return res` intacto.
  8. Éxito: `decididorCachePut`, y si `Eleccion.Skill != ""` setea
     `res.ExigenciaSkill` + encola `RegistrarOportunidadDecididor(arm, exigio=true)`;
     si eligió ninguna, igual encola la oportunidad con `exigio=false`.
- [ ] **F2.** `hayMatchFuerte(reflejas)` + `hashCat(arsenal)` (sha256 de name+desc+cuando,
  estable y ordenado) + `cohorteDe(ctx)` (project scope del principal) + `decididorCache`
  (calcado de `rerankCacheGet/Put` o reusando el `cognition.Cache`; **confirmar en
  implement** cuál, con el freno SIEMPRE fuera del hit).
- [ ] **F3.** `toolRecall` (methods.go:1610): tras `rerankSiCorresponde`, cargar
  `arsenal, _ := s.resolver.LoadSkills()` (catálogo nivel-1 COMPLETO = input del motor,
  D2b) y `reflejas, _ := s.resolver.ResolveConDetalle(skills.ResolveRequest{})`
  (reflejo sobre lo que el recall declara — hoy nada ⇒ vacío, gate pasa), y llamar
  `res = s.exigirSkillSiCorresponde(ctx, args.Query, res, reflejas, arsenal)`.
  Degrada con gracia si `resolver`/arsenal son nil (local sin skills).
- [ ] **F4.** Render de la línea imperativa: donde se arma el texto que ve el agente,
  anteponer `⚠️ cargá la skill `+"`X`"+` ahora, para esto: <razon>` cuando
  `res.ExigenciaSkill != nil`, **sin** el cuerpo (**R7**). (El `exigencia_skill`
  estructurado viaja igual para que el consumidor lo lea sin parsear prosa.)

## G. Tests de enganche e invariantes (internal/mcp)

- [ ] **G1.** **R1/E7** local-noop: cognition deshabilitado (NoopProvider) ⇒ `toolRecall`
  no llama al motor ni toca `res`. Sabotaje: quitar el guard ⇒ intenta el motor ⇒ el
  FAKE registra una llamada ⇒ rojo.
- [ ] **G2.** **R2/E6** dial-off-noop: dial ausente/off ⇒ no-op.
- [ ] **G3.** **R3/E1** gate: `reflejas` con una skill `PorGlob` ⇒ el motor **no** se
  llama (gateway_calls no sube). Sabotaje: quitar `hayMatchFuerte` ⇒ se llama ⇒ rojo.
- [ ] **G4.** **R4/E2** una-llamada + exigencia: comodín-only / sin match fuerte,
  presupuesto ok ⇒ el motor se llama **una** vez y `res.ExigenciaSkill` queda seteada,
  la línea antepuesta, sin cuerpo.
- [ ] **G5.** **R5/E3** freno degrada: cuota agotada ⇒ sin motor, `res` intacto,
  `motorDenied += 1`. Sabotaje: mover el chequeo de presupuesto fuera del miss ⇒
  cambia el conteo ⇒ rojo.
- [ ] **G6.** **R6/E4** best-effort: FAKE devuelve error/timeout ⇒ `res` model-free
  intacto, sin exigencia, sin romper.
- [ ] **G7.** **R7** línea-no-cuerpo: la respuesta con exigencia **no** contiene el
  cuerpo (`Rules`) de la skill elegida (aserción explícita del ausente).
- [ ] **G8.** **R11/E5** cache-hit cero-llamadas: segunda tarea idéntica ⇒ no llama al
  motor y reusa la exigencia; el freno **no** se consume en el hit.
- [ ] **G9.** **R12** read-only: `toolRecall` con el decididor activo no muta
  observaciones; el contador `decididor_ab` se escribe por el flusher async, **no**
  bajo el RLock del dispatch (verificar que la tool siga clasificada read-only en
  `registry.go` y que no haya escritura sincrónica bajo lock).

## H. Exposición de la lectura A/B (tool)

- [ ] **H1.** Extender `musubi_skill_usage` (o un corte nuevo en su handler) para
  devolver el lift por brazo vía `DecididorAB`/`CalcularLift`, model-free. **Sin tool
  MCP nueva** ⇒ sin golden nuevo; **verificar en implement** si el esquema de salida de
  `musubi_skill_usage` o `toolRecall` tiene golden y, si cambió, regenerarlo
  (`go test ./internal/mcp -run TestToolsListGolden -update` sólo si corresponde).

## Orden de ejecución sugerido

A → B → C → D (unidades puras, sin dependencias) → E (migración + medición) →
F (enganche, depende de A/B/C/D) → G (tests de enganche) → H (exposición).
Commit por área con asunto imperativo ≤50 (tope 72), cuerpo en viñetas; 1 feature = 1 PR;
entrada en CHANGELOG `[Unreleased]`.

## Riesgos / a confirmar en implement

- **Medición distribuida (E):** el numerador (`loaded`) nace local y el tratamiento
  (exigencia) nace en el central. `brazoAB` sobre `project_id` hace que los brazos
  coincidan sin protocolo, pero el flujo real de datos cruzando el sync se **verifica
  y se flaggea** en verify; el substrato + lector quedan correctos y testeados con
  fixture sembrado en esta PR.
- **Gate dormido en el camino recall (F3):** hoy `musubi_recall` no trae archivos, así
  que `reflejas` sale vacío y el gate R3 nunca corta por match fuerte en producción —
  está vivo y testeado como unidad; cablear señales de reflejo al recall es follow-up
  (coincide con la lista CEDE del diseño).
- **Caché (F2):** confirmar si se reusa `cognition.Cache` (LRU+TTL) o se calca
  `rerankCache`; en ambos el freno va SIEMPRE fuera del hit (R11).
