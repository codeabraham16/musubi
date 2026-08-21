package memory

import (
	"reflect"
	"strings"
	"testing"
)

// dtos_capped_test.go es la guarda de CONTRATO, y existe porque los tests de comportamiento
// no alcanzan para este bug: se puede sacar un campo de total, la suite sigue verde, y el
// consumidor vuelve a publicar el largo de un array recortado como si fuera el universo.
// Acá se afirma la FORMA de los DTOs que cruzan la red y el borde MCP, no su valor.
//
// INVARIANTE: en un DTO de render, toda colección recortable declara su total y si se
// recortó. Una colección nueva que se agregue sin ese par rompe la suite el día que se
// agrega, no el día que alguien mira el dashboard y desconfía del número.

// dtoRecortable describe un DTO cuyas colecciones se capan para el render.
type dtoRecortable struct {
	nombre string
	tipo   reflect.Type
	// flagsHeredadas mapea las colecciones cuya bandera de truncado se llama `truncated` a
	// secas por razones históricas: son campos YA publicados que leen el dashboard, el
	// cuerpo y el CRM, y renombrarlos rompería consumidores externos sin ganar verdad. La
	// excepción es nominal a propósito: sólo vale para estas colecciones, así que cualquier
	// colección NUEVA está obligada a traer su propio `<campo>_truncated`.
	flagsHeredadas map[string]string
}

func TestDTOsDeRenderDeclaranSusTotales(t *testing.T) {
	casos := []dtoRecortable{
		{
			nombre:         "BrainGraph",
			tipo:           reflect.TypeOf(BrainGraph{}),
			flagsHeredadas: map[string]string{"neurons": "truncated"},
		},
		{
			nombre:         "CodeGraphViz",
			tipo:           reflect.TypeOf(CodeGraphViz{}),
			flagsHeredadas: map[string]string{"nodes": "truncated"},
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			presentes := map[string]reflect.Kind{}
			var colecciones []string
			for i := 0; i < c.tipo.NumField(); i++ {
				f := c.tipo.Field(i)
				tag := strings.Split(f.Tag.Get("json"), ",")[0]
				if tag == "" || tag == "-" {
					t.Errorf("%s.%s cruza la red sin tag JSON explícito", c.nombre, f.Name)
					continue
				}
				presentes[tag] = f.Type.Kind()
				if f.Type.Kind() == reflect.Slice {
					colecciones = append(colecciones, tag)
				}
			}
			if len(colecciones) == 0 {
				t.Fatalf("%s no tiene ninguna colección: el caso de prueba quedó obsoleto", c.nombre)
			}

			for _, col := range colecciones {
				total := "total_" + col
				if k, ok := presentes[total]; !ok {
					t.Errorf("%s recorta %q y no declara %q: el consumidor va a publicar len(%s) como si fuera el total",
						c.nombre, col, total, col)
				} else if k != reflect.Int {
					t.Errorf("%s.%s debería ser int, es %s", c.nombre, total, k)
				}

				flag := col + "_truncated"
				if heredada, ok := c.flagsHeredadas[col]; ok {
					flag = heredada
				}
				if k, ok := presentes[flag]; !ok {
					t.Errorf("%s recorta %q y no declara %q: sin la bandera, un total igual al largo es ambiguo entre «no se recortó» y «nadie lo contó»",
						c.nombre, col, flag)
				} else if k != reflect.Bool {
					t.Errorf("%s.%s debería ser bool, es %s", c.nombre, flag, k)
				}
			}
		})
	}
}
