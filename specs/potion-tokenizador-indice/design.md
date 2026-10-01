---
artifact: design
schema_version: "1.0"
change: potion-tokenizador-indice
status: draft
---

# Diseño técnico — el embebedor completo tokeniza con el índice

## Decisiones de arquitectura

| # | Decisión | Alternativas consideradas | Por qué |
|---|----------|---------------------------|---------|
| D1 | Un solo criterio de «al día»: `indiceAlDia(dir, checksum, tabla, tok) *unigram`. Devuelve el tokenizer del índice si `cargarSidecars` lo acepta, si la identidad lleva el checksum y las huellas de esta carga, y si `leerIndiceTokenizer` lo puede leer. Si no, devuelve nil. Lo usan el camino rápido del completo y el chequeo de `escribirSidecarsSiHaceFalta`. | (a) Que `indiceAlDia` devuelva los bytes y que el completo los parsee aparte. (b) Dejar el chequeo escondido dentro de `escribirSidecarsSiHaceFalta`. | Con (a), un índice que pasa la identidad pero no se puede leer se da por «al día» y no se reescribe nunca: el completo pagaría el mapa en cada arranque. Es el mismo «atajo apagado para siempre» que `cabeceraVigente` vino a cerrar para UNA forma (el formato), y hay otras: crc interno, offsets, normalizer. Con «al día = se puede usar», toda forma de «inservible» se reescribe, y no hace falta enumerarlas. (b) no deja usar la respuesta. |
| D2 | `escribirSidecarsSiHaceFalta` devuelve el tokenizer del índice que deja vigente, o nil. Si está al día, devuelve ése: pudo escribirlo otro proceso entre la consulta del completo y este momento. Si lo escribe, devuelve el recién escrito, aunque después falle la identidad, porque esos bytes salen del mapa que se acaba de cargar. Si no lo escribe, devuelve nil. | Que el completo arme el índice en memoria cuando no se puede escribir. | Armarlo cuesta 0,7-1,1 s, y `capture` construye el proveedor en cada fin de turno. Sin escritura se queda el mapa, como hoy (R3). |
| D3 | `limpiarTemporalesHuerfanos` sale de `escribirSidecarsSiHaceFalta` y pasa a `NewStaticProvider`, antes del camino rápido. | Dejarla donde está. | Con el índice al día, `escribirSidecarsSiHaceFalta` ya no se llama, y la limpieza dejaría de correr en el caso más común. R6 la exige en cada construcción, y `TestLosTemporalesHuerfanosSeBorran` construye justo con los sidecars al día. |
| D4 | `cargarSidecars` deja de mirar `cabeceraVigente`, porque la mira `leerIndiceTokenizer`. El sabotaje de `TestUnIndiceDeOtroFormatoSeReescribe` pasa a la comparación del formato dentro de `cabeceraVigente`. | Dejar las dos comprobaciones. | Con D1, las dos comprobaciones están en serie y cada una tapa a la otra. El sabotaje de hoy (apagar la de `cargarSidecars`) daría VERDE, y el arnés diría, con razón, que la guarda está hueca. Los dos que preguntan siguen parseando: la consulta liviana sigue devolviendo `ErrSinAtajo` ante otro formato (envuelve el error de `leerIndiceTokenizer`). |
| D5 | Una costura nueva, `deserializarTokenizer = loadTokenizerBytes`, para contar cuántas veces se deserializa `tokenizer.json`. | Contar aperturas con `abrirParaLeer`. | `tokenizer.json` se sigue LEYENDO en el camino rápido, porque el checksum de contenido lo necesita (R1, R5). Lo que no tiene que pasar es la deserialización. |
| D6 | Las pruebas que comparan contra el completo toman como referencia una copia con el tokenizer del MAPA (`conElMapa`). `tablaDeJugueteConSidecars` devuelve esa copia. | Seguir comparando contra `NewStaticProvider` a secas. | Desde este cambio, el completo tokeniza con el índice: comparar la consulta liviana contra él sería comparar el índice consigo mismo. Lo medí: `parseNormalizer` de cero bytes devuelve `(nil, nil)`. Así, el sabotaje de `TestConsultaLivianaEsBitExactaConLaTablaReal` (`parseNormalizer(norm[:0])`) dejaría a los dos lados sin normalizer e IGUALES, y daría verde. |
| D7 | R7, con una guarda contra el asset real: el sha256 de `escribirIndiceTokenizer(real)` queda fijado en un mapa `formato → huella`. Como control previo, el sha256 de `tokenizer.json` se compara contra el de `KnownModels` en `pull.go`, que se deriva y no se copia. | Fijar la huella a mano sin control del asset. No fijar nada. | El completo pasa a confiar en un índice escrito por OTRO binario. Si la derivación cambia (`unkPenalty`, `maxRunes`, los ids) y `formatoIndice` no sube, los índices viejos en disco siguen valiendo con la derivación vieja. El control del asset separa «cambió el asset» de «cambió la derivación». La huella es un compromiso («formato 1 son estos bytes») y no un derivado copiado: es el mismo caso que la huella `7126b392b5d20d14` del charsmap. |

## Enfoque de implementación

`NewStaticProvider` (static.go):

```go
huellaTabla, errTabla := huellaDe(...)        // ANTES de leer, como hoy
huellaTok, errTok := huellaDe(...)
table, rows, dim, crcTabla, nTabla, err := cargarTablaEnStreaming(...)
tokRaw, err := os.ReadFile(tokenizer.json)    // se sigue leyendo: el checksum lo necesita
checksum := checksumDeCRC(crcTabla, nTabla, crc32.Checksum(tokRaw, castagnoli), int64(len(tokRaw)))
var tok tokenizer
if errTabla == nil && errTok == nil {
	limpiarTemporalesHuerfanos(dir, time.Now())
	if u := indiceAlDia(dir, checksum, huellaTabla, huellaTok); u != nil {
		tok = u // camino rápido: no se deserializa tokenizer.json
	}
}
if tok == nil {
	tok, err = deserializarTokenizer(tokRaw)
	...
	if u, ok := tok.(*unigram); ok && errTabla == nil && errTok == nil {   // ancla de L981, intacta
		if escrito := escribirSidecarsSiHaceFalta(dir, u, checksum, huellaTabla, huellaTok); escrito != nil {
			tok = escrito // el mapa queda para el recolector
		}
	}
}
```

- El checksum se calcula ANTES de deserializar: es la misma cuenta, sobre los mismos bytes.
- El orden de los errores no cambia: una tabla ilegible falla antes, y un `tokenizer.json` ilegible
  falla en la deserialización.
- Cuando la tabla es WordPiece, no hay sidecars y `indiceAlDia` devuelve nil: queda el camino de hoy.

`escribirSidecarsSiHaceFalta` (consulta_liviana.go) devuelve `*unigram`:

1. `if al := indiceAlDia(...); al != nil { return al }`: el chequeo, hoy en línea, pasa a la
   función. La subcadena del ancla de L917 vive ahí, una sola vez.
2. Vuelve a comparar las huellas contra el disco, como hoy. Si no coinciden, avisa y devuelve nil.
3. `escribirAtomico(índice)`, como hoy. Si falla, avisa y devuelve nil.
4. `escrito, _ := leerIndiceTokenizer(idx)`. Después escribe la identidad, como hoy, y en toda
   salida posterior devuelve `escrito`.

## Contratos / interfaces

- `func indiceAlDia(dir, checksum string, tabla, tok huellaArchivo) *unigram`: es nueva.
- `func escribirSidecarsSiHaceFalta(dir string, u *unigram, checksum string, tabla, tok huellaArchivo) *unigram`:
  hoy no devuelve nada.
- `var deserializarTokenizer = loadTokenizerBytes`: costura nueva, sólo para pruebas.
- `cargarSidecars`: misma firma. Deja de rechazar por cabecera; lo hace `leerIndiceTokenizer`.
- Sin cambios: el formato del índice (`formatoIndice = 1`), el de `identidad.json`, el `model_id`,
  la API pública y la consulta liviana.

## Trade-offs

- **Se gana:**
  - el arranque con el índice al día no paga `loadTokenizerBytes` (~1 s);
  - la tokenización del completo pasa del mapa (~80 ms por 1.000 caracteres) a la búsqueda binaria;
  - el mapa de 500.353 piezas deja de vivir en el proceso.
- **Se cede:**
  - el completo confía en un índice escrito por otro binario. Lo ata la guarda de D7 y lo
    respaldan la identidad (checksum de contenido + huellas) y el crc del índice;
  - una carpeta sin escritura se queda con el mapa lento, como hoy;
  - el primer arranque arma el mapa, escribe el índice y lo vuelve a leer: unos ms más que hoy,
    sobre un camino que ya cuesta ~2 s.
- **Riesgo de equivalencia:** desde ahora TODOS los vectores pasan por el índice. Se mitiga
  comparando ids de los dos caminos sobre la memoria real entera (2.907 notas, 8,3 M de
  caracteres; medición privada, no se commitea) y con pruebas de identidad en la suite y en
  `recall-gate`.

## Plan de pruebas

Primero las pruebas, cada guarda con su `// Sabotaje:` y su directiva del arnés. La forma del `de`
de cada directiva se fija al escribir la prueba, sobre el texto real.

| Prueba | Fija | Sabotaje |
|---|---|---|
| `TestElCompletoConElIndiceAlDiaNoDeserializaElJSON` (juguete) | Con los sidecars al día: 0 deserializaciones; el tokenizer es el del índice (`piezas != nil`); mismo `model_id`; vector bit a bit igual al del mapa en los 50 textos | `&& false` en el `if u := indiceAlDia(...)` del completo |
| `TestElCompletoUsaElIndiceQueAcabaDeEscribir` (juguete) | Primer arranque: tokenizer del índice recién escrito, vector igual al del mapa | `&& false` en el `escrito != nil` |
| `TestUnIndiceIlegibleConIdentidadVigenteSeReescribe` (juguete) | Un índice con identidad vigente que `leerIndiceTokenizer` rechaza se reescribe, y el completo termina con el índice nuevo. Para armarlo: `maxRunes = 0`, con el crc interno rehecho y la identidad apuntándolo | Volver al «al día» viejo: antes del `indiceAlDia` de `escribirSidecarsSiHaceFalta`, dar por al día lo que la identidad acepta sin leer el índice |
| `TestUnIndiceDeOtroFormatoSeReescribe` (existe) | Igual que hoy | Se mueve a la comparación del formato en `cabeceraVigente` (D4) |
| `TestConsultaLivianaEsBitExactaConLaTablaReal` (existe, `recall-gate`) | La consulta liviana contra el MAPA (D6), y además el completo (índice) contra el mapa, en los 50 textos | El de hoy (`parseNormalizer(norm[:0])`), que vuelve a dar rojo por D6 |
| `TestLaTokenizacionDelCharsmapEstaFijada` (existe, `recall-gate`) | La misma huella `7126b392b5d20d14` también por el camino del índice | El de hoy |
| `TestIndiceTokenizerBitExacto` (existe, `recall-gate`) | Suma texto real largo: ids del índice contra los del mapa sobre los `.go` del paquete (comentarios en español, con tildes, comillas y rayas), recortado a ~20.000 runas | El de hoy |
| `TestElIndiceRealEstaAtadoASuFormato` (nuevo, `recall-gate`) | R7: sha256 de `tokenizer.json` == `KnownModels` (control); sha256 del índice armado == `huellasDelIndiceReal[formatoIndice]` | `unkPenalty` 10.0 → 10.5 |

**`ci.yml`:** el paso «Embebedor de consulta bit-exacto contra el asset real» suma
`TestElIndiceRealEstaAtadoASuFormato` al `go test` y al bucle del `grep -- "--- PASS: $t "`. Lo
fija `TestCadaCompuertaDeCINombraUnTestQueExiste`.

**Ajustes a pruebas existentes:**
- `tablaDeJugueteConSidecars` devuelve la referencia del mapa (D6), y `TestUnIndiceAbiertoNoRompeNada`
  compara contra `conElMapa` y no contra el completo, que ahora tokeniza con el índice recién
  escrito;
- la limpieza de huérfanos se sigue probando por `NewStaticProvider` (D3).

**Arnés:**
- las anclas de L917 y L981 quedan con el mismo texto y una sola aparición;
- las de L559 y L722 no se tocan;
- se corre `musubi`/`internal/arnes` `Validar` + `Colisiones` sobre el árbol.

**Medición** (fuera de la suite, un proceso por medición, `nice`, el lock compartido para lo
cronometrado):
1. ids del mapa contra los del índice sobre las 2.907 notas reales, con tiempo por franja de largo;
2. los bytes de `tokenizer.idx` en disco contra los armados ahora. Si difieren, este cambio sube
   `formatoIndice`;
3. el heap vivo del mapa contra el del índice;
4. `NewStaticProvider` en frío, antes y después: tiempo, VmRSS y VmHWM.
