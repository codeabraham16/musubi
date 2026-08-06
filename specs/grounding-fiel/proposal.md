# Propuesta — Grounding fiel para `musubi_ask`

## El problema, en una frase

`musubi_ask` le manda al motor **el gist truncado** de cada memoria, sin el sello de procedencia.
El modelo sintetiza sobre un resumen cortado a mitad de frase y no puede distinguir una nota
verificada a mano de una inferencia de otro LLM.

## Cómo apareció

No salió de leer el código: salió de **auditar el cable**. Levantando un motor falso en loopback y
volcando el cuerpo exacto del pedido, lo que le llegó al motor fue esto:

```
[c1082c6b-…] (infra/credenciales · 2026-08-06 01:25:45)
Clave [[MSB:ai-provider-key:1]] — con eso el despliegue del…
```

Cortado en «del…». La observación entera decía a qué autentica y con qué header.

Y de paso quedó a la vista un segundo agujero que no estaba buscando: **el prompt no lleva el sello
de procedencia**. La línea sólo tiene id, topic y fecha. Una observación `llm:groq/llama-3.3`
corroborada —visible en el recall, marcada para el caller humano— entra al motor sintetizador
**indistinguible de una nota humana**. Q3 se cumple en la respuesta al caller y se incumple en el
prompt, que es donde importa para que el modelo no trate una inferencia como un hecho.

## Por qué esto va antes de encender un motor real

Hoy el pilar está apagado (`cognition.provider` vacío), así que nada de esto hace daño todavía. Pero
el próximo paso natural es enchufar un motor de verdad, y hacerlo primero significaría medir la
calidad de un modelo **contra una fuente mutilada** y concluir que el modelo no rinde. Se arregla la
fuente y después se enchufa el motor, no al revés.

## Qué NO es

- **No es cambiar qué memorias fundamentan la respuesta.** El recall sigue eligiendo exactamente lo
  mismo, con el mismo ranking model-free. Esto cambia la **profundidad**, no la **selección**.
- **No es mandar la base entera.** La hidratación tiene su propio techo de tokens; lo que no entra
  se queda en gist, como hoy.
- **No toca el camino model-free.** `musubi_recall` sigue devolviendo gists. El que cambia es el
  grounding de `ask`, que ya es opt-in.

## El costo que hay que decir de frente

Esto **agranda lo que cruza al motor externo**: antes salían gists truncados, ahora sale contenido
completo. El truncado estaba limitando la exposición por accidente, no por diseño.

Lo que hace aceptable el cambio es que el portero de privacidad (F1) ya está en el camino y quedó
**verificado en el cable** el 2026-08-05: con el secreto dentro del texto que viaja, al motor le
llegó `[[MSB:ai-provider-key:1]]` y el valor real se repuso en la respuesta al caller. Sin esa
muralla puesta primero, este cambio no se podría hacer. Es exactamente el orden que el roadmap pedía
—cerrar la jaula antes de subir la potencia— cobrando su primer dividendo.

## Costo y reversibilidad

Un helper de hidratación sin contabilizar acceso, una función de advertencia que pasa a exportada, y
el armado del prompt. Sin dependencias nuevas, sin esquema, sin red. Con el pilar apagado el
comportamiento es bit-idéntico, porque `musubi_ask` ni siquiera se puede invocar.
