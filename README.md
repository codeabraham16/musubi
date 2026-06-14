# Musubi

Servidor **MCP (Model Context Protocol)** en Go que funciona como **memoria persistente para
agentes de IA** — al estilo de Engram / Gentle AI. Guarda observaciones, las recupera por
palabra clave (FTS5) o por similitud semántica, resuelve skills dinámicamente según los archivos
en juego y registra telemetría de errores.

Local-first: todo vive en una base SQLite dentro de `.musubi/`. Sin servicios externos
obligatorios; los embeddings son opcionales.

## Requisitos

- (Opcional) [Ollama](https://ollama.com) local para búsqueda semántica.
- Go 1.26+ solo si compilás desde fuente.

## Instalación

### Una línea (binario precompilado)

Windows (PowerShell):

```powershell
iwr -useb https://raw.githubusercontent.com/codeabraham16/musubi/main/scripts/install.ps1 | iex
```

Linux / macOS:

```bash
curl -fsSL https://raw.githubusercontent.com/codeabraham16/musubi/main/scripts/install.sh | bash
```

El instalador descarga el binario de la última release, lo deja en una carpeta de tu PATH y
te queda el comando `musubi` disponible.

> Repo privado: para que la descarga anónima funcione, las releases deben ser accesibles
> públicamente. Si el repo es privado, instalá [gh CLI](https://cli.github.com) y autenticate
> (`gh auth login`) — el instalador usa `gh` como fallback.

### Doble clic (sin terminal)

Copiá `musubi-setup.bat` (y, si no tenés `musubi` en el PATH, también `musubi.exe`) en la raíz
de tu proyecto y hacé **doble clic**. Prepara el entorno en esa carpeta.

### Desde fuente

```bash
go build -o musubi ./cmd/musubi
```

## Uso rápido: inyectar en un proyecto

Un solo comando deja cualquier proyecto listo con Musubi:

```bash
cd mi-proyecto
musubi setup
```

`musubi setup` inyecta todo de punta a punta:

- Crea el workspace `.musubi/` (config + base de datos) en el proyecto.
- Escribe un skill de arranque en `.musubi/skills/`.
- Genera/mergea `.mcp.json` en la raíz, de modo que **Claude Code carga el servidor
  `musubi` automáticamente** al abrir el proyecto (con su propia memoria vía `MUSUBI_HOME`).
- Inyecta un **hook `SessionStart`** en `.claude/settings.json` para el auto-descubrimiento
  de skills (ver sección siguiente).
- Agrega `.musubi/memory.db` al `.gitignore`.

Reabrí el proyecto en Claude Code y las herramientas `musubi_*` quedan disponibles. Es
idempotente: respeta `.mcp.json`, skills, `.gitignore` y `.claude/settings.json` existentes.

## Auto-descubrimiento de skills

Al abrir el proyecto por primera vez en Claude Code, Musubi detecta automáticamente el
stack tecnológico y genera skills personalizadas sin que debas escribir YAML manualmente.

**Flujo completo:**

1. `musubi setup` inyecta en `.claude/settings.json` un hook `SessionStart` que ejecuta
   `musubi detect --hook-mode` al inicio de cada sesión.
2. Al abrir el proyecto, Claude Code ejecuta ese hook. Si el sentinel
   `.musubi/skills/.skills-generated` **no existe**, el hook emite instrucciones JSON
   que Claude recibe como contexto adicional.
3. Claude llama a `musubi_detect_stack` (detecta ecosistemas y frameworks inspeccionando
   manifests: `go.mod`, `package.json`, `Cargo.toml`, etc.).
4. Claude investiga la documentación **oficial** del stack detectado (`pkg.go.dev`,
   `react.dev`, `docs.python.org`, etc.) y sintetiza reglas.
5. Claude **confirma las reglas con el usuario** antes de guardar.
6. Por cada skill aprobada, Claude llama a `musubi_save_skill`, que escribe el archivo
   `.musubi/skills/{name}.yaml` y el sentinel. A partir de ahí el hook es silencioso
   (no vuelve a disparar hasta que borres el sentinel).

**Para regenerar las skills:** borrar `.musubi/skills/.skills-generated` y reabrir
el proyecto.

## Uso manual

```bash
# Inicializar solo el workspace (crea .musubi/ con config.yaml y memory.db)
musubi init

# Detectar el stack del proyecto (imprime JSON en stdout)
musubi detect

# Modo hook interno (usado por Claude Code al iniciar sesión)
musubi detect --hook-mode

# Arrancar el daemon MCP sobre stdin/stdout
musubi daemon
```

`musubi detect` inspecciona el directorio actual (o `MUSUBI_HOME`) y devuelve un JSON con
los ecosistemas y frameworks detectados. Es de solo lectura: no crea ni modifica archivos.

`musubi detect --hook-mode` es el modo que Claude Code invoca automáticamente al abrir el
proyecto. Si el sentinel `.musubi/skills/.skills-generated` existe, no produce output (silencioso).
Si no existe, emite el JSON de guía para que Claude inicie el flujo de auto-descubrimiento.

`musubi daemon` habla JSON-RPC 2.0 por stdin/stdout, listo para conectarse como servidor MCP
desde Claude Code, Cursor u otro cliente. Respeta la variable de entorno `MUSUBI_HOME` para
fijar el directorio del workspace (por defecto, el directorio actual).

## Configuración (`.musubi/config.yaml`)

```yaml
version: "1.0"
mode: local
skills_auto_resolve: true
embedding:
  provider: none          # none | ollama
  model: nomic-embed-text
  base_url: http://localhost:11434
  dimensions: 768
```

- `provider: none` (por defecto): la búsqueda semántica queda desactivada y `musubi_search_semantic`
  responde con un error explícito sugiriendo usar la búsqueda por palabra clave.
- `provider: ollama`: el servidor genera embeddings llamando a Ollama
  (`POST {base_url}/api/embeddings`). Los agentes pasan **texto**, no vectores.

Para activar embeddings con Ollama:

```bash
ollama pull nomic-embed-text
# editar .musubi/config.yaml -> embedding.provider: ollama
```

## Herramientas MCP

| Herramienta | Descripción |
|-------------|-------------|
| `musubi_save_observation` | Guarda una observación (`topic_key`, `content`, `id` opcional). Si hay embeddings, indexa para búsqueda semántica. |
| `musubi_search_semantic` | Busca por similitud a partir de **texto** (`query`). Requiere proveedor de embeddings. |
| `musubi_search_keyword` | Busca por texto completo FTS5 (`query_text`). Siempre disponible. |
| `musubi_log_error` | Registra un error de compilación/test para telemetría. |
| `musubi_resolve_telemetry` | Marca un log de telemetría como resuelto (`id`). |
| `musubi_resolve_skills` | Resuelve skills activas según `modified_files` + telemetría sin resolver. |
| `musubi_detect_stack` | Detecta el stack del proyecto (ecosistemas + frameworks) inspeccionando manifests. Sin parámetros. |
| `musubi_save_skill` | Guarda una skill generada como `{name}.yaml` en `.musubi/skills/` y crea el sentinel. Requiere `name`, `triggers`, `rules`. Parámetro opcional `overwrite` (por defecto `false`). |

## Tests

```bash
go test ./...            # suite completa
go test -race ./...      # con detector de carreras (como en CI)
```

## Arquitectura

```
cmd/musubi/        # CLI: setup, detect, init, daemon
internal/
  bootstrap/       # inyección: MergeMCPServer + MergeClaudeSettings (hooks)
  config/          # constantes de rutas + carga de config.yaml
  detector/        # DetectStack: inspección de manifests sin deps externas
  embedding/       # Provider (interfaz) + Ollama + Noop
  logx/            # logging estructurado a stderr
  mcp/             # servidor JSON-RPC 2.0 y herramientas MCP
  memory/          # SQLite: observaciones, embeddings, FTS5, telemetría, vectores
  skills/          # resolver dinámico de skills (triggers + capabilities)
```

## Estado y roadmap

Núcleo endurecido y cubierto con tests. Diferido a propósito:

- Orquestador / motor DAG.
- Loop de auto-corrección hot-patch (telemetría → parche automático → reintento). Hoy existe el
  registro y la resolución manual de telemetría.
- Escalado del índice vectorial: la búsqueda semántica recorre todos los vectores en memoria
  (O(n)), suficiente para volúmenes de prototipo.
