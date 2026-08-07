# Spec — El arsenal federado

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

---

## Las tools

### `musubi_promote_skill` — sube una skill local al arsenal del central

| Argumento | Tipo | Obligatorio | Significado |
|---|---|---|---|
| `name` | string | sí | slug de la skill local a promover |
| `overwrite` | bool | no | reemplaza la del arsenal si ya existe (default `false`) |

### `musubi_install_skill` — baja una skill del arsenal al proyecto

| Argumento | Tipo | Obligatorio | Significado |
|---|---|---|---|
| `name` | string | sí | slug de la skill del arsenal |
| `overwrite` | bool | no | reemplaza la local si ya existe (default `false`) |

Las dos **escriben**, así que ninguna es `readOnly`.

---

## Invariantes

### F1 — La skill viaja completa

Promover envía `name`, `description`, `triggers`, `capabilities` y `rules` tal como están en disco.
Ningún campo se pierde ni se inventa en el camino.

> Es el invariante de fidelidad: una skill que llega al arsenal sin sus `triggers` no dispara nunca
> y se ve como si estuviera instalada. Falla silenciosa, la clase de bug que este track viene
> arreglando.

### F2 — Sin central configurado, falla diciendo por qué

Con `sync.central_url` vacío o sync apagado, las dos tools devuelven un error explícito. **Nunca**
devuelven éxito sin haber hablado con nadie.

> El fallo silencioso es el enemigo declarado del track: un `promote` que "anda" sin subir nada es
> peor que uno que falla.

### F3 — Instalar pasa por la misma puerta que guardar *(el invariante de seguridad)*

`musubi_install_skill` escribe con el mismo camino que `musubi_save_skill`: gate de calidad y guarda
de path traversal. Un `name` malicioso del arsenal —`../evil`, rutas absolutas, separadores— **no
puede escribir fuera de `.musubi/skills/`**.

> El contenido del arsenal es dato remoto. Tratarlo como confiable porque "viene de nuestro central"
> es exactamente cómo se cuela un escape de directorio. La puerta ya existe y está probada; esta
> tool no abre una segunda.

### F4 — Una skill adoptada se distingue de una propia

La instalada queda marcada con su procedencia (`source`), y esa marca sobrevive en disco. Se puede
responder «¿esto lo escribí yo o lo bajé?» sin adivinar.

### F5 — Sin `overwrite`, nada se pisa

Si ya existe una skill con ese nombre —local al instalar, remota al promover— la operación se
rechaza con un mensaje que lo dice. Con `overwrite: true` se reemplaza.

### F6 — Promover una skill que no existe falla, no sube vacío

Un `name` que no está en el arsenal local devuelve error. No se promueve un esqueleto.

### F7 — Ninguna de las dos es `readOnly`

Las dos escriben. El guard de clasificación del Track 19 tiene que verlas como tools de escritura, y
el conjunto exacto de `readOnly` no cambia.

---

## Fuera de alcance, explícito

- **No hay sincronización automática.** Nada sube ni baja sin pedido. Ver la propuesta: 7 de las 11
  skills locales tienen trigger `*`, y la curaduría es del dueño.
- **Un proyecto nuevo todavía necesita `sync.central_url` configurado** para usar estas tools. Ese
  es el problema de la Fase B (arranque de proyecto), no de ésta.
- **No se resuelven colisiones de nombre entre proyectos.** El arsenal es un espacio de nombres
  plano y compartido; dos proyectos que promuevan `go-rules` compiten. Hoy se resuelve con
  `overwrite` explícito; si duele, es material de otra fase.

---

## Criterios de aceptación

1. Los 7 invariantes con test propio, cada uno verificado **fallando** al sabotear.
2. `go build ./...`, `go vet ./...`, `go test ./...` y `golangci-lint run` en verde.
3. `TestToolReadOnlyClassification` y `TestEveryReadOnlyToolClassified` siguen verdes **sin
   relajarse**.
4. Prueba de extremo a extremo contra el binario: una skill escrita en un proyecto aparece en el
   arsenal y se instala en OTRO proyecto.
