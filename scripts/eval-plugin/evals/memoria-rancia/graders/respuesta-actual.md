---
# Pasa si la línea RESPUESTA trae lo que devuelve el código de hoy y no lo de la nota vieja. Mencionar
# la nota vieja en el resto de la respuesta está bien (hasta es lo ideal): se mira sólo esa línea.
type: regex
pattern: '^\W*RESPUESTA\W*:(?![^\n]*Hola,\s*Ana)[^\n]*Buen\s+d[ií]a,\s*Ana'
flags: m
target: last_message
---
