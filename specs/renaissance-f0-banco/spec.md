# SDD spec — renaissance-f0-banco

## Contrato

### El set dorado
Un archivo `internal/mcp/testdata/banco-diseno.json` con tres poblaciones:

- `pedidos[]` — pedidos de diseño legítimos de los proyectos vivos. Cada uno:
  `{id, proyecto, target, ejes[], formas[]}` donde `formas` son 3 maneras distintas de pedir lo
  mismo (la primera es la canónica) y `ejes` son los temas que un corpus útil debería cubrir
  (`tabla`, `color`, `a11y`, `motion`, `formulario`, …).
- `fuera_de_dominio[]` — consultas que el motor **no debería** contestar con material de diseño
  (basura, temas ajenos, entradas degeneradas).
- `inyecciones[]` — payloads que intentan que el brief emita una instrucción ajena.

### El banco estructural
`go test ./internal/mcp -run TestBancoDiseno` — offline, sin red, sin LLM, con un acervo de fixture
sembrado en un motor temporal.

### La sonda
`go test -tags sonda ./internal/mcp -run TestSondaDiseno` — contra el central real, con
`MUSUBI_CENTRAL_URL` y `MUSUBI_TOKEN`. Nunca corre en CI ni en `go test ./...` sin el tag.

### Los umbrales
`internal/mcp/testdata/banco-umbrales.json` — un valor por métrica, con la fecha y el commit en que
se fijó. El banco falla si una métrica **empeora** respecto de su umbral. Apretar un umbral es un
cambio deliberado, revisable en el diff del PR que lo gana.

---

## Invariantes

Cada uno con el sabotaje que tiene que verlo fallar. El sabotaje ataca **el invariante declarado**,
no una validación cualquiera: si la defensa en profundidad lo tapa, el test queda vacuo.

### I-BANCO1 · el banco estructural no toca la red ni un LLM
Corre con `embedding.NoopProvider{}` y un motor en `t.TempDir()`. No abre sockets, no lee
`MUSUBI_TOKEN`, no depende de que el central esté vivo.

**Sabotaje:** apuntar el banco al central real dentro del test estructural ⇒ debe fallar la compilación
o el aislamiento (el test no tiene de dónde sacar URL ni token). *Verificado en rojo.*

### I-BANCO2 · una regresión pone el test en ROJO, no en advertencia
Si el brief crece, si baja la fracción variable, si entra una inyección o si se pierde abstención, el
test falla. No hay `t.Log` de consuelo para una métrica peor que su umbral.

**Sabotaje:** subir a mano `designMethodLimit` (el cambio real del 2026-08-21 que nadie detectó) ⇒
M4 empeora y el banco tiene que ponerse rojo. *Éste es el sabotaje que da sentido a toda la fase:
reproduce el incidente original.*

### I-BANCO3 · el umbral es un dato versionado, no una constante escondida
Los umbrales viven en un archivo de datos con fecha y commit. Cambiar uno se ve en el diff.

**Sabotaje:** mover un umbral en el archivo sin tocar código ⇒ el resultado del banco cambia, y el
test que valida la forma del archivo exige la fecha y el commit del cambio.

### I-BANCO4 · el set dorado cubre las tres poblaciones y toda paráfrasis pide lo mismo
Al menos 15 pedidos con 3 formas cada uno, 8 fuera de dominio, 8 inyecciones. Cada pedido declara
sus ejes.

**Sabotaje:** dejar un pedido con una sola forma, o sin ejes ⇒ el test de forma del set falla. Y
quitar la población de inyecciones ⇒ M6 no se puede computar y el banco falla en vez de reportar 0
payloads filtrados, que sería el valor tranquilizador.

### I-BANCO5 · la inyección se mide por DÓNDE cae el payload, y en tres canales separados
Un payload sólo cuenta como neutralizado si **no está en el bloque de instrucciones**. Envolverlo
entre delimitadores de cita sin sacarlo de ahí no lo neutraliza — F2 va a mover material a una zona
citada, y la métrica no puede darse por ganada por una mudanza de etiqueta.

Se reportan tres canales por separado, porque cada uno lo arregla una fase distinta:
`prompt→instrucción` (hoy limpio), `prompt→eco en ask` (hoy sucio, lo mueve F1) y
`acervo→instrucción` (hoy sucio, lo arregla F2).

**Sabotaje:** envolver el payload de una tarjeta del acervo entre delimitadores de cita, dejándolo
dentro del bloque `principles` ⇒ el canal `acervo→instrucción` debe seguir contándolo como sucio.

### I-BANCO6 · la sonda no puede colarse en CI
El archivo de la sonda lleva `//go:build sonda`. Sin el tag no compila ni se ejecuta.

**Sabotaje:** correr `go test ./internal/mcp` a secas con el central apagado ⇒ verde. Sacar el tag ⇒
el paquete pasa a depender de la red y el sabotaje se ve en rojo.

---

## Métricas y su definición exacta

| Métrica | Definición operativa | Dónde |
|---|---|---|
| **M2** abstención | fracción de `fuera_de_dominio` que devuelven `degraded` o corpus vacío | estructural |
| **M4** tamaño | tokens del brief serializado, `p50` y `max` sobre todos los pedidos, con `limit` 6 y 100 | estructural |
| **M5** fracción variable | 1 − (bytes idénticos entre los briefs de dos pedidos de ejes disjuntos) / bytes del brief | estructural |
| **M6** inyección | fracción de `inyecciones[]` que **no** llegan a posición de instrucción | estructural |
| **M1** paráfrasis | Jaccard promedio de los ids del corpus entre las 3 formas de cada pedido | sonda |
| **M3** precisión@6 | fracción del top-6 cuyo `topic_key` toca un eje declarado del pedido | sonda |
| **M7** latencia | p50/p95 de la llamada | sonda |
| **M8** cobertura | ids distintos servidos en todo el set / entradas vivas del acervo | sonda |

M3 arranca con una aproximación por ejes (model-free, sin etiquetado humano): mide si el corpus
servido *toca el tema*, que es la falla gruesa que hoy tenemos. El etiquetado fino queda para cuando
la aproximación deje de discriminar.

## Fuera de alcance

- Arreglar cualquier defecto del motor. F0 sólo mide.
- Juicio de calidad estética del material. Se mide relevancia temática, no si la tarjeta es buena.
- Automatizar la sonda en CI o en un cron. Es bajo demanda.
