package fleet

import "testing"

// LOS ESTADOS QUE RECORREN LAS OTRAS GUARDAS SON EL ENUM ENTERO.
//
// fleet.EstadosDeComando es la lista que recorren las guardas de internal/memory (qué deja
// escanearComando de una fila, para cada estado) y de internal/mcp (qué muestran la bitácora y la
// cronología de cada fila). Hasta A131 (tema T10) cada una clavaba el estado que miraba: la de las dos
// superficies probaba sólo `expirado`, y una cronología que derivaba `expirado` y nunca `perdido`
// pasaba en verde (C4-m5). Acá la lista se cierra contra el bloque const que la define: un estado
// nuevo que no esté en la lista no lo recorre nadie, y uno que la lista invente no existe.
//
// EXPOSICIÓN: la lista es nueva; lo que protege es que las guardas de las superficies no vuelvan a
// clavar un estado. Hoy el enum tiene cinco y la lista los cinco.
//
// Sabotaje: que la lista olvide `perdido` → las guardas de memory y mcp dejan de exigir el estado
// derivado que C4-m5 escondía.
// arnes: archivo="internal/fleet/estados_de_comando.go"
// arnes: de="EstadoExpirado, EstadoPerdido}"
// arnes: a="EstadoExpirado}"
// Sabotaje: declarar un estado nuevo sin sumarlo a la lista → ninguna superficie lo recorre.
// arnes: archivo="internal/fleet/comando.go"
// arnes: de="\tEstadoPerdido EstadoComando = \"perdido\"\n)"
// arnes: a="\tEstadoPerdido EstadoComando = \"perdido\"\n\tEstadoCancelado EstadoComando = \"cancelado\"\n)"
func TestLosEstadosDeComandoSonElEnumEntero(t *testing.T) {
	declarados := constantesDeclaradas(t, "EstadoComando")
	// EL PISO: si el barrido no encontrara nada, la comparación de abajo no mediría nada.
	if len(declarados) < 5 {
		t.Fatalf("el barrido encontró %d constantes EstadoComando (%v); el enum tenía 5 cuando se "+
			"escribió esta prueba: el barrido está roto", len(declarados), declarados)
	}
	enLista := map[string]bool{}
	for _, e := range EstadosDeComando {
		if enLista[string(e)] {
			t.Errorf("EstadosDeComando repite %q", e)
		}
		enLista[string(e)] = true
	}
	for valor, nombre := range declarados {
		if !enLista[valor] {
			t.Errorf("%s (%q) está declarado y falta en EstadosDeComando: las guardas de memory (qué se lee "+
				"de la fila) y de mcp (qué muestra cada superficie) no lo recorren, y una superficie que lo "+
				"dibujara mal quedaría en verde", nombre, valor)
		}
	}
	for valor := range enLista {
		if _, ok := declarados[valor]; !ok {
			t.Errorf("EstadosDeComando tiene %q y ninguna constante del paquete lo declara", valor)
		}
	}
}
