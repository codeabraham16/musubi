# Tareas — Hallazgos congelados

| # | tarea | estado |
|---|---|---|
| 1 | línea base de `Compute` con hexes obtenidos corriéndolo | ✅ |
| 2 | `internal/receipt/finding.go` | ✅ |
| 3 | banco H0-H8 | ✅ |
| 4 | 11 sabotajes, cada invariante en ROJO | ✅ |
| 5 | `receipt freeze` / `receipt verify` con cuerpo por stdin | ✅ |
| 6 | verificación de punta a punta con binario real (7 casos) | ✅ |
| 7 | CHANGELOG + bump | ✅ |

## Pendiente declarado

- **La skill todavía no ordena congelar.** Este commit entrega el mecanismo y los comandos; cablear
  `adversarial-review` para que congele antes de votar toca el mismo bloque `Rules` que F2, F3 y F4,
  y sumar un cuarto editor del mismo texto multiplicaría el conflicto de merge sin necesidad.
  Va cuando esas ramas estén resueltas.
- **`receiptShow` panica** con una huella corta escrita a mano (`r.Fingerprint[:12]`). Encontrado de
  paso, fuera de alcance.
