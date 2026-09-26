package memory

import (
	"path/filepath"
	"strings"
	"testing"

	"musubi/internal/fleet/fleettest"
)

// puertasDeLecturaDeComandos devuelve, por clave (`Tipo.Método` o `Función`), las PUERTAS DE
// LECTURA de device_commands: toda función exportada del paquete que llega a escanearComando —la
// única que convierte una fila en un fleet.Comando, y la que normaliza el origen al leer—, con el
// camino por el que llega.
//
// SALE DEL FUENTE Y NO DE UNA LISTA. TestUnOrigenRaroEnLaTablaSeLeeDesconocidoPorCadaPuerta tenía las
// cinco escritas a mano, y la segunda revisión de A131 (tema T9) lo señaló: hoy coincidían con los
// llamadores de escanearComando, pero una sexta —otra consulta de la bitácora, otra forma de
// entregar— habría quedado afuera sin que nada lo dijera. Ahora la prueba pide saber leer cada una de
// las que el grafo encuentra, y deja de aceptar las que ya no llegan.
//
// EL PISO: escanearComando tiene que existir, y el barrido tiene que encontrar al menos las cinco
// que había cuando se escribió esto. Menos es un barrido roto, no un paquete con menos puertas.
//
// LO QUE NO VE, dicho: un lector nuevo que lea `origen` con su PROPIO Scan, sin pasar por
// escanearComando. Eso no es una puerta de este grafo; es una copia del escaneo, y su origen crudo no
// lo mediría nadie. Hoy no hay ninguno: las cuatro consultas que usan columnasComando están en
// funciones que llegan a escanearComando.
func puertasDeLecturaDeComandos(t *testing.T) map[string][]string {
	t.Helper()
	const (
		escaneo = "escanearComando"
		piso    = 5
	)
	g, err := fleettest.LeerGrafo(filepath.Join("..", ".."), "internal/memory")
	if err != nil {
		t.Fatal(err)
	}
	if !g.Existe(escaneo) {
		t.Fatalf("el grafo de internal/memory no tiene a %s: se movió o cambió de nombre, y las puertas de "+
			"lectura de device_commands ya no se pueden derivar", escaneo)
	}
	out := map[string][]string{}
	for _, clave := range g.Claves() {
		if !fleettest.Exportada(clave) {
			continue
		}
		if camino := g.Camino(clave, escaneo); camino != nil {
			out[clave] = camino
		}
	}
	if len(out) < piso {
		var halladas []string
		for k, c := range out {
			halladas = append(halladas, k+" ("+strings.Join(c, " → ")+")")
		}
		t.Fatalf("el barrido encontró %d puertas de lectura de device_commands (%v) y cuando se escribió esta "+
			"prueba eran %d: el grafo está roto, o alguna dejó de pasar por %s", len(out), halladas, piso, escaneo)
	}
	return out
}
