package memory

import (
	"context"
	"testing"

	"musubi/internal/config"
)

// TestAlcanceDelCorrector fija la regla del vocabulario del corrector de tipeo, modo por modo. El
// defecto que cierra: la regla era sólo prosa en AlcanceDelTurno, y el merge limpio con el corrector
// armó el alcance con el filtro duro de las opciones, que en «aparte» es el proyecto propio. Así el
// corrector daba por muertas las palabras de las notas ajenas que el mismo turno devolvía.
//
// Sabotaje: «aparte» corrige con el vocabulario del proyecto propio.
// arnes: archivo="internal/memory/opciones_turno.go"
// arnes: de="\tif o.TopeOtrosProyectos > 0 {\n\t\treturn ProjectScope{}\n\t}\n"
// arnes: a="\tif false {\n\t\treturn ProjectScope{}\n\t}\n"
//
// Sabotaje: sin tope, el corrector mira todo el acervo y no el filtro del recall.
// arnes: archivo="internal/memory/opciones_turno.go"
// arnes: de="\treturn ProjectScope{ProjectID: o.ProjectScope, Federate: o.Federate}\n"
// arnes: a="\treturn ProjectScope{}\n"
func TestAlcanceDelCorrector(t *testing.T) {
	m := config.Default().Memory
	delModo := func(modo, propio string) RecallOptions {
		return OpcionesDeRecallDelTurno(m, AlcanceDelTurnoSegun(modo, 2, propio))
	}
	for _, c := range []struct {
		nombre string
		opts   RecallOptions
		quiere ProjectScope
	}{
		{"aparte", delModo(config.OtrosProyectosAparte, "musubi"), ProjectScope{}},
		{"aislado", delModo(config.OtrosProyectosAislado, "musubi"), ProjectScope{ProjectID: "musubi"}},
		{"mezclado", delModo(config.OtrosProyectosMezclado, "musubi"), ProjectScope{}},
		{"sin proyecto propio", delModo(config.OtrosProyectosAparte, ""), ProjectScope{}},
		// Fuera del turno, sin tope, es el filtro duro tal cual: el acotado y el federado explícito.
		{"acotado sin tope", RecallOptions{ProjectScope: "altura"}, ProjectScope{ProjectID: "altura"}},
		{"federado explícito", RecallOptions{ProjectScope: "altura", Federate: true}, ProjectScope{ProjectID: "altura", Federate: true}},
	} {
		if got := c.opts.AlcanceDelCorrector(); got != c.quiere {
			t.Errorf("%s: AlcanceDelCorrector() = %+v, quiere %+v (ProjectScope=%q, tope=%d)",
				c.nombre, got, c.quiere, c.opts.ProjectScope, c.opts.TopeOtrosProyectos)
		}
	}
}

// TestElCorrectorDelTurnoNoReescribeLoQueElTurnoTrae: la misma regla contra el motor real. Altura
// escribe «planilla» y Musubi «plantilla», a una letra. En «aparte» el recall desde Musubi trae las
// notas de Altura, así que «planilla» está viva y el corrector no la toca. En «aislado» ese recall no
// puede traerlas, y la corrige. Esa mitad muestra además que el corrector corre y que la primera no
// pasa por no haber corregido nada.
func TestElCorrectorDelTurnoNoReescribeLoQueElTurnoTrae(t *testing.T) {
	e := sembrarReparto(t, []notaDeReparto{
		{"musubi", "p-pla1", "sdd/plantillas", "La plantilla de artefactos SDD vive en el repo.", 0},
		{"musubi", "p-pla2", "cuerpo/lienzo", "El lienzo arranca desde una plantilla vacía.", 0},
		{"altura", "a-pla1", "rrhh/planilla", "La planilla del turno noche se cierra los viernes.", 0},
		{"altura", "a-pla2", "rrhh/horas", "Las horas extra se cargan en la planilla semanal.", 0},
	})
	const prompt = "cómo se cierra la planilla del turno noche"
	m := config.Default().Memory

	aparte := OpcionesDeRecallDelTurno(m, AlcanceDelTurnoSegun(config.OtrosProyectosAparte, 2, "musubi"))
	aparte.TokenBudget = 5000
	res, err := e.Recall(context.Background(), prompt, aparte)
	if err != nil {
		t.Fatal(err)
	}
	if !trae(res, "a-pla1") {
		t.Fatalf("precondición: el recall «aparte» tenía que traer la nota de Altura sobre la planilla: %v", idsDelReparto(res))
	}
	if got, cs := corregirSinApuro(e, prompt, aparte.AlcanceDelCorrector()); len(cs) != 0 {
		t.Errorf("en «aparte» el corrector reescribió %q (consulta %q): esas palabras están en notas que ese recall trae",
			resumenDeCorrecciones(cs), got)
	}

	aislado := OpcionesDeRecallDelTurno(m, AlcanceDelTurnoSegun(config.OtrosProyectosAislado, 2, "musubi"))
	if got, cs := corregirSinApuro(e, prompt, aislado.AlcanceDelCorrector()); resumenDeCorrecciones(cs) != "planilla→plantilla" {
		t.Errorf("en «aislado» «planilla» no está en nada que ese recall pueda traer: quería planilla→plantilla y fue %q (consulta %q)",
			resumenDeCorrecciones(cs), got)
	}
}
