# Spec — El arranque con arsenal

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

---

## B1 · `musubi_list_skills` gana `source`

| Argumento | Tipo | Obligatorio | Significado |
|---|---|---|---|
| `query` | string | no | filtro por subcadena sobre nombre y descripción (ya existía) |
| `limit` | int | no | techo de resultados (ya existía) |
| `source` | string | no | `local` (default) · `central` · `all` |

Cada entrada devuelta suma dos campos:

| Campo | Cuándo aparece | Significado |
|---|---|---|
| `origin` | siempre | `local` si salió del disco de este proyecto, `central` si salió del arsenal |
| `installed` | sólo en entradas `central` | `true` si este proyecto ya tiene una skill con ese nombre |

Sigue siendo **`readOnly`**: lee el disco local y hace una lectura remota; no escribe nada.

### G1 — El default no cambia nada

Sin `source`, la tool se comporta **exactamente** como hoy: lee `.musubi/skills/*.yaml` y no toca la
red. `query` y `limit` filtran igual.

> Está en producción desde la Fase A (F5.1) y la Forja del cuerpo la consume. Una Fase B que le
> cambie el default rompe a un consumidor vivo.

### G2 — `central` lista el arsenal, no el disco

Devuelve lo que hay en el central **aunque el proyecto local no tenga ninguna skill**. Es la prueba
de que no está leyendo el disco y llamándolo arsenal.

### G3 — Cada entrada del central dice si ya la tenés

`installed: true` cuando existe una skill local con ese nombre; `false` cuando no. Las entradas de
`origin: local` **no** traen el campo.

> Sin esto la lista no sirve para decidir: la pregunta que uno se hace mirando el arsenal es
> «¿qué me falta?», no «¿qué existe?».

### G4 — `all` no duplica

Una skill presente en los dos lados aparece **una sola vez**, con `origin: local`. El arsenal aporta
únicamente lo que falta.

> `all` responde «qué tengo y qué más podría tener». Listar `go-rules` dos veces no es más
> información, es ruido — y esconde lo único que importa, que es el faltante.

### G5 — Sin central configurado, `central` y `all` fallan diciendo por qué

Con `sync.central_url` vacío o sync apagado, las dos devuelven un error explícito que nombra las
claves de config. **Nunca** devuelven la lista local haciéndola pasar por el arsenal.

> Es el F2 de la Fase A aplicado a la lectura. Un `promote` que «anda» sin subir nada es malo; una
> lista que «anda» mostrando lo local como si fuera el arsenal es peor, porque parece un arsenal
> vacío y la conclusión es «no hay nada que instalar».

### G6 — Un `source` inválido es error, no un default silencioso

`source: "centrall"` devuelve error de argumento. No cae a `local`.

> Un typo que degrada a `local` produce exactamente la mentira de G5, pero por accidente.

### G7 — Sigue siendo `readOnly`

`TestToolReadOnlyClassification` y `TestEveryReadOnlyToolClassified` siguen verdes **sin
relajarse**, y el conjunto exacto de tools `readOnly` no cambia.

---

## B2 · `musubi provision --skills`

| Flag | Efecto |
|---|---|
| ausente | no se instala nada; el reporte deja un paso `todo` que dice cómo pedirlo |
| `--skills` | tras cablear el sync, instala en el proyecto las skills del arsenal que falten |
| `--skills --dry-run` | informa qué instalaría; no escribe |

### G8 — Sin el flag no se instala nada

`.musubi/skills/` queda byte por byte como estaba, y **no se hace ninguna llamada al central**.

> El paso informativo no puede costar una llamada de red: `provision` tiene que seguir sirviendo
> para unir una máquina aunque el arsenal esté caído.

### G9 — Con el flag, el arsenal aterriza

Las skills del arsenal quedan escritas en `.musubi/skills/` del proyecto, con sus `triggers`,
`capabilities` y `rules` completos, y marcadas con su procedencia (`source: arsenal-central`).

### G10 — No pisa nada de lo local

Una skill que ya existe con ese nombre se reporta como salteada y **su contenido en disco queda
intacto**. `provision` no tiene `overwrite`.

> `provision` es idempotente por diseño y se corre varias veces sobre el mismo proyecto. Que pise
> una skill que editaste a mano sería una pérdida silenciosa de trabajo.

### G11 — `--dry-run` no escribe

Ni un archivo nuevo ni uno modificado.

### G12 — La instalación pasa por la MISMA puerta *(el invariante de seguridad)*

`--skills` escribe reusando `musubi_install_skill`, que a su vez usa `writeSkillFile`: gate de
calidad y guarda de path traversal. **No abre una segunda puerta de escritura.** Un nombre
malicioso en el arsenal —`../evil`, ruta absoluta, separadores— no puede escribir fuera de
`.musubi/skills/`.

> Es el F3 de la Fase A, y la razón de repetirlo: el camino nuevo (provision) es exactamente donde
> se cuela una segunda puerta, porque «total ya validamos al instalar». El sabotaje que vale es
> escribir sin pasar por `writeSkillFile` — el que sólo quita la validación pasa en verde por la
> defensa en profundidad, y eso ya nos mordió una vez.

---

## Fuera de alcance, explícito

- **No se re-traen actualizaciones.** Si una skill del arsenal cambió después de instalada, esto no
  la refresca. La marca de procedencia deja la puerta abierta; la comparación no está construida.
- **No hay curaduría por proyecto.** `--skills` trae el arsenal entero. Elegir un subconjunto se
  hace con `musubi_install_skill` de a una.
- **No se resuelve el problema del trigger `*`.** Sigue sin poder distinguirse una skill
  intencionalmente universal de una a la que no se le puso trigger. Declarado en la propuesta.
- **El cuerpo no se toca.** La Forja gana la tool que le faltaba; recablearla es trabajo de su
  propia terminal.

---

## Criterios de aceptación

1. Los 12 invariantes con test propio, cada uno verificado **fallando** al sabotear.
2. `go build ./...`, `go vet ./...`, `go test ./...` y `golangci-lint run` en verde.
3. La clasificación `readOnly` no se relaja (G7).
4. Prueba de extremo a extremo contra el binario y un central real: en un proyecto vacío,
   `source: central` muestra el arsenal con `installed:false`, `provision --skills` lo instala, y
   una segunda corrida lo reporta salteado sin tocar el disco.
