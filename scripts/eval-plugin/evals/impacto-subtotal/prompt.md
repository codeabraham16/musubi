---
description: >-
  Radio de impacto en un repo de juguete con trampas para quien busca por texto: un método con el
  mismo nombre, una función homónima en otro paquete, un nombre que contiene «Subtotal» y el string
  "Subtotal". Mide si el grafo de código del plugin (musubi_impact y el hook de lectura) da una
  lista más correcta que Grep + lectura.
expected_outcome: >-
  AFECTADAS: CobrarPedido, ResumenDePedido, LineaDeCaja (directas) y Reintentar, ProcesarCarrito,
  AtenderCliente, CerrarCaja (indirectas). Sin ImprimirCarrito, SubtotalConEnvio,
  EtiquetaDeColumna ni TotalDelInforme.
tags: [codigo]
max_turns: 25
timeout_seconds: 420
allowed_tools: [Read, Glob, Grep, Skill]
---

Voy a cambiar la firma de `Subtotal` en `tienda/calculo.go` (la función suelta, no el método de `Carrito`): va a recibir además un porcentaje de descuento. Antes de tocar nada, decime qué funciones del repositorio quedan afectadas, directa o indirectamente: las que la llaman, las que llaman a esas, y así hasta arriba. No cambies ningún archivo.

Terminá tu respuesta con una línea que empiece con `AFECTADAS:` y liste sólo los nombres de esas funciones, separados por comas.
