package fleet

// muestra_techos_test.go custodia que Valida() ponga techo a los campos que llegan CRUDOS de una
// fuente que no controlamos.
//
// La muestra la manda el agente. El agente corre en una máquina ajena y puede estar comprometido:
// el encabezado de muestra.go lo dice con todas las letras, y por eso Valida() existe. Pero una
// guarda con una condición de más se apaga sola justo en el caso raro, y el caso raro es el que
// va a mandar el que quiera romperla.

import "testing"

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
		MemTotal:        8 << 30,
		MemUsada:        4 << 30,
		DiscoTotal:      100 << 30,
		DiscoUsado:      50 << 30,
		DiscoDisponible: 50 << 30,
		NumCPU:          4,
	}
}
