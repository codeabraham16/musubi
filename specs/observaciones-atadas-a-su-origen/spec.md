---
artifact: spec
schema_version: "1.0"
change: observaciones-atadas-a-su-origen
status: draft
---

# Especificación — Observaciones atadas al estado que las originó

## Requisitos

### Anclaje (al guardar)

- **R1** — `musubi_save_observation` DEBE aceptar un argumento opcional `origin_paths`: rutas de
  archivo relativas a la raíz del proyecto que la observación describe.
- **R2** — Por cada ruta declarada, el sistema DEBE persistir el par (ruta, fingerprint actual del
  contenido) junto al id de la observación, en el momento del guardado.
- **R3** — Una ruta declarada que NO exista al guardar DEBE rechazar el guardado con un error que
  nombre la ruta. Anclar a algo inexistente produciría una observación marcada como rancia desde
  su nacimiento.
- **R4** — El fingerprint DEBE calcularse con la MISMA función que ya usa el índice de código, para
  que un ancla y un `code_memory.fingerprint` de la misma ruta sean comparables.
- **R5** — Guardar SIN `origin_paths` DEBE comportarse exactamente como hoy: sin filas de ancla y
  sin ningún cambio de salida.
- **R6** — Las anclas NO DEBEN viajar en el outbox de sync al cerebro central: un fingerprint sólo
  tiene sentido contra el checkout de la máquina que lo calculó.

### Derivación (al recuperar)

- **R7** — `musubi_recall` DEBE, para cada observación que entra al resultado y tiene anclas,
  comparar el fingerprint guardado contra el vigente en disco.
- **R8** — Si al menos un ancla difiere, el resultado de esa observación DEBE incluir una marca de
  posible ranciedad que nombre las rutas que cambiaron.
- **R9** — Una ruta anclada que ya no exista DEBE contar como cambio (es la señal más fuerte de
  deriva), y la marca DEBE distinguirla de una que cambió de contenido.
- **R10** — La marca DEBE viajar en el gist, sin necesidad de hidratar la observación completa.
- **R11** — El sistema NO DEBE ocultar, archivar, despriorizar ni borrar una observación por estar
  marcada. La marca es advertencia; la decisión sigue siendo del usuario o de `musubi_judge`.
- **R12** — La verificación DEBE limitarse a las observaciones que ya entraron al resultado. NO
  DEBE recorrer la base entera ni alterar el ranking.
- **R13** — Si una ruta anclada no se puede leer por un error de E/S distinto de "no existe", la
  observación DEBERÍA devolverse SIN marca. Un disco ocupado no es evidencia de deriva.

### Integridad

- **R14** — Borrar una observación DEBE borrar sus anclas. No pueden quedar anclas huérfanas.
- **R15** — `musubi doctor` DEBE reportar anclas huérfanas (sin observación) como inconsistencia
  reparable.

## Escenarios

### Escenario: la nota queda vencida cuando el archivo cambia
- **Given** una observación guardada con `origin_paths=["internal/memory/workflow.go"]`
- **When** ese archivo cambia de contenido y se hace `musubi_recall` de un texto que la recupera
- **Then** el resultado la trae marcada como posiblemente rancia, nombrando `internal/memory/workflow.go`

### Escenario: sin ancla, nada cambia
- **Given** una observación guardada sin `origin_paths`
- **When** se hace `musubi_recall` que la recupera
- **Then** el resultado es idéntico al de hoy, sin marca ni campo extra

### Escenario: el archivo desaparece
- **Given** una observación anclada a `internal/viejo.go`
- **When** ese archivo se borra y se recupera la observación
- **Then** viene marcada, y la marca dice que la ruta ya no existe (distinguible de "cambió")

### Escenario: el archivo no se tocó
- **Given** una observación anclada a un archivo que no cambió desde el guardado
- **When** se la recupera
- **Then** viene SIN marca

### Escenario: anclar a lo inexistente se rechaza
- **Given** un intento de guardar con `origin_paths=["no/existe.go"]`
- **When** se llama a `musubi_save_observation`
- **Then** falla con un error que nombra `no/existe.go`, y no se guarda ni la observación ni el ancla

### Escenario: marcada pero servida
- **Given** una observación marcada como rancia
- **When** se la recupera
- **Then** aparece en el resultado en la misma posición del ranking que tendría sin la marca

### Escenario: el caso real que motivó el cambio
- **Given** una observación cuyo texto dice `PENDIENTE: gateway y bridge`, anclada al archivo que
  describe el estado de esas tareas
- **When** ese archivo cambia porque el pendiente se resolvió
- **Then** la próxima recuperación la trae marcada, sin que ningún agente haya tenido que notarlo

## Fuera de alcance

- Invalidar, archivar u ocultar automáticamente por deriva (R11 lo prohíbe explícitamente).
- Anclar a estado que no sea de archivos del repo: servicios, bases de datos, salidas de comandos,
  estado de máquinas remotas.
- Anclar a símbolos o rangos de líneas en vez de archivos completos. El grafo ya tiene
  `src_fingerprint` por símbolo y sería el refinamiento natural, pero no entra en esta iteración.
- Re-anclar o migrar observaciones existentes.
- Propagar anclas entre nodos de la malla.

## Preguntas abiertas

- [ ] ¿El ancla se infiere sola además de declararse? Hay señal disponible (el hook de codegraph
      conoce los archivos tocados en el turno), pero inferir mal ata la observación a un archivo
      del que no habla y genera ruido permanente. **Propuesta: en esta iteración sólo declarado.**
- [ ] ¿La marca es un campo estructurado del resultado o texto dentro del gist? R10 pide que viaje
      en el gist; falta decidir si además va como campo aparte para que el dashboard lo pinte.
- [ ] ¿Cuántas rutas por observación se permiten? Sin tope, una observación podría anclarse a cien
      archivos y volver caro el recall. **Propuesta: tope bajo y explícito.**
