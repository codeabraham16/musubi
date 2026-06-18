# Evaluación: ¿migrar al SDK oficial de MCP para Go?

**Fecha:** 2026-06-18 · **Decisión:** mantener el JSON-RPC a mano (revisar si cambian las premisas).

## Contexto

Musubi implementa JSON-RPC 2.0 sobre stdin/stdout **a mano** (~3.2k LOC en
[`internal/mcp/`](../internal/mcp)), sin SDK. Tiene un invariante explícito de
**minimizar dependencias**: su `go.mod` declara solo 3 directas
(`google/uuid`, `yaml.v3`, `modernc.org/sqlite`). Esta nota evalúa si conviene
migrar al [SDK oficial de Go](https://github.com/modelcontextprotocol/go-sdk).

## Estado del SDK (verificado)

| Aspecto | Dato |
|---|---|
| Mantenedor | MCP org (Anthropic) **+ Google** |
| Versión | **v1.6.1** (2026-05-22); primera estable **v1.0.0** (2025-09-30) |
| Estabilidad | v1.x con **garantía formal de no romper API** (SemVer; solo deprecaciones aditivas) |
| Spec | Soporta 2025-11-25 con fallback a versiones previas |
| CGo | No (grafo de deps todo Go puro) |
| Requiere | `go 1.25.0` (Musubi está en 1.26 → OK) |

API moderna basada en genéricos (`mcp.AddTool[In, Out]` infiere el JSON Schema
del struct), con modo "raw" (`json.RawMessage`) para conservar schemas a mano.
Transports: stdio, command, in-memory, SSE, streamable-HTTP. Cubre tools,
resources, prompts, notifications, cancelación vía `context` y el handshake
`initialize` con negociación de versión.

Es **lo bastante maduro para producción.**

## Por qué NO migramos ahora

1. **Rompe el invariante de dependencias mínimas.** El SDK agrega ~8 directas +
   ~3 indirectas (`golang-jwt`, `golang.org/x/oauth2`, `x/tools`, `jsonschema-go`,
   `segmentio/encoding`…). Varias existen solo para los transports HTTP/OAuth que
   Musubi **no usa** (es stdio puro). Pasaríamos de 3 a ~11 deps directas.
2. **Perderíamos el panic-recovery centralizado.** El SDK **no** recupera panics en
   handlers ([`server.go`](https://github.com/modelcontextprotocol/go-sdk/blob/main/mcp/server.go)):
   un panic deja al cliente colgado esperando respuesta. Musubi ya tiene `recover()`
   por request en [`server.go`](../internal/mcp/server.go). Habría que re-envolverlo
   igual, neutralizando parte del "menos código".
3. **Cambia el modelo de concurrencia.** El SDK maneja cada request en su propia
   goroutine (asíncrono), no serializado. Habría que auditar la seguridad concurrente
   de la capa de memoria/SQLite.
4. **Timeouts por request los seguiríamos poniendo a mano** (el SDK no impone uno;
   el `WithTimeout(60s)` central del dispatcher actual desaparecería).
5. **El ROI es bajo:** la implementación actual funciona, es stdio puro, cubre las
   ~24 tools y ya está hardened (context, timeouts, recover, validación `jsonrpc`).

## Cuándo reconsiderar

Migrar **sí** paga su costo si Musubi planea adoptar:

- Transports **HTTP/SSE** o **streamable-HTTP** (server remoto, no solo stdio local).
- **OAuth** / autenticación.
- Features de spec que hoy no implementamos: **resources, prompts, sampling**.
- Necesidad de seguir automáticamente versiones nuevas del protocolo sin mantener
  el handshake a mano.

Si seguimos en **stdio + tools**, el código a mano es la opción más alineada con
los principios del proyecto. Esta nota debería revisarse si alguna de las premisas
de arriba cambia.

## Fuentes

- https://github.com/modelcontextprotocol/go-sdk
- https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.0.0
- https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/go.mod
- https://raw.githubusercontent.com/modelcontextprotocol/go-sdk/main/mcp/server.go
- https://pkg.go.dev/github.com/modelcontextprotocol/go-sdk/mcp
