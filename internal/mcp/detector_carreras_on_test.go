//go:build race

package mcp

// corriendoBajoDetector le dice a las pruebas si están corriendo con `-race`.
//
// EXISTE POR UN PLAZO Y NO POR UNA CARRERA (A53). `TestPushDelPorteDeProduccionCruzaEntero`
// federa un grafo de 14.000 nodos —5,2 MB crudos, a propósito por encima de `maxRequestBody`—
// con plazos de 60 s. Bajo el detector, comprimir y serializar eso tarda más de 90 s y el test
// muere con `context deadline exceeded`. NO es una carrera: la corrida instrumentada no reporta
// un solo `DATA RACE`.
//
// LA ALTERNATIVA ERA ACHICAR EL GRAFO Y NO SIRVE: el test existe para cruzar el tope de 4 MiB,
// así que su tamaño tiene un piso. Bajarlo a apenas por encima del tope recortaría ~20 % del
// trabajo y seguiría pasándose de los 60 s.
//
// Lo que se escala es el PLAZO, no lo que se prueba: el test no mide latencia, mide que un grafo
// del porte del de producción cruce entero.
//
// Se hace con build tags porque Go no expone «estoy instrumentado» en runtime, y los dos archivos
// son `_test.go` para que la etiqueta no toque el binario de producción.
const corriendoBajoDetector = true
