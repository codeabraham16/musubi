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
//
// ENVUELVE A ErrPayloadInvalido, y eso NO es decoración. Este rechazo es determinista: el mismo
// texto vuelve a fallar igual dentro de un año. Sin el centinela, el borde MCP lo emitía como
// -32603 «error interno», que para el outbox de un nodo remoto significa «el central se está
// portando mal, insistí» — y el nodo insiste sin tope, por diseño. Ver ErrPayloadInvalido para la
// medición de lo que costó eso.
func ErrSobreDeLlamada(detalle string) error {
	return fmt.Errorf("%w: el `content` se comió el cierre de su propia llamada (%s). "+
		"Los campos que venían después del sobre —importance, mem_type, origin_paths— NO se "+
		"guardaron: quedaron como texto y sus columnas en el default, así que el recall los ordena "+
		"mal. Volvé a llamar con el texto cortado antes de `%s` y esos valores como parámetros",
		ErrPayloadInvalido, detalle, cierreDeContent)
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

// escanearSobres recorre las observaciones que arrastran el sobre y separa DOS cosas: cuántas son,
// y de ésas cuáles todavía declaran una importance que su columna no tiene.
//
// Es UNA sola función porque el check, el conteo y la reparación tienen que estar de acuerdo sobre
// qué significa «recuperable». Con dos definiciones, el `plan` anunciaría un número y el `apply`
// tocaría otro conjunto — y nadie se enteraría, porque los dos serían plausibles.
//
// EL NÚMERO SE BUSCA SÓLO EN LA COLA, después del último `</content>`, y no en todo el texto. La
// diferencia importa desde que ese número se ESCRIBE: una observación que documente este mismo
// defecto puede citar `<importance>1.9` en su prosa, y leerlo de ahí escribiría en la columna un
// valor que nadie pidió. La cola es, por definición, el sobre.
func escanearSobres(e *DbEngine) (int, []filaConSobre, error) {
	filas, err := e.db.Query(
		`SELECT id, content, importance FROM observations WHERE content LIKE '%' || ? || '%'`,
		cierreDeContent)
	if err != nil {
		return 0, nil, fmt.Errorf("no se pudieron leer las observaciones con el sobre comido: %w", err)
	}
	defer filas.Close()

	var conSobre int
	var recuperables []filaConSobre
	for filas.Next() {
		var id, content string
		var importancia float64
		if err := filas.Scan(&id, &content, &importancia); err != nil {
			return 0, nil, fmt.Errorf("error al escanear una observación con sobre: %w", err)
		}
		comido, _ := SobreDeLlamadaComido(content)
		if !comido {
			continue // cita la etiqueta en prosa: no es el defecto
		}
		conSobre++
		cola := content[strings.LastIndex(content, cierreDeContent):]
		m := importanciaDeclaradaEnElTexto.FindStringSubmatch(cola)
		if m == nil {
			continue
		}
		declarada := m[1]
		if declarada == "" {
			declarada = m[2]
		}
		// «Recuperable» = el sobre todavía dice qué importance se pidió Y la columna no la tiene.
		// Si coinciden, la fila es fea pero su ranking está bien y no hay nada que devolverle.
		if v, err := strconv.ParseFloat(declarada, 64); err == nil && v != importancia {
			recuperables = append(recuperables, filaConSobre{id: id, declarada: v})
		}
	}
	if err := filas.Err(); err != nil {
		return 0, nil, fmt.Errorf("error al recorrer las observaciones con sobre: %w", err)
	}
	return conSobre, recuperables, nil
}

// filaConSobre es una observación envenenada cuya importance todavía se puede devolver: el sobre
// que su texto arrastra declara el número que su columna nunca recibió.
type filaConSobre struct {
	id        string
	declarada float64
}

// checkSwallowedEnvelope señala las observaciones que se guardaron con el sobre adentro, ANTES de
// que existiera la guarda de saveObservation.
//
// El prefiltro es un LIKE barato y la decisión la toma `SobreDeLlamadaComido` en Go: el LIKE solo
// no distingue el bug de una observación que CITA la etiqueta, y una guarda que confunde las dos
// deja a la memoria sin poder documentar su propio defecto.
func checkSwallowedEnvelope(e *DbEngine) CheckResult {
	const code = "swallowed_envelope"
	conSobre, recuperables, err := escanearSobres(e)
	if err != nil {
		return CheckResult{Code: code, Status: "error", Message: err.Error()}
	}
	if conSobre == 0 {
		return CheckResult{Code: code, Status: "ok",
			Message: "ninguna observación arrastra el cierre de su propia llamada"}
	}
	msg := fmt.Sprintf("%d observación(es) se guardaron con el sobre de la llamada adentro del "+
		"`content`: los campos que venían después —importance, mem_type, origin_paths— quedaron "+
		"como texto y sus columnas en el default, así que el recall las ordena mal. Son de ANTES de "+
		"la guarda de saveObservation. El texto NO se lava: reescribirlo cambiaría el content_hash, "+
		"que es la clave del dedup y viaja en el sync", conSobre)
	if len(recuperables) > 0 {
		msg += fmt.Sprintf(". De ésas, %d todavía dicen en su sobre qué importance se les pidió y su "+
			"columna no la tiene: son las únicas que una reparación puede devolver de verdad, y "+
			"`musubi doctor --repair swallowed_envelope` les devuelve ESE número sin tocar el texto",
			len(recuperables))
	}
	// Reparable SÓLO si queda algo que devolver. Con el sobre presente pero sin ningún número
	// recuperable, ofrecer la reparación sería ofrecer un no-op: el usuario la corre, no pasa nada,
	// y aprende a desconfiar de la herramienta.
	return CheckResult{Code: code, Status: "warning", Message: msg, Repairable: len(recuperables) > 0}
}

// countSwallowedImportance cuenta lo que la reparación tocaría: NO las observaciones con el sobre,
// sino las que todavía pueden recuperar su importance. Es lo que el `plan` le promete al usuario, y
// tiene que ser exactamente el conjunto que `apply` modifica.
func countSwallowedImportance(e *DbEngine) (int, error) {
	_, recuperables, err := escanearSobres(e)
	if err != nil {
		return 0, err
	}
	return len(recuperables), nil
}

// applySwallowedImportance le devuelve a cada fila la importance que su sobre declara.
//
// TOCA UNA SOLA COLUMNA, Y ESO ES TODO EL DISEÑO. La nota original de este archivo daba tres
// razones para no tener `apply`, y las tres siguen en pie — pero las tres hablan de LAVAR EL TEXTO,
// no de corregir la columna:
//
//  1. «La reparación no puede devolver lo que nunca se escribió»: cierto, y por eso ésta no dice
//     que lo haga. Las que perdieron mem_type y origin_paths sin dejar rastro se siguen contando en
//     el check después de reparar — no hay falso verde, porque el texto sigue delatándolas.
//  2. «Lavar el texto borra la única evidencia»: no se lava. El número se lee del sobre y el sobre
//     se queda donde está. Esa nota además describía la forma correcta —«hay que LEER la importance
//     antes de recortar y escribirla en su columna»—; esto es exactamente eso, menos el recortar.
//  3. «Reescribir el contenido cambia el content_hash»: no se reescribe. ContentHash deriva sólo
//     del content, así que el hash queda idéntico y el dedup sigue reconociendo la fila.
//
// ES DELIBERADAMENTE LOCAL, y hay que decirlo porque parece un olvido. Como el content_hash no
// cambia, enqueueOutboxTx no re-encola, así que la corrección no viaja al central. Y no podría: el
// central rechazaría ese push con su propia guarda del sobre. La consecuencia práctica es que esta
// reparación se corre EN CADA CEREBRO, no en uno que después reparte.
func applySwallowedImportance(e *DbEngine) (int, error) {
	_, recuperables, err := escanearSobres(e)
	if err != nil {
		return 0, err
	}
	if len(recuperables) == 0 {
		return 0, nil
	}
	tx, err := e.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("error al iniciar la transacción de reparación: %w", err)
	}
	defer tx.Rollback()

	st, err := tx.Prepare(`UPDATE observations SET importance = ? WHERE id = ?`)
	if err != nil {
		return 0, fmt.Errorf("error al preparar la actualización de importance: %w", err)
	}
	defer st.Close()

	var reparadas int
	for _, f := range recuperables {
		res, err := st.Exec(f.declarada, f.id)
		if err != nil {
			return 0, fmt.Errorf("error al devolver la importance de %s: %w", f.id, err)
		}
		// Un UPDATE que no matchea nada es un éxito de SQL: sin contar filas afectadas, una
		// reparación que no tocó nada reportaría el mismo número que una que funcionó.
		n, err := res.RowsAffected()
		if err != nil {
			return 0, fmt.Errorf("error al contar las filas reparadas de %s: %w", f.id, err)
		}
		reparadas += int(n)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("error al commitear la reparación de importance: %w", err)
	}
	return reparadas, nil
}
