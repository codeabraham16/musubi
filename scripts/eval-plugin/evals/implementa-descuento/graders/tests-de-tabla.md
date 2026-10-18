---
# Tests de tabla: una función de test que recorre casos con range y llama a TotalConCupon.
type: regex
pattern: '^(?=[\s\S]*\bfunc\s+Test\w*\s*\(\s*\w+\s+\*testing\.T\s*\))(?=[\s\S]*\bTotalConCupon\s*\()(?=[\s\S]*\brange\b)'
target:
  source: file
  path: precios/cupon_test.go
---
