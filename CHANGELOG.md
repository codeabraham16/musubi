# Changelog

Todos los cambios notables de Musubi se documentan en este archivo.

El formato sigue [Keep a Changelog](https://keepachangelog.com/es-ES/1.1.0/)
y el proyecto adhiere a [Versionado Semántico](https://semver.org/lang/es/).

## [Unreleased]

### Added
- Soporte multi-agente en `musubi setup`: `--agent <claude|cursor>` registra el servidor MCP
  en la config del agente (`.mcp.json` para Claude, `.cursor/mcp.json` para Cursor). Abstracción
  `AgentTarget` + detección de agentes presentes en el proyecto. Los hooks siguen siendo
  específicos de Claude Code. Track B del roadmap.

## [0.13.0] - 2026-06-18

### Added
- **Motor de orquestación DAG (model-free)** — tool `musubi_workflow` (`start`/`next`/`complete`/`status`/`resume`).
  Musubi define el grafo (`.musubi/workflows/<id>.yaml`), persiste el estado del run en SQLite
  (tabla `workflow_runs`, **resumible entre sesiones**) y devuelve los steps listos; el agente
  ejecuta. Un step queda listo cuando todas sus `needs` están `done` o `skipped`. Tracks A1+A2.
- Control de flujo en workflows: un step puede llevar `when` (expresión model-free, ej.
  `step.build.result contains ok`); si es falsa el step se salta (`skipped`), expresando
  gate/if_then/switch sin tipos de step separados. Evaluador de expresiones seguro (sin eval).
- `musubi_workflow action=resume` para retomar un run en otra sesión (estado + steps listos).
- Loops en workflows: un step con `repeat_while` (+ `max_iterations`, cota anti-infinito) se
  re-ejecuta mientras la condición sea verdadera. Tracks A3.
- `musubi_workflow action=validate` (valida una definición sin correrla) y `action=list`
  (lista los runs con su progreso). Con esto Track A (motor DAG) queda completo.
- Templates de artefactos SDD (`proposal`/`spec`/`design`/`tasks`) versionados: `musubi setup`
  los deja en `.musubi/templates/sdd/`. Scaffold con `schema_version`, idempotente.
- `docs/Roadmap_spec-kit_adoption.md`: plan de orquestación DAG, multi-agente y templates SDD
  (inspirado en spec-kit, adaptado a local-first/model-free).

## [0.12.0] - 2026-06-18

### Added
- Skill cognitiva `audit-structure-flow` en el bundle de arranque: cada `musubi setup`
  la escribe en `.musubi/skills/`. Audita estructura y flujo del codebase (organización,
  acoplamiento, capas, ciclos, código muerto, propagación de context/errores) con
  hallazgos priorizados. También publicada en el catálogo de skills (#47, #48).
- VERSIONINFO del `.exe` reproducible: `cmd/musubi/versioninfo.json` + `go:generate`
  como única fuente de verdad (antes se editaban los `.syso` a mano) (#43).
- README con banner SVG animado y diagramas Mermaid (arquitectura, auto-descubrimiento,
  loop por turno) (#45).

### Changed
- Higiene de estructura (sin cambio de comportamiento): eliminado el paquete huérfano
  `internal/telemetry`; `methods.go` partido (1386→1073) extrayendo el catálogo de tools;
  `main.go` partido (601→207) a `setup.go` e `install.go` (#46).
- Más cobertura de tests en `cmd/musubi` (helpers de setup, calibrate, doctor, catalog) (#44).

## [0.11.0] - 2026-06-18

### Added
- Proveedor de embeddings `openai`: usa la API de OpenAI o cualquier servidor
  compatible con su esquema (LM Studio, vLLM, LocalAI…). La API key se lee de una
  env var (`api_key_env`, default `OPENAI_API_KEY`) y nunca se guarda en el yaml.
- `LICENSE` (MIT), este `CHANGELOG.md` y `CONTRIBUTING.md`.
- Plantillas de issue/PR en `.github/` y badges de CI, release y licencia en el README.

### Changed
- Hardening de robustez: propagación de `context.Context` con timeouts en la capa
  de memoria/embeddings, chequeo de `rows.Err()`, graceful shutdown del daemon
  (SIGINT/SIGTERM), recuperación de panics en los handlers JSON-RPC y validación
  del campo `jsonrpc`.
- Cobertura de tests: `internal/mcp` a 75.8% y `cmd/musubi` a 45.6%.

### Fixed
- `extract_deps`: parseo correcto de dependencias tipo `pydantic[extras]>=2.0`.

## [0.10.0] - 2026-06-16

### Added
- Memoria de código automática: hook `PreToolUse(Read)` que muestra el gist de un
  archivo antes de leerlo (#40).
- Gists de archivos con frescura por fingerprint, model-free (#39).

## [0.9.1] - 2026-06-16

### Changed
- Fin de la doble inyección priming↔turno: el priming siembra el delta (#38).
- Documentado el sistema de eficiencia de tokens; `calibrate` es opcional y gratis.

### Added
- Test de auditoría del footprint de tokens de Musubi (#37).

## [0.9.0] - 2026-06-16

### Added
- Calibración opt-in del estimador de tokens contra `count_tokens`, con
  contabilidad del priming (#36).

## [0.8.0] - 2026-06-16

### Added
- Núcleo de eficiencia de tokens: estimador calibrado + ledger + inyección delta,
  todo model-free (#35).

## [0.7.3] - 2026-06-16

### Fixed
- Resueltos los hallazgos BAJO de la auditoría completa (#34).

## [0.7.2] - 2026-06-16

### Fixed
- Hardening: arreglados los 9 hallazgos ALTO/MEDIO de la auditoría multi-agente (#33).

## [0.7.1] - 2026-06-16

### Changed
- Hardening de la capa de orquestación (auditoría multi-agente) (#31).

## [0.7.0] - 2026-06-16

### Added
- Multi-agente: pizarra compartida (`musubi_work`) para orquestar sub-agentes,
  model-free (#30).

## [0.6.0] - 2026-06-16

### Added
- Loop dirigido + pipeline por fases (`musubi_phase`) para orquestación model-free (#29).

## [0.5.0] - 2026-06-16

### Added
- Resolución de conflictos semánticos entre observaciones, model-free (#28).
- `musubi doctor` con auto-repair (y backup).

## [0.4.0] - 2026-06-15

### Changed
- Mejoras internas y bump de VERSIONINFO del `.exe` (#27).

## [0.3.1] - 2026-06-15

### Fixed
- VERSIONINFO del `.exe` actualizada (#25).

## [0.3.0] - 2026-06-15

### Added
- Auto-update del binario: comando `musubi update` + aviso de versión nueva al
  arrancar el daemon (#24).

## [0.2.4] - 2026-06-14

### Added
- Doble clic en `Musubi.exe` muestra el menú de instalación (local/global) (#18).

## [0.2.3] - 2026-06-14

### Fixed
- Reducción de falsos positivos de antivirus: VERSIONINFO en el `.exe` +
  checksums SHA-256 en las releases (#17).

## [0.2.2] - 2026-06-14

### Changed
- La release publica el binario de Windows como `Musubi.exe` (#16).

## [0.2.1] - 2026-06-14

### Added
- Icono embebido en el `.exe` de Windows (#15).

## [0.2.0] - 2026-06-14

### Added
- Instalador con elección de alcance: local al repo o global en la PC (#13).

## [0.1.0] - 2026-06-13

### Added
- Distribución inicial: instaladores de una línea, workflow de release y setup
  por doble clic.
- Servidor MCP en Go con memoria persistente local-first sobre SQLite (FTS5 +
  búsqueda semántica opcional vía Ollama), resolución dinámica de skills y
  telemetría de errores.

[Unreleased]: https://github.com/codeabraham16/musubi/compare/v0.13.0...HEAD
[0.13.0]: https://github.com/codeabraham16/musubi/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/codeabraham16/musubi/compare/v0.11.0...v0.12.0
[0.11.0]: https://github.com/codeabraham16/musubi/compare/v0.10.0...v0.11.0
[0.10.0]: https://github.com/codeabraham16/musubi/compare/v0.9.1...v0.10.0
[0.9.1]: https://github.com/codeabraham16/musubi/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/codeabraham16/musubi/compare/v0.8.0...v0.9.0
[0.8.0]: https://github.com/codeabraham16/musubi/compare/v0.7.3...v0.8.0
[0.7.3]: https://github.com/codeabraham16/musubi/compare/v0.7.2...v0.7.3
[0.7.2]: https://github.com/codeabraham16/musubi/compare/v0.7.1...v0.7.2
[0.7.1]: https://github.com/codeabraham16/musubi/compare/v0.7.0...v0.7.1
[0.7.0]: https://github.com/codeabraham16/musubi/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/codeabraham16/musubi/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/codeabraham16/musubi/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/codeabraham16/musubi/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/codeabraham16/musubi/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/codeabraham16/musubi/compare/v0.2.4...v0.3.0
[0.2.4]: https://github.com/codeabraham16/musubi/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/codeabraham16/musubi/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/codeabraham16/musubi/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/codeabraham16/musubi/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/codeabraham16/musubi/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/codeabraham16/musubi/releases/tag/v0.1.0
