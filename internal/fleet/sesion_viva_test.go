package fleet

// sesion_viva_test.go custodia la vista común del plano de entrar. Su modo de fallo es el más
// caro que puede tener un panel de este plano: decir que alguien sigue adentro de una máquina
// cuando ya salió, o al revés.

import (
	"testing"
	"time"
)

// «ABIERTA» SE DERIVA, Y LAS TRES FORMAS DE ESTAR CERRADA CUENTAN.
//
// Una sesión está abierta si NO se cerró y TODAVÍA no venció. Las dos condiciones son necesarias:
// mirar sólo `Cerrada` daría por viva una sesión que venció y que nadie cerró —que es lo normal
// cuando alguien apaga la pestaña— y mirar sólo `Vence` daría por viva una que se cerró temprano.
//
// El caso del vencimiento en cero es el que se olvida: una sesión sin `Vence` no es eterna, es una
// fila mal formada, y darla por abierta la dejaría en el panel para siempre.
//
// Sabotaje que la hace fallar: devolver `s.Cerrada.IsZero()` a secas.
func TestAbiertaSeDerivaYNoSeGuarda(t *testing.T) {
	ahora := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	casos := []struct {
		nombre  string
		vence   time.Time
		cerrada time.Time
		quiero  bool
	}{
		{"en curso", ahora.Add(10 * time.Minute), time.Time{}, true},
		{"cerrada temprano", ahora.Add(10 * time.Minute), ahora.Add(-time.Minute), false},
		{"vencida y nadie la cerró", ahora.Add(-time.Minute), time.Time{}, false},
		{"vence justo ahora", ahora, time.Time{}, false},
		{"sin vencimiento: fila mal formada, no eterna", time.Time{}, time.Time{}, false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s := SesionViva{Vence: c.vence, Cerrada: c.cerrada}
			if got := s.Abierta(ahora); got != c.quiero {
				t.Errorf("Abierta = %v, se esperaba %v", got, c.quiero)
			}
		})
	}
}

// LA MODALIDAD VIAJA, Y NO ES UN DETALLE DE PRESENTACIÓN.
//
// Una lista que junta pantallas y shells sin distinguirlas no sirve para decidir nada: los techos
// son distintos, el riesgo es distinto, y lo que hay que hacer al ver una abierta también. Y el
// nombre de la máquina viaja resuelto porque una lista de ids opacos no la lee nadie.
//
// Sabotaje que la hace fallar: devolver la misma Modalidad en las dos puertas.
func TestLaModalidadYElNombreDeLaMaquinaViajanEnLaVista(t *testing.T) {
	creada := time.Now().UTC()
	p := DesdeSesionPantalla(SesionPantalla{
		ID: "s1", DeviceID: "d1", ProjectID: "casa", Principal: "gio",
		Estado: EstadoSesion("entregada"), Creada: creada,
	}, "pc-gio")
	sh := DesdeSesionShell(SesionShell{
		ID: "s2", DeviceID: "d2", ProjectID: "casa", Principal: "gio",
		Estado: EstadoShell("abierta"), Creada: creada,
	}, "nas")

	if p.Modalidad != ModalidadPantalla || sh.Modalidad != ModalidadShell {
		t.Errorf("las modalidades no se distinguen: %q y %q", p.Modalidad, sh.Modalidad)
	}
	if p.Device != "pc-gio" || sh.Device != "nas" {
		t.Errorf("el nombre de la máquina no viajó: %q y %q", p.Device, sh.Device)
	}
	// El estado viaja como TEXTO y sin traducir: cada modalidad tiene su enum y no son el mismo
	// conjunto. Traducirlos a uno común inventaría estados que ninguna de las dos tiene.
	if p.Estado != "entregada" || sh.Estado != "abierta" {
		t.Errorf("el estado se tradujo o se perdió: %q y %q", p.Estado, sh.Estado)
	}
	// Y la vista NO tiene UltimoTrafico: meterlo dejaría la mitad de las filas en cero, y ese
	// cero se leería como «sin tráfico» en vez de como «no aplica».
	if p.Principal != "gio" || p.ProjectID != "casa" {
		t.Error("se perdió la identidad o el tenant en la traducción")
	}
}
