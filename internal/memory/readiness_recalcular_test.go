package memory

import (
	"math"
	"testing"
)

// Los custodios de ReadinessReport.Recalcular.
//
// ⚠️ ESTE ARCHIVO EXISTE PORQUE EL COMENTARIO DE Recalcular PROMETÍA UN TEST QUE NO EXISTÍA. Decía
// textual: «Si algún día una dimensión calcula su Score de otra manera, la idempotencia se rompe y
// EL TEST LO DICE». No había test. Un panel adversarial lo midió en `verify`: la idempotencia era
// VERDAD —medido con el engine real, antes == x1 == x2— pero lo prometido era el CUSTODIO, no el
// hecho, y un invariante sin custodio es un canario que nunca cantó: el día que se rompa, nada
// avisa. Es la misma familia que el `bench-scale` rojo hace siete semanas.
//
// Los esperados van como LITERALES calculados a mano, nunca derivados de `puntaje` ni de
// `redondear`. Si el test pidiera `puntaje(d.Checks)`, un sabotaje de la fórmula movería el
// esperado junto con lo medido y el verde no informaría nada — la trampa vacua de siempre.
//
// El montaje es siempre el mismo y está elegido para que ningún número se confunda con otro:
//
//	0.67 + 0.67 + 0.67 + 0 + 0 = 2.01     2.01 / 5 = 0.402     redondeado = 0.40
//
// 2/3 da 0,666… y redondea a 0.67, así que la suma arrastra el error de coma flotante
// (0.67*3 = 2.0100000000000002) y el global sin redondear sale 0.40200000000000005. El 0.4 exacto
// sólo aparece si `redondear` corrió. Un montaje con números redondos no distinguiría las dos cosas.
func reporteIntacto() *ReadinessReport {
	dosDeTres := []ReadinessCheck{
		{Name: "a", Pass: true},
		{Name: "b", Pass: true},
		{Name: "c", Pass: false},
	}
	return &ReadinessReport{
		Score: 0.4,
		Dimensions: []ReadinessDimension{
			{Key: "uno", Observed: true, Score: 0.67, Checks: dosDeTres},
			{Key: "dos", Observed: true, Score: 0.67, Checks: dosDeTres},
			{Key: "tres", Observed: true, Score: 0.67, Checks: dosDeTres},
			{Key: "cuatro", Observed: true, Score: 0, Checks: []ReadinessCheck{{Name: "d", Pass: false}}},
			// Sin dato: sin checks y en cero, que es lo que le toca por la regla de oro.
			{Key: "cinco", Observed: false, Score: 0},
		},
		Unobserved: []string{"cinco"},
	}
}

func foto(r *ReadinessReport) []float64 {
	f := []float64{r.Score}
	for _, d := range r.Dimensions {
		f = append(f, d.Score)
	}
	return f
}

// Aplicar Recalcular dos veces tiene que dar lo mismo que aplicarla una, y sobre un reporte intacto
// lo mismo que no aplicarla. Es la propiedad que el comentario de la función promete.
//
// De paso afirma la conducta que un lente del panel dejó marcada como LATENTE: una dimensión
// `Observed=true` SIN checks queda en 0, aunque traiga un puntaje de antes. Hoy eso es inocuo
// —`Readiness` sólo lo implementa `*DbEngine` y sus cinco dimensiones ponen `Score = puntaje(Checks)`
// o vuelven sin checks— pero `readinessReader` es una interfaz, así que el día que entre otro
// backend cuyas dimensiones puntúen de otra manera, `Recalcular` las APLASTA. Queda afirmado para
// que sea una decisión visible y no una sorpresa.
//
// Sabotaje que lo pone en rojo: acumular en vez de asignar el Score de cada dimensión.
// arnes: archivo="internal/memory/readiness.go"
// arnes: de="r.Dimensions[i].Score = puntaje(r.Dimensions[i].Checks)"
// arnes: a="r.Dimensions[i].Score += puntaje(r.Dimensions[i].Checks)"
func TestRecalcularEsIdempotente(t *testing.T) {
	r := reporteIntacto()
	antes := foto(r)

	r.Recalcular()
	x1 := foto(r)
	r.Recalcular()
	x2 := foto(r)

	for i := range antes {
		if antes[i] != x1[i] {
			t.Errorf("foto[%d]: antes=%v x1=%v — la primera pasada MOVIÓ un reporte intacto", i, antes[i], x1[i])
		}
		if x1[i] != x2[i] {
			t.Errorf("foto[%d]: x1=%v x2=%v — no es idempotente: la segunda pasada vuelve a mover", i, x1[i], x2[i])
		}
	}

	// Y el aplastamiento, con su propio montaje para que no se lea como parte del de arriba.
	ajeno := &ReadinessReport{
		Score: 0.9,
		Dimensions: []ReadinessDimension{
			{Key: "de-otro-backend", Observed: true, Score: 0.9},
			{Key: "tambien", Observed: true, Score: 0.9},
		},
	}
	ajeno.Recalcular()
	if ajeno.Dimensions[0].Score != 0 || ajeno.Dimensions[1].Score != 0 || ajeno.Score != 0 {
		t.Errorf("un reporte de dimensiones observadas CON puntaje y SIN checks quedó en %v / %v / %v; "+
			"se esperaba 0 / 0 / 0, que es la conducta declarada de la proyección",
			ajeno.Score, ajeno.Dimensions[0].Score, ajeno.Dimensions[1].Score)
	}
}

// El Score global se redondea con la MISMA regla que los de las dimensiones. Sin esto, el promedio
// publica 0.40200000000000005 donde el resto del reporte publica dos decimales.
//
// Sabotaje que lo pone en rojo: quitarle el redondeo al global.
// arnes: archivo="internal/memory/readiness.go"
// arnes: de="r.Score = redondear(suma / float64(len(r.Dimensions)))"
// arnes: a="r.Score = suma / float64(len(r.Dimensions))"
func TestRecalcularRedondeaElGlobal(t *testing.T) {
	r := reporteIntacto()
	r.Score = 0 // se parte de cero para que el 0.4 sea lo que CALCULÓ, no lo que ya estaba
	r.Recalcular()

	if r.Score != 0.4 {
		t.Errorf("Score global = %.17g, esperaba exactamente 0.4; sin redondear, 0.67*3/5 da "+
			"0.40200000000000005 y el reporte publica más decimales que sus propias dimensiones", r.Score)
	}
	for _, d := range r.Dimensions[:3] {
		if d.Score != 0.67 {
			t.Errorf("dimensión %q = %.17g, esperaba exactamente 0.67 (2 de 3 checks)", d.Key, d.Score)
		}
	}
}

// Recalcular NO toca Observed ni Unobserved: agregar un check no vuelve observada a una dimensión
// que no lo era. Es lo que el comentario de la función promete en su último párrafo, y la razón es
// de contrato: quién tiene dato lo decide quien lo mide, no quien recalcula.
//
// Sabotaje que lo pone en rojo: marcar observada a toda dimensión que pase por el recálculo.
// arnes: archivo="internal/memory/readiness.go"
// arnes: de="\t\tsuma += r.Dimensions[i].Score"
// arnes: a="\t\tr.Dimensions[i].Observed = true\n\t\tsuma += r.Dimensions[i].Score"
func TestRecalcularNoTocaObserved(t *testing.T) {
	r := reporteIntacto()
	r.Recalcular()

	if r.Dimensions[4].Observed {
		t.Errorf("la dimensión %q quedó Observed=true: recalcular un puntaje no es haber medido",
			r.Dimensions[4].Key)
	}
	if len(r.Unobserved) != 1 || r.Unobserved[0] != "cinco" {
		t.Errorf("Unobserved = %v, esperaba [cinco]", r.Unobserved)
	}
}

// Un reporte sin dimensiones vuelve intacto. Sin la guarda, `suma / 0` da NaN y el Score global se
// vuelve un número que no compara ni consigo mismo — una fila de telemetría envenenada que no se
// nota hasta que alguien ordena por ese campo.
//
// Sabotaje que lo pone en rojo: quitar la guarda del reporte vacío.
// arnes: archivo="internal/memory/readiness.go"
// arnes: de="\tif len(r.Dimensions) == 0 {\n\t\treturn\n\t}"
// arnes: a="\t// guarda del reporte vacio quitada a proposito"
func TestRecalcularConReporteVacioNoProduceNaN(t *testing.T) {
	r := &ReadinessReport{Score: 0.5}
	r.Recalcular()

	if math.IsNaN(r.Score) {
		t.Fatal("Score global = NaN: sin dimensiones, suma/len divide por cero")
	}
	if r.Score != 0.5 {
		t.Errorf("Score global = %v, esperaba 0.5 intacto: sin dimensiones no hay nada que recalcular", r.Score)
	}
}
