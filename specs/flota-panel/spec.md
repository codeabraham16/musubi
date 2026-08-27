# Spec — S9 · El panel de flota

Ver la flota, no leerla en JSON.

---

## Tres decisiones de forma, antes de una línea de código

### 1 · NO va dentro del bundle WebGL

El panel del cerebro es una visualización 3D de neuronas de memoria con three.js. Una flota es
**una tabla de máquinas con números**. Son dos problemas de UI distintos, y meter el segundo en el
motor del primero sería forzarlo.

Además, ese bundle se **commitea y la CI compara sus bytes** (job `panel`). Tocarlo para agregar
una tabla obligaría a reconstruirlo con las versiones exactas de esbuild y three — un riesgo
gratuito para algo que no necesita ni una línea de three.js.

**Página aparte, HTML+CSS+JS plano, sin paso de build.**

### 2 · Lo sirve el panel LOCAL y proxea al cerebro

Es el patrón que ya existe para el censo de actores y para el riel, y sus tres razones valen
igual acá:

1. **el bearer** — un `fetch` desde la página necesitaría el token EN la página: en el DOM, en el
   historial y en cualquier extensión instalada;
2. **CORS** — el cerebro no publica cabeceras para un origen `127.0.0.1:7777`;
3. **el panel ya tiene la credencial y la conexión** — duplicarla es una segunda copia del secreto
   para no ganar nada.

### 3 · El ESTADO viaja siempre, y es lo primero que se mira

Copiado del censo de actores, porque el problema es idéntico. **Una flota vacía puede significar
cinco cosas distintas**, y las cinco se dibujan igual si lo único que viaja es la lista:

| Estado | Qué pasó | Qué hacer |
|---|---|---|
| `apagado` | no hay enlace al cerebro central | configurar `MUSUBI_BRAIN_URL` |
| `caido` | el cerebro no respondió | mirar el servicio |
| `sin_permiso` | la credencial no tiene concesiones de flota | editar `principals.yaml` |
| `vacio` | no hay máquinas enroladas | `musubi_fleet_enroll` |
| `vivo` | hay flota | — |

Sin esto, alguien con el `fleet:` mal configurado ve una tabla vacía y cree que no tiene máquinas.

---

## H1 · La tabla dice la verdad

### I1 — Lo desconocido se dibuja como desconocido, nunca como cero

El invariante del track llega hasta el píxel. Un `cpu_pct` en `null` se muestra **`—`**, no
`0 %`. Un agente recién arrancado, un Windows sin sensor térmico, un macOS sin CPU: todos
dibujan un guion.

Pintar un 0 % sería peor acá que en el JSON: un gráfico no se lee, se mira de reojo.

### I2 — `admite` y `puedo` se muestran POR SEPARADO

Lo que la máquina admite es una propiedad de la máquina; lo que vos podés es de tu credencial.
Mostrar sólo lo primero enseña a ignorar el campo.

### I3 — La antigüedad de la muestra se ve

Una máquina que late pero dejó de medir muestra su última muestra buena para siempre y **parece
sana**. La edad del dato al lado del dato es lo que delata ese caso.

---

## H2 · El panel no agrega poder

### I4 — Es de SOLO LECTURA

No hay botón de ejecutar, ni de abrir pantalla, ni de revocar. Un panel que ejecuta es un panel
que ejecuta **con un click de más**, y el plano de terminal ya tiene su tool, su compuerta y su
bitácora.

### I5 — No inventa permisos

Muestra exactamente lo que la compuerta deja ver, porque **pregunta por las mismas tools**. No hay
una segunda ruta de datos que pueda desincronizarse de la primera.

---

## H3 · Lo que este slice NO hace

- **No toca el bundle WebGL** ni su job de CI.
- **No hay acciones.** Ver arriba.
- **No hay gráficos históricos.** Eso es Prometheus/Grafana, que ya está desplegado y para eso
  existe el export de S4b.
- **Cero dependencias nuevas**, ni de Go ni de JS.
