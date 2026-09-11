package mcp

import (
	"regexp"
	"strings"
	"testing"
)

// NADA DETECTABA QUE UNA MÁQUINA LLEGARA DOS VECES.
//
// ────────────────────────────────────────────────────────────────────────────────────────────
// `prometheus.yml` descarta `musubi_fleet_(device|service)_.*` del scrape justamente para que
// cada dato tenga UN SOLO PRODUCTOR: esa familia llega por el empuje OTLP. Si el descarte se
// saca, se comenta, o se despliega un `prometheus.yml` viejo, cada máquina empieza a llegar DOS
// veces con `instance` distinto.
//
// Y NO SE VE COMO UN ERROR: se ve como que todo anda. Medido acá el 2026-08-28 al encender el
// empuje: 5 alertas se convirtieron en 10 avisos. Y el `min` del SLA se quedó con el camino
// MUERTO — el «peor equipo» del reporte marcó 10,2 % cuando la misma máquina, por el camino vivo,
// estaba en 84,7 %.
//
// LAS RECORDING RULES YA LO COLAPSAN con `max by(...)`, y eso arregló el reporte. Pero colapsar
// no es DETECTAR: la duplicación sigue ahí, invisible, y cualquier regla nueva escrita contra la
// familia cruda vuelve a disparar doble. Hacía falta mirar la causa.
// ────────────────────────────────────────────────────────────────────────────────────────────
func TestAlgunaAlertaDetectaLaDuplicacionPorInstance(t *testing.T) {
	reglas := strings.Join(strings.Fields(leerDeploy(t, "musubi-alerts-flota.yml")), " ")

	// LA FORMA QUE DETECTA ES CONTAR SERIES POR MÁQUINA. Cualquier otra —comparar valores,
	// mirar `instance` como etiqueta— o no ve la duplicación o la ve sólo cuando los dos caminos
	// discrepan, que es tarde.
	cuenta := regexp.MustCompile(`count by\(project, device\) \(musubi_fleet_device_up\) > 1`)
	if !cuenta.MatchString(reglas) {
		t.Error("ninguna alerta cuenta cuántas series hay por máquina.\n" +
			"  Con el descarte de `prometheus.yml` sacado o mal desplegado, cada máquina llega dos " +
			"veces y CADA alerta suya sale duplicada — y el `min` del SLA se queda con el camino " +
			"muerto: así el «peor equipo» del reporte marcó 10,2 % siendo un fantasma.\n" +
			"  Las recording rules lo COLAPSAN con `max by(...)`, que arregla el reporte y no " +
			"detecta nada.")
	}
}

// Y LA MITAD QUE SOSTIENE A LA DE ARRIBA: el descarte tiene que seguir existiendo.
//
// Sin él la alerta nueva sonaría para toda la flota a la vez, que es correcto pero inútil — lo
// que hay que custodiar es que la causa no vuelva.
func TestElDescarteQueGarantizaUnSoloProductorSigueActivo(t *testing.T) {
	cfg := leerDeploy(t, "prometheus", "prometheus.yml")
	plano := strings.Join(strings.Fields(cfg), " ")

	if !strings.Contains(plano, "metric_relabel_configs") {
		t.Fatal("`prometheus.yml` perdió su `metric_relabel_configs`: sin el descarte, la familia " +
			"`musubi_fleet_(device|service)_.*` llega por el scrape Y por el empuje, y cada regla " +
			"de flota matchea dos series")
	}
	// EL REGEX Y LA ACCIÓN, JUNTOS. Un `regex:` sin su `action: drop` no descarta nada, y es
	// exactamente la forma en que esta cautela se puede quedar a medias sin que se note.
	if !strings.Contains(plano, `regex: "musubi_fleet_(device|service)_.*" action: drop`) {
		t.Errorf("el descarte no tiene la forma `regex: … action: drop` seguida: un `regex` sin su " +
			"acción no descarta nada, y desde afuera se ve idéntico a uno que sí.")
	}
}
