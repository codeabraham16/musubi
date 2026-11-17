---
# Pasa si HAY línea AFECTADAS y no nombra ninguna trampa: ImprimirCarrito llama al MÉTODO
# Carrito.Subtotal; SubtotalConEnvio sólo se llama parecido; EtiquetaDeColumna devuelve el string
# "Subtotal"; TotalDelInforme llama a informes.Subtotal, la homónima del otro paquete. Sin línea
# no pasa: el lookahead negativo va después de exigirla.
type: regex
pattern: '^\W*AFECTADAS\W*:(?![^\n]*\b(?:ImprimirCarrito|SubtotalConEnvio|EtiquetaDeColumna|TotalDelInforme)\b)'
flags: m
target: last_message
---
