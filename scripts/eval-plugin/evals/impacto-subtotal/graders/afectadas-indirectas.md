---
# El cierre transitivo: Reintentar (por CobrarPedido), ProcesarCarrito y AtenderCliente (por
# ResumenDePedido) y CerrarCaja (por LineaDeCaja, en el otro paquete).
type: regex
pattern: '^\W*AFECTADAS\W*:(?=[^\n]*\bReintentar\b)(?=[^\n]*\bProcesarCarrito\b)(?=[^\n]*\bAtenderCliente\b)(?=[^\n]*\bCerrarCaja\b)'
flags: m
target: last_message
---
