# shellcheck shell=bash
# Cuerpo del andamio de impacto-subtotal. NO corre solo: correr.sh le antepone la cabecera y
# lib/andamio-comun.sh, que definen andamio_* (ver el README de la suite).
declare -F andamio_falla >/dev/null || {
	echo "andamio.sh no corre solo: lo arma correr.sh con lib/andamio-comun.sh" >&2
	exit 1
}

# Un backend de juguete con dos paquetes. La respuesta correcta sale de las llamadas, no de los
# nombres: cada archivo marcado TRAMPA engaña a quien busca «Subtotal» por texto.
mkdir -p tienda informes
cat >go.mod <<'FIN'
module ejemplo.test/negocio

go 1.22
FIN
cat >tienda/item.go <<'FIN'
package tienda

// Item es una línea del pedido. Los importes van en centavos.
type Item struct {
	Nombre   string
	Precio   int
	Cantidad int
}

// Carrito agrupa los ítems de una compra.
type Carrito struct {
	Items []Item
}
FIN
cat >tienda/calculo.go <<'FIN'
package tienda

// Subtotal suma precio por cantidad de cada ítem.
func Subtotal(items []Item) int {
	total := 0
	for _, it := range items {
		total += it.Precio * it.Cantidad
	}
	return total
}

// Subtotal devuelve lo que suman los ítems del carrito.
func (c Carrito) Subtotal() int {
	n := 0
	for _, it := range c.Items {
		n += it.Precio * it.Cantidad
	}
	return n
}
FIN
cat >tienda/resumen.go <<'FIN'
package tienda

import "fmt"

// ResumenDePedido arma la línea de resumen que ve el cliente.
func ResumenDePedido(items []Item) string {
	return fmt.Sprintf("%d ítems, total %d", len(items), Subtotal(items))
}
FIN
cat >tienda/cobro.go <<'FIN'
package tienda

import "errors"

// ErrSinSaldo se devuelve cuando el saldo no alcanza.
var ErrSinSaldo = errors.New("sin saldo")

// CobrarPedido descuenta el importe del pedido del saldo.
func CobrarPedido(saldo int, items []Item) (int, error) {
	monto := Subtotal(items)
	if monto > saldo {
		return saldo, ErrSinSaldo
	}
	return saldo - monto, nil
}

// Reintentar vuelve a cobrar hasta n veces.
func Reintentar(saldo int, items []Item, n int) (int, error) {
	var err error
	for i := 0; i < n; i++ {
		saldo, err = CobrarPedido(saldo, items)
		if err == nil {
			return saldo, nil
		}
	}
	return saldo, err
}
FIN
# TRAMPA: ImprimirCarrito llama al MÉTODO Carrito.Subtotal, no a la función.
cat >tienda/carrito.go <<'FIN'
package tienda

import "strconv"

// ProcesarCarrito devuelve el resumen del carrito.
func ProcesarCarrito(c Carrito) string {
	return ResumenDePedido(c.Items)
}

// ImprimirCarrito muestra el total del carrito.
func ImprimirCarrito(c Carrito) string {
	return "Total: " + strconv.Itoa(c.Subtotal())
}
FIN
cat >tienda/atencion.go <<'FIN'
package tienda

// AtenderCliente procesa el carrito de un cliente.
func AtenderCliente(nombre string, c Carrito) string {
	return nombre + ": " + ProcesarCarrito(c)
}
FIN
# TRAMPA: se llama parecido y no la llama.
cat >tienda/envio.go <<'FIN'
package tienda

// SubtotalConEnvio suma el costo de envío al importe de los ítems.
func SubtotalConEnvio(items []Item, envio int) int {
	suma := envio
	for _, it := range items {
		suma += it.Precio * it.Cantidad
	}
	return suma
}
FIN
# TRAMPA: nombra «Subtotal» en un string y en un comentario.
cat >tienda/informe.go <<'FIN'
package tienda

// EtiquetaDeColumna es el encabezado de la columna de importes del informe.
// Antes el informe llamaba a Subtotal por cada fila; ahora recibe los importes ya calculados.
func EtiquetaDeColumna() string {
	return "Subtotal"
}
FIN
# TRAMPA: informes.Subtotal es homónima; TotalDelInforme llama a ESA. LineaDeCaja sí llama a
# tienda.Subtotal, calificada, y CerrarCaja a LineaDeCaja.
cat >informes/caja.go <<'FIN'
package informes

import (
	"strconv"

	"ejemplo.test/negocio/tienda"
)

// Subtotal suma los importes de las filas del informe.
func Subtotal(filas []int) int {
	s := 0
	for _, f := range filas {
		s += f
	}
	return s
}

// TotalDelInforme es la última fila del informe.
func TotalDelInforme(filas []int) string {
	return "Total: " + strconv.Itoa(Subtotal(filas))
}

// LineaDeCaja es la línea que imprime la caja por cada venta.
func LineaDeCaja(items []tienda.Item) string {
	return "Caja: " + strconv.Itoa(tienda.Subtotal(items))
}

// CerrarCaja arma las líneas de todas las ventas del día.
func CerrarCaja(ventas [][]tienda.Item) []string {
	lineas := make([]string, 0, len(ventas))
	for _, v := range ventas {
		lineas = append(lineas, LineaDeCaja(v))
	}
	return lineas
}
FIN
andamio_git init -q .
andamio_commit "negocio: tienda e informes"

# La memoria de un proyecto que Musubi ya conoce: su perfil y el grafo de código indexado, como
# queda después de la primera sesión (el daemon lo indexa al arrancar y cada hora).
andamio_activar_memoria
andamio_rpc siembra <<'FIN'
{"name":"musubi_save_observation","arguments":{"topic_key":"project/profile","mem_type":"semantic","content":"Negocio es el backend de una tienda, en Go. El paquete tienda tiene los pedidos y el cobro; el paquete informes arma la caja del día. Los importes van en centavos."}}
{"name":"musubi_codegraph_index","arguments":{"mode":"full"}}
FIN

andamio_estado_proyecto_conocido

# El plugin, arrancado como en la corrida, tiene que encontrar el símbolo en el grafo. No se exige
# la lista exacta: si el grafo se equivoca, eso es lo que el caso tiene que medir, no ocultar.
andamio_rpc plugin <<'FIN'
{"name":"musubi_impact","arguments":{"symbol":"tienda/calculo.go#func:Subtotal"}}
FIN
grep -q '\\"found\\":true' "${ANDAMIO_TMP}/salida.jsonl" ||
	andamio_falla "el grafo del plugin no tiene tienda/calculo.go#func:Subtotal: ¿no se indexó?"

andamio_cerrar
