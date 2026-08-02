---
artifact: proposal
schema_version: "1.0"
change: observaciones-atadas-a-su-origen
status: draft
---

# Propuesta — Observaciones atadas al estado que las originó

## Intención

Una observación puede seguir siendo recuperada como verdad vigente mucho después de que el
mundo del que hablaba cambió. Hoy Musubi no tiene forma de **derivar** eso: sólo compara
observaciones entre sí, así que una nota válida con una línea vencida adentro es invisible
para el detector de conflictos.

Pasó tres veces el 2026-08-01, con costo real:

1. Una nota decía `PENDIENTE: gateway y bridge` horas después de que ambos quedaran arreglados.
2. Tres notas del 2026-07-07 afirmaban que el tailnet era inalcanzable con NordVPN, un mes
   después de que dejara de ser cierto — y hubo que **borrarlas en duro** porque `supersedes`
   no las alcanzaba.
3. Una corrección pegada arriba de una nota vieja quedó ella misma desactualizada, dejando tres
   capas contradictorias sobre el mismo hecho.

El patrón es siempre el mismo: **el conocimiento no envejece, el mundo se mueve debajo.**

## Alcance

- **Incluye:**
  - Un ancla opcional entre una observación y los archivos del proyecto de los que habla
    (ruta + fingerprint al momento de guardar).
  - Derivación en tiempo de recall: comparar el fingerprint guardado contra el actual y
    **marcar** la observación como posiblemente rancia, diciendo qué cambió.
  - Que la marca viaje en el gist del recall, para que se vea sin hidratar la observación.
  - Que el ancla se pueda declarar al guardar y también inferir de lo que ya sabe Musubi.

- **No incluye:**
  - Ocultar, archivar o borrar nada automáticamente. La señal es **advertencia, no veredicto**:
    que un archivo cambie no prueba que la observación sea falsa. Decidir sigue siendo del
    usuario o de `musubi_judge`.
  - Anclar a estado que no sea del repositorio (servicios, bases, salidas de comandos).
  - Reescribir observaciones existentes: sin ancla se comportan exactamente como hoy.
  - Sustituir el detector de conflictos. Son ortogonales: uno compara notas entre sí, éste
    compara una nota contra el mundo.

## Enfoque

Reusar lo que ya existe en vez de inventar. Musubi **ya calcula y persiste fingerprints de
archivo**: `code_memory.fingerprint` (111 filas en la base local) y `code_graph_nodes.src_fingerprint`
(3771). El aparato de detección de deriva para código ya está construido y probado —
`musubi_detect_changes` vive de él. Lo único que falta es el **puente**: hoy no hay ninguna
tabla que relacione una observación con un archivo.

Entonces: una tabla satélite `observation_origins(observation_id, path, fingerprint, captured_at)`
y, en el recall, una comparación contra el fingerprint vigente. Sin ancla no cambia nada.

Es exactamente el mismo invariante que acabamos de aplicar al gate de verificación en
`feat/verify-gate-candidato-congelado`: **atar una afirmación a una identidad de contenido, y
re-derivarla en el momento de usarla.** Allá protege un veredicto; acá protege un recuerdo.

## Impacto

- Áreas/archivos afectados:
  - `internal/memory/` — esquema (tabla nueva), escritura del ancla al guardar, derivación en
    el recall.
  - `internal/mcp/` — argumento nuevo en `musubi_save_observation`; el gist del recall lleva la
    marca. `registry.go` + golden.
  - Sin cambios en el motor de workflows ni en la capa de cognición.

- Compatibilidad:
  - **Tabla nueva, ninguna columna modificada.** Las observaciones existentes no se tocan.
  - Sin ancla, `save` y `recall` se comportan idénticamente a hoy — la feature es opt-in.
  - El sync al cerebro central no cambia de formato: el ancla es local a cada nodo, porque un
    fingerprint sólo tiene sentido contra el checkout de esa máquina.

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|--------|------------|
| Costo de hashear archivos en cada recall | Sólo se verifican las observaciones que entran al resultado, no toda la base; pre-chequeo barato por mtime/tamaño antes de leer; reusar el fingerprint ya calculado por el codegraph cuando esté fresco |
| Falsos positivos: un `gofmt` marca todo como rancio | La marca es advertencia, nunca oculta; el mensaje dice QUÉ archivo cambió para que se juzgue en un vistazo |
| Falsos negativos: la observación envejece sin que el archivo cambie | Reconocido y declarado: esto cubre el conocimiento anclado a código, no todo. No reemplaza al juez |
| Anclas que apuntan a archivos borrados o movidos | Un archivo que ya no existe es la señal más fuerte de deriva, no un error: se marca como tal |
| Scope creep hacia "invalidación automática" | Explícitamente fuera de alcance en esta propuesta |

## Criterio de éxito

1. Guardar una observación con ancla, tocar uno de esos archivos, y que el recall la devuelva
   **marcada como posiblemente rancia nombrando el archivo** — sin intervención de ningún agente.
2. Que una observación sin ancla atraviese `save` y `recall` con salida byte-idéntica a la de hoy.
3. Que el caso real que motivó esto quede cubierto: reproducir en un test la nota
   `PENDIENTE: gateway y bridge` anclada a su archivo, y que se marque al cambiar el archivo.
4. Suite completa verde y `doctor` en ok, incluida la consistencia de la tabla nueva.
