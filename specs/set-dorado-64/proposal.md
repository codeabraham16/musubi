# El set dorado pasa de 16 a 67 pedidos

## Por qué, medido

Atacando el plan de cierre aparecieron cinco mejoras candidatas al motor. Ninguna se pudo
**decidir**, y no por falta de ideas: por falta de instrumento.

Bootstrap pareado sobre los 16 pedidos originales, midiendo el híbrido léxico+vectorial para el
paso consulta→eje:

```
DIFERENCIA  : +0,104
IC 95%      : [-0,063 , +0,312]   ← semi-amplitud 0,188
```

**Con 16 pedidos no se distingue 0,50 de 0,60.** Y todas las decisiones que quedan del track
—incluida la de cambiar de embebedor— dependen de diferencias de ese tamaño.

| pedidos | semi-amplitud del IC | ¿ve un efecto de +0,10? |
|---|---|---|
| 16 | 0,188 | no |
| 48 | 0,108 | no |
| **64** | **0,094** | **sí** |

## De dónde salen los 51 nuevos

De pantallas que **existen** en los repos, no de la imaginación:

- **16 páginas del CRM** (`crm-musubi/src/app/*/page.tsx`)
- **11 vistas del cuerpo** (`musubi-body/gpui-spike/src/*.rs`)
- **6 lentes del panel** del cerebro (`cmd/musubi/assets/src/*.mjs`)
- **6 superficies de Altura** que el set original ya cubría, con pedidos nuevos
- **12 tareas de diseño genéricas** frecuentes

Los **ejes** de las pantallas del CRM se cruzaron contra marcadores **medidos en su código**
(`<table>`, `<form>`, `Recharts`, `isLoading`, `Skeleton`, …), no contra lo que yo supusiera que
esas pantallas hacen. Donde el código no alcanza —el cuerpo está en Rust y el panel dibuja en
canvas— el eje sale de lo que la pantalla evidentemente es.

## ⚠️ El sesgo que este archivo no puede sacarse solo

**Las tres paráfrasis de cada pedido las escribió el mismo agente que mide el motor.** Si las tres
formas se parecen más entre sí de lo que se parecería el pedido de otra persona, **M1 sale mejor de
lo que es**.

Se mitigó variando el registro (técnico / coloquial), la palabra de entrada (sustantivo / verbo) y
el vocabulario. Pero queda declarado adentro del propio `banco-diseno.json`, para que nadie lea un
número de M1 sin saber de dónde salen las paráfrasis.

**La corrección barata:** que el usuario reescriba una forma de cada pedido con sus palabras.

## Lo que ya reveló

Con el set ampliado, el techo de M1 que implica el acuerdo del eje cae de **0,50 a 0,338**. O sea:
**los 16 pedidos originales eran más fáciles que la realidad**, y parte de la mejora que
celebramos era del set, no del motor. Ese es exactamente el tipo de cosa que un instrumento chico
no puede decir.

## Fuera de alcance

- Cambiar el motor. Este PR sólo cambia con qué se lo mide.
- Decidir el híbrido léxico+vectorial. Con 67 pedidos da +0,060 con IC [-0,005, +0,124] y
  P(dif≤0)=4,2 %: **al borde**, ya no claramente ruido, pero todavía no establecido.
