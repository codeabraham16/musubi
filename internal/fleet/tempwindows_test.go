package fleet

import "testing"

// LA UNIDAD DE `MSAcpi_ThermalZoneTemperature` SON DÉCIMAS DE KELVIN, Y LEERLA MAL NO DA ERROR.
//
// ════════════════════════════════════════════════════════════════════════════════════════════
// Los dos errores posibles se dibujan distinto, y el segundo es el caro:
//
//	leerla como Celsius directo   ->  «3032 grados», absurdo y se nota
//	restar 273,15 SIN dividir /10 ->  −243, que pasa por «bajo cero» y se dibuja como una
//	                                  máquina congelada en vez de como una unidad mal leída
//
// Esta prueba fija la conversión con valores reales de firmware y clava los dos bordes.
//
// A2 EXISTÍA PORQUE «WMI DESDE GO SIN DEPENDENCIAS ES COM CRUDO». La premisa se cayó: desde A42 el
// agente de Windows YA lanza PowerShell en cada latido, y desde A70 le pide `Win32_Service`. Lo
// que quedaba era el costo de una invocación más, no la imposibilidad.
//
// Sabotaje que la hace fallar: sacar el `/10`, o cambiar el signo del ajuste de Kelvin.
func TestLaZonaTermicaDeWindowsSeLeeEnDecikelvin(t *testing.T) {
	casos := []struct {
		nombre  string
		salida  string
		quiere  float64
		esperaC bool
	}{
		{"una zona, valor de firmware real", "3032", 30.05, true},
		{"con espacios y CRLF, como los devuelve PowerShell", "\r\n  3182  \r\n", 45.05, true},
		// CAMBIÓ EL 2026-09-05: antes este caso esperaba 30,05 —«la primera plausible, igual que
		// thermal_zone0 en Linux»— y la premisa se cayó cuando Linux dejó de leer thermal_zone0.
		// El orden en que el firmware lista sus zonas no significa nada, igual que no significaba
		// nada el número de zona en /sys; apostar a la primera es la misma apuesta que ya salió mal.
		{"varias zonas: gana la MÁS ALTA, no la primera — el orden del firmware no significa nada",
			"3032\n3312\n", 58.05, true},
		{"y no depende del orden: las mismas dos zonas al revés dan el mismo número",
			"3312\n3032\n", 58.05, true},
		{"un sensor clavado en 0,05 °C NO le gana a una lectura real (el piso de la banda)",
			"2732\n3032\n", 30.05, true},
		{"un sensor clavado en 0,05 °C solo se descarta entero, no se reporta como 0 grados",
			"2732", 0, false},
		{"un sensor apagado devuelve 0 dK, que son −273 °C y NO una máquina congelada",
			"0", 0, false},
		{"salida vacía: el firmware no publica la clase, que es el caso COMÚN", "", 0, false},
		{"texto que no es número (un error de PowerShell que se coló)", "Get-CimInstance : ...", 0, false},
		{"la primera zona es basura y la segunda sirve: no se descarta la salida entera",
			"0\n3032\n", 30.05, true},
		{"por encima del techo plausible: sensor roto, no un incendio", "5000", 0, false},
	}

	for _, c := range casos {
		got := ParsearTempDecikelvin(c.salida)
		if !c.esperaC {
			if got != nil {
				t.Errorf("%s: devolvió %.2f °C y se esperaba nil", c.nombre, *got)
			}
			continue
		}
		if got == nil {
			t.Errorf("%s: devolvió nil y se esperaba %.2f °C", c.nombre, c.quiere)
			continue
		}
		if diff := *got - c.quiere; diff > 0.01 || diff < -0.01 {
			t.Errorf("%s: %.2f °C, esperaba %.2f", c.nombre, *got, c.quiere)
		}
	}

	// LOS DOS ERRORES DE UNIDAD, cada uno con su síntoma, porque se arreglan distinto y un
	// mensaje que nombra la causa equivocada manda a mirar la línea que no es.
	v := ParsearTempDecikelvin("3032")
	switch {
	case v == nil:
		t.Error("3032 dK se descartó: si se leyó como Celsius directo da 3032, que el techo de " +
			"plausibilidad rechaza — el síntoma es una serie que desaparece, no un valor absurdo")
	case *v < 0:
		t.Errorf("3032 dK dio %.2f: se restó Kelvin SIN dividir por 10, y un negativo se dibuja "+
			"como una máquina bajo cero en vez de como una unidad mal leída", *v)
	}

	// Y QUE SIGNIFIQUE LO MISMO QUE EN LINUX. Esto no es redundante con los casos de arriba: es
	// la prueba de que los DOS parsers comparten la banda, que es lo que se rompió una vez ya
	// —`TempMaxPlausibleC` estuvo declarado en dos archivos a la vez— y lo que hace que
	// `musubi_fleet_device_temperature_celsius` signifique una cosa sola cuando el gráfico apila
	// una máquina Linux y una Windows en la misma serie.
	if ParsearTempMiligrados("0") != nil || ParsearTempDecikelvin("0") != nil {
		t.Error("un sensor en cero pasa por alguno de los dos caminos: la misma lectura tiene que dar nil en las dos plataformas")
	}
	// El piso, por los dos caminos, con la MISMA temperatura expresada en las dos unidades:
	// 50 miligrados y 2732 decikelvin son los dos 0,05 °C. EL DÍGITO IMPORTA y me lo equivoqué
	// una vez: 2731 dK son −0,05 °C, que el corte VIEJO (`c <= 0`) ya rechazaba, así que con 2731
	// esta prueba pasaba con y sin el piso — verificado sabotando el corte y viendo verde.
	if ParsearTempMiligrados("50") != nil || ParsearTempDecikelvin("2732") != nil {
		t.Error("0,05 °C pasa por alguno de los dos caminos: es un sensor que no está midiendo, " +
			"y aceptarlo lo pone a competir con una lectura real (medido en la workstation: SEN2/4/5 = 50 mC)")
	}
	// Y el techo, también por los dos: 200 °C en las dos unidades.
	if ParsearTempMiligrados("200000") != nil || ParsearTempDecikelvin("4731") != nil {
		t.Error("200 °C pasa por alguno de los dos caminos: la banda tiene que ser la misma en los dos parsers")
	}
}
