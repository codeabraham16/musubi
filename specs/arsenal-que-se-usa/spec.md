# Spec — El arsenal se usa

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

```go
// skills
func CuandoUsarla(sk Skill) string
func ASkillMD(sk Skill, checksum string) (string, error)
func ChecksumSkillMD(sk Skill) (string, error)
func ChecksumDeSKILLMD(contenido []byte) string
```

---

## H1 · Lo exportado dice CUÁNDO usarse

### E1 — La descripción exportada lleva el «cuándo»

En SKILL.md la selección la hace el consumidor leyendo la `description`. Una descripción que sólo
dice QUÉ hace deja a la skill sin forma de ser elegida — el archivo existiría y no serviría de nada.

### E2 — El «cuándo» sale de `applies_to`, y si no de `always_because`, y si no de los globs

En ese orden y por una razón: `applies_to` es vocabulario cerrado (traducible a una frase exacta),
`always_because` es prosa que el autor escribió a propósito, y los globs son el último recurso
mecánico. Nunca se inventa.

### E3 — Una descripción que YA dice cuándo no se duplica

`go-hygiene` empieza con «Usá cuando escribas o revises cualquier archivo .go». Anteponerle otro
«Usá cuando…» produciría una frase rota, y peor: sería el sistema pisando lo que un humano ya
escribió bien.

---

## H2 · Exportar es fiel y reversible

### E4 — El cuerpo de la skill viaja completo

Las `rules` son la skill. Un export que las recorte entregaría un archivo con cara de funcionar.

### E5 — El nombre y la ruta son los que el agente espera

`.claude/skills/<name>/SKILL.md`. Verificado contra una skill que ya funciona en esta máquina, no
contra la documentación.

### E6 — Es idempotente

Exportar dos veces sobre un árbol sin cambios no reescribe nada. Sin esto, cada `setup` tocaría
todos los archivos y ensuciaría cualquier diff o watcher.

---

## H3 · Nunca se pisa el trabajo de una persona

### E7 — Un SKILL.md editado a mano se PRESERVA

Misma regla de oro que `managedSkillAction`: ante la mínima duda, preservar. El checksum viaja en
`metadata.musubi_checksum`; si no coincide con el contenido, alguien lo editó y Musubi no lo toca.

### E8 — Un SKILL.md ajeno no se adopta

Un archivo sin checksum de Musubi no es de Musubi: puede ser una skill que el usuario instaló a mano
con el mismo nombre. No se pisa ni se reclama.

---

## H4 · Lo que se retira, se limpia

### E9 — Una skill borrada del origen se borra del export

Es el caso `starter.yaml`, ya documentado: una skill huérfana que nadie mantiene sigue costando
contexto en cada turno. Sólo se borra lo que **Musubi escribió y sigue intacto** (checksum válido):
lo editado a mano cae en E7 y sobrevive.

---

## Alcance declarado

- **`.claude/` está gitignored.** Lo exportado es estado local derivado; no se versiona.
- **Se exportan TODAS las skills de `.musubi/skills/`**, no sólo las cognitivas: el arsenal
  instalado desde el central es justamente el que se quiere poner a trabajar.
- **No se toca el resolvedor.** `musubi_resolve_skills` sigue como está para quien la llame.
- **No hay niveles.** Con un consumidor real andando, el costo se podrá medir en vez de proyectarlo.
