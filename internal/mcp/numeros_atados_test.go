package mcp

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"musubi/internal/fleet"
)

// TRES NÚMEROS QUE DEPENDEN DE OTRO, Y NADA LOS MANTENÍA JUNTOS.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// Un número DERIVADO escrito a mano es una copia, y una copia se queda vieja. Estos tres no son
// constantes: son consecuencias de otra cosa, y esa otra cosa se puede cambiar sin que nada avise.
//
//  1. EL DIVISOR 8640 sale de `interval: 5m` sobre una ventana de 30 días. Cambiar el intervalo
//     de las recording rules a 1m —una decisión razonable— dejaría la cobertura dividida por el
//     número viejo: `count_over_time` daría 43.200 y la cobertura marcaría 5,00, o sea un 500 %
//     que ninguna alerta sabe leer y que hace DESAPARECER las series `:sla30d` por el lado
//     contrario al que se pensó.
//
//  2. EL `for: 25h` DE MantenimientoEterno sale del techo de 24 h del dominio
//     (`fleet.MantenimientoMax`). Si el techo bajara a 8 h, la alerta seguiría esperando 25 y una
//     ventana olvidada quedaría 17 horas sin avisar; si subiera a 48, no podría disparar NUNCA.
//
//  3. `MaquinaSinInventario` CONTABA `musubi_fleet_service_up`, que se OMITE para los servicios
//     `ocioso` y `desconocido`. Una máquina que reportó su inventario completo, con todos sus
//     servicios ociosos, no produce ni una de esas series — y la regla la acusaba de «no decir qué
//     corre adentro». La serie que contesta esa pregunta es `service_declared`, y su propio HELP
//     dice que existe para eso.
// ────────────────────────────────────────────────────────────────────────────────────────────

func TestElDivisorDeLaCoberturaSaleDelIntervaloDeLasRecordingRules(t *testing.T) {
	reglas := leerDeploy(t, "musubi-recording.yml")

	m := regexp.MustCompile(`(?m)^\s*interval:\s*(\d+)([smh])\s*$`).FindStringSubmatch(reglas)
	if m == nil {
		t.Fatal("no encuentro el `interval:` de las recording rules: sin él no se puede derivar el " +
			"divisor de la cobertura, y esta guarda no estaría midiendo nada")
	}
	n, _ := strconv.Atoi(m[1])
	segundos := map[string]int{"s": 1, "m": 60, "h": 3600}[m[2]]
	esperado := 30 * 24 * 3600 / (n * segundos)

	// EL DIVISOR SE BUSCA EN LAS EXPRESIONES QUE DIVIDEN, no en todo el archivo: el número también
	// aparece en la prosa que lo explica, y contarlo ahí haría que el comentario correcto
	// satisficiera una regla equivocada.
	divisiones := regexp.MustCompile(`count_over_time\([^)]*\[30d\]\)\s*/\s*(\d+)`).FindAllStringSubmatch(reglas, -1)
	if len(divisiones) < 2 {
		t.Fatalf("encontré %d divisiones de cobertura y son al menos 2 (máquinas y servicios): "+
			"cambió la forma del archivo y esta guarda está en verde sin mirar nada", len(divisiones))
	}
	for _, d := range divisiones {
		if d[1] != fmt.Sprint(esperado) {
			t.Errorf("una cobertura divide por %s y con `interval: %s%s` sobre 30 días son %d muestras.\n"+
				"  Un divisor viejo NO da un número raro que alguien note: da una cobertura escalada "+
				"—5,00 en vez de 1,00— y las series `:sla30d` desaparecen o aparecen por el lado "+
				"equivocado del umbral de 0,95, en silencio.\n"+
				"  El divisor es una CONSECUENCIA del intervalo, no una constante.", d[1], m[1], m[2], esperado)
		}
	}
}

func TestElForDeMantenimientoEternoSaleDelTechoDelDominio(t *testing.T) {
	reglas := leerDeploy(t, "musubi-alerts.yml")

	i := strings.Index(reglas, "- alert: MantenimientoEterno")
	if i < 0 {
		t.Fatal("desapareció `MantenimientoEterno`: una ventana de mantenimiento que alguien se " +
			"olvidó de cerrar deja callada a esa máquina para siempre, y nadie lo avisa")
	}
	bloque := reglas[i:]
	if j := strings.Index(bloque[1:], "- alert:"); j > 0 {
		bloque = bloque[:j]
	}
	m := regexp.MustCompile(`for:\s*(\d+)h`).FindStringSubmatch(bloque)
	if m == nil {
		t.Fatalf("no encuentro el `for:` de MantenimientoEterno:\n%s", bloque)
	}
	horas, _ := strconv.Atoi(m[1])

	// LA PROPIEDAD, y no un número: el `for` tiene que estar POR ENCIMA del techo del dominio —si
	// no, dispara sobre ventanas legítimas— y no puede pasarse tanto como para dejar horas sin
	// aviso. Una hora de margen es lo que hay hoy.
	techo := int(fleet.MantenimientoMax.Hours())
	if horas <= techo {
		t.Errorf("`MantenimientoEterno` espera %dh y el techo del dominio es de %dh "+
			"(`fleet.MantenimientoMax`): con el `for` por debajo o igual, la alerta dispara sobre "+
			"ventanas PERFECTAMENTE legítimas que todavía no vencieron", horas, techo)
	}
	if horas > techo+2 {
		t.Errorf("`MantenimientoEterno` espera %dh contra un techo de %dh: una ventana olvidada "+
			"queda %d horas callando las alertas de esa máquina sin que nadie avise.\n"+
			"  El `for` es una CONSECUENCIA de `fleet.MantenimientoMax`, no una constante: si el "+
			"techo se mueve, este número se mueve con él.", horas, techo, horas-techo)
	}
}

// LA PREGUNTA «¿REPORTÓ INVENTARIO?» SE LE HACE A LA SERIE QUE SIEMPRE ESTÁ.
func TestMaquinaSinInventarioCuentaLaSerieQueNoSeOmite(t *testing.T) {
	reglas := leerDeploy(t, "musubi-alerts-flota.yml")
	i := strings.Index(reglas, "- alert: MaquinaSinInventario")
	if i < 0 {
		t.Fatal("desapareció `MaquinaSinInventario`")
	}
	bloque := reglas[i:]
	if j := strings.Index(bloque[1:], "- alert:"); j > 0 {
		bloque = bloque[:j]
	}
	expr := bloque
	if k := strings.Index(expr, "annotations:"); k > 0 {
		expr = expr[:k]
	}
	if strings.Contains(expr, "count by(project, device) (musubi_fleet_service_up)") {
		t.Error("`MaquinaSinInventario` cuenta `musubi_fleet_service_up`, que se OMITE para los " +
			"servicios `ocioso` y `desconocido`.\n" +
			"  Una máquina que reportó su inventario COMPLETO, con todos sus servicios ociosos, no " +
			"produce ni una de esas series: la regla la acusa de «no decir qué corre adentro» sobre " +
			"un inventario perfecto.\n" +
			"  Contá `musubi_fleet_service_declared`, que vale 1 siempre que el servicio esté " +
			"declarado — su propio HELP dice que existe exactamente para eso.")
	}
	if !strings.Contains(expr, "musubi_fleet_service_declared") {
		t.Error("`MaquinaSinInventario` ya no cuenta `musubi_fleet_service_declared`: si mira otra " +
			"serie, tiene que ser una que NO se omita, o vuelve el falso positivo")
	}
}
