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

## Los dos bocetos

| | |
|---|---|
| **`boceto-a`** · *El núcleo* | El elegido. Los actores salen de un núcleo central en todas las direcciones, repartidos por Fibonacci sobre la esfera: **no hay arriba**, y por eso deja de leerse como un árbol. |
| **`boceto-b`** · *Las láminas* | La alternativa. La profundidad es la posición: todo lo que está al nivel *N* vive en la lámina *N*. Calcado del análisis de Sholl y de la laminación cortical. |

## Qué se puede hacer adentro

| | |
|---|---|
| clic | señala **un hilo, una neurona o una nota** — no el haz. Queda marcado con un anillo y **la cámara no se mueve**. |
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
node --test comun.test.mjs   # 26 invariantes puros
node sabotajes.mjs           # cada uno, VISTO FALLAR bajo un sabotaje dirigido
```

Y `#prueba` en la página, que es donde corren los 37 que necesitan una GPU.

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
| `boceto-a.mjs` / `boceto-b.mjs` | los parámetros de cada uno, y su leyenda |
| `comun.test.mjs` / `sabotajes.mjs` | los invariantes y el banco que los rompe a propósito |
