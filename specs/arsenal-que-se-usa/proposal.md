# Propuesta — El arsenal se usa

Track: **Forja global**, F2. Nace de una medición que reordenó el track entero.

## El arsenal está inerte, y está medido

```
musubi_resolve_skills     0 llamadas en 30 días  (ledger LOCAL y ledger CENTRAL)
hooks configurados        capture · detect · precheck · turn — ninguno menciona skills
.claude/skills/           0 archivos
.musubi/skills/           11 skills
ledger de tokens          501 tokens en la sesión, 100% turn_recall. Skills: cero.
```

Las 11 skills viven en un directorio que **el mecanismo nativo del agente no lee**, ningún hook las
inyecta, y la única tool que las superficiaría **nunca se llamó**. Existen, están validadas, se
federan al central y se ven desde el cuerpo — y **nada las aplica a una sesión de trabajo**.

## Lo que eso corrige del plan

La investigación había puesto «progressive disclosure» como problema 2, con un número fuerte: ~2.018
tokens por resolución. **Ese costo es potencial y nadie lo paga**, porque nadie resuelve. Optimizar
un camino con cero tráfico no es una prioridad: es un adorno.

Y sobre `specs/alcance-declarado/` (F1, PR #274): el arreglo del contrato era correcto y era una
precondición real, pero **arregló una ruta por la que no pasa nadie**. No fue desperdicio; el orden
estaba mal, y sólo esta medición lo mostró.

**El problema 0, que no estaba en la lista: antes de hacer las skills globales, expresivas, por
niveles o medibles, algo tiene que aplicarlas.**

## Qué se construye

Musubi exporta sus skills al formato **SKILL.md** en `.claude/skills/<name>/SKILL.md`, que es el que
el agente **sí** lee.

El formato se copió de una skill que ya funciona en esta máquina (`~/.claude/skills/prompt-optimizer/`),
no de la documentación: frontmatter con `name` y `description`, y la convención real de meter el
**cuándo** adentro de la descripción.

## Por qué exportar y no inyectar por hook

- **Enciende lo que ya existe, hoy.** 11 skills escritas y validadas empiezan a aplicarse sin
  maquinaria nueva en el loop del agente.
- **Es la ambición del track, literal.** «Usables en cualquier repo, cualquier lenguaje, hardware»:
  SKILL.md es portable y anda incluso donde Musubi no esté instalado. Un hook ataría las skills a
  Musubi; exportar las suelta.
- **El costo del hook está al revés.** Inyectar en cada turno convierte los ~2.018 tokens
  potenciales en reales y permanentes, y exigiría hacer los niveles ANTES.
- **No viola el núcleo model-free.** Musubi genera el artefacto de forma determinista; quién decide
  cargarlo es el consumidor, con su propio mecanismo. La inferencia queda **del otro lado de la
  frontera**, que es el único lugar donde la investigación la admitía.

## La continuidad con F1

En SKILL.md el «cuándo usarla» va en la `description`. El `always_because` de las 6 wildcards y el
`applies_to` de F1 son **exactamente** el material para escribirla. El trabajo del slice anterior
alimenta a éste en vez de quedar colgado.

## Lo que NO se construye

- **No se toca el resolvedor ni sus tools.** Siguen existiendo para quien las llame.
- **No hay niveles todavía.** Con el consumidor real andando se podrá medir el costo de verdad, en
  vez de proyectarlo.
- **No se versiona nada nuevo:** `.claude/` está gitignored. Lo exportado es estado local de cada
  máquina, como corresponde a algo derivado.
