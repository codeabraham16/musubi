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

## Uso

```bash
# 1. Inicializar el workspace (crea .musubi/ con config.yaml y memory.db)
musubi init

# 2. Arrancar el daemon MCP sobre stdin/stdout
musubi daemon
```

`musubi daemon` habla JSON-RPC 2.0 por stdin/stdout, listo para conectarse como servidor MCP
desde Claude Code, Cursor u otro cliente.

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
