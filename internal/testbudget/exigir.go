package testbudget

import (
	"flag"
	"fmt"
	"time"
)

// TimeoutDeEstaCorrida devuelve el `-timeout` con el que arrancó este binario de test, y si se
// pudo leer.
//
// NO DEVUELVE 0 CUANDO NO SABE. `-timeout 0` es un valor LEGÍTIMO —significa «sin límite»— así
// que un cero no puede además significar «no encontré el flag»: son dos cosas opuestas (la más
// permisiva posible y la ignorancia total). Por eso el segundo retorno.
func TimeoutDeEstaCorrida() (time.Duration, bool) {
	f := flag.Lookup("test.timeout")
	if f == nil {
		return 0, false
	}
	g, ok := f.Value.(flag.Getter)
	if !ok {
		return 0, false
	}
	d, ok := g.Get().(time.Duration)
	if !ok {
		return 0, false
	}
	return d, true
}

// ExigirTimeoutSuficiente devuelve un motivo (no vacío) cuando este paquete está corriendo bajo
// `-race` con un `-timeout` MENOR que el techo que declara la política del repo.
//
// PARA QUÉ. `go test -race ./...` a secas usa el default de Go —10 min POR PAQUETE— y los dos
// paquetes caros de este repo ya están arriba de la mitad de eso. Cuando no alcanza, lo que se
// ve es «panic: test timed out after 10m0s» con el stack del test que le tocó estar corriendo:
// un mensaje que acusa a un test inocente, diez minutos después, y que no nombra el flag. Media
// docena de personas leyeron eso como un deadlock antes de que alguien midiera.
//
// Con esto, la misma corrida falla en el primer segundo diciendo QUÉ flag falta. Es la única
// forma que hay en Go de que «el que corre reciba el timeout solo»: `go test` no lee ninguna
// configuración del módulo, así que no se puede fijar un default; lo que sí se puede es que el
// error lo diga en vez de dejarlo adivinar.
//
// dir es desde dónde buscar la raíz del repo (normalmente "."). Si la política no se puede leer
// eso TAMBIÉN es un motivo: un guard que se calla cuando no puede mirar es el guard que este
// repo ya pagó dos veces.
func ExigirTimeoutSuficiente(dir string) string {
	d, leido := TimeoutDeEstaCorrida()
	return decidirTimeout(BajoDetector, d, leido, func() (Politica, error) {
		return CargarPoliticaDelRepo(dir)
	})
}

// decidirTimeout es la DECISIÓN, separada de dónde salen los datos.
//
// Está separada a propósito y no por gusto: `BajoDetector` es una constante de build tag, así
// que una prueba corriendo sin `-race` no podría ejercitar NUNCA la rama que importa —quedaría
// verde sin haber mirado nada, que es la clase de guarda que este repo ya juntó siete veces—.
// Con el detector como PARÁMETRO, los dos lados se prueban en cualquier corrida de Linux.
func decidirTimeout(bajoDetector bool, timeout time.Duration, timeoutLeido bool, cargar func() (Politica, error)) string {
	if !bajoDetector {
		// Sin el detector estos paquetes entran holgados en el default de Go; exigir el techo
		// grande acá volvería rojas corridas que hoy pasan sanas (el job `test-cross` y el paso
		// de treesitter corren sin -race).
		return ""
	}
	pol, err := cargar()
	if err != nil {
		return fmt.Sprintf("no se pudo leer la política de presupuesto: %v", err)
	}
	if !timeoutLeido {
		return "no se pudo leer el flag -test.timeout de esta corrida: el guard del presupuesto " +
			"no puede decir si el techo alcanza, y callarse sería darlo por bueno"
	}
	if timeout == 0 {
		return "" // -timeout 0 = sin límite. Alcanza por definición.
	}
	if timeout >= pol.Timeout {
		return ""
	}
	return fmt.Sprintf(
		"este paquete corre bajo -race con -timeout %s, y la política del repo (%s) pide %s.\n"+
			"    El default de Go son 10m POR PAQUETE y este paquete ya no entra ahí: la corrida\n"+
			"    terminaría en «panic: test timed out» diez minutos después, culpando al test que\n"+
			"    justo estuviera corriendo. Corré:\n"+
			"        go test -race -timeout %s ./...\n"+
			"    (o -timeout 0 para sin límite).",
		timeout, NombreArchivoPolitica, pol.Timeout, pol.Timeout)
}
