package recalleval

import (
	"encoding/binary"
	"hash/fnv"
	"strings"
	"unicode"
)

// perturbar.go le mete errores de tipeo a una consulta, para medir cuánto pierde el recall cuando
// la persona tipea mal. Es un INSTRUMENTO del banco: no corrige nada.
//
// POR CLASE, Y CON SUSTITUCIÓN INCLUIDA. El corrector que viene detrás arregla transposiciones,
// letras de menos y letras de más, y NO sustituciones (una conjugación válida como dejemos↔dejamos
// no se reescribe). Si el banco perturbara sólo con transposiciones, mediría el corrector con la
// misma clase de error que el corrector sabe arreglar: circular. Por eso cada clase es un brazo, y
// la sustitución está para ver lo que el corrector a propósito no toca.
//
// TODOS LOS TÉRMINOS DE 5 RUNAS O MÁS, NO SÓLO EL MÁS LARGO. Medido sobre golden.json: un tipeo en
// el término más largo no mueve el MRR (0,722 → 0,722), porque la consulta es un OR y basta un
// término vivo para encontrar lo mismo. Un instrumento que no mueve el dorado no puede defender un
// gate. El brazo de UN tipeo (PerturbarTerminoMasLargo) se conserva igual, porque es el caso más
// común en un prompt real y el que la línea base del frente ya midió.
//
// QUÉ ES UN TÉRMINO: lo mismo que para el recall (rankedTerms en internal/memory/recall.go), una
// corrida de letras o dígitos. Sólo se perturban los que son TODO letras y tienen 5 runas o más,
// que son los que el corrector va a revisar: un «deploy2prod» no es un tipeo que nadie haga, y
// partirlo por el dígito le metería al banco errores que el corrector no puede ver por diseño. Los
// cortos se dejan intactos porque ahí un tipeo suele dar otra palabra válida.

// ClaseDeTipeo es la forma del error que se le mete a cada término.
type ClaseDeTipeo string

const (
	// TipeoTransposicion invierte dos letras vecinas: información → infromación.
	TipeoTransposicion ClaseDeTipeo = "transposicion"
	// TipeoFalta saca una letra: información → infrmación.
	TipeoFalta ClaseDeTipeo = "falta"
	// TipeoSobra repite una letra: información → inforrmación.
	TipeoSobra ClaseDeTipeo = "sobra"
	// TipeoSustitucion cambia una letra por otra: información → infprmación.
	TipeoSustitucion ClaseDeTipeo = "sustitucion"
)

// ClasesDeTipeo son las cuatro clases, en el orden en que el banco las reporta.
var ClasesDeTipeo = []ClaseDeTipeo{TipeoTransposicion, TipeoFalta, TipeoSobra, TipeoSustitucion}

// minRunasPerturbables es el largo mínimo de un término para recibir un tipeo.
const minRunasPerturbables = 5

// alfabetoDeSustitucion son las letras con las que TipeoSustitucion reemplaza.
const alfabetoDeSustitucion = "abcdefghijklmnopqrstuvwxyz"

// PerturbarConsulta devuelve q con UN error de la clase pedida en CADA término perturbable (todo
// letras, 5 runas o más). Lo que no es término —espacios, signos— se copia tal cual. La posición
// del error sale de un hash de (semilla, q, índice del término), así que es DETERMINISTA: la misma
// entrada da la misma salida en cualquier máquina, y dos semillas dan dos perturbaciones distintas.
// La primera letra no se toca nunca: es la que casi nadie erra.
//
// Una clase desconocida devuelve q sin cambios.
func PerturbarConsulta(q string, clase ClaseDeTipeo, semilla uint64) string {
	return perturbar(q, clase, semilla, false)
}

// PerturbarTerminoMasLargo es PerturbarConsulta con UN solo error: en el término perturbable más
// largo (el primero, si empatan). Es el brazo «un tipeo» del banco.
func PerturbarTerminoMasLargo(q string, clase ClaseDeTipeo, semilla uint64) string {
	return perturbar(q, clase, semilla, true)
}

// tramo es un término de la consulta: runas[ini:fin], y su índice entre los términos.
type tramo struct{ indice, ini, fin int }

// perturbar tipea los términos perturbables de q: todos, o sólo el más largo.
func perturbar(q string, clase ClaseDeTipeo, semilla uint64, soloElMasLargo bool) string {
	runas := []rune(q)
	var aTipear []tramo
	for _, t := range terminosDe(runas) {
		if !perturbable(runas[t.ini:t.fin]) {
			continue
		}
		switch {
		case !soloElMasLargo:
			aTipear = append(aTipear, t)
		case len(aTipear) == 0 || t.fin-t.ini > aTipear[0].fin-aTipear[0].ini:
			aTipear = []tramo{t}
		}
	}
	if len(aTipear) == 0 {
		return q
	}
	var b strings.Builder
	b.Grow(len(q) + 8)
	copiado := 0 // hasta qué runa de q ya se escribió
	for _, t := range aTipear {
		b.WriteString(string(runas[copiado:t.ini]))
		b.WriteString(string(tipear(runas[t.ini:t.fin], clase, posicionDeTipeo(semilla, q, t.indice))))
		copiado = t.fin
	}
	b.WriteString(string(runas[copiado:]))
	return b.String()
}

// terminosDe corta runas en términos con el mismo criterio que rankedTerms: corridas de letras o
// dígitos.
func terminosDe(runas []rune) []tramo {
	esDeTermino := func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
	var out []tramo
	for i := 0; i < len(runas); {
		if !esDeTermino(runas[i]) {
			i++
			continue
		}
		j := i
		for j < len(runas) && esDeTermino(runas[j]) {
			j++
		}
		out = append(out, tramo{indice: len(out), ini: i, fin: j})
		i = j
	}
	return out
}

// perturbable dice si un término recibe tipeo: todo letras y 5 runas o más.
func perturbable(palabra []rune) bool {
	if len(palabra) < minRunasPerturbables {
		return false
	}
	for _, r := range palabra {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// posicionDeTipeo es el número del que sale dónde cae el error de un término.
func posicionDeTipeo(semilla uint64, q string, termino int) uint64 {
	h := fnv.New64a()
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], semilla)
	_, _ = h.Write(buf[:])
	binary.LittleEndian.PutUint64(buf[:], uint64(termino))
	_, _ = h.Write(buf[:])
	_, _ = h.Write([]byte(q))
	return h.Sum64()
}

// tipear aplica un error de la clase a una palabra de 5 runas o más, sin tocar la primera. Devuelve
// una copia: nunca escribe sobre w.
func tipear(w []rune, clase ClaseDeTipeo, h uint64) []rune {
	n := len(w)
	out := make([]rune, 0, n+1)
	switch clase {
	case TipeoTransposicion:
		// Intercambia w[k] y w[k+1] con k en [1, n-2]. Si las dos letras son iguales el cambio no
		// se ve, así que se prueba la siguiente posición; una palabra sin ningún par distinto
		// (no hay en castellano) queda como estaba.
		for intento := 0; intento < n-2; intento++ {
			k := 1 + int((h+uint64(intento))%uint64(n-2))
			if w[k] != w[k+1] {
				out = append(out, w...)
				out[k], out[k+1] = out[k+1], out[k]
				return out
			}
		}
		return append(out, w...)
	case TipeoFalta:
		k := 1 + int(h%uint64(n-1))
		out = append(out, w[:k]...)
		return append(out, w[k+1:]...)
	case TipeoSobra:
		k := 1 + int(h%uint64(n-1))
		out = append(out, w[:k+1]...)
		out = append(out, w[k])
		return append(out, w[k+1:]...)
	case TipeoSustitucion:
		k := 1 + int(h%uint64(n-1))
		alfa := []rune(alfabetoDeSustitucion)
		i := int((h / uint64(n)) % uint64(len(alfa)))
		nueva := alfa[i]
		if nueva == unicode.ToLower(w[k]) {
			nueva = alfa[(i+1)%len(alfa)]
		}
		if unicode.IsUpper(w[k]) {
			nueva = unicode.ToUpper(nueva)
		}
		out = append(out, w...)
		out[k] = nueva
		return out
	}
	return append(out, w...)
}
