# Musubi

Servidor **MCP (Model Context Protocol)** en Go que funciona como **memoria persistente para
agentes de IA** — al estilo de Engram / Gentle AI. Guarda observaciones, las recupera por
palabra clave (FTS5) o por similitud semántica, resuelve skills dinámicamente según los archivos
en juego y registra telemetría de errores.

Local-first: todo vive en una base SQLite dentro de `.musubi/`. Sin servicios externos
obligatorios; los embeddings son opcionales.

## Requisitos

- Go 1.26+
- (Opcional) [Ollama](https://ollama.com) local para búsqueda semántica.

## Build

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
- Agrega `.musubi/memory.db` al `.gitignore`.

Reabrí el proyecto en Claude Code y las herramientas `musubi_*` quedan disponibles. Es
idempotente: respeta `.mcp.json`, skills y `.gitignore` existentes.

## Uso manual

```bash
# Inicializar solo el workspace (crea .musubi/ con config.yaml y memory.db)
musubi init

# Arrancar el daemon MCP sobre stdin/stdout
musubi daemon
```

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

## Tests

```bash
go test ./...            # suite completa
go test -race ./...      # con detector de carreras (como en CI)
```

## Arquitectura

```
cmd/musubi/        # CLI: init, daemon
internal/
  config/          # constantes de rutas + carga de config.yaml
  embedding/       # Provider (interfaz) + Ollama + Noop
  logx/            # logging estructurado a stderr
  mcp/             # servidor JSON-RPC 2.0 y herramientas
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
