---
artifact: spec
schema_version: "1.0"
change: potion-tokenizador-indice
status: draft
---

# Especificación — el embebedor completo tokeniza con el índice

«Los sidecars están al día» quiere decir que se pueden USAR (D1 del diseño): `cargarSidecars` los
acepta, la identidad lleva el checksum de CONTENIDO recién calculado y las huellas de tabla y
tokenizer que se tomaron ANTES de leer, y además `leerIndiceTokenizer` lee el índice. Antes de este
cambio el índice no se leía entero: `cargarSidecars` ya rechazaba uno cuyo tamaño o crc32c no
coincidiera con la identidad, y uno de otro formato, pero uno ENTERO (tamaño y crc32c al día) con
la cabecera vigente y un interior que no se puede leer se daba por al día y no se reescribía nunca.

## Requisitos

- **R1** — Cuando los sidecars están al día, `NewStaticProvider`:
  - DEBE tokenizar con el índice (`tokenizer.idx`);
  - NO DEBE deserializar el vocabulario de `tokenizer.json`. Los bytes los sigue leyendo, porque
    los necesita para el checksum.
- **R2** — Cuando los sidecars no estaban al día y `NewStaticProvider` los escribe, DEBE tokenizar
  con el índice que acaba de escribir, si se puede leer. Si no, rige R3.
- **R3** — Si al terminar no queda un índice que usar, `NewStaticProvider` DEBE tokenizar con el
  mapa, como hoy. Los casos, y si se llega a armar el índice:
  - la carpeta no admite escritura, la tabla cambió durante la carga o no se pudieron tomar las
    huellas de la tabla o del tokenizer: NO DEBE armar el índice, porque armarlo es lo caro;
  - el índice armado no se pudo guardar (disco lleno, rename rechazado): ya se armó, porque se
    arma después de crear el temporal y antes del rename, y se tira;
  - el índice recién escrito no se puede leer;
  - el tokenizer es WordPiece, que no tiene índice.

  Si lo único que falla es escribir la identidad, el índice ya quedó en disco y el proveedor
  tokeniza con él (D2): el que se queda sin atajo es el camino liviano.
- **R4** — Para todo texto:
  - los ids del índice DEBEN ser iguales a los del mapa;
  - el vector de `StaticProvider` DEBE ser, bit a bit, el que daba con el mapa.
  - Se verifica sobre:
    - los 50 textos de `textosDePrueba`;
    - las 14 frases del charsmap;
    - la referencia de `testdata/spm_potion_ids.json`;
    - texto real largo;
    - cada una de las 500.353 piezas del vocabulario de POTION, escrita como texto.
  - La cabecera del índice DEBE ser la del mapa, campo por campo: `maxRunes`, `unkID`, `unkScore`
    y `repl`.
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

### Escenario: con qué tokeniza en cada salida de la escritura

- **Given** una tabla Unigram sin sidecars, salvo en la segunda fila, que los tiene al día
- **When** se construye `NewStaticProvider` bajo cada condición, y después otra vez sin ella
- **Then**, la primera vez:

  | Condición | Tokeniza con | Índices que arma este proceso | Identidad en disco | Consulta liviana |
  |---|---|---|---|---|
  | primer arranque | el índice recién escrito | 1 | sí | arranca |
  | los sidecars ya estaban al día | el índice del disco | 0 | sí, la que estaba | arranca |
  | otro proceso los deja al día mientras éste arma el mapa | el índice del otro | 0 | sí, la del otro | arranca |
  | la carpeta no admite escritura | el mapa | 0 | no | sin atajo |
  | no se pudieron tomar las huellas | el mapa | 0 | no | sin atajo |
  | la tabla cambia durante la carga | el mapa | 0 | no | sin atajo |
  | el índice armado no se puede guardar | el mapa | 1 | no | sin atajo |
  | el índice escrito no se puede leer | el mapa | 1 | no | sin atajo |
  | falla sólo la identidad | el índice recién escrito | 1 | no | sin atajo |

  - en todas las filas, el vector es bit a bit el del mapa;
  - la identidad queda en disco sólo si nombra un índice que se puede usar;
  - la segunda vez, el proveedor tokeniza con el índice y la consulta liviana arranca;
  - lo fija `TestConQueTokenizaElCompleto`. WordPiece no tiene índice y no entra en la tabla.

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
