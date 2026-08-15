package receipt

import (
	"strings"
	"testing"
	"time"
)

func TestComputeEsDeterministaYSensibleALasTresPiezas(t *testing.T) {
	base := Compute("headA", "diffA", "untrackedA")

	if base != Compute("headA", "diffA", "untrackedA") {
		t.Error("la misma entrada tiene que dar la misma huella")
	}

	// Las tres piezas hacen falta: si alguna no participara, el recibo sobreviviría a un cambio
	// real del árbol. Cada caso de acá corresponde a una forma concreta de colar código sin revisar.
	casos := map[string]string{
		"cambió el commit base (rebase bajo los mismos cambios)": Compute("headB", "diffA", "untrackedA"),
		"cambió el diff (una edición encima)":                    Compute("headA", "diffB", "untrackedA"),
		"apareció un archivo sin trackear":                       Compute("headA", "diffA", "untrackedB"),
	}
	for que, fp := range casos {
		if fp == base {
			t.Errorf("la huella no cambió cuando %s: el recibo sobreviviría", que)
		}
	}
}

// Sin separadores con longitud, mover un byte del final de un campo al principio del siguiente da
// la MISMA concatenación y por lo tanto la misma huella — dos árboles distintos con el mismo
// permiso. Este test fija que eso no pasa.
func TestComputeNoConfundeLimitesEntreCampos(t *testing.T) {
	a := Compute("ab", "c", "d")
	b := Compute("a", "bc", "d")
	if a == b {
		t.Error("dos estados distintos comparten huella: los campos se concatenan sin delimitar")
	}
}

func TestCheckSinReciboNoDejaPasar(t *testing.T) {
	d := Check(nil, "abc")
	if d.Allowed {
		t.Error("sin recibo la entrega tiene que estar bloqueada: el default seguro es no tener permiso")
	}
	if !strings.Contains(d.Reason, "nadie revisó") {
		t.Errorf("el motivo no orienta a emitir un recibo: %q", d.Reason)
	}

	// Un recibo con la huella vacía es basura, no un permiso.
	if Check(&Receipt{Verdict: Approved}, "abc").Allowed {
		t.Error("un recibo sin huella no puede aprobar nada")
	}
}

func TestCheckRechazadoNoDejaPasarYDiceElMotivo(t *testing.T) {
	r := &Receipt{Fingerprint: "abc", Verdict: Rejected, Reason: "toca auth sin tests"}
	d := Check(r, "abc")
	if d.Allowed {
		t.Error("un recibo rechazado no habilita la entrega aunque la huella coincida")
	}
	if !strings.Contains(d.Reason, "toca auth sin tests") {
		t.Errorf("el motivo del rechazo se perdió: %q", d.Reason)
	}
}

// EL CORAZÓN DEL MECANISMO: cambia un byte, muere el recibo.
func TestCheckHuellaDistintaVenceElPermiso(t *testing.T) {
	r := &Receipt{Fingerprint: Compute("head", "diff", ""), Verdict: Approved}

	if !Check(r, r.Fingerprint).Allowed {
		t.Fatal("con la huella exacta el recibo tiene que valer")
	}

	otra := Compute("head", "diff con un byte mas", "")
	d := Check(r, otra)
	if d.Allowed {
		t.Error("el recibo aprobó un árbol distinto del que se revisó")
	}
	// El motivo tiene que mandar a RE-REVISAR, no a emitir de cero ni a arreglar: son acciones
	// distintas y quien lee esto suele ser un agente.
	if !strings.Contains(d.Reason, "cambió después de la revisión") {
		t.Errorf("el motivo no explica que el permiso venció: %q", d.Reason)
	}
}

func TestEncodeDecodeIdaYVuelta(t *testing.T) {
	orig := Receipt{
		Fingerprint: Compute("h", "d", "u"),
		Head:        "1242f6f",
		Verdict:     Approved,
		Reason:      "cuatro lentes en verde",
		IssuedBy:    "adversarial-review",
		IssuedAt:    time.Now().UTC().Truncate(time.Second),
	}
	raw, err := Encode(orig)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	got := Decode(raw)
	if got == nil {
		t.Fatal("Decode devolvió nil sobre un recibo válido")
	}
	if got.Fingerprint != orig.Fingerprint || got.Verdict != orig.Verdict ||
		got.Reason != orig.Reason || got.IssuedBy != orig.IssuedBy {
		t.Errorf("el recibo no sobrevivió el viaje: %+v", got)
	}
}

// FALLA CERRADO: un valor corrupto en meta se trata como ausencia de recibo, no como error que
// nadie mira. La consecuencia correcta de no saber es no dejar pasar.
func TestDecodeToleraBasuraYFallaCerrado(t *testing.T) {
	for _, raw := range []string{"", "   ", "no soy json", "{", `{"fingerprint":123}`} {
		r := Decode(raw)
		if r != nil {
			t.Errorf("Decode(%q) devolvió un recibo en vez de nil: %+v", raw, r)
		}
		if Check(r, "abc").Allowed {
			t.Errorf("con meta corrupto (%q) la entrega quedó habilitada", raw)
		}
	}
}
