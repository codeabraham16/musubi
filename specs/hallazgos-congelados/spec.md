# Spec — Hallazgos congelados

## Contrato

```
musubi receipt freeze --id <debate>/<lente>    # el cuerpo va por STDIN
musubi receipt verify --id <debate>/<lente>    # el cuerpo va por STDIN
```

El cuerpo entra por **stdin** a propósito: por bandera quedaría en la línea de comandos, en el
historial del shell y en los logs, y además obligaría a escapar un texto multilínea — que es
justamente la forma del hallazgo típico.

La colección vive en `meta` bajo `rdd_findings`. Es una **colección** y no una clave única como
`rdd_receipt`: el recibo del árbol es «un candidato por vez», pero un panel emite N hallazgos.

## 🔴 Canonizar: la frontera exacta

| se normaliza | NO se normaliza |
|---|---|
| `\r\n` → `\n` | espacio interno |
| salto(s) final(es) | mayúsculas |
| | puntuación |

**El criterio es uno solo: se normaliza únicamente lo que NO PUEDE cambiar el significado.**

- **Sin** normalizar CRLF, en Windows cada verificación diría «mutó» y el mecanismo se apaga solo
  en una semana.
- Normalizando **de más**, «el índice puede estar vacío» y «el índice **no** puede estar vacío»
  darían la misma huella, y el congelado pasaría a **aprobar mutaciones reales**.

La segunda falla es peor, y por eso el banco sabotea **las dos mitades**.

El recorte del salto final es una extensión deliberada del «sólo CRLF» del plan: `echo` agrega uno,
`printf` no, y un archivo puede tener o no tener el suyo. Salió de un error real durante la
verificación de punta a punta.

## Los tres motivos de rechazo

Son distintos porque cada uno pide una **acción** distinta, y quien los lee suele ser un agente:

| motivo | pide |
|---|---|
| no hay hallazgo congelado con ese id | **congelar** |
| el cuerpo cambió | **re-emitir** o restaurar el texto |
| el árbol cambió | **re-verificar** contra el código de hoy |

Colapsarlos haría que el agente reintente la acción equivocada.

## Invariantes

| # | invariante |
|---|---|
| H0 | 🔴 `Compute` sigue dando los mismos bytes (línea base con hexes literales) |
| H1 | canonizar CRLF y el salto final, y **nada más** (las dos mitades) |
| H2 | el cuerpo no se guarda, sólo su huella |
| H3 | los tres motivos piden acciones distintas y son tres mensajes distintos |
| H4 | una postura reemplazada en silencio no pasa la verificación |
| H5 | la poda por árbol descarta lo de la vuelta anterior |
| H6 | la colección aguanta N y se serializa determinista |
| H7 | un registro corrupto degrada a colección vacía: falla **cerrado** |
| H8 | no se congela lo vacío |

## Sinergia con F4

La poda por árbol es **media respuesta** al alcance decreciente: tras un fix el árbol cambia, los
hallazgos viejos se caen solos y la vuelta siguiente sólo congela sobre el código de hoy. Nadie
tiene que acordarse de limpiar.

## Fuera de alcance, encontrado de paso

`receiptShow` hace `r.Fingerprint[:12]` sin validar el largo: una huella corta escrita a mano lo
hace **panicar**. No se arregló acá.
