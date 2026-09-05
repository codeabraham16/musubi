package fleet

import (
	"encoding/binary"
	"testing"
	"time"
)

// El parseo del punto fijo es LA trampa de vm.loadavg: leer los uint32 crudos daría cargas de
// varios miles. Como el colector de darwin no se puede correr desde acá, esta prueba es la que
// cubre su parte difícil.
//
// Sabotaje que la hace fallar: devolver los uint32 sin dividir por fscale.
func TestParseLoadavgDivideElPuntoFijo(t *testing.T) {
	// Un loadavg real de macOS: fscale = 2048, y cargas de 1.50 / 2.25 / 0.75.
	const escala = 2048
	b := make([]byte, 20)
	binary.LittleEndian.PutUint32(b[0:], uint32(1.50*escala))
	binary.LittleEndian.PutUint32(b[4:], uint32(2.25*escala))
	binary.LittleEndian.PutUint32(b[8:], uint32(0.75*escala))
	binary.LittleEndian.PutUint64(b[12:], escala)

	l1, l5, l15, ok := parseLoadavg(b)
	if !ok {
		t.Fatal("no parseó un loadavg válido")
	}
	for _, c := range []struct {
		nombre      string
		got, quiero float64
	}{{"l1", l1, 1.50}, {"l5", l5, 2.25}, {"l15", l15, 0.75}} {
		if c.got != c.quiero {
			t.Errorf("%s = %v, esperaba %v — ¿se leyó el uint32 sin dividir por fscale?", c.nombre, c.got, c.quiero)
		}
	}
}

func TestParseLoadavgRechazaBasura(t *testing.T) {
	casos := []struct {
		nombre string
		b      []byte
	}{
		{"buffer corto", make([]byte, 8)},
		{"vacío", nil},
		{"fscale cero", make([]byte, 20)},
	}
	for _, c := range casos {
		if _, _, _, ok := parseLoadavg(c.b); ok {
			t.Errorf("%s: se aceptó un buffer que no es loadavg", c.nombre)
		}
	}
	// Un buffer con basura que da cargas absurdas también se rechaza.
	b := make([]byte, 20)
	binary.LittleEndian.PutUint32(b[0:], 4000000000)
	binary.LittleEndian.PutUint64(b[12:], 1)
	if _, _, _, ok := parseLoadavg(b); ok {
		t.Error("se aceptó una carga de 4.000 millones")
	}
}

// kern.boottime es el INSTANTE del arranque, no los segundos corridos. Confundirlos daría
// uptimes de 55 años.
func TestParseBoottimeDevuelveElInstanteDeArranque(t *testing.T) {
	arranque := time.Now().Add(-3 * time.Hour).Unix()
	b := make([]byte, 16)
	binary.LittleEndian.PutUint64(b[0:], uint64(arranque))
	binary.LittleEndian.PutUint64(b[8:], 123456)

	got, ok := parseBoottime(b)
	if !ok {
		t.Fatal("no parseó un boottime válido")
	}
	if got != arranque {
		t.Fatalf("boottime = %d, esperaba %d", got, arranque)
	}
	// Y el uptime que se deriva es de ~3 horas, no de décadas.
	if up := time.Now().Unix() - got; up < 3*3600-5 || up > 3*3600+5 {
		t.Errorf("uptime derivado = %ds, esperaba ~10800s", up)
	}
}

// Un buffer truncado o con un boottime imposible no produce un uptime inventado.
// `syscall.Sysctl` descarta un NUL final, así que un buffer de 15 bytes es alcanzable.
func TestParseBoottimeRechazaLoImposible(t *testing.T) {
	casos := []struct {
		nombre string
		b      []byte
	}{
		{"buffer corto", make([]byte, 7)},
		{"todo ceros", make([]byte, 16)},
	}
	for _, c := range casos {
		if _, ok := parseBoottime(c.b); ok {
			t.Errorf("%s: se aceptó un boottime que no lo es", c.nombre)
		}
	}
	// Un arranque en el año 2200 tampoco.
	b := make([]byte, 16)
	binary.LittleEndian.PutUint64(b[0:], 7258118400)
	if _, ok := parseBoottime(b); ok {
		t.Error("se aceptó un arranque en el futuro lejano")
	}
	// Pero uno de 15 bytes (con el NUL final comido) SÍ, porque tv_sec entra en los primeros 8.
	corto := make([]byte, 15)
	binary.LittleEndian.PutUint64(corto[0:], uint64(time.Now().Add(-time.Hour).Unix()))
	if _, ok := parseBoottime(corto); !ok {
		t.Error("un buffer de 15 bytes debería servir: tv_sec entra en los primeros 8")
	}
}
