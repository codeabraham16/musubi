# Diseño — El arsenal se ve

## Dónde va

- **Cerebro**: `internal/mcp/registry.go` (registro) + `internal/mcp/methods.go` (handler), al lado
  de las otras tools de skills.
- **Cuerpo**: `cmd/musubi-body/bridge.go` (el error tragado).

## El handler

```go
func (s *McpServer) toolListSkills(raw json.RawMessage) (interface{}, *RpcError)
```

Envuelve `s.resolver.LoadSkills()`, que ya existe y ya hace el trabajo pesado: leer
`.musubi/skills/`, parsear YAML, saltear lo inválido con un warning, y devolver slice vacío —no
error— si el directorio no existe.

**No se duplica esa lógica.** Reimplementar la lectura acá crearía dos caminos que pueden divergir:
el día que `LoadSkills` cambie qué considera una skill válida, la tool seguiría con el criterio
viejo.

## El DTO, y por qué no se serializa `skills.Skill` directo

`skills.Skill` tiene **sólo tags YAML**. `json.Marshal` cae entonces a los nombres de campo de Go:

```json
{"Name":"...","Description":"...","Triggers":[...]}
```

El cuerpo desserializa contra tags en minúscula (`name`, `description`, …). El resultado no sería un
error sino algo peor: **un array del largo correcto con todos los campos vacíos**. El panel pasaría
de «vacío» a «N filas en blanco», y eso se lee como un dato, no como una falla.

Por eso hay un tipo propio en el paquete `mcp`:

```go
type skillListada struct {
    Name         string   `json:"name"`
    Description  string   `json:"description"`
    Triggers     []string `json:"triggers"`
    Capabilities []string `json:"capabilities"`
    Source       string   `json:"source"`
    SourceURL    string   `json:"source_url"`
    Rules        string   `json:"rules"`
}
```

Tenerlo del lado del cerebro y no reusar el del cuerpo es a propósito: **la tool define el
contrato**, y el cuerpo es un cliente más. Si mañana hay otro consumidor, no depende de un tipo que
vive en el binario de la UI.

### Qué campos NO se exponen, y por qué

`GeneratedBy`, `GeneratedAt` y `ManagedChecksum` quedan afuera. Son metadatos internos de cómo
Musubi gestiona el archivo (el checksum decide si una skill cognitiva fue editada a mano); un panel
que lista el arsenal no los usa, y exponerlos ata el contrato a un detalle de implementación que sí
puede cambiar.

## El array vacío (A2)

```go
out := make([]skillListada, 0, len(all))
```

Construirlo con `make` y no declararlo con `var` es la diferencia entre `[]` y `null` en la
respuesta. Es una línea, y es un invariante con test propio porque se pierde sin que nada avise.

## Clasificación de superficie de lectura (A5)

La tool es `readOnly: true` y lee **el filesystem**, no una tabla con `project_id`. Entra en
`noScopedRead` del `TestEveryReadOnlyToolClassified`, junto a `musubi_detect_stack`, que hace lo
mismo.

Ese guard es el que cierra el whack-a-mole del Track 19 por contrato: si no clasifico la tool, el
test falla y no me deja pasar. Es exactamente lo que tiene que hacer, y por eso se clasifica en vez
de eximirse.

**La decisión de fondo:** el arsenal del central es compartido a propósito —es el arsenal de
empresa—, así que no hay dato de tenant que aislar. Lo que sí es por-proyecto son las *decisiones*
sobre skills, y esta tool no las lee. Si algún día filtrara por decisiones, dejaría de pertenecer a
`noScopedRead` y habría que moverla al barrido.

## El arreglo del cuerpo (A7)

```go
sk, err := cc.ListSkills("", 40)
if err != nil {
    snap.Errors["arsenal"] = err.Error()
} else {
    for _, s := range sk { ... }
}
```

El patrón `if err == nil { … }` sin rama de error ya se usaba dos líneas más arriba para `Doctor()`
y `Conflicts()`. Acá se corrige el del arsenal, que es donde el silencio produce un dato falso: una
lista vacía **se lee como «no hay skills»**, mientras que un doctor que no responde deja el campo en
su cero y el panel no afirma nada.

`snap.Errors` ya existe y ya se usa para el canal central, así que no hay estructura nueva.

## Riesgos, dichos de frente

- **El filtro por `query` es subcadena, no semántico.** Buscar «testing» no encuentra una skill
  llamada «pruebas». Es deliberado: esta tool lista un arsenal chico y conocido; la búsqueda
  inteligente es el trabajo de `musubi_search_skills`.
- **Devuelve `rules` completo.** Una skill con reglas largas infla la respuesta. Con la escala real
  del arsenal no es problema, pero si crece, el corte natural es un `limit` por defecto — no
  truncar el contenido en silencio, que es justo el error que el PR #258 tuvo que arreglar en
  `musubi_ask`.
- **Que el panel muestre datos no prueba que sean del central.** El cuerpo cae al cerebro local si
  no hay token; la verificación de extremo a extremo tiene que confirmar contra qué habló.
