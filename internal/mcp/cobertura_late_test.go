package mcp

import (
	"strings"
	"testing"
)

// `verificar-cobertura.sh` NO LO CORRÍA NADIE.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// Contesta la pregunta de un nivel más adentro que su hermano: `verificar-despliegue.sh` dice
// «¿está la regla cargada?» y éste «¿esa regla vigila a ESTA máquina?». La diferencia se midió el
// 2026-09-02, con las 35 reglas desplegadas y todas sus métricas presentes: 13 de 19 dimensiones
// en las Windows.
//
// SU HERMANO GANÓ TIMER, LATIDO Y DEAD-MAN CON A115; ÉL QUEDÓ EN «CUANDO ALGUIEN SE ACUERDE»,
// que es exactamente la condición que A115 existió para eliminar. Y «cuando alguien se acuerde»
// no es una cadencia baja: es CERO, porque nadie se acuerda de correr un guion que no falla.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestLaVerificacionDeCoberturaCorreYLate(t *testing.T) {
	guion := leerDeploy(t, "comparar-y-latir.sh")

	if !strings.Contains(guion, "verificar-cobertura.sh") {
		t.Fatal("`comparar-y-latir.sh` no corre `verificar-cobertura.sh`: su hermano tiene timer, " +
			"latido y dead-man, y él queda en «cuando alguien se acuerde» — que no es una cadencia " +
			"baja, es cero")
	}

	// SU CÓDIGO VIAJA APARTE Y NO FUSIONADO. Son dos preguntas distintas que se arreglan distinto:
	// un 1 de allá es «producción diverge» y un 1 de acá es «hay dimensiones sin vigilar».
	// Fusionarlos daría un número que no dice cuál de las dos falló.
	if !strings.Contains(guion, "CODIGO_COBERTURA") {
		t.Error("el resultado de la verificación de cobertura no se guarda aparte: fusionarlo con el " +
			"del despliegue da un número que no dice cuál de las dos preguntas falló")
	}
	for _, serie := range []string{
		"musubi_verificacion_cobertura_ultima_seconds",
		"musubi_verificacion_cobertura_resultado",
	} {
		if !strings.Contains(guion, serie) {
			t.Errorf("el latido no lleva `%s`: sin ella no hay dead-man, y poner la verificación en "+
				"una corrida sin señal sólo mueve el silencio de lugar", serie)
		}
	}
}

// LAS DOS ALERTAS, Y LAS DOS CON `last_over_time`.
//
// Una serie EMPUJADA existe en el vector instantáneo sólo ~5 minutos después de cada empuje, y
// esto late cada 6 HORAS. Escritas con la métrica pelada, las alertas del latido estaban rotas en
// las dos direcciones a la vez —una disparaba a los cinco minutos de cada latido y las otras no
// podían dispararse nunca— y ninguna lectura del YAML lo mostraba. Se descubrió corriéndolo.
func TestLasAlertasDeLaCoberturaLeenLaSerieConLastOverTime(t *testing.T) {
	reglas := strings.Join(strings.Fields(leerDeploy(t, "musubi-alerts.yml")), " ")

	for _, a := range []struct{ nombre, serie string }{
		{"CoberturaPorMaquinaSinVerificar", "musubi_verificacion_cobertura_ultima_seconds"},
		{"MaquinasConDimensionesSinVigilar", "musubi_verificacion_cobertura_resultado"},
	} {
		i := strings.Index(reglas, "- alert: "+a.nombre)
		if i < 0 {
			t.Errorf("falta la alerta `%s`", a.nombre)
			continue
		}
		bloque := reglas[i:]
		if j := strings.Index(bloque[1:], "- alert:"); j > 0 {
			bloque = bloque[:j]
		}
		if !strings.Contains(bloque, "last_over_time("+a.serie) &&
			!strings.Contains(bloque, "absent_over_time("+a.serie) {
			t.Errorf("`%s` lee `%s` sin `last_over_time`/`absent_over_time`.\n"+
				"  Una serie empujada se pone rancia a los ~5 minutos y esto late cada 6 HORAS: "+
				"escrita pelada, la alerta o dispara en falso cada latido o no puede dispararse "+
				"nunca. Está medido contra este mismo servidor.", a.nombre, a.serie)
		}
	}

	// Y EL RESULTADO SE LEE CON `== 1` Y NO CON `> 0`: el 2 significa «no se pudo medir», que es
	// otra cosa y ya la dice el dead-man. Tratarlos igual manda a buscar huecos de cobertura
	// cuando el problema fue que no se pudo consultar Prometheus.
	if !strings.Contains(reglas, "musubi_verificacion_cobertura_resultado[7d])) == 1") {
		t.Error("la alerta del resultado de cobertura no usa `== 1`: con `> 0` trataría el 2 —«no se " +
			"pudo medir»— como si fueran huecos de cobertura, y mandaría a buscar al lugar equivocado")
	}
}
