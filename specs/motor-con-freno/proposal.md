# Propuesta — El motor tiene freno propio

Track: **Potencia medida**, F3 (el paso 4 de F1, que tres specs seguidos dejaron anotado como hueco).

## Lo que se midió antes de proponer, y cómo corrigió el plan

Yo había dicho que esto era **lo más urgente**, porque «cualquier `writer` del central puede gastar
la suscripción». El ledger del central dice otra cosa:

| | medido (30 días) |
|---|---|
| llamadas a `musubi_ask` | **3** |
| quién las hizo | `davantis-admin` — las tres |
| `gio` | 49 llamadas en 9 tools, **ninguna al motor** |
| `musubi_recall` en el central | 14 |

**Nadie fuera del dueño usó el motor jamás.** La exposición es real como *capacidad* y nula como
*hecho*. Esto no es apagar un incendio; es poner el freno antes de acelerar.

## Lo que YA existe y no hay que construir

- **La medición.** El ledger (`tool_invocations`) guarda tool, resultado, duración, principal y
  proyecto, sobrevive a los reinicios, y ya contestó «quién gastó» con una consulta.
- **Un limitador por principal.** `quotaLimiter`: ventana deslizante, en memoria, model-free.

## El hueco exacto

**La cuota cuenta todas las tools por igual.** El default es **600 llamadas por minuto** — un número
calibrado para tools gratis. Aplicado a `musubi_ask` no es un límite: son 864.000 llamadas al modelo
por día antes de que alguien diga que no.

## Por qué ahora y no después

Hoy el juez está apagado. Cuando se encienda —y el número de `specs/juez-real/` lo vuelve
tentador: acierta el primer puesto el 80,6% de las veces contra 33,3%— **cada recall pasa a ser una
llamada al modelo**. Hoy en el central son 14 recalls/mes, así que tampoco explota; el problema
aparece el día que el cuerpo o la cabina del CRM empiecen a recuperar contra el central, y ese día no
hay margen para ir a poner un freno.

Es barato ahora y es un incendio después.

## Quién puede gastar, medido y no supuesto

Exactamente **dos** caminos llegan al motor:

| tool | dónde | cuándo |
|---|---|---|
| `musubi_ask` | `s.cognition.Ask` | siempre que se la llama |
| `musubi_recall` | `cognition.Rerank` dentro de `rerankIfEnabled` | sólo con `read_time_rerank` encendido **y** el caché fallando |

Los dos detalles importan y los dos salieron de leer el código, no de suponerlo: el costo de
`musubi_recall` es **condicional**, y un acierto de caché **no gasta**. Cobrarle presupuesto a
cualquiera de esos dos casos estrangularía algo gratis.

## La asimetría, que es el corazón del diseño

Las dos tools se quedan sin presupuesto de formas distintas **a propósito**:

- **`musubi_ask` se RECHAZA.** Quien la llamó pidió una respuesta razonada. Devolverle otra cosa sin
  decirlo sería mentirle.
- **`musubi_recall` DEGRADA**: devuelve el orden model-free y no falla. Quien llamó pidió memoria, no
  un juez.

No es una regla nueva: es exactamente la que ya rige cuando el juez falla o se cae el motor
(`rerankIfEnabled` es best-effort desde F3.5c). Un límite de gasto es, para el recall, otra forma de
que el juez no esté disponible.

## Lo que NO se construye

- **No hay capability nueva** («este principal puede gastar el motor»). Sería diseñar autorización
  para usuarios que no existen: sólo el dueño tocó el motor, nunca nadie más. Cuando eso cambie, el
  freno ya va a estar y la capability se agrega encima.
- **No se enciende el juez.** Este spec es la precondición, no la decisión.
- **No hay costo en dinero ni en tokens.** Se cuentan LLAMADAS, que es lo que el sistema puede saber
  sin depender de que un proveedor reporte bien.
