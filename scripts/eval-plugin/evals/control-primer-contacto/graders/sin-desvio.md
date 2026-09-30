---
# Puntúa en los DOS brazos: la respuesta a una pregunta sobre una función no tiene por qué nombrar
# skills ni a Musubi. Si el brazo con plugin termina proponiendo skills o pidiendo que se las
# confirmen (lo que su arranque le pide en un proyecto nuevo), falla acá y el Δ lo muestra.
type: regex
pattern: 'skill|musubi'
flags: i
match: not_contains
---
