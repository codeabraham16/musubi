# Tareas — Profundidad derivada del diff

## Hechas

| # | tarea | archivo | estado |
|---|---|---|---|
| 1 | Contadores de líneas agregadas/borradas en el parser | `internal/codeintel/diff.go` | ✅ |
| 2 | La función pura: señales, escalones, niveles, paneles | `internal/codeintel/profundidad.go` | ✅ |
| 3 | El piso de honestidad (`RadioCiego`) y su motivo | `internal/codeintel/profundidad.go` | ✅ |
| 4 | Derivación de las cinco señales del diff | `internal/codeintel/profundidad.go` | ✅ |
| 5 | Banco P1-P9 | `internal/codeintel/profundidad_test.go` | ✅ |
| 6 | 14 sabotajes, cada invariante visto en ROJO | runner externo | ✅ |
| 7 | Cableado al grafo, con la desambiguación del cero | `internal/mcp/methods_detect.go` | ✅ |
| 8 | `revision` en la salida y en el `summary` | `internal/mcp/methods_detect.go` | ✅ |
| 9 | Descripción de la tool + golden regenerado | `internal/mcp/registry.go`, `testdata/` | ✅ |
| 10 | Paso 3 en `adversarial-review` + renumeración a 8 | `cmd/musubi/cognitive.go` | ✅ |
| 11 | CHANGELOG + bump | `CHANGELOG.md`, `VERSION`, `versioninfo.json` | ✅ |

## Verificación corrida

```bash
go build ./... && go vet ./internal/... ./cmd/...
gofmt -l internal/ cmd/                     # vacío
go test -count=1 ./internal/codeintel/ ./internal/mcp/ ./cmd/musubi/
go test ./internal/mcp -run TestToolsListGolden -update   # 1 línea de diff: la descripción
python sabotajes_f2.py                      # 14/14 en ROJO
```

## Lo que queda pendiente, y por qué se declara

- **`minima` va a ser raro en este repo** hasta que alguien indexe el grafo: hay `.go` sin un solo
  nodo, así que el piso de honestidad los eleva a `estandar`. Es el comportamiento correcto, no un
  bug — pero conviene medir cuántos archivos están fuera del índice antes de calibrar los escalones.
- **Los escalones no se calibraron contra el historial de PRs del repo.** Los cortes salen del plan,
  no de una distribución medida. Un barrido sobre los últimos N merges diría si `estandar` cae donde
  tiene que caer o si la mayoría termina en `profunda`.
- **La renumeración deja poco margen para F4.** `Rules` pasó de 3.120 a 3.936 runas sobre un umbral
  de 5.000: quedan **1.064** y F4 proyectaba unas 900. Entra, pero justo — conviene medirlo antes,
  no después.
- **`TestElBucleSeDetieneAlSerRevocado` falla**, y falla igual en `origin/main` limpio (3/3 en las
  dos). Es preexistente y ajeno a esta fase; queda anotado, no arreglado acá.
