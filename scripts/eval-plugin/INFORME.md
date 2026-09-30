# Informe: suite de evals del plugin de Musubi

Unidad `a87cf046-720f-4e5a-a379-6d2a60c6d8ea`, rama `medir/eval-plugin` (sale de `704c5013`).
**La evaluación no se corrió**: todo lo de acá se validó sin gastar. El cómo se usa está en
[README.md](README.md); este archivo tiene las respuestas a las preguntas de la unidad, lo que se
validó, los hallazgos del producto y el análisis de `userConfig` (parte B).

## 1. Formato de los casos y dónde vive `evals/`

- `claude plugin eval` (hace falta **2.1.269 o más nuevo**) busca los casos en `evals/` **dentro del
  plugin que evalúa**. Sin argumentos, evalúa el plugin que contiene la carpeta actual.
- Un caso es una carpeta: `case.yaml` (`schema_version: "1.1"`, `name`, `context.scaffold_script`),
  `prompt.md` (la consigna; en el frontmatter `description`, `expected_outcome`, `tags`, `max_turns`
  —10 por defecto—, `timeout_seconds` —300— y `allowed_tools`) y `graders/*.md` (el frontmatter dice
  el tipo: `regex`, `tool_used`, `llm`...; en uno `llm`, el cuerpo es la rúbrica).
- Detalles que importan y no son obvios: las regex son de **JavaScript** (`flags: i`, no `(?i)`);
  `tool_used` con `input_match` compara contra el input **serializado en JSON**; un grader `llm` pasa
  con 2 de 3 votos; `target: {source: file, path}` lee el archivo DESPUÉS de la corrida; los graders
  de `Skill` no puntúan en una corrida de dos brazos; `arm: with-only` deja un grader como
  indicador del brazo con plugin; las claves de `env` del caso tienen que ser `EVAL_*`.
- **El plugin de Musubi no vive en el repo**: lo genera `musubi agente instalar` y lo reescribe en
  cada instalación. Por eso los casos viven en `scripts/eval-plugin/evals/` y `correr.sh` arma en
  cada corrida un plugin DE PRUEBA en `~/.cache/musubi-eval-plugin/plugin/` (el mismo generador, con
  un settings descartable y `--sin-permisos`), le copia los casos y lo evalúa por ruta. El plugin
  instalado de verdad no se toca. `instalarPlugin` se niega a escribir en una carpeta con archivos
  ajenos, así que la marca de «esto lo armó correr.sh» va AL LADO de la carpeta, no adentro.

## 2. `--mocks record` y lo que necesitan los casos de memoria

Con `record` (el default), plugin eval **no arranca** el servidor MCP real: registra un sustituto con
el mismo nombre que contesta desde `evals/mocks/<servidor>/<tool>.md`. Una tool sin mock no existe
para el agente, y un servidor sin mocks queda `[not started: no mock]`. Con `_tools.json` el
sustituto copia descripciones y esquemas, pero la doc no prevé dónde irían las `instructions` que el
daemon real manda al inicializar. Los hooks del plugin corren igual (no son MCP).

Un mock sirve para ver si el agente LLAMA a una tool, no si la memoria encuentra lo que tiene que
encontrar. Por eso la suite usa `--mocks off`: arranca el `musubi daemon` real sobre la memoria que
siembra el andamio. Con `off`, las tools del plugin necesitan concesión por nombre
(`mcp__plugin_musubi_musubi__<tool>`). `correr.sh` concede 29 (memoria, grafo de código, SDD,
pizarra y skills; nada de flota, tokens, sync, cerebro ni motor cognitivo) y antes de lanzar nada
comprueba contra `tools/list` del daemon que existan todas: una concesión con un nombre que no
existe no da error, sólo no concede.

**Sin resolver:** plugin eval deja sólo tools de lectura en `allowed_tools` (Read, Glob, Grep,
NotebookRead, Skill, AskUserQuestion, Agent, TodoWrite, Task*). `ToolSearch` no está en esa lista.
Las tools MCP de Musubi llegan diferidas, así que no sé si el brazo con plugin puede cargarlas. Es lo
primero que hay que mirar en la traza del piloto.

## 3. Aislamiento

- **Qué base abre el plugin.** La del workspace de la corrida. El andamio corre en el workspace
  vacío, lo vuelve un repo git y crea `.musubi/` con el mismo binario que usa el plugin. Musubi
  resuelve la raíz así: `MUSUBI_HOME`, después el directorio de la sesión, después el ancestro más
  cercano con `.musubi/config.yaml` o la raíz de git. El andamio falla si algún ancestro tiene `.git`
  o `.musubi/config.yaml`, y fija `project_id: eval-<caso>`. Lo comprobé arrancando el daemon como
  lo arranca el plugin (sin `MUSUBI_HOME`, parado en la carpeta): trae lo sembrado.
- **La memoria real no está en el camino.** El workspace, el HOME y la configuración de la corrida
  son temporales, y en plugin eval los servidores MCP personales no se cargan nunca.
- **Sin cerebro.** El daemon toma el sync de `.musubi/config.yaml` (`sync.central_url` y
  `sync.auth_token_env`), no de `MUSUBI_CENTRAL_URL`. El andamio verifica `sync.enabled: false` y
  `central_url` vacío. `correr.sh` saca además por NOMBRE (`env -u`) toda variable que contenga
  `MUSUBI` y las que atan el proceso a la sesión de Claude Code. En esta sesión fueron 10:
  `MUSUBI_CENTRAL_URL`, `MUSUBI_TOKEN_FILE`, `MUSUBI_BIN`, `ALTURA_MUSUBI_TOKEN_FILE` y seis de la
  sesión. Después comprueba con `env -0` que no llegue ninguna: al `claude` falso le llegaron 97
  nombres, ninguno con MUSUBI.
- **Sin GitHub (arreglado en esta unidad).** El daemon que activa una memoria nueva consulta
  `api.github.com/repos/codeabraham16/musubi/releases/latest` para avisar de versiones nuevas. Esa
  consulta no lleva credencial y se repite cada 24 h por memoria. Medido con un proxy falso local:
  el `CONNECT api.github.com:443` sale aunque el daemon viva 0,2 s. Ahora todo musubi que corren el
  andamio y la sonda de `correr.sh` va con un proxy muerto en `127.0.0.1:9`. La memoria sembrada
  queda con `update.check_interval_hours: -1`, y el andamio falla si no es así, así que el daemon
  del plugin tampoco consulta durante la corrida. El aviso va a stderr, no al agente: apagarlo no
  cambia lo que se mide.
- **Sin publicar.** Siempre `--no-publish`: en 2.1.285 publicar el reporte en claude.ai es el
  default si la cuenta lo permite. El `claude` del PATH (2.1.223) tiene un `plugin eval` anterior a
  ese cambio (ahí publicar era optativo, con `--publish-report`) y no conoce `--no-publish`,
  `--mocks` ni `--trust-plugin`. `correr.sh` exige esos flags en la ayuda y se niega. El que sirve
  está en la extensión de VS Code (2.1.285): `--claude RUTA`.
- **Siempre los dos brazos.** `correr.sh` pasa `--ablation with-without` explícito. En 2.1.223 el
  default para un plugin evaluado por ruta, que es como se evalúa acá, era `none`: sin el brazo sin
  plugin, no hay Δ. En 2.1.285 es `with-without` sólo si el plugin «resuelve» desde la ruta.
- **Sin fuga del dato.** El andamio falla si el dato que el caso pregunta quedó en algún archivo de
  texto del workspace, incluidos `.git` y `.musubi`. La base se deja sin `-wal` (ver hallazgo 3).
- **Lo que NO se aísla.** Los hooks y el servidor MCP del plugin corren como el usuario, fuera del
  sandbox y con red. Así funciona plugin eval, y por eso marca esos puntajes como orientativos.
  `musubi_search_skills` sale a catálogos públicos. Está concedida porque `control-primer-contacto`
  mide justamente eso. En los andamios, `detect --hook-mode` (el hook de arranque) no abrió ninguna
  conexión (strace). `turn`, `precheck`, `capture` y `precompact` no los ejercita el andamio: que
  no usan red lo digo sólo por leer el código.

## Los casos y el costo

Seis casos: `recuerda-puerto`, `impacto-subtotal`, `implementa-descuento`, `memoria-rancia`,
`control-normalizar` y `control-primer-contacto`. La tabla con qué mide cada uno está en el README.
Todos usan datos sintéticos.

Costo estimado por corrida de UN brazo (USD, precio de lista de un modelo grande, sin medir):

| Caso | mín | típico | máx |
|---|---|---|---|
| recuerda-puerto | 0,05 | 0,12 | 0,30 |
| impacto-subtotal | 0,15 | 0,40 | 1,00 |
| implementa-descuento | 0,30 | 1,00 | 2,50 |
| memoria-rancia | 0,05 | 0,12 | 0,30 |
| control-normalizar | 0,08 | 0,15 | 0,40 |
| control-primer-contacto | 0,05 | 0,15 | 0,50 |

Total = Σ casos × corridas × 2 brazos + jueces (2 graders llm × 3 votos × 0,005 USD por corrida y
brazo). Piloto (`recuerda-puerto`, 1 corrida): **0,10 / 0,24 / 0,60**. Suite con 1 corrida:
**1,42 / 3,94 / 10,06**. Suite con 3 corridas: **4,26 / 11,82 / 30,18**. `correr.sh` hace esta
cuenta sola para lo que se elija. Los tres números de la suite los recalculé a mano y coinciden.

## Qué se validó sin gastar

- **`correr.sh` con un `claude` falso** que registra argumentos y NOMBRES de variables, nunca
  valores:
  - once pruebas: ayuda; plan sin `--confirmar`, que sale 0 sin invocar la evaluación;
    `--confirmar`, con los argumentos correctos (`--no-publish` y `--ablation with-without`
    incluidos), `--allow-tools` al final y el entorno del hijo sin ningún nombre con MUSUBI ni de
    la sesión; `--casos 'control-*'`, con el estimado verificado a mano; glob sin coincidencias;
    valores inválidos de `--corridas`, `-j`, `--techo-usd` y de una opción desconocida; claude
    viejo; una ayuda sin `--ablation`; salidas 2 y 1; aviso de techo; carpeta ajena;
  - tres sabotajes, cada uno bloqueado por la razón correcta: sin filtrar `*MUSUBI*`; una tool
    inexistente en la lista; un caso sin la línea de costo.
- **Con el `claude` real** sólo se corrieron `--version`, `plugin eval --help` y
  `plugin validate --json`, que es local. El validate dio verde, con un aviso por falta de `author`.
- **Los seis andamios** corren verdes con un entorno como el del arnés (`env -i` + PATH, HOME,
  TMPDIR, TERM=dumb), en 0,7–1,5 s, sin `-wal`. Cada verificación se vio ROJA saboteándola:
  - A: sin sembrar → «el recall del plugin no trae el puerto sembrado»;
  - B: el dato en `frontend/README.md` → fuga;
  - C1/C2: un ancestro con `.musubi` o con `.git`;
  - D: consumir la oferta de primer contacto antes de verificarla → «no ofrece descubrir skills»;
  - E: sin asentar la base → `-wal`, 3 de 3;
  - F: `central_url` con valor;
  - G: sin el proxy muerto → consultas DNS de api.github.com;
  - H: sin apagar el chequeo de versión → «update.check_interval_hours no es negativo».
- **Red, con `strace`.** G y la sonda de `correr.sh` se corrieron dentro de un namespace sin red
  (`unshare -rn`), para que el sabotaje no saliera de verdad. Arreglados, los andamios y `correr.sh`
  abren una sola conexión, a `127.0.0.1:9`. Saboteados, aparecen 4 consultas a `127.0.0.53:53`.
- **Graders:** un validador de esquema (PyYAML) con su sabotaje, y 48 pruebas de las regex en node
  contra respuestas buenas y malas, también con sabotaje.
- **Lo que NO se validó:** la corrida misma; si el brazo con plugin puede cargar las tools diferidas
  sin `ToolSearch`; que `--allow-tools` con `--mocks off` conceda como dice la doc; los costos, que
  son estimados.

## Hallazgos del producto (salieron de armar la suite)

1. **La marca de «posiblemente rancia» no llega al agente por el camino principal.**
   `musubi_recall` marca la nota anclada cuyo símbolo cambió (`"stale":[...]`):
   `markStaleOrigins` (internal/memory/origins.go) tiene un solo llamador,
   internal/memory/recall.go:341. El priming del arranque (`buildPrimingContext` en
   cmd/musubi/detect.go → `PrimeContextCtx` en internal/memory/prime.go) no pasa por ahí y la
   inyecta SIN la marca, y el hook de cada turno la deduplica porque ya la inyectó el arranque: en
   una sesión normal el agente ve la nota vieja como vigente. `musubi_memory_expand` también
   devuelve el contenido sin la marca. `memoria-rancia` mide el daño.
2. **El grafo de código pierde las llamadas a métodos sobre un parámetro.** Acierta con las llamadas
   calificadas entre paquetes y con los homónimos (7 callers exactos), pero `c.Subtotal()` con
   `c Carrito` le da 0 callers a `Carrito.Subtotal`. La advertencia del hook de lectura («no ve
   llamadas por interfaz ni métodos pasados como valor») se queda corta.
3. **El daemon que activa una memoria nueva sale sin volcar el `-wal`**: 7 de 8 veces
   `memory.db` queda con 4 KB y todo lo demás en el `-wal`. El `-wal`, a diferencia de la base, se
   lee como texto. `musubi tareas --json` abre, lee y cierra bien: eso la vuelca.
4. **El primer contacto le da órdenes al agente.** En un proyecto nuevo, el arranque inyecta «ANTES
   de responder al usuario: musubi_search_skills… CONFIRMAR con el usuario». Una pregunta cualquiera
   puede terminar en una búsqueda en la red y en una consulta al usuario. `control-primer-contacto`
   mide eso. Es en la SEGUNDA sesión: en la primera el plugin recién crea `.musubi/` y los hooks no
   inyectan nada.
5. **Cada memoria nueva consulta GitHub al activarse** (sección 3), sin credencial y con un límite
   de 60 por hora por IP. En sandboxes, CI o evals, eso es una consulta por corrida. **Mis propias
   pruebas lo dispararon unas 60 veces antes de que lo notara** (48 memorias nuevas en el
   scratchpad más las sondas de `correr.sh`): fueron GET sin credencial a la API pública de
   releases, sin datos de la memoria. No sé cuántas terminaron: el daemon vive 0,2 s.
6. **`skills_auto_resolve`** (config.go) no tiene ningún consumidor: es config muerta.
7. **`recuerda-puerto`:** el priming del arranque ya trae el 48213. El brazo con plugin lo tiene sin
   llamar a nada, y por eso `musubi_recall` es un indicador y no puntúa.
8. **Herramientas:** con `LC_NUMERIC=es_VE`, mawk imprime con coma decimal. `correr.sh` usa
   `LC_ALL=C awk`.

## Parte B: `userConfig` para `MUSUBI_TOKEN` y `MUSUBI_CENTRAL_URL`

Nada de esto se implementó ni se probó: es lectura de la doc y del código.

**Hoy.** El `.mcp.json` del plugin generado es sólo
`{"mcpServers":{"musubi":{"command":"<exe>","args":["daemon"]}}}`, sin `env`. El daemon hereda el
entorno del proceso de Claude Code y toma el sync de `.musubi/config.yaml` de cada proyecto:
`sync.central_url`, y `sync.auth_token_env`, que NOMBRA la variable del token. `MUSUBI_CENTRAL_URL`
lo leen sólo `musubi cerebro` y el dashboard.

**Qué pasa hoy si falta la URL:**

- El daemon del plugin no sincroniza, y en silencio.
- `musubi cerebro` sale con 1 y «falta la URL del cerebro (--url o $MUSUBI_CENTRAL_URL)» a stderr,
  que va al log del MCP. El agente no ve nada: «muere en el arranque sin error visible», como dice
  CLAUDE.md.

**Cómo sería.** En `plugin.json`:

```json
"userConfig": {
  "central_url": { "type": "string", "title": "URL del cerebro", "description": "…" },
  "musubi_token": { "type": "string", "title": "Token de Musubi", "description": "…", "sensitive": true }
}
```

y en `.mcp.json`, `"env": {"MUSUBI_TOKEN": "${user_config.musubi_token}"}`. `${user_config.KEY}` se
sustituye en la config de servidores MCP (command, args, env), en los `args` de hooks exec-form y
en skills y agentes; ahí sólo los valores no sensibles. Para la URL hay dos caminos:
`args: ["cerebro", "--url", "${user_config.central_url}"]` en un servidor aparte, o código nuevo
para que el daemon la tome del entorno, porque hoy no la lee.

**Dónde quedan los valores.** Los no sensibles, en `settings.json` del usuario, bajo
`pluginConfigs["<plugin>@<marketplace>"].options`: sólo alcance usuario o managed, no por proyecto.
Los `sensitive`, en el llavero de macOS. **Donde no hay llavero soportado van a
`~/.claude/.credentials.json`** (la doc sólo nombra el de macOS, así que eso es Linux y,
probablemente, Windows): un archivo en claro, igual que hoy `~/.bashrc`, sólo que fuera de
`settings.json`.

**Cuándo se piden.** El diálogo aparece sólo en el `/plugin` interactivo: al instalar desde un
marketplace, al habilitar o con `/plugin configure`. `claude plugin install` nunca pregunta y
acepta `--config KEY=VALUE`. Para un plugin `skills-dir` (así carga hoy el de Musubi,
`musubi@skills-dir`) no está documentado. `claude plugin --help` lista
`configure <plugin> … --values-stdin`, pero no lo probé.

**Si falta un valor.** Para los monitores está documentado que no arrancan. Para un servidor MCP que
referencia un `${user_config.X}` sin valor, no lo encontré documentado.

Riesgos:

1. **El generador pisa el manifiesto.** `instalarPlugin` reescribe `plugin.json` y `.mcp.json` en
   cada `musubi agente instalar`. El `userConfig` tiene que emitirlo el generador, o se pierde en
   la próxima actualización.
2. **El token llega a TODO hook.** `CLAUDE_PLUGIN_OPTION_<KEY>` se exporta a cada proceso de hook,
   sensibles incluidos. El token quedaría en el entorno de `detect`, `turn`, `precheck`, `capture`
   y `precompact` en cada turno. Las seis credenciales expuestas que registra la memoria salieron
   todas de imprimir el entorno.
3. **«sensitive» no es cifrado fuera de macOS** (ver arriba).
4. **El esquema es estricto.** Un `userConfig` mal formado hace que el plugin entero no cargue, y
   Musubi desaparece de todas las sesiones. Hay que pasarlo por `claude plugin validate`.
5. **Rotar el token no alcanza.** Los daemons vivos siguen con el valor viejo hasta reiniciarse, y
   sus 401 pueden bloquear por IP a la credencial nueva.
6. **La URL cambia de alcance.** `pluginConfigs` vale para la máquina y `sync.central_url`, para el
   proyecto: hay que decidir cuál gana.

Antes de implementarlo conviene medir dos cosas, gratis y en un `CLAUDE_CONFIG_DIR`/HOME
temporal: si un plugin `skills-dir` respeta `userConfig` (y `configure --values-stdin`), y qué hace
Claude Code con un servidor MCP que referencia un valor sin completar.
