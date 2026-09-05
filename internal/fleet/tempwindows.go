package fleet

// tempwindows.go — la aritmética de la zona térmica de Windows, FUERA del build tag.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// VIVE ACÁ Y NO EN colector_windows.go, Y HOY ESO NO ES UNA PREFERENCIA
//
// Es la tercera vez en este repo que un símbolo detrás de `//go:build windows` deja sin
// verificar al paquete entero en las plataformas donde no compila: pasó con `estadoDeSystemd`,
// pasó otra vez con `propiedadesPedidas` —y esa segunda la encontró una revisión, no una prueba,
// porque `go build` NO COMPILA LOS TESTS y daba verde en las tres plataformas—.
//
// Lo que va detrás del tag es SÓLO qué se ejecuta. Lo que se puede probar desde cualquier
// máquina, se prueba desde cualquier máquina.

import (
	"strconv"
	"strings"
)

// La banda de plausibilidad —TempMinPlausibleC / TempMaxPlausibleC— está en procparse.go, al
// lado de ParsearTempMiligrados, y la comparten LOS DOS parsers. Estuvo declarada acá y en
// procparse.go a la vez durante un rato: dos constantes con el mismo número es cómo una se
// queda vieja.

// ParsearTempDecikelvin convierte la salida de `MSAcpi_ThermalZoneTemperature`.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// LA UNIDAD ES DÉCIMAS DE KELVIN, Y ÉSE ES EL ERROR QUE HAY QUE NO COMETER
//
// `CurrentTemperature` NO viene en grados: viene en decikelvin. Una lectura de 3032 son 30,05 °C,
// y leída como Celsius directo daría «3032 grados» — que al menos es absurdo y se nota. El error
// caro es el otro: restar 273,15 SIN dividir por 10 da −243, que pasa por «bajo cero» y se
// dibuja como una máquina congelada en vez de como una unidad mal leída.
//
// SE TOMA LA MÁS ALTA DE LAS PLAUSIBLES, y eso CAMBIÓ el 2026-09-05 junto con el lado de Linux.
//
// Este comentario decía lo contrario —«la primera zona plausible, no el máximo, para que
// signifique LO MISMO que en Linux, que lee thermal_zone0»— y el razonamiento era correcto: una
// serie que cambia de definición entre máquinas es peor que una que falta, porque el gráfico las
// apila igual. Lo que se rompió es la premisa: Linux dejó de leer `thermal_zone0` y ahora elige
// por `type`, con la más alta que no sea `acpitz` como fallback. El invariante que esta función
// decía sostener quedó falso el mismo día en que se escribió, y así se mantiene: Windows no
// expone un `type` con el que preferir, así que la regla análoga —la única que puede aplicar— es
// justo el fallback de Linux. `Muestra.TempC` vuelve a significar una cosa sola en los dos SO.
//
// LA DIRECCIÓN DEL ERROR TAMBIÉN ES LA MISMA: equivocarse alto se investiga, equivocarse bajo no
// se ve. El techo de TempMaxPlausibleC es lo que impide que eso se vuelva una falsa alarma.
//
// nil SI NO HAY NADA QUE CREER. La mayoría de los equipos de escritorio NO exponen esta clase
// —el firmware sólo la publica si el fabricante quiso—, así que la ausencia es el caso COMÚN y
// no un fallo. `Muestra.TempC` es puntero justamente para eso (D3).
func ParsearTempDecikelvin(salida string) *float64 {
	var mejor *float64
	for _, linea := range strings.Split(strings.ReplaceAll(salida, "\r\n", "\n"), "\n") {
		campo := strings.TrimSpace(linea)
		if campo == "" {
			continue
		}
		dk, err := strconv.ParseFloat(campo, 64)
		if err != nil {
			continue // encabezados, líneas en blanco, lo que PowerShell decida agregar
		}
		c := dk/10 - 273.15
		// La misma banda que Linux, con el mismo piso y por el mismo motivo. El borde de abajo
		// importa más de lo que parece por DOS razones: un sensor apagado devuelve 0 decikelvin,
		// que da −273,15 y se dibujaría como el frío absoluto; y uno clavado en un valor bajo
		// —2731 decikelvin dan 0,05 °C— pasaba el corte viejo (`c <= 0`) y competía como si
		// midiera. Ver el comentario de la banda en procparse.go.
		if c < TempMinPlausibleC || c > TempMaxPlausibleC {
			continue
		}
		if mejor == nil || c > *mejor {
			v := c
			mejor = &v
		}
	}
	return mejor
}
