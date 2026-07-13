# Proposal — registros-historicos-nunca-son-destino

## Intención

**El 20% de la cola de conflictos son pares cuyo veredicto no puede significar nada — y el historial lo demuestra.**

Medido sobre las **169 relaciones** que existieron en la memoria real:

| | n | veredictos SUSTANTIVOS (`supersedes` / `conflicts_with`) |
|---|---:|---:|
| `sdd → sdd` | 83 | **0** |
| `commit → commit` | 16 | **0** |
| `nota → nota` | 30 | **8** (los únicos `supersedes` de toda la base) |

**Los 8 `supersedes` que existen son TODOS `nota → nota`.** Ninguno oculta un commit ni un contrato. La práctica ya venía respetando una regla que el código nunca escribió.

## La causa

G3 dice hoy: *«un registro histórico no puede ser el DESTINO de **otra clase**»*.

```go
if historicalRecord(b.topicKey) && !sameKind(a.topicKey, b.topicKey) {
```

Esa excepción — `!sameKind` — deja pasar **commit vs commit** y **sdd vs sdd de cambios distintos**. Y no tiene defensa:

- **`supersedes` OCULTA el destino del recall.** Que un commit oculte a otro commit es **borrar historia**. Un commit no deja de haber pasado porque venga otro después.
- **El veredicto está decidido de antemano**, que es exactamente el criterio que G3 ya aplica… a medias.

`sameKind` se justificó con *«dos commits pueden ser el mismo commit»*. **Es falso en la práctica**: 16 pares commit↔commit, 0 duplicados. Los commits son únicos por naturaleza — tienen SHA.

## La regla, dicha entera

> **El registro histórico es el libro mayor; la nota es la creencia actual. Sólo las creencias se reemplazan.**

Un commit (lo que **pasó**) y un contrato SDD (lo que se **acordó**) son asientos del libro. Se leen, se citan, se vuelven obsoletos — pero **no se tachan**. Lo que cambia cuando la realidad cambia es la **nota** que dice qué creemos hoy, y esa nota sí puede ser reemplazada por otra.

## Alcance

- **F1** — `complementaryPair` bloquea el par **siempre que el TARGET sea un registro histórico**, sin excepción por clase.

## El descubrimiento: las tres guardas eran UNA

Al quitar la excepción, **G1 y G2 quedan subsumidas por G3**:

- **G1** (hermanos del mismo cambio SDD) — su target es `sdd/<cambio>/<fase>` ⇒ **histórico**.
- **G2** (el evento vs el contrato) — su target es un commit o un spec ⇒ **histórico**.

Las tres reglas que se descubrieron por separado, en tres PRs distintos, eran **tres caras del mismo principio**. La función colapsa a un predicado:

```go
func complementaryPair(_, b obsRow) bool { return historicalRecord(b.topicKey) }
```

**Esto NO es un atajo de código: es la regla apareciendo por fin entera.** Los tests que pinean G1 y G2 siguen verdes **sin tocarse** — son a la vez la prueba de que el colapso preserva el comportamiento y la red que impide que se pierdan si alguien angosta `historicalRecord` mañana.

## Los tests que SÍ se rompen (y por qué está bien)

Cuatro tests pinean **deliberadamente la excepción** que este PR quita, y uno lleva el comentario *«la guarda se pasó de ancha»*. **No son tests equivocados: eran el contrato acordado, y la evidencia lo refutó.** Se reescriben para pinear el contrato nuevo.

Lo que esos tests **legítimamente protegían** — el miedo a que la guarda se vuelva un **martillo** — lo siguen cubriendo, intactos y verdes, `TestUnCommitSiPuedeVolverObsoletaUnaNota` y `TestGuardaNoTapaCommitVsNota`: ambos con **destino = nota**. Esa es la red anti-martillo de verdad, y no se toca.

## La asimetría se conserva (y es lo que impide que sea un martillo)

Se mira **sólo el target**. Un commit `feat: migrar de X a Y` **sí** puede volver obsoleta una nota que decía "usamos X" — el commit es **evidencia** de que la nota envejeció. Eso sigue funcionando: el target ahí es la *nota*, no el commit.

## Fuera de alcance

- NO se toca el scoring, ni los umbrales, ni la banda ciega, ni MMR.
- NO se borra ni se re-juzga ninguna relación ya existente. El cambio afecta a las relaciones **futuras**.

## Riesgos

- **Dos contratos SDD de cambios distintos SÍ pueden contradecirse de verdad.** Pasó en esta misma sesión: la spec de `gists-que-dejan-decidir` decía *«nunca truncar al medio»* y la medición la refutó.
  **Mitigación (y por qué igual es correcto bloquear):** el contrato registra **lo que se acordó entonces**. Ocultarlo pierde historia. Lo correcto es una **nota** con el principio nuevo que reemplace a la nota vieja — que es, literalmente, lo que hacen los 8 `supersedes` que ya existen.
- **Se pierde la capacidad de marcar dos commits como duplicados.** Nunca se usó (0 de 16) y los commits tienen SHA.

## Rollback

Un `if` de una línea. Revertir el PR restaura la excepción `sameKind` exacta.
