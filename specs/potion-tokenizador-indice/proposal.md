---
artifact: proposal
schema_version: "1.0"
change: potion-tokenizador-indice
status: draft
---

# Propuesta — el embebedor completo tokeniza con el índice

## Intención

`StaticProvider` (el embebedor de POTION que usan el daemon, `serve`, `capture`, `embed backfill`
y `doctor`) tokeniza por el camino del MAPA: en cada posición del texto prueba cada largo posible,
y cada prueba arma un string y lo busca en un mapa de 500.353 piezas. Cuesta ~80 ms por cada 1.000
caracteres. El embebedor de consulta liviana ya tokeniza con el índice ordenado (`tokenizer.idx`),
que da los mismos ids y es 67–120× más rápido de 250 a 16.000 caracteres. Las cifras salen de la
nota 0492a651: ahí dice 67–120× en todos los largos, pero en 60 caracteres medimos sólo 6×.

Además, para construir el mapa, `NewStaticProvider` deserializa las 500.353 piezas de
`tokenizer.json` en cada arranque: ~0,95-0,98 s, medido el 2026-09-11 en `arranque_real_test.go`.
El índice que reemplaza a ese mapa ya está escrito en disco, al lado de la tabla, y el completo lo
ignora.

## Alcance

- **Incluye:**
  - Si los sidecars (`tokenizer.idx` + `identidad.json`) describen exactamente la tabla recién
    cargada (mismo checksum de contenido y mismas huellas), `NewStaticProvider` toma el tokenizer
    del índice y no deserializa el vocabulario de `tokenizer.json`.
  - Si los sidecars no estaban al día y se acaban de escribir, el completo pasa a usar el índice
    recién escrito, si se puede leer, y el mapa queda para el recolector.
  - Pruebas de identidad, escritas primero:
    - los ids del índice contra los del mapa sobre texto real largo;
    - el vector del completo antes y después, bit a bit;
    - la huella del charsmap también por el camino del índice.
  - Una guarda que ate el contenido del índice a `formatoIndice`, porque desde ahora el completo
    también confía en un índice escrito por otro binario.
  - Medición de velocidad de tokenización, tiempo de arranque y memoria, antes y después, con un
    proceso por medición.
- **No incluye:**
  - el WordPiece (tablas inglesas, que este repo no usa);
  - el hook por turno (`turn.go`, que no se toca);
  - cambiar el formato del índice;
  - mapear la tabla con mmap;
  - granite (va aparte).
  - Tampoco se cambia qué tokens salen: el cambio es de velocidad y memoria, con los mismos ids.

## Enfoque

Mover la decisión «¿los sidecars están al día?», que hoy vive escondida dentro de
`escribirSidecarsSiHaceFalta`, a un lugar donde el completo pueda usar su respuesta:

1. Con el índice al día, el completo lo usa directamente.
2. Si no está al día, se arma el mapa como hoy, se escribe el índice y el completo usa el recién
   escrito, si se puede leer.
3. Si al final no queda índice que usar, queda el mapa, como hoy. Los casos están en R3 de la spec:
   carpeta sin escritura, tabla que cambió durante la carga, huellas que no se pudieron tomar,
   índice que no se pudo guardar (disco lleno) o que no se puede leer, y tokenizer WordPiece.

## Impacto

- **Archivos:**
  - `internal/embedding/static.go` (`NewStaticProvider`);
  - `internal/embedding/consulta_liviana.go` (`escribirSidecarsSiHaceFalta` devuelve el índice
    que deja vigente);
  - pruebas en `internal/embedding/`.
- **Quién llama** (`musubi_impact`): `factory.go` (`NewProvider`, `NewProviderDeConsulta`),
  `gateway.go` (`InspectGateway`), `cmd/musubi/embed.go` (`resolveEmbedder`) y
  `cmd/musubi/doctor.go`.
- **Compatibilidad:** sin cambio de formato ni de identidad (`model_id`). Los vectores ya
  guardados siguen siendo válidos porque los ids no cambian. Un `tokenizer.idx` viejo de formato 1
  sigue sirviendo.

## Riesgos y mitigaciones

| Riesgo | Mitigación |
|--------|------------|
| El índice y el mapa difieren en algún texto que las pruebas no cubren, y ahora los vectores pasan por el índice siempre que haya uno que usar | Comparar ids de los dos caminos sobre texto real largo y Unicode difícil (el corpus del charsmap y el fixture real), además de los 50 textos de hoy |
| Un índice escrito por un binario anterior, con otra derivación del vocab (unkScore, maxRunes, ids), y el mismo `formatoIndice`: el completo heredaría la derivación vieja | Guarda que fija la huella de los bytes del índice armado desde el asset real junto al `formatoIndice`: si el índice cambia sin subir el formato, rojo |
| Tabla reescrita durante la carga | Se conserva la regla actual: las huellas se toman ANTES de leer y el índice sólo se acepta si checksum de contenido + huellas coinciden con la identidad |
| Directorio sin escritura: no hay índice | Se queda con el mapa (el comportamiento de hoy); no se paga armar el índice en memoria en cada arranque |

## Rollback

Un único commit de código, sin migración ni cambio de formato en disco. Revertirlo vuelve al mapa,
y los sidecars quedan como están, porque los sigue usando la consulta liviana.

## Criterio de éxito

- Mismos ids y mismo vector, bit a bit, en todas las pruebas de identidad, con la tabla real.
- Tokenizar un texto real de varios miles de caracteres es ≥ 30× más rápido en el completo.
- `NewStaticProvider` con los sidecars al día ya no paga `loadTokenizerBytes`: debería bajar
  cerca de 1 s, aunque los absolutos de esta máquina no se trasladan a otra.
- La memoria viva del proveedor no sube: el índice pesa 14 MB contra el mapa, que se mide.
