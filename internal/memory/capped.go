package memory

// capped.go fija UN invariante para todo el paquete: NINGUNA COLECCIÓN SE RECORTA SIN
// RECIBIR SU TOTAL EN EL MISMO RETORNO.
//
// Por qué existe este archivo y no es sólo una convención: el patrón correcto ya estaba
// escrito una vez —el cap de neuronas de braingraph.go, que devuelve TotalNeurons y
// Truncated— y al copiarlo a mano a los otros tres recortes del render (sinapsis, aristas
// de código, módulos) se copió sólo la mitad: se recortaba y no se contaba. El resultado
// es la peor clase de falla, porque no rompe nada: el consumidor recibe un array corto,
// mide su largo y publica ese número como si fuera el universo. El dashboard llegó a
// mostrar "486 SINAPSIS" al lado de "300/3660 NEURONAS", y esa vecindad le enseña al ojo
// que donde no hay barra el número es total.
//
// Centralizarlo no lo vuelve imposible por sí solo —para eso están los constructores
// posicionales de BrainGraph y CodeGraphViz—, pero saca de circulación la forma de
// recortar que no cuenta: quien llame a capped recibe el total quiera o no.
func capped[T any](items []T, limit int) (out []T, total int, truncated bool) {
	total = len(items)
	if limit > 0 && total > limit {
		return items[:limit], total, true
	}
	return items, total, false
}
