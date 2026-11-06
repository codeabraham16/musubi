---
# Las que llaman a tienda.Subtotal directamente, incluida la del otro paquete (informes), que la
# llama calificada. Se mira sólo la línea AFECTADAS, que admite negritas o comillas alrededor.
type: regex
pattern: '^\W*AFECTADAS\W*:(?=[^\n]*\bCobrarPedido\b)(?=[^\n]*\bResumenDePedido\b)(?=[^\n]*\bLineaDeCaja\b)'
flags: m
target: last_message
---
