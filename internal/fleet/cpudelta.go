package fleet

// cpudelta.go es la ARITMÉTICA del porcentaje de CPU, separada de cómo cada sistema operativo
// entrega los contadores.
//
// Está en un archivo SIN build tag a propósito. Linux lee /proc/stat y Windows llama a
// GetSystemTimes: son fuentes distintas, pero los dos entregan lo mismo —contadores ACUMULADOS de
// tiempo ocupado y tiempo total— y las dos derivadas se calculan igual. Con el cálculo acá:
//
//   - Se prueba de verdad, en cualquier plataforma. La parte que NO se puede probar desde Linux
//     (la llamada a kernel32) queda reducida a leer dos números y pasarlos.
//   - No se duplica la lógica de saturación y reinicio en dos archivos que nadie compila a la vez.

// contadorCPU acumula el estado entre lecturas. El porcentaje es una DERIVADA: sin lectura
// anterior no hay número, y ésa es la razón de que el tipo tenga estado.
type contadorCPU struct {
	previoOcupado uint64
	previoTotal   uint64
	tienePrevio   bool
}

// delta incorpora una lectura y devuelve el porcentaje del intervalo, o nil si todavía no hay
// contra qué restar.
//
// Devuelve nil —y no 0— en tres casos, y los tres son «no lo sé», no «está ocioso»:
//
//   - Es la primera lectura.
//   - El total no avanzó: dos lecturas dentro del mismo tick del reloj del kernel. Dividir por
//     cero daría NaN, y un NaN en Prometheus es peor que una serie ausente.
//   - Los contadores RETROCEDIERON. Pasa de verdad: un contador que da la vuelta, un reinicio
//     entre latidos, o una VM migrada. Antes esto se acotaba a 0, que dibujaba una caída a cero
//     falsa justo después de un reinicio — exactamente el artefacto que el resto del diseño
//     evita. Ahora se descarta la muestra y se rearma la base.
func (c *contadorCPU) delta(ocupado, total uint64) *float64 {
	previoOcupado, previoTotal, tenia := c.previoOcupado, c.previoTotal, c.tienePrevio
	c.previoOcupado, c.previoTotal, c.tienePrevio = ocupado, total, true

	if !tenia || total <= previoTotal || ocupado < previoOcupado {
		return nil
	}
	dTotal := total - previoTotal
	dOcupado := ocupado - previoOcupado
	pct := float64(dOcupado) / float64(dTotal) * 100
	// El ocupado no puede superar al total, pero un kernel con contadores inconsistentes existe.
	// Acotar acá es correcto: el valor SÍ se midió, sólo que el techo es 100.
	if pct > 100 {
		pct = 100
	}
	return &pct
}

// ContadorCPUExportado es `contadorCPU` para los otros paquetes.
//
// El cerebro necesita uno POR DISPOSITIVO remoto: sin agente que recuerde la lectura anterior, el
// estado de la derivada lo lleva él. Se exporta el tipo y no el campo interno para que nadie
// pueda fabricar un estado a mano.
type ContadorCPUExportado struct{ c contadorCPU }

// Delta incorpora una lectura y devuelve el porcentaje del intervalo, o nil si no hay contra qué
// restar.
func (e *ContadorCPUExportado) Delta(ocupado, total uint64) *float64 {
	return e.c.delta(ocupado, total)
}
