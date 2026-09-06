# Diseño — Profundidad derivada del diff

## La separación que ordena todo: pura arriba, sucia abajo

```
internal/codeintel/profundidad.go        PURA — aritmética sobre enteros, sin I/O
  Senales{...}                           las siete entradas + el flag de radio ciego
  Profundidad(Senales) -> Veredicto      escalones -> puntos -> nivel -> panel
  SenalesDelDiff([]FileDiff, int)        las cinco que salen del diff

internal/mcp/methods_detect.go           SUCIA — habla con el grafo
  (*McpServer).profundidadDe(...)        completa las dos del radio y el flag
```

**Por qué la función es pura.** Las señales entran como números; no se van a buscar. Eso permite
tres cosas que importan:

1. **El gate del turno (F1) puede llamarla** sin base de datos ni MCP.
2. **El banco mide la política, no el árbol.** Un test que fuera a buscar el radio real mediría un
   estado que cambia mientras los tests corren; su resultado dependería de si alguien guardó un
   archivo hace diez segundos. Eso no es una medición.
3. **El sabotaje puede apuntar a un solo lugar.** Con I/O adentro, romper un escalón y romper una
   consulta se ven igual desde afuera.

## El orden de las consultas al grafo, y su costo

`profundidadDe` hace, en este orden:

1. `GraphFileFingerprintsCtx` — **una** llamada. Da el conjunto de archivos que el grafo vio.
2. Por archivo tocado: `IndexableForGraph` (puro) y presencia en ese conjunto. Sin consultas.
3. Por símbolo tocado, hasta **25**: `GetGraphNodeCtx` y, sólo si el nodo existe,
   `GraphImpactCtx` con profundidad 3 y tope de 200 nodos.

El tope de 25 existe porque el BFS es lo más caro que hace `detect_changes`: sin él, un cambio que
toca cien símbolos dispara cien recorridos. **Y cuando recorta, lo declara** en `motivos` — el radio
queda sub-contado y quien decide el panel merece saberlo.

## Por qué `GetGraphNodeCtx` antes de `GraphImpactCtx`

Es la línea que separa este diseño de uno que miente. `GraphImpactCtx` sobre una clave inexistente
devuelve **cero callers**, exactamente lo mismo que sobre un símbolo al que no llama nadie.
Preguntar primero si el nodo existe es lo único que distingue «no arrastra a nadie» de «pregunté
mal o no está indexado».

Las claves se construyen con `codeintel.SymbolKey(path, sym.Kind, sym.Ref())`: el grafo indexa por
`path#kind:nombre` y `Ref()` ya da la forma calificada (`Recv.Método`), que es la misma que usa el
indexador. Si algún día divergen, el efecto es `radio_ciego` — no un cero falso.

## Alternativas descartadas

| alternativa | por qué no |
|---|---|
| una curva continua en vez de escalones | más "fina" y peor: oscila con una línea de ruido y nadie puede recomputarla de cabeza |
| pedirle el nivel a un modelo | rompe model-free (regla 3), y el juicio ya se delega en el panel |
| exigir las dos condiciones (archivos **y** líneas) | dejaría pasar los dos casos que más se cuelan: muchos archivos chicos, y un archivo con un cambio enorme |
| contar `líneas` sólo del lado nuevo | un borrado puro mediría cero — el cambio más destructivo posible puntuando como el más inocente |
| que el piso de honestidad suba a `profunda` | castigaría un índice flojo como si fuera un cambio grande; `estandar` es la respuesta honesta a «no sé» |
