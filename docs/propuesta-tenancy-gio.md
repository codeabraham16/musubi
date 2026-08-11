# Propuesta: el principal `gio` escribe dentro del tenant de Musubi

**Estado:** propuesta. **No se ejecuta sin decisión del dueño** — es la credencial de otra persona y
su forma de trabajar. Acá se documenta el hallazgo y las opciones.
**Medido:** 2026-08-11, cerebro central `v0.102.1`.

## El hallazgo

```
principal `gio`  →  project_id = "musubi",  read = all,  write = any
```

`write: any` sin declarar proyecto en cada escritura cae al `project_id` del principal
(`writeOriginFor`, `internal/mcp/principals.go`). Resultado:

**130 observaciones de autor `gio` viven dentro del tenant `musubi`** — el 12 % de sus 1.086 activas.

| topic | cuántas |
|---|---|
| `last-chaos-nostalgia` | 50 |
| `review` | 28 |
| `emisario` | 19 |
| `terminales` | 17 |
| `project` | 9 |
| otros | 7 |

Existe un proyecto `last-chaos` en el central… con **1 sola observación**.

Lo mismo aplica a `b1-adjudicador` (`project_id: musubi`, `write: any`), que es otro agente del lado
de gio.

## Lo que NO es

**No es la causa de que la cola de conflictos crezca.** Lo dije antes y estaba equivocado: gio es el
12 % de la memoria del tenant y el 47 % de los pendientes, pero los pares son **gio × gio del mismo
dominio** — las 50 memorias de Last Chaos se emparejan entre sí. Mudarlas a otro tenant las deja
**idénticas**, sólo cambian de casa. Y los consumidores medidos (`davantis-admin`, `davantis-mando`)
son `read: all`, así que las verían igual.

Reatribuir es **higiene de tenancy**, no una optimización.

## Por qué importa igual

1. **Un lector acotado ve memoria ajena como propia.** El principal `davantis` es `read: own` sobre
   `musubi`: para él, las 130 memorias de Last Chaos y Minecraft son parte del proyecto Musubi. Hoy
   ese lector casi no se usa —el tráfico va por credenciales federadas— pero es el modelo correcto y
   el que un miembro nuevo del equipo recibiría.
2. **La atribución por persona ya funciona** (`author = gio` está bien puesto). Lo que está mal es el
   *proyecto*, no el autor. Es un arreglo de una línea de configuración, no de diseño.
3. **El puntaje de madurez por proyecto** (`musubi_readiness`) mezcla dos proyectos en uno.

## Antes de tocar nada: preguntar para qué

Puede que escribir en `musubi` sea **a propósito**. La memoria del central se usa como canal entre
terminales (ver la nota `gio-auditoria-terminales`), y los topics `emisario/`, `review/` y
`terminales/` tienen toda la pinta de ser justamente eso: coordinación, no memoria de proyecto.

Si es así, la separación correcta no es «gio a su proyecto» sino distinguir **memoria de coordinación**
de **memoria de proyecto**, que es otra conversación y más interesante.

Lo que casi seguro **no** corresponde al tenant de Musubi son las 50 de `last-chaos-nostalgia` y las
de `minecraft`.

## Opciones, de menos a más invasiva

**A · No hacer nada.** Cuesta poco hoy: el tráfico real es federado. El costo es conceptual y crece
si algún día se usa una credencial acotada de verdad.

**B · Reemitir el token de gio con su propio `project_id` y `write: own`.** Arregla **lo nuevo**;
las 130 que ya están se quedan donde están. Es la opción de mejor relación valor/riesgo, y hay que
coordinarla: rotar un token deja stale la env de esa terminal hasta que se actualice (pasó el
2026-08-09, está en `fire-test-cuerpo-y-token-rotacion`).

**C · B + re-atribuir las 130 filas viejas.** Un `UPDATE observations SET project_id=...` sobre
producción. **No lo recomiendo:** es cosmético —nadie está leyendo con credencial acotada— y toca
datos reales de otra persona. Si se hiciera, va con backup y acotado por `author='gio' AND
topic_key LIKE 'last-chaos-nostalgia/%'`, nunca por autor a secas.

**Recomendación: preguntarle a gio primero** para qué escribe ahí, y con esa respuesta elegir entre
A y B. C sólo si aparece un consumidor acotado real.

## Cómo verificar el estado actual

Read-only, desde esta terminal:

```bash
# el registro de principals (no muestra tokens)
mcp: musubi_token_list

# el reparto real, en el server
ssh -o ProxyCommand="tailscale nc %h %p" musubi@100.79.126.62 'python3 -' <<'PY'
import sqlite3
db = sqlite3.connect("file:/home/musubi/musubi-brain/.musubi/memory.db?mode=ro", uri=True)
for r in db.execute("""SELECT COALESCE(NULLIF(project_id,''),'(sin atribuir)'),
                              COALESCE(NULLIF(author,''),'(sin autor)'), COUNT(*)
                       FROM observations GROUP BY 1,2 ORDER BY 3 DESC LIMIT 12"""):
    print("  %-16s %-18s %d" % r)
PY
```

(El server no tiene el CLI `sqlite3`, pero sí `python3` con el módulo. `mode=ro` no toca el WAL del
servicio.)
