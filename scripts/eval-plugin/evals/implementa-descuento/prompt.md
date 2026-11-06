---
description: >-
  Implementar una función chica con sus tests en un proyecto que Musubi conoce. Mide dos cosas: si
  el plugin hace que el agente cargue su skill de implementación (sdd-flow), y si con eso el código
  sale mejor, igual o peor que sin plugin, con el mismo tope de turnos.
expected_outcome: >-
  precios/cupon.go con Cupon y Carrito.TotalConCupon según las reglas pedidas, y tests de tabla en
  precios/cupon_test.go.
tags: [skills, implementar]
max_turns: 50
timeout_seconds: 900
allowed_tools: [Read, Glob, Grep, Skill]
---

Agregá cupones de descuento al carrito de `precios`.

En un archivo nuevo, `precios/cupon.go`, definí:

```go
type Cupon struct {
	Codigo       string
	Porcentaje   int // de 1 a 90
	TopeCentavos int // 0 = sin tope
}

func (c Carrito) TotalConCupon(cu Cupon) (int, error)
```

Reglas de `TotalConCupon`:

- El descuento es `Porcentaje` por ciento de `c.Total()`, redondeado hacia abajo a centavos enteros.
- Si `TopeCentavos` es mayor que cero, el descuento no puede pasar de ese tope.
- Si `Porcentaje` está fuera de 1..90, devuelve un error.
- Devuelve el total menos el descuento.

Escribí tests de tabla en `precios/cupon_test.go`. No vas a poder correr comandos: dejá el código escrito.
