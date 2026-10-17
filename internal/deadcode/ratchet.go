// Package deadcode es el núcleo del trinquete de código muerto (A2): normaliza la salida de
// golang.org/x/tools/cmd/deadcode y la compara contra un baseline commiteado.
//
// PARA QUÉ. Musubi acumula un patrón de error caro y repetido: código que se DEFINE y no se
// CONECTA —una guarda registrada con cero callers reales, un grafo sin lectores, una pieza que se
// construye y no se enciende—. Hoy se descubre a ojo y tarde. deadcode lo encuentra por
// alcanzabilidad whole-program (RTA), que es justo donde el grafo AST propio tiene puntos ciegos
// (llamadas por interfaz/indirectas). Este paquete convierte esa salida en un rojo DETERMINISTA que
// nombra el símbolo exacto, con forma de trinquete: la deuda conocida se grandfatherea en un
// baseline, una función NUEVA inalcanzable falla el gate, y una que se conecta o se borra sólo avisa.
//
// POR QUÉ UN NÚCLEO PURO. Normalize/Compare/SanityBaseline no tocan disco ni el entorno: toman
// texto y devuelven texto. Toda la decisión del gate es testeable sin un proceso-helper y sin correr
// el analizador. El I/O (leer la salida, leer/escribir el baseline, los códigos de salida) vive en
// cmd/ratchet, fino sobre esto.
package deadcode

import (
	"bufio"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// lineaInalcanzable matchea una línea de salida de `deadcode`:
//
//	ruta:línea:col: unreachable func: SÍMBOLO
//
// La ruta es greedy a propósito: el `:línea:col:` siempre está pegado al final antes de
// ` unreachable func: `, así que el backtracking la ancla al último, aunque la ruta tuviera `:`.
var lineaInalcanzable = regexp.MustCompile(`^(.+):\d+:\d+: unreachable func: (.+)$`)

// Normalize toma la salida cruda de deadcode y la proyecta a un conjunto ordenado-único de
// identidades `ruta<TAB>símbolo`, SIN línea ni columna, con las barras invertidas de Windows
// pasadas a `/`. Soltar la línea/columna es lo que hace que mover una función dentro de un archivo
// NO churnee el baseline; pasar `\`→`/` es lo que hace que la misma función dé la misma identidad
// se mida en Windows o en Linux.
//
// FALLA-CERRADO. Si UNA línea no vacía no matchea el patrón, devuelve error y NO proyecta nada.
// Una comparación sobre una salida a medias es peor que ninguna: mostraría como «alta» o «baja» lo
// que en realidad no se pudo leer, y el gate perdería su sentido justo cuando el analizador cambió.
func Normalize(raw string) ([]string, error) {
	var ids []string
	sc := bufio.NewScanner(strings.NewReader(raw))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024) // rutas+símbolos largos no deben cortar el scan
	n := 0
	for sc.Scan() {
		n++
		linea := strings.TrimRight(sc.Text(), "\r") // CRLF de Windows
		if strings.TrimSpace(linea) == "" {
			continue
		}
		m := lineaInalcanzable.FindStringSubmatch(linea)
		if m == nil {
			return nil, fmt.Errorf("deadcode: línea %d no reconocida (se esperaba `ruta:línea:col: unreachable func: NOMBRE`): %q", n, linea)
		}
		ruta := strings.ReplaceAll(m[1], `\`, "/")
		ids = append(ids, ruta+"\t"+m[2])
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("deadcode: leyendo la salida: %w", err)
	}
	return ordenarUnico(ids), nil
}

// Compare clasifica el conjunto ACTUAL contra el BASELINE:
//
//	altas = en actual pero NO en baseline → funciones recién muertas; el gate FALLA.
//	bajas = en baseline pero NO en actual → se conectaron o se borraron; sólo aviso.
//
// El trinquete vive en esta asimetría: la deuda sólo puede bajar sin intervención; subirla exige
// regenerar el baseline a mano (y explicar por qué en el commit).
func Compare(baseline, actual []string) (altas, bajas []string) {
	base := make(map[string]bool, len(baseline))
	for _, b := range baseline {
		base[b] = true
	}
	act := make(map[string]bool, len(actual))
	for _, a := range actual {
		act[a] = true
	}
	for _, a := range actual {
		if !base[a] {
			altas = append(altas, a)
		}
	}
	for _, b := range baseline {
		if !act[b] {
			bajas = append(bajas, b)
		}
	}
	return ordenarUnico(altas), ordenarUnico(bajas)
}

// SanityBaseline exige que el baseline commiteado esté ordenado-único por bytes y que cada línea
// tenga forma `ruta<TAB>símbolo`. No se confía en un archivo editado a mano: si está desordenado,
// duplicado o deforme, el diff del trinquete sería ruido en vez de señal. El orden por bytes es el
// mismo que produce `LC_ALL=C sort` y que `sort.Strings`, así que un baseline regenerado con
// --update siempre lo cumple.
func SanityBaseline(lines []string) error {
	for i, l := range lines {
		if !strings.Contains(l, "\t") {
			return fmt.Errorf("baseline: la línea %d no tiene forma `ruta<TAB>símbolo`: %q", i+1, l)
		}
		if i > 0 {
			switch {
			case lines[i-1] == l:
				return fmt.Errorf("baseline: línea %d duplicada: %q; regenerá con --update", i+1, l)
			case lines[i-1] > l:
				return fmt.Errorf("baseline: desordenado en la línea %d (%q va después de %q); regenerá con --update", i+1, l, lines[i-1])
			}
		}
	}
	return nil
}

// Lineas parte un texto en líneas, saca el `\r` de un CRLF y descarta las vacías. Es la lectura
// tolerante del baseline y de cualquier texto de líneas; la validación estricta la hace
// SanityBaseline sobre el resultado.
func Lineas(texto string) []string {
	var out []string
	sc := bufio.NewScanner(strings.NewReader(texto))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		l := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}
		out = append(out, l)
	}
	return out
}

// Serializar arma el contenido de un baseline a partir de un conjunto ya normalizado: una identidad
// por línea y un salto final, o vacío si no hay ninguna.
func Serializar(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return strings.Join(ids, "\n") + "\n"
}

// ordenarUnico ordena por bytes (equivalente a LC_ALL=C) y quita duplicados consecutivos.
func ordenarUnico(xs []string) []string {
	if len(xs) == 0 {
		return nil
	}
	sort.Strings(xs)
	out := make([]string, 0, len(xs))
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}
