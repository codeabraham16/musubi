# Evals del plugin de Musubi

Mide si el plugin de Musubi hace mejor al agente. Cada caso corre dos veces por corrida: **con** el
plugin y **sin** él (`claude plugin eval`, ablación `with-without`), y lo que importa es el Δ entre
los dos brazos. Nada de esto se corrió todavía: la suite está armada y validada sin gastar (ver
«Qué se validó»).

## Los casos

| Caso | Qué mide | Puntúa | Indicador (no puntúa) |
|---|---|---|---|
| `recuerda-puerto` | una respuesta que sólo está en la memoria sembrada (el puerto de staging) | regex `48213` | ¿llamó a `musubi_recall`? |
| `impacto-subtotal` | radio de impacto en un repo de juguete con trampas para Grep | 3 regex sobre la línea `AFECTADAS` | ¿llamó a `musubi_impact`? |
| `implementa-descuento` | implementar una función con tests en un proyecto conocido | forma (regex), reglas (juez llm), tests de tabla (regex) | ¿cargó la skill `sdd-flow`? |
| `memoria-rancia` | una nota VIEJA anclada a un símbolo que cambió: ¿la memoria daña? | regex sobre la línea `RESPUESTA` | ¿fue a leer `texto.go`? |
| `control-normalizar` | CONTROL: un arreglo que no necesita memoria | juez llm + firma (regex) | ¿guardó una trivialidad? |
| `control-primer-contacto` | CONTROL: proyecto nuevo para Musubi, cuyo arranque le pide al agente buscar skills antes de responder | regex del resultado + «no nombra skills ni Musubi» | ¿llamó a `musubi_search_skills`? |

En los dos controles lo esperable es Δ ≈ 0: un Δ negativo ahí es daño de lo que el plugin inyecta.
Los indicadores son `arm: with-only` (o `tool_used: Skill`, que plugin eval excluye solo) y no entran
en el puntaje: dicen QUÉ parte del plugin actuó, no si sirvió.

## Cómo se corre

```bash
scripts/eval-plugin/correr.sh                  # NO gasta: arma todo, muestra plan, costo y comando
scripts/eval-plugin/correr.sh --confirmar --casos recuerda-puerto --corridas 1 --conservar   # piloto
scripts/eval-plugin/correr.sh --confirmar --corridas 3 --techo-usd 15 --modelo <el de siempre>
```

Hace falta un `claude` con `plugin eval` (2.1.269 o más nuevo; el del PATH puede ser más viejo:
`--claude RUTA`), `git` ≥ 2.31 y el binario de `musubi` (`--musubi RUTA`; tiene que llegar al
esquema de base con el que siembra, así que conviene uno compilado desde `main`). El resultado queda
en `~/.cache/musubi-eval-plugin/resultados/<fecha>/` (`aggregate-result.json` y el reporte HTML).

**Primero el piloto**, y en su traza mirar tres cosas antes de gastar en serio: que el brazo con
plugin vea las tools de Musubi (una tool diferida como `musubi_search_skills` necesita `ToolSearch`,
que no está en la lista de tools de solo lectura de plugin eval: no sé si la corrida la tiene), que
el andamio haya pasado (`scaffold failed` deja la corrida en 0) y el `costUsd` real por corrida,
para recalibrar la tabla de abajo.

## Costo

Cada `case.yaml` declara en un comentario el costo de UNA corrida de UN brazo (`correr.sh` lo lee):

| Caso | mínimo | típico | máximo (USD) |
|---|---|---|---|
| `recuerda-puerto` | 0,05 | 0,12 | 0,30 |
| `impacto-subtotal` | 0,15 | 0,40 | 1,00 |
| `implementa-descuento` | 0,30 | 1,00 | 2,50 |
| `memoria-rancia` | 0,05 | 0,12 | 0,30 |
| `control-normalizar` | 0,08 | 0,15 | 0,40 |
| `control-primer-contacto` | 0,05 | 0,15 | 0,50 |
| **suma** | **0,68** | **1,94** | **5,00** |

Total = suma × corridas × 2 brazos + jueces (los 2 graders llm de toda la suite × 3 votos × corrida × brazo, con haiku:
≈ 0,005 USD el voto).

| Corrida | mínimo | típico | máximo |
|---|---|---|---|
| piloto (`recuerda-puerto`, 1 corrida) | 0,10 | 0,24 | 0,60 |
| toda la suite, 1 corrida | 1,42 | 3,94 | 10,06 |
| toda la suite, 3 corridas | 4,26 | 11,82 | 30,18 |

Son estimados, no mediciones: salen de turnos por caso × contexto por turno (~25–40 mil tokens, casi
todo leído de caché) a precio de lista de un modelo grande (del orden de 5 USD por millón de tokens
de entrada y 25 de salida). Con un modelo más chico baja mucho. `--techo-usd` (el `--max-cost-usd`
de plugin eval) corta ANTES de lanzar cada corrida: las que ya estaban en vuelo terminan igual. Es un
estimado a precio de lista, no el consumo del plan.

## Dónde vive y en qué formato

plugin eval busca los casos en `evals/` DENTRO del plugin que evalúa. El plugin de Musubi no vive en
el repo: `musubi agente instalar` lo genera (en `~/.claude/skills/musubi`, `musubi@skills-dir`) y lo
reescribe en cada instalación. Por eso los casos viven acá y `correr.sh` arma en cada corrida un
plugin DE PRUEBA en `~/.cache/musubi-eval-plugin/plugin/` (el mismo generador, con un settings
descartable y sin permisos), le copia los casos y lo evalúa por ruta. El plugin instalado de verdad
no se toca.

Cada caso es una carpeta con `case.yaml` (nombre, costo y `scaffold_script`), `prompt.md` (la
consigna, con `max_turns`, `timeout_seconds` y `allowed_tools` en el frontmatter), `graders/*.md` y
`andamio.sh`. El `andamio.sh` de acá es sólo el CUERPO: `correr.sh` le antepone una cabecera con el
binario de musubi y el nombre del caso, y `lib/andamio-comun.sh`, que tiene todas las verificaciones.

## `--mocks record` y por qué la suite usa `--mocks off`

Con `record` (el valor por defecto), plugin eval NO arranca el servidor MCP real del plugin: registra
un sustituto con el mismo nombre que contesta desde `evals/mocks/<servidor>/<tool>.md`, y una tool
sin mock no existe para el agente. Sin mocks, el servidor `musubi` queda `[not started: no mock]`,
pero los hooks del plugin corren igual (no son MCP) y siguen inyectando la memoria. Un mock contesta
texto fijo: sirve para ver si el agente LLAMA a una tool, no si la memoria de Musubi encuentra lo que
tiene que encontrar, que es lo que miden estos casos. Además el sustituto sólo reemplaza tools (con
`_tools.json` copia descripciones y esquemas): la doc no prevé dónde irían las `instructions` que el
daemon real le manda al agente al inicializar. Por eso la suite corre con `--mocks off`: arranca el
`musubi daemon` real sobre la memoria que siembra el andamio, y sus tools necesitan concesión por
nombre (`--allow-tools mcp__plugin_musubi_musubi__<tool>`).

## Aislamiento: qué memoria abre el plugin

- **La base es la del workspace de la corrida.** El andamio corre en el workspace vacío de cada
  corrida, lo vuelve un repo git y crea ahí `.musubi/` con el mismo binario del plugin; el daemon del
  plugin arranca parado en esa carpeta y Musubi toma como raíz la `.musubi/config.yaml` más cercana.
  El andamio falla si algún ancestro tiene `.git` o `.musubi/config.yaml` (Musubi resolvería ese) y
  fija `project_id: eval-<caso>`. La memoria real (`.musubi/memory.db` de cualquier repo) no está en
  el camino: la corrida tiene HOME, workspace y configuración temporales, y los servidores MCP
  personales no se cargan nunca en plugin eval.
- **Sin cerebro.** El andamio verifica que la memoria sembrada tenga `sync.enabled: false` y
  `central_url` vacío, y `correr.sh` saca por NOMBRE (`env -u`) toda variable con `MUSUBI` en el
  nombre (`MUSUBI_TOKEN`, `MUSUBI_CENTRAL_URL`, `MUSUBI_HOME`...) y las que atan el proceso a la
  sesión de Claude Code que lo lanza; después comprueba contra `env -0` que no llegue ninguna.
- **Sin GitHub.** El daemon que activa una memoria nueva consulta la API pública de GitHub por
  versiones nuevas de musubi (medido: el `CONNECT api.github.com:443` sale aunque el daemon viva
  0,2 s). Todo musubi que corren el andamio y la sonda de `correr.sh` va con un proxy muerto en
  `127.0.0.1:9`, y la memoria sembrada queda con `update.check_interval_hours: -1` (el andamio lo
  verifica), así que el daemon del plugin tampoco consulta durante la corrida. Ese aviso va a
  stderr, no al agente: apagarlo no cambia lo que se mide.
- **Sin publicar.** Siempre `--no-publish`: sin él, plugin eval sube el reporte a claude.ai.
- **Sin fuga del dato.** El andamio falla si el dato que el caso pregunta quedó en algún archivo de
  texto del workspace (incluidos `.git` y `.musubi`): el brazo sin plugin lo encontraría con Grep.
  La base se deja volcada (sin `-wal`), porque el `-wal` sí se lee como texto.

Lo que NO está aislado, y conviene saber: los hooks y el servidor MCP del plugin corren como vos,
FUERA del sandbox del sistema y con red (así funciona plugin eval: por eso marca esos puntajes como
orientativos). `musubi_search_skills` sale a la red a leer catálogos públicos de skills; está
concedida porque `control-primer-contacto` mide justamente si el arranque manda al agente ahí.

## Trampas conocidas

- **No hay modo «sin gastar» en plugin eval** (ni `--dry-run` ni `list`): lo único gratis es
  `--help`. `correr.sh` sin `--confirmar` es ese modo.
- **Un límite de uso no deja la corrida parcial**: las corridas siguientes terminan con error y
  puntúan 0, como una regresión. Antes de creerle a un Δ, mirar `cases[].arms.*[].error`.
- **Fijá `--modelo`** para comparar corridas entre sí; el juez por defecto es haiku.
- **`-j` más de 2 en una máquina de 7 GB** es buscar problemas: cada corrida es un `claude` entero
  más su daemon de musubi.
- **Sin `Bash`**: la suite no lo concede, así que el agente no puede correr `go test`. Los graders
  leen el código (regex y juez), no lo ejecutan.
- **El chequeo de fuga es conservador**: si el dato aparece en cualquier archivo de texto, falla
  aunque el agente nunca lo fuera a encontrar.
- **Recall léxico**: la memoria sembrada tiene la config por defecto (`embedding.provider: none`,
  sin tabla), como un repo nuevo. Si en tu uso real tenés embeddings, la suite no mide eso.

## Qué se validó sin gastar

Los seis andamios corren verdes con un entorno como el del arnés (`env -i` + `PATH`, `HOME`, `TMPDIR`,
`TERM=dumb`), y cada verificación se vio ROJA saboteándola. `correr.sh` se probó con un `claude` falso
que registra argumentos y NOMBRES de variables, y con el real sólo con `--version` y
`plugin eval --help`. Con `strace` (y en un namespace sin red, para que un sabotaje no saliera de
verdad), los andamios y `correr.sh` sólo abren conexiones a `127.0.0.1:9`; sin el proxy muerto,
aparecen las consultas DNS de api.github.com. Las regex de los graders se probaron contra
respuestas buenas y malas. El
detalle, los hallazgos que salieron de armar esto y el análisis de `userConfig` para las credenciales
están en [INFORME.md](INFORME.md).
