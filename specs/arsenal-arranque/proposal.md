# Propuesta — El arranque con arsenal (Fase B del track «Conocimiento unificado»)

## Lo que el usuario quiere, en sus palabras

> *«que las skills que se hacen con Musubi en cualquier lado, el central pueda aprender de eso y
> guardarlas, **para que al comenzar un proyecto nuevo pueda ocuparlas**»*

La Fase A construyó la primera mitad (el central puede aprenderlas). Ésta es la segunda: que un
proyecto nuevo las pueda **ocupar**.

## La corrección que impone medir

La Fase A cerró declarando este hueco:

> *«Un proyecto nuevo todavía necesita `sync.central_url` configurado para usar estas tools.»*

**Es falso, o casi.** `musubi provision` ya escribe ese bloque —`ensureSyncConfig` en
`internal/provision/syncconfig.go`, paso 5 de `Run`— con `central_url`, `auth_token_env` y
`allow_insecure_token`. Un proyecto provisionado **ya puede** llamar a `promote` e `install`.

El hueco real es otro, y es peor porque no se ve:

> **No hay forma de VER el arsenal.** `musubi_list_skills` lee `.musubi/skills/*.yaml` del disco
> LOCAL. Para instalar algo del central hay que saber su nombre exacto de memoria.

Una tool de instalación sin descubrimiento no es una tool: es una adivinanza.

## La ironía útil

El descubrimiento **ya está construido y escondido**. `FetchSkill`
(`internal/mcp/skillfed.go:133`) llama al `musubi_list_skills` del central, recibe **el catálogo
entero**, y tira todo menos el match exacto:

```go
txt, err := c.callCentral("install-skill:"+name, "musubi_list_skills",
    map[string]any{"query": name})
...
for _, p := range lista {
    if p.Name != name { continue }   // ← el resto del arsenal se descarta acá
```

No hay que construir plomería nueva. Hay que **dejar de tirar lo que ya llega**.

## El estado, medido

| Dato | Valor |
|---|---|
| Skills en el arsenal del central | **1** (`go-table-driven-tests`, la de probar la Forja) |
| Skills locales en este repo | 11 |
| Promovidas desde que existe la Fase A | 0 |
| Tools en el central | 54 — `promote_skill`, `install_skill` y `list_skills` verificadas en vivo |

## Qué se construye

**B1 — Ver el arsenal desde cualquier proyecto.** `musubi_list_skills` gana `source`:

- `local` (default) — el disco de este proyecto. El comportamiento de hoy, intacto.
- `central` — el arsenal, con cada entrada marcada `installed: true|false`.
- `all` — las locales, más las del arsenal **que no estén ya instaladas**.

Es el desbloqueo: sin esto, `musubi_install_skill` es inusable salvo de memoria.

**B2 — `musubi provision --skills`.** Un séptimo paso: después de cablear el sync, instala el
arsenal en el proyecto. Sin el flag, deja un paso `todo` que dice cómo pedirlo. Nada se pisa.

## El principio que las une

> **Curaduría al subir, adopción en bloque al bajar.**

La Fase A hizo `promote` explícito justamente para que el arsenal se mantenga limpio: el dueño
decide qué merece ser conocimiento de empresa. Si esa curaduría es real, **todo lo que está en el
arsenal ya fue juzgado digno**, y bajarlo entero es coherente — no contradice el «nada automático»,
lo cobra.

Y deja la asimetría en el lugar correcto: subir ensucia el espacio de todos y por eso se pide de a
una; bajar sólo afecta tu proyecto y es reversible borrando un archivo.

## Qué NO es

- **No sincroniza solo.** `provision --skills` es un flag que se escribe a mano. Sin él no se
  instala nada.
- **No re-trae actualizaciones.** Si una skill del arsenal cambió, esto no la refresca. La marca
  de procedencia (`source: arsenal-central`, Fase A/F4) deja la puerta abierta; la comparación no
  está construida.
- **No cambia el modelo de resolución.** Los `triggers` y las `capabilities` siguen decidiendo
  cuándo dispara una skill. Ver la nota de abajo.

## La pregunta abierta que esto NO resuelve

El usuario preguntó, y con razón:

> *«si el proyecto no es go no aplica la skill, ¿y eso por qué está hecho así? yo si quiero crear
> una skill es para que todos mis proyectos la utilicen»*

La respuesta medida: **una skill universal ya se puede escribir hoy** — trigger `*`, sin
`capabilities`. 7 de las 11 locales son así. El filtro no limita qué podés guardar; limita qué se
inyecta en el contexto de cada turno, que es lo que pagás.

Pero el mecanismo tiene una debilidad real que conviene anotar antes de que el arsenal se llene:
**`*` es a la vez «esto aplica siempre» y «no se me ocurrió un trigger»**. Con skills de varias
personas adentro, `*` va a ser el default y el contexto se va a llenar de reglas que no aplican.
Eso es material de otra fase; acá sólo queda declarado.

## Consumidor y métrica (exigidos por la regla del track)

- **Consumidor:** la terminal, desde el primer día; y la Forja del cuerpo, que hoy no puede mostrar
  el arsenal porque no hay tool que lo liste.
- **Métrica de cierre:** en un proyecto recién provisionado, `musubi_list_skills` con
  `source: central` muestra el arsenal con su marca de instalación, y `provision --skills` lo deja
  en `.musubi/skills/` — verificado de punta a punta contra el binario y un central real.
