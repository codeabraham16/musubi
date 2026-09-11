package main

import "strings"

// escapes.go — LOS ESCAPADORES, EN UN ARCHIVO QUE COMPILA EN TODAS LAS PLATAFORMAS.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// POR QUÉ VIVEN ACÁ Y NO AL LADO DE QUIEN LOS USA
//
// Estas dos funciones interpolan texto QUE ARMA EL CEREBRO adentro de un programa que se va a
// EJECUTAR en la máquina de otra persona. Es la superficie con más consecuencia del repo: una
// comilla sin tapar cierra la cadena y lo que sigue corre como código.
//
// Y hasta hoy no tenían NI UNA PRUEBA, por un motivo puramente mecánico: vivían en
// `avisador_windows.go` y `avisador_darwin.go`, y el sufijo del nombre de archivo es una
// restricción de build por GOOS. En Linux —donde corre el desarrollo y donde corre CI— esas
// funciones NO EXISTEN, así que una prueba para ellas habría tenido que llamarse
// `*_windows_test.go`… que tampoco compila en Linux. Este repo ya pagó esa lección exacta: cuatro
// pruebas en un `_windows_test.go` no compilaban fuera de Windows, `go test` contestaba `ok`, y
// las pruebas simplemente no existían.
//
// Movidas a un archivo sin sufijo, compilan y se prueban en todos lados. El código que las llama
// sigue siendo específico de cada plataforma; la REGLA no tiene por qué serlo.
//
// Y ERAN TRES COPIAS, NO DOS. `install.go` tenía la misma regla de PowerShell escrita a mano
// —`strings.ReplaceAll(installDir, "'", "''")`— para armar el comando que toca el PATH del
// usuario. Es el hermano al que nadie fue a buscar cuando se revisó el primero: la copia que se
// queda vieja es siempre la del camino que se mira menos.
// ────────────────────────────────────────────────────────────────────────────────────────────

// escaparPS tapa las comillas simples para interpolar en una cadena de PowerShell.
//
// En PowerShell una comilla simple se escapa DUPLICÁNDOLA (no con barra invertida: adentro de una
// cadena simple la barra es un carácter común). Sin esto, el texto cierra la cadena y lo que
// sigue corre como código.
//
// El resultado va SIEMPRE adentro de comillas simples; las comillas las pone el llamador.
func escaparPS(s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	// Los saltos de línea se vuelven espacios: el guion viaja en UNA línea de `-Command`, y un
	// salto lo partiría en dos sentencias — o sea que el resto del texto sería un comando.
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.ReplaceAll(s, "\n", " ")
}

// escaparAppleScript tapa las barras y las comillas dobles del texto.
//
// EL ORDEN NO ES INTERCAMBIABLE: la barra se duplica PRIMERO. Al revés, la barra que se agrega
// para escapar la comilla se volvería a escapar y el resultado sería `\\"` — una barra literal
// seguida de una comilla que CIERRA la cadena, que es exactamente el agujero que esto tapa.
func escaparAppleScript(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `"`, `\"`)
}
