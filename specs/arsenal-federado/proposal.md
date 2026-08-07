# Propuesta — El arsenal federado (Fase A del track «Conocimiento unificado»)

## Lo que el usuario quiere, en sus palabras

> *«que las skills que se hacen con Musubi en cualquier lado, el central pueda aprender de eso y
> guardarlas, para que al comenzar un proyecto nuevo pueda ocuparlas — por eso hice el módulo de
> Forja en el cuerpo»*

Y el encuadre que ordena todo el track:

> *«queremos mudarnos al cuerpo, pero mientras tanto hay que trabajar desde las terminales»*

## La regla de reparto

> **Si el cuerpo puede hacer algo que la terminal no puede, ese algo está en el lugar equivocado.**

El **cerebro** es dueño de *qué es el conocimiento*: guardarlo, juzgarlo, federarlo.
El **cuerpo** es dueño de *cómo se ve y se usa*: la ventana, el editor, el diff, la voz del agente.

De ahí sale la decisión de dónde se construye esta fase: **en el cerebro**. Así la terminal la gana
el mismo día, trabajar desde terminal deja de ser un downgrade, y el día de la mudanza al cuerpo no
se migra conocimiento — se cambia de ventana.

Es además el diagnóstico de lo que ya pasó: la Forja se construyó bien y no funcionaba, porque la
capacidad no existía del otro lado. **Una UI no puede tapar un hueco del cerebro.**

## El estado, medido

| Pilar | ¿Se unifica hoy? |
|---|---|
| Memoria | **Sí** — outbox saliente + pull entrante, probado en vivo PC↔laptop |
| Grafo de código | **Sí** — federado desde Track 20 (`codegraph_push`) |
| **Skills** | **No.** Ni suben ni bajan |

Las skills son la única pata rota, y se puede probar por qué:

- **El sync NO mueve skills.** Cero menciones a skills en el outbox y en el inbound: sólo viajan
  observaciones. Una skill guardada desde una terminal se queda ahí para siempre.
- **No existe ninguna tool que instale una skill del central en un proyecto.** «Adoptar» en la Forja
  llama a `musubi_log_skill_decision`, que **sólo registra la decisión**.
- `musubi_save_skill` escribe en el `.musubi/skills/` de *la instancia que corre*. Por eso la Forja
  —que disca al central— sí llega, y una terminal no.

**La consecuencia se ve en el dato:** el arsenal del central tiene exactamente UNA skill,
`go-table-driven-tests`, que es la que se creó probando la Forja el 2026-08-04. Ninguna otra tuvo
camino hacia arriba. Esta PC tiene 11 que nunca subieron.

## Qué se construye

Dos tools en el cerebro, sobre plomería que ya existe:

- **`musubi_promote_skill`** — sube una skill local al arsenal del central. **Explícita**, nunca
  automática: el dueño elige qué merece ser conocimiento de empresa.
- **`musubi_install_skill`** — baja una skill del arsenal y la escribe en el proyecto. Convierte
  «Adoptar» en una acción real.

Y **procedencia**: una skill adoptada queda marcada como tal, para distinguirla de una propia y
poder re-traerla cuando cambie.

No hay transporte nuevo. `SyncClient` ya habla MCP-sobre-HTTP con el central (`Push`, `PushGraph`,
`Pull`); estas dos son dos métodos más sobre el mismo cliente, el mismo bearer y la misma config.

## Por qué explícita y no automática

Subir todas las skills sola sería tentador y está mal. Medido sobre las 11 locales: **7 tienen
trigger `*`**, o sea que disparan en cualquier archivo. La resolución por triggers protege a las que
declaran extensiones (`go-rules` → `*.go`), pero no a esas siete.

Y hay skills que son locales por naturaleza: `project-profile` describe *este* proyecto, `starter`
es la plantilla que deja `musubi setup`. Subirlas sería ensuciar el arsenal de todos.

La curaduría es del dueño. La herramienta sólo tiene que hacerla fácil.

## Qué NO es

- **No mueve skills solo.** Nada sube ni baja sin que alguien lo pida.
- **No ejecuta nada.** Una skill son reglas en YAML; instalar es escribir un archivo, igual que hoy.
- **No reemplaza el gate de calidad.** Instalar pasa por la misma puerta que `musubi_save_skill`.
- **No acelera la mudanza al cuerpo.** El cuerpo todavía necesita la voz (loop del agente) y los
  ojos (superficie de edición). Esto hace que el cuerpo *valga* cuando llegue, y que mientras tanto
  no se pierda nada trabajando en terminal.

## Consumidor y métrica (exigidos por la regla del track)

- **Consumidor:** la Forja del cuerpo, ya construida y esperando; y las terminales, desde el primer
  día.
- **Métrica de cierre:** una skill escrita en un proyecto aparece en el arsenal del central y se
  puede instalar en OTRO proyecto, verificado de punta a punta contra el binario.
