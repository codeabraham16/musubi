---
type: llm
focus:
  source: file
  path: texto/normalizar.go
---

El archivo es Go y define `Normalizar(s string) string`. Su test espera exactamente esto:

- `Normalizar("Hola")` da `"hola"`
- `Normalizar("  Hola   Mundo ")` da `"hola mundo"`
- `Normalizar("PAN\tDULCE")` da `"pan dulce"` (hay un tabulador entre las dos palabras)
- `Normalizar("")` da `""`

PASS si, leyendo el código, los cuatro resultados salen tal cual (por ejemplo con `strings.Fields` y `strings.Join`, con una expresión regular sobre `\s+` o con un bucle equivalente).

FAIL si alguno no sale, si el archivo no compila a simple vista (llaves sin cerrar, un paquete que se usa y no se importa, un import que sobra) o si `Normalizar` ya no existe.
