# Spec — El banco mide a escala real

Contrato observable. Cada invariante tiene una prueba que **sabe fallar**.

```go
func FixtureDesdeDB(rutaDB string, opts OpcionesFixtureReal) (*Fixture, error)
func ConsultaDesdeTopico(topic string) string
```

---

## H1 · Las etiquetas no salen del ranker

### K1 — La relevancia se deriva del `topic_key`

Una consulta sobre `tema/alfa` tiene como relevantes **exactamente** las observaciones de ese topic.
Es lo único que hace creíble a un fixture automático: el topic lo puso el autor al escribir, así que
no puede estar contaminado por el ranking que se está midiendo.

### K7 — La consulta se deriva del topic con una transformación tonta

`roadmap/track-potencia-medida` ⇒ `roadmap track potencia medida`. Nada de elegir palabras del
contenido ni pedirle la consulta a un LLM: eso metería en la pregunta información derivada de lo que
se está midiendo.

---

## H2 · El fixture no se infla solo

### K2 — Los cajones de sastre no son consultas

`git-commit` tiene **247** observaciones en la memoria real de este proyecto. No es un tema sobre el
que alguien pregunte, y una consulta con 247 relevantes sobre 1.210 docs no mide nada. Se excluye
por **prefijo** (es mecánico) y además por **tamaño**, para el cajón que aparezca mañana con otro
nombre.

### K3 — Los topics chicos tampoco

Con menos de 3 observaciones, las métricas de orden casi no informan. Y hay **834 topics distintos**
en la memoria real: sin este filtro el fixture sería casi todo ruido de una nota.

### K4 — Lo excluido sigue siendo corpus

Las observaciones de un topic que no llegó a consulta **siguen siendo docs**. Son distractores
legítimos: sacarlas dejaría a cada consulta compitiendo contra un corpus recortado a su medida, que
es una forma silenciosa de inflar el resultado.

---

## H3 · Generar es inocuo

### K5 — Determinista

Dos generaciones de la misma base dan el mismo fixture byte a byte. Un fixture que cambia solo
convierte cualquier comparación entre corridas en ruido.

### K6 — No toca la base

Esto lee la memoria de trabajo de alguien. Se verifica el **efecto**, no la intención: el archivo
queda byte-idéntico después de generar.

---

## Alcance declarado

- **Nada se escribe al repo.** El repo es público y la memoria real tiene infraestructura interna
  adentro. El fixture vive en memoria mientras dura la corrida: no hay archivo que commitear por
  descuido.
- **La medición se saltea sin `MUSUBI_FIXTURE_DB`.** CI no tiene —ni debe tener— memoria real.
- **Los absolutos están subestimados** por el etiquetado por topic (lo relevante que vive en otro
  topic cuenta como fallo). Lo comparable es el delta entre arms; el log de la medición lo dice cada
  vez que corre, para que nadie cite un R@10 fuera de contexto.
- **El fixture dorado no se toca.** Sigue siendo la red de regresión de CI.
