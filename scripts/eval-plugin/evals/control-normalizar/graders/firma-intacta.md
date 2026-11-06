---
# La firma pedida: un string entra y un string sale (el nombre del parámetro puede cambiar).
type: regex
pattern: 'func\s+Normalizar\s*\(\s*\w+\s+string\s*\)\s+string\s*\{'
target:
  source: file
  path: texto/normalizar.go
---
