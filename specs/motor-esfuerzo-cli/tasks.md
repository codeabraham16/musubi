# Tareas — `motor`: `--effort` desde el nivel del cliente

## Implementación (repo Musubi) — COMPLETA

- [x] **T1.** Agregar `Esfuerzo string` a `Pedido` (`motor.go`), con el comentario que explica por qué
  es un campo aparte de `Thinking` y la asimetría del clamp.
- [x] **T2.** Escribir `esfuerzoDesdeReasoning` (`entrada.go`): passthrough de los cinco válidos,
  `minimal→low`, desconocido→`""`, con `TrimSpace`+`ToLower`.
- [x] **T3.** Cablear en `PedidoDesdeProxy` (`entrada.go`):
  `Esfuerzo: esfuerzoDesdeReasoning(p.Options.Reasoning)`.
- [x] **T4.** Emitir `--effort <nivel>` en `argumentosCLI` (`lanzador.go`), con la guarda
  `if p.Esfuerzo != ""`, después de `--model`.

## Pruebas (con sabotaje verificado ROJO) — COMPLETA

- [x] **T5.** `TestElEsfuerzoSaleDelNivelYPasaDerechoAlCLI` (`entrada_test.go`): tabla de 8 casos vía
  `desdeJSON`. Sabotajes E30 (cableado), E31 (passthrough), E32 (`minimal→low`). Valores esperados
  distintos por caso — sin eje fijo que tape un passthrough roto.
- [x] **T6.** Extender `TestElArgvLlevaLasBanderasQueHabilitanElProtocolo` (`lanzador_test.go`) con
  `Esfuerzo: "high"` (aserción del par consecutivo `--effort high`) y el bloque `pelado`
  (`Pedido{}` que NO lleva `--effort`). Sabotajes M52 (`append` borrado), M53 (guarda siempre
  verdadera).
- [x] **T7.** Correr la puerta del proyecto: `GOWORK=off go build ./... && go vet ./... &&
  go test ./internal/motor -count=1` → VERDE.
- [x] **T8.** Sabotear cada uno de los 5 invariantes, verificar ROJO, revertir. 5/5.

## Cierre (repo Musubi) — COMPLETA

- [x] **T9.** Viñeta en `CHANGELOG.md` bajo `[Unreleased] → Added`, documentando la bandera, la
  asimetría y los 5 sabotajes. (El archivo `VERSION` NO se toca: las features no lo bumpean, sólo los
  commits `chore(release)`.)
- [x] **T10.** Debate adversarial (panel `estandar`): 3/3 `real`, cerrado.
- [x] **T11.** Archivar: `specs/motor-esfuerzo-cli/{proposal,spec,design,tasks}.md`.

## Cuerpo (repo thinkrail — cambio SEPARADO, mismo "haz las 2")

- [x] **T12.** `reasoning: true` en los seis modelos de `pi-musubi/index.ts`, con el comentario que
  referencia la cadena `Pedido.Esfuerzo ← esfuerzoDesdeReasoning ← options.reasoning`.
- [x] **T13.** Quitar el provider `anthropic` de la UI del cuerpo (filtrar en el límite del
  *launcher*, NO en bootHost), con test. (La segunda mitad de "haz las 2".) HECHO: `hiddenProviderIds`
  + `setHiddenModelProviders` en `modelAvailability.ts` (filtro en `accountAvailableModels`, el único
  límite de lectura de modelos), activado sólo en `registerBundledRuntime` (`extensions.ts`), export en
  `index.ts`, línea de contrato en `agent/SPEC.md`. Test `modelAvailability.test.ts`: 7/7 verde, 2
  sabotajes nuevos ROJO (quitar el filtro; quitar la guarda `size===0` del corto-circuito), `boot.test.ts`
  21/22 (el login de anthropic intacto).

## Entrega — EN ESPERA DEL OK DEL DUEÑO (acciones disruptivas)

- [ ] **E1.** Compilar el binario `musubi` completo (`GOWORK=off`, tags de treesitter completos).
- [ ] **E2.** Instalar `musubi.exe` nuevo (reinicia los MCP y esta sesión) — **PRIMERO**.
- [ ] **E3.** Rebuild del cuerpo (flip `reasoning:true` + filtro anthropic) — **DESPUÉS**.
- [ ] **E4.** Merge + `musubi receipt emit` + push — sólo con OK explícito.

**Orden de entrega (no negociable por la mecánica):** `musubi.exe` con `--effort` ANTES del rebuild
del cuerpo. Si el cuerpo muestra el selector antes de que el motor sepa leerlo, es un botón muerto.
