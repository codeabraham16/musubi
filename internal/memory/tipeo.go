package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"musubi/internal/logx"
)

// tipeo.go es el CORRECTOR DE TIPEO de la consulta: antes de buscar, cambia un término que no existe
// en ninguna nota por el término de la memoria que está a UN error de tipeo de distancia.
//
// POR QUÉ HACE FALTA. El recall es léxico: un «infroamcion», un «temrinal» o un «fichjae» (tipeos
// reales del dueño) no matchean nada, y el vector no los rescata — medido, el coseno entre el tipeo y
// la palabra correcta promedia 0,159 sobre 10 pares, que queda bajo el piso del pool vectorial.
//
// QUÉ CORRIGE, Y QUÉ NO:
//
//   - Sólo términos MUERTOS: los que no aparecen en ninguna nota visible del alcance, ni exactos ni
//     con la cláusula de prefijo de raíz del recall (stemForPrefix). Un término vivo no se toca
//     nunca, aunque haya uno parecido más frecuente: «deploys» con «deploy» en la memoria está vivo.
//   - Tres clases de error, en este orden estricto: dos letras vecinas invertidas, una letra de
//     menos, una letra de más. Gana la PRIMERA clase que tenga un candidato vivo.
//   - NUNCA la sustitución de una letra por otra: ahí el tipeo suele dar otra palabra válida
//     (dejemos↔dejamos), y reescribirla cambiaría lo que la persona pidió.
//
// LO QUE VE ES LO QUE EL RECALL PODRÍA DEVOLVER, y nada más: notas visibles (visibleObsPredicate) y
// del alcance que le pasa quien llama, que es el mismo filtro DURO que ese caller le aplica al
// recall (scopeClause). Con otro criterio, el corrector podría reescribir una consulta con una
// palabra que sólo existe en otro proyecto — o sea, filtrarle vocabulario ajeno a quien pregunta.
//
// NO ESCRIBE NADA. Sin tabla nueva ni migración: fts5vocab necesitaría un CREATE, y el corrector
// tiene que andar sobre un motor en sólo lectura (NewDbEngineSoloLectura). Por eso cada candidato
// se verifica contra el índice con un MATCH de término exacto.
//
// EL RECALL NO CORRIGE. Corrigen los que llaman (el hook del turno, musubi_recall, musubi_ask y el
// banco), ANTES de embeber: el texto corregido es el que va al embebedor y al recall, y el aviso de
// qué se corrigió lo arma cada superficie a su manera.

// Correccion es un término de la consulta que el corrector reemplazó por otro.
type Correccion struct {
	// Tipeado es el término como lo escribió la persona.
	Tipeado string `json:"tipeado"`
	// Buscado es el término que se buscó en su lugar.
	Buscado string `json:"buscado"`
	// DF es en cuántas notas visibles del alcance aparece Buscado como término exacto. No viaja en el
	// JSON: le sirve a quien mide el corrector, no al agente.
	DF int `json:"-"`
}

const (
	// minRunasDeTipeo: los términos más cortos no se revisan, porque ahí un tipeo suele dar otra
	// palabra válida. Es el mismo largo que perturba el banco (recalleval.PerturbarConsulta).
	minRunasDeTipeo = 5
	// maxTerminosDeTipeo acota cuántos términos distintos se revisan: un prompt pegado de una página
	// no puede convertir al corrector en el costo del turno.
	maxTerminosDeTipeo = 40
	// maxCorreccionesDeTipeo es el techo de términos muertos que se intentan corregir por consulta, los
	// más largos primero: son los que más discriminan y los que un tipeo más suele romper.
	maxCorreccionesDeTipeo = 4
	// dfMinimoDeCandidato: un candidato tiene que aparecer en DOS notas o más. Una palabra que está en
	// una sola nota puede ser, ella misma, el tipeo de alguien, y corregir hacia un tipeo no sirve.
	dfMinimoDeCandidato = 2
	// alfabetoDeTipeo son las letras que se prueban en «falta una letra». Sin tildes ni eñe A
	// PROPÓSITO: el FTS (tokenizer unicode61) pliega mayúsculas y diacríticos al indexar y al buscar,
	// así que «á» y «a», o «ñ» y «n», son el MISMO término del índice. Probarlas multiplicaría las
	// consultas por candidatos que el índice no distingue. Tampoco dígitos: sólo se revisan términos
	// que son todo letras.
	alfabetoDeTipeo = "abcdefghijklmnopqrstuvwxyz"
)

// PlazoDelCorrector es el techo de tiempo del corrector entero. Si se vence —o cualquier consulta
// falla—, la consulta sigue como vino: corregir es una ayuda, nunca el costo del turno.
//
// Lo que cuesta, medido sobre una copia de la base real (1,9 k notas, 9 segmentos de FTS) con los 54
// prompts reales que tienen términos muertos, y con el motor FRÍO como en el hook, que es un proceso
// nuevo por turno: p50 15 ms y p95 29 ms, máximo 38. Ver corregirTerminos para por qué en paralelo.
const PlazoDelCorrector = 60 * time.Millisecond

// CorregirConsulta devuelve q con sus términos muertos corregidos y la lista de lo que corrigió (nil
// si no cambió nada). alcance es el filtro duro que quien llama le aplica al recall: su valor cero es
// federado, o sea todo lo visible.
//
// Ante cualquier error o si se vence PlazoDelCorrector devuelve q tal cual y nil: una corrección a
// medias no se aplica.
func (e *DbEngine) CorregirConsulta(ctx context.Context, q string, alcance ProjectScope) (string, []Correccion) {
	return e.CorregirConsultaConPlazo(ctx, q, alcance, PlazoDelCorrector)
}

// CorregirConsultaConPlazo es CorregirConsulta con otro techo de tiempo. Las superficies usan
// CorregirConsulta; esto existe para el banco, que mide QUÉ corrige el corrector y no puede depender
// de cuán cargada está la máquina que corre las pruebas (el plazo se mide aparte, en el hook).
func (e *DbEngine) CorregirConsultaConPlazo(ctx context.Context, q string, alcance ProjectScope, plazo time.Duration) (string, []Correccion) {
	terminos := terminosDeTipeo(q)
	if len(terminos) == 0 {
		return q, nil
	}
	ctx, cancel := context.WithTimeout(ctx, plazo)
	defer cancel()
	correcciones, err := e.corregirTerminos(ctx, terminos, alcance)
	if err != nil {
		if ctx.Err() != nil {
			// Info y no Warn, como el timeout del embebedor en el turno: es la conducta diseñada de la
			// guarda, no una avería.
			logx.Info("corrector de tipeo: se venció el plazo, la consulta va como vino", "plazo", plazo)
		} else {
			logx.Warn("corrector de tipeo: falló, la consulta va como vino", "error", err)
		}
		return q, nil
	}
	if len(correcciones) == 0 {
		return q, nil
	}
	return reescribirConsulta(q, correcciones), correcciones
}

// terminoDeTipeo es un término revisable: como lo escribió la persona, en minúsculas, y su posición
// entre los revisables (para devolver las correcciones en el orden de la consulta).
type terminoDeTipeo struct {
	original, bajo string
	orden          int
}

// terminosDeTipeo son los términos que el corrector revisa: los de TerminosDeConsulta (la definición
// única de término) que son todo letras y tienen minRunasDeTipeo runas o más, sin repetir, hasta
// maxTerminosDeTipeo.
func terminosDeTipeo(q string) []terminoDeTipeo {
	vistos := map[string]bool{}
	var out []terminoDeTipeo
	for _, t := range TerminosDeConsulta(q) {
		r := []rune(t)
		if len(r) < minRunasDeTipeo || !todoLetras(r) {
			continue
		}
		bajo := strings.ToLower(t)
		if vistos[bajo] {
			continue
		}
		vistos[bajo] = true
		out = append(out, terminoDeTipeo{original: t, bajo: bajo, orden: len(out)})
		if len(out) == maxTerminosDeTipeo {
			break
		}
	}
	return out
}

func todoLetras(r []rune) bool {
	for _, c := range r {
		if !unicode.IsLetter(c) {
			return false
		}
	}
	return true
}

// corregirTerminos busca los términos muertos y, para los más largos, su corrección.
//
// CADA TÉRMINO MUERTO SE CORRIGE EN SU PROPIA GOROUTINE, con su conexión del pool. Son independientes,
// y lo caro es el candidato: cada uno es un MATCH contra todos los segmentos del índice, y «falta una
// letra» son 26 por posición (286 para una palabra de 10). En serie, cuatro muertos sin candidato en
// las primeras clases suman sus enumeraciones enteras, y con la base fría del hook eso pasaba el plazo:
// medido sobre la copia de la base real, p95 79 ms y 12 de 108 corridas por encima de 60 ms. En
// paralelo el costo es el del término más caro: p95 29 ms, ninguna por encima. Partir además cada
// clase en trozos no ganaba nada medible (p95 31-32 ms).
//
// Un pool más chico que los términos (el motor sin arranque tiene 2 conexiones) sólo los pone en fila.
func (e *DbEngine) corregirTerminos(ctx context.Context, terminos []terminoDeTipeo, alcance ProjectScope) ([]Correccion, error) {
	muertos, err := e.terminosMuertos(ctx, terminos, alcance)
	if err != nil || len(muertos) == 0 {
		return nil, err
	}
	sort.SliceStable(muertos, func(i, j int) bool {
		return len([]rune(muertos[i].bajo)) > len([]rune(muertos[j].bajo))
	})
	if len(muertos) > maxCorreccionesDeTipeo {
		muertos = muertos[:maxCorreccionesDeTipeo]
	}
	type hallada struct {
		Correccion
		orden int
		err   error
	}
	halladas := make([]hallada, len(muertos))
	var wg sync.WaitGroup
	for i, m := range muertos {
		wg.Add(1)
		go func() {
			defer wg.Done()
			buscado, df, err := e.mejorCandidato(ctx, m.bajo, alcance)
			halladas[i] = hallada{Correccion{Tipeado: m.original, Buscado: buscado, DF: df}, m.orden, err}
		}()
	}
	wg.Wait()
	// En el orden de la consulta, que es como se lee el aviso.
	sort.Slice(halladas, func(i, j int) bool { return halladas[i].orden < halladas[j].orden })
	out := make([]Correccion, 0, len(halladas))
	for _, h := range halladas {
		if h.err != nil {
			return nil, h.err
		}
		if h.Buscado != "" {
			out = append(out, h.Correccion)
		}
	}
	return out, nil
}

// terminosMuertos devuelve los términos sin ninguna nota visible del alcance. En dos pasadas, y las
// dos son la cláusula del recall partida en sus mitades: primero el término EXACTO (barato, y es el
// caso de casi todos los términos de un prompt), y sólo a los que no aparecen, el PREFIJO de su raíz
// (stemForPrefix), que es lo que hace el recall con recall_stemming encendido.
//
// La segunda pasada corre aunque recall_stemming esté apagado, a propósito: un término cuya raíz
// está en la memoria es una palabra VÁLIDA en otra flexión («deploys» con «deploy» indexado), y
// reescribirla sería corregir algo que no es un tipeo. Encontrar las flexiones es trabajo de
// recall_stemming, no del corrector.
func (e *DbEngine) terminosMuertos(ctx context.Context, terminos []terminoDeTipeo, alcance ProjectScope) ([]terminoDeTipeo, error) {
	exactos := make([]string, len(terminos))
	for i, t := range terminos {
		exactos[i] = expresionExacta(t.bajo)
	}
	vivos, err := e.expresionesVivas(ctx, exactos, alcance)
	if err != nil {
		return nil, err
	}
	var sinExacto []terminoDeTipeo
	var prefijos []string
	for i, t := range terminos {
		if terminoVivo(vivos, exactos[i]) {
			continue
		}
		sinExacto = append(sinExacto, t)
		prefijos = append(prefijos, `"`+stemForPrefix(t.bajo)+`"*`)
	}
	if len(sinExacto) == 0 {
		return nil, nil
	}
	vivos, err = e.expresionesVivas(ctx, prefijos, alcance)
	if err != nil {
		return nil, err
	}
	var muertos []terminoDeTipeo
	for i, t := range sinExacto {
		if terminoVivo(vivos, prefijos[i]) {
			continue
		}
		muertos = append(muertos, t)
	}
	return muertos, nil
}

// terminoVivo dice si la expresión de un término matcheó alguna nota visible del alcance.
func terminoVivo(vivas map[string]bool, expr string) bool {
	return vivas[expr]
}

// expresionExacta es cómo se busca un término como término EXACTO del índice: entre comillas y sin
// `*`. Con `*` sería un prefijo, y un candidato que no es una palabra de la memoria pasaría por vivo
// sólo por ser el comienzo de otra.
func expresionExacta(t string) string {
	return `"` + t + `"`
}

// expresionesVivas corre las expresiones MATCH en UNA consulta y devuelve las que tienen al menos una
// nota visible del alcance.
func (e *DbEngine) expresionesVivas(ctx context.Context, exprs []string, alcance ProjectScope) (map[string]bool, error) {
	lista, err := json.Marshal(exprs)
	if err != nil {
		return nil, err
	}
	clausula, argsAlcance := alcance.scopeClause("o")
	consulta := fmt.Sprintf(sqlExpresionesVivas, visibleObsPredicateDe("o"), clausula)
	args := append([]interface{}{string(lista)}, argsAlcance...)
	filas, err := e.db.QueryContext(ctx, consulta, args...)
	if err != nil {
		return nil, err
	}
	defer filas.Close()
	vivas := map[string]bool{}
	for filas.Next() {
		var expr string
		if err := filas.Scan(&expr); err != nil {
			return nil, err
		}
		vivas[expr] = true
	}
	return vivas, filas.Err()
}

// sqlExpresionesVivas: de una lista JSON de expresiones MATCH, las que matchean al menos una nota
// VISIBLE (primer %s) y del alcance (segundo %s, la scopeClause del caller).
const sqlExpresionesVivas = `SELECT c.value FROM json_each(?) c WHERE EXISTS (
	SELECT 1 FROM observations_fts f JOIN observations o ON o.rowid = f.rowid
	WHERE observations_fts MATCH c.value AND %s%s)`

// sqlDFDeCandidatos: para cada candidato de una lista JSON, en cuántas notas VISIBLES (primer %s) y
// del alcance (segundo %s) aparece como término EXACTO: entre comillas y sin `*` (ver
// expresionExacta). Los parámetros del alcance van ANTES que la lista, porque aparecen antes en el
// texto de la consulta.
const sqlDFDeCandidatos = `SELECT c.value, (
	SELECT COUNT(*) FROM observations_fts f JOIN observations o ON o.rowid = f.rowid
	WHERE observations_fts MATCH '"' || c.value || '"' AND %s%s) FROM json_each(?) c`

// mejorCandidato busca la corrección de un término muerto: la primera clase de error con un candidato
// vivo gana, y dentro de la clase, el candidato que aparece en más notas (con empate, el primero en
// orden alfabético, para que la respuesta no dependa del orden del mapa). "" si no hay ninguno.
func (e *DbEngine) mejorCandidato(ctx context.Context, termino string, alcance ProjectScope) (string, int, error) {
	r := []rune(termino)
	for _, clase := range [][]string{transposiciones(r), faltaUnaLetra(r), sobraUnaLetra(r)} {
		cands := candidatosValidos(clase, termino)
		if len(cands) == 0 {
			continue
		}
		mejor, df, err := e.mejorDeLaClase(ctx, cands, alcance)
		if err != nil {
			return "", 0, err
		}
		if mejor != "" {
			return mejor, df, nil
		}
	}
	return "", 0, nil
}

// candidatosValidos saca los repetidos, el término mismo y las stopwords: una stopword el recall la
// descarta igual, así que «corregir» hacia ella sólo sumaría un aviso.
func candidatosValidos(clase []string, termino string) []string {
	vistos := map[string]bool{termino: true}
	var out []string
	for _, c := range clase {
		if vistos[c] || ftsStopwords[c] {
			continue
		}
		vistos[c] = true
		out = append(out, c)
	}
	return out
}

// mejorDeLaClase cuenta el df exacto de cada candidato en una sola consulta y devuelve el mejor que
// llega a dfMinimoDeCandidato.
func (e *DbEngine) mejorDeLaClase(ctx context.Context, cands []string, alcance ProjectScope) (string, int, error) {
	lista, err := json.Marshal(cands)
	if err != nil {
		return "", 0, err
	}
	clausula, argsAlcance := alcance.scopeClause("o")
	consulta := fmt.Sprintf(sqlDFDeCandidatos, visibleObsPredicateDe("o"), clausula)
	args := append(append([]interface{}{}, argsAlcance...), string(lista))
	filas, err := e.db.QueryContext(ctx, consulta, args...)
	if err != nil {
		return "", 0, err
	}
	defer filas.Close()
	mejor, mejorDF := "", 0
	for filas.Next() {
		var c string
		var df int
		if err := filas.Scan(&c, &df); err != nil {
			return "", 0, err
		}
		if df < dfMinimoDeCandidato {
			continue
		}
		if df > mejorDF || (df == mejorDF && c < mejor) {
			mejor, mejorDF = c, df
		}
	}
	if err := filas.Err(); err != nil {
		return "", 0, err
	}
	return mejor, mejorDF, nil
}

// transposiciones: dos letras vecinas invertidas (infromacion → informacion).
func transposiciones(r []rune) []string {
	var out []string
	for k := 0; k+1 < len(r); k++ {
		if r[k] == r[k+1] {
			continue
		}
		c := append([]rune(nil), r...)
		c[k], c[k+1] = c[k+1], c[k]
		out = append(out, string(c))
	}
	return out
}

// faltaUnaLetra: la palabra con una letra más en cualquier posición (rasberry → raspberry). Las
// letras salen de alfabetoDeTipeo.
func faltaUnaLetra(r []rune) []string {
	out := make([]string, 0, (len(r)+1)*len(alfabetoDeTipeo))
	for i := 0; i <= len(r); i++ {
		for _, l := range alfabetoDeTipeo {
			out = append(out, string(r[:i])+string(l)+string(r[i:]))
		}
	}
	return out
}

// sobraUnaLetra: la palabra con una letra menos (inforrmacion → informacion).
func sobraUnaLetra(r []rune) []string {
	out := make([]string, 0, len(r))
	for i := range r {
		out = append(out, string(r[:i])+string(r[i+1:]))
	}
	return out
}

// reescribirConsulta cambia en q cada término corregido por lo que se buscó en su lugar. Corta los
// términos igual que camposDeConsulta y compara sin mayúsculas; todo lo demás de q queda como vino.
func reescribirConsulta(q string, cs []Correccion) string {
	reemplazo := make(map[string]string, len(cs))
	for _, c := range cs {
		reemplazo[strings.ToLower(c.Tipeado)] = c.Buscado
	}
	esDeTermino := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
	runas := []rune(q)
	var b strings.Builder
	b.Grow(len(q))
	for i := 0; i < len(runas); {
		if !esDeTermino(runas[i]) {
			b.WriteRune(runas[i])
			i++
			continue
		}
		j := i
		for j < len(runas) && esDeTermino(runas[j]) {
			j++
		}
		palabra := string(runas[i:j])
		if nuevo, ok := reemplazo[strings.ToLower(palabra)]; ok {
			b.WriteString(nuevo)
		} else {
			b.WriteString(palabra)
		}
		i = j
	}
	return b.String()
}
