package mcp

import (
	"regexp"
	"strings"
	"testing"
)

// EL DELTA DE COBERTURA SE TOMA POR MÁQUINA, NO SOBRE EL MÍNIMO DEL PROYECTO.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// `CoberturaDelSlaSeCayo` miraba `delta(musubi:project_up:cobertura30d[6h]) < -0.05`, y esa serie
// es `min by(project)` de la cobertura de cada máquina. La cobertura de una máquina RECIÉN
// ENROLADA arranca en cero —todavía no tiene historia—, así que sumar una máquina hunde el
// mínimo del proyecto de golpe: el delta a 6 h da unos -0,9 y la alerta suena diciendo «se
// perdió medición» sobre el evento contrario, que es que entró una máquina más a medirse.
//
// Y LA PREMISA ESCRITA EN SU PROPIA NOTA ERA FALSA PARA LA SERIE QUE MIRABA: «mientras el TSDB
// acumula historia la cobertura sólo sube». Cierto por máquina; falso para el mínimo de un
// conjunto que puede crecer. La hermana de servicios tenía el mismo defecto, y su nota lo
// documentaba como un paso de diagnóstico —«descartá lo barato: un servicio recién declarado
// arrastra el min»— o sea que el falso positivo estaba visto, aceptado y convertido en trabajo
// manual para quien atendiera la alerta.
//
// Tomado por máquina el caso se resuelve solo: la cobertura de una máquina nueva sólo SUBE desde
// cero, así que su delta es positivo. Y de paso la alerta pasa a decir CUÁL máquina perdió
// medición.
// ────────────────────────────────────────────────────────────────────────────────────────────

func TestElDeltaDeCoberturaSeTomaAntesDeAgregarPorProyecto(t *testing.T) {
	reglas := leerDeploy(t, "musubi-alerts-flota.yml")

	// `delta(...)` sobre cualquier serie grabada a nivel de PROYECTO es el defecto.
	deltaSobreProyecto := regexp.MustCompile(`delta\(\s*musubi:project_[a-z_]*:cobertura30d`)
	if m := deltaSobreProyecto.FindString(strings.Join(strings.Fields(reglas), " ")); m != "" {
		t.Errorf("hay un `%s…`: el delta se está tomando sobre una serie YA AGREGADA con "+
			"`min by(project)`.\n"+
			"  Una máquina (o un servicio) recién enrolada entra con cobertura 0 y hunde ese mínimo, "+
			"así que la alerta suena en cada ENROLAMIENTO diciendo que se perdió medición.\n"+
			"  Tomá el delta sobre la serie por máquina/servicio: ahí la cobertura de una nueva sólo "+
			"sube desde cero y su delta es positivo.", m)
	}

	// LA OTRA DIRECCIÓN: las dos alertas tienen que seguir existiendo. Sin esto, la guarda la
	// satisface borrarlas — y perder la vigilancia de la medición es peor que un falso positivo.
	for _, alerta := range []struct{ nombre, serie string }{
		{"CoberturaDelSlaSeCayo", "musubi:device_up:cobertura30d"},
		{"CoberturaDelSlaDeServiciosSeCayo", "musubi:service_up:cobertura30d"},
	} {
		i := strings.Index(reglas, "- alert: "+alerta.nombre)
		if i < 0 {
			t.Errorf("desapareció `%s`: sin ella, una pérdida de medición sobre el número que se le "+
				"factura a un cliente no la avisa nadie", alerta.nombre)
			continue
		}
		bloque := reglas[i:]
		if j := strings.Index(bloque[1:], "- alert:"); j > 0 {
			bloque = bloque[:j]
		}
		if !strings.Contains(bloque, "delta("+alerta.serie+"[") {
			t.Errorf("`%s` ya no toma el delta de `%s`: si mira otra serie, decí cuál y por qué — "+
				"la que agrega por proyecto es la que disparaba en cada enrolamiento", alerta.nombre, alerta.serie)
		}
	}
}
