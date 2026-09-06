package receipt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// finding.go aplica RDD a los HALLAZGOS de un panel: un hallazgo se emite en un momento y se
// juzga en otro, con una ronda de crítica cruzada en el medio, y entre esos dos momentos nada
// garantizaba que el texto siguiera siendo el mismo.
//
// EL AGUJERO CONCRETO. `PostPosture` hace `ON CONFLICT ... DO UPDATE SET stance=excluded.stance`:
// re-postear con la misma etiqueta REEMPLAZA la postura anterior en silencio, sin error y sin
// rastro. El tally es determinista sobre los VOTOS, pero no sobre el TEXTO que esos votos
// juzgaban — así que el recuento puede ser perfectamente fiel a una discusión que ya no existe.
//
// LA FORMA: UNA TRIPLETA. Un hallazgo congelado es un id estable, la huella de su cuerpo
// canonizado, y la huella del árbol contra el que se emitió. El CUERPO NO SE GUARDA, sólo su
// huella — y eso no es ahorro de espacio: obliga a que quien verifica tenga el texto en la mano,
// que es justamente el punto. Un verificador que puede leer el texto del propio registro no está
// verificando nada, está mirándose al espejo.
//
// 🔴 ESTE ARCHIVO NO TOCA Compute NI Check. El hook pre-push está instalado y vivo: cualquier
// cambio ahí toca el camino que decide si esta gente puede pushear, y si la salida de Compute
// cambiara un solo byte, TODOS los recibos vigentes se invalidarían y los push se bloquearían.
// El plan proponía generalizar la aridad de Compute; no se hizo, y la razón es que su propia
// regla es más fuerte: la función nueva va aparte aunque duplique unas líneas. Acá se duplica el
// esquema de longitud-prefijada en `huellaDe`, a propósito, y `compute_golden_test.go` congela la
// salida de Compute con hexes literales para que un refactor futuro no la mueva sin darse cuenta.

// FindingsMetaKey es la clave de `meta` donde vive la COLECCIÓN de hallazgos congelados.
//
// Es una colección y no una clave única como MetaKey, y la diferencia importa: el recibo del
// árbol es «un candidato por vez, no una pila de permisos», pero un panel emite N hallazgos a la
// vez y todos tienen que poder congelarse por separado.
const FindingsMetaKey = "rdd_findings"

// Finding es un hallazgo congelado.
type Finding struct {
	// ID identifica al hallazgo de forma estable entre el momento en que se emite y el momento
	// en que se vota. La convención es `<debate_id>/<lente>`.
	ID string `json:"id"`
	// BodyHash es la huella del cuerpo CANONIZADO. El cuerpo no está: ver el comentario de arriba.
	BodyHash string `json:"body_hash"`
	// Tree es la huella del árbol contra el que se emitió el hallazgo.
	Tree string `json:"tree"`
	// FrozenAt es cuándo se congeló.
	FrozenAt time.Time `json:"frozen_at"`
}

// CanonizarCuerpo normaliza el cuerpo de un hallazgo antes de hashearlo. Normaliza LOS FINALES
// DE LÍNEA Y NADA MÁS, y las dos mitades de esa frase son igual de importantes:
//
//   - SIN normalizar CRLF, en Windows cada verificación diría «mutó» —el mismo texto guardado por
//     dos editores distintos da dos huellas— y el mecanismo se apaga solo en una semana.
//   - Normalizando DE MÁS —espacio interno, minúsculas, puntuación— dos hallazgos con significados
//     distintos darían la misma huella, y el congelado pasaría a APROBAR MUTACIONES REALES. «El
//     índice puede estar vacío» y «el índice no puede estar vacío» no son el mismo hallazgo.
//
// Por eso el banco sabotea las dos mitades: una hace que el mecanismo se vuelva insoportable, la
// otra hace que mienta. La segunda es peor.
func CanonizarCuerpo(cuerpo string) string {
	// El salto FINAL tambien se recorta, y es una extension deliberada del «solo CRLF» del
	// plan. El criterio que la justifica es el mismo que prohibe el resto: se normaliza
	// unicamente lo que NO PUEDE cambiar el significado. Un salto al final no puede.
	//
	// Y sin esto el mecanismo tiene un filo que muerde el primer dia: `echo` agrega un salto
	// final, `printf` no, y un archivo puede tener o no tener el suyo. El mismo hallazgo
	// congelado desde un archivo y verificado desde la terminal daria «muto» — que es
	// exactamente la forma en que un mecanismo asi se apaga en una semana.
	//
	// Lo que NO se toca: el espacio INTERNO, las mayusculas y la puntuacion. Ahi si vive el
	// significado, y normalizarlos haria que el congelado apruebe mutaciones reales.
	return strings.TrimRight(strings.ReplaceAll(cuerpo, "\r\n", "\n"), "\n")
}

// huellaDe hashea con el mismo esquema de longitud-prefijada que Compute, DUPLICADO a propósito
// para no tocar la función de la que dependen los recibos vivos. Ver la cabecera del archivo.
func huellaDe(partes ...string) string {
	h := sha256.New()
	for _, p := range partes {
		fmt.Fprintf(h, "%d:", len(p))
		h.Write([]byte(p))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// HashCuerpo devuelve la huella del cuerpo canonizado de un hallazgo.
func HashCuerpo(cuerpo string) string { return huellaDe(CanonizarCuerpo(cuerpo)) }

// Congelar arma un hallazgo congelado. `tree` es la huella del árbol de ese momento.
func Congelar(id, cuerpo, tree string, cuando time.Time) (Finding, error) {
	if strings.TrimSpace(id) == "" {
		return Finding{}, fmt.Errorf("un hallazgo congelado necesita un id estable (convención: <debate_id>/<lente>)")
	}
	if strings.TrimSpace(cuerpo) == "" {
		return Finding{}, fmt.Errorf("no se puede congelar un hallazgo vacío: no habría nada que verificar después")
	}
	return Finding{ID: id, BodyHash: HashCuerpo(cuerpo), Tree: tree, FrozenAt: cuando.UTC()}, nil
}

// Verificar decide si un cuerpo y un árbol corresponden al hallazgo congelado con ese id.
//
// LOS TRES MOTIVOS DE RECHAZO SON DISTINTOS A PROPÓSITO, porque cada uno pide una ACCIÓN
// distinta y quien los lee suele ser un agente:
//
//   - «no hay hallazgo congelado con ese id» pide CONGELAR;
//   - «el cuerpo cambió» pide RE-EMITIR o restaurar el texto, porque el veredicto no vale sobre
//     otro texto;
//   - «el árbol cambió» pide RE-VERIFICAR contra el código de hoy.
//
// Colapsarlos en un solo mensaje haría que el agente reintente la acción equivocada, que es el
// mismo defecto que Check ya evita para los recibos del árbol.
func Verificar(cong map[string]Finding, id, cuerpo, treeActual string) Decision {
	f, ok := cong[id]
	if !ok {
		return Decision{false, "no hay hallazgo congelado con id " + id + ": congelalo antes de someterlo a voto"}
	}
	if h := HashCuerpo(cuerpo); h != f.BodyHash {
		return Decision{false, "el cuerpo del hallazgo " + id + " cambió después de congelarse (ahora " +
			short(h) + ", se congeló " + short(f.BodyHash) + "): el veredicto no vale sobre otro texto — " +
			"re-emitilo o restaurá el texto original"}
	}
	if f.Tree != treeActual {
		return Decision{false, "el hallazgo " + id + " se congeló contra otro árbol (hoy " + short(treeActual) +
			", se congeló " + short(f.Tree) + "): el código cambió debajo, hay que re-verificarlo"}
	}
	return Decision{true, "el hallazgo " + id + " sigue siendo el que se congeló, sobre el mismo árbol"}
}

// PodarPorArbol descarta los hallazgos congelados contra un árbol distinto del actual y devuelve
// cuántos sacó.
//
// SINERGIA CON EL ALCANCE DECRECIENTE: después de un fix el árbol cambia, así que los hallazgos
// de la vuelta anterior se caen SOLOS y la vuelta siguiente sólo congela sobre el código de hoy.
// Es media respuesta al «a la vuelta k+1 sólo van los hallazgos abiertos», sin que nadie tenga
// que acordarse de limpiar.
func PodarPorArbol(cong map[string]Finding, treeActual string) (map[string]Finding, int) {
	vivos := make(map[string]Finding, len(cong))
	for id, f := range cong {
		if f.Tree == treeActual {
			vivos[id] = f
		}
	}
	return vivos, len(cong) - len(vivos)
}

// EncodeFindings serializa la colección de forma DETERMINISTA (ids ordenados): dos colecciones
// iguales tienen que dar el mismo texto, o cada guardado se ve como un cambio.
func EncodeFindings(cong map[string]Finding) (string, error) {
	ids := make([]string, 0, len(cong))
	for id := range cong {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	lista := make([]Finding, 0, len(ids))
	for _, id := range ids {
		lista = append(lista, cong[id])
	}
	b, err := json.Marshal(lista)
	if err != nil {
		return "", fmt.Errorf("no pude serializar los hallazgos congelados: %w", err)
	}
	return string(b), nil
}

// DecodeFindings lee la colección. Un valor vacío o CORRUPTO es «no hay nada congelado», no un
// error: el default seguro de este mecanismo es no tener nada verificado, así que degradar hacia
// la colección vacía falla CERRADO — todo hallazgo pasa a necesitar que lo congelen de nuevo.
func DecodeFindings(raw string) map[string]Finding {
	out := map[string]Finding{}
	if strings.TrimSpace(raw) == "" {
		return out
	}
	var lista []Finding
	if err := json.Unmarshal([]byte(raw), &lista); err != nil {
		return out
	}
	for _, f := range lista {
		if f.ID != "" {
			out[f.ID] = f
		}
	}
	return out
}
