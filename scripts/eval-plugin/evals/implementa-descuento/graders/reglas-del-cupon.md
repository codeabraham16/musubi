---
type: llm
focus:
  source: file
  path: precios/cupon.go
---

El archivo es Go. `c.Total()` devuelve la suma del carrito en centavos (un `int`). Leé `TotalConCupon` y decidí qué devuelve en cada uno de estos casos:

1. Total 620, `Porcentaje` 10, `TopeCentavos` 0: tiene que devolver 558 y error nil.
2. Total 999, `Porcentaje` 15, `TopeCentavos` 0: el 15 % es 149,85; el descuento se redondea hacia abajo a 149, así que devuelve 850 y error nil.
3. Total 10000, `Porcentaje` 50, `TopeCentavos` 3000: el descuento sería 5000 pero el tope lo deja en 3000, así que devuelve 7000 y error nil.
4. `Porcentaje` 0 o `Porcentaje` 91: devuelve un error distinto de nil.
5. `Porcentaje` 90 y `Porcentaje` 1: NO devuelven error.

PASS si el código da exactamente eso en los cinco casos.

FAIL si alguno da otra cosa (por ejemplo, si redondea el descuento hacia arriba o al más cercano, si aplica el tope cuando `TopeCentavos` es 0, o si rechaza 1 o 90), si usa `float64` de una forma que cambia alguno de esos resultados, o si el archivo no compila a simple vista.
