// Package fleettest es código de PRUEBA que comparten las pruebas de varios paquetes de la flota
// (fleet, memory, mcp). No es producción: nada fuera de un `_test.go` lo importa.
//
// Existe por la misma razón que internal/memory/memtest y que internal/arbol: un helper de prueba no
// cruza paquetes, así que dos pruebas que necesitan lo mismo terminan con una copia cada una, y las
// copias divergen solas. La segunda revisión de A131 (tema T9) encontró dos: el corpus de orígenes
// raros (en fleet y en memory) y el recorrido del fuente que dice qué función llega a cuál.
//
// NO IMPORTA internal/fleet, Y NO PUEDE: las pruebas de `fleet` son del paquete `fleet` y lo
// importan, así que depender de él cerraría un ciclo. Por eso lo que devuelve son textos y claves, y
// cada prueba los convierte a sus tipos.
package fleettest

import (
	"sort"
	"strings"
)

// OrigenesParecidos deriva de los valores del enum de orígenes los que se le PARECEN sin serlo
// —en mayúsculas, con un blanco de más atrás o adelante, con una letra de más— y suma dos que no se
// parecen a nada. Son los que un llamador nuevo, una fila escrita a mano o una versión futura
// podrían traer. Vienen ordenados, sin repetir y sin ningún valor del enum.
//
// ES LA ÚNICA FUENTE DEL CORPUS. Hasta la revisión de T9 lo derivaban por su lado la prueba del
// dominio (TestElOrigenEsUnaListaBlancaCerradaContraSuEnum, internal/fleet) y las de las puertas de
// escritura y lectura (internal/memory): daban lo mismo, y nada impedía que una ganara una variante
// que la otra no. TestLosOrigenesParecidosSeDerivanDeCadaValorDelEnum cuida qué variantes salen.
func OrigenesParecidos(enum []string) []string {
	delEnum := map[string]bool{}
	for _, o := range enum {
		delEnum[o] = true
	}
	vistos := map[string]bool{}
	var out []string
	sumar := func(o string) {
		if delEnum[o] || vistos[o] {
			return
		}
		vistos[o] = true
		out = append(out, o)
	}
	for _, s := range enum {
		sumar(strings.ToUpper(s))
		sumar(s + " ")
		sumar(" " + s)
		sumar(s + "x")
	}
	for _, s := range OrigenesSinParecido {
		sumar(s)
	}
	sort.Strings(out)
	return out
}

// OrigenesSinParecido son los dos valores del corpus que no se derivan de ningún valor del enum:
// una categoría que alguien podría inventar (`cron`) y una que no se parece a nada (`robot`).
var OrigenesSinParecido = []string{"cron", "robot"}
