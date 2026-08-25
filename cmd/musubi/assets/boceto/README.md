# Boceto — la memoria como tracto neural

Exploración visual para el panel de Musubi. **No es producción**: vive fuera del bundle del panel,
`go:embed` no lo toca (`dashboard.go` embebe sólo `dashboard.html` y `dashboard.bundle.js`) y
`npm run build` no lo mira. Se construye con el **mismo motor** que el panel —three 0.169 y el
esbuild vendorizados en `../node_modules` y `../esbuild.exe`— y con **datos reales del cerebro
local**, porque un boceto con datos inventados no contesta la pregunta que se le hace.

## La idea, en una frase

**La rama no existe como objeto: existen los hilos, y la rama es lo que se ve cuando pasan muchos
juntos.** Es lo que es un tracto de verdad. De ahí sale el invariante que impide que el dibujo
mienta:

```
hilos(padre) = Σ hilos(hijos)
```

Un axón no aparece ni desaparece en una bifurcación. Así el grosor deja de ser una fórmula sobre el
dato y pasa a **ser** el dato: contás los hilos del núcleo y te da la suma de todas las hojas.

## Cómo se corre

**1 · Los datos.** El boceto lee `./grafo-local.json`, que **no está en el repo a propósito**: son
las notas reales de tu cerebro, con sus temas y sus gists, y este repo es público. Se genera en
local desde el panel:

```bash
musubi dashboard --port 7729 &                      # el panel, contra tu .musubi/memory.db
curl -s http://127.0.0.1:7729/api/graph > grafo-local.json
```

**2 · El bundle.** Tampoco se commitea (se rehace en ~80 ms y no lo embebe nadie):

```bash
cd cmd/musubi/assets
./esbuild.exe boceto/boceto-a.mjs --bundle --format=esm --target=es2022 \
  --outfile=boceto/boceto-a.js
```

`--target=es2022` y no menos: el boceto usa *top-level await* para cargar el grafo.

**3 · Servirlo.** Hace falta un servidor: los módulos ES no cargan por `file://`.

```bash
cd cmd/musubi/assets/boceto && python -m http.server 7731
# → http://127.0.0.1:7731/boceto-a.html
```

## Las seis formas

Las seis dibujan **el mismo dato con la misma maquinaria** —las mismas memorias, el mismo corte en
secciones, el mismo conteo de hilos, las mismas cuñas, los mismos invariantes—. Lo único que cambia
es **hacia dónde crece el tejido**, y eso es a propósito: si cada boceto tuviera su propio pipeline,
comparar dos sería comparar dos programas y no dos formas. Lo compartido vive en `forma.mjs`.

| | qué contesta | qué paga |
|---|---|---|
| **`a`** · *El núcleo* | ¿de quién es esto? Los actores salen de un centro en todas las direcciones (Fibonacci sobre la esfera): **no hay arriba** | oclusión: mirar una rama es tener otras trece detrás |
| **`b`** · *Las láminas* | ¿de qué cuelga esto? Nivel *N* ⇒ lámina *N*, siempre. Sholl y laminación cortical | lo orgánico: parece tejido en placa. Y **no afirma** que no tiene arriba |
| **`c`** · *El corte* | ¿cómo está repartida? Todo aplastado en una lámina, **nada tapado** de frente | el volumen, y las ramas que se esquivaban por profundidad ahora se cruzan |
| **`d`** · *La corona* | ¿qué se habla con qué? Las hojas **parejas** en un anillo y el medio vacío: lo que lo cruza son relaciones | la profundidad deja de ser distancia recorrida |
| **`e`** · *La corteza* | ¿dónde está lo que sé y dónde el camino? Un campo empuja hasta una cáscara: memorias afuera, tractos adentro | una superficie tiene menos lugar que un volumen: **108 de 220 bifurcaciones se aprietan** |
| **`f`** · *El nudo* | **la fusión de `a` y `d`.** El treemap esférico decide DÓNDE va cada hoja; el crecimiento del núcleo decide CÓMO llega | la esfera esconde su mitad de atrás: girar deja de ser opcional |

### Cómo se fusionan `a` y `d`, medido

| | borde (radio de las hojas) | reparto (variación del vecino) | isotropía (sesgo) | apretadas |
|---|---|---|---|---|
| `a` el núcleo | 483 **±94** | 60 % | **0,03** | 4 |
| `d` la corona | 268 **±0** | **5 %** | 0,31 | 0 |
| `f` el nudo | irregular **±63** | 55 % | **0,03** | 3 |

⚠ **El borde parejo se probó y se sacó.** Clavar las hojas a un radio da un borde perfecto — y una
**pelota**, que es exactamente lo que ya hacía `la corteza`. Lo que el treemap aporta es el reparto
parejo **en ángulo**; la distancia la sigue poniendo el crecimiento, así que la silueta queda
irregular como en el núcleo. Y encima el enredo baja: 0,209 → 0,063.

El nudo se queda con el borde de la corona y la isotropía del núcleo. Lo que **no** hereda entero es
el reparto perfecto: 55 % contra el 5 % de la corona — el trazo orgánico no puede aterrizar tan
parejo como un dendrograma, y `imán` es literalmente esa perilla (en 1 vuelve a ser un dendrograma).
El reparto de parcelas **sí** es parejo por su cuenta (13 % de variación); lo que lo afloja es el
camino, no el destino.

### 🔴 La rampa del imán, que era lo que apretaba

Las ramas salían, se doblaban y se **plegaban sobre sí mismas** hasta formar un puño. La causa era
que la fuerza del imán crecía con la profundidad — la idea era que arriba la rama tuviera lugar para
abrirse y abajo se acomodara, y el efecto es el contrario: dos hermanas nacen apuntando casi para el
mismo lado y recién se despegan varios niveles después. Es `bifurcar` otra vez con otro disfraz:
**lo que pasa al nacer manda.**

| | borde | ángulo P10 | mediana | enredo | apretadas |
|---|---|---|---|---|---|
| rampa (como estaba) | 7,1 | 0,128 | 0,462 | 0,794 | 9 |
| **apunta al nacer** | **1,7** | **0,657** | **1,796** | **0,209** | **3** |

Cinco métricas mejor a la vez, sin nada que pagar. Se mide el **percentil 10** del ángulo y no la
mediana: la mediana puede estar perfecta y el 10 % más apretado seguir pegado, que es justo lo que
se ve como amontonado.

Dos cosas más que el barrido de parámetros refutó, y quedan anotadas:
- **Más `curvatura` APRIETA**, no afloja: de 0 a 0,55 el ángulo P10 cae un 31 %. Y la sospecha de
  que la panza mete tramos en el hueco central es falsa — esa métrica queda plana.
- **`polarMin` sola no llega**: la frena `aperturaMax`. 1,30 · 1,45 · 1,60 devolvían **filas
  idénticas** y sólo subía el contador de apretadas. Se suben las dos juntas o no se mueve nada.

Se cambia de forma con la barra de arriba, sin volver a buscar la URL.

## Qué se puede hacer adentro

| | |
|---|---|
| clic | señala **un hilo, una neurona o una nota** — no el haz. Queda marcado con un anillo y **la cámara no se mueve**. |
| arrastrar | gira **alrededor de lo que agarraste**: al apoyar el dedo, lo que hay bajo el cursor pasa a ser el eje. Sin suavizado — el gesto es 1 a 1. |
| doble clic | vuela hasta lo que señalaste. |
| `↑ ↓ ← →` | padre · hija · hermanas. Moverse entre ramas no es puntería: es recorrer el árbol. |
| `A` | **ver esta sola** — apaga el resto y deja la rama legible hilo por hilo. |
| `0` · `Esc` | **ver todo** — vuelve al encuadre completo. |
| arrastrar · rueda | girar con inercia · acercar **hacia el cursor**. |

Anclas de URL, para capturar siempre el mismo encuadre entre dos versiones:
`#seccion=N` (aterriza en una sección), `#solo=N` (aterriza y la aísla), `#prueba` (corre los
invariantes en la página y los muestra medidos).

## Verificación

```bash
cd cmd/musubi/assets/boceto
node --test comun.test.mjs   # 30 invariantes puros
node sabotajes.mjs           # cada uno, VISTO FALLAR bajo un sabotaje dirigido
```

Y `#prueba` en la página, que es donde corren los 39 que necesitan una GPU — **en cada una de las
cinco**, porque una forma nueva puede romper un invariante viejo y ya pasó: «las láminas» reprobaba
«el núcleo no tiene arriba» hasta que se hizo lo correcto, que es que cada forma DECLARE lo que
afirma. Lo que no afirma se mide igual y se muestra como dato: declarar no aplicable no es esconder.

También hay `#perf`, que reparte el presupuesto de cuadro entre sondeo, selección y rótulos.
⚠ Bajo `--virtual-time-budget` `performance.now()` **no avanza** y todo daría 0,00 ms — que se lee
igual que «instantáneo». El panel comprueba el reloj y dice «no se pudo medir» en vez de mentir.

Los invariantes puros están en `node --test` y no sólo en la página por una razón medida: en
headless con `--virtual-time-budget` el navegador **no corre un solo cuadro**, así que cualquier
cosa que dependa del bucle de render devuelve el mismo valor cuando funciona y cuando está rota.
Un sabotaje ahí no se distingue de un arnés roto.

## Los archivos

| archivo | qué es |
|---|---|
| `comun.mjs` | matemática pura, sin escena: seccionar, contar hilos, colocar, enhebrar, la cámara, los rótulos |
| `escena.mjs` | los shaders, las instancias, el pase de identidad para señalar, la navegación y el HUD |
| `datos.mjs` | agrupa las memorias reales por actor y por tema |
| `forma.mjs` | lo que las cinco comparten: cargar, seccionar, contar hilos, montar, la leyenda y el conmutador |
| `boceto-a.mjs` … `boceto-e.mjs` | **sólo lo que distingue a cada forma**: su colocación, su encuadre y su apuesta |
| `comun.test.mjs` / `sabotajes.mjs` | los invariantes y el banco que los rompe a propósito |
