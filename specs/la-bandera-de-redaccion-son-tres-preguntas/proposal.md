# Propuesta — La bandera de redacción son tres preguntas

## El problema

Una sola línea de `internal/mcp/http.go` (hoy la **934**) decide tres cosas que no tienen
nada que ver entre sí:

```go
// Redacción forzada server-side (16.1d): un bind no-loopback es infra compartida ⇒
// redactar SIEMPRE (fail-closed, no se puede desactivar); un loopback puede optar por
// config. Cierra el hueco de un cliente que manda scope=local con un secreto crudo.
s.forceRedact = !loopback || cfg.ForceRedact
```

`loopback` sale de `isLoopbackHost(cfg.Addr)` — o sea de **en qué interfaz escucha el
proceso**. De ese único booleano cuelgan, medido en `origin/main` (`674d3366`):

| # | Quién lo lee | Cuántos sitios | Qué significa **ahí** |
|---|---|---|---|
| 1 | `redactIfForced` (`methods.go`) | **22** call sites en 5 archivos | «redactá el ingest: el pozo es compartido» |
| 2 | `methods.go`, el `scope = memory.ScopeShared` | **1** | «acá todo es shared, **aunque el cliente pida `local`**» |
| 3 | `arbolFueraDeAlcance()` (`methods_codegraph.go`) | **5** (el helper es `return s.forceRedact`) | «el árbol de código **no está en este disco**» |

⚠️ Esos conteos salen de un grep que **excluye la definición y los comentarios**. Un
`grep -c` pelado da 23 y 6: cuenta la línea del `func` y, en el caso del árbol, además un
comentario de `methods_codegraph.go:1577` que nombra al helper sin llamarlo.

Las tres respuestas coinciden hoy **por accidente de despliegue**, no porque la pregunta
sea la misma. El central escucha en `127.0.0.1:7717` detrás de `tailscale serve`; la
laptop escucha en `127.0.0.1` y es de una persona sola. **Las dos se ven idénticas desde
el bind**, y la única razón de que el central hoy redacte es que alguien le puso
`force_redact: true` a mano en el `config.yaml` — eso fue el paso 1, PR #777.

### Por qué eso es un defecto y no una comodidad

La #3 es la que tiene dientes. Si alguna de las otras dos obliga a poner la bandera en
`true` en un daemon **local**, el árbol de fuentes sí está en disco — y aun así quedarían
apagadas, en silencio, las tres rutas que lo consultan. Y el síntoma es engañoso: el
indicador que uno miraría para comprobar que el grafo está sano (`ghosts: 0`) es
exactamente lo que devuelve un grafo que **dejó de mirar el disco**.

Al revés también: hoy nadie puede apagar la deducción del árbol sin tocar la redacción.
Son la misma variable.

### Lo que **no** se puede borrar: 4 de los 5 sitios son guardas, no optimizaciones

Medido leyendo cada uno, el default del disco **no** es «no sé»:

| Sitio | Sin el predicado, el disco falla y queda… |
|---|---|
| `cgStale` | `FileFingerprint` da error ⇒ **`return true`**: marca TODO el grafo federado como rancio |
| `graphFreshness` | un `ghosts++` por archivo ⇒ **todo fantasma** |
| `pistaDelMiss` | el `os.Stat` falla ⇒ el hint dice **«es de otra rama»** cuando la verdad es «no tengo el árbol» |
| `methods.go`, la huella | `huella` queda `""` ⇒ **`frescuraDesconocida`** en vez de usar la huella del grafo |

El quinto —y único— que sí es una optimización es la cabecera de `methods.go` (el
`!s.arbolFueraDeAlcance() && s.dentroDelProyecto(path)`): ahí `readProjectFile` ya
devuelve error, así que sin el predicado el comportamiento sería idéntico, nomás con un
syscall de más.

O sea: **el predicado hace falta**. Lo que está mal es de dónde sale su valor.

## El precedente: esto ya pasó en este archivo, y se arregló al revés

`confianzaLocal` (`internal/mcp/http.go`, hoy las líneas **141-159**) tuvo el **mismo
error de clase**, y su comentario lo deja escrito:

> ANTES ESTO SE DEDUCÍA DEL BIND (`loopbackOnly`, que salía de `isLoopbackHost(cfg.Addr)`),
> y esa era la pregunta equivocada… LA PREGUNTA QUE SÍ DECIDE ES SI HAY CREDENCIAL

El arreglo no fue cambiar el default ni documentar mejor la deducción. Fue **dejar de
deducir y preguntar la pregunta que sí decide**:

```go
func (o httpOptions) confianzaLocal() bool { return o.registry == nil && o.token == "" }
```

Esta propuesta es ese mismo movimiento, tres veces.

## Lo que se propone

Un campo por pregunta, cada uno con su propio origen y su comentario de **por qué**:

| Pregunta | De dónde sale hoy | De dónde debería salir |
|---|---|---|
| ¿Redacto todo el ingest? | el bind, o la clave | **política declarada**: la clave `force_redact`. Es una decisión de operación, no un hecho medible — y el default se conserva **fail-closed** |
| ¿Fuerzo `scope=shared`? | el bind, o la clave | la pregunta es «¿este pozo es de más de una persona?», y **ya hay una señal para eso**: `s.memory.TeamMode`, la que usa `defaultScope()` cuando el scope viene vacío |
| ¿El árbol está en disco? | el bind, o la clave | **se mide**. No hay decisión que tomar |

La tercera es la que no necesita ninguna política, y el medidor **ya existe en el mismo
paquete**: `hayRepoGit(dir)` (`internal/mcp/codegraph_foto_al_commit.go`, línea ~123),
escrito para otra cosa y con su comentario de por qué no se le puede preguntar a `git`
directamente. Casa con la realidad en los dos extremos: el `MUSUBI_HOME` del central
(`/home/musubi/musubi-brain`, que «sólo contiene `.musubi/` y ni siquiera es un repo
git», según el comentario de `arbolFueraDeAlcance`, medido el 2026-09-05) da **false**;
un repo clonado da **true**.

### ⚠️ Y el contraejemplo que hay que decir antes de que lo encuentre el reviewer

**`hayRepoGit` no es exactamente la pregunta.** Un proyecto con código y **sin `.git`**
—una carpeta suelta, el caso que `inerte.go` menciona para el adjudicador B1— tendría
`hayRepoGit == false` y los archivos **sí** en disco. Hoy ese caso funciona bien (bind
loopback ⇒ `false`), y con `hayRepoGit` pelado **se rompería**: apagaría la frescura
justo donde hoy funciona. Es una regresión, chica pero real.

Así que esta propuesta **no fija el medidor**. Fija que la pregunta es medible, y deja al
`design` elegir entre los candidatos con su costo ya nombrado:

- **(a)** `hayRepoGit(projectPath)` — barato, ya existe. Falla en la carpeta sin `.git`.
- **(b)** `walkSourceTree()` no vacío — **exacto** (es la MISMA enumeración que usa el
  indexador, de donde ya sale el denominador de `graphFreshness`), pero recorre el árbol:
  inviable en el camino caliente, porque `cgStale` se llama **por nodo**.
- **(c)** (b) **una sola vez al arranque**, cacheada en el campo — exacta y barata donde
  importa. Es la forma del precedente: `confianzaLocal` también se resuelve una vez, de
  las opciones. Costo honesto: no ve un `git clone` posterior al arranque.

## Alcance

**Entra:** separar el campo en tres, cablear cada grupo de sitios al que le corresponde, y
un test por invariante que quede FALLANDO bajo sabotaje del invariante que declara.

⚠️ **El recableado es más chico de lo que sugiere la cuenta de 28**, y conviene decirlo
antes de que alguien dimensione el PR por ese número. Los 5 sitios del árbol **no se
tocan**: ya pasan todos por el helper `arbolFueraDeAlcance()`, así que la #3 se arregla
cambiando **un cuerpo de función**. Eso no es casualidad — el helper fue un paso previo en
esta misma dirección, y el comentario de `codegraph_miss_explicado_test.go` lo cuenta:
`cgStale` y `graphFreshness` ya preguntaban cada uno por su lado, `pistaDelMiss` era la
tercera que lo necesitaba y no lo tenía, «ahora las tres comparten `arbolFueraDeAlcance`».
Lo que quedó a medias es su **implementación**, que sigue siendo `return s.forceRedact`.
Esta propuesta termina ese movimiento; no deshace nada.

**No entra:**

- **Aflojar la redacción.** El #777 acaba de cerrar el agujero de que el instalador
  perdiera `force_redact`. El default de la #1 se queda **fail-closed**: si no hay clave
  declarada, conserva exactamente el valor que da hoy. Esta propuesta separa, no relaja.
- Cambiar `defaultScope()`, que ya hace bien su pregunta para el scope **ausente**. Lo
  que esta propuesta toca es el scope **explícito**, que es otra cosa.
- El paso 3 que quedaba del expediente de `force_redact`. Va al plan siguiente.
- Tocar el bind, `tailscale serve` o el despliegue del central.

## Rollback

El cambio es local a `internal/mcp` y no toca esquema de base, protocolo MCP ni formato de
config, así que **revertir el commit alcanza** y no deja estado que migrar. Las tres
propiedades que hay que comprobar después de revertir:

1. `go test ./internal/mcp -count=1` en verde;
2. el golden de `tools/list` sin cambios — esta propuesta no agrega ni cambia tools, así
   que un golden movido es señal de que se colaron otras cosas;
3. en el central, `force_redact: true` sigue produciendo `ghosts: 0` **y** redacción
   efectiva, que es la propiedad que el paso 1 dejó medida y no se puede perder.

Y una salvaguarda de despliegue: si la #3 pasa a medirse y el central quedara con el árbol
**dentro** de alcance por un error de medición, el síntoma no es silencioso —
`graphFreshness` empezaría a reportar fantasmas en masa, que es ruidoso y se ve en el
primer `musubi_map`. **(Corrección de la fase `verify`, 2026-10-31: esta oración decía
«el primer `musubi_readiness`» y era falsa — `graphFreshness` NO está conectado a
`musubi_readiness`, así que esa alarma no existe. Conectarlas quedó como cabo para el
plan siguiente.)** El modo de falla es el ruido, no el silencio. Eso es
deliberado: el silencio es lo que hace caro el defecto de hoy.
