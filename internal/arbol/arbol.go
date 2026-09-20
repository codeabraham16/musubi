// Package arbol contesta UNA pregunta, y por eso existe: qué archivos contiene el repo.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// SE LE PREGUNTA A GIT Y NO AL DISCO, Y ESO NO ES ESTILO: ES DÓNDE EMPIEZA Y TERMINA «EL REPO»
//
// La forma que este paquete reemplaza es `filepath.WalkDir` sobre la raíz con una lista de
// carpetas a saltear escrita a mano (`.git`, `vendor`, `node_modules`, `.claude`, `dist`). Esa
// lista ERA el defecto, por dos razones que se suman:
//
//  1. UNA LISTA DE EXCLUSIONES A MANO NO CONVERGE. Cada carpeta nueva que aparece en un árbol de
//     trabajo y no es código hay que acordarse de agregarla, y nadie se entera de que faltaba
//     hasta que muerde. La definición de «esto no es del repo» YA EXISTE y se llama `.gitignore`.
//  2. EL VEREDICTO DEPENDÍA DE LA MÁQUINA. Una guarda que barre el disco mide un conjunto
//     distinto en cada checkout: un `.go` sin trackear la pone ROJA sólo en local —el runner hace
//     checkout limpio— y un archivo que quedó en un worktree viejo la puede poner VERDE sobre una
//     cita fantasma. Las dos direcciones se midieron el 2026-09-19 (cabo A128).
//
// LA LECCIÓN ESTABA ESCRITA Y EN UN SOLO LUGAR. Alguien hizo esta conversión para un enumerador
// —en un archivo de prueba de `internal/mcp`— y los otros doce sitios nunca se enteraron, porque
// un helper de prueba no cruza paquetes. Por eso esto es un paquete y no otra copia: es la misma
// razón por la que existe `internal/guiones`.
//
// NO ES CÓDIGO DE PRODUCCIÓN Y NO PRETENDE SERLO: lo usan las guardas. Devuelve `error` en vez de
// tomar un `*testing.T` para que el paquete no dependa de `testing` y cada prueba decida cómo
// morir; el ayudante que llama a `t.Fatalf` vive del lado de quien prueba.
package arbol

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Archivos devuelve, ORDENADAS y relativas a `raiz`, las rutas de todo lo que git trackea y que
// además existe en el árbol de trabajo.
//
// Las dos condiciones hacen falta y no son la misma. «Trackeado» es lo que viaja al clone, que es
// la definición de «del repo»; «existe en el disco» es lo que se puede abrir y leer. Un archivo
// borrado pero todavía en el índice cumple la primera y no la segunda, y una guarda que lo
// intente leer se cae con un error que no tiene nada que ver con lo que está midiendo.
func Archivos(raiz string) ([]string, error) {
	salida, err := exec.Command("git", "-C", raiz, "ls-files", "-z").Output()
	if err != nil {
		return nil, fmt.Errorf("no pude preguntarle a git qué trackea desde %s: %w — no medí nada", raiz, err)
	}
	var out []string
	for _, rel := range strings.Split(string(salida), "\x00") {
		if rel == "" {
			continue
		}
		if _, err := os.Stat(filepath.Join(raiz, filepath.FromSlash(rel))); err != nil {
			continue // trackeado pero ausente del árbol: no hay nada que leer
		}
		out = append(out, rel)
	}
	// «NO PUDE MEDIR» NO PUEDE SALIR POR LA PUERTA DE «MEDÍ Y ESTÁ BIEN». Si git contesta vacío
	// —no es un repo, otra raíz, un checkout roto— toda guarda que cuelgue de acá daría verde sin
	// haber mirado un solo archivo, y ése es el modo de falla más caro que tiene una guarda.
	if len(out) == 0 {
		return nil, fmt.Errorf("git no listó NI UN archivo trackeado desde %s: eso no es «el repo está vacío», es que este enumerador no miró nada", raiz)
	}
	sort.Strings(out)
	return out, nil
}

// ConSufijo acota lo que devuelve Archivos a las rutas que terminan en `suf` (por ejemplo `.go`).
//
// Falla si no queda ninguna, por la misma razón que Archivos: un filtro que se come todo se lee
// igual que un árbol donde no hay nada que revisar, y son cosas opuestas.
func ConSufijo(raiz, suf string) ([]string, error) {
	todos, err := Archivos(raiz)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, rel := range todos {
		if strings.HasSuffix(rel, suf) {
			out = append(out, rel)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("de %d archivo(s) trackeados desde %s no quedó NI UNO que termine en %q — el filtro se comió todo y esta guarda no mediría nada", len(todos), raiz, suf)
	}
	return out, nil
}
