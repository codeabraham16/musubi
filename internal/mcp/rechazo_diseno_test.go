package mcp

// rechazo_diseno_test.go — invariantes del CHECKLIST DE RECHAZO (plan de cierre, fase 4).

import (
	"strings"
	"testing"
)

// I-RCH1 · EL NÚCLEO VIAJA SIEMPRE, TAMBIÉN SIN EJE.
//
// El checklist existe porque las prohibiciones del acervo estaban DILUIDAS: sólo llegaban si el
// ranking enganchaba justo esa tarjeta. Un checklist que también dependa de que algo salga bien
// repite el defecto que vino a arreglar.
//
// SABOTAJE: devolver "" cuando no hay eje ⇒ el pedido que no rutea se queda sin criterio de rechazo.
func TestRechazoElNucleoViajaSiempre(t *testing.T) {
	sinEje := tellsPara("")
	if !strings.Contains(sinEje, "NO ENTREGUES ESTO") {
		t.Fatal("sin eje no llegó el encabezado del checklist")
	}
	nucleo := 0
	for _, x := range tellsDeDiseno {
		if x.Eje == "" {
			nucleo++
			if !strings.Contains(sinEje, x.Texto) {
				t.Errorf("falta un tell del núcleo: %.60s…", x.Texto)
			}
		}
	}
	if nucleo == 0 {
		t.Fatal("no hay núcleo universal: el checklist entero dependería de que el ruteo acierte")
	}
}

// I-RCH2 · CON EJE SE SUMAN LOS TELLS DE ESE EJE, Y NO LOS DE OTRO.
//
// Es lo que lo mantiene VARIABLE. Servir los catorce avisos siempre sería el sermón que M5 frena.
//
// SABOTAJE: ignorar el eje y devolver todos los tells ⇒ aparecen los de otros temas y M5 cae.
func TestRechazoElEjeFiltra(t *testing.T) {
	tabla := tellsPara("tabla")
	if !strings.Contains(tabla, "columnas numéricas") && !strings.Contains(tabla, "numéricas") {
		t.Error("el eje tabla no trajo sus propios tells")
	}
	// Y NO trae los de un eje ajeno: si trajera todo, el bloque sería constante.
	var ajeno string
	for _, x := range tellsDeDiseno {
		if x.Eje == "dataviz" {
			ajeno = x.Texto
			break
		}
	}
	if ajeno == "" {
		t.Fatal("el fixture necesita un tell de dataviz para probar el filtro")
	}
	if strings.Contains(tabla, ajeno) {
		t.Errorf("el eje tabla trajo un tell de dataviz: el bloque no está filtrando")
	}
}

// I-RCH3 · EL BLOQUE DE EJE TIENE TOPE.
//
// Sin tope, un eje con muchos tells desbalancea el brief y se come el lugar del material.
//
// SABOTAJE: sacar designTellsPorEje ⇒ un eje cargado entrega un bloque sin límite.
func TestRechazoElBloqueDeEjeTieneTope(t *testing.T) {
	for _, e := range ejesDeDiseno {
		conEje := strings.Count(tellsPara(e.Nombre), "\n- ")
		sinEje := strings.Count(tellsPara(""), "\n- ")
		if conEje-sinEje > designTellsPorEje {
			t.Errorf("el eje %q agregó %d tells, sobre el tope %d", e.Nombre, conEje-sinEje, designTellsPorEje)
		}
	}
}

// I-RCH4 · CADA TELL DICE POR QUÉ, NO SÓLO QUÉ.
//
// Sin el porqué, el agente no puede decidir los casos que el tell no previó — y un checklist que
// sólo prohíbe produce diseños que esquivan la letra y repiten el defecto.
//
// SABOTAJE: dejar un tell como orden pelada ⇒ este test lo señala por nombre.
func TestRechazoCadaTellDiceElPorQue(t *testing.T) {
	for _, x := range tellsDeDiseno {
		if len(x.Texto) < 60 {
			t.Errorf("tell demasiado corto para llevar su porqué (%d chars): %q", len(x.Texto), x.Texto)
		}
		if !strings.HasPrefix(x.Texto, "NO ") {
			t.Errorf("un tell tiene que empezar por la prohibición: %.50s…", x.Texto)
		}
	}
}
