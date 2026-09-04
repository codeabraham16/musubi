package fleet

import "testing"

// CERO Y DOS COINCIDENCIAS DAN LO MISMO: NO MEDIDA. Con cero, la máquina puede estar encendida en
// otra red y afirmar «no está» sería inventar; con dos, elegir una es una moneda al aire, que es
// la misma decisión que A13 tomó para el id de pantalla.
//
// Sabotaje: devolver VidaAusente cuando no hay coincidencias, o quedarse con la primera cuando
// hay dos.
func TestLaVidaDeRedNoAfirmaLoQueNoPudoMedir(t *testing.T) {
	casos := []struct {
		nombre   string
		device   string
		pares    []ParDeTailnet
		esperado VidaDeRed
	}{
		{"sin pares no se puede decir nada", "pc", nil, VidaNoMedida},
		{"la máquina no está en el tailnet", "pc", []ParDeTailnet{{Nombre: "otra", EnLinea: true}}, VidaNoMedida},
		{"dos pares dicen ser la misma máquina", "pc",
			[]ParDeTailnet{{Nombre: "pc", EnLinea: true}, {Nombre: "pc", EnLinea: false}}, VidaNoMedida},
		{"un par en línea", "pc", []ParDeTailnet{{Nombre: "pc", EnLinea: true}}, VidaPresente},
		{"un par fuera de línea", "pc", []ParDeTailnet{{Nombre: "pc", EnLinea: false}}, VidaAusente},
		{"empareja por la primera etiqueta del DNS", "pc",
			[]ParDeTailnet{{Nombre: "", DNS: "pc.tail89e295.ts.net.", EnLinea: true}}, VidaPresente},
		{"no distingue mayúsculas", "PC-Gio", []ParDeTailnet{{Nombre: "pc-gio", EnLinea: true}}, VidaPresente},
		{"un nombre vacío no empareja con nada", "",
			[]ParDeTailnet{{Nombre: "", EnLinea: true}}, VidaNoMedida},
		// «srv-01» y «srv01» son DOS máquinas. Colapsarlas fabricaría la ambigüedad que la
		// función rechaza, y peor: haría que una diga la vida de la otra.
		{"no colapsa guiones", "srv-01", []ParDeTailnet{{Nombre: "srv01", EnLinea: true}}, VidaNoMedida},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if got := VidaDeRedDe(c.device, c.pares); got != c.esperado {
				t.Fatalf("VidaDeRedDe(%q) = %v, esperaba %v", c.device, got, c.esperado)
			}
		})
	}
}
