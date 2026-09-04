package fleet

// muestra_techos_test.go custodia que Valida() ponga techo a los campos que llegan CRUDOS de una
// fuente que no controlamos.
//
// La muestra la manda el agente. El agente corre en una máquina ajena y puede estar comprometido:
// el encabezado de muestra.go lo dice con todas las letras, y por eso Valida() existe. Pero una
// guarda con una condición de más se apaga sola justo en el caso raro, y el caso raro es el que
// va a mandar el que quiera romperla.

import (
	"strings"
	"testing"
	"time"
)

// TestLaRamLibreTieneTechoAunqueElTotalLlegueEnCero.
//
// La guarda original decía `m.MemLibre != nil && m.MemTotal > 0 && *m.MemLibre > m.MemTotal`. Con
// `mem_total: 0` la condición del medio la desactivaba entera, y `{"mem_total":0,
// "mem_libre":9223372036854775808}` pasaba como muestra válida — lo encontró la verificación
// adversarial, no una prueba. Con el total en cero, «libre» sólo puede ser cero: no hay memoria
// de la que sobre.
//
// Sabotaje que la hace fallar: devolver `m.MemTotal > 0 &&` a la condición.
func TestLaRamLibreTieneTechoAunqueElTotalLlegueEnCero(t *testing.T) {
	casos := []struct {
		nombre string
		total  uint64
		libre  uint64
		valida bool
	}{
		{"el caso que se colaba: total 0 y libre absurdo", 0, 1 << 63, false},
		{"total 0 y libre 0 es coherente", 0, 0, true},
		{"libre por encima del total, con total real", 8 << 30, 9 << 30, false},
		{"libre por debajo del total", 8 << 30, 2 << 30, true},
		{"libre igual al total: una máquina recién arrancada", 8 << 30, 8 << 30, true},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			libre := c.libre
			m := muestraMinimaValida()
			m.MemTotal, m.MemLibre = c.total, &libre
			if c.total > 0 {
				m.MemUsada = c.total / 2
			} else {
				m.MemUsada = 0
			}
			err := m.Valida()
			if c.valida && err != nil {
				t.Errorf("se rechazó una muestra legítima: %v", err)
			}
			if !c.valida && err == nil {
				t.Errorf("Valida() aceptó mem_total=%d con mem_libre=%d: la muestra es entrada NO confiable y este campo quedó sin techo", c.total, c.libre)
			}
		})
	}
}

// muestraMinimaValida arma lo mínimo que Valida() acepta, para que cada caso de arriba falle por
// LO QUE PRUEBA y no por un campo suelto que faltaba.
func muestraMinimaValida() Muestra {
	return Muestra{
		// `tomada` va acá y no en cada caso porque Valida() la EXIGE: una muestra sin hora no se
		// puede fechar, y la antigüedad se calcularía contra el año 1.
		Tomada:          time.Now().UTC(),
		MemTotal:        8 << 30,
		MemUsada:        4 << 30,
		DiscoTotal:      100 << 30,
		DiscoUsado:      50 << 30,
		DiscoDisponible: 50 << 30,
		NumCPU:          4,
	}
}

// UNA MUESTRA SIN `tomada` SE RECHAZA EN LA PUERTA.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// ERA EL ÚNICO CAMPO SIN GUARDA, Y ES EL QUE DECIDE SI TODO EL RESTO VALE
//
// `SaludServicio.Valida` ya lo exigía —con estas mismas palabras— y la muestra del HOST no, pese
// a entrar por la misma puerta no confiable: el cerebro DESERIALIZA lo que manda el agente y no
// sella la hora del lado del servidor. Un latido con `{"muestra":{"cpu_pct":5}}` se guardaba con
// `tomada` en cero.
//
// Lo que eso produce, medido sobre el código de las tres superficies que la consumen:
//
//	musubi_fleet_metrics  →  antiguedad_s: 9223372036, sin ninguna guarda que lo tape.
//	motor de políticas    →  la ve vieja por dos mil años y deja de actuar sobre esa máquina,
//	                         en silencio (fail-closed, pero mudo).
//	exportador            →  ÉSTE SÍ se protege (`!m.Tomada.IsZero()` omite la serie).
//
// Esa asimetría es la pista entera: una superficie lo sabía y las otras dos no. Se rechaza en la
// puerta, que es donde alcanza con arreglarlo una vez — y la muestra inválida se descarta ENTERA
// (D7) mientras el latido sigue valiendo: estar viva y saber medirse son cosas distintas.
//
// Sabotaje que la hace fallar: sacar el `if m.Tomada.IsZero()` de Valida.
func TestUnaMuestraSinHoraNoSeGuarda(t *testing.T) {
	sinHora := muestraMinimaValida()
	sinHora.Tomada = time.Time{}
	err := sinHora.Valida()
	if err == nil {
		t.Fatal("se aceptó una muestra sin `tomada`: su antigüedad se calcula contra el año 1 y `musubi_fleet_metrics` contesta 9223372036 segundos sin que nada lo tape")
	}
	if !strings.Contains(err.Error(), "tomada") {
		t.Errorf("el motivo del rechazo no nombra el campo: %v", err)
	}

	// Y la de al lado, para que quede claro que es la MISMA regla en las dos: la salud de un
	// servicio ya la exigía, y esta prueba se rompería si alguien la aflojara ahí.
	salud := SaludServicio{Estado: EstadoCorriendo}
	if salud.Valida() == nil {
		t.Error("SaludServicio aceptó una salud sin `tomada`: las dos entradas no confiables tienen que pedir lo mismo")
	}

	// El caso legítimo sigue pasando: esto no es un techo nuevo sobre datos buenos.
	if err := muestraMinimaValida().Valida(); err != nil {
		t.Errorf("se rechazó una muestra con hora: %v", err)
	}
}

// EL NÚMERO QUE JUSTIFICA LA GUARDA ES EL QUE EL CÓDIGO PRODUCE, NO LA RESTA.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// El comentario de Valida() y el de la prueba de arriba citan `antiguedad_s: 9223372036` como el
// disparate que se veía sin la guarda. La primera versión decía 63882345600 —la diferencia real
// en segundos contra el año 1— y ESE número el código no lo puede producir: `time.Time.Sub`
// devuelve un `time.Duration`, que son nanosegundos en un int64 y SATURA a los ~292 años.
//
// Una medición citada en un comentario es una afirmación como cualquier otra, y ésta era falsa
// por un factor de siete. La encontró una revisión adversaria del propio lote, no una prueba: por
// eso ahora hay una.
//
// Sabotaje que la hace fallar: cambiar el número esperado por la resta cruda (63882345600).
func TestLaAntiguedadDeUnaMuestraSinHoraSatura(t *testing.T) {
	// La MISMA cuenta que hace musubi_fleet_metrics: ahora.Sub(m.Tomada).Seconds().
	var sinHora Muestra
	antiguedad := int(time.Now().UTC().Sub(sinHora.Tomada).Seconds())

	const saturado = 9223372036 // maxInt64 nanosegundos, en segundos
	if antiguedad != saturado {
		t.Errorf("la antigüedad de una muestra sin hora dio %d y se documentó %d: el comentario de Valida() cita un número que el código no produce", antiguedad, saturado)
	}
	// Y el que NO es: la resta cruda contra el año 1. Está acá para que se vea que la diferencia
	// no es un detalle de redondeo sino un factor de siete.
	const restaCruda = 63882345600
	if antiguedad == restaCruda {
		t.Error("dio la resta cruda: entonces time.Duration dejó de saturar y los comentarios hay que revisarlos")
	}
}
