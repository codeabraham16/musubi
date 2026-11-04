---
artifact: proposal
schema_version: "1.0"
change: mcp-readonlyhint
status: draft
---

# Propuesta — Emitir `readOnlyHint` en el catálogo MCP

## Intención
Musubi emite **0 de 85** tools con la anotación `readOnlyHint` del protocolo MCP. Un cliente que
respeta esa anotación para decidir si pide aprobación —como el agente `pi` del cuerpo (fork de
ThinkRail)— no puede distinguir una lectura de una escritura, así que pide **"Allow" en cada
invocación**. `musubi_recall`, la tool más llamada, quedaría detrás de una aprobación en cada turno.
El registro YA sabe qué tool es de lectura (eje `readOnly` + eje `ro`); esta propuesta expone ese
conocimiento como la anotación estándar, para que el cliente no interrumpa en las lecturas.

## Alcance
- **Incluye:** un sobre `Annotations` en el DTO `Tool` (`internal/mcp/methods.go`); la derivación de
  `readOnlyHint: true` en el único emisor `handleToolsList` (`internal/mcp/registry.go`) cuando la
  tool es `readOnly` **o** `ro == roLeeConEscrituraIncidental`; la regeneración de los **dos** goldens
  de `tools/list`; un test con sabotaje; y la entrada en CHANGELOG con bump menor.
- **No incluye:**
  - Los demás hints del sobre MCP (`destructiveHint`, `idempotentHint`, `openWorldHint`): sólo
    `readOnlyHint`, que es el hallazgo. El resto queda para cuando haga falta.
  - **Reclasificar tools:** los ejes `readOnly`/`ro` se respetan tal cual. `musubi_recall` sigue
    `readOnly:false` a propósito (escribe: refuerza el acceso de lo que devuelve); su hint sale del
    eje `ro`, no de volverla readOnly.
  - **La huella del catálogo:** `catalog_sha256` y `contrato-tools.golden.json` NO se tocan. La
    anotación se setea en la copia local de `handleToolsList`, no en el spec que marshalea
    `huellaDelCatalogo`.

## Enfoque
Derivación pura en el único lugar que arma `tools/list`. `readOnlyHint` es función del eje `ro` que el
registro ya declara —no es un cuarto eje nuevo—, así que no hay fuente de verdad duplicada: una tool
de lectura lo es porque el registro ya lo dice. Un solo punto de emisión ⇒ lo ven todos los clientes
(central, relé y agente) sin sincronizar dos catálogos.

## Impacto
- **Áreas/archivos:** `internal/mcp/methods.go` (campo + struct `ToolAnnotations`),
  `internal/mcp/registry.go` (`handleToolsList`), `internal/mcp/testdata/toolslist.golden.json` y
  `toolslist-completo.golden.json` (regen con `-update`), un test nuevo
  (`internal/mcp/readonly_hint_test.go`), `CHANGELOG.md` ([Unreleased] Added + Destacado).
- **Compatibilidad:** **aditivo.** Con `omitempty`, una tool sin hint serializa **byte-idéntico** a
  hoy. `catalog_sha256` **no se mueve** (la huella marshalea `entries[i].Tool`, el spec crudo —
  verificado en `identity.go:32`—, no la salida de `handleToolsList`). Un cliente que ignora
  `annotations` no cambia de conducta. Sin migración de esquema.

## Riesgos y mitigaciones
| Riesgo | Mitigación |
|--------|------------|
| Un cliente trata `readOnlyHint` como autorización dura, no como hint de UI | Es un HINT del protocolo; la autorización real sigue en el eje `readOnly` del dispatch (RLock/Lock), intacta. No cambia qué puede llamar un principal lector |
| Derivar sólo de `readOnly` deja a `musubi_recall` pidiendo aprobación igual | El test de sabotaje clava esa rama: sacar `\|\| roLeeConEscrituraIncidental` pone ROJO el caso `recall` |
| Derivación siempre-verdadera marca escrituras como lectura | El sabotaje inverso: un mutador puro (`musubi_save_observation`) NO debe emitir el hint |
| Regenerar el golden tapa un cambio real no intencional | El `-update` se corre sobre un diff ya revisado; el golden completo (dormidas incl.) también se regenera |

## Criterio de éxito
`tools/list` emite `annotations.readOnlyHint: true` para las lecturas —incluida `musubi_recall` por la
vía `roLeeConEscrituraIncidental`— y **nada** para las escrituras (`musubi_save_observation`). Un
cliente MCP que respeta la anotación deja de pedir "Allow" en cada lectura. `catalog_sha256` sin
cambios. `go test ./internal/mcp -count=1` verde con los dos goldens regenerados.

## Rollback
Revertir el commit. El cambio es aditivo y aislado: sin migración, sin estado persistido, sin efecto
sobre la huella ni sobre la clasificación de autorización. Un `git revert` deja el catálogo
byte-idéntico al de hoy.
