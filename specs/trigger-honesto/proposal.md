# Propuesta — El trigger honesto

Track: **Conocimiento unificado**, la deuda que la Fase B dejó anotada de frente:

> *«Sigue sin poder distinguirse un `*` intencional de un `*` por pereza. Con skills de varias
> personas adentro va a ser el default, y el contexto se va a llenar de reglas que no aplican. Es
> el próximo problema real de este track.»*
> — `specs/arsenal-arranque/tasks.md`

## El diagnóstico que tenía, y por qué estaba mal

Yo lo planteé como un problema de disciplina: hay gente que pone `*` por no pensar el alcance, y
hace falta cobrárselo. **Medir las 12 skills locales dice otra cosa.**

```
adversarial-review      *              audit-structure-flow    *
orchestrate-multiagent  *              plan-ahead              *
project-profile         *              sdd-flow                *
starter                 *
analyze-project         *.go *.js *.ts *.tsx *.jsx
deduce-conventions      *.go *.js *.ts *.tsx *.jsx
designing-web-ui        *.html *.css *.tsx *.jsx *.vue *.svelte
go-hygiene              *.go            musubi-rules           *.go
```

7 de 12 con `*`. Pero **ninguna de las siete es pereza.** `orchestrate-multiagent` no aplica a
archivos `.go`; aplica a un **tipo de tarea**. `plan-ahead` y `sdd-flow`, igual. `project-profile`
describe el proyecto entero, no un archivo.

El vocabulario de triggers **sólo sabe de globs de archivo**. Una skill que se activa por tarea, por
lenguaje o por fase de trabajo no tiene cómo decirlo — y colapsa en `*`, que es lo único que le
queda. El `*` no es el defecto: es el **síntoma de un vocabulario pobre**.

Eso reencuadra el arreglo. No se trata de prohibir el `*` ni de regañar a quien lo usa: se trata de
que **el `*` diga qué quiso decir**.

## Lo que sí es un defecto medido

**El agujero:** `toolPromoteSkill` —la puerta al arsenal COMPARTIDO— no corre ningún gate de
calidad. `toolInstallSkill` sí lo corre al bajar, y `musubi_save_skill` y `musubi_author_skill`
también. La única puerta sin gate es justo la que publica para todos. Una skill escrita a mano, que
nunca pasó por ninguna tool, sube tal cual.

**El caso que todavía no existe y va a existir:** `allWildcard` exige que **todos** los triggers
sean `*`. Con `["*", "*.go"]` el chequeo no dispara — y ese caso es *peor*, porque el `*.go` hace
parecer que la skill está acotada cuando el `*` ya la activó en todo. Hoy no hay ninguna así; el
día que escriba skills alguien de otra máquina, la habrá.

## Qué se construye

1. **Un campo que declara el alcance no-archivo**: `always_because`. Viaja con la skill, así que
   quien la instala **lee por qué** está siempre encendida antes de adoptarla.
2. **El detector deja de ser ingenuo**: un solo `*` vuelve decorativos a los demás triggers, y eso
   se nombra aparte porque es una mentira sobre el alcance, no un descuido.
3. **La puerta de promoción cobra**: el gate de calidad corre al subir —hoy no corre— y un `*` sin
   declarar **no pasa**. Local seguís haciendo lo que quieras; el arsenal es de todos.

## Lo que NO se construye, dicho ahora

**Un vocabulario de alcance de verdad** (`applies_to: task | language | repo | phase`) es el arreglo
de fondo de lo que este spec diagnostica. No se hace acá: es especulativo sin saber qué skills va a
haber, y pertenece al track de la **Forja global** que el usuario abrió el 2026-08-07. Este spec
deja el problema *nombrado y contenido*, no resuelto de raíz — y lo dice en vez de fingir que sí.
