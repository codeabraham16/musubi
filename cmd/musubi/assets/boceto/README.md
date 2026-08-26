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

Y hay un **segundo cerebro**: el central. Se elige con `?cerebro=central` —hay un conmutador al
lado del de formas— y sale del dashboard que corre en el server, que escucha **sólo en loopback**:

```bash
ssh -o ProxyCommand="tailscale nc %h %p" musubi@100.79.126.62   'curl -s "http://127.0.0.1:7719/api/graph?lens=memory"' > grafo-central.json
```

Tampoco entra al repo, y por más motivo: son las notas de **todas** las máquinas de la malla.

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

## Los dos cerebros

El boceto se construyó entero contra el **local** y el panel va a mirar el **central**. Son dos
formas distintas del mismo dato, así que el dibujo tiene que aguantar las dos. Medido el
2026-08-25, mismo pipeline y misma configuración de «el nudo»:

| | notas | sinapsis | por nota | racimos | haces | ángulo P10 | enredo |
|---|---|---|---|---|---|---|---|
| local | 2.267 | 584 | 0,26 | 3 | 441 | 0,735 | 0,063 |
| central | 3.902 | 3.476 | **0,89** | 3 | 603 | **0,741** | **0,315** |

Dos cosas que el número dice y la intuición no. **La geometría aguanta**: el percentil 10 del
ángulo entre hermanas —la cola, que es lo que el ojo lee como amontonado— queda igual con 1,4×
más haces. Lo que se multiplica por cinco es el **enredo**, que es cruce entre haces ajenos, y lo
empuja la densidad de relaciones: 3,4× más por nota. *(El plan estimaba 6×; medido son 3,4×.)*

### 🔴 La tinta de las relaciones era una constante y tenía que ser un presupuesto

Contra el central el centro se **lavaba a blanco**. Medido acotando al lienzo y contando píxeles
brillantes que perdieron su color:

| | brillantes | lavados a blanco |
|---|---|---|
| local | 10.416 | 1.801 · 17,3 % |
| central | 37.827 | **11.254 · 29,8 %** |
| central **sin sinapsis** | 9.852 | 1.819 · 18,5 % |
| central **con presupuesto** | 11.612 | 1.819 · **15,7 %** |

La tercera fila es la que decide: apagando las relaciones, el central queda **tan limpio como el
local**. O sea el tejido aguanta 3.902 notas sin despeinarse y el **84 % del lavado lo ponía la
capa de relaciones** — una alfa afinada a ojo contra 584 trazos, aplicada a 3.476.

Así que la forma declara la tinta que quiere **para el cerebro de referencia** y `escalaTinta` la
reparte entre las relaciones que haya: más relaciones, cada una más tenue, misma luz total. La
cuarta fila es el resultado, y la leyenda **declara la atenuación** (`al 17 % de tinta`) porque
mostrar menos relación de la que hay sin decirlo sería la mentira de siempre.

⚠ El presupuesto tiene **piso**, y el piso y el presupuesto se pelean: pasadas
`referencia / piso` = **9.733** relaciones el piso gana y la luz total vuelve a crecer. Es una
decisión —una capa que se apaga sola miente igual que una que satura—, hoy el central está en
3.476 y un test vigila que el cruce siga lejos del dato real.

### 🔴 El medio era una colisión, y la causa estaba en el nacimiento

El centro del dibujo se veía como **tres cintas atravesándose**. No era el color ni la densidad:
los dos haces de primer nivel más gordos **compartían el 26 % de su volumen**.

`nucleo` es el **largo** del núcleo, y las hijas nacían a `nucleo * 0.55` = 22. Pero la **panza** del
núcleo sale de los hilos que lleva —igual que la de cualquier haz— y con 818 hilos mide **38,7**.
O sea que nacían **dentro del cuerpo**, y ahí no tienen dónde separarse: por conservación de hilos
sus discos **tapizan exactamente** el disco del padre, así que a una misma altura no pueden no
tocarse. Nacen en la superficie de la **cápsula** —tapa o panza, la que la dirección cruce primero— y
el solape es **0 en los dos cerebros**, sin alargar nada.

| | solape entre actores | enredo | apretadas |
|---|---|---|---|
| central, antes | **26,2 %** | 0,312 | 5 |
| central, ahora | **0** | 0,265 | 5 |
| local, antes | 0,2 % | 0,063 | 3 |
| local, ahora | **0** | 0,113 | 3 |

⛔ **Se probó compensar el largo** —descontarle al actor lo que se corrió su nacimiento, para que la
punta quede donde estaba— y sale peor: la rama primaria necesita su largo para parir a las suyas.
Apretadas 5 → 11 y enredo 0,259 → 0,332. Que el actor llegue un poco más lejos es más barato.

### Y un haz más ancho que largo muestra su cara cortada

Con largo 40 y panza 77 el núcleo era un **disco**, y de un disco de hilos no se ve el recorrido sino
las puntas: un cepillo. Encima cada hilo recibía `round(largo / largoNeurona)` = **dos** eslabones.
El piso es su propio diámetro, y con el nacimiento ya arreglado sale **gratis**: barriendo 40..120 en
los dos cerebros el enredo del local queda clavado en 0,113 y el del central se mueve 0,006.

### El núcleo es un CUERPO, y un cuerpo es redondo

Con el largo al piso el núcleo dejaba de ser un disco — y pasaba a ser un **ladrillo**: los hilos
corrían todos de punta a punta, así que la silueta era un cilindro. Ahora cada hilo viaja **la cuerda
que le toca dentro del elipsoide**: el del eje entero, el del borde apenas un tramo. No se agrega ni
se saca un hilo, cambia por dónde viaja cada uno.

Y los eslabones salen de lo que mide **ese** hilo, no la sección: con el número de la sección, un
hilo del borde —que viaja una décima parte— quedaría partido en astillas de dos unidades, y una
astilla no se lee como fibra sino como polvo.

Medido sobre el central: semiejes **38,7 y 38,7** —una esfera—, ningún hilo más allá de **1,17** del
radio *(la ondulación de `puntoHilo` mueve cada fibra un 17 % dentro del haz: la superficie es
peluda a propósito)*, y llega tanto al polo como al ecuador. Un cilindro daría **1,54** en las
esquinas, que es exactamente lo que se veía.

El nacimiento de las hijas pasa a salir del mismo elipsoide en vez de una cápsula: el mínimo entre
tapa y panza dejaba una arista donde el cuerpo ya no la tiene.

### ⚠ Y una sola muestra no es una medición

Las tres filas de puntería del `#prueba` salían de **una** sección —la última hoja gorda de la lista—.
Mover el nacimiento siete unidades la mandó detrás de una rama vecina y las tres se fueron de
`6/8 · 8/8 · 8/8` a `0/8`, **sin que el picking hubiera cambiado**. Con seis secciones repartidas a lo
largo del árbol: antes `36/55 · 50/55`, después `35/55 · 47/55`. Lo que medía era la suerte de una.

## La séptima forma: el colonizado — la rama CRECE

Las seis primeras **colocan** el árbol semántico de arriba hacia abajo; la séptima lo **crece**
(space colonization, Runions 2007): cada **hilo** de memoria —uno cada 6, la constante de todas
las formas— es un **atractor** puesto en su parcela del treemap, y el tejido crece de abajo hacia
arriba, bifurcándose donde el dato lo bifurca. Las ramas compiten por los atractores y **se
esquivan solas**: la anti-colisión que las otras formas parchean a mano acá es una propiedad del
proceso. El resultado se emite en el **mismo contrato de secciones**, así que todo lo demás
(conservación, hilos, picking, sinapsis, impulso) no se entera del cambio de paradigma.

Tres hallazgos de la calibración F0, medidos contra los dos cerebros:

- **El atractor es el hilo, no la memoria.** Un atractor por memoria dio 906 hojas de UNA memoria
  — un alambre por nota, enredo 1,06. Con el hilo (grupos de 6): mediana 6 memorias/hoja y los
  números vuelven a la clase de las otras formas.
- **`di` cubre el espaciado o el árbol levanta los atractores EN SERIE.** Con 46: 58 % forzados
  contra `maxIter`. Con 80: 1 %.
- **`inercia` y `piso` no llegan al enredo** — barrido con filas idénticas (0,95 ± 0,05). El
  enredo del colonizado es estructural: la topología emergente no respeta la adyacencia temática
  en vuelo. Queda 3× arriba del nudo y a juicio del ojo; si se lee como maraña, el paso siguiente
  es colonización jerárquica, no otra perilla.

Verificación: `node medir-g.mjs grafo-local.json [di=.. dk=..]` mide densidad, erizo, solape,
enredo y el CDF de edad sin abrir un navegador. Invariantes G3–G7 en el banco: toda memoria llega
(consumida o forzada, y lo forzado se declara en la leyenda), ninguna nace antes que su tronco, el
bosque es una función del dato, ninguna hoja sin carga, y la cuadrática no miente el camino
crecido.

### Carne + luz (el look híbrido del colonizado)

Los tractos con `fibras >= 3` son CARNE (el material opaco de siempre — la estructura se lee como
materia); por debajo del corte todo el subárbol es LUZ: fragmento sin difuso, blending aditivo, el
fresnel emitiendo por el borde como un neón. El corte va por `fibras` y no por nivel porque la
conservación lo hace monótono raíz→hoja: la frontera es limpia. El penacho paga del mismo
presupuesto.

**La luz terminal es presupuesto** (`TINTA_TERMINAL`, la lección de las sinapsis cobrada antes de
que muerda): la moneda es área emitida (Σ largo·radio), referencia = 12.125 u medida en el local;
el central emite 19.825 → la capa se atenúa al 61 % sola. Medido con el arnés de píxeles contra el
look sólido: cobertura 18,8 → 26 % y lavado a blanco 15,6 → **10,4 %** — más luz repartida, menos
quemado.

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
