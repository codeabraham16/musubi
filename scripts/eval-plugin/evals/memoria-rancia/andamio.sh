# shellcheck shell=bash
# Cuerpo del andamio de memoria-rancia. NO corre solo: correr.sh le antepone la cabecera y
# lib/andamio-comun.sh, que definen andamio_* (ver el README de la suite).
declare -F andamio_falla >/dev/null || {
	echo "andamio.sh no corre solo: lo arma correr.sh con lib/andamio-comun.sh" >&2
	exit 1
}

# Primero el código como ERA: la nota de memoria se escribe contra esta versión y queda anclada al
# símbolo, con su huella.
mkdir -p tienda
cat >go.mod <<'FIN'
module ejemplo.test/tienda

go 1.22
FIN
cat >tienda/texto.go <<'FIN'
package tienda

// Saludo arma el saludo que ve el cliente al entrar.
func Saludo(nombre string) string {
	return "Hola, " + nombre + "!"
}
FIN
cat >tienda/despedida.go <<'FIN'
package tienda

// Despedida arma el mensaje que ve el cliente al pagar.
func Despedida(nombre string) string {
	return "Gracias por tu compra, " + nombre + "."
}
FIN
andamio_git init -q .
andamio_commit "tienda: saludo y despedida"

andamio_activar_memoria
andamio_rpc siembra <<'FIN'
{"name":"musubi_save_observation","arguments":{"topic_key":"project/profile","mem_type":"semantic","content":"Tienda es un ejemplo chico en Go: el paquete tienda arma los mensajes que ve el cliente al entrar y al pagar."}}
{"name":"musubi_save_observation","arguments":{"topic_key":"tienda/saludo","mem_type":"semantic","content":"Saludo(nombre) de tienda/texto.go devuelve «Hola, <nombre>!»: por ejemplo, Saludo de Ana da «Hola, Ana!».","origin_paths":["tienda/texto.go#Saludo"]}}
FIN

# Después el código cambia, y la nota queda vieja.
cat >tienda/texto.go <<'FIN'
package tienda

// Saludo arma el saludo que ve el cliente al entrar. Desde la campaña de otoño es más cordial.
func Saludo(nombre string) string {
	return "¡Buen día, " + nombre + "!"
}
FIN
andamio_commit "tienda: saludo más cordial"

andamio_estado_proyecto_conocido

# El plugin tiene que traer la nota Y marcarla como posiblemente rancia: sin la marca, el caso
# mediría otra cosa (una memoria vieja que Musubi no detectó).
andamio_rpc plugin <<'FIN'
{"name":"musubi_recall","arguments":{"query":"qué devuelve Saludo de tienda"}}
FIN
grep -q 'tienda/saludo' "${ANDAMIO_TMP}/salida.jsonl" ||
	andamio_falla "el recall del plugin no trae la nota de Saludo"
grep -q '\\"stale\\":\[' "${ANDAMIO_TMP}/salida.jsonl" ||
	andamio_falla "el recall trae la nota de Saludo sin marcarla rancia: el ancla no detectó el cambio"

andamio_cerrar 'Hola, Ana'
