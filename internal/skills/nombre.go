package skills

import "regexp"

// nombreSeguroRe es la forma de un nombre de skill que se puede usar como nombre de directorio:
// minúsculas, dígitos y guiones, empezando por letra o dígito. Es la misma regla que ya exigía la
// puerta MCP al guardar una skill; vive acá para que la cumpla también lo que llega por git o a mano.
var nombreSeguroRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// NombreSeguro dice si el nombre de una skill es un slug: sin separadores de ruta, sin `..`, sin
// unidad de disco. Cualquier cosa que se vaya a unir a una ruta con él tiene que preguntarlo antes.
func NombreSeguro(name string) bool {
	return nombreSeguroRe.MatchString(name)
}
