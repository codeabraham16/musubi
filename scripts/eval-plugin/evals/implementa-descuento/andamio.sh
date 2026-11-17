# shellcheck shell=bash
# Cuerpo del andamio de implementa-descuento. NO corre solo: correr.sh le antepone la cabecera y
# lib/andamio-comun.sh, que definen andamio_* (ver el README de la suite).
declare -F andamio_falla >/dev/null || {
	echo "andamio.sh no corre solo: lo arma correr.sh con lib/andamio-comun.sh" >&2
	exit 1
}

# Un proyecto que Musubi ya conoce, con el carrito escrito y su test. La tarea es agregar una
# función con sus tests: justo lo que la skill sdd-flow del plugin dice cubrir.
mkdir -p precios
cat >go.mod <<'FIN'
module ejemplo.test/almacen

go 1.22
FIN
cat >README.md <<'FIN'
# Almacén

Precios y carrito de un almacén de barrio.
FIN
cat >precios/carrito.go <<'FIN'
package precios

// Linea es un renglón del carrito: qué se lleva, a cuánto y cuántos.
type Linea struct {
	Producto       string
	PrecioCentavos int
	Cantidad       int
}

// Carrito es la compra en curso.
type Carrito struct {
	Lineas []Linea
}

// Total suma las líneas del carrito, en centavos.
func (c Carrito) Total() int {
	total := 0
	for _, l := range c.Lineas {
		total += l.PrecioCentavos * l.Cantidad
	}
	return total
}
FIN
cat >precios/carrito_test.go <<'FIN'
package precios

import "testing"

func TestTotal(t *testing.T) {
	c := Carrito{Lineas: []Linea{
		{Producto: "pan", PrecioCentavos: 150, Cantidad: 2},
		{Producto: "leche", PrecioCentavos: 320, Cantidad: 1},
	}}
	if got := c.Total(); got != 620 {
		t.Fatalf("Total() = %d, se esperaba 620", got)
	}
}
FIN
andamio_git init -q .
andamio_commit "precios: carrito y total"

andamio_activar_memoria
andamio_rpc siembra <<'FIN'
{"name":"musubi_save_observation","arguments":{"topic_key":"project/profile","mem_type":"semantic","content":"Almacén es la librería de precios de un almacén de barrio, en Go. Los importes van en centavos, como int: nunca float64, para no perder centavos al redondear."}}
{"name":"musubi_save_observation","arguments":{"topic_key":"almacen/convenciones","mem_type":"semantic","content":"En Almacén los tests son de tabla (un slice de casos y un for con range) y los errores de validación se arman con fmt.Errorf, en minúscula y sin punto final."}}
FIN

andamio_estado_proyecto_conocido

# Lo que el agente tiene que escribir no puede estar ya en el repo.
andamio_cerrar 'TotalConCupon' 'TopeCentavos'
