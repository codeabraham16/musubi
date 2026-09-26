// Package fleettest les da a las pruebas de internal/fleet y de internal/mcp UNA clasificación de
// las formas en que un nombre buscado —el selector de una máquina, o el `service:` de una política—
// se le parece a un nombre del registro sin ser él (A131·T3).
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// POR QUÉ ES UN PAQUETE Y NO UNA LISTA EN CADA TABLA
//
// Cuatro tablas recorren esas formas, dos de máquinas y dos de servicios. La del dominio,
// TestUnSelectorAlcanzaSoloAlComodinOAlNombreExacto, mide SelectorAlcanza, SelectorNombra,
// EntradaDeAllowlist y Politica.Alcanza, y exigía cada forma en un PISO. La de los consumidores,
// TestUnaPoliticaActuaYFiguraSoloSobreLasMaquinasQueNombra, mide el barrido, el inventario, las
// compuertas de las concesiones y de los comandos y los informes del rename, y tenía su lista de filas
// escrita a mano, sin piso: le faltaba el GLOB. Medido en la revisión de T3 sobre la punta de la
// rama: con una regla glob agregada a tieneGrant (`davan*` alcanzando a `davantis` y a
// `davantis-1`), el paquete internal/mcp ENTERO quedaba en verde (ok 114,6 s); con la misma regla
// en el barrido de aplicarPoliticas, también (ok 83,4 s). Una concesión o una política actuando
// sobre máquinas que no nombran, y nada rojo.
//
// Y las dos de servicios —TestUnaPoliticaDeServicioEncuentraSuServicioPorElNombreExacto en el
// dominio, TestUnaPoliticaDeServicioSoloMiraElServicioQueNombra en el barrido— tenían cada una la
// suya, también escrita a mano y sin glob, hasta la revisión 2 de T3: una regla glob o la
// normalización de `.service` en ServicioEn dejaban verdes a los dos paquetes. Ahora las cuatro
// clasifican con DeParecido y exigen Formas(); las de servicios, en los dos sentidos del par.
//
// Un _test.go no se importa desde otro paquete, así que la clasificación que comparten vive acá,
// como internal/memory/memtest. Dos copias de «qué es un parecido» se separan como se separaron las
// dos listas de filas: la que se queda corta no avisa.
//
// NO IMPORTA internal/fleet, a propósito: las pruebas internas de ese paquete importan éste, y el
// ciclo no compilaría.
package fleettest

import (
	"path"
	"strings"
)

// Forma es una manera en que un selector de máquina se le parece a un nombre sin ser él. Cada una
// se define por la comparación FLOJA que confundiría al par: la que un consumidor podría escribir
// en lugar de la gramática, y que una tabla sin un par de esa forma dejaría pasar en verde.
type Forma string

// Las formas. El orden de la clasificación es el de la lista `formas`, más abajo.
const (
	// Prefijo es el selector como prefijo del nombre (`davantis` frente a `davantis-1`).
	Prefijo Forma = "prefijo"
	// NombrePrefijo es la comparación al revés: el nombre como prefijo del selector (`davantis-1`
	// frente a `davantis`).
	NombrePrefijo Forma = "el nombre como prefijo del selector"
	// Sufijo es el selector como sufijo del nombre (`antis` frente a `davantis`).
	Sufijo Forma = "sufijo"
	// Subcadena es el selector adentro del nombre sin tocar ninguno de sus bordes (`vant`).
	Subcadena Forma = "subcadena"
	// Mayusculas es el mismo nombre sin distinguir mayúsculas (`DAVANTIS`).
	Mayusculas Forma = "mayúsculas"
	// Glob es el selector que, leído como patrón, calza con el nombre (`davan*`).
	Glob Forma = "glob"
)

// formas es la clasificación EN ORDEN: un par se lleva la PRIMERA forma que lo describe. `davan`
// frente a `davantis` es un prefijo antes que una subcadena, así que una forma cuenta en un PISO
// sólo por los pares que ninguna anterior explica. Agregar una forma acá la vuelve obligatoria en
// cada tabla que exige Formas().
var formas = []struct {
	forma Forma
	laxa  func(selector, nombre string) bool
}{
	{Prefijo, func(s, n string) bool { return strings.HasPrefix(n, s) }},
	{NombrePrefijo, func(s, n string) bool { return strings.HasPrefix(s, n) }},
	{Sufijo, func(s, n string) bool { return strings.HasSuffix(n, s) }},
	{Subcadena, func(s, n string) bool { return strings.Contains(n, s) }},
	{Mayusculas, strings.EqualFold},
	// Un patrón que NO calza no es un parecido: la regla glob tampoco lo confundiría, y contarlo le
	// daría al PISO una fila que no pone rojo nada.
	{Glob, func(s, n string) bool {
		calza, err := path.Match(s, n)
		return err == nil && calza
	}},
}

// Formas devuelve todas las formas, en el orden de la clasificación. Un PISO que las exige todas se
// entera solo de una forma nueva.
func Formas() []Forma {
	out := make([]Forma, 0, len(formas))
	for _, f := range formas {
		out = append(out, f.forma)
	}
	return out
}

// DeParecido dice con qué forma se le parece un selector a un nombre de máquina, o "" si no se le
// parece en ninguna. Los bordes del selector se recortan, como los recorta la gramática: ` davantis `
// ES `davantis`, no un parecido; y un selector vacío, o una máquina sin nombre, no se parecen a nada.
//
// Sólo tiene sentido para un par que el selector NO alcanza. El comodín alcanza a todas —y como
// patrón calzaría con cualquiera—, así que quien llama saltea los pares que su fila alcanza.
//
// Para un servicio, el «selector» es el `service:` de la política y el «nombre», el que reporta la
// máquina; las tablas de servicios lo piden además al revés —el reportado como selector del
// buscado—, porque una búsqueda floja puede comparar en cualquiera de los dos sentidos.
func DeParecido(selector, nombre string) Forma {
	s := strings.TrimSpace(selector)
	if s == "" || nombre == "" || s == nombre {
		return ""
	}
	for _, f := range formas {
		if f.laxa(s, nombre) {
			return f.forma
		}
	}
	return ""
}
