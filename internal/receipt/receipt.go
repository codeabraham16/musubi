// Package receipt implementa el RECIBO: un permiso de entrega atado a una HUELLA del árbol de
// trabajo. Es la pieza mínima de RDD (Receipt Driven Development) y la única que Musubi no tenía.
//
// EL PROBLEMA QUE RESUELVE. Una revisión —humana o de un agente— es una OPINIÓN sobre un estado
// que ya cambió para cuando se entrega. «Lo revisé» no dice SOBRE QUÉ. El recibo convierte esa
// opinión en una AUTORIDAD VERIFICABLE Y CADUCABLE: dice «para esta huella exacta, el estado es
// aprobado». Si cambia un byte del árbol, la huella cambia y el recibo deja de valer solo. Nadie
// tiene que acordarse de invalidarlo.
//
// POR QUÉ ENCAJA CON LA REGLA DE ORO DEL REPO (la última palabra es del usuario): un recibo es
// exactamente el mecanismo que hace VINCULANTE una decisión humana. Sin él, la decisión sobrevive
// sólo mientras el agente se acuerde de respetarla.
//
// MODEL-FREE Y DETERMINISTA. Acá no hay ningún juicio: esto sólo hashea, compara y responde sí o
// no. Quién decide si el candidato está aprobado es otro (una persona, un agente, los lentes de
// revisión). Separar el JUICIO de la AUTORIDAD es lo que hace auditable al mecanismo.
package receipt

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MetaKey es la clave de `meta` donde vive el recibo vigente del workspace. Hay UNO solo a la vez:
// el modelo de RDD es un candidato por vez, no una pila de permisos.
const MetaKey = "rdd_receipt"

// Veredictos posibles. `approved` es el único que habilita la entrega; cualquier otro valor —o la
// ausencia de recibo— la bloquea. El default seguro es NO tener permiso.
const (
	Approved = "approved"
	Rejected = "rejected"
)

// Receipt es el permiso de entrega. Se serializa a JSON en la tabla meta.
type Receipt struct {
	// Fingerprint es la huella del árbol que se revisó. Es el corazón: el recibo vale para ESTA
	// huella y para ninguna otra.
	Fingerprint string `json:"fingerprint"`
	// Head es el commit sobre el que se emitió, sólo informativo (para que un humano se ubique).
	Head string `json:"head"`
	// Verdict es Approved o Rejected.
	Verdict string `json:"verdict"`
	// Reason es la justificación breve de quien emitió el veredicto.
	Reason string `json:"reason,omitempty"`
	// IssuedBy identifica a quien lo emitió (agente, persona, lente). Sin autoridad anónima.
	IssuedBy string `json:"issued_by,omitempty"`
	// IssuedAt es cuándo se emitió.
	IssuedAt time.Time `json:"issued_at"`
}

// Compute deriva la huella del árbol a partir de las tres piezas que definen su estado:
//
//   - head: el commit actual (qué base).
//   - diff: el diff COMPLETO contra HEAD, staged y sin stagear (qué cambió encima).
//   - untracked: la lista de archivos sin trackear (qué apareció y todavía no está en el diff).
//
// Las tres hacen falta. Sólo con `head` un recibo sobreviviría a cualquier edición sin commitear;
// sólo con el diff, sobreviviría a un rebase que cambia la base bajo los mismos cambios; y sin los
// untracked, agregar un archivo nuevo no invalidaría nada — que es justo como se cuela código sin
// revisar.
//
// Es pura a propósito: no toca git ni el disco, así que se puede testear sin repositorio.
func Compute(head, diff, untracked string) string {
	h := sha256.New()
	// Los separadores con longitud evitan la ambigüedad de concatenar: sin ellos, mover un byte
	// del final de un campo al principio del siguiente daría la MISMA huella para dos árboles
	// distintos.
	for _, parte := range []string{head, diff, untracked} {
		fmt.Fprintf(h, "%d:", len(parte))
		h.Write([]byte(parte))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Decision es el resultado de evaluar si la entrega puede pasar. Lleva el motivo porque un gate
// que sólo dice «no» obliga a adivinar, y quien lo lee suele ser un agente.
type Decision struct {
	Allowed bool
	Reason  string
}

// Check decide si el árbol con huella `actual` puede entregarse según el recibo guardado.
//
// Los tres motivos de rechazo son distintos a propósito: «no hay recibo» pide emitir uno, «el
// veredicto es rechazado» pide arreglar, y «la huella no coincide» pide RE-revisar. Colapsarlos en
// un solo mensaje haría que el agente reintente la acción equivocada.
func Check(r *Receipt, actual string) Decision {
	if r == nil || strings.TrimSpace(r.Fingerprint) == "" {
		return Decision{false, "no hay recibo para este árbol: nadie revisó este candidato todavía"}
	}
	if r.Verdict != Approved {
		motivo := r.Reason
		if motivo == "" {
			motivo = "sin motivo registrado"
		}
		return Decision{false, "el recibo está en " + r.Verdict + ": " + motivo}
	}
	if r.Fingerprint != actual {
		return Decision{false, "el árbol cambió después de la revisión (huella " + short(actual) +
			", el recibo aprueba " + short(r.Fingerprint) + "): el permiso venció, hay que revisar de nuevo"}
	}
	return Decision{true, "recibo válido para " + short(actual)}
}

// short recorta una huella para los mensajes. Doce caracteres alcanzan para distinguir dos estados
// del mismo árbol y no ensucian la salida.
func short(fp string) string {
	if len(fp) <= 12 {
		return fp
	}
	return fp[:12]
}

// Encode serializa el recibo para guardarlo en meta.
func Encode(r Receipt) (string, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return "", fmt.Errorf("no pude serializar el recibo: %w", err)
	}
	return string(b), nil
}

// Decode lee el recibo de meta. Un valor vacío es «no hay recibo», no un error: es el estado
// normal de un workspace donde todavía nadie revisó nada.
//
// Un valor CORRUPTO tampoco es un error para el caller: se trata como ausencia. El default seguro
// de este mecanismo es no tener permiso, así que degradar hacia «no hay recibo» falla cerrado.
func Decode(raw string) *Receipt {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var r Receipt
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil
	}
	return &r
}
