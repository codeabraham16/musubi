---
artifact: spec
schema_version: "1.0"
change: potion-tokenizador-indice
status: draft
---

# Especificación — el embebedor completo tokeniza con el índice

«Los sidecars están al día» quiere decir lo mismo que hoy en `escribirSidecarsSiHaceFalta`:
`cargarSidecars` los acepta, y la identidad lleva el checksum de CONTENIDO recién calculado y las
huellas de tabla y tokenizer que se tomaron ANTES de leer.

## Requisitos

- **R1** — Cuando los sidecars están al día, `NewStaticProvider`:
  - DEBE tokenizar con el índice (`tokenizer.idx`);
  - NO DEBE deserializar el vocabulario de `tokenizer.json`. Los bytes los sigue leyendo, porque
    los necesita para el checksum.
- **R2** — Cuando los sidecars no estaban al día y `NewStaticProvider` los escribe, DEBE tokenizar
  con el índice que acaba de escribir.
- **R3** — Si al terminar no queda un índice utilizable, `NewStaticProvider` DEBE tokenizar con el
  mapa, como hoy, y NO DEBE armar el índice en memoria. Los casos son:
  - la carpeta no admite escritura, o el rename falla;
  - la tabla cambió durante la carga;
  - el tokenizer es WordPiece.
- **R4** — Para todo texto:
  - los ids del índice DEBEN ser iguales a los del mapa;
  - el vector de `StaticProvider` DEBE ser, bit a bit, el que daba con el mapa.
  - Se verifica sobre:
    - los 50 textos de `textosDePrueba`;
    - las 14 frases del charsmap;
    - la referencia de `testdata/spm_potion_ids.json`;
    - texto real largo.
- **R5** — El `model_id` NO DEBE cambiar: la misma derivación, con el mismo checksum de los bytes.
- **R6** — Se DEBEN conservar las garantías de hoy:
  - los temporales huérfanos se limpian en cada construcción, también cuando los sidecars están
    al día;
  - las huellas se toman antes de leer;
  - una tabla reescrita con igual tamaño y fecha se sana;
  - los sidecars al día no se reescriben;
  - sin escritura no se arma el índice;
  - el aviso de «sin atajo» sale una vez por proceso y por carpeta.
- **R7** — Si cambian los bytes del índice armado desde el `tokenizer.json` real de POTION, DEBE
  cambiar `formatoIndice`. Lo fija una guarda contra el asset real que corre en el job
  `recall-gate`.
- **R8** — Desempeño: se mide, no se aserta en CI, porque depende de la máquina.
  - Sobre texto real de miles de caracteres, la tokenización del completo DEBERÍA ser ≥ 30× más
    rápida.
  - Con el índice al día, la construcción no paga `loadTokenizerBytes`.
  - La memoria viva del proveedor NO DEBERÍA subir.

## Escenarios

### Escenario: arranque con el índice al día

- **Given** una tabla Unigram con `tokenizer.idx` e `identidad.json` escritos por un arranque
  anterior, y sin cambios desde entonces
- **When** se construye `NewStaticProvider`
- **Then**:
  - no se deserializa el vocabulario de `tokenizer.json`;
  - el tokenizer es el del índice;
  - el `model_id` es el de siempre;
  - el vector es bit a bit el del mapa.

### Escenario: primer arranque, sin sidecars

- **Given** una tabla Unigram sin `tokenizer.idx`
- **When** se construye `NewStaticProvider` en una carpeta con permiso de escritura
- **Then**:
  - se arma el mapa y se escriben los sidecars;
  - el proveedor tokeniza con el índice recién escrito;
  - el vector es el del mapa.

### Escenario: carpeta sin escritura

- **Given** una tabla Unigram sin sidecars, en una carpeta donde no se puede crear un temporal
- **When** se construye `NewStaticProvider` dos veces
- **Then**:
  - el índice no se arma nunca;
  - el proveedor tokeniza con el mapa;
  - el aviso sale una vez.

### Escenario: tabla reescrita con el mismo tamaño y la misma fecha

- **Given** sidecars escritos para una tabla que después se reescribió con otro contenido, el mismo
  tamaño y la misma fecha
- **When** se construye `NewStaticProvider`
- **Then**:
  - el checksum de contenido no coincide con la identidad, así que el índice no se da por al día;
  - se reescriben la identidad y el índice;
  - el `model_id` es el del contenido nuevo.

### Escenario: la tabla cambia mientras se carga

- **Given** una tabla que se reemplaza en disco entre la carga y la escritura de los sidecars
- **When** se construye `NewStaticProvider`
- **Then**:
  - no se acepta el índice viejo y no se escribe una identidad con la huella nueva;
  - el proveedor tokeniza con el mapa;
  - el arranque siguiente deja todo al día.

### Escenario: índice de otro formato

- **Given** un `tokenizer.idx` con otro `formatoIndice`
- **When** se construye `NewStaticProvider`
- **Then** el índice se trata como ausente: se reescribe, y el proveedor usa el reescrito.

### Escenario: la derivación del índice cambia sin subir el formato

- **Given** un cambio de código que altera los bytes del índice armado desde el tokenizer real
  (por ejemplo, el `unkPenalty`)
- **When** corre la guarda del job `recall-gate`
- **Then** falla y pide subir `formatoIndice`. Si no se sube, los índices viejos escritos en disco
  seguirían usándose con la derivación anterior.

## Fuera de alcance

- WordPiece.
- El hook por turno (`turn.go`).
- El formato del índice.
- mmap de la tabla.
- La consulta liviana: no cambia, sólo deja de ser el único consumidor del índice.
- granite.

## Preguntas abiertas

- [x] ¿Armar el índice en memoria cuando no se puede escribir? **No.** Cuesta 0,7-1,1 s por
  construcción, y `capture` construye el proveedor en cada fin de turno. Una carpeta sin escritura
  se queda con el mapa, como hoy (R3).
- [x] ¿Hay hoy algún consumidor del índice en producción? **No.** `NewProviderDeConsulta` no tiene
  llamadores; sólo lo nombra un comentario en `cmd/musubi/embed.go:210`. Este cambio hace del
  completo el PRIMER consumidor real del índice, y por eso R4 pide texto real largo y no sólo los
  50 textos.
