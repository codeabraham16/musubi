package fleet

import "testing"

// La aritmética del porcentaje, probada en cualquier plataforma. Es lo que hace que el colector
// de Windows —que no se puede correr desde acá— tenga su parte difícil cubierta.
func TestElDeltaDeCPUDevuelveNilCuandoNoSabe(t *testing.T) {
	casos := []struct {
		nombre       string
		lecturas     [][2]uint64 // (ocupado, total)
		quieroUltimo *float64
		porque       string
	}{
		{"primera lectura", [][2]uint64{{100, 1000}}, nil,
			"no hay contra qué restar"},
		{"segunda lectura normal", [][2]uint64{{100, 1000}, {150, 1100}}, f64(50),
			"50 de 100 jiffies nuevos estuvieron ocupados"},
		{"el total no avanzó", [][2]uint64{{100, 1000}, {100, 1000}}, nil,
			"dos lecturas en el mismo tick: dividir daría NaN"},
		{"los contadores retrocedieron", [][2]uint64{{500, 5000}, {10, 100}}, nil,
			"reinicio entre latidos: 0 dibujaría una caída falsa"},
		{"el ocupado retrocedió solo", [][2]uint64{{500, 5000}, {400, 6000}}, nil,
			"contador inconsistente"},
		{"ocupado igual a total", [][2]uint64{{100, 1000}, {1100, 2000}}, f64(100),
			"todo el intervalo ocupado"},
		{"nada ocupado", [][2]uint64{{100, 1000}, {100, 2000}}, f64(0),
			"0 % MEDIDO es legítimo: hubo delta y fue cero"},
	}
	for _, c := range casos {
		var cnt contadorCPU
		var got *float64
		for _, l := range c.lecturas {
			got = cnt.delta(l[0], l[1])
		}
		switch {
		case c.quieroUltimo == nil && got != nil:
			t.Errorf("%s: obtuve %v, esperaba nil (%s)", c.nombre, *got, c.porque)
		case c.quieroUltimo != nil && got == nil:
			t.Errorf("%s: obtuve nil, esperaba %v (%s)", c.nombre, *c.quieroUltimo, c.porque)
		case c.quieroUltimo != nil && *got != *c.quieroUltimo:
			t.Errorf("%s: obtuve %v, esperaba %v (%s)", c.nombre, *got, *c.quieroUltimo, c.porque)
		}
	}
}

// Tras un retroceso, la base se REARMA: la lectura siguiente vuelve a dar número.
// Sabotaje: no actualizar el estado en el camino de retorno nil → el colector queda mudo para
// siempre después de un solo reinicio.
func TestTrasUnRetrocesoElContadorSeRearma(t *testing.T) {
	var c contadorCPU
	c.delta(500, 5000) // base
	if got := c.delta(10, 100); got != nil {
		t.Fatalf("el retroceso debería dar nil, dio %v", *got)
	}
	got := c.delta(60, 200) // 50 de 100 nuevos
	if got == nil {
		t.Fatal("tras un retroceso, el contador quedó mudo: no rearmó la base")
	}
	if *got != 50 {
		t.Errorf("obtuve %v, esperaba 50", *got)
	}
}

// Un 0 % MEDIDO no es lo mismo que un 0 % inventado, y el tipo tiene que poder distinguirlos.
func TestUnCeroMedidoNoEsLoMismoQueNoSaber(t *testing.T) {
	var c contadorCPU
	if c.delta(100, 1000) != nil {
		t.Fatal("la primera lectura no puede dar número")
	}
	got := c.delta(100, 2000) // el total avanzó, el ocupado no
	if got == nil {
		t.Fatal("con delta de total y ocupado quieto, 0 % es una MEDICIÓN, no un desconocido")
	}
	if *got != 0 {
		t.Errorf("obtuve %v, esperaba 0", *got)
	}
}
