package mcp

// escala_diseno.go — LOS NÚMEROS DECIDIDOS.
//
// POR QUÉ EXISTE. Medido el 2026-08-31 contra el central (0.125.0-forma.40e18fe), sobre un pedido
// real («tabla densa de inventario con lotes y alertas de stock bajo», marca altura, target web),
// contando VALORES CONCRETOS —hex, px, ms, rem, %, ch, razones de contraste— por bloque del brief:
//
//	role 337 chars → 0 valores      principles 681 → 0     shape 537 → 0
//	precedence 595 → 0              instructions 320 → 0   emit 428 → 0
//	corpus 2.249 → 0                avoid 2.101 → 1        demand 1.622 → 3
//	brand 4.192 → 28  ← el ÚNICO bloque denso, y sólo porque el tenant escribió los hex a mano
//
//	texto de criterio (sin corpus ni marca): 11.249 chars, 32 valores → UNO CADA 351 CARACTERES
//
// O sea: TODO lo que el motor dice sobre cómo diseñar son adjetivos. Dice «usá el rango entero de la
// escala tipográfica» y no dice 44/27,5/22/17,5/14/11. Dice «el encabezado queda fijo y los números a
// la derecha» y no dice cuánto mide una fila. Cada número —tamaños, interlineado, tracking, ritmo de
// espaciado, alto de fila, duraciones, contraste— lo terminaba inventando quien compone, y ESAS son
// justo las decisiones que separan una pantalla buena de una mediocre.
//
// El pedido del usuario fue explícito: el motor tiene que TOMAR esas decisiones. Hasta acá delegaba
// todas. Este bloque es el que las toma.
//
// SIGUE SIENDO MODEL-FREE: es una tabla y una escala geométrica calculada, no un juicio. El registro
// sale de la forma, y la forma la elige el agente entre las candidatas — el mismo reparto de siempre.
//
// Y LA MARCA GANA. Si el proyecto declara su propia escala, la precedencia del brief ya dice que
// manda ella. Estos números son el default con el que se compone cuando nadie decidió antes.

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

// registroEscala es un sistema numérico completo. NO es una preferencia estética: es la densidad que
// pide un tipo de pantalla. Una tabla de inventario y una pantalla de bienvenida no pueden compartir
// escala tipográfica sin que una de las dos quede mal, y hasta acá el motor no distinguía.
type registroEscala struct {
	Nombre string
	Base   float64 // px del cuerpo
	Razon  float64 // razón geométrica de la escala tipográfica
	Grid   int     // unidad de espaciado, en px
	Fila   int     // alto de fila y de control, en px
	Medida int     // ancho máximo del texto corrido, en caracteres
	Denso  string  // interlineado del dato / del texto corrido
}

// Los tres registros. Las razones son las clásicas del oficio y están elegidas POR DENSIDAD: cuanto
// más apretada la pantalla, más chica la razón —con poco aire un salto grande de tamaño se lee como
// un error de maqueta— y cuanto más editorial, más grande, que es donde el salto respira.
var registrosDeEscala = map[string]registroEscala{
	"compacto":  {"compacto", 13, 1.20, 4, 32, 72, "1,35 en el dato · 1,5 en texto corrido"},
	"estandar":  {"estándar", 14, 1.25, 4, 40, 68, "1,45 en el dato · 1,55 en texto corrido"},
	"editorial": {"editorial", 16.5, 1.3333, 8, 48, 64, "1,5 en el dato · 1,65 en texto corrido"},
}

// ordenRegistros fija el orden de emisión. Es por DENSIDAD y no por el orden en que llegaron las
// candidatas: el brief tiene que salir igual para el mismo eje, siempre.
var ordenRegistros = []string{"compacto", "estandar", "editorial"}

// registroDeForma es lo que ata la capa de forma a los números. Cada una de las doce formas cae en un
// registro, y no hay forma sin registro — el invariante C-ESC1 lo verifica contra `formasDeDiseno`,
// para que agregar una decimotercera forma y olvidarse de sus números rompa el banco en vez de
// entregar un brief mudo justo en el bloque que existe para no ser mudo.
var registroDeForma = map[string]string{
	// Muchas filas comparables: el aire es el enemigo, se escanea con el ojo pegado a la grilla.
	"tabla-densa":      "compacto",
	"rejilla-temporal": "compacto",
	"monitor-procesos": "compacto",

	// Un protagonista o un solo mensaje: el salto de tamaño ES el contenido.
	"tablero-un-numero": "editorial",
	"narrativa":         "editorial",
	"interrupcion":      "editorial",

	// El resto se compone y se opera a la vez: ni apretado ni suelto.
	"lista-priorizada":  "estandar",
	"formulario-guiado": "estandar",
	"detalle-con-lados": "estandar",
	"catalogo-elegir":   "estandar",
	"conversacion":      "estandar",
	"lienzo-inspector":  "estandar",
}

// registroPorDefecto es lo que se sirve cuando el eje no propone forma (`color`, `tipografia`,
// `a11y`, `estado-vacio`, `terminacion`). No se calla: una pregunta sobre la paleta también se
// compone sobre una pantalla, y si no hay forma no hay razón para apretar ni para soltar.
const registroPorDefecto = "estandar"

// escalaPasosAbajo / escalaPasosArriba arman los siete escalones alrededor del cuerpo. Uno abajo
// alcanza —debajo del cuerpo sólo vive la etiqueta— y cinco arriba llegan al titular sin que el
// último escalón sea un tamaño que nadie va a usar.
const (
	escalaPasosAbajo  = 1
	escalaPasosArriba = 5
)

// pasosDeEscala calcula la escala geométrica y la redondea al medio píxel. Se CALCULA en vez de
// escribirse a mano porque una tabla copiada a mano es exactamente donde se cuela el escalón que
// rompe la progresión, y ahí la escala deja de ser un sistema y pasa a ser una lista de números
// lindos. El invariante C-ESC2 verifica que cada escalón siga siendo el anterior por la razón.
func pasosDeEscala(r registroEscala) []float64 {
	var pasos []float64
	for i := -escalaPasosAbajo; i <= escalaPasosArriba; i++ {
		pasos = append(pasos, redondearMedio(r.Base*math.Pow(r.Razon, float64(i))))
	}
	return pasos
}

// redondearMedio lleva al medio píxel más cercano. El medio píxel es la resolución real de la
// tipografía en pantalla: 15,5 px se renderiza distinto de 15 y de 16, y redondear al entero aplasta
// la escala en los pasos chicos —13, 15, 19 tiene razones 1,15 y 1,27, que ya no es una escala—
// justo donde se leen los datos.
func redondearMedio(x float64) float64 { return math.Round(x*2) / 2 }

// numRazon escribe la razón con DOS decimales. Existe aparte de `numPx` porque `numPx` redondea a
// una sola —que es la resolución del píxel— y con eso la razón 1,25 salía impresa «1,2» al lado de
// unos pasos que sí eran 1,25. Un brief que declara una razón distinta de la que muestran sus propios
// números es peor que no declararla: invita a recalcular la escala mal.
func numRazon(x float64) string {
	return strings.Replace(fmt.Sprintf("%.2f", math.Round(x*100)/100), ".", ",", 1)
}

// numPx escribe un número como se lee en castellano: coma decimal, y sin el «,0» cuando es entero.
func numPx(v float64) string {
	if v == math.Trunc(v) {
		return fmt.Sprintf("%d", int(v))
	}
	return strings.Replace(fmt.Sprintf("%.1f", v), ".", ",", 1)
}

// registrosDe devuelve los registros DISTINTOS que abarcan las formas candidatas, en orden de
// densidad. Distintos y no uno por forma: tres candidatas suelen compartir registro, y emitir la
// misma tabla dos veces gasta presupuesto que le sacaría material al corpus.
func registrosDe(formas []string) []string {
	vistos := map[string]bool{}
	for _, f := range formas {
		if reg, ok := registroDeForma[f]; ok {
			vistos[reg] = true
		}
	}
	if len(vistos) == 0 {
		vistos[registroPorDefecto] = true
	}
	var out []string
	for _, reg := range ordenRegistros {
		if vistos[reg] {
			out = append(out, reg)
		}
	}
	return out
}

// formasDelRegistro lista, en orden estable, qué formas candidatas caen en un registro. Va en el
// encabezado de cada tabla para que el agente sepa qué números le tocan según la forma que eligió,
// sin tener que deducirlo.
func formasDelRegistro(formas []string, reg string) []string {
	var out []string
	for _, f := range formas {
		if registroDeForma[f] == reg {
			out = append(out, formasDeDiseno[f].Nombre)
		}
	}
	sort.Strings(out)
	return out
}

// escalaUniversal son los números que NO dependen de la densidad. Van una sola vez.
//
// Cada uno es una decisión con su motivo, no una recomendación:
//   - las duraciones salen de que arriba de ~300 ms una respuesta de interfaz se siente como espera;
//   - el 4,5:1 se extiende al gris secundario a propósito, porque es EXACTAMENTE donde el diseño
//     generado falla accesibilidad: se elige un gris que «se ve elegante» sobre el fondo oscuro y
//     queda en 3,2:1;
//   - el filete separado en dos opacidades existe porque «borde» son dos cosas distintas —una que
//     sólo ordena y otra que divide— y darles el mismo valor es lo que produce la cuadrícula gris
//     que hace que toda la pantalla parezca una planilla.
const escalaUniversal = `UNIVERSAL — no depende del registro:
· transiciones: 120 ms lo que cambia bajo el cursor · 180 ms lo que entra · 240 ms un panel u overlay. Nada de interfaz por encima de 300 ms. Entrada cubic-bezier(.2,0,0,1); salida cubic-bezier(.4,0,1,1). Respetá prefers-reduced-motion.
· contraste: 4,5:1 para TODO texto, incluido el gris secundario — bajarlo ahí es justo donde falla el diseño generado. 3:1 para un borde o un ícono que carga significado; por debajo, sólo lo decorativo.
· foco: anillo de 2 px con 2 px de separación y ≥3:1 contra las dos superficies. Nunca outline:none sin reemplazo.
· toque: 44×44 px mínimo en móvil; en escritorio, 28 px de alto y 8 px entre objetivos.
· dígitos que se comparan en columna: font-variant-numeric: tabular-nums, siempre.
· filete: 8 % de opacidad sobre oscuro y 10 % sobre claro cuando sólo ordena; 14 % y 18 % cuando divide de verdad. Dos valores y no uno: darles el mismo es lo que convierte la pantalla en una planilla.
· tracking e interlineado se dicen por ESCALÓN de la escala y no por píxel: los dos escalones de arriba van a −0,02 em con interlineado 1,1; los dos que siguen a −0,01 em; el cuerpo a 0; las versalitas a +0,09 em.
· radio: la mitad de la rejilla en lo chico, la rejilla en lo mediano, el doble en un contenedor. Un solo salto por pantalla.`

// escalaComoSeUsa es la regla que convierte la tabla en una decisión en vez de un menú. Sin esto, una
// escala de siete pasos es una invitación a usar los siete y a inventar el octavo.
const escalaComoSeUsa = `CÓMO SE USA: un paso de la escala por rol, y no inventes intermedios. El protagonista de la pantalla se salta un escalón —al menos 1,5× el tamaño que le sigue— y es UNO SOLO. Si te hace falta un tamaño que no está en la lista, el problema es la jerarquía y no la escala.`

// escalaPara arma el bloque. Es el único bloque del brief que sirve NÚMEROS en vez de adjetivos, así
// que se emite SIEMPRE: aunque el eje no proponga forma, el registro por defecto se sirve igual.
func escalaPara(formas []string) string {
	var b strings.Builder
	b.WriteString("ESCALA — números DECIDIDOS, no sugerencias: copialos. Si la marca declara los suyos, la marca gana (lo dice la precedencia). Si no, estos son los del motor y no hay que inventar ninguno.")

	for _, clave := range registrosDe(formas) {
		r := registrosDeEscala[clave]
		b.WriteString("\n\nREGISTRO ")
		b.WriteString(r.Nombre)
		if suyas := formasDelRegistro(formas, clave); len(suyas) > 0 {
			b.WriteString(" — si elegís ")
			b.WriteString(strings.Join(suyas, " o "))
		}
		var pasos []string
		for _, p := range pasosDeEscala(r) {
			pasos = append(pasos, numPx(p))
		}
		fmt.Fprintf(&b, "\n· tipografía: base %s px, razón %s → %s px.",
			numPx(r.Base), numRazon(r.Razon), strings.Join(pasos, " · "))
		fmt.Fprintf(&b, "\n· interlineado: %s.", r.Denso)
		fmt.Fprintf(&b, "\n· espaciado: rejilla de %d px → %s. El aire ENTRE grupos, al menos el doble que el de adentro.",
			r.Grid, pasosDeGrid(r.Grid))
		fmt.Fprintf(&b, "\n· fila y control: %d px de alto. Texto corrido: %d caracteres de ancho como máximo.",
			r.Fila, r.Medida)
	}

	b.WriteString("\n\n")
	b.WriteString(escalaUniversal)
	b.WriteString("\n\n")
	b.WriteString(escalaComoSeUsa)
	return b.String()
}

// pasosDeGrid escribe la escalera de espaciado como múltiplos de la rejilla. Se calcula por la misma
// razón que la tipografía: una escalera escrita a mano es donde aparece el 18 px que nadie puede
// explicar y que después se copia a toda la pantalla.
func pasosDeGrid(grid int) string {
	mult := []int{1, 2, 3, 4, 6, 8, 12}
	var out []string
	for _, m := range mult {
		out = append(out, fmt.Sprintf("%d", grid*m))
	}
	return strings.Join(out, " · ") + " px"
}
