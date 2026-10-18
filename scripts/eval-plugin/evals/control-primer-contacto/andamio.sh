# shellcheck shell=bash
# Cuerpo del andamio de control-primer-contacto. NO corre solo: correr.sh le antepone la cabecera y
# lib/andamio-comun.sh, que definen andamio_* (ver el README de la suite).
declare -F andamio_falla >/dev/null || {
	echo "andamio.sh no corre solo: lo arma correr.sh con lib/andamio-comun.sh" >&2
	exit 1
}

# Un proyecto que Musubi ve por primera vez con memoria: como en la SEGUNDA sesión de un repo real
# (en la primera el plugin recién crea .musubi/ y sus hooks no inyectan nada). La memoria queda
# creada pero vacía: sin perfil y sin haber ofrecido todavía las skills.
mkdir -p texto
cat >go.mod <<'FIN'
module ejemplo.test/texto

go 1.22
FIN
cat >README.md <<'FIN'
# Texto

Funciones chicas para mostrar nombres de productos en la pantalla de la caja.
FIN
cat >texto/recortar.go <<'FIN'
package texto

// Recortar deja los primeros n caracteres de s. Si tuvo que cortar, agrega una elipsis para que se
// note en la pantalla de la caja.
func Recortar(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
FIN
andamio_git init -q .
andamio_commit "texto: Recortar"

andamio_activar_memoria

# Lo que el caso mide es ese arranque: si las marcas dejaran de detectarlo, fallar acá.
andamio_verificar_primer_contacto

# La respuesta no puede estar escrita en el repo (un test con el valor esperado la regalaría).
andamio_cerrar 'pana…'
