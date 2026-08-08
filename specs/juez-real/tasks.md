# Tareas — El juez, contra el motor real

Estado: **completo**. Medido en vivo contra el cerebro central el 2026-08-08.

## EL NÚMERO

Cerebro central (`musubi-server`), fixture generado de su memoria viva: **1.216 docs · 36 consultas**.
Los dos brazos sobre el **mismo corpus, en la misma corrida**. Motor: `sonnet` vía litellm.

```
config                     MRR  R@1    nDCG@1   R@5    nDCG@5   R@10   nDCG@10
lexical                  0.477  0.082   0.333  0.361   0.364  0.513   0.424
lexical+juez             0.824  0.217   0.806  0.495   0.584  0.552   0.594
DELTA                   +0.347 +0.134  +0.472 +0.133  +0.220 +0.039  +0.170
```

36 llamadas al juez, 3m50s los dos brazos (~6,3 s por consulta).

## Qué dice, en una línea

**El juez pone algo relevante en el primer puesto en el 80,6% de las consultas, contra el 33,3% del
model-free.**

Ésa es la lectura de `nDCG@1` y no hay que interpretarla: con relevancia binaria, nDCG@1 **es**
precisión@1 (DCG@1 = 1 si el primer resultado es relevante, IDCG@1 = 1 siempre que exista al menos
un relevante). Es la cuenta directa de cuántas veces acertó el tope.

## Por qué el número es creíble: R@10 casi no se movió

`R@10` subió apenas **+0.039**, contra +0.472 de nDCG@1. **Tenía que ser así**, y es la mejor señal
de que el banco no está midiendo humo.

El juez reordena sólo la **cabeza** del ranking (top-12): no puede traer lo que el recall no
encontró. Su trabajo es el **orden**, no la **cobertura**. Si R@10 hubiera saltado igual que nDCG@1,
habría querido decir que el brazo está tocando algo que no debería.

## Qué NO prueba

- **Es una sola corrida.** Un juez LLM no es determinista. Tenemos un punto, no una distribución: no
  sabemos la varianza entre corridas ni si un mal día del modelo se come parte de la ganancia.
- **Los absolutos están subestimados**, por el etiquetado por `topic_key` de `fixture-real` (lo
  relevante que vive en otro topic cuenta como fallo). El sesgo es idéntico para los dos brazos, así
  que el **delta** aguanta; el 0.824 suelto, no.
- **No dice que convenga encenderlo**, y acá está la parte incómoda. ~6,3 s y una llamada de
  suscripción **por recall** es una decisión de latencia y de cuota, no sólo de calidad. El endpoint
  `cognicion-endpoint` del central **se dimensionó explícitamente para uso ocasional**, no para una
  llamada al modelo en cada recuperación. Este número dice que el juez ordena mejor; **de dónde sale
  la capacidad para pagarlo en cada recall sigue sin responderse**, y hoy no hay ni medición de gasto
  ni autorización por principal.

## Lo que costó llegar, que también es el hallazgo

- [x] **No hay Go ni clon del repo en el central**, y no se instaló ninguno de los dos: el binario de
      test se **compiló acá** (`GOOS=linux CGO_ENABLED=0 go test -c`, estático, 17 MB), se copió a
      `/tmp`, corrió, y se borró. El server quedó como estaba.

- [x] **El camino se validó con un motor falso antes de gastar.** Un juez que INVIERTE el orden hundió
      el MRR de 0.532 a 0.172 sobre la memoria local. Sin esa corrida, un delta de 0 en la medición
      real habría sido ambiguo: ¿el juez no aporta, o el brazo no está enchufado?

- [x] **★ Los contadores de `musubi_cognition_stats` son en-proceso y no dicen lo que parecen.**
      Antes de medir daba `enabled: true` con **todos** los contadores en cero, y de ahí saqué que la
      cognición del central «nunca se había usado». **Es falso**: hubo una llamada real verificada
      punta a punta el 2026-08-06. Los ceros son del **deploy del 2026-08-08**, que reinició el
      proceso y con él los contadores.

      El instrumento no distingue «nunca se usó» de «no se usó desde el último reinicio», y no lo
      advierte. Para saber si un motor se ejercitó alguna vez, los contadores en memoria no sirven.

- [x] **★ GOTCHA: `systemctl cat` se saltea los drop-ins que no puede leer, sin decirlo.** La clave
      del motor no está donde parecía. La cadena real es:

      ```
      musubi-brain.service  →  EnvironmentFile=/etc/musubi/musubi.env        (sin la clave)
                            →  .d/cognition.conf            (0600 root, 58 bytes)
                                 └─ EnvironmentFile=/etc/musubi-brain-cognition.env   ← acá vive
      ```

      Corriendo como `musubi`, `systemctl cat musubi-brain` mostró `body-update.conf` e `ingest.conf`
      y **omitió `cognition.conf` en silencio** por ser 0600. Nada en la salida dice que faltó un
      archivo. De ahí salieron dos corridas fallidas antes de encontrarlo.

## Lo que enseñó

**«Configurado» y «probado» son estados distintos, y sólo uno de los dos se puede afirmar.** La
cognición del central figuraba encendida desde hacía días y nadie la había ejercitado nunca. Un flag
en `true` es una intención; el contador en cero es el hecho.

**Un comando que falla mudo es peor que uno que falla.** La primera corrida murió con `401` porque la
variable llegó vacía: la versión de una línea había perdido el `set -eu` del script original. Costó
un viaje entero. Las versiones siguientes verifican que la clave no esté vacía **antes** de arrancar,
y si falla imprimen los **nombres** de las variables que sí encontraron — nunca los valores.
