# Propuesta — Gateway de privacidad para la cognición (F1)

## El problema, en una frase

Hoy, si alguien enciende el pilar de Cognición, **el texto que Musubi le manda al LLM sale tal
cual**. Ese texto es memoria del cerebro: puede contener tokens `msb_`, claves de API, contraseñas
en connection strings, IPs del tailnet. Y el plan de la flota gratis empeora el cuadro, porque
varios de esos proveedores **entrenan con lo que les mandás**.

## Por qué esto va primero

Es la fase F1 del plan [[roadmap/motor-cognicion-y-blindaje-cerebro]] y desbloquea todo lo demás:
sin un portero de privacidad no se puede usar ningún proveedor externo sin arriesgar una fuga. El
router (F2) y el caché (F3) mueven *cuánto* y *a quién* se llama; este cambio decide *qué sale*.

## Qué NO es

- **No es un segundo detector.** `internal/redact` ya detecta secretos y está auditado (reglas por
  forma + entropía de Shannon + catch-all de hex, con allowlist de placeholders). Acá se **reusa esa
  detección tal cual** y se le agrega lo único que le falta para este uso: poder **deshacerse**.
- **No toca el camino model-free.** Sin motor de cognición configurado, Musubi sigue siendo
  bit-idéntico. El gateway sólo existe donde ya hay una llamada a un LLM.
- **No es un juez de contenido.** No decide si algo es "sensible" por significado. Es determinista,
  sin red y sin modelo.

## La idea

Un **decorador** alrededor de `cognition.Provider`:

```
caller → [gateway] → Provider real → LLM
           │  scrub al salir
           └─ rehidratar al volver
```

El LLM ve `[[MSB:ai-provider-key:1]]` donde había una clave, razona igual, y la respuesta vuelve con
el valor real puesto de nuevo. El secreto **nunca cruza la red**.

Se instala dentro de `cognition.NewProvider`, que es el único constructor del pilar: así **no hay
forma de esquivarlo**, ni desde el código de hoy ni desde el que venga.

## Costo y reversibilidad

Un paquete nuevo (`internal/privacy`), un decorador de ~60 líneas y un campo de config. Sin
dependencias nuevas, sin red, sin estado en disco. Poner `cognition.gateway.mode: off` lo desactiva
por completo, y borrar el paquete devuelve el comportamiento previo sin tocar nada más.
