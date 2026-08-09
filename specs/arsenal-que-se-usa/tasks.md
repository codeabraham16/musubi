# Tareas — El arsenal se usa

Estado: **completo**. Build, `go vet ./...`, la suite entera y `golangci-lint` (0 issues) en verde.
10 invariantes (E1–E10), 10 sabotajes, los 10 en rojo.

## La medición que reordenó el track

- [x] **T0 — ★ El arsenal estaba inerte.** `musubi_resolve_skills`: **0 llamadas en 30 días** en el
      ledger LOCAL y en el CENTRAL. Ningún hook (`capture`, `detect`, `precheck`, `turn`) menciona
      skills. `.claude/skills/` vacío contra 11 en `.musubi/skills/`. El ledger de tokens de la
      sesión: 501, **100% `turn_recall`**, skills en cero.

- [x] **T0b — Y eso corrige el orden del track.** «Progressive disclosure» estaba como problema 2 con
      un número fuerte (~2.018 tokens por resolución). Ese costo es **potencial y nadie lo paga**.
      Optimizar un camino con cero tráfico no es prioridad.

- [x] **T0c — Y corrige el valor de F1 (PR #274).** El arreglo del contrato era correcto y era una
      precondición real, pero **arregló una ruta por la que no pasa nadie**. No fue desperdicio; el
      orden estaba mal y sólo esta medición lo mostró.

- [x] **T0d — El formato se copió de algo que YA funciona**, no de la documentación:
      `~/.claude/skills/prompt-optimizer/SKILL.md`. De ahí salió la ruta
      (`.claude/skills/<name>/SKILL.md`) y la convención real de meter el «cuándo» adentro de la
      `description`.

- [x] **T0e — `.claude/` está gitignored.** Lo exportado es estado local derivado; no se versiona
      nada nuevo.

## Lo construido

- [x] **T1 — `internal/skills/skillmd.go`**: `CuandoUsarla`, `DescripcionParaAgente`, `ASkillMD`,
      `ChecksumSkillMD`, `ChecksumDeSKILLMD`, `SigueIntacto`. Puro y testeable sin disco.
- [x] **T2 — `cmd/musubi/agentskills.go`**: escribe, preserva lo editado y retira huérfanas.
- [x] **T3 — Enganchado en `setup` y `provision`**, al lado de `writeCognitiveSkills`.
- [x] **T4 — Reporte con las tres categorías**: escritas, **preservadas** y **retiradas**. Un export
      que sólo cuenta éxitos esconde el caso que importa.
- [x] **T5 — 10 invariantes y 10 sabotajes**, los 10 en rojo.

      | Sabotaje | Inv. | Resultado |
      |---|---|---|
      | La descripción no lleva el «cuándo» | E1 | rojo |
      | `always_because` le gana a `applies_to` | E2 | rojo |
      | Antepone el «cuándo» aunque ya esté | E3 | rojo |
      | El cuerpo se recorta | E4 | rojo |
      | La ruta no es la que el agente espera | E5 | rojo |
      | Reescribe aunque no haya cambios | E6 | rojo |
      | Pisa lo editado a mano | E7 | rojo |
      | Adopta un SKILL.md ajeno | E8 | rojo |
      | Borra huérfanas editadas a mano | E9 | rojo |
      | El conector no depende de la fuente | E10 | rojo |

      **E7 y E8 comparten mutación a propósito**: son la misma guarda (`SigueIntacto`) con distinto
      input —archivo editado vs archivo ajeno— y no existe una mutación que rompa uno sin el otro.

## ★ El defecto que encontró MIRAR LA SALIDA, no un test

Los 9 invariantes estaban en verde y el export producía esto:

```
plan-ahead          "Usá cuando planificar es una FASE del trabajo…"    ← roto
adversarial-review  "Usá cuando se activa al cerrar un cambio…"          ← roto
sdd-flow            "Usá cuando gobierna el flujo completo…"             ← roto
go-hygiene          "Usá cuando escribas o revises cualquier .go…"       ← bien
```

**`always_because` es prosa escrita para explicarle a UNA PERSONA por qué la skill lleva `*`**, no
una cláusula que componga después de «Usá cuando». Los tests verificaban que la descripción
*contuviera* el «cuándo» — y lo contenía. Ninguno verificaba que la frase **cerrara**.

Y una descripción rota no es cosmética: en SKILL.md es **lo único** que decide si la skill se
activa.

El arreglo: el conector depende de la fuente. `applies_to` y globs traducen a frases escritas para
seguir a «Usá cuando»; `always_because` va con «Cuándo: …». Quedó como E10, con su sabotaje.

## Medido después del arreglo

Sobre el arsenal real (copia temporal, sin tocar el repo): **11 de 11** con el «cuándo» en la
descripción, y las once leyendo bien.

## Fuera de alcance, dicho de frente

- **No se exportó al repo real todavía.** Escribir en `.claude/skills/` cambia lo que el agente carga
  en cada sesión futura de este proyecto: es decisión del dueño, no un efecto colateral de un PR.
- **Los YAML en disco son viejos.** El `applies_to` de F1 vive en `cognitive.go` y las skills en
  `.musubi/skills/` todavía no se regeneraron, así que hoy caen al fallback de `always_because`. Se
  actualizan solas en el próximo `setup` con un binario nuevo, y ahí las frases mejoran otra vez.
- **No se toca el resolvedor.** Sigue como está para quien lo llame.
- **No hay niveles.** Con un consumidor real andando, el costo se podrá **medir** en vez de
  proyectarlo — que es justamente lo que faltaba para decidirlo bien.
