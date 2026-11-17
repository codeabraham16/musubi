# shellcheck shell=bash
# Cuerpo del andamio de control-normalizar. NO corre solo: correr.sh le antepone la cabecera y
# lib/andamio-comun.sh, que definen andamio_* (ver el README de la suite).
declare -F andamio_falla >/dev/null || {
	echo "andamio.sh no corre solo: lo arma correr.sh con lib/andamio-comun.sh" >&2
	exit 1
}

# CONTROL: la tarea se resuelve leyendo dos archivos. La memoria existe y el arranque la inyecta
# como en cualquier proyecto conocido, pero no dice nada que ayude ni que confunda: si el brazo con
# plugin saca menos que el otro, lo que lo empeoró es lo que Musubi le agrega a la sesión.
mkdir -p texto
cat >go.mod <<'FIN'
module ejemplo.test/texto

go 1.22
FIN
cat >texto/normalizar.go <<'FIN'
package texto

import "strings"

// Normalizar prepara un nombre para compararlo: minúsculas, sin espacios en los bordes y con un
// solo espacio entre palabras.
func Normalizar(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}
FIN
cat >texto/normalizar_test.go <<'FIN'
package texto

import "testing"

func TestNormalizar(t *testing.T) {
	casos := map[string]string{
		"Hola":            "hola",
		"  Hola   Mundo ": "hola mundo",
		"PAN\tDULCE":      "pan dulce",
		"":                "",
	}
	for entrada, esperado := range casos {
		if obtenido := Normalizar(entrada); obtenido != esperado {
			t.Errorf("Normalizar(%q) = %q, se esperaba %q", entrada, obtenido, esperado)
		}
	}
}
FIN
andamio_git init -q .
andamio_commit "texto: Normalizar y su test"

andamio_activar_memoria
andamio_rpc siembra <<'FIN'
{"name":"musubi_save_observation","arguments":{"topic_key":"project/profile","mem_type":"semantic","content":"Texto es una librería chica en Go que prepara los nombres de los clientes de una panadería antes de buscarlos en la agenda."}}
{"name":"musubi_save_observation","arguments":{"topic_key":"texto/dependencias","mem_type":"semantic","content":"La librería Texto usa sólo la biblioteca estándar de Go: se decidió no sumar dependencias externas para que compile en la caja registradora vieja."}}
FIN

andamio_estado_proyecto_conocido

# El arreglo no puede estar ya escrito en el repo (una versión corregida que se coló al editar
# este andamio dejaría el caso sin nada que medir).
andamio_cerrar 'strings.Fields' 'strings.Join'
