---
# La forma pedida: el tipo con sus tres campos y el método con esa firma (los nombres de los
# parámetros pueden cambiar).
type: regex
pattern: '^(?=[\s\S]*\btype\s+Cupon\s+struct\s*\{)(?=[\s\S]*\bCodigo\b)(?=[\s\S]*\bPorcentaje\b)(?=[\s\S]*\bTopeCentavos\b)(?=[\s\S]*\bfunc\s*\(\s*\w+\s+Carrito\s*\)\s*TotalConCupon\s*\(\s*\w+\s+Cupon\s*\)\s*\(\s*int\s*,\s*error\s*\))'
target:
  source: file
  path: precios/cupon.go
---
