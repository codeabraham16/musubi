---
# Recortar corta por runas y agrega «…» (un solo carácter). Se acepta también «...» escrito a mano:
# el caso mide si el agente contesta, no su tipografía.
type: regex
pattern: '^\W*RESULTADO\W*:[^\n]*\bpana(?:…|\.\.\.)'
flags: m
---
