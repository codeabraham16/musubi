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

// TempMaxPlausibleC es el techo que separa una lectura de un sensor roto. Es el mismo que aplica
// ParsearTempMiligrados en Linux, y comparten el motivo: un sensor que no está midiendo devuelve
// un valor imposible mucho más seguido que una máquina que de verdad está a 200 grados.
const TempMaxPlausibleC = 150

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
// SE TOMA LA PRIMERA ZONA PLAUSIBLE, no el máximo, para que signifique LO MISMO que en Linux:
// `Muestra.TempC` está documentada como «la primera zona térmica» y la lee
// /sys/class/thermal/thermal_zone0/temp. Tomar el máximo acá haría que el mismo campo signifique
// cosas distintas según el sistema operativo, y una serie que cambia de definición entre máquinas
// es peor que una que falta: el gráfico las apila igual.
//
// nil SI NO HAY NADA QUE CREER. La mayoría de los equipos de escritorio NO exponen esta clase
// —el firmware sólo la publica si el fabricante quiso—, así que la ausencia es el caso COMÚN y
// no un fallo. `Muestra.TempC` es puntero justamente para eso (D3).
func ParsearTempDecikelvin(salida string) *float64 {
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
		// El mismo rango que Linux. El borde de abajo importa más de lo que parece: un sensor
		// apagado devuelve 0 decikelvin, que da −273,15 y se dibujaría como el frío absoluto.
		if c <= 0 || c > TempMaxPlausibleC {
			continue
		}
		return &c
	}
	return nil
}
