# Investigación — Forja global

Track: **Forja global**, F0. El usuario pidió explícitamente **investigación antes que código**:

> «formular un plan de máxima capacidad de creación de skills, globales… para generar skills que las
> puedas utilizar en cualquier repo, en cualquier lenguaje, hardware, lo que sea. Hacer la idea
> principal primero: investigar cómo se crean las skills, cómo potenciarlas y estudios de las skills.»

Y después, una corrección de rumbo que cambió el análisis:

> «tené en cuenta **por qué está hecho así**, y el **costo de tokens**, pero también la **calidad** de
> lo que se puede hacer.»

Este documento **no propone un diseño**. Mide, y ordena el problema por esos tres ejes.

---

## 1. Por qué está hecho así (no es un descuido)

El resolvedor de skills matchea globs de archivo y nada más. La lectura fácil es «le falta
vocabulario». La lectura correcta está en el README, declarada como identidad del producto:

> «El núcleo es **model-free**: no hace inferencia ni gasta dinero.»
> «El núcleo permanece **model-free y determinista**.»

La cognición es el **tercer pilar y es opt-in**: nace apagada y el core es bit-idéntico sin ella. Un
resolvedor tipo Anthropic —donde el modelo lee descripciones y decide— pondría **inferencia en el
camino de toda resolución de skill**. Eso no es una mejora pendiente: es exactamente lo que Musubi
se niega a hacer.

**Consecuencia para este track:** cualquier propuesta que resuelva el problema metiendo un modelo en
el matcher está descartada de entrada, por principio y no por costo. La pregunta útil es otra:
**¿se pueden expresar los ejes que faltan sin inferencia?**

---

## 2. Lo que hay acá, medido

### El arsenal: 11 skills

| | |
|---|---|
| skills en `.musubi/skills/` | **11** |
| con `triggers: ['*']` | **6** — todas con `always_because` (el arreglo de `trigger-honesto` está puesto) |
| con globs reales | 5 |
| peso total | 17.076 B (~4.269 tokens) |

> Corrige la nota vieja de `roadmap/forja-global`, que decía 12 skills y 7 con `*`.

### ★ El límite no es el vocabulario: es la PREGUNTA

```go
func (r *Resolver) ResolveSkills(modifiedFiles []string) ([]Skill, error)
```

Y en el borde de la tool (`methods.go:2599`):

```go
if len(args.ModifiedFiles) == 0 {
    return nil, rpcErrorf(codeInvalidParams, "modified_files no puede estar vacío")
}
```

**No se puede preguntar «¿qué skill aplica?» sin nombrar un archivo.** Está prohibido por contrato.
Una skill sobre la **fase** del trabajo (`plan-ahead`) o la **forma** de la tarea
(`orchestrate-multiagent`) sólo se descubre nombrando un archivo que no tiene nada que ver con por
qué la skill importa. El `*` no es pereza: es la única forma de decir «no dependo de archivos».

Agregar tipos de trigger nuevos atacaría el síntoma. Mientras la pregunta sea *«dados estos
archivos, ¿qué aplica?»*, un vocabulario más rico sigue sin poder expresar una skill que no habla de
archivos.

### Lo que NO es el hueco (verificado, para no inventarlo)

- `musubi_search_skills` y `musubi_discover_skills` buscan skills **para instalar** en un catálogo
  remoto. No deciden cuál de las instaladas aplica.
- `capabilities` **sí** se verifica de verdad: `exec.LookPath` contra el PATH.

---

## 3. El costo en tokens, medido

`toolResolveSkills` devuelve los `Skill` **completos**, `rules` incluido. No hay niveles.

| | bytes | ~tokens |
|---|---|---|
| las 6 skills `*` (matchean **siempre**) | 8.074 | **~2.018** |
| el arsenal completo (11) | 17.076 | ~4.269 |
| sólo `name`+`description` de las 11 | 1.866 | **~466** (~42 por skill) |

**Hoy se inyectan ~2.018 tokens en cada resolución**, relevantes o no, porque las 6 wildcard matchean
cualquier archivo. El nivel 1 de progressive disclosure —nombre y descripción de **las once**— cuesta
466. Es **4,3× menos que lo que hoy se paga por seis**.

Con 11 skills eso es incómodo, no grave. La cuenta que importa es la de la ambición del usuario: un
arsenal global. Con 100 skills del mismo perfil (~55% wildcard, ~1.346 B promedio):

| modelo | costo por resolución |
|---|---|
| el actual (cuerpos completos) | **~18.500 tokens** |
| con niveles | ~4.200 fijos + un cuerpo (~340) |

Ése es el muro, y es lineal: no aparece de golpe, se va comiendo el contexto a medida que el arsenal
crece. **Progressive disclosure no es una optimización: es la condición para que «global» sea
posible.** Y —esto importa— es **model-free**: cargar por niveles no requiere inferencia, sólo
devolver menos.

---

## 4. El techo de calidad: dónde está la información que falta

Acá apareció lo más útil de la investigación, y no es mecánico.

El validador `quality.go` **ya tiene la regla correcta**, como warning:

> `desc_no_trigger` — «la description no dice CUÁNDO usar la skill; sin eso el agente casi no la dispara»

Medido contra las 11 instaladas:

| | genuinamente dice cuándo | trigger |
|---|---|---|
| `designing-web-ui` | sí («use when») | `*.html`, `*.css`… |
| `go-hygiene` | sí («Usá cuando escribas o revises cualquier archivo .go») | `*.go` |
| `musubi-rules` | sí («Usá cuando toques código de ESTE repo») | `*.go` |
| `adversarial-review` | **no** (pasa el validador por error, ver abajo) | `*` |
| `audit-structure-flow`, `orchestrate-multiagent`, `plan-ahead`, `project-profile`, `sdd-flow` | no | `*` |
| `analyze-project`, `deduce-conventions` | no | globs reales |

**La correlación es perfecta: las 6 skills con `*` son exactamente las 6 que no dicen cuándo
usarlas.** Y las 3 que sí lo dicen son las que menos lo necesitan, porque su glob ya lo expresa.

### ★★ CORRECCIÓN — la información SÍ está escrita, en `always_because`

Escribí que para esas 6 skills «la información de cuándo aplican no está en ninguna parte del
artefacto». **Es falso, y verificarlo cambió el plan entero.** Está en `always_because`, el campo que
`trigger-honesto` creó para que un `*` tenga que justificarse ante un humano:

| skill | lo que su `always_because` ya dice | eje |
|---|---|---|
| `plan-ahead` | «planificar es una **FASE** del trabajo: aplica antes de tocar cualquier cosa» | fase |
| `sdd-flow` | «gobierna el **flujo completo de un cambio**; no depende del lenguaje ni del archivo» | fase |
| `adversarial-review` | «se activa **al cerrar un cambio de riesgo**; el disparador es el **momento**, no el archivo» | fase |
| `orchestrate-multiagent` | «se activa por la **FORMA de la tarea** (grande y paralelizable)» | forma de tarea |
| `audit-structure-flow` | «el disparador es el **pedido de auditoría**, no un archivo» | forma de tarea |
| `project-profile` | «describe el **proyecto entero**; no hay un archivo al que atarlo» | alcance |

**Seis autores, por separado, escribieron exactamente qué eje necesitaban** — y caen en tres grupos,
dos de los cuales son los que el §5 identificó como faltantes. Ninguno inventó un eje raro.

Esto convierte a `always_because` en algo que no era su propósito: **un corpus empírico del
vocabulario que falta**. No hay que diseñar una taxonomía desde cero y esperar que alguien la use;
hay que leer la que ya se usó.

Y reordena el trabajo. Lo que hay que construir no es «hacer que las skills digan cuándo aplican»
—ya lo dicen— sino **volver matcheable lo que está escrito en prosa**. El campo es texto libre para
humanos; el resolvedor no lo mira.

### Un corolario que importa para los niveles

Si se adopta progressive disclosure, el nivel 1 es `name`+`description`. Para estas 6 skills el
«cuándo» **no vive ahí**: vive en `always_because`. Un nivel 1 que no lo incluya las deja mudas
justo en la capa donde se decide si se cargan.

### Un defecto concreto del validador

`adversarial-review` **pasa** el check y no debería. La lista `triggerClauses` incluye `"al "`, y el
match es por subcadena: su descripción dice «Revisión **adversarial estilo** debate…», y
«adversari**al** » contiene `"al "`. Un token de tres caracteres buscado dentro de palabras produce
falsos aprobados.

Cuentas reales: el validador reporta **6 de 11** en warning; las que de verdad no dicen cuándo son
**8 de 11**.

---

## 5. Los cuatro ejes, y cuántos faltan de verdad

| eje | ¿se expresa hoy? | ¿se puede model-free? |
|---|---|---|
| archivo / lenguaje por extensión | **sí** (`triggers`) | ya lo es |
| entorno / toolchain / «hardware» | **sí** (`capabilities`, vía `exec.LookPath`) | ya lo es |
| forma de la tarea | no | **sí, si el llamador la declara** |
| fase del trabajo | no | **sí, si el llamador la declara** |

`capabilities` es el eje que **ningún otro sistema expresa** y que Musubi ya tiene: es literalmente
el «cualquier hardware» que el usuario pidió, funcionando, sólo que pensado como precondición
técnica y no como alcance.

**La salida al dilema model-free.** El modelo no necesita *juzgar* qué skill aplica — necesita
*declarar qué está haciendo*. Un llamador que dice «estoy planificando» o «esto es un refactor»
convierte fase y forma-de-tarea en dos campos más para matchear **deterministamente**, sin una sola
llamada al modelo. El agente ya sabe en qué fase está; hoy no tiene dónde decirlo.

Eso preserva el principio del §1 y no gasta un token de inferencia.

### Y el techo honesto de eso

Lo que un matcher determinista **nunca** va a poder: «esta skill aplica cuando el código huele a
X». Eso es juicio, y requiere un modelo. La decisión de diseño que quedaría por tomar —y que este
documento no toma— es si ese caso vale un camino opt-in, como ya lo es la cognición.

---

## 6. El estado del arte

| Sistema | Cómo expresa alcance | Quién decide | ¿Gasta inferencia? |
|---|---|---|---|
| **Anthropic Agent Skills** | `name` + `description` («qué hace **y cuándo usarla**»), 3 niveles | el modelo | sí |
| **Cursor rules** (`.mdc`) | `alwaysApply` · `globs` · `description` · manual | según el modo | según el modo |
| **Copilot instructions** | `applyTo` con globs | determinista | no |
| **Musubi** | `triggers` (globs) + `capabilities` (PATH) | determinista | **no** |

Musubi y Copilot comparten el enfoque determinista, pero Copilot **inyecta** contexto de forma
pasiva mientras que Musubi expone un **resolvedor que se consulta** — y ese resolvedor es el que
rechaza la pregunta sin archivos.

Dato de calibración: Anthropic estima **~100 tokens por skill** en el nivel 1. Las descripciones de
Musubi promedian **~42**. Son la mitad de largas, lo cual es coherente con el §4: no cargan el
«cuándo».

---

## 7. Nadie mide si una skill sirvió

`skill_decisions` guarda `skill_id`, `name`, `decision`, `reason`: es «acepté o rechacé
**instalarla**», no «me sirvió cuando se activó». El ledger de uso mide **tools**, no skills.

Hoy no hay dato para decidir qué skill vale la pena. Cualquier plan que prometa «potenciar las
skills» sin resolver esto se va a evaluar con opinión.

---

## 8. Lo que esta investigación NO contestó

- **¿Hay estudios, o sólo best-practices de vendor?** Lo encontrado es documentación de producto
  (Anthropic, Cursor, GitHub) y divulgación. **No apareció evaluación empírica independiente** sobre
  qué hace efectiva a una skill. Se dice así de crudo para que nadie lo cite como estado del arte
  académico.
- **¿Una skill «para cualquier lenguaje» se escribe una vez o se parametriza por stack?** Depende de
  qué se decida en el eje «forma de la tarea».
- **El costo con niveles está calculado, no medido.** La proyección a 100 skills asume el perfil
  actual.

## 9. Lo que NO se decidió acá

Lo que la investigación deja firme es el **orden de los problemas**, ya corregido por el hallazgo
del §4:

1. **lo escrito en `always_because` no es matcheable** (§4) — la información existe, en prosa, y el
   resolvedor no la mira. Es el eslabón más barato y el que habilita todo lo demás;
2. **la pregunta exige archivos** (§2) — y el vocabulario para arreglarla ya está escrito en esas
   seis justificaciones, no hay que inventarlo;
3. **sin niveles, «global» no escala** (§3) — con la advertencia de que el nivel 1 tiene que incluir
   el «cuándo», que hoy no vive en la descripción;
4. **sin medir utilidad, no se puede priorizar** (§7).

Y una restricción que atraviesa los cuatro: **nada de esto puede meter inferencia en el matcher**
(§1). Los cuatro son resolubles sin ella.

---

## Fuentes

- [Introduction to agent skills — Anthropic Courses](https://anthropic.skilljar.com/introduction-to-agent-skills)
- [Agent Skills: Progressive Disclosure as a System Design Pattern](https://www.newsletter.swirlai.com/p/agent-skills-progressive-disclosure)
- [Cursor Rules: .mdc Frontmatter, globs & alwaysApply](https://techsy.io/en/blog/cursor-rules-guide)
- [Adding repository custom instructions for GitHub Copilot](https://docs.github.com/en/copilot/how-tos/configure-custom-instructions-in-your-ide/add-repository-instructions-in-your-ide)
- [Agent Skills Explained: SKILL.md Format and Adoption (2026)](https://atlan.com/know/ai-agent/ai-agent-skills/what-are-agent-skills/)
