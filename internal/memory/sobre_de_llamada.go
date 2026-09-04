package memory

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ── El content que se comió el cierre de su propia llamada ───────────────────────────────────
//
// SÍNTOMA MEDIDO el 2026-09-04 contra el cerebro central en solo-lectura, 4502 observaciones:
// **73 tienen `</content>` DENTRO del content**, y todo lo que sigue es el sobre de la llamada que
// las trajo. Tres formas, y las tres empiezan en el mismo lugar:
//
//	36 · el content termina en `</content>` y nada más
//	29 · sigue hasta `</invoke>`
//	 8 · corta en medio del sobre (`<parameter name="importance">1.6`)
//
// Cero terminan en `</invoke>` sin tener antes `</content>`: el corte es SIEMPRE ahí.
//
// EL DAÑO NO ES COSMÉTICO, y es la razón por la que esto es una guarda y no un lavado de texto.
// Los campos que el sobre traía quedaron SIN SETEAR: **13 de las 73 declaran su importance adentro
// del texto y su columna dice otra cosa** — casi siempre 1.0, el default, que es el valor más común
// de toda la memoria (2303 de 4502). Se hundieron en el montón justo las que alguien marcó como
// importantes. El recall ordena por `importance`: una observación que se guardó con 1.9 y quedó en
// 1.0 no es una fea, es una que no se va a recordar cuando haga falta.
//
// (Ese 13 fue 4 en la primera medición, y la diferencia es una lección chica: la primera consulta
// buscaba sólo `<importance>` y no la otra forma, `<parameter name="importance">`. Un conteo que
// nombra una sola de las formas que uno mismo acaba de medir sub-reporta con cara de dato.)
//
// Y NO FALLA RUIDOSAMENTE: el save acepta el texto y devuelve OK. Gotea desde el 2026-07-13 hasta
// hoy — dos meses.
//
// POR QUÉ NO ALCANZA CON BUSCAR `</content>` A SECAS: una observación que DOCUMENTE este defecto
// tiene que poder citar la etiqueta. Lo que distingue el bug de la mención es qué viene DESPUÉS —
// en el bug, sobre; en la mención, prosa. Una guarda que rechazara la mención dejaría la memoria
// sin poder hablar de su propio defecto, que es el peor lugar donde ponerle un candado.

const cierreDeContent = "</content>"

// SobreDeLlamadaComido dice si `content` arrastra el cierre de la llamada que lo trajo, y con qué.
//
// El criterio es POSICIONAL y no léxico: se corta en el ÚLTIMO `</content>` y se mira la cola. Es
// sobre si cada línea no vacía de esa cola empieza con `<` — las tres formas medidas cumplen, y la
// prosa de una mención no, porque una frase no arranca con un signo de menor.
//
// Limitación declarada: una observación cuya prosa después de un `</content>` citado empiece TODAS
// sus líneas con `<` se leería como sobre. No apareció en las 4502 y el costo de equivocarse es un
// rechazo con mensaje, no una pérdida.
func SobreDeLlamadaComido(content string) (bool, string) {
	i := strings.LastIndex(content, cierreDeContent)
	if i < 0 {
		return false, ""
	}
	cola := content[i+len(cierreDeContent):]
	if strings.TrimSpace(cola) == "" {
		return true, "el texto termina en `" + cierreDeContent + "`"
	}
	var etiquetas []string
	for _, linea := range strings.Split(cola, "\n") {
		linea = strings.TrimSpace(linea)
		if linea == "" {
			continue
		}
		if !strings.HasPrefix(linea, "<") {
			// Hay prosa después: el `</content>` es una MENCIÓN, no el sobre.
			return false, ""
		}
		if len(etiquetas) < 4 {
			etiquetas = append(etiquetas, primerosN(linea, 40))
		}
	}
	if len(etiquetas) == 0 {
		return true, "el texto termina en `" + cierreDeContent + "`"
	}
	return true, "después de `" + cierreDeContent + "` sigue el sobre: " + strings.Join(etiquetas, " · ")
}

// ErrSobreDeLlamada es el error con el que se rechaza un content que se comió su sobre. Nombra lo
// que se tragó, porque el llamador tiene que poder reescribir la llamada sin adivinar: el texto se
// cortó en el `</content>` y los campos de después nunca llegaron a sus columnas.
func ErrSobreDeLlamada(detalle string) error {
	return fmt.Errorf("el `content` se comió el cierre de su propia llamada (%s). "+
		"Los campos que venían después del sobre —importance, mem_type, origin_paths— NO se "+
		"guardaron: quedaron como texto y sus columnas en el default, así que el recall los ordena "+
		"mal. Volvé a llamar con el texto cortado antes de `%s` y esos valores como parámetros",
		detalle, cierreDeContent)
}

func primerosN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ── El chequeo del doctor: las que YA están ───────────────────────────────────────────────────

// importanciaDeclaradaEnElTexto saca la importance que el sobre traía, si el texto la arrastró.
// Cubre las dos formas medidas: `<importance>1.6</importance>` y `<parameter name="importance">1.6`.
var importanciaDeclaradaEnElTexto = regexp.MustCompile(
	`<importance>\s*([0-9]*\.?[0-9]+)|<parameter name="importance">\s*([0-9]*\.?[0-9]+)`)

// checkSwallowedEnvelope señala las observaciones que se guardaron con el sobre adentro, ANTES de
// que existiera la guarda de saveObservation.
//
// El prefiltro es un LIKE barato y la decisión la toma `SobreDeLlamadaComido` en Go: el LIKE solo
// no distingue el bug de una observación que CITA la etiqueta, y una guarda que confunde las dos
// deja a la memoria sin poder documentar su propio defecto.
//
// POR QUÉ NO TIENE `apply`, Y ES LA DECISIÓN DE DISEÑO. Hay tres razones, y cada una alcanza:
//
//  1. LA REPARACIÓN NO PUEDE DEVOLVER LO QUE NUNCA SE ESCRIBIÓ. De las 73 medidas el 2026-09-04,
//     13 arrastran su importance en el texto; las otras 60 perdieron `mem_type` y `origin_paths`
//     sin dejar rastro. Un `apply` que lave el texto se vería como «arreglado» y dejaría 60 con
//     sus columnas en el default para siempre — un falso verde sobre una pérdida.
//  2. LAVAR EL TEXTO BORRA LA ÚNICA EVIDENCIA. Para esas 13, el número declarado vive en el texto
//     que el lavado se lleva. Si se repara, hay que LEER la importance antes de recortar y
//     escribirla en su columna, en la misma transacción — y eso no es una limpieza, es una
//     migración con criterio.
//  3. REESCRIBIR EL CONTENIDO CAMBIA EL `content_hash`, que es la clave del dedup y viaja en el
//     sync. Una observación reparada deja de parecerse a sí misma: el dedup ya no la reconoce y un
//     re-save del mismo texto crearía un duplicado, y el nodo que la había sincronizado la ve como
//     contenido nuevo. Es exactamente el tipo de daño que este check existe para no causar.
//
// Así que REPORTA. Lo que faltaba no era la capacidad de reescribir filas, era enterarse de que
// están — y saber cuántas de ellas todavía se pueden reparar de verdad.
func checkSwallowedEnvelope(e *DbEngine) CheckResult {
	const code = "swallowed_envelope"
	filas, err := e.db.Query(
		`SELECT content, importance FROM observations WHERE content LIKE '%' || ? || '%'`,
		cierreDeContent)
	if err != nil {
		return CheckResult{Code: code, Status: "error",
			Message: "no se pudieron leer las observaciones con el sobre comido: " + err.Error()}
	}
	defer filas.Close()

	var conSobre, recuperables int
	for filas.Next() {
		var content string
		var importancia float64
		if err := filas.Scan(&content, &importancia); err != nil {
			return CheckResult{Code: code, Status: "error", Message: "error al escanear: " + err.Error()}
		}
		comido, _ := SobreDeLlamadaComido(content)
		if !comido {
			continue // cita la etiqueta en prosa: no es el defecto
		}
		conSobre++
		// «Recuperable» = el texto todavía dice qué importance se pidió Y la columna no la tiene.
		// Si coinciden, la fila es fea pero su ranking está bien y no hay nada que devolverle.
		if m := importanciaDeclaradaEnElTexto.FindStringSubmatch(content); m != nil {
			declarada := m[1]
			if declarada == "" {
				declarada = m[2]
			}
			if v, err := strconv.ParseFloat(declarada, 64); err == nil && v != importancia {
				recuperables++
			}
		}
	}
	if err := filas.Err(); err != nil {
		return CheckResult{Code: code, Status: "error", Message: "error al recorrer: " + err.Error()}
	}
	if conSobre == 0 {
		return CheckResult{Code: code, Status: "ok",
			Message: "ninguna observación arrastra el cierre de su propia llamada"}
	}
	msg := fmt.Sprintf("%d observación(es) se guardaron con el sobre de la llamada adentro del "+
		"`content`: los campos que venían después —importance, mem_type, origin_paths— quedaron "+
		"como texto y sus columnas en el default, así que el recall las ordena mal. Son de ANTES de "+
		"la guarda de saveObservation; no se reparan solas y no hay `apply` a propósito (ver "+
		"sobre_de_llamada.go: reescribir el content cambia el content_hash, que es la clave del "+
		"dedup y viaja en el sync)", conSobre)
	if recuperables > 0 {
		msg += fmt.Sprintf(". De ésas, %d todavía dicen en su texto qué importance se les pidió y "+
			"su columna no la tiene: son las únicas que una reparación podría devolver de verdad, y "+
			"hay que leer el número ANTES de recortar el texto que lo contiene", recuperables)
	}
	return CheckResult{Code: code, Status: "warning", Message: msg}
}
